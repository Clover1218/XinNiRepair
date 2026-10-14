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

/**
 * 管理端可用动作
 * V1.2：audit/accept/complete/reject/reopen/update_finance
 * V1.5：新增 sign（补签正式完工）、defer_sign（稍后补签）
 */
export type AvailableActionKey =
  | 'audit'
  | 'accept'
  | 'complete'
  | 'reject'
  | 'reopen'
  | 'update_finance'
  | 'sign'
  | 'defer_sign'

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
  /* ── V1.4 / V1.5 后端派生字段 ── */
  /** 展示标签（后端 ComputeDisplayTag 派生，如「待补签」「已完工」） */
  display_tag?: string
  display_tag_type?: string
  /** 签字状态：已签字 / 待补签 */
  signature_status?: 'signed' | 'pending_sign' | string
  /** 是否已有收据图 */
  receipt_uploaded?: boolean
  /** 结单结果（complete / customer_cancelled / unrepairable） */
  finish_outcome?: string
  /** 完工金额（V1.4 起列表返回，用于卡片展示） */
  amount?: number
  /** 维修操作内容 */
  repair_content?: string
  /** 当前用户对该工单的可执行操作（后端按状态生成，前端按角色裁剪后渲染按钮） */
  available_actions?: AvailableAction[]
}

/** 管理端工单列表响应（含各状态数量角标） */
export interface AdminOrderListResult extends PageResult<AdminOrderListItem> {
  /**
   * 状态计数。key 为状态值（reported/pending_accept/processing/completed/cancelled），
   * 外加聚合键 'all'（全量）与 'unfinished'（未完成），
   * 以及派生键 'pending_sign'（待补签，按 signature_status 统计，非工单 status）。
   */
  status_counts: Record<string, number>
}

/** 管理端工单列表查询参数 */
export interface AdminOrderListParams {
  page?: number
  page_size?: number
  status?: string
  urgency?: string
  keyword?: string
  enterprise_id?: string
  category_id?: string
  property_id?: string
  date_from?: string
  date_to?: string
  reporter_id?: string
  sort_by?: string
  sort_order?: string
}

/** 报修人候选（GET /admin/orders/reporters） */
export interface ReporterOption {
  id: string
  nickname: string
  avatar_url?: string
}

/* ==================== 完工 / 补签 / 对账 入参 ==================== */

/** 结单结果三值 */
export type FinishResult = 'complete' | 'customer_cancelled' | 'unrepairable'

/** 完工登记入参 */
export interface CompleteOrderInput {
  remark?: string
  receipt_ids?: string[]
  quantity: number
  unit_price: number
  repair_content: string
  metadata?: OrderMetadata
  /** 结单结果，默认 complete */
  result?: FinishResult
  /** 签字模式：signed=已签字；pending_sign=稍后补签（保持 processing） */
  sign_mode?: 'signed' | 'pending_sign'
}

/** 补签正式完工入参 */
export interface SignOrderInput {
  remark?: string
  receipt_ids?: string[]
  quantity?: number
  unit_price?: number
  repair_content?: string
  metadata?: OrderMetadata
}

/** 修改对账入参 */
export interface UpdateFinanceInput {
  quantity?: number
  unit_price?: number
  repair_content?: string
  metadata?: OrderMetadata
  remark?: string
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
  auditor_name?: string
  repairer_id?: string
  repairer_name?: string
  audited_at?: string | null
  accepted_at?: string | null
  completed_at?: string | null
  available_actions: AvailableAction[]
  /* ── V1.5 签字与结单结果 ── */
  /** 签字状态：signed 已签字 / pending_sign 待补签 */
  signature_status?: 'signed' | 'pending_sign' | string
  /** 结单结果 */
  finish_outcome?: string
  /** 展示标签（后端派生） */
  display_tag?: string
  /** 备注 */
  remark?: string
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

/** 区间运营指标响应（GET /admin/orders/stats/metrics，5.20） */
export interface OrderMetrics {
  /** 当前待审核存量（reported，不计时间） */
  pending_review: number
  /** 区间上报 */
  submitted: number
  /** 区间审核通过 */
  audited: number
  /** 区间退回 */
  rejected: number
  range: OrderStatsRange
  updated_at: string
}

/** 维修员个人汇总响应（GET /admin/orders/stats/repairer-overview，5.22） */
export interface RepairerOverview {
  /** 可接单存量 */
  pending_accept: number
  /** 我的累计接单 */
  my_accepted: number
  /** 处理中 */
  my_processing: number
  /** 累计完工 */
  my_completed: number
  /** 今日接单 */
  today_accepted: number
  /** 今日完工 */
  today_completed: number
  updated_at: string
}

/** 统计类接口公共查询参数 */
export interface StatsWindowParams {
  /** 单位审核员必填（限本单位）；店方可空=全域 */
  enterprise_id?: string
  /** 起始时间 YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss */
  start?: string
  /** 结束时间（纯日期按整天含当天处理） */
  end?: string
}

/* ==================== 首页待办看板（GET /dashboard） ==================== */

/** 看板卡片内的一条工单摘要 */
export interface DashboardOrder {
  id: string
  order_no: string
  description: string
  enterprise_name: string
  room: string
  reporter_name: string
  urgency: string
  urgency_label: string
  /** 后端派生的展示标签（如 待补签 / 处理中） */
  display_tag: string
  display_tag_type: string
  /** 原始状态值（completed / rejected …）：「提醒」卡据此决定落到哪个列表 Tab */
  status: string
  updated_at: string
  /** 有他人新动态且我尚未查看 → 显示小红点 */
  unread: boolean
}

/** 一张状态卡 */
export interface DashboardCard {
  /**
   * 稳定英文键（V1.7 角色化矩阵）：
   * pending_review / repairer_processing / pending_accept / in_progress / notice
   */
  key: string
  /** 卡片标题（按角色已由后端本地化） */
  label: string
  /** 跳工单列表时用的 display_tag 枚举键（空串 = 全部） */
  tag: string
  /** 该卡命中的工单总数 */
  total: number
  /** 其中未读工单数（卡片标题旁角标；仅「提醒」卡启用） */
  unread: number
  /** 最近 N 条工单 */
  orders: DashboardOrder[]
}

/** 首页看板响应 */
export interface DashboardResult {
  cards: DashboardCard[]
  unread_total: number
  updated_at: string
}

