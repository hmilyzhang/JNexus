<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card :header="$t('report.generate')">
      <div class="tpl-cards">
        <div v-for="tpl in templates" :key="tpl.key" class="tpl-card"
             :class="{ active: form.template === tpl.key }" @click="form.template = tpl.key">
          <div class="tpl-name">{{ tplName(tpl) }}
            <el-button size="small" text type="primary" style="margin-left:4px"
                       @click.stop="showScript(tpl)">{{ $t('report.viewScript') }}</el-button>
          </div>
          <div class="tpl-desc">{{ tplDesc(tpl) }}</div>
          <el-tag v-if="tpl.has_ps" size="small" type="info" class="tpl-os">bash + PowerShell</el-tag>
        </div>
      </div>
      <el-form label-width="90px" style="margin-top:14px">
        <el-form-item :label="$t('cron.target')">
          <el-select v-model="form.host_ids" multiple filterable style="width:100%"
                     :max-collapse-tags="2" collapse-tags :placeholder="$t('cron.target')">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('report.targetTip') }}</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="generating" :disabled="!form.template"
                     @click="generate">{{ $t('report.generate') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-dialog v-model="scriptVisible" :title="$t('report.viewScript')" width="780px">
      <template v-if="scriptTpl">
        <div class="script-tag">bash — Linux</div>
        <pre class="script-pre">{{ scriptTpl.cmd }}</pre>
        <template v-if="scriptTpl.has_ps">
          <div class="script-tag" style="margin-top:10px">PowerShell — Windows</div>
          <pre class="script-pre">{{ scriptTpl.cmd_ps }}</pre>
        </template>
      </template>
      <template #footer>
        <el-button @click="scriptVisible = false">{{ $t('common.close') }}</el-button>
      </template>
    </el-dialog>
    <el-card :header="$t('report.listTitle')" style="margin-top:16px">
      <el-table :data="pagedReports" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column :label="$t('report.name')" min-width="200">
          <template #default="{ row }">{{ reportLabel(row) }}</template>
        </el-table-column>
        <el-table-column prop="operator" :label="$t('tasks.operator')" width="110" />
        <el-table-column prop="host_count" :label="$t('cron.target')" width="90" />
        <el-table-column :label="$t('tasks.status')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'done' ? 'success' : 'warning'">
              {{ row.status === 'done' ? $t('exec.taskDone') : $t('exec.taskRunning') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('tasks.createdAt')" width="170" />
        <el-table-column :label="$t('common.operation')" width="80" fixed="right">
          <template #default="{ row }">
            <el-popconfirm v-if="store.isAdmin" :title="$t('report.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
        <div class="list-pager">
          <el-pagination v-model:current-page="pageNum" v-model:page-size="pageSize" :total="reports.length"
                         :page-sizes="[10, 20, 50, 100]" layout="total, sizes, prev, pager, next, jumper" small background />
        </div>
    </el-card>

    <!-- Report detail -->
    <el-drawer v-model="detailVisible" :title="detail ? reportLabel(detail.report) : $t('report.detail')" size="780px">
      <template v-if="detail">
        <div style="display:flex; gap:8px; align-items:center; margin-bottom:10px">
          <el-tag size="small" type="success">{{ okCount }} {{ $t('report.okShort') }}</el-tag>
          <el-tag size="small" :type="failCount ? 'danger' : 'info'">{{ failCount }} {{ $t('report.failShort') }}</el-tag>
          <el-tag v-if="runningCount" size="small" type="warning">{{ runningCount }} {{ $t('exec.taskRunning') }}</el-tag>
          <el-checkbox v-model="onlyFailed" style="margin-left:8px">{{ $t('report.onlyFailedHosts') }}</el-checkbox>
          <span style="flex:1"></span>
          <el-button size="small" @click="download('log')">{{ $t('tasks.exportLog') }}</el-button>
          <el-button size="small" @click="download('csv')">{{ $t('tasks.exportCsv') }}</el-button>
        </div>

        <!-- Ports & certificates matrix (portcert template only) -->
        <template v-if="detail?.report?.template === 'portcert'">
          <div style="display:flex; align-items:center; gap:10px; margin:6px 0 10px">
            <span style="font-weight:600">{{ $t('report.portsMatrixTitle') }}</span>
            <el-button size="small" :loading="portsMatrixLoading" @click="loadPortsMatrix">{{ $t('common.refresh') }}</el-button>
          </div>
          <el-table :data="portsMatrix" size="small" border style="margin-bottom:14px">
            <el-table-column prop="host" :label="$t('hosts.name')" min-width="120" />
            <el-table-column prop="ip" :label="$t('hosts.ip')" min-width="110" />
            <el-table-column prop="ports" :label="$t('report.portsTotal')" width="90" align="center" />
            <el-table-column prop="https" :label="$t('report.portsHttps')" width="90" align="center" />
            <el-table-column prop="cert_valid" :label="$t('report.certValid')" width="90" align="center" />
            <el-table-column prop="cert_expired" :label="$t('report.certExpired')" width="100" align="center" />
          </el-table>
        </template>

        <!-- Cross-host account comparison (accounts template only) -->
        <template v-if="detail?.report?.template === 'accounts'">
          <div style="display:flex; gap:10px; align-items:center; margin:6px 0 10px">
            <span style="font-weight:600">{{ $t('report.acctMatrixTitle') }}</span>
            <el-checkbox v-model="acctDiffOnly">{{ $t('report.acctOnlyDiff') }}</el-checkbox>
            <span style="flex:1"></span>
            <span style="color:#909399; font-size:12px">{{ acctMatrix.accounts?.length ?? 0 }} {{ $t('report.acctCount') }} · {{ (acctMatrix.hosts || []).length }} {{ $t('cron.target') }}</span>
          </div>
          <el-alert v-if="acctMatrix.old_format" :title="$t('report.acctOldFormat')" type="info" :closable="false" style="margin-bottom:10px" />
          <template v-if="!acctMatrix.old_format">
            <div v-if="(acctMatrix.suspicious || []).length" style="margin-bottom:10px">
              <div style="font-weight:600; font-size:13px; margin-bottom:6px">{{ $t('report.acctSuspicious') }}</div>
              <el-tag v-for="(sc, i) in acctMatrix.suspicious" :key="i" size="small" type="danger" style="margin:0 6px 6px 0">
                {{ sc.host }} · {{ sc.username }} · {{ acctReason(sc) }}
              </el-tag>
            </div>
            <el-table :data="acctRows" size="small" border max-height="360">
              <el-table-column prop="username" label="Username" min-width="120" />
              <el-table-column prop="uid" label="UID" width="70" />
              <el-table-column :label="$t('monitor.type')" width="80">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.type === 'human' ? 'warning' : 'info'">{{ row.type }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="$t('osac.loginEnabled')" width="80" align="center">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.login_enabled ? 'success' : 'info'">{{ row.login_enabled ? 'Y' : 'N' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column v-for="(h, hi) in acctMatrix.hosts || []" :key="h.item_id"
                               :label="h.host_name" min-width="90" align="center">
                <template #default="{ row }">
                  <span v-if="row.cells[hi]" style="color:#67c23a">✓</span>
                  <span v-else style="color:#dcdfe6">—</span>
                </template>
              </el-table-column>
            </el-table>
          </template>
        </template>
        <el-collapse v-model="expandedHosts">
          <el-collapse-item v-for="it in shownItems" :key="it.id" :name="it.id">
            <template #title>
              <el-tag size="small" :type="it.status === 'success' ? 'success' : it.status === 'failed' ? 'danger' : 'warning'" style="margin-right:8px">
                {{ it.status === 'success' ? 'OK' : it.status === 'failed' ? 'FAIL' : $t('exec.taskRunning') }}
              </el-tag>
              <span style="font-weight:600">{{ it.host_name }}</span>
              <span class="mono" style="color:#909399; margin-left:8px">{{ it.host_ip }}</span>
              <span v-if="it.error" style="color:#f56c6c; margin-left:8px; font-size:12px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap">{{ it.error }}</span>
            </template>
            <div class="log-box" style="max-height:320px">{{ it.content || $t('report.noOutput') }}</div>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="!shownItems.length" :description="$t('report.allOk')" :image-size="60" />
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const route = useRoute()
const store = useUserStore()
const templates = ref([])
const scriptVisible = ref(false)
const scriptTpl = ref(null)
const showScript = tpl => { scriptTpl.value = tpl; scriptVisible.value = true }
const reports = ref([])
const pageNum = ref(1)
const pageSize = ref(20)
const pagedReports = computed(() => {
  const start = (pageNum.value - 1) * pageSize.value
  return reports.value.slice(start, start + pageSize.value)
})
watch(() => reports.value.length, n => {
  const maxPage = Math.max(1, Math.ceil(n / pageSize.value))
  if (pageNum.value > maxPage) pageNum.value = maxPage
})
const hosts = ref([])
const loading = ref(false)
const generating = ref(false)
const detailVisible = ref(false)
const detail = ref(null)
const onlyFailed = ref(false)
const expandedHosts = ref([])
const okCount = computed(() => (detail.value?.items || []).filter(i => i.status === 'success').length)
const failCount = computed(() => (detail.value?.items || []).filter(i => i.status === 'failed').length)
const runningCount = computed(() => (detail.value?.items || []).filter(i => i.status !== 'success' && i.status !== 'failed').length)
const shownItems = computed(() => {
  const items = detail.value?.items || []
  return onlyFailed.value ? items.filter(i => i.status === 'failed') : items
})
const autoExpandFailures = () => {
  const items = detail.value?.items || []
  expandedHosts.value = items.filter(i => i.status === 'failed').map(i => i.id)
}
let pollTimer = null

const generate = async () => {
  generating.value = true
  try {
    const res = await api.post('/reports', { template: form.value.template, host_ids: form.value.host_ids })
    ElMessage.success(t('report.generated'))
    await load()
    openDetail({ id: res.report_id })
  } finally { generating.value = false }
}

const form = ref({ template: 'accounts', host_ids: [] })
// Template name/description prefer language-pack keys (report.tpl_<key>), falling back to backend values when missing
const tplName = tpl => i18n.global.te(`report.tpl_${tpl.key}`)
  ? i18n.global.t(`report.tpl_${tpl.key}`) : tpl.name
const tplDesc = tpl => i18n.global.te(`report.tpl_${tpl.key}_desc`)
  ? i18n.global.t(`report.tpl_${tpl.key}_desc`) : tpl.desc
// Stored report names embed the localized template name; re-localize on display via the template key
const fmtStamp = iso => {
  const d = iso ? new Date(iso) : null
  if (!d || isNaN(d)) return ''
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`
}
const reportLabel = r => {
  if (r.template && i18n.global.te(`report.tpl_${r.template}`))
    return `${i18n.global.t(`report.tpl_${r.template}`)} ${fmtStamp(r.created_at)}`.trim()
  return r.name
}
// ---- Cross-host account comparison ----
const acctMatrix = ref({})
const portsMatrix = ref([])
const portsMatrixLoading = ref(false)
const acctDiffOnly = ref(false)
const acctRows = computed(() => {
  const accounts = acctMatrix.value.accounts || []
  const list = acctDiffOnly.value ? accounts.filter(a => a.diff) : accounts
  return list.map(a => {
    const cells = (acctMatrix.value.hosts || []).map((h, i) => (a.present || []).includes(i))
    return { ...a, cells }
  })
})
const acctReason = sc => ({
  uid0: t('report.acctUid0'),
  system_login: t('report.acctSysLogin'),
  dup_uid: t('report.acctDupUid'),
}[sc.reason] || sc.reason)
const loadAcctMatrix = async id => {
  acctMatrix.value = {}
  acctDiffOnly.value = false
  try {
    acctMatrix.value = await api.get(`/reports/accounts-matrix/${id}`)
  } catch { /* old reports without a CSV are flagged via old_format */ }
}
const openDetail = async row => {
  detail.value = await api.get(`/reports/${row.id}`)
  autoExpandFailures()
  detailVisible.value = true
  pollDetail(row.id)
  if (detail.value?.report?.template === 'accounts') loadAcctMatrix(row.id)
  if (detail.value?.report?.template === 'portcert') loadPortsMatrix(row.id)
}

const loadPortsMatrix = async id => {
  portsMatrixLoading.value = true
  try { portsMatrix.value = await api.get(`/reports/ports-matrix/${id}`) } finally { portsMatrixLoading.value = false }
}
const pollDetail = id => {
  clearInterval(pollTimer)
  pollTimer = setInterval(async () => {
    try {
      const d = await api.get(`/reports/${id}`)
      detail.value = d
      autoExpandFailures()
      if (d.report.status === 'done') clearInterval(pollTimer)
    } catch { clearInterval(pollTimer) }
  }, 2000)
}
onUnmounted(() => clearInterval(pollTimer))

const download = async format => {
  const data = await api.get(`/reports/${detail.value.report.id}/export`, { params: { format }, responseType: 'blob' })
  const url = URL.createObjectURL(new Blob([data]))
  const a = document.createElement('a')
  a.href = url
  a.download = `report-${detail.value.report.id}.${format}`
  a.click()
  URL.revokeObjectURL(url)
}

const load = async () => {
  loading.value = true
  try {
    reports.value = await api.get('/reports')
    templates.value = await api.get('/reports/templates')
  } finally { loading.value = false }
}

const del = async row => { await api.delete(`/reports/${row.id}`); load() }

onMounted(async () => {
  await load()
  hosts.value = await api.get('/hosts')
  if (route.query.tpl) form.value.template = route.query.tpl
})
</script>

<style scoped>
.list-pager { display: flex; justify-content: flex-end; margin-top: 10px; }

.tpl-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; }
.tpl-card {
  border: 1px solid var(--el-border-color); border-radius: 6px; padding: 12px; cursor: pointer;
  transition: border-color .2s;
}
.tpl-card:hover { border-color: var(--el-color-primary); }
.tpl-card.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.tpl-name { font-weight: 600; margin-bottom: 4px; color: var(--el-text-color-primary); }
.tpl-desc { color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5; }
.tpl-os { margin-top: 6px; }
.script-tag { font-weight: 600; margin: 6px 0 4px; }
.script-pre { background: var(--el-fill-color-light); border-radius: 6px; padding: 10px;
  font-size: 12px; line-height: 1.5; overflow: auto; max-height: 320px; white-space: pre-wrap;
  word-break: break-word; margin: 0; }
</style>
