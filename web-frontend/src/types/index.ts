/** 平台角色：0=普通用户 1=维修业务员 2=超级管理员 */
export type PlatformRole = 0 | 1 | 2

/** 单位内成员身份字符串（接口统一 reviewer/member，兼容旧值 admin） */
export type MemberRole = 'member' | 'reviewer'

/** 用户所属企业信息（登录响应 /auth/me） */
export interface EnterpriseMembership {
  enterprise_id: string
  enterprise_name: string
  role: MemberRole
  status: string
}

/** 当前登录用户 */
export interface UserInfo {
  id: string
  nickname: string
  avatar_url: string
  phone: string
  role: PlatformRole
  enterprises: EnterpriseMembership[]
}

/** 登录接口响应 */
export interface LoginResult {
  access_token: string
  expires_in: number
  user: UserInfo
}

/** 工单状态（V1.4 状态机，无 reviewed） */
export type OrderStatus =
  | 'draft'
  | 'reported'
  | 'pending_accept'
  | 'processing'
  | 'completed'
  | 'cancelled'
  | 'rejected'

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
  enterprise_id: string
  enterprise_name: string
  /** 项目大类 ID / 名称快照（V1.4 替代 project_name） */
  category_id: string
  category_name: string
  /** 项目属性 ID / 名称快照 */
  property_id: string
  property_name: string
  description: string
  urgency: Urgency
  urgency_label: string
  status: OrderStatus
  status_label: string
  image_count: number
  submitted_at: string
  created_at: string
  /** V1.4 列表展示：维修操作内容 / 金额（未完工为空/0） */
  repair_content?: string
  amount?: number
}

/** 报修人筛选下拉选项（GET /admin/orders/reporters，来源于历史工单报修人） */
export interface ReporterOption {
  id: string
  nickname: string
  avatar_url: string | null
}

/** 维修附加元数据（完工/对账使用） */
export interface OrderMetadata {
  repair_result?: string
  repair_method?: string
  warranty_period?: string
  extra_remark?: string
  repair_duration?: number
}

/** 对账信息（完工提交 / 修改对账共用） */
export interface FinanceInfo {
  quantity?: number
  unit_price?: number
  amount?: number
  repair_content?: string
  metadata?: OrderMetadata
}

/** 可执行动作（5.2 available_actions；store 全量，审核员由前端按角色裁剪） */
export interface AvailableAction {
  action: 'audit' | 'accept' | 'complete' | 'reject' | 'reopen' | 'update_finance'
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
  enterprise_id: string
  enterprise_name: string
  category_id: string
  category_name: string
  property_id: string
  property_name: string
  description: string
  urgency: Urgency
  urgency_label: string
  room: string
  contact: string
  status: OrderStatus
  status_label: string
  reject_reason: string | null
  repair_content: string
  quantity: number
  unit_price: number
  amount: number
  metadata?: OrderMetadata
  auditor_id?: string
  auditor_name?: string
  repairer_id?: string
  repairer_name?: string
  images: OrderImage[]
  receipts: OrderImage[]
  timeline: TimelineItem[]
  available_actions: AvailableAction[]
  created_at: string
  submitted_at: string
  audited_at?: string
  accepted_at?: string
  completed_at?: string
  updated_at: string
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

/** 企业详情（5.9 / 3.3，管理端返回 order_count/status） */
export interface EnterpriseDetail {
  id: string
  name: string
  invite_code: string
  invite_code_expires_at: string | null
  auto_approve: boolean
  member_count: number
  order_count?: number
  status?: 'active' | 'inactive'
  created_at: string
}

/** 企业详情（GET /enterprises/:id 单位视角：单位审核员使用，含 my_role） */
export interface EnterpriseMineDetail {
  id: string
  name: string
  invite_code: string
  invite_code_expires_at: string | null
  auto_approve: boolean
  member_count?: number
  my_role?: MemberRole
  created_at: string
}

/** 成员列表项（5.10 / 3.5） */
export interface MemberItem {
  membership_id: string
  user_id: string
  nickname: string
  avatar_url: string
  phone: string
  role: MemberRole
  role_label: string
  status: 'pending' | 'approved' | 'rejected' | 'removed'
  status_label: string
  order_count: number
  joined_at: string
}

/** 用户列表项（6.1） */
export interface UserListItem {
  id: string
  openid: string
  nickname: string
  avatar_url: string
  phone: string
  role: PlatformRole
  created_at: string
  updated_at: string
  enterprise_name: string | null
  member_role: number | null
  member_status: string | null
}

/** 用户详情（6.2） */
export interface UserMembership {
  id: string
  enterprise_id: string
  enterprise_name: string
  role: number
  status: string
  joined_at: string | null
  created_at: string
}

export interface UserDetail {
  id: string
  openid: string
  unionid: string
  nickname: string
  avatar_url: string
  phone: string
  role: number
  created_at: string
  updated_at: string
  memberships: UserMembership[]
}

// ───────────────────────────────────────────
// 项目字典 / 报修选项（V1.4 库表化；问题直接隶属属性）
// ───────────────────────────────────────────

/** 报修选项大类（4.1 /orders/options） */
export interface CategoryOption {
  id: string
  name: string
  description: string
  sort_order: number
  properties: PropertyOption[]
}

/** 报修选项常见问题 */
export interface ProblemOption {
  id: string
  name: string
  description: string
}

/** /orders/options 响应 */
export interface OrderOptions {
  categories: CategoryOption[]
  enterprises: Array<{ id: string; name: string }>
}

/** 字典管理：大类（6.5.1，超管） */
export interface DictionaryCategory {
  id: string
  name: string
  description: string
  sort_order: number
}

/** 字典管理：属性（6.5.2；树形接口携带其下常见问题） */
export interface DictionaryProperty {
  id: string
  category_id: string
  name: string
  description: string
  sort_order: number
  problems?: DictionaryProblem[]
}

/** 字典管理：常见问题（6.5.3，隶属项目属性） */
export interface DictionaryProblem {
  id: string
  property_id: string
  name: string
  description: string
  sort_order: number
}

/** 字典树（含子项，6.5 大类列表返回；问题随属性返回） */
export interface DictionaryCategoryTree extends DictionaryCategory {
  properties: DictionaryProperty[]
}

/** 报修选项属性（4.1，含其下常见问题） */
export interface PropertyOption {
  id: string
  name: string
  problems?: ProblemOption[]
}

/* ===== 工单汇总统计（5.17，V1.3 · 对应小程序 C20~C22 统计面板） ===== */

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

/** 报修人排行项 */
export interface OrderReporterStat {
  user_id: string
  nickname: string
  avatar_url: string | null
  count: number
}

/** 今日概况（不受时间范围影响，按服务器本地日） */
export interface OrderStatsToday {
  pending_review: number
  submitted_today: number
  audited_today: number
  rejected_today: number
}

/** 生效时间范围 */
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

/* ===== 独立统计页：企业对比 / 维修员业绩（5.18 / 5.19，仅店方/超管） ===== */

/** 企业维度分组聚合项 */
export interface EnterpriseStatsRow {
  enterprise_id: string
  enterprise_name: string
  total: number
  by_status: OrderStatusStat[]
}

/** 企业维度分组聚合响应（5.18 GET /admin/orders/stats/by-enterprise） */
export interface EnterpriseStats {
  range: OrderStatsRange
  list: EnterpriseStatsRow[]
  updated_at: string
}

/** 维修员业绩聚合项 */
export interface RepairerStatsRow {
  repairer_id: string
  repairer_name: string
  /** 接单/处理量（待接单/处理中/已完工） */
  assigned: number
  /** 完工量（completed_at 非空） */
  completed: number
}

/** 维修员业绩聚合响应（5.19 GET /admin/orders/stats/by-repairer） */
export interface RepairerStats {
  range: OrderStatsRange
  list: RepairerStatsRow[]
  updated_at: string
}

/* ===== 统计页二期「纵向双区卡」（5.20 / 5.21） ===== */

/** 区间运营指标响应（5.20 GET /admin/orders/stats/metrics） */
export interface OrderStatsMetrics {
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

/** 维修员区间汇总项（5.21） */
export interface RepairerSummaryRow {
  repairer_id: string
  repairer_name: string
  /** 区间接单（accepted_at∈窗口） */
  accepted: number
  /** 区间完工（completed_at∈窗口） */
  completed: number
}

/** 维修员区间汇总响应（5.21 GET /admin/orders/stats/repairer-summary） */
export interface RepairerSummary {
  /** 全局待接单存量（不受所选业务员影响） */
  global_pending_accept: number
  list: RepairerSummaryRow[]
  range: OrderStatsRange
  updated_at: string
}
