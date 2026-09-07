<template>
  <view class="page">
    <!-- 未登录 -->
    <view v-if="!isLoggedIn" class="empty-wrap">
      <view class="empty-icon">⚖</view>
      <view class="empty-text">登录后查看审核任务</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <view v-else-if="reviewerOptions.length === 0" class="empty-wrap">
      <view class="empty-icon">⚖</view>
      <view class="empty-text">暂无担任审核员的单位</view>
    </view>

    <template v-else>
      <!-- 工具栏：审核单位选择 + 管理本单位 -->
      <view class="toolbar">
        <picker
          mode="selector"
          :range="reviewerOptions"
          range-key="enterprise_name"
          :value="currentReviewerIndex"
          @change="onReviewerChange"
        >
          <view class="ent-picker">
            <text class="ent-picker-label">审核单位</text>
            <text class="ent-picker-value">{{ currentReviewerName }}</text>
            <text class="ent-picker-arrow">▾</text>
          </view>
        </picker>
        <wd-button size="small" plain round @click="goManage">管理本单位</wd-button>
      </view>

      <!-- 状态 Tab -->
      <view class="tabs-wrap">
        <wd-tabs :model-value="activeTab" @change="onTabChange">
          <wd-tab v-for="tab in tabs" :key="tab.value" :title="tab.label" :name="tab.value"></wd-tab>
        </wd-tabs>
      </view>

      <!-- 工单列表 -->
      <view class="order-list">
        <view v-for="item in list" :key="item.id" class="order-card">
          <view class="card-top" @click="goDetail(item.id)">
            <view class="card-header">
              <text class="card-no">{{ item.order_no || '未生成单号' }}</text>
              <wd-tag :type="statusTagType(item.status)" round>{{ item.status_label }}</wd-tag>
            </view>
            <view class="card-cat">
              {{ item.category_name || '未分类' }}<template v-if="item.property_name">/{{ item.property_name }}</template>
            </view>
            <view class="card-desc">{{ item.description || '（无描述）' }}</view>
            <view class="card-meta">
              <text>报修人：{{ item.reporter?.nickname || '--' }}</text>
              <text class="meta-time">{{ formatDateTime(item.submitted_at || item.created_at) }}</text>
            </view>
          </view>

          <!-- 行操作：reported 行可审核/退回 -->
          <view v-if="item.status === 'reported'" class="card-actions">
            <wd-button size="small" plain round @click.stop="showAuditModal(item)">审核通过</wd-button>
            <wd-button size="small" type="danger" plain round @click.stop="showRejectModal(item)">退回</wd-button>
            <wd-button size="small" plain round @click.stop="goDetail(item.id)">详情</wd-button>
          </view>
          <view v-else class="card-actions">
            <wd-button size="small" plain round @click.stop="goDetail(item.id)">详情</wd-button>
          </view>
        </view>

        <view v-if="!loading && list.length === 0" class="empty">
          <view class="empty-icon">📋</view>
          <view class="empty-text">暂无工单</view>
        </view>

        <view v-if="list.length > 0" class="load-more">
          <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { PAGE_SIZE } from '@/utils/config'
import { normalizePage, formatDateTime, statusTagType } from '@/utils/format'
import { reviewerEnterprises, isUnitReviewer } from '@/utils/auth'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      enterpriseStore: useEnterpriseStore(),
      formatDateTime,
      statusTagType
    }
  },
  data() {
    return {
      isLoggedIn: false,
      reviewerOptions: [] as { enterprise_id: string; enterprise_name: string }[],
      currentReviewerIndex: 0,
      tabs: [
        { label: '待审核', value: 'reported', status: 'reported' },
        { label: '待接单', value: 'pending_accept', status: 'pending_accept' },
        { label: '处理中', value: 'processing', status: 'processing' },
        { label: '已完成', value: 'completed', status: 'completed' },
        { label: '已取消', value: 'cancelled', status: 'cancelled' }
      ],
      activeTab: 'reported',
      list: [] as any[],
      page: 1,
      totalPages: 1,
      loading: false,
      finished: false
    }
  },
  computed: {
    currentReviewerName(): string {
      const opt = this.reviewerOptions[this.currentReviewerIndex]
      return opt ? opt.enterprise_name : ''
    },
    currentEnterpriseId(): string {
      const opt = this.reviewerOptions[this.currentReviewerIndex]
      return opt ? opt.enterprise_id : ''
    }
  },
  onShow() {
    this.init()
  },
  onPullDownRefresh() {
    this.loadList(true).finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.isLoggedIn && !this.finished && !this.loading) {
      this.loadList(false)
    }
  },
  methods: {
    async init() {
      if (!this.userStore.token) {
        const ok = await this.userStore.ensureLoggedIn()
        if (!ok) {
          this.isLoggedIn = false
          return
        }
      }
      this.isLoggedIn = true
      try {
        await this.userStore.fetchUserInfo()
      } catch (e) {
        // 401 由请求层兜底
      }
      if (!isUnitReviewer()) {
        uni.showToast({ title: '无审核权限', icon: 'none' })
        setTimeout(() => uni.navigateBack(), 600)
        return
      }
      const opts = reviewerEnterprises()
      this.reviewerOptions = opts
      // 默认选中当前上下文单位（若是审核单位），否则第一个审核单位
      const cur = this.enterpriseStore.currentEnterpriseId
      const curIdx = opts.findIndex((e) => e.enterprise_id === cur)
      this.currentReviewerIndex = curIdx >= 0 ? curIdx : 0
      this.loadList(true)
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    onReviewerChange(e: { detail: { value: number } }) {
      this.currentReviewerIndex = Number(e.detail.value) || 0
      this.loadList(true)
    },
    onTabChange(e: { name: string }) {
      this.activeTab = e.name
      this.loadList(true)
    },
    currentStatusParam(): string {
      const t = this.tabs.find((x) => x.value === this.activeTab)
      return t ? t.status : ''
    },
    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const data = await http.get<any>('/admin/orders', {
          page: targetPage,
          page_size: PAGE_SIZE,
          status: this.currentStatusParam(),
          enterprise_id: this.currentEnterpriseId
        })
        const res = normalizePage(data)
        this.page = res.page
        this.totalPages = res.total_pages
        this.finished = this.page >= this.totalPages
        this.list = reset ? res.list : [...this.list, ...res.list]
      } catch (e) {
        console.error('加载审核工单失败', e)
      } finally {
        this.loading = false
      }
    },
    goManage() {
      uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${this.currentEnterpriseId}` })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: `/pages/admin/order/detail?id=${id}` })
    },
    showAuditModal(item: any) {
      const that = this
      uni.showModal({
        title: '审核通过',
        content: '审核通过后工单进入待接单并上报维修业务员，确认？',
        editable: true,
        placeholderText: '备注（可选，≤100字）',
        success: async (res) => {
          if (!res.confirm) return
          const remark = ((res.content || '') as string).trim()
          if (remark.length > 100) {
            uni.showToast({ title: '备注不能超过100字', icon: 'none' })
            return
          }
          try {
            await http.post(`/admin/orders/${item.id}/audit`, { remark })
            uni.showToast({ title: '已审核通过', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            console.error('审核失败', e)
          }
        }
      })
    },
    showRejectModal(item: any) {
      const that = this
      uni.showModal({
        title: '退回工单',
        content: '请填写退回原因',
        editable: true,
        placeholderText: '退回原因（≥10字，≤200字）',
        success: async (res) => {
          if (!res.confirm) return
          const reason = ((res.content || '') as string).trim()
          if (reason.length < 10) {
            uni.showToast({ title: '退回原因至少10字', icon: 'none' })
            return
          }
          if (reason.length > 200) {
            uni.showToast({ title: '退回原因不能超过200字', icon: 'none' })
            return
          }
          try {
            await http.post(`/admin/orders/${item.id}/reject`, { reason })
            uni.showToast({ title: '已退回', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            console.error('退回失败', e)
          }
        }
      })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding-bottom: 40rpx;
  background-color: #f5f6f8;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  background-color: #ffffff;
  border-bottom: 1rpx solid #f0f0f0;

  .ent-picker {
    display: flex;
    align-items: center;
    background: #f5f6f8;
    border-radius: 999rpx;
    padding: 10rpx 20rpx;
    max-width: 460rpx;

    .ent-picker-label {
      font-size: 24rpx;
      color: #999999;
      margin-right: 12rpx;
    }

    .ent-picker-value {
      font-size: 26rpx;
      color: #333333;
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .ent-picker-arrow {
      margin-left: 8rpx;
      color: #999999;
      font-size: 22rpx;
    }
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

    .card-no {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
      margin-right: 16rpx;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .card-cat {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #666666;
  }

  .card-desc {
    margin-top: 12rpx;
    font-size: 26rpx;
    color: #333333;
    line-height: 1.5;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    overflow: hidden;
  }

  .card-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #999999;
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 20rpx;

    wd-button {
      margin-left: 12rpx;
    }
  }
}

.empty-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 220rpx;

  .empty-icon {
    font-size: 88rpx;
  }

  .empty-text {
    margin: 24rpx 0 32rpx;
    font-size: 30rpx;
    color: #666666;
  }
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 140rpx;

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
