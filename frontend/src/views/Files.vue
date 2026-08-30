<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card :header="$t('files.title')">
      <el-steps :active="step" simple style="margin-bottom:16px">
        <el-step :title="$t('files.step1')" />
        <el-step :title="$t('files.step2')" />
        <el-step :title="$t('files.step3')" />
      </el-steps>

      <el-form label-width="120px" style="max-width:700px">
        <el-form-item :label="$t('files.localFile')">
          <input type="file" @change="onFileChange" />
          <span v-if="uploadedName" style="color:#67c23a; margin-left:8px">✓ {{ $t('files.uploaded') }}：{{ uploadedName }}</span>
        </el-form-item>
        <el-form-item :label="$t('files.targetHosts')">
          <div style="width:100%">
            <el-tree-select v-model="selectedNodes" :data="treeData" multiple :render-after-expand="false"
                            default-expand-all :placeholder="$t('files.targetHosts')" style="width:100%"
                            node-key="value" :max-collapse-tags="3" collapse-tags />
            <el-input v-model="ipInput" :placeholder="$t('files.ipInputPlaceholder')" style="margin-top:8px" clearable>
              <template #prepend>{{ $t('files.ipInput') }}</template>
            </el-input>
          </div>
        </el-form-item>
        <el-form-item :label="$t('files.remoteDir')">
          <el-input v-model="remoteDir" placeholder="/tmp" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('files.remoteName')">
          <el-input v-model="remoteName" :placeholder="$t('files.remoteNamePlaceholder')" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :disabled="!uploadedName || (!selectedNodes.length && !ipInput.trim()) || !remoteDir"
                     :loading="distributing" @click="distribute">{{ $t('files.start') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card :header="$t('files.progress')" style="margin-top:16px" v-if="taskId">
      <div>{{ $t('files.progressTitle') }} #{{ taskId }} ·
        <el-tag size="small" :type="done ? (failed ? 'danger' : 'success') : 'warning'">
          {{ done ? (failed ? $t('files.doneFailed') : $t('files.done')) : $t('files.distributing') }}
        </el-tag>
      </div>
      <el-table :data="progress" size="small" border style="margin-top:12px">
        <el-table-column prop="result_id" label="ID" width="80" />
        <el-table-column :label="$t('common.status')" min-width="200">
          <template #default="{ row }">
            <el-progress v-if="row.percent !== undefined" :percentage="row.percent" />
            <span v-else style="color:#909399">{{ row.status || $t('files.waiting') }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-button style="margin-top:12px" @click="$router.push(`/tasks?detail=${taskId}`)">{{ $t('files.viewDetail') }}</el-button>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const hosts = ref([])
const selectedNodes = ref([])
const ipInput = ref('')
const remoteDir = ref('/tmp')
const remoteName = ref('')
const uploadedName = ref('')
const step = ref(0)
const distributing = ref(false)
const taskId = ref(null)
const done = ref(false)
const failed = ref(false)
const progress = ref([])
let ws = null

const treeData = computed(() => {
  const byGroup = new Map()
  for (const h of hosts.value) {
    const key = h.group_id ? `g-${h.group_id}` : 'g-none'
    if (!byGroup.has(key)) byGroup.set(key, { value: key, label: h.group?.name || t('hosts.uncategorized'), children: [] })
    byGroup.get(key).children.push({ value: h.id, label: `${h.name} · ${h.ip}` })
  }
  return [...byGroup.values()]
})

const resolveSelected = () => {
  const ids = new Set()
  for (const v of selectedNodes.value) {
    if (typeof v === 'number') { ids.add(v); continue }
    const gid = v === 'g-none' ? null : Number(String(v).slice(2))
    for (const h of hosts.value) {
      if ((gid === null && !h.group_id) || h.group_id === gid) ids.add(h.id)
    }
  }
  return [...ids]
}

const onFileChange = async ev => {
  const file = ev.target.files[0]
  if (!file) return
  const fd = new FormData()
  fd.append('file', file)
  const res = await api.post('/files/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  uploadedName.value = res.file
  step.value = 1
  ElMessage.success(t('files.uploadOk'))
}

const distribute = async () => {
  distributing.value = true
  try {
    const payload = {
      local_file: uploadedName.value,
      remote_dir: remoteDir.value,
      remote_name: remoteName.value || ''
    }
    const ids = resolveSelected()
    if (ids.length) payload.host_ids = ids
    const ips = ipInput.value.split(',').map(s => s.trim()).filter(Boolean)
    if (ips.length) payload.ips = ips
    const res = await api.post('/files/distribute', payload)
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
      if (!p) { p = { result_id: msg.result_id, status: t('files.transferred') }; progress.value.push(p) }
      if (msg.type === 'progress') p.percent = msg.percent
      if (msg.type === 'status') p.status = msg.status
    }
  } finally { distributing.value = false }
}

onMounted(async () => { hosts.value = await api.get('/hosts') })
onUnmounted(() => ws?.close())
</script>
