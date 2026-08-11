<template>
  <view class="page">
    <view class="form-card">
      <view class="form-label">企业名称</view>
      <input
        class="form-input"
        v-model="name"
        type="text"
        maxlength="50"
        placeholder="请输入企业名称（2-50字符）"
        placeholder-class="input-placeholder"
      />
    </view>

    <wd-button
      class="submit-btn"
      type="primary"
      size="large"
      block
      round
      :loading="loading"
      @click="onCreate"
    >
      创建企业
    </wd-button>

    <view class="tip">创建后将自动生成 6 位邀请码，可进入企业详情查看</view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { isPlatformAdmin } from '@/utils/jwt'

export default defineComponent({
  data() {
    return {
      name: '',
      loading: false
    }
  },
  onShow() {
    if (!isPlatformAdmin()) {
      uni.showToast({ title: '无管理权限', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 600)
    }
  },
  methods: {
    async onCreate() {
      const name = this.name.trim()
      if (name.length < 2 || name.length > 50) {
        uni.showToast({ title: '请输入2-50字符的企业名称', icon: 'none' })
        return
      }
      if (this.loading) return
      this.loading = true
      try {
        const data = await http.post<{ id: string; name: string; invite_code: string }>(
          '/enterprises',
          { name }
        )
        uni.showToast({ title: '创建成功', icon: 'success' })
        // 返回列表并刷新
        setTimeout(() => uni.navigateBack(), 800)
      } catch (e) {
        console.error('创建企业失败', e)
      } finally {
        this.loading = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding: 40rpx 48rpx;
  box-sizing: border-box;
}

.form-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 36rpx 28rpx;

  .form-label {
    font-size: 26rpx;
    color: #999999;
  }

  .form-input {
    margin-top: 24rpx;
    height: 96rpx;
    font-size: 34rpx;
    color: #333333;
    background-color: #f5f6f8;
    border-radius: 16rpx;
    padding: 0 24rpx;
  }
}

.input-placeholder {
  color: #bbbbbb;
  font-size: 30rpx;
}

.submit-btn {
  margin-top: 48rpx;
}

.tip {
  margin-top: 32rpx;
  text-align: center;
  font-size: 24rpx;
  color: #aaaaaa;
}
</style>
