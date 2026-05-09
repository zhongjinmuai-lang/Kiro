import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/login',     component: () => import('@/views/Login.vue'),     meta: { requiresAuth: false } },
  { path: '/',          redirect: '/dashboard' },
  { path: '/dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '工作台' } },
  { path: '/genealogy', component: () => import('@/views/Genealogy.vue'), meta: { title: '族谱可视化' } },
  { path: '/files',     component: () => import('@/views/Files.vue'),     meta: { title: '文件管理' } },
  { path: '/messages',  component: () => import('@/views/Messages.vue'),  meta: { title: '消息中心' } },
  { path: '/account',   component: () => import('@/views/Account.vue'),   meta: { title: '账户设置' } },
]

const router = createRouter({ history: createWebHistory(), routes })
router.beforeEach((to, _f, next) => {
  const token = localStorage.getItem('mu_token')
  if (to.meta.requiresAuth !== false && !token) next('/login')
  else next()
})
export default router
