import axios from 'axios'

export const api = axios.create({ baseURL: '', timeout: 30000 })

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
