import axios from 'axios'

/**
 * 终端客户后台独立/贴牌部署：构建时指定开发商 API 地址
 *   VITE_API_BASE_URL=https://api.mu.example.com npm run build
 * 默认空字符串走同域 Nginx 反代
 */
const baseURL = (import.meta as any).env?.VITE_API_BASE_URL || ''

export const api = axios.create({ baseURL, timeout: 30000 })

api.interceptors.request.use((cfg) => {
  const t = localStorage.getItem('mu_token')
  if (t) cfg.headers.Authorization = `Bearer ${t}`
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
