import client from './client'
import type {
  DictionaryCategory,
  DictionaryCategoryTree,
  DictionaryProblem,
  DictionaryProperty,
  DashboardCard,
  DashboardResult,
  EnterpriseDetail,
  EnterpriseListItem,
  EnterpriseMineDetail,
  EnterpriseStats,
  MemberItem,
  OrderDetail,
  OrderDraftResult,
  OrderListResult,
  OrderMetadata,
  OrderOptions,
  OrderStats,
  OrderStatsMetrics,
  PageResult,
  RepairerStats,
  RepairerSummary,
  ReporterOption,
  RepairerListItem,
  RepairerScope,
  ScopeRepairerItem,
  SubmitResult,
  UserDetail,
  UserListItem,
  SettingsResult
} from '@/types'

export interface OrderListParams {
  page?: number
  page_size?: number
  status?: string
  urgency?: string
  keyword?: string
  /** display_tag 枚举键（英文；'' 表示不过滤；与 status 二选一，优先级高于 status） */
  display_tag?: string
  /** 仅「我接的单」（后端只认字面量 'me'，不接受任意 UUID 防越权）；维修员「我的维修」Tab 用 */
  repairer_id?: string
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
  /** 本质标签筛选：映射到后端 finish_outcome（不能修 / 客户取消） */
  finish_outcome?: string
  /** 本质标签筛选：映射到后端 signature_status（待补签 / 已签字） */
  signature_status?: string
}

export interface EnterpriseListParams {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
}

/** 创建/更新工单草稿入参（4.2 / 4.3，字段均可选，服务端容忍空对象） */
export interface OrderDraftInput {
  enterprise_id?: string
  category_id?: string
  property_id?: string
  description?: string
  urgency?: string
  room?: string
  /** 联系人+电话，约定格式「姓名 空格 电话」 */
  contact?: string
  /** 完整图片 URL 列表（全量替换，旧版兼容；新代码统一用 image_ids） */
  images?: string[]
  /** 仅 update 生效：已上传图片的 id 列表；服务端做 diff（新→active、移除→deleted） */
  image_ids?: string[]
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
    client.get<OrderListResult>('/admin/orders', { params }),

  // 5.1 配套：报修人筛选下拉候选（按企业域+昵称模糊；来源于历史工单报修人）
  getReporters: (params: { enterprise_id?: string; keyword?: string }) =>
    client.get<{ list: ReporterOption[] }>('/admin/orders/reporters', { params }),

  // 5.2 工单详情（管理后台）
  getOrderDetail: (orderId: string) =>
    client.get<OrderDetail>(`/admin/orders/${orderId}`),

  // 首页待办看板（按状态聚合的 3-4 张卡 + 工单级未读；非时间流）
  getDashboard: (params?: { enterprise_id?: string }) =>
    client.get<DashboardResult>('/dashboard', { params }),

  // 标记工单已读：进详情页时调用，写入「该用户 × 该工单」的已读时间，首页红点消失
  markOrderRead: (orderId: string) => client.post(`/orders/${orderId}/read`),

  // 首页看板单卡全量（突破每卡预览上限；首页「查看全部」展开）
  getDashboardCard: (key: string, enterpriseId?: string) =>
    client.get<DashboardCard>('/dashboard/card-detail', {
      params: enterpriseId ? { key, enterprise_id: enterpriseId } : { key }
    }),

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

  // V1.9 退单回池（processing → pending_accept；仅接单人本人或超管，清财务字段+清旧收据图）
  returnOrder: (orderId: string, reason: string) =>
    client.post(`/admin/orders/${orderId}/return`, { reason }),

  // 5.6 完工登记（仅店方；req② 三结果：result=complete/customer_cancelled/unrepairable）
  // 当 result=complete 时，sign_mode=signed(已签字,必填收据) | pending_sign(稍后补签,保持处理中)
  completeOrder: (
    orderId: string,
    data: {
      remark: string
      receipt_ids: string[]
      quantity: number
      unit_price: number
      repair_content: string
      metadata: OrderMetadata
      result: string
      sign_mode?: string
    }
  ) => client.post(`/admin/orders/${orderId}/complete`, data),

  // req④ 补签正式完工（仅待补签工单 → completed + 写完工时间 + signature_status=signed）
  signOrder: (
    orderId: string,
    data: {
      remark?: string
      receipt_ids?: string[]
      quantity: number
      unit_price: number
      repair_content?: string
      metadata?: OrderMetadata
    }
  ) => client.post(`/admin/orders/${orderId}/signature`, data),

  // 5.6.1 修改对账信息（仅 completed 状态店方，字段至少传一个）
  updateFinance: (
    orderId: string,
    data: {
      quantity?: number
      unit_price?: number
      repair_content?: string
      metadata?: OrderMetadata
      remark?: string
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
    }),

  // ── V1.5 维修业务员经营范围 ──

  /**
   * 业务员列表（含已分配企业数 scope_count）。
   * 注意：该接口（5.15）返回体为 { list: [...] }，非裸数组，与新增的经营范围接口不同。
   */
  getRepairers: () =>
    client.get<{ list: RepairerListItem[] }>('/admin/repairers'),

  /** 某业务员的经营范围 */
  getRepairerScope: (repairerId: string) =>
    client.get<RepairerScope>(`/admin/repairers/${repairerId}/enterprises`),

  /**
   * 全量覆盖某业务员的经营范围（幂等；超管）。
   * 传空数组表示清空——该业务员随后将看不到任何企业的数据（已接工单除外）。
   */
  updateRepairerScope: (repairerId: string, enterpriseIds: string[]) =>
    client.put<RepairerScope>(
      `/admin/repairers/${repairerId}/enterprises`,
      { enterprise_ids: enterpriseIds }
    ),

  /** 服务该企业的业务员（反向查询） */
  getEnterpriseRepairers: (enterpriseId: string) =>
    client.get<ScopeRepairerItem[]>(`/admin/enterprises/${enterpriseId}/repairers`),

  /** 当前登录者自身的可见范围 */
  getMyScope: () => client.get<RepairerScope>('/admin/my/enterprises'),

  // ── 上报工单（4.2/4.3/4.4/4.5；与小程序同源，仅 JWTAuth，不限制平台角色） ──

  /**
   * 4.2 创建工单草稿。可携带全量内容一次性建单（无需先建空草稿）。
   * 注意：Create 阶段服务端会忽略 images 字段，图片必须拿到 order_id 后单独上传。
   */
  createOrder: (data: OrderDraftInput) =>
    client.post<OrderDraftResult>('/orders', data),

  /**
   * 4.3 更新草稿。唯一作用之一是把上传后处于 temporary 的图片转成 active，
   * 因此 images 传的是**完整列表**（全量替换），不是增量追加。
   */
  updateOrder: (orderId: string, data: OrderDraftInput) =>
    client.put<OrderDraftResult>(`/orders/${orderId}`, data),

  /** 4.4 提交上报（draft/rejected → reported，服务端此时生成工单号） */
  submitOrder: (orderId: string) =>
    client.post<SubmitResult>(`/orders/${orderId}/submit`, {}),

  /**
   * 4.5 上传故障图片（multipart/form-data，字段名 file）。
   * 前置：工单必须处于 draft/rejected，因此必须先拿到 order_id。
   * 限制：单张 ≤5MB，jpg/png/webp，同一工单 ≤9 张（服务端校验）。
   */
  uploadOrderImage: (orderId: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return client.post<{ id: string; url: string; sort_order: number; file_size: number }>(
      `/orders/${orderId}/images`,
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    )
  },

  /**
   * 4.5 预上传故障图片（multipart/form-data）。不要求 order_id：图片尚未绑定工单
   * （如新建单尚未落库）时调用，后端写入「未绑定哨兵」的 temporary 记录，
   * 提交建单/保存草稿时由 updateOrder(image_ids) 关联为 active。
   */
  uploadOrderImageFree: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return client.post<{ id: string; url: string; sort_order: number; file_size: number }>(
      `/orders/images`,
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    )
  },

  // ── 系统设置（仅超级管理员） ──

  // 列出全部可配置项
  getSettings: () => client.get<SettingsResult>('/admin/settings'),

  // 更新单个设置项（key + value）
  updateSetting: (key: string, value: string) =>
    client.put('/admin/settings', { key, value })
}

/** 报修选项（4.1 /orders/options；管理端用于大类下拉等） */
export function fetchOrderOptions() {
  return client.get<OrderOptions>('/orders/options')
}
