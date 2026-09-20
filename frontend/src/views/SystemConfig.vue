<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div v-loading="loading">
    <el-tabs v-model="activeTab">
    <el-tab-pane :label="$t('system.general')" name="general">
    <el-card>
      <el-form label-width="140px">
        <el-form-item :label="$t('system.systemName')">
          <el-input v-model="form.system_name" />
        </el-form-item>
        <el-form-item :label="$t('system.version')">
          <el-tag>v{{ appVersion }}</el-tag>
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

    <el-card style="margin-top:16px">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('rot.applicableAccts') }}（{{ rotAccounts.length }}）</span>
          <el-button size="small" @click="loadRotAccounts">{{ $t('common.refresh') }}</el-button>
          <el-button size="small" type="warning" :disabled="!rotAccounts.length"
                     @click="runAllNow">{{ $t('rot.runNow') }}</el-button>
        </div>
      </template>
      <el-table :data="rotAccounts" size="small" border max-height="420">
        <el-table-column prop="host" :label="$t('menu.hosts')" min-width="150" show-overflow-tooltip />
        <el-table-column prop="username" :label="$t('hosts.credUser')" min-width="110" />
        <el-table-column :label="$t('rot.enable')" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.rotate_enabled" size="small">{{ row.days }}{{ $t('rot.daysUnit') }}</el-tag>
            <span v-else style="color:var(--el-text-color-secondary)">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.due')" width="80" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.due ? 'danger' : 'success'">{{ row.due ? $t('rot.dueYes') : $t('rot.dueNo') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.lastRot')" min-width="150">
          <template #default="{ row }">
            <span :style="{ color: (row.last_rotation_result || '').includes('失败') ? 'var(--el-color-danger)' : 'inherit' }">
              {{ row.last_rotation_result || '—' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="100" align="center">
          <template #default="{ row }">
            <el-button size="small" link type="warning" @click="revealAcct(row)">{{ $t('rot.view') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

    <el-dialog v-model="revealVisible" :title="$t('rot.viewTitle')" width="420px" append-to-body>
      <el-form label-width="90px">
        <el-form-item :label="$t('hosts.credUser')"><span class="mono">{{ revealAcctData.username }}</span></el-form-item>
        <el-form-item :label="$t('hosts.password')"><span class="mono" style="font-weight:bold">{{ revealAcctData.password }}</span></el-form-item>
      </el-form>
      <el-alert type="warning" :closable="false" :title="$t('rot.revealAudit')" />
    </el-dialog>
    </el-card>
    </el-tab-pane>

    <!-- AI assistant tab: left nav + right content -->
    <el-tab-pane :label="$t('ai.tab')" name="ai">
    <el-card>
      <div style="display:flex; gap:18px; min-height:420px">
        <!-- Left nav -->
        <div class="ai-side-nav">
          <div class="ai-side-nav-title">{{ $t('ai.tab') }}</div>
          <div class="ai-nav-item" :class="{ active: aiTabSection === 'conn' }" @click="aiTabSection = 'conn'">
            <el-icon><Connection /></el-icon> {{ $t('ai.navConn') }}
          </div>
          <div class="ai-nav-item" :class="{ active: aiTabSection === 'role' }" @click="aiTabSection = 'role'">
            <el-icon><User /></el-icon> {{ $t('ai.navRole') }}
          </div>
          <div class="ai-nav-item" :class="{ active: aiTabSection === 'sec' }" @click="aiTabSection = 'sec'">
            <el-icon><Lock /></el-icon> {{ $t('ai.navSec') }}
          </div>
          <!-- Alert diagnostics moved to Monitoring Center (AI Diagnostics tab) -->
        </div>
        <!-- Right content -->
        <div style="flex:1; min-width:0; padding-left:20px; border-left:1px solid var(--el-border-color-lighter)">
          <!-- Connection settings -->
          <template v-if="aiTabSection === 'conn'">
            <div style="font-weight:600; font-size:15px; margin-bottom:16px">{{ $t('ai.cfgTitle') }}</div>
            <el-form label-width="130px" style="max-width:520px">
              <el-form-item :label="$t('ai.enabled')">
                <el-switch v-model="form.ai_enabled" active-value="true" inactive-value="false" />
              </el-form-item>
              <el-form-item :label="$t('ai.baseUrl')">
                <el-input v-model="form.ai_base_url" class="mono" placeholder="http://127.0.0.1:11434/v1" />
                <div style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.baseUrlTip') }}</div>
              </el-form-item>
              <el-form-item :label="$t('ai.apiKey')">
                <el-input v-model="form.ai_api_key" type="password" show-password class="mono" :placeholder="$t('ai.apiKeyTip')" />
              </el-form-item>
              <el-form-item :label="$t('ai.model')">
                <el-input v-model="form.ai_model" class="mono" placeholder="qwen2.5:7b" />
              </el-form-item>
              <el-form-item :label="$t('ai.timeout')">
                <el-input-number v-model="aiTimeoutNum" :min="5" :max="600" />
                <span style="margin-left:8px; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.timeoutTip') }}</span>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
                <el-button :loading="aiTesting" @click="testAi">{{ $t('ai.testConn') }}</el-button>
              </el-form-item>
            </el-form>
          </template>
          <!-- Role settings: add / remove / edit roles -->
          <template v-if="aiTabSection === 'role'">
            <div style="font-weight:600; font-size:15px; margin-bottom:16px">{{ $t('ai.roleTitle') }}</div>
            <el-form label-width="130px" style="max-width:520px">
              <el-form-item :label="$t('ai.roleLabel')">
                <div style="display:flex; gap:8px; width:100%">
                  <el-select v-model="aiRole" style="flex:1" @change="syncRoleEditor">
                    <el-option v-for="r in aiRoles" :key="r.key" :value="r.key" :label="r.name" />
                  </el-select>
                  <el-button @click="addAiRole">{{ $t('ai.addRole') }}</el-button>
                  <el-button :disabled="!currentAiRole" @click="delAiRole">{{ $t('ai.delRole') }}</el-button>
                </div>
              </el-form-item>
              <el-form-item :label="$t('ai.roleName')">
                <el-input v-model="currentAiRoleName" :placeholder="currentAiRole ? currentAiRole.key : ''" />
              </el-form-item>
              <el-form-item :label="$t('ai.systemPrompt')">
                <el-input v-model="aiSystemPromptEdit" type="textarea" :rows="6" class="mono" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="aiRolesSaving" @click="saveAiRoles">{{ $t('common.save') }}</el-button>
              </el-form-item>
            </el-form>
          </template>
          <!-- Security settings: chat rate limit + prompt-injection guards -->
          <template v-if="aiTabSection === 'sec'">
            <div style="font-weight:600; font-size:15px; margin-bottom:6px">{{ $t('ai.secTitle') }}</div>
            <div style="color:var(--el-text-color-secondary); font-size:12px; margin-bottom:14px">{{ $t('ai.secTip') }}</div>
            <el-form label-width="130px" style="max-width:560px">
              <el-form-item :label="$t('ai.rateLimit')">
                <el-input-number v-model="aiRateLimitNum" :min="0" :max="1000" :step="10" />
                <span style="margin-left:8px; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.rateLimitTip') }}</span>
              </el-form-item>
              <el-form-item :label="$t('ai.injGuard')">
                <el-switch v-model="form.ai_injection_guard" active-value="true" inactive-value="false" />
                <div style="width:100%; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.injGuardTip') }}</div>
              </el-form-item>
              <el-form-item :label="$t('ai.snapFilter')">
                <el-switch v-model="form.ai_snapshot_filter" active-value="true" inactive-value="false" />
                <div style="width:100%; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.snapFilterTip') }}</div>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
              </el-form-item>
            </el-form>
          </template>
          <!-- Alert diagnostics (config + cleanup catalog) moved to Monitoring Center → AI Diagnostics tab -->
        </div>
      </div>
    </el-card>
    </el-tab-pane>


    <el-tab-pane :label="$t('system.ldap')" name="ldap">
    <el-card>
      <el-form label-width="140px">
        <el-form-item :label="$t('system.ldapEnabled')"><el-switch v-model="form.ldap_enabled" active-value="true" inactive-value="false" /></el-form-item>
        <template v-if="form.ldap_enabled === 'true'">
          <el-form-item :label="$t('system.ldapHost')">
            <el-input v-model="form.ldap_host" placeholder="ldap.example.com" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapPort')"><el-input-number v-model="ldapPort" :min="1" :max="65535" /></el-form-item>
          <el-form-item :label="$t('system.ldapTls')"><el-switch v-model="form.ldap_tls" active-value="true" inactive-value="false" /></el-form-item>
          <el-form-item :label="$t('system.ldapBindDn')">
            <el-input v-model="form.ldap_bind_dn" placeholder="cn=admin,dc=example,dc=com" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapBindPwd')">
            <el-input v-model="form.ldap_bind_password" type="password" show-password :placeholder="$t('system.ldapBindPwdPlaceholder')" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapBaseDn')">
            <el-input v-model="form.ldap_base_dn" placeholder="dc=example,dc=com" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapUserFilter')">
            <el-input v-model="form.ldap_user_filter" placeholder="(uid=%s)" class="mono" />
            <div style="color:#909399; font-size:12px">{{ $t('system.ldapUserFilterTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.ldapAttrUsername')">
            <el-input v-model="form.ldap_attr_username" placeholder="uid" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.ldapDefaultRole')">
            <el-select v-model="form.ldap_default_role" filterable>
              <el-option-group :label="$t('users.builtinRoles')">
                <el-option label="Viewer" value="viewer" />
                <el-option :label="$t('layout.roleOps')" value="ops" />
                <el-option :label="$t('layout.rolePublisher')" value="publisher" />
              </el-option-group>
              <el-option-group v-if="allRoleKeys.filter(k => !['admin','ops','publisher','viewer','auditor','k8s'].includes(k)).length"
                               :label="$t('users.customRoles')">
                <el-option v-for="r in allRoleKeys.filter(k => !['admin','ops','publisher','viewer','auditor','k8s'].includes(k))"
                           :key="r" :label="r" :value="r" />
              </el-option-group>
            </el-select>
          </el-form-item>
          <el-divider style="margin:8px 0 16px" />
          <el-form-item :label="$t('system.ldapGroupCheck')">
            <el-switch v-model="form.ldap_group_check" active-value="true" inactive-value="false" />
            <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('system.ldapGroupCheckTip') }}</div>
          </el-form-item>
          <template v-if="form.ldap_group_check === 'true'">
            <el-form-item :label="$t('system.ldapGroupBaseDn')">
              <el-input v-model="form.ldap_group_base_dn" placeholder="ou=groups,dc=example,dc=com" class="mono" />
            </el-form-item>
            <el-form-item :label="$t('system.ldapGroupFilter')">
              <el-input v-model="form.ldap_group_filter" placeholder="(member=%s)" class="mono" />
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
            <el-input v-model="form.smtp_host" placeholder="smtp.example.com" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('system.smtpPort')"><el-input-number v-model="smtpPort" :min="1" :max="65535" /></el-form-item>
          <el-form-item :label="$t('system.smtpMode')">
            <el-checkbox v-model="smtpSsl">{{ $t('system.smtpSsl') }} (465)</el-checkbox>
            <el-checkbox v-model="form.smtp_tls" style="margin-left:12px">{{ $t('system.smtpTls') }} (587)</el-checkbox>
          </el-form-item>
          <el-form-item :label="$t('system.smtpUsername')"><el-input v-model="form.smtp_username" class="mono" /></el-form-item>
          <el-form-item :label="$t('system.smtpPassword')">
            <el-input v-model="form.smtp_password" type="password" show-password :placeholder="$t('system.ldapBindPwdPlaceholder')" />
          </el-form-item>
          <el-form-item :label="$t('system.smtpFrom')"><el-input v-model="form.smtp_from" placeholder="autoops@example.com" class="mono" /></el-form-item>
          <el-form-item :label="$t('system.smtpRecipients')">
            <el-input v-model="form.smtp_recipients" type="textarea" :rows="2" class="mono"
                      placeholder="ops@example.com,boss@example.com" />
            <div style="color:#909399; font-size:12px">{{ $t('system.smtpRecipientsTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.smtpNotify')"><el-switch v-model="form.smtp_notify" active-value="true" inactive-value="false" />
            <div style="color:#909399; font-size:12px">{{ $t('system.smtpNotifyTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('system.smtpTest')">
            <div style="display:flex; gap:8px; align-items:center">
              <el-input v-model="smtpTestTo" :placeholder="$t('system.smtpTestTo')" style="flex:1" class="mono" />
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
          <template #default="{ row }">
              <el-tag size="small" :type="row.custom ? 'primary' : ''" effect="plain">{{ row.labelKey ? $t(row.labelKey) : row.role }}</el-tag>
              <el-tag v-if="row.custom" size="small" type="info">{{ $t('system.customTag') }}</el-tag>
            </template>
        </el-table-column>
        <el-table-column :label="$t('scripts.desc')" min-width="200">
          <template #default="{ row }">
            <el-input :model-value="row.descKey ? $t(row.descKey) : row.desc" size="small"
                      :disabled="row.role === 'admin'"
                      @input="v => (row.desc = v)" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="90" align="center">
          <template #default="{ row }">
            <el-popconfirm v-if="row.custom" :title="$t('system.delRoleConfirm')" @confirm="delRole(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
        <el-table-column :label="$t('system.capMatrix')" min-width="130" align="center">
          <template #default="{ row }">
            <el-button size="small" :disabled="row.role === 'admin'" @click="openCapDlg(row)">
              {{ capCount(row) }} · {{ $t('system.capEdit') }}
            </el-button>
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
        <el-button @click="openNewRole">{{ $t('system.newRole') }}</el-button>
        <span style="color:#909399; font-size:12px; margin-left:10px">{{ $t('system.rolesTip') }}</span>
      </div>
    </el-card>
    </el-tab-pane>

    <el-tab-pane :label="$t('system.tabExt')" name="ext">
    <el-card>
      <div style="display:flex; gap:18px; min-height:380px">
        <div class="ext-side-nav">
          <div class="ext-side-nav-title">{{ $t('system.tabExt') }}</div>
          <div class="ext-nav-item" :class="{ active: extSection === 'keys' }" @click="extSection = 'keys'">{{ $t('system.apiKeys') }}</div>
          <div class="ext-nav-item" :class="{ active: extSection === 'oo' }" @click="extSection = 'oo'">{{ $t('oo.tab') }}</div>
          <div class="ext-nav-item" :class="{ active: extSection === 'ticketing' }" @click="extSection = 'ticketing'; loadTicketing">{{ $t('tk.nav') }}</div>
        </div>
        <div style="flex:1; min-width:0">
          <!-- OpenObserve integration -->
          <template v-if="extSection === 'oo'">
            <div class="ext-sec-head" style="margin-bottom:14px"><span style="font-weight:600">{{ $t('oo.tab') }}</span></div>
            <el-alert type="info" :title="$t('oo.tip')" :closable="false" style="margin-bottom:16px" />
            <el-form label-width="150px" style="max-width:560px">
              <el-form-item :label="$t('oo.enabled')"><el-switch v-model="form.oo_enabled" active-value="true" inactive-value="false" /></el-form-item>
              <template v-if="form.oo_enabled === 'true'">
                <el-form-item :label="$t('oo.url')">
                  <el-input v-model="form.oo_url" class="mono" placeholder="http://openobserve:5080" />
                </el-form-item>
                <el-form-item :label="$t('oo.org')">
                  <el-input v-model="form.oo_org" class="mono" placeholder="default" />
                </el-form-item>
                <el-form-item :label="$t('oo.token')">
                  <el-input v-model="form.oo_token" type="password" show-password class="mono" :placeholder="$t('oo.tokenTip')" />
                  <div style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('oo.tokenHelp') }}</div>
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
                  <el-button :loading="ooTesting" @click="testOO">{{ $t('ai.testConn') }}</el-button>
                </el-form-item>
              </template>
            </el-form>
          </template>

          <!-- Ticketing systems -->
          <template v-if="extSection === 'ticketing'">
            <div class="ext-sec-head" style="margin-bottom:14px"><span style="font-weight:600">{{ $t('tk.cfgTitle') }}</span></div>
            <el-row :gutter="18">
              <el-col :span="12">
                <div style="font-weight:600; margin-bottom:10px">ServiceNow</div>
                <el-form label-width="90px" style="max-width:420px">
                  <el-form-item label="URL"><el-input v-model="tkForm.sn_url" class="mono" placeholder="https://instance.service-now.com" /></el-form-item>
                  <el-form-item :label="$t('hosts.user')"><el-input v-model="tkForm.sn_user" class="mono" /></el-form-item>
                  <el-form-item :label="$t('hosts.password')"><el-input v-model="tkForm.sn_pass" type="password" show-password class="mono" :placeholder="$t('oa.keepPwd')" /></el-form-item>
                  <el-form-item>
                    <el-button size="small" :loading="tkTesting === 'servicenow'" @click="testTicketing('servicenow')">{{ $t('ai.testConn') }}</el-button>
                  </el-form-item>
                </el-form>
              </el-col>
              <el-col :span="12">
                <div style="font-weight:600; margin-bottom:10px">{{ $t('tk.sdpTitle') }} <el-tag size="small" effect="plain">{{ $t('tk.onPrem') }}</el-tag></div>
                <el-form label-width="90px" style="max-width:420px">
                  <el-form-item label="URL"><el-input v-model="tkForm.sdp_url" class="mono" placeholder="http://sdp.internal:8080" /></el-form-item>
                  <el-form-item :label="$t('tk.techKey')"><el-input v-model="tkForm.sdp_token" type="password" show-password class="mono" :placeholder="$t('oa.keepPwd')" /></el-form-item>
                  <el-form-item :label="$t('tk.requester')"><el-input v-model="tkForm.sdp_requester" :placeholder="$t('tk.requesterPh')" /></el-form-item>
                  <el-form-item>
                    <el-button size="small" :loading="tkTesting === 'sdp'" @click="testTicketing('sdp')">{{ $t('ai.testConn') }}</el-button>
                  </el-form-item>
                </el-form>
              </el-col>
            </el-row>
            <div style="margin-top:16px; display:flex; align-items:center; gap:10px">
              <el-button type="primary" size="small" :loading="tkSaving" @click="saveTicketing">{{ $t('common.save') }}</el-button>
              <span style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('tk.cfgTip') }}</span>
            </div>
          </template>

          <!-- API keys -->
          <template v-if="extSection === 'keys'">
            <div class="ext-sec-head" style="margin-bottom:14px; display:flex; align-items:center">
              <span style="font-weight:600; flex:1">{{ $t('system.apiKeysTitle') }}</span>
              <el-button size="small" type="primary" @click="openApiKeyDlg">{{ $t('system.apiKeyCreate') }}</el-button>
              <el-button size="small" @click="openDoc">{{ $t('system.apiKeyDoc') }}</el-button>
            </div>

      <el-table :data="apiKeys" size="small" border>
        <el-table-column prop="name" :label="$t('system.apiKeyName')" min-width="140" />
        <el-table-column label="Key" width="150">
          <template #default="{ row }"><span class="mono">aok_{{ row.key_id }}...</span></template>
        </el-table-column>
        <el-table-column prop="owner" :label="$t('system.apiKeyOwner')" width="110" />
        <el-table-column prop="purpose" :label="$t('system.apiKeyPurpose')" min-width="140" show-overflow-tooltip />
        <el-table-column :label="$t('system.apiKeyExpires')" width="160">
          <template #default="{ row }">{{ fmtApiTime(row.expires_at) }}</template>
        </el-table-column>
        <el-table-column :label="$t('system.apiKeyLastUsed')" width="150">
          <template #default="{ row }">{{ fmtApiTime(row.last_used_at) }}</template>
        </el-table-column>
        <el-table-column :label="$t('system.apiKeyEnabled')" width="90" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="toggleApiKey(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="90" fixed="right">
          <template #default="{ row }">
            <el-popconfirm :title="$t('system.apiKeyDelConfirm')" @confirm="delApiKey(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="color:#909399; font-size:12px; margin-top:10px">{{ $t('system.apiKeyTip') }}</div>
    
          </template>
        </div>
      </div>
    </el-card>
    </el-tab-pane>
    </el-tabs>

    <!-- API key create -->
    <el-dialog v-model="apiKeyDlgVisible" :title="$t('system.apiKeyCreate')" width="480px">
      <el-form label-width="120px">
        <el-form-item :label="$t('system.apiKeyName')"><el-input v-model="apiKeyForm.name" /></el-form-item>
        <el-form-item :label="$t('system.apiKeyPurpose')">
          <el-input v-model="apiKeyForm.purpose" :placeholder="$t('system.apiKeyPurposeTip')" />
        </el-form-item>
        <el-form-item :label="$t('system.apiKeyOwner')">
          <el-select v-model="apiKeyForm.owner_user_id" style="width:100%" filterable>
            <el-option v-for="u in users" :key="u.id" :label="u.username + ' (' + u.role + ')'" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('system.apiKeyExpires')">
          <el-date-picker v-model="apiKeyForm.expires_at" type="datetime"
                          value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" placeholder="-" />
        </el-form-item>
        <el-form-item :label="$t('system.apiKeyIpList')">
          <el-input v-model="apiKeyForm.ip_allowlist" class="mono" placeholder="10.0.0.8, 10.0.1.0/24" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="apiKeyDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createApiKey">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- One-time key reveal -->
    <el-dialog v-model="keyShownVisible" :title="$t('system.apiKeyCreatedTitle')" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :title="$t('system.apiKeyOnceTip')" :closable="false" style="margin-bottom:12px" />
      <div class="mono" style="background:var(--el-fill-color-light); padding:10px; border-radius:4px; word-break:break-all; font-size:13px">{{ createdKey }}</div>
      <div style="margin-top:10px; text-align:right">
        <el-button size="small" @click="copyKey">{{ $t('system.apiKeyCopy') }}</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="keyShownVisible = false">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Menu permission editor -->
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

    <!-- New custom role -->
    <el-dialog v-model="newRoleVisible" :title="$t('system.newRole')" width="480px">
      <el-form label-width="110px">
        <el-form-item :label="$t('system.roleKey')"><el-input v-model="newRoleForm.key" placeholder="sre" /></el-form-item>
        <el-form-item :label="$t('system.roleDesc')"><el-input v-model="newRoleForm.desc" /></el-form-item>
        <el-form-item :label="$t('system.copyFrom')">
          <el-select v-model="newRoleForm.copy_of" clearable style="width:100%">
            <el-option v-for="r in allRoleKeys" :key="r" :label="r" :value="r" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="newRoleVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createRole">{{ $t('common.add') }}</el-button>
      </template>
    </el-dialog>

    <!-- Module permission matrix -->
    <el-dialog v-model="capDlgVisible" :title="`${$t('system.capMatrix')}：${capRow?.label || ''}`" width="560px">
      <el-table :data="capabilities" size="small" border>
        <el-table-column prop="key" :label="$t('system.capModule')" width="160" />
        <el-table-column :label="$t('system.capActions')">
          <template #default="{ row }">
            <el-checkbox v-for="a in row.actions" :key="a" style="margin-right:12px"
                         :model-value="capHas(capRow, row.key, a)"
                         @update:model-value="v => capSet(capRow, row.key, a, v)">
              {{ $t('system.cap_' + row.key + '_' + a) }}
            </el-checkbox>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:10px; color:#909399; font-size:12px">{{ $t('system.capTip') }}</div>
      <template #footer>
        <el-button @click="capDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="capDlgVisible = false">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import Paired from './Paired.vue'

const { t } = i18n.global
const route = useRoute()
const activeTab = ref('general')
const extSection = ref('keys')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const appVersion = ref('1.0')
const form = reactive({
  system_name: '', ldap_enabled: 'false', ldap_host: '', ldap_port: '389', ldap_tls: 'false',
  ldap_bind_dn: '', ldap_bind_password: '', ldap_base_dn: '', ldap_user_filter: '(uid=%s)',
  ldap_attr_username: 'uid', ldap_default_role: 'viewer',
  ldap_group_check: 'false', ldap_group_base_dn: '', ldap_group_filter: '(member=%s)', ldap_required_groups: '',
  smtp_enabled: 'false', smtp_host: '', smtp_port: '25', smtp_ssl: 'false', smtp_tls: 'true',
  smtp_username: '', smtp_password: '', smtp_from: '', smtp_recipients: '', smtp_notify: 'true',
  rotation_enabled: 'false',
  ai_enabled: 'false', ai_base_url: '', ai_api_key: '', ai_model: '', ai_timeout_sec: '120',
  ai_chat_rate_limit: '30', ai_injection_guard: 'true', ai_snapshot_filter: 'true',
  oo_enabled: 'false', oo_url: '', oo_org: 'default', oo_token: ''
})
const rotationLength = ref(20)
const rotationComplexity = ref('high')
const rotationDays = ref(90)
const smtpPort = ref(25)
const smtpSsl = ref(false)
const smtpTestTo = ref('')
const smtpTesting = ref(false)
const ldapPort = ref(389)

// ---- API keys ----
const apiKeys = ref([])
const users = ref([])
const apiKeyDlgVisible = ref(false)
const keyShownVisible = ref(false)
const createdKey = ref('')
const apiKeyForm = reactive({ name: '', owner_user_id: null, expires_at: '', ip_allowlist: '', purpose: '' })

const fmtApiTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const loadApiKeys = async () => {
  apiKeys.value = await api.get('/api_keys')
  users.value = await api.get('/users')
}
const openApiKeyDlg = async () => {
  Object.assign(apiKeyForm, { name: '', owner_user_id: null, expires_at: '', ip_allowlist: '', purpose: '' })
  if (!users.value.length) users.value = await api.get('/users')
  apiKeyDlgVisible.value = true
}
const createApiKey = async () => {
  const payload = { ...apiKeyForm }
  if (!payload.expires_at) delete payload.expires_at
  const r = await api.post('/api_keys', payload)
  createdKey.value = r.key
  apiKeyDlgVisible.value = false
  keyShownVisible.value = true
  loadApiKeys()
}
const copyKey = async () => {
  try { await navigator.clipboard.writeText(createdKey.value); ElMessage.success(t('common.success')) } catch (e) { /* ignore */ }
}
const toggleApiKey = async row => {
  await api.put('/api_keys/' + row.id, { enabled: row.enabled })
}
const delApiKey = async row => {
  await api.delete('/api_keys/' + row.id)
  loadApiKeys()
}

const openDoc = () => { window.open('/docs/api.html', '_blank') }

const roleRows = ref([])
// Role names/descriptions are localized via language packs (system.role<Cap> / layout.role<Cap>); the display value is what gets saved
const roleCaps = { admin: 'Admin', ops: 'Ops', publisher: 'Publisher', viewer: 'Viewer', auditor: 'Auditor', k8s: 'K8s' }
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
  { key: 'monitor', label: 'menu.monitor' },
  { key: 'observe', label: 'menu.observe' },
  { key: 'k8s', label: 'k8s.title' },
  { key: 'files', label: 'menu.files' },
  { key: 'scripts', label: 'menu.scripts' },
  { key: 'logtail', label: 'menu.logtail' },
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

const capabilities = ref([])
const capDlgVisible = ref(false)
const capRow = ref(null)
const capCount = row => Object.values(row.perms || {}).reduce((n, arr) => n + arr.length, 0)
const capHas = (row, module, action) => (row.perms?.[module] || []).includes(action)
const capSet = (row, module, action, on) => {
  const arr = new Set(row.perms?.[module] || [])
  if (on) arr.add(action)
  else arr.delete(action)
  row.perms = { ...row.perms, [module]: [...arr] }
}
const openCapDlg = row => { capRow.value = row; capDlgVisible.value = true }

// ---- Custom roles ----
const newRoleVisible = ref(false)
const newRoleForm = ref({ key: '', desc: '', copy_of: '' })
const openNewRole = () => { newRoleForm.value = { key: '', desc: '', copy_of: 'viewer' }; newRoleVisible.value = true }
const createRole = async () => {
  if (!newRoleForm.value.key) { ElMessage.warning(t('users.role')); return }
  try {
    await api.post('/system/roles', newRoleForm.value)
    ElMessage.success(t('common.success'))
    newRoleVisible.value = false
    await loadRoles()
  } catch { /* interceptor already showed the error */ }
}
const delRole = async row => {
  try {
    await api.delete(`/system/roles/${row.role}`)
    ElMessage.success(t('common.success'))
    await loadRoles()
  } catch { /* interceptor already showed the error */ }
}

const allRoleKeys = ref([])
const roleMeta = ref({})
const loadRoles = async () => {
  api.get('/system/capabilities').then(r => { capabilities.value = r }).catch(() => {})
  const rs = await api.get('/system/roles')
  allRoleKeys.value = Object.keys(rs)
  roleMeta.value = rs
  roleRows.value = Object.entries(rs).map(([role, v]) => {
    const cap = roleCaps[role] || ''
    return {
      role,
      custom: !['admin', 'ops', 'publisher', 'viewer', 'auditor', 'k8s'].includes(role),
      labelKey: cap ? 'layout.role' + cap : '',
      descKey: cap && i18n.global.te('system.role' + cap) ? 'system.role' + cap : '',
      desc: v.desc || '',
      menus: [...(v.menus || [])],
      perms: { ...(v.perms || {}) },
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
      payload[r.role] = { desc: r.desc, menus: r.menus, perms: r.perms }
    }
    await api.put('/system/roles', payload)
    ElMessage.success(t('system.saved'))
  } finally { savingRoles.value = false }
}

// ---- Password rotation: applicable accounts + run-all-now (reuses the async batch) ----
const rotAccounts = ref([])
const rotBatchId = ref('')
const rotBatchProg = ref({ done: 0, total: 0, ok: 0, failed: 0, running: false, results: [] })
let rotBatchTimer = null
const rotBatchPollStop = () => { if (rotBatchTimer) { clearInterval(rotBatchTimer); rotBatchTimer = null } }

const loadRotAccounts = async () => {
  try {
    const r = await api.get('/system/rotation/accounts')
    rotAccounts.value = r.accounts || []
  } catch { /* ignore */ }
}

const runAllNow = async () => {
  try {
    await ElMessageBox.confirm(t('rot.runNowConfirm'), t('rot.tab'), { type: 'warning' })
  } catch { return }
  try {
    const r = await api.post('/system/rotation/run-now')
    if (!r.batch) { ElMessage.info(t('rot.noAccounts')); return }
    rotBatchId.value = r.batch
    rotBatchPollStop()
    rotBatchTimer = setInterval(async () => {
      try {
        const st = await api.get('/credentials/rotate-batch/' + rotBatchId.value)
        rotBatchProg.value = st
        if (!st.running) { rotBatchPollStop(); loadRotAccounts() }
      } catch { rotBatchPollStop() }
    }, 1500)
  } catch { /* interceptor shows the error */ }
}

// ---- View account passwords (admin only, audited) ----
const revealVisible = ref(false)
const revealAcctData = ref({ username: '', password: '' })
const revealAcct = async row => {
  try {
    const r = await api.post(`/credentials/${row.id}/reveal`)
    revealAcctData.value = { username: row.username, password: r.password }
    revealVisible.value = true
  } catch { /* interceptor shows the error */ }
}

// ---- AI assistant: config + connectivity test + test chat + role settings (backend ai_roles, add/remove supported) ----
const aiTabSection = ref('conn')
const aiRateLimitNum = computed({
  get: () => Number(form.ai_chat_rate_limit) || 0,
  set: v => { form.ai_chat_rate_limit = String(v) }
})
const aiTesting = ref(false)
const aiSending = ref(false)
const aiTimeoutNum = ref(120)
const aiPrompt = ref('')
const aiReply = ref('')
const aiRoles = ref([])
const aiRole = ref('')
const aiRolesSaving = ref(false)

const currentAiRole = computed(() => aiRoles.value.find(r => r.key === aiRole.value))
const currentAiRoleName = computed({
  get: () => currentAiRole.value?.name || '',
  set: v => { if (currentAiRole.value) currentAiRole.value.name = v }
})
const aiSystemPromptEdit = computed({
  get: () => currentAiRole.value?.prompt || '',
  set: v => { if (currentAiRole.value) currentAiRole.value.prompt = v }
})

// ---- AI alert diagnostics + cleanup catalog: moved to Monitoring Center (Monitor.vue AI Diagnostics tab) ----

const loadAiRoles = async () => {
  try {
    aiRoles.value = await api.get('/ai/roles') || []
    if (!aiRoles.value.some(r => r.key === aiRole.value)) {
      aiRole.value = aiRoles.value.some(r => r.key === 'general') ? 'general' : (aiRoles.value[0]?.key || '')
    }
  } catch { /* ignore */ }
}

const syncRoleEditor = () => { /* the v-model computed is already two-way bound; this is just a placeholder for the select @change */ }

const addAiRole = async () => {
  let value = ''
  try {
    ({ value } = await ElMessageBox.prompt(t('ai.roleNameTip'), t('ai.addRole'), {
      inputPlaceholder: 'e.g. Network Engineer',
      inputPattern: /\S+/,
      inputErrorMessage: t('ai.roleNameTip')
    }))
  } catch { return }
  const key = 'custom_' + Date.now()
  aiRoles.value.push({ key, name: value.trim(), prompt: '' })
  aiRole.value = key
}

const delAiRole = async () => {
  const cur = currentAiRole.value
  if (!cur) return
  try {
    await ElMessageBox.confirm(t('ai.roleDeleteConfirm', { name: cur.name }), t('ai.delRole'), { type: 'warning' })
  } catch { return }
  aiRoles.value = aiRoles.value.filter(r => r.key !== cur.key)
  aiRole.value = aiRoles.value.some(r => r.key === 'general') ? 'general' : (aiRoles.value[0]?.key || '')
}

const saveAiRoles = async () => {
  aiRolesSaving.value = true
  try {
    await api.post('/ai/roles', aiRoles.value)
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { aiRolesSaving.value = false }
}

// ---- Ticketing integrations (ServiceNow / SDP on-premise) ----
const tkForm = reactive({ sn_url: '', sn_user: '', sn_pass: '', sdp_url: '', sdp_token: '', sdp_requester: '' })
const tkSaving = ref(false)
const tkTesting = ref('')

const loadTicketing = async () => {
  try {
    const cfg = await api.get('/system/ticketing')
    tkForm.sn_url = cfg.servicenow?.url || ''
    tkForm.sn_user = cfg.servicenow?.user || ''
    tkForm.sn_pass = cfg.servicenow?.pass || ''
    tkForm.sdp_url = cfg.sdp?.url || ''
    tkForm.sdp_token = cfg.sdp?.token_set ? '******' : ''
    tkForm.sdp_requester = cfg.sdp?.requester || ''
  } catch { /* ignore */ }
}
const saveTicketing = async () => {
  tkSaving.value = true
  try {
    await api.put('/system/ticketing', {
      servicenow: { url: tkForm.sn_url, user: tkForm.sn_user, password: tkForm.sn_pass === '******' ? '' : tkForm.sn_pass },
      sdp: { url: tkForm.sdp_url, token: tkForm.sdp_token === '******' ? '' : tkForm.sdp_token, requester: tkForm.sdp_requester },
    })
    ElMessage.success(t('system.saved'))
    await loadTicketing()
  } catch { /* interceptor shows the error */ } finally { tkSaving.value = false }
}
const testTicketing = async provider => {
  tkTesting.value = provider
  try {
    const body = provider === 'servicenow'
      ? { servicenow: { url: tkForm.sn_url, user: tkForm.sn_user, password: tkForm.sn_pass === '******' ? '' : tkForm.sn_pass } }
      : { sdp: { url: tkForm.sdp_url, token: tkForm.sdp_token === '******' ? '' : tkForm.sdp_token } }
    await api.post('/system/ticketing/test', body)
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { tkTesting.value = '' }
}
const loadAiConfig = async () => {
  try {
    const cfg = await api.get('/system/config')
    form.ai_enabled = cfg.ai_enabled || 'false'
    form.ai_base_url = cfg.ai_base_url || ''
    form.ai_model = cfg.ai_model || ''
    form.ai_timeout_sec = cfg.ai_timeout_sec || '120'
    aiTimeoutNum.value = Number(cfg.ai_timeout_sec) || 120
    form.ai_api_key = cfg.ai_api_key === '******' ? '******' : (cfg.ai_api_key || '')
    form.ai_chat_rate_limit = cfg.ai_chat_rate_limit || '30'
    form.ai_injection_guard = cfg.ai_injection_guard || 'true'
    form.ai_snapshot_filter = cfg.ai_snapshot_filter || 'true'
    aiRateLimitNum.value = Number(form.ai_chat_rate_limit) || 0
  } catch { /* ignore */ }
}

const testAi = async () => {
  await save()
  aiTesting.value = true
  try {
    const r = await api.post('/system/ai/test')
    aiReply.value = r.reply || ('OK · ' + (r.elapsed_ms || 0) + 'ms')
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { aiTesting.value = false }
}

const sendAi = async () => {
  if (!aiPrompt.value.trim()) { ElMessage.warning(t('ai.promptTip')); return }
  aiSending.value = true
  aiReply.value = ''
  try {
    const r = await api.post('/ai/chat', { prompt: aiPrompt.value })
    aiReply.value = r.reply || ''
  } catch { /* interceptor shows the error */ } finally { aiSending.value = false }
}

onMounted(async () => {

  // deep link: /system?tab=xxx opens the requested tab directly
  const qtab = route.query.tab
  if (qtab && ['general', 'rotation', 'ai', 'ext', 'ldap', 'smtp', 'paired', 'roles', 'apikeys'].includes(String(qtab))) {
    activeTab.value = String(qtab)
  }
  try {
    api.get('/system/info').then(info => { appVersion.value = info.version || '1.0' }).catch(() => {})
  } catch { /* ignore */ }
  loadRotAccounts()
  loadAiConfig()
  loadAiRoles()
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
    await loadApiKeys()
  } finally { loading.value = false }
})

const save = async () => {
  saving.value = true
  try {
    const payload = { ...form, ldap_port: String(ldapPort.value), smtp_port: String(smtpPort.value),
      smtp_ssl: smtpSsl.value ? 'true' : 'false',
      rotation_length: String(rotationLength.value), rotation_complexity: rotationComplexity.value,
      rotation_days: String(rotationDays.value),
      ai_enabled: String(form.ai_enabled),
      ai_base_url: form.ai_base_url,
      ai_api_key: form.ai_api_key === '******' ? '' : form.ai_api_key,
      ai_model: form.ai_model,
      ai_timeout_sec: String(aiTimeoutNum.value),
      ai_system_prompt: aiSystemPromptEdit.value,
      ai_chat_rate_limit: String(aiRateLimitNum.value),
      ai_injection_guard: String(form.ai_injection_guard),
      ai_snapshot_filter: String(form.ai_snapshot_filter),
      oo_enabled: String(form.oo_enabled),
      oo_url: form.oo_url,
      oo_org: form.oo_org,
      oo_token: form.oo_token === '******' ? '' : form.oo_token }
    await api.put('/system/config', payload)
    localStorage.setItem('system_name', form.system_name)
    document.title = form.system_name
    // notify MainLayout so the Log Search menu appears/disappears immediately
    window.dispatchEvent(new CustomEvent('oo-state-changed', { detail: form.oo_enabled === 'true' }))
    ElMessage.success(t('system.saved'))
  } finally { saving.value = false }
}

// OpenObserve connection test: save first, then round-trip a tiny search
const ooTesting = ref(false)
const testOO = async () => {
  await save()
  ooTesting.value = true
  try {
    await api.post('/system/oo/test')
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { ooTesting.value = false }
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

<style scoped>
/* AI assistant tab: left nav (same style as the template settings list in Monitor) */
.ext-side-nav { width: 180px; flex-shrink: 0; }
.ext-side-nav-title {
  font-weight: 600; font-size: 14px; color: var(--el-text-color-primary);
  padding: 0 14px; margin-bottom: 10px;
}
.ext-nav-item {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; margin-bottom: 4px;
  font-size: 14px; border-radius: 4px; cursor: pointer;
  color: var(--el-text-color-regular); border: 1px solid transparent;
  transition: background .15s, color .15s;
}
.ext-nav-item:hover { background: var(--el-fill-color-light); }
.ext-nav-item.active {
  background: var(--el-color-primary-light-9); color: var(--el-color-primary);
  font-weight: 600; border-color: var(--el-color-primary-light-7);
}
.ext-sec-head { font-size: 14px; }
.ai-side-nav { width: 180px; flex-shrink: 0; }
.ai-side-nav-title {
  font-weight: 600; font-size: 14px; color: var(--el-text-color-primary);
  padding: 0 14px; margin-bottom: 10px;
}
.ai-nav-item {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; margin-bottom: 4px;
  font-size: 14px; border-radius: 4px; cursor: pointer;
  color: var(--el-text-color-regular); border: 1px solid transparent;
  transition: background .15s, color .15s;
}
.ai-nav-item:hover { background: var(--el-fill-color-light); }
.ai-nav-item.active {
  background: var(--el-color-primary-light-9); color: var(--el-color-primary);
  font-weight: 600; border-color: var(--el-color-primary-light-7);
}
</style>
