<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- Native OpenObserve log search: stream picker, time range, SQL editor, CSV export.
     Queries go through the JNexus proxy (/monitors/oo/search); accounts stay in JNexus. -->
<template>
  <div>
    <el-card v-loading="busy">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap">
          <span style="font-weight:600">{{ $t('menu.observe') }}</span>
          <el-tag v-if="lastTook" size="small" type="info" effect="plain">
            {{ $t('oo.pageRowsTag', { n: pageRows }) }} · {{ lastTook }} ms
          </el-tag>
          <span style="flex:1"></span>
          <el-button size="small" :disabled="!rows.length" @click="exportCsv">{{ $t('oo.exportCsv') }}</el-button>
        </div>
      </template>

      <el-alert v-if="err" type="warning" :title="err" :closable="false" style="margin-bottom:12px" show-icon />

      <!-- Toolbar: stream + time range + run -->
      <div style="display:flex; gap:8px; flex-wrap:wrap; align-items:center; margin-bottom:10px">
        <el-select v-model="stream" style="width:180px" @change="onStreamChange">
          <el-option v-for="s in presetStreams" :key="s.value" :value="s.value" :label="$t(s.label)" />
          <el-option v-for="s in customStreams" :key="s" :value="s" :label="s" />
        </el-select>
        <el-select v-model="rangePreset" style="width:140px" @change="onRangeChange">
          <el-option value="1" :label="$t('oo.range1h')" />
          <el-option value="24" :label="$t('oo.range24h')" />
          <el-option value="168" :label="$t('oo.range7d')" />
          <el-option value="720" :label="$t('oo.range30d')" />
          <el-option value="custom" :label="$t('oo.rangeCustom')" />
        </el-select>
        <el-date-picker v-if="rangePreset === 'custom'" v-model="customRange" type="datetimerange"
                        :start-placeholder="$t('oo.startTime')" :end-placeholder="$t('oo.endTime')"
                        value-format="x" style="width:360px" />
        <el-select v-if="stream === 'windows_events'" v-model="logFilter" style="width:130px">
          <el-option value="all" :label="$t('oo.logAll')" />
          <el-option value="Security" label="Security" />
          <el-option value="System" label="System" />
          <el-option value="Application" label="Application" />
        </el-select>
        <el-select v-if="stream === 'linux_events'" v-model="unitFilter" style="width:130px">
          <el-option value="all" :label="$t('oo.logAll')" />
          <el-option v-for="u in unitOptions" :key="u" :value="u" :label="u" />
        </el-select>
        <el-select v-if="stream === 'host_metrics' || stream === 'windows_events'" v-model="hostFilter" filterable style="width:150px">
          <el-option value="all" :label="$t('oo.hostAll')" />
          <el-option v-for="h in hostOptions" :key="h" :value="h" :label="h" />
        </el-select>
        <el-input v-if="stream === 'windows_events'" v-model="evIdFilter" clearable
                  :placeholder="$t('oo.evIdPh')" style="width:130px" class="mono" />
        <el-button type="primary" :loading="busy" @click="search">{{ $t('oo.run') }}</el-button>
        <span style="flex:1"></span>
        <span style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('oo.sqlTip') }}</span>
      </div>

      <el-input v-model="sql" type="textarea" :rows="3" class="mono"
                :placeholder="'SELECT host, cpu_percent FROM host_metrics WHERE cpu_percent > 90'" />

      <!-- Field hints for the selected stream -->
      <div v-if="streamHint" style="margin-top:8px; color:var(--el-text-color-secondary); font-size:12px">
        {{ $t('oo.fields') }}: <code class="mono">{{ streamHint }}</code>
      </div>

      <el-table v-if="cols.length" :data="displayRows" size="small" border style="margin-top:12px" max-height="560">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="oo-expand">
              <div v-for="c in expandKeys(row)" :key="c" class="oo-kv">
                <span class="oo-k mono">{{ c }}</span>
                <span class="oo-v mono">{{ formatCell(row[c]) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('oo.time')" width="165">
          <template #default="{ row }">{{ fmtTs(row._timestamp) }}</template>
        </el-table-column>
        <el-table-column v-for="c in cols" :key="c" :prop="c" :label="c" min-width="150">
          <template #default="{ row }">{{ formatCell(row[c]) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="total > 0" style="margin-top:12px; display:flex; justify-content:flex-end">
        <el-pagination background layout="total, sizes, prev, pager, next"
                       :total="total" :current-page="page" :page-size="pageSize"
                       :page-sizes="[50, 100, 200, 500]"
                       @current-change="onPageChange" @size-change="onSizeChange" />
      </div>
      <el-empty v-else-if="!cols.length && !busy && !err" :description="$t('oo.empty')" :image-size="80" />
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global

const presetStreams = [
  { value: 'host_metrics', label: 'oo.streamHostMetrics' },
  { value: 'task_logs', label: 'oo.streamTaskLogs' },
  { value: 'alert_events', label: 'oo.streamAlertEvents' },
  { value: 'windows_events', label: 'oo.streamWindowsEvents' },
  { value: 'linux_events', label: 'oo.streamLinuxEvents' },
  { value: 'db_audit', label: 'oo.streamDbAudit' },
]
const stream = ref('host_metrics')
const rangePreset = ref('24')
const customRange = ref(null)
const sql = ref('')
const busy = ref(false)
const err = ref('')
const rows = ref([])
const cols = ref([])
const total = ref(0)
const lastTook = ref(0)
const customStreams = ref([]) // streams discovered from previous results' organization
const page = ref(1)
const pageSize = ref(100)
const pageRows = ref(0)
const hasMore = ref(false)
const hostFilter = ref('all')

const STREAM_FIELDS = {
  host_metrics: 'host, host_id, cpu_percent, mem_percent, disk_percent, collected_at',
  task_logs: 'task_id, type, operator, host, os_user, status, exit_code, output',
  alert_events: 'kind, level, target, message',
  windows_events: 'host, log_name, level, event_id, provider, event_time, message',
  linux_events: 'host, unit, message, event_time',
  linux_events: 'host, unit, message, event_time',
  db_audit: 'username, action, resource, ip, status, detail',
}

const streamHint = computed(() => STREAM_FIELDS[stream.value] || '')

const defaultSQL = s => ({
  host_metrics: 'SELECT host, cpu_percent, mem_percent, disk_percent FROM host_metrics ORDER BY _timestamp DESC',
  task_logs: 'SELECT task_id, host, os_user, status, exit_code, output FROM task_logs ORDER BY _timestamp DESC',
  alert_events: 'SELECT kind, level, target, message FROM alert_events ORDER BY _timestamp DESC',
  windows_events: "SELECT host, log_name, level, event_id, message FROM windows_events ORDER BY _timestamp DESC",
  linux_events: 'SELECT host, unit, message, event_time FROM linux_events ORDER BY _timestamp DESC',
  db_audit: 'SELECT username, action, resource, ip, status FROM db_audit ORDER BY _timestamp DESC',
}[s] || `SELECT * FROM ${s} LIMIT 100`)

const onStreamChange = () => {
  sql.value = defaultSQL(stream.value)
  logFilter.value = 'all'
  unitFilter.value = 'all'
  evIdFilter.value = ''
  hostFilter.value = 'all'
  page.value = 1
  search(false)
}
const onPageChange = p => { page.value = p; search(true) }
const onSizeChange = sz => { pageSize.value = sz; page.value = 1; search(true) }
const onRangeChange = () => { if (rangePreset.value !== 'custom') search(false) }

const resolveRange = () => {
  const end = Date.now()
  if (rangePreset.value === 'custom') {
    const r = customRange.value
    if (Array.isArray(r) && r[0] && r[1]) return [Number(r[0]), Number(r[1])]
    return null
  }
  return [end - Number(rangePreset.value) * 3600 * 1000, end]
}

const formatCell = v => {
  if (v === null || v === undefined) return '-'
  if (typeof v === 'object') return JSON.stringify(v)
  const s = String(v)
  if (/^\d{16,}$/.test(s)) { // _timestamp is epoch microseconds
    const d = new Date(Number(s) / 1000)
    if (!isNaN(d)) return d.toLocaleString()
  }
  return s
}

// detail view: all fields except the internal timestamp (shown as its own column)
const expandKeys = row => Object.keys(row).filter(k => k !== '_timestamp')
const fmtTs = v => {
  const n = Number(v)
  return isNaN(n) || n <= 0 ? '-' : new Date(n / 1000).toLocaleString()
}

// windows_events quick filter: narrow fetched rows by log_name client-side
const logFilter = ref('all')
const unitFilter = ref('all')
const unitOptions = computed(() => [...new Set(rows.value.map(r => r.unit).filter(Boolean))])
const evIdFilter = ref('')
const hostOptions = computed(() => [...new Set(rows.value.map(r => r.host).filter(Boolean))])
// quick filters: narrow fetched rows client-side (win → log_name/event_id/host, host_metrics → host)
const displayRows = computed(() => {
  let list = rows.value
  if (stream.value === 'windows_events') {
    if (logFilter.value !== 'all') list = list.filter(r => r.log_name === logFilter.value)
    const ev = evIdFilter.value.trim()
    if (ev) list = list.filter(r => String(r.event_id ?? '').includes(ev))
  }
  if (stream.value === 'linux_events' && unitFilter.value !== 'all') list = list.filter(r => r.unit === unitFilter.value)
  if (stream.value === 'host_metrics' && hostFilter.value !== 'all') list = list.filter(r => r.host === hostFilter.value)
  return list
})

const csvCell = v => {
  const s = v === null || v === undefined ? '' : String(v)
  return /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
}

const exportCsv = () => {
  const head = cols.value.map(csvCell).join(',')
  const body = displayRows.value.map(r => cols.value.map(c => csvCell(formatCell(r[c]))).join(',')).join('\n')
  const blob = new Blob(['\uFEFF' + head + '\n' + body], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${stream.value}_${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

const search = async (keepPage = true) => {
  if (!keepPage) page.value = 1
  const range = resolveRange()
  if (!range) { err.value = t('oo.pickRange'); return }
  busy.value = true
  err.value = ''
  try {
    // request one extra row to detect whether a next page exists (OO's `total`
    // only reports from+size, so it cannot drive a numeric pager)
    const from = (page.value - 1) * pageSize.value
    const r = await api.post('/monitors/oo/search', {
      sql: (sql.value || defaultSQL(stream.value)).trim(),
      start_ms: range[0], end_ms: range[1], from, size: pageSize.value + 1,
    })
    const hits = r.hits || []
    hasMore.value = hits.length > pageSize.value
    rows.value = hasMore.value ? hits.slice(0, pageSize.value) : hits
    const set = new Set()
    for (const h of rows.value.slice(0, 50)) Object.keys(h).forEach(k => { if (k !== '_timestamp') set.add(k) })
    cols.value = [...set]
    pageRows.value = rows.value.length
    // pager total: full pages before the current one + this page (+1 virtual page while more exist)
    total.value = rows.value.length + (hasMore.value ? pageSize.value : 0)
    lastTook.value = r.took ?? 0
    if (!rows.value.length) err.value = t('oo.emptyResult')
  } catch (e) {
    rows.value = []
    cols.value = []
    pageRows.value = 0
    hasMore.value = false
    total.value = 0
    lastTook.value = 0
    err.value = e?.response?.data?.error || t('oo.notEnabled')
  } finally { busy.value = false }
}

onMounted(async () => {
  sql.value = defaultSQL(stream.value)
  search(false)
  // Discover streams present in OpenObserve (custom streams become queryable in the picker)
  try {
    const r = await api.get('/monitors/oo/streams')
    const builtins = new Set(presetStreams.map(s => s.value))
    customStreams.value = (r.streams || []).filter(s => !builtins.has(s))
  } catch { /* disabled → preset streams only */ }
})
</script>


<style scoped>
/* compact rows: single-line ellipsis — full content is shown in the expand panel */
:deep(.el-table .el-table__body .cell) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.oo-expand { padding: 4px 12px 8px 24px; }
.oo-kv { display: flex; gap: 10px; padding: 2px 0; font-size: 12px; line-height: 1.6; }
.oo-k { color: var(--el-text-color-secondary); width: 110px; flex-shrink: 0; text-align: right; }
.oo-v { white-space: pre-wrap; word-break: break-all; color: var(--el-text-color-primary); }
</style>
