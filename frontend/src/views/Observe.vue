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
            {{ total }} hits · {{ lastTook }} ms
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

      <el-table v-if="cols.length" :data="rows" size="small" border style="margin-top:12px" max-height="560">
        <el-table-column v-for="c in cols" :key="c" :prop="c" :label="c" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ formatCell(row[c]) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-else-if="!busy && !err" :description="$t('oo.empty')" :image-size="80" />
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

const STREAM_FIELDS = {
  host_metrics: 'host, host_id, cpu_percent, mem_percent, disk_percent, collected_at',
  task_logs: 'task_id, type, operator, host, os_user, status, exit_code, output',
  alert_events: 'kind, level, target, message',
  windows_events: 'host, log_name, level, event_id, provider, event_time, message',
  db_audit: 'username, action, resource, ip, status, detail',
}

const streamHint = computed(() => STREAM_FIELDS[stream.value] || '')

const defaultSQL = s => ({
  host_metrics: 'SELECT host, cpu_percent, mem_percent, disk_percent FROM host_metrics ORDER BY _timestamp DESC',
  task_logs: 'SELECT task_id, host, os_user, status, exit_code, output FROM task_logs ORDER BY _timestamp DESC',
  alert_events: 'SELECT kind, level, target, message FROM alert_events ORDER BY _timestamp DESC',
  windows_events: "SELECT host, log_name, level, event_id, message FROM windows_events ORDER BY _timestamp DESC",
  db_audit: 'SELECT username, action, resource, ip, status FROM db_audit ORDER BY _timestamp DESC',
}[s] || `SELECT * FROM ${s} LIMIT 100`)

const onStreamChange = () => { sql.value = defaultSQL(stream.value) }
const onRangeChange = () => { if (rangePreset.value !== 'custom') search() }

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

const csvCell = v => {
  const s = v === null || v === undefined ? '' : String(v)
  return /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
}

const exportCsv = () => {
  const head = cols.value.map(csvCell).join(',')
  const body = rows.value.map(r => cols.value.map(c => csvCell(formatCell(r[c]))).join(',')).join('\n')
  const blob = new Blob(['\uFEFF' + head + '\n' + body], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${stream.value}_${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

const search = async () => {
  const range = resolveRange()
  if (!range) { err.value = t('oo.pickRange'); return }
  busy.value = true
  err.value = ''
  try {
    const r = await api.post('/monitors/oo/search', {
      sql: (sql.value || defaultSQL(stream.value)).trim(),
      start_ms: range[0], end_ms: range[1], from: 0, size: 500,
    })
    const hits = r.hits || []
    const set = new Set()
    for (const h of hits.slice(0, 50)) Object.keys(h).forEach(k => { if (k !== '_timestamp') set.add(k) })
    cols.value = [...set]
    rows.value = hits
    total.value = r.total ?? hits.length
    lastTook.value = r.took ?? 0
    if (!hits.length) err.value = t('oo.emptyResult')
  } catch (e) {
    rows.value = []
    cols.value = []
    total.value = 0
    lastTook.value = 0
    err.value = e?.response?.data?.error || t('oo.notEnabled')
  } finally { busy.value = false }
}

onMounted(async () => {
  sql.value = defaultSQL(stream.value)
  search()
  // Discover streams present in OpenObserve (custom streams become queryable in the picker)
  try {
    const r = await api.get('/monitors/oo/streams')
    const builtins = new Set(presetStreams.map(s => s.value))
    customStreams.value = (r.streams || []).filter(s => !builtins.has(s))
  } catch { /* disabled → preset streams only */ }
})
</script>
