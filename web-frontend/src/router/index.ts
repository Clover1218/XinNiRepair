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
        // 首页（登录落地页）：按角色展示待办队列，区块「详情 →」直达工单管理并自带筛选
        path: '',
        name: 'Home',
        component: () => import('@/views/home/index.vue'),
        meta: { title: '首页', permissions: ['store', 'reviewer'] }
      },
      {
        path: 'orders',
        name: 'OrderList',
        component: () => import('@/views/orders/list.vue'),
        // 按角色自动切换：店方看全域（默认待接单），单位审核员看本单位（默认待审核）
        meta: { title: '工单管理', permissions: ['store', 'reviewer'] }
      },
      {
        // 必须注册在 orders/:id 之前，否则会被 :id 抢占匹配（id="create"）
        path: 'orders/create',
        name: 'OrderCreate',
        component: () => import('@/views/orders/create.vue'),
        // 审核员/业务员也可代单位上报；后端会校验其是否为该单位已审批成员
        meta: { title: '上报工单', permissions: ['store', 'reviewer'] }
      },
      {
        path: 'orders/:id',
        name: 'OrderDetail',
        component: () => import('@/views/orders/detail.vue'),
        meta: { title: '工单处理', permissions: ['store', 'reviewer'] }
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
        // 企业列表仅超管（业务维修员无权管理企业）；审核员从首页直达自己单位的详情页
        meta: { title: '企业管理', permissions: ['super'] }
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
      },
      {
        path: 'repairers',
        name: 'RepairerScope',
        component: () => import('@/views/repairers/index.vue'),
        // V1.5 经营范围分配：仅超级管理员（业务员数据权限管控入口）
        meta: { title: '经营范围', permissions: ['super'] }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/index.vue'),
        // 系统设置（完工时限等超管参数）：仅超级管理员
        meta: { title: '系统设置', permissions: ['super'] }
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

  // 已登录访问登录页 → 按角色落地（首页 '/' 本身可访问，不参与重定向，否则 landingPath='/' 会构成死循环）
  if (to.path === '/login' && userStore.token) {
    return userStore.landingPath
  }

  // 权限校验：requirements 全部不满足 → 回到自己的落地页
  if (to.meta.requiresAuth && userStore.token && !canAccess(to.meta.permissions as RoutePermission[] | undefined)) {
    return userStore.landingPath
  }

  return true
})

export default router
