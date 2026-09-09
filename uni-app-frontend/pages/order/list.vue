<template>
  <view class="page">
    <!-- 未登录：空态 + 登录引导 -->
    <view v-if="!isLoggedIn" class="not-login">
      <view class="empty-icon">📋</view>
      <view class="empty-text">登录后查看我的工单</view>
      <view class="empty-tip">登录后可发起报修、跟进处理进度</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <template v-else>
      <!-- 工具栏 -->
      <view class="toolbar">
        <view class="toolbar-title">我的工单</view>
        <view class="toolbar-right">
          <wd-button size="small" type="primary" round @click="createOrder">+ 新建</wd-button>
        </view>
      </view>

      <!-- 多企业时提供单位筛选 -->
      <view v-if="enterpriseFilterOptions.length > 2" class="filter-bar">
        <picker
          mode="selector"
          :range="enterpriseFilterOptions"
          range-key="name"
          :value="enterpriseFilterIndex"
          @change="onEnterpriseFilterChange"
        >
          <view class="filter-picker">
            <text class="filter-label">企业</text>
            <text class="filter-value">{{ currentEnterpriseFilterName }}</text>
            <text class="filter-arrow">▾</text>
          </view>
        </picker>
      </view>

      <!-- 状态 Tab -->
      <view class="tabs-wrap">
        <wd-tabs :model-value="activeTab" @change="onTabChange">
          <wd-tab v-for="tab in tabs" :key="tab.value" :title="tab.label" :name="tab.value"></wd-tab>
        </wd-tabs>
      </view>

      <!-- 列表：统一工单卡片（与审核工单 / 处理工单列表完全一致） -->
      <view class="order-list">
        <order-card
          v-for="item in list"
          :key="item.id"
          :item="item"
          @click="goOrderDetail(item.id)"
        >
          <!-- 被退回提示条（已退回状态专属） -->
          <view v-if="item.status === 'rejected' && item.reject_reason" class="reject-bar">
            已退回：{{ item.reject_reason }}
          </view>
          <!-- 已完成金额 -->
          <view v-if="item.status === 'completed' && item.amount" class="amount-line">
            金额：￥{{ formatAmount(item.amount) }}
          </view>

          <!-- 操作区：冒泡拦截放在原生 view 上（wd-button 是组件，组件事件上的 .stop 不生效，
               否则点按钮会先冒泡到卡片、触发“进入详情”）
               按钮为圆角矩形（12rpx，非胶囊），且为实心填充 + 白字：背景用对应 type 色
               （编辑/提交/删除=主题蓝、取消(草稿/已退回)=危险红、取消(已上报/待接单)=警告橙），
               与底栏「当前状态」的胶囊标签（小圆角、彩色底）形成明显区分 -->
          <template #actions>
            <view
              v-if="item.status === 'draft' || item.status === 'rejected' || item.status === 'reported' || item.status === 'pending_accept'"
              class="card-actions"
              @click.stop="stopCardClick"
            >
              <template v-if="item.status === 'draft' || item.status === 'rejected'">
                <wd-button size="small" :custom-style="actBtnStyle" @click="editOrder(item.id)">编辑</wd-button>
                <wd-button size="small" :custom-style="actBtnStyle" @click="submitDraft(item)">提交</wd-button>
                <wd-button size="small" :custom-style="actBtnStyle" @click="deleteOrder(item)">删除</wd-button>
              </template>
              <template v-else>
                <wd-button size="small" type="warning" :custom-style="actBtnStyle" @click="cancelOrder(item)">
                  取消
                </wd-button>
              </template>
            </view>
          </template>
        </order-card>

        <!-- 空状态 -->
        <view v-if="!loading && list.length === 0" class="empty">
          <view class="empty-icon">📋</view>
          <view class="empty-text">暂无工单</view>
          <view class="empty-tip">点击「+ 新建」发起报修</view>
        </view>

        <!-- 加载更多 -->
        <view v-if="list.length > 0" class="load-more">
          <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
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
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { PAGE_SIZE } from '@/utils/config'
import { normalizePage, formatAmount } from '@/utils/format'
import { useUserStore } from '@/stores/user'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      formatAmount
    }
  },
  data() {
    return {
      isLoggedIn: false,
      enterpriseFilterOptions: [] as { id: string; name: string }[],
      enterpriseFilterIndex: 0,
      // 我的工单 Tab：进行中(默认) / 草稿 / 已退回 / 已完成 / 已取消（V1.2 3.2 沿用；C19 新增「已退回」独立 Tab）
      tabs: [
        // { label: '进行中', value: 'active', status: 'reported,pending_accept,processing' },
		{ label: '进行中', value: 'active', status: 'reported,pending_accept,processing' },
        { label: '草稿', value: 'draft', status: 'draft' },
        { label: '已退回', value: 'rejected', status: 'rejected' },
        { label: '已完成', value: 'completed', status: 'completed' },
        { label: '已取消', value: 'cancelled', status: 'cancelled' }
      ],
      activeTab: 'active',
      /**
       * 卡片操作按钮：实心填充 + 白字，圆角矩形（12rpx），与底栏「当前状态」的胶囊标签明确区分。
       * - 背景 = 对应 type 色（编辑/提交/删除=主题蓝、取消=危险红/警告橙），文字白色；
       * - 去掉 wd-button 的 `plain`（默认即为实心白字），底色即 type 色；
       * - border-radius 走 custom-style 内联样式（12rpx 圆角矩形），因为 wd-button 圆角由
       *   `.wd-button.is-small` / `.is-round` 两个类选择器控制，普通自定义类优先级不够，覆盖不掉。
       */
      actBtnStyle: 'margin-left: 12rpx; border-radius: 12rpx',
      /** 取消工单弹窗 */
      showCancelPopup: false,
      cancelReason: '',
      cancelTargetId: '',
      cancelSubmitting: false,
      list: [] as any[],
      page: 1,
      totalPages: 1,
      loading: false,
      finished: false
    }
  },
  computed: {
    approvedEnterprises(): { enterprise_id: string; enterprise_name: string }[] {
      return (this.userStore.userInfo?.enterprises ?? []).filter((e) => e.status === 'approved')
    },
    currentEnterpriseFilterName(): string {
      const opt = this.enterpriseFilterOptions[this.enterpriseFilterIndex]
      return opt ? opt.name : '全部企业'
    }
  },
  onShow() {
    this.init()
  },
  onPullDownRefresh() {
    this.loadList(true).finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.isLoggedIn && !this.finished && !this.loading) {
      this.loadList(false)
    }
  },
  methods: {
    async init() {
      if (!this.userStore.token) {
        const ok = await this.userStore.ensureLoggedIn()
        if (!ok) {
          this.isLoggedIn = false
          return
        }
      }
      this.isLoggedIn = true
      try {
        await this.userStore.fetchUserInfo()
      } catch (e) {
        // 401 已由请求层静默续期/清态处理
      }
      this.buildEnterpriseFilter()
      this.loadList(true)
    },
    buildEnterpriseFilter() {
      this.enterpriseFilterOptions = [
        { id: '', name: '全部企业' },
        ...this.approvedEnterprises.map((e) => ({ id: e.enterprise_id, name: e.enterprise_name }))
      ]
      if (this.enterpriseFilterIndex >= this.enterpriseFilterOptions.length) {
        this.enterpriseFilterIndex = 0
      }
    },
    onEnterpriseFilterChange(e: { detail: { value: number } }) {
      this.enterpriseFilterIndex = Number(e.detail.value) || 0
      this.loadList(true)
    },
    currentStatusParam(): string {
      const t = this.tabs.find((x) => x.value === this.activeTab)
      return t ? t.status : ''
    },
    onTabChange(e: { name: string }) {
      this.activeTab = e.name
      this.loadList(true)
    },
    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = { page: targetPage, page_size: PAGE_SIZE }
        const status = this.currentStatusParam()
        if (status) params.status = status
        const opt = this.enterpriseFilterOptions[this.enterpriseFilterIndex]
        if (opt && opt.id) params.enterprise_id = opt.id
        const data = await http.get<any>('/orders', params)
        const res = normalizePage(data)
        this.page = res.page
        this.totalPages = res.total_pages
        this.finished = this.page >= this.totalPages
        this.list = reset ? res.list : [...this.list, ...res.list]
      } catch (e) {
        console.error('加载我的工单失败', e)
      } finally {
        this.loading = false
      }
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    /** 操作区空处理：仅用于拦截冒泡，避免卡片点击进入详情 */
    stopCardClick() {
      // no-op
    },
    createOrder() {
      uni.showLoading({ title: '创建中...' })
      http
        .post<{ order_id: string }>('/orders', {})
        .then((data) => {
          uni.hideLoading()
          uni.navigateTo({ url: `/pages/order/edit?id=${data.order_id}` })
        })
        .catch(() => uni.hideLoading())
    },
    editOrder(orderId: string) {
      uni.navigateTo({ url: `/pages/order/edit?id=${orderId}` })
    },
    goOrderDetail(orderId: string) {
      uni.navigateTo({ url: `/pages/order/detail?id=${orderId}` })
    },
    /** 草稿提交 */
    submitDraft(item: any) {
      const that = this
      uni.showModal({
        title: '提交报修',
        content: '提交后工单将上报本单位审核员审核，确认提交？',
        success: async (res) => {
          if (!res.confirm) return
          try {
            await http.post(`/orders/${item.id}/submit`, {})
            uni.showToast({ title: '提交成功', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            // 后端 4502（必填不全）等错误已 Toast
          }
        }
      })
    },
    /** 取消工单：打开原因弹窗（与 order-action-bar 退回弹窗统一） */
    cancelOrder(item: any) {
      this.cancelTargetId = item.id
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
        await http.post(`/orders/${this.cancelTargetId}/cancel`, { reason })
        uni.showToast({ title: '已取消', icon: 'success' })
        this.showCancelPopup = false
        this.loadList(true)
      } catch (e) {
        console.error('取消失败', e)
      } finally {
        this.cancelSubmitting = false
      }
    },
    /** 删除草稿 */
    deleteOrder(item: any) {
      const that = this
      uni.showModal({
        title: '删除草稿',
        content: '确定删除该草稿吗？删除后不可恢复',
        confirmColor: '#fa5151',
        success: async (res) => {
          if (!res.confirm) return
          try {
            await http.delete(`/orders/${item.id}`)
            uni.showToast({ title: '已删除', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            console.error('删除失败', e)
          }
        }
      })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding-bottom: 40rpx;
  background-color: #f5f6f8;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  background-color: #ffffff;
  border-bottom: 1rpx solid #f0f0f0;
  position: sticky;
  top: 0;
  z-index: 20;

  .toolbar-title {
    font-size: 32rpx;
    font-weight: 600;
    color: #1a1a1a;
  }
}

.filter-bar {
  padding: 16rpx 24rpx;
  background-color: #ffffff;
  border-bottom: 1rpx solid #f5f5f5;

  .filter-picker {
    display: inline-flex;
    align-items: center;
    background-color: #f5f6f8;
    padding: 10rpx 20rpx;
    border-radius: 999rpx;
    max-width: 320rpx;

    .filter-label {
      font-size: 24rpx;
      color: #999999;
      margin-right: 12rpx;
    }

    .filter-value {
      font-size: 26rpx;
      color: #333333;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .filter-arrow {
      margin-left: 8rpx;
      font-size: 22rpx;
      color: #999999;
    }
  }
}

.tabs-wrap {
  background-color: #ffffff;
  padding: 0 24rpx;
}

.order-list {
  padding: 20rpx 24rpx;
}

/* 卡片主体由 components/order-card 统一渲染；此处仅保留「我的工单」专属的附加块样式 */
.order-list {
  .reject-bar {
    margin-top: 16rpx;
    background-color: #fff7e6;
    color: #ad6800;
    font-size: 24rpx;
    padding: 10rpx 16rpx;
    border-radius: 10rpx;
    line-height: 1.5;
  }

  .amount-line {
    margin-top: 16rpx;
    font-size: 28rpx;
    font-weight: 600;
    color: #fa5151;
  }

  /* 操作按钮：圆角矩形 + 实心白字（背景=type 色），区别于底栏状态胶囊；圆角由 actBtnStyle 内联下发 */
  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 24rpx;
  }
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 140rpx;

  .empty-icon {
    font-size: 88rpx;
  }

  .empty-text {
    margin-top: 24rpx;
    font-size: 30rpx;
    color: #666666;
  }

  .empty-tip {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #aaaaaa;
  }
}

.not-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 200rpx;

  .empty-icon {
    font-size: 96rpx;
  }

  .empty-text {
    margin-top: 24rpx;
    font-size: 30rpx;
    color: #666666;
  }

  .empty-tip {
    margin-top: 12rpx;
    margin-bottom: 32rpx;
    font-size: 24rpx;
    color: #aaaaaa;
  }
}

.load-more {
  padding: 24rpx 0 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: #bbbbbb;
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
