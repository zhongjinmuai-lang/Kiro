import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/utils/request'

export interface UserInfo {
  id: string
  username: string
  nickname: string
  tenantId: string
  level: 'developer' | 'provider' | 'customer'
  role: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('mu_token') || '')
  const user = ref<UserInfo | null>(null)

  const isAuthenticated = computed(() => !!token.value)

  async function login(username: string, password: string) {
    const res = await api.post('/api/v1/auth/login', { username, password })
    token.value = res.data.data.token
    user.value = res.data.data.user
    localStorage.setItem('mu_token', token.value)
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('mu_token')
  }

  return { token, user, isAuthenticated, login, logout }
})
