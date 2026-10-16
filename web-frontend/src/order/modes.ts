/**
 * 工单列表「配置化驱动」核心：mode → 页面配置（Web 端，镜像小程序 pages/order/modes.ts）
 *
 * 三大角色入口各对应一个 mode：
 * - my_repair   ：普通用户 / 申报人（走 GET /orders，owner 隔离）
 * - review      ：单位审核员（走 GET /admin/orders，限定审核单位）
 * - worker_task ：维修业务员（走 GET /admin/orders，按经营范围收敛）
 *
 * 每个 mode 决定：Tab 维度（display_tag 枚举键）、是否显示「+新建」、调哪个数据源。
 * Tab 的 label 由各 mode 自定义（同一枚举键在不同 mode 下中文文案可不同，
 * 如 processing 在 worker_task 叫「处理中」）；精确条件映射全在后端。
 *
 * 注意：前端向后端一律传「英文枚举键」（tag），后端 MapDisplayTagToQuery 翻译，
 * 绝不含中文判断（详见 src/order/tags.ts 与后端 displayTagFilterMap）。
 */

/** 工单列表模式 */
export type OrderMode = 'my_repair' | 'review' | 'worker_task'

/** 数据源：mine=用户端 /orders；admin=管理端 /admin/orders */
export type OrderSource = 'mine' | 'admin'

export interface ModeTab {
  /** 唯一标识（用于 activeTabKey） */
  key: string
  /** 传给后端的 display_tag 枚举键；空串 '' 表示不过滤（如 my_repair 的「全部」） */
  tag: string
  /** Tab 展示文案 */
  label: string
  /**
   * 排序方式：'board' = 状态优先级 + 更新时间倒序（与首页看板完全一致）；
   * 未设置则用该数据源的默认排序（用户端创建时间倒序 / 管理端提交时间倒序）。
   */
  sort?: string
  /**
   * 仅维修业务员：该 Tab 只看「我接的单」（后端传 repairer_id=me）。
   * 服务入口「我的维修」用；接单大厅等 Tab 不设。
   */
  repairerMe?: boolean
}

export interface ModeConfig {
  /** 是否显示「+新建」按钮 */
  showCreate: boolean
  /** 数据源 */
  source: OrderSource
  /** 状态 Tab 列表 */
  tabs: ModeTab[]
}

export const ORDER_MODES: Record<OrderMode, ModeConfig> = {
  // ── 申报人：全部 / 进行中 / 待补签 / 草稿 / 已退回 / 已完成 / 已取消；可新建 ──
  // 「全部」「进行中」用 board 排序：与首页看板顺序一致（处理中 → 待补签 → 待接单 → 待审核 → 其它）
  my_repair: {
    showCreate: true,
    source: 'mine',
    tabs: [
      { key: 'all', tag: '', label: '全部', sort: 'board' },
      { key: 'in_progress', tag: 'in_progress', label: '进行中', sort: 'board' },
      { key: 'pending_sign', tag: 'pending_sign', label: '待补签' },
      { key: 'draft', tag: 'draft', label: '草稿' },
      { key: 'rejected', tag: 'rejected', label: '已退回' },
      { key: 'finished', tag: 'finished', label: '已完成' },
      { key: 'closed', tag: 'closed', label: '已取消' }
    ]
  },

  // ── 审核员：待审核 / 处理中 / 结束；不可新建（统计已下放到独立「统计」页）──
  review: {
    showCreate: false,
    source: 'admin',
    tabs: [
      { key: 'pending_review', tag: 'pending_review', label: '待审核', sort: 'board' },
      { key: 'processing', tag: 'processing', label: '处理中', sort: 'board' },
      { key: 'ended', tag: 'ended', label: '结束' }
    ]
  },

  // ── 维修员：我的维修 / 待接单 / 处理中 / 待补签 / 结束；不可新建 ──
  // 「我的维修」= 我接的单（repairer_id=me，不限状态）；「待接单」= 业务范围内可接的单（不带 repairerMe）
  // 「处理中 / 待补签 / 结束」= 我接的单的对应状态（repairer_id=me 口径，仅看本人，不展开全经营范围）
  worker_task: {
    showCreate: false,
    source: 'admin',
    tabs: [
      { key: 'mine', tag: '', label: '我的维修', sort: 'board', repairerMe: true },
      { key: 'pending_accept', tag: 'pending_accept', label: '待接单', sort: 'board' },
      // 处理中 / 待补签 与首页「处理中」待办卡口径一致：都是「我接的单」，不是全量经营范围
      { key: 'processing', tag: 'processing', label: '处理中', sort: 'board', repairerMe: true },
      { key: 'pending_sign', tag: 'pending_sign', label: '待补签', repairerMe: true },
      { key: 'ended', tag: 'ended', label: '结束', repairerMe: true }
    ]
  }
}

/**
 * 由角色推导默认 mode（无显式 mode 入参时的兜底）：
 * 优先级：审核员(review) > 维修业务员(worker_task) > 普通用户(my_repair)。
 * 与交接文档 2.6 一致（审核员优先），与小程序首页服务入口优先级一致。
 *
 * Web 后台仅单位审核员与业务维修员可进入（纯普通用户不进后台），故「普通用户」分支
 * 实际对应审核员/维修员查看「自己上报的工单」（经看板卡片 ?mode=my_repair 进入）。
 * 超管(isStoreStaff=true)复用 staff 视图（Web 后台定位，已与用户确认）。
 */
export function defaultModeFromRole(role: {
  isStoreStaff: boolean
  hasReviewerRole: boolean
}): OrderMode {
  if (role.hasReviewerRole) return 'review'
  if (role.isStoreStaff) return 'worker_task'
  return 'my_repair'
}

/**
 * 校验某个 mode 是否对当前用户可用（防止普通用户越权进入 review/worker_task）。
 * 不可用则回落到 my_repair。
 */
export function resolveMode(
  mode: string | undefined,
  role: { isStoreStaff: boolean; hasReviewerRole: boolean }
): OrderMode {
  if (mode === 'review') return role.hasReviewerRole ? 'review' : 'my_repair'
  if (mode === 'worker_task') return role.isStoreStaff ? 'worker_task' : 'my_repair'
  if (mode === 'my_repair') return 'my_repair'
  return defaultModeFromRole(role)
}

// ─────────────────────────────────────────────────────────
// 首页待办卡片 → 工单列表的 mode + tag 映射（与小程序 home/index.vue 一致）
// ─────────────────────────────────────────────────────────

/** 卡片 key → 列表 mode */
export function cardKeyToMode(key: string): OrderMode {
  if (key === 'pending_review') return 'review'
  if (key === 'repairer_processing' || key === 'pending_accept') return 'worker_task'
  return 'my_repair'
}

/** 卡片 key → 列表 Tab 的 tag（按状态聚合的多态卡落在「全部」） */
export function cardKeyToTag(key: string): string {
  switch (key) {
    case 'pending_review':
      return 'pending_review'
    case 'repairer_processing':
      return 'processing'
    case 'pending_accept':
      return 'pending_accept'
    default:
      return '' // my_repair 的「全部」Tab
  }
}

/**
 * 卡片内单条工单 → 目标 Tab tag。
 * 「提醒」卡混装已完工与已退回，按该工单自身 status 落到对应 Tab；其余沿用卡片本身。
 */
export function orderStatusToTag(key: string, status: string): string {
  if (key === 'notice') {
    return status === 'completed' ? 'finished' : 'rejected'
  }
  return cardKeyToTag(key)
}
