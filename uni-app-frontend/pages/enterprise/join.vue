<template>
  <view class="page">
    <view class="join-card">
      <view class="join-label">输入邀请码加入企业</view>
      <input
        class="join-input"
        v-model="inviteCode"
        type="text"
        maxlength="6"
        placeholder="请输入6位邀请码"
        placeholder-class="input-placeholder"
        @input="onInput"
      />
    </view>

    <view class="scan-btn" @click="scanQR">
      <text class="scan-icon">▣</text>
      <text class="scan-text">扫一扫</text>
    </view>

    <wd-button
      class="join-btn"
      type="primary"
      size="large"
      block
      round
      :loading="loading"
      @click="joinEnterprise"
    >
      确认加入
    </wd-button>

    <view class="join-tip">通过企业管理员分享的邀请码或二维码加入</view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'

export default defineComponent({
  data() {
    return {
      inviteCode: '',
      loading: false
    }
  },
  methods: {
    onInput(e: { detail: { value: string } }) {
      // 自动转大写
      this.inviteCode = e.detail.value.toUpperCase()
    },
    /** 扫码获取邀请码 */
    scanQR() {
      uni.scanCode({
        onlyFromCamera: true,
        success: (res) => {
          const code = this.extractCode(res.result)
          if (code) {
            this.inviteCode = code
          } else {
            uni.showToast({ title: '未识别到有效邀请码', icon: 'none' })
          }
        },
        fail: () => {
          uni.showToast({ title: '扫码失败', icon: 'none' })
        }
      })
    },
    /** 从扫码结果中提取6位邀请码：优先取 code 参数，否则匹配连续字母数字 */
    extractCode(raw: string): string {
      if (!raw) return ''
      const match = raw.match(/[?&]code=([A-Za-z0-9]{6})/)
      if (match) return match[1].toUpperCase()
      const m = raw.match(/([A-Za-z0-9]{6})/)
      return m ? m[1].toUpperCase() : ''
    },
    async joinEnterprise() {
      if (this.inviteCode.length !== 6) {
        uni.showToast({ title: '请输入6位邀请码', icon: 'none' })
        return
      }
      if (this.loading) return
      this.loading = true
      try {
        // 前端任务文档约定：POST /enterprises/join { invite_code }
        // （后端接口文档路径为 /enterprises/{enterprise_id}/join，若后端要求该形式可在此调整）
        const data = await http.post<{ status: string; enterprise_name: string; tip?: string }>(
          '/enterprises/join',
          { invite_code: this.inviteCode }
        )
        const msg =
          data.status === 'approved'
            ? `已加入「${data.enterprise_name}」`
            : data.tip || '申请已提交，请等待管理员审核'
        uni.showToast({ title: msg, icon: 'none', duration: 2500 })
        setTimeout(() => uni.navigateBack(), 1200)
      } catch (e) {
        console.error('加入企业失败', e)
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
  padding: 48rpx 48rpx;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}

.join-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 36rpx 28rpx;

  .join-label {
    font-size: 26rpx;
    color: #999999;
  }

  .join-input {
    margin-top: 24rpx;
    height: 100rpx;
    font-size: 44rpx;
    letter-spacing: 12rpx;
    font-weight: 600;
    color: #333333;
    text-align: center;
    background-color: #f5f6f8;
    border-radius: 16rpx;
  }
}

.input-placeholder {
  color: #bbbbbb;
  font-weight: 400;
  letter-spacing: 4rpx;
}

.scan-btn {
  margin-top: 32rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx 0;

  .scan-icon {
    font-size: 34rpx;
    color: #4d80f0;
    margin-right: 12rpx;
  }

  .scan-text {
    font-size: 30rpx;
    color: #4d80f0;
  }
}

.join-btn {
  margin-top: 48rpx;
}

.join-tip {
  margin-top: 32rpx;
  text-align: center;
  font-size: 24rpx;
  color: #aaaaaa;
}
</style>
