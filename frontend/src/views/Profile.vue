<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-row :gutter="16">
      <!-- Basic info -->
      <el-col :span="12">
        <el-card>
          <template #header>{{ $t('profile.basicInfo') }}</template>
          <el-form label-width="110px" style="max-width:460px">
            <el-form-item :label="$t('users.username')">
              <el-input :model-value="me.username" disabled />
            </el-form-item>
            <el-form-item :label="$t('users.displayName')">
              <el-input v-model="me.display_name" :disabled="me.auth_source === 'ldap'"
                        :placeholder="me.auth_source === 'ldap' ? $t('users.displayNameLdapHint') : ''">
                <template #append v-if="me.auth_source !== 'ldap'">
                  <el-button @click="saveDisplayName">{{ $t('common.save') }}</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item :label="$t('users.role')">
              <el-input :model-value="roleLabel" disabled />
            </el-form-item>
            <el-form-item :label="$t('users.authSource')">
              <el-tag size="small" :type="me.auth_source === 'ldap' ? 'warning' : 'info'">
                {{ me.auth_source === 'ldap' ? $t('users.authLdap') : $t('users.authLocal') }}
              </el-tag>
            </el-form-item>
            <el-form-item :label="$t('users.email')">
              <el-input v-model="me.email" :disabled="me.auth_source === 'ldap'"
                        :placeholder="me.auth_source === 'ldap' ? $t('users.emailLdapHint') : ''">
                <template #append v-if="me.auth_source !== 'ldap'">
                  <el-button @click="saveEmail">{{ $t('common.save') }}</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item :label="$t('users.lastLogin')">
              <span class="muted">{{ me.last_login_at || '-' }}</span>
            </el-form-item>
          </el-form>
          <el-divider style="margin:6px 0 16px" />
          <div v-if="me.auth_source !== 'ldap'">
            <div style="font-weight:600; margin-bottom:10px">{{ $t('layout.changePwd') }}</div>
            <el-form label-width="110px" style="max-width:460px">
              <el-form-item :label="$t('layout.oldPwd')"><el-input v-model="pwd.old_password" type="password" show-password /></el-form-item>
              <el-form-item :label="$t('layout.newPwd')"><el-input v-model="pwd.new_password" type="password" show-password /></el-form-item>
              <el-form-item>
                <el-button type="primary" @click="doChangePwd">{{ $t('common.confirm') }}</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-card>
      </el-col>

      <!-- MFA -->
      <el-col :span="12">
        <el-card>
          <template #header>{{ $t('layout.mfaSecurity') }}</template>
          <div v-if="!mfaEnabled">
            <template v-if="!mfaSetup">
              <div style="color:#909399; font-size:13px; line-height:1.7; margin-bottom:12px">{{ $t('mfa.disabledTip') }}</div>
              <el-button type="primary" @click="mfaSetupStart">{{ $t('mfa.startSetup') }}</el-button>
            </template>
            <template v-else>
              <div style="text-align:center"><img v-if="mfaSetup.qr" :src="mfaSetup.qr" style="width:180px; height:180px" alt="QR" /></div>
              <div style="color:#909399; font-size:12px; text-align:center; margin:6px 0">{{ $t('mfa.scanTip') }}</div>
              <div class="mono" style="font-size:12px; text-align:center; margin-bottom:10px; word-break:break-all">{{ $t('mfa.secretKey') }}：{{ mfaSetup.secret }}</div>
              <el-input v-model="mfaCode" maxlength="6" class="mono" style="margin-bottom:10px"
                        :placeholder="$t('mfa.codePlaceholder')" @keyup.enter="mfaEnableNow" />
              <div style="display:flex; gap:8px">
                <el-button type="primary" style="flex:1" @click="mfaEnableNow">{{ $t('mfa.enable') }}</el-button>
                <el-button @click="mfaSetup = null">{{ $t('common.cancel') }}</el-button>
              </div>
            </template>
          </div>
          <template v-else>
            <el-result icon="success" :title="$t('mfa.enabledTitle')" style="padding:6px 0 12px" />
            <el-input v-model="mfaCode" maxlength="6" class="mono" style="margin-bottom:10px"
                      :placeholder="$t('mfa.disableTip')" @keyup.enter="mfaDisableNow" />
            <el-button type="danger" style="width:100%" @click="mfaDisableNow">{{ $t('mfa.disable') }}</el-button>
          </template>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const me = reactive({ username: '', role: '', auth_source: 'local', email: '', last_login_at: '' })
const pwd = reactive({ old_password: '', new_password: '' })

const roleLabel = computed(() => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer'),
  auditor: t('layout.roleAuditor')
}[me.role] || me.role))

const loadMe = async () => {
  const info = await api.get('/me')
  Object.assign(me, info)
  me.last_login_at = me.last_login_at ? String(me.last_login_at).replace('T', ' ').slice(0, 19) : ''
}

const saveEmail = async () => {
  await api.put('/me', { email: me.email })
  ElMessage.success(t('common.success'))
}

const saveDisplayName = async () => {
  await api.put('/me', { display_name: me.display_name })
  ElMessage.success(t('common.success'))
}

const doChangePwd = async () => {
  await api.post('/change_password', pwd)
  ElMessage.success(t('common.success'))
  pwd.old_password = ''
  pwd.new_password = ''
}

// ---- MFA ----
const mfaEnabled = ref(false)
const mfaSetup = ref(null)
const mfaCode = ref('')
const loadMfa = async () => {
  const s = await api.get('/mfa/status')
  mfaEnabled.value = !!s.enabled
  if (!mfaEnabled.value) mfaSetup.value = null
  mfaCode.value = ''
}
const mfaSetupStart = async () => { mfaSetup.value = await api.post('/mfa/setup') }
const mfaEnableNow = async () => {
  if (mfaCode.value.length !== 6) { ElMessage.warning(t('mfa.codePlaceholder')); return }
  await api.post('/mfa/enable', { code: mfaCode.value })
  ElMessage.success(t('mfa.enableOk'))
  mfaSetup.value = null
  mfaCode.value = ''
  loadMfa()
}
const mfaDisableNow = async () => {
  await api.post('/mfa/disable', { code: mfaCode.value })
  ElMessage.success(t('mfa.disableOk'))
  mfaCode.value = ''
  loadMfa()
}

onMounted(async () => {
  await loadMe()
  await loadMfa()
})
</script>

<style scoped>
.muted { color: #909399; }
</style>
