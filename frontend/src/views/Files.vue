<template>
  <div>
    <el-card header="文件批量分发">
      <el-steps :active="step" simple style="margin-bottom:16px">
        <el-step title="1. 选择文件" />
        <el-step title="2. 选择目标" />
        <el-step title="3. 开始分发" />
      </el-steps>

      <el-form label-width="110px" style="max-width:640px">
        <el-form-item label="本地文件">
          <input type="file" @change="onFileChange" />
          <span v-if="uploadedName" style="color:#67c23a; margin-left:8px">✓ 已上传：{{ uploadedName }}</span>
        </el-form-item>
        <el-form-item label="目标主机">
          <el-select v-model="selectedIds" multiple filterable placeholder="选择主机" style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标目录">
          <el-input v-model="remoteDir" placeholder="/tmp" class="mono" />
        </el-form-item>
        <el-form-item label="重命名(可选)">
          <el-input v-model="remoteName" placeholder="留空保持原文件名" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :disabled="!uploadedName || !selectedIds.length || !remoteDir"
                     :loading="distributing" @click="distribute">开始分发</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card header="分发进度" style="margin-top:16px" v-if="taskId">
      <div>任务 #{{ taskId }} ·
        <el-tag size="small" :type="done ? (failed ? 'danger' : 'success') : 'warning'">
          {{ done ? (failed ? '完成(有失败)' : '完成') : '分发中' }}
        </el-tag>
      </div>
      <el-table :data="progress" size="small" border style="margin-top:12px">
        <el-table-column prop="result_id" label="结果ID" width="80" />
        <el-table-column label="进度" min-width="200">
          <template #default="{ row }">
            <el-progress v-if="row.percent !== undefined" :percentage="row.percent" />
            <span v-else style="color:#909399">{{ row.status || '等待中' }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-button style="margin-top:12px" @click="$router.push(`/tasks?detail=${taskId}`)">查看任务详情</el-button>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

const hosts = ref([])
const selectedIds = ref([])
const remoteDir = ref('/tmp')
const remoteName = ref('')
const uploadedName = ref('')
const uploadedSize = ref(0)
const step = ref(0)
const distributing = ref(false)
const taskId = ref(null)
const done = ref(false)
const failed = ref(false)
const progress = ref([])
let ws = null

const onFileChange = async ev => {
  const file = ev.target.files[0]
  if (!file) return
  const fd = new FormData()
  fd.append('file', file)
  const res = await api.post('/files/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  uploadedName.value = res.file
  uploadedSize.value = res.size
  step.value = 1
  ElMessage.success('文件已上传到服务端')
}

const distribute = async () => {
  distributing.value = true
  try {
    const res = await api.post('/files/distribute', {
      local_file: uploadedName.value,
      host_ids: selectedIds.value,
      remote_dir: remoteDir.value,
      remote_name: remoteName.value || ''
    })
    taskId.value = res.task_id
    progress.value = []
    done.value = false
    failed.value = false
    step.value = 2
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${location.host}/api/ws/task/${taskId.value}?token=${localStorage.getItem('token')}`)
    ws.onmessage = ev => {
      const msg = JSON.parse(ev.data)
      if (msg.type === 'task_done') { done.value = true; failed.value = msg.status === 'failed'; ws.close(); return }
      let p = progress.value.find(x => x.result_id === msg.result_id)
      if (!p) { p = { result_id: msg.result_id, status: '传输中' }; progress.value.push(p) }
      if (msg.type === 'progress') p.percent = msg.percent
      if (msg.type === 'status') p.status = msg.status
    }
  } finally { distributing.value = false }
}

onMounted(async () => { hosts.value = await api.get('/hosts') })
onUnmounted(() => ws?.close())
</script>
