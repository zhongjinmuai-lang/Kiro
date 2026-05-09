import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: { title: '控制台' },
  },
  {
    path: '/tenants',
    name: 'Tenants',
    component: () => import('@/views/tenants/List.vue'),
    meta: { title: '租户管理' },
  },
  {
    path: '/providers',
    name: 'Providers',
    component: () => import('@/views/providers/List.vue'),
    meta: { title: '服务商管理' },
  },
  {
    path: '/payment',
    name: 'Payment',
    component: () => import('@/views/platform/Payment.vue'),
    meta: { title: '支付中台' },
  },
  {
    path: '/storage',
    name: 'Storage',
    component: () => import('@/views/platform/Storage.vue'),
    meta: { title: '存储中台' },
  },
  {
    path: '/notify',
    name: 'Notify',
    component: () => import('@/views/platform/Notify.vue'),
    meta: { title: '通知中台' },
  },
  {
    path: '/plugins',
    name: 'Plugins',
    component: () => import('@/views/agent/Plugins.vue'),
    meta: { title: '插件管理' },
  },
  {
    path: '/agent',
    name: 'Agent',
    component: () => import('@/views/agent/Engine.vue'),
    meta: { title: '智能体引擎' },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/Settings.vue'),
    meta: { title: '系统设置' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 路由守卫
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('mu_token')
  if (to.meta.requiresAuth !== false && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
