<template>
  <view class="page">
    <!-- 工具栏 -->
    <view class="toolbar">
      <view class="toolbar-title">我的报修</view>
      <wd-button size="small" type="primary" round @click="createOrder">+ 新建</wd-button>
    </view>

    <!-- 状态 Tab -->
    <view class="tabs-wrap">
      <wd-tabs v-model="activeTab" @change="onTabChange">
        <wd-tab
          v-for="tab in tabs"
          :key="tab.value"
          :title="tab.label"
          :name="tab.value"
        ></wd-tab>
      </wd-tabs>
    </view>

    <!-- 工单列表 -->
    <view class="order-list">
      <view v-for="item in orderList" :key="item.id" class="order-card">
        <view class="card-header">
          <view class="card-title">{{ item.project_name || '未命名工单' }}</view>
          <wd-tag :type="statusTagType(item.status)" round>{{ item.status_label }}</wd-tag>
        </view>

        <view class="card-meta">
          <wd-tag :type="urgencyTagType(item.urgency)" plain round>{{
            item.urgency_label
          }}</wd-tag>
          <text class="card-time">{{ formatDateTime(item.created_at) }}</text>
        </view>

        <view v-if="item.order_no" class="card-no">单号：{{ item.order_no }}</view>

        <view class="card-actions">
          <wd-button v-if="item.status === 'draft'" size="small" plain round @click="editOrder(item.id)">
            编辑
          </wd-button>
          <wd-button size="small" type="primary" plain round @click="viewOrder(item.id)">
            详情
          </wd-button>
        </view>
      </view>

      <!-- 空状态 -->
      <view v-if="!loading && orderList.length === 0" class="empty">
        <view class="empty-icon">📋</view>
        <view class="empty-text">暂无工单</view>
        <view class="empty-tip">点击右上角「新建」发起报修</view>
      </view>

      <!-- 加载更多 -->
      <view v-if="orderList.length > 0" class="load-more">
        <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { formatDateTime, statusTagType, urgencyTagType } from '@/utils/format'
import type { OrderListItem, PageResult } from '@/types'

interface TabItem {
  label: string
  value: string
  status: string
}

export default defineComponent({
  setup() {
    return {
      statusTagType,
      urgencyTagType,
      formatDateTime
    }
  },
  data() {
    return {
      tabs: [
        { label: '全部', value: 'all', status: '' },
        { label: '草稿', value: 'draft', status: 'draft' },
        { label: '处理中', value: 'processing', status: 'reported,reviewed,processing' },
        { label: '已完成', value: 'done', status: 'completed,cancelled' }
      ] as TabItem[],
      activeTab: 'all',
      statusParam: '',
      orderList: [] as OrderListItem[],
      page: 1,
      pageSize: 10,
      totalPages: 1,
      loading: false,
      finished: false
    }
  },
  onShow() {
    // 未登录跳转登录页
    if (!uni.getStorageSync('token')) {
      uni.reLaunch({ url: '/pages/auth/login' })
      return
    }
    this.loadOrders(true)
  },
  onPullDownRefresh() {
    this.loadOrders(true).finally(() => {
      uni.stopPullDownRefresh()
    })
  },
  onReachBottom() {
    if (!this.finished && !this.loading) {
      this.loadOrders(false)
    }
  },
  methods: {
    async loadOrders(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: this.pageSize
        }
        if (this.statusParam) {
          params.status = this.statusParam
        }
        const data = await http.get<PageResult<OrderListItem>>('/orders', params)
        this.page = data.page || targetPage
        this.totalPages = data.total_pages || 1
        this.finished = this.page >= this.totalPages
        this.orderList = reset ? data.list : [...this.orderList, ...data.list]
      } catch (e) {
        // 错误信息已由请求封装统一 Toast
        console.error('加载工单失败', e)
      } finally {
        this.loading = false
      }
    },
    onTabChange(e: { name: string }) {
      const tab = this.tabs.find((t) => t.value === e.name)
      this.activeTab = e.name
      this.statusParam = tab ? tab.status : ''
      this.loadOrders(true)
    },
    /** 新建：创建空草稿后跳转编辑页 */
    createOrder() {
      uni.showLoading({ title: '创建中...' })
      http
        .post<{ order_id: string }>('/orders', {})
        .then((data) => {
          uni.hideLoading()
          uni.navigateTo({ url: `/pages/order/edit?id=${data.order_id}` })
        })
        .catch(() => uni.hideLoading())
    },
    editOrder(orderId: string) {
      uni.navigateTo({ url: `/pages/order/edit?id=${orderId}` })
    },
    viewOrder(orderId: string) {
      uni.navigateTo({ url: `/pages/order/detail?id=${orderId}` })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  background-color: #f5f6f8;
}

.toolbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  background-color: #ffffff;
  border-bottom: 1rpx solid #f0f0f0;

  .toolbar-title {
    font-size: 32rpx;
    font-weight: 600;
    color: #1a1a1a;
  }
}

.tabs-wrap {
  background-color: #ffffff;
  padding: 0 24rpx;
}

.order-list {
  padding: 20rpx 24rpx;
}

.order-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx;
  margin-bottom: 20rpx;
  box-shadow: 0 4rpx 16rpx rgba(0, 0, 0, 0.04);

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .card-title {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
      flex: 1;
      margin-right: 16rpx;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }
  }

  .card-meta {
    display: flex;
    align-items: center;
    margin-top: 16rpx;

    .card-time {
      margin-left: 16rpx;
      font-size: 24rpx;
      color: #999999;
    }
  }

  .card-no {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #bbbbbb;
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 24rpx;

    ::v-deep wd-button {
      margin-left: 16rpx;
    }
  }
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 160rpx;

  .empty-icon {
    font-size: 96rpx;
  }

  .empty-text {
    margin-top: 24rpx;
    font-size: 30rpx;
    color: #666666;
  }

  .empty-tip {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #aaaaaa;
  }
}

.load-more {
  padding: 24rpx 0 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: #bbbbbb;
}
</style>
