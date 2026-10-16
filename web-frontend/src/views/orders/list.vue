<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Download } from '@element-plus/icons-vue'
import OrderCardC from '@/components/OrderCardC.vue'
import { adminAPI, fetchOrderOptions, type OrderListParams } from '@/api/admin'
import { orderAPI, type UserOrderListParams } from '@/api/order'
import { useUserStore } from '@/stores/user'
import { useOrderListQueryStore } from '@/stores/orderListQuery'
import {
  ORDER_MODES,
  resolveMode,
  type OrderMode,
  type ModeTab
} from '@/order/modes'
import { TAG_STATUS_GROUPS } from '@/order/tags'
import type {
  CategoryOption,
  OrderListItem,
  ReporterOption,
  Urgency
} from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

// 仅店方/审核员可停留本页（纯普通用户不进后台，由路由守卫拦截）
if (!userStore.isStoreStaff && !userStore.hasReviewerRole) {
  router.replace('/')
}

// ─────────────────────────────────────────────────────────
// mode 驱动：my_repair(用户端) / review / worker_task(管理端)
// Tab 一律由 ORDER_MODES[mode].tabs 配置渲染，禁止按原始 status 硬编码。
// ─────────────────────────────────────────────────────────
const mode = ref<OrderMode>('my_repair')
const activeTabKey = ref<string>('')

/** 从 URL ?mode=&tag=&enterprise=&reporter= 解析并落定 mode / 当前 Tab；URL 优先于会话记忆 */
const initModeAndTab = () => {
  const qMode = typeof route.query.mode === 'string' ? route.query.mode : undefined
  mode.value = resolveMode(qMode, {
    isStoreStaff: userStore.isStoreStaff,
    hasReviewerRole: userStore.hasReviewerRole
  })
  const qEnt = typeof route.query.enterprise === 'string' ? route.query.enterprise : ''
  if (qEnt) enterpriseId.value = qEnt
  const qRep = typeof route.query.reporter === 'string' ? route.query.reporter : ''
  if (qRep) reporterId.value = qRep
  const qTag = typeof route.query.tag === 'string' ? route.query.tag : ''
  activeTabKey.value = tabs.value.some(t => t.key === qTag) ? qTag : tabs.value[0].key
}

const modeConfig = computed(() => ORDER_MODES[mode.value])
const tabs = computed<ModeTab[]>(() => modeConfig.value.tabs)
const isUserSource = computed(() => modeConfig.value.source === 'mine')
const activeTab = computed<ModeTab | undefined>(
  () => tabs.value.find(t => t.key === activeTabKey.value) || tabs.value[0]
)
/** 该 Tab 是否使用看板排序（状态优先级 + 更新时间倒序） */
const isBoardTab = computed(() => activeTab.value?.sort === 'board')

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

/**
 * 「按时间」控件状态：
 * - 看板 Tab（待审核 / 处理中 / 待接单 / 我的维修 …）排序由后端 board 优先级决定，
 *   时间排序不生效 → 置为静默态（title 提示「看板顺序」），点击无动作；
 * - 其余 Tab（结束 / 草稿 / 已退回 …）交由「按时间 ↑↓」控制提交时间正/倒序。
 */
const timeSortArrow = computed(() => (sortOrder.value === 'asc' ? '↑' : '↓'))
/** 「用户」筛选占位文案：未选企业时后端报修人接口会拒答，故灰显提示先选企业 */
const reporterPlaceholder = computed(() => (enterpriseId.value ? '全部用户' : '请先选择企业'))

const loading = ref(false)
const list = ref<OrderListItem[]>([])
const total = ref(0)
/** 各状态实时计数（角标用），来自 /admin/orders 响应的 status_counts；用户端无此字段则留空 */
const statusCounts = ref<Record<string, number>>({})
/** 各状态未读工单数（红点提醒用），来自列表响应的 unread_counts；对齐小程序提醒逻辑 */
const unreadCounts = ref<Record<string, number>>({})
/**
 * 维修业务员（worker_task）专属：「我接的单」视角下的稳定角标计数。
 * 后台 status_counts 会随当前 Tab 的 repairer_id 收敛而波动（repairer_id=me 时只数本人，
 * 无 repairer_id 时数全经营范围），导致「处理中 / 待补签」角标在切换 Tab 时忽有忽无。
 * 故单独用 repairer_id=me、不带 display_tag 的请求拉一份稳定计数，专供这两个 Tab 角标使用。
 */
const workerCounts = ref<Record<string, number>>({})
/** 详细筛选展开栏显隐（点击「详细筛选」在下一行展开，不再用右侧抽屉） */
const advVisible = ref(false)
/** 已启用的详细筛选项数量（用于按钮角标提示） */
const advActiveCount = computed(() => {
  let n = 0
  if (categoryId.value) n++
  if (propertyId.value) n++
  if (urgency.value) n++
  if (essenceFilter.value) n++
  if (keyword.value.trim()) n++
  if (dateRange.value) n++
  return n
})

/** 把当前 mode / Tab / 企业 / 报修人写回 URL，刷新/分享/后退都能停在同一个视图 */
const syncUrl = () => {
  const query: Record<string, string> = { mode: mode.value, tag: activeTabKey.value }
  if (enterpriseId.value) query.enterprise = enterpriseId.value
  if (reporterId.value) query.reporter = reporterId.value
  router.replace({ name: 'OrderList', query })
}

/** 当前页内状态是否已与 URL query 一致（一致则 watcher 无需重复拉取） */
const queryMatchesState = (q: Record<string, unknown>) =>
  q.mode === mode.value &&
  q.tag === activeTabKey.value &&
  (q.enterprise || '') === enterpriseId.value &&
  (q.reporter || '') === reporterId.value

/** 构造「我接的单」稳定角标计数请求参数（repairer_id=me，不带 display_tag，复用当前高级筛选） */
const buildWorkerCountParams = (): OrderListParams => {
  const p: OrderListParams = { page: 1, page_size: 1, repairer_id: 'me' }
  if (enterpriseId.value) p.enterprise_id = enterpriseId.value
  if (categoryId.value) p.category_id = categoryId.value
  if (propertyId.value) p.property_id = propertyId.value
  if (reporterId.value) p.reporter_id = reporterId.value
  if (keyword.value.trim()) p.keyword = keyword.value.trim()
  if (urgency.value) p.urgency = urgency.value
  if (essenceFilter.value === 'unrepairable' || essenceFilter.value === 'customer_cancelled') {
    p.finish_outcome = essenceFilter.value
  } else if (essenceFilter.value === 'pending_sign' || essenceFilter.value === 'signed') {
    p.signature_status = essenceFilter.value
  }
  if (dateRange.value) {
    p.date_from = `${dateRange.value[0]}T00:00:00+08:00`
    p.date_to = `${dateRange.value[1]}T23:59:59+08:00`
  }
  return p
}

const fetchList = async () => {
  loading.value = true
  try {
    const tab = activeTab.value
    if (isUserSource.value) {
      // 用户端 GET /orders：仅支持 display_tag + sort(''|'board') + enterprise_id
      const params: UserOrderListParams = {
        page: page.value,
        page_size: pageSize.value,
        sort: isBoardTab.value ? 'board' : ''
      }
      if (tab?.tag) params.display_tag = tab.tag
      if (enterpriseId.value) params.enterprise_id = enterpriseId.value
      // 其余筛选（分类/报修人/紧急度/关键字/日期）用户端不支持，一律忽略
      const res = await orderAPI.list(params)
      list.value = res.data.list
      // 用户端 /orders 的总数在 pagination.total（与管理端 /admin/orders 的顶层 total 不同）
      total.value = res.data.pagination?.total ?? 0
      statusCounts.value = {} // 用户端不返回 status_counts
      unreadCounts.value = (res.data.unread_counts as Record<string, number>) || {}
    } else {
      // 管理端 GET /admin/orders：支持 display_tag / repairer_id=me / sort_by / 全部筛选
      const params: OrderListParams = {
        page: page.value,
        page_size: pageSize.value
      }
      if (tab?.tag) params.display_tag = tab.tag
      if (tab?.repairerMe) params.repairer_id = 'me'
      if (isBoardTab.value) {
        // 看板类 Tab：状态优先级 + 更新时间（与首页看板一致）
        params.sort_by = 'board'
        params.sort_order = 'asc'
      } else {
        // 其余 Tab：提交时间排序，方向由「按时间 ↑↓」控制
        params.sort_by = sortBy.value || 'submitted_at'
        params.sort_order = sortOrder.value
      }
      if (enterpriseId.value) params.enterprise_id = enterpriseId.value
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
      // 维修业务员：并行拉一份「我接的单」稳定计数，专供处理中/待补签角标（不受当前 Tab 的 repairer_id 收敛影响）
      const cntReq =
        mode.value === 'worker_task' ? adminAPI.getOrders(buildWorkerCountParams()) : null
      const res = await adminAPI.getOrders(params)
      list.value = res.data.list
      total.value = res.data.total
      statusCounts.value = (res.data.status_counts as Record<string, number>) || {}
      unreadCounts.value = (res.data.unread_counts as Record<string, number>) || {}
      if (cntReq) {
        const cntRes = await cntReq
        workerCounts.value = (cntRes.data.status_counts as Record<string, number>) || {}
      }
    }
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  page.value = 1
  fetchList()
}

/** 点击 Tab → 选中并刷新（写回 URL） */
const selectTab = (key: string) => {
  activeTabKey.value = key
  queryStore.touched = true
  syncUrl()
  fetchList()
}

/**
 * 预估文本渲染宽度（13px / 500 字号）：CJK 与全角标点按 14px，ASCII 按 7.5px。
 * el-select 内部 search input 有默认固有宽度，靠 CSS 无法让胶囊贴合文字，
 * 因此按展示文案长度反算宽度并内联下发，实现「宽度随文字长度变化」。
 */
function estimateTextWidth(text: string): number {
  let w = 0
  for (const ch of text) {
    w += /[\u4e00-\u9fa5\u3000-\u303f\uff00-\uffef]/.test(ch) ? 14 : 7.5
  }
  return Math.ceil(w)
}

/** 胶囊内 el-select 的宽度 = 文案宽度 + 下拉箭头/内边距（约 24px，值紧贴箭头），上限 240px */
const selectWidthOf = (text: string): number =>
  Math.min(Math.max(estimateTextWidth(text) + 24, 58), 240)

/** 企业筛选：未选时按 placeholder「全部企业」定宽，选中后按企业名定宽 */
const enterpriseSelectWidth = computed(() => {
  const name = enterpriseOptions.value.find(e => e.id === enterpriseId.value)?.name
  return selectWidthOf(name || '全部企业')
})

/** 用户筛选：未选/禁用时按当前 placeholder，选中后按用户昵称定宽 */
const reporterSelectWidth = computed(() => {
  const nickname = reporterOptions.value.find(r => r.id === reporterId.value)?.nickname
  return selectWidthOf(nickname || reporterPlaceholder.value)
})

/** Tab 角标：
 *  - review 仅「待审核」显示未读红点（处理中/结束不显示）；
 *  - my_repair 仅「待补签 / 已退回 / 已完成 / 已取消」显示未读红点；
 *  - worker_task（维修业务员）仅「处理中 / 待补签」显示角标，且一律取自稳定的
 *    workerCounts（repairer_id=me 视角），不受当前 Tab 是否带 repairer_id 收敛影响，
 *    切到任意 Tab 都稳定挂着；其余 Tab（我的维修 / 待接单 / 结束）不显示。
 *  未读数据来自 unread_counts，进详情页标记已读后随之减少。 */
const statusCountOf = (tab: ModeTab): number => {
  if (!tab.tag) return 0
  if (mode.value === 'review' && tab.key !== 'pending_review') return 0
  if (mode.value === 'my_repair' && !['pending_sign', 'rejected', 'finished', 'closed'].includes(tab.key)) return 0
  if (mode.value === 'worker_task') {
    if (tab.key === 'processing') return workerCounts.value['processing'] || 0
    if (tab.key === 'pending_sign') return workerCounts.value['pending_sign'] || 0
    return 0
  }
  const useUnread = mode.value === 'review' || mode.value === 'my_repair'
  const source = useUnread ? unreadCounts.value : statusCounts.value
  const statuses = TAG_STATUS_GROUPS[tab.tag] || []
  return statuses.reduce((sum, k) => sum + (source[k] || 0), 0)
}

/** 企业变化：报修人候选随企业域变化，清空已选报修人并重拉候选 */
const handleEnterpriseChange = () => {
  reporterId.value = ''
  reporterName.value = ''
  reporterOptions.value = []
  loadReporterOptions()
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
  activeTabKey.value = tabs.value[0].key
  syncUrl()
  fetchList()
}

/**
 * 「按时间 ↑↓」：非看板 Tab 切换提交时间正/倒序。
 * 看板类 Tab 的排序由后端看板优先级决定（与首页一致），此处不生效，直接忽略。
 */
const toggleTimeSort = () => {
  if (isBoardTab.value) return
  if (sortBy.value === 'submitted_at') {
    sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  } else {
    // 此前按表格列排序 → 回到按提交时间，默认倒序
    sortBy.value = 'submitted_at'
    sortOrder.value = 'desc'
  }
  page.value = 1
  fetchList()
}

/** 详细筛选抽屉内重置（仅清空抽屉里的字段） */
const handleDrawerReset = () => {
  categoryId.value = ''
  propertyId.value = ''
  urgency.value = ''
  essenceFilter.value = ''
  keyword.value = ''
  dateRange.value = null
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
/** 企业下拉候选（管理端筛选 + 导出弹窗共用；仅含 id/name） */
interface EntOption {
  id: string
  name: string
}
const enterpriseOptions = ref<EntOption[]>([])
const repairerOptions = ref<Array<{ id: string; nickname: string; avatar_url: string }>>([])

/**
 * 企业候选来源分两类：
 * - 店方/超管（role>=1）：接口 /admin/enterprises（挂 RequirePlatformAdmin，role>=1 可调）
 * - 纯单位审核员（role=0 + membership.role=1）：接口会 403，只能取登录时下发的单位清单
 * 与小程序 `ensureManageOptions` 完全一致的取数口径。
 */
const loadEnterpriseOptions = async () => {
  if (enterpriseOptions.value.length) return
  if (userStore.isStoreStaff) {
    const res = await adminAPI.getEnterprises({ page: 1, page_size: 100 })
    enterpriseOptions.value = res.data.list.map(e => ({ id: e.id, name: e.name }))
  } else {
    enterpriseOptions.value = userStore.reviewerEnterprises.map(e => ({
      id: e.enterprise_id,
      name: e.enterprise_name
    }))
  }
}

const loadCategoryOptions = async () => {
  if (categoryOptions.value.length) return
  const res = await fetchOrderOptions()
  categoryOptions.value = res.data.categories
}

/**
 * 报修人候选：按当前「企业」域拉取（后端对审核员/业务员强制要求 enterprise_id，
 * 全部企业时不调接口，候选仅留「全部用户」，彻底规避 403 / uuid 解析 500）。
 * getReporters 最多返回 200 条，直接用 filterable 下拉即可，无需远程搜索。
 */
const loadReporterOptions = async () => {
  if (!enterpriseId.value) {
    reporterOptions.value = []
    return
  }
  reporterSearching.value = true
  try {
    const res = await adminAPI.getReporters({ enterprise_id: enterpriseId.value })
    const list = res.data.list || []
    // 已选报修人若不在新候选内（如跨企业切换），保留其条目以回显
    if (reporterId.value && !list.some(o => o.id === reporterId.value) && reporterName.value) {
      list.unshift({ id: reporterId.value, nickname: reporterName.value, avatar_url: null })
    }
    reporterOptions.value = list
  } finally {
    reporterSearching.value = false
  }
}

const handleReporterChange = (val: string) => {
  const hit = reporterOptions.value.find(o => o.id === val)
  reporterName.value = hit?.nickname || ''
  handleSearch()
}

const loadRepairerOptions = async () => {
  if (repairerOptions.value.length) return
  const res = await adminAPI.getRepairers()
  repairerOptions.value = res.data.list
}

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

// ---- 待审核行就地快操作：审核通过 / 退回 ----
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

// 项目大类选项（完整保留嵌套属性，供"项目属性"筛选联动）
const categoryOptions = ref<CategoryOption[]>([])
const propertyOptions = computed(() => {
  if (!categoryId.value) return []
  const c = categoryOptions.value.find(x => x.id === categoryId.value)
  return (c?.properties || []).map(p => ({ id: p.id, name: p.name }))
})

// 报修人筛选（按企业域拉取，来源 /admin/orders/reporters）
const reporterOptions = ref<ReporterOption[]>([])
const reporterSearching = ref(false)

onMounted(async () => {
  const rid = queryStore.reporterId
  const rname = queryStore.reporterName
  if (rid && rname) {
    reporterName.value = rname
    if (!reporterOptions.value.some(o => o.id === rid)) {
      reporterOptions.value = [{ id: rid, nickname: rname, avatar_url: null }]
    }
  }
  initModeAndTab()
  await loadEnterpriseOptions()
  if (enterpriseId.value) await loadReporterOptions()
  await loadCategoryOptions()
  fetchList()
})

// 从其它路由（如首页卡片）带 ?mode=&tag= 进入时，重新落定视图并拉取。
// 本页 syncUrl 写回的 query 变化（切 Tab/筛选）已在事件里自行 fetchList，跳过避免双倍请求。
watch(
  () => route.query,
  q => {
    if (route.name !== 'OrderList') return
    if (queryMatchesState(q as Record<string, unknown>)) return
    initModeAndTab()
    fetchList()
  }
)
</script>

<template>
  <div>
    <el-card shadow="never" class="filter-card">
      <!-- 顶部 Tab（由 ORDER_MODES[mode].tabs 配置驱动，display_tag 英文键，绝不用中文查库） + 实时角标 -->
      <div class="status-seg">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="['seg-btn', { active: activeTabKey === tab.key }]"
          @click="selectTab(tab.key)"
        >
          <span class="seg-label">{{ tab.label }}</span>
          <span v-if="statusCountOf(tab)" class="seg-count">{{ statusCountOf(tab) }}</span>
        </button>
      </div>

      <!-- 第二行：企业(默认全部企业) + 用户(默认全部用户，未选企业时灰显「请先选择企业」) + 按时间排序 -->
      <!-- 筛选器对齐小程序胶囊样式：浅灰底胶囊 + 灰色标签前缀 + 黑色筛选文字 -->
      <div class="filter-bar">
        <div class="pill-filter">
          <span class="pill-label">企业</span>
          <el-select
            v-model="enterpriseId"
            placeholder="全部企业"
            clearable
            filterable
            class="pill-select"
            :style="{ width: `${enterpriseSelectWidth}px` }"
            popper-class="pill-select-popper"
            @change="handleEnterpriseChange"
          >
            <el-option
              v-for="e in enterpriseOptions"
              :key="e.id"
              :label="e.name"
              :value="e.id"
            />
          </el-select>
        </div>
        <div v-if="!isUserSource" class="pill-filter">
          <span class="pill-label">用户</span>
          <el-select
            v-model="reporterId"
            :placeholder="reporterPlaceholder"
            clearable
            filterable
            :disabled="!enterpriseId"
            :loading="reporterSearching"
            class="pill-select"
            :style="{ width: `${reporterSelectWidth}px` }"
            popper-class="pill-select-popper"
            @change="handleReporterChange"
          >
            <el-option
              v-for="r in reporterOptions"
              :key="r.id"
              :label="r.nickname"
              :value="r.id"
            />
          </el-select>
        </div>

        <!-- 按时间 ↑↓：非看板 Tab 控制提交时间正/倒序；看板类 Tab 由后端优先级决定，此处不生效 -->
        <button
          v-if="!isUserSource"
          type="button"
          class="time-sort on"
          :title="isBoardTab ? '该状态按看板顺序排列（与首页一致），不随此切换' : '点击切换提交时间 倒序/正序'"
          @click="toggleTimeSort"
        >
          <span>按时间</span>
          <span class="time-sort-arrow">{{ timeSortArrow }}</span>
        </button>

        <button
          v-if="!isUserSource"
          type="button"
          class="adv-toggle"
          :class="{ on: advVisible }"
          @click="advVisible = !advVisible"
        >
          <span>详细筛选</span>
          <span v-if="advActiveCount" class="adv-badge">{{ advActiveCount }}</span>
          <el-icon class="adv-arrow" :class="{ expanded: advVisible }"><ArrowDown /></el-icon>
        </button>
        <el-button @click="handleReset">重置</el-button>
        <div class="spacer" />
        <!-- 导出为店方专属能力（5.14），单位审核员不展示 -->
        <el-button v-if="userStore.isStoreStaff" type="primary" plain @click="openExportDialog">
          <el-icon><Download /></el-icon>
          导出
        </el-button>
        <!-- 视图切换并入筛选行（右上角），不再单独占一行 -->
        <el-radio-group v-model="viewMode" size="small" class="view-switch">
          <el-radio-button value="card">卡片 (C)</el-radio-button>
          <el-radio-button value="table">表格 (D)</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 详细筛选：点击按钮后在下一行就地展开（仅管理端），不再走右侧抽屉 -->
      <div v-if="advVisible && !isUserSource" class="adv-panel">
        <div class="adv-grid">
          <div class="adv-item">
            <div class="adv-label">项目大类</div>
            <el-select
              v-model="categoryId"
              placeholder="全部大类"
              clearable
              filterable
              class="adv-full"
              @change="handleCategoryChange"
            >
              <el-option
                v-for="c in categoryOptions"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              />
            </el-select>
          </div>
          <div class="adv-item">
            <div class="adv-label">问题类型（属性）</div>
            <el-select
              v-model="propertyId"
              placeholder="全部类型"
              clearable
              filterable
              :disabled="!categoryId"
              class="adv-full"
              @change="handleSearch"
            >
              <el-option
                v-for="p in propertyOptions"
                :key="p.id"
                :label="p.name"
                :value="p.id"
              />
            </el-select>
          </div>
          <div class="adv-item">
            <div class="adv-label">紧急程度</div>
            <el-select
              v-model="urgency"
              placeholder="全部紧急度"
              clearable
              class="adv-full"
              @change="handleSearch"
            >
              <el-option label="普通" value="normal" />
              <el-option label="紧急" value="urgent" />
              <el-option label="非常紧急" value="very_urgent" />
            </el-select>
          </div>
          <div class="adv-item">
            <div class="adv-label">本质标签</div>
            <el-select
              v-model="essenceFilter"
              placeholder="全部标签"
              clearable
              class="adv-full"
              @change="handleSearch"
            >
              <el-option label="不能修" value="unrepairable" />
              <el-option label="客户取消" value="customer_cancelled" />
              <el-option label="待补签" value="pending_sign" />
              <el-option label="已签字" value="signed" />
            </el-select>
          </div>
          <div class="adv-item">
            <div class="adv-label">工单号关键字</div>
            <el-input
              v-model="keyword"
              placeholder="输入工单号后回车"
              clearable
              class="adv-full"
              @keyup.enter="handleSearch"
            />
          </div>
          <div class="adv-item">
            <div class="adv-label">提交日期</div>
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              class="adv-full adv-date"
            />
          </div>
        </div>
        <div class="adv-actions">
          <el-button link type="primary" @click="handleDrawerReset">清空详细筛选</el-button>
          <el-button size="small" type="primary" @click="handleSearch">应用</el-button>
        </div>
      </div>
    </el-card>

    <el-card shadow="never">
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
        <!-- 报修人列仅管理端数据源显示：用户端 /orders 行不含 reporter（owner 隔离全是本人单），
             且历史数据可能缺 reporter，模板一律判空，避免渲染抛错导致整页卡死（转圈不消失） -->
        <el-table-column
          v-if="!isUserSource"
          prop="reporter"
          label="报修人"
          width="150"
          sortable="custom"
        >
          <template #default="{ row }">
            <div class="reporter">
              <el-avatar :size="24" :src="row.reporter?.avatar_url">
                {{ row.reporter?.nickname?.charAt(0) }}
              </el-avatar>
              <span>{{ row.reporter?.nickname || '—' }}</span>
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
          :hide-enterprise="!!enterpriseId"
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

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 4px;
}

/* 筛选胶囊（对齐小程序）：浅灰底 + 灰色标签前缀 + 黑色筛选文字，宽度随文字长度自适应 */
.pill-filter {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  height: 32px;
  padding: 0 14px 0 18px;
  background: #eef2f4;
  border-radius: 16px;
  transition: background 0.15s ease;
}

.pill-filter:hover {
  background: #e3e9ee;
}

.pill-label {
  font-size: 13px;
  color: #6b7280;
  flex-shrink: 0;
}

/* 胶囊宽度随选中值长度自适应：宽度由 JS 按文字长度估算后内联下发 */
.pill-select {
  max-width: 240px;
  flex: 0 0 auto;
}

/* 胶囊内的 el-select：去边框去底色，文字黑色加粗；压缩值与箭头间距 */
.pill-filter :deep(.el-select__wrapper) {
  background: transparent;
  box-shadow: none !important;
  min-height: 32px;
  padding: 0 0 0 2px;
  gap: 2px;
  font-size: 13px;
}

.pill-filter :deep(.el-select__selection) {
  gap: 0;
  margin-right: 0;
}

.pill-filter :deep(.el-select__suffix) {
  margin-left: 0;
  gap: 0;
}

.pill-filter :deep(.el-select__placeholder),
.pill-filter :deep(.el-select__selected-item) {
  color: #111827;
  font-weight: 500;
}

/* 禁用态（未选企业时的用户筛选）：文字仍保持深灰可读，光标不变 */
.pill-filter :deep(.el-select__wrapper.is-disabled) {
  cursor: not-allowed;
}

.pill-filter :deep(.el-select__wrapper.is-disabled .el-select__placeholder),
.pill-filter :deep(.el-select__wrapper.is-disabled .el-select__selected-item) {
  color: #6b7280;
}

/* 「按时间 ↑↓」胶囊：与状态 Tab 同苍岭绿语义，非看板 Tab 生效，看板 Tab 置静默态 */
.time-sort {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 7px 14px;
  border: 1px solid var(--xn-primary-light-5);
  border-radius: 16px;
  background: var(--xn-primary-light-9);
  color: var(--xn-primary);
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  transition: all 0.15s ease;
}

.time-sort:hover {
  border-color: var(--xn-primary);
}

.time-sort.on {
  border-color: var(--xn-primary);
}

.time-sort-arrow {
  font-size: 12px;
  font-weight: 700;
}

/* ── 详细筛选：按钮 + 就地展开面板（下一行） ── */
.adv-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 7px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 16px;
  background: #fff;
  color: #606266;
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  transition: all 0.15s ease;
}

.adv-toggle:hover,
.adv-toggle.on {
  border-color: var(--xn-primary);
  color: var(--xn-primary);
}

.adv-arrow {
  font-size: 14px;
  line-height: 1;
  transition: transform 0.2s ease;
}

.adv-arrow.expanded {
  transform: rotate(180deg);
}

.adv-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: var(--xn-primary);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.adv-panel {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--xn-border-soft);
}

.adv-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 12px 16px;
}

.adv-item {
  min-width: 0;
}

.adv-label {
  font-size: 12px;
  color: var(--xn-text-500);
  margin-bottom: 6px;
}

.adv-full {
  width: 100%;
}

.adv-date {
  max-width: 260px;
}

.adv-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 12px;
}

.drawer-full {
  width: 100%;
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

/* 视图切换（并入筛选行右端） */
.view-switch {
  margin-left: 8px;
  flex-shrink: 0;
}

/* 顶部状态按钮组（对齐小程序审核/接单页） */
.status-seg {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.seg-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 1px solid #dcdfe6;
  border-radius: 16px;
  background: #fff;
  color: #606266;
  font-size: 13px;
  line-height: 1.2;
  cursor: pointer;
  transition: all 0.15s ease;
}

.seg-btn:hover {
  border-color: #409eff;
  color: #409eff;
}

.seg-btn.active {
  border-color: #0f766e;
  background: #0f766e;
  color: #fff;
}

.seg-label {
  font-weight: 500;
}

.seg-count {
  position: absolute;
  top: -8px;
  right: -8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  border: 2px solid #fff;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  background: #f56c6c;
  color: #fff;
  box-sizing: border-box;
}

.seg-btn.active .seg-count {
  background: #f56c6c;
  color: #fff;
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
