import { defineStore } from 'pinia'
import type { EnterpriseMembership, UserInfo } from '@/types'

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
    // base64url → base64，补齐缺失的 padding，否则 atob 在部分长度下抛 InvalidCharacterError
    let base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    while (base64.length % 4) base64 += '='
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
    /** JWT payload 中的平台角色（1=维修业务员 2=超级管理员） */
    jwtRole(): string | number | null {
      if (!this.token) return null
      return decodeJwt(this.token)?.role ?? null
    },
    /** 平台角色（优先 userInfo.role，兼容仅剩 token 的场景） */
    platformRole(): number | null {
      const r = this.userInfo?.role ?? Number(this.jwtRole)
      return Number.isNaN(r) ? null : r
    },
    /** 店方角色：维修业务员(role=1) / 超级管理员(role=2)，可跨单位操作 */
    isStoreStaff(): boolean {
      return Number(this.platformRole) >= 1
    },
    /** 超级管理员（可管理用户与项目字典） */
    isSuperAdmin(): boolean {
      return Number(this.platformRole) >= 2
    },
    /** 已通过的单位关系（单位审核员/普通成员均可） */
    approvedMemberships(): EnterpriseMembership[] {
      return (this.userInfo?.enterprises ?? []).filter(e => e.status === 'approved')
    },
    /** 本人担任“单位审核员”的单位（仅这些单位可在 Web 审核/管理） */
    reviewerEnterprises(): EnterpriseMembership[] {
      return this.approvedMemberships.filter(e => e.role === 'reviewer')
    },
    /** 是否具备单位审核员身份（登录后可进“单位审核”视图） */
    hasReviewerRole(): boolean {
      return this.reviewerEnterprises.length > 0
    },
    /** 是否可管理指定单位（店方全可；单位审核员仅其担任审核员的单位） */
    canManageEnterprise(): (enterpriseId: string) => boolean {
      return (enterpriseId: string) =>
        this.isStoreStaff ||
        this.reviewerEnterprises.some(e => e.enterprise_id === enterpriseId)
    },
    /** 登录后落地页 */
    landingPath(): string {
      if (this.isStoreStaff) return '/orders'
      if (this.hasReviewerRole) return '/review'
      return '/login'
    }
  }
})
