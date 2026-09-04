<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <!-- 主机基础资源（CPU / 内存 / 磁盘） -->
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('monitor.hostRes') }}</span>
          <el-input v-model="hostKw" size="small" style="width:200px" clearable :placeholder="$t('tasks.searchOutput')" />
        </div>
      </template>
      <el-table :data="filteredHosts" v-loading="hostsLoading" size="small" border
                @row-click="openHostTrend" style="cursor:pointer">
        <el-table-column prop="name" :label="$t('monitor.targetHost')" min-width="150">
          <template #default="{ row }">{{ row.name }}<div style="color:#909399; font-size:12px">{{ row.ip }}</div></template>
        </el-table-column>
        <el-table-column :label="$t('monitor.group')" min-width="110">
          <template #default="{ row }">{{ row.group || '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('monitor.cpu')" min-width="160" prop="cpu" sortable>
          <template #default="{ row }">
            <template v-if="row.collected_at">
              <el-progress :percentage="row.cpu || 0" :color="barColor(row.cpu)" :stroke-width="10" />
            </template>
            <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('monitor.mem')" min-width="160" prop="mem" sortable>
          <template #default="{ row }">
            <template v-if="row.collected_at">
              <el-progress :percentage="row.mem || 0" :color="barColor(row.mem)" :stroke-width="10" />
            </template>
            <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('monitor.disk')" min-width="160" prop="disk" sortable>
          <template #default="{ row }">
            <template v-if="row.collected_at">
              <el-progress :percentage="row.disk || 0" :color="barColor(row.disk)" :stroke-width="10" />
            </template>
            <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('monitor.lastCheck')" width="150">
          <template #default="{ row }">{{ fmtTime(row.collected_at) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 应用监控（HTTP / TCP / Ping） -->
    <el-card style="margin-top:16px">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('monitor.appMon') }}</span>
          <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
          <el-button size="small" type="primary" @click="openDlg()">{{ $t('monitor.addMon') }}</el-button>
        </div>
      </template>
      <div v-if="!monitors.length" style="color:#909399; padding:12px 0">{{ $t('monitor.noData') }}</div>
      <div v-for="row in monitors" :key="row.monitor.id" class="mon-row">
        <span class="dot" :class="statusClass(row)"></span>
        <div style="flex:1; min-width:0">
          <div style="font-weight:600">
            {{ row.monitor.name }}
            <el-tag size="small" type="info" style="margin-left:6px">{{ typeLabel(row.monitor.type) }}</el-tag>
            <el-tag v-if="!row.monitor.enabled" size="small" type="warning" style="margin-left:6px">{{ $t('monitor.paused') }}</el-tag>
          </div>
          <div style="color:#909399; font-size:12px; word-break:break-all">
            {{ monitorTarget(row.monitor) }}
            <span v-if="row.monitor.last_error" style="color:#f56c6c"> — {{ row.monitor.last_error }}</span>
          </div>
        </div>
        <div class="hb">
          <span v-for="(s, i) in row.recent || []" :key="i" class="hb-bar" :class="s.status === 'up' ? 'hb-up' : 'hb-down'"
                :title="`${fmtTime(s.at)} · ${s.resp_ms}ms`"></span>
          <span v-if="!(row.recent || []).length" style="color:#c0c4cc; font-size:12px">{{ $t('monitor.notYet') }}</span>
        </div>
        <div class="mon-stats">
          <div class="stat">
            <div class="stat-val">{{ statusText(row) }}</div>
            <div class="stat-lbl">{{ row.monitor.last_resp_ms ? row.monitor.last_resp_ms + 'ms' : '—' }}</div>
          </div>
          <div class="stat">
            <div class="stat-val" :style="{ color: (row.uptime24h ?? 100) < 99 ? '#e6a23c' : '#67c23a' }">
              {{ row.uptime24h != null ? row.uptime24h + '%' : '—' }}
            </div>
            <div class="stat-lbl">{{ $t('monitor.uptime24h') }}</div>
          </div>
        </div>
        <div style="display:flex; gap:4px; align-items:center">
          <el-button size="small" link type="primary" :disabled="!row.monitor.enabled" @click="testNow(row)">{{ $t('monitor.testNow') }}</el-button>
          <el-button size="small" link @click="openDlg(row.monitor)">{{ $t('common.edit') }}</el-button>
          <el-button size="small" link @click="togglePause(row)">{{ row.monitor.enabled ? $t('common.disabled') : $t('common.enabled') }}</el-button>
          <el-popconfirm :title="$t('monitor.delConfirm')" @confirm="del(row.monitor)">
            <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
          </el-popconfirm>
        </div>
      </div>
    </el-card>

    <!-- 主机资源趋势 -->
    <el-drawer v-model="trendVisible" :title="`${trendHost?.name} — ${$t('monitor.trend')}`" size="640px">
      <div v-if="trendRows.length === 0" style="color:#909399">{{ $t('monitor.noSamples') }}</div>
      <template v-else>
        <div v-for="k in ['cpu', 'mem', 'disk']" :key="k" style="margin-bottom:18px">
          <div style="font-weight:600; margin-bottom:4px">{{ metricLabel(k) }}</div>
          <svg :viewBox="`0 0 600 80`" width="100%" height="80" style="background:#f5f7fa; border-radius:4px">
            <polyline :points="sparkPoints(trendRows.map(r => r[k + '_percent']))" fill="none" stroke="#409eff" stroke-width="2" />
          </svg>
          <div style="font-size:12px; color:#909399">0% — {{ maxOf(trendRows.map(r => r[k + '_percent'])) }}%</div>
        </div>
      </template>
    </el-drawer>

    <!-- 新建/编辑监控项 -->
    <el-dialog v-model="dlgVisible" :title="form.id ? $t('monitor.editMon') : $t('monitor.addMon')" width="520px">
      <el-form label-width="120px">
        <el-form-item :label="$t('monitor.monName')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('monitor.monType')">
          <el-radio-group v-model="form.type">
            <el-radio-button value="http">{{ $t('monitor.typeHttp') }}</el-radio-button>
            <el-radio-button value="tcp">{{ $t('monitor.typeTcp') }}</el-radio-button>
            <el-radio-button value="ping">{{ $t('monitor.typePing') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('monitor.target')">
          <el-input v-model="form.target" :placeholder="form.type === 'http' ? $t('monitor.urlTip') : $t('monitor.hostTip')" />
        </el-form-item>
        <el-form-item v-if="form.type === 'tcp'" :label="$t('monitor.port')">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <template v-if="form.type === 'http'">
          <el-form-item :label="$t('monitor.method')">
            <el-radio-group v-model="form.method">
              <el-radio value="GET">GET</el-radio>
              <el-radio value="HEAD">HEAD</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item :label="$t('monitor.acceptedStatus')">
            <el-input v-model="form.accepted_status" :placeholder="$t('monitor.acceptedTip')" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('monitor.keyword')">
            <el-input v-model="form.keyword" :placeholder="$t('monitor.keywordTip')" />
          </el-form-item>
          <el-form-item v-if="form.keyword" :label="$t('monitor.keywordType')">
            <el-radio-group v-model="form.keyword_type">
              <el-radio value="contain">{{ $t('monitor.kwContain') }}</el-radio>
              <el-radio value="absent">{{ $t('monitor.kwAbsent') }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>
        <el-form-item :label="$t('monitor.interval')"><el-input-number v-model="form.interval_sec" :min="15" :max="86400" /></el-form-item>
        <el-form-item :label="$t('monitor.timeout')"><el-input-number v-model="form.timeout_sec" :min="1" :max="120" /></el-form-item>
        <el-form-item :label="$t('monitor.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const hostsLoading = ref(false)
const hostRows = ref([])
const hostKw = ref('')
const monitors = ref([])
const loading = ref(false)
const dlgVisible = ref(false)
const form = reactive({})
let timer = null

const filteredHosts = computed(() => {
  const kw = hostKw.value.trim().toLowerCase()
  if (!kw) return hostRows.value
  return hostRows.value.filter(h => `${h.name} ${h.ip} ${h.group || ''}`.toLowerCase().includes(kw))
})

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const typeLabel = ty => ({ http: t('monitor.typeHttp'), tcp: t('monitor.typeTcp'), ping: t('monitor.typePing') }[ty] || ty)
const monitorTarget = m => (m.type === 'http' ? m.target : m.type === 'tcp' ? `${m.target}:${m.port}` : m.target)
const statusClass = row => (!row.monitor.enabled ? 'dot-paused' : row.monitor.last_status === 'up' ? 'dot-up' : row.monitor.last_status === 'down' ? 'dot-down' : 'dot-paused')
const statusText = row => (!row.monitor.enabled ? t('monitor.paused') : row.monitor.last_status === 'up' ? t('monitor.up') : row.monitor.last_status === 'down' ? t('monitor.down') : t('monitor.notYet'))
const barColor = v => (v == null ? '#909399' : v >= 90 ? '#f56c6c' : v >= 75 ? '#e6a23c' : '#67c23a')

const load = async () => {
  loading.value = true
  hostsLoading.value = true
  try {
    const [hs, ms] = await Promise.all([api.get('/monitoring/hosts'), api.get('/monitors')])
    hostRows.value = hs
    monitors.value = ms
  } finally {
    hostsLoading.value = false
    loading.value = false
  }
}

// ---- 主机趋势 ----
const trendVisible = ref(false)
const trendHost = ref(null)
const trendRows = ref([])
const maxOf = arr => Math.max(10, ...arr.map(v => Math.ceil(v || 0)))
const metricLabel = k => ({ cpu: t('monitor.cpu'), mem: t('monitor.mem'), disk: t('monitor.disk') }[k])
const sparkPoints = arr => {
  const max = maxOf(arr)
  const n = arr.length
  if (!n) return ''
  return arr.map((v, i) => `${(i / Math.max(1, n - 1)) * 600},${80 - ((v || 0) / max) * 76}`).join(' ')
}
const openHostTrend = async row => {
  trendHost.value = row
  trendRows.value = await api.get(`/monitoring/hosts/${row.host_id}/history`, { params: { hours: 6 } })
  trendVisible.value = true
}

// ---- 监控项 CRUD ----
const openDlg = m => {
  Object.assign(form, m || {}, {
    type: m?.type || 'http', method: m?.method || 'GET',
    accepted_status: m?.accepted_status || '200-299',
    keyword_type: m?.keyword_type || 'contain',
    interval_sec: m?.interval_sec || 60, timeout_sec: m?.timeout_sec || 10,
    enabled: m ? !!m.enabled : true, port: m?.port || 80,
  })
  dlgVisible.value = true
}
const save = async () => {
  const payload = { ...form }
  if (form.id) {
    await api.put(`/monitors/${form.id}`, payload)
  } else {
    await api.post('/monitors', payload)
    ElMessage.success(t('monitor.created'))
  }
  dlgVisible.value = false
  load()
}
const del = async m => { await api.delete(`/monitors/${m.id}`); load() }
const togglePause = async row => {
  await api.put(`/monitors/${row.monitor.id}`, { ...row.monitor, enabled: !row.monitor.enabled })
  load()
}
const testNow = async row => {
  const r = await api.post(`/monitors/${row.monitor.id}/test`)
  if (r.up) ElMessage.success(`${t('monitor.testUp')} · ${r.resp_ms}ms`)
  else ElMessage.error(`${t('monitor.testDown')}: ${r.error}`)
  load()
}

onMounted(() => {
  load()
  timer = setInterval(load, 30000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.mon-row {
  display: flex; align-items: center; gap: 14px; padding: 10px 4px;
  border-bottom: 1px solid #ebeef5;
}
.dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.dot-up { background: #67c23a; box-shadow: 0 0 0 3px rgba(103, 194, 58, .2); }
.dot-down { background: #f56c6c; box-shadow: 0 0 0 3px rgba(245, 108, 108, .2); }
.dot-paused { background: #c0c4cc; }
.hb { display: flex; gap: 2px; align-items: flex-end; height: 26px; flex-shrink: 0; }
.hb-bar { width: 5px; border-radius: 2px; display: inline-block; height: 100%; }
.hb-up { background: #67c23a; }
.hb-down { background: #f56c6c; height: 60%; }
.mon-stats { display: flex; gap: 18px; text-align: center; flex-shrink: 0; }
.stat-val { font-weight: 600; }
.stat-lbl { font-size: 11px; color: #909399; }
@media (max-width: 1100px) { .hb { display: none; } }
</style>
