<template>
  <view class="page">
    <view class="header">
      <view class="header-title">管理后台</view>
      <view class="header-sub">新泥报修系统 · 平台管理员</view>
    </view>

    <view class="menu-card" @click="goEnterprise">
      <view class="menu-icon">🏢</view>
      <view class="menu-text">
        <view class="menu-title">企业管理</view>
        <view class="menu-sub">企业搜索 · 成员审核 · 刷新邀请码 · 创建企业</view>
      </view>
      <text class="menu-arrow">›</text>
    </view>

    <view class="menu-card" @click="goOrder">
      <view class="menu-icon">📋</view>
      <view class="menu-text">
        <view class="menu-title">工单管理</view>
        <view class="menu-sub">查阅 · 接单 · 完工 · 退回 · 收据上传</view>
      </view>
      <text class="menu-arrow">›</text>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { isPlatformAdmin } from '@/utils/jwt'

export default defineComponent({
  onShow() {
    if (!isPlatformAdmin()) {
      uni.showToast({ title: '无管理权限', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 600)
    }
  },
  methods: {
    goEnterprise() {
      uni.navigateTo({ url: '/pages/admin/enterprise/list' })
    },
    goOrder() {
      uni.navigateTo({ url: '/pages/admin/order/list' })
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

.header {
  padding: 40rpx 16rpx 32rpx;

  .header-title {
    font-size: 44rpx;
    font-weight: 700;
    color: #1a1a1a;
  }

  .header-sub {
    margin-top: 12rpx;
    font-size: 26rpx;
    color: #999999;
  }
}

.menu-card {
  display: flex;
  align-items: center;
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 36rpx 32rpx;
  margin-bottom: 24rpx;

  .menu-icon {
    width: 88rpx;
    height: 88rpx;
    border-radius: 20rpx;
    background-color: #f0f4ff;
    font-size: 44rpx;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .menu-text {
    flex: 1;
    margin-left: 28rpx;

    .menu-title {
      font-size: 32rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .menu-sub {
      margin-top: 10rpx;
      font-size: 24rpx;
      color: #999999;
    }
  }

  .menu-arrow {
    font-size: 40rpx;
    color: #cccccc;
  }
}
</style>
