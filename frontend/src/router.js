import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from './store'

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('./views/MainLayout.vue'),
    redirect: '/hosts',
    children: [
      { path: 'hosts', name: 'hosts', component: () => import('./views/Hosts.vue'), meta: { title: '主机管理', icon: 'Monitor' } },
      { path: 'exec', name: 'exec', component: () => import('./views/Exec.vue'), meta: { title: '批量执行', icon: 'Promotion' } },
      { path: 'tasks', name: 'tasks', component: () => import('./views/Tasks.vue'), meta: { title: '执行记录', icon: 'List' } },
      { path: 'files', name: 'files', component: () => import('./views/Files.vue'), meta: { title: '文件分发', icon: 'FolderOpened' } },
      { path: 'scripts', name: 'scripts', component: () => import('./views/Scripts.vue'), meta: { title: '脚本中心', icon: 'Document' } },
      { path: 'apps', name: 'apps', component: () => import('./views/Apps.vue'), meta: { title: '应用管理', icon: 'Box' } },
      { path: 'releases', name: 'releases', component: () => import('./views/Releases.vue'), meta: { title: '发布中心', icon: 'UploadFilled' } },
      { path: 'users', name: 'users', component: () => import('./views/Users.vue'), meta: { title: '用户权限', icon: 'User', adminOnly: true } },
      { path: 'danger', name: 'danger', component: () => import('./views/DangerRules.vue'), meta: { title: '危险命令规则', icon: 'Warning', adminOnly: true } },
      { path: 'audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: '审计日志', icon: 'Notebook' } },
      { path: 'terminal/:hostId', name: 'terminal', component: () => import('./views/Terminal.vue'), meta: { title: 'Web 终端', hideMenu: true } }
    ]
  }
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(to => {
  const store = useUserStore()
  if (!to.meta.public && !store.token) return '/login'
  if (to.meta.adminOnly && !store.isAdmin) return '/hosts'
})

export default router
