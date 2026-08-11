import { useUserStore } from '@/stores/user'

/**
 * base64url 解码为 UTF-8 字符串。
 * 微信小程序环境无原生 atob，这里用 uni 自带能力简化：
 * 通过 Base64.decode 或手动实现。此处用 base64 手工解码并转 UTF-8。
 */
export function base64UrlDecode(input: string): string {
  // 还原 base64url -> base64
  let base64 = input.replace(/-/g, '+').replace(/_/g, '/')
  while (base64.length % 4 !== 0) {
    base64 += '='
  }
  // 小程序中 wx.base64ToArrayBuffer 可用，做 UTF-8 解码
  const bytes = uni.base64ToArrayBuffer(base64)
  const view = new Uint8Array(bytes)
  // 简易 UTF-8 解码
  let str = ''
  for (let i = 0; i < view.length; i++) {
    str += String.fromCharCode(view[i])
  }
  try {
    return decodeURIComponent(
      str.replace(/%([0-9A-F]{2})/g, (_, hex) => String.fromCharCode(parseInt(hex, 16)))
    )
  } catch (e) {
    return str
  }
}

/** 解析 JWT payload（第二段，base64url 编码的 JSON） */
export function decodeToken(token: string): Record<string, unknown> | null {
  if (!token) return null
  try {
    const parts = token.split('.')
    if (parts.length < 2) return null
    const json = base64UrlDecode(parts[1])
    return JSON.parse(json) as Record<string, unknown>
  } catch (e) {
    return null
  }
}

/**
 * 是否为平台管理员（role === 1）：
 * 优先读取登录/me 接口返回的 user.role（更可靠），
 * 兜底解析本地 JWT payload 的 role 字段。
 */
export function isPlatformAdmin(): boolean {
  const user = useUserStore().userInfo
  if (user && typeof user.role === 'number') {
    return user.role === 1
  }
  const token = uni.getStorageSync('token') as string
  return decodeToken(token)?.role === 1
}
