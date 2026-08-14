<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type UploadRequestOptions, type UploadUserFile } from 'element-plus'
import { adminAPI } from '@/api/admin'
import type { AvailableAction, OrderDetail } from '@/types'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const orderId = route.params.id as string

const loading = ref(false)
const detail = ref<OrderDetail | null>(null)

// 状态中文映射（附录A）
const statusLabelMap: Record<string, string> = {
  draft: '草稿',
  reported: '已上报',
  reviewed: '已阅',
  processing: '处理中',
  completed: '已完成',
  cancelled: '已取消'
}

const statusTagType: Record<string, string> = {
  draft: 'info',
  reported: 'danger',
  reviewed: 'warning',
  processing: 'primary',
  completed: 'success',
  cancelled: 'info'
}

const imageUrls = computed(() => detail.value?.images.map(i => i.url) ?? [])
const receiptUrls = computed(() => detail.value?.receipts.map(r => r.url) ?? [])

/** V1.1：对账信息是否展示（completed 状态且有对账数据） */
const showFinance = computed(() => detail.value?.status === 'completed')

/** V1.1：附加信息是否展示（completed 状态且有 metadata 数据） */
const showMetadata = computed(() => {
  const d = detail.value
  if (!d || d.status !== 'completed') return false
  const m = d.metadata
  if (!m) return false
  return !!(m.repair_result || m.repair_method || m.warranty_period || m.repair_duration != null || m.extra_remark)
})

/** 金额格式化（8.4）：保留两位小数 */
const formatMoney = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  return Number(value).toFixed(2)
}

const fetchDetail = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getOrderDetail(orderId)
    detail.value = res.data
  } finally {
    loading.value = false
  }
}

// ---- 查阅 / 接单（简单操作，可带确认文案） ----
const runSimpleAction = async (action: AvailableAction) => {
  if (action.confirm_message) {
    try {
      await ElMessageBox.confirm(action.confirm_message, '操作确认', {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消'
      })
    } catch {
      return
    }
  }
  if (action.action === 'review') {
    await adminAPI.reviewOrder(orderId)
  } else if (action.action === 'accept') {
    await adminAPI.acceptOrder(orderId)
  }
  ElMessage.success(`${action.label}成功`)
  await fetchDetail()
}

// ---- 退回（弹窗填写原因，≥10 字） ----
const rejectDialogVisible = ref(false)
const rejectReason = ref('')
const rejectMinLength = ref(10)
const rejecting = ref(false)

const handleRejectSubmit = async () => {
  const reason = rejectReason.value.trim()
  if (reason.length < rejectMinLength.value) {
    ElMessage.warning(`退回原因至少 ${rejectMinLength.value} 个字`)
    return
  }
  if (reason.length > 200) {
    ElMessage.warning('退回原因不能超过 200 字')
    return
  }
  rejecting.value = true
  try {
    await adminAPI.rejectOrder(orderId, reason)
    ElMessage.success('已退回')
    rejectDialogVisible.value = false
    await fetchDetail()
  } finally {
    rejecting.value = false
  }
}

// ---- 完工（弹窗：维修备注 + 对账信息 + 收据上传） ----
const completeDialogVisible = ref(false)
const completeRemark = ref('')
const receipts = ref<string[]>([])
const receiptFileList = ref<UploadUserFile[]>([])
const receiptUrlMap = new Map<number, string>()
const completing = ref(false)
const previewVisible = ref(false)
const previewUrl = ref('')

// 对账信息（5.6 必填）
const completeQuantity = ref<number | undefined>(undefined)
const completeUnitPrice = ref<number | undefined>(undefined)
const completeRepairContent = ref('')
// 维修附加信息（metadata）
const metaRepairResult = ref('')
const metaRepairMethod = ref('')
const metaWarrantyPeriod = ref('')
const metaRepairDuration = ref<number | undefined>(undefined)
const metaExtraRemark = ref('')

const repairResultOptions = ['完全修复', '部分修复', '无法修复']
const repairMethodOptions = ['上门维修', '返店维修', '远程协助']

const buildMetadata = () => ({
  repair_result: metaRepairResult.value || undefined,
  repair_method: metaRepairMethod.value || undefined,
  warranty_period: metaWarrantyPeriod.value || undefined,
  extra_remark: metaExtraRemark.value || undefined,
  repair_duration: metaRepairDuration.value
})

const handleCompleteSubmit = async () => {
  const remark = completeRemark.value.trim()
  if (!remark) {
    ElMessage.warning('请填写维修备注')
    return
  }
  if (remark.length > 200) {
    ElMessage.warning('备注不能超过 200 字')
    return
  }
  const quantity = completeQuantity.value
  const unitPrice = completeUnitPrice.value
  if (quantity === undefined || Number.isNaN(Number(quantity)) || quantity < 0) {
    ElMessage.warning('请填写正确的数量（≥0）')
    return
  }
  if (unitPrice === undefined || Number.isNaN(Number(unitPrice)) || unitPrice < 0) {
    ElMessage.warning('请填写正确的单价（≥0）')
    return
  }
  const repairContent = completeRepairContent.value.trim()
  if (!repairContent) {
    ElMessage.warning('请填写维修操作内容')
    return
  }
  if (repairContent.length > 200) {
    ElMessage.warning('维修操作内容不能超过 200 字')
    return
  }
  completing.value = true
  try {
    await adminAPI.completeOrder(orderId, {
      remark,
      receipts: receipts.value,
      quantity,
      unit_price: unitPrice,
      repair_content: repairContent,
      metadata: buildMetadata()
    })
    ElMessage.success('完工成功')
    completeDialogVisible.value = false
    await fetchDetail()
  } finally {
    completing.value = false
  }
}

const beforeReceiptUpload = (rawFile: File) => {
  const validTypes = ['image/jpeg', 'image/png', 'image/webp']
  if (!validTypes.includes(rawFile.type)) {
    ElMessage.error('仅支持 jpg/png/webp 格式')
    return false
  }
  if (rawFile.size > 5 * 1024 * 1024) {
    ElMessage.error('单张图片不能超过 5MB')
    return false
  }
  if (receiptFileList.value.length >= 3) {
    ElMessage.warning('同一工单最多上传 3 张收据图片')
    return false
  }
  return true
}

const handleReceiptUpload = async (options: UploadRequestOptions) => {
  try {
    const res = await adminAPI.uploadReceipt(orderId, options.file)
    receipts.value.push(res.data.url)
    receiptUrlMap.set(options.file.uid, res.data.url)
    const item = receiptFileList.value.find(f => f.uid === options.file.uid)
    if (item) {
      item.url = res.data.url
      item.status = 'success'
    }
    options.onSuccess(res.data)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '上传失败'
    options.onError(
      new Error(msg) as unknown as Parameters<UploadRequestOptions['onError']>[0]
    )
  }
}

const handleReceiptRemove = (file: UploadUserFile) => {
  const uid = file.uid
  if (uid === undefined) return
  const url = receiptUrlMap.get(uid)
  if (url) {
    receipts.value = receipts.value.filter(u => u !== url)
    receiptUrlMap.delete(uid)
  }
}

const handleReceiptPreview = (file: UploadUserFile) => {
  if (file.url) {
    previewUrl.value = file.url
    previewVisible.value = true
  }
}

const openCompleteDialog = () => {
  completeRemark.value = ''
  receipts.value = []
  receiptFileList.value = []
  receiptUrlMap.clear()
  // 重置对账信息与元数据
  completeQuantity.value = undefined
  completeUnitPrice.value = undefined
  completeRepairContent.value = ''
  metaRepairResult.value = ''
  metaRepairMethod.value = ''
  metaWarrantyPeriod.value = ''
  metaRepairDuration.value = undefined
  metaExtraRemark.value = ''
  completeDialogVisible.value = true
}

// ---- V1.1 修改对账信息（5.6.1，completed 状态） ----
const financeDialogVisible = ref(false)
const financeQuantity = ref<number | undefined>(undefined)
const financeUnitPrice = ref<number | undefined>(undefined)
const financeRepairContent = ref('')
const financeAmount = computed(() => {
  const q = Number(financeQuantity.value)
  const p = Number(financeUnitPrice.value)
  if (!Number.isNaN(q) && !Number.isNaN(p) && q >= 0 && p >= 0) {
    return (q * p).toFixed(2)
  }
  return '-'
})
const financeMetaRepairResult = ref('')
const financeMetaRepairMethod = ref('')
const financeMetaWarrantyPeriod = ref('')
const financeMetaRepairDuration = ref<number | undefined>(undefined)
const financeMetaExtraRemark = ref('')
const updatingFinance = ref(false)

const openFinanceDialog = () => {
  const d = detail.value
  financeQuantity.value = d?.quantity ?? undefined
  financeUnitPrice.value = d?.unit_price ?? undefined
  financeRepairContent.value = d?.repair_content ?? ''
  financeMetaRepairResult.value = d?.metadata?.repair_result ?? ''
  financeMetaRepairMethod.value = d?.metadata?.repair_method ?? ''
  financeMetaWarrantyPeriod.value = d?.metadata?.warranty_period ?? ''
  financeMetaRepairDuration.value = d?.metadata?.repair_duration ?? undefined
  financeMetaExtraRemark.value = d?.metadata?.extra_remark ?? ''
  financeDialogVisible.value = true
}

const handleUpdateFinance = async () => {
  const quantity = financeQuantity.value
  const unitPrice = financeUnitPrice.value
  if (quantity !== undefined && (Number.isNaN(Number(quantity)) || quantity < 0)) {
    ElMessage.warning('请填写正确的数量（≥0）')
    return
  }
  if (unitPrice !== undefined && (Number.isNaN(Number(unitPrice)) || unitPrice < 0)) {
    ElMessage.warning('请填写正确的单价（≥0）')
    return
  }
  if (financeRepairContent.value.trim().length > 200) {
    ElMessage.warning('维修操作内容不能超过 200 字')
    return
  }
  // 组装非空字段（后端字段可选，至少传一个）
  const data: {
    quantity?: number
    unit_price?: number
    repair_content?: string
    metadata?: Record<string, unknown>
  } = {}
  if (quantity !== undefined) data.quantity = quantity
  if (unitPrice !== undefined) data.unit_price = unitPrice
  if (financeRepairContent.value.trim()) data.repair_content = financeRepairContent.value.trim()
  const meta: Record<string, unknown> = {}
  if (financeMetaRepairResult.value) meta.repair_result = financeMetaRepairResult.value
  if (financeMetaRepairMethod.value) meta.repair_method = financeMetaRepairMethod.value
  if (financeMetaWarrantyPeriod.value) meta.warranty_period = financeMetaWarrantyPeriod.value
  if (financeMetaRepairDuration.value !== undefined) meta.repair_duration = financeMetaRepairDuration.value
  if (financeMetaExtraRemark.value) meta.extra_remark = financeMetaExtraRemark.value
  if (Object.keys(meta).length) data.metadata = meta

  if (Object.keys(data).length === 0) {
    ElMessage.warning('请至少修改一项对账信息')
    return
  }
  updatingFinance.value = true
  try {
    await adminAPI.updateFinance(orderId, data)
    ElMessage.success('对账信息已更新')
    financeDialogVisible.value = false
    await fetchDetail()
  } finally {
    updatingFinance.value = false
  }
}

const openRejectDialog = (action: AvailableAction) => {
  rejectMinLength.value = action.reason_min_length ?? 10
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

// ---- 动作分发 ----
const handleAction = (action: AvailableAction) => {
  if (action.action === 'reject') {
    openRejectDialog(action)
  } else if (action.action === 'complete') {
    openCompleteDialog()
  } else if (action.action === 'update_finance') {
    openFinanceDialog()
  } else {
    runSimpleAction(action)
  }
}

onMounted(fetchDetail)
</script>

<template>
  <div v-loading="loading">
    <el-card shadow="never">
      <template #header>
        <div class="detail-header">
          <el-button link @click="router.back()">
            <el-icon><ArrowLeft /></el-icon>
            返回列表
          </el-button>
          <span class="detail-title">{{ detail?.order_no }}</span>
          <el-tag v-if="detail" :type="statusTagType[detail.status]" class="detail-status">
            {{ detail.status_label }}
          </el-tag>
        </div>
      </template>

      <template v-if="detail">
        <div class="meta-row">
          <span v-if="detail.reporter" class="meta-item">
            <el-icon><User /></el-icon>
            报修人：{{ detail.reporter.nickname }}
          </span>
          <span class="meta-item">
            <el-icon><Clock /></el-icon>
            提交时间：{{ formatDateTime(detail.submitted_at) }}
          </span>
          <span class="meta-item">
            <el-icon><Warning /></el-icon>
            紧急程度：
            <span class="urgency-text">{{ detail.urgency_label }}</span>
          </span>
        </div>

        <el-alert
          v-if="detail.reject_reason"
          type="warning"
          :closable="false"
          show-icon
          class="reject-alert"
          :title="`退回原因：${detail.reject_reason}`"
        />

        <div class="section">
          <h3 class="section-title">报修信息</h3>
          <div class="info-grid">
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">工单号</span>
                <span class="info-value">{{ detail.order_no }}</span>
              </div>
              <div v-if="detail.enterprise_name" class="info-cell">
                <span class="info-label">报修企业</span>
                <span class="info-value">{{ detail.enterprise_name }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">项目名称</span>
                <span class="info-value">{{ detail.project_name }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">维修类别</span>
                <span class="info-value">{{ detail.category_label }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">维修类型</span>
                <span class="info-value">{{ detail.property_label }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">报修位置</span>
                <span class="info-value">{{ detail.room }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">联系方式</span>
                <span class="info-value">{{ detail.contact }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell info-cell--full">
                <span class="info-label">报修描述</span>
                <span class="info-value">{{ detail.description }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- V1.1 新增：对账信息（completed 状态展示） -->
        <div v-if="showFinance" class="section">
          <h3 class="section-title">对账信息</h3>
          <div class="info-grid">
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">数量</span>
                <span class="info-value">{{ detail.quantity ?? '-' }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">单价</span>
                <span class="info-value">{{ formatMoney(detail.unit_price) }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">金额</span>
                <span class="info-value">{{ formatMoney(detail.amount) }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell info-cell--full">
                <span class="info-label">维修操作内容</span>
                <span class="info-value">{{ detail.repair_content || '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- V1.1 新增：附加信息（completed 状态展示，metadata 独立区块） -->
        <div v-if="showMetadata" class="section">
          <h3 class="section-title">附加信息</h3>
          <div class="info-grid">
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">维修结果</span>
                <span class="info-value">{{ detail.metadata?.repair_result || '-' }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">维修方式</span>
                <span class="info-value">{{ detail.metadata?.repair_method || '-' }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">保修期</span>
                <span class="info-value">{{ detail.metadata?.warranty_period || '-' }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">维修时长</span>
                <span class="info-value">{{ detail.metadata?.repair_duration != null ? `${detail.metadata.repair_duration} 分钟` : '-' }}</span>
              </div>
              <div class="info-cell info-cell--full">
                <span class="info-label">额外备注</span>
                <span class="info-value">{{ detail.metadata?.extra_remark || '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="imageUrls.length" class="section">
          <h3 class="section-title">故障图片</h3>
          <el-image
            v-for="(url, index) in imageUrls"
            :key="url"
            :src="url"
            :preview-src-list="imageUrls"
            :initial-index="index"
            fit="cover"
            class="preview-img"
            preview-teleported
          />
        </div>

        <div v-if="receiptUrls.length" class="section">
          <h3 class="section-title">收据图片</h3>
          <el-image
            v-for="(url, index) in receiptUrls"
            :key="url"
            :src="url"
            :preview-src-list="receiptUrls"
            :initial-index="index"
            fit="cover"
            class="preview-img"
            preview-teleported
          />
        </div>

        <div class="section">
          <h3 class="section-title">处理时间轴</h3>
          <el-timeline>
            <el-timeline-item
              v-for="item in detail.timeline"
              :key="item.id"
              :timestamp="formatDateTime(item.created_at)"
              placement="top"
            >
              <div class="timeline-item">
                <div class="timeline-head">
                  <span class="timeline-action">{{ item.action_label }}</span>
                  <span class="timeline-operator">{{ item.operator_name }}</span>
                  <span v-if="item.from_status || item.to_status" class="timeline-status">
                    {{ item.from_status ? `${statusLabelMap[item.from_status] ?? item.from_status} → ` : '' }}{{ statusLabelMap[item.to_status ?? ''] ?? item.to_status }}
                  </span>
                </div>
                <div v-if="item.remark" class="timeline-remark">{{ item.remark }}</div>
              </div>
            </el-timeline-item>
          </el-timeline>
        </div>
      </template>
    </el-card>

    <!-- 操作区 -->
    <div v-if="detail?.available_actions?.length" class="action-bar">
      <el-button
        v-for="action in detail.available_actions"
        :key="action.action"
        :type="action.action === 'reject' ? 'danger' : 'primary'"
        size="large"
        @click="handleAction(action)"
      >
        {{ action.label }}
      </el-button>
    </div>

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

    <!-- 完工弹窗 -->
    <el-dialog v-model="completeDialogVisible" title="完工" width="640px">
      <el-form label-position="top">
        <!-- 第一栏：维修备注 -->
        <div class="complete-block-title">维修备注（必填，≤200 字）</div>
        <el-form-item>
          <el-input
            v-model="completeRemark"
            type="textarea"
            :rows="3"
            :maxlength="200"
            show-word-limit
            placeholder="请输入维修过程及结果，如：已更换电源模块，恢复正常使用"
          />
        </el-form-item>

        <!-- 第二栏：对账信息 -->
        <div class="complete-block-title">对账信息（必填）</div>
        <div class="finance-row">
          <el-form-item label="数量（≥0）">
            <el-input-number v-model="completeQuantity" :min="0" :precision="0" controls-position="right" />
          </el-form-item>
          <el-form-item label="单价（元，≥0）">
            <el-input-number v-model="completeUnitPrice" :min="0" :precision="2" controls-position="right" />
          </el-form-item>
        </div>
        <el-form-item label="维修操作内容（必填，≤200 字）">
          <el-input
            v-model="completeRepairContent"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
            placeholder="如：更换台式机电源模块，长城600W"
          />
        </el-form-item>

        <!-- 第三栏：收据图片 -->
        <div class="complete-block-title">收据图片（最多 3 张）</div>
        <el-form-item>
          <el-upload
            v-model:file-list="receiptFileList"
            list-type="picture-card"
            accept=".jpg,.jpeg,.png,.webp"
            :http-request="handleReceiptUpload"
            :before-upload="beforeReceiptUpload"
            :on-remove="handleReceiptRemove"
            :on-preview="handleReceiptPreview"
          >
            <el-icon><Plus /></el-icon>
          </el-upload>
        </el-form-item>

        <!-- 第四栏：维修附加信息 -->
        <div class="complete-block-title">维修附加信息（可选）</div>
        <div class="finance-row">
          <el-form-item label="维修结果">
            <el-select v-model="metaRepairResult" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairResultOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
          <el-form-item label="维修方式">
            <el-select v-model="metaRepairMethod" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairMethodOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
        </div>
        <div class="finance-row">
          <el-form-item label="保修期">
            <el-input v-model="metaWarrantyPeriod" placeholder="如：3个月" class="full-width" />
          </el-form-item>
          <el-form-item label="维修时长（分钟）">
            <el-input-number v-model="metaRepairDuration" :min="0" :precision="0" controls-position="right" class="full-width" />
          </el-form-item>
        </div>
        <el-form-item label="额外备注">
          <el-input
            v-model="metaExtraRemark"
            type="textarea"
            :rows="2"
            :maxlength="200"
            placeholder="选填"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="completeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="completing" @click="handleCompleteSubmit">
          确认完工
        </el-button>
      </template>
    </el-dialog>

    <!-- 修改对账信息弹窗（5.6.1） -->
    <el-dialog v-model="financeDialogVisible" title="修改对账信息" width="560px">
      <el-form label-position="top">
        <el-form-item label="工单号">
          <span class="finance-order-no">{{ detail?.order_no }}</span>
        </el-form-item>
        <div class="finance-row">
          <el-form-item label="数量">
            <el-input-number v-model="financeQuantity" :min="0" :precision="0" controls-position="right" class="full-width" />
          </el-form-item>
          <el-form-item label="单价（元）">
            <el-input-number v-model="financeUnitPrice" :min="0" :precision="2" controls-position="right" class="full-width" />
          </el-form-item>
          <el-form-item label="金额（只读预览）">
            <span class="finance-amount">{{ financeAmount }}</span>
          </el-form-item>
        </div>
        <el-form-item label="维修操作内容（≤200 字）">
          <el-input
            v-model="financeRepairContent"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
          />
        </el-form-item>
        <el-divider content-position="left">维修附加信息</el-divider>
        <div class="finance-row">
          <el-form-item label="维修结果">
            <el-select v-model="financeMetaRepairResult" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairResultOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
          <el-form-item label="维修方式">
            <el-select v-model="financeMetaRepairMethod" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairMethodOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
        </div>
        <div class="finance-row">
          <el-form-item label="保修期">
            <el-input v-model="financeMetaWarrantyPeriod" placeholder="如：6个月" class="full-width" />
          </el-form-item>
          <el-form-item label="维修时长（分钟）">
            <el-input-number v-model="financeMetaRepairDuration" :min="0" :precision="0" controls-position="right" class="full-width" />
          </el-form-item>
        </div>
        <el-form-item label="额外备注">
          <el-input
            v-model="financeMetaExtraRemark"
            type="textarea"
            :rows="2"
            :maxlength="200"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="financeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="updatingFinance" @click="handleUpdateFinance">
          保存修改
        </el-button>
      </template>
    </el-dialog>

    <el-image-viewer
      v-if="previewVisible"
      :url-list="[previewUrl]"
      @close="previewVisible = false"
    />
  </div>
</template>

<style scoped>
:deep(.el-descriptions__label) {
  white-space: nowrap;
}

.info-grid {
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.info-row {
  display: flex;
  flex-wrap: wrap;
  border-top: 1px solid var(--el-border-color);
  align-items: stretch;
}

.info-row:first-child {
  border-top: none;
}

.info-cell {
  display: flex;
  border-right: 1px solid var(--el-border-color);
  align-items: stretch;
}

.info-cell:last-child {
  border-right: none;
}

.info-cell--full {
  flex: 1;
}

.info-label {
  padding: 12px 16px;
  font-weight: 700;
  color: var(--el-text-color-regular);
  background-color: var(--el-fill-color-light);
  white-space: nowrap;
  border-right: 1px solid var(--el-border-color);
  display: flex;
  align-items: center;
}

.info-value {
  padding: 12px 16px;
  color: var(--el-text-color-primary);
  word-break: break-all;
  display: flex;
  align-items: center;
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.detail-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.detail-status {
  margin-left: auto;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 28px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #606266;
}

.urgency-text {
  color: #f56c6c;
  font-weight: 600;
}

.reject-alert {
  margin-bottom: 16px;
}

.section {
  margin-top: 20px;
}

.section-title {
  margin: 0 0 12px;
  font-size: 15px;
  color: #303133;
  border-left: 3px solid #409eff;
  padding-left: 8px;
}

.preview-img {
  width: 120px;
  height: 120px;
  border-radius: 6px;
  margin-right: 8px;
  cursor: zoom-in;
}

.timeline-item {
  background-color: #f5f7fa;
  border-radius: 6px;
  padding: 10px 14px;
}

.timeline-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.timeline-action {
  font-weight: 600;
  color: #303133;
}

.timeline-operator {
  color: #606266;
  font-size: 13px;
}

.timeline-status {
  color: #909399;
  font-size: 12px;
}

.timeline-remark {
  margin-top: 6px;
  color: #606266;
  font-size: 13px;
}

.finance-row {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.finance-row .el-form-item {
  flex: 1;
  min-width: 150px;
}

.full-width {
  width: 100%;
}

.finance-order-no {
  color: #303133;
  font-weight: 600;
}

.finance-amount {
  color: #f56c6c;
  font-weight: 600;
}

.complete-block-title {
  font-weight: 700;
  font-size: 15px;
  color: #303133;
  margin: 4px 0 10px;
  padding-left: 8px;
  border-left: 3px solid #409eff;
  line-height: 1.4;
}

.action-bar {
  position: sticky;
  bottom: 16px;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 20px;
  background-color: #fff;
  border-radius: 8px;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.06);
}
</style>
