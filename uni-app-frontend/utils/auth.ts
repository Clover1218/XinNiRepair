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

/* ==================== V1.3：服务主页入口可见性 ==================== */

/** 本人已通过（approved）的成员单位 */
export function memberEnterprises(): { enterprise_id: string; enterprise_name: string }[] {
  return (useUserStore().userInfo?.enterprises ?? [])
    .filter((e) => e.status === 'approved')
    .map((e) => ({ enterprise_id: e.enterprise_id, enterprise_name: e.enterprise_name }))
}

export interface ServiceEntryVisibility {
  /** 我的工单 */
  myOrders: boolean
  /** 审核工单 */
  reviewOrders: boolean
  /** 处理工单 */
  repairOrders: boolean
  /** 企业管理 */
  enterprise: boolean
}

/**
 * 服务主页入口可见性矩阵（V1.3）：
 * - 普通用户(0)：我的工单
 * - 单位审核员(0+reviewer)：我的工单 + 审核工单 + 企业管理
 * - 维修业务员(1)：处理工单 + 企业管理；兼任审核员时加审核工单；兼任单位成员时加我的工单
 * - 超级管理员(2)：全部
 */
export function serviceEntryVisibility(): ServiceEntryVisibility {
  const role = platformRole()
  const reviewer = isUnitReviewer()
  const member = memberEnterprises().length > 0
  return {
    myOrders: role === 0 || role === 2 || (role === 1 && member),
    reviewOrders: role === 2 || reviewer,
    repairOrders: role >= 1,
    enterprise: role >= 1 || reviewer
  }
}
