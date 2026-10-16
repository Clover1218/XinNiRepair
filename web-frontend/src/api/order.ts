import client from './client'
import type { OrderListItem } from '@/types'

/**
 * 用户端工单列表响应（GET /orders）。
 * ⚠️ 与管理端 /admin/orders 的分页信封不同：总数在 pagination.total，
 * 且行内**没有** reporter / amount / repair_content / image_count 字段
 * （owner 隔离，全是本人单）——模板里访问这些字段必须判空。
 */
export interface UserOrderListResult {
  list: OrderListItem[]
  pagination: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
  /** 各状态未读工单数（红点提醒用），key 为状态值 + pending_sign */
  unread_counts?: Record<string, number>
}

/**
 * 用户端工单列表入参（GET /orders）。
 * 注意：用户端仅支持下列参数（后端 service/order.go List 明确解析），
 * 不支持 category/property/reporter/urgency/keyword/date 等管理端筛选。
 */
export interface UserOrderListParams {
  page?: number
  page_size?: number
  /** 企业精确筛选（用户端支持，但需先通过成员关系校验） */
  enterprise_id?: string
  /** display_tag 枚举键（英文；'' 表示不过滤） */
  display_tag?: string
  /** 排序：''（创建时间倒序）/ 'board'（状态优先级 + 更新时间倒序）；其余值后端拒绝 */
  sort?: string
}

/**
 * 用户端工单列表（4.6 GET /orders）。
 * 仅返回当前登录用户本人上报的工单（owner 隔离），不返回 status_counts。
 * 返回结构沿用 client 拦截器的信封约定：调用方取 res.data.list / res.data.pagination.total。
 */
export const orderAPI = {
  list: (params: UserOrderListParams) =>
    client.get<UserOrderListResult>('/orders', { params })
}
