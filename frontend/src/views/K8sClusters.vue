<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('k8s.title') }}</span>
          <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
          <el-button v-if="canManage" size="small" type="primary" @click="openDlg()">{{ $t('k8s.addCluster') }}</el-button>
        </div>
      </template>
      <el-table :data="clusters" v-loading="loading" size="small" border>
        <el-table-column :label="$t('k8s.cluster')" min-width="170">
          <template #default="{ row }">
            <div style="font-weight:600">{{ row.name }}</div>
            <div class="mono" style="color:#909399; font-size:12px">{{ row.api_server }}</div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('k8s.support')" width="110">
          <template #default="{ row }">{{ row.support || '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('k8s.status')" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'info'">
              {{ row.status === 'online' ? $t('k8s.online') : row.status === 'offline' ? $t('k8s.offline') : '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('k8s.version')" width="110">
          <template #default="{ row }">{{ row.version || '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('k8s.nodes')" width="80" align="center">
          <template #default="{ row }">{{ row.node_count || '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('k8s.certExpiry')" min-width="150">
          <template #default="{ row }">
            <template v-if="row.cert_expiry">
              <el-tag size="small" :type="certTagType(row.cert_expiry)">
                {{ fmtTime(row.cert_expiry) }}<span v-if="daysLeft(row.cert_expiry) <= 30">（{{ daysLeft(row.cert_expiry) }}天）</span>
              </el-tag>
            </template>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('k8s.myRole')" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.my_role === 'admin' ? 'warning' : 'info'">{{ roleText(row.my_role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link type="primary" @click="openDetail(row)">{{ $t('k8s.manage') }}</el-button>
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

    <!-- 集群详情抽屉 -->
    <el-drawer v-model="detailVisible" size="72%" :title="$t('k8s.clusterDetail') + '：' + (detailCluster?.name || '')">
      <div style="display:flex; gap:10px; align-items:center; margin-bottom:12px">
        <el-select v-model="detailNs" size="small" style="width:200px" clearable :placeholder="$t('k8s.allNamespaces')">
          <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
        </el-select>
        <el-radio-group v-model="detailTab" size="small">
          <el-radio-button value="pods">Pods</el-radio-button>
          <el-radio-button value="deployments">Deployments</el-radio-button>
          <el-radio-button value="nodes">Nodes</el-radio-button>
        </el-radio-group>
        <span style="flex:1"></span>
        <el-tag size="small" :type="detailCluster?.status === 'online' ? 'success' : 'danger'">{{ detailCluster?.status }}</el-tag>
      </div>
      <div v-if="detailTab === 'nodes'">
        <el-table :data="nodes" size="small" border v-loading="detailLoading">
          <el-table-column prop="name" label="Node" min-width="140" />
          <el-table-column prop="roles" label="Roles" min-width="120" />
          <el-table-column prop="internal_ip" label="IP" width="130" />
          <el-table-column prop="status" :label="$t('k8s.status')" width="90" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'Ready' ? 'success' : 'danger'">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="version" :label="$t('k8s.version')" width="110" />
        </el-table>
      </div>
      <div v-if="detailTab === 'pods'">
        <el-table :data="detailPods" size="small" border v-loading="detailLoading">
          <el-table-column :label="$t('k8s.podName')" min-width="180">
            <template #default="{ row }">{{ row.name }}</template>
          </el-table-column>
          <el-table-column prop="namespace" label="Namespace" width="120" />
          <el-table-column prop="status" :label="$t('k8s.status')" width="130" />
          <el-table-column prop="node" label="Node" min-width="110" />
          <el-table-column prop="ip" label="IP" width="120" />
          <el-table-column prop="restarts" :label="$t('k8s.restarts')" width="90" align="center" />
          <el-table-column :label="$t('common.operation')" width="150" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link type="primary" @click="showLog(row)">{{ $t('k8s.logs') }}</el-button>
              <el-popconfirm v-if="myRole(row) !== 'viewer'" :title="$t('k8s.podDelConfirm')" @confirm="deletePodAction(row)">
                <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div v-if="detailTab === 'deployments'">
        <el-table :data="detailDeps" size="small" border v-loading="detailLoading">
          <el-table-column :label="$t('k8s.deployment')" min-width="180">
            <template #default="{ row }">{{ row.name }}<div class="mono" style="color:#909399; font-size:12px">{{ row.namespace }}</div></template>
          </el-table-column>
          <el-table-column :label="$t('k8s.replicas')" width="120" align="center">
            <template #default="{ row }">{{ row.ready }}/{{ row.replicas }}</template>
          </el-table-column>
          <el-table-column :label="$t('common.operation')" width="130" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link type="warning" @click="restartDeployment(row)">{{ $t('k8s.restart') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>

    <!-- Pod 日志 -->
    <el-dialog v-model="logVisible" :title="logTitle" width="820px">
      <pre class="mono" style="background:#1e2a35; color:#d8e4f0; padding:14px; border-radius:6px; max-height:480px; overflow:auto; font-size:12px; line-height:1.6">{{ logText }}</pre>
    </el-dialog>

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

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const daysLeft = v => {
  if (!v) return 9999
  return Math.floor((new Date(v).getTime() - Date.now()) / 86400000)
}
const certTagType = v => (daysLeft(v) <= 7 ? 'danger' : daysLeft(v) <= 30 ? 'warning' : 'success')
const roleText = r => ({ admin: t('k8s.roleAdmin'), user: t('k8s.roleUser'), viewer: t('k8s.roleViewer') }[r] || '-')

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

// ---- 集群详情（节点 / Pods / Deployments） ----
const detailVisible = ref(false)
const detailCluster = ref(null)
const detailTab = ref('pods')
const detailNs = ref('')
const detailLoading = ref(false)
const detailPods = ref([])
const detailNodes = ref([])
const detailDeps = ref([])
const namespaces = ref([])
const logVisible = ref(false)
const logTitle = ref('')
const logText = ref('')

const myRole = row => (row.my_role === 'viewer' ? 'viewer' : 'user')

const openDetail = async row => {
  detailCluster.value = row
  detailTab.value = 'pods'
  detailNs.value = ''
  detailVisible.value = true
  await loadDetail()
}
const loadDetail = async () => {
  if (!detailCluster.value) return
  const id = detailCluster.value.id
  detailLoading.value = true
  try {
    const q = detailNs.value ? { namespace: detailNs.value } : {}
    const reqs = {
      pods: api.get(`/k8s/clusters/${id}/pods`, { params: q }),
      deps: api.get(`/k8s/clusters/${id}/deployments`, { params: q }),
      nodes: api.get(`/k8s/clusters/${id}/nodes`),
      nss: api.get(`/k8s/clusters/${id}/namespaces`),
    }
    const [pods, deps, nodes, nss] = await Promise.all([reqs.pods, reqs.deps, reqs.nodes, reqs.nss])
    detailPods.value = pods
    detailDeps.value = deps
    detailNodes.value = nodes
    namespaces.value = nss
  } finally { detailLoading.value = false }
}
watch(detailNs, () => loadDetail())

const showLog = async row => {
  const q = { namespace: row.namespace, pod: row.name }
  const r = await api.get(`/k8s/clusters/${detailCluster.value.id}/podlog`, { params: q })
  logTitle.value = `${row.namespace}/${row.name}`
  logText.value = r.log || t('k8s.noLog')
  logVisible.value = true
}
const deletePodAction = async row => {
  await api.delete(`/k8s/clusters/${detailCluster.value.id}/pods/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  loadDetail()
}

const restartDeployment = async row => {
  await api.post(`/k8s/clusters/${detailCluster.value.id}/deployments/${row.namespace}/${row.name}/restart`)
  ElMessage.success(t('common.success'))
  loadDetail()
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
