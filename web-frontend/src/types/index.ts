/** 用户所属企业信息 */
export interface EnterpriseMembership {
  enterprise_id: string
  enterprise_name: string
  role: string
  status: string
}

/** 当前登录用户 */
export interface UserInfo {
  id: string
  nickname: string
  avatar_url: string
  phone: string
  enterprises: EnterpriseMembership[]
}

/** 登录接口响应 */
export interface LoginResult {
  access_token: string
  expires_in: number
  user: UserInfo
}

/** 工单状态 */
export type OrderStatus =
  | 'draft'
  | 'reported'
  | 'reviewed'
  | 'processing'
  | 'completed'
  | 'cancelled'

/** 紧急程度 */
export type Urgency = 'normal' | 'urgent' | 'very_urgent'

/** 图片信息 */
export interface OrderImage {
  id: string
  url: string
  sort_order: number
  file_size: number
}

/** 时间轴记录 */
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

/** 工单列表项（5.1） */
export interface OrderListItem {
  id: string
  order_no: string
  reporter: {
    id: string
    nickname: string
    avatar_url: string
  }
  /** V1.1 新增：企业 ID */
  enterprise_id: string
  /** V1.1 新增：企业名称 */
  enterprise_name: string
  project_name: string
  description: string
  urgency: Urgency
  urgency_label: string
  status: OrderStatus
  status_label: string
  image_count: number
  submitted_at: string
  created_at: string
}

/** V1.1 新增：维修附加元数据（完工/对账使用） */
export interface OrderMetadata {
  repair_result?: string // 维修结果，如：完全修复
  repair_method?: string // 维修方式，如：上门维修
  warranty_period?: string // 保修期，如：3个月
  extra_remark?: string // 额外备注
  repair_duration?: number // 维修时长（分钟）
}

/** V1.1 新增：对账信息（完工提交 / 修改对账共用） */
export interface FinanceInfo {
  quantity?: number // 数量
  unit_price?: number // 单价
  amount?: number // 金额（后端 GENERATED 列自动计算，只读展示）
  repair_content?: string // 具体维修操作内容
  metadata?: OrderMetadata
}

/** 可执行动作（5.2 available_actions） */
export interface AvailableAction {
  action: 'review' | 'accept' | 'complete' | 'reject' | 'update_finance'
  label: string
  to_status: string
  require_reason?: boolean
  reason_min_length?: number
  require_confirm?: boolean
  confirm_message?: string
}

/** 工单详情（5.2 / 4.7） */
export interface OrderDetail {
  id: string
  order_no: string
  /** 报修人信息（管理后台详情接口可能返回，4.7 用户端无此字段） */
  reporter?: {
    id: string
    nickname: string
    avatar_url: string
  }
  project_name: string
  category: string
  category_label: string
  property: string
  property_label: string
  description: string
  urgency: Urgency
  urgency_label: string
  room: string
  contact: string
  status: OrderStatus
  status_label: string
  reject_reason: string | null
  images: OrderImage[]
  receipts: OrderImage[]
  timeline: TimelineItem[]
  available_actions: AvailableAction[]
  created_at: string
  submitted_at: string
  updated_at: string
  reviewed_at?: string
  accepted_at?: string
  completed_at?: string
  /** V1.1 新增：对账信息（completed 状态展示，完工/修改对账提交） */
  quantity?: number
  unit_price?: number
  amount?: number
  repair_content?: string
  metadata?: OrderMetadata
  /** V1.1 新增：企业 ID / 企业名称 */
  enterprise_id?: string
  enterprise_name?: string
}

/** 分页响应 */
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

/** 企业列表项（5.8） */
export interface EnterpriseListItem {
  id: string
  name: string
  member_count: number
  order_count: number
  pending_count: number
  status: 'active' | 'inactive'
  created_at: string
}

/** 企业详情（5.9） */
export interface EnterpriseDetail {
  id: string
  name: string
  invite_code: string
  invite_code_expires_at: string | null
  member_count: number
  order_count: number
  status: 'active' | 'inactive'
  created_at: string
}

/** 成员列表项（5.10 / 3.5） */
export interface MemberItem {
  membership_id: string
  user_id: string
  nickname: string
  avatar_url: string
  phone: string
  role: string
  role_label?: string
  status: string
  status_label?: string
  order_count: number
  joined_at: string
}
