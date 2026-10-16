<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useUserStore } from '@/stores/user'
import ServiceIcon from '@/components/ServiceIcon.vue'
import { defaultModeFromRole } from '@/order/modes'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const displayName = computed(() => userStore.userInfo?.nickname || '管理员')

const roleLabel = computed(() => {
  if (userStore.isSuperAdmin) return '超级管理员'
  if (userStore.isStoreStaff) return '维修业务员'
  if (userStore.hasReviewerRole) return '单位审核员'
  return ''
})

interface MenuItem {
  /** el-menu 的 index（可带 query，如 /orders?mode=review&tag=pending_review） */
  index: string
  title: string
  /** 项目根 icons/*.svg 的图标 key（优先）；缺省用 Element Plus 图标名 */
  svg?: string
  icon?: string
}

const menus = computed<MenuItem[]>(() => {
  const items: MenuItem[] = []
  const push = (m: MenuItem) => {
    // 按 index 去重，避免多角色（如既是业务员又是审核员）出现重复入口
    if (!items.some(i => i.index === m.index)) items.push(m)
  }

  // 所有角色统一的首页入口（按角色渲染待办队列）
  push({ index: '/', title: '首页', svg: 'home' })

  // 单位审核员：我的工单 / 审核工单 / 报修 / 企业管理（单单位走详情，多单位走列表）/ 统计
  if (userStore.hasReviewerRole) {
    push({ index: '/orders?mode=my_repair', title: '我的工单', svg: 'my' })
    push({ index: '/orders?mode=review&tag=pending_review', title: '审核工单', svg: 'rev' })
    push({ index: '/orders/create', title: '报修', svg: 'fix' })
    const first = userStore.reviewerEnterprises[0]
    if (first) {
      push({ index: `/enterprises/${first.enterprise_id}`, title: '企业管理', svg: 'ent' })
    }
    push({ index: '/stats', title: '统计', svg: 'stats' })
  }

  // 维修业务员（含超管）：接单大厅 / 我的维修 / 报修 / 统计
  // （企业管理为超管专属：业务维修员无权管理企业，V1.7.1 起移除其入口）
  if (userStore.isStoreStaff) {
    push({ index: '/orders?mode=worker_task&tag=pending_accept', title: '接单大厅', svg: 'hall' })
    push({ index: '/orders?mode=worker_task&tag=mine', title: '我的维修', svg: 'my' })
    push({ index: '/orders/create', title: '报修', svg: 'fix' })
    push({ index: '/stats', title: '统计', svg: 'stats' })
  }

  // 超管专属管理项（沿用 Element Plus 图标）
  if (userStore.isSuperAdmin) {
    push({ index: '/enterprises', title: '企业管理', svg: 'ent' })
    push({ index: '/dictionary', title: '项目字典', icon: 'Collection' })
    push({ index: '/users', title: '用户管理', icon: 'User' })
    push({ index: '/repairers', title: '经营范围', icon: 'Connection' })
    push({ index: '/settings', title: '系统设置', icon: 'Setting' })
  }

  return items
})

/** 当前高亮的菜单 index：工单类入口按 mode(+tag) 精确命中 */
const activeMenu = computed<string>(() => {
  const path = route.path
  const q = route.query
  const match = (index: string) => menus.value.find(m => m.index === index)?.index
  if (path === '/orders') {
    const mode =
      (typeof q.mode === 'string' && q.mode) ||
      defaultModeFromRole({
        isStoreStaff: userStore.isStoreStaff,
        hasReviewerRole: userStore.hasReviewerRole
      })
    if (mode === 'review') return match('/orders?mode=review&tag=pending_review') || path
    if (mode === 'worker_task') {
      return (
        (q.tag === 'pending_accept'
          ? match('/orders?mode=worker_task&tag=pending_accept')
          : match('/orders?mode=worker_task&tag=mine')) || path
      )
    }
    return match('/orders?mode=my_repair') || path
  }
  if (path === '/orders/create') return match('/orders/create') || path
  if (path.startsWith('/enterprises')) {
    const exact = menus.value.find(m => m.index === path)
    const anyEnt = menus.value.find(m => m.index.startsWith('/enterprises'))
    return exact?.index || anyEnt?.index || path
  }
  if (path === '/stats') return match('/stats') || path
  if (path === '/dictionary') return match('/dictionary') || path
  if (path === '/users') return match('/users') || path
  if (path === '/repairers') return match('/repairers') || path
  if (path === '/settings') return match('/settings') || path
  if (path === '/') return match('/') || path
  return path
})

/** 页头标题：取当前高亮菜单的标题，否则回退路由 meta.title */
const headerTitle = computed(() => {
  const hit = menus.value.find(m => m.index === activeMenu.value)
  return hit?.title || (route.meta.title as string) || ''
})

const handleLogout = async () => {
  await ElMessageBox.confirm('确定退出登录吗？', '提示', {
    type: 'warning',
    confirmButtonText: '退出',
    cancelButtonText: '取消'
  })
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <el-icon :size="24" color="#0F766E"><Tools /></el-icon>
        <span class="logo-title">报修系统</span>
      </div>
      <el-menu :default-active="activeMenu" router class="menu">
        <el-menu-item v-for="item in menus" :key="item.index" :index="item.index">
          <!-- svg 图标包一层固定宽度的 .menu-ico，与下方 el-icon 项保持同一文字起点 -->
          <span v-if="item.svg" class="menu-ico">
            <ServiceIcon :name="item.svg" :size="20" />
          </span>
          <el-icon v-else><component :is="item.icon" /></el-icon>
          <span class="menu-txt">{{ item.title }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="page-title">{{ headerTitle }}</div>
        <div class="user-area">
          <el-avatar :size="32" :src="userStore.userInfo?.avatar_url">
            {{ displayName.charAt(0) }}
          </el-avatar>
          <span class="user-name">{{ displayName }}</span>
          <el-tag v-if="roleLabel" size="small" type="warning">{{ roleLabel }}</el-tag>
          <el-button link type="primary" @click="handleLogout">
            <el-icon><SwitchButton /></el-icon>
            退出登录
          </el-button>
        </div>
      </el-header>
      <el-main class="main">
        <!-- path 作 key：/enterprises/:id 切换单位时重挂载详情页（审核员多单位场景）。
             刻意不包含 query —— 工单管理会把 Tab 写进 ?status=xxx，若连 query 一起重挂载
             会导致每次切 Tab 整页重建（失去滚动位置与筛选组件状态）。 -->
        <router-view :key="route.path" />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  height: 100%;
}

.aside {
  display: flex;
  flex-direction: column;
  background-color: #001529;
  overflow: hidden;
}

.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 60px;
  padding: 0 20px;
  flex-shrink: 0;
}

.logo-title {
  color: #fff;
  font-size: 17px;
  font-weight: 600;
  white-space: nowrap;
}

.menu {
  flex: 1;
  border-right: none;
  background-color: transparent;
  overflow-y: auto;
  overflow-x: hidden;
}

.menu :deep(.el-menu-item) {
  color: rgba(255, 255, 255, 0.68);
  position: relative;
  /* 菜单项之间留出呼吸间距（圆角悬浮条样式），不再挤成一整列 */
  margin: 4px 12px;
  height: 46px;
  line-height: 46px;
  border-radius: 8px;
  font-size: 14px;
}

.menu :deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.menu :deep(.el-menu-item.is-active) {
  background-color: rgba(15, 118, 110, 0.18);
  color: #fff;
}

.menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  border-radius: 3px 0 0 3px;
  background: var(--xn-primary);
}

/* 自绘 svg 图标：与 el-icon 同宽对齐，图标与文字留出 8px 间距 */
.menu :deep(.menu-ico) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  margin-right: 8px;
  flex-shrink: 0;
}

.menu :deep(.menu-txt) {
  white-space: nowrap;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: #fff;
  border-bottom: 1px solid var(--xn-border);
  flex-shrink: 0;
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--xn-text-900);
}

.user-area {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-name {
  font-size: 14px;
  color: var(--xn-text-500);
}

.main {
  background-color: var(--xn-bg);
  padding: 16px;
  overflow-y: auto;
  /* 预留滚动条槽位：内容变高（如展开对账明细）出现滚动条时，整页不再横向抖动 */
  scrollbar-gutter: stable;
}
</style>
