/**
 * display_tag 枚举字典（中英文解耦的唯一数据源，镜像小程序 pages/order/tags.ts）
 *
 * 设计：前端向后端传「英文枚举键」(tag)，后端 MapDisplayTagToQuery 翻译成
 * status / signature_status / finish_outcome 等底层条件（绝不含中文判断）。
 * 中文 label / type 仅用于 UI 展示，由本文件集中维护。
 *
 * 与后端 service/order.go 的 displayTagFilterMap 一一对应（键名必须一致）。
 */

export type DisplayTagType = 'info' | 'warning' | 'primary' | 'success' | 'danger'

export interface DisplayTagMeta {
  /** 中文展示文案（Tab / 兜底用，非接口传参） */
  label: string
  /** 配色语义类型（对齐 status 胶囊的 cls） */
  type: DisplayTagType
}

/** 枚举键 → 中文标签 + 配色类型 */
export const DISPLAY_TAGS: Record<string, DisplayTagMeta> = {
  draft: { label: '草稿', type: 'info' },
  pending_review: { label: '待审核', type: 'danger' },
  pending_accept: { label: '待接单', type: 'warning' },
  in_progress: { label: '进行中', type: 'primary' },
  processing: { label: '处理中', type: 'primary' },
  pending_sign: { label: '待补签', type: 'warning' },
  finished: { label: '已完成', type: 'success' },
  ended: { label: '结束', type: 'info' },
  unrepairable: { label: '不能修', type: 'danger' },
  customer_cancelled: { label: '客户取消', type: 'info' },
  closed: { label: '已取消', type: 'info' },
  rejected: { label: '已退回', type: 'danger' }
}

/** 由枚举键取中文标签（未知键回退为原键） */
export function tagLabel(key: string): string {
  return DISPLAY_TAGS[key]?.label || key
}

/** 由枚举键取配色类型（未知键回退为 info） */
export function tagType(key: string): DisplayTagType {
  return DISPLAY_TAGS[key]?.type || 'info'
}

/** display_tag_type → 胶囊配色 class（Web 端：tag-${type}） */
export function tagCls(type: string | undefined): string {
  return `tag-${type || 'info'}`
}

/**
 * 计数聚合映射：display_tag 枚举键 → 组成该标签的底层 status 集合。
 * 与后端 displayTagFilterMap 语义对齐，仅用于前端 Tab 角标计数聚合
 * （不含 signature_status / finish_outcome 维度，当前 Tab 不涉及）。
 * 后端 status_counts 返回的键即为这些原始 status 值 + 独立的 `pending_sign`。
 */
export const TAG_STATUS_GROUPS: Record<string, string[]> = {
  draft: ['draft'],
  pending_review: ['reported'],
  pending_accept: ['pending_accept'],
  in_progress: ['reported', 'pending_accept', 'processing'],
  processing: ['processing'],
  pending_sign: ['pending_sign'],
  finished: ['completed'],
  ended: ['completed', 'cancelled'],
  unrepairable: ['cancelled'],
  customer_cancelled: ['cancelled'],
  closed: ['cancelled'],
  rejected: ['rejected']
}
