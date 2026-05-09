import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/utils/request'

export interface UserInfo {
  id: string
  tenant_id: string
  username: string
  nickname?: string
  email?: string
  avatar?: string
}

export const useAuthStore = defineStore('developer-auth', () => {
  const token = ref<string>(localStorage.getItem('mu_token') || '')
  const refresh = ref<string>(localStorage.getItem('mu_refresh') || '')
  const user = ref<UserInfo | null>(null)
  const isAuthenticated = computed(() => !!token.value)

  /**
   * 开发商后台登录
   * 使用 /admin/v1/auth/login（走 admin-server）
   */
  async function login(tenantCode: string, username: string, password: string) {
    const { data } = await api.post('/admin/v1/auth/login', {
      tenant_code: tenantCode,
      username,
      password,
    })
    token.value = data.data.token.access_token
    refresh.value = data.data.token.refresh_token
    user.value = data.data.user
    localStorage.setItem('mu_token', token.value)
    localStorage.setItem('mu_refresh', refresh.value)
  }

  async function logout() {
    try { await api.post('/admin/v1/auth/logout', { refresh_token: refresh.value }) } catch {}
    token.value = ''
    refresh.value = ''
    user.value = null
    localStorage.removeItem('mu_token')
    localStorage.removeItem('mu_refresh')
  }

  return { token, refresh, user, isAuthenticated, login, logout }
})
