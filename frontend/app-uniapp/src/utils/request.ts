/**
 * MU UniApp 移动端 HTTP 请求封装（v2.6）
 * 
 * 基于 uni.request 封装，支持：
 * - Token 自动注入
 * - 智能续签（响应头 x-new-access-token）
 * - 401 自动跳转登录
 * - 请求/响应拦截
 * - 超时控制
 */

const STORAGE_KEY_TOKEN = 'mu_app_token'
const STORAGE_KEY_REFRESH = 'mu_app_refresh'

// API 基地址（开发环境可通过条件编译切换）
// #ifdef H5
const BASE_URL = ''
// #endif
// #ifndef H5
const BASE_URL = 'https://api.example.com' // 非 H5 端需要完整地址
// #endif

interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  data?: any
  header?: Record<string, string>
  timeout?: number
  showLoading?: boolean
}

interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

/**
 * 统一请求方法
 */
export function request<T = any>(options: RequestOptions): Promise<ApiResponse<T>> {
  const { url, method = 'GET', data, header = {}, timeout = 30000, showLoading = false } = options

  // 注入 Token
  const token = uni.getStorageSync(STORAGE_KEY_TOKEN)
  if (token) {
    header['Authorization'] = `Bearer ${token}`
  }
  header['Content-Type'] = header['Content-Type'] || 'application/json'

  if (showLoading) {
    uni.showLoading({ title: '加载中...', mask: true })
  }

  return new Promise((resolve, reject) => {
    uni.request({
      url: BASE_URL + url,
      method,
      data,
      header,
      timeout,
      success: (res: any) => {
        if (showLoading) uni.hideLoading()

        // 智能续签：检查响应头
        const newToken = res.header?.['x-new-access-token'] || res.header?.['X-New-Access-Token']
        if (newToken) {
          uni.setStorageSync(STORAGE_KEY_TOKEN, newToken)
        }
        const newRefresh = res.header?.['x-new-refresh-token'] || res.header?.['X-New-Refresh-Token']
        if (newRefresh) {
          uni.setStorageSync(STORAGE_KEY_REFRESH, newRefresh)
        }

        if (res.statusCode === 200 || res.statusCode === 201) {
          resolve(res.data as ApiResponse<T>)
        } else if (res.statusCode === 401) {
          // Token 过期，尝试刷新
          handleUnauthorized().then(() => {
            // 重试原请求
            request<T>(options).then(resolve).catch(reject)
          }).catch(() => {
            // 刷新失败，跳转登录
            clearAuth()
            uni.reLaunch({ url: '/pages/login/login' })
            reject(new Error('登录已过期'))
          })
        } else {
          const msg = (res.data as any)?.message || `请求失败(${res.statusCode})`
          uni.showToast({ title: msg, icon: 'none', duration: 2000 })
          reject(new Error(msg))
        }
      },
      fail: (err: any) => {
        if (showLoading) uni.hideLoading()
        uni.showToast({ title: '网络异常', icon: 'none' })
        reject(new Error(err.errMsg || '网络错误'))
      },
    })
  })
}

/** 刷新 Token */
let isRefreshing = false
let refreshPromise: Promise<void> | null = null

async function handleUnauthorized(): Promise<void> {
  if (isRefreshing && refreshPromise) return refreshPromise

  const refreshToken = uni.getStorageSync(STORAGE_KEY_REFRESH)
  if (!refreshToken) return Promise.reject(new Error('无 refresh token'))

  isRefreshing = true
  refreshPromise = new Promise<void>((resolve, reject) => {
    uni.request({
      url: BASE_URL + '/api/v1/auth/refresh',
      method: 'POST',
      data: { refresh_token: refreshToken },
      header: { 'Content-Type': 'application/json' },
      success: (res: any) => {
        if (res.statusCode === 200 && res.data?.data) {
          const { access_token, refresh_token } = res.data.data
          if (access_token) uni.setStorageSync(STORAGE_KEY_TOKEN, access_token)
          if (refresh_token) uni.setStorageSync(STORAGE_KEY_REFRESH, refresh_token)
          resolve()
        } else {
          reject(new Error('刷新失败'))
        }
      },
      fail: () => reject(new Error('网络错误')),
      complete: () => { isRefreshing = false; refreshPromise = null },
    })
  })
  return refreshPromise
}

/** 清除认证信息 */
export function clearAuth() {
  uni.removeStorageSync(STORAGE_KEY_TOKEN)
  uni.removeStorageSync(STORAGE_KEY_REFRESH)
}

/** 是否已登录 */
export function isLoggedIn(): boolean {
  return !!uni.getStorageSync(STORAGE_KEY_TOKEN)
}

/** 保存登录信息 */
export function saveAuth(accessToken: string, refreshToken: string) {
  uni.setStorageSync(STORAGE_KEY_TOKEN, accessToken)
  uni.setStorageSync(STORAGE_KEY_REFRESH, refreshToken)
}

// ========== 便捷方法 ==========

export const api = {
  get: <T = any>(url: string, data?: any) => request<T>({ url, method: 'GET', data }),
  post: <T = any>(url: string, data?: any) => request<T>({ url, method: 'POST', data }),
  put: <T = any>(url: string, data?: any) => request<T>({ url, method: 'PUT', data }),
  del: <T = any>(url: string, data?: any) => request<T>({ url, method: 'DELETE', data }),
}

export { STORAGE_KEY_TOKEN, STORAGE_KEY_REFRESH }
