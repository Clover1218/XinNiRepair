<template>
  <view class="page" v-if="order">
    <!-- 状态卡片 -->
    <view class="status-card">
      <view class="status-row">
        <view class="status-title">
          <text v-if="order.category_name">{{ order.category_name }}</text>
          <template v-if="order.property_name">
            <text class="cat-sep">·</text>{{ order.property_name }}
          </template>
          <text v-if="!order.category_name">报修工单</text>
        </view>
        <text class="status-text" :style="{ color: statusColor(order.status) }">{{ order.status_label }}</text>
      </view>
      <view v-if="order.order_no" class="order-no">{{ order.order_no }}</view>
      <view class="status-sub">
        创建于 {{ formatDateTime(order.created_at) }}
        <text v-if="order.submitted_at"> · 提交于 {{ formatDateTime(order.submitted_at) }}</text>
      </view>
    </view>

    <!-- 报修人（提交账户） -->
    <view class="reporter-card">
      <image
        v-if="order.reporter && order.reporter.avatar_url"
        class="reporter-avatar"
        :src="order.reporter.avatar_url"
        mode="aspectFill"
      ></image>
      <view v-else class="reporter-avatar reporter-avatar--default">{{ reporterInitial }}</view>
      <view class="reporter-info">
        <view class="reporter-name">{{ order.reporter && order.reporter.nickname ? order.reporter.nickname : '未知用户' }}</view>
        <view class="reporter-tag">报修人</view>
      </view>
    </view>

    <!-- 被退回提示：仅当前状态为“已退回”时展示（重提并走完正常流程后不再显示） -->
    <view v-if="order.status === 'rejected' && order.reject_reason" class="reject-card">
      <text class="reject-title">已退回：</text>
      <text class="reject-text">{{ order.reject_reason }}</text>
    </view>

    <!-- 报修信息 -->
    <view class="info-card">
      <view class="info-row">
        <text class="info-label">报修单位</text>
        <text class="info-value">{{ order.enterprise_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目大类</text>
        <text class="info-value">{{ order.category_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目属性</text>
        <text class="info-value">{{ order.property_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">紧急程度</text>
        <text class="info-value">{{ order.urgency_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">位置</text>
        <text class="info-value">{{ order.room }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">联系人</text>
        <text class="info-value">{{ order.contact }}</text>
      </view>
    </view>

    <!-- 报修描述 -->
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

    <!-- 完工对账信息 -->
    <view v-if="order.status === 'completed'" class="info-card">
      <view class="card-title">维修结算</view>
      <view class="info-row" v-if="order.repair_content">
        <text class="info-label">维修内容</text>
        <text class="info-value">{{ order.repair_content }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">数量 / 单价</text>
        <text class="info-value">{{ order.quantity }} × ￥{{ formatAmount(order.unit_price) }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">金额</text>
        <text class="info-value amount">￥{{ formatAmount(order.amount) }}</text>
      </view>
      <template v-if="order.metadata">
        <view class="info-row" v-if="order.metadata.repair_result">
          <text class="info-label">维修结果</text>
          <text class="info-value">{{ order.metadata.repair_result }}</text>
        </view>
        <view class="info-row" v-if="order.metadata.repair_method">
          <text class="info-label">维修方式</text>
          <text class="info-value">{{ order.metadata.repair_method }}</text>
        </view>
        <view class="info-row" v-if="order.metadata.warranty_period">
          <text class="info-label">保修期</text>
          <text class="info-value">{{ order.metadata.warranty_period }}</text>
        </view>
        <view class="info-row" v-if="order.metadata.repair_duration !== undefined && order.metadata.repair_duration !== null">
          <text class="info-label">维修时长</text>
          <text class="info-value">{{ order.metadata.repair_duration }} 分钟</text>
        </view>
        <view class="info-row" v-if="order.metadata.extra_remark">
          <text class="info-label">额外备注</text>
          <text class="info-value">{{ order.metadata.extra_remark }}</text>
        </view>
      </template>
      <view class="info-row" v-if="order.auditor_name">
        <text class="info-label">审核人</text>
        <text class="info-value">{{ order.auditor_name }}</text>
      </view>
      <view class="info-row" v-if="order.repairer_name">
        <text class="info-label">维修员</text>
        <text class="info-value">{{ order.repairer_name }}</text>
      </view>
    </view>

    <!-- 收据 -->
    <view v-if="order.receipts && order.receipts.length > 0" class="img-card">
      <view class="card-title">收据凭证</view>
      <view class="img-grid">
        <image
          v-for="(img, index) in order.receipts"
          :key="img.id"
          class="img-item"
          :src="img.url"
          mode="aspectFill"
          @click="previewReceipt(index)"
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
      <template v-if="order.status === 'draft' || order.status === 'rejected'">
        <view class="footer-btn">
          <wd-button type="primary" round block @click="goEdit">编辑</wd-button>
        </view>
      </template>
      <template v-else-if="order.status === 'reported' || order.status === 'pending_accept'">
        <view class="footer-btn">
          <wd-button type="warning" plain round block @click="cancelOrder">取消工单</wd-button>
        </view>
      </template>
    </view>

    <!-- 取消工单弹窗：与 order-action-bar 的退回弹窗统一（wd-popup + wd-textarea） -->
    <wd-popup v-model="showCancelPopup" position="center" round custom-style="width: 86%;">
      <view class="popup-body">
        <view class="popup-title">取消工单</view>
        <view class="popup-label">
          取消原因<text class="popup-label-req">（必填，≤200字）</text>
        </view>
        <wd-textarea
          v-model="cancelReason"
          placeholder="请填写取消原因"
          :maxlength="200"
          show-word-limit
          auto-height
          custom-style="min-height: 160rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
        />
        <view class="popup-actions">
          <wd-button plain :custom-style="actBtnStyle" size="small" @click="showCancelPopup = false">取消</wd-button>
          <wd-button type="danger" :custom-style="actBtnStyle" size="small" :loading="cancelSubmitting" @click="confirmCancel">
            确认取消
          </wd-button>
        </view>
      </view>
    </wd-popup>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { formatAmount, formatDateTime, statusTagType, statusColor } from '@/utils/format'
import type { OrderDetail } from '@/types'

export default defineComponent({
  setup() {
    return {
      formatAmount,
      formatDateTime,
      statusTagType,
      statusColor
    }
  },
  computed: {
    reporterInitial(): string {
      const name = this.order && this.order.reporter ? this.order.reporter.nickname : ''
      return name ? name.charAt(0) : '?'
    }
  },
  data() {
    return {
      orderId: '',
      order: null as OrderDetail | null,
      /** 取消工单弹窗 */
      showCancelPopup: false,
      cancelReason: '',
      cancelSubmitting: false,
      /** 弹窗按钮内联样式：圆角矩形 + 实心白字 */
      actBtnStyle: 'margin-left: 12rpx; border-radius: 12rpx'
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
    previewReceipt(index: number) {
      if (!this.order) return
      uni.previewImage({
        current: this.order.receipts[index].url,
        urls: this.order.receipts.map((i) => i.url)
      })
    },
    goEdit() {
      uni.navigateTo({ url: `/pages/order/edit?id=${this.orderId}` })
    },
    /** 取消工单：打开原因弹窗（与 order-action-bar 退回弹窗统一） */
    cancelOrder() {
      this.cancelReason = ''
      this.showCancelPopup = true
    },
    /** 确认取消工单 */
    async confirmCancel() {
      const reason = this.cancelReason.trim()
      if (!reason) {
        uni.showToast({ title: '请填写取消原因', icon: 'none' })
        return
      }
      if (this.cancelSubmitting) return
      this.cancelSubmitting = true
      try {
        await http.post(`/orders/${this.orderId}/cancel`, { reason })
        uni.showToast({ title: '已取消', icon: 'success' })
        this.showCancelPopup = false
        this.loadDetail()
      } catch (e) {
        console.error('取消失败', e)
      } finally {
        this.cancelSubmitting = false
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

    .status-title {
      flex: 1;
      min-width: 0;
      margin-right: 20rpx;
      display: flex;
      align-items: center;
      font-size: 36rpx;
      font-weight: 600;
      color: #1a1a1a;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;

      .cat-sep {
        margin: 0 8rpx;
        color: #cccccc;
        font-weight: 400;
      }
    }

    /* 状态：纯文字 + 颜色（替代胶囊标签） */
    .status-text {
      flex-shrink: 0;
      font-size: 28rpx;
      font-weight: 600;
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

.reporter-card {
  display: flex;
  align-items: center;
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 24rpx 28rpx;
  margin-top: 20rpx;

  .reporter-avatar {
    width: 84rpx;
    height: 84rpx;
    border-radius: 50%;
    margin-right: 20rpx;
    flex-shrink: 0;
    background-color: #4d80f0;

    &--default {
      display: flex;
      align-items: center;
      justify-content: center;
      color: #ffffff;
      font-size: 36rpx;
      font-weight: 600;
    }
  }

  .reporter-info {
    flex: 1;
    min-width: 0;

    .reporter-name {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .reporter-tag {
      margin-top: 6rpx;
      font-size: 24rpx;
      color: #999999;
    }
  }
}

.reject-card {
  background-color: #fff7e6;
  border: 2rpx solid #ffd591;
  border-radius: 20rpx;
  padding: 20rpx 28rpx;
  margin-top: 20rpx;
  font-size: 26rpx;
  line-height: 1.5;

  .reject-title {
    color: #ad6800;
    font-weight: 600;
  }

  .reject-text {
    color: #ad6800;
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

    &.amount {
      color: #fa5151;
      font-weight: 600;
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

/* 取消工单弹窗（与 order-action-bar 退回弹窗一致） */
.popup-body {
  padding: 36rpx 32rpx 28rpx;
  background-color: #ffffff;
  border-radius: 20rpx;
}

.popup-title {
  font-size: 32rpx;
  font-weight: 600;
  color: #1a1a1a;
}

.popup-label {
  margin-top: 24rpx;
  font-size: 26rpx;
  color: #333333;

  .popup-label-req {
    color: #fa5151;
  }
}

.popup-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 28rpx;
}
</style>
