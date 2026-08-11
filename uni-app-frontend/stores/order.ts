import { defineStore } from 'pinia'
import { http } from '@/utils/request'
import type { OrderDetail, OrderListItem, PageResult } from '@/types'

interface OrderState {
  orderList: OrderListItem[]
  currentOrder: OrderDetail | null
}

export const useOrderStore = defineStore('order', {
  state: (): OrderState => ({
    orderList: [],
    currentOrder: null
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
    }
  }
})
