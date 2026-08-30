<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div v-loading="loading" style="max-width:860px">
    <el-card :header="$t('system.general')">
      <el-form label-width="140px">
        <el-form-item :label="$t('system.systemName')">
          <el-input v-model="form.system_name" style="width:320px" />
        </el-form-item>
        <el-form-item :label="$t('system.version')">
          <el-tag>Version 1.0</el-tag>
          <el-tag type="info" style="margin-left:8px">By JJ Zhang</el-tag>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card :header="$t('system.ldap')" style="margin-top:16px">
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
          <el-form-item>
            <el-button @click="testLdap" :loading="testing">{{ $t('system.testConn') }}</el-button>
          </el-form-item>
        </template>
      </el-form>
    </el-card>

    <el-card :header="$t('system.roles')" style="margin-top:16px">
      <el-table :data="roleRows" size="small" border>
        <el-table-column :label="$t('users.role')" width="140">
          <template #default="{ row }"><el-tag size="small">{{ row.label }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="desc" />
      </el-table>
    </el-card>

    <div style="margin-top:16px">
      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const form = reactive({
  system_name: '', ldap_enabled: 'false', ldap_host: '', ldap_port: '389', ldap_tls: 'false',
  ldap_bind_dn: '', ldap_bind_password: '', ldap_base_dn: '', ldap_user_filter: '(uid=%s)',
  ldap_attr_username: 'uid', ldap_default_role: 'viewer'
})
const ldapPort = ref(389)

const roleRows = computed(() => [
  { label: 'Admin', desc: t('system.roleAdmin') },
  { label: 'Ops', desc: t('system.roleOps') },
  { label: 'Publisher', desc: t('system.rolePublisher') },
  { label: 'Viewer', desc: t('system.roleViewer') }
])

onMounted(async () => {
  try {
    const cfg = await api.get('/system/config')
    for (const k of Object.keys(form)) {
      if (cfg[k] !== undefined && cfg[k] !== null) form[k] = cfg[k]
    }
    ldapPort.value = Number(form.ldap_port) || 389
    if (cfg.ldap_bind_password === '******') form.ldap_bind_password = '******'
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
