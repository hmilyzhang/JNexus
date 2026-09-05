<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-tabs v-model="activeTab">
      <!-- Tab 1: CMD 监控（CPU / 内存 / 磁盘） -->
      <el-tab-pane :label="$t('monitor.tabCmd')" name="cmd">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.hostRes') }}</span>
              <el-select v-model="groupFilter" size="small" style="width:170px" clearable :placeholder="$t('monitor.allGroups')">
                <el-option v-for="g in groupOptions" :key="g" :label="g" :value="g" />
              </el-select>
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
      </el-tab-pane>

      <!-- Tab 2: 应用监控 -->
      <el-tab-pane :label="$t('monitor.tabApp')" name="app">
        <el-card>
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
              <el-button size="small" link @click="openDlg(row.monitor, row.channel_ids)">{{ $t('common.edit') }}</el-button>
              <el-button size="small" link @click="togglePause(row)">{{ row.monitor.enabled ? $t('common.disabled') : $t('common.enabled') }}</el-button>
              <el-popconfirm :title="$t('monitor.delConfirm')" @confirm="del(row.monitor)">
                <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
              </el-popconfirm>
            </div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 3: Alert 配置（通知通道） -->
      <el-tab-pane :label="$t('monitor.tabAlert')" name="alert">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.chTitle') }}</span>
              <el-button size="small" :loading="chLoading" @click="loadChannels">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" @click="openChDlg()">{{ $t('monitor.chAdd') }}</el-button>
            </div>
          </template>
          <div style="color:#909399; font-size:12px; margin-bottom:10px">{{ $t('monitor.chTip') }}</div>
          <div v-if="!channels.length" style="color:#909399; padding:8px 0">{{ $t('monitor.chEmpty') }}</div>
          <el-table :data="channels" size="small" border>
            <el-table-column :label="$t('monitor.monName')" min-width="150">
              <template #default="{ row }">
                {{ row.name }}
                <el-tag v-if="!row.enabled" size="small" type="warning" style="margin-left:6px">{{ $t('monitor.paused') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.chType')" width="130">
              <template #default="{ row }"><el-tag size="small">{{ chTypeLabel(row.type) }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="$t('monitor.chConfig')" min-width="220">
              <template #default="{ row }"><span class="mono" style="font-size:12px; word-break:break-all">{{ chConfigSummary(row) }}</span></template>
            </el-table-column>
            <el-table-column :label="$t('common.operation')" width="220" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link type="primary" @click="testChannel(row)">{{ $t('monitor.chTest') }}</el-button>
                <el-button size="small" link @click="openChDlg(row)">{{ $t('common.edit') }}</el-button>
                <el-button size="small" link @click="toggleCh(row)">{{ row.enabled ? $t('common.disabled') : $t('common.enabled') }}</el-button>
                <el-popconfirm :title="$t('monitor.chDelConfirm')" @confirm="delChannel(row)">
                  <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

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
        <el-form-item :label="$t('monitor.notif')">
          <el-select v-model="form.channel_ids" multiple style="width:100%" :placeholder="$t('monitor.chEmpty')">
            <el-option v-for="ch in channels" :key="ch.id" :label="`${ch.name}（${chTypeLabel(ch.type)}）`" :value="ch.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('monitor.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 新建/编辑通知通道 -->
    <el-dialog v-model="chDlgVisible" :title="chForm.id ? $t('monitor.chEdit') : $t('monitor.chAdd')" width="520px">
      <el-form label-width="120px">
        <el-form-item :label="$t('monitor.monName')"><el-input v-model="chForm.name" /></el-form-item>
        <el-form-item :label="$t('monitor.chType')">
          <el-select v-model="chForm.type" style="width:100%" @change="onChTypeChange">
            <el-option v-for="ty in channelTypes" :key="ty.value" :label="ty.label" :value="ty.value" />
          </el-select>
        </el-form-item>
        <template v-if="chForm.type">
          <el-form-item v-if="chForm.type === 'email'" :label="$t('monitor.chRecipients')">
            <el-input v-model="chForm.f_recipients" :placeholder="$t('monitor.chRecipientsTip')" />
          </el-form-item>
          <el-form-item v-if="chForm.type === 'telegram'" :label="$t('monitor.chBotToken')">
            <el-input v-model="chForm.f_bot_token" class="mono" />
          </el-form-item>
          <el-form-item v-if="chForm.type === 'telegram'" :label="$t('monitor.chChatId')">
            <el-input v-model="chForm.f_chat_id" class="mono" />
          </el-form-item>
          <el-form-item v-if="chForm.type !== 'email' && chForm.type !== 'telegram'" :label="$t('monitor.chUrl')">
            <el-input v-model="chForm.f_url" class="mono" :placeholder="$t('monitor.chUrlTip')" />
          </el-form-item>
        </template>
        <el-form-item :label="$t('monitor.enabled')"><el-switch v-model="chForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="chDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveChannel">{{ $t('common.save') }}</el-button>
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
const activeTab = ref('cmd')
const hostsLoading = ref(false)
const hostRows = ref([])
const hostKw = ref('')
const groupFilter = ref('')
const monitors = ref([])
const channels = ref([])
const loading = ref(false)
const chLoading = ref(false)
const dlgVisible = ref(false)
const chDlgVisible = ref(false)
const form = reactive({})
const chForm = reactive({})
let timer = null

const groupOptions = computed(() => [...new Set(hostRows.value.map(h => h.group).filter(Boolean))])
const filteredHosts = computed(() => {
  let list = hostRows.value
  if (groupFilter.value) list = list.filter(h => h.group === groupFilter.value)
  const kw = hostKw.value.trim().toLowerCase()
  if (kw) list = list.filter(h => `${h.name} ${h.ip} ${h.group || ''}`.toLowerCase().includes(kw))
  return list
})

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const typeLabel = ty => ({ http: t('monitor.typeHttp'), tcp: t('monitor.typeTcp'), ping: t('monitor.typePing') }[ty] || ty)
const monitorTarget = m => (m.type === 'http' ? m.target : m.type === 'tcp' ? `${m.target}:${m.port}` : m.target)
const statusClass = row => (!row.monitor.enabled ? 'dot-paused' : row.monitor.last_status === 'up' ? 'dot-up' : row.monitor.last_status === 'down' ? 'dot-down' : 'dot-paused')
const statusText = row => (!row.monitor.enabled ? t('monitor.paused') : row.monitor.last_status === 'up' ? t('monitor.up') : row.monitor.last_status === 'down' ? t('monitor.down') : t('monitor.notYet'))
const barColor = v => (v == null ? '#909399' : v >= 90 ? '#f56c6c' : v >= 75 ? '#e6a23c' : '#67c23a')

const channelTypes = [
  { value: 'email', label: 'Email' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'wecom', label: 'WeCom' },
  { value: 'dingtalk', label: 'DingTalk' },
  { value: 'feishu', label: 'Feishu' },
  { value: 'telegram', label: 'Telegram' },
]
const chTypeLabel = v => (channelTypes.find(x => x.value === v) || {}).label || v
const chConfigSummary = row => {
  try {
    const cfg = JSON.parse(row.config || '{}')
    if (row.type === 'email') return cfg.recipients || '-'
    if (row.type === 'telegram') return cfg.chat_id || '-'
    return cfg.url || '-'
  } catch { return '-' }
}

const load = async () => {
  loading.value = true
  hostsLoading.value = true
  try {
    const [hs, ms, chs] = await Promise.all([
      api.get('/monitoring/hosts'), api.get('/monitors'), api.get('/alert_channels'),
    ])
    hostRows.value = hs
    monitors.value = ms
    channels.value = chs
  } finally {
    hostsLoading.value = false
    loading.value = false
  }
}
const loadChannels = async () => { channels.value = await api.get('/alert_channels') }

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
const openDlg = (m, channelIds) => {
  Object.assign(form, m || {}, {
    type: m?.type || 'http', method: m?.method || 'GET',
    accepted_status: m?.accepted_status || '200-299',
    keyword_type: m?.keyword_type || 'contain',
    interval_sec: m?.interval_sec || 60, timeout_sec: m?.timeout_sec || 10,
    enabled: m ? !!m.enabled : true, port: m?.port || 80,
    channel_ids: [...(channelIds || [])],
  })
  dlgVisible.value = true
}
const save = async () => {
  const payload = { ...form }
  if (form.id) await api.put(`/monitors/${form.id}`, payload)
  else {
    await api.post('/monitors', payload)
    ElMessage.success(t('monitor.created'))
  }
  dlgVisible.value = false
  load()
}
const del = async m => { await api.delete(`/monitors/${m.id}`); load() }
const togglePause = async row => {
  await api.put(`/monitors/${row.monitor.id}`, { ...row.monitor, channel_ids: row.channel_ids || [], enabled: !row.monitor.enabled })
  load()
}
const testNow = async row => {
  const r = await api.post(`/monitors/${row.monitor.id}/test`)
  if (r.up) ElMessage.success(`${t('monitor.testUp')} · ${r.resp_ms}ms`)
  else ElMessage.error(`${t('monitor.testDown')}: ${r.error}`)
  load()
}

// ---- 通知通道 ----
const onChTypeChange = () => { /* 切换类型时保留已填值由用户自行修改 */ }
const openChDlg = ch => {
  let cfg = {}
  try { cfg = JSON.parse(ch?.config || '{}') } catch { /* ignore */ }
  Object.assign(chForm, {
    id: ch?.id, name: ch?.name || '', type: ch?.type || '', enabled: ch ? !!ch.enabled : true,
    f_recipients: cfg.recipients || '', f_url: cfg.url || '',
    f_bot_token: cfg.bot_token || '', f_chat_id: cfg.chat_id || '',
  })
  chDlgVisible.value = true
}
const buildChConfig = () => {
  const cfg = {}
  if (chForm.type === 'email') cfg.recipients = chForm.f_recipients
  else if (chForm.type === 'telegram') { cfg.bot_token = chForm.f_bot_token; cfg.chat_id = chForm.f_chat_id }
  else cfg.url = chForm.f_url
  return JSON.stringify(cfg)
}
const saveChannel = async () => {
  if (!chForm.name || !chForm.type) { ElMessage.warning(t('monitor.chNameTypeRequired')); return }
  const payload = { name: chForm.name, type: chForm.type, config: buildChConfig(), enabled: chForm.enabled }
  if (chForm.id) await api.put(`/alert_channels/${chForm.id}`, payload)
  else await api.post('/alert_channels', payload)
  chDlgVisible.value = false
  loadChannels()
}
const delChannel = async ch => { await api.delete(`/alert_channels/${ch.id}`); loadChannels() }
const toggleCh = async ch => {
  await api.put(`/alert_channels/${ch.id}`, { ...ch, enabled: !ch.enabled })
  loadChannels()
}
const testChannel = async ch => {
  try {
    await api.post(`/alert_channels/${ch.id}/test`)
    ElMessage.success(t('monitor.chTestOk'))
  } catch { /* 错误提示由拦截器展示 */ }
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
