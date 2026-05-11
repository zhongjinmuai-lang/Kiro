import axios from 'axios'
import type { AxiosError } from 'axios'

/**
 * 服务商后台 HTTP 客户端（v2.4）
 * 修复：localStorage key 改为 mu_pvd_token 避免多端冲突 + 添加 refresh 逻辑
 */
const baseURL = (import.meta as any).env?.VITE_API_BASE_URL || ''

const STORAGE_KEY_TOKEN = 'mu_pvd_token'
const STORAGE_KEY_REFRESH = 'mu_pvd_refresh'

export const api = axios.create({ baseURL, timeout: 30000 })

let isRefreshing = false
let refreshSubscribers: ((token: string) => void)[] = []

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token))
  refreshSubscribers = []
}

api.interceptors.request.use((cfg) => {
  const t = localStorage.getItem(STORAGE_KEY_TOKEN)
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

api.interceptors.response.use(
  (r) => {
    const nt = r.headers['x-new-access-token']
    if (nt) localStorage.setItem(STORAGE_KEY_TOKEN, nt)
    const nr = r.headers['x-new-refresh-token']
    if (nr) localStorage.setItem(STORAGE_KEY_REFRESH, nr)
    return r
  },
  async (err: AxiosError) => {
    const originalRequest = err.config as any
    if (err.response?.status === 401 && !originalRequest._retry) {
      const refreshToken = localStorage.getItem(STORAGE_KEY_REFRESH)
      if (refreshToken && !originalRequest.url?.includes('/auth/')) {
        if (!isRefreshing) {
          isRefreshing = true
          try {
            const { data } = await axios.post(
              `${baseURL}/admin/v1/auth/refresh`,
              { refresh_token: refreshToken },
            )
            const newToken = data.data?.access_token
            const newRefresh = data.data?.refresh_token
            if (newToken) {
              localStorage.setItem(STORAGE_KEY_TOKEN, newToken)
              if (newRefresh) localStorage.setItem(STORAGE_KEY_REFRESH, newRefresh)
              onRefreshed(newToken)
              isRefreshing = false
              originalRequest._retry = true
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              return api(originalRequest)
            }
          } catch { isRefreshing = false }
        } else {
          return new Promise((resolve) => {
            refreshSubscribers.push((token: string) => {
              originalRequest.headers.Authorization = `Bearer ${token}`
              resolve(api(originalRequest))
            })
          })
        }
      }
      localStorage.removeItem(STORAGE_KEY_TOKEN)
      localStorage.removeItem(STORAGE_KEY_REFRESH)
      if (!location.pathname.endsWith('/login')) window.location.href = '/login'
    }
    return Promise.reject(err)
  },
)

export { STORAGE_KEY_TOKEN, STORAGE_KEY_REFRESH }
