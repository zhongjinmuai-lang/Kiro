import axios from 'axios'

export const api = axios.create({
  baseURL: '',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

// 请求：注入 Bearer token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('mu_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应：智能续签 + 401 登出
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
