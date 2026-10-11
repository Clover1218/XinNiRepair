<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { OfficeBuilding } from '@element-plus/icons-vue'
import OrderCardC from '@/components/OrderCardC.vue'
import { adminAPI, fetchOrderOptions, type OrderListParams } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import { useOrderListQueryStore } from '@/stores/orderListQuery'
import type {
  CategoryOption,
  EnterpriseListItem,
  OrderListItem,
  ReporterOption,
  Urgency
} from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

// 既非店方也非审核员：不允许停留本页
if (!userStore.isStoreStaff && !userStore.hasReviewerRole) {
  router.replace('/')
}

// ─────────────────────────────────────────────────────────
// 角色化状态 Tab
// 同一套状态、只换标签，不是两套数据：pending_accept 对维修员是「待接单」
// （要去接的活），对审核员是「已审核通过」（已批过的单）。行级状态胶囊
// 仍显示真实状态，因此不会误导。
// ─────────────────────────────────────────────────────────
interface StatusTab {
  label: string
  value: string
}
/** 「未完成」= 还没干完的单：待审核 + 待接单 + 处理中（按约定排除「已取消」） */
const UNFINISHED = 'reported,pending_accept,processing'

const storeTabs = (): StatusTab[] => [
  { label: '全部', value: '' },
  { label: '待接单', value: 'pending_accept' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' },
  { label: '未完成', value: UNFINISHED }
]

const reviewerTabs = (): StatusTab[] => [
  { label: '全部', value: '' },
  { label: '待审核', value: 'reported' },
  { label: '已审核通过', value: 'pending_accept' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' },
  { label: '未完成', value: UNFINISHED }
]

/** 兼任多角色时 Tab 取并集：以店方为主线，补入仅审核员才有的「待审核」 */
const statusTabs = computed<StatusTab[]>(() => {
  if (userStore.isStoreStaff && userStore.hasReviewerRole) {
    const tabs = storeTabs()
    tabs.splice(1, 0, { label: '待审核', value: 'reported' })
    return tabs
  }
  return userStore.isStoreStaff ? storeTabs() : reviewerTabs()
})

/** 该角色的默认 Tab：进来就落在"该动手"的那一栏 */
const defaultStatus = computed(() => {
  if (userStore.hasReviewerRole) return 'reported'
  if (userStore.isStoreStaff) return 'pending_accept'
  return ''
})

/** 纯审核员（非店方）：后端强制限定本单位，前端必须选单位 */
const reviewerOnly = computed(() => !userStore.isStoreStaff && userStore.hasReviewerRole)
const reviewerUnits = computed(() => userStore.reviewerEnterprises)
/** 多单位时出现「切换单位」信息提示 */
const showUnitSwitch = computed(() => reviewerOnly.value && reviewerUnits.value.length > 1)
const activeUnitId = ref('')

// 视图模式：C=紧凑卡片 / D=合并表格，可切换并记忆
type OrderViewMode = 'card' | 'table'
const VIEW_MODE_KEY = 'xn_order_list_view_mode'
const viewMode = ref<OrderViewMode>(
  (localStorage.getItem(VIEW_MODE_KEY) as OrderViewMode) || 'table'
)
watch(viewMode, v => localStorage.setItem(VIEW_MODE_KEY, v))

// 放大后的状态展示（颜色 + 文案），含 V1.5 本质标签（待补签等）
const statusClassMap: Record<string, string> = {
  draft: 'st-gray',
  reported: 'st-red',
  pending_accept: 'st-orange',
  processing: 'st-blue',
  completed: 'st-green',
  cancelled: 'st-gray',
  rejected: 'st-red'
}
function statusView(row: OrderListItem) {
  if (row.signature_status === 'pending_sign') {
    return { text: '待补签', cls: 'st-pending-sign' }
  }
  return {
    text: row.display_tag || row.status_label,
    cls: statusClassMap[row.status] || 'st-gray'
  }
}

// 紧急程度标识
const urgencyMap: Record<Urgency, { label: string; color: string; dots: number }> = {
  normal: { label: '普通', color: '#409eff', dots: 1 },
  urgent: { label: '紧急', color: '#e6a23c', dots: 2 },
  very_urgent: { label: '非常紧急', color: '#f56c6c', dots: 3 }
}

// 筛选/分页/排序会话态（存 Pinia，返回本页不重置）
const queryStore = useOrderListQueryStore()
const {
  activeStatus,
  enterpriseId,
  categoryId,
  propertyId,
  reporterId,
  keyword,
  urgency,
  dateRange,
  sortBy,
  sortOrder,
  essenceFilter,
  page,
  pageSize
} = storeToRefs(queryStore)

const reporterName = ref(queryStore.reporterName)

const loading = ref(false)
const list = ref<OrderListItem[]>([])
const total = ref(0)

// ── URL 承载筛选：首页区块「详情 →」带 ?status=xxx 直达此处，Tab 自动选中 ──
/** 把当前 Tab（及审核员单位）写回 URL，刷新/分享/后退都能停在同一个 Tab */
const syncUrl = () => {
  const query: Record<string, string> = {}
  if (activeStatus.value) query.status = activeStatus.value
  if (reviewerOnly.value && activeUnitId.value) query.enterprise = activeUnitId.value
  router.replace({ name: 'OrderList', query })
}

/** 初始化顺序：URL 参数 > 会话记忆 > 角色默认 Tab */
const initFromQuery = () => {
  if (reviewerOnly.value) {
    const qEnt = typeof route.query.enterprise === 'string' ? route.query.enterprise : ''
    activeUnitId.value = reviewerUnits.value.some(u => u.enterprise_id === qEnt)
      ? qEnt
      : reviewerUnits.value[0]?.enterprise_id || ''
  }
  const qStatus = typeof route.query.status === 'string' ? route.query.status : undefined
  if (qStatus !== undefined && statusTabs.value.some(t => t.value === qStatus)) {
    activeStatus.value = qStatus
    queryStore.touched = true
  } else if (!queryStore.touched) {
    activeStatus.value = defaultStatus.value
  }
}

// 项目大类选项（完整保留嵌套属性，供"项目属性"筛选联动）
const categoryOptions = ref<CategoryOption[]>([])
/** 当前大类下的项目属性选项 */
const propertyOptions = computed(() => {
  if (!categoryId.value) return []
  const c = categoryOptions.value.find(x => x.id === categoryId.value)
  return (c?.properties || []).map(p => ({ id: p.id, name: p.name }))
})

// 报修人筛选（远程搜索，来源 /admin/orders/reporters）
const reporterOptions = ref<ReporterOption[]>([])
const reporterSearching = ref(false)
let reporterTimer: ReturnType<typeof setTimeout> | undefined

const fetchList = async () => {
  loading.value = true
  try {
    const params: OrderListParams = {
      page: page.value,
      page_size: pageSize.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value
    }
    if (activeStatus.value) params.status = activeStatus.value
    // 纯审核员走"切换单位"，其余走搜索栏的企业下拉
    const entId = reviewerOnly.value ? activeUnitId.value : enterpriseId.value
    if (entId) params.enterprise_id = entId
    if (categoryId.value) params.category_id = categoryId.value
    if (propertyId.value) params.property_id = propertyId.value
    if (reporterId.value) params.reporter_id = reporterId.value
    if (keyword.value.trim()) params.keyword = keyword.value.trim()
    if (urgency.value) params.urgency = urgency.value
    // V1.5 本质标签筛选：映射到后端派生字段（finish_outcome / signature_status）
    if (essenceFilter.value === 'unrepairable' || essenceFilter.value === 'customer_cancelled') {
      params.finish_outcome = essenceFilter.value
    } else if (essenceFilter.value === 'pending_sign' || essenceFilter.value === 'signed') {
      params.signature_status = essenceFilter.value
    }
    if (dateRange.value) {
      params.date_from = `${dateRange.value[0]}T00:00:00+08:00`
      params.date_to = `${dateRange.value[1]}T23:59:59+08:00`
    }
    const res = await adminAPI.getOrders(params)
    list.value = res.data.list
    total.value = res.data.total
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  page.value = 1
  fetchList()
}

/** 企业变化：报修人候选随企业域变化，清空已选报修人 */
const handleEnterpriseChange = () => {
  reporterId.value = ''
  reporterName.value = ''
  reporterOptions.value = []
  handleSearch()
}

/** 大类变化：清空已选属性再查 */
const handleCategoryChange = () => {
  propertyId.value = ''
  handleSearch()
}

const handleReset = () => {
  queryStore.reset()
  reporterName.value = ''
  reporterOptions.value = []
  // 重置后回到"该动手"的角色默认 Tab，而不是「全部」
  activeStatus.value = defaultStatus.value
  syncUrl()
  fetchList()
}

const handleTabChange = () => {
  page.value = 1
  queryStore.touched = true
  syncUrl()
  fetchList()
}

/** 审核员切换单位：重置分页并回写 URL */
const handleUnitChange = () => {
  page.value = 1
  syncUrl()
  fetchList()
}

// 表格列排序（sortable="custom"，服务端排序）
const handleSortChange = ({
  prop,
  order
}: {
  prop: string
  order: 'ascending' | 'descending' | null
}) => {
  if (order === null) {
    // 取消排序：恢复默认（提交时间倒序）
    sortBy.value = 'submitted_at'
    sortOrder.value = 'desc'
  } else {
    sortBy.value = prop || 'submitted_at'
    sortOrder.value = order === 'ascending' ? 'asc' : 'desc'
  }
  page.value = 1
  fetchList()
}

const handlePageChange = (p: number) => {
  page.value = p
  fetchList()
}

const handleSizeChange = (s: number) => {
  pageSize.value = s
  page.value = 1
  fetchList()
}

const goDetail = (row: OrderListItem) => {
  router.push(`/orders/${row.id}`)
}

// ---- 导出工单（5.14，仅店方） ----
const exportDialogVisible = ref(false)
const exportMode = ref<'enterprise' | 'repairer'>('enterprise')
const exportEnterpriseId = ref('')
const exportRepairerId = ref('')
const exportDateRange = ref<[string, string] | null>(null)
const exportFields = ref<string[]>(['order_no', 'time', 'content', 'quantity', 'unit_price', 'amount', 'remark'])
const exporting = ref(false)
const enterpriseOptions = ref<EnterpriseListItem[]>([])
// 维修员(业务员)选项（5.15：导出弹窗业务员模式下拉）
const repairerOptions = ref<Array<{ id: string; nickname: string; avatar_url: string }>>([])

// 加载企业列表（搜索栏企业下拉 / 导出弹窗共用）
const loadEnterpriseOptions = async () => {
  if (enterpriseOptions.value.length) return
  const res = await adminAPI.getEnterprises({ page: 1, page_size: 100 })
  enterpriseOptions.value = res.data.list
}

// 加载项目大类选项（保留嵌套属性，供项目属性筛选联动）
const loadCategoryOptions = async () => {
  if (categoryOptions.value.length) return
  const res = await fetchOrderOptions()
  categoryOptions.value = res.data.categories
}

// 报修人筛选：远程按昵称关键字搜索候选（企业域随当前企业筛选）
const onReporterRemote = (q: string) => {
  if (reporterTimer) clearTimeout(reporterTimer)
  reporterTimer = setTimeout(async () => {
    reporterSearching.value = true
    try {
      const res = await adminAPI.getReporters({
        ...(enterpriseId.value ? { enterprise_id: enterpriseId.value } : {}),
        ...(q?.trim() ? { keyword: q.trim() } : {})
      })
      reporterOptions.value = res.data.list
    } finally {
      reporterSearching.value = false
    }
  }, 300)
}

const handleReporterChange = (val: string) => {
  const hit = reporterOptions.value.find(o => o.id === val)
  reporterName.value = hit?.nickname || ''
  handleSearch()
}

// 加载维修员列表（5.15：导出弹窗业务员模式下拉）
const loadRepairerOptions = async () => {
  if (repairerOptions.value.length) return
  const res = await adminAPI.getRepairers()
  repairerOptions.value = res.data.list
}

// 导出可选字段（5.14 可用字段表）
const exportFieldOptions = [
  { value: 'order_no', label: '工单号' },
  { value: 'time', label: '日期' },
  { value: 'content', label: '内容' },
  { value: 'quantity', label: '数量' },
  { value: 'unit_price', label: '单价' },
  { value: 'amount', label: '金额' },
  { value: 'remark', label: '备注' },
  { value: 'repair_result', label: '维修结果' },
  { value: 'repair_method', label: '维修方式' },
  { value: 'warranty_period', label: '保修期' },
  { value: 'repair_duration', label: '维修时长' },
  { value: 'reporter', label: '报修人' },
  { value: 'repairer', label: '维修员' },
  { value: 'room', label: '位置' },
  { value: 'contact', label: '联系方式' },
  { value: 'enterprise_name', label: '客户名称' }
]

const openExportDialog = async () => {
  exportMode.value = 'enterprise'
  exportEnterpriseId.value = ''
  exportRepairerId.value = ''
  exportDateRange.value = null
  exportFields.value = ['order_no', 'time', 'content', 'quantity', 'unit_price', 'amount', 'remark']
  await loadEnterpriseOptions()
  await loadRepairerOptions()
  exportDialogVisible.value = true
}

const handleExport = async () => {
  if (!exportDateRange.value) {
    ElMessage.warning('请选择日期范围')
    return
  }
  if (exportMode.value === 'enterprise' && !exportEnterpriseId.value) {
    ElMessage.warning('请选择企业')
    return
  }
  if (exportMode.value === 'repairer' && !exportRepairerId.value) {
    ElMessage.warning('请选择维修员')
    return
  }
  if (!exportFields.value.length) {
    ElMessage.warning('请至少选择一个导出字段')
    return
  }
  exporting.value = true
  try {
    const res = await adminAPI.exportOrders({
      mode: exportMode.value,
      enterprise_id: exportMode.value === 'enterprise' ? exportEnterpriseId.value : undefined,
      repairer_id: exportMode.value === 'repairer' ? exportRepairerId.value : undefined,
      date_from: exportDateRange.value[0],
      date_to: exportDateRange.value[1],
      fields: exportFields.value
    })
    const blob = res.data as Blob
    // 解析 Content-Disposition 中的 filename（后端用 mime.FormatMediaType，
    // 中文文件名走 RFC 5987 形式 filename*=utf-8''...，需优先匹配）
    const disposition = res.headers['content-disposition'] as string | undefined
    let filename = '工单导出.xlsx'
    if (disposition) {
      const starMatch = disposition.match(/filename\*=utf-8''([^;]+)/i)
      if (starMatch) {
        filename = decodeURIComponent(starMatch[1].trim())
      } else {
        const plainMatch = disposition.match(/filename="?([^";]+)"?/i)
        if (plainMatch) filename = plainMatch[1].trim()
      }
    }
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
    exportDialogVisible.value = false
  } finally {
    exporting.value = false
  }
}

// ---- 待审核行就地快操作：审核通过 / 退回（与 /review 同源逻辑，省去进详情页） ----
const auditingId = ref('')

const handleAudit = async (row: OrderListItem) => {
  try {
    await ElMessageBox.confirm('审核通过后工单进入待接单并上报维修业务员，确认？', '审核通过', {
      type: 'warning',
      confirmButtonText: '审核通过',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  auditingId.value = row.id
  try {
    await adminAPI.auditOrder(row.id)
    ElMessage.success('已审核通过并上报维修业务员')
    fetchList()
  } finally {
    auditingId.value = ''
  }
}

const rejectDialogVisible = ref(false)
const rejectOrderId = ref('')
const rejectReason = ref('')
const rejecting = ref(false)

const openReject = (row: OrderListItem) => {
  rejectOrderId.value = row.id
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

const handleRejectSubmit = async () => {
  const reason = rejectReason.value.trim()
  if (reason.length < 2) {
    ElMessage.warning('退回原因至少 2 个字')
    return
  }
  if (reason.length > 200) {
    ElMessage.warning('退回原因不能超过 200 字')
    return
  }
  rejecting.value = true
  try {
    await adminAPI.rejectOrder(rejectOrderId.value, reason)
    ElMessage.success('已退回（报修人可修改后重新提交）')
    rejectDialogVisible.value = false
    fetchList()
  } finally {
    rejecting.value = false
  }
}

onMounted(() => {
  // 从会话态恢复"报修人"选中项的回显（远程选项列表已随页面卸载清空）
  const rid = queryStore.reporterId
  const rname = queryStore.reporterName
  if (rid && rname && !reporterOptions.value.some(o => o.id === rid)) {
    reporterOptions.value = [{ id: rid, nickname: rname, avatar_url: null }]
  }
  initFromQuery()
  fetchList()
  loadEnterpriseOptions()
  loadCategoryOptions()
})
</script>

<template>
  <div>
    <el-card shadow="never" class="filter-card">
      <!-- 多单位时：出现「切换单位」信息提示 -->
      <div v-if="showUnitSwitch" class="unit-bar">
        <el-icon class="unit-icon"><Switch /></el-icon>
        <span class="unit-hint">
          您担任 {{ reviewerUnits.length }} 个单位的审核员，请切换单位查看对应工单
        </span>
        <el-select
          v-model="activeUnitId"
          size="small"
          class="unit-select"
          @change="handleUnitChange"
        >
          <el-option
            v-for="u in reviewerUnits"
            :key="u.enterprise_id"
            :label="u.enterprise_name"
            :value="u.enterprise_id"
          />
        </el-select>
      </div>
      <div v-else-if="reviewerOnly" class="unit-bar unit-bar--plain">
        <el-icon class="unit-icon"><OfficeBuilding /></el-icon>
        <span class="unit-hint">
          当前单位：{{ reviewerUnits[0]?.enterprise_name || '—' }}（仅显示本单位工单）
        </span>
      </div>

      <el-tabs v-model="activeStatus" @tab-change="handleTabChange">
        <el-tab-pane
          v-for="tab in statusTabs"
          :key="tab.value"
          :label="tab.label"
          :name="tab.value"
        />
      </el-tabs>

      <div class="filter-bar">
        <el-select
          v-if="!reviewerOnly"
          v-model="enterpriseId"
          placeholder="按企业搜索"
          clearable
          filterable
          class="filter-enterprise"
          @change="handleEnterpriseChange"
        >
          <el-option
            v-for="e in enterpriseOptions"
            :key="e.id"
            :label="e.name"
            :value="e.id"
          />
        </el-select>
        <el-select
          v-model="categoryId"
          placeholder="项目大类"
          clearable
          filterable
          class="filter-input"
          @change="handleCategoryChange"
        >
          <el-option
            v-for="c in categoryOptions"
            :key="c.id"
            :label="c.name"
            :value="c.id"
          />
        </el-select>
        <el-select
          v-model="propertyId"
          placeholder="问题类型（属性）"
          clearable
          filterable
          :disabled="!categoryId"
          class="filter-input"
          @change="handleSearch"
        >
          <el-option
            v-for="p in propertyOptions"
            :key="p.id"
            :label="p.name"
            :value="p.id"
          />
        </el-select>
        <el-select
          v-model="reporterId"
          placeholder="报修人"
          clearable
          filterable
          remote
          :remote-method="onReporterRemote"
          :loading="reporterSearching"
          class="filter-input"
          @change="handleReporterChange"
        >
          <el-option
            v-for="r in reporterOptions"
            :key="r.id"
            :label="r.nickname"
            :value="r.id"
          />
        </el-select>
        <el-input
          v-model="keyword"
          placeholder="工单号"
          clearable
          class="filter-input"
          @keyup.enter="handleSearch"
        />
        <el-select
          v-model="urgency"
          placeholder="紧急程度"
          clearable
          class="filter-urgency"
        >
          <el-option label="普通" value="normal" />
          <el-option label="紧急" value="urgent" />
          <el-option label="非常紧急" value="very_urgent" />
        </el-select>
        <el-select
          v-model="essenceFilter"
          placeholder="本质标签"
          clearable
          class="filter-input"
          @change="handleSearch"
        >
          <el-option label="不能修" value="unrepairable" />
          <el-option label="客户取消" value="customer_cancelled" />
          <el-option label="待补签" value="pending_sign" />
          <el-option label="已签字" value="signed" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          class="filter-date"
        />
        <el-button type="primary" @click="handleSearch">
          <el-icon><Search /></el-icon>
          搜索
        </el-button>
        <el-button @click="handleReset">重置</el-button>
        <div class="spacer" />
        <!-- 导出为店方专属能力（5.14），单位审核员不展示 -->
        <el-button v-if="userStore.isStoreStaff" type="primary" plain @click="openExportDialog">
          <el-icon><Download /></el-icon>
          导出
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <div class="list-toolbar">
        <span class="toolbar-title">工单列表</span>
        <el-radio-group v-model="viewMode" size="small">
          <el-radio-button value="card">卡片视图 (C)</el-radio-button>
          <el-radio-button value="table">表格视图 (D)</el-radio-button>
        </el-radio-group>
      </div>

      <el-table
        v-if="viewMode === 'table'"
        v-loading="loading"
        :data="list"
        stripe
        :default-sort="queryStore.defaultSort"
        @row-click="goDetail"
        @sort-change="handleSortChange"
      >
        <el-table-column prop="order_no" label="工单号" width="160" sortable="custom" />
        <el-table-column label="工单内容" min-width="280">
          <template #default="{ row }">
            <div class="cell-content">
              <div class="cc-cat">
                <span v-if="row.category_name">{{ row.category_name }}</span>
                <span v-if="row.property_name"> · {{ row.property_name }}</span>
              </div>
              <div class="cc-desc" :title="row.description">{{ row.description || '（无故障描述）' }}</div>
              <div v-if="row.amount && row.amount > 0" class="cc-amount">¥{{ Number(row.amount).toFixed(2) }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="reporter" label="报修人" width="150" sortable="custom">
          <template #default="{ row }">
            <div class="reporter">
              <el-avatar :size="24" :src="row.reporter.avatar_url">
                {{ row.reporter.nickname?.charAt(0) }}
              </el-avatar>
              <span>{{ row.reporter.nickname }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="enterprise_name" label="客户" width="150" show-overflow-tooltip sortable="custom" />
        <el-table-column prop="urgency" label="紧急度" width="120" sortable="custom">
          <template #default="{ row }">
            <span :style="{ color: urgencyMap[row.urgency as Urgency]?.color }">
              <span class="urgency-dots">
                <span
                  v-for="i in urgencyMap[row.urgency as Urgency]?.dots"
                  :key="i"
                  class="urgency-dot"
                  :style="{ backgroundColor: urgencyMap[row.urgency as Urgency]?.color }"
                />
              </span>
              {{ urgencyMap[row.urgency as Urgency]?.label }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="130" sortable="custom">
          <template #default="{ row }">
            <span class="status-pill" :class="statusView(row).cls">{{ statusView(row).text }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="submitted_at" label="提交时间" width="170" sortable="custom">
          <template #default="{ row }">
            {{ formatDateTime(row.submitted_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <!-- 待审核行就地快操作，无需先进详情 -->
            <template v-if="row.status === 'reported'">
              <el-button
                link
                type="success"
                :loading="auditingId === row.id"
                @click.stop="handleAudit(row)"
              >审核通过</el-button>
              <el-button link type="danger" @click.stop="openReject(row)">退回</el-button>
            </template>
            <el-button link type="primary" @click.stop="goDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- C 布局：紧凑扫读卡片（与首页共用 OrderCardC 组件，样式/图标对齐一致） -->
      <div v-else v-loading="loading" class="card-grid">
        <OrderCardC
          v-for="row in list"
          :key="row.id"
          :row="row"
          @click="goDetail(row)"
        />
        <div v-if="!loading && !list.length" class="card-empty">暂无工单</div>
      </div>

      <div class="pagination">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>

    <!-- 导出工单弹窗（5.14） -->
    <el-dialog v-model="exportDialogVisible" title="导出工单记录" width="560px">
      <el-form label-position="top">
        <el-form-item label="导出模式">
          <el-radio-group v-model="exportMode">
            <el-radio-button value="enterprise">企业对账单</el-radio-button>
            <el-radio-button value="repairer">业务员汇总</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="exportMode === 'enterprise'" label="企业（必填）">
          <el-select v-model="exportEnterpriseId" placeholder="请选择企业" filterable class="export-select">
            <el-option
              v-for="e in enterpriseOptions"
              :key="e.id"
              :label="e.name"
              :value="e.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item v-else label="维修员（必填）">
          <el-select v-model="exportRepairerId" placeholder="请选择维修员" filterable class="export-select">
            <el-option
              v-for="r in repairerOptions"
              :key="r.id"
              :label="r.nickname"
              :value="r.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="日期范围（必填）">
          <el-date-picker
            v-model="exportDateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            class="export-select"
          />
        </el-form-item>

        <el-form-item label="导出字段（默认 7 项全选）">
          <el-checkbox-group v-model="exportFields" class="field-group">
            <el-checkbox v-for="opt in exportFieldOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="exportDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="handleExport">
          确认导出
        </el-button>
      </template>
    </el-dialog>

    <!-- 退回工单弹窗（待审核行就地操作） -->
    <el-dialog v-model="rejectDialogVisible" title="退回工单" width="480px">
      <el-form label-position="top">
        <el-form-item label="退回原因（报修人将收到并修改后重新提交）">
          <el-input
            v-model="rejectReason"
            type="textarea"
            :rows="4"
            :maxlength="200"
            show-word-limit
            placeholder="请输入退回原因，至少 2 个字"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rejectDialogVisible = false">取消</el-button>
        <el-button type="danger" :loading="rejecting" @click="handleRejectSubmit">
          确认退回
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.filter-card {
  margin-bottom: 16px;
}

/* 多单位审核员：「切换单位」信息提示条 */
.unit-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px 12px;
  border: 1px solid #b3d8ff;
  border-radius: 8px;
  background-color: #ecf5ff;
}

.unit-bar--plain {
  border-color: #e4e7ed;
  background-color: #f5f7fa;
}

.unit-icon {
  color: #409eff;
  flex-shrink: 0;
}

.unit-bar--plain .unit-icon {
  color: #909399;
}

.unit-hint {
  font-size: 13px;
  color: #409eff;
}

.unit-bar--plain .unit-hint {
  color: #606266;
}

.unit-select {
  width: 280px;
  margin-left: auto;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 4px;
}

.filter-enterprise {
  width: 180px;
}

.filter-input {
  width: 170px;
}

.filter-urgency {
  width: 130px;
}

.filter-date {
  width: 300px;
}

.spacer {
  flex: 1;
}

.export-select {
  width: 100%;
}

.field-group {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
}

.reporter {
  display: flex;
  align-items: center;
  gap: 6px;
}

.urgency-dots {
  display: inline-flex;
  gap: 2px;
  margin-right: 4px;
}

.urgency-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.cell-muted {
  color: #c0c4cc;
}

:deep(.el-table__row) {
  cursor: pointer;
}

/* 工具栏：视图切换 */
.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.toolbar-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

/* D 布局：工单内容主列（项目 + 故障描述 + 金额） */
.cell-content {
  line-height: 1.5;
}

.cc-cat {
  font-size: 12px;
  color: #909399;
}

.cc-desc {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin: 2px 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.cc-amount {
  font-size: 14px;
  font-weight: 700;
  color: #f56c6c;
}

/* 状态放大胶囊（C / D 共用） */
.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  color: #fff;
  white-space: nowrap;
}

.st-red {
  background: #f56c6c;
}

.st-orange {
  background: #e6a23c;
}

.st-blue {
  background: #409eff;
}

.st-green {
  background: #67c23a;
}

.st-gray {
  background: #909399;
}

.st-pending-sign {
  background: linear-gradient(90deg, #ff8c1a, #ff6a00);
  box-shadow: 0 1px 4px rgba(255, 106, 0, 0.45);
  letter-spacing: 1px;
}

/* C 布局：卡片网格容器（卡片本体在 OrderCardC 组件内） */
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
  min-height: 120px;
}

.card-empty {
  grid-column: 1 / -1;
  text-align: center;
  color: #c0c4cc;
  padding: 40px 0;
}
</style>
