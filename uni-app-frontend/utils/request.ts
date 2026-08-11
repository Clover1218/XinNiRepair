import { BASE_URL } from './config'

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

/**
 * 统一请求封装：
 * - 自动携带 Authorization: Bearer <token>
 * - 401 时清除本地登录态并跳转登录页
 * - code !== 0 时统一 Toast 错误信息
 */
export function request<T = unknown>(options: RequestOptions): Promise<T> {
  const token = uni.getStorageSync('token')
  return new Promise((resolve, reject) => {
    uni.request({
      url: BASE_URL + options.url,
      method: options.method || 'GET',
      data: options.data,
      header: {
        'Content-Type': 'application/json',
        Authorization: token ? `Bearer ${token}` : ''
      },
      success: (res) => {
        const body = res.data as ApiResponse<T>
        if (res.statusCode === 401) {
          uni.removeStorageSync('token')
          uni.removeStorageSync('currentEnterpriseId')
          uni.showToast({ title: '登录已过期，请重新登录', icon: 'none' })
          setTimeout(() => {
            uni.reLaunch({ url: '/pages/auth/login' })
          }, 600)
          reject(body)
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

/**
 * 工单图片上传（POST /orders/{order_id}/images，multipart/form-data）
 * 仅工单状态为 draft 时可上传，单张 ≤5MB，jpg/png/webp
 */
export function uploadOrderImage(orderId: string, filePath: string): Promise<{
  id: string
  url: string
  sort_order: number
  file_size: number
}> {
  const token = uni.getStorageSync('token')
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}/orders/${orderId}/images`,
      filePath,
      name: 'file',
      header: {
        Authorization: token ? `Bearer ${token}` : ''
      },
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as ApiResponse
          if (body.code === 0) {
            resolve(body.data as never)
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
 * 头像上传（POST /upload/avatar，multipart/form-data）
 * 注册前公开接口，无需 token；单张 ≤2MB，jpg/png/webp
 */
export function uploadAvatar(filePath: string): Promise<{ url: string }> {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}/upload/avatar`,
      filePath,
      name: 'file',
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as ApiResponse
          if (body.code === 0) {
            resolve(body.data as never)
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
 * 收据图片上传（POST /admin/orders/{order_id}/receipts，multipart/form-data）
 * 仅管理员可上传，工单状态为 processing，单张 ≤5MB，jpg/png/webp，同工单 ≤3 张
 */
export function uploadReceipt(
  orderId: string,
  filePath: string
): Promise<{ id: string; url: string; file_size: number }> {
  const token = uni.getStorageSync('token')
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}/admin/orders/${orderId}/receipts`,
      filePath,
      name: 'file',
      header: {
        Authorization: token ? `Bearer ${token}` : ''
      },
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as ApiResponse
          if (body.code === 0) {
            resolve(body.data as never)
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
