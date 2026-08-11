<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminAPI } from '@/api/admin'
import type { EnterpriseListItem } from '@/types'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const list = ref<EnterpriseListItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const fetchList = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getEnterprises({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value.trim() || undefined
    })
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
  keyword.value = ''
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

const goDetail = (row: EnterpriseListItem) => {
  router.push(`/enterprises/${row.id}`)
}

// ---- 创建企业 ----
const createDialogVisible = ref(false)
const enterpriseName = ref('')
const creating = ref(false)

const openCreateDialog = () => {
  enterpriseName.value = ''
  createDialogVisible.value = true
}

const handleCreate = async () => {
  const name = enterpriseName.value.trim()
  if (name.length < 2 || name.length > 50) {
    ElMessage.warning('企业名称需为 2-50 个字符')
    return
  }
  creating.value = true
  try {
    await adminAPI.createEnterprise(name)
    ElMessage.success('创建成功')
    createDialogVisible.value = false
    fetchList()
  } finally {
    creating.value = false
  }
}

onMounted(fetchList)
</script>

<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索企业名称"
          clearable
          class="keyword-input"
          @keyup.enter="handleSearch"
        />
        <el-button type="primary" @click="handleSearch">
          <el-icon><Search /></el-icon>
          搜索
        </el-button>
        <el-button @click="handleReset">重置</el-button>
        <div class="spacer" />
        <el-button type="primary" plain @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          创建企业
        </el-button>
      </div>

      <el-table v-loading="loading" :data="list" stripe>
        <el-table-column prop="name" label="企业名称" min-width="200" show-overflow-tooltip />
        <el-table-column prop="member_count" label="成员数" width="100" />
        <el-table-column prop="order_count" label="工单数" width="100" />
        <el-table-column prop="pending_count" label="待审核" width="100" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '正常' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="goDetail(row)">查看</el-button>
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

    <!-- 创建企业弹窗 -->
    <el-dialog v-model="createDialogVisible" title="创建企业" width="460px">
      <el-form label-position="top">
        <el-form-item label="企业名称">
          <el-input
            v-model="enterpriseName"
            :maxlength="50"
            show-word-limit
            placeholder="请输入企业名称（2-50 个字符）"
            @keyup.enter="handleCreate"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">
          确认创建
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.keyword-input {
  width: 280px;
}

.spacer {
  flex: 1;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
