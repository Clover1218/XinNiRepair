<template>
  <view class="page">
    <!-- 未登录: 展示未登录卡片 + 登录按钮 -->
    <view v-if="!isLoggedIn" class="not-logged-in">
      <wd-avatar
        class="user-avatar"
        text="登"
        shape="round"
        size="large"
        bg-color="#cccccc"
        color="#ffffff"
      ></wd-avatar>
      <view class="not-logged-info">
        <view class="not-logged-text">未登录</view>
        <view class="not-logged-tip">登录后查看个人信息与企业</view>
      </view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <!-- 已登录: 正常展示 -->
    <template v-else>
    <!-- 用户信息卡片 -->
    <view class="user-card">
      <wd-avatar
        class="user-avatar"
        :src="avatarUrl"
        :text="avatarText"
        shape="round"
        size="large"
        bg-color="#4d80f0"
        color="#ffffff"
      ></wd-avatar>
      <view class="user-info">
        <view class="user-name">{{ userInfo?.nickname || '微信用户' }}</view>
        <view class="user-phone">{{ maskPhone(userInfo?.phone) || '未绑定手机号' }}</view>
      </view>
    </view>

    <!-- 管理员入口（仅平台管理员 role===1 显示） -->
    <view v-if="isAdmin" class="admin-entry" @click="goAdmin">
      <view class="admin-entry-icon">⚙</view>
      <view class="admin-entry-text">
        <view class="admin-entry-title">管理后台</view>
        <view class="admin-entry-sub">企业管理 · 工单管理</view>
      </view>
      <text class="admin-entry-arrow">›</text>
    </view>

    <!-- 企业列表 -->
    <view class="ent-title">
      <text>我的企业</text>
      <text class="ent-add" @click="goJoin">+ 添加</text>
    </view>
        <!-- :class="{ current: ent.enterprise_id === currentEnterpriseId }" -->
    <view class="ent-list">
      <view
        v-for="ent in enterprises"
        :key="ent.enterprise_id"
        class="ent-card"

        @click="onSwitchEnterprise(ent)"
      >
        <view class="ent-name-wrap">
          <text class="ent-name">{{ ent.enterprise_name }}</text>
<!--          <text v-if="ent.enterprise_id === currentEnterpriseId" class="ent-check">✓</text> -->
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
import { isPlatformAdmin } from '@/utils/jwt'
import { maskPhone, roleLabel, memberStatusLabel } from '@/utils/format'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      enterpriseStore: useEnterpriseStore(),
      maskPhone,
      roleLabel,
      memberStatusLabel
    }
  },
  data() {
    return {
      isAdmin: false,
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
    }
  },
  onShow() {
    // 按微信官方要求: 不强制登录, 未登录时展示未登录卡片
    this.isLoggedIn = !!uni.getStorageSync('token')
    if (!this.isLoggedIn) return
    this.loadData()
    this.isAdmin = isPlatformAdmin()
  },
  methods: {
    /** 跳转登录页 */
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    async loadData() {
      try {
        await this.userStore.fetchUserInfo()
        // 拉取用户信息后重新判定（登录响应 user.role 可能已更新）
        this.isAdmin = isPlatformAdmin()
      } catch (e) {
        console.error('获取用户信息失败', e)
      }
    },
    goAdmin() {
      uni.navigateTo({ url: '/pages/admin/index' })
    },
    async onSwitchEnterprise(ent: { enterprise_id: string; status: string }) {
      if (ent.enterprise_id === this.currentEnterpriseId) return
      if (ent.status !== 'approved') {
        uni.showToast({ title: '该企业暂不可用', icon: 'none' })
        return
      }
      uni.showLoading({ title: '切换中...' })
      try {
        await this.enterpriseStore.switchEnterprise(ent.enterprise_id)
        uni.hideLoading()
        uni.showToast({ title: '切换成功', icon: 'success' })
      } catch (e) {
        uni.hideLoading()
        console.error('切换企业失败', e)
      }
    },
    goJoin() {
      uni.navigateTo({ url: '/pages/enterprise/join' })
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
  background: linear-gradient(135deg, #4d80f0 0%, #6ea1ff 100%);
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
  background: linear-gradient(135deg, #4d80f0 0%, #6ea1ff 100%);
  border-radius: 20rpx;
  padding: 40rpx 32rpx;
  color: #ffffff;

  .user-avatar {
    flex-shrink: 0;
  }

  .user-info {
    margin-left: 28rpx;

    .user-name {
      font-size: 36rpx;
      font-weight: 600;
    }

    .user-phone {
      margin-top: 12rpx;
      font-size: 26rpx;
      opacity: 0.85;
    }
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
    color: #4d80f0;
  }
}

.admin-entry {
  display: flex;
  align-items: center;
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx 32rpx;
  margin-top: 20rpx;
  border: 2rpx solid rgba(77, 128, 240, 0.15);

  .admin-entry-icon {
    width: 72rpx;
    height: 72rpx;
    border-radius: 16rpx;
    background: linear-gradient(135deg, #4d80f0 0%, #6ea1ff 100%);
    color: #ffffff;
    font-size: 36rpx;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .admin-entry-text {
    flex: 1;
    margin-left: 24rpx;

    .admin-entry-title {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .admin-entry-sub {
      margin-top: 6rpx;
      font-size: 24rpx;
      color: #999999;
    }
  }

  .admin-entry-arrow {
    font-size: 36rpx;
    color: #cccccc;
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
      border-color: #4d80f0;
      background-color: rgba(77, 128, 240, 0.04);
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
        color: #4d80f0;
        font-size: 28rpx;
        font-weight: 700;
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

.logout-wrap {
  margin-top: 48rpx;
}
</style>
