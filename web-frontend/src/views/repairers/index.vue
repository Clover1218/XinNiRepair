<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminAPI } from '@/api/admin'
import type { EnterpriseListItem, RepairerListItem } from '@/types'

/**
 * 维修业务员经营范围分配（V1.5）
 *
 * 业务含义：业务员默认看不到任何企业的数据，必须在此显式分配后才能接单/查看。
 * 这是问题 #10（不能所有企业都能查看）与 #28（防止企业名单泄露）的管控入口。
 */

const loading = ref(false)
const list = ref<RepairerListItem[]>([])
const enterprises = ref<EnterpriseListItem[]>([])

// ── 分配弹窗 ──
const dialogVisible = ref(false)
const saving = ref(false)
const current = ref<RepairerListItem | null>(null)
/** 当前勾选的企业 id（全量覆盖提交） */
const checkedIds = ref<string[]>([])
const originalIds = ref<string[]>([])

const roleLabel: Record<number, string> = {
  0: '普通用户',
  1: '维修业务员',
  2: '超级管理员'
}

const dirty = computed(() => {
  const a = [...checkedIds.value].sort()
  const b = [...originalIds.value].sort()
  return a.length !== b.length || a.some((v, i) => v !== b[i])
})

const allChecked = computed(
  () =>
    enterprises.value.length > 0 &&
    checkedIds.value.length === enterprises.value.length
)

async function loadRepairers() {
  loading.value = true
  try {
    const res = await adminAPI.getRepairers()
    list.value = res.data?.list || []
  } catch {
    // 错误 Toast 由 axios 拦截器统一处理
  } finally {
    loading.value = false
  }
}

async function loadEnterprises() {
  try {
    const res = await adminAPI.getEnterprises({ page: 1, page_size: 200 })
    enterprises.value = res.data.list || []
  } catch {
    enterprises.value = []
  }
}

async function openDialog(row: RepairerListItem) {
  if (row.role === 2) {
    ElMessage.info('超级管理员不受经营范围限制，无需分配')
    return
  }
  current.value = row
  checkedIds.value = []
  originalIds.value = []
  dialogVisible.value = true
  try {
    const res = await adminAPI.getRepairerScope(row.id)
    const ids = (res.data.enterprises || []).map(e => e.enterprise_id)
    // 仅保留仍然有效的企业，避免展示已删除的历史指派
    const valid = new Set(enterprises.value.map(e => e.id))
    checkedIds.value = ids.filter(id => valid.has(id))
    originalIds.value = [...checkedIds.value]
  } catch {
    dialogVisible.value = false
  }
}

function toggleAll() {
  checkedIds.value = allChecked.value ? [] : enterprises.value.map(e => e.id)
}

async function handleSave() {
  if (!current.value) return
  saving.value = true
  try {
    await adminAPI.updateRepairerScope(current.value.id, checkedIds.value)
    ElMessage.success(
      checkedIds.value.length
        ? `已分配 ${checkedIds.value.length} 家企业`
        : '已清空经营范围（该业务员将看不到企业数据）'
    )
    dialogVisible.value = false
    await loadRepairers()
  } catch {
    // 错误 Toast 由 axios 拦截器统一处理
  } finally {
    saving.value = false
  }
}

async function handleClear(row: RepairerListItem) {
  if (row.role === 2) return
  if (!row.scope_count) return
  try {
    await ElMessageBox.confirm(
      `清空后「${row.nickname}」将看不到任何企业的数据（已接工单除外），确定继续吗？`,
      '清空经营范围',
      { type: 'warning', confirmButtonText: '清空', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  await adminAPI.updateRepairerScope(row.id, [])
  ElMessage.success('已清空')
  await loadRepairers()
}

onMounted(async () => {
  await loadEnterprises()
  await loadRepairers()
})
</script>

<template>
  <div class="page">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="经营范围：业务员只能查看与处理已分配企业的数据"
      description="未分配的业务员看不到任何企业的工单与统计（其名下已接工单除外）。超级管理员不受此限制。"
      class="tip"
    />

    <el-table v-loading="loading" :data="list" border stripe class="mt">
      <el-table-column prop="nickname" label="业务员" min-width="140" />
      <el-table-column label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 2 ? 'danger' : 'success'" size="small">
            {{ roleLabel[row.role] || row.role }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="经营范围" min-width="260">
        <template #default="{ row }">
          <el-tag v-if="row.role === 2" type="info" size="small">不受限</el-tag>
          <el-tag v-else-if="!row.scope_count" type="warning" size="small">
            未分配
          </el-tag>
          <template v-else>
            <el-tag
              v-for="e in row.enterprises"
              :key="e.enterprise_id"
              type="primary"
              size="small"
              class="ent-tag"
            >
              {{ e.enterprise_name }}
            </el-tag>
          </template>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" align="center">
        <template #default="{ row }">
          <el-button
            v-if="row.role !== 2"
            type="primary"
            link
            @click="openDialog(row)"
          >
            分配
          </el-button>
          <el-button
            v-if="row.role !== 2 && row.scope_count"
            type="danger"
            link
            @click="handleClear(row)"
          >
            清空
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span>暂无维修业务员</span>
      </template>
    </el-table>

    <el-dialog
      v-model="dialogVisible"
      :title="`分配经营范围 — ${current?.nickname || ''}`"
      width="520px"
    >
      <div v-if="!enterprises.length" class="empty">
        暂无企业，请先在「企业管理」中创建
      </div>
      <template v-else>
        <div class="dialog-bar">
          <el-checkbox
            :model-value="allChecked"
            @change="toggleAll"
          >
            全选（{{ enterprises.length }} 家）
          </el-checkbox>
          <span class="checked-count">已选 {{ checkedIds.length }} 家</span>
        </div>
        <el-checkbox-group v-model="checkedIds" class="ent-group">
          <el-checkbox
            v-for="e in enterprises"
            :key="e.id"
            :value="e.id"
            class="ent-item"
          >
            {{ e.name }}
          </el-checkbox>
        </el-checkbox-group>
        <el-alert
          v-if="!checkedIds.length"
          type="warning"
          :closable="false"
          class="mt"
          title="未选择任何企业：保存后该业务员将看不到企业数据"
        />
      </template>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="saving"
          :disabled="!dirty"
          @click="handleSave"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page {
  padding: 4px;
}
.tip {
  margin-bottom: 16px;
}
.mt {
  margin-top: 12px;
}
.empty {
  padding: 24px 0;
  text-align: center;
  color: #909399;
}
.dialog-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.checked-count {
  font-size: 13px;
  color: #909399;
}
.ent-group {
  display: flex;
  flex-direction: column;
  max-height: 320px;
  overflow-y: auto;
}
.ent-item {
  height: 32px;
}
.ent-tag {
  margin: 2px 6px 2px 0;
}
</style>
