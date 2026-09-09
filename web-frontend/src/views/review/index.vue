<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminAPI } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { EnterpriseMembership, OrderListItem, OrderStatus } from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()
const userStore = useUserStore()

/** 本人担任审核员的单位 */
const enterprises = ref<EnterpriseMembership[]>(userStore.reviewerEnterprises)

const statusTabs = [
  { label: '待审核', value: 'reported' },
  { label: '待接单', value: 'pending_accept' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' }
]

const statusTagType: Record<string, string> = {
  draft: 'info',
  reported: 'danger',
  pending_accept: 'warning',
  processing: 'primary',
  completed: 'success',
  cancelled: 'info'
}

const activeEnterpriseId = ref('')
const activeStatus = ref('reported')
const loading = ref(false)
const list = ref<OrderListItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const fetchList = async () => {
  if (!activeEnterpriseId.value) {
    list.value = []
    total.value = 0
    return
  }
  loading.value = true
  try {
    // 后端对 role=0 单位审核员强制 enterprise_id 且限定本单位
    const res = await adminAPI.getOrders({
      page: page.value,
      page_size: pageSize.value,
      status: activeStatus.value || undefined,
      enterprise_id: activeEnterpriseId.value,
      sort_by: 'submitted_at',
      sort_order: 'desc'
    })
    list.value = res.data.list
    total.value = res.data.total
  } finally {
    loading.value = false
  }
}

const switchEnterprise = (entId: string) => {
  page.value = 1
  activeEnterpriseId.value = entId
  fetchList()
}

const switchTab = (value: string) => {
  page.value = 1
  activeStatus.value = value
  fetchList()
}

const handlePageChange = (p: number) => {
  page.value = p
  fetchList()
}

const goDetail = (row: OrderListItem) => {
  router.push(`/orders/${row.id}`)
}

// ---- 审核通过 / 退回（仅 reported 状态行显示） ----
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
  if (reason.length < 10) {
    ElMessage.warning('退回原因至少 10 个字')
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

// 初始化：默认选中第一个审核单位；无审核单位时给出引导
watch(
  () => userStore.userInfo?.enterprises,
  () => {
    enterprises.value = userStore.reviewerEnterprises
    if (!activeEnterpriseId.value && enterprises.value.length) {
      activeEnterpriseId.value = enterprises.value[0].enterprise_id
    }
  },
  { deep: true }
)

onMounted(() => {
  if (!userStore.hasReviewerRole) {
    ElMessage.warning('您暂无担任审核员的单位')
    router.replace(userStore.isStoreStaff ? '/orders' : '/login')
    return
  }
  if (enterprises.value.length) {
    activeEnterpriseId.value = enterprises.value[0].enterprise_id
  }
  fetchList()
})
</script>

<template>
  <div>
    <el-card shadow="never" class="head-card">
      <div class="head-row">
        <div class="head-left">
          <span class="head-title">单位审核</span>
          <el-select
            :model-value="activeEnterpriseId"
            placeholder="选择单位（仅显示本人担任审核员的单位）"
            class="ent-select"
            @change="switchEnterprise"
          >
            <el-option
              v-for="e in enterprises"
              :key="e.enterprise_id"
              :label="e.enterprise_name"
              :value="e.enterprise_id"
            />
          </el-select>
        </div>
        <span class="head-hint">企业成员审批等操作请在侧边栏【企业管理】进行</span>
      </div>
      <div v-if="!enterprises.length" class="empty-tip">
        您当前未在任何单位担任“单位审核员”。如需审核本单位工单，请联系维修业务员为您开通审核员身份。
      </div>
    </el-card>

    <!-- 统计入口已独立为侧边栏【统计】页 /stats（V1.3，审核员限本单位） -->
    <el-card v-if="activeEnterpriseId" shadow="never">
      <el-tabs :model-value="activeStatus" @tab-change="switchTab">
        <el-tab-pane
          v-for="tab in statusTabs"
          :key="tab.value"
          :label="tab.label"
          :name="tab.value"
        />
      </el-tabs>

      <el-table v-loading="loading" :data="list" stripe @row-click="goDetail">
        <el-table-column label="工单号" width="160">
          <template #default="{ row }">{{ row.order_no || '-' }}</template>
        </el-table-column>
        <el-table-column label="报修人" width="130">
          <template #default="{ row }">
            <div class="reporter">
              <el-avatar :size="24" :src="row.reporter.avatar_url">
                {{ row.reporter.nickname?.charAt(0) }}
              </el-avatar>
              <span>{{ row.reporter.nickname }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="项目" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.property_name">{{ row.category_name }} · {{ row.property_name }}</span>
            <span v-else>{{ row.category_name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="urgency_label" label="紧急度" width="100" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusTagType[row.status as OrderStatus]">
              {{ row.status_label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="160">
          <template #default="{ row }">{{ formatDateTime(row.submitted_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'reported'">
              <el-button
                link
                type="success"
                :loading="auditingId === row.id"
                @click.stop="handleAudit(row)"
              >审核通过</el-button>
              <el-button link type="danger" @click.stop="openReject(row)">退回</el-button>
            </template>
            <el-button v-else link type="primary" @click.stop="goDetail(row)">查看</el-button>
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
        />
      </div>
    </el-card>

    <!-- 退回弹窗 -->
    <el-dialog v-model="rejectDialogVisible" title="退回工单" width="480px">
      <el-form label-position="top">
        <el-form-item label="退回原因（报修人将收到并修改后重新提交）">
          <el-input
            v-model="rejectReason"
            type="textarea"
            :rows="4"
            :maxlength="200"
            show-word-limit
            placeholder="请输入退回原因，至少 10 个字"
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
.head-card {
  margin-bottom: 16px;
}

.head-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.head-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.head-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  white-space: nowrap;
}

.ent-select {
  width: 320px;
}

.head-hint {
  font-size: 12px;
  color: #909399;
}

.empty-tip {
  margin-top: 12px;
  color: #909399;
  font-size: 13px;
}

.reporter {
  display: flex;
  align-items: center;
  gap: 6px;
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
