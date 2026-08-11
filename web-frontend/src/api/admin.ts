import client from './client'
import type {
  AvailableAction,
  EnterpriseDetail,
  EnterpriseListItem,
  MemberItem,
  OrderDetail,
  OrderListItem,
  OrderMetadata,
  PageResult
} from '@/types'

export interface OrderListParams {
  page?: number
  page_size?: number
  status?: string
  urgency?: string
  keyword?: string
  /** V1.1：企业精确筛选（下拉选择） */
  enterprise_id?: string
  /** V1.1：工单号模糊搜索（独立） */
  order_no?: string
  /** V1.1：项目名称模糊搜索（独立） */
  project_name?: string
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
  role?: string
  keyword?: string
}

export const adminAPI = {
  // 5.1 工单列表（管理后台）
  getOrders: (params: OrderListParams) =>
    client.get<PageResult<OrderListItem>>('/admin/orders', { params }),

  // 5.2 工单详情（管理后台）
  getOrderDetail: (orderId: string) =>
    client.get<OrderDetail & { available_actions: AvailableAction[] }>(
      `/admin/orders/${orderId}`
    ),

  // 5.3 查阅工单
  reviewOrder: (orderId: string, remark?: string) =>
    client.post(`/admin/orders/${orderId}/review`, { remark: remark ?? '' }),

  // 5.4 接单维修
  acceptOrder: (orderId: string, remark?: string) =>
    client.post(`/admin/orders/${orderId}/accept`, { remark: remark ?? '' }),

  // 5.5 退回工单
  rejectOrder: (orderId: string, reason: string) =>
    client.post(`/admin/orders/${orderId}/reject`, { reason }),

  // 5.6 完工（V1.1：新增必填对账字段）
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

  // V1.1 新增 5.6.1 修改对账信息（仅 completed 状态，字段至少传一个）
  updateFinance: (
    orderId: string,
    data: {
      quantity?: number
      unit_price?: number
      repair_content?: string
      metadata?: OrderMetadata
    }
  ) => client.post(`/admin/orders/${orderId}/finance`, data),

  // V1.1 新增 5.14 导出工单记录（返回 Excel 文件流）
  exportOrders: (params: {
    mode: 'enterprise' | 'repairer'
    enterprise_id?: string
    repairer_id?: string
    date_from: string
    date_to: string
    fields?: string[]
    status?: string
  }) => client.get('/admin/orders/export', { params, responseType: 'blob' }),

  // V1.1 新增 5.15 维修员(业务员)列表（导出弹窗下拉使用）
  getRepairers: () =>
    client.get<{ list: Array<{ id: string; nickname: string; avatar_url: string }> }>('/admin/repairers'),

  // 5.7 上传收据图片
  uploadReceipt: (orderId: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return client.post<{ id: string; url: string; file_size: number }>(
      `/admin/orders/${orderId}/receipts`,
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    )
  },

  // 5.8 管理员企业列表
  getEnterprises: (params: EnterpriseListParams) =>
    client.get<PageResult<EnterpriseListItem>>('/admin/enterprises', { params }),

  // 5.9 管理员企业详情
  getEnterpriseDetail: (enterpriseId: string) =>
    client.get<EnterpriseDetail>(`/admin/enterprises/${enterpriseId}`),

  // 5.10 管理员成员列表
  getMembers: (enterpriseId: string, params: MemberListParams) =>
    client.get<PageResult<MemberItem>>(
      `/admin/enterprises/${enterpriseId}/members`,
      { params }
    ),

  // 3.1 创建企业
  createEnterprise: (name: string) => client.post('/enterprises', { name }),

  // 3.9 刷新邀请码
  refreshInviteCode: (enterpriseId: string, validity: string) =>
    client.post<{ invite_code: string; expires_at: string | null }>(
      `/enterprises/${enterpriseId}/refresh/code`,
      { validity }
    ),

  // 3.6 审核通过成员
  approveMembers: (enterpriseId: string, userIds: string[]) =>
    client.put(`/enterprises/${enterpriseId}/members/approve`, {
      user_ids: userIds
    }),

  // 3.7 拒绝成员申请
  rejectMembers: (enterpriseId: string, userIds: string[]) =>
    client.put(`/enterprises/${enterpriseId}/members/reject`, {
      user_ids: userIds
    }),

  // 3.8 移除成员
  removeMember: (enterpriseId: string, userId: string) =>
    client.delete(`/enterprises/${enterpriseId}/members/${userId}`)
}
