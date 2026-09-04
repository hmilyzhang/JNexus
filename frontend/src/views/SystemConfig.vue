<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div v-loading="loading" style="max-width:860px">
    <el-tabs v-model="activeTab">
    <el-tab-pane :label="$t('system.general')" name="general">
    <el-card>
      <el-form label-width="140px">
        <el-form-item :label="$t('system.systemName')">
          <el-input v-model="form.system_name" style="width:320px" />
        </el-form-item>
        <el-form-item :label="$t('system.version')">
          <el-tag>Version 1.0</el-tag>
          <el-tag type="info" style="margin-left:8px">By JJ Zhang</el-tag>
        </el-form-item>
      </el-form>
      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </el-card>
    </el-tab-pane>

    <el-tab-pane :label="$t('rot.tab')" name="rotation">
    <el-card>
      <el-form label-width="150px">
        <el-form-item :label="$t('rot.globalEnable')">
          <el-switch v-model="form.rotation_enabled" active-value="true" inactive-value="false" />
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('rot.globalTip') }}</div>
        </el-form-item>
        <template v-if="form.rotation_enabled === 'true'">
          <el-form-item :label="$t('rot.policyLength')">
            <el-input-number v-model="rotationLength" :min="8" :max="64" />
            <span style="margin-left:8px; color:#909399; font-size:12px">8-64</span>
          </el-form-item>
          <el-form-item :label="$t('rot.policyComplexity')">
            <el-radio-group v-model="rotationComplexity">
              <el-radio value="high">High</el-radio>
              <el-radio value="medium">Medium</el-radio>
              <el-radio value="low">Low</el-radio>
            </el-radio-group>
            <div style="color:#909399; font-size:12px; width:100%">
              High = {{ $t('rot.cHigh') }}；Medium = {{ $t('rot.cMedium') }}；Low = {{ $t('rot.cLow') }}
            </div>
          </el-form-item>
          <el-form-item :label="$t('rot.policyDays')">
            <el-input-number v-model="rotationDays" :min="1" :max="365" />
            <span style="margin-left:8px; color:#909399; font-size:12px">{{ $t('rot.policyDaysTip') }}</span>
          </el-form-item>
        </template>
      </el-form>
      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </el-card>
    </el-tab-pane>

    <el-tab-pane :label="$t('system.ldap')" name="ldap">
    <el-card>
      <el-form label-width="140px">
        <el-form-item :label="$t('system.ldapEnabled')"><el-switch v-model="form.ldap_enabled" active-value="true" inactive-value="false" /></el-form-item>
        <template v-if="form.ldap_enabled === 'true'">
          <el-form-item :label="$t('system.ldapHost')">
            <el-input v-model="form.ldap_host" placeholder="ldap.example.com" style="width:320px" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapPort')"><el-input-number v-model="ldapPort" :min="1" :max="65535" /></el-form-item>
          <el-form-item :label="$t('system.ldapTls')"><el-switch v-model="form.ldap_tls" active-value="true" inactive-value="false" /></el-form-item>
          <el-form-item :label="$t('system.ldapBindDn')">
            <el-input v-model="form.ldap_bind_dn" placeholder="cn=admin,dc=example,dc=com" style="width:420px" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapBindPwd')">
            <el-input v-model="form.ldap_bind_password" type="password" show-password :placeholder="$t('system.ldapBindPwdPlaceholder')" style="width:320px" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapBaseDn')">
            <el-input v-model="form.ldap_base_dn" placeholder="dc=example,dc=com" style="width:420px" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapUserFilter')">
            <el-input v-model="form.ldap_user_filter" placeholder="(uid=%s)" style="width:320px" class="mono" />
            <div style="color:#909399; font-size:12px">{{ $t('system.ldapUserFilterTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.ldapAttrUsername')">
            <el-input v-model="form.ldap_attr_username" placeholder="uid" style="width:200px" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapDefaultRole')">
            <el-select v-model="form.ldap_default_role" style="width:200px">
              <el-option label="Viewer" value="viewer" />
              <el-option :label="$t('layout.roleOps')" value="ops" />
              <el-option :label="$t('layout.rolePublisher')" value="publisher" />
            </el-select>
          </el-form-item>
          <el-divider style="margin:8px 0 16px" />
          <el-form-item :label="$t('system.ldapGroupCheck')">
            <el-switch v-model="form.ldap_group_check" active-value="true" inactive-value="false" />
            <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('system.ldapGroupCheckTip') }}</div>
          </el-form-item>
          <template v-if="form.ldap_group_check === 'true'">
            <el-form-item :label="$t('system.ldapGroupBaseDn')">
              <el-input v-model="form.ldap_group_base_dn" placeholder="ou=groups,dc=example,dc=com" style="width:420px" class="mono" />
            </el-form-item>
            <el-form-item :label="$t('system.ldapGroupFilter')">
              <el-input v-model="form.ldap_group_filter" placeholder="(member=%s)" style="width:320px" class="mono" />
              <div style="color:#909399; font-size:12px">{{ $t('system.ldapGroupFilterTip') }}</div>
            </el-form-item>
            <el-form-item :label="$t('system.ldapRequiredGroups')">
              <el-input v-model="form.ldap_required_groups" type="textarea" :rows="3" class="mono"
                        placeholder="cn=ops,ou=groups,dc=example,dc=com&#10;ops-admin" />
              <div style="color:#909399; font-size:12px">{{ $t('system.ldapRequiredGroupsTip') }}</div>
            </el-form-item>
          </template>
          <el-form-item>
            <el-button @click="testLdap" :loading="testing">{{ $t('system.testConn') }}</el-button>
          </el-form-item>
        </template>
      </el-form>
    </el-card>
    <div style="margin-top:12px">
      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </div>
    </el-tab-pane>

    <el-tab-pane :label="$t('system.smtp')" name="smtp">
    <el-card>
      <el-form label-width="150px">
        <el-form-item :label="$t('system.smtpEnabled')"><el-switch v-model="form.smtp_enabled" active-value="true" inactive-value="false" /></el-form-item>
        <template v-if="form.smtp_enabled === 'true'">
          <el-form-item :label="$t('system.smtpHost')">
            <el-input v-model="form.smtp_host" placeholder="smtp.example.com" style="width:320px" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.smtpPort')"><el-input-number v-model="smtpPort" :min="1" :max="65535" /></el-form-item>
          <el-form-item :label="$t('system.smtpMode')">
            <el-checkbox v-model="smtpSsl">{{ $t('system.smtpSsl') }} (465)</el-checkbox>
            <el-checkbox v-model="form.smtp_tls" style="margin-left:12px">{{ $t('system.smtpTls') }} (587)</el-checkbox>
          </el-form-item>
          <el-form-item :label="$t('system.smtpUsername')"><el-input v-model="form.smtp_username" style="width:320px" class="mono" /></el-form-item>
          <el-form-item :label="$t('system.smtpPassword')">
            <el-input v-model="form.smtp_password" type="password" show-password :placeholder="$t('system.ldapBindPwdPlaceholder')" style="width:320px" />
          </el-form-item>
          <el-form-item :label="$t('system.smtpFrom')"><el-input v-model="form.smtp_from" placeholder="autoops@example.com" style="width:320px" class="mono" /></el-form-item>
          <el-form-item :label="$t('system.smtpRecipients')">
            <el-input v-model="form.smtp_recipients" type="textarea" :rows="2" class="mono"
                      placeholder="ops@example.com,boss@example.com" style="width:420px" />
            <div style="color:#909399; font-size:12px">{{ $t('system.smtpRecipientsTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.smtpNotify')"><el-switch v-model="form.smtp_notify" active-value="true" inactive-value="false" />
            <div style="color:#909399; font-size:12px">{{ $t('system.smtpNotifyTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.smtpTest')">
            <div style="display:flex; gap:8px; align-items:center">
              <el-input v-model="smtpTestTo" :placeholder="$t('system.smtpTestTo')" style="width:240px" class="mono" />
              <el-button :loading="smtpTesting" @click="testSmtp">{{ $t('system.smtpSendTest') }}</el-button>
            </div>
          </el-form-item>
        </template>
      </el-form>
      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </el-card>
    </el-tab-pane>

    <el-tab-pane :label="$t('menu.paired')" name="paired">
      <Paired />
    </el-tab-pane>

    <el-tab-pane :label="$t('system.roles')" name="roles">
    <el-card>
      <el-table :data="roleRows" size="small" border>
        <el-table-column :label="$t('users.role')" min-width="100">
          <template #default="{ row }"><el-tag size="small">{{ row.label }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="$t('scripts.desc')" min-width="200">
          <template #default="{ row }">
            <el-input v-model="row.desc" size="small" :disabled="row.role === 'admin'" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.credPerm')" min-width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.cred" size="small" :disabled="row.role === 'admin'" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.reportPerm')" min-width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.report" size="small" :disabled="row.role === 'admin'" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.hostPerms')" min-width="190">
          <template #default="{ row }">
            <div style="display:flex; flex-wrap:wrap; row-gap:2px">
              <el-checkbox v-model="row.host.view" size="small" style="width:50%; margin-right:0">{{ $t('system.permView') }}</el-checkbox>
              <el-checkbox v-model="row.host.create" size="small" style="width:50%; margin-right:0" :disabled="row.role === 'admin'">{{ $t('system.permCreate') }}</el-checkbox>
              <el-checkbox v-model="row.host.edit" size="small" style="width:50%; margin-right:0" :disabled="row.role === 'admin'">{{ $t('system.permEdit') }}</el-checkbox>
              <el-checkbox v-model="row.host.delete" size="small" style="width:50%; margin-right:0" :disabled="row.role === 'admin'">{{ $t('system.permDelete') }}</el-checkbox>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.menuPerms')" min-width="150">
          <template #default="{ row }">
            <el-button size="small" :disabled="row.role === 'admin'" @click="openMenuDlg(row)">
              {{ row.menus.length }}/{{ menuKeys.length }} · {{ $t('system.editMenus') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px">
        <el-button type="primary" :loading="savingRoles" @click="saveRoles">{{ $t('common.save') }}</el-button>
        <span style="color:#909399; font-size:12px; margin-left:10px">{{ $t('system.rolesTip') }}</span>
      </div>
    </el-card>
    </el-tab-pane>
    </el-tabs>

    <!-- 菜单权限编辑 -->
    <el-dialog v-model="menuDlgVisible" :title="`${$t('system.menuPerms')}：${menuDlgRole?.label}`" width="420px">
      <el-checkbox-group v-model="menuDlgSelection">
        <el-checkbox v-for="m in menuKeys" :key="m.key" :value="m.key" style="display:block; margin-left:0">
          {{ $t(m.label) }}<span style="color:#c0c4cc; font-size:12px">（{{ m.key }}）</span>
        </el-checkbox>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="menuDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmMenuDlg">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import Paired from './Paired.vue'

const { t } = i18n.global
const activeTab = ref('general')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const form = reactive({
  system_name: '', ldap_enabled: 'false', ldap_host: '', ldap_port: '389', ldap_tls: 'false',
  ldap_bind_dn: '', ldap_bind_password: '', ldap_base_dn: '', ldap_user_filter: '(uid=%s)',
  ldap_attr_username: 'uid', ldap_default_role: 'viewer',
  ldap_group_check: 'false', ldap_group_base_dn: '', ldap_group_filter: '(member=%s)', ldap_required_groups: '',
  smtp_enabled: 'false', smtp_host: '', smtp_port: '25', smtp_ssl: 'false', smtp_tls: 'true',
  smtp_username: '', smtp_password: '', smtp_from: '', smtp_recipients: '', smtp_notify: 'true',
  rotation_enabled: 'false'
})
const rotationLength = ref(20)
const rotationComplexity = ref('high')
const rotationDays = ref(90)
const smtpPort = ref(25)
const smtpSsl = ref(false)
const smtpTestTo = ref('')
const smtpTesting = ref(false)
const ldapPort = ref(389)

const roleRows = ref([])
// 角色名/描述按语言包本地化显示（system.role<Cap> / layout.role<Cap>），保存的是显示值
const roleCaps = { admin: 'Admin', ops: 'Ops', publisher: 'Publisher', viewer: 'Viewer', auditor: 'Auditor' }
const menuKeys = [
  { key: 'dashboard', label: 'menu.dashboard' },
  { key: 'shell', label: 'shell.title' },
  { key: 'hosts', label: 'menu.hosts' },
  { key: 'osaccounts', label: 'menu.osaccounts' },
  { key: 'paired', label: 'menu.paired' },
  { key: 'exec', label: 'menu.exec' },
  { key: 'tasks', label: 'menu.tasks' },
  { key: 'cron', label: 'menu.cron' },
  { key: 'reports', label: 'menu.reports' },
  { key: 'files', label: 'menu.files' },
  { key: 'scripts', label: 'menu.scripts' },
  { key: 'apps', label: 'menu.apps' },
  { key: 'releases', label: 'menu.releases' },
  { key: 'users', label: 'menu.users' },
  { key: 'danger', label: 'menu.danger' },
  { key: 'audit', label: 'menu.audit' },
  { key: 'system', label: 'menu.system' }
]
const menuDlgVisible = ref(false)
const menuDlgRole = ref(null)
const menuDlgSelection = ref([])
const savingRoles = ref(false)

const loadRoles = async () => {
  const rs = await api.get('/system/roles')
  roleRows.value = Object.entries(rs).map(([role, v]) => {
    const cap = roleCaps[role] || ''
    return {
      role,
      label: cap ? t('layout.role' + cap) : role,
      desc: cap && i18n.global.te('system.role' + cap) ? t('system.role' + cap) : v.desc,
      menus: [...(v.menus || [])],
      host: { ...v.host }, cred: !!v.cred, report: !!v.report
    }
  })
}
const openMenuDlg = row => {
  menuDlgRole.value = row
  menuDlgSelection.value = [...row.menus]
  menuDlgVisible.value = true
}
const confirmMenuDlg = () => {
  menuDlgRole.value.menus = [...menuDlgSelection.value]
  menuDlgVisible.value = false
}
const saveRoles = async () => {
  savingRoles.value = true
  try {
    const payload = {}
    for (const r of roleRows.value) {
      payload[r.role] = { desc: r.desc, menus: r.menus, host: r.host, cred: r.cred, report: r.report }
    }
    await api.put('/system/roles', payload)
    ElMessage.success(t('system.saved'))
  } finally { savingRoles.value = false }
}

onMounted(async () => {
  try {
    const cfg = await api.get('/system/config')
    for (const k of Object.keys(form)) {
      if (cfg[k] !== undefined && cfg[k] !== null) form[k] = cfg[k]
    }
    rotationLength.value = Number(form.rotation_length) || 20
    rotationComplexity.value = form.rotation_complexity || 'high'
    rotationDays.value = Number(form.rotation_days) || 90
    smtpPort.value = Number(form.smtp_port) || 25
    smtpSsl.value = form.smtp_ssl === 'true'
    ldapPort.value = Number(form.ldap_port) || 389
    if (cfg.ldap_bind_password === '******') form.ldap_bind_password = '******'
    await loadRoles()
  } finally { loading.value = false }
})

const save = async () => {
  saving.value = true
  try {
    const payload = { ...form, ldap_port: String(ldapPort.value), smtp_port: String(smtpPort.value),
      smtp_ssl: smtpSsl.value ? 'true' : 'false',
      rotation_length: String(rotationLength.value), rotation_complexity: rotationComplexity.value,
      rotation_days: String(rotationDays.value) }
    await api.put('/system/config', payload)
    localStorage.setItem('system_name', form.system_name)
    document.title = form.system_name
    ElMessage.success(t('system.saved'))
  } finally { saving.value = false }
}

const testSmtp = async () => {
  smtpTesting.value = true
  try {
    await api.put('/system/config', { ...form, ldap_port: String(ldapPort.value), smtp_port: String(smtpPort.value), smtp_ssl: smtpSsl.value ? 'true' : 'false' })
    await api.post('/system/smtp/test', { to: smtpTestTo.value })
    ElMessage.success(t('system.smtpTestOk'))
  } finally { smtpTesting.value = false }
}

const testLdap = async () => {
  testing.value = true
  try {
    await api.put('/system/config', { ...form, ldap_port: String(ldapPort.value) })
    await api.post('/system/ldap/test')
    ElMessage.success(t('system.testOk'))
  } finally { testing.value = false }
}
</script>
