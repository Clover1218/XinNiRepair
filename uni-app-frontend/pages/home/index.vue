<template>
  <view class="home">
    <!-- 问候区 -->
    <view class="hd">
      <view class="hd-top">
        <text class="hd-hi">{{ greeting }}，{{ displayName }}</text>
        <wd-button v-if="!isLoggedIn" size="small" type="primary" @click="goLogin">去登录</wd-button>
      </view>
      <view class="hd-bot">
        <text class="hd-sub">{{ greetingSub }}</text>
        <text v-for="r in roleLabels" :key="r" class="hd-tag">{{ r }}</text>
      </view>
    </view>

    <!-- 多单位审核员：单位切换条 -->
    <view v-if="showUnitSwitch" class="unit-bar">
      <text class="unit-hint">您担任多个单位的审核员</text>
      <picker mode="selector" :range="unitNames" :value="unitIndex" @change="onUnitChange">
        <view class="unit-sel">
          <text class="unit-sel-txt">{{ activeUnitName }}</text>
          <text class="unit-sel-arw">▾</text>
        </view>
      </picker>
    </view>

    <!-- 上半：待办看板（按角色出卡，空卡不显示） -->
    <view class="board">
      <view class="board-hd">
        <text class="board-title">待办</text>
        <text v-if="unreadTotal > 0" class="board-unread">{{ unreadTotal }} 条未读</text>
      </view>

      <view v-if="loading && !cards.length" class="board-tip">加载中…</view>

      <template v-else>
        <view v-for="c in visibleCards" :key="c.key" class="bcard">
          <!-- 卡头：点击进该状态的工单列表 -->
          <view class="bcard-hd" @click="goCard(c)">
            <text class="bcard-label">{{ c.label }}</text>
            <text class="bcard-total">({{ c.total }})</text>
            <text v-if="c.unread > 0" class="bcard-badge">{{ c.unread > 99 ? '99+' : c.unread }}</text>
            <text class="bcard-more">›</text>
          </view>

          <!-- 卡体：最近几条工单，点击进入列表并自动打开详情（返回键落在列表页） -->
          <view v-if="!c.orders.length" class="bcard-empty">暂无</view>
          <view v-else class="bcard-list">
            <view
              v-for="o in c.orders"
              :key="o.id"
              class="orow"
              @click="goDetail(c, o)"
            >
              <view class="orow-dot" :class="{ 'orow-dot-on': o.unread }"></view>
              <text class="orow-tag" :class="tagCls(o.display_tag_type)">{{ o.display_tag }}</text>
              <text class="orow-desc">{{ o.description || '（无故障描述）' }}</text>
            </view>
          </view>
        </view>

        <view v-if="!visibleCards.length" class="board-tip">当前没有与您相关的待办</view>
      </template>
    </view>

    <!-- 下半：服务入口（常驻，不再受待办数量门控） -->
    <view class="svc">
      <view class="svc-title">服务</view>
      <view class="entry-grid">
        <view
          v-for="e in serviceEntries"
          :key="e.key"
          class="entry-card"
          @click="onEntry(e)"
        >
          <view class="entry-ico" :class="e.cls">
            <text class="entry-ico-txt">{{ e.icon }}</text>
          </view>
          <text class="entry-title">{{ e.title }}</text>
          <text v-if="e.sub" class="entry-sub">{{ e.sub }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'
import { dashboardAPI } from '@/api'
import { isStoreStaff, isUnitReviewer, isSuperAdmin, reviewerEnterprises } from '@/utils/auth'
import { setTabBarSelected } from '@/utils/tabbar'
import type { DashboardCard, DashboardOrder } from '@/types'

/**
 * 卡片 key → 工单列表的 mode（与 pages/order/modes.ts 一致）
 * - 待审核            → 审核员视角 review
 * - 处理中 / 待接单    → 维修业务员视角 worker_task
 * - 进行中 / 提醒      → 申报人视角 my_repair
 */
function cardMode(key: string): string {
  if (key === 'pending_review') return 'review'
  if (key === 'repairer_processing' || key === 'pending_accept') return 'worker_task'
  return 'my_repair'
}

/**
 * 卡片 key → 工单列表要选中的 Tab key。
 * 进行中 / 提醒两张卡命中多个状态，统一落在「全部」Tab（其排序与看板完全一致）。
 */
function cardTag(key: string): string {
  switch (key) {
    case 'pending_review':
      return 'pending_review'
    case 'repairer_processing':
      return 'processing'
    case 'pending_accept':
      return 'pending_accept'
    default:
      return '' // my_repair 的「全部」Tab
  }
}

/**
 * 卡内单条工单 → 目标 Tab key。
 * 「提醒」卡混装已完工与已退回，按该工单自身状态落到对应 Tab（已完成 / 已退回）；
 * 其余卡沿用卡片本身的 Tab。
 */
function orderTag(key: string, status: string): string {
  if (key === 'notice') {
    return status === 'completed' ? 'finished' : 'rejected'
  }
  return cardTag(key)
}

interface ServiceEntry {
  key: string
  icon: string
  title: string
  sub: string
  cls: string
  action: () => void
}

export default defineComponent({
  name: 'HomeIndex',
  data() {
    return {
      isLoggedIn: false,
      cards: [] as DashboardCard[],
      unreadTotal: 0,
      loading: false,
      /** 审核员当前选中的单位 */
      activeUnitId: '',
      /** init 防并发令牌：快速切换 tab / 双 onShow 时以最新一次为准 */
      initToken: 0
    }
  },
  computed: {
    userStore() {
      return useUserStore()
    },
    displayName(): string {
      return this.userStore.userInfo?.nickname || '您好'
    },
    roleLabels(): string[] {
      const out: string[] = []
      if (isSuperAdmin()) out.push('超级管理员')
      else if (isStoreStaff()) out.push('维修业务员')
      if (isUnitReviewer()) out.push('单位审核员')
      return out
    },
    greeting(): string {
      const h = new Date().getHours()
      if (h < 6) return '凌晨好'
      if (h < 12) return '早上好'
      if (h < 14) return '中午好'
      if (h < 18) return '下午好'
      return '晚上好'
    },
    greetingSub(): string {
      const d = new Date()
      const date = `${d.getMonth() + 1}月${d.getDate()}日`
      if (!this.isLoggedIn) return `${date} · 登录后查看您的待办`
      return this.unreadTotal > 0
        ? `${date} · 您有 ${this.unreadTotal} 项新进展`
        : `${date} · 暂无新进展`
    },
    /** 本人担任审核员的单位 */
    unitList(): { enterprise_id: string; enterprise_name: string }[] {
      return reviewerEnterprises()
    },
    unitNames(): string[] {
      return this.unitList.map((u) => u.enterprise_name)
    },
    unitIndex(): number {
      const i = this.unitList.findIndex((u) => u.enterprise_id === this.activeUnitId)
      return i < 0 ? 0 : i
    },
    /** 仅多单位才显示切换条 */
    showUnitSwitch(): boolean {
      return isUnitReviewer() && this.unitList.length > 1
    },
    activeUnitName(): string {
      return this.unitList[this.unitIndex]?.enterprise_name || '我的单位'
    },
    /** 空卡隐藏：后端已过滤一层，这里再兜一次底 */
    visibleCards(): DashboardCard[] {
      return this.cards.filter((c) => c.total > 0)
    },
    /** 下半部分服务入口（严格按角色互斥：审核员 / 业务员 / 普通用户） */
    serviceEntries(): ServiceEntry[] {
      if (isUnitReviewer()) {
        return [
          {
            key: 'ent_manage',
            icon: '企',
            title: '企业管理',
            sub: '成员与邀请码',
            cls: 'reviewer',
            action: () => this.goEnterpriseManage()
          },
          {
            key: 'my_orders',
            icon: '单',
            title: '我的工单',
            sub: '我报修的',
            cls: 'plain',
            action: () => this.goList('my_repair', '')
          },
          {
            key: 'create',
            icon: '报',
            title: '报修',
            sub: '提交新工单',
            cls: 'primary',
            action: () => this.goCreate()
          },
          {
            key: 'review',
            icon: '审',
            title: '审核工单',
            sub: '本单位待审核',
            cls: 'reviewer',
            action: () => this.goList('review', 'pending_review')
          }
        ]
      }
      if (isStoreStaff()) {
        return [
          {
            key: 'hall',
            icon: '接',
            title: '接单大厅',
            sub: '可接的工单',
            cls: 'staff',
            action: () => this.goList('worker_task', 'pending_accept')
          },
          {
            key: 'my_repair_work',
            icon: '修',
            title: '我的维修',
            sub: '我接的工单',
            cls: 'staff',
            action: () => this.goList('worker_task', 'mine')
          }
        ]
      }
      return [
        {
          key: 'my_orders',
          icon: '单',
          title: '我的工单',
          sub: '查看处理进度与结果',
          cls: 'plain',
          action: () => this.goList('my_repair', '')
        },
        {
          key: 'create',
          icon: '报',
          title: '报修',
          sub: '提交新工单',
          cls: 'primary',
          action: () => this.goCreate()
        }
      ]
    }
  },
  methods: {
    async init() {
      const token = ++this.initToken
      if (!this.userStore.token) {
        const ok = await this.userStore.ensureLoggedIn()
        if (!ok) {
          this.isLoggedIn = false
          this.cards = []
          return
        }
      }
      this.isLoggedIn = true
      try {
        await this.userStore.fetchUserInfo()
      } catch (e) {
        /* 用户信息系统异常不阻塞首页渲染 */
      }
      // 若期间又触发了一次 init（快速切换 tab / 双 onShow），放弃本次，以最新为准
      if (token !== this.initToken) return
      if (!this.activeUnitId) {
        this.activeUnitId = this.unitList[0]?.enterprise_id || ''
      }
      this.fetchDashboard()
    },

    async fetchDashboard() {
      this.loading = true
      try {
        // 审核员带上当前所选单位；其余角色后端忽略该参数
        const params = isUnitReviewer() && this.activeUnitId ? { enterprise_id: this.activeUnitId } : undefined
        const res = await dashboardAPI.get(params)
        this.cards = res.cards || []
        this.unreadTotal = res.unread_total || 0
      } catch (e) {
        this.cards = []
        this.unreadTotal = 0
      } finally {
        this.loading = false
      }
    },

    /** 切换单位：按新单位重拉看板 */
    onUnitChange(e: { detail: { value: number } }) {
      this.activeUnitId = this.unitList[e.detail.value]?.enterprise_id || ''
      this.fetchDashboard()
    },

    /** 点击卡片标题 → 工单列表并选中对应 Tab */
    goCard(c: DashboardCard) {
      this.goList(cardMode(c.key), cardTag(c.key))
    },

    /**
     * 点击卡片内的工单 → 先进「已按该状态筛选好的列表页」，由列表页自动打开详情。
     * 栈为 [首页 → 列表 → 详情]，返回键落在列表页而不是直接回首页。
     */
    goDetail(c: DashboardCard, o: DashboardOrder) {
      this.goList(cardMode(c.key), orderTag(c.key, o.status), o.id)
    },

    /**
     * 跳工单列表页。
     * V1.7：工单列表不再是 tabBar 页，可用 navigateTo 直接带参；
     * open 非空时列表页会就位后自动打开该工单详情。
     */
    goList(mode: string, tag: string, openId?: string) {
      let url = `/pages/order/list?mode=${mode}`
      if (tag) url += `&tag=${tag}`
      if (openId) url += `&open=${openId}`
      uni.navigateTo({ url })
    },

    /** 企业管理：单单位直达详情，多单位先选单位 */
    goEnterpriseManage() {
      const units = reviewerEnterprises()
      if (units.length === 1) {
        uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${units[0].enterprise_id}` })
        return
      }
      if (units.length > 1) {
        uni.showActionSheet({
          itemList: units.map((u) => u.enterprise_name),
          success: (res) => {
            const target = units[res.tapIndex]
            if (target) {
              uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${target.enterprise_id}` })
            }
          }
        })
        return
      }
      uni.showToast({ title: '您还不是任何单位的审核员', icon: 'none' })
    },

    goCreate() {
      uni.navigateTo({ url: '/pages/order/edit' })
    },

    onEntry(e: ServiceEntry) {
      e.action()
    },

    /** display_tag_type → 胶囊配色 */
    tagCls(type: string): string {
      switch (type) {
        case 'warning':
          return 'tag-warning'
        case 'danger':
          return 'tag-danger'
        case 'success':
          return 'tag-success'
        case 'primary':
          return 'tag-primary'
        default:
          return 'tag-info'
      }
    },

    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    }
  },
  onShow() {
    setTabBarSelected(0)
    this.init()
  }
})
</script>

<style lang="scss" scoped>
.home {
  padding: 24rpx 24rpx 40rpx;
}

/* 问候区 */
.hd {
  padding: 8rpx 4rpx 24rpx;
}
.hd-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.hd-hi {
  font-size: 36rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.hd-bot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  margin-top: 10rpx;
}
.hd-sub {
  font-size: 24rpx;
  color: var(--xn-text-500);
}
.hd-tag {
  margin-left: 12rpx;
  padding: 4rpx 14rpx;
  border-radius: 6rpx;
  background: var(--xn-primary-soft);
  color: var(--xn-primary);
  font-size: 22rpx;
}

/* 单位切换条 */
.unit-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
  background: var(--xn-surface);
  border: 1rpx solid var(--xn-border);
  border-radius: var(--xn-radius-card);
}
.unit-hint {
  font-size: 26rpx;
  color: var(--xn-text-500);
}
.unit-sel {
  display: flex;
  align-items: center;
  padding: 8rpx 20rpx;
  background: var(--xn-primary-soft);
  border-radius: var(--xn-radius-sm);
}
.unit-sel-txt {
  font-size: 26rpx;
  color: var(--xn-primary);
  font-weight: 500;
}
.unit-sel-arw {
  margin-left: 8rpx;
  font-size: 22rpx;
  color: var(--xn-primary);
}

/* 待办看板 */
.board {
  margin-bottom: 32rpx;
}
.board-hd {
  display: flex;
  align-items: center;
  padding: 0 4rpx 16rpx;
}
.board-title {
  font-size: 26rpx;
  font-weight: 600;
  color: var(--xn-text-700);
}
.board-unread {
  margin-left: auto;
  font-size: 24rpx;
  color: #dc2626;
}
.board-tip {
  padding: 40rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: var(--xn-text-400);
  background: var(--xn-surface);
  border-radius: var(--xn-radius-card);
}

.bcard {
  margin-bottom: 16rpx;
  background: var(--xn-surface);
  border: 1rpx solid var(--xn-border);
  border-radius: var(--xn-radius-card);
  overflow: hidden;
}
.bcard-hd {
  display: flex;
  align-items: center;
  padding: 20rpx 24rpx 16rpx;
}
.bcard-label {
  font-size: 30rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.bcard-total {
  margin-left: 8rpx;
  font-size: 26rpx;
  color: var(--xn-text-500);
}
.bcard-badge {
  margin-left: 12rpx;
  min-width: 32rpx;
  padding: 2rpx 12rpx;
  border-radius: 18rpx;
  background: #dc2626;
  color: #fff;
  font-size: 22rpx;
  text-align: center;
}
.bcard-more {
  margin-left: auto;
  font-size: 30rpx;
  color: var(--xn-text-400);
}
.bcard-empty {
  padding: 0 24rpx 24rpx;
  font-size: 24rpx;
  color: var(--xn-text-400);
}
.bcard-list {
  padding: 0 24rpx 12rpx;
}
.orow {
  display: flex;
  align-items: center;
  padding: 16rpx 0;
  border-top: 1rpx solid var(--xn-border);
}
.orow-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  margin-right: 12rpx;
  flex-shrink: 0;
}
.orow-dot-on {
  background: #dc2626;
}
.orow-tag {
  flex-shrink: 0;
  padding: 2rpx 12rpx;
  border-radius: 6rpx;
  font-size: 22rpx;
  background: var(--xn-tag-bg);
  color: var(--xn-text-600);
}
.orow-desc {
  flex: 1;
  min-width: 0;
  margin-left: 12rpx;
  font-size: 26rpx;
  color: var(--xn-text-800);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 状态胶囊配色（与卡片组件一致的四色语义） */
.tag-warning {
  background: #fff7ed;
  color: #ea580c;
}
.tag-danger {
  background: #fef2f2;
  color: #dc2626;
}
.tag-success {
  background: #e6fffb;
  color: #0f766e;
}
.tag-primary {
  background: #eff6ff;
  color: #2563eb;
}
.tag-info {
  background: var(--xn-tag-bg);
  color: var(--xn-text-600);
}

/* 服务入口 */
.svc {
  margin-top: 8rpx;
}
.svc-title {
  display: block;
  padding: 0 4rpx 16rpx;
  font-size: 26rpx;
  font-weight: 600;
  color: var(--xn-text-700);
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}
.entry-card {
  padding: 24rpx 20rpx;
  background: var(--xn-surface);
  border: 1rpx solid var(--xn-border);
  border-radius: var(--xn-radius-card);
}
.entry-ico {
  width: 56rpx;
  height: 56rpx;
  border-radius: 14rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 14rpx;
}
.entry-ico-txt {
  font-size: 26rpx;
  font-weight: 600;
}
.entry-ico.primary {
  background: var(--xn-primary-soft);
}
.entry-ico.primary .entry-ico-txt {
  color: var(--xn-primary);
}
.entry-ico.staff {
  background: #fff7ed;
}
.entry-ico.staff .entry-ico-txt {
  color: #ea580c;
}
.entry-ico.reviewer {
  background: #fffbeb;
}
.entry-ico.reviewer .entry-ico-txt {
  color: #b45309;
}
.entry-ico.plain {
  background: var(--xn-tag-bg);
}
.entry-ico.plain .entry-ico-txt {
  color: var(--xn-text-700);
}
.entry-title {
  display: block;
  font-size: 28rpx;
  color: var(--xn-text-900);
}
.entry-sub {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: var(--xn-text-500);
}
</style>
