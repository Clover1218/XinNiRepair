<template>
  <view class="page">
    <!-- 问候区 -->
    <view class="greeting">
      <view class="hi">你好，{{ greetingName }}</view>
      <view class="sub">{{ greetingSub }}</view>
      <wd-button v-if="!isLoggedIn" type="primary" round size="small" @click="goLogin">
        去登录
      </wd-button>
    </view>

    <!-- 我的服务 -->
    <view class="group">
      <view class="group-title">我的服务</view>
      <view class="entry-list">
        <view class="entry" @click="go('/pages/order/list')">
          <view class="entry-icon mine">📋</view>
          <view class="entry-text">我的工单</view>
          <text class="entry-arrow">›</text>
        </view>
      </view>
    </view>

    <!-- 工作服务（按角色渲染；无可见入口时整组隐藏） -->
    <view v-if="hasWorkService" class="group">
      <view class="group-title">工作服务</view>
      <view class="entry-list">
        <view v-if="vis.reviewOrders" class="entry" @click="goReview">
          <view class="entry-icon review">🔍</view>
          <view class="entry-text">审核工单</view>
          <text class="entry-arrow">›</text>
        </view>
        <view v-if="vis.repairOrders" class="entry" @click="go('/pages/admin/order/list')">
          <view class="entry-icon repair">🔧</view>
          <view class="entry-text">处理工单</view>
          <text class="entry-arrow">›</text>
        </view>
        <view v-if="vis.enterprise" class="entry" @click="goEnterprise">
          <view class="entry-icon ent">🏢</view>
          <view class="entry-text">企业管理</view>
          <text class="entry-arrow">›</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import {
  serviceEntryVisibility,
  isStoreStaff,
  isUnitReviewer,
  reviewerEnterprises
} from '@/utils/auth'
import type { ServiceEntryVisibility } from '@/utils/auth'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'

const DEFAULT_VIS: ServiceEntryVisibility = {
  myOrders: true,
  reviewOrders: false,
  repairOrders: false,
  enterprise: false
}

export default defineComponent({
  setup() {
    return { userStore: useUserStore(), enterpriseStore: useEnterpriseStore() }
  },
  data() {
    return {
      isLoggedIn: false,
      vis: { ...DEFAULT_VIS } as ServiceEntryVisibility
    }
  },
  computed: {
    greetingName(): string {
      return this.isLoggedIn ? this.userStore.userInfo?.nickname || '微信用户' : '请先登录'
    },
    greetingSub(): string {
      return this.isLoggedIn ? '欢迎回来' : '登录后可发起报修、跟进处理进度'
    },
    hasWorkService(): boolean {
      return this.vis.reviewOrders || this.vis.repairOrders || this.vis.enterprise
    }
  },
  onShow() {
    this.init()
  },
  methods: {
    async init() {
      if (!this.userStore.token) {
        const ok = await this.userStore.ensureLoggedIn()
        if (!ok) {
          this.isLoggedIn = false
          this.vis = { ...DEFAULT_VIS }
          return
        }
      }
      this.isLoggedIn = true
      try {
        await this.userStore.fetchUserInfo()
      } catch (e) {
        // 401 由请求层静默续期/清态兜底
      }
      this.vis = serviceEntryVisibility()
    },
    async go(url: string) {
      if (!(await this.ensureLogin())) return
      uni.navigateTo({ url })
    },
    async goReview() {
      if (!(await this.ensureLogin())) return
      if (!isUnitReviewer() && !isStoreStaff()) {
        uni.showToast({ title: '无审核权限', icon: 'none' })
        return
      }
      uni.navigateTo({ url: '/pages/review/index' })
    },
    async goEnterprise() {
      if (!(await this.ensureLogin())) return
      if (isStoreStaff()) {
        uni.navigateTo({ url: '/pages/admin/enterprise/list' })
        return
      }
      // 审核员（非店方）：单个审核单位直达详情，多个单位弹选
      const units = reviewerEnterprises()
      if (units.length === 0) {
        uni.showToast({ title: '暂无可管理的单位', icon: 'none' })
        return
      }
      if (units.length === 1) {
        uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${units[0].enterprise_id}` })
        return
      }
      uni.showActionSheet({
        itemList: units.map((u) => u.enterprise_name),
        success: (res) => {
          const target = units[res.tapIndex]
          if (target) {
            uni.navigateTo({ url: `/pages/admin/enterprise/detail?id=${target.enterprise_id}` })
          }
        }
      })
    },
    async ensureLogin(): Promise<boolean> {
      if (this.userStore.token) return true
      const ok = await this.userStore.ensureLoggedIn()
      if (ok) {
        await this.init()
        return true
      }
      uni.navigateTo({ url: '/pages/auth/login' })
      return false
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding: 0 24rpx 60rpx;
  box-sizing: border-box;
  background-color: #f5f6f8;
}

.greeting {
  padding: 48rpx 8rpx 40rpx;

  .hi {
    font-size: 44rpx;
    font-weight: 700;
    color: #1a1a1a;
  }

  .sub {
    margin-top: 12rpx;
    margin-bottom: 24rpx;
    font-size: 28rpx;
    color: #999999;
  }
}

.group {
  margin-bottom: 40rpx;

  .group-title {
    margin: 0 8rpx 20rpx;
    font-size: 28rpx;
    font-weight: 600;
    color: #666666;
  }
}

.entry-list {
  background-color: #ffffff;
  border-radius: 20rpx;
  overflow: hidden;

  .entry {
    display: flex;
    align-items: center;
    padding: 32rpx 28rpx;
    border-bottom: 1rpx solid #f2f3f5;

    &:last-child {
      border-bottom: none;
    }

    .entry-icon {
      width: 64rpx;
      height: 64rpx;
      border-radius: 16rpx;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 32rpx;
      flex-shrink: 0;

      &.mine {
        background: rgba(77, 128, 240, 0.12);
      }

      &.review {
        background: rgba(255, 193, 7, 0.16);
      }

      &.repair {
        background: rgba(7, 193, 96, 0.14);
      }

      &.ent {
        background: rgba(153, 153, 153, 0.14);
      }
    }

    .entry-text {
      flex: 1;
      margin-left: 24rpx;
      font-size: 30rpx;
      color: #1a1a1a;
      display: flex;
      align-items: center;
    }

    .entry-arrow {
      font-size: 34rpx;
      color: #cccccc;
    }
  }
}
</style>
