import axios from 'axios'

export const api = axios.create({
  baseURL: '',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器：注入Token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('mu_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器：统一错误处理
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('mu_token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)
