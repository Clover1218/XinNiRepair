<template>
  <view class="agreement-page">
    <view v-if="loading" class="loading-wrap">
      <text>加载中...</text>
    </view>
    <rich-text v-else-if="htmlContent" :nodes="htmlContent"></rich-text>
    <view v-else class="error-wrap">
      <text>加载失败，请稍后重试</text>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'

export default defineComponent({
  data() {
    return {
      loading: true,
      htmlContent: ''
    }
  },
  onLoad(options: { type?: string }) {
    const type = options.type || 'user'
    this.loadAgreement(type)
  },
  methods: {
    async loadAgreement(type: string) {
      this.loading = true
      try {
        const data = await http.get<{ content: string }>(`/agreement/${type}`)
        this.htmlContent = `<div style="padding:16px 12px;font-size:15px;line-height:1.8;color:#333;">${data.content}</div>`
      } catch {
        // 错误 Toast 已由 request 封装统一处理
      } finally {
        this.loading = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.agreement-page {
  min-height: 100vh;
  background-color: #ffffff;
}

.loading-wrap,
.error-wrap {
  display: flex;
  justify-content: center;
  align-items: center;
  padding-top: 200rpx;
  font-size: 28rpx;
  color: #999999;
}
</style>
