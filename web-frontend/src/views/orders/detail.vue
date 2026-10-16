<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type UploadRequestOptions, type UploadUserFile } from 'element-plus'
import { adminAPI } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { AvailableAction, OrderDetail, OrderMetadata } from '@/types'
import { formatDateTime, formatShortDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const orderId = route.params.id as string

const loading = ref(false)
const detail = ref<OrderDetail | null>(null)

const imageUrls = computed(() => detail.value?.images.map(i => i.url) ?? [])
const receiptUrls = computed(() => detail.value?.receipts.map(r => r.url) ?? [])

// ── 新增：详情页状态胶囊（四色体系，不用 green 深浅区分） ──
const detailStatusClassMap: Record<string, string> = {
  draft: 'st-gray',
  reported: 'st-amber',
  pending_accept: 'st-orange',
  processing: 'st-blue',
  completed: 'st-teal',
  cancelled: 'st-gray',
  rejected: 'st-red'
}
function statusViewOf(d?: OrderDetail | null) {
  if (!d) return { text: '', cls: 'st-gray' }
  if (d.signature_status === 'pending_sign') return { text: '待补签', cls: 'st-pending-sign' }
  return { text: d.display_tag || d.status_label, cls: detailStatusClassMap[d.status] || 'st-gray' }
}

// 对账信息卡不再按角色隐藏（V1.7.1 起审核员/报修人也可查看金额）；
// 店方专属操作（编辑对账等）由 visibleActions 的 update_finance 控制，无需本地角色判断

/** 对账信息卡：有对账数据（数量/单价/金额/维修内容/备注/收据/附加信息）才展示，所有角色可见 */
const hasFinanceData = computed(() => {
  const d = detail.value
  if (!d) return false
  return (
    (d.amount && d.amount > 0) ||
    (d.quantity && d.quantity > 0) ||
    d.unit_price > 0 ||
    !!d.repair_content ||
    !!d.remark ||
    (d.receipts && d.receipts.length > 0) ||
    !!(d.metadata &&
      (d.metadata.repair_result ||
        d.metadata.repair_method ||
        d.metadata.warranty_period ||
        d.metadata.repair_duration != null))
  )
})
/** 早期状态（草稿/待审核/待接单）尚未产生费用，强制隐藏对账信息卡 */
const noFinanceStatuses = ['draft', 'reported', 'pending_accept']
/** 处理中/待接单下，对账数据是否仍为默认值（金额 0、数量 0/1、单价 0，且无任何人工填写内容）→ 一并隐藏 */
const isFinanceDefault = computed(() => {
  const d = detail.value
  if (!d) return true
  const m = d.metadata
  return (
    !d.amount &&
    !d.unit_price &&
    (!d.quantity || d.quantity <= 1) &&
    !d.repair_content &&
    !d.remark &&
    !(d.receipts && d.receipts.length > 0) &&
    !(m &&
      (m.repair_result ||
        m.repair_method ||
        m.warranty_period ||
        m.repair_duration != null))
  )
})
const showFinance = computed(() => {
  const st = detail.value?.status ?? ''
  if (noFinanceStatuses.includes(st)) return false
  if (!hasFinanceData.value) return false
  if ((st === 'processing' || st === 'pending_accept') && isFinanceDefault.value) return false
  return true
})
const financeEditVisible = computed(() =>
  visibleActions.value.some(a => a.action === 'update_finance')
)

/** 对账信息卡：是否存在维修附加信息（结果/保修/时长） */
const hasRepairMeta = computed(() => {
  const m = detail.value?.metadata
  return !!(m && (m.repair_result || m.warranty_period || m.repair_duration != null))
})

/** 故障描述折叠（超 60 字折叠，点「查看全文」展开） */
const descExpanded = ref(false)
const isLongDesc = computed(() => (detail.value?.description?.length ?? 0) > 60)

/** 处理记录默认折叠 */
const timelineExpanded = ref(false)
/** 对账信息「查看明细」折叠 */
const financeDetailExpanded = ref(false)

/** 拨打电话链接 */
// contact 存的是「姓名 电话」合并串（如 "Clover 15957761287"），取号码用于显示与拨号，避免姓名重复
const contactPhone = computed(() => {
  const raw = detail.value?.contact ?? ''
  const m = raw.match(/\d[\d\s-]{5,}/)
  return m ? m[0].replace(/[\s-]/g, '') : raw.trim()
})
/** 报修人真实姓名：从 contact（"姓名 电话"合并串）中剔除号码后取姓名部分 */
const contactName = computed(() => {
  const raw = detail.value?.contact ?? ''
  const m = raw.match(/\d[\d\s-]{5,}/)
  const name = m ? raw.slice(0, m.index).trim() : raw.trim()
  return name
})
/** 报修人账户名（系统账号昵称） */
const accountName = computed(() => detail.value?.reporter?.nickname ?? '')
/** 展示用的报修人姓名：优先真实姓名，缺省回退账户名 */
const reporterName = computed(() => contactName.value || accountName.value)
/** 头像下方是否展示账户名：仅当存在真实姓名且与账户名不同（避免重复） */
const showAccountName = computed(
  () => !!contactName.value && contactName.value !== accountName.value
)
const telHref = computed(() => `tel:${contactPhone.value}`)

/** 紧急程度标签样式（小号圆角标签，不用纯文本） */
const urgencyTagClass = computed(() => {
  const u = detail.value?.urgency
  if (u === 'very_urgent') return 'ut-very'
  if (u === 'urgent') return 'ut-urgent'
  return 'ut-normal'
})

/** 复制工单号到剪贴板 */
const copyOrderNo = async () => {
  const no = detail.value?.order_no
  if (!no) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(no)
    } else {
      const ta = document.createElement('textarea')
      ta.value = no
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    ElMessage.success('工单号已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

const formatMoney = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  return Number(value).toFixed(2)
}

/** 状态中文标签（含 rejected，时间轴状态流转胶囊用） */
function statusLabel(s?: string | null): string {
  if (!s) return '-'
  const map: Record<string, string> = {
    draft: '草稿',
    reported: '已上报',
    pending_accept: '待接单',
    processing: '处理中',
    completed: '已处理',
    cancelled: '已取消',
    rejected: '已退回'
  }
  return map[s] ?? s
}

/** 时间轴节点配色（按动作类型上色：草稿灰 / 上报蓝 / 接单橙 / 完工绿） */
function tlNodeClass(action?: string): string {
  switch (action) {
    case 'submit':
      return 'tl-blue'
    case 'accept':
    case 'reopen':
      return 'tl-orange'
    case 'complete':
    case 'audit':
    case 'sign':
      return 'tl-teal'
    case 'reject':
    case 'cancel':
      return 'tl-red'
    case 'return_to_pool':
      return 'tl-orange'
    case 'cleanup_receipts':
      return 'tl-gray'
    default:
      return 'tl-gray'
  }
}

const fetchDetail = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getOrderDetail(orderId)
    detail.value = res.data
    // 进详情页即视为已读：清除首页待办看板上该工单的未读红点（失败不阻塞展示）
    adminAPI.markOrderRead(orderId).catch(() => {})
  } finally {
    loading.value = false
  }
}

// ── 动作可见性：店方全量；单位审核员仅 audit（reported 审核通过）与 reported/pending_accept 退回 ──
// V1.9：放开审核员在 pending_accept 的「退回」（后端 CanAccessOrder 已放行，仅 Web 此前裁剪了按钮）
const userStore = useUserStore()
const visibleActions = computed<AvailableAction[]>(() => {
  const actions = detail.value?.available_actions ?? []
  if (userStore.isStoreStaff) return actions
  return actions.filter(
    a =>
      a.action === 'audit' ||
      (a.action === 'reject' &&
        (detail.value?.status === 'reported' || detail.value?.status === 'pending_accept'))
  )
})

// ── 底部操作栏：按状态/角色拆分「主操作 / 弱化驳回 / 次级操作」 ──
const primaryAction = computed(() =>
  visibleActions.value.find(a => ['accept', 'audit', 'complete', 'sign'].includes(a.action))
)
const rejectAction = computed(() =>
  visibleActions.value.find(a => a.action === 'reject')
)
const secondaryActions = computed(() =>
  visibleActions.value.filter(
    a =>
      !['accept', 'audit', 'complete', 'sign', 'reject', 'update_finance'].includes(a.action)
  )
)
// 操作卡片标题：有「退回」（审核员视角）时叫审核操作，否则叫工单操作
const actionCardTitle = computed(() => (rejectAction.value ? '审核操作' : '工单操作'))

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

// ---- 退单回池（弹窗填写原因，≥2 字；V1.9） ----
const returnDialogVisible = ref(false)
const returnReason = ref('')
const returnMinLength = ref(2)
const returning = ref(false)

const handleReturnSubmit = async () => {
  const reason = returnReason.value.trim()
  if (reason.length < returnMinLength.value) {
    ElMessage.warning(`退单原因至少 ${returnMinLength.value} 个字`)
    return
  }
  if (reason.length > 200) {
    ElMessage.warning('退单原因不能超过 200 字')
    return
  }
  returning.value = true
  try {
    await adminAPI.returnOrder(orderId, reason)
    ElMessage.success('已退单回池')
    returnDialogVisible.value = false
    await fetchDetail()
  } finally {
    returning.value = false
  }
}

const openReturnDialog = (action: AvailableAction) => {
  returnMinLength.value = action.reason_min_length ?? 2
  returnReason.value = ''
  returnDialogVisible.value = true
}

// ---- 完工（弹窗：维修备注 + 对账信息 + 收据上传） ----
const completeDialogVisible = ref(false)
const completeRemark = ref('')
const receipts = ref<string[]>([])
const receiptFileList = ref<UploadUserFile[]>([])
const receiptIdMap = new Map<number, string>()  // uid -> 图片 id（移除时定位）
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
const signIds = ref<string[]>([])   // 存图片 id（即时上传后回填），提交时作为 receipt_ids
const signIdMap = new Map<number, string>()  // uid -> 图片 id
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
      receipt_ids: receiptsPayload,
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
  signIds.value = existingReceipts.map(r => r.id)
  signIdMap.clear()
  existingReceipts.forEach((r, idx) => signIdMap.set(-(idx + 1), r.id))
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
    signIds.value.push(res.data.id)
    signIdMap.set(options.file.uid, res.data.id)
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
  const id = signIdMap.get(uid)
  if (id) {
    signIds.value = signIds.value.filter(x => x !== id)
    signIdMap.delete(uid)
  }
}

const handleSignSubmit = async () => {
  if (signIds.value.length === 0) {
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
      receipt_ids: signIds.value,
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
    receipts.value.push(res.data.id)
    receiptIdMap.set(options.file.uid, res.data.id)
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
  const id = receiptIdMap.get(uid)
  if (id) {
    receipts.value = receipts.value.filter(x => x !== id)
    receiptIdMap.delete(uid)
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
  receiptIdMap.clear()
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
  } else if (action.action === 'return_to_pool') {
    openReturnDialog(action)
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
  <div v-loading="loading" class="detail-page">
    <template v-if="detail">
      <!-- 页头：返回 + 工单号 + 状态胶囊 -->
      <div class="d-header">
        <el-button link @click="router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <span class="d-order-no">{{ detail.order_no }}</span>
      </div>

      <!-- 退回原因提示 -->
      <el-alert
        v-if="detail.status === 'rejected' && detail.reject_reason"
        type="warning"
        :closable="false"
        show-icon
        class="reject-alert"
        :title="`退回原因：${detail.reject_reason}`"
      />

      <div class="xn-grid">
      <div class="xn-col xn-col-main">
      <!-- 第一组 · 现场执行卡（最高优先级） -->
      <section class="xn-card onsite">
        <div class="xn-card-title">
          <el-icon><Location /></el-icon>
          报修信息
          <span class="status-pill" :class="statusViewOf(detail).cls">{{ statusViewOf(detail).text }}</span>
        </div>
        <div class="onsite-place">
          {{ detail.enterprise_name }}：{{ detail.room || '未填写地点' }}
        </div>
        <div v-if="detail.category_name || detail.property_name" class="onsite-tags">
          报修内容：<template v-if="detail.category_name">{{ detail.category_name }}</template><template v-if="detail.category_name && detail.property_name"> → </template><template v-if="detail.property_name">{{ detail.property_name }}</template>
        </div>
        <div class="onsite-row">
          <div class="onsite-reporter">
            <el-avatar :size="26" :src="detail.reporter?.avatar_url" class="or-avatar">
              {{ accountName.charAt(0) }}
            </el-avatar>
            <span v-if="showAccountName" class="or-account">{{ accountName }}</span>
            <span v-if="showAccountName" class="or-divider"></span>
            <div class="or-text">
              <div class="or-name">{{ reporterName || '未知报修人' }}</div>
              <div class="or-phone">{{ contactPhone || '未留联系方式' }}</div>
            </div>
          </div>
          <a v-if="contactPhone" class="call-btn" :href="telHref" title="拨打电话">
            <el-icon><Phone /></el-icon>
          </a>
        </div>
      </section>

      <!-- 第二组 · 故障详情卡 -->
      <section class="xn-card">
        <div class="xn-card-title">
          <el-icon><Warning /></el-icon>
          故障详情
        </div>
        <div class="fault-desc" :class="{ clamp: isLongDesc && !descExpanded }">
          {{ detail.description || '（无故障描述）' }}
        </div>
        <el-button v-if="isLongDesc" link type="primary" class="desc-toggle" @click="descExpanded = !descExpanded">
          {{ descExpanded ? '收起' : '查看全文' }}
        </el-button>
        <div v-if="imageUrls.length" class="img-strip">
          <el-image
            v-for="(url, index) in imageUrls"
            :key="url"
            :src="url"
            :preview-src-list="imageUrls"
            :initial-index="index"
            fit="cover"
            preview-teleported
          />
        </div>
      </section>

      <!-- 第四组 · 对账信息卡（全角色可见；与故障详情同级，左列展示金额/维修操作内容/收据） -->
      <section v-if="showFinance" class="xn-card finance-card">
        <div class="xn-card-title">
          对账信息
          <el-button v-if="financeEditVisible" link type="primary" class="edit-link" @click="openFinanceDialog">
            编辑
          </el-button>
        </div>

        <!-- 核心：金额（主显，左侧）+ 右侧详情箭头（点开才展示单价/数量/维修方式） -->
        <div class="fin-amount">
          <span class="fa-value">¥{{ formatMoney(detail.amount) }}</span>
          <span
            class="fin-detail-toggle"
            :class="{ expanded: financeDetailExpanded }"
            role="button"
            aria-label="展开对账明细"
            @click="financeDetailExpanded = !financeDetailExpanded"
          >
            <el-icon><ArrowRight /></el-icon>
          </span>
        </div>
        <div v-if="financeDetailExpanded" class="fin-detail">
          <div class="kv">
            <span class="k">数量</span>
            <span class="v">{{ detail.quantity ?? '-' }}</span>
          </div>
          <div class="kv">
            <span class="k">单价</span>
            <span class="v">¥{{ formatMoney(detail.unit_price) }}</span>
          </div>
          <div v-if="detail.metadata?.repair_method" class="kv">
            <span class="k">维修方式</span>
            <span class="v">{{ detail.metadata.repair_method }}</span>
          </div>
        </div>

        <!-- 维修操作内容 / 维修备注 / 维修附加信息：任一存在才显示内容区 -->
        <template v-if="detail.repair_content || detail.remark || hasRepairMeta">
          <div class="fin-divider"></div>

          <!-- 维修操作内容 -->
          <div v-if="detail.repair_content" class="fin-section">
            <div class="fin-subtitle">维修操作内容</div>
            <div class="fin-text">{{ detail.repair_content }}</div>
          </div>

          <!-- 维修备注 -->
          <div v-if="detail.remark" class="fin-section">
            <div class="fin-subtitle">维修备注</div>
            <div class="fin-text">{{ detail.remark }}</div>
          </div>

          <!-- 维修附加信息 -->
          <div v-if="hasRepairMeta" class="fin-section">
            <div class="fin-subtitle">维修附加信息</div>
            <div class="fin-text">
              <span v-if="detail.metadata?.repair_result">结果：{{ detail.metadata.repair_result }}</span>
              <span v-if="detail.metadata?.warranty_period"> · 保修：{{ detail.metadata.warranty_period }}</span>
              <span v-if="detail.metadata?.repair_duration != null"> · 时长：{{ detail.metadata.repair_duration }} 分钟</span>
            </div>
          </div>
        </template>

        <!-- 收据：仅当存在收据图片时展示，避免空卡出现多余横线 -->
        <template v-if="receiptUrls.length">
          <div class="fin-divider"></div>
          <div class="fin-section">
            <div class="fin-subtitle">收据</div>
            <div class="img-strip">
              <el-image
                v-for="(url, index) in receiptUrls"
                :key="url"
                :src="url"
                :preview-src-list="receiptUrls"
                :initial-index="index"
                fit="cover"
                preview-teleported
              />
            </div>
          </div>
        </template>
      </section>
      </div><!-- /xn-col-main -->
      <div class="xn-col xn-col-side">
      <!-- 第三组 · 归档信息卡（单列纵向 Flex，窄侧栏下绝对整齐） -->
      <section class="xn-card">
        <div class="xn-card-title">归档信息</div>
        <div class="kv-list">
          <div class="kv">
            <span class="k">工单号</span>
            <span class="v v-strong arch-no">
              {{ detail.order_no }}
              <el-icon class="copy-ico" title="复制工单号" @click="copyOrderNo"><DocumentCopy /></el-icon>
            </span>
          </div>
          <div class="kv">
            <span class="k">提交时间</span>
            <span class="v">{{ formatDateTime(detail.submitted_at) }}</span>
          </div>
          <div class="kv">
            <span class="k">紧急程度</span>
            <span class="v">
              <span class="urgency-tag" :class="urgencyTagClass">{{ detail.urgency_label }}</span>
            </span>
          </div>
          <div v-if="detail.auditor_name" class="kv">
            <span class="k">审核人</span>
            <span class="v v-strong">{{ detail.auditor_name }}</span>
          </div>
          <div v-if="detail.audited_at" class="kv">
            <span class="k">审核时间</span>
            <span class="v">{{ formatDateTime(detail.audited_at) }}</span>
          </div>
          <div v-if="detail.repairer_name" class="kv">
            <span class="k">维修人员</span>
            <span class="v v-strong">{{ detail.repairer_name }}</span>
          </div>
          <div v-if="detail.accepted_at" class="kv">
            <span class="k">接单时间</span>
            <span class="v">{{ formatDateTime(detail.accepted_at) }}</span>
          </div>
          <div v-if="detail.completed_at" class="kv">
            <span class="k">完工时间</span>
            <span class="v">{{ formatDateTime(detail.completed_at) }}</span>
          </div>
        </div>
      </section>

      <!-- 第五组 · 处理记录卡（默认折叠） -->
      <section class="xn-card">
        <div class="xn-card-title">
          处理记录
          <span class="title-count">（{{ detail.timeline.length }}）</span>
        </div>
        <div v-if="!timelineExpanded" class="timeline-collapsed">
          <el-button link type="primary" @click="timelineExpanded = true">查看全部处理轨迹</el-button>
        </div>
        <el-timeline v-else class="xn-timeline">
          <el-timeline-item
            v-for="item in detail.timeline"
            :key="item.id"
            :class="tlNodeClass(item.action)"
            placement="top"
          >
            <div class="tl-body">
              <!-- 第一行：动作名称（加粗深色）+ 操作人（小字灰，同行） -->
              <div class="tl-row1">
                <span class="tl-action">{{ item.action_label }}</span>
                <span v-if="item.operator_name" class="tl-operator">{{ item.operator_name }}</span>
              </div>
              <!-- 第二行：时间（小字浅灰，仅占一点空间） -->
              <div class="tl-row2">{{ formatShortDateTime(item.created_at) }}</div>
              <!-- 第三行：状态流转胶囊（旧 ➔ 新；相同则隐藏） -->
              <div
                v-if="item.from_status && item.to_status && item.from_status !== item.to_status"
                class="tl-row3"
              >
                <span class="tl-pill tl-pill-from">{{ statusLabel(item.from_status) }}</span>
                <el-icon class="tl-arrow"><Right /></el-icon>
                <span class="tl-pill tl-pill-to">{{ statusLabel(item.to_status) }}</span>
              </div>
              <div v-if="item.remark" class="tl-remark">{{ item.remark }}</div>
            </div>
          </el-timeline-item>
        </el-timeline>
        <el-button v-if="timelineExpanded" link type="primary" class="timeline-collapse-btn" @click="timelineExpanded = false">
          收起
        </el-button>
      </section>

      <!-- 审核操作卡片：右栏底部；审核通过（绿实心主按钮，占满整行）/ 退回（灰线框次按钮，占满整行） -->
      <section v-if="visibleActions.length" class="xn-card action-card">
        <div class="xn-card-title">
          <el-icon><Stamp /></el-icon>
          {{ actionCardTitle }}
        </div>
        <el-button
          v-if="primaryAction"
          type="primary"
          size="large"
          class="act-primary"
          @click="handleAction(primaryAction)"
        >
          {{ primaryAction.label }}
        </el-button>
        <el-button
          v-if="rejectAction"
          plain
          size="large"
          class="reject-btn"
          @click="handleAction(rejectAction)"
        >
          {{ rejectAction.label }}
        </el-button>
        <el-button
          v-for="a in secondaryActions"
          :key="a.action"
          plain
          size="large"
          @click="handleAction(a)"
        >
          {{ a.label }}
        </el-button>
      </section>
      </div><!-- /xn-col-side -->
      </div><!-- /xn-grid -->
    </template>

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

    <!-- 退单回池弹窗 -->
    <el-dialog v-model="returnDialogVisible" title="退单回池" width="480px">
      <el-form label-position="top">
        <el-form-item label="退单原因（将通知报修人，工单重回待接单池等待其他维修员接单）">
          <el-input
            v-model="returnReason"
            type="textarea"
            :rows="4"
            :maxlength="200"
            show-word-limit
            :placeholder="`请输入退单原因，至少 ${returnMinLength} 个字`"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="returnDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="returning" @click="handleReturnSubmit">
          确认退单
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
        <div class="finance-amount-line">金额（只读预览）：¥{{ financeAmount }}</div>
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
.detail-page {
  max-width: 1000px;
  margin: 0 auto;
  padding-bottom: 16px;
}

/* 双栏布局：左 2/3（报修信息 + 故障详情 + 对账信息），右 1/3（归档 + 处理记录 + 审核操作） */
.xn-grid {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 16px;
  align-items: start;
}

.xn-col {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

@media (max-width: 860px) {
  .xn-grid {
    grid-template-columns: 1fr;
  }
}

/* 页头 */
.d-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.d-order-no {
  font-size: 17px;
  font-weight: 600;
  color: var(--xn-text-900);
}

.xn-card-title .status-pill {
  margin-left: auto;
}

/* 通用卡片 */
.xn-card {
  background: var(--xn-surface);
  border: 1px solid var(--xn-border);
  border-radius: 12px;
  padding: 16px;
  margin-top: 0;
}

.xn-card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 600;
  color: var(--xn-text-900);
  padding-bottom: 10px;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--xn-border-soft);
}

.xn-card-title .el-icon {
  color: var(--xn-text-500);
}

.title-count {
  font-size: 13px;
  font-weight: 400;
  color: var(--xn-text-400);
}

.edit-link {
  margin-left: auto;
  font-weight: 500;
}

/* ── 报修信息卡（原「现场执行」） ── */
.onsite-place {
  font-size: 20px;
  font-weight: 700;
  line-height: 1.35;
  color: var(--xn-text-900);
  display: flex;
  align-items: center;
  gap: 8px;
}

.onsite-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 10px;
  font-size: 13px;
  color: var(--xn-text-500);
}

.onsite-row {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--xn-border-soft);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: nowrap;
}

.onsite-reporter {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.or-avatar {
  flex-shrink: 0;
}

/* 头像右侧账户名（仅当与报修人姓名不同时展示） */
.or-account {
  font-size: 12px;
  line-height: 1;
  color: var(--xn-text-400);
  flex-shrink: 0;
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 账户名与报修人姓名之间的分隔线 */
.or-divider {
  width: 1px;
  height: 24px;
  background: var(--xn-border);
  flex-shrink: 0;
  margin: 0 2px;
}

.or-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.or-name {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--xn-text-900);
}

.or-phone {
  font-size: 13px;
  color: var(--xn-text-500);
  letter-spacing: 0.3px;
  margin-top: 2px;
}

.call-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--xn-text-500);
  text-decoration: none;
  border: 1px solid var(--xn-border);
  transition: color 0.2s, border-color 0.2s;
}

.call-btn .el-icon {
  font-size: 18px;
}

.call-btn:hover {
  color: var(--xn-primary);
  border-color: var(--xn-primary);
}

/* ── 故障详情卡 ── */
.fault-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.fault-tag {
  display: inline-flex;
  align-items: center;
  background: var(--xn-tag-bg);
  color: var(--xn-text-700);
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 600;
}

.fault-tag-empty {
  font-size: 13px;
  color: var(--xn-text-400);
}

.fault-desc {
  font-size: 16px;
  line-height: 1.7;
  color: var(--xn-text-700);
  white-space: pre-wrap;
  word-break: break-word;
}

.fault-desc.clamp {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.desc-toggle {
  padding-left: 0;
  margin-top: 2px;
}

/* 缩略图网格（故障图 / 收据，自动换行，一行 3-4 张） */
.img-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.img-strip :deep(.el-image) {
  width: 100%;
  height: 104px;
  border-radius: 8px;
  overflow: hidden;
  cursor: zoom-in;
  background: var(--xn-tag-bg);
}

.img-strip :deep(.el-image img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* ── 归档 / 对账 键值列表（两列网格，紧凑） ── */
/* ── 归档信息：单列纵向 Flex，两端对齐，窄侧栏绝对整齐 ── */
.kv-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.kv {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 0;
}

.kv .k {
  color: var(--xn-text-500);
  font-size: 12px;
  line-height: 18px;
  flex-shrink: 0;
}

.kv .v {
  color: #1f2937;
  font-size: 13px;
  line-height: 18px;
  text-align: right;
  word-break: break-all;
}

/* 重要内容加粗 */
.kv .v-strong {
  font-weight: 600;
}

/* 工单号 + 复制图标 */
.arch-no {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.copy-ico {
  font-size: 13px;
  color: var(--xn-text-400);
  cursor: pointer;
  transition: color 0.15s ease;
}

.copy-ico:hover {
  color: var(--xn-primary);
}

/* 紧急程度：小号圆角标签（不用纯文本） */
.urgency-tag {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
}

.urgency-tag.ut-normal {
  background: var(--xn-tag-bg);
  color: var(--xn-text-500);
}

.urgency-tag.ut-urgent {
  background: rgba(234, 88, 12, 0.12);
  color: #ea580c;
}

.urgency-tag.ut-very {
  background: rgba(220, 38, 38, 0.12);
  color: #dc2626;
}

/* ── 对账信息卡：金额主显 + 折叠明细 + 内容/收据分区 ── */
.finance-card {
  padding: 20px;
}

/* 金额：独立成块，大字号加粗主色 + 右侧详情箭头 */
.fin-amount {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 2px;
}

.fa-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--xn-primary);
  letter-spacing: 0.5px;
}

.fin-detail-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--xn-text-secondary);
  cursor: pointer;
  transition: transform 0.2s ease, background 0.2s ease, color 0.2s ease;
}

.fin-detail-toggle:hover {
  background: var(--xn-tag-bg);
  color: var(--xn-primary);
}

.fin-detail-toggle .el-icon {
  font-size: 16px;
  transition: transform 0.2s ease;
}

.fin-detail-toggle.expanded .el-icon {
  transform: rotate(90deg);
}

/* 折叠明细：横向平铺键值对 */
.fin-detail {
  margin-top: 12px;
  padding: 10px 12px;
  background: var(--xn-bg);
  border-radius: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 28px;
}

.fin-detail .kv {
  flex-direction: row;
  align-items: center;
  /* 覆盖通用 .kv 的 space-between：明细项内容紧挨，避免键值被拉开后挤压换行 */
  justify-content: flex-start;
  gap: 6px;
  padding: 0;
}

.fin-detail .kv .k {
  font-size: 13px;
  color: var(--xn-text-500);
}

.fin-detail .kv .v {
  font-size: 13px;
  color: var(--xn-text-900);
}

/* 模块间分割线 */
.fin-divider {
  height: 1px;
  background: var(--xn-border);
  margin: 16px 0;
}

/* 内容分区（维修操作内容 / 维修备注 / 附加信息 / 收据） */
.fin-section + .fin-section {
  margin-top: 14px;
}

/* 小标题：左侧主色竖线 + 深灰加粗，强化视觉 */
.fin-subtitle {
  display: flex;
  align-items: center;
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
  border-left: 3px solid var(--xn-primary);
  padding-left: 8px;
  margin-bottom: 10px;
}

.fin-text {
  font-size: 15px;
  color: var(--xn-text-700);
  line-height: 1.7;
  word-break: break-word;
}

/* 收据图片：顶部间距拉大 + 极浅灰色描边 */
.fin-section .img-strip {
  margin-top: 12px;
}

.fin-section .img-strip :deep(.el-image) {
  border: 1px solid var(--xn-border);
}

/* 收据占位 */
.fin-receipt-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 96px;
  border: 1px dashed var(--xn-border);
  border-radius: 8px;
  color: var(--xn-text-400);
  font-size: 13px;
  background: var(--xn-bg);
}

/* ── 处理记录（折叠） ── */
.timeline-collapsed {
  padding: 4px 0;
}

.timeline-collapse-btn {
  padding-left: 0;
  margin-top: 4px;
}

/* ── 处理记录时间轴：去背景化 + 节点上色 + 三段式信息层级 ── */
.xn-timeline {
  padding-left: 2px;
}

/* 节点与节点之间紧凑（约 16px），最后一个不占距 */
.xn-timeline .el-timeline-item {
  padding-bottom: 16px;
}

.xn-timeline .el-timeline-item:last-child {
  padding-bottom: 0;
}

/* 竖线：1px + 浅色 #E5E7EB */
.xn-timeline .el-timeline-item__tail {
  border-left-width: 1px;
  border-left-color: var(--xn-border);
  left: 5px;
}

/* 节点圆圈按动作类型上色（去掉默认灰块） */
.xn-timeline .el-timeline-item__node {
  background-color: #9ca3af;
  border: none;
}

.xn-timeline .tl-gray .el-timeline-item__node {
  background-color: #9ca3af;
}

.xn-timeline .tl-blue .el-timeline-item__node {
  background-color: #2563eb;
}

.xn-timeline .tl-orange .el-timeline-item__node {
  background-color: #ea580c;
}

.xn-timeline .tl-teal .el-timeline-item__node {
  background-color: var(--xn-primary);
}

.xn-timeline .tl-red .el-timeline-item__node {
  background-color: #dc2626;
}

/* 内容直接呈现在卡片背景上，无灰底 */
.tl-body {
  padding: 0;
}

.tl-row1 {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.tl-action {
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
  line-height: 1.3;
}

.tl-operator {
  font-size: 12px;
  color: #6b7280;
}

.tl-row2 {
  margin-top: 2px;
  font-size: 12px;
  color: #9ca3af;
  line-height: 1.3;
}

/* 状态流转胶囊：旧 ➔ 新 */
.tl-row3 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
}

.tl-pill {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 18px;
  font-weight: 500;
}

.tl-pill-from {
  background: var(--xn-tag-bg);
  color: var(--xn-text-500);
}

.tl-pill-to {
  background: rgba(15, 118, 110, 0.12);
  color: var(--xn-primary);
}

.tl-arrow {
  font-size: 12px;
  color: var(--xn-text-400);
}

.tl-remark {
  margin-top: 6px;
  color: var(--xn-text-500);
  font-size: 13px;
  line-height: 1.6;
}

/* ── 审核操作卡片：竖排按钮，主按钮(绿实心)占满整行在上，退回(灰线框)占满整行在下 ── */
.action-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* 占满整行，并去除 Element Plus 相邻按钮默认的左边距 */
.action-card .el-button {
  width: 100%;
  margin-left: 0;
}

/* 弱化驳回按钮：灰色线框，不与主按钮并排强调 */
.reject-btn {
  color: var(--xn-text-500);
  border-color: var(--xn-border);
}

/* 弱化驳回按钮：灰色、不与主按钮并排强调 */
.reject-btn {
  color: var(--xn-text-500);
  border-color: var(--xn-border);
}

.reject-btn:hover {
  color: var(--xn-text-700);
  border-color: var(--xn-text-400);
  background: var(--xn-bg);
}

/* ── 以下为弹窗内复用样式（完工 / 补签 / 对账） ── */
.reject-alert {
  margin-bottom: 14px;
}

.complete-block-title {
  font-weight: 700;
  font-size: 15px;
  color: var(--xn-text-900);
  margin: 4px 0 10px;
  padding-left: 8px;
  border-left: 3px solid var(--xn-primary);
  line-height: 1.4;
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
  color: var(--xn-text-500);
  font-size: 14px;
  margin: 0 0 10px;
}

/* 状态放大胶囊（详情头 / 列表共用色板，定义在全局） */
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
  flex-shrink: 0;
}

.st-red {
  background: var(--st-red);
}

.st-amber {
  background: var(--st-amber);
}

.st-orange {
  background: var(--st-orange);
}

.st-blue {
  background: var(--st-blue);
}

.st-teal {
  background: var(--st-teal);
}

.st-gray {
  background: var(--st-gray);
}

.st-pending-sign {
  background: linear-gradient(90deg, #f59e0b, #ea580c);
  box-shadow: 0 1px 4px rgba(234, 88, 12, 0.35);
  letter-spacing: 1px;
}
</style>
