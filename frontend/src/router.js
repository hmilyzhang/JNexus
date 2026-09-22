// JNexus Ops Platform — By JJ Zhang, Version 1.0
import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from './store'

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  { path: '/k8s/manage/:id', name: 'k8s-manage', component: () => import('./views/K8sManage.vue'), meta: { title: 'k8s.title' } },
  { path: '/screen', name: 'screen', component: () => import('./views/Screen.vue'), meta: { title: 'monitor.bigScreen' } },
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
      { path: 'rdp', name: 'rdp', component: () => import('./views/RdpClient.vue'), meta: { title: 'hosts.rdp' } },
      { path: 'hosts', name: 'hosts', component: () => import('./views/Hosts.vue'), meta: { title: 'menu.assets', icon: 'Monitor' } },
      { path: 'databases', name: 'databases', component: () => import('./views/Databases.vue'), meta: { title: 'menu.databases', icon: 'Coin' } },
      { path: 'os-accounts', name: 'os-accounts', component: () => import('./views/OsAccounts.vue'), meta: { title: 'menu.osaccounts', icon: 'Avatar' } },
      { path: 'exec', name: 'exec', component: () => import('./views/Exec.vue'), meta: { title: 'menu.exec', icon: 'Promotion' } },
      { path: 'tasks', name: 'tasks', component: () => import('./views/Tasks.vue'), meta: { title: 'menu.tasks', icon: 'List' } },
      { path: 'crons', name: 'crons', component: () => import('./views/CronJobs.vue'), meta: { title: 'menu.cron', icon: 'Timer' } },
      { path: 'reports', name: 'reports', component: () => import('./views/Reports.vue'), meta: { title: 'menu.reports', icon: 'DataAnalysis' } },
      { path: 'monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: 'menu.monitor', icon: 'Cpu' } },
      { path: 'observe', name: 'observe', component: () => import('./views/Observe.vue'), meta: { title: 'menu.observe', icon: 'DataLine' } },
      { path: 'files', name: 'files', component: () => import('./views/Files.vue'), meta: { title: 'menu.files', icon: 'FolderOpened' } },
      { path: 'scripts', name: 'scripts', component: () => import('./views/Scripts.vue'), meta: { title: 'menu.scripts', icon: 'Document' } },
      { path: 'logtail', name: 'logtail', component: () => import('./views/LogTail.vue'), meta: { title: 'menu.logtail', icon: 'View' } },
      { path: 'webapps', name: 'webapps', component: () => import('./views/WebApps.vue'), meta: { title: 'menu.webapps', icon: 'Link' } },
      { path: 'web-session/:id', name: 'web-session', component: () => import('./views/WebSession.vue'), meta: { title: 'webapp.sessionTitle' } },
      { path: 'apps', name: 'apps', component: () => import('./views/Apps.vue'), meta: { title: 'menu.apps', icon: 'Box' } },
      { path: 'releases', name: 'releases', component: () => import('./views/Releases.vue'), meta: { title: 'menu.releases', icon: 'UploadFilled' } },
      { path: 'users', name: 'users', component: () => import('./views/Users.vue'), meta: { title: 'menu.users', icon: 'User', adminOnly: true } },
      { path: 'danger', name: 'danger', component: () => import('./views/DangerRules.vue'), meta: { title: 'menu.danger', icon: 'Warning', adminOnly: true } },
      { path: 'audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: 'menu.audit', icon: 'Notebook', auditOnly: true } },
      { path: 'system', name: 'system', component: () => import('./views/SystemConfig.vue'), meta: { title: 'menu.system', icon: 'Setting', adminOnly: true } },
      { path: 'oo-admin', name: 'oo-admin', component: () => import('./views/ObserveAdmin.vue'), meta: { title: 'menu.observeAdmin', icon: 'DataLine', adminOnly: true } }
    ]
  },
  // Unknown paths: back to the dashboard (avoids a blank page on stale links)
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({ history: createWebHistory(), routes })

// Role menu config sourced from the main menu (fetched before the first navigation; if the fetch fails, allow through — the backend is the final guard)
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
  // Non-admin roles: redirect to the dashboard if the page is not in the role's visible menu
  // (menus only control the UI; this is a frontend consistency guard)
  if (!to.meta.public && !store.isAdmin && to.name) {
    const rs = await loadRoleMenus()
    const conf = rs?.[store.role]
    if (conf && Array.isArray(conf.menus)) {
      const allowed = new Set(conf.menus)
      // Correspondence between route names and menu keys (menu keys such as exec/files/scripts match route names)
      // Route name to role menu key mapping differences (crons→cron, os-accounts→osaccounts)
      const nameToKey = { crons: 'cron', 'os-accounts': 'osaccounts' }
      const key = nameToKey[to.name] || to.name
      const alwaysAllowed = ['dashboard', 'profile', 'k8s-exec', 'k8s-manage', 'shell', 'login', 'rdp']
      if (!allowed.has(key) && !alwaysAllowed.includes(to.name)) {
        return '/dashboard'
      }
    }
  }
})

export default router
