<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center">
        <el-select v-model="appId" :placeholder="$t('releases.filterByApp')" clearable style="width:200px" @change="load">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-button type="primary" @click="releaseDlg">{{ $t('releases.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="releases" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="app_name" :label="$t('releases.app')" width="140" />
        <el-table-column prop="package_name" :label="$t('releases.pkg')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="operator" :label="$t('releases.operator')" width="110" />
        <el-table-column :label="$t('tasks.status')" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : row.status === 'rollback' ? 'warning' : 'primary'">
              {{ { success: $t('releases.statusSuccess'), failed: $t('releases.statusFailed'), rollback: $t('releases.statusRollback') }[row.status] || $t('releases.statusRunning') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('releases.time')" width="170" />
        <el-table-column :label="$t('common.operation')" width="140" fixed="right">
          <template #default="{ row }">
            <el-popconfirm v-if="row.status === 'failed'" :title="$t('releases.rollbackConfirm')" @confirm="rollback(row)">
              <template #reference><el-button size="small" type="warning" link>{{ $t('releases.rollbackBtn') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Start a release -->
    <el-dialog v-model="relVisible" :title="$t('releases.create')" width="520px">
      <el-form label-width="110px">
        <el-form-item :label="$t('releases.app')">
          <el-select v-model="relForm.app_id" style="width:100%">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('releases.pkgName')">
          <input type="file" @change="onFile" />
        </el-form-item>
        <el-form-item v-if="relForm.package_name" :label="$t('common.confirm')">
          <el-alert type="info" :closable="false" :title="$t('releases.confirmMsg', { pkg: relForm.package_name })" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="relVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :disabled="!relForm.package_file" :loading="releasing" @click="doRelease">{{ $t('releases.create') }}</el-button>
      </template>
    </el-dialog>

    <!-- Release detail (live) -->
    <el-drawer v-model="detailVisible" :title="`${$t('releases.releaseOrder')} #${relId}`" size="680px">
      <div v-for="item in items" :key="item.id" style="margin-bottom:16px">
        <div style="font-weight:600; font-size:14px; margin-bottom:6px">
          {{ item.host_name }}（{{ item.host_ip }}）
          <el-tag size="small" :type="item.status === 'success' ? 'success' : item.status === 'failed' ? 'danger' : item.status === 'running' ? 'warning' : 'info'">
            {{ stepLabel(item.step) }} · {{ item.status }}
          </el-tag>
        </div>
        <div class="log-box" style="max-height:160px">{{ item.log || $t('releases.waiting') }}</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const apps = ref([])
const releases = ref([])
const appId = ref(null)
const loading = ref(false)
const relVisible = ref(false)
const relForm = ref({ app_id: null, package_file: '', package_name: '' })
const releasing = ref(false)
const detailVisible = ref(false)
const relId = ref(null)
const items = ref([])
let ws = null

const stepLabel = s => ({
  pending: t('releases.stepPending'), stop: t('releases.stepStop'), backup: t('releases.stepBackup'),
  upload: t('releases.stepUpload'), start: t('releases.stepStart'), health: t('releases.stepHealth')
}[s] || s)

const load = async () => {
  loading.value = true
  try {
    releases.value = await api.get('/releases', { params: appId.value ? { app_id: appId.value } : {} })
  } finally { loading.value = false }
}
onMounted(async () => { load(); apps.value = await api.get('/apps') })
onUnmounted(() => ws?.close())

const releaseDlg = () => {
  relForm.value = { app_id: apps.value[0]?.id || null, package_file: '', package_name: '' }
  relVisible.value = true
}
const onFile = async ev => {
  const file = ev.target.files[0]
  if (!file) return
  const fd = new FormData()
  fd.append('file', file)
  const res = await api.post('/files/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  relForm.value.package_file = res.file
  relForm.value.package_name = res.file
}
const doRelease = async () => {
  releasing.value = true
  try {
    const res = await api.post('/releases', relForm.value)
    relVisible.value = false
    ElMessage.success(t('releases.created', { id: res.release_id }))
    openDetail({ id: res.release_id })
    load()
  } finally { releasing.value = false }
}
const rollback = async row => {
  const res = await api.post(`/releases/${row.id}/rollback`)
  ElMessage.success(t('releases.rollbackCreated', { id: res.release_id }))
  openDetail({ id: res.release_id })
  load()
}

const openDetail = async row => {
  relId.value = row.id
  const data = await api.get(`/releases/${row.id}`)
  items.value = data.items
  detailVisible.value = true
  ws?.close()
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/release/${row.id}?token=${localStorage.getItem('token')}`)
  ws.onmessage = ev => {
    const msg = JSON.parse(ev.data)
    if (msg.type === 'item') {
      const idx = items.value.findIndex(x => x.id === msg.item.id)
      if (idx >= 0) items.value[idx] = msg.item
      else items.value.push(msg.item)
    } else if (msg.type === 'release_done') {
      load()
    }
  }
}
</script>
