<template>
  <view class="page">
    <!-- 未登录：空态 + 登录引导 -->
    <view v-if="!isLoggedIn" class="not-login">
      <view class="empty-text">登录后查看我的工单</view>
      <view class="empty-tip">登录后可发起报修、跟进处理进度</view>
      <wd-button type="primary" round size="small" @click="goLogin">去登录</wd-button>
    </view>

    <template v-else>
      <!-- 工具栏 -->
      <view class="toolbar">
        <view class="toolbar-title">{{ manageMode ? '工单' : '我的工单' }}</view>
        <view class="toolbar-right">
          <view v-if="manageMode" class="tb-filter" :class="{ on: activeFilterCount > 0 }" @click="onOpenDrawer">
            <text>筛选</text>
            <text v-if="activeFilterCount > 0" class="tb-filter-badge">{{ activeFilterCount }}</text>
          </view>
          <wd-button size="small" type="primary" round @click="createOrder">+ 新建</wd-button>
        </view>
      </view>

      <!-- ============ 管理模式（店方 / 单位审核员）：状态 Tab + 角标 ============ -->
      <scroll-view v-if="manageMode" class="seg" scroll-x :show-scrollbar="false">
        <view class="seg-inner">
          <view
            v-for="t in manageTabs"
            :key="t.value"
            class="seg-btn"
            :class="{ active: activeTab === t.value }"
            @click="onManageTab(t.value)"
          >
            <text class="seg-txt">{{ t.label }}</text>
            <text v-if="tabCount(t.value) > 0" class="seg-count">{{ tabCount(t.value) }}</text>
          </view>
        </view>
      </scroll-view>

      <!-- ============ 管理模式：企业 / 用户 / 时间排序（置于状态 Tab 下方） ============ -->
      <view v-if="manageMode" class="filter-scope">
        <picker
          mode="selector"
          :range="entScopeOptions"
          range-key="name"
          :value="scopeEntIndex"
          @change="onScopeEntChange"
        >
          <view class="scope-picker">
            <text class="scope-label">企业</text>
            <text class="scope-value">{{ scopeEntName }}</text>
            <text class="scope-arrow">▾</text>
          </view>
        </picker>
        <picker
          mode="selector"
          :range="reporterScopeOptions"
          range-key="name"
          :value="scopeReporterIndex"
          :disabled="!fEnterpriseId"
          @change="onScopeReporterChange"
        >
          <view class="scope-picker" :class="{ 'is-disabled': !fEnterpriseId }">
            <text class="scope-label">用户</text>
            <text class="scope-value">{{ fEnterpriseId ? scopeReporterName : '请先选企业' }}</text>
            <text class="scope-arrow">▾</text>
          </view>
        </picker>
        <view class="scope-sort" :class="{ on: timeSort === 'desc' || timeSort === 'asc' }" @click="toggleTimeSort">
          <text>按时间</text>
          <text class="scope-sort-arrow">{{ timeSort === 'asc' ? '↑' : '↓' }}</text>
        </view>
      </view>

      <!-- 普通模式：原文保留的多企业筛选 + 状态 Tab -->
      <view v-else-if="enterpriseFilterOptions.length > 2" class="filter-bar">
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
      <view v-if="!manageMode" class="tabs-wrap">
        <wd-tabs :model-value="activeTab" @change="onTabChange">
          <wd-tab v-for="tab in myTabs" :key="tab.value" :title="tab.label" :name="tab.value"></wd-tab>
        </wd-tabs>
      </view>

      <!-- 已生效筛选：可单条清除 -->
      <scroll-view v-if="crumbs.length > 0" class="crumbs" scroll-x :show-scrollbar="false">
        <view v-for="c in crumbs" :key="c.key" class="crumb" @click="c.clear">
          <text class="crumb-txt">{{ c.label }}</text>
          <text class="crumb-x">×</text>
        </view>
      </scroll-view>

      <!-- 列表：统一工单卡片 -->
      <view class="order-list">
        <order-card
          v-for="item in list"
          :key="item.id"
          :item="item"
          :hide-enterprise="cardHideEnterprise"
          @click="goOrderDetail(item.id)"
        >
          <!-- 被退回提示条（金额已由 order-card 内置显示，此处不重复渲染） -->
          <view v-if="item.status === 'rejected' && item.reject_reason" class="reject-bar">
            已退回：{{ item.reject_reason }}
          </view>

          <!--
            操作区：冒泡拦截放在原生 view 上（wd-button 是组件，组件事件上的 .stop 不生效）。
            管理模式：卡片上不再提供就地操作（待审核的「审核通过」已移除，统一到详情页处理）。
            普通模式：草稿/退回→编辑·提交·删除；已上报·待接单→取消。
          -->
          <template #actions>
            <view
              v-if="!manageMode && myActionable(item.status)"
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
          <view class="empty-text">暂无工单</view>
          <view class="empty-tip" @click="clearAllFilters">
            {{ crumbs.length ? '试试清除筛选条件' : '点击「+ 新建」发起报修' }}
          </view>
        </view>

        <!-- 加载更多 -->
        <view v-if="list.length > 0" class="load-more">
          <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
        </view>
      </view>

      <!-- ============ 管理模式：筛选抽屉 ============ -->
      <wd-popup v-model="showFilterDrawer" position="bottom" round>
        <view class="drawer">
          <view class="drawer-hd">
            <text class="drawer-title">筛选</text>
            <text class="drawer-reset" @click="resetFilter">重置</text>
          </view>

          <view class="drawer-item">
            <text class="drawer-label">项目大类</text>
            <picker mode="selector" :range="categoryOptions" range-key="name" :value="fcCatIndex" @change="onCatChange">
              <view class="drawer-picker">
                <text>{{ fcCatIndex >= 0 ? categoryOptions[fcCatIndex].name : '不限' }}</text>
                <text class="drawer-arrow">▾</text>
              </view>
            </picker>
          </view>

          <view class="drawer-item">
            <text class="drawer-label">问题类型</text>
            <picker
              mode="selector"
              :range="propertyOptions"
              range-key="name"
              :value="fcPropIndex"
              :disabled="fcCatIndex < 0"
              @change="onPropChange"
            >
              <view class="drawer-picker" :class="{ off: fcCatIndex < 0 }">
                <text>{{ fcPropIndex >= 0 ? propertyOptions[fcPropIndex].name : fcCatIndex < 0 ? '请先选择大类' : '不限' }}</text>
                <text class="drawer-arrow">▾</text>
              </view>
            </picker>
          </view>

          <view class="drawer-item">
            <text class="drawer-label">紧急程度</text>
            <picker mode="selector" :range="urgencyLabels" :value="fcUrgencyIndex" @change="onUrgencyChange">
              <view class="drawer-picker">
                <text>{{ fcUrgencyIndex >= 0 ? urgencyLabels[fcUrgencyIndex] : '不限' }}</text>
                <text class="drawer-arrow">▾</text>
              </view>
            </picker>
          </view>

          <view class="drawer-item">
            <text class="drawer-label">提交日期</text>
            <view class="drawer-range">
              <picker mode="date" :value="fcDateFrom || ''" @change="onDateFromChange">
                <view class="drawer-date">{{ fcDateFrom || '开始' }}</view>
              </picker>
              <text class="drawer-sep">至</text>
              <picker mode="date" :value="fcDateTo || ''" @change="onDateToChange">
                <view class="drawer-date">{{ fcDateTo || '结束' }}</view>
              </picker>
            </view>
          </view>

          <view class="drawer-item">
            <text class="drawer-label">关键字</text>
            <input
              v-model="fcKeyword"
              class="drawer-input"
              placeholder="工单号 / 故障描述"
              placeholder-style="color:#9ca3af;font-size:26rpx"
              confirm-type="done"
            />
          </view>

          <view class="drawer-actions">
            <wd-button plain block size="large" @click="showFilterDrawer = false">取消</wd-button>
            <wd-button type="primary" block size="large" @click="applyFilter">确定</wd-button>
          </view>
        </view>
      </wd-popup>

      <!-- 取消工单弹窗（普通模式） -->
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
    </template>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { PAGE_SIZE } from '@/utils/config'
import { normalizePage, formatAmount } from '@/utils/format'
import { adminAPI, orderAPI, enterpriseAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { useOrderStore } from '@/stores/order'
import { isStoreStaff, isUnitReviewer, reviewerEnterprises } from '@/utils/auth'
import type { CategoryOption, PropertyOption, ReporterOption } from '@/types'

/** 「未完成」聚合状态（传后端为逗号串） */
const UNFINISHED = 'reported,pending_accept,processing'

interface StatusTab {
  label: string
  value: string
}

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      orderStore: useOrderStore(),
      formatAmount
    }
  },
  data() {
    return {
      isLoggedIn: false,
      list: [] as any[],
      page: 1,
      totalPages: 1,
      loading: false,
      finished: false,
      /** 管理模式：状态淘宝数量角标 */
      statusCounts: {} as Record<string, number>,
      /** 首页跳转带来的待消费上下文 */
      activeTab: '',

      /* ---- 普通模式 ---- */
      enterpriseFilterOptions: [] as { id: string; name: string }[],
      enterpriseFilterIndex: 0,
      myTabs: [
        { label: '进行中', value: 'active' as string, status: 'reported,pending_accept,processing' },
        { label: '草稿', value: 'draft' as string, status: 'draft' },
        { label: '已退回', value: 'rejected' as string, status: 'rejected' },
        { label: '已完成', value: 'completed' as string, status: 'completed' },
        { label: '已取消', value: 'cancelled' as string, status: 'cancelled' }
      ] as (StatusTab & { status: string })[],

      /* ---- 管理模式：筛选 ---- */
      showFilterDrawer: false,
      enterpriseOptions: [] as { id: string; name: string }[],
      categoryOptions: [] as CategoryOption[],
      /** 已提交的筛选（点「确定」后才生效） */
      fEnterpriseId: '',
      fCategoryId: '',
      fPropertyId: '',
      fReporterId: '',
      fUrgency: '',
      fDateFrom: '',
      fDateTo: '',
      fKeyword: '',
      /** 按提交时间排序：desc=新→旧 / asc=旧→新 */
      timeSort: 'desc' as 'desc' | 'asc',
      /** 报修人候选（顶部「用户」筛选用） */
      reporterOptions: [] as ReporterOption[],
      /** 抽屉内的临时值 */
      fcCatIndex: -1,
      fcPropIndex: -1,
      fcUrgencyIndex: -1,
      fcDateFrom: '',
      fcDateTo: '',
      fcKeyword: '',
      urgencyLabels: ['普通', '紧急', '非常紧急'] as string[],
      urgencyKeys: ['normal', 'urgent', 'very_urgent'] as string[],

      /* ---- 卡片操作 ---- */
      actBtnStyle: 'margin-left: 16rpx; border-radius: 12rpx',
      showCancelPopup: false,
      cancelReason: '',
      cancelTargetId: '',
      cancelSubmitting: false
    }
  },
  computed: {
    /**
     * 是否走管理数据源。
     * 维修业务员(role>=1) 与 单位审核员 → /admin/orders；
     * 普通报修用户 → /orders（自己报的单）。
     */
    manageMode(): boolean {
      return isStoreStaff() || isUnitReviewer()
    },
    /**
     * 卡片是否隐藏企业名：
     * - 管理模式：顶部「企业」筛选选定了具体企业（fEnterpriseId 非空）时隐藏；
     * - 普通模式：企业下拉选定了具体企业（enterpriseFilterIndex !== 0）时隐藏；
     * 二者在「全部企业」时均不隐藏（卡片可能来自不同企业，顶部无上下文）。
     */
    cardHideEnterprise(): boolean {
      return this.manageMode ? !!this.fEnterpriseId : this.enterpriseFilterIndex !== 0
    },
    /**
     * 管理模式状态 Tab（移植自 Web 端 views/orders/list.vue）：
     * 同一套状态只换标签——pending_accept 对维修员是「待接单」、对审核员是「已审核通过」。
     */
    manageTabs(): StatusTab[] {
      if (!this.manageMode) return []
      const store = () =>
        [
          { label: '全部', value: '' },
          { label: '待接单', value: 'pending_accept' },
          { label: '处理中', value: 'processing' },
          { label: '待补签', value: 'pending_sign' },
          { label: '已完成', value: 'completed' },
          { label: '已取消', value: 'cancelled' },
          { label: '未完成', value: UNFINISHED }
        ] as StatusTab[]
      const reviewer = () =>
        [
          { label: '全部', value: '' },
          { label: '待审核', value: 'reported' },
          { label: '已审核通过', value: 'pending_accept' },
          { label: '处理中', value: 'processing' },
          { label: '待补签', value: 'pending_sign' },
          { label: '已完成', value: 'completed' },
          { label: '已取消', value: 'cancelled' },
          { label: '未完成', value: UNFINISHED }
        ] as StatusTab[]
      // 兼任双角色：取并集，把「待审核」插到第 2 位
      if (isStoreStaff() && isUnitReviewer()) {
        const tabs = store()
        tabs.splice(1, 0, { label: '待审核', value: 'reported' })
        return tabs
      }
      return isStoreStaff() ? store() : reviewer()
    },
    /** 默认 Tab：未选择时一律「全部」 */
    defaultStatus(): string {
      return ''
    },
    approvedEnterprises(): { enterprise_id: string; enterprise_name: string }[] {
      return (this.userStore.userInfo?.enterprises ?? []).filter((e) => e.status === 'approved')
    },
    currentEnterpriseFilterName(): string {
      const opt = this.enterpriseFilterOptions[this.enterpriseFilterIndex]
      return opt ? opt.name : '全部企业'
    },
    /** 顶部「企业」筛选：首项为全部企业 */
    entScopeOptions(): { id: string; name: string }[] {
      return [{ id: '', name: '全部企业' }, ...this.enterpriseOptions]
    },
    scopeEntIndex(): number {
      const i = this.entScopeOptions.findIndex((x) => x.id === this.fEnterpriseId)
      return i < 0 ? 0 : i
    },
    scopeEntName(): string {
      return this.entScopeOptions[this.scopeEntIndex]?.name || '全部企业'
    },
    /** 顶部「用户」筛选：首项为全部用户，候选跟随企业范围 */
    reporterScopeOptions(): { id: string; name: string }[] {
      return [
        { id: '', name: '全部用户' },
        ...this.reporterOptions.map((r) => ({ id: r.id, name: r.nickname }))
      ]
    },
    scopeReporterIndex(): number {
      const i = this.reporterScopeOptions.findIndex((x) => x.id === this.fReporterId)
      return i < 0 ? 0 : i
    },
    scopeReporterName(): string {
      return this.reporterScopeOptions[this.scopeReporterIndex]?.name || '全部用户'
    },
    /** 所选大类下的属性选项 */
    propertyOptions(): PropertyOption[] {
      if (this.fcCatIndex < 0) return []
      return this.categoryOptions[this.fcCatIndex]?.properties ?? []
    },
    /** 已生效筛选项数 */
    activeFilterCount(): number {
      let n = 0
      if (this.fEnterpriseId) n++
      if (this.fReporterId) n++
      if (this.fCategoryId) n++
      if (this.fPropertyId) n++
      if (this.fUrgency) n++
      if (this.fDateFrom || this.fDateTo) n++
      if (this.fKeyword) n++
      return n
    },
    /** 面包屑：已生效条件，可单条清除 */
    crumbs(): { key: string; label: string; clear: () => void }[] {
      const out: { key: string; label: string; clear: () => void }[] = []
      if (!this.manageMode) return out
      // 企业 / 用户 已常驻显示在顶部 filter-scope 选择器，此处不再重复成面包屑气泡（避免信息重复）。
      if (this.fCategoryId) {
        const c = this.categoryOptions.find((x) => x.id === this.fCategoryId)
        out.push({
          key: 'cat',
          label: `大类：${c?.name ?? ''}`,
          clear: () => {
            this.fCategoryId = ''
            this.fPropertyId = ''
            this.loadList(true)
          }
        })
      }
      if (this.fPropertyId) {
        const p = this.propertyOptions.find((x) => x.id === this.fPropertyId)
        out.push({
          key: 'prop',
          label: `问题：${p?.name ?? ''}`,
          clear: () => {
            this.fPropertyId = ''
            this.loadList(true)
          }
        })
      }
      if (this.fUrgency) {
        const i = this.urgencyKeys.indexOf(this.fUrgency)
        out.push({
          key: 'urg',
          label: `紧急：${this.urgencyLabels[i] ?? ''}`,
          clear: () => {
            this.fUrgency = ''
            this.loadList(true)
          }
        })
      }
      if (this.fDateFrom || this.fDateTo) {
        out.push({
          key: 'date',
          label: `日期：${this.fDateFrom || '不限'} ~ ${this.fDateTo || '不限'}`,
          clear: () => {
            this.fDateFrom = ''
            this.fDateTo = ''
            this.loadList(true)
          }
        })
      }
      if (this.fKeyword) {
        out.push({
          key: 'kw',
          label: `关键字：${this.fKeyword}`,
          clear: () => {
            this.fKeyword = ''
            this.loadList(true)
          }
        })
      }
      return out
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
        /* 401 已由请求层静默续期处理 */
      }

      // 消费首页「详情 ›」带来的上下文（switchTab 不支持 URL 参数，改由 store 传递）。
      // 从首页跳转：企业=全部、用户=全部、状态=对应待办区块；非首页进入则保留上次所选。
      const pending = this.orderStore.consumePendingJump()
      if (this.manageMode) {
        if (pending) {
          this.activeTab = pending.status
          this.fEnterpriseId = ''
          this.fReporterId = ''
        }
        await this.ensureManageOptions()
        await this.loadReporterOptions()
        if (!this.activeTab) this.activeTab = this.defaultStatus
      } else {
        if (!this.activeTab) this.activeTab = 'active'
        this.buildEnterpriseFilter()
      }
      this.loadList(true)
    },

    /* ==================== 状态 Tab ==================== */

    tabCount(value: string): number {
      if (value === '') return this.statusCounts['all'] ?? 0
      if (value === UNFINISHED) return this.statusCounts['unfinished'] ?? 0
      return this.statusCounts[value] ?? 0
    },
    onManageTab(value: string) {
      if (this.activeTab === value) return
      this.activeTab = value
      this.loadList(true)
    },
    onTabChange(e: { name: string }) {
      this.activeTab = e.name
      this.loadList(true)
    },
    myActionable(status: string): boolean {
      return ['draft', 'rejected', 'reported', 'pending_accept'].includes(status)
    },
    currentStatusParam(): string {
      if (this.manageMode) return this.activeTab
      const t = this.myTabs.find((x) => x.value === this.activeTab)
      return t ? t.status : ''
    },

    /* ==================== 数据加载 ==================== */

    async loadList(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = { page: targetPage, page_size: PAGE_SIZE }

        const status = this.currentStatusParam()
        if (status) params.status = status

        if (this.manageMode) {
          if (this.fEnterpriseId) params.enterprise_id = this.fEnterpriseId
          if (this.fCategoryId) params.category_id = this.fCategoryId
          if (this.fPropertyId) params.property_id = this.fPropertyId
          if (this.fReporterId) params.reporter_id = this.fReporterId
          if (this.fUrgency) params.urgency = this.fUrgency
          if (this.fDateFrom) params.date_from = this.fDateFrom
          if (this.fDateTo) params.date_to = this.fDateTo
          if (this.fKeyword) params.keyword = this.fKeyword
          // 按提交时间排序（顶部「按时间」按钮切换方向，默认新→旧）
          params.sort_by = 'submitted_at'
          params.sort_order = this.timeSort

          const data = await adminAPI.getOrders(params)
          this.statusCounts = (data && (data as any).status_counts) || {}
          this.applyPage(data, reset)
        } else {
          const opt = this.enterpriseFilterOptions[this.enterpriseFilterIndex]
          if (opt && opt.id) params.enterprise_id = opt.id
          const data = await orderAPI.list(params)
          this.applyPage(data, reset)
        }
      } catch (e) {
        console.error('加载工单失败', e)
      } finally {
        this.loading = false
      }
    },
    applyPage(data: any, reset = false) {
      const res = normalizePage(data)
      this.page = res.page
      this.totalPages = res.total_pages
      this.finished = this.page >= this.totalPages
      this.list = reset ? res.list : [...this.list, ...res.list]
    },

    /* ==================== 管理模式：下拉数据源 ==================== */

    async ensureManageOptions() {
      if (this.enterpriseOptions.length) {
        if (!this.categoryOptions.length) await this.loadCategoryOptions()
        return
      }
      // ⚠️ /admin/enterprises 要求 role>=1，纯审核员调用会 403，
      // 因此审核员一律用 userInfo 里已下发的企业清单。
      if (isStoreStaff()) {
        try {
          const res = await enterpriseAPI.list({ page: 1, page_size: 100 })
          this.enterpriseOptions = (res.list || []).map((e) => ({ id: e.id, name: e.name }))
        } catch (e) {
          this.enterpriseOptions = []
        }
      } else {
        this.enterpriseOptions = reviewerEnterprises().map((e) => ({
          id: e.enterprise_id,
          name: e.enterprise_name
        }))
      }
      await this.loadCategoryOptions()
    },
    /**
     * 顶部「用户」筛选的报修人候选：按当前企业范围拉取。
     * ⚠️ 后端 getReporters 对维修业务员 / 单位审核员强制要求 enterprise_id：
     *    - 全部企业（fEnterpriseId 为空）时若仍调用接口，后端会 403 返回「请先选择要查看的企业」；
     *    - 且 uni.request 会把值为 undefined 的键序列化成字面量 "undefined"，后端按 uuid 解析报 22P02 → 500「数据库操作失败」。
     * 因此：企业为空时直接不调接口（候选仅留「全部用户」），彻底规避两类报错。
     */
    async loadReporterOptions() {
      if (!this.manageMode) return
      if (!this.fEnterpriseId) {
        this.reporterOptions = []
        return
      }
      try {
        const res = await adminAPI.getReporters({ enterprise_id: this.fEnterpriseId })
        this.reporterOptions = (res && (res as any).list) || []
      } catch (e) {
        this.reporterOptions = []
      }
    },
    async loadCategoryOptions() {
      try {
        const res = await orderAPI.options()
        this.categoryOptions = res.categories || []
      } catch (e) {
        this.categoryOptions = []
      }
    },

    /* ==================== 管理模式：筛选抽屉 ==================== */

    syncDrawerFromFilter() {
      this.fcCatIndex = this.categoryOptions.findIndex((x) => x.id === this.fCategoryId)
      this.fcPropIndex = this.propertyOptions.findIndex((x) => x.id === this.fPropertyId)
      this.fcUrgencyIndex = this.urgencyKeys.indexOf(this.fUrgency)
      this.fcDateFrom = this.fDateFrom
      this.fcDateTo = this.fDateTo
      this.fcKeyword = this.fKeyword
    },
    /** 打开抽屉时同步一次临时值 */
    onOpenDrawer() {
      this.syncDrawerFromFilter()
      this.showFilterDrawer = true
    },
    /* ==================== 管理模式：顶部 企业 / 用户 / 时间排序 ==================== */

    onScopeEntChange(e: { detail: { value: number } }) {
      const idx = Number(e.detail.value) || 0
      this.fEnterpriseId = this.entScopeOptions[idx]?.id || ''
      // 切换企业清空已选用户并重新拉取候选
      this.fReporterId = ''
      this.loadReporterOptions()
      this.loadList(true)
    },
    onScopeReporterChange(e: { detail: { value: number } }) {
      const idx = Number(e.detail.value) || 0
      this.fReporterId = this.reporterScopeOptions[idx]?.id || ''
      this.loadList(true)
    },
    toggleTimeSort() {
      this.timeSort = this.timeSort === 'desc' ? 'asc' : 'desc'
      this.loadList(true)
    },
    onCatChange(e: { detail: { value: number } }) {
      const v = Number(e.detail.value)
      this.fcCatIndex = this.fcCatIndex === v ? -1 : v
      this.fcPropIndex = -1
    },
    onPropChange(e: { detail: { value: number } }) {
      this.fcPropIndex = Number(e.detail.value) >= 0 ? Number(e.detail.value) : -1
    },
    onUrgencyChange(e: { detail: { value: number } }) {
      const v = Number(e.detail.value)
      this.fcUrgencyIndex = this.fcUrgencyIndex === v ? -1 : v
    },
    onDateFromChange(e: { detail: { value: string } }) {
      this.fcDateFrom = e.detail.value || ''
    },
    onDateToChange(e: { detail: { value: string } }) {
      this.fcDateTo = e.detail.value || ''
    },
    applyFilter() {
      // 企业 / 用户由顶部筛选条控制，不在抽屉内
      this.fCategoryId = this.fcCatIndex >= 0 ? this.categoryOptions[this.fcCatIndex].id : ''
      this.fPropertyId = this.fcPropIndex >= 0 ? this.propertyOptions[this.fcPropIndex]?.id || '' : ''
      this.fUrgency = this.fcUrgencyIndex >= 0 ? this.urgencyKeys[this.fcUrgencyIndex] : ''
      this.fDateFrom = this.fcDateFrom
      this.fDateTo = this.fcDateTo
      this.fKeyword = (this.fcKeyword || '').trim()
      this.showFilterDrawer = false
      this.loadList(true)
    },
    resetFilter() {
      // 仅重置抽屉内筛选（企业 / 用户已由顶部筛选条常驻控制）
      this.fcCatIndex = -1
      this.fcPropIndex = -1
      this.fcUrgencyIndex = -1
      this.fcDateFrom = ''
      this.fcDateTo = ''
      this.fcKeyword = ''
      this.fCategoryId = ''
      this.fPropertyId = ''
      this.fUrgency = ''
      this.fDateFrom = ''
      this.fDateTo = ''
      this.fKeyword = ''
      this.showFilterDrawer = false
      this.loadList(true)
    },
    clearAllFilters() {
      if (this.manageMode) this.resetFilter()
    },

    /* ==================== 导航 ==================== */

    goLogin() {
      uni.navigateTo({ url: '/pages/auth/login' })
    },
    goOrderDetail(orderId: string) {
      const url = this.manageMode
        ? `/pages/admin/order/detail?id=${orderId}`
        : `/pages/order/detail?id=${orderId}`
      uni.navigateTo({ url })
    },
    stopCardClick() {
      // no-op：仅拦截冒泡
    },
    createOrder() {
      uni.navigateTo({ url: '/pages/order/edit' })
    },
    editOrder(orderId: string) {
      uni.navigateTo({ url: `/pages/order/edit?id=${orderId}` })
    },

    /* ==================== 普通模式：卡片操作 ==================== */

    submitDraft(item: any) {
      const that = this
      uni.showModal({
        title: '提交报修',
        content: '提交后工单将上报本单位审核员审核，确认提交？',
        success: async (res) => {
          if (!res.confirm) return
          try {
            await orderAPI.submit(item.id)
            uni.showToast({ title: '提交成功', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            /* 后端错误已 Toast */
          }
        }
      })
    },
    cancelOrder(item: any) {
      this.cancelTargetId = item.id
      this.cancelReason = ''
      this.showCancelPopup = true
    },
    async confirmCancel() {
      const reason = (this.cancelReason || '').trim()
      if (!reason) {
        uni.showToast({ title: '请填写取消原因', icon: 'none' })
        return
      }
      if (this.cancelSubmitting) return
      this.cancelSubmitting = true
      try {
        await orderAPI.cancel(this.cancelTargetId, { reason })
        uni.showToast({ title: '已取消', icon: 'success' })
        this.showCancelPopup = false
        this.loadList(true)
      } catch (e) {
        console.error('取消失败', e)
      } finally {
        this.cancelSubmitting = false
      }
    },
    deleteOrder(item: any) {
      const that = this
      uni.showModal({
        title: '删除草稿',
        content: '确定删除该草稿吗？删除后不可恢复',
        confirmColor: '#dc2626',
        success: async (res) => {
          if (!res.confirm) return
          try {
            await orderAPI.remove(item.id)
            uni.showToast({ title: '已删除', icon: 'success' })
            that.loadList(true)
          } catch (e) {
            console.error('删除失败', e)
          }
        }
      })
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
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding-bottom: 40rpx;
  background-color: var(--xn-bg);
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 24rpx;
  background-color: var(--xn-surface);
  border-bottom: 1rpx solid var(--xn-border-soft);
  position: sticky;
  top: 0;
  z-index: 20;
}
.toolbar-title {
  font-size: 32rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.toolbar-right {
  display: flex;
  align-items: center;
}
.tb-filter {
  display: flex;
  align-items: center;
  margin-right: 24rpx;
  padding: 8rpx 20rpx;
  border-radius: 999rpx;
  background: var(--xn-tag-bg);
  font-size: 26rpx;
  color: var(--xn-text-700);
}
.tb-filter.on {
  background: var(--xn-primary-soft);
  color: var(--xn-primary);
}
.tb-filter-badge {
  margin-left: 8rpx;
  min-width: 28rpx;
  padding: 0 8rpx;
  border-radius: 999rpx;
  background: var(--xn-primary);
  color: #fff;
  font-size: 20rpx;
  text-align: center;
}

/* 管理模式：横向滚动状态 Tab + 角标 */
.seg {
  background-color: var(--xn-surface);
  border-bottom: 1rpx solid var(--xn-border-soft);
  white-space: nowrap;
}
.seg-inner {
  display: inline-flex;
  padding: 16rpx 24rpx;
}
.seg-btn {
  display: inline-flex;
  align-items: center;
  padding: 12rpx 28rpx;
  margin-right: 16rpx;
  border-radius: var(--xn-radius-sm);
  background: var(--xn-tag-bg);
  flex-shrink: 0;
}
.seg-btn.active {
  background: var(--xn-primary);
}
.seg-txt {
  font-size: 26rpx;
  color: var(--xn-text-700);
}
.seg-btn.active .seg-txt {
  color: #fff;
  font-weight: 500;
}
.seg-count {
  margin-left: 8rpx;
  font-size: 22rpx;
  color: var(--xn-text-500);
}
.seg-btn.active .seg-count {
  color: rgba(255, 255, 255, 0.85);
}

/* 管理模式：状态 Tab 下方的 企业 / 用户 / 时间排序 常驻筛选条 */
.filter-scope {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 16rpx 24rpx;
  background-color: var(--xn-surface);
  border-bottom: 1rpx solid var(--xn-border-soft);
  flex-wrap: wrap;
}
.scope-picker {
  display: inline-flex;
  align-items: center;
  max-width: 280rpx;
  padding: 12rpx 20rpx;
  background: var(--xn-tag-bg);
  border-radius: 999rpx;
}
.scope-label {
  font-size: 24rpx;
  color: var(--xn-text-400);
  margin-right: 12rpx;
  flex-shrink: 0;
}
.scope-value {
  font-size: 26rpx;
  color: var(--xn-text-900);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.scope-arrow {
  margin-left: 8rpx;
  font-size: 22rpx;
  color: var(--xn-text-400);
}
/* 全部企业时「用户」筛选不可用：灰显 + 提示「请先选企业」 */
.scope-picker.is-disabled {
  background: var(--xn-bg-soft);
  opacity: 0.55;
}
.scope-sort {
  display: inline-flex;
  align-items: center;
  margin-left: auto;
  padding: 12rpx 22rpx;
  border-radius: 999rpx;
  background: var(--xn-tag-bg);
  font-size: 26rpx;
  color: var(--xn-text-700);
}
.scope-sort.on {
  background: var(--xn-primary-soft);
  color: var(--xn-primary);
}
.scope-sort-arrow {
  margin-left: 8rpx;
  font-weight: 600;
}

/* 面包屑 */
.crumbs {
  background-color: var(--xn-surface);
  padding: 16rpx 24rpx;
  white-space: nowrap;
}
.crumb {
  display: inline-flex;
  align-items: center;
  padding: 8rpx 18rpx;
  margin-right: 16rpx;
  border-radius: 999rpx;
  background: var(--xn-primary-soft);
  flex-shrink: 0;
}
.crumb-txt {
  font-size: 24rpx;
  color: var(--xn-primary);
}
.crumb-x {
  margin-left: 8rpx;
  font-size: 26rpx;
  color: var(--xn-primary);
}

.filter-bar {
  padding: 16rpx 24rpx;
  background-color: var(--xn-surface);
  border-bottom: 1rpx solid var(--xn-border-soft);
}
.filter-picker {
  display: inline-flex;
  align-items: center;
  background-color: var(--xn-tag-bg);
  padding: 10rpx 20rpx;
  border-radius: 999rpx;
  max-width: 320rpx;
}
.filter-label {
  font-size: 24rpx;
  color: var(--xn-text-400);
  margin-right: 12rpx;
}
.filter-value {
  font-size: 26rpx;
  color: var(--xn-text-900);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.filter-arrow {
  margin-left: 8rpx;
  font-size: 22rpx;
  color: var(--xn-text-400);
}

.tabs-wrap {
  background-color: var(--xn-surface);
  padding: 0 24rpx;
}

.order-list {
  padding: 20rpx 24rpx;

  .reject-bar {
    margin-top: 16rpx;
    background-color: #fef3c7;
    color: #92400e;
    font-size: 24rpx;
    padding: 10rpx 16rpx;
    border-radius: 10rpx;
    line-height: 1.5;
  }
  .amount-line {
    margin-top: 16rpx;
    font-size: 28rpx;
    font-weight: 600;
    color: var(--xn-primary);
  }
  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 24rpx;
  }
}

.empty,
.not-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 140rpx;
}
.empty-text {
  margin-top: 24rpx;
  font-size: 30rpx;
  color: var(--xn-text-700);
}
.empty-tip {
  margin-top: 12rpx;
  margin-bottom: 32rpx;
  font-size: 24rpx;
  color: var(--xn-text-400);
}

.load-more {
  padding: 24rpx 0 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: var(--xn-text-400);
}

/* 筛选抽屉 */
.drawer {
  padding: 32rpx 32rpx 0;
  background-color: var(--xn-surface);
}
.drawer-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12rpx;
}
.drawer-title {
  font-size: 32rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.drawer-reset {
  font-size: 26rpx;
  color: var(--xn-primary);
}
.drawer-item {
  padding: 22rpx 0;
  border-bottom: 1rpx solid var(--xn-border-soft);
}
.drawer-label {
  display: block;
  margin-bottom: 14rpx;
  font-size: 26rpx;
  color: var(--xn-text-500);
}
.drawer-picker {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16rpx 20rpx;
  background: var(--xn-bg);
  border-radius: var(--xn-radius-sm);
  font-size: 28rpx;
  color: var(--xn-text-900);
}
.drawer-picker.off {
  color: var(--xn-text-400);
}
.drawer-arrow {
  font-size: 22rpx;
  color: var(--xn-text-400);
}
.drawer-range {
  display: flex;
  align-items: center;
}
.drawer-date {
  flex: 1;
  padding: 16rpx 20rpx;
  background: var(--xn-bg);
  border-radius: var(--xn-radius-sm);
  font-size: 28rpx;
  color: var(--xn-text-900);
  text-align: center;
}
.drawer-sep {
  padding: 0 20rpx;
  color: var(--xn-text-400);
  font-size: 26rpx;
}
.drawer-input {
  padding: 16rpx 20rpx;
  background: var(--xn-bg);
  border-radius: var(--xn-radius-sm);
  font-size: 28rpx;
  color: var(--xn-text-900);
}
.drawer-actions {
  display: flex;
  padding: 28rpx 0 40rpx;
  gap: 20rpx;
}

/* 弹窗 */
.popup-body {
  padding: 36rpx 32rpx 28rpx;
  background-color: var(--xn-surface);
  border-radius: 20rpx;
}
.popup-title {
  font-size: 32rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.popup-label {
  margin-top: 24rpx;
  font-size: 26rpx;
  color: var(--xn-text-700);
}
.popup-label-req {
  color: var(--xn-st-red);
}
.popup-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 28rpx;
}
</style>
