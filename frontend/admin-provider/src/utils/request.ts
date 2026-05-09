import axios from 'axios'

/**
 * 服务商后台独立部署到服务商自己服务器：
 * 构建时注入开发商 API 地址，支持跨域
 *   VITE_API_BASE_URL=https://api.mu-developer.com npm run build
 *
 * 默认空字符串走同域 Nginx 反代（当与 API 同机时）
 */
const baseURL = (import.meta as any).env?.VITE_API_BASE_URL || ''

export const api = axios.create({ baseURL, timeout: 30000 })

api.interceptors.request.use((cfg) => {
  const token = localStorage.getItem('mu_token')
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

api.interceptors.response.use(
  (r) => {
    const nt = r.headers['x-new-access-token']
    if (nt) localStorage.setItem('mu_token', nt)
    return r
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
