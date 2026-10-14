<script setup lang="ts">
/**
 * 首页 · 待办看板（按状态聚合，非时间流）
 *
 * 上半：3-4 张状态卡，每张给出该状态的工单总数、未读数与最近 1-2 条工单；
 *       未读 = 存在他人产生的、晚于「我最后一次查看该工单」的动态。
 * 下半：常驻服务入口（不再受待办数量门控）。
 */
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { adminAPI } from '@/api/admin'
import type { DashboardCard, DashboardOrder } from '@/types'

const router = useRouter()
const userStore = useUserStore()

interface ServiceEntry {
  key: string
  icon: string
  title: string
  sub: string
  cls: string
  action: () => void
}

const cards = ref<DashboardCard[]>([])
const unreadTotal = ref(0)
const loading = ref(false)

// ── 审核员多单位：首页同样提供「切换单位」 ──
const reviewerUnits = computed(() => userStore.reviewerEnterprises)
const showUnitSwitch = computed(() => reviewerUnits.value.length > 1)
const activeUnitId = ref(reviewerUnits.value[0]?.enterprise_id || '')

/**
 * 卡片 key → 工单列表的 status 筛选值（V1.7 角色化卡片矩阵）
 * 空串 = 不过滤（全部）：进行中 / 提醒 两张卡命中多状态，跳列表时统一落在「全部」
 */
const STATUS_OF_KEY: Record<string, string> = {
  pending_review: 'reported',
  repairer_processing: 'processing',
  pending_accept: 'pending_accept',
  in_progress: '',
  notice: ''
}

const fetchDashboard = async () => {
  loading.value = true
  try {
    const params = userStore.hasReviewerRole && activeUnitId.value ? { enterprise_id: activeUnitId.value } : undefined
    const res = await adminAPI.getDashboard(params)
    // client 响应拦截器返回信封 { code, message, data }
    const data = (res as unknown as { data: { cards: DashboardCard[]; unread_total: number } }).data
    cards.value = data?.cards || []
    unreadTotal.value = data?.unread_total || 0
  } catch {
    cards.value = []
    unreadTotal.value = 0
  } finally {
    loading.value = false
  }
}

const handleUnitChange = () => fetchDashboard()

/** 点击卡片标题 → 工单列表并自动选中对应状态 */
const goOrders = (c: DashboardCard) => {
  const query: Record<string, string> = { status: STATUS_OF_KEY[c.key] || '' }
  if (activeUnitId.value) query.enterprise = activeUnitId.value
  router.push({ name: 'OrderList', query })
}

/** 点击卡片内的工单 → 直达详情，并标记已读 */
const goDetail = async (row: DashboardOrder) => {
  adminAPI.markOrderRead(row.id).catch(() => {})
  router.push(`/orders/${row.id}`)
}

const serviceEntries = computed<ServiceEntry[]>(() => {
  const out: ServiceEntry[] = [
    {
      key: 'orders',
      icon: '单',
      title: '工单管理',
      sub: '全部工单列表',
      cls: 'plain',
      action: () => router.push({ name: 'OrderList' })
    }
  ]
  if (userStore.isStoreStaff || userStore.hasReviewerRole) {
    out.push({
      key: 'stats',
      icon: '统',
      title: '数据统计',
      sub: '企业与业务员统计',
      cls: 'primary',
      action: () => router.push({ name: 'Stats' })
    })
  }
  if (userStore.hasReviewerRole) {
    out.push({
      key: 'enterprise',
      icon: '企',
      title: '企业管理',
      sub: '成员与邀请码',
      cls: 'staff',
      action: () => router.push({ name: 'EnterpriseList' })
    })
  }
  return out
})

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

/** display_tag_type → 胶囊配色 */
const tagCls = (type: string) => `tag-${type || 'info'}`

onMounted(fetchDashboard)
</script>

<template>
  <div class="home">
    <div class="home-head">
      <div class="head-text">
        <div class="greeting">{{ greeting }}，{{ nickname }}</div>
        <div class="sub">
          <span v-if="unreadTotal > 0" class="unread">您有 {{ unreadTotal }} 项新进展</span>
          <span v-else>按您的角色展示待办工单</span>
        </div>
      </div>
      <el-tag v-if="roleLabel" type="warning" size="small">{{ roleLabel }}</el-tag>
    </div>

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

    <div v-loading="loading" class="board">
      <div v-for="c in cards" :key="c.key" class="bcard">
        <div class="bcard-head">
          <span class="bcard-title">{{ c.label }}</span>
          <span class="bcard-total">({{ c.total }})</span>
          <el-badge v-if="c.unread > 0" :value="c.unread" class="bcard-badge" />
          <el-button link type="primary" class="bcard-more" @click="goOrders(c)">详情 →</el-button>
        </div>

        <div v-if="!c.orders.length" class="bcard-empty">暂无</div>
        <div v-else class="bcard-list">
          <div v-for="o in c.orders" :key="o.id" class="orow" @click="goDetail(o)">
            <span class="orow-dot" :class="{ 'orow-dot-on': o.unread }" />
            <span class="orow-tag" :class="tagCls(o.display_tag_type)">{{ o.display_tag }}</span>
            <span class="orow-desc">{{ o.description || '（无故障描述）' }}</span>
            <span class="orow-ent">{{ o.enterprise_name }}</span>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && !cards.length" description="当前没有与您相关的工单" />
    </div>

    <div class="svc">
      <div class="svc-title">服务</div>
      <div class="entry-grid">
        <div v-for="e in serviceEntries" :key="e.key" class="entry-card" @click="e.action()">
          <span class="entry-ico" :class="e.cls">{{ e.icon }}</span>
          <span class="entry-title">{{ e.title }}</span>
          <span class="entry-sub">{{ e.sub }}</span>
        </div>
      </div>
    </div>
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

.unread {
  color: #dc2626;
}

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

/* 待办看板 */
.board {
  min-height: 80px;
}

.bcard {
  margin-bottom: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background-color: #fff;
  overflow: hidden;
}

.bcard-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px 12px;
}

.bcard-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.bcard-total {
  font-size: 13px;
  color: #909399;
}

.bcard-badge {
  margin-top: -2px;
}

.bcard-more {
  margin-left: auto;
}

.bcard-empty {
  padding: 0 16px 16px;
  font-size: 13px;
  color: #c0c4cc;
}

.bcard-list {
  padding: 0 16px 8px;
}

.orow {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 0;
  border-top: 1px solid #f2f6fc;
  cursor: pointer;
}

.orow:hover {
  background-color: #f5f7fa;
}

.orow-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.orow-dot-on {
  background: #dc2626;
}

.orow-tag {
  flex-shrink: 0;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 12px;
  background: #f1efe8;
  color: #5f5e5a;
}

.orow-desc {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.orow-ent {
  flex-shrink: 0;
  font-size: 12px;
  color: #909399;
}

.tag-warning {
  background: #fff7ed;
  color: #ea580c;
}
.tag-danger {
  background: #fef2f2;
  color: #dc2626;
}
.tag-success {
  background: #e6fffb;
  color: #0f766e;
}
.tag-primary {
  background: #eff6ff;
  color: #2563eb;
}
.tag-info {
  background: #f1efe8;
  color: #5f5e5a;
}

/* 服务入口 */
.svc {
  margin-top: 24px;
}

.svc-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 12px;
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.entry-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background-color: #fff;
  cursor: pointer;
}

.entry-card:hover {
  border-color: #0f766e;
}

.entry-ico {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 10px;
}

.entry-ico.primary {
  background: #e6fffb;
  color: #0f766e;
}
.entry-ico.staff {
  background: #fff7ed;
  color: #ea580c;
}
.entry-ico.plain {
  background: #f1efe8;
  color: #5f5e5a;
}

.entry-title {
  font-size: 14px;
  color: #303133;
}

.entry-sub {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}
</style>
