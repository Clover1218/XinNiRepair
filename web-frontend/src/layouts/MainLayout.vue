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

const menus = computed<Array<{ path: string; title: string; icon: string }>>(() => {
  if (!userStore.isPlatformAdmin) return []
  return [
    { path: '/orders', title: '工单管理', icon: 'Tickets' },
    { path: '/enterprises', title: '企业管理', icon: 'OfficeBuilding' }
  ]
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
        <el-icon :size="24" color="#409eff"><Tools /></el-icon>
        <span class="logo-title">新泥维修后台</span>
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
          <el-button link type="primary" @click="handleLogout">
            <el-icon><SwitchButton /></el-icon>
            退出登录
          </el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
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
}

.menu :deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.menu :deep(.el-menu-item.is-active) {
  background-color: #409eff;
  color: #fff;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: #fff;
  border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0;
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.user-area {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-name {
  font-size: 14px;
  color: #606266;
}

.main {
  background-color: #f5f7fa;
  padding: 16px;
  overflow-y: auto;
}
</style>
