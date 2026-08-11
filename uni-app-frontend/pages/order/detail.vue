<template>
  <view class="page" v-if="order">
    <!-- 状态卡片 -->
    <view class="status-card">
      <view class="status-row">
        <view class="status-project">{{ order.project_name || '未命名工单' }}</view>
        <wd-tag :type="statusTagType(order.status)" round>{{ order.status_label }}</wd-tag>
      </view>
      <view v-if="order.order_no" class="order-no">{{ order.order_no }}</view>
      <view class="status-sub">
        创建于 {{ formatDateTime(order.created_at) }}
        <text v-if="order.submitted_at"> · 提交于 {{ formatDateTime(order.submitted_at) }}</text>
      </view>
    </view>

    <!-- 报修信息 -->
    <view class="info-card">
      <view class="info-row">
        <text class="info-label">项目大类</text>
        <text class="info-value">{{ order.category_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目属性</text>
        <text class="info-value">{{ order.property_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">紧急程度</text>
        <text class="info-value">{{ order.urgency_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">房间号</text>
        <text class="info-value">{{ order.room }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">联系人</text>
        <text class="info-value">{{ order.contact }}</text>
      </view>
      <view v-if="order.reject_reason" class="info-row info-reject">
        <text class="info-label">退回原因</text>
        <text class="info-value reject-text">{{ order.reject_reason }}</text>
      </view>
    </view>

    <!-- 报修描述（独立卡片） -->
    <view v-if="order.description" class="desc-card">
      <view class="desc-title">报修描述</view>
      <view class="desc-content">{{ order.description }}</view>
    </view>

    <!-- 故障图片 -->
    <view v-if="order.images && order.images.length > 0" class="img-card">
      <view class="card-title">故障图片</view>
      <view class="img-grid">
        <image
          v-for="(img, index) in order.images"
          :key="img.id"
          class="img-item"
          :src="img.url"
          mode="aspectFill"
          @click="previewImage(index)"
        ></image>
      </view>
    </view>

    <!-- 收据 -->
    <view v-if="order.receipts && order.receipts.length > 0" class="img-card">
      <view class="card-title">收据凭证</view>
      <view class="img-grid">
        <image
          v-for="img in order.receipts"
          :key="img.id"
          class="img-item"
          :src="img.url"
          mode="aspectFill"
        ></image>
      </view>
    </view>

    <!-- 时间轴 -->
    <view class="timeline-card">
      <view class="card-title">进度记录</view>
      <view class="timeline">
        <view v-for="(t, index) in order.timeline" :key="t.id" class="timeline-item">
          <view class="timeline-dot" :class="{ last: index === order.timeline.length - 1 }"></view>
          <view class="timeline-content">
            <view class="timeline-action">{{ t.action_label }}</view>
            <view class="timeline-meta">
              {{ t.operator_name }} · {{ formatDateTime(t.created_at) }}
            </view>
            <view v-if="t.remark" class="timeline-remark">{{ t.remark }}</view>
          </view>
        </view>
      </view>
    </view>

    <!-- 底部操作 -->
    <view class="footer">
      <template v-if="order.status === 'draft'">
        <view class="footer-btn">
          <wd-button type="primary" round block @click="goEdit">编辑</wd-button>
        </view>
        <view class="footer-btn">
          <wd-button plain round block @click="cancelOrder">取消工单</wd-button>
        </view>
      </template>
      <template v-else-if="order.status === 'reported' || order.status === 'reviewed'">
        <view class="footer-btn">
          <wd-button type="warning" plain round block @click="cancelOrder">取消工单</wd-button>
        </view>
      </template>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { formatDateTime, statusTagType } from '@/utils/format'
import type { OrderDetail } from '@/types'

export default defineComponent({
  setup() {
    return {
      formatDateTime,
      statusTagType
    }
  },
  data() {
    return {
      orderId: '',
      order: null as OrderDetail | null
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
    if (!this.orderId) {
      uni.showToast({ title: '参数错误', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 800)
      return
    }
    this.loadDetail()
  },
  methods: {
    async loadDetail() {
      try {
        this.order = await http.get<OrderDetail>(`/orders/${this.orderId}`)
        if (!this.order) {
          uni.showToast({ title: '工单不存在', icon: 'none' })
        }
      } catch (e) {
        console.error('加载详情失败', e)
      }
    },
    previewImage(index: number) {
      if (!this.order) return
      uni.previewImage({
        current: this.order.images[index].url,
        urls: this.order.images.map((i) => i.url)
      })
    },
    goEdit() {
      uni.navigateTo({ url: `/pages/order/edit?id=${this.orderId}` })
    },
    /** 取消工单：填写原因后调用 cancel 接口 */
    async cancelOrder() {
      const res = await uni.showModal({
        title: '取消工单',
        content: '请填写取消原因',
        editable: true,
        placeholderText: '如：问题已自行解决'
      })
      if (!res.confirm) return
      const reason = (res.content || '').trim()
      if (!reason) {
        uni.showToast({ title: '请填写取消原因', icon: 'none' })
        return
      }
      try {
        await http.post(`/orders/${this.orderId}/cancel`, { reason })
        uni.showToast({ title: '已取消', icon: 'success' })
        this.loadDetail()
      } catch (e) {
        console.error('取消失败', e)
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding: 20rpx 24rpx 160rpx;
  box-sizing: border-box;
}

.status-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx 32rpx;

  .status-row {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .status-project {
      flex: 1;
      margin-right: 20rpx;
      font-size: 36rpx;
      font-weight: 600;
      color: #1a1a1a;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .order-no {
    margin-top: 12rpx;
    font-size: 26rpx;
    color: #999999;
  }

  .status-sub {
    margin-top: 8rpx;
    font-size: 24rpx;
    color: #999999;
  }
}

.info-card,
.img-card,
.timeline-card,
.desc-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 12rpx 28rpx;
  margin-top: 20rpx;
}

/* 报修描述（独立卡片） */
.desc-card {
  padding-bottom: 28rpx;

  .desc-title {
    padding: 28rpx 0 20rpx;
    font-size: 30rpx;
    font-weight: 700;
    color: #1a1a1a;
  }

  .desc-content {
    width: 94%;
    font-size: 28rpx;
    line-height: 1.7;
    color: #333333;
  }
}

.info-row {
  display: flex;
  padding: 24rpx 0;
  border-bottom: 1rpx solid #f2f3f5;

  &:last-child {
    border-bottom: none;
  }

  .info-label {
    width: 160rpx;
    font-size: 28rpx;
    color: #999999;
    flex-shrink: 0;
  }

  .info-value {
    flex: 1;
    font-size: 28rpx;
    color: #333333;
    text-align: right;
  }

  &.info-desc {
    .info-desc-text {
      text-align: left;
      line-height: 1.6;
      color: #333333;
    }
  }

  &.info-reject {
    .reject-text {
      color: #fa5151;
    }
  }
}

.card-title {
  padding: 28rpx 0 16rpx;
  font-size: 30rpx;
  font-weight: 600;
  color: #333333;
}

.img-grid {
  display: flex;
  flex-wrap: wrap;
  padding-bottom: 24rpx;

  .img-item {
    width: 200rpx;
    height: 200rpx;
    margin-right: 16rpx;
    margin-bottom: 16rpx;
    border-radius: 16rpx;
    background-color: #f5f6f8;
  }
}

.timeline {
  padding-bottom: 24rpx;

  .timeline-item {
    display: flex;
    padding: 16rpx 0;

    .timeline-dot {
      width: 16rpx;
      height: 16rpx;
      border-radius: 50%;
      background-color: #4d80f0;
      margin-top: 12rpx;
      margin-right: 20rpx;
      flex-shrink: 0;

      &.last {
        background-color: #cccccc;
      }
    }

    .timeline-content {
      flex: 1;
      padding-bottom: 8rpx;

      .timeline-action {
        font-size: 28rpx;
        color: #333333;
      }

      .timeline-meta {
        margin-top: 8rpx;
        font-size: 22rpx;
        color: #999999;
      }

      .timeline-remark {
        margin-top: 8rpx;
        font-size: 24rpx;
        color: #666666;
      }
    }
  }
}

.footer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  padding: 20rpx 24rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
  background-color: #ffffff;
  box-shadow: 0 -4rpx 16rpx rgba(0, 0, 0, 0.04);

  .footer-btn {
    flex: 1;
    margin-right: 20rpx;

    &:last-child {
      margin-right: 0;
    }
  }
}
</style>
