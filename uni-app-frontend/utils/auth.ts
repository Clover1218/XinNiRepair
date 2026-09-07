import { decodeToken } from '@/utils/jwt'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'
import type { PlatformRole } from '@/types'

/**
 * V1.2 角色能力判定（双层模型：全局 users.role × 单位内 memberships.role）。
 * 替代 V1.1 utils/jwt.ts 中 isPlatformAdmin 的语义。
 */

function platformRole(): PlatformRole {
  const user = useUserStore().userInfo
  if (user && typeof user.role === 'number') return user.role as PlatformRole
  const token = uni.getStorageSync('token') as string
  return (decodeToken(token)?.role as PlatformRole) ?? 0
}

/** 店方：维修业务员(role=1) 或 超级管理员(role=2) */
export function isStoreStaff(): boolean {
  return platformRole() >= 1
}

/** 超级管理员 */
export function isSuperAdmin(): boolean {
  return platformRole() === 2
}

/** 本人担任“单位审核员”且已通过的单位列表 */
export function reviewerEnterprises(): { enterprise_id: string; enterprise_name: string }[] {
  return (useUserStore().userInfo?.enterprises ?? [])
    .filter((e) => e.status === 'approved' && (e.role === 'reviewer' || e.role === 'admin'))
    .map((e) => ({ enterprise_id: e.enterprise_id, enterprise_name: e.enterprise_name }))
}

/** 是否具备审核员身份（任一已通过审核单位） */
export function isUnitReviewer(): boolean {
  return reviewerEnterprises().length > 0
}

/** 是否在某单位内担任审核员 */
export function isReviewerOf(enterpriseId?: string): boolean {
  if (!enterpriseId) return false
  return reviewerEnterprises().some((e) => e.enterprise_id === enterpriseId)
}

/** 当前上下文单位是否为我担任审核员的单位 */
export function isCurrentEnterpriseReviewer(): boolean {
  return isReviewerOf(useEnterpriseStore().currentEnterpriseId)
}
