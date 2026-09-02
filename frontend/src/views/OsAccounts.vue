<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
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
        <el-button v-if="canManageCreds" type="success" plain @click="batchVisible = true">{{ $t('hosts.credBatchBtn') }}</el-button>
        <el-button v-if="canManageCreds" type="primary" @click="dlgAdd()">{{ $t('hosts.credAdd') }}</el-button>
      </div>

      <el-table :data="filtered" v-loading="loading" size="small" border :row-class-name="rowClass">
        <el-table-column :label="$t('menu.hosts')" min-width="150">
          <template #default="{ row }">{{ row.host_name }}（{{ row.host_ip }}）</template>
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
            <el-tag v-if="row.rotate_enabled" size="small">90天</el-tag>
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
        <el-table-column :label="$t('common.operation')" width="210" fixed="right">
          <template #default="{ row }">
            <template v-if="canManageCreds">
              <el-button v-if="row.auth_type === 'password'" size="small" type="warning" link
                         :loading="rotating === row.id" @click="rotateNow(row)">{{ $t('rot.now') }}</el-button>
              <el-button v-if="row.auth_type === 'password' && store.isAdmin" size="small" link @click="revealPwd(row)">{{ $t('rot.view') }}</el-button>
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

    <!-- 添加/编辑账号 -->
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

    <!-- 查看密码 -->
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

    <!-- 批量添加账号 -->
    <el-dialog v-model="batchVisible" :title="$t('hosts.credBatch')" width="560px">
      <el-form label-width="110px">
        <el-form-item :label="$t('files.targetHosts')">
          <el-select v-model="batchForm.host_ids" multiple filterable style="width:100%" :max-collapse-tags="2" collapse-tags>
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('hosts.credUser')"><el-input v-model="batchForm.username" class="mono" /></el-form-item>
        <el-form-item :label="$t('hosts.credLabel')"><el-input v-model="batchForm.label" :placeholder="$t('hosts.credLabelPlaceholder')" /></el-form-item>
        <el-form-item :label="$t('hosts.commonPassword')">
          <el-input v-model="batchForm.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="batchRunning" @click="runBatch">{{ $t('common.execute') }}</el-button>
      </template>
    </el-dialog>
  </div>
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
const batchVisible = ref(false)
const batchRunning = ref(false)
const batchForm = ref({ host_ids: [], username: '', label: '', auth_type: 'password', password: '' })

const canManageCreds = computed(() => {
  if (store.isAdmin) return true
  return !!roleSettings.value[store.role]?.cred
})

const isFailed = row => row.rotate_enabled && (row.last_rotation_result || '').includes('失败')
const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const rowClass = ({ row }) => (isFailed(row) ? 'fail-row' : '')

const filtered = computed(() => {
  let list = [...rows.value]
  // 失败优先
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
const delCred = async row => {
  await api.delete(`/credentials/${row.id}`)
  ElMessage.success(t('hosts.deleted'))
  load()
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
const runBatch = async () => {
  if (!batchForm.value.host_ids.length) { ElMessage.warning(t('exec.needHosts')); return }
  if (!batchForm.value.username || !batchForm.value.password) { ElMessage.warning(t('hosts.needIpUser')); return }
  batchRunning.value = true
  try {
    const res = await api.post('/credentials/batch', { ...batchForm.value, auto_pair: true })
    batchVisible.value = false
    router.push(`/tasks?detail=${res.task_id}`)
  } finally { batchRunning.value = false }
}
</script>

<style scoped>
:deep(.fail-row) { background: #fef0f0; }
</style>
