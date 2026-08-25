import { defineStore } from 'pinia'
import type { UserInfo } from '@/types'

interface JwtPayload {
  user_id: string
  openid: string
  role: string | number
  nickname: string
  iat: number
  exp: number
}

/** 解码 JWT payload（不做签名校验，仅取 role 用） */
function decodeJwt(token: string): JwtPayload | null {
  try {
    const parts = token.split('.')
    if (parts.length < 2) return null
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const json = decodeURIComponent(
      atob(base64)
        .split('')
        .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )
    return JSON.parse(json) as JwtPayload
  } catch {
    return null
  }
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    userInfo: JSON.parse(localStorage.getItem('userInfo') || 'null') as
      | UserInfo
      | null
  }),
  actions: {
    setUser(user: UserInfo, token: string) {
      this.userInfo = user
      this.token = token
      localStorage.setItem('token', token)
      localStorage.setItem('userInfo', JSON.stringify(user))
    },
    logout() {
      this.userInfo = null
      this.token = ''
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
    }
  },
  getters: {
    /** JWT payload 中的 role：0=普通用户，1=平台管理员，2=超级管理员 */
    jwtRole(): string | number | null {
      if (!this.token) return null
      return decodeJwt(this.token)?.role ?? null
    },
    /** 是否为平台管理员（含超级管理员，可访问后台） */
    isPlatformAdmin(): boolean {
      return Number(this.jwtRole) >= 1
    },
    /** 是否为超级管理员（可管理用户） */
    isSuperAdmin(): boolean {
      return Number(this.jwtRole) >= 2
    }
  }
})
