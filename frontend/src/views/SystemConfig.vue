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

    <el-tab-pane :label="$t('system.roles')" name="roles">
    <el-card>
      <el-table :data="roleRows" size="small" border>
        <el-table-column :label="$t('users.role')" width="110">
          <template #default="{ row }"><el-tag size="small">{{ row.label }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="$t('scripts.desc')" min-width="200">
          <template #default="{ row }">
            <el-input v-model="row.desc" size="small" :disabled="row.role === 'admin'" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.hostPerms')" width="300">
          <template #default="{ row }">
            <el-checkbox v-model="row.host.view" size="small">{{ $t('system.permView') }}</el-checkbox>
            <el-checkbox v-model="row.host.create" size="small" :disabled="row.role === 'admin'">{{ $t('system.permCreate') }}</el-checkbox>
            <el-checkbox v-model="row.host.edit" size="small" :disabled="row.role === 'admin'">{{ $t('system.permEdit') }}</el-checkbox>
            <el-checkbox v-model="row.host.delete" size="small" :disabled="row.role === 'admin'">{{ $t('system.permDelete') }}</el-checkbox>
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.menuPerms')" min-width="160">
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

const { t } = i18n.global
const activeTab = ref('general')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const form = reactive({
  system_name: '', ldap_enabled: 'false', ldap_host: '', ldap_port: '389', ldap_tls: 'false',
  ldap_bind_dn: '', ldap_bind_password: '', ldap_base_dn: '', ldap_user_filter: '(uid=%s)',
  ldap_attr_username: 'uid', ldap_default_role: 'viewer',
  ldap_group_check: 'false', ldap_group_base_dn: '', ldap_group_filter: '(member=%s)', ldap_required_groups: ''
})
const ldapPort = ref(389)

const roleRows = ref([])
const roleLabels = {
  admin: 'Admin', ops: 'Ops', publisher: 'Publisher', viewer: 'Viewer', auditor: 'Auditor'
}
const menuKeys = [
  { key: 'dashboard', label: 'menu.dashboard' },
  { key: 'shell', label: 'shell.title' },
  { key: 'hosts', label: 'menu.hosts' },
  { key: 'exec', label: 'menu.exec' },
  { key: 'tasks', label: 'menu.tasks' },
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
  roleRows.value = Object.entries(rs).map(([role, v]) => ({
    role, label: roleLabels[role] || role,
    desc: v.desc, menus: [...(v.menus || [])],
    host: { ...v.host }
  }))
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
      payload[r.role] = { desc: r.desc, menus: r.menus, host: r.host }
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
    ldapPort.value = Number(form.ldap_port) || 389
    if (cfg.ldap_bind_password === '******') form.ldap_bind_password = '******'
    await loadRoles()
  } finally { loading.value = false }
})

const save = async () => {
  saving.value = true
  try {
    const payload = { ...form, ldap_port: String(ldapPort.value) }
    await api.put('/system/config', payload)
    localStorage.setItem('system_name', form.system_name)
    ElMessage.success(t('system.saved'))
  } finally { saving.value = false }
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
