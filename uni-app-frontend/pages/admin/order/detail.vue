<template>
  <view class="page" v-if="detail">
    <!-- 状态卡片 -->
    <view class="status-card">
      <view class="status-row">
        <view class="status-project">{{ detail.project_name || '未命名工单' }}</view>
        <wd-tag :type="statusTagType(detail.status)" round>{{ detail.status_label }}</wd-tag>
      </view>
      <view v-if="detail.order_no" class="order-no">{{ detail.order_no }}</view>
      <view class="status-sub">
        提交于 {{ formatDateTime(detail.submitted_at || detail.created_at) }}
      </view>
    </view>

    <!-- 报修信息 -->
    <view class="info-card">
      <view class="info-row">
        <text class="info-label">企业</text>
        <text class="info-value">{{ detail.enterprise_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">报修人</text>
        <text class="info-value">{{ reporterName }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目大类</text>
        <text class="info-value">{{ detail.category_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目属性</text>
        <text class="info-value">{{ detail.property_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">紧急程度</text>
        <text class="info-value">{{ detail.urgency_label }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">位置</text>
        <text class="info-value">{{ detail.room }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">联系人</text>
        <text class="info-value">{{ detail.contact }}</text>
      </view>
      <view class="info-row info-desc">
        <text class="info-label">报修描述</text>
        <text class="info-value info-desc-text">{{ detail.description }}</text>
      </view>
      <view v-if="detail.reject_reason" class="info-row info-reject">
        <text class="info-label">退回原因</text>
        <text class="info-value reject-text">{{ detail.reject_reason }}</text>
      </view>
    </view>

    <!-- 故障图片 -->
    <view v-if="detail.images && detail.images.length > 0" class="img-card">
      <view class="card-title">故障图片</view>
      <view class="img-grid">
        <image
          v-for="(img, index) in detail.images"
          :key="img.id"
          class="img-item"
          :src="img.url"
          mode="aspectFill"
          @click="previewFaultImages(index)"
        ></image>
      </view>
    </view>

    <!-- 收据 -->
    <view v-if="detail.receipts && detail.receipts.length > 0" class="img-card">
      <view class="card-title">收据凭证</view>
      <view class="img-grid">
        <image
          v-for="img in detail.receipts"
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
        <view v-for="(t, index) in detail.timeline" :key="t.id" class="timeline-item">
          <view class="timeline-dot" :class="{ last: index === detail.timeline.length - 1 }"></view>
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
    <view v-if="actions.length > 0" class="footer">
      <view v-for="act in actions" :key="act.action" class="footer-btn">
        <wd-button
          :type="actionBtnType(act.action)"
          round
          block
          @click="onAction(act)"
        >
          {{ act.label }}
        </wd-button>
      </view>
    </view>

    <!-- 退回弹窗 -->
    <wd-popup v-model="showRejectPopup" position="center" round custom-style="width: 86%;">
      <view class="popup-body">
        <view class="popup-title">退回工单</view>
        <wd-textarea
          v-model="rejectReason"
          placeholder="请输入退回原因（10-200字符）"
          :maxlength="200"
          show-word-limit
          auto-height
          custom-style="min-height: 160rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
        />
        <view class="popup-actions">
          <wd-button plain round size="small" @click="showRejectPopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmReject">
            确认退回
          </wd-button>
        </view>
      </view>
    </wd-popup>

    <!-- 完工弹窗 -->
    <wd-popup v-model="showCompletePopup" position="center" round custom-style="width: 86%;">
      <view class="popup-body">
        <view class="popup-title">完工</view>
        <wd-textarea
          v-model="completeRemark"
          placeholder="请输入维修备注（≤200字符）"
          :maxlength="200"
          show-word-limit
          auto-height
          custom-style="min-height: 140rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
        />
        <view class="receipt-section">
          <view class="receipt-title">收据图片（最多3张）</view>
          <view class="receipt-grid">
            <view v-for="(r, index) in receiptList" :key="index" class="receipt-item">
              <image class="receipt-img" :src="r.localPath || r.url" mode="aspectFill" />
              <view class="receipt-del" @click.stop="removeReceipt(index)">×</view>
            </view>
            <view v-if="receiptList.length < 3" class="receipt-add" @click="chooseReceipt">
              <text class="receipt-add-icon">+</text>
              <text class="receipt-add-text">上传图片</text>
            </view>
          </view>
        </view>
        <view class="popup-actions">
          <wd-button plain round size="small" @click="showCompletePopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmComplete">
            确认完工
          </wd-button>
        </view>
      </view>
    </wd-popup>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http, uploadReceipt } from '@/utils/request'
import { isPlatformAdmin } from '@/utils/jwt'
import { formatDateTime, statusTagType } from '@/utils/format'
import type { AdminOrderDetail, AvailableAction } from '@/types'

interface ReceiptItem {
  url: string
  localPath?: string
}

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
      detail: null as AdminOrderDetail | null,
      showRejectPopup: false,
      rejectReason: '',
      showCompletePopup: false,
      completeRemark: '',
      receiptList: [] as ReceiptItem[],
      submitting: false
    }
  },
  computed: {
    actions(): AvailableAction[] {
      return this.detail?.available_actions || []
    },
    reporterName(): string {
      // 管理员详情返回 reporter 对象；若无则回退使用工单号（占位）
      const d = this.detail as AdminOrderDetail & { reporter?: { nickname: string } }
      return d?.reporter?.nickname || '--'
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
  },
  onShow() {
    if (!isPlatformAdmin()) {
      uni.showToast({ title: '无管理权限', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 600)
      return
    }
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
        this.detail = await http.get<AdminOrderDetail>(`/admin/orders/${this.orderId}`)
      } catch (e) {
        console.error('加载工单详情失败', e)
      }
    },
    previewFaultImages(index: number) {
      if (!this.detail) return
      uni.previewImage({
        current: this.detail.images[index].url,
        urls: this.detail.images.map((i) => i.url)
      })
    },
    actionBtnType(action: string): 'primary' | 'warning' | 'danger' | 'success' {
      switch (action) {
        case 'complete':
          return 'success'
        case 'reject':
          return 'danger'
        case 'review':
          return 'primary'
        case 'accept':
          return 'warning'
        default:
          return 'primary'
      }
    },
    onAction(act: AvailableAction) {
      if (act.action === 'reject') {
        this.showRejectPopup = true
        this.rejectReason = ''
        return
      }
      if (act.action === 'complete') {
        this.showCompletePopup = true
        this.completeRemark = ''
        this.receiptList = []
        return
      }
      // review / accept：有 require_confirm 先弹确认
      const doSubmit = () => {
        if (act.action === 'review') this.reviewOrder()
        else if (act.action === 'accept') this.acceptOrder()
      }
      if (act.require_confirm || act.confirm_message) {
        uni.showModal({
          title: act.label,
          content: act.confirm_message || `确认执行「${act.label}」？`,
          success: (res) => {
            if (res.confirm) doSubmit()
          }
        })
      } else {
        doSubmit()
      }
    },
    async reviewOrder() {
      if (this.submitting) return
      this.submitting = true
      try {
        await http.post(`/admin/orders/${this.orderId}/review`, {})
        uni.showToast({ title: '已查阅', icon: 'success' })
        this.loadDetail()
      } catch (e) {
        console.error('查阅失败', e)
      } finally {
        this.submitting = false
      }
    },
    async acceptOrder() {
      if (this.submitting) return
      this.submitting = true
      try {
        await http.post(`/admin/orders/${this.orderId}/accept`, {})
        uni.showToast({ title: '已接单', icon: 'success' })
        this.loadDetail()
      } catch (e) {
        console.error('接单失败', e)
      } finally {
        this.submitting = false
      }
    },
    async confirmReject() {
      const reason = this.rejectReason.trim()
      const action = this.actions.find((a) => a.action === 'reject')
      const minLength = action?.reason_min_length || 10
      if (reason.length < minLength) {
        uni.showToast({ title: `退回原因至少${minLength}字`, icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      try {
        await http.post(`/admin/orders/${this.orderId}/reject`, { reason })
        uni.showToast({ title: '已退回', icon: 'success' })
        this.showRejectPopup = false
        this.loadDetail()
      } catch (e) {
        console.error('退回失败', e)
      } finally {
        this.submitting = false
      }
    },
    /** 从微信聊天记录选择收据图片并上传 */
    chooseReceipt() {
      const remain = 3 - this.receiptList.length
      if (remain <= 0) {
        uni.showToast({ title: '最多上传3张收据', icon: 'none' })
        return
      }
      wx.chooseMessageFile({
        count: remain,
        type: 'image',
        success: async (res) => {
          const files: { path: string; name: string; size: number }[] = res.tempFiles || []
          const validFiles = files.filter((file) => {
            const ext = (file.name || '').split('.').pop()?.toLowerCase()
            return ['jpg', 'jpeg', 'png', 'webp'].includes(ext) && file.size <= 5 * 1024 * 1024
          })
          if (validFiles.length === 0) {
            uni.showToast({ title: '仅支持 jpg/png/webp 且 ≤5MB 的图片', icon: 'none' })
            return
          }
          uni.showLoading({ title: '上传中...' })
          let uploaded = 0
          for (const file of validFiles) {
            try {
              const data = await uploadReceipt(this.orderId, file.path)
              this.receiptList.push({ url: data.url, localPath: file.path })
              uploaded++
            } catch (e) {
              console.error('收据上传失败', e)
            }
          }
          uni.hideLoading()
          if (uploaded > 0) {
            uni.showToast({ title: `已上传${uploaded}张`, icon: 'success' })
          }
        },
        fail: (err) => {
          console.error('选择图片失败', err)
        }
      })
    },
    removeReceipt(index: number) {
      this.receiptList.splice(index, 1)
    },
    async confirmComplete() {
      const remark = this.completeRemark.trim()
      if (!remark) {
        uni.showToast({ title: '请填写维修备注', icon: 'none' })
        return
      }
      if (this.receiptList.length === 0) {
        uni.showToast({ title: '请上传收据图片', icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      try {
        await http.post(`/admin/orders/${this.orderId}/complete`, {
          remark,
          receipts: this.receiptList.map((r) => r.url)
        })
        uni.showToast({ title: '完工成功', icon: 'success' })
        this.showCompletePopup = false
        this.loadDetail()
      } catch (e) {
        console.error('完工失败', e)
      } finally {
        this.submitting = false
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
.timeline-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 12rpx 28rpx;
  margin-top: 20rpx;
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
      margin-top: 10rpx;
      flex-shrink: 0;

      &.last {
        background-color: #cccccc;
      }
    }

    .timeline-content {
      margin-left: 20rpx;
      flex: 1;

      .timeline-action {
        font-size: 28rpx;
        color: #333333;
      }

      .timeline-meta {
        margin-top: 6rpx;
        font-size: 24rpx;
        color: #999999;
      }

      .timeline-remark {
        margin-top: 8rpx;
        font-size: 24rpx;
        color: #666666;
        background-color: #f5f6f8;
        border-radius: 12rpx;
        padding: 12rpx 16rpx;
        line-height: 1.5;
      }
    }
  }
}

.footer {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  background-color: #ffffff;
  padding: 20rpx 24rpx calc(20rpx + env(safe-area-inset-bottom));
  box-shadow: 0 -4rpx 16rpx rgba(0, 0, 0, 0.04);

  .footer-btn {
    flex: 1;
    margin: 0 12rpx;
  }
}

.popup-body {
  padding: 40rpx 32rpx 32rpx;

  .popup-title {
    font-size: 34rpx;
    font-weight: 600;
    color: #1a1a1a;
    text-align: center;
    margin-bottom: 28rpx;
  }

  .popup-actions {
    display: flex;
    margin-top: 32rpx;

    wd-button {
      flex: 1;
      margin: 0 12rpx;
    }
  }
}

.receipt-section {
  margin-top: 24rpx;

  .receipt-title {
    font-size: 26rpx;
    color: #999999;
  }

  .receipt-grid {
    display: flex;
    flex-wrap: wrap;
    margin-top: 16rpx;

    .receipt-item {
      position: relative;
      width: 150rpx;
      height: 150rpx;
      margin-right: 16rpx;
      margin-bottom: 16rpx;

      .receipt-img {
        width: 100%;
        height: 100%;
        border-radius: 12rpx;
        background-color: #f5f6f8;
      }

      .receipt-del {
        position: absolute;
        top: -12rpx;
        right: -12rpx;
        width: 40rpx;
        height: 40rpx;
        border-radius: 50%;
        background-color: rgba(0, 0, 0, 0.6);
        color: #ffffff;
        font-size: 28rpx;
        line-height: 40rpx;
        text-align: center;
      }
    }

    .receipt-add {
      width: 150rpx;
      height: 150rpx;
      border-radius: 12rpx;
      border: 2rpx dashed #d9d9d9;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      background-color: #fafbfc;

      .receipt-add-icon {
        font-size: 48rpx;
        color: #bbbbbb;
        line-height: 1;
      }

      .receipt-add-text {
        margin-top: 8rpx;
        font-size: 22rpx;
        color: #bbbbbb;
      }
    }
  }
}
</style>
