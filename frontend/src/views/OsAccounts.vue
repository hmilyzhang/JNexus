<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px; flex-wrap:wrap; align-items:center">
        <el-select v-model="hostFilter" :placeholder="$t('menu.hosts')" style="width:200px" clearable filterable @change="load">
          <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
        </el-select>
        <el-input v-model="keyword" :placeholder="$t('tasks.searchOutput')" style="width:200px" clearable @input="applyFilter" />
        <el-select v-model="rotFilter" style="width:170px" clearable :placeholder="$t('osac.rotFilterAll')" @change="applyFilter">
          <el-option :label="$t('osac.rotFailed')" value="failed" />
          <el-option :label="$t('osac.rotOn')" value="on" />
          <el-option :label="$t('osac.rotOff')" value="off" />
        </el-select>
        <span style="flex:1"></span>
        <el-button v-if="canManageCreds" type="success" plain @click="openBatch">
          {{ $t('hosts.credBatchBtn') }}{{ selRows.length ? ` (${selRows.length})` : '' }}</el-button>
        <el-button v-if="canManageCreds" type="primary" @click="dlgAdd()">{{ $t('hosts.credAdd') }}</el-button>

        <el-button v-if="canManageCreds" type="warning" plain :disabled="!selRows.length"
                   @click="startBatchRotate">{{ $t('osac.batchRotate') }}{{ selRows.length ? ` (${selRows.length})` : '' }}</el-button>
        <el-button v-if="canManageCreds" type="danger" plain :disabled="!selRows.length"
                   @click="batchDelCreds">{{ $t('k8s.batchDelete') }}{{ selRows.length ? ` (${selRows.length})` : '' }}</el-button>
      </div>

      <el-table :data="filtered" v-loading="loading" size="small" border :row-class-name="rowClass"
                @selection-change="s => (selRows = s)">
        <el-table-column type="selection" width="38" />
        <el-table-column :label="$t('menu.hosts')" min-width="150">
          <template #default="{ row }">
            <span v-if="!row.host_name && !row.host_ip" style="color:var(--el-text-color-secondary)">{{ $t('osac.unbound') }}</span>
            <span v-else-if="row.host_name">{{ row.host_name }} <span class="mono" style="color:var(--el-text-color-secondary)">({{ row.host_ip }})</span></span>
            <span v-else class="mono">{{ row.host_ip }} <span style="color:var(--el-text-color-secondary)">({{ $t('osac.unnamedHost') }})</span></span>
          </template>
        </el-table-column>
        <el-table-column prop="username" :label="$t('hosts.credUser')" width="120" />
        <el-table-column prop="label" :label="$t('hosts.credLabel')" width="120" />
        <el-table-column :label="$t('hosts.auth')" width="80">
          <template #default="{ row }">{{ row.auth_type === 'key' ? $t('hosts.authKey') : $t('hosts.authPassword') }}</template>
        </el-table-column>
        <el-table-column :label="$t('hosts.credDefault')" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.is_default" size="small" type="success">{{ $t('hosts.credDefault') }}</el-tag>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.enable')" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.rotate_enabled" size="small">90d</el-tag>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('osac.pwdChanged')" width="150">
          <template #default="{ row }">
            <span v-if="row.has_password">{{ fmtTime(row.last_rotated_at || row.created_at) }}</span>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.status')" min-width="150">
          <template #default="{ row }">
            <template v-if="row.rotate_enabled">
              <el-tag v-if="isFailed(row)" size="small" type="danger">{{ row.last_rotation_result }}</el-tag>
              <span v-else style="font-size:12px">{{ row.last_rotation_result || '-' }}</span>
              <div style="color:#909399; font-size:12px">{{ $t('rot.last') }}: {{ fmtTime(row.last_rotated_at) }}</div>
            </template>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="290" fixed="right">
          <template #default="{ row }">
            <template v-if="canManageCreds">
              <el-button v-if="row.has_password" size="small" type="warning" link
                         :loading="rotating === row.id" @click="rotateNow(row)">{{ $t('rot.now') }}</el-button>
              <el-button v-if="row.has_password && store.isAdmin" size="small" link @click="revealPwd(row)">{{ $t('rot.view') }}</el-button>
              <el-button v-if="row.has_password && store.isAdmin" size="small" link @click="showHistory(row)">{{ $t('rot.history') }}</el-button>
              <el-button size="small" link @click="dlgEdit(row)">{{ $t('common.edit') }}</el-button>
              <el-popconfirm :title="$t('hosts.credDelConfirm')" @confirm="delCred(row)">
                <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
              </el-popconfirm>
            </template>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add / edit account -->
    <el-dialog v-model="editVisible" :title="form.id ? $t('hosts.editHost') : $t('hosts.credAdd')" width="480px">
      <el-form label-width="110px">
        <el-form-item :label="$t('menu.hosts')">
          <el-select v-model="form.host_id" filterable style="width:100%" :disabled="!!form.id">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('hosts.credUser')"><el-input v-model="form.username" /></el-form-item>
        <el-form-item :label="$t('hosts.credLabel')"><el-input v-model="form.label" :placeholder="$t('hosts.credLabelPlaceholder')" /></el-form-item>
        <el-form-item :label="$t('hosts.authType')">
          <el-radio-group v-model="form.auth_type">
            <el-radio value="key">{{ $t('hosts.key') }}</el-radio>
            <el-radio value="password">{{ $t('hosts.password') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('hosts.key')" v-if="form.auth_type === 'key' && !form.auto_pair">
          <el-select v-model="form.ssh_key_id" :placeholder="$t('hosts.keyPlaceholder')" style="width:100%">
            <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('hosts.password')" v-else>
          <el-input v-model="form.password" type="password" show-password :placeholder="form.id ? $t('hosts.passwordKeep') : ''" />
          <el-checkbox v-if="!form.id" v-model="form.auto_pair" style="margin-top:4px">{{ $t('hosts.autoPair') }}</el-checkbox>
        </el-form-item>
        <el-form-item :label="$t('hosts.credDefault')"><el-switch v-model="form.is_default" /></el-form-item>
        <template v-if="form.auth_type === 'password'">
          <el-divider style="margin:8px 0 14px">{{ $t('rot.section') }}</el-divider>
          <el-form-item :label="$t('rot.enable')">
            <el-switch v-model="form.rotate_enabled" />
            <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('rot.enableTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('rot.days')" v-if="form.rotate_enabled">
            <el-input-number v-model="form.rotate_days" :min="0" :max="365" />
            <span style="margin-left:4px">{{ $t('rot.daysUnit') }}</span>
            <div style="color:#909399; font-size:12px; width:100%">{{ $t('rot.daysTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('rot.isLdap')" v-if="form.rotate_enabled">
            <el-switch v-model="form.is_ldap" />
            <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('rot.isLdapTip') }}</div>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- View password -->
    <el-dialog v-model="revealVisible" :title="$t('rot.viewTitle')" width="420px">
      <el-form label-width="90px">
        <el-form-item :label="$t('hosts.credUser')"><span class="mono">{{ revealData.username }}</span></el-form-item>
        <el-form-item :label="$t('hosts.password')"><span class="mono" style="font-weight:bold">{{ revealData.password }}</span></el-form-item>
      </el-form>
      <el-alert type="warning" :closable="false" :title="$t('rot.revealAudit')" />
      <template #footer>
        <el-button @click="revealVisible = false">{{ $t('common.cancel') }}</el-button>
      </template>
    </el-dialog>

    <!-- Password history -->
    <el-dialog v-model="histVisible" :title="$t('rot.histTitle')" width="620px">
      <el-alert type="warning" :closable="false" :title="$t('rot.revealAudit')" style="margin-bottom:12px" />
      <el-table :data="histRows" size="small" border max-height="380">
        <el-table-column :label="$t('rot.histTime')" width="160">
          <template #default="{ row }">{{ fmtTime(row.changed_at) }}</template>
        </el-table-column>
        <el-table-column :label="$t('rot.histSource')" width="130">
          <template #default="{ row }">
            {{ srcText(row.source) }}
            <el-tag v-if="row.current" size="small" type="success" style="margin-left:4px">{{ $t('rot.histCurrent') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.histOperator')" prop="operator" width="120" show-overflow-tooltip />
        <el-table-column :label="$t('hosts.password')">
          <template #default="{ row }"><span class="mono" style="font-weight:bold">{{ row.password }}</span></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="histVisible = false">{{ $t('common.cancel') }}</el-button>
      </template>
    </el-dialog>

    <!-- Batch add accounts -->
    <el-dialog v-model="batchVisible" :title="$t('hosts.credBatch')" width="640px" @open="loadTpls">
      <el-form label-width="110px">
        <el-form-item :label="$t('hosts.batchByGroup')">
          <div style="width:100%">
            <el-select v-model="batchGroups" multiple filterable clearable :max-collapse-tags="2" collapse-tags
                       style="width:100%" :placeholder="$t('hosts.batchByGroupPh')" @change="onBatchGroups">
              <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
            </el-select>
            <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('hosts.batchByGroupTip') }}</div>
          </div>
        </el-form-item>
        <el-form-item :label="$t('files.targetHosts')">
          <el-select v-model="batchForm.host_ids" multiple filterable clearable style="width:100%" :max-collapse-tags="2" collapse-tags>
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('hosts.tplFrom')">
          <el-select v-model="batchTpl" filterable clearable style="width:100%"
                     :placeholder="$t('hosts.tplFromPlaceholder')" @change="applyBatchTpl">
            <el-option v-for="tp in tpls" :key="tp.id" :value="tp.id"
                       :label="`${tp.label || tp.username} (${tp.username})`" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('hosts.tplFromTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('users.mode')">
          <el-radio-group v-model="batchMulti" @change="onBatchMode">
            <el-radio :value="false">{{ $t('hosts.credBatchOne') }}</el-radio>
            <el-radio :value="true">{{ $t('hosts.credBatchMulti') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="!batchMulti">
          <el-form-item :label="$t('hosts.credUser')"><el-input v-model="batchForm.username" class="mono" /></el-form-item>
          <el-form-item :label="$t('hosts.credLabel')"><el-input v-model="batchForm.label" :placeholder="$t('hosts.credLabelPlaceholder')" /></el-form-item>
          <el-form-item :label="$t('hosts.commonPassword')">
            <el-input v-model="batchForm.password" type="password" show-password autocomplete="new-password" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item :label="$t('hosts.accountList')">
            <div style="width:100%">
              <div v-for="(acc, i) in batchAccounts" :key="i"
                   style="display:flex; gap:6px; margin-bottom:6px; align-items:center">
                <el-input v-model="acc.username" class="mono" :placeholder="$t('users.username')" style="width:140px" />
                <el-input v-model="acc.password" type="password" show-password :placeholder="$t('hosts.password')" style="width:150px" />
                <el-input v-model="acc.label" :placeholder="$t('hosts.credLabel')" style="width:120px" />
                <el-checkbox v-model="acc.is_ldap">AD</el-checkbox>
                <el-button type="danger" link size="small" @click="batchAccounts.splice(i, 1)">{{ $t('apps.remove') }}</el-button>
              </div>
              <el-button size="small" @click="batchAccounts.push({ username: '', password: '', label: '', is_ldap: false })">{{ $t('system.newRole') === '' ? '' : $t('k8s.memberAdd') }}</el-button>
              <el-button size="small" @click="batchAccounts.push({ username: '', password: '', label: '', is_ldap: true })">+ AD</el-button>
              <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('hosts.accountListTip') }}</div>
            </div>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="batchVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="batchRunning" @click="runBatch">{{ $t('common.execute') }}</el-button>
      </template>
    </el-dialog>
  </div>

    <!-- Batch rotation progress -->
    <el-dialog v-model="batchRotateDlg" :title="$t('osac.batchRotateTitle')" width="560px"
               :close-on-click-modal="!rotBatchRunning" @closed="rotBatchPollStop">
      <div style="margin-bottom:12px">
        <el-progress :percentage="rotBatchProg" :status="rotBatchRunning ? '' : 'success'" />
        <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:6px">
          {{ rotBatch.done }} / {{ rotBatch.total }} · ✅ {{ rotBatch.ok }} · ❌ {{ rotBatch.failed }}
          <span v-if="rotBatchRunning"> · {{ $t('exec.taskRunning') }}</span>
        </div>
      </div>
      <el-table :data="rotBatch.results || []" size="small" border max-height="320">
        <el-table-column prop="host" :label="$t('menu.hosts')" min-width="130" />
        <el-table-column prop="username" :label="$t('hosts.credUser')" min-width="110" />
        <el-table-column :label="$t('tasks.status')" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.ok === undefined ? 'warning' : row.ok ? 'success' : 'danger'">
              {{ row.ok === undefined ? $t('exec.taskRunning') : row.ok ? $t('report.okShort') : $t('report.failShort') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="result" :label="$t('audit.detail')" min-width="140" show-overflow-tooltip />
      </el-table>
      <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:8px">{{ $t('osac.batchRotateTip') }}</div>
    </el-dialog>

</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const route = useRoute()
const router = useRouter()
const store = useUserStore()
const roleSettings = ref({})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})
const rows = ref([])
const hosts = ref([])
const keys = ref([])
const loading = ref(false)
const hostFilter = ref(null)
const keyword = ref('')
const rotFilter = ref('')
const editVisible = ref(false)
const form = ref({})
const revealVisible = ref(false)
const revealData = ref({})
const rotating = ref(null)
const histVisible = ref(false)
const histRows = ref([])
const batchVisible = ref(false)
const batchRunning = ref(false)
const batchForm = ref({ host_ids: [], username: '', label: '', auth_type: 'password', password: '' })
// Batch dialog state (bound by the template above — must be defined or the radio/list break)
const batchMulti = ref(false)
const batchAccounts = ref([])
const batchTpl = ref(null)
const tpls = ref([])
const groups = ref([])
const batchGroups = ref([])
const onBatchMode = () => { /* keeps entered values when switching modes */ }

// Batch group selection: expand the chosen groups (incl. all descendant groups,
// same cascade as batch execution) and merge their hosts into the target list
const onBatchGroups = () => {
  if (!batchGroups.value.length) return
  const gset = new Set(batchGroups.value)
  let changed = true
  while (changed) { // cascade down the multi-level tree
    changed = false
    for (const g of groups.value) {
      if (g.parent_id && gset.has(g.parent_id) && !gset.has(g.id)) { gset.add(g.id); changed = true }
    }
  }
  const ids = new Set(batchForm.value.host_ids)
  for (const h of hosts.value) {
    if (h.group_id && gset.has(h.group_id)) ids.add(h.id)
  }
  batchForm.value.host_ids = [...ids]
}

// Apply a credential template: fills single-account fields or appends one account entry
const applyBatchTpl = async tplId => {
  if (!tplId) return
  const tp = tpls.value.find(x => x.id === tplId)
  if (!tp) return
  let password = ''
  try {
    const r = await api.post(`/credentials/${tp.id}/reveal`)
    password = r.password || ''
  } catch { /* interceptor shows the error; still fill username/label */ }
  if (batchMulti.value) {
    batchAccounts.value.push({ username: tp.username, password, label: tp.label || tp.username, is_ldap: !!tp.is_ldap })
  } else {
    batchForm.value.username = tp.username
    batchForm.value.label = tp.label || tp.username
    batchForm.value.password = password
  }
  ElMessage.success(t('hosts.tplApplied'))
}

const canManageCreds = computed(() => {
  if (store.isAdmin) return true
  return !!roleSettings.value[store.role]?.cred
})

const isFailed = row => row.rotate_enabled && (row.last_rotation_result || '').includes('失败')
const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const rowClass = ({ row }) => (isFailed(row) ? 'fail-row' : '')

const filtered = computed(() => {
  let list = [...rows.value]
  // Failed items first
  const w = r => (isFailed(r) ? 0 : 1)
  list.sort((a, b) => w(a) - w(b) || a.id - b.id)
  if (hostFilter.value) list = list.filter(r => r.host_id === hostFilter.value)
  const kw = keyword.value.trim().toLowerCase()
  if (kw) {
    list = list.filter(r =>
      (r.username || '').toLowerCase().includes(kw) ||
      (r.label || '').toLowerCase().includes(kw) ||
      (r.host_name || '').toLowerCase().includes(kw) ||
      (r.host_ip || '').toLowerCase().includes(kw))
  }
  if (rotFilter.value === 'failed') list = list.filter(isFailed)
  else if (rotFilter.value === 'on') list = list.filter(r => r.rotate_enabled)
  else if (rotFilter.value === 'off') list = list.filter(r => !r.rotate_enabled)
  return list
})

const load = async () => {
  loading.value = true
  try {
    const params = {}
    if (hostFilter.value) params.host_id = hostFilter.value
    rows.value = await api.get('/credentials', { params })
  } finally { loading.value = false }
}

const loadBase = async () => {
    hosts.value = await api.get('/hosts')
    keys.value = await api.get('/ssh_keys')
    api.get('/host_groups').then(gs => { groups.value = gs || [] }).catch(() => { groups.value = [] })
}

onMounted(async () => {
  await Promise.all([loadBase(), load()])
  const qh = Number(route.query.host)
  if (qh) hostFilter.value = qh
})

const dlgAdd = () => {
  form.value = { host_id: hosts.value[0]?.id, username: '', label: '', auth_type: 'password',
    ssh_key_id: null, password: '', is_default: false, auto_pair: true,
    rotate_enabled: false, rotate_days: 0, is_ldap: false }
  editVisible.value = true
}
const dlgEdit = row => {
  form.value = { ...row, password: '' }
  editVisible.value = true
}
const save = async () => {
  if (!form.value.username) { ElMessage.warning(t('hosts.credUser')); return }
  if (form.value.id) {
    await api.put(`/credentials/${form.value.id}`, form.value)
  } else {
    await api.post(`/hosts/${form.value.host_id}/credentials`, form.value)
  }
  ElMessage.success(t('hosts.saved'))
  editVisible.value = false
  load()
}
const selRows = ref([])
const batchDelCreds = async () => {
  if (!selRows.value.length) return
  try {
    await ElMessageBox.confirm(t('hosts.batchDelConfirm', { n: selRows.value.length }), t('common.delete'), { type: 'warning' })
  } catch { return }
  const r = await api.post('/credentials/batch-delete', { ids: selRows.value.map(c => c.id) })
  const failed = Object.keys(r.failed || {}).length
  if (failed) ElMessage.warning(t('hosts.batchDelDone', { ok: r.deleted.length, fail: failed }))
  else ElMessage.success(t('common.success'))
  selRows.value = []
  load()
}

const delCred = async row => {
  await api.delete(`/credentials/${row.id}`)
  ElMessage.success(t('hosts.deleted'))
  load()
}
// ---- Batch rotation (async batch + progress polling) ----
const rotBatchDlg = ref(false)
const rotBatchRunning = ref(false)
const rotBatchId = ref('')
const rotBatch = ref({ total: 0, done: 0, ok: 0, failed: 0, results: [], running: false })
const rotBatchProg = computed(() => (rotBatch.value.total ? Math.round((rotBatch.value.done / rotBatch.value.total) * 100) : 0))
let rotBatchTimer = null
const rotBatchPollStop = () => { if (rotBatchTimer) { clearInterval(rotBatchTimer); rotBatchTimer = null } }
const startBatchRotate = async () => {
  if (!selRows.value.length) return
  try {
    await ElMessageBox.confirm(
      t('osac.batchRotateConfirm').replace('{n}', String(selRows.value.length)),
      t('osac.batchRotateTitle'),
      { type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel') }
    )
  } catch { return }
  const ids = selRows.value.map(r => r.id)
  try {
    const r = await api.post('/credentials/rotate-batch', { ids })
    rotBatchId.value = r.batch
    rotBatch.value = { total: r.total, done: 0, ok: 0, failed: 0, results: [], running: true }
    rotBatchDlg.value = true
    rotBatchPollStop()
    rotBatchTimer = setInterval(async () => {
      try {
        const st = await api.get('/credentials/rotate-batch/' + rotBatchId.value)
        rotBatch.value = st
        if (!st.running) { rotBatchPollStop(); load() }
      } catch { rotBatchPollStop() }
    }, 1500)
  } catch { /* interceptor shows the error */ }
}
const rotateNow = async row => {
  try {
    await ElMessageBox.confirm(t('rot.confirm'), t('common.tip'), { type: 'warning' })
  } catch { return }
  rotating.value = row.id
  try {
    const r = await api.post(`/credentials/${row.id}/rotate`)
    if (r.ok) ElMessage.success(r.result)
    else ElMessage.error(r.result)
    load()
  } finally { rotating.value = null }
}
const revealPwd = async row => {
  const r = await api.post(`/credentials/${row.id}/reveal`)
  revealData.value = { username: row.username, password: r.password }
  revealVisible.value = true
}
const showHistory = async row => {
  histRows.value = await api.get(`/credentials/${row.id}/password-history`)
  histVisible.value = true
}
const srcText = s => ({ created: t('rot.srcCreated'), manual: t('rot.srcManual'), scheduled: t('rot.srcScheduled') }[s] || s)
// Open the batch dialog. With table rows selected: preload those accounts
// (multi-account mode, passwords revealed one-by-one) so they can be re-applied to other hosts.
const openBatch = async () => {
  const sel = [...selRows.value]
  if (!sel.length) {
    batchForm.value = { host_ids: [], username: '', label: '', auth_type: 'password', password: '' }
    batchMulti.value = false
    batchAccounts.value = []
    batchTpl.value = null
    batchGroups.value = []
    batchVisible.value = true
    return
  }
  batchGroups.value = []
  batchForm.value.host_ids = [...new Set(sel.map(r => r.host_id).filter(Boolean))]
  batchMulti.value = true
  batchAccounts.value = sel.map(r => ({ username: r.username, password: '', label: r.label || r.username, is_ldap: !!r.is_ldap }))
  batchTpl.value = null
  batchVisible.value = true
  for (let i = 0; i < sel.length; i++) {
    if (sel[i].auth_type === 'key') continue // key accounts carry no password
    try {
      const r = await api.post(`/credentials/${sel[i].id}/reveal`)
      if (batchAccounts.value[i]) batchAccounts.value[i].password = r.password || ''
    } catch { /* leave empty; user can type it */ }
  }
}

const loadTpls = async () => {
  try { tpls.value = await api.get('/credentials/templates') || [] } catch { tpls.value = [] }
}

const runBatch = async () => {
  if (!batchForm.value.host_ids.length) { ElMessage.warning(t('exec.needHosts')); return }
  if (batchMulti.value) {
    if (!batchAccounts.value.length) { ElMessage.warning(t('hosts.accountListTip')); return }
  } else if (!batchForm.value.username || !batchForm.value.password) { ElMessage.warning(t('osac.needUserPwd')); return }
  batchRunning.value = true
  try {
    const payload = batchMulti.value
      ? { ...batchForm.value, accounts: batchAccounts.value, auto_pair: false }
      : { ...batchForm.value, auto_pair: true }
    const res = await api.post('/credentials/batch', payload)
    batchVisible.value = false
    router.push(`/tasks?detail=${res.task_id}`)
  } finally { batchRunning.value = false }
}
</script>

<style scoped>
:deep(.fail-row) { background: var(--el-color-danger-light-9); }
</style>
