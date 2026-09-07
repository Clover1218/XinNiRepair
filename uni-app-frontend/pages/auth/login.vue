<template>
  <view class="login-page">
    <view class="login-header">
      <image class="login-logo" src="/static/logo.png" mode="aspectFit"></image>
      <view class="login-title">新泥百电脑</view>
      <view class="login-subtitle">报修系统</view>
    </view>

    <view class="login-body">
      <wd-button
        type="primary"
        size="large"
        block
        round
        :loading="loading"
        loading-color="#ffffff"
        @click="handleLogin"
      >
        一键登录
      </wd-button>
      <view class="login-agreement">
        <wd-checkbox v-model="agreed" size="18px" custom-style="margin-right: 8rpx;" />
        <text>已阅读并同意</text>
        <text class="agreement-link" @click="openAgreement('user')">《用户协议》</text>
        <text>和</text>
        <text class="agreement-link" @click="openAgreement('privacy')">《隐私政策》</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore()
    }
  },
  data() {
    return {
      loading: false,
      // V1.2：协议勾选只在“注册那一刻”要求；老用户本地已有标记，默认视为同意
      agreed: uni.getStorageSync('agreedPrivacy') === '1'
    }
  },
  onLoad() {
    // 本地已有 token：直接进入首页（token 有效性由首页 onShow/请求层 401 静默续期兜底）
    const token = uni.getStorageSync('token')
    if (token) {
      uni.switchTab({ url: '/pages/order/list' })
    }
  },
  methods: {
    /** 打开协议文档页 */
    openAgreement(type: string) {
      const title = type === 'user' ? '用户协议' : '隐私政策'
      uni.navigateTo({ url: `/pages/auth/agreement?type=${type}` })
      // 动态设置导航栏标题
      setTimeout(() => {
        uni.setNavigationBarTitle({ title })
      }, 100)
    },
    /** 微信一键登录：wx.login 拿 code -> POST /auth/login 换取 JWT */
    async handleLogin() {
      if (this.loading) return
      if (!this.agreed) {
        uni.showToast({ title: '请先阅读并同意用户协议和隐私政策', icon: 'none' })
        return
      }
      this.loading = true
      try {
        const code = await new Promise<string>((resolve, reject) => {
          uni.login({
            provider: 'weixin',
            success: (res) => resolve(res.code),
            fail: reject
          })
        })
        const { needProfile } = await this.userStore.login(code)
        // 登录/注册成功即视为已同意协议，写入本地（老用户后续静默登录不再强制勾选）
        uni.setStorageSync('agreedPrivacy', '1')
        if (needProfile) {
          // 新用户：先完善资料（头像/昵称/手机号），注册成功即登录进入首页
          uni.redirectTo({ url: '/pages/auth/profile' })
          return
        }
        uni.showToast({ title: '登录成功', icon: 'success' })
        setTimeout(() => {
          uni.switchTab({ url: '/pages/order/list' })
        }, 500)
      } catch (e) {
        // 错误信息已由请求封装统一 Toast
        console.error('登录失败', e)
      } finally {
        this.loading = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  background: linear-gradient(180deg, #eef3ff 0%, #ffffff 55%);
}

.login-header {
  margin-top: 220rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.login-logo {
  width: 176rpx;
  height: 176rpx;
  border-radius: 40rpx;
  background-color: #ffffff;
  box-shadow: 0 12rpx 40rpx rgba(77, 128, 240, 0.15);
}

.login-title {
  margin-top: 36rpx;
  font-size: 44rpx;
  font-weight: 700;
  color: #1a1a1a;
}

.login-subtitle {
  margin-top: 12rpx;
  font-size: 26rpx;
  color: #999999;
}

.login-body {
  margin-top: 200rpx;
  width: 100%;
  padding: 0 64rpx;
  box-sizing: border-box;
}

.login-agreement {
  margin-top: 32rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24rpx;
  color: #999999;
}

.agreement-link {
  color: #4d80f0;
}
</style>
