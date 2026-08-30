<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-container style="height: 100vh">
    <el-aside width="200px" style="background:#1d2935; display:flex; flex-direction:column">
      <div class="logo">{{ systemName }}</div>
      <el-menu :default-active="$route.path" router background-color="#1d2935" text-color="#a7b1c2"
               active-text-color="#ffffff" style="border-right:none; flex:1; overflow-y:auto">
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ $t(m.title) }}</span>
        </el-menu-item>
      </el-menu>
      <div class="byline">{{ $t('layout.byline') }}</div>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="title">{{ $route.meta.title ? $t($route.meta.title) : 'AutoOps' }}</div>
        <div style="display:flex; align-items:center; gap:16px">
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
              {{ store.user?.username }}（{{ roleLabel }}）
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="password">{{ $t('layout.changePwd') }}</el-dropdown-item>
                <el-dropdown-item command="logout" divided>{{ $t('layout.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main style="padding:16px; background:#f5f7fa">
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
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n, { locales, setLocale } from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const store = useUserStore()
const router = useRouter()
const { t } = i18n.global

const systemName = ref(localStorage.getItem('system_name') || 'AutoOps')
api.get('/system/info').then(info => {
  systemName.value = info.system_name || 'AutoOps'
  localStorage.setItem('system_name', info.system_name || 'AutoOps')
}).catch(() => {})

const roleLabel = computed(() => t('layout.role' + (store.role || '').replace(/^(\w)/, c => c.toUpperCase())) ||
  ({ admin: '管理员', ops: '运维', publisher: '发布员', viewer: '只读' })[store.role] || store.role)

const menus = computed(() => {
  const items = [
    { path: '/dashboard', title: 'menu.dashboard', icon: 'Odometer' },
    { path: '/hosts', title: 'menu.hosts', icon: 'Monitor' },
    { path: '/exec', title: 'menu.exec', icon: 'Promotion' },
    { path: '/tasks', title: 'menu.tasks', icon: 'List' },
    { path: '/files', title: 'menu.files', icon: 'FolderOpened' },
    { path: '/scripts', title: 'menu.scripts', icon: 'Document' },
    { path: '/apps', title: 'menu.apps', icon: 'Box' },
    { path: '/releases', title: 'menu.releases', icon: 'UploadFilled' },
    { path: '/users', title: 'menu.users', icon: 'User', admin: true },
    { path: '/danger', title: 'menu.danger', icon: 'Warning', admin: true },
    { path: '/audit', title: 'menu.audit', icon: 'Notebook' },
    { path: '/system', title: 'menu.system', icon: 'Setting', admin: true }
  ]
  return items.filter(m => !m.admin || store.isAdmin)
})

const localeLabel = computed(() => (locales.find(l => l.value === i18n.global.locale.value) || {}).label || '中文')
const onLocale = v => setLocale(v)

const pwdVisible = ref(false)
const pwdForm = ref({ old_password: '', new_password: '' })

const onCmd = cmd => {
  if (cmd === 'logout') {
    store.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.value = { old_password: '', new_password: '' }
    pwdVisible.value = true
  }
}

const doChangePwd = async () => {
  await api.post('/change_password', pwdForm.value)
  ElMessage.success(t('common.success'))
  pwdVisible.value = false
}
</script>

<style scoped>
.logo {
  color: #fff; font-size: 18px; font-weight: bold;
  padding: 18px 20px 12px; letter-spacing: 1px;
}
.byline {
  color: #6b7a8d; font-size: 11px; text-align: center; padding: 10px 0 14px;
  border-top: 1px solid #2a3947; letter-spacing: .5px;
}
.header {
  background: #fff; display: flex; align-items: center; justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0,21,41,.08);
}
.title { font-size: 16px; font-weight: 600; }
.user-info { cursor: pointer; display: flex; align-items: center; gap: 6px; color: #333; font-size: 14px; }
</style>
