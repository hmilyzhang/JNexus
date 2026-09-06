<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
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
                            :default-expand-all="false" :placeholder="$t('files.targetHosts')" style="width:100%"
                            node-key="value" :max-collapse-tags="3" collapse-tags />
            <el-input v-model="ipInput" :placeholder="$t('files.ipInputPlaceholder')" style="margin-top:8px" clearable>
              <template #prepend>{{ $t('files.ipInput') }}</template>
            </el-input>
          </div>
        </el-form-item>
        <el-form-item :label="$t('hosts.credOsAccount')">
          <el-select v-model="credentialId" style="width:100%" clearable :placeholder="$t('hosts.credSelectPlaceholder')">
            <el-option v-for="c in usableCreds" :key="c.id"
                       :label="`${c.host_name} · ${c.host_ip} — ${c.username}${c.label ? '（' + c.label + '）' : ''}`" :value="c.id" />
          </el-select>
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
const credentialId = ref(null)
const usableCreds = ref([])
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

const groups = ref([])

// 多级分组树：分组按 parent_id 嵌套，主机挂到所在分组节点
const treeData = computed(() => {
  const byId = new Map(groups.value.map(g => [g.id, { value: `g-${g.id}`, label: g.name, children: [] }]))
  const roots = []
  for (const g of groups.value) {
    const node = byId.get(g.id)
    if (g.parent_id && byId.has(g.parent_id)) byId.get(g.parent_id).children.push(node)
    else roots.push(node)
  }
  for (const h of hosts.value) {
    const leaf = { value: h.id, label: `${h.name} · ${h.ip}` }
    if (h.group_id && byId.has(h.group_id)) byId.get(h.group_id).children.push(leaf)
    else roots.push(leaf)
  }
  return roots
})

const resolveSelected = () => {
  const ids = new Set()
  const descendants = gid => {
    const out = new Set([gid])
    let changed = true
    while (changed) {
      changed = false
      for (const g of groups.value) {
        if (g.parent_id && out.has(g.parent_id) && !out.has(g.id)) { out.add(g.id); changed = true }
      }
    }
    return out
  }
  for (const v of selectedNodes.value) {
    if (typeof v === 'number') { ids.add(v); continue }
    if (v === 'g-none') continue
    const gset = descendants(Number(String(v).slice(2)))
    for (const h of hosts.value) {
      if (h.group_id && gset.has(h.group_id)) ids.add(h.id)
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
    if (credentialId.value) payload.credential_id = credentialId.value
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

onMounted(async () => {
  hosts.value = await api.get('/hosts')
  usableCreds.value = await api.get('/credentials/usable')
  groups.value = await api.get('/host_groups')
})
onUnmounted(() => ws?.close())
</script>
