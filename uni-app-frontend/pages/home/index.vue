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

    <!-- 待办区块（按角色渲染） -->
    <view v-for="s in sections" :key="s.key" class="sect">
      <view class="sect-hd">
        <view class="sect-hd-left">
          <text class="sect-title">{{ s.title }}</text>
          <text v-if="s.total > 0" class="sect-badge">{{ s.total }}</text>
        </view>
        <text class="sect-more" @click="goOrders(s)">详情 ›</text>
      </view>
      <view class="sect-hint">{{ s.hint }}</view>

      <view v-if="s.loading && !s.list.length" class="sect-tip">加载中…</view>
      <template v-else>
        <order-card v-for="o in s.list" :key="o.id" :item="o" @click="goDetail(o)" />
        <view v-if="!s.list.length" class="sect-empty">暂无工单</view>
      </template>
    </view>

    <!-- 普通报修用户（无管理待办）：快捷入口 -->
    <block v-if="isLoggedIn && !sections.length">
      <view class="quick">
        <view class="quick-card" @click="goCreate">
          <view class="quick-ico quick-ico-primary">
            <text class="quick-ico-txt">报</text>
          </view>
          <view class="quick-txt">
            <text class="quick-title">设备出问题了？</text>
            <text class="quick-sub">填写故障信息，师傅很快联系你</text>
          </view>
          <text class="quick-arrow">›</text>
        </view>
        <view class="quick-card" @click="goMyOrders">
          <view class="quick-ico">
            <text class="quick-ico-txt">单</text>
          </view>
          <view class="quick-txt">
            <text class="quick-title">我的报修记录</text>
            <text class="quick-sub">查看处理进度与结果</text>
          </view>
          <text class="quick-arrow">›</text>
        </view>
      </view>
      <view class="guest-note">您暂无待处理的工单任务</view>
    </block>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'
import { useOrderStore } from '@/stores/order'
import { adminAPI } from '@/api'
import { isStoreStaff, isUnitReviewer, isSuperAdmin, reviewerEnterprises } from '@/utils/auth'
import type { AdminOrderListItem } from '@/types'

/** 每区块预览条数 */
const PREVIEW_SIZE = 5

interface TodoSection {
  key: string
  title: string
  status: string
  hint: string
  /** 是否受所选单位限定（仅审核员的「待审核」区块） */
  unitScoped: boolean
  list: AdminOrderListItem[]
  total: number
  loading: boolean
}

export default defineComponent({
  name: 'HomeIndex',
  data() {
    return {
      isLoggedIn: false,
      sections: [] as TodoSection[],
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
    orderStore() {
      return useOrderStore()
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
      const total = this.sections.reduce((sum, s) => sum + s.total, 0)
      if (!this.isLoggedIn) return `${date} · 登录后查看您的待办`
      return total > 0 ? `${date} · 您有 ${total} 项待办` : `${date} · 暂无待办`
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
      return this.unitList.length > 1
    },
    activeUnitName(): string {
      return this.unitList[this.unitIndex]?.enterprise_name || '我的单位'
    }
  },
  methods: {
    async init() {
      const token = ++this.initToken
      if (!this.userStore.token) {
        const ok = await this.userStore.ensureLoggedIn()
        if (!ok) {
          this.isLoggedIn = false
          this.sections = []
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
      this.buildSections()
    },

    /**
     * 按角色构建待办区块（移植自 Web 端 views/home/index.vue）：
     * - 单位审核员：待审核（限定所选单位）
     * - 店方：待接单 + 处理中
     * - 兼任角色：取并集
     */
    /**
     * 按角色构建待办区块（移植自 Web 端 views/home/index.vue）：
     * - 单位审核员：待审核（限定所选单位）
     * - 店方：待接单 + 处理中
     * - 兼任角色：取并集
     *
     * 关键修复：复用上一轮已加载的 list，避免切回首页时整块清空再重拉导致的
     * “加载不出来 / 一闪而过”；嵌套对象属性变更后重新赋值数组引用，确保重渲染。
     */
    async buildSections() {
      const wanted: { key: string; title: string; status: string; hint: string; unitScoped: boolean }[] = []
      if (isUnitReviewer()) {
        wanted.push({
          key: 'reported',
          title: '待审核',
          status: 'reported',
          hint: '等待您确认后，转给维修业务员处理',
          unitScoped: true
        })
      }
      if (isStoreStaff()) {
        wanted.push({
          key: 'pending_accept',
          title: '待接单',
          status: 'pending_accept',
          hint: '审核已通过，可以接单处理',
          unitScoped: false
        })
        wanted.push({
          key: 'processing',
          title: '处理中',
          status: 'processing',
          hint: '已接单，等待完工结单',
          unitScoped: false
        })
      }
      // 复用已有区块的已加载数据，避免重建为空导致空白闪烁
      const prevMap: Record<string, TodoSection> = {}
      this.sections.forEach((s) => {
        prevMap[s.key] = s
      })
      this.sections = wanted.map((w) => {
        const old = prevMap[w.key]
        return old
          ? { ...w, list: old.list, total: old.total, loading: old.loading }
          : { ...w, list: [], total: 0, loading: false }
      })
      // 拉取各区块数据，全部完成后按待办数降序排一次（并列保持角色原始顺序）
      await Promise.all(this.sections.map((s) => this.fetchSection(s)))
      this.sortSectionsOnce()
    },

    /** 待办区块按待办数降序排一次（稳定排序：并列保持原顺序） */
    sortSectionsOnce() {
      this.sections = [...this.sections].sort((a, b) => b.total - a.total)
    },

    async fetchSection(s: TodoSection) {
      s.loading = true
      try {
        const res = await adminAPI.getOrders({
          page: 1,
          page_size: PREVIEW_SIZE,
          status: s.status,
          sort_by: 'submitted_at',
          sort_order: 'desc',
          // 单位限定的区块带上所选单位，保证与全局视角一致
          ...(s.unitScoped && this.activeUnitId ? { enterprise_id: this.activeUnitId } : {})
        })
        s.list = res.list || []
        s.total = res.total || 0
      } catch (e) {
        s.list = []
        s.total = 0
      } finally {
        s.loading = false
        // 重新赋值数组引用，确保嵌套对象属性变更触发重渲染
        this.sections = [...this.sections]
      }
    },

    /** 切换单位：只刷新受单位限定的区块 */
    onUnitChange(e: { detail: { value: number } }) {
      this.activeUnitId = this.unitList[e.detail.value]?.enterprise_id || ''
      this.sections.filter((s) => s.unitScoped).forEach((s) => this.fetchSection(s))
    },

    /** 「详情 ›」→ 工单 tab；企业/用户默认全部（工单页据此重置），状态=对应区块 */
    goOrders(s: TodoSection) {
      this.orderStore.setPendingJump(s.status, '')
      uni.switchTab({ url: '/pages/order/list' })
    },

    goDetail(o: AdminOrderListItem) {
      uni.navigateTo({ url: `/pages/admin/order/detail?id=${o.id}` })
    },

    goCreate() {
      uni.navigateTo({ url: '/pages/order/edit' })
    },

    goMyOrders() {
      uni.switchTab({ url: '/pages/order/list' })
    },

    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    }
  },
  onShow() {
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

/* 待办区块 */
.sect {
  margin-bottom: 32rpx;
}
.sect-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4rpx;
}
.sect-hd-left {
  display: flex;
  align-items: center;
}
.sect-title {
  font-size: 32rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.sect-badge {
  margin-left: 12rpx;
  min-width: 36rpx;
  padding: 2rpx 12rpx;
  border-radius: 18rpx;
  background: var(--xn-primary);
  color: #fff;
  font-size: 22rpx;
  text-align: center;
}
.sect-more {
  font-size: 26rpx;
  color: var(--xn-primary);
}
.sect-hint {
  padding: 6rpx 4rpx 16rpx;
  font-size: 24rpx;
  color: var(--xn-text-400);
}
.sect-tip,
.sect-empty {
  padding: 40rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: var(--xn-text-400);
  background: var(--xn-surface);
  border-radius: var(--xn-radius-card);
}

/* 普通用户快捷入口 */
.quick-card {
  display: flex;
  align-items: center;
  padding: 28rpx 24rpx;
  margin-bottom: 20rpx;
  background: var(--xn-surface);
  border-radius: var(--xn-radius-card);
  border: 1rpx solid var(--xn-border);
}
.quick-ico {
  width: 76rpx;
  height: 76rpx;
  border-radius: 18rpx;
  background: var(--xn-tag-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
  flex-shrink: 0;
}
.quick-ico-primary {
  background: var(--xn-primary-soft);
}
.quick-ico-txt {
  font-size: 32rpx;
  font-weight: 600;
  color: var(--xn-text-700);
}
.quick-ico-primary .quick-ico-txt {
  color: var(--xn-primary);
}
.quick-txt {
  flex: 1;
  min-width: 0;
}
.quick-title {
  display: block;
  font-size: 30rpx;
  color: var(--xn-text-900);
}
.quick-sub {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: var(--xn-text-500);
}
.quick-arrow {
  font-size: 34rpx;
  color: var(--xn-text-400);
}
.guest-note {
  padding: 24rpx 0;
  text-align: center;
  font-size: 24rpx;
  color: var(--xn-text-400);
}
</style>
