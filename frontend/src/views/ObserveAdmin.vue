<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- OpenObserve integration management (admin): connection health, per-stream
     push stats + toggles, custom push tester and the external push API guide. -->
<template>
  <div>
    <!-- Connection health -->
    <el-card v-loading="loading">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="font-weight:600">{{ $t('oa.connTitle') }}</span>
          <el-tag v-if="st" size="small" :type="st.enabled ? (st.reachable ? 'success' : 'danger') : 'info'">
            {{ st.enabled ? (st.reachable ? $t('oa.online') : $t('oa.unreachable')) : $t('oa.enabledOff') }}
          </el-tag>
          <el-tag v-if="st && st.enabled && st.reachable" size="small" type="info" effect="plain">{{ st.latency_ms }} ms</el-tag>
          <span style="flex:1"></span>
          <el-button size="small" :loading="testing" @click="testConn">{{ $t('ai.testConn') }}</el-button>
          <el-button size="small" @click="$router.push('/system')">{{ $t('oa.gotoSettings') }}</el-button>
          <el-button size="small" @click="load">{{ $t('common.refresh') }}</el-button>
        </div>
      </template>
      <div v-if="st" class="oa-grid">
        <div class="oa-kv"><span>{{ $t('oo.enabled') }}</span><b>{{ st.enabled ? 'Yes' : 'No' }}</b></div>
        <div class="oa-kv"><span>{{ $t('oo.url') }}</span><b class="mono">{{ st.url || '—' }}</b></div>
        <div class="oa-kv"><span>{{ $t('oo.org') }}</span><b class="mono">{{ st.org || '—' }}</b></div>
        <div class="oa-kv"><span>{{ $t('oo.token') }}</span><b>{{ st.token_set ? $t('oa.tokenSet') : $t('oa.tokenMissing') }}</b></div>
        <div v-if="st.error" class="oa-kv"><span>{{ $t('oa.lastError') }}</span><b style="color:var(--el-color-danger)">{{ st.error }}</b></div>
      </div>
    </el-card>

    <!-- Built-in integrations -->
    <el-card style="margin-top:14px">
      <template #header>
        <span style="font-weight:600">{{ $t('oa.builtinTitle') }}</span>
      </template>
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
    </el-card>

    <!-- Custom ingestion -->
    <el-card style="margin-top:14px">
      <template #header>
        <span style="font-weight:600">{{ $t('oa.customTitle') }}</span>
      </template>
      <el-row :gutter="18">
        <!-- Push tester -->
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
        <!-- External API guide -->
        <el-col :span="12">
          <div style="font-weight:600; margin-bottom:10px">{{ $t('oa.apiTitle') }}</div>
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-bottom:8px">{{ $t('oa.apiTip') }}</div>
          <div class="mono oa-code">curl -X POST "$JNEXUS/api/ext/oo/my_stream" \<br>
            &nbsp;&nbsp;-H "Authorization: Bearer aok_&lt;id&gt;.&lt;secret&gt;" \<br>
            &nbsp;&nbsp;-H "Content-Type: application/json" \<br>
            &nbsp;&nbsp;-d '[{"device":"fw-1","temp_c":62.5}]'</div>
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:8px">{{ $t('oa.apiNote') }}</div>
        </el-col>
      </el-row>
    </el-card>
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

const load = async () => {
  loading.value = true
  try {
    st.value = await api.get('/system/oo/status')
    discovered.value = st.value.streams || []
  } finally { loading.value = false }
}

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
.oa-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 10px 24px; }
.oa-kv { display: flex; gap: 10px; font-size: 13px; align-items: baseline; }
.oa-kv span { color: var(--el-text-color-secondary); flex-shrink: 0; }
.oa-kv b { word-break: break-all; }
.oa-code {
  background: var(--el-fill-color-light); border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px; padding: 10px 12px; font-size: 12px; line-height: 1.7;
  overflow-x: auto; white-space: nowrap;
}
</style>
