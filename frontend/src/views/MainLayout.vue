<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <el-container style="height: 100vh">
    <el-aside :width="collapsed ? '64px' : '176px'" style="background:#1d2935; display:flex; flex-direction:column; overflow-x:hidden">
      <div class="logo" v-if="!collapsed">
        <img :src="logoMark" alt="logo" style="width:26px; height:26px; flex-shrink:0" />
        <span>{{ systemName }}</span>
      </div>
      <div class="logo logo-mini" v-else :title="systemName">
        <img :src="logoMark" alt="logo" style="width:26px; height:26px" />
      </div>
      <el-menu ref="menuRef" :default-active="$route.path" router :collapse="collapsed" :collapse-transition="false"
               popper-class="sidebar-popper" background-color="#1d2935" text-color="#a7b1c2"
               active-text-color="#ffffff" style="border-right:none; flex:1; overflow-y:auto" :ellipsis="false"
               unique-opened
               @select="onMenuSelect">
        <template v-for="m in menus" :key="m.key">
          <el-sub-menu v-if="m.children" :index="m.key">
            <template #title>
              <el-icon><component :is="m.icon" /></el-icon>
              <span>{{ $t(m.title) }}</span>
            </template>
            <el-menu-item v-for="c in m.children" :key="c.key" :index="c.path">
              <el-icon><component :is="c.icon" /></el-icon>
              <span>{{ $t(c.title) }}</span>
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item v-else :index="m.path">
            <el-icon><component :is="m.icon" /></el-icon>
            <span>{{ $t(m.title) }}</span>
          </el-menu-item>
        </template>
      </el-menu>
      <div class="byline" v-if="!collapsed">{{ appVersion ? $t('layout.byline', { version: appVersion }) : 'By JJ Zhang' }}</div>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div style="display:flex; align-items:center; gap:12px">
          <el-button text @click="toggleCollapse" style="padding:6px">
            <el-icon :size="18"><component :is="collapsed ? 'Expand' : 'Fold'" /></el-icon>
          </el-button>
          <div class="title">{{ $route.meta.title ? $t($route.meta.title) : 'JNexus' }}</div>
        </div>
        <div class="header-quote" :title="quote.text" @click="pickQuote">
          <span v-if="quote.icon" class="q-icon">{{ quote.icon }}</span>{{ quote.text }}
        </div>
        <div style="display:flex; align-items:center; gap:16px">
          <el-button class="theme-toggle" text @click="toggleTheme" style="padding:6px" :title="isDark ? $t('layout.lightMode') : $t('layout.darkMode')">
            <el-icon :size="18"><component :is="isDark ? 'Sunny' : 'Moon'" /></el-icon>
          </el-button>
          <el-dropdown @command="onLocale">
            <span class="user-info"><el-icon><Clock /></el-icon>{{ localeLabel }}</span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="l in locales" :key="l.value" :command="l.value">{{ l.label }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-dropdown @command="onCmd">
            <span class="user-info">
              <el-icon><User /></el-icon>
              {{ store.user?.display_name || store.user?.username }}（{{ roleLabel }}）
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="password">{{ $t('layout.changePwd') }}</el-dropdown-item>
                <el-dropdown-item command="profile">{{ $t('layout.profile') }}</el-dropdown-item>
                <el-dropdown-item command="mfa">{{ $t('layout.mfaSecurity') }}</el-dropdown-item>
                <el-dropdown-item command="logout" divided>{{ $t('layout.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <a class="gh-link" href="https://github.com/hmilyzhang/JNexus" target="_blank" rel="noopener noreferrer"
             title="GitHub" aria-label="GitHub">
            <svg viewBox="0 0 16 16" width="19" height="19" fill="currentColor" aria-hidden="true">
              <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"/>
            </svg>
          </a>
        </div>
      </el-header>
      <el-main style="padding:16px; background:var(--app-bg)">
        <router-view />
      </el-main>
    </el-container>
  </el-container>

  <el-dialog v-model="pwdVisible" :title="$t('layout.changePwd')" width="400px">
    <el-form label-width="90px">
      <el-form-item :label="$t('layout.oldPwd')"><el-input v-model="pwdForm.old_password" type="password" show-password /></el-form-item>
      <el-form-item :label="$t('layout.newPwd')"><el-input v-model="pwdForm.new_password" type="password" show-password /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="pwdVisible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="doChangePwd">{{ $t('common.confirm') }}</el-button>
    </template>
  </el-dialog>

  <!-- MFA (TOTP two-step verification) self-service -->
  <el-dialog v-model="mfaVisible" :title="$t('layout.mfaSecurity')" width="440px" @open="loadMfa">
    <div v-if="mfaLoading" v-loading style="height:100px"></div>
    <template v-else>
      <template v-if="!mfaEnabled">
        <template v-if="!mfaSetup">
          <div style="color:#909399; font-size:13px; line-height:1.7; margin-bottom:12px">{{ $t('mfa.disabledTip') }}</div>
          <el-button type="primary" style="width:100%" @click="mfaSetupStart">{{ $t('mfa.startSetup') }}</el-button>
        </template>
        <template v-else>
          <div style="text-align:center"><img v-if="mfaSetup.qr" :src="mfaSetup.qr" style="width:200px; height:200px" alt="QR" /></div>
          <div style="color:#909399; font-size:12px; text-align:center; margin:6px 0 4px">{{ $t('mfa.scanTip') }}</div>
          <div class="mono" style="font-size:12px; text-align:center; margin-bottom:12px; word-break:break-all">{{ $t('mfa.secretKey') }}：{{ mfaSetup.secret }}</div>
          <el-input v-model="mfaCode" maxlength="6" class="mono" size="large"
                    :placeholder="$t('mfa.codePlaceholder')" style="text-align:center; letter-spacing:8px; margin-bottom:10px"
                    @keyup.enter="mfaEnableNow" />
          <el-button type="primary" style="width:100%" @click="mfaEnableNow">{{ $t('mfa.enable') }}</el-button>
        </template>
      </template>
      <template v-else>
        <el-result icon="success" :title="$t('mfa.enabledTitle')" style="padding:6px 0 14px" />
        <el-input v-model="mfaCode" maxlength="6" class="mono" size="large"
                  :placeholder="$t('mfa.disableTip')" style="text-align:center; letter-spacing:8px; margin-bottom:10px"
                  @keyup.enter="mfaDisableNow" />
        <el-button type="danger" style="width:100%" @click="mfaDisableNow">{{ $t('mfa.disable') }}</el-button>
      </template>
    </template>
  </el-dialog>

  <!-- Site-wide floating AI assistant -->
  <transition name="ai-fade">
    <div v-if="aiOpen && aiChatAllowed" class="ai-panel" :style="{ width: aiSize.w + 'px', height: aiSize.h + 'px' }">
      <!-- resize handles: drag left edge / top edge to enlarge -->
      <div class="ai-resize-x" @mousedown="startResizeAI($event, 'x')"></div>
      <div class="ai-resize-y" @mousedown="startResizeAI($event, 'y')"></div>
      <div class="ai-head">
        <span style="font-weight:700">{{ $t('ai.assistantTitle') }}</span>
        <el-select v-model="aiRole" size="small" style="flex:1; margin:0 6px"
                   :placeholder="$t('ai.roleLabel')" :title="$t('ai.roleLabel')" @change="onChatRoleChange">
          <el-option v-for="r in aiRoles" :key="r.key" :value="r.key" :label="r.name" />
        </el-select>
        <el-button text size="small" style="color:#909399" @click="aiOpen = false">
          <el-icon><Close /></el-icon>
        </el-button>
      </div>
      <div class="ai-messages" ref="aiMsgBox">
        <div v-for="(msg, i) in aiMessages" :key="i" class="ai-msg" :class="msg.role">
          <div class="ai-bubble">
            <span v-if="msg.role === 'user'" style="white-space:pre-wrap">{{ msg.text }}</span>
            <!-- bot replies are markdown-rendered and sanitized -->
            <div v-else class="ai-md" v-html="renderMd(msg.text)"></div>
          </div>
        </div>
        <div v-if="aiThinking" class="ai-msg ai-user-side" style="opacity:.6"><div class="ai-bubble">…</div></div>
      </div>
      <div class="ai-input-row">
        <el-input v-model="aiInput" :placeholder="$t('ai.assistantPlaceholder')" size="default"
                  @keyup.enter="sendToAI" :disabled="aiBusy" clearable />
        <el-button type="primary" :loading="aiBusy" @click="sendToAI">{{ $t('ai.send') }}</el-button>
      </div>
    </div>
  </transition>
  <transition name="ai-fade">
    <div v-if="!aiOpen && aiChatAllowed" class="ai-fab" @click="aiOpen = true" :title="$t('ai.assistantTitle')">
      <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
        <circle cx="9" cy="10" r="0.5" fill="currentColor"/>
        <circle cx="13" cy="10" r="0.5" fill="currentColor"/>
        <circle cx="17" cy="10" r="0.5" fill="currentColor"/>
      </svg>
    </div>
  </transition>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n, { locales, setLocale } from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'
import logoMark from '../assets/logo-mark.png'
import { jokes, soulSoups } from '../data/quotations'

const store = useUserStore()
const router = useRouter()
const { t } = i18n.global

const systemName = ref(localStorage.getItem('system_name') || 'JNexus')
const appVersion = ref('')
api.get('/system/info').then(info => {
  systemName.value = info.system_name || 'JNexus'
  appVersion.value = info.version || ''
  localStorage.setItem('system_name', info.system_name || 'JNexus')
  document.title = info.system_name || 'JNexus'
}).catch(() => {})

const roleLabel = computed(() => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer'),
  auditor: t('layout.roleAuditor')
})[store.role] || store.role)

// Menus are driven by role settings (admins always get everything)
const menuItems = [
  { key: 'dashboard', path: '/dashboard', title: 'menu.dashboard', icon: 'Odometer' },
  { key: 'hosts', path: '/hosts', title: 'menu.assets', icon: 'Collection' },
    { key: 'databases', path: '/databases', title: 'menu.databases', icon: 'Coin' },
  { key: 'osaccounts', path: '/os-accounts', title: 'menu.osaccounts', icon: 'Avatar' },
  { key: 'jobs', title: 'menu.jobs', icon: 'Operation', children: [
    { key: 'exec', path: '/exec', title: 'menu.exec', icon: 'Promotion' },
    { key: 'files', path: '/files', title: 'menu.files', icon: 'FolderOpened' },
    { key: 'scripts', path: '/scripts', title: 'menu.scripts', icon: 'Document' },
    { key: 'logtail', path: '/logtail', title: 'menu.logtail', icon: 'View' },
    { key: 'webapps', path: '/webapps', title: 'menu.webapps', icon: 'Link' }
  ] },
  { key: 'tasks', path: '/tasks', title: 'menu.tasks', icon: 'List' },
  { key: 'cron', path: '/crons', title: 'menu.cron', icon: 'Timer' },
  { key: 'reports', path: '/reports', title: 'menu.reports', icon: 'DataAnalysis' },
  { key: 'monitor', path: '/monitor', title: 'menu.monitor', icon: 'Cpu' },
  { key: 'observe', path: '/observe', title: 'menu.observe', icon: 'DataLine' },
  { key: 'k8s', path: '/k8s', title: 'k8s.title', icon: 'Grid' },
  { key: 'apps', path: '/apps', title: 'menu.apps', icon: 'Box' },
  { key: 'releases', path: '/releases', title: 'menu.releases', icon: 'UploadFilled' },
  { key: 'sysadmin', title: 'menu.sysadmin', icon: 'Setting', children: [
    { key: 'users', path: '/users', title: 'menu.users', icon: 'User' },
    { key: 'danger', path: '/danger', title: 'menu.danger', icon: 'Warning' },
    { key: 'audit', path: '/audit', title: 'menu.audit', icon: 'Notebook' },
    { key: 'system', path: '/system', title: 'menu.system', icon: 'Setting' },
    { key: 'oo-admin', path: '/oo-admin', title: 'menu.observeAdmin', icon: 'DataLine' }
  ] }
]
const roleSettings = ref({})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})

// Log Search menu visibility follows the OpenObserve enabled flag (system settings);
// SystemConfig dispatches 'oo-state-changed' after saving so the menu refreshes live.
const ooEnabled = ref(true)
const loadOOState = () => api.get('/observe/state').then(r => { ooEnabled.value = !!r.enabled }).catch(() => {})
loadOOState()
const ooStateListener = e => { ooEnabled.value = !!e.detail }
window.addEventListener('oo-state-changed', ooStateListener)

const menuRef = ref(null)
const collapsed = ref(localStorage.getItem('sidebar_collapsed') === '1')
const toggleCollapse = () => {
  collapsed.value = !collapsed.value
  localStorage.setItem('sidebar_collapsed', collapsed.value ? '1' : '0')
}
const jobsChildren = ['/exec', '/files', '/scripts']
// Collapse the "Jobs" dropdown when a non-child menu item is selected
const onMenuSelect = index => {
  if (!jobsChildren.includes(index)) {
    try { menuRef.value?.close('jobs') } catch { /* ignore */ }
  }
}

// AI assistant entry: only for roles granted the ai:chat capability
// (default: admin/ops/publisher/k8s; viewer/auditor excluded)
const aiChatAllowed = computed(() => {
  if (store.isAdmin) return true
  const conf = roleSettings.value[store.role]
  const perms = conf?.perms || {}
  return (perms.ai || []).includes('chat')
})

const menus = computed(() => {
  const visible = list => list.filter(m => m.key !== 'observe' || ooEnabled.value)
  if (store.isAdmin) return visible(menuItems)
  const conf = roleSettings.value[store.role]
  const allowed = new Set(conf?.menus || [])
  const out = []
  for (const m of menuItems) {
    if (m.children) {
      const kids = m.children.filter(c => (allowed.has(c.key) ||
        (c.key === 'logtail' && allowed.has('exec'))) && // log access follows exec permission
        (c.key !== 'observe' || ooEnabled.value))
      if (kids.length) out.push({ ...m, children: kids })
    } else if (allowed.has(m.key) && (m.key !== 'observe' || ooEnabled.value)) {
      out.push(m)
    }
  }
  return out
})

// ---- Light/dark theme (bootstrap script lives in index.html to avoid first-paint flicker) ----
const isDark = ref(document.documentElement.classList.contains('dark'))
const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('jtheme', isDark.value ? 'dark' : 'light')
}

const localeLabel = computed(() => (locales.find(l => l.value === i18n.global.locale.value) || {}).label || '中文')
const onLocale = v => setLocale(v)

// ---- Random header quote: jokes / soul soups (auto-rotated every 10 minutes) ----
const QUOTE_POOLS = [
  { icon: '', pool: jokes },
  { icon: '', pool: soulSoups },
]
const quote = ref({ icon: '😄', text: '' })
let lastQuote = ''
const pickQuote = () => {
  const g = QUOTE_POOLS[Math.random() < 0.5 ? 0 : 1]
  let text = g.pool[Math.floor(Math.random() * g.pool.length)]
  if (text === lastQuote) text = g.pool[(g.pool.indexOf(text) + 1) % g.pool.length]
  lastQuote = text
  quote.value = { icon: g.icon, text }
}
pickQuote()
const quoteTimer = setInterval(pickQuote, 10 * 60 * 1000)
onBeforeUnmount(() => {
  clearInterval(quoteTimer)
  window.removeEventListener('oo-state-changed', ooStateListener)
})

// ---- Site-wide floating AI chat panel ----
const aiOpen = ref(false)
const aiBusy = ref(false)
const aiInput = ref('')

// ---- AI reply markdown rendering (sanitized) ----
import { marked } from 'marked'
import DOMPurify from 'dompurify'
marked.setOptions({ breaks: true, gfm: true })
// AI replies may contain links (possibly injected): harden them
DOMPurify.addHook('afterSanitizeAttributes', node => {
  if (node.tagName === 'A') {
    node.setAttribute('rel', 'noopener noreferrer nofollow')
    node.setAttribute('target', '_blank')
  }
})
const renderMd = t => DOMPurify.sanitize(marked.parse(t || ''))

// ---- AI panel resizable (drag left edge / top edge; persisted) ----
const AI_SIZE_KEY = 'jai_size'
let savedSize = {}
try { savedSize = JSON.parse(localStorage.getItem(AI_SIZE_KEY) || '{}') } catch { /* ignore */ }
const aiSize = reactive({
  w: Math.min(Math.max(savedSize.w || 380, 340), window.innerWidth - 40),
  h: Math.min(Math.max(savedSize.h || 480, 380), window.innerHeight - 100),
})
const startResizeAI = (e, dir) => {
  e.preventDefault()
  const startX = e.clientX, startY = e.clientY
  const sw = aiSize.w, sh = aiSize.h
  const maxW = window.innerWidth - 40, maxH = window.innerHeight - 100
  const move = ev => {
    if (dir === 'x') aiSize.w = Math.min(maxW, Math.max(340, sw + (startX - ev.clientX)))
    else aiSize.h = Math.min(maxH, Math.max(380, sh + (startY - ev.clientY)))
  }
  const up = () => {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
    localStorage.setItem(AI_SIZE_KEY, JSON.stringify({ w: aiSize.w, h: aiSize.h }))
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}
const aiMessages = ref([{ role: 'bot', text: t('ai.greeting') }])
const aiMsgBox = ref(null)
// AI roles: fetch list after login; send selected role with each chat; persist choice locally
const aiRoles = ref([])
const aiRole = ref(localStorage.getItem('ai_role') || 'general')
const loadAiRoles = async () => {
  try {
    aiRoles.value = await api.get('/ai/roles') || []
    if (!aiRoles.value.some(r => r.key === aiRole.value)) {
      aiRole.value = aiRoles.value.some(r => r.key === 'general') ? 'general' : (aiRoles.value[0]?.key || '')
    }
  } catch { aiRoles.value = [] }
}
const onChatRoleChange = () => localStorage.setItem('ai_role', aiRole.value)
loadAiRoles()

const sendToAI = async () => {
  const text = aiInput.value.trim()
  if (!text || aiBusy.value) return
  // Automatically attach the current route path and selected role as context
  aiMessages.value.push({ role: 'user', text })
  aiBusy.value = true
  try {
    const r = await api.post('/ai/chat', { prompt: text, page: router.currentRoute.value.path, role: aiRole.value })
    aiMessages.value.push({ role: 'bot', text: r.reply || '…' })
  } catch {
    aiMessages.value.push({ role: 'bot', text: t('ai.error') })
  } finally {
    aiInput.value = ''
    aiBusy.value = false
    setTimeout(() => { const b = aiMsgBox.value; if (b) b.scrollTop = b.scrollHeight }, 50)
  }
}

const onCmd = cmd => {
  if (cmd === 'logout') {
    store.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.value = { old_password: '', new_password: '' }
    pwdVisible.value = true
  } else if (cmd === 'mfa') {
    mfaVisible.value = true
  } else if (cmd === 'profile') {
    router.push('/profile')
  }
}

const doChangePwd = async () => {
  await api.post('/change_password', pwdForm.value)
  ElMessage.success(t('common.success'))
  pwdVisible.value = false
}

// ---- MFA (TOTP two-step verification) ----
const mfaVisible = ref(false)
const mfaLoading = ref(false)
const mfaEnabled = ref(false)
const mfaSetup = ref(null)
const mfaCode = ref('')
const loadMfa = async () => {
  mfaLoading.value = true
  mfaSetup.value = null
  mfaCode.value = ''
  try {
    const s = await api.get('/mfa/status')
    mfaEnabled.value = !!s.enabled
  } finally { mfaLoading.value = false }
}
const mfaSetupStart = async () => { mfaSetup.value = await api.post('/mfa/setup') }
const mfaEnableNow = async () => {
  if (mfaCode.value.length !== 6) { ElMessage.warning(t('mfa.codePlaceholder')); return }
  try {
    await api.post('/mfa/enable', { code: mfaCode.value })
    ElMessage.success(t('mfa.enableOk'))
    loadMfa()
  } catch { /* error toast shown by the interceptor */ }
}
const mfaDisableNow = async () => {
  try {
    await api.post('/mfa/disable', { code: mfaCode.value })
    ElMessage.success(t('mfa.disableOk'))
    loadMfa()
  } catch { /* error toast shown by the interceptor */ }
}
</script>

<style>
/* Submenu popper matches the sidebar color in collapsed mode */
.sidebar-popper.el-menu--vertical {
  background-color: #1d2935;
  border: none;
}
.sidebar-popper .el-menu {
  background-color: #1d2935;
}
.sidebar-popper .el-menu-item {
  color: #a7b1c2;
  background-color: #1d2935;
}
.sidebar-popper .el-menu-item:hover {
  background-color: #263445;
  color: #ffffff;
}
</style>
<style scoped>
.logo-mini { text-align: center; padding-left: 0; padding-right: 0; padding-top: 10px; padding-bottom: 6px; font-size: 20px; }
/* Compact menu items: longer menus fit common viewport heights without a scrollbar */
aside :deep(.el-menu-item),
aside :deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  padding-left: 14px !important;
  padding-right: 10px !important;
  /* near-instant hover feedback: EP's default 0.3s color transition reads as lag on dark bg */
  transition: background-color .06s linear, color .06s linear !important;
}
/* Long labels never wrap; ellipsize overflow to prevent horizontal scrollbars */
aside :deep(.el-menu-item span),
aside :deep(.el-sub-menu__title span) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
aside :deep(.el-menu) {
  overflow-x: hidden;
}
/* Indent and de-emphasize submenu items to distinguish them from top-level items */
aside :deep(.el-sub-menu .el-menu .el-menu-item) {
  padding-left: 24px !important;
  padding-right: 6px !important;
  font-size: 12px;
  height: 34px;
  line-height: 34px;
  color: #93a1b5;
}
aside :deep(.el-sub-menu .el-menu .el-menu-item .el-icon) {
  font-size: 14px;
  margin-right: 4px;
}
aside :deep(.el-sub-menu .el-menu .el-menu-item.is-active) {
  color: #ffffff;
}
/* Hide the scrollbar visually (scrolling still works, even in very short windows) */
aside::-webkit-scrollbar {
  width: 0;
  display: none;
}
aside {
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.logo {
  color: #fff; font-size: 18px; font-weight: bold;
  padding: 12px 20px 8px; letter-spacing: 1px;
  display: flex; align-items: center; gap: 10px;
}
.byline {
  color: #6b7a8d; font-size: 11px; text-align: center; padding: 8px 0 10px;
  border-top: 1px solid #2a3947; letter-spacing: .5px;
}
.header {
  background: var(--app-surface); display: flex; align-items: center; justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0,21,41,.08);
}
.title { font-size: 16px; font-weight: 600; }
.header-quote {
  flex: 1; min-width: 0; margin: 0 24px; text-align: center;
  font-size: 13px; color: #98a3b3; cursor: pointer;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  user-select: none;
}
.header-quote:hover { color: #66788f; }
.header-quote .q-icon { margin-right: 6px; }
.gh-link { display: inline-flex; align-items: center; color: var(--el-text-color-regular); }
.gh-link:hover { color: var(--el-color-primary); }
.user-info { cursor: pointer; display: flex; align-items: center; gap: 6px; color: #333; font-size: 14px; }

/* ---- Site-wide floating AI assistant ---- */
.ai-fab {
  position: fixed; bottom: 22px; right: 22px; z-index: 2000;
  width: 48px; height: 48px; border-radius: 50%;
  background: linear-gradient(135deg, #67c23a, #4fc3a1); color: #fff;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; border: none; box-shadow: 0 4px 14px rgba(103,194,58,.4);
  transition: transform .2s, box-shadow .2s;
}
.ai-fab:hover { transform: scale(1.1); box-shadow: 0 6px 20px rgba(103,194,58,.5); }
.ai-fab svg { width: 24px; height: 24px; }

.ai-panel {
  position: fixed; bottom: 80px; right: 22px; z-index: 2001;
  display: flex; flex-direction: column;
  background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px; box-shadow: 0 12px 40px rgba(0,0,0,.18);
  overflow: hidden;
}
/* resize handles: left edge (width) + top edge (height) */
.ai-resize-x { position: absolute; left: 0; top: 0; bottom: 0; width: 7px; cursor: ew-resize; z-index: 5; }
.ai-resize-y { position: absolute; top: 0; left: 0; right: 0; height: 7px; cursor: ns-resize; z-index: 5; }
.ai-resize-x:hover, .ai-resize-y:hover { background: rgba(79,140,255,.18); }
.ai-head {
  display: flex; align-items: center; gap: 8px; padding: 12px 14px 8px;
  font-size: 14px; font-weight: 700; color: var(--el-text-color-primary);
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.ai-messages { flex: 1; overflow-y: auto; padding: 12px 14px; }
.ai-msg { margin-bottom: 10px; display: flex; }
.ai-msg.bot { justify-content: flex-start; }
.ai-msg.user { justify-content: flex-end; }
.ai-bubble {
  max-width: 82%; padding: 8px 12px; border-radius: 12px;
  font-size: 13.5px; line-height: 1.55; word-break: break-word;
  background: var(--el-fill-color); color: var(--el-text-color-primary);
}
.ai-msg.user .ai-bubble { background: var(--el-color-primary-light-8); }
/* markdown content inside bot bubbles */
.ai-md :deep(p) { margin: 0 0 8px; }
.ai-md :deep(p:last-child) { margin-bottom: 0; }
.ai-md :deep(h1), .ai-md :deep(h2), .ai-md :deep(h3), .ai-md :deep(h4) {
  margin: 10px 0 6px; font-size: 14px; font-weight: 700; line-height: 1.4;
}
.ai-md :deep(h1:first-child), .ai-md :deep(h2:first-child), .ai-md :deep(h3:first-child) { margin-top: 0; }
.ai-md :deep(ul), .ai-md :deep(ol) { margin: 6px 0 8px; padding-left: 20px; }
.ai-md :deep(li) { margin: 3px 0; }
.ai-md :deep(li > p) { margin: 2px 0; }
.ai-md :deep(code) {
  font-family: ui-monospace, Consolas, Menlo, monospace; font-size: 12px;
  background: rgba(127,127,127,.15); border-radius: 4px; padding: 1px 5px;
}
.ai-md :deep(pre) {
  background: var(--el-fill-color-dark, rgba(127,127,127,.12)); border-radius: 8px;
  padding: 10px 12px; margin: 8px 0; overflow-x: auto;
}
.ai-md :deep(pre code) { background: transparent; padding: 0; font-size: 12px; line-height: 1.6; }
.ai-md :deep(table) { border-collapse: collapse; margin: 8px 0; font-size: 12.5px; }
.ai-md :deep(th), .ai-md :deep(td) { border: 1px solid var(--el-border-color-lighter); padding: 4px 8px; }
.ai-md :deep(blockquote) { margin: 6px 0; padding: 2px 10px; border-left: 3px solid var(--el-color-primary); color: var(--el-text-color-secondary); }
.ai-md :deep(a) { color: var(--el-color-primary); }
.ai-md :deep(hr) { border: none; border-top: 1px solid var(--el-border-color-lighter); margin: 8px 0; }
.ai-input-row { display: flex; gap: 8px; padding: 8px 12px 12px; }

.ai-fade-enter-active, .ai-fade-leave-active { transition: opacity .25s, transform .25s; }
.ai-fade-enter-from, .ai-fade-leave-to { opacity: 0; transform: translateY(12px) scale(.97); }
</style>
