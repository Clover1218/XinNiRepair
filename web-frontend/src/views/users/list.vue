<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminAPI } from '@/api/admin'
import type { UserListItem } from '@/types'
// import type { FormInstance } from 'element-plus'

// ── 列表 ──
const loading = ref(false)
const list = ref<UserListItem[]>([])
const total = ref(0)
const query = reactive({
  page: 1,
  page_size: 20,
  keyword: '',
  role: undefined as number | undefined
})

const roleOptions = [
  { value: 0, label: '普通用户' },
  { value: 1, label: '维修业务员' },
  { value: 2, label: '超级管理员' }
]

const roleTagType: Record<number, '' | 'success' | 'warning' | 'danger'> = {
  0: '',
  1: 'success',
  2: 'danger'
}

const roleLabel: Record<number, string> = {
  0: '普通用户',
  1: '维修业务员',
  2: '超级管理员'
}

const memberStatusLabel: Record<string, string> = {
  approved: '已加入',
  pending: '待审核',
  rejected: '已拒绝',
  removed: '已移除'
}

/** 是否为单位审核员（role=0 且在其单位中 membership.role=1 并已通过） */
function isReviewer(row: UserListItem): boolean {
  return row.member_role === 1 && row.member_status === 'approved'
}

async function loadList() {
  loading.value = true
  try {
    const res = await adminAPI.getUsers(query)
    list.value = res.data.list
    total.value = res.data.total
  } catch {
    // 错误 Toast 由 axios 拦截器统一处理
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  loadList()
}

function handlePageChange(page: number) {
  query.page = page
  loadList()
}

// ── 编辑用户 ──
const editVisible = ref(false)
// const editFormRef = ref<FormInstance>()
const editForm = reactive({
  id: '',
  nickname: '',
  role: 0,
  phone: ''
})
/** 打开弹窗时的原始角色，用于判断 role 是否被修改（未修改则不提交，避免超管回传自身 role=2 触发校验） */
const originalRole = ref(0)
const editLoading = ref(false)

function openEdit(row: UserListItem) {
  editForm.id = row.id
  editForm.nickname = row.nickname
  editForm.role = row.role
  editForm.phone = row.phone
  originalRole.value = row.role
  editVisible.value = true
}

async function submitEdit() {
  editLoading.value = true
  try {
    const data: { nickname: string; role?: number; phone: string } = {
      nickname: editForm.nickname,
      phone: editForm.phone
    }
    // 角色未变更时不提交，避免超级管理员修改自身昵称/手机号时被误判为"设置超管"
    if (editForm.role !== originalRole.value) {
      data.role = editForm.role
    }
    await adminAPI.updateUser(editForm.id, data)
    ElMessage.success('修改成功')
    editVisible.value = false
    loadList()
  } catch {
    // 错误 Toast 由 axios 拦截器统一处理
  } finally {
    editLoading.value = false
  }
}

// ── 重置/设置密码（6.4） ──
// 适用范围：维修业务员/超管（role>=1）重置；单位审核员（role=0 + reviewer 身份）设置 Web 登录密码；纯微信用户不可。
async function handleResetPassword(row: UserListItem, asReviewer = false) {
  const title = asReviewer
    ? `设置密码，开通「${row.nickname}」的 Web 后台登录`
    : `重置 ${row.nickname} 的密码`
  try {
    const { value } = await ElMessageBox.prompt('请输入新密码（6-32位）', title, {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /^.{6,32}$/,
      inputErrorMessage: '密码长度需 6-32 位'
    })
    await adminAPI.resetPassword(row.id, value)
    ElMessage.success(asReviewer ? '已设置，该账号可用密码登录 Web 后台' : '密码已重置')
  } catch {
    // 用户取消或请求失败
  }
}

function handleResetEntry(row: UserListItem) {
  if (row.role >= 1) {
    handleResetPassword(row, false)
  } else if (isReviewer(row)) {
    handleResetPassword(row, true)
  } else {
    ElMessage.warning('纯微信用户无登录密码；仅单位审核员或维修业务员可设置')
  }
}

// ── 初始化 ──
onMounted(loadList)
</script>

<template>
  <div class="users-page">
    <!-- 搜索栏 -->
    <el-card class="search-card" shadow="never">
      <div class="search-bar">
        <el-input
          v-model="query.keyword"
          placeholder="搜索昵称或手机号"
          clearable
          style="width: 240px"
          @keyup.enter="handleSearch"
        />
        <el-select
          v-model="query.role"
          placeholder="角色筛选"
          clearable
          style="width: 150px"
          @change="handleSearch"
        >
          <el-option
            v-for="opt in roleOptions"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
        <el-button type="primary" @click="handleSearch">搜索</el-button>
      </div>
    </el-card>

    <!-- 用户表格 -->
    <el-card shadow="never">
      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column label="头像" width="60">
          <template #default="{ row }">
            <el-avatar :size="36" :src="row.avatar_url">
              {{ row.nickname.charAt(0) }}
            </el-avatar>
          </template>
        </el-table-column>
        <el-table-column prop="nickname" label="昵称" min-width="120" />
        <el-table-column prop="phone" label="手机号" min-width="120">
          <template #default="{ row }">
            {{ row.phone || '—' }}
          </template>
        </el-table-column>
        <el-table-column label="角色" width="120">
          <template #default="{ row }">
            <el-tag :type="roleTagType[row.role] || ''" size="small">
              {{ roleLabel[row.role] || '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="所属企业" min-width="140">
          <template #default="{ row }">
            <span v-if="row.enterprise_name">{{ row.enterprise_name }}</span>
            <span v-else style="color: #999">未加入</span>
          </template>
        </el-table-column>
        <el-table-column label="成员状态" width="100">
          <template #default="{ row }">
            <span v-if="row.member_status">
              {{ memberStatusLabel[row.member_status] || row.member_status }}
            </span>
            <span v-else style="color: #999">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" min-width="160">
          <template #default="{ row }">
            {{ new Date(row.created_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button
              v-if="row.role >= 1"
              link
              type="warning"
              @click="handleResetEntry(row)"
            >重置密码</el-button>
            <el-button
              v-else-if="isReviewer(row)"
              link
              type="success"
              @click="handleResetEntry(row)"
            >设置密码</el-button>
            <el-button
              v-else
              link
              type="info"
              disabled
            >重置密码</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          v-model:current-page="query.page"
          :page-size="query.page_size"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editVisible" title="编辑用户" width="480px">
      <el-form ref="editFormRef" :model="editForm" label-width="80px">
        <el-form-item label="昵称">
          <el-input v-model="editForm.nickname" maxlength="32" show-word-limit />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="editForm.phone" placeholder="11位手机号" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select
            v-model="editForm.role"
            style="width: 100%"
            :disabled="originalRole === 2"
          >
            <el-option v-if="originalRole === 2" label="超级管理员" :value="2" />
            <el-option label="普通用户" :value="0" />
            <el-option label="维修业务员" :value="1" />
            <!-- 超级管理员不可通过界面设置 -->
          </el-select>
          <div v-if="originalRole === 2" class="role-tip">超级管理员角色不可修改</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editLoading" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.users-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.search-card {
  border-radius: 8px;
}

.search-bar {
  display: flex;
  gap: 12px;
  align-items: center;
}

.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.role-tip {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
}
</style>
