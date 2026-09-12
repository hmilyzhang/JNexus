<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-container style="height: 100vh">
    <el-aside :width="collapsed ? '64px' : '210px'" style="background:#1d2935; display:flex; flex-direction:column; transition: width .2s; overflow-x:hidden; background:#1d2935; display:flex; flex-direction:column; transition: width .2s">
      <div class="logo" v-if="!collapsed">{{ systemName }}</div>
      <div class="logo logo-mini" v-else :title="systemName">{{ systemName.charAt(0).toUpperCase() }}</div>
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
      <div class="byline" v-if="!collapsed">{{ $t('layout.byline', { version: appVersion }) }}</div>
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

  <!-- MFA（TOTP 两步验证）自助管理 -->
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

  <!-- 全站悬浮 AI 助手 -->
  <transition name="ai-fade">
    <div v-if="aiOpen" class="ai-panel">
      <div class="ai-head">
        <span style="font-weight:700">{{ $t('ai.assistantTitle') }}</span>
        <el-button text size="small" style="color:#909399" @click="aiOpen = false">
          <el-icon><Close /></el-icon>
        </el-button>
      </div>
      <div class="ai-messages" ref="aiMsgBox">
        <div v-for="(msg, i) in aiMessages" :key="i" class="ai-msg" :class="msg.role">
          <div class="ai-bubble">{{ msg.text }}</div>
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
    <div v-if="!aiOpen" class="ai-fab" @click="aiOpen = true" :title="$t('ai.assistantTitle')">
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
import { computed, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n, { locales, setLocale } from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'
import { jokes, soulSoups } from '../data/quotations'

const store = useUserStore()
const router = useRouter()
const { t } = i18n.global

const systemName = ref(localStorage.getItem('system_name') || 'JNexus')
const appVersion = ref('1.0')
api.get('/system/info').then(info => {
  systemName.value = info.system_name || 'JNexus'
  appVersion.value = info.version || '1.0'
  localStorage.setItem('system_name', info.system_name || 'JNexus')
  document.title = info.system_name || 'JNexus'
}).catch(() => {})

const roleLabel = computed(() => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer'),
  auditor: t('layout.roleAuditor')
})[store.role] || store.role)

// 菜单由「角色设置」配置驱动（管理员恒为全部）
const menuItems = [
  { key: 'dashboard', path: '/dashboard', title: 'menu.dashboard', icon: 'Odometer' },
  { key: 'hosts', path: '/hosts', title: 'menu.hosts', icon: 'Monitor' },
  { key: 'osaccounts', path: '/os-accounts', title: 'menu.osaccounts', icon: 'Avatar' },
  { key: 'jobs', title: 'menu.jobs', icon: 'Operation', children: [
    { key: 'exec', path: '/exec', title: 'menu.exec', icon: 'Promotion' },
    { key: 'files', path: '/files', title: 'menu.files', icon: 'FolderOpened' },
    { key: 'scripts', path: '/scripts', title: 'menu.scripts', icon: 'Document' },
    { key: 'logtail', path: '/logtail', title: 'menu.logtail', icon: 'View' }
  ] },
  { key: 'tasks', path: '/tasks', title: 'menu.tasks', icon: 'List' },
  { key: 'cron', path: '/crons', title: 'menu.cron', icon: 'Timer' },
  { key: 'reports', path: '/reports', title: 'menu.reports', icon: 'DataAnalysis' },
  { key: 'monitor', path: '/monitor', title: 'menu.monitor', icon: 'Cpu' },
  { key: 'k8s', path: '/k8s', title: 'k8s.title', icon: 'Grid' },
  { key: 'apps', path: '/apps', title: 'menu.apps', icon: 'Box' },
  { key: 'releases', path: '/releases', title: 'menu.releases', icon: 'UploadFilled' },
  { key: 'sysadmin', title: 'menu.sysadmin', icon: 'Setting', children: [
    { key: 'users', path: '/users', title: 'menu.users', icon: 'User' },
    { key: 'danger', path: '/danger', title: 'menu.danger', icon: 'Warning' },
    { key: 'audit', path: '/audit', title: 'menu.audit', icon: 'Notebook' },
    { key: 'system', path: '/system', title: 'menu.system', icon: 'Setting' }
  ] }
]
const roleSettings = ref({})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})

const menuRef = ref(null)
const collapsed = ref(localStorage.getItem('sidebar_collapsed') === '1')
const toggleCollapse = () => {
  collapsed.value = !collapsed.value
  localStorage.setItem('sidebar_collapsed', collapsed.value ? '1' : '0')
}
const jobsChildren = ['/exec', '/files', '/scripts']
// 点击非「任务执行」子项的菜单时，自动收起该下拉
const onMenuSelect = index => {
  if (!jobsChildren.includes(index)) {
    try { menuRef.value?.close('jobs') } catch { /* ignore */ }
  }
}

const menus = computed(() => {
  if (store.isAdmin) return menuItems
  const conf = roleSettings.value[store.role]
  const allowed = new Set(conf?.menus || [])
  const out = []
  for (const m of menuItems) {
    if (m.children) {
      const kids = m.children.filter(c => allowed.has(c.key) ||
        (c.key === 'logtail' && allowed.has('exec'))) // 日志输出跟随执行权限
      if (kids.length) out.push({ ...m, children: kids })
    } else if (allowed.has(m.key)) {
      out.push(m)
    }
  }
  return out
})

// ---- 深浅色主题（引导脚本在 index.html，避免首屏闪烁）----
const isDark = ref(document.documentElement.classList.contains('dark'))
const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('jtheme', isDark.value ? 'dark' : 'light')
}

const localeLabel = computed(() => (locales.find(l => l.value === i18n.global.locale.value) || {}).label || '中文')
const onLocale = v => setLocale(v)

// ---- 顶栏随机一句话：笑话 / 心灵鸡汤（10 分钟自动轮换，10 分钟自动轮换）----
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
onBeforeUnmount(() => clearInterval(quoteTimer))

// ---- 全站悬浮 AI 对话框 ----
const aiOpen = ref(false)
const aiBusy = ref(false)
const aiInput = ref('')
const aiMessages = ref([])
const aiMsgBox = ref(null)

const sendToAI = async () => {
  const text = aiInput.value.trim()
  if (!text || aiBusy.value) return
  aiMessages.value.push({ role: 'user', text })
  aiBusy.value = true
  try {
    const r = await api.post('/ai/chat', { prompt: text })
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

// ---- MFA（TOTP 两步验证）----
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
  } catch { /* 错误提示由拦截器展示 */ }
}
const mfaDisableNow = async () => {
  try {
    await api.post('/mfa/disable', { code: mfaCode.value })
    ElMessage.success(t('mfa.disableOk'))
    loadMfa()
  } catch { /* 错误提示由拦截器展示 */ }
}
</script>

<style>
/* 折叠模式下拉出的子菜单与侧栏同色 */
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
/* 紧凑菜单项：更长菜单在常规视口高度内不出现滚动条 */
aside :deep(.el-menu-item),
aside :deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  padding-left: 14px !important;
  padding-right: 10px !important;
}
/* 英文长标签不换行、超出省略，杜绝横向滚动条 */
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
/* 子菜单项缩进 + 弱化样式：区分子菜单与主菜单 */
aside :deep(.el-sub-menu .el-menu .el-menu-item) {
  padding-left: 42px !important;
  font-size: 13px;
  height: 34px;
  line-height: 34px;
  color: #93a1b5;
}
aside :deep(.el-sub-menu .el-menu .el-menu-item.is-active) {
  color: #ffffff;
}
/* 滚动条视觉隐藏（保留滚动能力，极矮窗口仍可滚动到底） */
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
.user-info { cursor: pointer; display: flex; align-items: center; gap: 6px; color: #333; font-size: 14px; }

/* ---- 全站悬浮 AI 助手 ---- */
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
  width: 380px; height: 480px; display: flex; flex-direction: column;
  background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px; box-shadow: 0 12px 40px rgba(0,0,0,.18);
  overflow: hidden;
}
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
.ai-input-row { display: flex; gap: 8px; padding: 8px 12px 12px; }

.ai-fade-enter-active, .ai-fade-leave-active { transition: opacity .25s, transform .25s; }
.ai-fade-enter-from, .ai-fade-leave-to { opacity: 0; transform: translateY(12px) scale(.97); }
</style>
