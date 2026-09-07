/**
 * base64url 解码为 UTF-8 字符串。
 * 微信小程序环境无原生 atob，这里用 uni 自带能力简化：
 * 通过 uni.base64ToArrayBuffer 解码字节再做 UTF-8 转义。
 */
export function base64UrlDecode(input: string): string {
  // 还原 base64url -> base64
  let base64 = input.replace(/-/g, '+').replace(/_/g, '/')
  while (base64.length % 4 !== 0) {
    base64 += '='
  }
  const bytes = uni.base64ToArrayBuffer(base64)
  const view = new Uint8Array(bytes)
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
 * 角色能力判定已迁移至 utils/auth.ts（V1.2 双层模型）：
 * isStoreStaff / isSuperAdmin / isUnitReviewer / isReviewerOf 等。
 * 本模块仅保留 JWT 解码能力。
 */
