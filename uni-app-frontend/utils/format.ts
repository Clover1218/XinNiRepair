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

/**
 * 联系人拆分：后端当前为单字段 contact（"张三 13800001234"）。
 * V1.3 需后端拆分为 contact_name / contact_phone（见开发文档第十一章 #1），
 * 过渡期由前端按“尾部 11 位手机号”解析；解析失败则整体视为联系人。
 */
export function splitContact(contact?: string | null): { name: string; phone: string } {
  const raw = (contact || '').trim()
  if (!raw) return { name: '', phone: '' }
  const m = raw.match(/^(.*?)[\s,，·、/-]*(1\d{10})$/)
  if (m) return { name: m[1].trim(), phone: m[2] }
  return { name: raw, phone: '' }
}

/** 联系人展示：张三 · 138****1234（无电话时仅显示姓名） */
export function contactDisplay(contact?: string | null, fallbackName?: string): string {
  const { name, phone } = splitContact(contact)
  const shownName = name || (fallbackName || '').trim()
  if (!shownName) return phone ? maskPhone(phone) : ''
  return phone ? `${shownName} · ${maskPhone(phone)}` : shownName
}

/** 相对时间：刚刚 / N分钟前 / N小时前 / 昨天 HH:mm / MM-DD / YYYY-MM-DD（V1.3 卡片右上角） */
export function relativeTime(value?: string | null): string {
  if (!value) return ''
  const t = new Date(value).getTime()
  if (Number.isNaN(t)) return value
  const diff = Math.floor((Date.now() - t) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`
  if (diff < 172800) return `昨天 ${hm}`
  if (d.getFullYear() === new Date().getFullYear()) return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 描述首行（卡片标题） */
export function descFirstLine(desc?: string | null): string {
  const lines = (desc || '').split(/\r?\n/).map((s) => s.trim()).filter(Boolean)
  return lines[0] || '（无描述）'
}

/** 描述次行（卡片副标题，无则返回空） */
export function descSecondLine(desc?: string | null): string {
  const lines = (desc || '').split(/\r?\n/).map((s) => s.trim()).filter(Boolean)
  return lines.slice(1).join(' ')
}

/* ==================== V1.3 状态分组 Tab 常量 ==================== */

export interface StatusGroupTab {
  value: string
  label: string
  /** 传给 GET /admin/orders 的 status（支持逗号多值） */
  status: string
}

/** 审核工单页：统计（C20/C21 起置首且页面默认，无 status 过滤串） / 待审核 / 处理中 / 结束 */
export const REVIEW_TABS: StatusGroupTab[] = [
  { value: 'stats', label: '统计', status: '' },
  { value: 'pending', label: '待审核', status: 'reported' },
  { value: 'processing', label: '处理中', status: 'pending_accept,processing' },
  { value: 'finished', label: '结束', status: 'completed,cancelled' }
]

/** 处理工单页：待接单 / 处理中 / 结束 */
export const REPAIR_TABS: StatusGroupTab[] = [
  { value: 'pending_accept', label: '待接单', status: 'pending_accept' },
  { value: 'processing', label: '处理中', status: 'processing' },
  { value: 'finished', label: '结束', status: 'completed,cancelled' }
]

/** 金额展示：保留两位小数，空值返回空串 */
export function formatAmount(v?: number | string | null): string {
  if (v === null || v === undefined || v === '') return ''
  const n = Number(v)
  if (Number.isNaN(n)) return ''
  return n.toFixed(2)
}

/** 工单状态中文（与《数据库字段设计文档 V1.4》一致；V1.2 移除 reviewed，新增 pending_accept；C19 新增 rejected=已退回） */
export const STATUS_LABELS: Record<string, string> = {
  draft: '草稿',
  reported: '已上报',
  pending_accept: '待接单',
  processing: '处理中',
  rejected: '已退回',
  completed: '已处理',
  cancelled: '已取消',
  pending_sign: '待补签'
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
    case 'rejected':
      return 'danger'
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
 * wd-tag 类型 -> 纯文字颜色。
 * 用于将胶囊形状态标签改为「纯文字 + 颜色」，避免胶囊样式视觉偏重。
 */
const TAG_TYPE_COLOR: Record<'default' | 'primary' | 'success' | 'warning' | 'danger', string> = {
  default: '#999999',
  primary: '#0f766e',
  success: '#07c160',
  warning: '#ff9f0a',
  danger: '#fa5151'
}

/**
 * 工单状态 -> 颜色（苍岭绿状态六色体系，与 Web 端 style.css 的 --st-* 一一对应）。
 *
 * ⚠️ 关键点：pending_accept(橙 #ea580c) 与 processing(蓝 #2563eb) 必须是**两个不同色**。
 * 旧实现走 wd-tag 的 `primary`，把这两者压成同一个蓝，扫读时无法分辨。
 * 注：红色 #dc2626 在体系里专指「已退回」，因此金额等正数值不得用红。
 */
const STATUS_COLOR: Record<string, string> = {
  draft: '#64748b',
  reported: '#f59e0b',
  pending_accept: '#ea580c',
  processing: '#2563eb',
  completed: '#0f766e',
  cancelled: '#64748b',
  rejected: '#dc2626'
}

/** 工单状态 -> 纯文字颜色 */
export function statusColor(s?: string): string {
  return (s && STATUS_COLOR[s]) || '#64748b'
}

/**
 * 工单状态 -> 胶囊 class（与 Web 端 OrderCardC.vue / detail.vue 的 .st-* 同名，
 * 便于两端 CSS 复用）。
 * 需配合在页面/组件内定义 .st-amber / .st-orange / .st-blue / .st-teal /
 * .st-red / .st-gray / .st-pending-sign 七组样式（底色即 STATUS_COLOR）。
 */
export function statusPillClass(s?: string): string {
  switch (s) {
    case 'reported':
      return 'st-amber'
    case 'pending_accept':
      return 'st-orange'
    case 'processing':
      return 'st-blue'
    case 'completed':
      return 'st-teal'
    case 'rejected':
      return 'st-red'
    default:
      return 'st-gray' // draft / cancelled
  }
}

/**
 * 状态胶囊「文案 + class」，与 Web 端 `statusView()` 逻辑一致：
 * 1) signature_status === 'pending_sign' 优先，文案固定「待补签」，用渐变胶囊；
 * 2) 否则取后端派生标签 display_tag，回退 status_label。
 */
export function statusPill(row?: {
  status?: string
  status_label?: string
  display_tag?: string
  signature_status?: string
}): { text: string; cls: string } {
  if (!row) return { text: '未知', cls: 'st-gray' }
  if (row.signature_status === 'pending_sign' || row.status === 'pending_sign') {
    return { text: '待补签', cls: 'st-pending-sign' }
  }
  return {
    text: row.display_tag || row.status_label || statusLabel(row.status),
    cls: statusPillClass(row.status)
  }
}

/** 成员状态 -> 纯文字颜色 */
export function memberStatusColor(s?: string): string {
  return TAG_TYPE_COLOR[memberStatusTagType(s)]
}

/** 企业状态 -> 纯文字颜色 */
export function enterpriseStatusColor(s?: string): string {
  return TAG_TYPE_COLOR[enterpriseStatusTagType(s)]
}

/** 紧急程度 -> 纯文字颜色 */
export function urgencyColor(s?: string): string {
  return TAG_TYPE_COLOR[urgencyTagType(s)]
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
