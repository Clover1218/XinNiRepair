/**
 * 工单页面「配置化驱动」核心：mode → 页面配置
 *
 * 三大角色入口各对应一个 mode：
 * - my_repair   ：普通用户 / 申报人（走 GET /orders，owner 隔离）
 * - review      ：单位审核员（走 GET /admin/orders，限定审核单位）
 * - worker_task ：维修业务员（走 GET /admin/orders，按经营范围收敛）
 *
 * 每个 mode 决定：Tab 维度（display_tag 枚举键）、是否显示「+新建」、调哪个数据源。
 * Tab 的 label 由各 mode 自定义（同一枚举键在不同 mode 下中文文案可不同，
 * 如 processing 在 worker_task 叫「处理中」）；精确条件映射全在后端。
 */

import { isStoreStaff, isUnitReviewer } from '@/utils/auth'

export type OrderMode = 'my_repair' | 'review' | 'worker_task'

/** 数据源：mine=用户端 /orders；admin=管理端 /admin/orders */
export type OrderSource = 'mine' | 'admin'

export interface ModeTab {
  /** 唯一标识（用于 activeTab） */
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

  // ── 审核员：统计 / 待审核 / 处理中 / 结束；不可新建 ──
  review: {
    showCreate: false,
    source: 'admin',
    tabs: [
      { key: 'stats', tag: '', label: '统计' },
      { key: 'pending_review', tag: 'pending_review', label: '待审核', sort: 'board' },
      { key: 'processing', tag: 'processing', label: '处理中', sort: 'board' },
      { key: 'ended', tag: 'ended', label: '结束' }
    ]
  },

  // ── 维修员：我的维修 / 待接单 / 处理中 / 待补签 / 结束；不可新建 ──
  // 「我的维修」= 我接的单（repairer_id=me，不限状态）；「待接单」= 业务范围内可接的单
  worker_task: {
    showCreate: false,
    source: 'admin',
    tabs: [
      { key: 'mine', tag: '', label: '我的维修', sort: 'board', repairerMe: true },
      { key: 'pending_accept', tag: 'pending_accept', label: '待接单', sort: 'board' },
      // 处理中 / 待补签 与首页「处理中」待办卡口径一致：都是「我接的单」，不是全量经营范围
      { key: 'processing', tag: 'processing', label: '处理中', sort: 'board', repairerMe: true },
      { key: 'pending_sign', tag: 'pending_sign', label: '待补签', repairerMe: true },
      { key: 'ended', tag: 'ended', label: '结束' }
    ]
  }
}

/**
 * 由角色推导默认 mode（无显式 mode 入参时的兜底）：
 * - 业务维修员(role=1) → worker_task
 * - 纯单位审核员(role=0 + reviewer) → review
 * - 普通用户 / 超级管理员(role=2) → my_repair（小程序端暂不做超管专属能力）
 * 兼任（staff + reviewer）默认 worker_task（首页待办入口会带各自 mode 精确进入）。
 */
export function defaultModeFromRole(): OrderMode {
  if (isStoreStaff()) return 'worker_task'
  if (isUnitReviewer()) return 'review'
  return 'my_repair'
}
