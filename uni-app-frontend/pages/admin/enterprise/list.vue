<template>
  <view class="page">
    <!-- 工具栏 -->
    <view class="toolbar">
      <view class="toolbar-title">企业管理</view>
      <wd-button size="small" type="primary" round @click="goCreate">+ 创建</wd-button>
    </view>

    <!-- 搜索 -->
    <wd-search v-model="keyword" placeholder="搜索企业名称" @search="onSearch" @clear="onSearch" />

    <!-- 企业列表 -->
    <view class="ent-list">
      <view
        v-for="item in list"
        :key="item.id"
        class="ent-card"
        @click="goDetail(item.id)"
      >
        <view class="card-header">
          <view class="ent-name">{{ item.name }}</view>
          <wd-tag :type="enterpriseStatusTagType(item.status)" round>
            {{ enterpriseStatusLabel(item.status) }}
          </wd-tag>
        </view>
        <view class="card-stats">
          <view class="stat">
            <text class="stat-num">{{ item.member_count }}</text>
            <text class="stat-label">成员</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ item.order_count }}</text>
            <text class="stat-label">工单</text>
          </view>
          <view class="stat" v-if="item.pending_count > 0">
            <text class="stat-num warn">{{ item.pending_count }}</text>
            <text class="stat-label">待审核</text>
          </view>
        </view>
        <view class="card-time">创建于 {{ formatDate(item.created_at) }}</view>
      </view>

      <!-- 空状态 -->
      <view v-if="!loading && list.length === 0" class="empty">
        <view class="empty-icon">🏢</view>
        <view class="empty-text">暂无企业</view>
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
import { formatDate, enterpriseStatusLabel, enterpriseStatusTagType } from '@/utils/format'
import type { AdminEnterpriseItem, PageResult } from '@/types'

export default defineComponent({
  setup() {
    return {
      formatDate,
      enterpriseStatusLabel,
      enterpriseStatusTagType
    }
  },
  data() {
    return {
      keyword: '',
      list: [] as AdminEnterpriseItem[],
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
        const data = await http.get<PageResult<AdminEnterpriseItem>>('/admin/enterprises', {
          page: targetPage,
          page_size: this.pageSize,
          keyword: this.keyword
        })
        this.page = data.page || targetPage
        this.totalPages = data.total_pages || 1
        this.finished = this.page >= this.totalPages
        this.list = reset ? data.list : [...this.list, ...data.list]
      } catch (e) {
        console.error('加载企业列表失败', e)
      } finally {
        this.loading = false
      }
    },
    onSearch() {
      this.loadList(true)
    },
    goCreate() {
      uni.navigateTo({ url: '/pages/admin/enterprise/create' })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${id}` })
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

.ent-list {
  margin-top: 16rpx;

  .ent-card {
    background-color: #ffffff;
    border-radius: 20rpx;
    padding: 28rpx;
    margin-bottom: 20rpx;

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;

      .ent-name {
        font-size: 32rpx;
        font-weight: 600;
        color: #1a1a1a;
      }
    }

    .card-stats {
      display: flex;
      margin-top: 24rpx;

      .stat {
        display: flex;
        align-items: baseline;
        margin-right: 40rpx;

        .stat-num {
          font-size: 36rpx;
          font-weight: 700;
          color: #4d80f0;

          &.warn {
            color: #fa5151;
          }
        }

        .stat-label {
          margin-left: 8rpx;
          font-size: 24rpx;
          color: #999999;
        }
      }
    }

    .card-time {
      margin-top: 16rpx;
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
