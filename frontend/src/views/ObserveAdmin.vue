<!-- OpenObserve integration management (admin): left-nav layout -
     connection status (default), builtin integrations, security watchlists,
     retention policy, database ingestion and the push tester.
     Connection settings (url/org/token) are maintained in System Settings. -->
<template>
  <div>
    <el-card class="oa-shell-card" v-loading="loading">
      <div style="display:flex; gap:18px; min-height:480px">
        <div class="oa-side-nav">
          <div class="oa-side-nav-title">{{ $t('menu.observeAdmin') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'conn' }" @click="oaSection = 'conn'">{{ $t('oa.navConn') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'streams' }" @click="oaSection = 'streams'">{{ $t('oa.navStreams') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'watch' }" @click="oaSection = 'watch'">{{ $t('oa.navWatch') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'retention' }" @click="oaSection = 'retention'">{{ $t('oa.navRetention') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'db' }" @click="oaSection = 'db'">{{ $t('oa.navDb') }}</div>
          <div class="oa-nav-item" :class="{ active: oaSection === 'push' }" @click="oaSection = 'push'">{{ $t('oa.navPush') }}</div>
        </div>

        <div style="flex:1; min-width:0">
          <!-- Connection status (readonly; url/org/token maintained in System Settings) -->
          <template v-if="oaSection === 'conn'">
            <div style="display:flex; align-items:center; gap:10px; margin-bottom:14px; flex-wrap:wrap">
              <el-tag v-if="st" size="small" :type="st.enabled ? (st.reachable ? 'success' : 'danger') : 'info'">
                {{ st.enabled ? (st.reachable ? $t('oa.online') : $t('oa.unreachable')) : $t('oa.enabledOff') }}
              </el-tag>
              <el-tag v-if="st && st.enabled && st.reachable" size="small" type="info" effect="plain">{{ st.latency_ms }} ms</el-tag>
              <span style="flex:1"></span>
              <el-button size="small" :loading="testing" @click="testConn">{{ $t('ai.testConn') }}</el-button>
              <el-button size="small" @click="$router.push('/system')">{{ $t('oa.gotoSettings') }}</el-button>
              <el-button size="small" @click="load">{{ $t('common.refresh') }}</el-button>
            </div>
            <div v-if="st" class="oa-grid">
              <div class="oa-kv"><span>{{ $t('oo.enabled') }}</span><b>{{ st.enabled ? 'Yes' : 'No' }}</b></div>
              <div class="oa-kv"><span>{{ $t('oo.url') }}</span><b class="mono">{{ st.url || '—' }}</b></div>
              <div class="oa-kv"><span>{{ $t('oo.org') }}</span><b class="mono">{{ st.org || '—' }}</b></div>
              <div class="oa-kv"><span>{{ $t('oo.token') }}</span><b>{{ st.token_set ? $t('oa.tokenSet') : $t('oa.tokenMissing') }}</b></div>
              <div v-if="st.error" class="oa-kv"><span>{{ $t('oa.lastError') }}</span><b style="color:var(--el-color-danger)">{{ st.error }}</b></div>
              <div v-if="st.enabled && !st.reachable && /(^|\.)?(localhost|127\.0\.0\.1)(:|\/|$)/.test(st.url || '')"
                   class="oa-kv"><span>{{ $t('oa.containerHint') }}</span><b>{{ $t('oa.containerHintText') }}</b></div>
            </div>
            <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:14px">{{ $t('oa.connMaintainHint') }}</div>
          </template>

          <!-- Builtin integrations -->
          <template v-if="oaSection === 'streams'">
            <div class="oa-sec-head" style="margin-bottom:14px"><span style="font-weight:600">{{ $t('oa.builtinTitle') }}</span></div>
            <el-table :data="builtinRows" size="small" border>
              <el-table-column prop="stream" label="Stream" width="160">
                <template #default="{ row }"><span class="mono">{{ row.stream }}</span></template>
              </el-table-column>
              <el-table-column prop="desc" :label="$t('oa.descCol')" min-width="200" />
              <el-table-column :label="$t('oa.enabledCol')" width="90" align="center">
                <template #default="{ row }">
                  <el-switch :model-value="row.enabled" @change="v => toggle(row.stream, v)" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('oa.pushedCol')" width="100" align="center">
                <template #default="{ row }">{{ stat(row.stream).pushed }}</template>
              </el-table-column>
              <el-table-column :label="$t('oa.failedCol')" width="90" align="center">
                <template #default="{ row }">
                  <span :style="stat(row.stream).failed ? 'color:var(--el-color-danger)' : ''">{{ stat(row.stream).failed }}</span>
                </template>
              </el-table-column>
              <el-table-column :label="$t('oa.lastPushCol')" width="170">
                <template #default="{ row }">{{ fmtTime(stat(row.stream).last_push_at) }}</template>
              </el-table-column>
              <el-table-column :label="$t('oa.lastErrorCol')" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  <span v-if="stat(row.stream).last_error" style="color:var(--el-color-danger)">{{ stat(row.stream).last_error }}</span>
                  <span v-else>—</span>
                </template>
              </el-table-column>
            </el-table>
            <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:8px">{{ $t('oa.statsHint') }}</div>
          </template>

          <!-- Security watchlists -->
          <template v-if="oaSection === 'watch'">
            <div class="oa-sec-head" style="margin-bottom:14px; display:flex; align-items:center">
              <span style="font-weight:600; flex:1">{{ $t('oa.secWatchTitle') }}</span>
              <el-button size="small" type="primary" :loading="secWatchSaving" @click="saveSecWatch">{{ $t('common.save') }}</el-button>
            </div>

            <div style="font-weight:600; margin-bottom:8px">{{ $t('oa.secWatchWin') }}</div>
            <el-table :data="secWin" size="small" border>
              <el-table-column prop="id" label="Event ID" width="100" align="center" />
              <el-table-column :label="$t('oa.secWatchName')">
                <template #default="{ row }"><el-input v-model="row.name" size="small" /></template>
              </el-table-column>
              <el-table-column :label="$t('oa.secWatchImmediate')" width="110" align="center">
                <template #default="{ row }"><el-switch v-model="row.immediate" /></template>
              </el-table-column>
              <el-table-column :label="$t('oa.enabledCol')" width="90" align="center">
                <template #default="{ row }"><el-switch v-model="row.on" /></template>
              </el-table-column>
              <el-table-column width="70" align="center">
                <template #default="{ $index }">
                  <el-button size="small" link type="danger" @click="delWinRow($index)">{{ $t('common.delete') }}</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div style="display:flex; gap:8px; margin-top:8px">
              <el-input v-model="newWinId" size="small" placeholder="Event ID" style="width:140px" />
              <el-button size="small" @click="addWinRow">{{ $t('common.add') }}</el-button>
            </div>

            <div style="font-weight:600; margin:16px 0 8px">{{ $t('oa.secWatchLinux') }}</div>
            <el-table :data="secLinux" size="small" border>
              <el-table-column :label="$t('oa.secWatchKw')" min-width="220">
                <template #default="{ row }"><el-input v-model="row.kw" size="small" class="mono" /></template>
              </el-table-column>
              <el-table-column :label="$t('oa.secWatchName')">
                <template #default="{ row }"><el-input v-model="row.name" size="small" /></template>
              </el-table-column>
              <el-table-column :label="$t('oa.secWatchImmediate')" width="110" align="center">
                <template #default="{ row }"><el-switch v-model="row.immediate" /></template>
              </el-table-column>
              <el-table-column :label="$t('oa.enabledCol')" width="90" align="center">
                <template #default="{ row }"><el-switch v-model="row.on" /></template>
              </el-table-column>
              <el-table-column width="70" align="center">
                <template #default="{ $index }">
                  <el-button size="small" link type="danger" @click="delLinuxRow($index)">{{ $t('common.delete') }}</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" style="margin-top:8px" @click="addLinuxRow">{{ $t('common.add') }}</el-button>

            <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:10px">{{ $t('oa.secWatchTip') }}</div>
          </template>

          <!-- Retention policy -->
          <template v-if="oaSection === 'retention'">
            <div class="oa-sec-head" style="margin-bottom:14px"><span style="font-weight:600">{{ $t('oa.retentionTitle') }}</span></div>
            <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap">
              <span>{{ $t('oa.retentionKeep') }}</span>
              <el-input-number v-model="retentionDays" :min="0" :max="3650" :step="10" />
              <span>{{ $t('oa.retentionDays') }}</span>
              <el-button size="small" type="primary" :loading="retentionApplying" @click="applyRetention">{{ $t('oa.retentionApply') }}</el-button>
              <span style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('oa.retentionTip') }}</span>
            </div>
            <div v-if="retentionResults.length" style="margin-top:10px">
              <div v-for="r in retentionResults" :key="r.stream" style="font-size:12.5px; line-height:1.9">
                <span class="mono">{{ r.stream }}</span>:
                <span v-if="r.ok" style="color:var(--el-color-success)">✓</span>
                <span v-else style="color:var(--el-color-danger)">✗ {{ r.error }}</span>
              </div>
            </div>
          </template>

          <!-- Database ingestion -->
          <template v-if="oaSection === 'db'">
            <div class="oa-sec-head" style="margin-bottom:14px; display:flex; align-items:center">
              <span style="font-weight:600; flex:1">{{ $t('oa.dbTitle') }}</span>
              <el-button size="small" type="primary" @click="dbDlg()">{{ $t('oa.dbAdd') }}</el-button>
            </div>
            <el-table :data="dbSources" size="small" border v-loading="dbLoading">
              <el-table-column prop="name" :label="$t('oa.dbName')" width="150">
                <template #default="{ row }"><span class="mono">{{ row.name }}</span></template>
              </el-table-column>
              <el-table-column prop="db_type" :label="$t('oa.dbType')" width="90" align="center">
                <template #default="{ row }"><el-tag size="small" effect="plain">{{ row.db_type }}</el-tag></template>
              </el-table-column>
              <el-table-column :label="$t('oa.dbConn')" min-width="180">
                <template #default="{ row }"><span class="mono">{{ row.host }}:{{ row.port }} / {{ row.database }}</span></template>
              </el-table-column>
              <el-table-column prop="query" :label="$t('oa.dbQuery')" min-width="180" show-overflow-tooltip />
              <el-table-column prop="stream" :label="$t('oa.streamCol')" width="120">
                <template #default="{ row }"><span class="mono">{{ row.stream }}</span></template>
              </el-table-column>
              <el-table-column :label="$t('oa.enabledCol')" width="80" align="center">
                <template #default="{ row }">
                  <el-switch :model-value="row.enabled" @change="v => dbToggle(row, v)" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('oa.lastRunCol')" width="160">
                <template #default="{ row }">
                  <div>{{ fmtTime(row.last_run_at) }}</div>
                  <div v-if="row.last_error" style="color:var(--el-color-danger); font-size:11px" :title="row.last_error">{{ row.last_error.slice(0, 40) }}</div>
                </template>
              </el-table-column>
              <el-table-column :label="$t('common.actions')" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" link type="primary" :loading="row._running" @click="dbRun(row)">{{ $t('oo.run') }}</el-button>
                  <el-button size="small" link type="primary" @click="dbDlg(row)">{{ $t('common.edit') }}</el-button>
                  <el-popconfirm :title="$t('oa.dbDelConfirm')" @confirm="dbDel(row)">
                    <template #reference><el-button size="small" link type="danger">{{ $t('common.delete') }}</el-button></template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </template>

          <!-- Push tester + external API guide -->
          <template v-if="oaSection === 'push'">
            <el-row :gutter="18">
              <el-col :span="12">
                <div style="font-weight:600; margin-bottom:10px">{{ $t('oa.testerTitle') }}</div>
                <el-form label-width="90px">
                  <el-form-item :label="$t('oa.streamCol')">
                    <el-select v-model="pushStream" filterable allow-create default-first-option style="width:100%">
                      <el-option v-for="s in knownStreams" :key="s" :value="s" :label="s" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="$t('oa.jsonCol')">
                    <el-input v-model="pushJSON" type="textarea" :rows="6" class="mono"
                              :placeholder="'[{&quot;device&quot;: &quot;fw-1&quot;, &quot;temp_c&quot;: 62.5}]'" />
                  </el-form-item>
                  <el-form-item>
                    <el-button type="primary" :loading="pushing" @click="pushTest">{{ $t('oo.run') }}</el-button>
                  </el-form-item>
                </el-form>
              </el-col>
              <el-col :span="12">
                <div style="font-weight:600; margin-bottom:10px">{{ $t('oa.apiTitle') }}</div>
                <div style="color:var(--el-text-color-secondary); font-size:12px; margin-bottom:8px">{{ $t('oa.apiTip') }}</div>
                <div class="mono oa-code">curl -X POST "$JNEXUS/api/ext/oo/my_stream" \<br>
                  &nbsp;&nbsp;-H "Authorization: Bearer aok_&lt;id&gt;.&lt;secret&gt;" \\<br>
                  &nbsp;&nbsp;-H "Content-Type: application/json" \\<br>
                  &nbsp;&nbsp;-d '[{"device":"fw-1","temp_c":62.5}]'</div>
                <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:8px">{{ $t('oa.apiNote') }}</div>
              </el-col>
            </el-row>
          </template>
        </div>
      </div>
    </el-card>

    <!-- DB source dialog -->
    <el-dialog v-model="dbDlgVisible" :title="dbForm.id ? $t('oa.dbEdit') : $t('oa.dbAdd')" width="560px">
      <el-form label-width="110px">
        <el-form-item :label="$t('oa.dbName')" required>
          <el-input v-model="dbForm.name" class="mono" placeholder="billing-db" />
        </el-form-item>
        <el-form-item :label="$t('oa.dbType')" required>
          <el-radio-group v-model="dbForm.db_type">
            <el-radio value="mysql">MySQL</el-radio>
            <el-radio value="mssql">MSSQL</el-radio>
            <el-radio value="pgsql">PostgreSQL</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('hosts.ip')" required>
          <div style="display:flex; gap:8px; width:100%">
            <el-input v-model="dbForm.host" class="mono" style="flex:1" placeholder="10.0.0.10" />
            <el-input-number v-model="dbForm.port" :min="1" :max="65535" style="width:120px" />
          </div>
        </el-form-item>
        <el-form-item :label="$t('hosts.user')" required>
          <el-input v-model="dbForm.username" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('hosts.password')">
          <el-input v-model="dbForm.password" type="password" show-password class="mono"
                    :placeholder="dbForm.id ? $t('oa.keepPwd') : ''" />
        </el-form-item>
        <el-form-item :label="$t('oa.dbName2')" required>
          <el-input v-model="dbForm.database" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('oa.dbQuery')" required>
          <el-input v-model="dbForm.query" type="textarea" :rows="4" class="mono"
                    :placeholder="'SELECT id, status, created_at FROM orders WHERE created_at > NOW() - INTERVAL 1 DAY'" />
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:4px">{{ $t('oa.dbQueryTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('oa.streamCol')">
          <el-input v-model="dbForm.stream" class="mono" :placeholder="'db_billing_db'" />
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:4px">{{ $t('oa.dbStreamTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('oa.intervalLabel')">
          <div style="display:flex; align-items:center; gap:8px">
            <el-input-number v-model="dbForm.interval_sec" :min="30" :max="86400" :step="30" />
            <span style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('oa.dbIntervalTip') }}</span>
          </div>
        </el-form-item>
        <el-form-item :label="$t('oa.enabledCol')">
          <el-switch v-model="dbForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dbDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="dbSaving" @click="dbSave">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const st = ref(null)
const loading = ref(false)
const testing = ref(false)
const pushing = ref(false)
const pushStream = ref('custom_stream')
const pushJSON = ref('')
const discovered = ref([])

const builtinRows = computed(() => [
  { stream: 'host_metrics', desc: t('oa.descHostMetrics'), enabled: isEnabled('host_metrics') },
  { stream: 'task_logs', desc: t('oa.descTaskLogs'), enabled: isEnabled('task_logs') },
  { stream: 'alert_events', desc: t('oa.descAlertEvents'), enabled: isEnabled('alert_events') },
  { stream: 'windows_events', desc: t('oa.descWindowsEvents'), enabled: isEnabled('windows_events') },
  { stream: 'linux_events', desc: t('oa.descLinuxEvents'), enabled: isEnabled('linux_events') },
  { stream: 'db_audit', desc: t('oa.descDbAudit'), enabled: isEnabled('db_audit') },
  { stream: 'k8s_capacity', desc: t('oa.descK8sCapacity'), enabled: isEnabled('k8s_capacity') },
  { stream: 'audit', desc: t('oa.descAuditStream'), enabled: isEnabled('audit') },
])
const knownStreams = computed(() => [...new Set(['custom_stream', ...builtinRows.value.map(r => r.stream), ...discovered.value])])

const integrations = computed(() => {
  try { return JSON.parse(st.value?.integrations || '{}') } catch { return {} }
})
const isEnabled = s => {
  const v = integrations.value[s]
  if (typeof v === 'boolean') return v
  return true // builtin streams default on
}
const stat = s => (st.value?.stats || {})[s] || { pushed: 0, failed: 0 }

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '—')

const oaSection = ref('conn')

const load = async () => {
  loading.value = true
  try {
    st.value = await api.get('/system/oo/status')
    discovered.value = st.value.streams || []
  } finally { loading.value = false }
  loadDbSources()
  loadSecWatch()
  loadRetention()
}

// save connection settings from the admin page; empty token keeps the stored one


const testConn = async () => {
  testing.value = true
  try {
    await api.post('/system/oo/test')
    ElMessage.success(t('system.saved'))
    await load()
  } catch { /* interceptor shows the error */ } finally { testing.value = false }
}

const toggle = async (stream, enabled) => {
  try {
    await api.post('/system/oo/integrations', { stream, enabled })
    ElMessage.success(t('system.saved'))
    await load()
  } catch { /* interceptor shows the error */ }
}

// ---- Security watchlist (Windows event ids + Linux keyword patterns) ----
const secWin = ref([])
const secLinux = ref([])
const secWatchSaving = ref(false)
const newWinId = ref('')

const loadSecWatch = async () => {
  try {
    const r = await api.get('/system/sec/watch')
    secWin.value = r.win || []
    secLinux.value = r.linux || []
  } catch { /* interceptor shows the error */ }
}
const saveSecWatch = async () => {
  secWatchSaving.value = true
  try {
    await api.put('/system/sec/watch', { win: secWin.value, linux: secLinux.value })
    ElMessage.success(t('system.saved'))
    await loadSecWatch()
  } catch { /* interceptor shows the error */ } finally { secWatchSaving.value = false }
}
const addWinRow = () => {
  const id = Number(newWinId.value)
  if (!id) { ElMessage.warning(t('oa.secWatchNeedId')); return }
  if (secWin.value.some(w => w.id === id)) { ElMessage.warning(t('oa.secWatchDup')); return }
  secWin.value.push({ id, name: '', on: true, immediate: false })
  newWinId.value = ''
}
const addLinuxRow = () => {
  secLinux.value.push({ kw: '', name: '', on: true, immediate: false })
}
const delWinRow = idx => secWin.value.splice(idx, 1)
const delLinuxRow = idx => secLinux.value.splice(idx, 1)

// ---- Retention policy: push a retention period to every OO stream ----
const retentionDays = ref(30)
const retentionApplying = ref(false)
const retentionResults = ref([])

const loadRetention = async () => {
  try {
    const r = await api.get('/system/oo/retention')
    if (r.days > 0) retentionDays.value = r.days
  } catch { /* keep default */ }
}
const applyRetention = async () => {
  retentionApplying.value = true
  retentionResults.value = []
  try {
    const r = await api.post('/system/oo/retention', { days: Number(retentionDays.value) || 0 })
    retentionResults.value = r.results || []
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { retentionApplying.value = false }
}

// ---- Database ingestion sources ----
const dbSources = ref([])
const dbLoading = ref(false)
const dbDlgVisible = ref(false)
const dbSaving = ref(false)
const dbForm = ref({})

const loadDbSources = async () => {
  dbLoading.value = true
  try { dbSources.value = await api.get('/system/oo/dbsources') || [] }
  finally { dbLoading.value = false }
}

const dbDlg = row => {
  dbForm.value = row
    ? { ...row, password: '' }
    : { id: null, name: '', db_type: 'mysql', host: '', port: 3306, username: '', password: '',
        database: '', query: '', interval_sec: 300, stream: '', enabled: true }
  dbDlgVisible.value = true
}

const dbSave = async () => {
  dbSaving.value = true
  try {
    await api.post('/system/oo/dbsources', dbForm.value)
    ElMessage.success(t('system.saved'))
    dbDlgVisible.value = false
    await load(); await loadDbSources()
  } catch { /* interceptor shows the error */ } finally { dbSaving.value = false }
}

const dbToggle = async (row, enabled) => {
  try {
    await api.post(`/system/oo/dbsources/${row.id}/enabled`, { enabled })
    ElMessage.success(t('system.saved')); await loadDbSources()
  } catch { /* interceptor shows the error */ }
}

const dbDel = async row => {
  try {
    await api.delete(`/system/oo/dbsources/${row.id}`)
    ElMessage.success(t('system.saved')); await loadDbSources()
  } catch { /* interceptor shows the error */ }
}

const dbRun = async row => {
  row._running = true
  try {
    const r = await api.post(`/system/oo/dbsources/${row.id}/run`)
    ElMessage.success(t('oa.pushOk', { n: r.rows }))
    await load(); await loadDbSources()
  } catch { /* interceptor shows the error */ } finally { row._running = false }
}

const pushTest = async () => {
  let records
  try {
    records = JSON.parse(pushJSON.value)
    if (!Array.isArray(records)) records = [records]
  } catch { ElMessage.warning(t('oa.badJson')); return }
  pushing.value = true
  try {
    const r = await api.post('/system/oo/push', { stream: pushStream.value, records })
    ElMessage.success(t('oa.pushOk', { n: r.successful }))
    await load()
  } catch { /* interceptor shows the error */ } finally { pushing.value = false }
}

onMounted(load)
</script>

<style scoped>
/* left-nav shell (mirrors the AI assistant tab nav) */
.oa-shell-card { padding: 0; }
.oa-shell-card :deep(.el-card__body) { padding: 18px; }
.oa-side-nav { width: 170px; flex-shrink: 0; padding-top: 4px; }
.oa-side-nav-title {
  font-weight: 600; font-size: 14px; color: var(--el-text-color-primary);
  padding: 0 14px; margin-bottom: 10px;
}
.oa-nav-item {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; margin-bottom: 4px;
  font-size: 14px; border-radius: 4px; cursor: pointer;
  color: var(--el-text-color-regular); border: 1px solid transparent;
  transition: background .15s, color .15s;
}
.oa-nav-item:hover { background: var(--el-fill-color-light); }
.oa-nav-item.active {
  background: var(--el-color-primary-light-9); color: var(--el-color-primary);
  font-weight: 600; border-color: var(--el-color-primary-light-7);
}
.oa-sec-head { font-size: 14px; }
.oa-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 10px 24px; }
.oa-kv { display: flex; gap: 10px; font-size: 13px; align-items: baseline; }
.oa-kv span { color: var(--el-text-color-secondary); flex-shrink: 0; }
.oa-kv b { word-break: break-all; }
.oa-edit-row { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.oa-edit-label { width: 140px; flex-shrink: 0; text-align: right; color: var(--el-text-color-regular); font-size: 13px; }
.oa-code {
  background: var(--el-fill-color-light); border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px; padding: 10px 12px; font-size: 12px; line-height: 1.7;
  overflow-x: auto; white-space: nowrap;
}
</style>
