<template>
  <view v-if="filteredActions.length" class="action-bar" @click.stop>
    <wd-button
      v-for="act in filteredActions"
      :key="act.action"
      size="small"
      :type="btnType(act.action)"
      :custom-style="btnStyle"
      :loading="submitting && currentAction === act.action"
      @click="onAction(act)"
    >
      {{ act.label }}
    </wd-button>

    <!-- 退回弹窗 -->
    <wd-popup v-model="showReject" position="center" round custom-style="width: 86%;">
      <view class="popup-body">
        <view class="popup-title">退回工单</view>
        <view class="popup-label">
          退回原因<text class="popup-label-req">（必填，{{ rejectMin }}–200字）</text>
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
          <wd-button plain :custom-style="btnStyle" size="small" @click="showReject = false">取消</wd-button>
          <wd-button type="primary" :custom-style="btnStyle" size="small" :loading="submitting" @click="confirmReject">
            确认退回
          </wd-button>
        </view>
      </view>
    </wd-popup>

    <!-- 通用确认弹窗：审核通过 / 接单维修 / 重新打开（可填备注） -->
    <wd-popup v-model="showSimple" position="center" round custom-style="width: 86%;">
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
          custom-style="min-height: 120rpx; padding: 20rpx; background: #f5f6f8; border-radius: 12rpx;"
        />
        <view class="popup-actions">
          <wd-button plain :custom-style="btnStyle" size="small" @click="showSimple = false">取消</wd-button>
          <wd-button
            :type="simpleBtnType"
            :custom-style="btnStyle"
            size="small"
            :loading="submitting"
            @click="confirmSimpleAction"
          >
            确认{{ simpleAction?.label }}
          </wd-button>
        </view>
      </view>
    </wd-popup>
  </view>
</template>

<script lang="ts">
import { defineComponent, type PropType } from 'vue'
import { http } from '@/utils/request'
import { isStoreStaff } from '@/utils/auth'
import type { AvailableAction, AvailableActionKey } from '@/types'

/**
 * V1.3 工单操作按钮栏（审核工单列表 / 处理工单列表 共用）
 *
 * 与工单详情页（pages/admin/order/detail）的「底部操作」能力一致，但下沉为可复用组件：
 * - 按后端下发的 `available_actions`（按状态生成）渲染按钮；
 * - 按当前登录角色裁剪：店方(role>=1)/超管 显示全部，`isUnitReviewer` 仅保留 audit / reject；
 * - 按钮样式：圆角矩形 + 实心白字（背景=对应 type 色），与卡片「当前状态」胶囊明确区分，
 *   与「我的工单」卡片操作按钮风格统一；圆角经 custom-style 内联下发（压过组件库双类选择器）。
 * - 简单动作（audit / accept / reopen）就地弹通用确认框；reject 就地弹退回原因框；
 *   complete（完工登记）/ update_finance（修改对账）需完整详情表单，点击跳转详情页执行。
 * - 外层 view 统一 @click.stop，避免触发卡片点击（进详情）。
 * - 任何操作成功后 emit('done')，由父列表刷新。
 */
export default defineComponent({
  name: 'OrderActionBar',
  props: {
    orderId: { type: String, required: true },
    actions: {
      type: Array as PropType<AvailableAction[]>,
      default: () => []
    }
  },
  emits: ['done'],
  data() {
    return {
      submitting: false,
      currentAction: '' as string,
      // 退回弹窗
      showReject: false,
      rejectReason: '',
      rejectMin: 10,
      // 通用确认弹窗
      showSimple: false,
      simpleAction: null as AvailableAction | null,
      simpleRemark: '',
      simpleMaxLen: 100,
      simpleBtnType: 'primary' as 'primary' | 'warning' | 'danger' | 'success',
      /** 圆角矩形 + 实心白字按钮内联样式 */
      btnStyle: 'margin-left: 12rpx; border-radius: 12rpx'
    }
  },
  computed: {
    /** 按角色裁剪后的可执行动作：审核员(role=0)仅 audit/reject，店方/超管全部 */
    filteredActions(): AvailableAction[] {
      const acts = this.actions || []
      if (isStoreStaff()) return acts
      return acts.filter((a) => a.action === 'audit' || a.action === 'reject')
    }
  },
  methods: {
    btnType(action: AvailableActionKey): 'primary' | 'warning' | 'danger' | 'success' {
      switch (action) {
        case 'complete':
          return 'success'
        case 'reject':
          return 'danger'
        case 'accept':
        case 'reopen':
          return 'warning'
        case 'audit':
        case 'update_finance':
        default:
          return 'primary'
      }
    },
    onAction(act: AvailableAction) {
      if (act.action === 'reject') {
        this.openReject(act)
        return
      }
      if (act.action === 'complete' || act.action === 'update_finance') {
        // 复杂表单需完整详情（含 receipts / 对账），跳转详情页执行
        uni.navigateTo({ url: `/pages/admin/order/detail?id=${this.orderId}` })
        return
      }
      // audit / accept / reopen：通用确认弹窗
      this.openSimple(act)
    },
    /* ========= 退回 ========= */
    openReject(act: AvailableAction) {
      this.showReject = true
      this.rejectReason = ''
      this.rejectMin = act.reason_min_length || 10
    },
    async confirmReject() {
      const reason = this.rejectReason.trim()
      if (reason.length < this.rejectMin) {
        uni.showToast({ title: `退回原因至少${this.rejectMin}字`, icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      this.currentAction = 'reject'
      try {
        await http.post(`/admin/orders/${this.orderId}/reject`, { reason })
        uni.showToast({ title: '已退回', icon: 'success' })
        this.showReject = false
        this.$emit('done')
      } catch (e) {
        console.error('退回失败', e)
      } finally {
        this.submitting = false
        this.currentAction = ''
      }
    },
    /* ========= 通用确认：audit / accept / reopen ========= */
    openSimple(act: AvailableAction) {
      this.simpleAction = act
      this.simpleRemark = ''
      this.simpleMaxLen = act.action === 'reopen' ? 200 : 100
      this.simpleBtnType = this.btnType(act.action)
      this.showSimple = true
    },
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
      this.currentAction = act.action
      try {
        const body = remark ? { remark } : {}
        await http.post(url, body)
        uni.showToast({ title: '操作成功', icon: 'success' })
        this.showSimple = false
        this.$emit('done')
      } catch (e) {
        console.error('操作失败', e)
      } finally {
        this.submitting = false
        this.currentAction = ''
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.action-bar {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  margin-top: 24rpx;
}

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

.popup-tip {
  margin-top: 12rpx;
  font-size: 26rpx;
  color: #888888;
  line-height: 1.6;
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
