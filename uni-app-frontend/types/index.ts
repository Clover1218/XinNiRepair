/** 全局角色：0=普通用户 / 1=维修业务员(店方) / 2=超级管理员（V1.2 双层模型） */
export type PlatformRole = 0 | 1 | 2

/** 用户在单位维度下的简要信息（/auth/me、登录响应） */
export interface EnterpriseBrief {
  enterprise_id: string
  enterprise_name: string
  /** 兼容旧值 'admin'（等价 reviewer），见接口文档 3.5 */
  role: 'member' | 'reviewer' | 'admin'
  status: 'pending' | 'approved' | 'rejected' | 'removed'
}

export interface UserInfo {
  id: string
  nickname: string
  avatar_url: string
  phone: string
  role: PlatformRole
  enterprises: EnterpriseBrief[]
}

export interface LoginResult {
  access_token: string
  expires_in?: number
  user: UserInfo | null
  /** true=需先完善资料(新用户), 调 /auth/register 后获取完整登录结果 */
  need_profile?: boolean
}

/** 工单状态（V1.2） */
export type OrderStatus =
  | 'draft'
  | 'reported'
  | 'pending_accept'
  | 'processing'
  | 'rejected'
  | 'completed'
  | 'cancelled'

/* ==================== 报修选项（GET /orders/options，三级字典树） ==================== */

export interface ProblemOption {
  id: string
  name: string
  description?: string
}

export interface PropertyOption {
  id: string
  name: string
  description?: string
  sort_order?: number
  problems?: ProblemOption[]
}

export interface CategoryOption {
  id: string
  name: string
  description?: string
  sort_order?: number
  properties: PropertyOption[]
}

export interface OptionsResult {
  categories: CategoryOption[]
  enterprises: { id: string; name: string }[]
  urgent_levels: { value: string; label: string }[]
}

/* ==================== 工单 ==================== */

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

/** 用户端工单列表项（GET /orders） */
export interface OrderListItem {
  id: string
  order_no: string
  category_id?: string
  category_name?: string
  property_id?: string
  property_name?: string
  description: string
  enterprise_id?: string
  enterprise_name?: string
  urgency: string
  urgency_label?: string
  status: OrderStatus
  status_label?: string
  created_at: string
  submitted_at: string | null
  /** 完成工单金额（列表项后端确认后返回；V1.2 需确认 #4） */
  amount?: number
  /** 退回原因（被退回草稿；V1.2 需确认 #5） */
  reject_reason?: string | null
  /* ── V1.3 统一工单卡片所需字段（后端补充，见开发文档 V1.3 5.4） ── */
  /** 报修位置/房间号 */
  room?: string
  /** 联系人及电话（"王五 12345678910"，待后端拆分为 contact_name/contact_phone） */
  contact?: string
  /** 故障图（用于卡片缩略图预览） */
  images?: OrderImage[]
}

/** 完工对账 metadata */
export interface OrderMetadata {
  repair_result?: string
  repair_method?: string
  warranty_period?: string
  repair_duration?: number
  extra_remark?: string
}

export interface OrderDetail {
  id: string
  order_no: string
  enterprise_id?: string
  enterprise_name?: string
  category_id?: string
  category_name?: string
  property_id?: string
  property_name?: string
  description: string
  urgency: string
  urgency_label?: string
  room: string
  contact: string
  status: OrderStatus
  status_label?: string
  reject_reason: string | null
  /** 报修人（提交账户）摘要 */
  reporter?: { id: string; nickname: string; avatar_url: string }
  images: OrderImage[]
  receipts: OrderImage[]
  timeline: TimelineItem[]
  /** 完工对账（completed 后有值） */
  quantity?: number
  unit_price?: number
  amount?: number
  repair_content?: string
  metadata?: OrderMetadata | null
  auditor_name?: string
  repairer_name?: string
  available_actions: AvailableAction[]
  created_at: string
  submitted_at: string | null
  updated_at: string
}

/* ==================== 分页 ==================== */

/** 扁平分页结构（接口文档 1.2 / 管理端 5.x） */
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

/** 嵌套 pagination 结构（接口文档 4.6 用户端列表） */
export interface Pagination { total: number; page: number; page_size: number; total_pages: number }
export interface PaginationResult<T> { list: T[]; pagination: Pagination }

/* ==================== 管理端：工单 ==================== */

/** 管理端可用动作（V1.2：删除 review，新增 audit/reopen/update_finance） */
export type AvailableActionKey =
  | 'audit'
  | 'accept'
  | 'complete'
  | 'reject'
  | 'reopen'
  | 'update_finance'

export interface AvailableAction {
  action: AvailableActionKey
  label: string
  to_status: OrderStatus | 'draft'
  require_reason?: boolean
  reason_min_length?: number
  confirm_message?: string
}

/** 管理端工单列表项（GET /admin/orders） */
export interface AdminOrderListItem {
  id: string
  order_no: string
  reporter: { id: string; nickname: string; avatar_url: string }
  enterprise_id?: string
  enterprise_name?: string
  category_id?: string
  category_name?: string
  property_id?: string
  property_name?: string
  description: string
  urgency: string
  urgency_label?: string
  status: OrderStatus
  status_label?: string
  image_count: number
  submitted_at: string | null
  created_at: string
  /* ── V1.3 卡片所需字段（后端补充，见开发文档 V1.3 第五章） ── */
  /** 报修位置/房间号 */
  room?: string
  /** 联系人及电话（待后端拆分为 contact_name/contact_phone） */
  contact?: string
  /** 故障图（用于卡片缩略图预览） */
  images?: OrderImage[]
  /** 当前用户对该工单的可执行操作（后端按状态生成，前端按角色裁剪后渲染按钮） */
  available_actions?: AvailableAction[]
}

/** 维修员个人汇总（5.22 GET /admin/orders/stats/repairer-overview；处理工单页顶部统计卡） */
export interface RepairerOverview {
  /** 可接单存量（待接单；随企业筛选） */
  pending_accept: number
  /** 我的累计接单 */
  my_accepted: number
  /** 处理中（接单人为我） */
  my_processing: number
  /** 累计完工 */
  my_completed: number
  /** 今日接单 */
  today_accepted: number
  /** 今日完工 */
  today_completed: number
  updated_at: string
}

/** 管理端工单详情（GET /admin/orders/{id}） */
export interface AdminOrderDetail extends OrderDetail {
  reporter?: { id: string; nickname: string; avatar_url: string }
  auditor_id?: string
  repairer_id?: string
  audited_at?: string | null
  accepted_at?: string | null
  completed_at?: string | null
  available_actions: AvailableAction[]
}

/* ==================== 企业 / 成员 ==================== */

/** 店方企业列表项（GET /admin/enterprises） */
export interface AdminEnterpriseItem {
  id: string
  name: string
  member_count: number
  order_count: number
  pending_count: number
  status: 'active' | 'inactive'
  created_at: string
}

/** 企业详情（GET /enterprises/{id} 或 GET /admin/enterprises/{id}） */
export interface EnterpriseDetail {
  id: string
  name: string
  invite_code: string
  invite_code_expires_at: string | null
  auto_approve?: boolean
  member_count?: number
  order_count?: number
  status?: string
  /** 当前用户在该单位的身份（member/reviewer），成员端接口返回 */
  my_role?: string
  created_at: string
}

/** 成员列表项（GET /enterprises/{id}/members，V1.2 增加 role） */
export interface MemberItem {
  membership_id: string
  user_id: string
  nickname: string
  avatar_url: string
  phone: string
  role: 'member' | 'reviewer' | 'admin'
  role_label: string
  status: 'pending' | 'approved' | 'rejected' | 'removed'
  status_label: string
  order_count: number
  joined_at: string | null
}

/* ==================== 上传 ==================== */

/** 图片上传返回（故障图/收据/头像） */
export interface UploadResult {
  id: string
  url: string
  file_size?: number
  sort_order?: number
}

/* ==================== 工单汇总统计（GET /admin/orders/stats，V1.3 C20） ==================== */

/** 状态分布项 */
export interface OrderStatusStat {
  status: string
  label: string
  count: number
}

/** 项目大类分布项 */
export interface OrderCategoryStat {
  category_id: string | null
  category_name: string
  count: number
}

/** 报修人排行项（按提交账号昵称） */
export interface OrderReporterStat {
  user_id: string
  nickname: string
  avatar_url: string | null
  count: number
}

/** 今日概况（独立于时间范围；服务器本地日 0点~次日0点） */
export interface OrderStatsToday {
  pending_review: number
  submitted_today: number
  audited_today: number
  rejected_today: number
}

/** 实际生效时间范围（start/end 为 null 表示不限） */
export interface OrderStatsRange {
  start: string | null
  end: string | null
}

/** 工单汇总统计响应 */
export interface OrderStats {
  today: OrderStatsToday
  total: number
  range: OrderStatsRange
  by_status: OrderStatusStat[]
  by_category: OrderCategoryStat[]
  top_reporters: OrderReporterStat[]
  updated_at: string
}

