<template>
  <view class="oc" @click="onClick">
    <view class="oc-row">
    <!-- 缩略图（64px 对齐 Web；图片缺失用 image 图标占位） -->
    <view class="oc-thumb">
      <image v-if="firstImage" class="oc-thumb-img" :src="firstImage" mode="aspectFill"></image>
      <wd-icon v-else name="image" size="48rpx" color="#9ca3af"></wd-icon>
    </view>

    <view class="oc-body">
      <!-- 第一行：故障描述（单行截断） + 大字金额 -->
      <view class="oc-top">
        <view class="oc-desc">{{ item.description || '（无故障描述）' }}</view>
        <view v-if="amountText" class="oc-amount">¥{{ amountText }}</view>
      </view>

      <!-- 第二行：状态胶囊 + 项目大类·属性 -->
      <view class="oc-mid">
        <text class="status-pill" :class="pill.cls">{{ pill.text }}</text>
        <text class="oc-cat">{{ problemType || '未分类' }}</text>
      </view>

      <!--
        元信息（浅灰 #9CA3AF / 12px 视觉降噪）：
        - 隐藏企业名（顶部已选定具体企业）：报修人 · 地点 · 时间 合并一行
        - 否则：企业 · 地点 一行；报修人 + 时间 一行
      -->
      <view class="oc-meta">
        <view v-if="hideEnterprise" class="oc-line oc-line-compact">
          <wd-icon name="user" size="22rpx" color="#9ca3af"></wd-icon>
          <text class="oc-val">{{ reporterName || '—' }}</text>
          <wd-icon name="location" size="22rpx" color="#9ca3af"></wd-icon>
          <text class="oc-val oc-val-flex">{{ location || '未填地点' }}</text>
          <text class="oc-time">{{ timeText }}</text>
        </view>
        <template v-else>
          <view class="oc-line">
            <wd-icon name="shop" size="26rpx" color="#9ca3af"></wd-icon>
            <text class="oc-val oc-val-flex">{{ enterpriseName }}</text>
            <text class="oc-sep">|</text>
            <wd-icon name="location" size="26rpx" color="#9ca3af"></wd-icon>
            <text class="oc-val oc-val-flex">{{ location || '未填地点' }}</text>
          </view>
          <view class="oc-line">
            <wd-icon name="user" size="26rpx" color="#9ca3af"></wd-icon>
            <text class="oc-val">{{ reporterName || '—' }}</text>
            <text class="oc-time">{{ timeText }}</text>
          </view>
        </template>
      </view>
    </view>
    <!-- 引导箭头：列表态不展示任何操作按钮，统一引导进入详情页再操作 -->
    <view class="oc-arrow" aria-hidden="true">
      <wd-icon name="arrow-right" size="36rpx" color="#cbd5e1"></wd-icon>
    </view>
    </view>

    <!-- 额外信息位（如退回提示） -->
    <slot />
  </view>
</template>

<script lang="ts">
import { defineComponent, type PropType } from 'vue'
import type { AdminOrderListItem, OrderListItem } from '@/types'
import { statusPill, formatAmount, formatListTime, splitContact } from '@/utils/format'

/**
 * 统一工单卡片（对齐 Web 端 components/OrderCardC.vue）
 *
 * ┌────────────────────────────────────┐
 * │ ┌─────┐  故障描述（单行截断）  ¥120 │
 * │ │缩略图│                            │
 * │ └─────┘  [处理中]  电脑 · 更换      │
 * │          🏪 测试企业3 | 📍 高三13班  │
 * │          👤 张三      2026-10-11 14:30 │
 * └────────────────────────────────────┘
 *
 * 与旧版（5 行「标签：值」列表）的关键差异：
 * 1. 元信息由 5 行压成 2 行，用图标 + 值，企业与地点合并一行、报修人与时间合并一行；
 * 2. 状态由「纯文字染色」改为六色实心胶囊，并支持「待补签」渐变胶囊；
 * 3. 金额独立成大字 hero（苍岭绿，非红色——红色在体系里专指「已退回」）；
 * 4. 描述单行截断，保证网格内卡片等高、底边对齐。
 *
 * 首页、工单 tab（普通/管理列表）三处共用此组件，改一处全局生效。
 */
export default defineComponent({
  name: 'OrderCard',
  props: {
    item: {
      type: Object as PropType<AdminOrderListItem | OrderListItem>,
      required: true
    },
    /** 是否隐藏企业名：顶部已选定具体企业时由父组件传入 true（避免与顶部筛选重复） */
    hideEnterprise: {
      type: Boolean as PropType<boolean>,
      default: false
    }
  },
  emits: ['click'],
  computed: {
    /** 状态胶囊：文案 + class（待补签优先） */
    pill(): { text: string; cls: string } {
      return statusPill(this.item as any)
    },
    timeText(): string {
      return formatListTime(this.item.submitted_at || this.item.created_at)
    },
    images(): string[] {
      return (this.item.images || []).map((img) => img.url).filter(Boolean)
    },
    firstImage(): string {
      return this.images[0] || ''
    },
    location(): string {
      return this.item.room || ''
    },
    /** 报修人：从 contact（"王五 12345678910"）分隔提取真实姓名 */
    reporterName(): string {
      return splitContact(this.item.contact).name
    },
    /** 金额（>0 才显示，对齐 Web） */
    amountText(): string {
      const v = (this.item as any).amount
      return v && Number(v) > 0 ? formatAmount(v) : ''
    },
    /** 问题类型：大类 · 属性 */
    problemType(): string {
      return [this.item.category_name, this.item.property_name].filter(Boolean).join(' · ')
    },
    enterpriseName(): string {
      return this.item.enterprise_name || '未指定单位'
    }
  },
  methods: {
    onClick() {
      this.$emit('click', this.item)
    }
  }
})
</script>

<style lang="scss" scoped>
.oc {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  padding: 24rpx 28rpx;
  background: var(--xn-surface);
  border: 2rpx solid var(--xn-border);
  border-radius: 20rpx;
}
.oc-row {
  display: flex;
  align-items: stretch;
  min-width: 0;
}
/* 引导箭头：紧跟正文右侧、垂直居中，提示「点击进入详情」 */
.oc-arrow {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  margin-left: 12rpx;
  padding-left: 8rpx;
}

/* 缩略图 64px */
.oc-thumb {
  width: 128rpx;
  height: 128rpx;
  flex-shrink: 0;
  margin-right: 24rpx;
  border-radius: 16rpx;
  overflow: hidden;
  background: var(--xn-tag-bg);
  display: flex;
  align-items: center;
  justify-content: center;
}
.oc-thumb-img {
  width: 100%;
  height: 100%;
  display: block;
}

.oc-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.oc-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
}
.oc-desc {
  font-size: 28rpx;
  font-weight: 600;
  color: var(--xn-text-900);
  line-height: 1.4;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}
.oc-amount {
  font-size: 36rpx;
  font-weight: 600;
  color: var(--xn-primary);
  white-space: nowrap;
  flex-shrink: 0;
}

.oc-mid {
  display: flex;
  align-items: center;
  gap: 16rpx;
  margin-top: 16rpx;
  min-width: 0;
}
.oc-cat {
  font-size: 24rpx;
  color: var(--xn-text-500);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

/* 状态胶囊（与 Web 端 .status-pill 一致） */
.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 6rpx 24rpx;
  border-radius: 28rpx;
  font-size: 26rpx;
  font-weight: 600;
  line-height: 1;
  color: #fff;
  white-space: nowrap;
  flex-shrink: 0;
}
.st-amber {
  background: var(--st-amber);
}
.st-orange {
  background: var(--st-orange);
}
.st-blue {
  background: var(--st-blue);
}
.st-teal {
  background: var(--st-teal);
}
.st-red {
  background: var(--st-red);
}
.st-gray {
  background: var(--st-gray);
}
.st-pending-sign {
  background: linear-gradient(90deg, #f59e0b, #ea580c);
  letter-spacing: 2rpx;
}

/* 图标化元信息：浅灰 #9CA3AF + 12px（设计 token --xn-text-400）视觉降噪 */
.oc-meta {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: var(--xn-text-400);
}
.oc-line {
  display: flex;
  align-items: center;
  gap: 12rpx;
  min-width: 0;
  line-height: 1.2;
}
.oc-line + .oc-line {
  margin-top: 12rpx;
}
/* 隐藏企业名时的合并单行：报修人 · 地点 · 时间 */
.oc-line-compact {
  gap: 10rpx;
}
.oc-val {
  color: inherit;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
/* 地点 / 企业占据剩余宽度，超出即截断 */
.oc-val-flex {
  flex: 1;
}
.oc-sep {
  color: var(--xn-text-400);
  flex-shrink: 0;
}

/* 时间与报修人同行，靠右对齐 */
.oc-time {
  margin-left: auto;
  padding-left: 16rpx;
  font-size: 24rpx;
  color: var(--xn-text-400);
  white-space: nowrap;
  flex-shrink: 0;
}
</style>
