/** 企业在用户维度下的简要信息 */
export interface EnterpriseBrief {
  enterprise_id: string
  enterprise_name: string
  role: string
  status: string
}

export interface UserInfo {
  id: string
  nickname: string
  avatar_url: string
  phone: string
  /** 全局角色：0=普通用户，1=平台管理员 */
  role?: number
  enterprises: EnterpriseBrief[]
}

export interface LoginResult {
  access_token: string
  expires_in: number
  user: UserInfo | null
  /** true=需先完善资料(新用户), 调 /auth/register 后获取完整登录结果 */
  need_profile?: boolean
}

/** 工单列表项 */
export interface OrderListItem {
  id: string
  order_no: string
  project_name: string
  category: string
  category_label: string
  description: string
  enterprise_id?: string
  enterprise_name?: string
  urgency: string
  urgency_label: string
  status: string
  status_label: string
  created_at: string
  submitted_at: string | null
}

export interface OrderImage {
  id: string
  url: string
  sort_order: number
  file_size?: number
}

export interface TimelineItem {
  id: string
  action: string
  action_label: string
  operator_name: string
  operator_role?: string
  from_status: string | null
  to_status: string | null
  remark: string | null
  ip_address?: string
  created_at: string
}

/** 工单详情（用户端） */
export interface OrderDetail {
  id: string
  order_no: string
  project_name: string
  category: string
  category_label: string
  property: string
  property_label: string
  description: string
  urgency: string
  urgency_label: string
  room: string
  contact: string
  status: string
  status_label: string
  reject_reason: string | null
  enterprise_id?: string
  enterprise_name?: string
  images: OrderImage[]
  receipts: OrderImage[]
  timeline: TimelineItem[]
  available_actions: AvailableAction[]
  created_at: string
  submitted_at: string | null
  updated_at: string
}

export interface ProjectCategory {
  id: string
  name: string
  attributes: unknown[]
}

/** GET /orders/options 返回 */
export interface OptionsResult {
  project_categories: ProjectCategory[]
  properties: { value: string; label: string }[]
  common_issues: Record<string, string[]>
  urgent_levels: { value: string; label: string }[]
  enterprises: { id: string; name: string }[]
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

/* ==================== 管理员模块 ==================== */

/** 管理员 - 企业列表项（GET /admin/enterprises） */
export interface AdminEnterpriseItem {
  id: string
  name: string
  member_count: number
  order_count: number
  pending_count: number
  status: 'active' | 'inactive'
  created_at: string
}

/** 管理员 - 企业详情（GET /admin/enterprises/{id}） */
export interface AdminEnterpriseDetail {
  id: string
  name: string
  invite_code: string
  invite_code_expires_at: string | null
  member_count: number
  order_count: number
  status: string
  created_at: string
}

/** 管理员 - 成员列表项（GET /admin/enterprises/{id}/members） */
export interface AdminMemberItem {
  membership_id: string
  user_id: string
  nickname: string
  avatar_url: string
  phone: string
  role: string
  role_label: string
  status: string
  status_label: string
  order_count: number
  joined_at: string
}

/** 管理员 - 工单列表项（GET /admin/orders） */
export interface AdminOrderListItem {
  id: string
  order_no: string
  reporter: { id: string; nickname: string; avatar_url: string }
  enterprise_id?: string
  enterprise_name?: string
  project_name: string
  description: string
  urgency: string
  urgency_label: string
  status: string
  status_label: string
  image_count: number
  submitted_at: string | null
  created_at: string
}

/** 管理员 - 可用动作 */
export interface AvailableAction {
  action: 'review' | 'accept' | 'complete' | 'reject'
  label: string
  to_status: string
  require_reason?: boolean
  reason_min_length?: number
  require_confirm?: boolean
  confirm_message?: string
}

/** 管理员 - 工单详情（用户端详情 + available_actions） */
export interface AdminOrderDetail extends OrderDetail {
  enterprise_name?: string
  available_actions: AvailableAction[]
}

/** 收据上传返回（POST /admin/orders/{id}/receipts） */
export interface UploadResult {
  id: string
  url: string
  file_size: number
}
