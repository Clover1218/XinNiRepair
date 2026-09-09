<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminAPI, fetchOrderOptions, type OrderListParams } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import { useOrderListQueryStore } from '@/stores/orderListQuery'
import type {
  CategoryOption,
  EnterpriseListItem,
  OrderListItem,
  OrderStatus,
  ReporterOption,
  Urgency
} from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const userStore = useUserStore()

// 单位审核员不进入本页（独立 /review 视图），直接跳转
if (!userStore.isStoreStaff) {
  router.replace('/review')
}

// 状态 Tab（全部=不传，V1.4 状态机）
const statusTabs = [
  { label: '全部', value: '' },
  { label: '待审核', value: 'reported' },
  { label: '待接单', value: 'pending_accept' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' }
]

// 状态标签颜色
const statusTagType: Record<OrderStatus, string> = {
  draft: 'info',
  reported: 'danger',
  pending_accept: 'warning',
  processing: 'primary',
  completed: 'success',
  cancelled: 'info',
  rejected: 'danger'
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
  page,
  pageSize
} = storeToRefs(queryStore)

const reporterName = ref(queryStore.reporterName)

const loading = ref(false)
const list = ref<OrderListItem[]>([])
const total = ref(0)

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
    if (enterpriseId.value) params.enterprise_id = enterpriseId.value
    if (categoryId.value) params.category_id = categoryId.value
    if (propertyId.value) params.property_id = propertyId.value
    if (reporterId.value) params.reporter_id = reporterId.value
    if (keyword.value.trim()) params.keyword = keyword.value.trim()
    if (urgency.value) params.urgency = urgency.value
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
  fetchList()
}

const handleTabChange = () => {
  page.value = 1
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

onMounted(() => {
  // 从会话态恢复"报修人"选中项的回显（远程选项列表已随页面卸载清空）
  const rid = queryStore.reporterId
  const rname = queryStore.reporterName
  if (rid && rname && !reporterOptions.value.some(o => o.id === rid)) {
    reporterOptions.value = [{ id: rid, nickname: rname, avatar_url: null }]
  }
  fetchList()
  loadEnterpriseOptions()
  loadCategoryOptions()
})
</script>

<template>
  <div>
    <el-card shadow="never" class="filter-card">
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
        <el-button type="primary" plain @click="openExportDialog">
          <el-icon><Download /></el-icon>
          导出
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table
        v-loading="loading"
        :data="list"
        stripe
        :default-sort="queryStore.defaultSort"
        @row-click="goDetail"
        @sort-change="handleSortChange"
      >
        <el-table-column prop="order_no" label="工单号" width="160" sortable="custom" />
        <el-table-column prop="enterprise_name" label="企业" width="140" show-overflow-tooltip sortable="custom" />
        <el-table-column prop="reporter" label="报修人" width="140" sortable="custom">
          <template #default="{ row }">
            <div class="reporter">
              <el-avatar :size="24" :src="row.reporter.avatar_url">
                {{ row.reporter.nickname?.charAt(0) }}
              </el-avatar>
              <span>{{ row.reporter.nickname }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="category_name" label="项目" min-width="180" show-overflow-tooltip sortable="custom">
          <template #default="{ row }">
            <span v-if="row.property_name">{{ row.category_name }} · {{ row.property_name }}</span>
            <span v-else>{{ row.category_name }}</span>
          </template>
        </el-table-column>
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
        <el-table-column prop="status" label="状态" width="110" sortable="custom">
          <template #default="{ row }">
            <el-tag :type="statusTagType[row.status as OrderStatus]">
              {{ row.status_label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="submitted_at" label="提交时间" width="160" sortable="custom">
          <template #default="{ row }">
            {{ formatDateTime(row.submitted_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="repair_content" label="维修操作内容" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.repair_content">{{ row.repair_content }}</span>
            <span v-else class="cell-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="110" align="right">
          <template #default="{ row }">
            <span v-if="row.amount">{{ `¥${Number(row.amount).toFixed(2)}` }}</span>
            <span v-else class="cell-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="goDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>

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
</style>
