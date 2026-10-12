<template>
  <view class="page" v-if="order">
    <scroll-view scroll-y class="body-scroll">
    <!-- 被退回提示（仅 status==='rejected' 才显示，避免重提后残留旧原因） -->
    <view v-if="order.status === 'rejected' && order.reject_reason" class="reject-bar">
      <text class="reject-title">退回原因：</text>
      <text class="reject-text">{{ order.reject_reason }}</text>
    </view>

    <!-- ① 报修信息：地点大字 + 报修内容 + 报修人 + 状态胶囊在标题右 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <wd-icon name="location" size="30rpx" color="#6b7280"></wd-icon>
        <text class="xn-card-title-text">报修信息</text>
        <text class="status-pill" :class="pill.cls">{{ pill.text }}</text>
      </view>
      <view class="onsite-place">
        {{ order.enterprise_name || '未指定单位' }}：{{ order.room || '未填写地点' }}
      </view>
      <view v-if="order.category_name || order.property_name" class="onsite-tags">
        报修内容：{{ order.category_name || '—' }} → {{ order.property_name || '—' }}
      </view>
      <view class="onsite-row">
        <view class="onsite-reporter">
          <image
            v-if="reporterAvatar"
            class="or-avatar"
            :src="reporterAvatar"
            mode="aspectFill"
          ></image>
          <view v-else class="or-avatar or-avatar--default">{{ reporterInitial }}</view>
          <view class="or-text">
            <view class="or-name">{{ reporterName }}</view>
            <view class="or-phone">{{ contactPhone || '未留联系方式' }}</view>
          </view>
        </view>
      </view>
    </view>

    <!-- ② 故障详情：描述（长文折叠）+ 故障图网格 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <wd-icon name="warning" size="30rpx" color="#6b7280"></wd-icon>
        <text class="xn-card-title-text">故障详情</text>
      </view>
      <view class="fault-desc" :class="{ clamp: isLongDesc && !descExpanded }">
        {{ order.description || '（无故障描述）' }}
      </view>
      <view v-if="isLongDesc" class="desc-toggle" @click="descExpanded = !descExpanded">
        {{ descExpanded ? '收起' : '查看全文' }}
      </view>
      <view v-if="order.images && order.images.length" class="img-strip">
        <image
          v-for="(img, index) in order.images"
          :key="img.id"
          class="img-cell"
          :src="img.url"
          mode="aspectFill"
          @click="previewImages(order.images, index)"
        ></image>
      </view>
    </view>

    <!-- ③ 维修结算：仅已完成且有对账数据（金额苍岭绿，红色专指已退回） -->
    <view v-if="showFinance" class="xn-card finance-card">
      <view class="xn-card-title">
        <text class="xn-card-title-text">维修结算</text>
      </view>
      <view class="fin-amount">
        <text class="fa-value">¥{{ financeAmountText }}</text>
      </view>
      <view class="fin-divider"></view>
      <view v-if="order.repair_content" class="fin-section">
        <view class="fin-subtitle">维修操作内容</view>
        <view class="fin-text">{{ order.repair_content }}</view>
      </view>
      <template
        v-if="order.metadata && (order.metadata.repair_result || order.metadata.warranty_period || (order.metadata.repair_duration !== undefined && order.metadata.repair_duration !== null))"
      >
        <view class="fin-section">
          <view class="fin-subtitle">维修附加信息</view>
          <view class="fin-text">
            <text v-if="order.metadata.repair_result">结果：{{ order.metadata.repair_result }}</text>
            <text v-if="order.metadata.repair_result && order.metadata.warranty_period"> · </text>
            <text v-if="order.metadata.warranty_period">保修期：{{ order.metadata.warranty_period }}</text>
            <text
              v-if="(order.metadata.repair_result || order.metadata.warranty_period) && order.metadata.repair_duration !== undefined && order.metadata.repair_duration !== null"
            >
              ·
            </text>
            <text v-if="order.metadata.repair_duration !== undefined && order.metadata.repair_duration !== null">
              时长：{{ order.metadata.repair_duration }} 分钟
            </text>
          </view>
        </view>
      </template>
      <view class="fin-divider"></view>
      <view class="fin-section">
        <view class="fin-subtitle">收据</view>
        <view v-if="order.receipts && order.receipts.length" class="img-strip img-strip--receipt">
          <image
            v-for="(img, index) in order.receipts"
            :key="img.id"
            class="img-cell"
            :src="img.url"
            mode="aspectFill"
            @click="previewImages(order.receipts, index)"
          ></image>
        </view>
        <view v-else class="fin-receipt-empty">暂无收据</view>
      </view>
    </view>

    <!-- ④ 归档信息：两端对齐 kv，行间无分隔线 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <text class="xn-card-title-text">归档信息</text>
      </view>
      <view class="kv-list">
        <view class="kv">
          <text class="k">工单号</text>
          <view class="v v-strong arch-no">
            <text>{{ order.order_no }}</text>
            <wd-icon name="copy" size="26rpx" color="#9ca3af" @click="copyOrderNo"></wd-icon>
          </view>
        </view>
        <view class="kv">
          <text class="k">提交时间</text>
          <text class="v">{{ formatDateTime(order.submitted_at || order.created_at) }}</text>
        </view>
        <view class="kv">
          <text class="k">紧急程度</text>
          <view class="v">
            <text class="urgency-tag" :class="urgencyTagClass">{{ order.urgency_label || '普通' }}</text>
          </view>
        </view>
        <view v-if="order.auditor_name" class="kv">
          <text class="k">审核人</text>
          <text class="v v-strong">{{ order.auditor_name }}</text>
        </view>
        <view v-if="order.repairer_name" class="kv">
          <text class="k">维修人员</text>
          <text class="v v-strong">{{ order.repairer_name }}</text>
        </view>
      </view>
    </view>

    <!-- ⑤ 处理记录：默认折叠，竖线 + 五色节点 + 状态流转胶囊 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <text class="xn-card-title-text">处理记录</text>
        <text class="title-count">（{{ order.timeline.length }}）</text>
      </view>
      <view v-if="!timelineExpanded" class="timeline-collapsed">
        <text class="timeline-expand-btn" @click="timelineExpanded = true">查看全部处理轨迹</text>
      </view>
      <view v-else class="xn-timeline">
        <view
          v-for="(t, index) in order.timeline"
          :key="t.id"
          class="tl-item"
          :class="tlNodeClass(t.action)"
        >
          <view v-if="index < order.timeline.length - 1" class="tl-tail"></view>
          <view class="tl-node"></view>
          <view class="tl-body">
            <view class="tl-row1">
              <text class="tl-action">{{ t.action_label || timeAxisActionLabel(t.action) }}</text>
              <text v-if="t.operator_name" class="tl-operator">{{ t.operator_name }}</text>
            </view>
            <view class="tl-row2">{{ formatDateTime(t.created_at) }}</view>
            <view
              v-if="t.from_status && t.to_status && t.from_status !== t.to_status"
              class="tl-row3"
            >
              <text class="tl-pill tl-pill-from">{{ statusLabel(t.from_status) }}</text>
              <wd-icon name="arrow-right" size="20rpx" color="#9ca3af"></wd-icon>
              <text class="tl-pill tl-pill-to">{{ statusLabel(t.to_status) }}</text>
            </view>
            <view v-if="t.remark" class="tl-remark">{{ t.remark }}</view>
          </view>
        </view>
        <text class="timeline-collapse-btn" @click="timelineExpanded = false">收起</text>
      </view>
    </view>
    </scroll-view>

    <!-- ⑥ 操作区：固定底栏（详情区独立滚动，底栏不遮挡内容） -->
    <view v-if="showFooterActions" class="footer">
      <view v-if="order.status === 'draft' || order.status === 'rejected'" class="footer-btn footer-btn--primary">
        <wd-button type="primary" round block size="large" @click="goEdit">编辑</wd-button>
      </view>
      <view v-else-if="order.status === 'reported' || order.status === 'pending_accept'" class="footer-btn">
        <wd-button type="warning" plain round block size="large" @click="cancelOrder">取消工单</wd-button>
      </view>
    </view>

    <!-- 取消工单弹窗 -->
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
import { formatAmount, formatDateTime, statusLabel, statusPill, timeAxisActionLabel } from '@/utils/format'
import type { OrderDetail, OrderImage } from '@/types'

export default defineComponent({
  setup() {
    return {
      formatAmount,
      formatDateTime,
      statusLabel,
      statusPill,
      timeAxisActionLabel
    }
  },
  computed: {
    /** 状态胶囊（与 Web 端 statusView 一致：待补签优先） */
    pill(): { text: string; cls: string } {
      return statusPill(this.order || undefined)
    },
    reporterAvatar(): string {
      return this.order?.reporter?.avatar_url || ''
    },
    reporterInitial(): string {
      const r = this.order?.reporter
      return r && r.nickname ? r.nickname.charAt(0) : '?'
    },
    reporterName(): string {
      return this.order?.reporter?.nickname || '未知用户'
    },
    /** 从 contact("王五 12345678910") 中解析手机号 */
    contactPhone(): string {
      const c = this.order?.contact || ''
      const m = c.match(/1\d{10}/)
      return m ? m[0] : c
    },
    /** 故障描述是否超过 60 字需要折叠 */
    isLongDesc(): boolean {
      return (this.order?.description || '').length > 60
    },
    /** 维修结算卡：仅已完成且有对账数据（金额敏感，但本人工单可见） */
    showFinance(): boolean {
      const d = this.order
      if (!d || d.status !== 'completed') return false
      const hasFinance =
        d.amount != null ||
        d.quantity != null ||
        d.unit_price != null ||
        !!d.repair_content ||
        !!(d.receipts && d.receipts.length > 0)
      return !!hasFinance
    },
    financeAmountText(): string {
      const a = this.order?.amount
      return a != null ? formatAmount(a) : '--'
    },
    /** 紧急程度配色 class */
    urgencyTagClass(): string {
      const u = this.order?.urgency
      if (u === 'very_urgent') return 'urg-very'
      if (u === 'urgent') return 'urg-high'
      return 'urg-normal'
    },
    /** 底部操作区是否展示（草稿/退回→编辑；已上报/待接单→取消） */
    showFooterActions(): boolean {
      const s = this.order?.status
      return s === 'draft' || s === 'rejected' || s === 'reported' || s === 'pending_accept'
    }
  },
  data() {
    return {
      orderId: '',
      order: null as OrderDetail | null,
      descExpanded: false,
      timelineExpanded: false,
      showCancelPopup: false,
      cancelReason: '',
      cancelSubmitting: false,
      /** 弹窗按钮内联样式：圆角矩形 + 实心白字 */
      actBtnStyle: 'margin-left: 16rpx; border-radius: 12rpx'
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
    previewImages(imgs: OrderImage[], index: number) {
      if (!imgs || !imgs.length) return
      uni.previewImage({ current: imgs[index].url, urls: imgs.map((i) => i.url) })
    },
    /** 复制工单号 */
    copyOrderNo() {
      const no = this.order?.order_no
      if (!no) return
      uni.setClipboardData({
        data: no,
        success: () => uni.showToast({ title: '工单号已复制', icon: 'none' })
      })
    },
    /** 时间轴节点配色 class（与 Web 端节点配色一致） */
    tlNodeClass(action?: string): string {
      switch (action) {
        case 'submit':
          return 'tl-blue'
        case 'accept':
        case 'reopen':
          return 'tl-orange'
        case 'complete':
        case 'audit':
        case 'sign':
          return 'tl-teal'
        case 'reject':
        case 'cancel':
          return 'tl-red'
        default:
          return 'tl-gray'
      }
    },
    goEdit() {
      uni.navigateTo({ url: `/pages/order/edit?id=${this.orderId}` })
    },
    /** 取消工单：打开原因弹窗 */
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
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-sizing: border-box;
}

/* 详情滚动区：占满底栏以上剩余空间，内容滚动不经过底栏 */
.body-scroll {
  flex: 1;
  min-height: 0;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
}

/* ===== 通用卡片外壳（对齐 Web 端 / admin 详情：1px 边框 + 24rpx 圆角 + 32rpx 内边距） ===== */
.xn-card {
  background-color: #ffffff;
  border: 1rpx solid var(--xn-border, #e5e7eb);
  border-radius: 24rpx;
  padding: 32rpx;
  margin-top: 20rpx;
}

.xn-card-title {
  display: flex;
  align-items: center;
  font-size: 30rpx;
  font-weight: 600;
  color: #111827;
  margin-bottom: 20rpx;
}

.xn-card-title-text {
  margin-left: 8rpx;
}

.title-count {
  font-size: 24rpx;
  color: #9ca3af;
  font-weight: 400;
  margin-left: 8rpx;
}

/* 状态胶囊（底色=状态色 12% 透明，文字=状态色；对齐 Web 端 .st-* 语义） */
.status-pill {
  margin-left: auto;
  display: inline-block;
  padding: 4rpx 18rpx;
  border-radius: 100rpx;
  font-size: 24rpx;
  font-weight: 600;
  line-height: 1.5;
}

.st-amber { color: #f59e0b; background: rgba(245, 158, 11, 0.12); }
.st-orange { color: #ea580c; background: rgba(234, 88, 12, 0.12); }
.st-blue { color: #2563eb; background: rgba(37, 99, 235, 0.12); }
.st-teal { color: #0f766e; background: rgba(15, 118, 110, 0.12); }
.st-red { color: #dc2626; background: rgba(220, 38, 38, 0.12); }
.st-gray { color: #64748b; background: rgba(100, 116, 139, 0.12); }
.st-pending-sign {
  color: #ffffff;
  background: linear-gradient(135deg, #0f766e, #2563eb);
}

/* ===== ① 报修信息 ===== */
.onsite-place {
  font-size: 34rpx;
  font-weight: 600;
  color: #111827;
  line-height: 1.4;
  margin: 8rpx 0 12rpx;
}

.onsite-tags {
  font-size: 26rpx;
  color: #6b7280;
  margin-bottom: 20rpx;
}

.onsite-row {
  display: flex;
  align-items: center;
}

.onsite-reporter {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
}

.or-avatar {
  width: 72rpx;
  height: 72rpx;
  border-radius: 50%;
  margin-right: 16rpx;
  flex-shrink: 0;
  background-color: #e5e7eb;

  &--default {
    display: flex;
    align-items: center;
    justify-content: center;
    color: #ffffff;
    font-size: 32rpx;
    font-weight: 600;
    background-color: #0f766e;
  }
}

.or-text {
  min-width: 0;
}

.or-name {
  font-size: 28rpx;
  font-weight: 600;
  color: #111827;
}

.or-phone {
  font-size: 24rpx;
  color: #6b7280;
  margin-top: 4rpx;
}

/* ===== 退回原因条 ===== */
.reject-bar {
  background-color: #fef2f2;
  border: 1rpx solid #fecaca;
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 20rpx;
}

.reject-title {
  font-size: 26rpx;
  color: #dc2626;
  font-weight: 600;
}

.reject-text {
  font-size: 26rpx;
  color: #b91c1c;
}

/* ===== ② 故障详情 ===== */
.fault-desc {
  font-size: 28rpx;
  color: #374151;
  line-height: 1.6;

  &.clamp {
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
}

.desc-toggle {
  margin-top: 12rpx;
  font-size: 26rpx;
  color: #0f766e;
}

.img-strip {
  display: flex;
  flex-wrap: wrap;
  margin-top: 20rpx;
}

.img-cell {
  width: 200rpx;
  height: 200rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
  border-radius: 12rpx;
  background-color: #f5f6f8;
}

.img-strip--receipt {
  margin-top: 12rpx;
}

/* ===== ③ 维修结算 ===== */
.fin-amount {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.fa-value {
  font-size: 56rpx;
  font-weight: 700;
  color: #0f766e;
}

.fin-divider {
  height: 1rpx;
  background-color: #eef2f4;
  margin: 20rpx 0;
}

.fin-section {
  margin-top: 16rpx;
}

.fin-subtitle {
  font-size: 24rpx;
  color: #9ca3af;
  margin-bottom: 8rpx;
}

.fin-text {
  font-size: 28rpx;
  color: #374151;
  line-height: 1.5;
}

.fin-receipt-empty {
  font-size: 26rpx;
  color: #9ca3af;
}

/* ===== ④ 归档信息：两端对齐 kv，无分隔线 ===== */
.kv-list {
  display: block;
}

.kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22rpx 0;
}

.k {
  font-size: 28rpx;
  color: #6b7280;
  flex-shrink: 0;
}

.v {
  font-size: 28rpx;
  color: #111827;
  margin-left: 24rpx;
  text-align: right;

  &.v-strong {
    font-weight: 600;
  }
}

.arch-no {
  display: flex;
  align-items: center;
  justify-content: flex-end;

  .wd-icon {
    margin-left: 10rpx;
  }
}

/* 紧急程度标签 */
.urgency-tag {
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
  font-size: 24rpx;
}

.urg-normal {
  color: #64748b;
  background-color: #f1f5f9;
}

.urg-high {
  color: #ea580c;
  background-color: rgba(234, 88, 12, 0.12);
}

.urg-very {
  color: #dc2626;
  background-color: rgba(220, 38, 38, 0.12);
}

/* ===== ⑤ 处理记录：竖线 + 五色节点 + 状态流转胶囊 ===== */
.timeline-collapsed {
  padding: 4rpx 0;
}

.timeline-expand-btn {
  font-size: 26rpx;
  color: #0f766e;
}

.timeline-collapse-btn {
  display: block;
  text-align: center;
  margin-top: 8rpx;
  font-size: 26rpx;
  color: #0f766e;
}

.xn-timeline {
  padding: 8rpx 0 4rpx;
}

.tl-item {
  position: relative;
  padding-left: 44rpx;
  padding-bottom: 28rpx;

  &:last-child {
    padding-bottom: 4rpx;
  }
}

.tl-tail {
  position: absolute;
  left: 13rpx;
  top: 26rpx;
  bottom: 0;
  width: 2rpx;
  background-color: #e5e7eb;
}

.tl-node {
  position: absolute;
  left: 6rpx;
  top: 8rpx;
  width: 16rpx;
  height: 16rpx;
  border-radius: 50%;
  background-color: #cbd5e1;
  z-index: 1;
}

.tl-blue .tl-node { background-color: #2563eb; }
.tl-orange .tl-node { background-color: #ea580c; }
.tl-teal .tl-node { background-color: #0f766e; }
.tl-red .tl-node { background-color: #dc2626; }
.tl-gray .tl-node { background-color: #94a3b8; }

.tl-body {
  min-width: 0;
}

.tl-row1 {
  display: flex;
  align-items: center;
}

.tl-action {
  font-size: 28rpx;
  font-weight: 600;
  color: #111827;
}

.tl-operator {
  font-size: 24rpx;
  color: #9ca3af;
  margin-left: 12rpx;
}

.tl-row2 {
  font-size: 24rpx;
  color: #9ca3af;
  margin-top: 4rpx;
}

.tl-row3 {
  display: flex;
  align-items: center;
  margin-top: 8rpx;
}

.tl-pill {
  font-size: 22rpx;
  padding: 2rpx 12rpx;
  border-radius: 100rpx;
  background-color: #f1f5f9;
  color: #475569;
}

.tl-row3 .wd-icon {
  margin: 0 6rpx;
}

.tl-remark {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: #6b7280;
  background-color: #f5f6f8;
  border-radius: 12rpx;
  padding: 12rpx 16rpx;
  line-height: 1.5;
}

/* ===== ⑥ 操作区（固定底栏：详情区独立滚动，底栏 flex-shrink:0 常驻底部） ===== */
.footer {
  flex-shrink: 0;
  display: flex;
  background-color: #ffffff;
  padding: 20rpx 24rpx calc(20rpx + env(safe-area-inset-bottom));
  border-top: 1rpx solid var(--xn-border, #e5e7eb);

  .footer-btn {
    flex: 1;
    margin: 0 12rpx;

    &--primary {
      flex: 1.4;
    }
  }
}

/* ===== 取消工单弹窗 ===== */
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
    color: #dc2626;
  }
}

.popup-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 28rpx;
}
</style>
