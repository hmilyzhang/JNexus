<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('k8s.title') }}</span>
          <el-popover placement="bottom-end" :width="220" trigger="click">
            <template #reference>
              <el-button size="small" text>{{ $t('k8s.columns') }}</el-button>
            </template>
            <el-checkbox-group v-model="visibleCols" style="display:flex; flex-direction:column; gap:6px">
              <el-checkbox v-for="c in colDefs" :key="c.key" :value="c.key">{{ $t(c.label) }}</el-checkbox>
            </el-checkbox-group>
          </el-popover>
          <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
          <el-button v-if="canManage" size="small" type="primary" @click="openDlg()">{{ $t('k8s.addCluster') }}</el-button>
        </div>
      </template>
      <el-table :data="clusters" v-loading="loading" size="small" border>
        <el-table-column v-if="colOn('cluster')" :label="$t('k8s.cluster')" min-width="170">
          <template #default="{ row }">
            <div style="font-weight:600">{{ row.name }}</div>
            <div class="mono" style="color:#909399; font-size:12px">{{ row.api_server }}</div>
          </template>
        </el-table-column>
        <el-table-column v-if="colOn('description')" :label="$t('scripts.desc')" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.description || '-' }}</template>
        </el-table-column>
        <el-table-column v-if="colOn('support')" :label="$t('k8s.support')" width="110">
          <template #default="{ row }">{{ row.support || '-' }}</template>
        </el-table-column>
        <el-table-column v-if="colOn('status')" :label="$t('k8s.status')" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'info'">
              {{ row.status === 'online' ? $t('k8s.online') : row.status === 'offline' ? $t('k8s.offline') : '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="colOn('version')" :label="$t('k8s.version')" width="110">
          <template #default="{ row }">{{ row.version || '-' }}</template>
        </el-table-column>
        <el-table-column v-if="colOn('nodes')" :label="$t('k8s.nodes')" width="80" align="center">
          <template #default="{ row }">{{ row.node_count || '-' }}</template>
        </el-table-column>
        <el-table-column v-if="colOn('certExpiry')" :label="$t('k8s.certExpiry')" min-width="150">
          <template #default="{ row }">
            <template v-if="row.cert_expiry">
              <el-tag size="small" :type="certTagType(row.cert_expiry)">
                {{ fmtTime(row.cert_expiry) }}<span v-if="daysLeft(row.cert_expiry) <= 30"> ({{ daysLeft(row.cert_expiry) }}d)</span>
              </el-tag>
            </template>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column v-if="colOn('myRole')" :label="$t('k8s.myRole')" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.my_role === 'admin' ? 'warning' : 'info'">{{ roleText(row.my_role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link type="primary" @click="$router.push(`/k8s/manage/${row.id}`)">{{ $t('k8s.manage') }}</el-button>
            <el-button size="small" link type="primary" @click="testNow(row)">{{ $t('k8s.testNow') }}</el-button>
            <el-button v-if="row.my_role === 'admin'" size="small" link @click="openMembers(row)">{{ $t('k8s.members') }}</el-button>
            <el-button v-if="row.my_role === 'admin'" size="small" link @click="openDlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm v-if="row.my_role === 'admin'" :title="$t('k8s.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="color:#909399; font-size:12px; margin-top:10px">{{ $t('k8s.certTip') }}</div>
    </el-card>

    <!-- 添加/编辑集群 -->
    <el-dialog v-model="dlgVisible" :title="form.id ? $t('common.edit') : $t('k8s.addCluster')" width="560px">
      <el-form label-width="120px">
        <el-form-item :label="$t('k8s.clusterName')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('k8s.apiServer')">
          <el-input v-model="form.api_server" class="mono" placeholder="https://192.168.1.10:6443" />
        </el-form-item>
        <el-form-item :label="$t('k8s.support')"><el-input v-model="form.support" /></el-form-item>
        <el-form-item :label="$t('scripts.desc')"><el-input v-model="form.description" /></el-form-item>
        <el-divider content-position="left">{{ $t('k8s.authSection') }}</el-divider>
        <el-form-item :label="$t('k8s.kubeconfig')">
          <el-input v-model="form.kubeconfig" type="textarea" :rows="4" class="mono"
                    :placeholder="$t('k8s.kubeconfigTip')" />
        </el-form-item>
        <el-divider content-position="left">{{ $t('k8s.trioSection') }}</el-divider>
        <el-form-item label="CA"><el-input v-model="form.ca" type="textarea" :rows="2" class="mono" placeholder="PEM" /></el-form-item>
        <el-form-item :label="$t('k8s.clientCert')"><el-input v-model="form.client_cert" type="textarea" :rows="2" class="mono" placeholder="PEM" /></el-form-item>
        <el-form-item :label="$t('k8s.clientKey')"><el-input v-model="form.client_key" type="textarea" :rows="2" class="mono" placeholder="PEM" /></el-form-item>
        <el-form-item v-if="form.id" :label="$t('system.apiKeyEnabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 成员管理 -->
    <el-dialog v-model="memberDlgVisible" :title="`${$t('k8s.members')}：${memberCluster?.name}`" width="520px">
      <div v-for="(m, i) in memberRows" :key="i" style="display:flex; gap:8px; align-items:center; margin-bottom:8px">
        <el-select v-model="m.user_id" filterable size="small" style="flex:1" :placeholder="$t('users.username')">
          <el-option v-for="u in users" :key="u.id" :label="u.username" :value="u.id" />
        </el-select>
        <el-select v-model="m.role" size="small" style="width:130px">
          <el-option value="admin" :label="$t('k8s.roleAdmin')" />
          <el-option value="user" :label="$t('k8s.roleUser')" />
          <el-option value="viewer" :label="$t('k8s.roleViewer')" />
        </el-select>
        <el-button type="danger" link size="small" @click="memberRows.splice(i, 1)">{{ $t('apps.remove') }}</el-button>
      </div>
      <el-button size="small" @click="memberRows.push({ user_id: null, role: 'user' })">{{ $t('k8s.memberAdd') }}</el-button>
      <template #footer>
        <el-button @click="memberDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveMembers">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const clusters = ref([])
const users = ref([])
const loading = ref(false)
const saving = ref(false)
const dlgVisible = ref(false)
const memberDlgVisible = ref(false)
const memberCluster = ref(null)
const memberRows = ref([])
const form = reactive({})
const canManage = computed(() => store.isAdmin)

// ---- 列自定义（localStorage 持久化） ----
const colDefs = [
  { key: 'cluster', label: 'k8s.cluster' },
  { key: 'description', label: 'scripts.desc' },
  { key: 'support', label: 'k8s.support' },
  { key: 'status', label: 'k8s.status' },
  { key: 'version', label: 'k8s.version' },
  { key: 'nodes', label: 'k8s.nodes' },
  { key: 'certExpiry', label: 'k8s.certExpiry' },
  { key: 'myRole', label: 'k8s.myRole' },
]
const COL_STORE = 'k8s_cluster_cols'
const visibleCols = ref(JSON.parse(localStorage.getItem(COL_STORE) || 'null') ||
  ['cluster', 'description', 'support', 'status', 'version', 'nodes', 'certExpiry', 'myRole'])
const colOn = k => visibleCols.value.includes(k)

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const daysLeft = v => {
  if (!v) return 9999
  return Math.floor((new Date(v).getTime() - Date.now()) / 86400000)
}
const certTagType = v => (daysLeft(v) <= 7 ? 'danger' : daysLeft(v) <= 30 ? 'warning' : 'success')
const roleText = r => ({ admin: t('k8s.roleAdmin'), user: t('k8s.roleUser'), viewer: t('k8s.roleViewer') }[r] || '-')

watch(visibleCols, v => localStorage.setItem(COL_STORE, JSON.stringify(v)), { deep: true })

const load = async () => {
  loading.value = true
  try {
    clusters.value = await api.get('/k8s/clusters')
  } finally { loading.value = false }
}

const openDlg = row => {
  Object.assign(form, row || {}, {
    kubeconfig: '', ca: '', client_cert: '', client_key: '',
    enabled: row ? !!row.enabled : true,
  })
  dlgVisible.value = true
}
const save = async () => {
  if (!form.name || !form.api_server) { ElMessage.warning(t('k8s.nameServerRequired')); return }
  const payload = { ...form }
  if (form.id) await api.put(`/k8s/clusters/${form.id}`, payload)
  else {
    const r = await api.post('/k8s/clusters', payload)
    ElMessage.success(`${t('k8s.addCluster')} OK · v${r.version || '?'} / ${r.node_count || 0} ${t('k8s.nodes')}`)
  }
  dlgVisible.value = false
  load()
}
const del = async row => {
  await api.delete(`/k8s/clusters/${row.id}`)
  ElMessage.success(t('common.success'))
  load()
}
const testNow = async row => {
  const r = await api.post(`/k8s/clusters/${row.id}/test`)
  ElMessage.success(`${t('k8s.testOk')} · v${r.version || '?'} / ${r.node_count || 0} ${t('k8s.nodes')}`)
  load()
}

// ---- 成员管理 ----
const openMembers = async row => {
  memberCluster.value = row
  const ms = await api.get(`/k8s/clusters/${row.id}/members`)
  memberRows.value = ms.map(m => ({ user_id: m.user_id, role: m.role }))
  if (!users.value.length) users.value = await api.get('/users')
  memberDlgVisible.value = true
}
const saveMembers = async () => {
  await api.put(`/k8s/clusters/${memberCluster.value.id}/members`,
    { members: memberRows.value.filter(m => m.user_id) })
  ElMessage.success(t('common.success'))
  memberDlgVisible.value = false
  load()
}

onMounted(load)
</script>
