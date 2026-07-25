import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/HomeView.vue'), meta: { title: '社区首页' } },
    { path: '/boards/:slug', component: () => import('./views/HomeView.vue'), meta: { title: '社区板块' } },
    { path: '/posts/:id', component: () => import('./views/PostView.vue'), meta: { title: '帖子详情' } },
    { path: '/posts/new', component: () => import('./views/CreatePostView.vue'), meta: { requiresAuth: true, title: '发布新帖' } },
    { path: '/search', component: () => import('./views/SearchView.vue'), meta: { title: '搜索' } },
    { path: '/resources', component: () => import('./views/ResourceView.vue'), meta: { title: '资源中心' } },
    { path: '/resources/new', component: () => import('./views/CreateResourceView.vue'), meta: { requiresAuth: true, title: '发布资源' } },
    { path: '/resources/:id', component: () => import('./views/ResourceDetailView.vue'), meta: { title: '资源详情' } },
    { path: '/users', component: () => import('./views/UserSearchView.vue'), meta: { title: '用户发现' } },
    { path: '/users/:userID', component: () => import('./views/UserProfileView.vue'), meta: { title: '个人主页' } },
    { path: '/messages', component: () => import('./views/MessagesView.vue'), meta: { requiresAuth: true, title: '私信与群聊' } },
    { path: '/messages/join', component: () => import('./views/GroupJoinView.vue'), meta: { requiresAuth: true, title: '加入群聊' } },
    { path: '/notifications', component: () => import('./views/NotificationsView.vue'), meta: { requiresAuth: true, title: '通知' } },
    { path: '/bookmarks', component: () => import('./views/BookmarksView.vue'), meta: { requiresAuth: true, title: '收藏' } },
    { path: '/wallet', component: () => import('./views/WalletView.vue'), meta: { requiresAuth: true, title: '我的钱包' } },
    { path: '/avatar-frames', component: () => import('./views/AvatarFrameMarketView.vue'), meta: { requiresAuth: true, title: '头像框市场' } },
    { path: '/membership', component: () => import('./views/MembershipView.vue'), meta: { requiresAuth: true, title: '会员中心' } },
    { path: '/check-in', component: () => import('./views/CheckInView.vue'), meta: { requiresAuth: true, title: '签到与资历' } },
    { path: '/settings/profile', component: () => import('./views/ProfileSettingsView.vue'), meta: { requiresAuth: true, title: '资料设置' } },
    { path: '/verification/apply', component: () => import('./views/VerificationApplyView.vue'), meta: { requiresAuth: true, title: '蓝微认证' } },
    { path: '/login', component: () => import('./views/LoginView.vue'), meta: { public: true, title: '登录' } },
    { path: '/register', component: () => import('./views/RegisterView.vue'), meta: { public: true, title: '注册' } },
    { path: '/forgot-password', component: () => import('./views/ForgotPasswordView.vue'), meta: { public: true, title: '找回密码' } },
    { path: '/reset-password', component: () => import('./views/ResetPasswordView.vue'), meta: { public: true, title: '重置密码' } },
    { path: '/admin/ads', component: () => import('./views/AdminAdsView.vue'), meta: { admin: true, title: '广告管理' } },
    { path: '/admin/notices', component: () => import('./views/AdminNoticesView.vue'), meta: { admin: true, title: '公告管理' } },
    { path: '/admin/settings', component: () => import('./views/AdminSettingsView.vue'), meta: { admin: true, title: '系统设置' } },
    { path: '/admin/resources', component: () => import('./views/AdminResourcesView.vue'), meta: { admin: true, title: '资源审核' } },
    { path: '/admin/payouts', component: () => import('./views/AdminPayoutsView.vue'), meta: { admin: true, title: '提现审核' } },
    { path: '/admin/wallet', component: () => import('./views/AdminWalletView.vue'), meta: { admin: true, title: '钱包与兑换码' } },
    { path: '/admin/verifications', component: () => import('./views/AdminVerificationsView.vue'), meta: { admin: true, title: '蓝微审核' } },
    { path: '/admin/users', component: () => import('./views/AdminUsersView.vue'), meta: { admin: true, title: '用户管理' } },
    { path: '/admin/posts', component: () => import('./views/AdminPostsView.vue'), meta: { admin: true, title: '内容审核' } },
    { path: '/admin/avatar-frames', component: () => import('./views/AdminAvatarFramesView.vue'), meta: { admin: true, title: '头像框管理' } },
  ],
})

const chunkLoadError = /failed to fetch dynamically imported module|importing a module script failed|loading chunk .* failed|error loading dynamically imported module/i
const chunkReloadKey = (path: string) => `robforum:chunk-reload:${path}`

router.onError((error, to) => {
  if (typeof window === 'undefined' || !chunkLoadError.test(String(error))) return
  const key = chunkReloadKey(to.fullPath || '/')
  try {
    if (window.sessionStorage.getItem(key) === '1') return
    window.sessionStorage.setItem(key, '1')
  } catch {
    // A hard reload is still preferable when storage is unavailable.
  }
  window.location.replace(to.fullPath || '/')
})

router.afterEach((to) => {
  try { window.sessionStorage.removeItem(chunkReloadKey(to.fullPath || '/')) } catch { /* private mode */ }
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.bootstrap()
  if (to.meta.admin && !auth.isAdmin) return auth.isAuthenticated ? '/' : '/login'
  if (to.meta.requiresAuth && !auth.isAuthenticated) return { path: '/login', query: { redirect: to.fullPath } }
  if (to.meta.public && auth.isAuthenticated && ['/login', '/register'].includes(to.path)) return '/'
  return true
})

export default router
