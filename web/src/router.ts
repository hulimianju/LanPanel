import { createRouter, createWebHistory } from 'vue-router'
import { useApp } from '@/stores/app'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'panel', component: () => import('@/views/PanelView.vue') },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/setup', name: 'setup', component: () => import('@/views/SetupView.vue') },
    {
      path: '/admin',
      component: () => import('@/layouts/AdminLayout.vue'),
      meta: { admin: true },
      children: [
        { path: '', redirect: '/admin/settings' },
        { path: 'settings', name: 'settings', component: () => import('@/views/admin/SettingsView.vue'), meta: { title: '设置' } },
        { path: 'discovery', name: 'discovery', component: () => import('@/views/admin/ComingSoon.vue'), meta: { title: '设备发现' } },
        { path: 'scan', name: 'scan', component: () => import('@/views/admin/ComingSoon.vue'), meta: { title: '扫描任务' } },
        { path: 'rules', name: 'rules', component: () => import('@/views/admin/ComingSoon.vue'), meta: { title: '端口规则' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const app = useApp()
  if (!app.ready) await app.load()
  if (app.needSetup) return to.name === 'setup' ? true : { name: 'setup' }
  if (to.name === 'setup') return { name: 'panel' }
  if (to.name === 'login') return app.user ? { name: 'panel' } : true
  if (to.meta.admin && !app.user) return { name: 'login', query: { next: to.fullPath } }
  if (to.name === 'panel' && !app.user && !app.settings?.publicPanel) return { name: 'login' }
  return true
})

export default router
