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

/** 工单状态中文 */
export const STATUS_LABELS: Record<string, string> = {
  draft: '草稿',
  reported: '已上报',
  reviewed: '已阅',
  processing: '处理中',
  completed: '已处理',
  cancelled: '已取消'
}

/** 紧急程度中文 */
export const URGENCY_LABELS: Record<string, string> = {
  normal: '普通',
  urgent: '紧急',
  very_urgent: '非常紧急'
}

/** 成员角色中文 */
export const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
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
    case 'reviewed':
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
