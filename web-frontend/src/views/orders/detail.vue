<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type UploadRequestOptions, type UploadUserFile } from 'element-plus'
import { adminAPI } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { AvailableAction, OrderDetail, OrderMetadata } from '@/types'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const orderId = route.params.id as string

const loading = ref(false)
const detail = ref<OrderDetail | null>(null)

// 状态中文映射（时间轴流转展示用；卡片文案以后端 status_label 为准）
const statusLabelMap: Record<string, string> = {
  draft: '草稿',
  reported: '已上报',
  pending_accept: '待接单',
  processing: '处理中',
  completed: '已处理',
  cancelled: '已取消'
}

const statusTagType: Record<string, string> = {
  draft: 'info',
  reported: 'danger',
  pending_accept: 'warning',
  processing: 'primary',
  completed: 'success',
  cancelled: 'info'
}

const imageUrls = computed(() => detail.value?.images.map(i => i.url) ?? [])
const receiptUrls = computed(() => detail.value?.receipts.map(r => r.url) ?? [])

/** 对账信息是否展示（completed 状态） */
const showFinance = computed(() => detail.value?.status === 'completed')

/** 附加信息是否展示（completed 状态且有 metadata 数据） */
const showMetadata = computed(() => {
  const d = detail.value
  if (!d || d.status !== 'completed') return false
  const m = d.metadata
  if (!m) return false
  return !!(m.repair_result || m.repair_method || m.warranty_period || m.repair_duration != null)
})

/** 金额格式化：保留两位小数 */
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

// ── 动作可见性：店方全量；单位审核员仅 audit（reported 审核通过）与 reported 退回 ──
const userStore = useUserStore()
const visibleActions = computed<AvailableAction[]>(() => {
  const actions = detail.value?.available_actions ?? []
  if (userStore.isStoreStaff) return actions
  return actions.filter(
    a =>
      a.action === 'audit' ||
      (a.action === 'reject' && detail.value?.status === 'reported')
  )
})

// ---- 简单操作：审核通过 / 接单 / 重新打开（可带确认文案） ----
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
  if (action.action === 'audit') {
    await adminAPI.auditOrder(orderId)
  } else if (action.action === 'accept') {
    await adminAPI.acceptOrder(orderId)
  } else if (action.action === 'reopen') {
    await adminAPI.reopenOrder(orderId)
  }
  ElMessage.success(`${action.label}成功`)
  await fetchDetail()
}

// ---- 退回（弹窗填写原因，≥2 字） ----
const rejectDialogVisible = ref(false)
const rejectReason = ref('')
const rejectMinLength = ref(2)
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

const openRejectDialog = (action: AvailableAction) => {
  rejectMinLength.value = action.reason_min_length ?? 2
  rejectReason.value = ''
  rejectDialogVisible.value = true
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

// 对账信息（必填；需求③：数量默认 1）
const completeQuantity = ref<number>(1)
const completeUnitPrice = ref<number>(0)
const completeRepairContent = ref('')
// 维修附加信息（metadata）
const metaRepairResult = ref('')
const metaRepairMethod = ref('')
const metaWarrantyPeriod = ref('')
const metaRepairDuration = ref<number | undefined>(undefined)

const repairResultOptions = ['完全修复', '部分修复', '无法修复']
const repairMethodOptions = ['上门维修', '返店维修', '远程协助']

// 结单结果（req② 三按钮）: 完成 / 客户原因取消 / 不能修
const finishResult = ref<'complete' | 'customer_cancelled' | 'unrepairable'>('complete')
const finishResultOptions = [
  { value: 'complete', label: '完成' },
  { value: 'customer_cancelled', label: '客户原因取消' },
  { value: 'unrepairable', label: '不能修' }
] as const

// 签字模式（仅 result=complete 生效）: 已签字 / 稍后补签
const signMode = ref<'signed' | 'pending_sign'>('signed')

// 补签弹窗状态
const signDialogVisible = ref(false)
const signing = ref(false)
const signFileList = ref<UploadUserFile[]>([])
const signUrls = ref<string[]>([])
const signUrlMap = new Map<number, string>()
const signQuantity = ref<number>(1)
const signUnitPrice = ref<number>(0)
const signRepairContent = ref('')
const signRemark = ref('')
// 补签补登：维修附加信息（与完工弹窗一致）
const signMetaRepairResult = ref('')
const signMetaRepairMethod = ref('')
const signMetaWarrantyPeriod = ref('')
const signMetaRepairDuration = ref<number | undefined>(undefined)

const completeButtonLabel = computed(() => {
  if (finishResult.value === 'customer_cancelled') return '确认客户取消'
  if (finishResult.value === 'unrepairable') return '确认不能修'
  return signMode.value === 'pending_sign' ? '暂留处理中（稍后补签）' : '确认完工'
})

// 维修备注：完成类选填；客户取消/不能修类必填（与「额外备注」语义区分，避免冗余必填）
const completeRemarkRequired = computed(() => finishResult.value !== 'complete')
const completeRemarkTitle = computed(() =>
  completeRemarkRequired.value ? '维修备注（必填，≤200 字）' : '维修备注（选填，≤200 字）'
)
const completeRemarkPlaceholder = computed(() => {
  if (finishResult.value === 'customer_cancelled')
    return '必填：请说明客户取消的原因，如：客户已自行解决、改由厂家保修、重复报修等'
  if (finishResult.value === 'unrepairable')
    return '必填：请说明无法维修的原因，如：配件已停产、设备损坏无维修价值等'
  return '选填：可简述维修情况，如：已更换电源模块并测试正常'
})

const buildMetadata = () => ({
  repair_result: metaRepairResult.value || undefined,
  repair_method: metaRepairMethod.value || undefined,
  warranty_period: metaWarrantyPeriod.value || undefined,
  repair_duration: metaRepairDuration.value
})

const handleCompleteSubmit = async () => {
  const remark = completeRemark.value.trim()
  if (remark.length > 200) {
    ElMessage.warning('维修备注不能超过 200 字')
    return
  }
  if (completeRemarkRequired.value && !remark) {
    const tip =
      finishResult.value === 'customer_cancelled'
        ? '请填写维修备注：说明客户取消的原因'
        : '请填写维修备注：说明无法维修的原因'
    ElMessage.warning(tip)
    return
  }

  const isComplete = finishResult.value === 'complete'

  // 客户原因取消 / 不能修：终态，不要求收据/金额/维修内容
  let quantity = 1
  let unitPrice = 0
  let repairContent = ''
  let receiptsPayload: string[] = []
  let signModePayload = 'signed'
  if (isComplete) {
    quantity = completeQuantity.value ?? 1
    unitPrice = completeUnitPrice.value ?? 0
    if (quantity < 0) {
      ElMessage.warning('数量不能为负')
      return
    }
    if (unitPrice < 0) {
      ElMessage.warning('单价不能为负')
      return
    }
    signModePayload = signMode.value
    if (signMode.value === 'signed') {
      // 已签字：收据/维修内容必填
      repairContent = completeRepairContent.value.trim()
      if (!repairContent) {
        ElMessage.warning('请填写维修操作内容')
        return
      }
      if (repairContent.length > 200) {
        ElMessage.warning('维修操作内容不能超过 200 字')
        return
      }
      if (receipts.value.length === 0) {
        ElMessage.warning('请上传至少一张收据图片')
        return
      }
      receiptsPayload = receipts.value
    } else {
      // 稍后补签：收据/维修内容选填，仅做记录；上传的收据一并提交，避免信息丢失
      repairContent = completeRepairContent.value.trim()
      receiptsPayload = receipts.value
    }
  }

  completing.value = true
  try {
    await adminAPI.completeOrder(orderId, {
      remark,
      receipts: receiptsPayload,
      quantity,
      unit_price: unitPrice,
      repair_content: repairContent,
      metadata: buildMetadata(),
      result: finishResult.value,
      sign_mode: signModePayload
    })
    const msg =
      finishResult.value !== 'complete'
        ? finishResult.value === 'unrepairable'
          ? '已标记为不能修'
          : '已标记为客户原因取消'
        : signModePayload === 'pending_sign'
          ? '已暂留处理中（待补签）'
          : '完工成功'
    ElMessage.success(msg)
    completeDialogVisible.value = false
    await fetchDetail()
  } finally {
    completing.value = false
  }
}

const openSignDialog = () => {
  // 预填已存的收据图片（如稍后补签时曾在完工弹窗上传过），避免信息展示不完全
  const existingReceipts = detail.value?.receipts ?? []
  signFileList.value = existingReceipts.map((r, idx) => ({
    uid: -(idx + 1),
    name: `receipt-${idx + 1}.jpg`,
    url: r.url,
    status: 'success' as const
  }))
  signUrls.value = existingReceipts.map(r => r.url)
  signUrlMap.clear()
  existingReceipts.forEach((r, idx) => signUrlMap.set(-(idx + 1), r.url))
  signQuantity.value = detail.value?.quantity ?? 1
  signUnitPrice.value = detail.value?.unit_price ?? 0
  signRepairContent.value = detail.value?.repair_content ?? ''
  // 回显之前已写的维修备注（如稍后补签时曾在完工弹窗填写过）
  signRemark.value = detail.value?.remark ?? ''
  signMetaRepairResult.value = detail.value?.metadata?.repair_result ?? ''
  signMetaRepairMethod.value = detail.value?.metadata?.repair_method ?? ''
  signMetaWarrantyPeriod.value = detail.value?.metadata?.warranty_period ?? ''
  signMetaRepairDuration.value = detail.value?.metadata?.repair_duration ?? undefined
  signDialogVisible.value = true
}

const beforeSignUpload = (rawFile: File) => {
  const validTypes = ['image/jpeg', 'image/png', 'image/webp']
  if (!validTypes.includes(rawFile.type)) {
    ElMessage.error('仅支持 jpg/png/webp 格式')
    return false
  }
  if (rawFile.size > 5 * 1024 * 1024) {
    ElMessage.error('单张图片不能超过 5MB')
    return false
  }
  if (signFileList.value.length >= 3) {
    ElMessage.warning('收据图片/签字单最多上传 3 张')
    return false
  }
  return true
}

const handleSignUpload = async (options: UploadRequestOptions) => {
  try {
    const res = await adminAPI.uploadReceipt(orderId, options.file)
    signUrls.value.push(res.data.url)
    signUrlMap.set(options.file.uid, res.data.url)
    const item = signFileList.value.find(f => f.uid === options.file.uid)
    if (item) {
      item.url = res.data.url
      item.status = 'success'
    }
    options.onSuccess(res.data)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '上传失败')
    const item = signFileList.value.find(f => f.uid === options.file.uid)
    if (item) item.status = 'fail'
  }
}

const handleSignRemove = (file: UploadUserFile) => {
  const uid = file.uid
  if (uid === undefined) return
  const url = signUrlMap.get(uid)
  if (url) {
    signUrls.value = signUrls.value.filter(u => u !== url)
    signUrlMap.delete(uid)
  }
}

const handleSignSubmit = async () => {
  if (signUrls.value.length === 0) {
    ElMessage.warning('请先上传客户签字单')
    return
  }
  const remark = signRemark.value.trim()
  if (remark.length > 200) {
    ElMessage.warning('维修备注不能超过 200 字')
    return
  }
  const meta: OrderMetadata = {}
  if (signMetaRepairResult.value) meta.repair_result = signMetaRepairResult.value
  if (signMetaRepairMethod.value) meta.repair_method = signMetaRepairMethod.value
  if (signMetaWarrantyPeriod.value) meta.warranty_period = signMetaWarrantyPeriod.value
  if (signMetaRepairDuration.value !== undefined) meta.repair_duration = signMetaRepairDuration.value
  signing.value = true
  try {
    await adminAPI.signOrder(orderId, {
      remark,
      receipts: signUrls.value,
      quantity: signQuantity.value ?? 1,
      unit_price: signUnitPrice.value ?? 0,
      repair_content: signRepairContent.value.trim(),
      metadata: meta
    })
    ElMessage.success('已正式完工')
    signDialogVisible.value = false
    await fetchDetail()
  } finally {
    signing.value = false
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
  finishResult.value = 'complete'
  signMode.value = 'signed'
  receipts.value = []
  receiptFileList.value = []
  receiptUrlMap.clear()
  completeQuantity.value = 1
  completeUnitPrice.value = 0
  completeRepairContent.value = ''
  metaRepairResult.value = ''
  metaRepairMethod.value = ''
  metaWarrantyPeriod.value = ''
  metaRepairDuration.value = undefined
  completeDialogVisible.value = true
}

// ---- 修改对账信息（completed 状态店方） ----
const financeDialogVisible = ref(false)
const financeQuantity = ref<number>(1)
const financeUnitPrice = ref<number>(0)
const financeRepairContent = ref('')
const financeRemark = ref('')
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
const updatingFinance = ref(false)

const openFinanceDialog = () => {
  const d = detail.value
  financeQuantity.value = d?.quantity ?? 1
  financeUnitPrice.value = d?.unit_price ?? 0
  financeRepairContent.value = d?.repair_content ?? ''
  financeRemark.value = d?.remark ?? ''
  financeMetaRepairResult.value = d?.metadata?.repair_result ?? ''
  financeMetaRepairMethod.value = d?.metadata?.repair_method ?? ''
  financeMetaWarrantyPeriod.value = d?.metadata?.warranty_period ?? ''
  financeMetaRepairDuration.value = d?.metadata?.repair_duration ?? undefined
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
  const remark = financeRemark.value.trim()
  if (remark.length > 200) {
    ElMessage.warning('维修备注不能超过 200 字')
    return
  }
  // 组装非空字段（后端字段可选，至少传一个）
  const data: {
    quantity?: number
    unit_price?: number
    repair_content?: string
    remark?: string
    metadata?: Record<string, unknown>
  } = {}
  if (quantity !== undefined) data.quantity = quantity
  if (unitPrice !== undefined) data.unit_price = unitPrice
  if (financeRepairContent.value.trim()) data.repair_content = financeRepairContent.value.trim()
  data.remark = remark
  const meta: Record<string, unknown> = {}
  if (financeMetaRepairResult.value) meta.repair_result = financeMetaRepairResult.value
  if (financeMetaRepairMethod.value) meta.repair_method = financeMetaRepairMethod.value
  if (financeMetaWarrantyPeriod.value) meta.warranty_period = financeMetaWarrantyPeriod.value
  if (financeMetaRepairDuration.value !== undefined) meta.repair_duration = financeMetaRepairDuration.value
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

// ---- 动作分发 ----
const handleAction = (action: AvailableAction) => {
  if (action.action === 'reject') {
    openRejectDialog(action)
  } else if (action.action === 'complete') {
    openCompleteDialog()
  } else if (action.action === 'sign') {
    openSignDialog()
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
          <el-tag v-if="detail" :type="(detail.display_tag_type as string) || statusTagType[detail.status]" class="detail-status">
            {{ detail.display_tag || detail.status_label }}
          </el-tag>
        </div>
      </template>

      <template v-if="detail">
        <div class="meta-row">
          <span class="meta-item">
            <el-icon><OfficeBuilding /></el-icon>
            企业：{{ detail.enterprise_name }}
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
          <span v-if="detail.auditor_name" class="meta-item">
            <el-icon><Stamp /></el-icon>
            审核人：{{ detail.auditor_name }}
            <span class="meta-sub">（{{ formatDateTime(detail.audited_at) }}）</span>
          </span>
          <span v-if="detail.repairer_name" class="meta-item">
            <el-icon><User /></el-icon>
            接单/维修：{{ detail.repairer_name }}
          </span>
        </div>

        <el-alert
          v-if="detail.status === 'rejected' && detail.reject_reason"
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
              <div class="info-cell">
                <span class="info-label">报修企业</span>
                <span class="info-value">{{ detail.enterprise_name }}</span>
              </div>
            </div>
            <div class="info-row">
              <div class="info-cell">
                <span class="info-label">项目大类</span>
                <span class="info-value">{{ detail.category_name || '-' }}</span>
              </div>
              <div class="info-cell">
                <span class="info-label">项目属性</span>
                <span class="info-value">{{ detail.property_name || '-' }}</span>
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

        <!-- 对账信息（completed 状态展示） -->
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
            <div v-if="detail.remark" class="info-row">
              <div class="info-cell info-cell--full">
                <span class="info-label">维修备注</span>
                <span class="info-value">{{ detail.remark }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 附加信息（completed 状态 metadata 独立区块） -->
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

    <!-- 操作区（按钮由 visibleActions 驱动；退回/完工/补签等均在同行展示） -->
    <div v-if="visibleActions.length" class="action-bar">
      <el-button
        v-for="action in visibleActions"
        :key="action.action"
        :type="action.action === 'reject' ? 'danger' : action.action === 'sign' ? 'warning' : 'primary'"
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
            :placeholder="`请输入退回原因，至少 ${rejectMinLength} 个字`"
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
        <el-form-item label="结单结果">
          <el-radio-group v-model="finishResult">
            <el-radio-button
              v-for="opt in finishResultOptions"
              :key="opt.value"
              :value="opt.value"
            >{{ opt.label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="finishResult === 'complete'">
          <el-form-item label="客户签字">
            <el-radio-group v-model="signMode">
              <el-radio-button value="signed">客户已签字</el-radio-button>
              <el-radio-button value="pending_sign">暂时无法签字，稍后补签</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <template v-if="signMode === 'signed' || signMode === 'pending_sign'">
            <el-alert
              v-if="signMode === 'pending_sign'"
              type="warning"
              :closable="false"
              show-icon
              title="该单会暂留处理中，不会扣钱，可随时补传签字单后正式完工"
              description="选择稍后补签后，工单保持「待补签」状态，金额等可现在填也可后续补登，不影响客户，也不计入已完工。"
            />
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
          </template>
        </template>

        <!-- 维修备注：移动到对账信息/收据图片之后；取消/不能修类无对账块时也展示 -->
        <div class="complete-block-title">{{ completeRemarkTitle }}</div>
        <el-form-item>
          <el-input
            v-model="completeRemark"
            type="textarea"
            :rows="3"
            :maxlength="200"
            show-word-limit
            :placeholder="completeRemarkPlaceholder"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="completeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="completing" @click="handleCompleteSubmit">
          {{ completeButtonLabel }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 补签正式完工弹窗 -->
    <el-dialog v-model="signDialogVisible" title="上传签字单，正式完工" width="640px">
      <el-form label-position="top">
        <div class="complete-block-title">收据图片 / 签字单（必传，最多 3 张）</div>
        <el-form-item>
          <el-upload
            v-model:file-list="signFileList"
            list-type="picture-card"
            accept=".jpg,.jpeg,.png,.webp"
            :http-request="handleSignUpload"
            :before-upload="beforeSignUpload"
            :on-remove="handleSignRemove"
            :on-preview="handleReceiptPreview"
          >
            <el-icon><Plus /></el-icon>
          </el-upload>
        </el-form-item>
        <el-alert
          type="success"
          :closable="false"
          show-icon
          title="上传签字单并确认后，工单将正式完工（写完工时间），可在此补登金额。"
        />
        <div class="complete-block-title">补登对账信息（可选）</div>
        <div class="finance-row">
          <el-form-item label="数量（≥0）">
            <el-input-number v-model="signQuantity" :min="0" :precision="0" controls-position="right" />
          </el-form-item>
          <el-form-item label="单价（元，≥0）">
            <el-input-number v-model="signUnitPrice" :min="0" :precision="2" controls-position="right" />
          </el-form-item>
        </div>
        <el-form-item label="维修操作内容（≤200 字）">
          <el-input
            v-model="signRepairContent"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
            placeholder="如：更换台式机电源模块，长城600W"
          />
        </el-form-item>

        <div class="complete-block-title">维修附加信息（可选，与完工弹窗一致）</div>
        <div class="finance-row">
          <el-form-item label="维修结果">
            <el-select v-model="signMetaRepairResult" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairResultOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
          <el-form-item label="维修方式">
            <el-select v-model="signMetaRepairMethod" placeholder="请选择" clearable class="full-width">
              <el-option v-for="opt in repairMethodOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
        </div>
        <div class="finance-row">
          <el-form-item label="保修期">
            <el-input v-model="signMetaWarrantyPeriod" placeholder="如：3个月" class="full-width" />
          </el-form-item>
          <el-form-item label="维修时长（分钟）">
            <el-input-number v-model="signMetaRepairDuration" :min="0" :precision="0" controls-position="right" class="full-width" />
          </el-form-item>
        </div>

        <div class="complete-block-title">维修备注（选填，≤200 字）</div>
        <el-form-item>
          <el-input
            v-model="signRemark"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
            placeholder="选填：可补充完工说明，如：已当面交付并签字确认"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="signDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="signing" @click="handleSignSubmit">
          确认正式完工
        </el-button>
      </template>
    </el-dialog>

    <!-- 修改对账信息弹窗 -->
    <el-dialog v-model="financeDialogVisible" title="修改对账信息" width="560px">
      <el-form label-position="top">
        <div class="complete-block-title">对账信息</div>
        <div class="finance-row">
          <el-form-item label="数量（≥0）">
            <el-input-number v-model="financeQuantity" :min="0" :precision="0" controls-position="right" class="full-width" />
          </el-form-item>
          <el-form-item label="单价（元，≥0）">
            <el-input-number v-model="financeUnitPrice" :min="0" :precision="2" controls-position="right" class="full-width" />
          </el-form-item>
        </div>
        <div class="finance-amount-line">金额（只读预览）：{{ financeAmount }}</div>
        <el-form-item label="维修操作内容（≤200 字）">
          <el-input
            v-model="financeRepairContent"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
          />
        </el-form-item>

        <div class="complete-block-title">维修附加信息（可选）</div>
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

        <div class="complete-block-title">维修备注（选填，≤200 字）</div>
        <el-form-item>
          <el-input
            v-model="financeRemark"
            type="textarea"
            :rows="3"
            :maxlength="200"
            show-word-limit
            placeholder="选填：可补充/修正完工说明，如：已当面交付并签字确认"
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

.meta-sub {
  color: #909399;
  font-size: 12px;
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

.finance-amount-line {
  color: #606266;
  font-size: 14px;
  margin: 0 0 10px;
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
