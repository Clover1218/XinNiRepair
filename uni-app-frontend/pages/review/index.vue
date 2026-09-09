<template>
  <view class="page">
    <!-- 未登录 -->
    <view v-if="!isLoggedIn" class="empty-wrap">
      <view class="empty-icon">🔍</view>
      <view class="empty-text">登录后查看审核任务</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <view v-else-if="!hasPermission" class="empty-wrap">
      <view class="empty-icon">🔍</view>
      <view class="empty-text">暂无可审核的单位</view>
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

      <!-- 统计视图（C20/C21「统计」Tab；C22：两段独立卡 + 历史统计=先时间→总单数→指标Tab，普通排版） -->
      <view v-if="activeTab === 'stats'" class="stats-view">
        <!-- 审核员选了「全部企业」且非店方：后端多单位聚合未支持，提示先选具体单位 -->
        <view v-if="statsUnavailable" class="stats-hint">
          <view class="stats-hint-icon">📊</view>
          <view class="stats-hint-text">请先选择具体单位查看统计</view>
        </view>

        <template v-else>
          <!-- ① 今日概况（独立卡，固定置顶，不受时间范围影响） -->
          <view class="stats-card">
            <view class="sc-title">今日概况</view>
            <view class="today-grid">
              <view class="today-tile">
                <view class="today-row">
                  <text class="today-key">当前待审核</text>
                  <text class="today-val" style="color: #4d80f0">{{ todayVal('pending_review') }}</text>
                </view>
              </view>
              <view class="today-tile">
                <view class="today-row">
                  <text class="today-key">今日新增上报</text>
                  <text class="today-val" style="color: #ff9f0a">{{ todayVal('submitted_today') }}</text>
                </view>
              </view>
              <view class="today-tile">
                <view class="today-row">
                  <text class="today-key">今日审核通过</text>
                  <text class="today-val" style="color: #07c160">{{ todayVal('audited_today') }}</text>
                </view>
              </view>
              <view class="today-tile">
                <view class="today-row">
                  <text class="today-key">今日退回</text>
                  <text class="today-val" style="color: #fa5151">{{ todayVal('rejected_today') }}</text>
                </view>
              </view>
            </view>
          </view>

          <!-- ② 历史统计大卡：先选时间 → 总工单数 → 指标 Tab（C22） -->
          <view class="stats-card">
            <view class="sc-title">历史统计</view>

            <!-- 先选时间 -->
            <view class="chip-row">
              <view
                v-for="r in rangeOptions"
                :key="r.value"
                class="chip"
                :class="{ active: statsRangeKey === r.value }"
                @click="pickRange(r.value)"
              >
                {{ r.label }}
              </view>
            </view>
            <view v-if="statsRangeKey === 'custom'" class="custom-range">
              <picker mode="date" :value="customStart" start="2020-01-01" :end="today" @change="onCustomStart">
                <view class="range-field">{{ customStart ? `开始：${customStart}` : '请选择开始日期' }}</view>
              </picker>
              <text class="range-sep">至</text>
              <picker mode="date" :value="customEnd" start="2020-01-01" :end="today" @change="onCustomEnd">
                <view class="range-field">{{ customEnd ? `结束：${customEnd}` : '请选择结束日期' }}</view>
              </picker>
            </view>

            <!-- 总工单数（字段在前、数值在后；附时间范围·更新时间） -->
            <view class="total-row">
              <view class="total-left">
                <text class="total-key">总工单数</text>
                <text class="total-val">{{ stats ? stats.total : '--' }}</text>
              </view>
              <text class="total-meta">{{ statsMeta }}</text>
            </view>

            <!-- 指标 Tab（数据同源，切换不重新请求） -->
            <view class="seg-row">
              <view
                class="seg"
                :class="{ active: statsViewTab === 'status' }"
                @click="statsViewTab = 'status'"
              >状态</view>
              <view
                class="seg"
                :class="{ active: statsViewTab === 'category' }"
                @click="statsViewTab = 'category'"
              >大类</view>
              <view
                class="seg"
                :class="{ active: statsViewTab === 'reporter' }"
                @click="statsViewTab = 'reporter'"
              >报修人排行</view>
            </view>

            <!-- 面板：状态分布 -->
            <view v-if="statsViewTab === 'status'">
              <view v-if="stats && stats.total > 0" class="bar-list">
                <view v-for="row in stats.by_status" :key="row.status" class="bar-item">
                  <view class="bar-head">
                    <text class="bar-name" :style="{ color: statusColor(row.status) }">{{ row.label }}</text>
                    <text class="bar-count">{{ row.count }}<text v-if="row.count" class="bar-pct">（{{ ratioText(row.count) }}）</text></text>
                  </view>
                  <view class="bar-track">
                    <view
                      class="bar-fill"
                      :style="{ width: ratioWidth(row.count), backgroundColor: statusColor(row.status) }"
                    ></view>
                  </view>
                </view>
              </view>
              <view v-else-if="stats && !statsLoading" class="sc-empty">暂无数据</view>
            </view>

            <!-- 面板：项目大类分布 -->
            <view v-else-if="statsViewTab === 'category'">
              <view v-if="displayCategories.length > 0" class="bar-list">
                <view v-for="(row, i) in displayCategories" :key="i" class="bar-item">
                  <view class="bar-head">
                    <text class="bar-name" :style="{ color: catColors[i % catColors.length] }">{{ catName(row.category_name) }}</text>
                    <text class="bar-count">{{ row.count }}<text v-if="row.count" class="bar-pct">（{{ ratioText(row.count) }}）</text></text>
                  </view>
                  <view class="bar-track">
                    <view
                      class="bar-fill"
                      :style="{ width: ratioWidth(row.count), backgroundColor: catColors[i % catColors.length] }"
                    ></view>
                  </view>
                </view>
              </view>
              <view v-else-if="stats && !statsLoading" class="sc-empty">暂无数据</view>
            </view>

            <!-- 面板：报修人 Top5 -->
            <view v-else>
              <view v-if="stats && stats.top_reporters.length > 0" class="rank-list">
                <view v-for="(r, i) in stats.top_reporters" :key="r.user_id" class="rank-item">
                  <view class="rank-no" :class="{ first: i === 0 }">{{ i + 1 }}</view>
                  <view class="rank-avatar">
                    <image v-if="r.avatar_url" class="rank-avatar-img" :src="r.avatar_url" mode="aspectFill"></image>
                    <text v-else class="rank-avatar-txt">{{ (r.nickname || '用户').charAt(0) }}</text>
                  </view>
                  <text class="rank-name">{{ r.nickname || '未命名用户' }}</text>
                  <text class="rank-count">{{ r.count }} 单</text>
                </view>
              </view>
              <view v-else-if="stats && !statsLoading" class="sc-empty">暂无数据</view>
            </view>

            <view v-if="statsLoading" class="sc-loading">加载中…</view>
          </view>
        </template>
      </view>

      <!-- 工单列表 -->
      <view v-else class="order-list">
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
import { formatDateTime, normalizePage, REVIEW_TABS, statusColor } from '@/utils/format'
import { isStoreStaff, isUnitReviewer, reviewerEnterprises } from '@/utils/auth'
import type { AdminOrderListItem, OrderCategoryStat, OrderStats, OrderStatsToday } from '@/types'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'

/** 「统计」Tab 时间范围快捷项（C20） */
const RANGE_OPTIONS: { value: string; label: string }[] = [
  { value: 'month', label: '本月' },
  { value: 'quarter', label: '本季' },
  { value: 'all', label: '全部' },
  { value: 'custom', label: '自定义' }
]

/** 大类分布条配色（按序循环） */
const CAT_COLORS = ['#4d80f0', '#07c160', '#ff9f0a', '#534ab7', '#d4537e', '#888780']

/** 本地日期 YYYY-MM-DD */
function fmtDate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 当月首日（含当天至今） */
function monthStart(): string {
  const n = new Date()
  return fmtDate(new Date(n.getFullYear(), n.getMonth(), 1))
}

/** 当季首日 */
function quarterStart(): string {
  const n = new Date()
  const q = Math.floor(n.getMonth() / 3) * 3
  return fmtDate(new Date(n.getFullYear(), q, 1))
}

/**
 * V1.3 审核工单页（第五章）
 * - Header：企业筛选（单个企业 / 全部企业）
 * - 状态分组：待审核(reported) / 处理中(pending_accept,processing) / 结束(completed,cancelled)
 * - 卡片底部按当前权限渲染可操作按钮（order-action-bar：审核通过/退回/接单/完工/重新打开/改对账），简单动作就地弹窗执行，复杂表单（完工/对账）跳转详情页；与详情页可用操作一致
 */
export default defineComponent({
  setup() {
    return { userStore: useUserStore(), enterpriseStore: useEnterpriseStore() }
  },
  data() {
    return {
      isLoggedIn: false,
      hasPermission: false,
      storeStaff: false,
      enterpriseOptions: [] as { id: string; name: string }[],
      enterpriseIndex: 0,
      tabs: REVIEW_TABS,
      // C21：统计为首页 Tab 且为页面默认
      activeTab: 'stats',
      // ── 「统计」Tab（C20） ──
      rangeOptions: RANGE_OPTIONS,
      catColors: CAT_COLORS,
      statsRangeKey: 'month',
      customStart: monthStart(),
      customEnd: fmtDate(new Date()),
      stats: null as OrderStats | null,
      statsLoading: false,
      statsUnavailable: false,
      /** 历史统计卡内指标 Tab：status / category / reporter（C22） */
      statsViewTab: 'status',
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
    },
    today(): string {
      return fmtDate(new Date())
    },
    statsRangeLabel(): string {
      const map: Record<string, string> = { month: '本月', quarter: '本季', all: '全部时间', custom: '自定义' }
      if (this.statsRangeKey === 'custom' && this.customStart && this.customEnd) {
        return `${this.customStart} ~ ${this.customEnd}`
      }
      return map[this.statsRangeKey] || '全部时间'
    },
    /** 历史统计小字：时间范围 · 更新时间 */
    statsMeta(): string {
      const base = this.statsRangeLabel
      if (!this.stats || !this.stats.updated_at) return base
      const time = this.formatDateTime(this.stats.updated_at)
      return `${base} · 更新 ${time}`
    },
    /** 大类分布：Top5 + 其余合并「其他」 */
    displayCategories(): { category_name: string; count: number }[] {
      if (!this.stats) return []
      const list: OrderCategoryStat[] = this.stats.by_category || []
      const shown: { category_name: string; count: number }[] = list
        .slice(0, 5)
        .map((c) => ({ category_name: c.category_name, count: c.count }))
      if (list.length > 5) {
        shown.push({ category_name: '其他', count: list.slice(5).reduce((a, b) => a + b.count, 0) })
      }
      return shown
    }
  },
  onShow() {
    this.init()
  },
  onPullDownRefresh() {
    const p = this.activeTab === 'stats' ? this.loadStats() : this.loadList(true)
    p.finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.activeTab !== 'stats' && this.isLoggedIn && !this.finished && !this.loading) {
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
      this.storeStaff = isStoreStaff()
      if (!isUnitReviewer() && !this.storeStaff) {
        this.hasPermission = false
        uni.showToast({ title: '无审核权限', icon: 'none' })
        return
      }
      this.hasPermission = true
      await this.buildEnterpriseOptions()
      if (this.activeTab === 'stats') {
        this.loadStats()
      } else {
        this.loadList(true)
      }
    },
    /** 企业筛选数据源：店方=全部企业+企业列表；审核员=其担任审核员的单位（多单位时提供“全部企业”） */
    async buildEnterpriseOptions() {
      let opts: { id: string; name: string }[] = []
      if (this.storeStaff) {
        opts = [{ id: '', name: '全部企业' }]
        try {
          const data = await http.get<any>('/admin/enterprises', { page: 1, page_size: 100 })
          const res = normalizePage<{ id: string; name: string }>(data)
          opts = opts.concat(res.list.map((e) => ({ id: e.id, name: e.name })))
        } catch (e) {
          // 拉不到企业列表时仍可用“全部企业”
        }
      } else {
        const units = reviewerEnterprises().map((u) => ({
          id: u.enterprise_id,
          name: u.enterprise_name
        }))
        opts = units.length > 1 ? [{ id: '', name: '全部企业' }].concat(units) : units
      }
      this.enterpriseOptions = opts
      // 默认选中当前上下文单位（在可选列表中），否则第一个可选项
      const cur = this.enterpriseStore.currentEnterpriseId
      const idx = opts.findIndex((o) => o.id && o.id === cur)
      this.enterpriseIndex = idx >= 0 ? idx : 0
    },
    onEnterpriseChange(index: number) {
      this.enterpriseIndex = index
      if (this.activeTab === 'stats') {
        this.loadStats()
      } else {
        this.loadList(true)
      }
    },
    onTabChange(value: string) {
      this.activeTab = value
      if (value === 'stats') {
        this.loadStats()
      } else {
        this.loadList(true)
      }
    },
    currentStatusParam(): string {
      const t = this.tabs.find((x) => x.value === this.activeTab)
      return t ? t.status : ''
    },
    // ── 「统计」Tab 逻辑（C20） ──
    pickRange(v: string) {
      this.statsRangeKey = v
      if (v === 'custom' && (!this.customStart || !this.customEnd)) {
        this.customStart = monthStart()
        this.customEnd = fmtDate(new Date())
      }
      this.loadStats()
    },
    onCustomStart(e: any) {
      const v = e.detail?.value
      if (!v) return
      this.customStart = v
      if (this.customEnd && v > this.customEnd) {
        uni.showToast({ title: '开始日期不能晚于结束日期', icon: 'none' })
        return
      }
      this.loadStats()
    },
    onCustomEnd(e: any) {
      const v = e.detail?.value
      if (!v) return
      this.customEnd = v
      if (this.customStart && v < this.customStart) {
        uni.showToast({ title: '结束日期不能早于开始日期', icon: 'none' })
        return
      }
      this.loadStats()
    },
    statsParams(): Record<string, unknown> {
      const p: Record<string, unknown> = {}
      if (this.currentEnterpriseId) p.enterprise_id = this.currentEnterpriseId
      switch (this.statsRangeKey) {
        case 'month':
          p.start = monthStart()
          break
        case 'quarter':
          p.start = quarterStart()
          break
        case 'all':
          break
        case 'custom':
          if (this.customStart) p.start = this.customStart
          if (this.customEnd) p.end = this.customEnd
          break
      }
      return p
    },
    async loadStats() {
      if (this.statsLoading) return
      // 审核员选「全部企业」且非店方：后端多单位聚合未支持（同 5.2/第十一章 #4 过渡限制），提示先选单位
      if (!this.storeStaff && !this.currentEnterpriseId) {
        this.stats = null
        this.statsUnavailable = true
        this.statsLoading = false
        return
      }
      this.statsUnavailable = false
      this.statsLoading = true
      try {
        this.stats = await http.get<OrderStats>('/admin/orders/stats', this.statsParams())
      } catch (e) {
        console.error('加载统计失败', e)
      } finally {
        this.statsLoading = false
      }
    },
    ratioText(count: number): string {
      const total = this.stats ? this.stats.total : 0
      if (!total) return '0%'
      const p = (count / total) * 100
      return `${p >= 100 ? 100 : Math.round(p * 10) / 10}%`
    },
    ratioWidth(count: number): string {
      const total = this.stats ? this.stats.total : 0
      if (!total || count <= 0) return '0%'
      const p = Math.min(100, (count / total) * 100)
      return `${Math.max(2, Math.round(p * 10) / 10)}%`
    },
    catName(name: string): string {
      return name || '未分类'
    },
    todayVal(k: keyof OrderStatsToday): number {
      return this.stats && this.stats.today ? this.stats.today[k] : 0
    },
    statusColor,
    formatDateTime,
    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const status = this.currentStatusParam()
        // 审核员选择“全部企业”且后端不支持多单位：逐单位取首页后按时间合并（本期不分页）
        if (!this.storeStaff && !this.currentEnterpriseId) {
          const merged = await this.loadMergedUnits(status)
          this.list = merged
          this.finished = true
          this.totalPages = 1
          this.page = 1
          return
        }
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: PAGE_SIZE,
          status
        }
        if (this.currentEnterpriseId) params.enterprise_id = this.currentEnterpriseId
        const data = await http.get<any>('/admin/orders', params)
        const res = normalizePage<AdminOrderListItem>(data)
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
    /** 多审核单位合并：各取首页第一页，按提交时间倒序合并 */
    async loadMergedUnits(status: string): Promise<AdminOrderListItem[]> {
      const units = reviewerEnterprises()
      const results = await Promise.all(
        units.map((u) =>
          http
            .get<any>('/admin/orders', {
              page: 1,
              page_size: PAGE_SIZE,
              status,
              enterprise_id: u.enterprise_id
            })
            .then((d) => normalizePage<AdminOrderListItem>(d).list)
            .catch(() => [] as AdminOrderListItem[])
        )
      )
      const all = results.reduce((acc, cur) => acc.concat(cur), [] as AdminOrderListItem[])
      all.sort((a, b) => {
        const ta = new Date(a.submitted_at || a.created_at).getTime()
        const tb = new Date(b.submitted_at || b.created_at).getTime()
        return tb - ta
      })
      return all
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

/* ── 「统计」Tab（C20） ── */
.stats-view {
  padding: 20rpx 24rpx;
}

.stats-card {
  background-color: #ffffff;
  border-radius: 16rpx;
  padding: 24rpx;
  margin-bottom: 20rpx;
}

.stats-overview {
  display: flex;
  align-items: center;
  justify-content: space-between;

  .so-main {
    display: flex;
    align-items: baseline;
  }

  .so-num {
    font-size: 64rpx;
    font-weight: 600;
    color: #1a1a1a;
    line-height: 1;
  }

  .so-unit {
    margin-left: 16rpx;
    font-size: 26rpx;
    color: #999999;
  }

  .so-side {
    text-align: right;
  }

  .so-range {
    display: block;
    font-size: 26rpx;
    color: #666666;
  }

  .so-upd {
    display: block;
    margin-top: 8rpx;
    font-size: 22rpx;
    color: #bbbbbb;
  }
}

.sc-title {
  font-size: 28rpx;
  font-weight: 600;
  color: #1a1a1a;
  margin-bottom: 20rpx;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
}

.chip {
  margin: 0 12rpx 12rpx 0;
  padding: 8rpx 28rpx;
  font-size: 24rpx;
  color: #666666;
  background-color: #f5f6f8;
  border-radius: 999rpx;

  &.active {
    color: #ffffff;
    background-color: #4d80f0;
  }
}

.custom-range {
  display: flex;
  align-items: center;
  margin-top: 8rpx;
}

.range-field {
  flex: 1;
  padding: 16rpx 20rpx;
  border: 1rpx solid #e5e6eb;
  border-radius: 12rpx;
  font-size: 26rpx;
  color: #1a1a1a;
  background-color: #fafafa;
}

.range-sep {
  margin: 0 16rpx;
  color: #999999;
  font-size: 24rpx;
}

.bar-item {
  margin-bottom: 20rpx;

  &:last-child {
    margin-bottom: 0;
  }
}

.bar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8rpx;
}

.bar-name {
  font-size: 26rpx;
  font-weight: 500;
}

.bar-count {
  font-size: 26rpx;
  color: #1a1a1a;
}

.bar-pct {
  font-size: 22rpx;
  color: #999999;
}

.bar-track {
  height: 12rpx;
  background-color: #f0f1f3;
  border-radius: 999rpx;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 999rpx;
}

.sc-empty {
  padding: 40rpx 0;
  text-align: center;
  font-size: 24rpx;
  color: #999999;
}

.sc-loading {
  padding: 8rpx 0 0;
  text-align: center;
  font-size: 22rpx;
  color: #bbbbbb;
}

.rank-item {
  display: flex;
  align-items: center;
  padding: 14rpx 0;
  border-bottom: 1rpx solid #f2f3f5;

  &:last-child {
    border-bottom: none;
  }
}

.rank-no {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36rpx;
  height: 36rpx;
  margin-right: 16rpx;
  border-radius: 8rpx;
  background-color: #f0f1f3;
  color: #666666;
  font-size: 22rpx;

  &.first {
    background-color: #4d80f0;
    color: #ffffff;
  }
}

.rank-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 52rpx;
  height: 52rpx;
  margin-right: 16rpx;
  border-radius: 50%;
  background-color: #4d80f0;
  color: #ffffff;
  font-size: 24rpx;
  overflow: hidden;
}

.rank-avatar-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.rank-avatar-txt {
  line-height: 1;
}

.rank-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 28rpx;
  color: #1a1a1a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-count {
  font-size: 24rpx;
  color: #999999;
}

.stats-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 80rpx 24rpx;
  background-color: #ffffff;
  border-radius: 16rpx;

  .stats-hint-icon {
    font-size: 64rpx;
  }

  .stats-hint-text {
    margin-top: 20rpx;
    font-size: 26rpx;
    color: #999999;
  }
}

/* 今日概况卡：标签在左、数值在右（C22 普通排布） */
.today-grid {
  display: flex;
  flex-wrap: wrap;
  margin: -6rpx;
}

.today-tile {
  box-sizing: border-box;
  width: 50%;
  padding: 6rpx;
}

.today-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22rpx 20rpx;
  border-radius: 12rpx;
  background-color: #f7f8fa;
}

.today-key {
  font-size: 24rpx;
  color: #666666;
}

.today-val {
  font-size: 32rpx;
  font-weight: 600;
  line-height: 1;
}

/* 历史统计：总工单数行（字段在前、数值在后） */
.total-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 20rpx 2rpx 0;
}

.total-left {
  display: flex;
  align-items: baseline;
  gap: 16rpx;
}

.total-key {
  font-size: 26rpx;
  color: #666666;
}

.total-val {
  font-size: 36rpx;
  font-weight: 600;
  color: #1a1a1a;
}

.total-meta {
  font-size: 20rpx;
  color: #999999;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 历史统计：指标胶囊 Tab（C22） */
.seg-row {
  display: flex;
  margin: 20rpx 0;
  padding: 4rpx;
  border-radius: 999rpx;
  background-color: #f2f3f5;
}

.seg {
  flex: 1;
  padding: 10rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: #666666;
  border-radius: 999rpx;

  &.active {
    background-color: #ffffff;
    color: #4d80f0;
    font-weight: 500;
  }
}
</style>
