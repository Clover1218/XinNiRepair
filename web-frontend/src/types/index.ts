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
  common_solutions?: string[]
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
  common_solutions: string[]
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
