import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { STORAGE_KEY_TOKEN } from '@/utils/request'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'Login', component: () => import('@/views/Login.vue'), meta: { requiresAuth: false } },
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '控制台' } },
  { path: '/tenants', name: 'Tenants', component: () => import('@/views/tenants/List.vue'), meta: { title: '租户管理' } },
  { path: '/providers', name: 'Providers', component: () => import('@/views/providers/List.vue'), meta: { title: '服务商管理' } },
  { path: '/payment', name: 'Payment', component: () => import('@/views/platform/Payment.vue'), meta: { title: '支付中台' } },
  { path: '/storage', name: 'Storage', component: () => import('@/views/platform/Storage.vue'), meta: { title: '存储中台' } },
  { path: '/notify', name: 'Notify', component: () => import('@/views/platform/Notify.vue'), meta: { title: '通知中台' } },
  { path: '/plugins', name: 'Plugins', component: () => import('@/views/agent/Plugins.vue'), meta: { title: '插件管理' } },
  { path: '/agent', name: 'Agent', component: () => import('@/views/agent/Engine.vue'), meta: { title: '智能体引擎' } },
  { path: '/settings', name: 'Settings', component: () => import('@/views/Settings.vue'), meta: { title: '系统设置' } },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to, _from, next) => {
  // 设置页面标题
  const title = to.meta.title as string
  document.title = title ? `${title} - MU 开发商后台` : 'MU 开发商后台'

  // 认证守卫
  const token = localStorage.getItem(STORAGE_KEY_TOKEN)
  if (to.meta.requiresAuth !== false && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
