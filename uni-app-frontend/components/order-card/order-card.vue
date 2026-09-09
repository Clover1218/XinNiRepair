<template>
  <view class="oc" @click="onClick">
    <!-- 主体：左缩略图（images[0]） + 完整报修描述（≤3 行截断） -->
    <view class="oc-body">
      <image v-if="firstImage" class="oc-thumb" :src="firstImage" mode="aspectFill"></image>
      <view class="oc-text">
        <view class="oc-desc">{{ item.description || '（无描述）' }}</view>
      </view>
    </view>

    <!-- 其他信息：问题类型 / 报修企业 / 报修地点 / 报修人 / 联系方式 -->
    <view class="oc-info">
      <view class="oc-info-row">
        <text class="oc-info-label">问题类型：</text>
        <text class="oc-info-value">{{ problemType || '—' }}</text>
      </view>
      <view class="oc-info-row">
        <text class="oc-info-label">报修企业：</text>
        <text class="oc-info-value">{{ enterpriseName }}</text>
      </view>
      <view class="oc-info-row">
        <text class="oc-info-label">报修地点：</text>
        <text class="oc-info-value">{{ location || '—' }}</text>
      </view>
      <view class="oc-info-row">
        <text class="oc-info-label">报修人：</text>
        <text class="oc-info-value">{{ reporterName || '—' }}</text>
      </view>
      <view class="oc-info-row">
        <text class="oc-info-label">联系方式：</text>
        <text class="oc-info-value">{{ contactPhone || '—' }}</text>
      </view>
    </view>

    <!-- 额外信息位（如：退回提示 / 金额） -->
    <slot />

    <!-- 底栏：当前状态（状态标签） + 提交时间（相对时间） -->
    <view class="oc-foot">
      <view class="oc-foot-side">
        <text class="oc-foot-label">当前状态：</text>
        <text class="oc-status-text" :style="{ color: statusColor(item.status) }">{{ statusText }}</text>
      </view>
      <view class="oc-foot-side">
        <text class="oc-foot-label">提交时间：</text>
        <text class="oc-foot-time">{{ timeText }}</text>
      </view>
    </view>

    <!-- 操作位（如：我的工单草稿的 编辑/提交/取消/删除） -->
    <slot name="actions" />
  </view>
</template>

<script lang="ts">
import { defineComponent, type PropType } from 'vue'
import type { AdminOrderListItem, OrderListItem } from '@/types'
import { statusTagType, statusColor, statusLabel, relativeTime, splitContact } from '@/utils/format'

/**
 * V1.3 统一工单卡片（审核工单 / 处理工单 / 我的工单 三处列表共用）
 *
 * ┌────────────────────────────────┐
 * │ ┌──────┐  报修描述（≤3行）      │
 * │ │ 缩略图│                       │
 * │ └──────┘                       │
 * │ 问题类型：设备类型-问题类型      │
 * │ 报修企业：xxx                   │
 * │ 报修地点：xxx                   │
 * │ 报修人：xxx                     │
 * │ 联系方式：xxx                   │
 * │ ────────────────────────────── │
 * │ 当前状态：[标签]  提交时间：x分钟前 │
 * └────────────────────────────────┘
 *
 * 两个插槽：
 * - 默认插槽：卡片信息区之后、分隔线之前的额外信息（退回提示 / 金额）
 * - actions：底栏之下的操作按钮（注意自行加 @click.stop 拦截冒泡）
 */
export default defineComponent({
  name: 'OrderCard',
  props: {
    item: {
      type: Object as PropType<AdminOrderListItem | OrderListItem>,
      required: true
    }
  },
  emits: ['click'],
  computed: {
    statusText(): string {
      return this.item.status_label || statusLabel(this.item.status)
    },
    timeText(): string {
      return relativeTime(this.item.submitted_at || this.item.created_at)
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
    /** 报修人：从 contact（"王五 12345678910"）分隔提取 */
    reporterName(): string {
      return splitContact(this.item.contact).name
    },
    /** 联系方式：从 contact 分隔提取（不脱敏） */
    contactPhone(): string {
      return splitContact(this.item.contact).phone
    },
    /** 问题类型：设备类型-问题类型 */
    problemType(): string {
      return [this.item.category_name, this.item.property_name].filter(Boolean).join('-')
    },
    enterpriseName(): string {
      return this.item.enterprise_name || '未指定单位'
    }
  },
  methods: {
    statusTagType,
    statusColor,
    onClick() {
      this.$emit('click', this.item)
    }
  }
})
</script>

<style lang="scss" scoped>
.oc {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx;
  margin-bottom: 20rpx;
  box-shadow: 0 4rpx 16rpx rgba(0, 0, 0, 0.04);
}

.oc-body {
  display: flex;
  align-items: flex-start;

  .oc-thumb {
    width: 160rpx;
    height: 160rpx;
    border-radius: 12rpx;
    margin-right: 20rpx;
    flex-shrink: 0;
    background-color: #f5f6f8;
  }

  .oc-text {
    flex: 1;
    min-width: 0;
  }

  /* 完整报修描述，最多三行截断 */
  .oc-desc {
    font-size: 28rpx;
    color: #333333;
    line-height: 1.5;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    overflow: hidden;
  }
}

.oc-info {
  margin-top: 20rpx;
  font-size: 24rpx;
  line-height: 1.7;
}

.oc-info-row {
  display: flex;
  align-items: baseline;
}

.oc-info-label {
  flex-shrink: 0;
  color: #999999;
}

.oc-info-value {
  flex: 1;
  min-width: 0;
  color: #333333;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.oc-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f2f3f5;

  .oc-foot-side {
    display: flex;
    align-items: center;
    min-width: 0;
  }

  .oc-foot-label {
    flex-shrink: 0;
    font-size: 24rpx;
    color: #999999;
  }

  /* 状态：纯文字 + 颜色（不再使用胶囊标签） */
  .oc-status-text {
    font-size: 24rpx;
    font-weight: 600;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .oc-foot-time {
    font-size: 24rpx;
    color: #666666;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
}
</style>
