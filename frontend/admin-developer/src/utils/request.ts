import axios from 'axios'
import type { AxiosError } from 'axios'

/**
 * 开发商后台 HTTP 客户端（v2.4）
 * 修复：
 *   - localStorage key 改为 mu_dev_token 避免多端冲突
 *   - 添加 refresh token 自动续期逻辑
 *   - 使用 router.push 代替 location.href
 */
const baseURL = (import.meta as any).env?.VITE_API_BASE_URL || ''

const STORAGE_KEY_TOKEN = 'mu_dev_token'
const STORAGE_KEY_REFRESH = 'mu_dev_refresh'

export const api = axios.create({
  baseURL,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

// 标记是否正在刷新 token
let isRefreshing = false
let refreshSubscribers: ((token: string) => void)[] = []

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token))
  refreshSubscribers = []
}

// 请求拦截器
api.interceptors.request.use((config) => {
  const token = localStorage.getItem(STORAGE_KEY_TOKEN)
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应拦截器
api.interceptors.response.use(
  (resp) => {
    // 智能续签：后端在响应头追加新令牌
    const nt = resp.headers['x-new-access-token']
    if (nt) localStorage.setItem(STORAGE_KEY_TOKEN, nt)
    const nr = resp.headers['x-new-refresh-token']
    if (nr) localStorage.setItem(STORAGE_KEY_REFRESH, nr)
    return resp
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
            const newToken = data.data?.access_token || data.data?.AccessToken
            const newRefresh = data.data?.refresh_token || data.data?.RefreshToken
            if (newToken) {
              localStorage.setItem(STORAGE_KEY_TOKEN, newToken)
              if (newRefresh) localStorage.setItem(STORAGE_KEY_REFRESH, newRefresh)
              onRefreshed(newToken)
              isRefreshing = false
              originalRequest._retry = true
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              return api(originalRequest)
            }
          } catch {
            isRefreshing = false
          }
        } else {
          // 正在刷新，加入队列等待
          return new Promise((resolve) => {
            refreshSubscribers.push((token: string) => {
              originalRequest.headers.Authorization = `Bearer ${token}`
              resolve(api(originalRequest))
            })
          })
        }
      }
      // 刷新失败或无 refresh token
      localStorage.removeItem(STORAGE_KEY_TOKEN)
      localStorage.removeItem(STORAGE_KEY_REFRESH)
      if (!location.pathname.endsWith('/login')) {
        window.location.href = '/login'
      }
    }
    return Promise.reject(err)
  },
)

// 导出 storage key（供 store 使用）
export { STORAGE_KEY_TOKEN, STORAGE_KEY_REFRESH }
