<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { adminAPI } from '@/api/admin'
import type { OrderListItem } from '@/types'
import OrderCardC from '@/components/OrderCardC.vue'

const router = useRouter()
const userStore = useUserStore()

/** 每个区块预览条数 */
const PREVIEW_SIZE = 5

interface Section {
  /** 与工单管理 Tab 的 value 一致，供「详情 →」直接带筛选跳转 */
  key: string
  title: string
  status: string
  hint: string
  /** 是否受"单位"限定（审核员的待审核只能看本单位） */
  unitScoped: boolean
  list: OrderListItem[]
  total: number
  loading: boolean
}

/** 按角色取区块并集：审核员看待审核；维修员看待接单 + 处理中 */
const buildSections = (): Section[] => {
  const list: Section[] = []
  if (userStore.hasReviewerRole) {
    list.push({
      key: 'reported',
      title: '待审核',
      status: 'reported',
      hint: '等待您确认后上报维修业务员',
      unitScoped: true,
      list: [],
      total: 0,
      loading: false
    })
  }
  if (userStore.isStoreStaff) {
    list.push({
      key: 'pending_accept',
      title: '待接单',
      status: 'pending_accept',
      hint: '审核已通过，可以接单',
      unitScoped: false,
      list: [],
      total: 0,
      loading: false
    })
    list.push({
      key: 'processing',
      title: '处理中',
      status: 'processing',
      hint: '已接单，等待完工结单',
      unitScoped: false,
      list: [],
      total: 0,
      loading: false
    })
  }
  return list
}

const sections = ref<Section[]>(buildSections())

// ── 审核员多单位：首页同样提供「切换单位」 ──
const reviewerUnits = computed(() => userStore.reviewerEnterprises)
const showUnitSwitch = computed(() => reviewerUnits.value.length > 1)
const activeUnitId = ref(reviewerUnits.value[0]?.enterprise_id || '')

const fetchSection = async (s: Section) => {
  s.loading = true
  try {
    const res = await adminAPI.getOrders({
      page: 1,
      page_size: PREVIEW_SIZE,
      status: s.status,
      sort_by: 'submitted_at',
      sort_order: 'desc',
      // 审核员的「待审核」限定在所选单位
      ...(s.unitScoped && activeUnitId.value ? { enterprise_id: activeUnitId.value } : {})
    })
    s.list = res.data.list
    s.total = res.data.total
  } finally {
    s.loading = false
  }
}

const fetchAll = () => {
  sections.value.forEach(s => fetchSection(s))
}

const handleUnitChange = () => {
  sections.value.filter(s => s.unitScoped).forEach(s => fetchSection(s))
}

/** 「详情 →」直达工单管理并自动选中对应 Tab */
const goOrders = (s: Section) => {
  const query: Record<string, string> = { status: s.status }
  // 单位限定的区块连单位一起带过去，保证两边看到的是同一批数据
  if (s.unitScoped && activeUnitId.value) query.enterprise = activeUnitId.value
  router.push({ name: 'OrderList', query })
}

const goDetail = (row: OrderListItem) => {
  router.push(`/orders/${row.id}`)
}

const nickname = computed(() => userStore.userInfo?.nickname || '管理员')

const roleLabel = computed(() => {
  const labels: string[] = []
  if (userStore.isSuperAdmin) labels.push('超级管理员')
  else if (userStore.isStoreStaff) labels.push('维修业务员')
  if (userStore.hasReviewerRole) labels.push('单位审核员')
  return labels.join(' · ')
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '凌晨好'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

onMounted(fetchAll)
</script>

<template>
  <div class="home">
    <div class="home-head">
      <div class="head-text">
        <div class="greeting">{{ greeting }}，{{ nickname }}</div>
        <div class="sub">
          按您的角色展示待办工单；点区块右上角「详情 →」进入工单管理查看完整列表
        </div>
      </div>
      <el-tag v-if="roleLabel" type="warning" size="small">{{ roleLabel }}</el-tag>
    </div>

    <!-- 多单位时：出现「切换单位」信息提示 -->
    <div v-if="showUnitSwitch" class="unit-bar">
      <el-icon class="unit-icon"><Switch /></el-icon>
      <span class="unit-hint">您担任 {{ reviewerUnits.length }} 个单位的审核员，请切换单位查看待审核工单</span>
      <el-select v-model="activeUnitId" size="small" class="unit-select" @change="handleUnitChange">
        <el-option
          v-for="u in reviewerUnits"
          :key="u.enterprise_id"
          :label="u.enterprise_name"
          :value="u.enterprise_id"
        />
      </el-select>
    </div>

    <div v-for="s in sections" :key="s.key" class="sec">
      <div class="sec-head">
        <span class="sec-title">{{ s.title }}</span>
        <el-badge :value="s.total" :hidden="s.total === 0" class="sec-count" />
        <span class="sec-hint">{{ s.hint }}</span>
        <el-button link type="primary" class="sec-more" @click="goOrders(s)">详情 →</el-button>
      </div>

      <div v-loading="s.loading" class="card-grid">
        <OrderCardC
          v-for="row in s.list"
          :key="row.id"
          :row="row"
          @click="goDetail(row)"
        />
        <div v-if="!s.loading && !s.list.length" class="card-empty">暂无{{ s.title }}工单</div>
      </div>
    </div>

    <el-empty v-if="!sections.length" description="您的账号暂无任何待办角色" />
  </div>
</template>

<style scoped>
.home {
  max-width: 1200px;
}

.home-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.greeting {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.sub {
  margin-top: 4px;
  font-size: 13px;
  color: #909399;
}

/* 多单位审核员：「切换单位」信息提示条 */
.unit-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
  padding: 8px 12px;
  border: 1px solid #b3d8ff;
  border-radius: 8px;
  background-color: #ecf5ff;
}

.unit-icon {
  color: #409eff;
  flex-shrink: 0;
}

.unit-hint {
  font-size: 13px;
  color: #409eff;
}

.unit-select {
  width: 280px;
  margin-left: auto;
}

.sec {
  margin-bottom: 16px;
}

.sec-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.sec-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.sec-count {
  margin-top: -2px;
}

.sec-hint {
  font-size: 12px;
  color: #909399;
}

.sec-more {
  margin-left: auto;
}

/* 卡片网格（与工单管理「卡片视图 (C)」共用同一套 OrderCardC 组件，样式完全一致） */
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
  min-height: 40px;
}

.card-empty {
  grid-column: 1 / -1;
  text-align: center;
  color: #c0c4cc;
  padding: 40px 0;
}
</style>
