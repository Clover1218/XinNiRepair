import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Home',
        redirect: '/orders'
      },
      {
        path: 'enterprises',
        name: 'EnterpriseList',
        component: () => import('@/views/enterprises/list.vue'),
        meta: { title: '企业管理', permission: 'platform_admin' }
      },
      {
        path: 'enterprises/:id',
        name: 'EnterpriseDetail',
        component: () => import('@/views/enterprises/detail.vue'),
        meta: { title: '企业详情', permission: 'platform_admin' }
      },
      {
        path: 'orders',
        name: 'OrderList',
        component: () => import('@/views/orders/list.vue'),
        meta: { title: '工单管理', permission: 'platform_admin' }
      },
      {
        path: 'orders/:id',
        name: 'OrderDetail',
        component: () => import('@/views/orders/detail.vue'),
        meta: { title: '工单处理', permission: 'platform_admin' }
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/orders'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(to => {
  const userStore = useUserStore()

  // 未登录访问受保护页面 → 登录页
  if (to.meta.requiresAuth && !userStore.token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  // 已登录访问登录页 → 首页
  if (to.path === '/login' && userStore.token) {
    return userStore.isPlatformAdmin ? '/enterprises' : '/login'
  }

  // 根路径 → 按角色跳转
  if (to.path === '/' && userStore.token) {
    return userStore.isPlatformAdmin ? '/enterprises' : '/login'
  }

  // 权限校验：所有后台页面仅平台管理员可访问
  if (to.meta.requiresAuth && userStore.token) {
    const required = to.meta.permission as string
    if (required === 'platform_admin' && !userStore.isPlatformAdmin) {
      return '/login'
    }
  }

  return true
})

export default router
