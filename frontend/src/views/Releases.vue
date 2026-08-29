<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center">
        <el-select v-model="appId" placeholder="按应用筛选" clearable style="width:200px" @change="load">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-button type="primary" @click="releaseDlg">发起发布</el-button>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-table :data="releases" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="app_name" label="应用" width="140" />
        <el-table-column prop="package_name" label="发布包" min-width="180" show-overflow-tooltip />
        <el-table-column prop="operator" label="操作人" width="110" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : row.status === 'rollback' ? 'warning' : 'primary'">
              {{ row.status === 'rollback' ? '回滚' : row.status === 'success' ? '成功' : row.status === 'failed' ? '失败' : '进行中' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-popconfirm v-if="row.status === 'failed'" title="回滚到最近一次备份?" @confirm="rollback(row)">
              <template #reference><el-button size="small" type="warning" link>回滚</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 发起发布 -->
    <el-dialog v-model="relVisible" title="发起发布" width="520px">
      <el-form label-width="90px">
        <el-form-item label="应用">
          <el-select v-model="relForm.app_id" style="width:100%">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="发布包">
          <input type="file" @change="onFile" />
        </el-form-item>
        <el-form-item v-if="relForm.package_name" label="确认">
          <el-alert type="info" :closable="false"
            :title="`将把 ${relForm.package_name} 发布到所选应用绑定的全部主机：停服务 → 备份 → 替换 → 启动 → 健康检查`" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="relVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!relForm.package_file" :loading="releasing" @click="doRelease">开始发布</el-button>
      </template>
    </el-dialog>

    <!-- 发布详情（实时） -->
    <el-drawer v-model="detailVisible" :title="`发布单 #${relId}`" size="680px">
      <div v-for="item in items" :key="item.id" style="margin-bottom:16px">
        <div style="font-weight:600; font-size:14px; margin-bottom:6px">
          {{ item.host_name }}（{{ item.host_ip }}）
          <el-tag size="small" :type="item.status === 'success' ? 'success' : item.status === 'failed' ? 'danger' : item.status === 'running' ? 'warning' : 'info'">
            {{ stepLabel(item.step) }} · {{ item.status }}
          </el-tag>
        </div>
        <div class="log-box" style="max-height:160px">{{ item.log || '(等待执行)' }}</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

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
  pending: '待执行', stop: '停止服务', backup: '备份', upload: '上传', start: '启动', health: '健康检查'
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
    ElMessage.success(`发布单 #${res.release_id} 已创建`)
    openDetail({ id: res.release_id })
    load()
  } finally { releasing.value = false }
}
const rollback = async row => {
  const res = await api.post(`/releases/${row.id}/rollback`)
  ElMessage.success(`已发起回滚，发布单 #${res.release_id}`)
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
