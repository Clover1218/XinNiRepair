import { defineStore } from 'pinia'

/**
 * 工单管理列表筛选/分页/排序会话态。
 * 解决「点进工单详情再返回，筛选与排序被重置」：页面卸载不丢状态，
 * 返回 /orders 时从本 store 恢复并自动按保存的条件重新拉取。
 */
export const useOrderListQueryStore = defineStore('orderListQuery', {
  state: () => ({
    page: 1,
    pageSize: 20,
    activeStatus: '',
    enterpriseId: '',
    categoryId: '',
    propertyId: '',
    reporterId: '',
    reporterName: '',
    keyword: '',
    urgency: '',
    dateRange: null as [string, string] | null,
    sortBy: 'submitted_at',
    sortOrder: 'desc' as 'asc' | 'desc',
    /** 首次进入默认全部条件；后续进入沿用上次条件 */
    touched: false
  }),
  getters: {
    defaultSort(state) {
      return {
        prop: state.sortBy || 'submitted_at',
        order: (state.sortOrder === 'asc' ? 'ascending' : 'descending') as 'ascending' | 'descending'
      }
    }
  },
  actions: {
    reset() {
      this.page = 1
      this.activeStatus = ''
      this.enterpriseId = ''
      this.categoryId = ''
      this.propertyId = ''
      this.reporterId = ''
      this.reporterName = ''
      this.keyword = ''
      this.urgency = ''
      this.dateRange = null
      this.sortBy = 'submitted_at'
      this.sortOrder = 'desc'
    }
  }
})
