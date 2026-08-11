<template>
  <view class="page">
    <!-- 工具栏 -->
    <view class="toolbar">
      <view class="toolbar-title">工单管理</view>
    </view>

    <!-- 搜索 -->
    <wd-search v-model="keyword" placeholder="搜索工单号/项目/报修人" @search="onSearch" @clear="onSearch" />

    <!-- 状态 Tab -->
    <view class="tabs-wrap">
      <wd-tabs v-model="activeTab" @change="onTabChange">
        <wd-tab v-for="tab in tabs" :key="tab.value" :title="tab.label" :name="tab.value"></wd-tab>
      </wd-tabs>
    </view>

    <!-- 紧急度筛选 -->
    <view class="urgency-filter">
      <text
        v-for="u in urgencyOptions"
        :key="u.value"
        class="filter-item"
        :class="{ active: urgency === u.value }"
        @click="onUrgencyChange(u.value)"
      >
        {{ u.label }}
      </text>
    </view>

    <!-- 工单列表 -->
    <view class="order-list">
      <view v-for="item in list" :key="item.id" class="order-card" @click="goDetail(item.id)">
        <view class="card-header">
          <view class="card-title">{{ item.project_name || '未命名工单' }}</view>
          <wd-tag :type="statusTagType(item.status)" round>{{ item.status_label }}</wd-tag>
        </view>
        <view class="card-meta">
          <wd-tag :type="urgencyTagType(item.urgency)" plain round>
            {{ item.urgency_label }}
          </wd-tag>
          <text v-if="item.order_no" class="card-no">单号：{{ item.order_no }}</text>
        </view>
        <view class="card-info">
          <view v-if="item.enterprise_name" class="info-row">
            <text class="info-label">企业</text>
            <text class="info-value">{{ item.enterprise_name }}</text>
          </view>
          <view class="info-row">
            <text class="info-label">报修人</text>
            <text class="info-value">{{ item.reporter?.nickname || '--' }}</text>
          </view>
        </view>
        <view class="card-time">
          {{ item.submitted_at ? `提交于 ${formatDateTime(item.submitted_at)}` : `创建于 ${formatDateTime(item.created_at)}` }}
        </view>
      </view>

      <!-- 空状态 -->
      <view v-if="!loading && list.length === 0" class="empty">
        <view class="empty-icon">📋</view>
        <view class="empty-text">暂无工单</view>
      </view>

      <!-- 加载更多 -->
      <view v-if="list.length > 0" class="load-more">
        <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { isPlatformAdmin } from '@/utils/jwt'
import { formatDateTime, statusTagType, urgencyTagType } from '@/utils/format'
import type { AdminOrderListItem, PageResult } from '@/types'

export default defineComponent({
  setup() {
    return {
      formatDateTime,
      statusTagType,
      urgencyTagType
    }
  },
  data() {
    return {
      keyword: '',
      tabs: [
        { label: '全部', value: 'all', status: '' },
        { label: '待查阅', value: 'reported', status: 'reported' },
        { label: '已阅', value: 'reviewed', status: 'reviewed' },
        { label: '处理中', value: 'processing', status: 'processing' },
        { label: '已完成', value: 'completed', status: 'completed' },
        { label: '已取消', value: 'cancelled', status: 'cancelled' }
      ],
      activeTab: 'all',
      statusParam: '',
      urgencyOptions: [
        { value: '', label: '全部' },
        { value: 'normal', label: '普通' },
        { value: 'urgent', label: '紧急' },
        { value: 'very_urgent', label: '非常紧急' }
      ],
      urgency: '',
      list: [] as AdminOrderListItem[],
      page: 1,
      pageSize: 20,
      totalPages: 1,
      loading: false,
      finished: false
    }
  },
  onShow() {
    if (!isPlatformAdmin()) {
      uni.showToast({ title: '无管理权限', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 600)
      return
    }
    this.loadList(true)
  },
  onPullDownRefresh() {
    this.loadList(true).finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (!this.finished && !this.loading) this.loadList(false)
  },
  methods: {
    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: this.pageSize
        }
        if (this.statusParam) params.status = this.statusParam
        if (this.urgency) params.urgency = this.urgency
        if (this.keyword) params.keyword = this.keyword
        const data = await http.get<PageResult<AdminOrderListItem>>('/admin/orders', params)
        this.page = data.page || targetPage
        this.totalPages = data.total_pages || 1
        this.finished = this.page >= this.totalPages
        this.list = reset ? data.list : [...this.list, ...data.list]
      } catch (e) {
        console.error('加载工单失败', e)
      } finally {
        this.loading = false
      }
    },
    onTabChange(e: { name: string }) {
      const tab = this.tabs.find((t) => t.value === e.name)
      this.activeTab = e.name
      this.statusParam = tab ? tab.status : ''
      this.loadList(true)
    },
    onUrgencyChange(value: string) {
      this.urgency = value
      this.loadList(true)
    },
    onSearch() {
      this.loadList(true)
    },
    goDetail(id: string) {
      uni.navigateTo({ url: `/pages/admin/order/detail?id=${id}` })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16rpx;

  .toolbar-title {
    font-size: 36rpx;
    font-weight: 600;
    color: #1a1a1a;
  }
}

.urgency-filter {
  display: flex;
  align-items: center;
  padding: 8rpx 4rpx 16rpx;
  overflow-x: auto;

  .filter-item {
    flex-shrink: 0;
    font-size: 24rpx;
    color: #666666;
    padding: 10rpx 28rpx;
    margin-right: 16rpx;
    border-radius: 999rpx;
    background-color: #ffffff;

    &.active {
      color: #ffffff;
      background-color: #4d80f0;
    }
  }
}

.order-list {
  .order-card {
    background-color: #ffffff;
    border-radius: 20rpx;
    padding: 28rpx;
    margin-bottom: 20rpx;

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;

      .card-title {
        font-size: 32rpx;
        font-weight: 600;
        color: #1a1a1a;
      }
    }

    .card-meta {
      display: flex;
      align-items: center;
      margin-top: 16rpx;

      .card-no {
        margin-left: 16rpx;
        font-size: 24rpx;
        color: #bbbbbb;
      }
    }

    .card-info {
      margin-top: 16rpx;
      padding-top: 16rpx;
      border-top: 1rpx solid #f2f3f5;

      .info-row {
        display: flex;
        align-items: center;
        margin-bottom: 8rpx;
        font-size: 26rpx;

        &:last-child {
          margin-bottom: 0;
        }

        .info-label {
          width: 120rpx;
          color: #999999;
          flex-shrink: 0;
        }

        .info-value {
          color: #333333;
        }
      }
    }

    .card-time {
      margin-top: 12rpx;
      font-size: 24rpx;
      color: #bbbbbb;
    }
  }
}

.empty {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 80rpx 0;
  text-align: center;

  .empty-icon {
    font-size: 64rpx;
  }

  .empty-text {
    margin-top: 20rpx;
    font-size: 28rpx;
    color: #999999;
  }
}

.load-more {
  padding: 24rpx 0 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: #bbbbbb;
}
</style>
