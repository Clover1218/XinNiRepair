<template>
  <view class="page">
    <!-- 未登录 -->
    <view v-if="!isLoggedIn" class="empty-wrap">
      <view class="empty-icon">🔧</view>
      <view class="empty-text">登录后查看待处理工单</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <view v-else-if="!hasPermission" class="empty-wrap">
      <view class="empty-icon">🔧</view>
      <view class="empty-text">仅维修业务员可进入处理工单</view>
    </view>

    <template v-else>
      <!-- 企业筛选 Header + 状态分组 Tab -->
      <order-filter-bar
        :enterprises="enterpriseOptions"
        :enterprise-index="enterpriseIndex"
        :tabs="tabs"
        :active-tab="activeTab"
        @enterprise-change="onEnterpriseChange"
        @tab-change="onTabChange"
      ></order-filter-bar>

      <!-- 工单列表 -->
      <view class="order-list">
        <order-card
          v-for="item in list"
          :key="item.id"
          :item="item"
          @click="goDetail"
        >
          <template #actions>
            <order-action-bar
              :order-id="item.id"
              :actions="item.available_actions"
              @done="loadList(true)"
            ></order-action-bar>
          </template>
        </order-card>

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
import { normalizePage, REPAIR_TABS } from '@/utils/format'
import { isStoreStaff } from '@/utils/auth'
import type { AdminOrderListItem } from '@/types'
import { useUserStore } from '@/stores/user'

/**
 * V1.3 处理工单页（第六章，维修业务员/超管）
 * - Header：企业筛选（默认“全部企业”）
 * - 状态分组：待接单(pending_accept) / 处理中(processing) / 结束(completed,cancelled)
 * - 列表卡片与审核工单页、我的工单页完全一致（order-card 统一卡片：缩略图 + 完整描述≤3行 + 报修企业/报修地点/报修人/联系方式/问题类型 + 分隔线 + 当前状态与提交时间）
 * - 卡片底部按当前权限渲染可操作按钮（order-action-bar：审核通过/退回/接单/完工/重新打开/改对账），简单动作就地弹窗执行，复杂表单（完工/对账）跳转详情页；与详情页可用操作一致
 */
export default defineComponent({
  setup() {
    return { userStore: useUserStore() }
  },
  data() {
    return {
      isLoggedIn: false,
      hasPermission: false,
      enterpriseOptions: [] as { id: string; name: string }[],
      enterpriseIndex: 0,
      tabs: REPAIR_TABS,
      activeTab: 'pending_accept',
      list: [] as AdminOrderListItem[],
      page: 1,
      totalPages: 1,
      loading: false,
      finished: false
    }
  },
  computed: {
    currentEnterpriseId(): string {
      const opt = this.enterpriseOptions[this.enterpriseIndex]
      return opt ? opt.id : ''
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
      if (!isStoreStaff()) {
        this.hasPermission = false
        uni.showToast({ title: '无处理工单权限', icon: 'none' })
        return
      }
      this.hasPermission = true
      await this.buildEnterpriseOptions()
      this.loadList(true)
    },
    async buildEnterpriseOptions() {
      const opts: { id: string; name: string }[] = [{ id: '', name: '全部企业' }]
      try {
        const data = await http.get<any>('/admin/enterprises', { page: 1, page_size: 100 })
        const res = normalizePage<{ id: string; name: string }>(data)
        res.list.forEach((e) => opts.push({ id: e.id, name: e.name }))
      } catch (e) {
        // 拉取失败时仍可用“全部企业”
      }
      this.enterpriseOptions = opts
      if (this.enterpriseIndex >= opts.length) this.enterpriseIndex = 0
    },
    onEnterpriseChange(index: number) {
      this.enterpriseIndex = index
      this.loadList(true)
    },
    onTabChange(value: string) {
      this.activeTab = value
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
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: PAGE_SIZE,
          status: this.currentStatusParam()
        }
        if (this.currentEnterpriseId) params.enterprise_id = this.currentEnterpriseId
        const data = await http.get<any>('/admin/orders', params)
        const res = normalizePage<AdminOrderListItem>(data)
        this.page = res.page
        this.totalPages = res.total_pages
        this.finished = this.page >= this.totalPages
        this.list = reset ? res.list : [...this.list, ...res.list]
      } catch (e) {
        console.error('加载处理工单失败', e)
      } finally {
        this.loading = false
      }
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    goDetail(item: AdminOrderListItem) {
      uni.navigateTo({ url: `/pages/admin/order/detail?id=${item.id}` })
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

.order-list {
  padding: 20rpx 24rpx;
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
