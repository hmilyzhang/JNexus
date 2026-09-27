<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div v-loading="loading">
    <el-row :gutter="16">
      <el-col :span="8">
        <el-card class="user-card">
          <div style="display:flex; align-items:center; gap:14px">
            <el-avatar :size="56" style="background:#409eff; font-size:24px">{{ initial }}</el-avatar>
            <div>
              <div style="font-size:18px; font-weight:600">{{ info.user?.display_name || info.user?.username }}</div>
              <div style="color:#909399; font-size:13px; margin-top:4px">
                {{ roleLabel }} · {{ info.user?.auth_source === 'ldap' ? $t('users.authLdap') : $t('users.authLocal') }}
              </div>
            </div>
          </div>
          <el-divider style="margin:14px 0" />
          <div class="user-meta">
            <div><span>{{ $t('dashboard.role') }}</span><b>{{ roleLabel }}</b></div>
            <div><span>{{ $t('dashboard.authSource') }}</span><b>{{ info.user?.auth_source === 'ldap' ? 'LDAP' : 'Local' }}</b></div>
            <div><span>{{ $t('dashboard.lastLogin') }}</span><b>{{ formatTime(info.user?.last_login_at) }}</b></div>
          </div>
        </el-card>

        <el-card :header="$t('dashboard.quickNav')" style="margin-top:16px">
          <div class="quick-nav">
            <el-button v-for="q in quickNavs" :key="q.path" @click="$router.push(q.path)">
              <el-icon style="margin-right:4px"><component :is="q.icon" /></el-icon>{{ $t(q.label) }}
            </el-button>
          </div>
        </el-card>
      </el-col>

      <el-col :span="16">
        <el-row :gutter="16">
          <el-col :span="12" v-for="s in statCards" :key="s.key">
            <el-card class="stat-card" @click="s.path && $router.push(s.path)">
              <div class="stat-icon" :style="{ background: s.color }"><el-icon :size="26"><component :is="s.icon" /></el-icon></div>
              <div>
                <div class="stat-value">{{ info[s.key] ?? '-' }}</div>
                <div class="stat-label">{{ $t(s.label) }}</div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const loading = ref(true)
const info = ref({ user: {} })

const roleLabel = computed(() => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer'),
  auditor: t('layout.roleAuditor')
})[store.role] || store.role)
const initial = computed(() => (info.value.user?.display_name || store.user?.display_name || store.user?.username || '?')[0].toUpperCase())

const statCards = computed(() => {
  const all = [
    { key: 'hosts_total', label: 'dashboard.hostsTotal', icon: 'Monitor', color: '#409eff', path: '/hosts' },
    { key: 'hosts_online', label: 'dashboard.hostsOnline', icon: 'Connection', color: '#67c23a', path: '/hosts' },
    { key: 'host_groups', label: 'dashboard.hostGroups', icon: 'Folder', color: '#e6a23c', path: '/hosts' },
    { key: 'tasks', label: 'dashboard.tasks', icon: 'Promotion', color: '#909399', path: '/tasks' },
    { key: 'apps', label: 'dashboard.apps', icon: 'Box', color: '#f56c6c', path: '/apps' },
    { key: 'releases', label: 'dashboard.releases', icon: 'UploadFilled', color: '#9254de', path: '/releases' },
    { key: 'users', label: 'dashboard.users', icon: 'User', color: '#36cfc9', path: '/users', admin: true },
    { key: 'danger_rules', label: 'dashboard.dangerRules', icon: 'Warning', color: '#fa8c16', path: '/danger', admin: true }
  ]
  // Hide cards without permission (users/danger rules are admin-only; tasks hidden from viewers)
  return all.filter(c => {
    if (c.admin && !store.isAdmin) return false
    if (c.key === 'tasks' && store.role === 'viewer') return false
    return true
  })
})

// Quick nav mirrors the main menu: for non-admins, filter by the menus visible in role settings
const roleSettings = ref({})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})
const quickNavs = computed(() => {
  const all = [
    { path: '/exec', key: 'exec', label: 'menu.exec', icon: 'Promotion' },
    { path: '/releases', key: 'releases', label: 'menu.releases', icon: 'UploadFilled' },
    { path: '/hosts', key: 'hosts', label: 'menu.hosts', icon: 'Monitor' },
    { path: '/audit', key: 'audit', label: 'menu.audit', icon: 'Notebook' }
  ]
  if (store.isAdmin) return all.filter(q => q.path !== '/audit' || store.isAdmin || store.isAuditor)
  const conf = roleSettings.value[store.role]
  const allowed = new Set(conf?.menus || [])
  // Hide quick nav entries whose target page is not in the role's menus (viewer defaults to dashboard only, so the list is empty)
  return all.filter(q => q.key === 'audit' ? (store.isAdmin || store.isAuditor) : allowed.has(q.key))
})

const formatTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')

onMounted(async () => {
  try { info.value = await api.get('/dashboard') } finally { loading.value = false }
})
</script>

<style scoped>
.user-card .user-meta > div {
  display: flex; justify-content: space-between; font-size: 13px; padding: 5px 0; color: var(--el-text-color-regular);
}
.user-meta span { color: #909399; }
.stat-card {
  margin-bottom: 16px; cursor: pointer; display: flex;
}
.stat-card :deep(.el-card__body) { display: flex; align-items: center; gap: 16px; width: 100%; }
.stat-icon {
  width: 56px; height: 56px; border-radius: 10px; color: #fff;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.stat-value { font-size: 28px; font-weight: 700; line-height: 1.2; }
.stat-label { color: #909399; font-size: 13px; margin-top: 2px; }
.quick-nav { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
