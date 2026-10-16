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

    <!-- 上半：待办看板（按角色出卡；个人工单归「我的」组、其余归「待办」组；空卡不显示） -->
    <view class="board">
      <view v-if="loading && !cards.length" class="board-tip">加载中…</view>

      <template v-else>
        <template v-for="sec in boardSections" :key="sec.title">
          <!-- 分组小标题（与「待办」同源样式） -->
          <view class="board-hd">
            <text class="board-title">{{ sec.title }}</text>
            <text v-if="unreadTotal > 0 && sec.title === unreadSectionTitle" class="board-unread">{{ unreadTotal }} 条未读</text>
          </view>

          <view v-for="c in sec.cards" :key="c.key" class="bcard">
            <!-- 卡头：点击进该状态的工单列表 -->
            <view class="bcard-hd" @click="goCard(c)">
              <text class="bcard-label">{{ c.label }}</text>
              <text class="bcard-total">({{ c.total }})</text>
              <text class="bcard-more">›</text>
            </view>

            <!-- 卡体：最近几条工单，点击直达工单详情（返回键落回首页） -->
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
                <text class="orow-ent">{{ o.enterprise_name }}</text>
              </view>
            </view>

            <!-- 卡底：展开态恒显示「收起」；未展开且仅当还有更多工单时显示「查看全部」 -->
            <view
              v-if="expandedKeys[c.key] || c.total > c.orders.length"
              class="bcard-foot"
              @click="expandCard(c)"
            >
              <text class="bcard-foot-txt">
                {{ expandedKeys[c.key] ? '收起' : '查看全部 ' + c.total + ' 条' }}
              </text>
              <text class="bcard-foot-arw">{{ expandedKeys[c.key] ? '▲' : '▼' }}</text>
            </view>
          </view>
        </template>

        <view v-if="!visibleCards.length" class="board-tip">当前没有与您相关的待办</view>
      </template>
    </view>

    <!-- 下半：服务入口（线性 SVG 图标栅格） -->
    <view class="svc">
      <view class="svc-title">服务</view>
      <view class="svc-grid" :style="{ gridTemplateColumns: 'repeat(' + svcColumns + ', 1fr)' }">
        <view
          v-for="e in serviceEntries"
          :key="e.key"
          class="svc-item"
          @click="onEntry(e)"
        >
          <view
            class="svc-ico"
            :style="{ backgroundImage: 'url(' + iconUri(e.icon) + ')' }"
          ></view>
          <text class="svc-label">{{ e.title }}</text>
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
import { serviceIconUri } from './icons'
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
 * 「提醒」卡混装已完工/已退回/已取消，按该工单自身状态落到对应 Tab（已完成/已退回/已取消）；
 * 其余卡沿用卡片本身的 Tab。
 */
function orderTag(key: string, status: string): string {
  if (key === 'notice') {
    if (status === 'completed') return 'finished'
    if (status === 'cancelled') return 'closed'
    return 'rejected'
  }
  return cardTag(key)
}

interface ServiceEntry {
  key: string
  /** 图标 key（pages/home/icons.ts PATHS） */
  icon: string
  title: string
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
      /** init 防并发令牌：快速切换 tab / 双 onShow 时以最新一次为准 */
      initToken: 0,
      /** 各卡片展开态（key → 是否已展开全部） */
      expandedKeys: {} as Record<string, boolean>,
      /** 各卡片展开前的预览工单快照（用于收起时还原） */
      previewMap: {} as Record<string, DashboardOrder[]>,
      /** 正在加载全量的卡片 key（loading 态） */
      loadingCardKey: ''
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
    /** 空卡隐藏：后端已过滤一层，这里再兜一次底 */
    visibleCards(): DashboardCard[] {
      return this.cards.filter((c) => c.total > 0)
    },
    /** 个人工单卡（「我报修的」：后端 personal=true）→ 归「我的」分组 */
    personalCards(): DashboardCard[] {
      return this.visibleCards.filter((c) => c.personal)
    },
    /** 角色工作卡（待审核 / 待接单 / 处理中 …）→ 归「待办」分组 */
    workCards(): DashboardCard[] {
      return this.visibleCards.filter((c) => !c.personal)
    },
    /** 看板分组：个人在前、工作在后；空组不出现 */
    boardSections(): { title: string; cards: DashboardCard[] }[] {
      const secs: { title: string; cards: DashboardCard[] }[] = []
      if (this.personalCards.length) secs.push({ title: '我的', cards: this.personalCards })
      if (this.workCards.length) secs.push({ title: '待办', cards: this.workCards })
      return secs
    },
    /** 未读总数挂在首个分组标题上（个人组优先） */
    unreadSectionTitle(): string {
      if (this.personalCards.length) return '我的'
      if (this.workCards.length) return '待办'
      return ''
    },
    /** 下半部分服务入口（严格按角色互斥：审核员 / 业务员 / 普通用户） */
    serviceEntries(): ServiceEntry[] {
      if (isUnitReviewer()) {
        return [
          {
            key: 'ent_manage',
            icon: 'ent',
            title: '企业管理',
            action: () => this.goEnterpriseManage()
          },
          {
            key: 'my_orders',
            icon: 'my',
            title: '我的工单',
            action: () => this.goList('my_repair', '')
          },
          {
            key: 'create',
            icon: 'fix',
            title: '报修',
            action: () => this.goCreate()
          },
          {
            key: 'review',
            icon: 'rev',
            title: '审核工单',
            action: () => this.goList('review', 'pending_review')
          }
        ]
      }
      if (isStoreStaff()) {
        return [
          {
            key: 'hall',
            icon: 'hall',
            title: '接单大厅',
            action: () => this.goList('worker_task', 'pending_accept')
          },
          {
            key: 'my_repair_work',
            icon: 'my',
            title: '我的维修',
            action: () => this.goList('worker_task', 'mine')
          }
        ]
      }
      return [
        {
          key: 'my_orders',
          icon: 'my',
          title: '我的工单',
          action: () => this.goList('my_repair', '')
        },
        {
          key: 'create',
          icon: 'fix',
          title: '报修',
          action: () => this.goCreate()
        }
      ]
    },
    /** 图标栅格列数：≤4 按实际项数均分一行（审核员 4、业务员/普通 2） */
    svcColumns(): number {
      return Math.min(this.serviceEntries.length, 4) || 1
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
      this.fetchDashboard()
    },

    async fetchDashboard() {
      this.loading = true
      try {
        // 看板按角色出卡：拉取本人全部相关工单（审核员含其全部单位，不再按单单位筛选）
        const res = await dashboardAPI.get()
        this.cards = res.cards || []
        this.unreadTotal = res.unread_total || 0
        // 记录每张卡的预览快照，供「查看全部」收起时还原
        const preview: Record<string, DashboardOrder[]> = {}
        for (const c of this.cards) preview[c.key] = c.orders
        this.previewMap = preview
        this.expandedKeys = {}
      } catch (e) {
        this.cards = []
        this.unreadTotal = 0
      } finally {
        this.loading = false
      }
    },

    /** 点击卡片标题 → 工单列表并选中对应 Tab */
    goCard(c: DashboardCard) {
      this.goList(cardMode(c.key), cardTag(c.key))
    },

    /**
     * 点击卡片内的工单 → 直达工单详情页（不再经过列表中间页）。
     * 栈为 [首页 → 详情]，返回键直接落回首页。
     * 报修人视角（进行中 / 提醒）进用户端详情；审核 / 维修视角进管理端详情。
     */
    goDetail(c: DashboardCard, o: DashboardOrder) {
      const mode = cardMode(c.key)
      if (mode === 'my_repair') {
        uni.navigateTo({ url: `/pages/order/detail?id=${o.id}` })
      } else {
        uni.navigateTo({ url: `/pages/admin/order/detail?id=${o.id}` })
      }
    },

    /**
     * 卡片底部「查看全部 / 收起」：首次展开调全量接口突破 3 条上限；
     * 提醒卡后端仍只返回未读工单（仅全部未读）。再次点击收起回到预览快照。
     */
    async expandCard(c: DashboardCard) {
      if (this.expandedKeys[c.key]) {
        c.orders = this.previewMap[c.key] || []
        this.expandedKeys[c.key] = false
        return
      }
      this.loadingCardKey = c.key
      try {
        const res = await dashboardAPI.getCardDetail(c.key)
        if (res && res.orders) {
          this.previewMap[c.key] = c.orders
          c.orders = res.orders
          this.expandedKeys[c.key] = true
        }
      } catch (e) {
        uni.showToast({ title: '加载失败', icon: 'none' })
      } finally {
        this.loadingCardKey = ''
      }
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

    /** 图标 key → data-uri 背景图 */
    iconUri(key: string): string {
      return serviceIconUri(key)
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
  /* 底部留出 tabBar 高度 + 安全区 + 20px 呼吸位，避免最后一张卡片被固定导航栏遮挡 */
  padding: 24rpx 24rpx calc(var(--tabbar-height, 100rpx) + var(--safe-bottom, 0px) + 20px);
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
.bcard-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 14rpx 0 18rpx;
  border-top: 1rpx solid var(--xn-border);
}
.bcard-foot-txt {
  font-size: 24rpx;
  color: var(--xn-primary);
}
.bcard-foot-arw {
  margin-left: 8rpx;
  font-size: 20rpx;
  color: var(--xn-primary);
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
/* 每条工单来自的企业（右对齐，超长截断；与 Web 端看板一致） */
.orow-ent {
  flex-shrink: 0;
  max-width: 200rpx;
  margin-left: 12rpx;
  padding-left: 12rpx;
  border-left: 1rpx solid var(--xn-border);
  font-size: 22rpx;
  color: var(--xn-text-400);
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

/* 服务入口：纯净图标栅格（无卡片、无底色） */
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

.svc-grid {
  display: grid;
  row-gap: 16rpx;
}
.svc-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12rpx 0;
}
.svc-ico {
  width: 64rpx;
  height: 64rpx;
  background-size: contain;
  background-repeat: no-repeat;
  background-position: center;
}
.svc-label {
  margin-top: 12rpx;
  font-size: 24rpx;
  color: var(--xn-text-700);
}
</style>
