import { defineStore } from 'pinia'
import { http } from '@/utils/request'
import type { OrderDetail, OrderListItem, PageResult } from '@/types'

/** 跨 tabBar 页面的跳转上下文 */
export interface PendingJump {
  /** 目标状态 Tab 值（如 reported / pending_accept / processing；my_repair 下为 display_tag 枚举键） */
  status: string
  /** 目标企业 ID（审核员单位视角；空串 = 全部负责单位） */
  enterprise: string
  /**
   * 入口指定的页面 mode（my_repair / review / worker_task）。
   * 用于按角色多重身份时（如审核员同时是成员）精确进入对应视图，而非按角色兜底。
   */
  mode?: string
}

interface OrderState {
  orderList: OrderListItem[]
  currentOrder: OrderDetail | null
  /**
   * 「首页 → 工单 tab」的筛选上下文。
   *
   * ⚠️ uni-app 限制：tabBar 页面之间只能用 uni.switchTab 跳转，而 switchTab **不支持携带 URL 参数**。
   * Web 端那种 `router.push('/orders?status=xxx&enterprise=yyy')` 在这里行不通，
   * 只能借全局 store 传递，由工单 tab 在 onShow 里一次性消费。
   */
  pending: PendingJump | null
}

export const useOrderStore = defineStore('order', {
  state: (): OrderState => ({
    orderList: [],
    currentOrder: null,
    pending: null
  }),
  actions: {
    /** 我的工单列表（支持分页 / 状态筛选 / 企业筛选） */
    async fetchOrders(params?: Record<string, unknown>) {
      const data = await http.get<PageResult<OrderListItem>>('/orders', params)
      this.orderList = data.list || []
      return data
    },
    /** 工单详情 */
    async fetchOrderDetail(orderId: string) {
      const data = await http.get<OrderDetail>(`/orders/${orderId}`)
      this.currentOrder = data
      return data
    },

    /**
     * 记录待跳转的筛选上下文（首页待办区块「详情 ›」/ 快捷入口调用）。
     * @param status 目标状态 / Tab 值
     * @param enterprise 目标企业，空串表示不限单位
     * @param mode 入口指定的页面 mode（my_repair / review / worker_task）
     */
    setPendingJump(status: string, enterprise = '', mode?: string) {
      this.pending = { status, enterprise, mode }
    },

    /**
     * 工单 tab 在 onShow 中消费（一次性，读完即清，避免返回时重复套用）。
     */
    consumePendingJump(): PendingJump | null {
      const p = this.pending
      this.pending = null
      return p
    }
  }
})
