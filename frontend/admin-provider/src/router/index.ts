import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { STORAGE_KEY_TOKEN } from '@/utils/request'

const routes: RouteRecordRaw[] = [
  { path: '/login', component: () => import('@/views/Login.vue'), meta: { requiresAuth: false } },
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '控制台' } },
  { path: '/customers', component: () => import('@/views/Customers.vue'), meta: { title: '客户管理' } },
  { path: '/payment', component: () => import('@/views/Payment.vue'), meta: { title: '支付配置' } },
  { path: '/storage', component: () => import('@/views/Storage.vue'), meta: { title: '存储管理' } },
  { path: '/notify', component: () => import('@/views/Notify.vue'), meta: { title: '通知管理' } },
  { path: '/permissions', component: () => import('@/views/Permissions.vue'), meta: { title: '权限管理' } },
  { path: '/settings', component: () => import('@/views/Settings.vue'), meta: { title: '系统设置' } },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to, _f, next) => {
  const title = to.meta.title as string
  document.title = title ? `${title} - MU 服务商后台` : 'MU 服务商后台'
  const token = localStorage.getItem(STORAGE_KEY_TOKEN)
  if (to.meta.requiresAuth !== false && !token) next('/login')
  else next()
})

export default router
