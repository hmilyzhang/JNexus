<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- Cloud account management + CSP asset sync dialog for the hosts page -->
<template>
  <div>
    <el-dialog v-model="visible" :title="$t('cloud.title')" width="860px" @open="loadAccounts">
      <el-table :data="accounts" v-loading="loading" size="small" border>
        <el-table-column prop="name" :label="$t('webapp.name')" min-width="110" />
        <el-table-column :label="$t('cloud.provider')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="{ aws: 'warning', azure: 'primary', huawei: 'danger' }[row.provider]">
              {{ row.provider.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="regions" :label="$t('cloud.regions')" min-width="130" show-overflow-tooltip />
        <el-table-column :label="$t('cloud.interval')" width="110">
          <template #default="{ row }">
            {{ row.sync_interval_min ? row.sync_interval_min + ' min' : $t('cloud.manual') }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('cloud.lastSync')" min-width="150">
          <template #default="{ row }">
            <div>{{ fmtTime(row.last_sync_at) || '—' }}</div>
            <div style="color:#909399; font-size:12px">{{ row.last_sync_result || '' }}</div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="success" link :loading="syncingId === row.id" @click="syncNow(row)">{{ $t('cloud.syncNow') }}</el-button>
            <el-button size="small" type="warning" link @click="testNow(row)">{{ $t('db.run') }}</el-button>
            <el-button size="small" link @click="showConflicts(row)">{{ $t('cloud.conflicts') }}</el-button>
            <el-button size="small" link @click="dlgAccount(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('common.delete') + '?'" @confirm="delAccount(row)">
              <template #reference>
                <el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('cloud.tip') }}</div>
      <template #footer>
        <el-button type="primary" @click="dlgAccount()">{{ $t('cloud.addAccount') }}</el-button>
        <el-button @click="cloudDlgVisible = false">{{ $t('common.cancel') }}</el-button>
      </template>
    </el-dialog>

    <!-- add/edit cloud account -->
    <el-dialog v-model="accDlgVisible" :title="accForm.id ? $t('common.edit') : $t('cloud.addAccount')" width="520px"
               append-to-body @open="loadGroups">
      <el-form label-width="130px">
        <el-form-item :label="$t('cloud.provider')">
          <el-radio-group v-model="accForm.provider" :disabled="!!accForm.id">
            <el-radio value="aws">AWS</el-radio>
            <el-radio value="azure">Azure</el-radio>
            <el-radio value="huawei">华为云</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('webapp.name')"><el-input v-model="accForm.name" /></el-form-item>

        <template v-if="accForm.provider === 'aws' || accForm.provider === 'huawei'">
          <el-form-item :label="$t('cloud.accessKey')"><el-input v-model="cred.access_key" class="mono" /></el-form-item>
          <el-form-item :label="$t('cloud.secretKey')">
            <el-input v-model="cred.secret_key" type="password" show-password class="mono"
                      :placeholder="accForm.id ? $t('webapp.pwdKeep') : ''" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item :label="$t('cloud.tenant')"><el-input v-model="cred.tenant_id" class="mono" /></el-form-item>
          <el-form-item :label="$t('cloud.clientId')"><el-input v-model="cred.client_id" class="mono" /></el-form-item>
          <el-form-item :label="$t('cloud.clientSecret')">
            <el-input v-model="cred.client_secret" type="password" show-password class="mono"
                      :placeholder="accForm.id ? $t('webapp.pwdKeep') : ''" />
          </el-form-item>
          <el-form-item :label="$t('cloud.subscription')"><el-input v-model="cred.subscription_id" class="mono" /></el-form-item>
        </template>
        <el-form-item :label="$t('cloud.endpoint')">
          <el-input v-model="cred.endpoint" class="mono" :placeholder="$t('cloud.endpointPh')" />
        </el-form-item>

        <el-form-item :label="$t('cloud.regions')">
          <el-select v-model="regionList" multiple filterable allow-create default-first-option
                     style="width:100%" :placeholder="$t('cloud.regionsPh')" />
        </el-form-item>
        <el-form-item :label="$t('hosts.groupMgmt')">
          <el-select v-model="accForm.target_group_id" clearable style="width:100%" :placeholder="$t('db.allGroups')">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('cloud.tagGroupKey')">
          <el-input v-model="accForm.tag_group_key" class="mono" :placeholder="$t('cloud.tagGroupPh')" style="width:60%" />
        </el-form-item>
        <el-form-item :label="$t('cloud.template')">
          <el-select v-model="accForm.template_id" clearable style="width:100%" :placeholder="$t('cloud.templatePh')">
            <el-option v-for="tp in templates" :key="tp.id" :label="tp.username + (tp.label ? ' · ' + tp.label : '')" :value="tp.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('cloud.templateTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('cloud.importStopped')"><el-switch v-model="accForm.import_stopped" /></el-form-item>
        <el-form-item :label="$t('cloud.autoDelete')"><el-switch v-model="accForm.auto_delete" /></el-form-item>
        <el-form-item :label="$t('cloud.interval')">
          <el-input-number v-model="accForm.sync_interval_min" :min="0" :max="10080" />
          <span style="margin-left:6px; color:#909399; font-size:12px">min / 0={{ $t('cloud.manual') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="accDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveAccount">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- conflicts -->
    <el-dialog v-model="conflictVisible" :title="$t('cloud.conflictTitle')" width="560px" append-to-body>
      <el-table :data="conflicts" size="small" border max-height="320">
        <el-table-column :label="$t('cloud.conflictDetail')" >
          <template #default="{ row }">{{ row }}</template>
        </el-table-column>
      </el-table>
      <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('cloud.conflictTip') }}</div>
      <template #footer>
        <el-button @click="conflictVisible = false">{{ $t('common.cancel') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global
const visible = ref(false)
const accDlgVisible = ref(false)
const conflictVisible = ref(false)
const accounts = ref([])
const groups = ref([])
const templates = ref([])
const conflicts = ref([])
const loading = ref(false)
const syncingId = ref(null)
const accForm = ref({})
const cred = ref({})
const regionList = ref([])

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 16) : '')

const loadAccounts = async () => {
  loading.value = true
  try { accounts.value = await api.get('/cloudaccounts') } finally { loading.value = false }
}
const loadGroups = async () => {
  try { groups.value = await api.get('/host_groups') } catch { groups.value = [] }
  try { templates.value = await api.get('/credentials/templates') } catch { templates.value = [] }
}

const syncNow = async row => {
  syncingId.value = row.id
  try {
    const r = await api.post(`/cloudaccounts/${row.id}/sync`)
    ElMessage.success(t('cloud.syncDone', { a: r.added, u: r.updated, s: r.skipped_ip, d: r.removed }))
    loadAccounts()
  } finally { syncingId.value = null }
}
const testNow = async row => {
  const r = await api.post(`/cloudaccounts/${row.id}/test`)
  ElMessage.success(t('cloud.testOk', { n: r.total }))
}
const showConflicts = async row => {
  const r = await api.get(`/cloudaccounts/${row.id}/conflicts`)
  conflicts.value = r.conflicts || []
  conflictVisible.value = true
}

const dlgAccount = row => {
  accForm.value = row
    ? { ...row }
    : { provider: 'aws', name: '', regions: '', target_group_id: null, template_id: null, import_stopped: false,
        tag_group_key: '', sync_interval_min: 0, auto_delete: false }
  cred.value = {}
  regionList.value = accForm.value.regions ? accForm.value.regions.split(',').map(x2 => x2.trim()).filter(Boolean) : []
  accDlgVisible.value = true
}
const saveAccount = async () => {
  if (!accForm.value.name) { ElMessage.warning(t('webapp.needNameUrl')); return }
  const payload = { ...accForm.value, regions: regionList.value.join(','), credentials: { ...cred.value } }
  if (payload.id) await api.put(`/cloudaccounts/${payload.id}`, payload)
  else await api.post('/cloudaccounts', payload)
  ElMessage.success(t('common.success'))
  accDlgVisible.value = false
  loadAccounts()
}
const delAccount = async row => {
  await api.delete(`/cloudaccounts/${row.id}`)
  ElMessage.success(t('common.success'))
  loadAccounts()
}
defineExpose({ open: () => { visible.value = true; loadAccounts() } })
</script>
