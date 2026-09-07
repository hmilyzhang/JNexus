// JNexus 运维平台 — By JJ Zhang, Version 1.0
import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from './store'

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  { path: '/k8s/manage/:id', name: 'k8s-manage', component: () => import('./views/K8sManage.vue'), meta: { title: 'k8s.title' } },
  {
    path: '/',
    component: () => import('./views/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: 'menu.dashboard', icon: 'Odometer' } },
      { path: 'profile', name: 'profile', component: () => import('./views/Profile.vue'), meta: { title: 'layout.profile' } },
      { path: 'k8s', name: 'k8s', component: () => import('./views/K8sClusters.vue'), meta: { title: 'k8s.title', icon: 'Grid' } },
      { path: 'k8s/exec', name: 'k8s-exec', component: () => import('./views/K8sExecShell.vue'), meta: { title: 'k8s.title' } },
      { path: 'shell', name: 'shell', component: () => import('./views/ShellWorkspace.vue'), meta: { title: 'shell.title', icon: 'Terminal' } },
      { path: 'hosts', name: 'hosts', component: () => import('./views/Hosts.vue'), meta: { title: 'menu.hosts', icon: 'Monitor' } },
      { path: 'os-accounts', name: 'os-accounts', component: () => import('./views/OsAccounts.vue'), meta: { title: 'menu.osaccounts', icon: 'Avatar' } },
      { path: 'exec', name: 'exec', component: () => import('./views/Exec.vue'), meta: { title: 'menu.exec', icon: 'Promotion' } },
      { path: 'tasks', name: 'tasks', component: () => import('./views/Tasks.vue'), meta: { title: 'menu.tasks', icon: 'List' } },
      { path: 'crons', name: 'crons', component: () => import('./views/CronJobs.vue'), meta: { title: 'menu.cron', icon: 'Timer' } },
      { path: 'reports', name: 'reports', component: () => import('./views/Reports.vue'), meta: { title: 'menu.reports', icon: 'DataAnalysis' } },
      { path: 'monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: 'menu.monitor', icon: 'Cpu' } },
      { path: 'files', name: 'files', component: () => import('./views/Files.vue'), meta: { title: 'menu.files', icon: 'FolderOpened' } },
      { path: 'scripts', name: 'scripts', component: () => import('./views/Scripts.vue'), meta: { title: 'menu.scripts', icon: 'Document' } },
      { path: 'apps', name: 'apps', component: () => import('./views/Apps.vue'), meta: { title: 'menu.apps', icon: 'Box' } },
      { path: 'releases', name: 'releases', component: () => import('./views/Releases.vue'), meta: { title: 'menu.releases', icon: 'UploadFilled' } },
      { path: 'users', name: 'users', component: () => import('./views/Users.vue'), meta: { title: 'menu.users', icon: 'User', adminOnly: true } },
      { path: 'danger', name: 'danger', component: () => import('./views/DangerRules.vue'), meta: { title: 'menu.danger', icon: 'Warning', adminOnly: true } },
      { path: 'audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: 'menu.audit', icon: 'Notebook', auditOnly: true } },
      { path: 'system', name: 'system', component: () => import('./views/SystemConfig.vue'), meta: { title: 'menu.system', icon: 'Setting', adminOnly: true } }
    ]
  }
]

const router = createRouter({ history: createWebHistory(), routes })

// 与主菜单同源的角色菜单配置（首次导航前拉取；拉不到时放行由后端兜底）
let roleMenus = null
const loadRoleMenus = async () => {
  if (roleMenus) return roleMenus
  try {
    const token = localStorage.getItem('token')
    if (!token) return null
    const rs = await fetch('/api/system/roles', { headers: { Authorization: 'Bearer ' + token } }).then(r => r.ok ? r.json() : null)
    roleMenus = rs || {}
  } catch { roleMenus = {} }
  return roleMenus
}

router.beforeEach(async to => {
  const store = useUserStore()
  if (!to.meta.public && !store.token) return '/login'
  if (to.meta.adminOnly && !store.isAdmin) return '/dashboard'
  if (to.meta.auditOnly && !(store.isAdmin || store.isAuditor)) return '/dashboard'
  // 非管理角色：页面不在「角色设置」的可见菜单里时回仪表盘（菜单仅控制界面，此为前端一致性守卫）
  if (!to.meta.public && !store.isAdmin && to.name) {
    const rs = await loadRoleMenus()
    const conf = rs?.[store.role]
    if (conf && Array.isArray(conf.menus)) {
      const allowed = new Set(conf.menus)
      // 路由 name 与菜单 key 的对应（menus key: exec/files/scripts 等与路由 name 一致）
      // 路由 name 与角色菜单 key 的映射差异（crons→cron、os-accounts→osaccounts）
      const nameToKey = { crons: 'cron', 'os-accounts': 'osaccounts' }
      const key = nameToKey[to.name] || to.name
      const alwaysAllowed = ['dashboard', 'profile', 'k8s-exec', 'k8s-manage', 'shell', 'login']
      if (!allowed.has(key) && !alwaysAllowed.includes(to.name)) {
        return '/dashboard'
      }
    }
  }
})

export default router
