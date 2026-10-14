<template>
  <view class="page">
    <!-- 未登录：展示未登录卡片 + 登录按钮 -->
    <view v-if="!isLoggedIn" class="not-logged-in">
      <wd-avatar
        class="user-avatar"
        text="登"
        shape="round"
        size="large"
        bg-color="#0f766e"
        color="#ffffff"
      ></wd-avatar>
      <view class="not-logged-info">
        <view class="not-logged-text">未登录</view>
        <view class="not-logged-tip">登录后查看个人信息与企业</view>
      </view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <template v-else>
      <!-- 用户信息卡片 -->
      <view class="user-card">
        <wd-avatar
          class="user-avatar"
          :src="avatarUrl"
          :text="avatarText"
          shape="round"
          size="large"
          bg-color="#0f766e"
          color="#ffffff"
        ></wd-avatar>
        <view class="user-info">
          <view class="user-name">
            {{ userInfo?.nickname || '微信用户' }}
            <text class="role-badge" :class="`role-${platformRoleValue}`">{{ platformRoleText }}</text>
          </view>
          <view class="user-phone">{{ maskPhone(userInfo?.phone) || '未绑定手机号' }}</view>
        </view>
      </view>

      <!-- 我的企业 -->
      <view class="ent-title">
        <text>我的企业</text>
        <text class="ent-add" @click="goJoin">+ 添加</text>
      </view>
      <view class="ent-list">
        <view
          v-for="ent in enterprises"
          :key="ent.enterprise_id"
          class="ent-card"
          :class="{ current: ent.enterprise_id === currentEnterpriseId }"
          @click="onSwitchEnterprise(ent)"
        >
          <view class="ent-name-wrap">
            <text class="ent-name">{{ ent.enterprise_name }}</text>
            <text v-if="ent.enterprise_id === currentEnterpriseId" class="ent-check">✓</text>
            <text v-if="ent.role === 'reviewer' || ent.role === 'admin'" class="ent-badge reviewer">
              审核员
            </text>
          </view>
          <view class="ent-meta">
            <text class="ent-role">{{ roleLabel(ent.role) }}</text>
            <text class="ent-status" :class="ent.status">{{ memberStatusLabel(ent.status) }}</text>
          </view>
        </view>

        <view v-if="enterprises.length === 0" class="ent-empty">
          <text>还没有加入任何企业</text>
        </view>
      </view>

      <!-- 管理入口：仅店方 / 单位审核员可见。
           注：统计页、Excel 导出、超管四件套（项目字典/用户管理/经营范围/系统设置）
           本期均不移植小程序，故此处不提供对应入口。 -->
      <template v-if="showManageEntry">
        <view class="ent-title"><text>管理</text></view>
        <view
          v-for="entry in manageEntries"
          :key="entry.key"
          class="entry-card"
          @click="entry.action"
        >
          <view class="entry-icon" :class="entry.cls">{{ entry.icon }}</view>
          <view class="entry-text">
            <view class="entry-title">{{ entry.title }}</view>
            <view class="entry-sub">{{ entry.sub }}</view>
          </view>
          <text class="entry-arrow">›</text>
        </view>
      </template>

      <!-- 协议入口 -->
      <view class="link-list">
        <view class="link-item" @click="openAgreement('user')">
          <text>用户协议</text>
          <text class="link-arrow">›</text>
        </view>
        <view class="link-item" @click="openAgreement('privacy')">
          <text>隐私政策</text>
          <text class="link-arrow">›</text>
        </view>
      </view>

      <!-- 退出登录 -->
      <view class="logout-wrap">
        <wd-button block plain round @click="onLogout">退出登录</wd-button>
      </view>
    </template>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'
import { maskPhone, roleLabel, memberStatusLabel } from '@/utils/format'
import { isStoreStaff, reviewerEnterprises, serviceEntryVisibility } from '@/utils/auth'
import { setTabBarSelected } from '@/utils/tabbar'
import { useOrderStore } from '@/stores/order'
import type { OrderMode } from '@/pages/order/modes'
import type { EnterpriseBrief, PlatformRole } from '@/types'

const ROLE_TEXT: Record<number, string> = {
  0: '普通用户',
  1: '维修业务员',
  2: '超级管理员'
}

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      enterpriseStore: useEnterpriseStore(),
      orderStore: useOrderStore(),
      maskPhone,
      roleLabel,
      memberStatusLabel
    }
  },
  data() {
    return {
      isLoggedIn: false
    }
  },
  computed: {
    userInfo() {
      return this.userStore.userInfo
    },
    enterprises() {
      return this.enterpriseStore.enterprises
    },
    currentEnterpriseId() {
      return this.enterpriseStore.currentEnterpriseId
    },
    avatarUrl() {
      return this.userInfo?.avatar_url || ''
    },
    avatarText() {
      return (this.userInfo?.nickname || '用').slice(0, 1)
    },
    platformRoleValue(): PlatformRole {
      return (this.userInfo?.role ?? 0) as PlatformRole
    },
    platformRoleText(): string {
      return ROLE_TEXT[this.platformRoleValue] || '普通用户'
    },
    /** 是否有管理身份（店方或单位审核员） */
    showManageEntry(): boolean {
      return isStoreStaff() || reviewerEnterprises().length > 0
    },
    /** 管理枢纽入口（V1.3 服务主页可见性矩阵驱动）：按角色显示 我的工单/审核工单/处理工单/企业管理 */
    manageEntries(): {
      key: string
      title: string
      sub: string
      icon: string
      cls: 'reviewer' | 'staff'
      action: () => void
    }[] {
      const vis = serviceEntryVisibility() as Record<string, boolean>
      const all: {
        key: string
        title: string
        sub: string
        icon: string
        cls: 'reviewer' | 'staff'
        action: () => void
      }[] = [
        {
          key: 'myOrders',
          title: '我的工单',
          sub: '我提交的报修记录',
          icon: '📋',
          cls: 'staff',
          action: () => this.goOrderList('my_repair')
        },
        {
          key: 'reviewOrders',
          title: '审核工单',
          sub: '待我审核的单位工单',
          icon: '✅',
          cls: 'reviewer',
          action: () => this.goOrderList('review')
        },
        {
          key: 'repairOrders',
          title: '处理工单',
          sub: '待我接单与处理',
          icon: '🔧',
          cls: 'staff',
          action: () => this.goOrderList('worker_task')
        },
        {
          key: 'enterprise',
          title: '企业管理',
          sub: '客户单位与成员管理',
          icon: '🏢',
          cls: 'reviewer',
          action: () => this.goEnterpriseManage()
        }
      ]
      return all.filter((e) => !!vis[e.key])
    }
  },
  onShow() {
    setTabBarSelected(2)
    this.init()
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
        // 401 已由请求层处理
      }
      // V1.3：若当前上下文单位已不是 approved（被移除/拒绝），store 会自动重置
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    openAgreement(type: string) {
      uni.navigateTo({ url: `/pages/auth/agreement?type=${type}` })
    },
    goJoin() {
      uni.navigateTo({ url: '/pages/enterprise/join' })
    },
    /**
     * 企业管理入口：仅「企业管理员（单位审核员）」可见（矩阵已控制显隐）。
     * - 单单位 → 直达本单位详情
     * - 多单位 → 弹出选择器
     */
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
      uni.showToast({ title: '暂无可管理的企业', icon: 'none' })
    },
    /**
     * 进入工单列表并显式指定 mode（我的工单/审核工单/处理工单）。
     * V1.7：order/list 已从 tabBar 移出，改为普通页面，可直接 navigateTo 带 mode 参数，
     * 不再需要 orderStore 中转（旧的 switchTab + setPendingJump 方案随之废弃）。
     */
    goOrderList(mode: OrderMode) {
      uni.navigateTo({ url: `/pages/order/list?mode=${mode}` })
    },
    /** V1.2：切换企业 = 纯本地上下文切换（approved 才可切换） */
    onSwitchEnterprise(ent: EnterpriseBrief) {
      if (ent.status !== 'approved') {
        uni.showToast({ title: '该企业申请尚未通过，暂不可切换', icon: 'none' })
        return
      }
      if (ent.enterprise_id === this.currentEnterpriseId) return
      this.enterpriseStore.setCurrent(ent.enterprise_id)
      uni.showToast({ title: `已切换到「${ent.enterprise_name}」`, icon: 'none' })
    },
    async onLogout() {
      const res = await uni.showModal({ title: '提示', content: '确定退出登录吗？' })
      if (!res.confirm) return
      this.userStore.logout()
      uni.reLaunch({ url: '/pages/auth/login' })
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

.not-logged-in {
  display: flex;
  align-items: center;
  background: linear-gradient(135deg, #0f766e 0%, #1aa392 100%);
  border-radius: 20rpx;
  padding: 40rpx 32rpx;
  color: #ffffff;

  .user-avatar {
    flex-shrink: 0;
  }

  .not-logged-info {
    flex: 1;
    margin-left: 28rpx;

    .not-logged-text {
      font-size: 36rpx;
      font-weight: 600;
    }

    .not-logged-tip {
      margin-top: 12rpx;
      font-size: 26rpx;
      opacity: 0.85;
    }
  }
}

.user-card {
  display: flex;
  align-items: center;
  background: linear-gradient(135deg, #0f766e 0%, #1aa392 100%);
  border-radius: 20rpx;
  padding: 40rpx 32rpx;
  color: #ffffff;

  .user-avatar {
    flex-shrink: 0;
  }

  .user-info {
    flex: 1;
    min-width: 0;
    margin-left: 28rpx;

    .user-name {
      display: flex;
      align-items: center;
      font-size: 36rpx;
      font-weight: 600;

      .role-badge {
        margin-left: 16rpx;
        flex-shrink: 0;
        font-size: 20rpx;
        font-weight: 400;
        padding: 4rpx 14rpx;
        border-radius: 999rpx;
        background-color: rgba(255, 255, 255, 0.22);

        &.role-1 {
          background-color: rgba(255, 193, 7, 0.9);
          color: #4a3200;
        }

        &.role-2 {
          background-color: rgba(255, 82, 82, 0.9);
          color: #ffffff;
        }
      }
    }

    .user-phone {
      margin-top: 12rpx;
      font-size: 26rpx;
      opacity: 0.85;
    }
  }
}

/* 入口卡 */
.entry-card {
  display: flex;
  align-items: center;
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx 32rpx;
  margin-top: 20rpx;

  .entry-icon {
    width: 72rpx;
    height: 72rpx;
    border-radius: 16rpx;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 36rpx;
    flex-shrink: 0;

    &.reviewer {
      background: rgba(255, 193, 7, 0.14);
    }

    &.staff {
      background: rgba(15, 118, 110, 0.12);
    }
  }

  .entry-text {
    flex: 1;
    margin-left: 24rpx;

    .entry-title {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .entry-sub {
      margin-top: 6rpx;
      font-size: 24rpx;
      color: #999999;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .entry-arrow {
    font-size: 36rpx;
    color: #cccccc;
  }
}

.ent-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 32rpx 8rpx 20rpx;
  font-size: 30rpx;
  font-weight: 600;
  color: #333333;

  .ent-add {
    font-size: 26rpx;
    font-weight: 400;
    color: #0f766e;
  }
}

.ent-list {
  .ent-card {
    background-color: #ffffff;
    border-radius: 20rpx;
    padding: 28rpx;
    margin-bottom: 20rpx;
    border: 2rpx solid transparent;

    &.current {
      border-color: #0f766e;
      background-color: rgba(15, 118, 110, 0.04);
    }

    .ent-name-wrap {
      display: flex;
      align-items: center;

      .ent-name {
        font-size: 30rpx;
        font-weight: 600;
        color: #1a1a1a;
      }

      .ent-check {
        margin-left: 12rpx;
        color: #0f766e;
        font-size: 28rpx;
        font-weight: 700;
      }

      .ent-badge {
        margin-left: 12rpx;
        font-size: 20rpx;
        padding: 2rpx 12rpx;
        border-radius: 999rpx;

        &.reviewer {
          color: #b26a00;
          background-color: rgba(255, 193, 7, 0.16);
        }
      }
    }

    .ent-meta {
      display: flex;
      align-items: center;
      margin-top: 12rpx;

      .ent-role {
        font-size: 24rpx;
        color: #999999;
      }

      .ent-status {
        margin-left: 16rpx;
        font-size: 22rpx;
        padding: 4rpx 14rpx;
        border-radius: 999rpx;
        background-color: #f5f6f8;
        color: #999999;

        &.approved {
          color: #07c160;
          background-color: rgba(7, 193, 96, 0.1);
        }

        &.pending {
          color: #ff976a;
          background-color: rgba(255, 151, 106, 0.1);
        }
      }
    }
  }

  .ent-empty {
    background-color: #ffffff;
    border-radius: 20rpx;
    padding: 60rpx 0;
    text-align: center;
    font-size: 26rpx;
    color: #999999;
  }
}

.link-list {
  background-color: #ffffff;
  border-radius: 20rpx;
  overflow: hidden;

  .link-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 28rpx;
    font-size: 28rpx;
    color: #333333;
    border-bottom: 1rpx solid #f2f3f5;

    &:last-child {
      border-bottom: none;
    }

    .link-arrow {
      font-size: 32rpx;
      color: #cccccc;
    }
  }
}

.logout-wrap {
  margin-top: 48rpx;
}
</style>
