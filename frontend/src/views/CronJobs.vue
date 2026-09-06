<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-button type="primary" @click="dlg()">{{ $t('cron.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="jobs" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" :label="$t('cron.name')" min-width="140" />
        <el-table-column :label="$t('exec.mode')" width="90">
          <template #default="{ row }">
            <el-tag size="small">{{ row.type === 'script' ? $t('tasks.typeScript') : $t('tasks.typeCommand') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cron_expr" label="Cron" width="120" class-name="mono" />
        <el-table-column :label="$t('cron.target')" min-width="120">
          <template #default="{ row }">
            {{ row.host_count ? $t('cron.nHosts', { n: row.host_count }) : (row.ips || $t('cron.byGroup')) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.status')" width="90">
          <template #default="{ row }">
            <el-switch :model-value="row.enabled" @change="v => toggle(row, v)" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('cron.lastRun')" width="170">
          <template #default="{ row }">
            <div v-if="row.last_run_at">
              <span style="font-size:12px">{{ fmtTime(row.last_run_at) }}</span>
              <el-button v-if="row.last_task_id" size="small" link @click="openTask(row.last_task_id)">{{ $t('exec.viewDetail') }}</el-button>
            </div>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('cron.nextRun')" width="170">
          <template #default="{ row }">{{ row.enabled ? fmtTime(row.next_run_at) : '-' }}</template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="170" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link :loading="running === row.id" @click="runNow(row)">{{ $t('tasks.runNow') }}</el-button>
            <el-button size="small" link @click="historyDlg(row)">{{ $t('tasks.title') }}</el-button>
            <el-dropdown trigger="click" @command="cmd => onCmd(cmd, row)">
              <el-button size="small" link>···</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">{{ $t('common.edit') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" style="color:#f56c6c">{{ $t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新建/编辑 -->
    <el-dialog v-model="visible" :title="form.id ? $t('cron.edit') : $t('cron.create')" width="640px">
      <el-form label-width="110px">
        <el-form-item :label="$t('cron.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('exec.mode')">
          <el-radio-group v-model="form.type">
            <el-radio value="command">{{ $t('exec.modeCommand') }}</el-radio>
            <el-radio value="script">{{ $t('exec.modeScript') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('exec.command')" v-if="form.type === 'command'">
          <el-input v-model="form.command" type="textarea" :rows="4" :placeholder="$t('exec.commandPlaceholder')" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('exec.script')" v-else>
          <el-select v-model="form.script_id" :placeholder="$t('exec.script')" style="width:100%">
            <el-option v-for="sc in scripts" :key="sc.id" :label="sc.name" :value="sc.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('cron.expr')">
          <div style="display:flex; gap:8px; width:100%; flex-wrap:wrap; align-items:center">
            <el-input v-model="form.cron_expr" class="mono" style="width:200px" placeholder="*/5 * * * *" />
            <el-select v-model="preset" :placeholder="$t('cron.preset')" style="width:160px" @change="v => form.cron_expr = v">
              <el-option v-for="p in presets" :key="p.expr" :label="p.label" :value="p.expr" />
            </el-select>
          </div>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('cron.exprTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('cron.target')">
          <el-select v-model="form.host_ids" multiple filterable style="width:100%" :max-collapse-tags="2" collapse-tags>
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('cron.targetTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('hosts.credOsAccount')">
          <el-select v-model="form.credential_id" clearable style="width:100%" :placeholder="$t('hosts.credSelectPlaceholder')">
            <el-option v-for="c in usableCreds" :key="c.id"
                       :label="`${c.host_name} · ${c.host_ip} — ${c.username}`" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('exec.timeout')"><el-input-number v-model="form.timeout_sec" :min="5" :max="3600" /></el-form-item>
        <el-form-item :label="$t('exec.concurrency')"><el-input-number v-model="form.concurrency" :min="1" :max="100" /></el-form-item>
        <el-form-item :label="$t('common.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 执行历史 -->
    <el-drawer v-model="historyVisible" :title="`${$t('cron.history')}：${current?.name}`" size="560px">
      <el-table :data="history" v-loading="historyLoading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column :label="$t('tasks.status')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'done' ? 'success' : row.status === 'failed' ? 'danger' : 'primary'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('tasks.createdAt')" width="170" />
        <el-table-column :label="$t('common.operation')" width="90">
          <template #default="{ row }">
            <el-button size="small" link @click="$router.push(`/tasks?detail=${row.id}`)">{{ $t('tasks.detail') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const router = useRouter()
const jobs = ref([])
const hosts = ref([])
const scripts = ref([])
const usableCreds = ref([])
const loading = ref(false)
const visible = ref(false)
const form = ref({})
const historyVisible = ref(false)
const history = ref([])
const historyLoading = ref(false)
const current = ref(null)
const running = ref(null)
const preset = ref(null)

const presets = computed(() => [
  { label: t('cron.every5min'), expr: '*/5 * * * *' },
  { label: t('cron.everyHour'), expr: '0 * * * *' },
  { label: t('cron.everyDay3'), expr: '0 3 * * *' },
  { label: t('cron.everyMonday'), expr: '0 3 * * 1' },
  { label: t('cron.everyMonth'), expr: '0 3 1 * *' }
])

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')

const load = async () => {
  loading.value = true
  try {
    jobs.value = await api.get('/crons')
  } finally { loading.value = false }
}

onMounted(async () => {
  load()
  hosts.value = await api.get('/hosts')
  scripts.value = await api.get('/scripts')
  usableCreds.value = await api.get('/credentials/usable')
})

const dlg = () => {
  form.value = {
    name: '', type: 'command', command: '', script_id: null, script_args: '',
    host_ids: [], cron_expr: '*/5 * * * *', timeout_sec: 300, concurrency: 10, enabled: true
  }
  preset.value = null
  visible.value = true
}

const save = async () => {
  if (!form.value.name) { ElMessage.warning(t('cron.name')); return }
  if (form.value.type === 'command' && !form.value.command.trim()) { ElMessage.warning(t('exec.needCommand')); return }
  if (form.value.type === 'script' && !form.value.script_id) { ElMessage.warning(t('exec.needScript')); return }
  if (form.value.id) await api.put(`/crons/${form.value.id}`, form.value)
  else await api.post('/crons', form.value)
  ElMessage.success(t('hosts.saved'))
  visible.value = false
  load()
}

const toggle = async (row, v) => {
  await api.post(`/crons/${row.id}/toggle`)
  load()
}

const runNow = async row => {
  running.value = row.id
  try {
    const res = await api.post(`/crons/${row.id}/run`)
    ElMessage.success(t('cron.runNowOk'))
    router.push(`/tasks?detail=${res.task_id}`)
  } finally { running.value = null }
}

const historyDlg = async row => {
  current.value = row
  historyLoading.value = true
  historyVisible.value = true
  try { history.value = await api.get(`/crons/${row.id}/history`) } finally { historyLoading.value = false }
}

const onCmd = async (cmd, row) => {
  if (cmd === 'edit') {
    form.value = {
      id: row.id, name: row.name, type: row.type,
      command: row.command || '', script_id: row.script_id || null, script_args: row.script_args || '',
      host_ids: row.host_ids || [], credential_id: row.credential_id || null,
      cron_expr: row.cron_expr, timeout_sec: row.timeout_sec || 300,
      concurrency: row.concurrency || 10, enabled: row.enabled
    }
    preset.value = null
    visible.value = true
  } else if (cmd === 'delete') {
    await api.delete(`/crons/${row.id}`)
    ElMessage.success(t('hosts.deleted'))
    load()
  }
}
</script>
