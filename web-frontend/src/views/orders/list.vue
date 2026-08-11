<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminAPI, type OrderListParams } from '@/api/admin'
import type { EnterpriseListItem, OrderListItem, OrderStatus, Urgency } from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

// 状态 Tab（全部=不传）
const statusTabs = [
  { label: '全部', value: '' },
  { label: '待查阅', value: 'reported' },
  { label: '已阅', value: 'reviewed' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' }
]

// 8.1 状态标签颜色
const statusTagType: Record<OrderStatus, string> = {
  draft: 'info',
  reported: 'danger',
  reviewed: 'warning',
  processing: 'primary',
  completed: 'success',
  cancelled: 'info'
}

// 8.2 紧急程度标识
const urgencyMap: Record<Urgency, { label: string; color: string; dots: number }> = {
  normal: { label: '普通', color: '#409eff', dots: 1 },
  urgent: { label: '紧急', color: '#e6a23c', dots: 2 },
  very_urgent: { label: '非常紧急', color: '#f56c6c', dots: 3 }
}

const activeStatus = ref('')
const enterpriseId = ref('')
const orderNo = ref('')
const projectName = ref('')
const urgency = ref('')
const dateRange = ref<[string, string] | null>(null)
// 排序状态（默认按提交时间倒序）
const sortBy = ref('submitted_at')
const sortOrder = ref<'asc' | 'desc'>('desc')

const loading = ref(false)
const list = ref<OrderListItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

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
    if (orderNo.value.trim()) params.order_no = orderNo.value.trim()
    if (projectName.value.trim()) params.project_name = projectName.value.trim()
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

const handleReset = () => {
  enterpriseId.value = ''
  orderNo.value = ''
  projectName.value = ''
  urgency.value = ''
  dateRange.value = null
  activeStatus.value = ''
  sortBy.value = 'submitted_at'
  sortOrder.value = 'desc'
  page.value = 1
  fetchList()
}

const handleTabChange = () => {
  page.value = 1
  fetchList()
}

// ---- V1.1 表格列排序（sortable="custom"，服务端排序） ----
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

// ---- V1.1 导出工单（5.14） ----
const exportDialogVisible = ref(false)
const exportMode = ref<'enterprise' | 'repairer'>('enterprise')
const exportEnterpriseId = ref('')
const exportRepairerId = ref('')
const exportDateRange = ref<[string, string] | null>(null)
const exportFields = ref<string[]>(['order_no', 'time', 'content', 'quantity', 'unit_price', 'amount', 'remark'])
const exporting = ref(false)
const enterpriseOptions = ref<EnterpriseListItem[]>([])
// 维修员(业务员)选项（V1.1 5.15：导出弹窗业务员模式下拉）
const repairerOptions = ref<Array<{ id: string; nickname: string; avatar_url: string }>>([])

// 加载企业列表（V1.1：搜索栏企业下拉 / 导出弹窗共用）
const loadEnterpriseOptions = async () => {
  if (enterpriseOptions.value.length) return
  const res = await adminAPI.getEnterprises({ page: 1, page_size: 100 })
  enterpriseOptions.value = res.data.list
}

// 加载维修员列表（V1.1 5.15：导出弹窗业务员模式下拉）
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
  // 加载企业列表供下拉选择（mode=enterprise 时使用）
  await loadEnterpriseOptions()
  // 加载维修员列表供下拉选择（mode=repairer 时使用）
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
    // 解析 Content-Disposition 中的 filename
    const disposition = res.headers['content-disposition'] as string | undefined
    let filename = '工单导出.xlsx'
    const match = disposition?.match(/filename="?([^";]+)"?/)
    if (match) filename = decodeURIComponent(match[1])
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
  fetchList()
  loadEnterpriseOptions()
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
        <!-- V1.1：三个独立搜索（企业 / 工单号 / 项目名）+ 紧急程度 + 日期 -->
        <el-select
          v-model="enterpriseId"
          placeholder="按企业搜索"
          clearable
          filterable
          class="filter-enterprise"
          @change="handleSearch"
        >
          <el-option
            v-for="e in enterpriseOptions"
            :key="e.id"
            :label="e.name"
            :value="e.id"
          />
        </el-select>
        <el-input
          v-model="orderNo"
          placeholder="工单号"
          clearable
          class="filter-input"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="projectName"
          placeholder="项目名称"
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
        <el-table-column prop="project_name" label="项目名称" min-width="180" show-overflow-tooltip sortable="custom" />
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
  width: 180px;
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

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
