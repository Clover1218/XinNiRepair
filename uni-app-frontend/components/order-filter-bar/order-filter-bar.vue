<template>
  <view class="fb">
    <!-- 企业筛选 Header：点击展开（单个企业 / 全部企业） -->
    <view class="fb-ent" @click="pickEnterprise">
      <text class="fb-ent-name">{{ currentName }}</text>
      <text class="fb-ent-arrow">▾</text>
    </view>

    <!-- 状态分组 Tab -->
    <view class="fb-tabs">
      <view
        v-for="t in tabs"
        :key="t.value"
        class="fb-tab"
        :class="{ active: t.value === activeTab }"
        @click="onTab(t.value)"
      >
        {{ t.label }}
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent, type PropType } from 'vue'
import type { StatusGroupTab } from '@/utils/format'

/**
 * V1.3 列表筛选栏（审核工单页 / 处理工单页共用）
 * 上：企业筛选 Header（点击切换单个企业或“全部企业”）
 * 下：状态分组 Tab
 */
export default defineComponent({
  name: 'OrderFilterBar',
  props: {
    /** 可选企业列表（首项为“全部企业”时 id 为空串） */
    enterprises: {
      type: Array as PropType<{ id: string; name: string }[]>,
      default: () => []
    },
    enterpriseIndex: {
      type: Number,
      default: 0
    },
    tabs: {
      type: Array as PropType<StatusGroupTab[]>,
      required: true
    },
    activeTab: {
      type: String,
      default: ''
    }
  },
  emits: ['enterprise-change', 'tab-change'],
  computed: {
    currentName(): string {
      const opt = this.enterprises[this.enterpriseIndex]
      return opt ? opt.name : '全部企业'
    }
  },
  methods: {
    onTab(value: string) {
      if (value === this.activeTab) return
      this.$emit('tab-change', value)
    },
    pickEnterprise() {
      if (this.enterprises.length === 0) return
      uni.showActionSheet({
        itemList: this.enterprises.map((e) => e.name),
        success: (res) => {
          this.$emit('enterprise-change', Number(res.tapIndex) || 0)
        },
        fail: () => {
          // 用户取消
        }
      })
    }
  }
})
</script>

<style lang="scss" scoped>
.fb {
  background-color: #ffffff;
  padding: 20rpx 24rpx 0;
}

.fb-ent {
  display: inline-flex;
  align-items: center;
  padding: 8rpx 4rpx;

  .fb-ent-name {
    font-size: 34rpx;
    font-weight: 600;
    color: #1a1a1a;
    max-width: 560rpx;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .fb-ent-arrow {
    margin-left: 8rpx;
    font-size: 24rpx;
    color: #999999;
  }
}

.fb-tabs {
  display: flex;
  align-items: center;
  margin-top: 20rpx;

  .fb-tab {
    position: relative;
    padding: 12rpx 28rpx 20rpx;
    font-size: 28rpx;
    color: #666666;

    &.active {
      color: #4d80f0;
      font-weight: 600;

      &::after {
        content: '';
        position: absolute;
        left: 50%;
        transform: translateX(-50%);
        bottom: 6rpx;
        width: 48rpx;
        height: 6rpx;
        border-radius: 6rpx;
        background-color: #4d80f0;
      }
    }
  }
}
</style>
