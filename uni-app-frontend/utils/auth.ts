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

/**
 * 业务维修员（店方）：仅维修业务员(role=1)。
 * ⚠️ 小程序端不把超级管理员(role=2)视为店方——超管专属能力（项目字典 / 用户管理 /
 * 经营范围 / 系统设置等）不在小程序端实现，故超管在小程序端退化为「普通用户」能力。
 * 后端 IsStoreStaff 仍含 role>=2（操作鉴权一致），此处仅为前端 UI 角色分流。
 */
export function isStoreStaff(): boolean {
  return platformRole() === 1
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
 * 服务主页入口可见性矩阵（V1.3 / 权限重分）：
 * - 普通用户(0)：我的工单（仅上报）
 * - 企业管理员 = 单位审核员(0+reviewer)：我的工单 + 审核工单 + 企业管理
 * - 业务维修员 = 维修业务员(1)：我的工单 + 处理工单
 * - 超级管理员(2)：小程序端暂不做超管专属能力，仅保留「我的工单」（上报）兜底
 * 注：企业管理仅对「企业管理员（单位审核员）」开放；业务维修员（店方）仅维修、不管理客户企业。
 */
export function serviceEntryVisibility(): ServiceEntryVisibility {
  const role = platformRole()
  const reviewer = isUnitReviewer()
  const repairer = role === 1 // 业务维修员（店方）
  return {
    myOrders: true, // 所有角色均可上报自己的工单
    reviewOrders: reviewer, // 企业管理员
    repairOrders: repairer, // 业务维修员
    enterprise: reviewer // 企业管理员（单位审核员）
  }
}
