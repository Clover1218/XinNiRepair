<template>
  <view class="profile-page">
    <view class="profile-card">
      <view class="profile-header">
        <button class="avatar-btn" open-type="chooseAvatar" @chooseavatar="onChooseAvatar">
          <image v-if="avatarUrl" class="avatar-img" :src="avatarUrl" mode="aspectFill"></image>
          <view v-else class="avatar-placeholder">
            <text class="avatar-plus">＋</text>
            <text class="avatar-tip">选择头像</text>
          </view>
        </button>
        <view class="avatar-hint">头像用于展示与工单报修人识别</view>
      </view>

      <view class="form-item">
        <text class="form-label">昵称</text>
        <input
          v-model="nickname"
          class="form-input"
          type="nickname"
          placeholder="请输入昵称"
          placeholder-class="form-placeholder"
        />
      </view>

      <view class="form-item">
        <text class="form-label">手机号</text>
        <view class="phone-tabs">
          <view
            class="phone-tab"
            :class="{ active: phoneMode === 'wechat' }"
            @click="phoneMode = 'wechat'"
          >
            手机号快捷登录
          </view>
          <view
            class="phone-tab"
            :class="{ active: phoneMode === 'manual' }"
            @click="phoneMode = 'manual'"
          >
            手动输入
          </view>
        </view>

        <!-- Tab1: 微信授权 -->
        <button v-if="phoneMode === 'wechat'" class="phone-btn" open-type="getPhoneNumber" @getphonenumber="onGetPhone">
          <text v-if="!phoneCode">快捷获取手机号</text>
          <text v-else class="phone-done-text">已授权获取手机号</text>
        </button>

        <!-- Tab2: 手动输入 -->
        <view v-else class="phone-manual">
          <input
            v-model="phone"
            class="form-input"
            type="number"
            maxlength="11"
            placeholder="请输入 11 位手机号"
            placeholder-class="form-placeholder"
          />
        </view>
      </view>

      <wd-button
        type="primary"
        size="large"
        block
        round
        :loading="submitting"
        loading-color="#ffffff"
        :disabled="!canSubmit"
        @click="handleSubmit"
      >
        完成注册
      </wd-button>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'
import { uploadAvatar } from '@/utils/request'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore()
    }
  },
  data() {
    return {
      nickname: '',
      avatarUrl: '',
      phoneCode: '',
      phone: '',
      phoneMode: 'wechat' as 'wechat' | 'manual',
      submitting: false
    }
  },
  computed: {
    /** 当前手机号是否有效：微信授权已取 code，或手动输入符合 11 位 1 开头 */
    phoneValid(): boolean {
      if (this.phoneMode === 'wechat') return !!this.phoneCode
      return /^1\d{10}$/.test(this.phone)
    },
    canSubmit(): boolean {
      return !!this.nickname.trim() && !!this.avatarUrl && this.phoneValid
    }
  },
  methods: {
    /** 微信头像选择回调（chooseAvatar） */
    async onChooseAvatar(e: any) {
      const temp = e.detail && e.detail.avatarUrl
      if (!temp) return
      try {
        const { url } = await uploadAvatar(temp)
        this.avatarUrl = url
      } catch (err) {
        // 错误已由 uploadAvatar 统一 Toast
      }
    },
    /** 微信手机号授权回调 */
    onGetPhone(e: any) {
      if (e.detail && e.detail.code) {
        this.phoneCode = e.detail.code
      } else if (e.detail && e.detail.errMsg && !String(e.detail.errMsg).includes('ok')) {
        uni.showToast({ title: '手机号授权失败', icon: 'none' })
      }
    },
    /** 提交注册 */
    async handleSubmit() {
      if (this.submitting || !this.canSubmit) return
      this.submitting = true
      try {
        const code = await new Promise<string>((resolve, reject) => {
          uni.login({
            provider: 'weixin',
            success: (res) => resolve(res.code),
            fail: reject
          })
        })
        await this.userStore.register({
          code,
          nickname: this.nickname.trim(),
          avatar_url: this.avatarUrl,
          // 微信授权优先；否则走手动输入
          ...(this.phoneCode ? { phone_code: this.phoneCode } : { phone: this.phone.trim() })
        })
        uni.showToast({ title: '注册成功', icon: 'success' })
        setTimeout(() => {
          uni.switchTab({ url: '/pages/order/list' })
        }, 500)
      } catch (err) {
        console.error('注册失败', err)
      } finally {
        this.submitting = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.profile-page {
  min-height: 100vh;
  background: #f5f7fa;
  padding: 40rpx 32rpx;
  box-sizing: border-box;
}

.profile-card {
  background: #ffffff;
  border-radius: 24rpx;
  padding: 48rpx 40rpx;
}

.profile-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 48rpx;
}

.avatar-btn {
  width: 144rpx;
  height: 144rpx;
  border-radius: 50%;
  overflow: hidden;
  background: #eef3ff;
  padding: 0;
  margin: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2rpx dashed #4d80f0;
  line-height: 1;
}

.avatar-btn::after {
  border: none;
}

.avatar-img {
  width: 100%;
  height: 100%;
}

.avatar-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.avatar-plus {
  font-size: 48rpx;
  color: #4d80f0;
  line-height: 1;
}

.avatar-tip {
  font-size: 20rpx;
  color: #4d80f0;
  margin-top: 8rpx;
}

.avatar-hint {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: #999999;
}

.form-item {
  margin-bottom: 40rpx;
}

.form-label {
  display: block;
  font-size: 26rpx;
  color: #333333;
  margin-bottom: 16rpx;
}

.form-input {
  width: 100%;
  height: 88rpx;
  background: #f5f7fa;
  border-radius: 16rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  font-size: 28rpx;
}

.form-placeholder {
  color: #999999;
}

/* 手机号来源 Tab 切换 */
.phone-tabs {
  display: flex;
  background: #f5f7fa;
  border-radius: 16rpx;
  padding: 6rpx;
  margin-bottom: 16rpx;
}

.phone-tab {
  flex: 1;
  height: 64rpx;
  line-height: 64rpx;
  text-align: center;
  font-size: 26rpx;
  color: #666666;
  border-radius: 12rpx;
  transition: all 0.2s;
}

.phone-tab.active {
  background: #ffffff;
  color: #4d80f0;
  font-weight: 600;
  box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.06);
}

.phone-manual .form-input {
  background: #f5f7fa;
}

.phone-btn {
  width: 100%;
  height: 88rpx;
  background: #f5f7fa;
  border-radius: 16rpx;
  font-size: 28rpx;
  color: #4d80f0;
  line-height: 88rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  text-align: left;
}

.phone-btn::after {
  border: none;
}

.phone-done-text {
  color: #67c23a;
}
</style>
