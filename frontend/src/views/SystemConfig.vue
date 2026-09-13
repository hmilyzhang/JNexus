<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
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

    <!-- AI 助手：左侧导航 + 右侧内容 -->
    <el-tab-pane :label="$t('ai.tab')" name="ai">
    <el-card>
      <div style="display:flex; gap:0; min-height:420px">
        <!-- 左侧导航 -->
        <div class="ai-side-nav">
          <div style="font-weight:600; font-size:14px; margin-bottom:12px; color:var(--el-text-color-primary)">{{ $t('ai.tab') }}</div>
          <div class="ai-nav-item" :class="{ active: aiTabSection === 'conn' }" @click="aiTabSection = 'conn'">
            <el-icon><Connection /></el-icon> {{ $t('ai.navConn') }}
          </div>
          <div class="ai-nav-item" :class="{ active: aiTabSection === 'role' }" @click="aiTabSection = 'role'">
            <el-icon><User /></el-icon> {{ $t('ai.navRole') }}
          </div>
        </div>
        <!-- 右侧内容 -->
        <div style="flex:1; min-width:0; padding-left:20px; border-left:1px solid var(--el-border-color-lighter)">
          <!-- 连接配置 -->
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
          <!-- 角色设置 -->
          <template v-if="aiTabSection === 'role'">
            <div style="font-weight:600; font-size:15px; margin-bottom:16px">{{ $t('ai.roleTitle') }}</div>
            <el-form label-width="130px" style="max-width:520px">
              <el-form-item :label="$t('ai.roleLabel')">
                <el-select v-model="aiRole" style="width:100%" @change="onRoleChange">
                  <el-option v-for="r in aiRolePresets" :key="r.key" :value="r.key" :label="$t('ai.' + r.labelKey)" />
                </el-select>
              </el-form-item>
              <el-form-item :label="$t('ai.systemPrompt')">
                <el-input v-model="aiSystemPromptEdit" type="textarea" :rows="6" class="mono" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
              </el-form-item>
            </el-form>
          </template>
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

    <el-tab-pane :label="$t('system.apiKeys')" name="apikeys">
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('system.apiKeysTitle') }}</span>
          <el-button size="small" type="primary" @click="openApiKeyDlg">{{ $t('system.apiKeyCreate') }}</el-button>
          <el-button size="small" @click="openDoc">{{ $t('system.apiKeyDoc') }}</el-button>
        </div>
      </template>
      <el-table :data="apiKeys" size="small" border>
        <el-table-column prop="name" :label="$t('system.apiKeyName')" min-width="140" />
        <el-table-column label="Key" width="150">
          <template #default="{ row }"><span class="mono">aok_{{ row.key_id }}...</span></template>
        </el-table-column>
        <el-table-column prop="owner" :label="$t('system.apiKeyOwner')" width="110" />
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
    </el-card>
    </el-tab-pane>
    </el-tabs>

    <!-- API 密钥创建 -->
    <el-dialog v-model="apiKeyDlgVisible" :title="$t('system.apiKeyCreate')" width="480px">
      <el-form label-width="120px">
        <el-form-item :label="$t('system.apiKeyName')"><el-input v-model="apiKeyForm.name" /></el-form-item>
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

    <!-- 密钥一次性展示 -->
    <el-dialog v-model="keyShownVisible" :title="$t('system.apiKeyCreatedTitle')" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :title="$t('system.apiKeyOnceTip')" :closable="false" style="margin-bottom:12px" />
      <div class="mono" style="background:#f5f7fa; padding:10px; border-radius:4px; word-break:break-all; font-size:13px">{{ createdKey }}</div>
      <div style="margin-top:10px; text-align:right">
        <el-button size="small" @click="copyKey">{{ $t('system.apiKeyCopy') }}</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="keyShownVisible = false">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

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

    <!-- 新建自定义角色 -->
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

    <!-- 模块权限矩阵 -->
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
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import Paired from './Paired.vue'

const { t } = i18n.global
const activeTab = ref('general')
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
  ai_enabled: 'false', ai_base_url: '', ai_api_key: '', ai_model: '', ai_timeout_sec: '120'
})
const rotationLength = ref(20)
const rotationComplexity = ref('high')
const rotationDays = ref(90)
const smtpPort = ref(25)
const smtpSsl = ref(false)
const smtpTestTo = ref('')
const smtpTesting = ref(false)
const ldapPort = ref(389)

// ---- API 密钥 ----
const apiKeys = ref([])
const users = ref([])
const apiKeyDlgVisible = ref(false)
const keyShownVisible = ref(false)
const createdKey = ref('')
const apiKeyForm = reactive({ name: '', owner_user_id: null, expires_at: '', ip_allowlist: '' })

const fmtApiTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const loadApiKeys = async () => {
  apiKeys.value = await api.get('/api_keys')
  users.value = await api.get('/users')
}
const openApiKeyDlg = async () => {
  Object.assign(apiKeyForm, { name: '', owner_user_id: null, expires_at: '', ip_allowlist: '' })
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
// 角色名/描述按语言包本地化显示（system.role<Cap> / layout.role<Cap>），保存的是显示值
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

// ---- 自定义角色 ----
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
  } catch { /* 拦截器已提示 */ }
}
const delRole = async row => {
  try {
    await api.delete(`/system/roles/${row.role}`)
    ElMessage.success(t('common.success'))
    await loadRoles()
  } catch { /* 拦截器已提示 */ }
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

// ---- 查看账号密码（管理员，带审计） ----
const revealVisible = ref(false)
const revealAcctData = ref({ username: '', password: '' })
const revealAcct = async row => {
  try {
    const r = await api.post(`/credentials/${row.id}/reveal`)
    revealAcctData.value = { username: row.username, password: r.password }
    revealVisible.value = true
  } catch { /* interceptor shows the error */ }
}

// ---- AI 助手：配置 + 连通性测试 + 测试对话 + 角色设置 ----
const aiTabSection = ref('conn')
const aiTesting = ref(false)
const aiSending = ref(false)
const aiTimeoutNum = ref(120)
const aiPrompt = ref('')
const aiReply = ref('')
const aiRole = ref('general')
const aiSystemPromptEdit = ref('')
const aiRolePresets = [
  { key: 'sre', labelKey: 'roleSre', prompt: '你是一名资深 SRE（站点可靠性工程师），擅长故障排查、根因分析、容量规划和 SLO 制定。回答注重可操作性，给出具体命令和排查步骤。' },
  { key: 'dba', labelKey: 'roleDba', prompt: '你是一名资深数据库管理员（DBA），擅长 MySQL/PostgreSQL/Redis 的运维、SQL 优化、备份恢复、主从复制和慢查询分析。回答注重安全性，涉及破坏性操作时提醒确认。' },
  { key: 'devops', labelKey: 'roleDevops', prompt: '你是一名 DevOps 工程师，擅长 CI/CD、容器化、基础设施即代码和自动化运维。回答注重效率和最佳实践。' },
  { key: 'security', labelKey: 'roleSecurity', prompt: '你是一名安全分析师，擅长漏洞评估、入侵检测、加固建议和合规审计。回答注重风险等级和修复优先级。' },
  { key: 'general', labelKey: 'roleGeneral', prompt: '你是一名通用运维助手，能回答各类技术问题和运维场景咨询。' },
]

const onRoleChange = key => {
  const preset = aiRolePresets.find(r => r.key === key)
  if (preset) aiSystemPromptEdit.value = preset.prompt
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
    aiSystemPromptEdit.value = cfg.ai_system_prompt || aiRolePresets[aiRolePresets.length - 1].prompt
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
  try {
    api.get('/system/info').then(info => { appVersion.value = info.version || '1.0' }).catch(() => {})
  } catch { /* ignore */ }
  loadRotAccounts()
  loadAiConfig()
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
      ai_system_prompt: aiSystemPromptEdit.value }
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
