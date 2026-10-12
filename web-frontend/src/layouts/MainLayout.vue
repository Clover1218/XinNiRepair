<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const activeMenu = computed(() => route.path)

const displayName = computed(() => userStore.userInfo?.nickname || '管理员')

const roleLabel = computed(() => {
  if (userStore.isSuperAdmin) return '超级管理员'
  if (userStore.isStoreStaff) return '维修业务员'
  if (userStore.hasReviewerRole) return '单位审核员'
  return ''
})

const menus = computed<Array<{ path: string; title: string; icon: string }>>(() => {
  const items: Array<{ path: string; title: string; icon: string }> = []
  const has = (predicate: (p: string) => boolean) => items.some(i => predicate(i.path))

  // 所有角色统一的首页入口（按角色渲染待办队列）
  items.push({ path: '/', title: '首页', icon: 'HomeFilled' })

  if (userStore.isStoreStaff) {
    items.push(
      { path: '/orders', title: '工单管理', icon: 'Tickets' },
      // 代单位上报故障（与小程序同源接口，后端校验该单位成员身份）
      { path: '/orders/create', title: '上报工单', icon: 'EditPen' },
      { path: '/stats', title: '统计', icon: 'DataAnalysis' },
      { path: '/enterprises', title: '企业管理', icon: 'OfficeBuilding' }
    )
    if (userStore.isSuperAdmin) {
      items.push(
        { path: '/dictionary', title: '项目字典', icon: 'Collection' },
        { path: '/users', title: '用户管理', icon: 'User' },
        // V1.5 经营范围：业务员能看到哪些客户企业的数据
        { path: '/repairers', title: '经营范围', icon: 'Connection' },
        // 系统设置（完工时限等超管参数）
        { path: '/settings', title: '系统设置', icon: 'Setting' }
      )
    }
  }
  // 兼任多角色时追加审核员专属入口（按 path 去重，避免重复菜单）
  if (userStore.hasReviewerRole) {
    // 工单管理：审核员复用与店方同一模块（自带"本单位"限定 + "待审核"默认 Tab + 待审核行就地审核/退回）
    if (!has(p => p === '/orders')) {
      items.push({ path: '/orders', title: '工单管理', icon: 'Tickets' })
    }
    if (!has(p => p === '/orders/create')) {
      items.push({ path: '/orders/create', title: '上报工单', icon: 'EditPen' })
    }
    const first = userStore.reviewerEnterprises[0]
    // 去重：店方已加「企业管理」列表页（精确 '/enterprises'）或任一企业详情（'/enterprises/' 前缀）时不再追加
    if (first && !has(p => p === '/enterprises' || p.startsWith('/enterprises/'))) {
      items.push({
        path: `/enterprises/${first.enterprise_id}`,
        title: '企业管理',
        icon: 'OfficeBuilding'
      })
    }
    // 统计（限本单位，隐藏企业对比/维修员业绩）
    if (!has(p => p === '/stats')) {
      items.push({ path: '/stats', title: '统计', icon: 'DataAnalysis' })
    }
  }
  return items
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
        <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.title }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="page-title">{{ route.meta.title }}</div>
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
}

.menu :deep(.el-menu-item) {
  color: rgba(255, 255, 255, 0.68);
  position: relative;
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
  background: var(--xn-primary);
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
}
</style>
