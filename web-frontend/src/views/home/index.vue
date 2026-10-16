<script setup lang="ts">
/**
 * 首页 · 待办看板（角色化，镜像小程序 V1.7）
 *
 * 上半：按角色出待办卡片矩阵（审核员/维修员/普通），每张卡给总数 + 未读数 + 最近 1-2 条；
 *       卡片按 personal 分组为「我的」(我报修的) 与「待办」(角色工作)；
 *       未读 = 存在他人产生、晚于我上次查看该工单的动态。
 * 下半：角色化服务入口（审核员 4 项 / 维修员 2 项；超管额外可见企业管理/数据统计）。
 *
 * 与小程序一致：首页全部单位，不出现单位切换条（单位收敛仅发生在列表页）。
 */
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import ServiceIcon from '@/components/ServiceIcon.vue'
import { useUserStore } from '@/stores/user'
import { adminAPI } from '@/api/admin'
import {
  cardKeyToMode,
  cardKeyToTag,
  type OrderMode
} from '@/order/modes'
import { tagCls } from '@/order/tags'
import type { DashboardCard, DashboardOrder } from '@/types'

const router = useRouter()
const userStore = useUserStore()

const cards = ref<DashboardCard[]>([])
const unreadTotal = ref(0)
const loading = ref(false)

// 各卡片展开态（key → 是否已展开全部）与预览快照
const expandedKeys = ref<Record<string, boolean>>({})
const previewMap = ref<Record<string, DashboardOrder[]>>({})
const loadingCardKey = ref('')

// 角色化服务入口（审核员优先；与小程序 home 服务入口矩阵一致）
interface ServiceEntry {
  key: string
  /** 图标 key（src/order/serviceIcons.ts，同源于根目录 icons/*.svg） */
  icon: string
  title: string
  sub: string
  cls: string
  action: () => void
}

const serviceEntries = computed<ServiceEntry[]>(() => {
  const out: ServiceEntry[] = []
  if (userStore.hasReviewerRole) {
    out.push(
      {
        key: 'enterprise',
        icon: 'ent',
        title: '企业管理',
        sub: '成员与邀请码',
        cls: 'staff',
        action: () => router.push({ name: 'EnterpriseList' })
      },
      {
        key: 'my_orders',
        icon: 'my',
        title: '我的工单',
        sub: '我上报的工单',
        cls: 'plain',
        action: () => goList('my_repair')
      },
      {
        key: 'create',
        icon: 'fix',
        title: '报修',
        sub: '提交新工单',
        cls: 'plain',
        action: () => router.push({ name: 'OrderCreate' })
      },
      {
        key: 'review',
        icon: 'rev',
        title: '审核工单',
        sub: '待我审核',
        cls: 'primary',
        action: () => goList('review', 'pending_review')
      }
    )
  } else if (userStore.isStoreStaff) {
    // 维修业务员（含超管）：接单大厅 / 我的维修
    out.push(
      {
        key: 'hall',
        icon: 'hall',
        title: '接单大厅',
        sub: '待接工单',
        cls: 'primary',
        action: () => goList('worker_task', 'pending_accept')
      },
      {
        key: 'my_repair_work',
        icon: 'my',
        title: '我的维修',
        sub: '我接的单',
        cls: 'staff',
        action: () => goList('worker_task', 'mine')
      }
    )
    // 超管额外可见企业管理 / 数据统计
    if (userStore.isSuperAdmin) {
      out.push(
        {
          key: 'enterprise_super',
          icon: 'ent',
          title: '企业管理',
          sub: '单位与成员',
          cls: 'staff',
          action: () => router.push({ name: 'EnterpriseList' })
        },
        {
          key: 'stats_super',
          icon: 'stats',
          title: '数据统计',
          sub: '运营概览',
          cls: 'primary',
          action: () => router.push({ name: 'Stats' })
        }
      )
    }
  }
  return out
})

/** 首页全部单位（不再按单单位收敛），拉取本人全部相关工单卡片 */
const fetchDashboard = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getDashboard()
    const data = res.data
    cards.value = data?.cards || []
    unreadTotal.value = data?.unread_total || 0
    // 记录预览快照，供「查看全部」收起时还原
    const preview: Record<string, DashboardOrder[]> = {}
    for (const c of cards.value) preview[c.key] = c.orders
    previewMap.value = preview
    expandedKeys.value = {}
  } catch {
    cards.value = []
    unreadTotal.value = 0
  } finally {
    loading.value = false
  }
}

/** 空卡隐藏：后端已过滤一层，这里再兜一次底（total==0 不显示） */
const visibleCards = computed(() => cards.value.filter(c => c.total > 0))
const personalCards = computed(() => visibleCards.value.filter(c => c.personal))
const workCards = computed(() => visibleCards.value.filter(c => !c.personal))

/** 看板分组：个人在前、工作在后；空组不出现 */
const boardSections = computed(() => {
  const secs: { title: string; cards: DashboardCard[] }[] = []
  if (personalCards.value.length) secs.push({ title: '我的', cards: personalCards.value })
  if (workCards.value.length) secs.push({ title: '待办', cards: workCards.value })
  return secs
})
/** 未读总数挂在首个分组标题上（个人组优先） */
const unreadSectionTitle = computed(() => {
  if (personalCards.value.length) return '我的'
  if (workCards.value.length) return '待办'
  return ''
})

/** 跳工单列表页（带 mode + 可选 tag） */
const goList = (mode: OrderMode, tag?: string) => {
  const query: Record<string, string> = { mode }
  if (tag) query.tag = tag
  router.push({ name: 'OrderList', query })
}

/** 卡片标题 → 工单列表对应 Tab */
const goCard = (c: DashboardCard) => goList(cardKeyToMode(c.key), cardKeyToTag(c.key))

/** 卡片内工单 → 直达详情并标记已读 */
const goDetail = async (_c: DashboardCard, o: DashboardOrder) => {
  adminAPI.markOrderRead(o.id).catch(() => {})
  router.push(`/orders/${o.id}`)
}

/** 「查看全部 / 收起」：展开调全量接口突破 3 条预览上限，再点收起还原预览 */
const expandCard = async (c: DashboardCard) => {
  if (expandedKeys.value[c.key]) {
    c.orders = previewMap.value[c.key] || []
    expandedKeys.value[c.key] = false
    return
  }
  loadingCardKey.value = c.key
  try {
    const res = await adminAPI.getDashboardCard(c.key)
    if (res.data?.orders) {
      previewMap.value[c.key] = c.orders
      c.orders = res.data.orders
      expandedKeys.value[c.key] = true
    }
  } catch {
    // 静默失败，保留预览
  } finally {
    loadingCardKey.value = ''
  }
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

    <div v-loading="loading" class="board">
      <template v-for="sec in boardSections" :key="sec.title">
        <div class="board-hd">
          <span class="board-title">{{ sec.title }}</span>
          <span v-if="unreadTotal > 0 && sec.title === unreadSectionTitle" class="board-unread">
            {{ unreadTotal }} 条未读
          </span>
        </div>

        <div v-for="c in sec.cards" :key="c.key" class="bcard">
          <div class="bcard-head" @click="goCard(c)">
            <span class="bcard-title">{{ c.label }}</span>
            <span v-if="c.unread > 0" class="bcard-unread-dot" title="有未读工单" />
            <span class="bcard-total">({{ c.total }})</span>
          </div>

          <div v-if="!c.orders.length" class="bcard-empty">暂无</div>
          <div v-else class="bcard-list">
            <div
              v-for="o in c.orders"
              :key="o.id"
              class="orow"
              @click="goDetail(c, o)"
            >
              <span class="orow-dot" :class="{ 'orow-dot-on': o.unread }" />
              <span class="orow-tag" :class="tagCls(o.display_tag_type)">{{ o.display_tag }}</span>
              <span class="orow-desc">{{ o.description || '（无故障描述）' }}</span>
              <span class="orow-ent">{{ o.enterprise_name }}</span>
            </div>
          </div>

          <div
            v-if="expandedKeys[c.key] || c.total > c.orders.length"
            class="bcard-foot"
            @click="expandCard(c)"
          >
            <span class="bcard-foot-txt">
              {{ expandedKeys[c.key] ? '收起' : `查看全部 ${c.total} 条` }}
            </span>
            <span class="bcard-foot-arw">{{ expandedKeys[c.key] ? '▲' : '▼' }}</span>
          </div>
        </div>
      </template>

      <el-empty v-if="!loading && !visibleCards.length" description="当前没有与您相关的工单" />
    </div>

    <div class="svc">
      <div class="svc-title">服务</div>
      <div class="entry-grid" :class="{ 'entry-grid--4': serviceEntries.length > 2 }">
        <div v-for="e in serviceEntries" :key="e.key" class="entry-card" @click="e.action()">
          <span class="entry-ico" :class="e.cls">
            <ServiceIcon :name="e.icon" :size="20" />
          </span>
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

/* 待办看板 */
.board {
  min-height: 80px;
}

.board-hd {
  display: flex;
  align-items: center;
  padding: 4px 2px 12px;
}

.board-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.board-unread {
  margin-left: auto;
  font-size: 13px;
  color: #dc2626;
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
  cursor: pointer;
}

.bcard-head:hover {
  background-color: #fafafa;
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

.bcard-unread-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dc2626;
  flex-shrink: 0;
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

.bcard-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px 0 14px;
  border-top: 1px solid #f2f6fc;
  cursor: pointer;
}

.bcard-foot-txt {
  font-size: 13px;
  color: #0f766e;
}

.bcard-foot-arw {
  margin-left: 6px;
  font-size: 11px;
  color: #0f766e;
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
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.entry-grid--4 {
  grid-template-columns: repeat(4, 1fr);
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
  width: 34px;
  height: 34px;
  border-radius: 8px;
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
