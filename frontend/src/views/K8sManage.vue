<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="km-page">
    <!-- 左侧集群菜单 -->
    <aside class="km-side">
      <div class="km-brand">JNexus <span>K8S</span></div>
      <div class="km-cluster mono" v-if="cluster">{{ cluster.name }}</div>
      <el-menu :default-active="active" class="km-menu" background-color="transparent"
               text-color="#a7b1c2" active-text-color="#ffffff" @select="k => (active = k)">
        <el-menu-item index="overview">
          <el-icon><Odometer /></el-icon><span>{{ $t('k8s.navOverview') }}</span>
        </el-menu-item>
        <el-sub-menu index="grp-cluster">
          <template #title><el-icon><OfficeBuilding /></el-icon><span>{{ $t('k8s.navCluster') }}</span></template>
          <el-menu-item index="nodes">Nodes</el-menu-item>
          <el-menu-item index="namespaces">{{ $t('k8s.namespaces') }}</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-workloads">
          <template #title><el-icon><Box /></el-icon><span>{{ $t('k8s.navWorkloads') }}</span></template>
          <el-menu-item index="pods">Pods</el-menu-item>
          <el-menu-item index="deployments">Deployments</el-menu-item>
          <el-menu-item index="daemonsets">DaemonSets</el-menu-item>
          <el-menu-item index="statefulsets">StatefulSets</el-menu-item>
          <el-menu-item index="jobs">Jobs</el-menu-item>
          <el-menu-item index="cronjobs">CronJobs</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-discovery">
          <template #title><el-icon><Connection /></el-icon><span>{{ $t('k8s.navDiscovery') }}</span></template>
          <el-menu-item index="services">Services</el-menu-item>
          <el-menu-item index="ingresses">Ingresses</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-config">
          <template #title><el-icon><Setting /></el-icon><span>{{ $t('k8s.navConfig') }}</span></template>
          <el-menu-item index="configmaps">ConfigMaps</el-menu-item>
          <el-menu-item index="secrets">Secrets</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-storage">
          <template #title><el-icon><Coin /></el-icon><span>{{ $t('k8s.navStorage') }}</span></template>
          <el-menu-item index="pvcs">PVCs</el-menu-item>
          <el-menu-item index="pvs">PVs</el-menu-item>
          <el-menu-item index="storageclasses">StorageClasses</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-access">
          <template #title><el-icon><Key /></el-icon><span>{{ $t('k8s.navAccess') }}</span></template>
          <el-menu-item index="serviceaccounts">ServiceAccounts</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <!-- 主区域 -->
    <main class="km-main">
      <header class="km-top">
        <el-button size="small" @click="$router.push('/k8s')">
          <el-icon style="margin-right:4px"><Back /></el-icon>{{ $t('k8s.backToList') }}
        </el-button>
        <b style="font-size:15px">{{ cluster?.name || '...' }}</b>
        <el-tag v-if="summary?.version" size="small" type="info" class="mono">{{ summary.version }}</el-tag>
        <el-tag v-if="cluster" size="small" :type="cluster.status === 'online' ? 'success' : 'danger'">
          {{ cluster.status === 'online' ? $t('k8s.online') : $t('k8s.offline') }}
        </el-tag>
        <span style="flex:1"></span>
        <el-select v-model="ns" size="small" clearable style="width:190px" :placeholder="$t('k8s.allNamespaces')"
                   :disabled="!namespacedActive">
          <el-option v-for="n in namespaces" :key="n" :label="n" :value="n" />
        </el-select>
        <el-tag v-if="cluster" size="small" :type="cluster.my_role === 'admin' ? 'warning' : cluster.my_role === 'user' ? 'success' : 'info'">
          {{ roleText(cluster.my_role) }}
        </el-tag>
        <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
      </header>

      <section class="km-body">
        <!-- 概览 -->
        <template v-if="active === 'overview'">
          <div class="km-cards">
            <div v-for="card in cards" :key="card.key" class="km-card" :style="{ '--c': card.color }" @click="active = card.go">
              <div class="km-card-num">{{ card.text }}</div>
              <div class="km-card-label">{{ card.label }}</div>
            </div>
          </div>
          <h4 class="km-h4">{{ $t('k8s.recentEvents') }}</h4>
          <el-table :data="events" size="small" border v-loading="loading">
            <el-table-column prop="namespace" label="Namespace" width="110" />
            <el-table-column :label="$t('monitor.evType')" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.type === 'Warning' ? 'warning' : 'info'">{{ row.type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="Reason" width="140" />
            <el-table-column prop="object" label="Object" min-width="150" />
            <el-table-column prop="message" :label="$t('audit.detail')" min-width="240" show-overflow-tooltip />
            <el-table-column prop="count" label="#" width="60" align="center" />
            <el-table-column :label="$t('audit.time')" width="160">
              <template #default="{ row }">{{ fmtTime(row.last_seen) }}</template>
            </el-table-column>
          </el-table>
        </template>

        <!-- 资源列表（通用表格） -->
        <template v-else>
          <div class="km-toolbar">
            <h4 class="km-h4" style="margin:0">{{ sectionTitle }}</h4>
            <span style="flex:1"></span>
            <template v-if="active === 'cronjobs' && canOp">
              <el-button size="small" type="primary" @click="openCronDlg">{{ $t('k8s.cronAdd') }}</el-button>
            </template>
            <template v-if="active === 'serviceaccounts' && canOp">
              <el-button size="small" type="primary" @click="openSaDlg">{{ $t('k8s.saAdd') }}</el-button>
            </template>
          </div>
          <el-table :data="rows" size="small" border v-loading="loading">
            <el-table-column v-for="col in cols" :key="col.prop" :min-width="col.w || 120" :align="col.align">
              <template #header><span v-if="col.headerHtml" v-html="col.headerHtml"></span><span v-else>{{ colLabel(col) }}</span></template>
              <template #default="{ row }">
                <el-tag v-if="col.tag" size="small" :type="col.tag(row)">
                  {{ col.text ? col.text(row) : row[col.prop] }}
                </el-tag>
                <template v-else>{{ col.text ? col.text(row) : row[col.prop] }}</template>
              </template>
            </el-table-column>
            <el-table-column v-if="hasOps" :label="$t('common.operation')" :width="opWidth" fixed="right">
              <template #default="{ row }">
                <!-- Pods -->
                <template v-if="active === 'pods'">
                  <el-button size="small" link type="primary" @click="showLog(row)">{{ $t('k8s.logs') }}</el-button>
                  <el-button size="small" link type="success" @click="openShell(row)">Shell</el-button>
                  <el-popconfirm v-if="canOp" :title="$t('k8s.podDelConfirm')" @confirm="deletePod(row)">
                    <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                  </el-popconfirm>
                </template>
                <!-- Deployments -->
                <el-button v-if="active === 'deployments' && canOp" size="small" link type="warning" @click="restartDeployment(row)">
                  {{ $t('k8s.restart') }}
                </el-button>
                <!-- CronJobs -->
                <template v-if="active === 'cronjobs' && canOp">
                  <el-button size="small" link type="warning" @click="toggleCron(row)">
                    {{ row.suspend ? $t('k8s.cronResume') : $t('k8s.cronSuspendBtn') }}
                  </el-button>
                  <el-popconfirm :title="$t('k8s.cronDelConfirm')" @confirm="deleteCron(row)">
                    <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                  </el-popconfirm>
                </template>
                <!-- ConfigMaps / Secrets / ServiceAccounts -->
                <el-popconfirm v-if="delConfirmKey && canOp" :title="$t(delConfirmKey)" @confirm="deleteResource(row)">
                  <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </template>
      </section>
    </main>

    <!-- Pod 日志 -->
    <el-dialog v-model="logVisible" :title="logTitle" width="820px">
      <pre class="mono" style="background:#1e2a35; color:#d8e4f0; padding:14px; border-radius:6px; max-height:480px; overflow:auto; font-size:12px; line-height:1.6">{{ logText }}</pre>
    </el-dialog>

    <!-- CronJob 创建 -->
    <el-dialog v-model="cronDlgVisible" :title="$t('k8s.cronAdd')" width="520px">
      <el-form label-width="110px">
        <el-form-item :label="$t('k8s.cronName')"><el-input v-model="cronForm.name" /></el-form-item>
        <el-form-item label="Namespace"><el-input v-model="cronForm.namespace" placeholder="default" /></el-form-item>
        <el-form-item label="Schedule"><el-input v-model="cronForm.schedule" class="mono" placeholder="*/5 * * * *" /></el-form-item>
        <el-form-item :label="$t('k8s.cronCommand')">
          <el-input v-model="cronForm.command" type="textarea" :rows="2" class="mono" placeholder="echo hello" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cronDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createCron">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- SA 创建 -->
    <el-dialog v-model="saDlgVisible" :title="$t('k8s.saAdd')" width="460px">
      <el-form label-width="110px">
        <el-form-item :label="$t('k8s.saName')"><el-input v-model="saForm.name" /></el-form-item>
        <el-form-item label="Namespace"><el-input v-model="saForm.namespace" placeholder="default" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="saDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createSa">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Odometer, OfficeBuilding, Box, Connection, Setting, Coin, Key, Back } from '@element-plus/icons-vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const route = useRoute()
const id = route.params.id
const P = `/k8s/clusters/${id}`

const cluster = ref(null)
const summary = ref(null)
const events = ref([])
const rows = ref([])
const namespaces = ref([])
const ns = ref('')
const active = ref('overview')
const loading = ref(false)

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const roleText = r => ({ admin: t('k8s.roleAdmin'), user: t('k8s.roleUser'), viewer: t('k8s.roleViewer') }[r] || '-')
const canOp = computed(() => !!cluster.value && cluster.value.my_role !== 'viewer')
const namespacedActive = !['nodes', 'pvs', 'storageclasses', 'overview'].includes(active.value)
const nsParams = () => (ns.value ? { params: { namespace: ns.value } } : {})

// ---- 各区块表格列定义（label 为 i18n key 或字面量） ----
const statusTag = v => (v === 'Ready' || v === 'Bound' || v === 'Available' || v === 'Active' || v === 'Running' ? 'success' : v === 'Pending' ? 'warning' : 'danger')
const colDefs = {
  nodes: [
    { prop: 'name', w: 160 }, { prop: 'roles', w: 140 }, { prop: 'internal_ip', label: 'IP', w: 130 },
    { prop: 'status', tag: row => statusTag(row.status), w: 100, align: 'center' }, { prop: 'version', w: 110 },
  ],
  namespaces: [{ prop: 'name', w: 200 }],
  pods: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'status', tag: row => statusTag(row.status), w: 130 },
    { prop: 'node', w: 130 }, { prop: 'ip', label: 'IP', w: 120 },
    { prop: 'restarts', label: 'k8s.restarts', w: 90, align: 'center' },
  ],
  deployments: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'replicas', label: 'k8s.replicas', w: 100, align: 'center', text: row => `${row.ready}/${row.replicas}` },
  ],
  daemonsets: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  statefulsets: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  jobs: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'status', tag: row => (row.status === 'Running' ? 'warning' : row.status === 'Succeeded' ? 'success' : 'danger'), w: 110, align: 'center' },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  cronjobs: [
    { prop: 'name', w: 160 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'schedule', w: 130 }, { prop: 'active', label: 'k8s.cronActive', w: 70, align: 'center' },
    { prop: 'suspend', label: 'k8s.cronSuspend', w: 100, align: 'center', tag: row => (row.suspend ? 'warning' : 'success'), text: row => (row.suspend ? 'Suspended' : 'Active') },
  ],
  services: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'type', label: 'k8s.svcType', w: 110 },
    { prop: 'cluster_ip', label: 'Cluster IP', w: 130 }, { prop: 'ports', label: 'k8s.ports', w: 120 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  ingresses: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'hosts', label: 'k8s.hosts', w: 220 }, { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  configmaps: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 130 },
    { prop: 'data_keys', label: 'monitor.dataKeys', w: 100, align: 'center' },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  secrets: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 130 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  pvcs: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'phase', tag: row => statusTag(row.phase), w: 100, align: 'center' },
    { prop: 'capacity', label: 'k8s.capacity', w: 100 }, { prop: 'storage_class', label: 'StorageClass', w: 140 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  pvs: [
    { prop: 'name', w: 180 },
    { prop: 'phase', tag: row => statusTag(row.phase), w: 100, align: 'center' },
    { prop: 'capacity', label: 'k8s.capacity', w: 100 }, { prop: 'claim', label: 'k8s.claim', w: 180 },
    { prop: 'storage_class', label: 'StorageClass', w: 140 }, { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  storageclasses: [
    { prop: 'name', w: 200 }, { prop: 'provisioner', label: 'k8s.provisioner', w: 240 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  serviceaccounts: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 160 },
    { prop: 'secrets', label: 'Secrets', w: 100, align: 'center' },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
}
const cols = computed(() => (colDefs[active.value] || []).map(c => ({ ...c, headerHtml: c.headerHtml })))
const colLabel = col => (col.label ? (col.label.includes('.') ? t(col.label) : col.label) : col.prop.replace(/_/g, ' ').toUpperCase())
const sectionTitle = computed(() => {
  const names = {
    nodes: 'Nodes', namespaces: t('k8s.namespaces'), pods: 'Pods', deployments: 'Deployments',
    daemonsets: 'DaemonSets', statefulsets: 'StatefulSets', jobs: 'Jobs', cronjobs: 'CronJobs',
    services: 'Services', ingresses: 'Ingresses', configmaps: 'ConfigMaps', secrets: 'Secrets',
    pvcs: 'PVCs', pvs: 'PVs', storageclasses: 'StorageClasses', serviceaccounts: 'ServiceAccounts',
  }
  return names[active.value] || active.value
})
const hasOps = computed(() => ['pods', 'deployments', 'cronjobs', 'configmaps', 'secrets', 'serviceaccounts'].includes(active.value))
const opWidth = computed(() => (active.value === 'pods' ? 170 : active.value === 'cronjobs' ? 150 : 100))
const delConfirmKey = computed(() => ({
  configmaps: 'monitor.configDelConfirm', secrets: 'monitor.configDelConfirm', serviceaccounts: 'k8s.podDelConfirm',
}[active.value] || ''))

// ---- 概览卡片 ----
const cards = computed(() => {
  const s = summary.value
  if (!s) return []
  return [
    { key: 'nodes', label: `${t('k8s.nodes')} ${t('k8s.online')}`, text: `${s.nodes_ready}/${s.nodes}`, color: '#e6a23c', go: 'nodes' },
    { key: 'pods', label: 'Pods', text: `${s.running_pods}/${s.pods}`, color: '#409eff', go: 'pods' },
    { key: 'namespaces', label: t('k8s.namespaces'), text: s.namespaces, color: '#67c23a', go: 'namespaces' },
    { key: 'deployments', label: 'Deployments', text: s.deployments, color: '#f56c6c', go: 'deployments' },
    { key: 'daemonsets', label: 'DaemonSets', text: s.daemonsets, color: '#9c27b0', go: 'daemonsets' },
    { key: 'statefulsets', label: 'StatefulSets', text: s.statefulsets, color: '#ff9800', go: 'statefulsets' },
    { key: 'jobs', label: 'Jobs', text: s.jobs, color: '#00bcd4', go: 'jobs' },
    { key: 'cronjobs', label: 'CronJobs', text: s.cronjobs, color: '#3f51b5', go: 'cronjobs' },
    { key: 'services', label: 'Services', text: s.services, color: '#8bc34a', go: 'services' },
    { key: 'ingresses', label: 'Ingresses', text: s.ingresses, color: '#e91e63', go: 'ingresses' },
    { key: 'pvs', label: 'PVs', text: s.pvs, color: '#795548', go: 'pvs' },
    { key: 'pvcs', label: 'PVCs', text: s.pvcs, color: '#607d8b', go: 'pvcs' },
    { key: 'configmaps', label: 'ConfigMaps', text: s.configmaps, color: '#cddc39', go: 'configmaps' },
    { key: 'secrets', label: 'Secrets', text: s.secrets, color: '#f44336', go: 'secrets' },
    { key: 'serviceaccounts', label: 'ServiceAccounts', text: s.serviceaccounts, color: '#009688', go: 'serviceaccounts' },
  ]
})

// ---- 数据加载 ----
const loaders = {
  nodes: () => api.get(`${P}/nodes`),
  namespaces: () => api.get(`${P}/namespaces`),
  pods: () => api.get(`${P}/pods`, nsParams()),
  deployments: () => api.get(`${P}/deployments`, nsParams()),
  daemonsets: () => api.get(`${P}/daemonsets`, nsParams()),
  statefulsets: () => api.get(`${P}/statefulsets`, nsParams()),
  jobs: () => api.get(`${P}/jobs`, nsParams()),
  cronjobs: () => api.get(`${P}/cronjobs`, nsParams()),
  services: () => api.get(`${P}/services`, nsParams()),
  ingresses: () => api.get(`${P}/ingresses`, nsParams()),
  configmaps: () => api.get(`${P}/configmaps`, nsParams()),
  secrets: () => api.get(`${P}/secrets`, nsParams()),
  pvcs: () => api.get(`${P}/pvcs`, nsParams()),
  pvs: () => api.get(`${P}/pvs`),
  storageclasses: () => api.get(`${P}/storageclasses`),
  serviceaccounts: () => api.get(`${P}/serviceaccounts`, nsParams()),
}

const load = async () => {
  loading.value = true
  try {
    if (active.value === 'overview') {
      const r = await api.get(`${P}/summary`)
      summary.value = r.summary
      if (!events.value.length || !ns.value) events.value = await api.get(`${P}/events`)
    } else {
      rows.value = await loaders[active.value]()
    }
  } finally { loading.value = false }
}

watch(active, () => load())
watch(ns, () => { if (active.value !== 'overview') load() })

// ---- Pods 操作 ----
const logVisible = ref(false)
const logTitle = ref('')
const logText = ref('')
const showLog = async row => {
  const r = await api.get(`${P}/podlog`, { params: { namespace: row.namespace, pod: row.name } })
  logTitle.value = `${row.namespace}/${row.name}`
  logText.value = r.log || t('k8s.noLog')
  logVisible.value = true
}
const openShell = row => {
  window.open(`/k8s/exec?clusterId=${id}&namespace=${row.namespace}&pod=${row.name}`, '_blank')
}
const deletePod = async row => {
  await api.delete(`${P}/pods/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}

// ---- Deployments ----
const restartDeployment = async row => {
  await api.post(`${P}/deployments/${row.namespace}/${row.name}/restart`)
  ElMessage.success(t('common.success'))
  load()
}

// ---- CronJobs / SA / Config 删除与创建 ----
const deleteCron = async row => {
  await api.delete(`${P}/cronjobs/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}
const toggleCron = async row => {
  await api.put(`${P}/cronjobs/${row.namespace}/${row.name}/suspend`, { suspend: !row.suspend })
  ElMessage.success(t('common.success'))
  load()
}
const cronDlgVisible = ref(false)
const cronForm = reactive({ name: '', namespace: '', schedule: '', command: '' })
const openCronDlg = () => {
  Object.assign(cronForm, { name: '', namespace: ns.value || '', schedule: '', command: '' })
  cronDlgVisible.value = true
}
const createCron = async () => {
  if (!cronForm.name || !cronForm.schedule || !cronForm.command) { ElMessage.warning(t('k8s.nameServerRequired')); return }
  await api.post(`${P}/cronjobs`, cronForm)
  ElMessage.success(t('common.success'))
  cronDlgVisible.value = false
  load()
}

const saDlgVisible = ref(false)
const saForm = reactive({ name: '', namespace: '' })
const openSaDlg = () => {
  Object.assign(saForm, { name: '', namespace: ns.value || '' })
  saDlgVisible.value = true
}
const createSa = async () => {
  if (!saForm.name) { ElMessage.warning(t('k8s.saName')); return }
  await api.post(`${P}/serviceaccounts/${saForm.namespace || 'default'}`, { name: saForm.name })
  ElMessage.success(t('common.success'))
  saDlgVisible.value = false
  load()
}

const deleteResource = async row => {
  const kind = { configmaps: 'configmaps', secrets: 'secrets', serviceaccounts: 'serviceaccounts' }[active.value]
  await api.delete(`${P}/${kind}/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}

onMounted(async () => {
  const clusters = await api.get('/k8s/clusters')
  cluster.value = clusters.find(c => String(c.id) === String(id)) || null
  namespaces.value = await api.get(`${P}/namespaces`).catch(() => [])
  load()
})
</script>

<style scoped>
.km-page { display: flex; height: 100vh; background: #f0f2f5; }
.km-side {
  width: 210px; flex-shrink: 0; background: #1d2935; color: #a7b1c2;
  display: flex; flex-direction: column; overflow-y: auto;
}
.km-brand { color: #fff; font-size: 17px; font-weight: 700; padding: 16px 18px 4px; }
.km-brand span { color: #409eff; }
.km-cluster { color: #6b7a8c; font-size: 12px; padding: 0 18px 10px; border-bottom: 1px solid #2a3947; }
.km-menu { border-right: none; flex: 1; }
.km-menu :deep(.el-menu-item.is-active) { background: #409eff !important; }
.km-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.km-top {
  background: #fff; padding: 10px 16px; display: flex; gap: 10px; align-items: center;
  border-bottom: 1px solid #e4e7ed; box-shadow: 0 1px 4px rgba(0, 21, 41, .06);
}
.km-body { flex: 1; overflow: auto; padding: 16px; }
.km-h4 { margin: 0 0 10px; color: #303133; }
.km-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.km-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; margin-bottom: 18px; }
.km-card {
  background: #fff; border-radius: 6px; padding: 14px 16px; cursor: pointer;
  border-left: 3px solid var(--c); box-shadow: 0 1px 3px rgba(0, 21, 41, .08);
  transition: transform .15s;
}
.km-card:hover { transform: translateY(-2px); }
.km-card-num { font-size: 24px; font-weight: 700; color: var(--c); }
.km-card-label { font-size: 12px; color: #909399; margin-top: 2px; }
</style>
