import { BASE_URL } from './config'
import type { LoginResult } from '@/types'

/** 后端统一响应结构 */
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  request_id?: string
}

interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  data?: Record<string, unknown>
}

/** 清理本地登录态（token + 上下文单位） */
function clearLoginState() {
  uni.removeStorageSync('token')
  uni.removeStorageSync('currentEnterpriseId')
}

/**
 * 裸登录请求：直接走 uni.request，不触发本模块的 401 续期逻辑（避免递归）。
 * 仅用于 ensureLoggedIn / 401 静默续期内部的 /auth/login 调用。
 */
export function rawLoginRequest(code: string): Promise<LoginResult> {
  return new Promise((resolve, reject) => {
    uni.request({
      url: `${BASE_URL}/auth/login`,
      method: 'POST',
      data: { code },
      header: { 'Content-Type': 'application/json' },
      success: (res) => {
        const body = res.data as ApiResponse<LoginResult>
        if (body && body.code === 0 && body.data) {
          resolve(body.data)
        } else {
          reject(body || new Error('登录失败'))
        }
      },
      fail: (err) => reject(err)
    })
  })
}

/** wx.login 获取 code */
export function wxLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    uni.login({ provider: 'weixin', success: (r) => resolve(r.code), fail: reject })
  })
}

/**
 * 静默续期：wx.login 换新 token。
 * 成功（已注册用户）时写入本地 token 并返回 true；全新用户/失败返回 false。
 * 不做任何 Toast/跳转，由调用方决定后续行为。
 */
let renewalPromise: Promise<boolean> | null = null

export function silentRenewSession(): Promise<boolean> {
  if (renewalPromise) return renewalPromise
  renewalPromise = new Promise<boolean>((resolve) => {
    wxLoginCode()
      .then((code) => rawLoginRequest(code))
      .then((data) => {
        if (data.need_profile || !data.user) {
          resolve(false)
          return
        }
        uni.setStorageSync('token', data.access_token)
        resolve(true)
      })
      .catch(() => resolve(false))
      .finally(() => {
        renewalPromise = null
      })
  })
  return renewalPromise
}

/**
 * 统一请求封装：
 * - 自动携带 Authorization: Bearer <token>
 * - 401 时先静默续期一次（wx.login 换新 token），成功后重放原请求
 * - 续期失败才清理登录态并跳转登录页
 * - code !== 0 时统一 Toast 错误信息
 */
export function request<T = unknown>(options: RequestOptions): Promise<T> {
  const token = uni.getStorageSync('token')
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`

  const send = (): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      uni.request({
        url: BASE_URL + options.url,
        method: options.method || 'GET',
        data: options.data,
        header: headers,
        success: (res) => {
          const body = res.data as ApiResponse<T>
          if (res.statusCode === 401) {
            handleUnauthorized()
              .then((renewed) => {
                if (renewed) {
                  // 用新 token 重放原请求
                  const retryToken = uni.getStorageSync('token')
                  const retryHeaders: Record<string, string> = { 'Content-Type': 'application/json' }
                  if (retryToken) retryHeaders.Authorization = `Bearer ${retryToken}`
                  uni.request({
                    url: BASE_URL + options.url,
                    method: options.method || 'GET',
                    data: options.data,
                    header: retryHeaders,
                    success: (retryRes) => {
                      const retryBody = retryRes.data as ApiResponse<T>
                      if (retryBody && retryBody.code === 0) {
                        resolve(retryBody.data)
                      } else {
                        const msg = (retryBody && retryBody.message) || '请求失败'
                        uni.showToast({ title: msg, icon: 'none' })
                        reject(retryBody)
                      }
                    },
                    fail: (err) => {
                      uni.showToast({ title: '网络异常，请稍后重试', icon: 'none' })
                      reject(err)
                    }
                  })
                } else {
                  // 续期失败：新用户或网络异常，仅对“已登录态失效”场景做清理
                  clearLoginState()
                  uni.showToast({ title: '登录已过期，请重新登录', icon: 'none' })
                  setTimeout(() => {
                    uni.reLaunch({ url: '/pages/auth/login' })
                  }, 600)
                  reject(body)
                }
              })
              .catch(() => reject(body))
            return
          }
          if (body && body.code === 0) {
            resolve(body.data)
          } else {
            const msg = (body && body.message) || '请求失败'
            uni.showToast({ title: msg, icon: 'none' })
            reject(body)
          }
        },
        fail: (err) => {
          uni.showToast({ title: '网络异常，请稍后重试', icon: 'none' })
          reject(err)
        }
      })
    })

  return send()
}

/** 401 统一处理：静默续期一次 */
function handleUnauthorized(): Promise<boolean> {
  return silentRenewSession()
}

/** REST 便捷方法 */
export const http = {
  get: <T = unknown>(url: string, data?: Record<string, unknown>) =>
    request<T>({ url, method: 'GET', data }),
  post: <T = unknown>(url: string, data?: Record<string, unknown>) =>
    request<T>({ url, method: 'POST', data }),
  put: <T = unknown>(url: string, data?: Record<string, unknown>) =>
    request<T>({ url, method: 'PUT', data }),
  delete: <T = unknown>(url: string, data?: Record<string, unknown>) =>
    request<T>({ url, method: 'DELETE', data })
}

/* =============== 上传（multipart/form-data，微信小程序端） =============== */

function uploadFile<T = unknown>(url: string, filePath: string, needToken: boolean): Promise<T> {
  const token = uni.getStorageSync('token')
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}${url}`,
      filePath,
      name: 'file',
      header: needToken ? { Authorization: token ? `Bearer ${token}` : '' } : {},
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as ApiResponse<T>
          if (body.code === 0) {
            resolve(body.data)
          } else {
            uni.showToast({ title: body.message || '上传失败', icon: 'none' })
            reject(body)
          }
        } catch (e) {
          reject(e)
        }
      },
      fail: (err) => {
        uni.showToast({ title: '上传失败，请重试', icon: 'none' })
        reject(err)
      }
    })
  })
}

/**
 * 工单故障图上传（POST /orders/{order_id}/images，multipart/form-data）
 * 仅工单状态为 draft 时可上传，单张 ≤5MB，jpg/png/webp
 */
export function uploadOrderImage(
  orderId: string,
  filePath: string
): Promise<{ id: string; url: string; sort_order: number; file_size: number }> {
  return uploadFile(`/orders/${orderId}/images`, filePath, true)
}

/** 头像上传（POST /upload/avatar，multipart/form-data）注册前公开，无需 token */
export function uploadAvatar(filePath: string): Promise<{ url: string }> {
  return uploadFile('/upload/avatar', filePath, false)
}

/**
 * 收据图片上传（POST /admin/orders/{order_id}/receipts，multipart/form-data）
 * 店方/审核员角色可传；工单状态 processing 或 completed，同工单 ≤3 张
 */
export function uploadReceipt(
  orderId: string,
  filePath: string
): Promise<{ id: string; url: string; file_size: number }> {
  return uploadFile(`/admin/orders/${orderId}/receipts`, filePath, true)
}
