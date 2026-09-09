import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'

export type RoutePermission = 'store' | 'reviewer' | 'super'

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
        path: 'orders',
        name: 'OrderList',
        component: () => import('@/views/orders/list.vue'),
        // 店方专用列表；单位审核员请走 /review（独立视图）
        meta: { title: '工单管理', permissions: ['store'] }
      },
      {
        path: 'orders/:id',
        name: 'OrderDetail',
        component: () => import('@/views/orders/detail.vue'),
        meta: { title: '工单处理', permissions: ['store', 'reviewer'] }
      },
      {
        path: 'review',
        name: 'ReviewIndex',
        component: () => import('@/views/review/index.vue'),
        meta: { title: '单位审核', permissions: ['reviewer'] }
      },
      {
        path: 'stats',
        name: 'Stats',
        component: () => import('@/views/stats/index.vue'),
        // 独立统计页（V1.3 第十三章）：三类角色均可进入；审核员限本单位、隐藏企业对比/维修员业绩
        meta: { title: '统计', permissions: ['store', 'reviewer'] }
      },
      {
        path: 'enterprises',
        name: 'EnterpriseList',
        component: () => import('@/views/enterprises/list.vue'),
        meta: { title: '企业管理', permissions: ['store'] }
      },
      {
        path: 'enterprises/:id',
        name: 'EnterpriseDetail',
        component: () => import('@/views/enterprises/detail.vue'),
        meta: { title: '企业详情', permissions: ['store', 'reviewer'] }
      },
      {
        path: 'dictionary',
        name: 'Dictionary',
        component: () => import('@/views/dictionary/index.vue'),
        meta: { title: '项目字典', permissions: ['super'] }
      },
      {
        path: 'users',
        name: 'UserList',
        component: () => import('@/views/users/list.vue'),
        meta: { title: '用户管理', permissions: ['super'] }
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

/** 判断用户是否满足路由的 permissions 数组之一 */
function canAccess(permissions: RoutePermission[] | undefined): boolean {
  const userStore = useUserStore()
  if (!permissions || permissions.length === 0) return true
  return permissions.some(p => {
    if (p === 'store') return userStore.isStoreStaff
    if (p === 'super') return userStore.isSuperAdmin
    if (p === 'reviewer') return userStore.hasReviewerRole
    return false
  })
}

router.beforeEach(to => {
  const userStore = useUserStore()

  // 未登录访问受保护页面 → 登录页
  if (to.meta.requiresAuth && !userStore.token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  // 已登录访问登录页 / 根路径 → 按角色落地
  if ((to.path === '/login' || to.path === '/') && userStore.token) {
    return userStore.landingPath
  }

  // 权限校验：requirements 全部不满足 → 回到自己的落地页
  if (to.meta.requiresAuth && userStore.token && !canAccess(to.meta.permissions as RoutePermission[] | undefined)) {
    return userStore.landingPath
  }

  return true
})

export default router
