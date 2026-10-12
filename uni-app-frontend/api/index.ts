/**
 * 集中式接口层（移植自 Web 管理后台 web-frontend/src/api/admin.ts）
 *
 * 范围说明：
 * - 承接 Web 端工单管理、企业/成员管理、报修_FORM 的全部接口
 * - 不含：统计聚合（本期不做统计页）、Excel 导出（仅在 Web 后台）
 * - 不含：项目字典 / 用户管理 / 维修员经营范围 / 系统设置（超管四件套，仅 Web 后台）
 *
 * 所有 URL 均为相对路径，BASE_URL 已含 /api/v1（见 utils/config.ts）
 */
import { http } from '@/utils/request'
import type {
  AdminEnterpriseItem,
  AdminOrderDetail,
  AdminOrderListParams,
  AdminOrderListResult,
  AvailableAction,
  CompleteOrderInput,
  EnterpriseDetail,
  LoginResult,
  MemberItem,
  OptionsResult,
  OrderDetail,
  OrderListItem,
  PageResult,
  ReporterOption,
  SignOrderInput,
  UpdateFinanceInput,
  UploadResult,
  UserInfo
} from '@/types'

/* ==================== 认证 ==================== */

export const authAPI = {
  /** 微信 code 登录 */
  login: (code: string) => http.post<LoginResult>('/auth/login', { code }),
  /** 新用户完善资料注册 */
  register: (data: { nickname: string; avatar_url?: string; phone_code?: string; phone?: string }) =>
    http.post<LoginResult>('/auth/register', data),
  /** 当前用户信息 */
  getMe: () => http.get<UserInfo>('/auth/me')
}

/* ==================== 用户端工单（报修人视角） ==================== */

/** 用户端工单列表查询参数 */
export interface MyOrderParams {
  page?: number
  page_size?: number
  status?: string
  enterprise_id?: string
}

export const orderAPI = {
  /** 我的工单列表 GET /orders */
  list: (params?: MyOrderParams) =>
    http.get<PageResult<OrderListItem>>('/orders', params as Record<string, unknown>),

  /** 工单详情 GET /orders/{id} */
  detail: (orderId: string) => http.get<OrderDetail>(`/orders/${orderId}`),

  /** 报修选项（三级字典树 + 可见单位 + 紧急程度）GET /orders/options */
  options: () => http.get<OptionsResult>('/orders/options'),

  /** 创建草稿 POST /orders */
  create: (data: Record<string, unknown>) => http.post<OrderDetail>('/orders', data),

  /** 更新草稿 PUT /orders/{id}（含 images 全量回写，把 temporary 转 active） */
  update: (orderId: string, data: Record<string, unknown>) =>
    http.put<OrderDetail>(`/orders/${orderId}`, data),

  /** 提交上报 POST /orders/{id}/submit（此时生成工单号） */
  submit: (orderId: string) => http.post<OrderDetail>(`/orders/${orderId}/submit`, {}),

  /** 取消工单 POST /orders/{id}/cancel */
  cancel: (orderId: string, data?: { reason?: string }) =>
    http.post<OrderDetail>(`/orders/${orderId}/cancel`, data),

  /** 删除草稿 DELETE /orders/{id} */
  remove: (orderId: string) => http.delete<null>(`/orders/${orderId}`)
}

/* ==================== 管理端工单（店方 / 单位审核员） ==================== */

export const adminAPI = {
  /**
   * 工单管理列表 GET /admin/orders
   * 返回 status_counts 供状态 Tab 显示数量角标。
   * - 审核员：enterprise_id 为空时由后端按其所任审核员的全部单位做 IN 收敛
   * - 店方：按经营范围收敛
   */
  getOrders: (params?: AdminOrderListParams) =>
    http.get<AdminOrderListResult>('/admin/orders', params as Record<string, unknown>),

  /** 工单详情 GET /admin/orders/{id} */
  getOrderDetail: (orderId: string) => http.get<AdminOrderDetail>(`/admin/orders/${orderId}`),

  /** 报修人候选（远程搜索）GET /admin/orders/reporters */
  getReporters: (params?: { enterprise_id?: string; keyword?: string }) =>
    http.get<{ list: ReporterOption[] }>('/admin/orders/reporters', params as Record<string, unknown>),

  /* ---- 状态流转 ---- */

  /** 审核通过 reported → pending_accept */
  auditOrder: (orderId: string, data?: { remark?: string }) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/audit`, data),

  /** 撤回审核（V1.5）pending_accept → reported */
  unauditOrder: (orderId: string, data?: { remark?: string }) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/unaudit`, data),

  /** 接单 pending_accept → processing */
  acceptOrder: (orderId: string, data?: { remark?: string }) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/accept`, data),

  /** 退回 → draft，原因长度由后端 ReasonMinLength 控制（当前 2-200 字） */
  rejectOrder: (orderId: string, reason: string) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/reject`, { reason }),

  /** 完工登记 */
  completeOrder: (orderId: string, data: CompleteOrderInput) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/complete`, data as unknown as Record<string, unknown>),

  /** 补签正式完工（V1.5）processing + pending_sign → completed */
  signOrder: (orderId: string, data: SignOrderInput) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/signature`, data as unknown as Record<string, unknown>),

  /** 修改对账信息 */
  updateFinance: (orderId: string, data: UpdateFinanceInput) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/finance`, data as unknown as Record<string, unknown>),

  /** 重新打开 completed → processing */
  reopenOrder: (orderId: string, data?: { remark?: string }) =>
    http.post<AdminOrderDetail>(`/admin/orders/${orderId}/reopen`, data)
}

/* ==================== 企业 / 成员 ==================== */

export interface MemberListParams {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export const enterpriseAPI = {
  /**
   * 店方企业列表 GET /admin/enterprises
   *
   * ⚠️ 权限坑（与 Web 端 history: mulberry-2026-10-11 同源）：
   * 该接口在后端挂在 staff.Use(RequirePlatformAdmin()) 下，**要求 role>=1**。
   * 纯单位审核员（role=0）调用会被 403「仅管理员可执行该操作」。
   * 因此：审核员侧的企业下拉一律用 userStore 里已下发的企业清单，禁止调本方法。
   */
  list: (params?: { page?: number; page_size?: number; keyword?: string }) =>
    http.get<PageResult<AdminEnterpriseItem>>(
      '/admin/enterprises',
      params as Record<string, unknown>
    ),

  /** 企业详情（店方视角，含工单数）GET /admin/enterprises/{id} */
  detail: (enterpriseId: string) =>
    http.get<EnterpriseDetail>(`/admin/enterprises/${enterpriseId}`),

  /** 企业详情（成员/审核员视角，含 my_role）GET /enterprises/{id} */
  detailMine: (enterpriseId: string) =>
    http.get<EnterpriseDetail>(`/enterprises/${enterpriseId}`),

  /** 成员列表（店方）GET /admin/enterprises/{id}/members */
  members: (enterpriseId: string, params?: MemberListParams) =>
    http.get<PageResult<MemberItem>>(
      `/admin/enterprises/${enterpriseId}/members`,
      params as Record<string, unknown>
    ),

  /** 成员列表（审核员）GET /enterprises/{id}/members */
  membersMine: (enterpriseId: string, params?: MemberListParams) =>
    http.get<PageResult<MemberItem>>(
      `/enterprises/${enterpriseId}/members`,
      params as Record<string, unknown>
    ),

  // 注：创建企业 POST /enterprises 自 V1.5 起收紧为仅超级管理员（后端 RequireSuperAdmin），
  // 超管能力本期不移植到小程序，故此处不封装。

  /** 更新企业 PUT /enterprises/{id}（改名 / 免审核开关，局部更新） */
  update: (enterpriseId: string, data: { name?: string; auto_approve?: boolean }) =>
    http.put<EnterpriseDetail>(`/enterprises/${enterpriseId}`, data as unknown as Record<string, unknown>),

  /** 刷新邀请码 POST /enterprises/{id}/refresh/code */
  refreshInviteCode: (enterpriseId: string, validity: number) =>
    http.post<{ invite_code: string; expires_at: string }>(
      `/enterprises/${enterpriseId}/refresh/code`,
      { validity }
    ),

  /** 批量通过成员 PUT /enterprises/{id}/members/approve */
  approveMembers: (enterpriseId: string, userIds: string[]) =>
    http.put<{ updated?: number }>(`/enterprises/${enterpriseId}/members/approve`, {
      user_ids: userIds
    }),

  /** 批量拒绝成员 PUT /enterprises/{id}/members/reject */
  rejectMembers: (enterpriseId: string, userIds: string[]) =>
    http.put<{ updated?: number }>(`/enterprises/${enterpriseId}/members/reject`, {
      user_ids: userIds
    }),

  /** 移除成员 DELETE /enterprises/{id}/members/{userId} */
  removeMember: (enterpriseId: string, userId: string) =>
    http.delete<null>(`/enterprises/${enterpriseId}/members/${userId}`),

  /** 任免单位审核员 PUT /enterprises/{id}/members/{userId}/role */
  setMemberRole: (enterpriseId: string, userId: string, role: 'reviewer' | 'member') =>
    http.put<{ updated?: number }>(
      `/enterprises/${enterpriseId}/members/${userId}/role`,
      { role }
    ),

  /** 邀请码加入企业 POST /enterprises/join */
  join: (code: string) => http.post<EnterpriseDetail>('/enterprises/join', { code })
}

/* ==================== 其它 ==================== */

export const miscAPI = {
  /** 用户协议 / 隐私政策 GET /agreement/{type} */
  agreement: (type: string) => http.get<{ title: string; content: string }>(`/agreement/${type}`)
}

export type { AvailableAction, UploadResult }
