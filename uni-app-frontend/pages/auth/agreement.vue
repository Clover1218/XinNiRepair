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
import { BASE_URL } from '@/utils/config'

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
      uni.request({
        url: `${BASE_URL}/agreement/${type}`,
        method: 'GET',
        success: (res) => {
          if (res.statusCode === 200 && typeof res.data === 'string') {
            // 提取 body 内容, 去掉外层 html/head/body 标签
            let html = res.data as string
            const bodyMatch = html.match(/<body[^>]*>([\s\S]*)<\/body>/i)
            if (bodyMatch) {
              html = bodyMatch[1].trim()
            }
            // 注入页面级样式
            this.htmlContent = `<div style="padding:16px 12px;font-size:15px;line-height:1.8;color:#333;">${html}</div>`
          } else {
            uni.showToast({ title: '加载失败', icon: 'none' })
          }
        },
        fail: () => {
          uni.showToast({ title: '网络异常', icon: 'none' })
        },
        complete: () => {
          this.loading = false
        }
      })
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
