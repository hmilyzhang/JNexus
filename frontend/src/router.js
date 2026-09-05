// AutoOps 运维平台 — By JJ Zhang, Version 1.0
import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from './store'

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('./views/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: 'menu.dashboard', icon: 'Odometer' } },
      { path: 'profile', name: 'profile', component: () => import('./views/Profile.vue'), meta: { title: 'layout.profile' } },
      { path: 'k8s', name: 'k8s', component: () => import('./views/K8sClusters.vue'), meta: { title: 'k8s.title', icon: 'Grid' } },
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

router.beforeEach(to => {
  const store = useUserStore()
  if (!to.meta.public && !store.token) return '/login'
  if (to.meta.adminOnly && !store.isAdmin) return '/dashboard'
  if (to.meta.auditOnly && !(store.isAdmin || store.isAuditor)) return '/dashboard'
})

export default router
