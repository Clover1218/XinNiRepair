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
      <!-- 顶部统计卡（5.22 维修员个人汇总：累计主行 + 今日副行，可折叠） -->
      <view class="stats-card">
        <view class="stats-head" @click="toggleOverview">
          <text class="stats-title">我的处理概况</text>
          <text class="stats-toggle">{{ overviewCollapsed ? '展开 ▾' : '收起 ▴' }}</text>
        </view>

        <view v-if="overviewCollapsed" class="stats-collapsed">
          <text class="sc-item">可接单 {{ overview ? overview.pending_accept : '--' }}</text>
          <text class="sc-dot">·</text>
          <text class="sc-item">我的接单 {{ overview ? overview.my_accepted : '--' }}</text>
          <text class="sc-dot">·</text>
          <text class="sc-item">已完工 {{ overview ? overview.my_completed : '--' }}</text>
        </view>

        <template v-else>
          <view class="stats-grid">
            <view v-for="m in overviewMetrics" :key="m.key" class="stats-box">
              <text class="stats-key">{{ m.key }}</text>
              <text class="stats-val" :style="{ color: m.color }">{{ m.value }}</text>
            </view>
          </view>
          <view class="stats-today">
            <text class="st-item">今日接单 {{ overview ? overview.today_accepted : '--' }}</text>
            <text class="st-item">今日完工 {{ overview ? overview.today_completed : '--' }}</text>
            <text class="st-item">完工率 {{ overviewCompletionRate }}</text>
          </view>
        </template>
      </view>

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
              @done="onActionDone"
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
import type { AdminOrderListItem, RepairerOverview } from '@/types'
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
      finished: false,
      /* ── 顶部统计卡（5.22 维修员个人汇总：累计 + 今日双行） ── */
      overview: null as RepairerOverview | null,
      overviewCollapsed: false,
      overviewLoading: false
    }
  },
  computed: {
    currentEnterpriseId(): string {
      const opt = this.enterpriseOptions[this.enterpriseIndex]
      return opt ? opt.id : ''
    },
    /** 统计卡主行：累计口径（可接单/我的接单/处理中/已完工） */
    overviewMetrics(): { key: string; value: number | string; color: string }[] {
      const o = this.overview
      return [
        { key: '可接单', value: o ? o.pending_accept : '--', color: '#4d80f0' },
        { key: '我的接单', value: o ? o.my_accepted : '--', color: '#1a1a1a' },
        { key: '处理中', value: o ? o.my_processing : '--', color: '#ff9f0a' },
        { key: '已完工', value: o ? o.my_completed : '--', color: '#07c160' }
      ]
    },
    /** 完工率 = 累计完工 / 累计接单 */
    overviewCompletionRate(): string {
      const o = this.overview
      if (!o || !o.my_accepted) return '--'
      return `${Math.round((o.my_completed / o.my_accepted) * 100)}%`
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
      this.loadOverview()
    },
    /** 顶部统计卡数据（5.22）：不传 repairer_id 即统计本人；随当前企业筛选 */
    async loadOverview() {
      this.overviewLoading = true
      try {
        const params: Record<string, unknown> = {}
        if (this.currentEnterpriseId) params.enterprise_id = this.currentEnterpriseId
        const data = await http.get<RepairerOverview>('/admin/orders/stats/repairer-overview', params)
        this.overview = data
      } catch (e) {
        console.error('加载处理统计失败', e)
        this.overview = null
      } finally {
        this.overviewLoading = false
      }
    },
    toggleOverview() {
      this.overviewCollapsed = !this.overviewCollapsed
    },
    /** 卡片动作执行成功后：列表与统计同步刷新 */
    onActionDone() {
      this.loadList(true)
      this.loadOverview()
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
      this.loadOverview()
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

/* ── 顶部统计卡（5.22 维修员个人汇总） ── */
.stats-card {
  margin: 20rpx 24rpx 0;
  padding: 24rpx 28rpx;
  border-radius: 20rpx;
  background-color: #ffffff;
}

.stats-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stats-title {
  font-size: 30rpx;
  font-weight: 600;
  color: #1a1a1a;
}

.stats-toggle {
  font-size: 24rpx;
  color: #8a8f99;
}

.stats-grid {
  display: flex;
  margin-top: 20rpx;
}

.stats-box {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.stats-key {
  font-size: 24rpx;
  color: #8a8f99;
}

.stats-val {
  margin-top: 8rpx;
  font-size: 40rpx;
  font-weight: 600;
  line-height: 1.1;
}

.stats-today {
  display: flex;
  margin-top: 20rpx;
  padding-top: 18rpx;
  border-top: 2rpx solid #f0f1f3;
}

.st-item {
  flex: 1;
  font-size: 24rpx;
  color: #606266;
}

.stats-collapsed {
  display: flex;
  align-items: center;
  margin-top: 12rpx;
}

.sc-item {
  font-size: 26rpx;
  color: #303133;
}

.sc-dot {
  margin: 0 12rpx;
  color: #c0c4cc;
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
