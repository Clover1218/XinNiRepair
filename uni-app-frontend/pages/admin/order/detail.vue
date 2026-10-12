<template>
  <view class="page" v-if="detail">
    <!-- 被退回提示（Web 同样要求 status==='rejected' 才显示，避免重新提交后残留旧原因） -->
    <view v-if="detail.status === 'rejected' && detail.reject_reason" class="reject-bar">
      <text class="reject-title">退回原因：</text>
      <text class="reject-text">{{ detail.reject_reason }}</text>
    </view>

    <!-- ① 报修信息：地点大字 + 报修内容 + 报修人（含拨号）+ 状态胶囊在标题右 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <wd-icon name="location" size="30rpx" color="#6b7280"></wd-icon>
        <text class="xn-card-title-text">报修信息</text>
        <text class="status-pill" :class="pill.cls">{{ pill.text }}</text>
      </view>
      <view class="onsite-place">
        {{ detail.enterprise_name || '未指定单位' }}：{{ detail.room || '未填写地点' }}
      </view>
      <view v-if="detail.category_name || detail.property_name" class="onsite-tags">
        报修内容：{{ detail.category_name || '—' }} → {{ detail.property_name || '—' }}
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
        <view v-if="contactPhone" class="call-btn" hover-class="call-btn--hover" @click="callPhone">
          <wd-icon name="phone" size="36rpx" color="#6b7280"></wd-icon>
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
        {{ detail.description || '（无故障描述）' }}
      </view>
      <view v-if="isLongDesc" class="desc-toggle" @click="descExpanded = !descExpanded">
        {{ descExpanded ? '收起' : '查看全文' }}
      </view>
      <view v-if="detail.images && detail.images.length" class="img-strip">
        <image
          v-for="(img, index) in detail.images"
          :key="img.id"
          class="img-cell"
          :src="img.url"
          mode="aspectFill"
          @click="previewImages(detail.images, index)"
        ></image>
      </view>
    </view>

    <!-- ③ 对账信息：仅店方可见 + 有对账数据 + 非早期状态（金额敏感，审核员隐藏） -->
    <view v-if="showFinance" class="xn-card finance-card">
      <view class="xn-card-title">
        <text class="xn-card-title-text">对账信息</text>
        <text v-if="financeEditVisible" class="edit-link" @click="openFinanceDialog">编辑</text>
      </view>

      <!-- 金额 hero + 右侧详情箭头 -->
      <view class="fin-amount">
        <text class="fa-value">¥{{ financeAmountText }}</text>
        <view
          class="fin-detail-toggle"
          :class="{ expanded: financeDetailExpanded }"
          hover-class="fin-detail-toggle--hover"
          @click="financeDetailExpanded = !financeDetailExpanded"
        >
          <wd-icon name="arrow-right" size="28rpx"></wd-icon>
        </view>
      </view>

      <!-- 展开明细：横向平铺键值对 -->
      <view v-if="financeDetailExpanded" class="fin-detail">
        <view class="fd-item">
          <text class="fd-k">数量</text>
          <text class="fd-v">{{ detail.quantity ?? '-' }}</text>
        </view>
        <view class="fd-item">
          <text class="fd-k">单价</text>
          <text class="fd-v">¥{{ formatAmount(detail.unit_price) }}</text>
        </view>
        <view v-if="detail.metadata && detail.metadata.repair_method" class="fd-item">
          <text class="fd-k">维修方式</text>
          <text class="fd-v">{{ detail.metadata.repair_method }}</text>
        </view>
      </view>

      <view class="fin-divider"></view>
      <view v-if="detail.repair_content" class="fin-section">
        <view class="fin-subtitle">维修操作内容</view>
        <view class="fin-text">{{ detail.repair_content }}</view>
      </view>
      <view v-if="detail.remark" class="fin-section">
        <view class="fin-subtitle">维修备注</view>
        <view class="fin-text">{{ detail.remark }}</view>
      </view>
      <template v-if="detail.metadata && (detail.metadata.repair_result || detail.metadata.warranty_period || detail.metadata.repair_duration)">
        <view class="fin-section">
          <view class="fin-subtitle">维修附加信息</view>
          <view class="fin-text">
            <text v-if="detail.metadata.repair_result">结果：{{ detail.metadata.repair_result }}</text>
            <text v-if="detail.metadata.repair_result && detail.metadata.warranty_period"> · </text>
            <text v-if="detail.metadata.warranty_period">保修期：{{ detail.metadata.warranty_period }}</text>
            <text v-if="(detail.metadata.repair_result || detail.metadata.warranty_period) && detail.metadata.repair_duration != null"> · </text>
            <text v-if="detail.metadata.repair_duration != null">时长：{{ detail.metadata.repair_duration }} 分钟</text>
          </view>
        </view>
      </template>

      <view class="fin-divider"></view>
      <view class="fin-section">
        <view class="fin-subtitle">收据</view>
        <view v-if="detail.receipts && detail.receipts.length" class="img-strip img-strip--receipt">
          <image
            v-for="(img, index) in detail.receipts"
            :key="img.id"
            class="img-cell"
            :src="img.url"
            mode="aspectFill"
            @click="previewImages(detail.receipts, index)"
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
            <text>{{ detail.order_no }}</text>
            <wd-icon name="copy" size="26rpx" color="#9ca3af" @click="copyOrderNo"></wd-icon>
          </view>
        </view>
        <view class="kv">
          <text class="k">提交时间</text>
          <text class="v">{{ formatDateTime(detail.submitted_at || detail.created_at) }}</text>
        </view>
        <view class="kv">
          <text class="k">紧急程度</text>
          <view class="v">
            <text class="urgency-tag" :class="urgencyTagClass">{{ detail.urgency_label || '普通' }}</text>
          </view>
        </view>
        <view v-if="detail.auditor_name" class="kv">
          <text class="k">审核人</text>
          <text class="v v-strong">{{ detail.auditor_name }}</text>
        </view>
        <view v-if="detail.audited_at" class="kv">
          <text class="k">审核时间</text>
          <text class="v">{{ formatShortDateTime(detail.audited_at) }}</text>
        </view>
        <view v-if="detail.repairer_name" class="kv">
          <text class="k">维修人员</text>
          <text class="v v-strong">{{ detail.repairer_name }}</text>
        </view>
        <view v-if="detail.accepted_at" class="kv">
          <text class="k">接单时间</text>
          <text class="v">{{ formatShortDateTime(detail.accepted_at) }}</text>
        </view>
        <view v-if="detail.completed_at" class="kv">
          <text class="k">完工时间</text>
          <text class="v">{{ formatShortDateTime(detail.completed_at) }}</text>
        </view>
      </view>
    </view>

    <!-- ⑤ 处理记录：默认折叠，竖线 + 五色节点 + 状态流转胶囊 -->
    <view class="xn-card">
      <view class="xn-card-title">
        <text class="xn-card-title-text">处理记录</text>
        <text class="title-count">（{{ detail.timeline.length }}）</text>
      </view>
      <view v-if="!timelineExpanded" class="timeline-collapsed">
        <text class="timeline-expand-btn" @click="timelineExpanded = true">查看全部处理轨迹</text>
      </view>
      <view v-else class="xn-timeline">
        <view v-for="(t, index) in detail.timeline" :key="t.id" class="tl-item" :class="tlNodeClass(t.action)">
          <view v-if="index < detail.timeline.length - 1" class="tl-tail"></view>
          <view class="tl-node"></view>
          <view class="tl-body">
            <view class="tl-row1">
              <text class="tl-action">{{ t.action_label || timeAxisActionLabel(t.action) }}</text>
              <text v-if="t.operator_name" class="tl-operator">{{ t.operator_name }}</text>
            </view>
            <view class="tl-row2">{{ formatShortDateTime(t.created_at) }}</view>
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

    <!-- ⑥ 操作区：保留固定底栏（移动端更好用），但内部按 Web 三档排序 -->
    <view v-if="actionButtons.length > 0" class="footer">
      <view class="footer-btn footer-btn--primary">
        <wd-button type="primary" round block size="large" @click="onAction(actionButtons[0])">
          {{ actionButtons[0].label }}
        </wd-button>
      </view>
      <view v-for="act in actionButtons.slice(1)" :key="act.action" class="footer-btn">
        <wd-button
          :type="actionBtnType(act.action)"
          round
          block
          size="large"
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
        <view class="popup-label">
          退回原因<text class="popup-label-req">（必填，2-200字）</text>
        </view>
        <wd-textarea
          v-model="rejectReason"
          placeholder="请填写退回原因"
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

    <!-- 完工弹窗（对齐 Web 最新版：结单结果 + 客户签字；仅“完成”展示对账信息块） -->
    <wd-popup v-model="showCompletePopup" position="center" round custom-style="width: 90%;">
      <view class="popup-body complete-body">
        <view class="popup-title">完工</view>
        <scroll-view scroll-y class="complete-scroll">
          <!-- 结单结果 -->
          <view class="complete-block-title">结单结果</view>
          <view class="result-group">
            <view
              v-for="opt in finishResultOptions"
              :key="opt.value"
              class="result-chip"
              :class="{ active: finishResult === opt.value }"
              @click="finishResult = opt.value"
            >{{ opt.label }}</view>
          </view>

          <!-- 仅“完成”展示对账信息 + 客户签字 -->
          <template v-if="finishResult === 'complete'">
            <view class="complete-block-title">客户签字</view>
            <view class="result-group">
              <view
                class="result-chip"
                :class="{ active: signMode === 'signed' }"
                @click="signMode = 'signed'"
              >客户已签字</view>
              <view
                class="result-chip"
                :class="{ active: signMode === 'pending_sign' }"
                @click="signMode = 'pending_sign'"
              >暂时无法签字，稍后补签</view>
            </view>
            <view v-if="signMode === 'pending_sign'" class="complete-tip">
              该单会暂留「待补签」状态，金额等可现在填也可后续补登，不影响客户，也不计入已完工。
            </view>

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
              <text class="cell-label">维修操作内容（{{ signMode === 'signed' ? '必填' : '选填' }}，≤200字）</text>
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
              <view class="receipt-title">收据图片（{{ signMode === 'signed' ? '必传，1-3张' : '选填，≤3张' }}）</view>
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
              <text class="cell-label">额外备注（选填）</text>
              <textarea
                class="cell-textarea"
                v-model="meta.extraRemark"
                placeholder=""
                placeholder-class="input-placeholder"
                maxlength="200"
                auto-height
              />
            </view>
          </template>

          <!-- 维修备注：移动到对账信息/收据之后；取消/不能修类无对账块时也展示 -->
          <view class="complete-block-title">{{ completeRemarkTitle }}</view>
          <wd-textarea
            v-model="completeRemark"
            :placeholder="completeRemarkPlaceholder"
            :maxlength="200"
            show-word-limit
            auto-height
            custom-style="min-height: 120rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
          />
        </scroll-view>

        <view class="popup-actions">
          <wd-button plain round size="small" @click="showCompletePopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmComplete">
            {{ completeButtonLabel }}
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
                placeholder=""
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">单价（元，≥0）</text>
              <input
                class="cell-input"
                v-model="finance.unitPrice"
                type="digit"
                placeholder=""
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">维修操作内容（≤200字）</text>
            <textarea
              class="cell-textarea"
              v-model="finance.repairContent"
              placeholder=""
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
                placeholder=""
                placeholder-class="input-placeholder"
              />
            </view>
            <view class="form-cell form-cell-half">
              <text class="cell-label">维修时长（分钟）</text>
              <input
                class="cell-input"
                v-model="meta.repairDuration"
                type="number"
                placeholder=""
                placeholder-class="input-placeholder"
              />
            </view>
          </view>
          <view class="form-cell form-cell-column">
            <text class="cell-label">额外备注</text>
            <textarea
              class="cell-textarea"
              v-model="meta.extraRemark"
              placeholder=""
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

    <!-- 通用确认弹窗：审核通过 / 接单维修 / 重新打开 -->
    <wd-popup v-model="showSimplePopup" position="center" round custom-style="width: 86%;">
      <view class="popup-body">
        <view class="popup-title">{{ simpleAction?.label }}</view>
        <view class="popup-tip">{{ simpleAction?.confirm_message || '请确认执行该操作。' }}</view>
        <view class="popup-label">备注（选填，≤{{ simpleMaxLen }}字）</view>
        <wd-textarea
          v-model="simpleRemark"
          placeholder="请输入备注（可选）"
          :maxlength="simpleMaxLen"
          show-word-limit
          auto-height
          custom-style="min-height: 140rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
        />
        <view class="popup-actions">
          <wd-button plain round size="small" @click="showSimplePopup = false">取消</wd-button>
          <wd-button :type="simpleBtnType" round size="small" :loading="submitting" @click="confirmSimpleAction">
            确认{{ simpleAction?.label }}
          </wd-button>
        </view>
      </view>
    </wd-popup>

    <!-- 补签弹窗（pending_sign 工单正式完工） -->
    <wd-popup v-model="showSignPopup" position="center" round custom-style="width: 90%;">
      <view class="popup-body">
        <view class="popup-title">补签正式完工</view>
        <view class="popup-tip">该工单已完工，请补充收据并确认签字完成。</view>
        <view class="receipt-section">
          <view class="receipt-title">收据凭证（必传，≤3张）</view>
          <view class="receipt-grid">
            <view v-for="(r, index) in receiptList" :key="index" class="receipt-item">
              <image class="receipt-img" :src="r.localPath || r.url" mode="aspectFill" @click="previewReceiptList(index)"></image>
              <view class="receipt-del" @click="removeReceipt(index)">×</view>
            </view>
            <view v-if="receiptList.length < 3" class="receipt-add" @click="chooseReceipt">
              <text class="receipt-add-icon">+</text>
              <text class="receipt-add-text">上传</text>
            </view>
          </view>
        </view>
        <view class="form-cell form-cell-column">
          <text class="cell-label">备注（选填，≤200字）</text>
          <wd-textarea
            v-model="signRemark"
            placeholder="请输入备注（选填）"
            :maxlength="200"
            auto-height
            custom-style="min-height: 100rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
          />
        </view>
        <view class="popup-actions">
          <wd-button plain round size="small" @click="showSignPopup = false">取消</wd-button>
          <wd-button type="primary" round size="small" :loading="submitting" @click="confirmSign">
            确认补签
          </wd-button>
        </view>
      </view>
    </wd-popup>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http, uploadReceipt } from '@/utils/request'
import { adminAPI } from '@/api'
import { isStoreStaff } from '@/utils/auth'
import {
  formatAmount,
  formatDateTime,
  statusTagType,
  statusColor,
  statusLabel,
  statusPill,
  timeAxisActionLabel
} from '@/utils/format'
import type { AdminOrderDetail, AvailableAction, OrderImage, SignOrderInput } from '@/types'

interface ReceiptItem {
  id?: string
  url: string
  localPath?: string
  name?: string
  size?: number
  uploaded?: boolean
}

export default defineComponent({
  setup() {
    return {
      formatAmount,
      formatDateTime,
      statusTagType,
      statusColor,
      statusLabel,
      statusPill,
      timeAxisActionLabel
    }
  },
  data() {
    return {
      orderId: '',
      detail: null as AdminOrderDetail | null,
      submitting: false,
      // 折叠态
      descExpanded: false,
      financeDetailExpanded: false,
      timelineExpanded: false,
      // 弹窗显隐
      showRejectPopup: false,
      rejectReason: '',
      showCompletePopup: false,
      showFinancePopup: false,
      // 通用确认弹窗（审核通过 / 接单维修 / 重新打开）
      showSimplePopup: false,
      // 补签弹窗（pending_sign 工单正式完工）
      showSignPopup: false,
      signRemark: '',
      simpleAction: null as AvailableAction | null,
      simpleRemark: '',
      simpleMaxLen: 100,
      simpleBtnType: 'primary' as 'primary' | 'warning' | 'danger' | 'success',
      completeRemark: '',
      receiptList: [] as ReceiptItem[],
      // 完工：结单结果（完成 / 客户原因取消 / 不能修）
      finishResult: 'complete' as 'complete' | 'customer_cancelled' | 'unrepairable',
      finishResultOptions: [
        { value: 'complete', label: '完成' },
        { value: 'customer_cancelled', label: '客户原因取消' },
        { value: 'unrepairable', label: '不能修' }
      ],
      // 客户签字模式（仅 finishResult==='complete' 生效）
      signMode: 'signed' as 'signed' | 'pending_sign',
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
    /** 状态胶囊（与 Web 端 statusView 逻辑一致：待补签优先） */
    pill(): { text: string; cls: string } {
      return statusPill(this.detail || undefined)
    },
    /** 报修人头像 */
    reporterAvatar(): string {
      return this.detail?.reporter?.avatar_url || ''
    },
    reporterInitial(): string {
      const r = this.detail?.reporter
      return r && r.nickname ? r.nickname.charAt(0) : '?'
    },
    /** 报修人显示名 */
    reporterName(): string {
      return this.detail?.reporter?.nickname || '未知用户'
    },
    /** 从 contact("王五 12345678910") 中解析手机号用于拨号 */
    contactPhone(): string {
      const c = this.detail?.contact || ''
      const m = c.match(/1\d{10}/)
      return m ? m[0] : c
    },
    /** 故障描述是否超过 60 字需要折叠 */
    isLongDesc(): boolean {
      return (this.detail?.description || '').length > 60
    },
    /** 对账卡可见：店方 + 有对账数据 + 非早期状态（金额敏感，审核员隐藏） */
    showFinance(): boolean {
      if (!isStoreStaff()) return false
      const d = this.detail
      if (!d) return false
      const early = ['draft', 'reported', 'rejected', 'cancelled']
      if (early.includes(d.status)) return false
      const hasFinance =
        d.amount != null || d.quantity != null || d.unit_price != null || !!d.repair_content
      return !!hasFinance
    },
    /** 对账编辑入口：仅店方在已完成状态可见 */
    financeEditVisible(): boolean {
      return isStoreStaff() && this.detail?.status === 'completed'
    },
    financeAmountText(): string {
      const a = this.detail?.amount
      return a != null ? formatAmount(a) : '--'
    },
    /** 紧急程度配色 class */
    urgencyTagClass(): string {
      const u = this.detail?.urgency
      if (u === 'very_urgent') return 'urg-very'
      if (u === 'urgent') return 'urg-high'
      return 'urg-normal'
    },
    /** 底部操作按钮（单位审核员仅 audit/reject） */
    actionButtons(): AvailableAction[] {
      const actions = this.detail?.available_actions || []
      if (isStoreStaff()) return actions
      return actions.filter((a) => a.action === 'audit' || a.action === 'reject')
    },
    /** 维修备注是否必填：仅“完成”类选填，客户取消/不能修类必填 */
    completeRemarkRequired(): boolean {
      return this.finishResult !== 'complete'
    },
    completeRemarkTitle(): string {
      return this.completeRemarkRequired ? '维修备注（必填，≤200 字）' : '维修备注（选填，≤200 字）'
    },
    completeRemarkPlaceholder(): string {
      if (this.finishResult === 'customer_cancelled') {
        return '必填：请说明客户取消的原因，如：客户已自行解决、改由厂家保修、重复报修等'
      }
      if (this.finishResult === 'unrepairable') {
        return '必填：请说明无法维修的原因，如：配件已停产、设备损坏无维修价值等'
      }
      return '选填：可简述维修情况，如：已更换电源模块并测试正常'
    },
    /** 确认按钮文案随结单结果/签字模式变化 */
    completeButtonLabel(): string {
      if (this.finishResult === 'customer_cancelled') return '确认客户取消'
      if (this.finishResult === 'unrepairable') return '确认不能修'
      return this.signMode === 'pending_sign' ? '暂留处理中（稍后补签）' : '确认完工'
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
    /** 拨打电话（从 contact 解析出的手机号） */
    callPhone() {
      const phone = this.contactPhone
      if (!phone) {
        uni.showToast({ title: '未留联系方式', icon: 'none' })
        return
      }
      uni.makePhoneCall({ phoneNumber: phone })
    },
    /** 复制工单号 */
    copyOrderNo() {
      const no = this.detail?.order_no
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
    /** 短日期时间（YYYY-MM-DD HH:mm）用于时间轴/归档 */
    formatShortDateTime(s?: string): string {
      if (!s) return ''
      const d = new Date(s)
      if (Number.isNaN(d.getTime())) return s
      const pad = (n: number) => (n < 10 ? '0' + n : '' + n)
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
    },
    /** 对账编辑入口（复用现有弹窗） */
    openFinanceDialog() {
      this.openFinance()
    },
    /* ================== 补签正式完工 ================== */
    openSign() {
      this.showSignPopup = true
      this.signRemark = ''
      this.receiptList = (this.detail?.receipts || []).map((r) => ({ id: r.id, url: r.url }))
    },
    async confirmSign() {
      if (this.receiptList.length === 0) {
        uni.showToast({ title: '请上传收据图片', icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      try {
        const ids = await this.collectReceiptIds()
        const d = this.detail
        const payload: Record<string, unknown> = {
          receipt_ids: ids,
          remark: this.signRemark.trim() || undefined
        }
        // 沿用已存在的对账数据（补签不改金额，仅确认签字）
        if (d) {
          if (d.quantity != null) payload.quantity = d.quantity
          if (d.unit_price != null) payload.unit_price = d.unit_price
          if (d.repair_content) payload.repair_content = d.repair_content
          if (d.metadata) payload.metadata = d.metadata
        }
        await adminAPI.signOrder(this.orderId, payload as unknown as SignOrderInput)
        uni.showToast({ title: '补签成功', icon: 'success' })
        this.showSignPopup = false
        this.loadDetail()
      } catch (e: any) {
        console.error('补签失败', e)
        // 透传真实错误（后端 message / 上传失败原因等），不再用通用文案掩盖
        const raw = e?.message || e?.data?.message || e?.errMsg || ''
        const title = raw ? String(raw).slice(0, 40) : '补签失败，请重试'
        uni.showToast({ title, icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
    previewReceiptList(index: number) {
      const item = this.receiptList[index]
      uni.previewImage({
        current: item.url || item.localPath,
        urls: this.receiptList.map((r) => r.url || r.localPath)
      })
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
      if (act.action === 'sign') {
        this.openSign()
        return
      }
      // audit / accept / reopen：通用确认弹窗（可带备注）
      this.openSimplePopup(act)
    },
    /* ========= 通用确认弹窗：audit / accept / reopen ========= */
    /** 打开通用确认弹窗（备注可选，不同动作长度上限不同） */
    openSimplePopup(act: AvailableAction) {
      this.simpleAction = act
      this.simpleRemark = ''
      this.simpleMaxLen = act.action === 'reopen' ? 200 : 100
      this.simpleBtnType = this.actionBtnType(act.action)
      this.showSimplePopup = true
    },
    /** 提交通用确认弹窗（备注留空则不带 remark 字段） */
    async confirmSimpleAction() {
      const act = this.simpleAction
      if (!act || this.submitting) return
      const remark = this.simpleRemark.trim()
      if (remark.length > this.simpleMaxLen) {
        uni.showToast({ title: `备注不能超过${this.simpleMaxLen}字`, icon: 'none' })
        return
      }
      const urls: Record<string, string> = {
        audit: `/admin/orders/${this.orderId}/audit`,
        accept: `/admin/orders/${this.orderId}/accept`,
        reopen: `/admin/orders/${this.orderId}/reopen`
      }
      const url = urls[act.action]
      if (!url) {
        uni.showToast({ title: '暂不支持该操作', icon: 'none' })
        return
      }
      this.submitting = true
      try {
        const body = remark ? { remark } : {}
        await http.post(url, body)
        uni.showToast({ title: '操作成功', icon: 'success' })
        this.showSimplePopup = false
        this.loadDetail()
      } catch (e) {
        console.error('操作失败', e)
        uni.showToast({ title: '操作失败，请重试', icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
    async confirmReject() {
      const reason = this.rejectReason.trim()
      const action = this.actionButtons.find((a) => a.action === 'reject')
      const minLength = action?.reason_min_length || 2
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
        uni.showToast({ title: '退回失败，请重试', icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
    /* ================== 完工 ================== */
    openComplete() {
      this.showCompletePopup = true
      this.finishResult = 'complete'
      this.signMode = 'signed'
      this.completeRemark = ''
      this.receiptList = (this.detail?.receipts || []).map((r) => ({ id: r.id, url: r.url }))
      // 数量默认 1、单价默认 0（与 Web 一致；不强制用户改）
      this.finance = { quantity: '1', unitPrice: '0', repairContent: '' }
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
          // 选图即传：写入 temporary 记录并取得图片 id；完工/补签时回传 receipt_ids 由后端 diff 激活
          for (const file of validFiles) {
            try {
              const data = await uploadReceipt(that.orderId, file.path)
              that.receiptList.push({
                id: data.id,
                url: data.url,
                localPath: file.path,
                name: file.name,
                size: file.size,
                uploaded: true
              })
            } catch (err: any) {
              uni.showToast({ title: '收据上传失败，请重试', icon: 'none' })
              console.error('收据上传失败', err)
            }
          }
        },
        fail: () => {}
      })
    },
    /** 收集 receiptList 中的图片 id（选图时已即时上传，落 temporary；既有服务端收据也带 id）。
     *  仍有本地未传的先补传。返回 id 列表，供完工/补签时回传 receipt_ids 由后端 diff 激活。 */
    async collectReceiptIds(): Promise<string[]> {
      const ids: string[] = []
      for (const r of this.receiptList) {
        if (r.id) {
          ids.push(r.id)
          continue
        }
        if (r.localPath) {
          try {
            const data = await uploadReceipt(this.orderId, r.localPath)
            r.id = data.id
            r.url = data.url
            r.uploaded = true
            ids.push(data.id)
          } catch (err: any) {
            // 把真实错误透传出去，避免被 confirmSign 的通用 toast 掩盖
            throw new Error(`收据上传失败：${err?.message || err?.data?.message || err || '未知错误'}`)
          }
        }
      }
      return ids
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
      // 已完成单：本弹窗仅用于「补传收据」，不调用 complete 接口（后端仅允许 processing 状态完工）
      if (this.detail?.status === 'completed') {
        if (this.submitting) return
        const pendingCount = this.receiptList.filter((r) => !r.id).length
        if (pendingCount === 0) {
          uni.showToast({ title: '没有新的收据需要上传', icon: 'none' })
          return
        }
        this.submitting = true
        try {
          await this.collectReceiptIds()
          uni.showToast({ title: '收据已更新', icon: 'success' })
          this.showCompletePopup = false
          this.loadDetail()
        } catch (e) {
          console.error('收据上传失败', e)
          uni.showToast({ title: '收据上传失败，请重试', icon: 'none' })
        } finally {
          this.submitting = false
        }
        return
      }

      const remark = this.completeRemark.trim()
      if (remark.length > 200) {
        uni.showToast({ title: '维修备注不能超过200字', icon: 'none' })
        return
      }
      const isComplete = this.finishResult === 'complete'
      // 客户取消 / 不能修：终态，不要求对账/收据，但维修备注必填
      if (!isComplete && !remark) {
        const tip =
          this.finishResult === 'customer_cancelled'
            ? '请填写维修备注：说明客户取消的原因'
            : '请填写维修备注：说明无法维修的原因'
        uni.showToast({ title: tip, icon: 'none' })
        return
      }

      // 默认对账值（仅完成类使用）
      let quantity = 1
      let unitPrice = 0
      let repairContent = ''
      let receiptIds: string[] = []
      let signModePayload = 'signed'

      if (isComplete) {
        quantity = Number(this.finance.quantity)
        unitPrice = Number(this.finance.unitPrice)
        if (this.finance.quantity === '' || Number.isNaN(quantity) || quantity < 0) {
          uni.showToast({ title: '请填写正确的数量（≥0）', icon: 'none' })
          return
        }
        if (this.finance.unitPrice === '' || Number.isNaN(unitPrice) || unitPrice < 0) {
          uni.showToast({ title: '请填写正确的单价（≥0）', icon: 'none' })
          return
        }
        signModePayload = this.signMode
        repairContent = this.finance.repairContent.trim()
        if (this.signMode === 'signed') {
          // 已签字：维修内容 + 收据必填
          if (!repairContent) {
            uni.showToast({ title: '请填写维修操作内容', icon: 'none' })
            return
          }
          if (repairContent.length > 200) {
            uni.showToast({ title: '维修操作内容不能超过200字', icon: 'none' })
            return
          }
          if (this.receiptList.length === 0) {
            uni.showToast({ title: '请上传至少一张收据图片', icon: 'none' })
            return
          }
          receiptIds = await this.collectReceiptIds()
        } else {
          // 稍后补签：收据/维修内容选填，上传的收据一并提交，避免信息丢失
          receiptIds = await this.collectReceiptIds()
        }
      }

      if (this.submitting) return
      this.submitting = true
      try {
        await http.post(`/admin/orders/${this.orderId}/complete`, {
          remark,
          receipt_ids: receiptIds,
          quantity,
          unit_price: unitPrice,
          repair_content: repairContent,
          metadata: this.buildMetadata(),
          result: this.finishResult,
          sign_mode: signModePayload
        })
        const msg =
          this.finishResult !== 'complete'
            ? this.finishResult === 'unrepairable'
              ? '已标记为不能修'
              : '已标记为客户原因取消'
            : signModePayload === 'pending_sign'
              ? '已暂留处理中（待补签）'
              : '完工成功'
        uni.showToast({ title: msg, icon: 'success' })
        this.showCompletePopup = false
        this.loadDetail()
      } catch (e) {
        console.error('完工失败', e)
        uni.showToast({ title: '完工失败，请重试', icon: 'none' })
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
  padding: 20rpx 24rpx;
  box-sizing: border-box;
}

/* ===== 通用卡片外壳（对齐 Web 端 detail 卡片：1px 边框 + 24rpx 圆角 + 32rpx 内边距） ===== */
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

.edit-link {
  margin-left: auto;
  font-size: 26rpx;
  color: #0f766e;
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

/* ===== ① 报修信息：现场执行卡 ===== */
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

.call-btn {
  width: 64rpx;
  height: 64rpx;
  border-radius: 50%;
  background-color: #eef2f4;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  &--hover {
    background-color: #e2e8f0;
  }
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

/* ===== ③ 对账信息 ===== */
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

.fin-detail-toggle {
  padding: 8rpx;
  color: #9ca3af;

  &.expanded {
    transform: rotate(90deg);
  }
}

.fin-detail {
  display: flex;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.fd-item {
  width: 33.33%;
  box-sizing: border-box;
  padding: 8rpx 0;
}

.fd-k {
  display: block;
  font-size: 24rpx;
  color: #9ca3af;
}

.fd-v {
  display: block;
  font-size: 28rpx;
  color: #111827;
  margin-top: 4rpx;
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

.footer {
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

.popup-body {
  padding: 40rpx 32rpx 32rpx;

  .popup-title {
    font-size: 34rpx;
    font-weight: 600;
    color: #1a1a1a;
    text-align: center;
    margin-bottom: 28rpx;
  }

  .popup-label {
    font-size: 26rpx;
    color: #333333;
    margin-bottom: 12rpx;

    .popup-label-req {
      color: #fa5151;
    }
  }

  .popup-actions {
    display: flex;
    gap: 32rpx;
    margin-top: 32rpx;

    wd-button {
      flex: 1;
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
  border-left: 6rpx solid #0f766e;
  line-height: 1.4;
}

.result-group {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  margin-bottom: 8rpx;
}

.result-chip {
  padding: 14rpx 28rpx;
  border-radius: 12rpx;
  background: #f5f6f8;
  color: #374151;
  font-size: 26rpx;
  border: 1rpx solid transparent;

  &.active {
    background: rgba(15, 118, 110, 0.12);
    color: #0f766e;
    border-color: #0f766e;
    font-weight: 600;
  }
}

.complete-tip {
  font-size: 24rpx;
  color: #ea580c;
  background: rgba(234, 88, 12, 0.08);
  border-radius: 12rpx;
  padding: 16rpx 20rpx;
  line-height: 1.6;
  margin: 4rpx 0 8rpx;
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
