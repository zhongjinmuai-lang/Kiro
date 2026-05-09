import axios from 'axios'

/**
 * 开发商后台：构建时通过 VITE_API_BASE_URL 注入 API 基地址
 * - 自身服务器同机部署：空字符串，走同域 Nginx 反代
 * - 跨机部署：填完整 HTTPS URL（一般不推荐，开发商后台建议与 API 同机）
 */
const baseURL = (import.meta as any).env?.VITE_API_BASE_URL || ''

export const api = axios.create({
  baseURL,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('mu_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (resp) => {
    const nt = resp.headers['x-new-access-token']
    if (nt) localStorage.setItem('mu_token', nt)
    const nr = resp.headers['x-new-refresh-token']
    if (nr) localStorage.setItem('mu_refresh', nr)
    return resp
  },
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('mu_token')
      localStorage.removeItem('mu_refresh')
      if (!location.pathname.endsWith('/login')) location.href = '/login'
    }
    return Promise.reject(err)
  },
)
