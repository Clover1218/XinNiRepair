<template>
  <view class="page" v-if="detail">
    <!-- 状态卡片 -->
    <view class="status-card">
      <view class="status-row">
        <view class="status-title">
          <text v-if="detail.category_name">{{ detail.category_name }}</text>
          <template v-if="detail.property_name">
            <text class="cat-sep">·</text>{{ detail.property_name }}
          </template>
          <text v-if="!detail.category_name">工单处理</text>
        </view>
        <wd-tag :type="statusTagType(detail.status)" round>{{ detail.status_label }}</wd-tag>
      </view>
      <view v-if="detail.order_no" class="order-no">{{ detail.order_no }}</view>
      <view class="status-sub">
        提交于 {{ formatDateTime(detail.submitted_at || detail.created_at) }}
        <text v-if="detail.accepted_at"> · 接单于 {{ formatDateTime(detail.accepted_at) }}</text>
        <text v-if="detail.completed_at"> · 完工于 {{ formatDateTime(detail.completed_at) }}</text>
      </view>
    </view>

    <!-- 被退回提示 -->
    <view v-if="detail.reject_reason" class="reject-card">
      <text class="reject-title">退回原因：</text>
      <text class="reject-text">{{ detail.reject_reason }}</text>
    </view>

    <!-- 报修信息 -->
    <view class="info-card">
      <view class="info-row">
        <text class="info-label">单位</text>
        <text class="info-value">{{ detail.enterprise_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">报修人</text>
        <text class="info-value">{{ reporterName }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目大类</text>
        <text class="info-value">{{ detail.category_name || '--' }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">项目属性</text>
        <text class="info-value">{{ detail.property_name || '--' }}</text>
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
      <view class="info-row">
        <text class="info-label">报修描述</text>
        <text class="info-value desc-text">{{ detail.description }}</text>
      </view>
      <view class="info-row" v-if="detail.auditor_name">
        <text class="info-label">审核人</text>
        <text class="info-value">{{ detail.auditor_name }}</text>
      </view>
      <view class="info-row" v-if="detail.repairer_name">
        <text class="info-label">接单人</text>
        <text class="info-value">{{ detail.repairer_name }}</text>
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
          @click="previewImages(detail.images, index)"
        ></image>
      </view>
    </view>

    <!-- 对账信息 -->
    <view v-if="detail.status === 'completed'" class="info-card">
      <view class="card-title">对账信息</view>
      <view class="info-row" v-if="detail.repair_content">
        <text class="info-label">维修内容</text>
        <text class="info-value">{{ detail.repair_content }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">数量 / 单价</text>
        <text class="info-value">{{ detail.quantity }} × ￥{{ formatAmount(detail.unit_price) }}</text>
      </view>
      <view class="info-row">
        <text class="info-label">金额</text>
        <text class="info-value amount">￥{{ formatAmount(detail.amount) }}</text>
      </view>
      <template v-if="detail.metadata">
        <view class="info-row" v-if="detail.metadata.repair_result">
          <text class="info-label">维修结果</text>
          <text class="info-value">{{ detail.metadata.repair_result }}</text>
        </view>
        <view class="info-row" v-if="detail.metadata.repair_method">
          <text class="info-label">维修方式</text>
          <text class="info-value">{{ detail.metadata.repair_method }}</text>
        </view>
        <view class="info-row" v-if="detail.metadata.warranty_period">
          <text class="info-label">保修期</text>
          <text class="info-value">{{ detail.metadata.warranty_period }}</text>
        </view>
        <view class="info-row" v-if="detail.metadata.repair_duration !== undefined && detail.metadata.repair_duration !== null">
          <text class="info-label">维修时长</text>
          <text class="info-value">{{ detail.metadata.repair_duration }} 分钟</text>
        </view>
        <view class="info-row" v-if="detail.metadata.extra_remark">
          <text class="info-label">额外备注</text>
          <text class="info-value">{{ detail.metadata.extra_remark }}</text>
        </view>
      </template>
    </view>

    <!-- 收据凭证 -->
    <view v-if="detail.receipts && detail.receipts.length > 0" class="img-card">
      <view class="card-title">收据凭证</view>
      <view class="img-grid">
        <image
          v-for="(img, index) in detail.receipts"
          :key="img.id"
          class="img-item"
          :src="img.url"
          mode="aspectFill"
          @click="previewImages(detail.receipts, index)"
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
            <view class="timeline-action">{{ t.action_label || timeAxisActionLabel(t.action) }}</view>
            <view class="timeline-meta">
              {{ t.operator_name }} · {{ formatDateTime(t.created_at) }}
            </view>
            <view v-if="t.remark" class="timeline-remark">{{ t.remark }}</view>
          </view>
        </view>
      </view>
    </view>

    <!-- 底部操作 -->
    <view v-if="filteredActions.length > 0" class="footer">
      <view v-for="act in filteredActions" :key="act.action" class="footer-btn">
        <wd-button :type="actionBtnType(act.action)" round block @click="onAction(act)">
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
    <wd-popup v-model="showCompletePopup" position="center" round custom-style="width: 90%;">
      <view class="popup-body complete-body">
        <view class="popup-title">完工登记</view>
        <scroll-view scroll-y class="complete-scroll">
          <view class="complete-block-title">维修备注（必填，≤200字）</view>
          <wd-textarea
            v-model="completeRemark"
            placeholder="请输入维修过程及结果"
            :maxlength="200"
            show-word-limit
            auto-height
            custom-style="min-height: 120rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
          />

          <view class="complete-block-title">对账信息（必填）</view>
          <view class="form-row">
            <view class="form-cell form-cell-half">
              <text class="cell-label">数量（≥0）</text>
              <input
                class="cell-input"
                v-model="finance.quantity"
                type="number"
                placeholder="如：1"
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">单价（元，≥0）</text>
              <input
                class="cell-input"
                v-model="finance.unitPrice"
                type="digit"
                placeholder="如：150"
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">维修操作内容（必填，≤200字）</text>
            <textarea
              class="cell-textarea"
              v-model="finance.repairContent"
              placeholder="如：更换台式机电源模块，长城600W"
              placeholder-class="input-placeholder"
              maxlength="200"
              auto-height
            />
          </view>

          <view class="receipt-section">
            <view class="receipt-title">收据图片（1-3张）</view>
            <view class="receipt-grid">
              <view v-for="(r, index) in receiptList" :key="index" class="receipt-item">
                <image class="receipt-img" :src="r.localPath || r.url" mode="aspectFill" @click="previewReceiptList(index)" />
                <view class="receipt-del" @click.stop="removeReceipt(index)">×</view>
              </view>
              <view v-if="receiptList.length < 3" class="receipt-add" @click="chooseReceipt">
                <text class="receipt-add-icon">+</text>
                <text class="receipt-add-text">上传图片</text>
              </view>
            </view>
          </view>

          <view class="complete-block-title">维修附加信息（可选）</view>
          <view class="form-row">
            <picker class="form-cell-picker" mode="selector" :range="repairResultOptions" @change="onMetaResultChange">
              <view class="form-cell">
                <text class="cell-label">维修结果</text>
                <view class="cell-value" :class="{ 'is-placeholder': !meta.repairResult }">
                  {{ meta.repairResult || '请选择' }}
                  <text class="cell-arrow">›</text>
                </view>
              </view>
            </picker>
            <picker class="form-cell-picker" mode="selector" :range="repairMethodOptions" @change="onMetaMethodChange">
              <view class="form-cell">
                <text class="cell-label">维修方式</text>
                <view class="cell-value" :class="{ 'is-placeholder': !meta.repairMethod }">
                  {{ meta.repairMethod || '请选择' }}
                  <text class="cell-arrow">›</text>
                </view>
              </view>
            </picker>
          </view>
          <view class="form-row">
            <view class="form-cell form-cell-half">
              <text class="cell-label">保修期</text>
              <input
                class="cell-input"
                v-model="meta.warrantyPeriod"
                placeholder="如：3个月"
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">维修时长（分钟）</text>
              <input
                class="cell-input"
                v-model="meta.repairDuration"
                type="number"
                placeholder="如：60"
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">额外备注</text>
            <textarea
              class="cell-textarea"
              v-model="meta.extraRemark"
              placeholder="选填"
              placeholder-class="input-placeholder"
              maxlength="200"
              auto-height
            />
          </view>
        </scroll-view>

        <view class="popup-actions">
          <wd-button plain round size="small" @click="showCompletePopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmComplete">
            确认完工
          </wd-button>
        </view>
      </view>
    </wd-popup>

    <!-- 修改对账弹窗 -->
    <wd-popup v-model="showFinancePopup" position="center" round custom-style="width: 90%;">
      <view class="popup-body complete-body">
        <view class="popup-title">修改对账信息</view>
        <scroll-view scroll-y class="complete-scroll">
          <view class="popup-tip">留空表示不修改该项；至少修改一项后提交。</view>
          <view class="form-row">
            <view class="form-cell form-cell-half">
              <text class="cell-label">数量（≥0）</text>
              <input
                class="cell-input"
                v-model="finance.quantity"
                type="number"
                placeholder="不修改请留空"
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">单价（元，≥0）</text>
              <input
                class="cell-input"
                v-model="finance.unitPrice"
                type="digit"
                placeholder="不修改请留空"
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">维修操作内容（≤200字）</text>
            <textarea
              class="cell-textarea"
              v-model="finance.repairContent"
              placeholder="不修改请留空"
              placeholder-class="input-placeholder"
              maxlength="200"
              auto-height
            />
          </view>
          <view class="form-row">
            <picker class="form-cell-picker" mode="selector" :range="repairResultOptions" @change="onMetaResultChange">
              <view class="form-cell">
                <text class="cell-label">维修结果</text>
                <view class="cell-value" :class="{ 'is-placeholder': !meta.repairResult }">
                  {{ meta.repairResult || '不修改/清空' }}
                  <text class="cell-arrow">›</text>
                </view>
              </view>
            </picker>
            <picker class="form-cell-picker" mode="selector" :range="repairMethodOptions" @change="onMetaMethodChange">
              <view class="form-cell">
                <text class="cell-label">维修方式</text>
                <view class="cell-value" :class="{ 'is-placeholder': !meta.repairMethod }">
                  {{ meta.repairMethod || '不修改/清空' }}
                  <text class="cell-arrow">›</text>
                </view>
              </view>
            </picker>
          </view>
          <view class="form-row">
            <view class="form-cell form-cell-half">
              <text class="cell-label">保修期</text>
              <input
                class="cell-input"
                v-model="meta.warrantyPeriod"
                placeholder="不修改请留空"
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">维修时长（分钟）</text>
              <input
                class="cell-input"
                v-model="meta.repairDuration"
                type="number"
                placeholder="不修改请留空"
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">额外备注</text>
            <textarea
              class="cell-textarea"
              v-model="meta.extraRemark"
              placeholder="不修改请留空"
              placeholder-class="input-placeholder"
              maxlength="200"
              auto-height
            />
          </view>
        </scroll-view>

        <view class="popup-actions">
          <wd-button plain round size="small" @click="showFinancePopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmFinance">
            确认修改
          </wd-button>
        </view>
      </view>
    </wd-popup>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http, uploadReceipt } from '@/utils/request'
import { isStoreStaff } from '@/utils/auth'
import {
  formatAmount,
  formatDateTime,
  statusTagType,
  timeAxisActionLabel
} from '@/utils/format'
import type { AdminOrderDetail, AvailableAction, OrderImage } from '@/types'

interface ReceiptItem {
  url: string
  localPath?: string
}

export default defineComponent({
  setup() {
    return {
      formatAmount,
      formatDateTime,
      statusTagType,
      timeAxisActionLabel
    }
  },
  data() {
    return {
      orderId: '',
      detail: null as AdminOrderDetail | null,
      submitting: false,
      // 弹窗显隐
      showRejectPopup: false,
      rejectReason: '',
      showCompletePopup: false,
      showFinancePopup: false,
      completeRemark: '',
      receiptList: [] as ReceiptItem[],
      // 完工/对账：数量/单价/维修内容
      finance: {
        quantity: '',
        unitPrice: '',
        repairContent: ''
      },
      // 完工/对账：维修附加信息
      meta: {
        repairResult: '',
        repairMethod: '',
        warrantyPeriod: '',
        repairDuration: '',
        extraRemark: ''
      },
      repairResultOptions: ['完全修复', '部分修复', '无法修复'],
      repairMethodOptions: ['上门维修', '返店维修', '远程协助']
    }
  },
  computed: {
    /** 店方专属动作：接单/完工/重新打开/修改对账（单位审核员不可见） */
    filteredActions(): AvailableAction[] {
      const actions = this.detail?.available_actions || []
      if (isStoreStaff()) return actions
      // 单位审核员（role=0）：仅保留 audit/reject
      return actions.filter((a) => a.action === 'audit' || a.action === 'reject')
    },
    reporterName(): string {
      const d = this.detail as AdminOrderDetail
      return d?.reporter?.nickname || '--'
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
  },
  onShow() {
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
    previewImages(imgs: OrderImage[], index: number) {
      uni.previewImage({ current: imgs[index].url, urls: imgs.map((i) => i.url) })
    },
    previewReceiptList(index: number) {
      uni.previewImage({ current: this.receiptList[index].url, urls: this.receiptList.map((r) => r.url) })
    },
    actionBtnType(action: string): 'primary' | 'warning' | 'danger' | 'success' {
      switch (action) {
        case 'complete':
          return 'success'
        case 'reject':
          return 'danger'
        case 'accept':
          return 'warning'
        case 'audit':
          return 'primary'
        case 'reopen':
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
        this.openComplete()
        return
      }
      if (act.action === 'update_finance') {
        this.openFinance()
        return
      }
      // audit / accept / reopen：确认 Modal（可带备注）
      this.promptSimpleAction(act)
    },
    /** 通用确认：audit/accept/reopen（备注可选，不同动作长度上限不同） */
    promptSimpleAction(act: AvailableAction) {
      const maxLen = act.action === 'reopen' ? 200 : 100
      const ph = act.action === 'reopen'
        ? `备注（可选，≤${maxLen}字）`
        : `备注（可选，≤${maxLen}字），可留空`
      const that = this
      uni.showModal({
        title: act.label,
        content: act.confirm_message || `确认执行「${act.label}」？`,
        editable: true,
        placeholderText: ph,
        success: async (res) => {
          if (!res.confirm) return
          const remark = ((res.content || '') as string).trim()
          if (remark.length > maxLen) {
            uni.showToast({ title: `备注不能超过${maxLen}字`, icon: 'none' })
            return
          }
          const body = remark ? { remark } : {}
          const urls: Record<string, string> = {
            audit: `/admin/orders/${that.orderId}/audit`,
            accept: `/admin/orders/${that.orderId}/accept`,
            reopen: `/admin/orders/${that.orderId}/reopen`
          }
          const url = urls[act.action]
          if (!url) return
          try {
            await http.post(url, body)
            uni.showToast({ title: '操作成功', icon: 'success' })
            that.loadDetail()
          } catch (e) {
            console.error('操作失败', e)
          }
        }
      })
    },
    async confirmReject() {
      const reason = this.rejectReason.trim()
      const action = this.filteredActions.find((a) => a.action === 'reject')
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
    /* ================== 完工 ================== */
    openComplete() {
      this.showCompletePopup = true
      this.completeRemark = ''
      this.receiptList = (this.detail?.receipts || []).map((r) => ({ url: r.url }))
      this.finance = { quantity: '', unitPrice: '', repairContent: '' }
      this.resetMeta()
    },
    resetMeta() {
      this.meta = {
        repairResult: '',
        repairMethod: '',
        warrantyPeriod: '',
        repairDuration: '',
        extraRemark: ''
      }
    },
    chooseReceipt() {
      const remain = 3 - this.receiptList.length
      if (remain <= 0) {
        uni.showToast({ title: '最多3张收据', icon: 'none' })
        return
      }
      const that = this
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
              const data = await uploadReceipt(that.orderId, file.path)
              that.receiptList.push({ url: data.url, localPath: file.path })
              uploaded++
            } catch (e) {
              console.error('收据上传失败', e)
            }
          }
          uni.hideLoading()
          if (uploaded > 0) {
            uni.showToast({ title: `已上传${uploaded}张`, icon: 'success' })
            // completed 状态下补传收据即直接生效，刷新详情
            if (that.detail && that.detail.status === 'completed') {
              that.showCompletePopup = false
              that.loadDetail()
            }
          }
        },
        fail: () => {}
      })
    },
    removeReceipt(index: number) {
      this.receiptList.splice(index, 1)
    },
    onMetaResultChange(e: any) {
      this.meta.repairResult = this.repairResultOptions[Number(e.detail.value)]
    },
    onMetaMethodChange(e: any) {
      this.meta.repairMethod = this.repairMethodOptions[Number(e.detail.value)]
    },
    buildMetadata(): Record<string, unknown> {
      const m: Record<string, unknown> = {}
      if (this.meta.repairResult) m.repair_result = this.meta.repairResult
      if (this.meta.repairMethod) m.repair_method = this.meta.repairMethod
      const warranty = this.meta.warrantyPeriod.trim()
      if (warranty) m.warranty_period = warranty
      if (this.meta.repairDuration !== '') {
        const duration = Number(this.meta.repairDuration)
        if (!Number.isNaN(duration) && duration >= 0) m.repair_duration = duration
      }
      const extra = this.meta.extraRemark.trim()
      if (extra) m.extra_remark = extra
      return m
    },
    async confirmComplete() {
      const remark = this.completeRemark.trim()
      if (!remark) {
        uni.showToast({ title: '请填写维修备注', icon: 'none' })
        return
      }
      const qty = Number(this.finance.quantity)
      const price = Number(this.finance.unitPrice)
      if (this.finance.quantity === '' || Number.isNaN(qty) || qty < 0) {
        uni.showToast({ title: '请填写正确的数量（≥0）', icon: 'none' })
        return
      }
      if (this.finance.unitPrice === '' || Number.isNaN(price) || price < 0) {
        uni.showToast({ title: '请填写正确的单价（≥0）', icon: 'none' })
        return
      }
      const repairContent = this.finance.repairContent.trim()
      if (!repairContent) {
        uni.showToast({ title: '请填写维修操作内容', icon: 'none' })
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
          receipts: this.receiptList.map((r) => r.url),
          quantity: qty,
          unit_price: price,
          repair_content: repairContent,
          metadata: this.buildMetadata()
        })
        uni.showToast({ title: '完工成功', icon: 'success' })
        this.showCompletePopup = false
        this.loadDetail()
      } catch (e) {
        console.error('完工失败', e)
      } finally {
        this.submitting = false
      }
    },
    /* ================== 修改对账 ================== */
    openFinance() {
      const d = this.detail
      if (!d) return
      this.showFinancePopup = true
      this.finance = {
        quantity: d.quantity !== undefined && d.quantity !== null ? String(d.quantity) : '',
        unitPrice: d.unit_price !== undefined && d.unit_price !== null ? String(d.unit_price) : '',
        repairContent: d.repair_content || ''
      }
      const md = d.metadata || {}
      this.meta = {
        repairResult: md.repair_result || '',
        repairMethod: md.repair_method || '',
        warrantyPeriod: md.warranty_period || '',
        repairDuration: md.repair_duration !== undefined && md.repair_duration !== null ? String(md.repair_duration) : '',
        extraRemark: md.extra_remark || ''
      }
    },
    hasFinanceChanges(): boolean {
      const d = this.detail
      if (!d) return false
      const qtyChanged =
        this.finance.quantity !== '' &&
        Number(this.finance.quantity) !== Number(d.quantity)
      const priceChanged =
        this.finance.unitPrice !== '' &&
        Number(this.finance.unitPrice) !== Number(d.unit_price)
      const contentChanged =
        this.finance.repairContent.trim() !== '' && this.finance.repairContent.trim() !== (d.repair_content || '')
      const md = d.metadata || {}
      const metaChanged =
        (this.meta.repairResult && this.meta.repairResult !== (md.repair_result || '')) ||
        (this.meta.repairMethod && this.meta.repairMethod !== (md.repair_method || '')) ||
        (this.meta.warrantyPeriod.trim() && this.meta.warrantyPeriod.trim() !== (md.warranty_period || '')) ||
        (this.meta.repairDuration !== '' &&
          Number(this.meta.repairDuration) !== Number(md.repair_duration)) ||
        (this.meta.extraRemark.trim() && this.meta.extraRemark.trim() !== (md.extra_remark || ''))
      return qtyChanged || priceChanged || contentChanged || metaChanged
    },
    async confirmFinance() {
      if (!this.hasFinanceChanges()) {
        uni.showToast({ title: '请至少修改一项内容', icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      try {
        const payload: Record<string, unknown> = {}
        if (this.finance.quantity !== '') payload.quantity = Number(this.finance.quantity)
        if (this.finance.unitPrice !== '') payload.unit_price = Number(this.finance.unitPrice)
        if (this.finance.repairContent.trim()) payload.repair_content = this.finance.repairContent.trim()
        payload.metadata = this.buildMetadata()
        await http.post(`/admin/orders/${this.orderId}/finance`, payload)
        uni.showToast({ title: '对账信息已更新', icon: 'success' })
        this.showFinancePopup = false
        this.loadDetail()
      } catch (e) {
        console.error('修改对账失败', e)
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

    &.desc-text {
      text-align: left;
      line-height: 1.6;
    }

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

.popup-tip {
  font-size: 24rpx;
  color: #999999;
  margin-bottom: 16rpx;
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

.complete-body {
  padding-bottom: 24rpx;
}

.complete-scroll {
  max-height: 58vh;
}

.complete-block-title {
  font-size: 26rpx;
  font-weight: 600;
  color: #333333;
  margin: 24rpx 0 12rpx;
  padding-left: 14rpx;
  border-left: 6rpx solid #4d80f0;
  line-height: 1.4;
}

.form-row {
  display: flex;
  margin-bottom: 4rpx;

  .form-cell-half,
  .form-cell-picker {
    flex: 1;
    min-width: 0;
    margin-right: 16rpx;

    &:last-child {
      margin-right: 0;
    }
  }

  .form-cell-picker {
    .form-cell {
      width: 100%;
      box-sizing: border-box;
    }
  }
}

.form-cell {
  background: #f5f6f8;
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 16rpx;

  .cell-label {
    display: block;
    font-size: 24rpx;
    color: #999999;
    margin-bottom: 12rpx;
  }

  .cell-input {
    width: 100%;
    font-size: 28rpx;
    color: #333333;
  }

  .cell-value {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 28rpx;
    color: #333333;

    &.is-placeholder {
      color: #bbbbbb;
    }

    .cell-arrow {
      color: #bbbbbb;
      font-size: 28rpx;
    }
  }
}

.form-cell-column {
  .cell-textarea {
    width: 100%;
    min-height: 100rpx;
    font-size: 28rpx;
    color: #333333;
    line-height: 1.5;
  }
}

.input-placeholder {
  color: #bbbbbb;
}
</style>
