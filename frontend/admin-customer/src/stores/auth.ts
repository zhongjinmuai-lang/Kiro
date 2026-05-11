import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api, STORAGE_KEY_TOKEN, STORAGE_KEY_REFRESH } from '@/utils/request'

export const useAuthStore = defineStore('customer-auth', () => {
  const token = ref<string>(localStorage.getItem(STORAGE_KEY_TOKEN) || '')
  const refresh = ref<string>(localStorage.getItem(STORAGE_KEY_REFRESH) || '')
  const user = ref<any>(null)
  const isAuthenticated = computed(() => !!token.value)

  async function login(tenantCode: string, username: string, password: string) {
    const { data } = await api.post('/api/v1/auth/login', {
      tenant_code: tenantCode, username, password,
    })
    token.value = data.data.token.access_token
    refresh.value = data.data.token.refresh_token
    user.value = data.data.user
    localStorage.setItem(STORAGE_KEY_TOKEN, token.value)
    localStorage.setItem(STORAGE_KEY_REFRESH, refresh.value)
  }

  async function fetchProfile() {
    if (!token.value) return
    try { const { data } = await api.get('/api/v1/me'); user.value = data.data } catch {}
  }

  async function logout() {
    try { await api.post('/api/v1/auth/logout', { refresh_token: refresh.value }) } catch {}
    token.value = ''; refresh.value = ''; user.value = null
    localStorage.removeItem(STORAGE_KEY_TOKEN)
    localStorage.removeItem(STORAGE_KEY_REFRESH)
  }

  return { token, refresh, user, isAuthenticated, login, logout, fetchProfile }
})
