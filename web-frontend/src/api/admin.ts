import client from './client'
import type {
  DictionaryCategory,
  DictionaryCategoryTree,
  DictionaryProblem,
  DictionaryProperty,
  EnterpriseDetail,
  EnterpriseListItem,
  EnterpriseMineDetail,
  EnterpriseStats,
  MemberItem,
  OrderDetail,
  OrderListItem,
  OrderMetadata,
  OrderOptions,
  OrderStats,
  OrderStatsMetrics,
  PageResult,
  RepairerStats,
  RepairerSummary,
  ReporterOption,
  UserDetail,
  UserListItem
} from '@/types'

export interface OrderListParams {
  page?: number
  page_size?: number
  status?: string
  urgency?: string
  keyword?: string
  /** 企业精确筛选（下拉选择；单位审核员必传且限定本单位） */
  enterprise_id?: string
  /** 项目大类筛选（V1.4） */
  category_id?: string
  /** 项目属性/问题类型筛选（V1.4，需属于所选大类） */
  property_id?: string
  date_from?: string
  date_to?: string
  reporter_id?: string
  sort_by?: string
  sort_order?: string
}

export interface EnterpriseListParams {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
}

export interface MemberListParams {
  page?: number
  page_size?: number
  status?: string
  role?: 'reviewer' | 'member'
  keyword?: string
}

export const adminAPI = {
  // 5.1 工单列表（管理后台；单位审核员必须携带 enterprise_id，后端按 Operator 校验）
  getOrders: (params: OrderListParams) =>
    client.get<PageResult<OrderListItem>>('/admin/orders', { params }),

  // 5.1 配套：报修人筛选下拉候选（按企业域+昵称模糊；来源于历史工单报修人）
  getReporters: (params: { enterprise_id?: string; keyword?: string }) =>
    client.get<{ list: ReporterOption[] }>('/admin/orders/reporters', { params }),

  // 5.2 工单详情（管理后台）
  getOrderDetail: (orderId: string) =>
    client.get<OrderDetail>(`/admin/orders/${orderId}`),

  // 5.17 工单汇总统计（含今日概况；店方可全域/审核员限本单位）
  getOrderStats: (params: { enterprise_id?: string; start?: string; end?: string }) =>
    client.get<OrderStats>('/admin/orders/stats', { params }),

  // 5.18 企业维度分组聚合（仅店方/超管；供「统计」页企业对比）
  getOrderStatsByEnterprise: (params: { enterprise_id?: string; start?: string; end?: string }) =>
    client.get<EnterpriseStats>('/admin/orders/stats/by-enterprise', { params }),

  // 5.19 维修员维度聚合（仅店方/超管；供「统计」页维修员业绩）
  getOrderStatsByRepairer: (params: { enterprise_id?: string; start?: string; end?: string }) =>
    client.get<RepairerStats>('/admin/orders/stats/by-repairer', { params }),

  // 5.20 区间运营指标（店方全域/审核员限本单位；企业数据卡指标行）
  getOrderStatsMetrics: (params: { enterprise_id?: string; start?: string; end?: string }) =>
    client.get<OrderStatsMetrics>('/admin/orders/stats/metrics', { params }),

  // 5.21 维修员区间汇总（仅店方/超管；业务员统计卡：全局待接单 + 按接单/完工时间戳）
  getOrderStatsRepairerSummary: (params: { repairer_id?: string; start?: string; end?: string }) =>
    client.get<RepairerSummary>('/admin/orders/stats/repairer-summary', { params }),

  // 5.3 审核通过（reported → pending_accept；店方或本单位审核员）
  auditOrder: (orderId: string, remark?: string) =>
    client.post(`/admin/orders/${orderId}/audit`, { remark: remark ?? '' }),

  // 5.4 接单维修（仅店方）
  acceptOrder: (orderId: string, remark?: string) =>
    client.post(`/admin/orders/${orderId}/accept`, { remark: remark ?? '' }),

  // 5.5 退回工单（reported：审核员/店方；pending_accept/processing：仅店方）
  rejectOrder: (orderId: string, reason: string) =>
    client.post(`/admin/orders/${orderId}/reject`, { reason }),

  // 5.6 完工（仅店方，必填对账字段）
  completeOrder: (
    orderId: string,
    data: {
      remark: string
      receipts: string[]
      quantity: number
      unit_price: number
      repair_content: string
      metadata: OrderMetadata
    }
  ) => client.post(`/admin/orders/${orderId}/complete`, data),

  // 5.6.1 修改对账信息（仅 completed 状态店方，字段至少传一个）
  updateFinance: (
    orderId: string,
    data: {
      quantity?: number
      unit_price?: number
      repair_content?: string
      metadata?: OrderMetadata
    }
  ) => client.post(`/admin/orders/${orderId}/finance`, data),

  // 5.16 重新打开（completed → processing，仅店方）
  reopenOrder: (orderId: string, remark?: string) =>
    client.post(`/admin/orders/${orderId}/reopen`, { remark: remark ?? '' }),

  // 5.14 导出工单记录（返回 Excel 文件流，仅店方）
  exportOrders: (params: {
    mode: 'enterprise' | 'repairer'
    enterprise_id?: string
    repairer_id?: string
    date_from: string
    date_to: string
    fields?: string[]
    status?: string
  }) => client.get('/admin/orders/export', { params, responseType: 'blob' }),

  // 5.15 维修员(业务员)列表（导出弹窗下拉使用，仅店方）
  getRepairers: () =>
    client.get<{ list: Array<{ id: string; nickname: string; avatar_url: string }> }>('/admin/repairers'),

  // 5.7 上传收据图片（multipart/form-data）
  uploadReceipt: (orderId: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return client.post<{ id: string; url: string; file_size: number }>(
      `/admin/orders/${orderId}/receipts`,
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    )
  },

  // ── 项目字典（6.5，仅超管） ──

  // 6.5.1 大类树（含属性/常见问题子项）
  getCategories: () =>
    client.get<{ list: DictionaryCategoryTree[] }>('/admin/categories'),

  createCategory: (data: {
    name: string
    description?: string
    sort_order?: number
  }) => client.post<DictionaryCategory>('/admin/categories', data),

  updateCategory: (
    categoryId: string,
    data: Partial<{ name: string; description: string; sort_order: number }>
  ) => client.put<DictionaryCategory>(`/admin/categories/${categoryId}`, data),

  deleteCategory: (categoryId: string) =>
    client.delete(`/admin/categories/${categoryId}`),

  // 6.5.2 属性（category_id 必传）
  getProperties: (categoryId: string) =>
    client.get<{ list: DictionaryProperty[] }>('/admin/properties', {
      params: { category_id: categoryId }
    }),

  createProperty: (data: {
    category_id: string
    name: string
    description?: string
    sort_order?: number
  }) => client.post<DictionaryProperty>('/admin/properties', data),

  updateProperty: (
    propertyId: string,
    data: Partial<{ name: string; description: string; sort_order: number }>
  ) => client.put<DictionaryProperty>(`/admin/properties/${propertyId}`, data),

  deleteProperty: (propertyId: string) =>
    client.delete(`/admin/properties/${propertyId}`),

  // 6.5.3 常见问题（property_id 筛选/新增，问题隶属属性）
  getProblems: (propertyId: string) =>
    client.get<{ list: DictionaryProblem[] }>('/admin/problems', {
      params: { property_id: propertyId }
    }),

  createProblem: (data: {
    property_id: string
    name: string
    description?: string
    sort_order?: number
  }) => client.post<DictionaryProblem>('/admin/problems', data),

  updateProblem: (
    problemId: string,
    data: Partial<{
      name: string
      description: string
      sort_order: number
    }>
  ) => client.put<DictionaryProblem>(`/admin/problems/${problemId}`, data),

  deleteProblem: (problemId: string) =>
    client.delete(`/admin/problems/${problemId}`),

  // ── 企业管理 ──

  // 5.8 管理员企业列表（仅店方）
  getEnterprises: (params: EnterpriseListParams) =>
    client.get<PageResult<EnterpriseListItem>>('/admin/enterprises', { params }),

  // 5.9 管理员企业详情（仅店方）
  getEnterpriseDetail: (enterpriseId: string) =>
    client.get<EnterpriseDetail>(`/admin/enterprises/${enterpriseId}`),

  // 3.2 企业详情（单位视角；单位审核员访问本单位使用，含 my_role）
  getEnterpriseMine: (enterpriseId: string) =>
    client.get<EnterpriseMineDetail>(`/enterprises/${enterpriseId}`),

  getMembers: (enterpriseId: string, params: MemberListParams) =>
    client.get<PageResult<MemberItem>>(
      `/admin/enterprises/${enterpriseId}/members`,
      { params }
    ),

  // 3.5 成员列表（企业路由；店方/单位审核员均可，审核员限本单位）
  getEnterpriseMembers: (enterpriseId: string, params: MemberListParams) =>
    client.get<PageResult<MemberItem>>(`/enterprises/${enterpriseId}/members`, {
      params
    }),

  // 3.1 创建企业（仅店方）
  createEnterprise: (name: string) => client.post('/enterprises', { name }),

  // 3.3 更新企业设置（名称 / 免审核开关；店方或本单位审核员）
  updateEnterprise: (
    enterpriseId: string,
    data: { name?: string; auto_approve?: boolean }
  ) => client.put<EnterpriseDetail>(`/enterprises/${enterpriseId}`, data),

  // 3.9 刷新邀请码（店方或本单位审核员）
  refreshInviteCode: (enterpriseId: string, validity: string) =>
    client.post<{ invite_code: string; expires_at: string | null }>(
      `/enterprises/${enterpriseId}/refresh/code`,
      { validity }
    ),

  // 3.6 审核通过成员（店方或本单位审核员）
  approveMembers: (enterpriseId: string, userIds: string[]) =>
    client.put(`/enterprises/${enterpriseId}/members/approve`, {
      user_ids: userIds
    }),

  // 3.7 拒绝成员申请（店方或本单位审核员）
  rejectMembers: (enterpriseId: string, userIds: string[]) =>
    client.put(`/enterprises/${enterpriseId}/members/reject`, {
      user_ids: userIds
    }),

  // 3.8 移除成员（店方或本单位审核员；仅 approved）
  removeMember: (enterpriseId: string, userId: string) =>
    client.delete(`/enterprises/${enterpriseId}/members/${userId}`),

  // 3.10 设置成员审核员身份（仅店方；role: reviewer/member）
  setMemberRole: (
    enterpriseId: string,
    userId: string,
    role: 'reviewer' | 'member'
  ) =>
    client.put<{ updated: boolean }>(
      `/enterprises/${enterpriseId}/members/${userId}/role`,
      { role }
    ),

  // ── 第六章：用户管理（仅超管） ──

  // 6.1 用户列表
  getUsers: (params: {
    page?: number
    page_size?: number
    keyword?: string
    role?: number
  }) => client.get<PageResult<UserListItem>>('/admin/users', { params }),

  // 6.2 用户详情
  getUserDetail: (userId: string) =>
    client.get<UserDetail>(`/admin/users/${userId}`),

  // 6.3 更新用户属性
  updateUser: (
    userId: string,
    data: { nickname?: string; role?: number; phone?: string }
  ) => client.put(`/admin/users/${userId}`, data),

  // 6.4 设置/重置密码（超管；维修业务员 role>=1 或单位审核员 role=0+membership.role=1）
  resetPassword: (userId: string, newPassword: string) =>
    client.post(`/admin/users/${userId}/reset-password`, {
      new_password: newPassword
    })
}

/** 报修选项（4.1 /orders/options；管理端用于大类下拉等） */
export function fetchOrderOptions() {
  return client.get<OrderOptions>('/orders/options')
}
