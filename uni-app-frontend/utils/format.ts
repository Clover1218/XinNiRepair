/** 将 ISO 时间格式化为 "yyyy-MM-dd HH:mm" */
export function formatDateTime(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 将 ISO 时间格式化为 "yyyy-MM-dd" */
export function formatDate(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 手机号脱敏：138****8000 */
export function maskPhone(phone?: string): string {
  if (!phone || phone.length < 7) return phone || ''
  return `${phone.slice(0, 3)}****${phone.slice(-4)}`
}

/** 金额展示：保留两位小数，空值返回空串 */
export function formatAmount(v?: number | string | null): string {
  if (v === null || v === undefined || v === '') return ''
  const n = Number(v)
  if (Number.isNaN(n)) return ''
  return n.toFixed(2)
}

/** 工单状态中文（与《数据库字段设计文档 V1.4》一致；V1.2 移除 reviewed，新增 pending_accept） */
export const STATUS_LABELS: Record<string, string> = {
  draft: '草稿',
  reported: '已上报',
  pending_accept: '待接单',
  processing: '处理中',
  completed: '已处理',
  cancelled: '已取消'
}

/** 时间轴/日志动作中文（V1.2：含 audit/reopen/update_finance；review 兼容旧值展示为审核通过） */
export const ACTION_LABELS: Record<string, string> = {
  create_draft: '创建草稿',
  submit: '提交报修',
  audit: '审核通过',
  review: '审核通过',
  accept: '接单维修',
  complete: '完工',
  reopen: '重新打开',
  reject: '退回',
  cancel: '取消',
  upload_receipt: '上传收据',
  update_finance: '修改对账信息'
}

/** 紧急程度中文 */
export const URGENCY_LABELS: Record<string, string> = {
  normal: '普通',
  urgent: '紧急',
  very_urgent: '非常紧急'
}

/** 成员角色中文（reviewer=单位审核员；admin 旧值兼容=审核员） */
export const ROLE_LABELS: Record<string, string> = {
  reviewer: '单位审核员',
  admin: '单位审核员',
  member: '普通成员'
}

/** 成员状态中文 */
export const MEMBER_STATUS_LABELS: Record<string, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
  removed: '已移除'
}

export function statusLabel(s?: string): string {
  return (s && STATUS_LABELS[s]) || s || ''
}

/** 时间轴动作中文（后端已给 action_label 时直接用；此处做本地兜底映射） */
export function timeAxisActionLabel(action?: string): string {
  return (action && ACTION_LABELS[action]) || action || ''
}

export function urgencyLabel(s?: string): string {
  return (s && URGENCY_LABELS[s]) || s || ''
}

export function roleLabel(s?: string): string {
  return (s && ROLE_LABELS[s]) || s || ''
}

export function memberStatusLabel(s?: string): string {
  return (s && MEMBER_STATUS_LABELS[s]) || s || ''
}

/** 成员状态 -> wd-tag 类型 */
export function memberStatusTagType(
  s?: string
): 'default' | 'primary' | 'success' | 'warning' | 'danger' {
  switch (s) {
    case 'pending':
      return 'warning'
    case 'approved':
      return 'success'
    case 'rejected':
      return 'danger'
    case 'removed':
      return 'default'
    default:
      return 'default'
  }
}

/** 企业状态中文 */
export const ENTERPRISE_STATUS_LABELS: Record<string, string> = {
  active: '活跃',
  inactive: '已禁用'
}

export function enterpriseStatusLabel(s?: string): string {
  return (s && ENTERPRISE_STATUS_LABELS[s]) || s || ''
}

/** 企业状态 -> wd-tag 类型 */
export function enterpriseStatusTagType(
  s?: string
): 'default' | 'primary' | 'success' | 'warning' | 'danger' {
  return s === 'active' ? 'success' : 'default'
}

/** 工单状态 -> wd-tag 类型 */
export function statusTagType(s?: string): 'default' | 'primary' | 'success' | 'warning' | 'danger' {
  switch (s) {
    case 'draft':
      return 'default'
    case 'reported':
      return 'warning'
    case 'pending_accept':
      return 'primary'
    case 'processing':
      return 'primary'
    case 'completed':
      return 'success'
    case 'cancelled':
      return 'default'
    default:
      return 'default'
  }
}

/** 紧急程度 -> wd-tag 类型 */
export function urgencyTagType(s?: string): 'default' | 'primary' | 'success' | 'warning' | 'danger' {
  switch (s) {
    case 'normal':
      return 'default'
    case 'urgent':
      return 'warning'
    case 'very_urgent':
      return 'danger'
    default:
      return 'default'
  }
}

/**
 * 分页响应归一化：兼容两种后端结构
 *  - 扁平：{ list, total, page, page_size, total_pages }
 *  - 嵌套：{ list, pagination: { total, page, page_size, total_pages } }
 */
export function normalizePage<T = unknown>(raw: unknown): {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
} {
  const data = (raw || {}) as Record<string, unknown>
  const pag = (data.pagination || data) as Record<string, unknown>
  return {
    list: ((data.list as T[]) || []) as T[],
    total: Number(pag.total ?? 0) || 0,
    page: Number(pag.page ?? 1) || 1,
    page_size: Number(pag.page_size ?? 20) || 20,
    total_pages: Number(pag.total_pages ?? 1) || 1
  }
}
