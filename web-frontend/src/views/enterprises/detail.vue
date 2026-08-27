<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import QRCode from 'qrcode'
import { adminAPI } from '@/api/admin'
import type { EnterpriseDetail, MemberItem } from '@/types'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const enterpriseId = route.params.id as string

const loading = ref(false)
const detail = ref<EnterpriseDetail | null>(null)

const memberTabs = [
  { label: '全部', value: '' },
  { label: '已通过', value: 'approved' },
  { label: '待审核', value: 'pending' },
  { label: '已移除', value: 'removed' }
]

const activeTab = ref('')
const members = ref<MemberItem[]>([])
const memberTotal = ref(0)
const memberPage = ref(1)
const memberPageSize = ref(20)
const memberLoading = ref(false)

const roleLabelMap: Record<string, string> = {
  admin: '管理员',
  member: '普通成员'
}

const statusLabelMap: Record<string, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
  removed: '已移除'
}

const statusTagType: Record<string, string> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
  removed: 'info'
}

const fetchDetail = async () => {
  const res = await adminAPI.getEnterpriseDetail(enterpriseId)
  detail.value = res.data
}

const fetchMembers = async () => {
  memberLoading.value = true
  try {
    const res = await adminAPI.getMembers(enterpriseId, {
      page: memberPage.value,
      page_size: memberPageSize.value,
      status: activeTab.value || undefined
    })
    members.value = res.data.list
    memberTotal.value = res.data.total
  } finally {
    memberLoading.value = false
  }
}

const reload = async () => {
  await Promise.all([fetchDetail(), fetchMembers()])
}

const handleTabChange = () => {
  memberPage.value = 1
  fetchMembers()
}

const handleMemberPageChange = (p: number) => {
  memberPage.value = p
  fetchMembers()
}

const handleMemberSizeChange = (s: number) => {
  memberPageSize.value = s
  memberPage.value = 1
  fetchMembers()
}

const displayRole = (m: MemberItem) => m.role_label || roleLabelMap[m.role] || m.role
const displayStatus = (m: MemberItem) => m.status_label || statusLabelMap[m.status] || m.status

// ---- 成员操作 ----
const handleApprove = async (row: MemberItem) => {
  await adminAPI.approveMembers(enterpriseId, [row.user_id])
  ElMessage.success('已通过')
  reload()
}

const handleReject = async (row: MemberItem) => {
  await ElMessageBox.confirm(`确定拒绝成员「${row.nickname}」的加入申请吗？`, '提示', {
    type: 'warning',
    confirmButtonText: '确定',
    cancelButtonText: '取消'
  })
  await adminAPI.rejectMembers(enterpriseId, [row.user_id])
  ElMessage.success('已拒绝')
  reload()
}

const handleRemove = async (row: MemberItem) => {
  await ElMessageBox.confirm(
    `确定移除成员「${row.nickname}」吗？移除后该成员将不可再提交报修。`,
    '提示',
    { type: 'warning', confirmButtonText: '移除', cancelButtonText: '取消' }
  )
  await adminAPI.removeMember(enterpriseId, row.user_id)
  ElMessage.success('已移除')
  reload()
}

// ---- 编辑企业设置 ----
const editDialogVisible = ref(false)
const editName = ref('')
const editAutoApprove = ref(false)
const saving = ref(false)

const openEditDialog = () => {
  editName.value = detail.value?.name || ''
  editAutoApprove.value = detail.value?.auto_approve ?? false
  editDialogVisible.value = true
}

const handleSave = async () => {
  const name = editName.value.trim()
  if (name.length < 2 || name.length > 50) {
    ElMessage.warning('企业名称需为 2-50 字符')
    return
  }
  saving.value = true
  try {
    await adminAPI.updateEnterprise(enterpriseId, {
      name,
      auto_approve: editAutoApprove.value
    })
    ElMessage.success('企业设置已更新')
    editDialogVisible.value = false
    await fetchDetail()
  } finally {
    saving.value = false
  }
}

// ---- 刷新邀请码 ----
const refreshDialogVisible = ref(false)
const validityOptions = [
  { label: '永久', value: 'permanent' },
  { label: '7 天', value: '7days' },
  { label: '1 天', value: '1days' },
  { label: '2 小时', value: '2hours' },
  { label: '5 分钟', value: '5mins' }
]
const selectedValidity = ref('permanent')
const refreshing = ref(false)

const openRefreshDialog = () => {
  selectedValidity.value = 'permanent'
  refreshDialogVisible.value = true
}

const handleRefresh = async () => {
  refreshing.value = true
  try {
    const res = await adminAPI.refreshInviteCode(enterpriseId, selectedValidity.value)
    ElMessage.success('邀请码已刷新')
    refreshDialogVisible.value = false
    await fetchDetail()
    if (res.data.invite_code !== detail.value?.invite_code) {
      // 以接口返回为准
      detail.value = { ...detail.value!, invite_code: res.data.invite_code, invite_code_expires_at: res.data.expires_at }
    }
  } finally {
    refreshing.value = false
  }
}

const inviteExpiryText = computed(() => {
  const expiresAt = detail.value?.invite_code_expires_at
  if (!expiresAt) return '永不过期'
  return `有效期至 ${formatDateTime(expiresAt)}`
})

// ---- 邀请二维码 ----
const qrDialogVisible = ref(false)
const qrDataUrl = ref('')
const qrGenerating = ref(false)

const openQrDialog = async () => {
  const code = detail.value?.invite_code
  if (!code) return
  qrGenerating.value = true
  try {
    qrDialogVisible.value = true
    qrDataUrl.value = await QRCode.toDataURL(`https://xin-ni.com/join?code=${code}`, {
      width: 260,
      margin: 1
    })
  } finally {
    qrGenerating.value = false
  }
}

const downloadQr = () => {
  const a = document.createElement('a')
  a.href = qrDataUrl.value
  a.download = `${detail.value?.name || '企业'}-邀请二维码.png`
  a.click()
}

const inviteUrl = computed(() =>
  detail.value?.invite_code ? `https://xin-ni.com/join?code=${detail.value.invite_code}` : ''
)

onMounted(reload)
</script>

<template>
  <div v-loading="loading">
    <el-card shadow="never" class="enterprise-card">
      <template #header>
        <div class="detail-header">
          <el-button link @click="router.back()">
            <el-icon><ArrowLeft /></el-icon>
            返回列表
          </el-button>
          <span class="detail-title">{{ detail?.name }}</span>
          <el-tag v-if="detail" :type="detail.status === 'active' ? 'success' : 'info'" class="detail-status">
            {{ detail.status === 'active' ? '正常' : '停用' }}
          </el-tag>
        </div>
      </template>

      <template v-if="detail">
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">成员数</span>
            <span class="info-value">{{ detail.member_count }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">工单数</span>
            <span class="info-value">{{ detail.order_count }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">邀请码</span>
            <span class="invite-code">{{ detail.invite_code }}</span>
            <span class="invite-expiry">{{ inviteExpiryText }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">创建时间</span>
            <span class="info-value">{{ formatDateTime(detail.created_at) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">免审核</span>
            <span class="info-value">
              <el-tag :type="detail.auto_approve ? 'success' : 'info'" size="small">
                {{ detail.auto_approve ? '已开启' : '未开启' }}
              </el-tag>
            </span>
          </div>
        </div>

        <div class="invite-actions">
          <el-button @click="openEditDialog">
            <el-icon><Edit /></el-icon>
            编辑
          </el-button>
          <el-button @click="openRefreshDialog">
            <el-icon><Refresh /></el-icon>
            刷新邀请码
          </el-button>
          <el-button type="primary" plain @click="openQrDialog">
            <el-icon><Download /></el-icon>
            下载二维码
          </el-button>
        </div>
      </template>
    </el-card>

    <el-card shadow="never">
      <template #header>
        <span>成员列表（{{ memberTotal }}人）</span>
      </template>

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane
          v-for="tab in memberTabs"
          :key="tab.value"
          :label="tab.label"
          :name="tab.value"
        />
      </el-tabs>

      <el-table v-loading="memberLoading" :data="members" stripe>
        <el-table-column label="昵称" min-width="160">
          <template #default="{ row }">
            <div class="nickname-cell">
              <el-avatar :size="28" :src="row.avatar_url">
                {{ row.nickname?.charAt(0) }}
              </el-avatar>
              <span>{{ row.nickname }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" width="140" />
        <el-table-column label="角色" width="120">
          <template #default="{ row }">{{ displayRole(row) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusTagType[row.status]">{{ displayStatus(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="order_count" label="工单数" width="90" />
        <el-table-column label="加入时间" width="160">
          <template #default="{ row }">{{ formatDateTime(row.joined_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button link type="success" @click="handleApprove(row)">通过</el-button>
              <el-button link type="danger" @click="handleReject(row)">拒绝</el-button>
            </template>
            <el-button v-else-if="row.status === 'approved'" link type="danger" @click="handleRemove(row)">
              移除
            </el-button>
            <span v-else class="no-action">—</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="memberPage"
          v-model:page-size="memberPageSize"
          :total="memberTotal"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handleMemberPageChange"
          @size-change="handleMemberSizeChange"
        />
      </div>
    </el-card>

    <!-- 编辑企业设置弹窗 -->
    <el-dialog v-model="editDialogVisible" title="编辑企业设置" width="420px">
      <el-form label-position="top">
        <el-form-item label="企业名称（2-50 字符）">
          <el-input
            v-model="editName"
            :maxlength="50"
            placeholder="请输入企业名称"
          />
        </el-form-item>
        <el-form-item label="免审核">
          <div class="auto-approve-row">
            <el-switch v-model="editAutoApprove" />
            <span class="auto-approve-tip">开启后，新成员扫码加入无需管理员审核</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">
          保存
        </el-button>
      </template>
    </el-dialog>

    <!-- 刷新邀请码弹窗 -->
    <el-dialog v-model="refreshDialogVisible" title="刷新邀请码" width="420px">
      <el-form label-position="top">
        <el-form-item label="有效期">
          <el-radio-group v-model="selectedValidity">
            <el-radio-button
              v-for="opt in validityOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refreshDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="refreshing" @click="handleRefresh">
          确认刷新
        </el-button>
      </template>
    </el-dialog>

    <!-- 邀请二维码弹窗 -->
    <el-dialog v-model="qrDialogVisible" title="邀请二维码" width="360px" align-center>
      <div v-loading="qrGenerating" class="qr-body">
        <img v-if="qrDataUrl" :src="qrDataUrl" alt="邀请二维码" class="qr-img" />
        <p class="qr-url">{{ inviteUrl }}</p>
      </div>
      <template #footer>
        <el-button type="primary" :disabled="!qrDataUrl" @click="downloadQr">
          <el-icon><Download /></el-icon>
          下载二维码
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.enterprise-card {
  margin-bottom: 16px;
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

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 20px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-label {
  font-size: 13px;
  color: #909399;
}

.info-value {
  font-size: 16px;
  color: #303133;
  font-weight: 600;
}

.invite-code {
  font-size: 22px;
  font-weight: 700;
  color: #409eff;
  letter-spacing: 3px;
}

.invite-expiry {
  font-size: 12px;
  color: #909399;
}

.invite-actions {
  margin-top: 16px;
  display: flex;
  gap: 12px;
}

.auto-approve-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.auto-approve-tip {
  font-size: 13px;
  color: #909399;
}

.nickname-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.no-action {
  color: #c0c4cc;
}

.qr-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 0;
}

.qr-img {
  width: 260px;
  height: 260px;
}

.qr-url {
  margin-top: 12px;
  font-size: 13px;
  color: #606266;
  word-break: break-all;
  text-align: center;
}
</style>
