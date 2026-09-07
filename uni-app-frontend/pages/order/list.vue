<template>
  <view class="page">
    <!-- 未登录：空态 + 登录引导 -->
    <view v-if="!isLoggedIn" class="not-login">
      <view class="empty-icon">📋</view>
      <view class="empty-text">登录后查看工单 / 维修任务</view>
      <view class="empty-tip">登录后可发起报修、跟进处理进度</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <template v-else>
      <!-- 顶部工具栏：角色自适应 -->
      <view class="toolbar">
        <!-- 店方：维修工作台 / 我的报修 双入口 -->
        <view v-if="isStoreStaff" class="seg">
          <view
            class="seg-item"
            :class="{ active: mode === 'store' }"
            @click="switchMode('store')"
          >
            维修工作台
          </view>
          <view
            class="seg-item"
            :class="{ active: mode === 'member' }"
            @click="switchMode('member')"
          >
            我的报修
          </view>
        </view>
        <view v-else class="toolbar-title">我的报修</view>

        <view class="toolbar-right">
          <!-- 店方工作台：可接单提示入口 -->
          <wd-button
            v-if="mode === 'store'"
            size="small"
            type="primary"
            round
            @click="refresh(true)"
          >
            刷新
          </wd-button>
          <wd-button v-else size="small" type="primary" round @click="createOrder">+ 新建</wd-button>
        </view>
      </view>

      <!-- 单位/企业筛选 -->
      <view v-if="mode === 'store' || showMemberEnterprisePicker" class="filter-bar">
        <picker
          mode="selector"
          :range="enterpriseFilterOptions"
          range-key="name"
          @change="onEnterpriseFilterChange"
        >
          <view class="filter-picker">
            <text class="filter-label">{{ mode === 'store' ? '单位' : '企业' }}</text>
            <text class="filter-value">{{ currentEnterpriseFilterName }}</text>
            <text class="filter-arrow">▾</text>
          </view>
        </picker>
        <!-- 店方：搜索工单号/关键字 -->
        <view v-if="mode === 'store'" class="store-search">
          <input
            v-model="keyword"
            class="search-input"
            placeholder="单号/描述/报修人"
            placeholder-class="search-placeholder"
            confirm-type="search"
            @confirm="onSearch"
          />
          <text class="search-btn" @click="onSearch">搜索</text>
        </view>
      </view>

      <!-- 单位审核待办卡（报修人模式下，当前上下文单位为审核单位时展示） -->
      <view
        v-if="mode === 'member' && showReviewTodo"
        class="review-todo"
        @click="goReviewModule"
      >
        <view class="todo-icon">⚡</view>
        <view class="todo-body">
          <view class="todo-title">本单位待审核工单</view>
          <view class="todo-sub">
            {{ reviewTodoCount > 0 ? `有待审核 ${reviewTodoCount} 单（按提交时间倒序）` : '暂无待审核工单' }}
          </view>
        </view>
        <view class="todo-link">
          去审核 ›
        </view>
      </view>

      <!-- 状态 Tab -->
      <view class="tabs-wrap">
        <wd-tabs
          :model-value="activeTab"
          @change="onTabChange"
        >
          <wd-tab
            v-for="tab in currentTabs"
            :key="tab.value"
            :title="tab.label"
            :name="tab.value"
          ></wd-tab>
        </wd-tabs>
      </view>

      <!-- 列表 -->
      <view class="order-list">
        <!-- ── 店方工作台卡片 ── -->
        <template v-if="mode === 'store'">
          <view v-for="item in list" :key="item.id" class="order-card" @click="goAdminDetail(item.id)">
            <view class="card-header">
              <view class="card-title">
                <text class="title-no">{{ item.order_no || '未生成单号' }}</text>
                <text v-if="item.urgency_label" class="title-urgency">{{ item.urgency_label }}</text>
              </view>
              <wd-tag :type="statusTagType(item.status)" round>{{ item.status_label }}</wd-tag>
            </view>
            <view class="card-cat">
              {{ item.enterprise_name || '未指定单位' }}
              <template v-if="item.category_name">
                · {{ item.category_name }}<template v-if="item.property_name">/{{ item.property_name }}</template>
              </template>
            </view>
            <view class="card-desc">{{ item.description || '（无描述）' }}</view>
            <view class="card-meta">
              <text>报修人：{{ item.reporter?.nickname || '--' }}</text>
              <text class="meta-time">{{ formatDateTime(item.submitted_at || item.created_at) }}</text>
            </view>
            <view class="card-actions">
              <wd-button size="small" type="primary" plain round>查看并处理</wd-button>
            </view>
          </view>
        </template>

        <!-- ── 我的报修卡片 ── -->
        <template v-else>
          <view v-for="item in list" :key="item.id" class="order-card" @click="goOrderDetail(item.id)">
            <view class="card-header">
              <view class="card-title member-title">
                <text>{{ item.category_name || '未分类' }}</text>
                <template v-if="item.property_name"><text class="cat-sep">·</text>{{ item.property_name }}</template>
              </view>
              <wd-tag :type="statusTagType(item.status)" round>{{ item.status_label }}</wd-tag>
            </view>
            <view class="card-desc">{{ item.description || '（无描述）' }}</view>
            <view v-if="item.enterprise_name" class="card-enterprise">{{ item.enterprise_name }}</view>
            <view class="card-meta">
              <wd-tag :type="urgencyTagType(item.urgency)" plain round>{{ item.urgency_label }}</wd-tag>
              <text class="meta-time">{{ formatDateTime(item.submitted_at || item.created_at) }}</text>
            </view>
            <!-- 被退回草稿提示条 -->
            <view v-if="item.status === 'draft' && item.reject_reason" class="reject-bar">
              已退回：{{ item.reject_reason }}
            </view>
            <!-- 已完成金额 -->
            <view v-if="item.status === 'completed' && item.amount" class="amount-line">
              金额：￥{{ formatAmount(item.amount) }}
            </view>
            <view v-if="item.order_no" class="card-no">单号：{{ item.order_no }}</view>

            <view class="card-actions">
              <template v-if="item.status === 'draft'">
                <wd-button size="small" plain round @click.stop="editOrder(item.id)">编辑</wd-button>
                <wd-button size="small" plain round @click.stop="submitDraft(item)">提交</wd-button>
                <wd-button size="small" type="danger" plain round @click.stop="cancelOrder(item)">取消</wd-button>
                <wd-button size="small" plain round @click.stop="deleteOrder(item)">删除</wd-button>
              </template>
              <template v-else-if="item.status === 'reported' || item.status === 'pending_accept'">
                <wd-button size="small" type="warning" plain round @click.stop="cancelOrder(item)">取消</wd-button>
              </template>
            </view>
          </view>
        </template>

        <!-- 空状态 -->
        <view v-if="!loading && list.length === 0" class="empty">
          <view class="empty-icon">{{ mode === 'store' ? '🔧' : '📋' }}</view>
          <view class="empty-text">
            {{ mode === 'store' ? '当前条件下暂无工单' : '暂无工单' }}
          </view>
          <view v-if="mode === 'member'" class="empty-tip">点击「+ 新建」发起报修</view>
        </view>

        <!-- 加载更多 -->
        <view v-if="list.length > 0" class="load-more">
          <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { PAGE_SIZE } from '@/utils/config'
import { normalizePage, formatDateTime, formatAmount, statusTagType, urgencyTagType } from '@/utils/format'
import { isStoreStaff, isReviewerOf } from '@/utils/auth'
import { useUserStore } from '@/stores/user'
import { useEnterpriseStore } from '@/stores/enterprise'

type Mode = 'member' | 'store'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      enterpriseStore: useEnterpriseStore(),
      formatDateTime,
      formatAmount,
      statusTagType,
      urgencyTagType
    }
  },
  data() {
    return {
      mode: 'member' as Mode,
      isLoggedIn: false,
      // 企业筛选（店方：全部单位/具体单位；报修人：多企业筛选）
      enterpriseFilterOptions: [] as { id: string; name: string }[],
      enterpriseFilterIndex: 0,
      keyword: '',
      // 成员模式 Tab：进行中(默认) / 草稿 / 已完成 / 已取消
      memberTabs: [
        { label: '进行中', value: 'active', status: 'reported,pending_accept,processing' },
        { label: '草稿', value: 'draft', status: 'draft' },
        { label: '已完成', value: 'completed', status: 'completed' },
        { label: '已取消', value: 'cancelled', status: 'cancelled' }
      ],
      // 店方模式 Tab：待接单(默认) / 处理中 / 已完成 / 已取消
      storeTabs: [
        { label: '待接单', value: 'pending_accept', status: 'pending_accept' },
        { label: '处理中', value: 'processing', status: 'processing' },
        { label: '已完成', value: 'completed', status: 'completed' },
        { label: '已取消', value: 'cancelled', status: 'cancelled' }
      ],
      activeMemberTab: 'active',
      activeStoreTab: 'pending_accept',
      list: [] as any[],
      page: 1,
      totalPages: 1,
      loading: false,
      finished: false,
      // 审核待办
      reviewTodoCount: 0,
      loadingReviewTodo: false
    }
  },
  computed: {
    isStoreStaff(): boolean {
      return isStoreStaff()
    },
    activeTab(): string {
      return this.mode === 'store' ? this.activeStoreTab : this.activeMemberTab
    },
    currentTabs() {
      return this.mode === 'store' ? this.storeTabs : this.memberTabs
    },
    currentEnterpriseId(): string {
      return this.enterpriseStore.currentEnterpriseId
    },
    approvedEnterprises(): { enterprise_id: string; enterprise_name: string }[] {
      return (this.userStore.userInfo?.enterprises ?? []).filter((e) => e.status === 'approved')
    },
    showMemberEnterprisePicker(): boolean {
      return this.approvedEnterprises.length > 1
    },
    /** 店方模式下可查看自己的报修（本人也是某单位 approved 成员） */
    showReviewTodo(): boolean {
      return this.mode === 'member' && !!this.currentEnterpriseId && isReviewerOf(this.currentEnterpriseId)
    },
    currentEnterpriseFilterName(): string {
      const idx = this.enterpriseFilterIndex
      if (idx <= 0 || idx >= this.enterpriseFilterOptions.length) {
        return this.mode === 'store' ? '全部单位' : '全部企业'
      }
      return this.enterpriseFilterOptions[idx].name
    }
  },
  onShow() {
    this.init()
  },
  onPullDownRefresh() {
    this.refresh(true).finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.isLoggedIn && !this.finished && !this.loading) {
      this.loadList(false)
    }
  },
  methods: {
    async init() {
      // 无 token：先尝试静默续期一次（老用户无感登录）
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
      this.mode = isStoreStaff() ? 'store' : 'member'
      uni.setNavigationBarTitle({
        title: this.mode === 'store' ? '维修工作台' : '我的报修'
      })
      await this.buildEnterpriseFilter()
      this.refresh(true)
    },
    switchMode(mode: Mode) {
      if (this.mode === mode) return
      this.mode = mode
      uni.setNavigationBarTitle({ title: mode === 'store' ? '维修工作台' : '我的报修' })
      // 重新构建对应模式的筛选下拉（保留原选项 id 一致时不重复请求）
      if (mode === 'store') {
        this.loadStoreEnterprises()
      } else {
        this.enterpriseFilterOptions = [
          { id: '', name: '全部企业' },
          ...this.approvedEnterprises.map((e) => ({ id: e.enterprise_id, name: e.enterprise_name }))
        ]
        this.enterpriseFilterIndex = 0
      }
      this.refresh(true)
    },
    /** 店方：全部单位 + 下拉（数据源 GET /admin/enterprises） */
    async loadStoreEnterprises() {
      try {
        const data = await http.get<any>('/admin/enterprises', { page: 1, page_size: 100 })
        const res = normalizePage(data)
        const opts = (res.list as any[]).map((e) => ({ id: e.id, name: e.name }))
        this.enterpriseFilterOptions = [{ id: '', name: '全部单位' }, ...opts]
        if (this.enterpriseFilterIndex >= this.enterpriseFilterOptions.length) {
          this.enterpriseFilterIndex = 0
        }
      } catch (e) {
        this.enterpriseFilterOptions = [{ id: '', name: '全部单位' }]
      }
    },
    /** 报修人：多企业筛选列表 */
    async buildMemberEnterprises() {
      this.enterpriseFilterOptions = [
        { id: '', name: '全部企业' },
        ...this.approvedEnterprises.map((e) => ({ id: e.enterprise_id, name: e.enterprise_name }))
      ]
    },
    async buildEnterpriseFilter() {
      if (this.mode === 'store') {
        await this.loadStoreEnterprises()
      } else {
        this.buildMemberEnterprises()
      }
    },
    onEnterpriseFilterChange(e: { detail: { value: number } }) {
      this.enterpriseFilterIndex = Number(e.detail.value) || 0
      this.refresh(true)
    },
    currentStatusParam(): string {
      const tab = this.mode === 'store' ? this.storeTabs : this.memberTabs
      const cur = this.activeTab
      const found = tab.find((t) => t.value === cur)
      return found ? found.status : ''
    },
    onSearch() {
      this.refresh(true)
    },
    onTabChange(e: { name: string }) {
      if (this.mode === 'store') {
        this.activeStoreTab = e.name
      } else {
        this.activeMemberTab = e.name
      }
      this.refresh(true)
    },
    async refresh(reset = true) {
      if (this.mode === 'member' && this.showReviewTodo) {
        this.loadReviewTodoCount()
      }
      return this.loadList(reset)
    },
    /** 加载待审核数（审核员访问 GET /admin/orders 需带 enterprise_id） */
    async loadReviewTodoCount() {
      if (this.loadingReviewTodo) return
      this.loadingReviewTodo = true
      try {
        const data = await http.get<any>('/admin/orders', {
          status: 'reported',
          enterprise_id: this.currentEnterpriseId,
          page: 1,
          page_size: 1
        })
        const res = normalizePage(data)
        this.reviewTodoCount = res.total
      } catch (e) {
        this.reviewTodoCount = 0
      } finally {
        this.loadingReviewTodo = false
      }
    },
    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: PAGE_SIZE
        }
        const status = this.currentStatusParam()
        if (status) params.status = status
        const entId =
          this.enterpriseFilterIndex > 0 && this.enterpriseFilterIndex < this.enterpriseFilterOptions.length
            ? this.enterpriseFilterOptions[this.enterpriseFilterIndex].id
            : ''
        if (entId) params.enterprise_id = entId

        let data: any
        if (this.mode === 'store') {
          if (this.keyword.trim()) params.keyword = this.keyword.trim()
          data = await http.get<any>('/admin/orders', params)
        } else {
          data = await http.get<any>('/orders', params)
        }
        const res = normalizePage(data)
        this.page = res.page
        this.totalPages = res.total_pages
        this.finished = this.page >= this.totalPages
        this.list = reset ? res.list : [...this.list, ...res.list]
      } catch (e) {
        console.error('加载工单列表失败', e)
      } finally {
        this.loading = false
      }
    },
    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
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
    goAdminDetail(orderId: string) {
      uni.navigateTo({ url: `/pages/admin/order/detail?id=${orderId}` })
    },
    goReviewModule() {
      uni.navigateTo({ url: '/pages/review/index' })
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
    /** 取消工单（填原因） */
    cancelOrder(item: any) {
      const that = this
      uni.showModal({
        title: '取消工单',
        content: '请填写取消原因',
        editable: true,
        placeholderText: '如：问题已自行解决（≤200字）',
        success: async (res) => {
          if (!res.confirm) return
          const reason = (res.content || '').trim()
          if (!reason) {
            uni.showToast({ title: '请填写取消原因', icon: 'none' })
            return
          }
          try {
            await http.post(`/orders/${item.id}/cancel`, { reason })
            uni.showToast({ title: '已取消', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            console.error('取消失败', e)
          }
        }
      })
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

  .seg {
    display: flex;
    background: #f0f2f5;
    border-radius: 999rpx;
    padding: 4rpx;

    .seg-item {
      padding: 10rpx 28rpx;
      font-size: 26rpx;
      color: #666666;
      border-radius: 999rpx;

      &.active {
        background: #ffffff;
        color: #4d80f0;
        font-weight: 600;
        box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.08);
      }
    }
  }
}

.filter-bar {
  display: flex;
  align-items: center;
  padding: 16rpx 24rpx;
  background-color: #ffffff;
  border-bottom: 1rpx solid #f5f5f5;

  .filter-picker {
    display: flex;
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

  .store-search {
    flex: 1;
    display: flex;
    align-items: center;
    background: #f5f6f8;
    border-radius: 999rpx;
    padding: 4rpx 8rpx 4rpx 20rpx;
    margin-left: 16rpx;

    .search-input {
      flex: 1;
      font-size: 24rpx;
      height: 56rpx;
      color: #333333;
    }

    .search-btn {
      padding: 8rpx 20rpx;
      font-size: 24rpx;
      color: #4d80f0;
    }
  }
}

.search-placeholder {
  color: #bbbbbb;
}

/* 单位审核待办卡 */
.review-todo {
  display: flex;
  align-items: center;
  margin: 20rpx 24rpx 0;
  padding: 24rpx 28rpx;
  background: linear-gradient(135deg, #fff7e6 0%, #ffffff 100%);
  border: 2rpx solid #ffd591;
  border-radius: 20rpx;

  .todo-icon {
    font-size: 44rpx;
    margin-right: 20rpx;
  }

  .todo-body {
    flex: 1;
    min-width: 0;

    .todo-title {
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .todo-sub {
      margin-top: 8rpx;
      font-size: 24rpx;
      color: #8a6d3b;
    }
  }

  .todo-link {
    font-size: 26rpx;
    color: #4d80f0;
    flex-shrink: 0;
  }
}

.tabs-wrap {
  background-color: #ffffff;
  padding: 0 24rpx;
}

.order-list {
  padding: 20rpx 24rpx;
}

.order-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx;
  margin-bottom: 20rpx;
  box-shadow: 0 4rpx 16rpx rgba(0, 0, 0, 0.04);

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .card-title {
      flex: 1;
      min-width: 0;
      margin-right: 16rpx;
      font-size: 30rpx;
      font-weight: 600;
      color: #1a1a1a;
      display: flex;
      align-items: center;

      .title-no {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
      }

      .title-urgency {
        margin-left: 12rpx;
        flex-shrink: 0;
        font-size: 20rpx;
        color: #fa5151;
        background-color: rgba(250, 81, 81, 0.08);
        border-radius: 6rpx;
        padding: 2rpx 10rpx;
      }

      .cat-sep {
        margin: 0 8rpx;
        color: #cccccc;
        font-weight: 400;
      }
    }
  }

  .card-cat {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #666666;
  }

  .card-desc {
    margin-top: 12rpx;
    font-size: 26rpx;
    color: #333333;
    line-height: 1.5;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    overflow: hidden;
  }

  .card-enterprise {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #4d80f0;
  }

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

  .card-no {
    margin-top: 12rpx;
    font-size: 22rpx;
    color: #bbbbbb;
  }

  .card-meta {
    display: flex;
    align-items: center;
    margin-top: 16rpx;

    .meta-time {
      margin-left: 16rpx;
      font-size: 24rpx;
      color: #999999;
    }
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 24rpx;

    wd-button {
      margin-left: 12rpx;
    }
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
</style>
