<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { errorMessage } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useNotificationsStore } from '@/stores/notifications'
import SideNavigation from '@/components/SideNavigation.vue'
import RightRail from '@/components/RightRail.vue'
import AppIcon from '@/components/AppIcon.vue'
import SidebarAccount from '@/components/SidebarAccount.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const route = useRoute(); const router = useRouter(); const auth = useAuthStore(); const siteStore = useSiteStore(); const notifications = useNotificationsStore()
const site = computed(() => siteStore.settings); const mobileOpen = ref(false); const mobileActionsOpen = ref(false); const isMobile = ref(false); const bootLoading = ref(true); const bootError = ref(''); const signingOut = ref(false)
const siteName = computed(() => site.value?.site_name || '罗布玩家社区')
const brandImage = computed(() => site.value?.logo_url || site.value?.avatar_url || '')
const brandInitial = computed(() => siteName.value.trim().charAt(0).toUpperCase() || 'R')
const publicRoute = computed(() => Boolean(route.meta.public))
const pageTitle = computed(() => String(route.meta.title || '社区首页'))
const mobileFocusMode = computed(() => route.path === '/posts/new')
const profileRoute = computed(() => /^\/users\/\d+$/.test(route.path))
const postDetailRoute = computed(() => /^\/posts\/\d+$/.test(route.path))
const profileEditRoute = computed(() => route.path === '/settings/profile')
const ownsMainHeader = computed(() => profileRoute.value || postDetailRoute.value || profileEditRoute.value)
const mobileBottomNavVisible = computed(() => !mobileFocusMode.value && !profileRoute.value && !profileEditRoute.value)
const mobileFABVisible = computed(() => {
  const path = route.path
  return path === '/' || path.startsWith('/boards/') || (path.startsWith('/posts/') && path !== '/posts/new') || /^\/users\/\d+$/.test(path)
})
const selectedKey = computed(() => { const p = route.path; if (p.startsWith('/admin/')) return p.startsWith('/admin/settings')?'/admin/settings':p.startsWith('/admin/ads')?'/admin/ads':p.startsWith('/admin/notices')?'/admin/notices':p.startsWith('/admin/resources')?'/admin/resources':p.startsWith('/admin/payouts')?'/admin/payouts':p.startsWith('/admin/verifications')?'/admin/verifications':p.startsWith('/admin/users')?'/admin/users':'/admin/posts'; if (p.startsWith('/settings')) return '/settings/profile'; if (p.startsWith('/verification')) return '/verification/apply'; if (p.startsWith('/resources')) return '/resources'; if (p.startsWith('/wallet')) return '/wallet'; if (p.startsWith('/messages')) return '/messages'; if (p.startsWith('/notifications')) return '/notifications'; if (p.startsWith('/bookmarks')) return '/bookmarks'; if (p.startsWith('/users')) return auth.user && p === `/users/${auth.user.id}` ? p : '/users'; if (p.startsWith('/boards/guides')) return '/boards/guides'; if (p.startsWith('/boards/team-up')) return '/boards/team-up'; return '/' })
function updateViewport() { isMobile.value = window.innerWidth < 1020 }
function handleSessionExpired() {
  const hadUser = !!auth.user
  auth.invalidateSession()
  notifications.markRead()
  if (hadUser && (route.meta.requiresAuth || route.meta.admin)) {
    router.replace({ path: '/login', query: { redirect: route.fullPath } })
  }
}
onMounted(async () => { updateViewport(); window.addEventListener('resize', updateViewport); window.addEventListener('robforum:session-expired', handleSessionExpired); try { await Promise.all([siteStore.load(), auth.bootstrap()]); if (auth.user) await notifications.refresh(); document.title = siteName.value } catch (e) { bootError.value = errorMessage(e, '站点暂时无法加载') } finally { bootLoading.value = false } })
onBeforeUnmount(() => { window.removeEventListener('resize', updateViewport); window.removeEventListener('robforum:session-expired', handleSessionExpired) })
watch(() => route.path, () => { mobileOpen.value = false; mobileActionsOpen.value = false })
async function signOut() {
  if (signingOut.value) return
  signingOut.value = true
  try {
    await auth.logout()
    await router.push('/login')
  } finally {
    signingOut.value = false
  }
}
async function runMobileAction(path: string) { mobileActionsOpen.value = false; await router.push(path) }
</script>

<template>
  <div v-if="bootLoading" class="rf-boot"><div class="rf-boot-mark">{{ brandInitial }}</div><strong>{{ siteName }}</strong><div class="rf-boot-line"><i /></div><p>正在加载社区</p></div>
  <div v-else-if="publicRoute" class="rf-auth-shell"><div class="rf-auth-brand"><RouterLink to="/"><span class="rf-brand-mark">{{ brandInitial }}</span><strong>{{ siteName }}</strong></RouterLink></div><RouterView /></div>
  <div v-else class="rf-app-shell">
    <Transition name="rf-rail"><aside v-if="!isMobile" class="rf-left-rail"><RouterLink to="/" class="rf-brand"><img v-if="brandImage" :src="brandImage" :alt="siteName" /><span v-else class="rf-brand-mark">{{ brandInitial }}</span><strong>{{ siteName }}</strong></RouterLink><SideNavigation :selected-key="selectedKey" /><SidebarAccount v-if="auth.user" :user="auth.user" :busy="signingOut" @logout="signOut" /></aside></Transition>
    <header v-if="isMobile && !ownsMainHeader" class="rf-mobile-header"><button type="button" class="rf-icon-button" aria-label="打开导航" @click="mobileOpen = true"><AppIcon name="menu" size="21" /></button><RouterLink to="/" class="rf-mobile-brand"><img v-if="brandImage" :src="brandImage" :alt="siteName" /><span v-else class="rf-brand-mark">{{ brandInitial }}</span></RouterLink><button type="button" class="rf-icon-button" aria-label="搜索用户" @click="router.push('/users')"><AppIcon name="search" size="20" /></button></header>
    <main class="rf-main-column" :class="{ 'rf-mobile-focus': isMobile && mobileFocusMode, 'rf-profile-route': profileRoute }"><header v-if="!ownsMainHeader" class="rf-main-header"><div><h1>{{ pageTitle }}</h1><span v-if="bootError" class="rf-error-inline">{{ bootError }}</span></div><div class="rf-header-actions"><RouterLink v-if="auth.user && !['/', '/posts/new'].includes(route.path)" to="/posts/new" class="rf-primary-button">发布</RouterLink><template v-else-if="!auth.user"><RouterLink to="/login" class="rf-text-button">登录</RouterLink><RouterLink to="/register" class="rf-primary-button">注册</RouterLink></template></div></header><a v-if="site?.banner_enabled && site.banner_text" class="rf-global-banner" :href="site.banner_link || undefined" target="_blank" rel="noopener noreferrer">{{ site.banner_text }}</a><RouterView v-slot="{ Component, route: childRoute }"><Transition name="rf-route" mode="out-in" appear><component :is="Component" :key="childRoute.fullPath" /></Transition></RouterView></main>
    <RightRail />
    <Transition name="rf-drawer"><div v-if="isMobile && mobileOpen" class="rf-mobile-overlay" @click.self="mobileOpen = false"><aside class="rf-mobile-drawer"><div class="rf-drawer-head"><RouterLink to="/" class="rf-brand" @click="mobileOpen=false"><span class="rf-brand-mark">{{ brandInitial }}</span><strong>{{ siteName }}</strong></RouterLink><button type="button" class="rf-icon-button" aria-label="关闭导航" @click="mobileOpen=false"><AppIcon name="close" size="20" /></button></div><SideNavigation :selected-key="selectedKey" @select="mobileOpen=false" /><SidebarAccount v-if="auth.user" :user="auth.user" :busy="signingOut" @logout="signOut" /></aside></div></Transition>
    <nav v-if="isMobile && mobileBottomNavVisible" class="rf-bottom-nav" aria-label="移动导航"><RouterLink to="/" :class="{active:selectedKey==='/' }"><AppIcon name="home" size="22" /><span>首页</span></RouterLink><RouterLink to="/users" :class="{active:selectedKey==='/users'}"><AppIcon name="search" size="22" /><span>发现</span></RouterLink><RouterLink to="/boards/general" :class="{active:route.path.startsWith('/boards/')}"><AppIcon name="category" size="22" /><span>板块</span></RouterLink><RouterLink :to="auth.user ? '/messages' : '/login'" :class="{active:selectedKey==='/messages'}"><AppIcon name="message" size="22" /><span>{{ auth.user ? '消息' : '登录' }}</span></RouterLink><RouterLink :to="auth.user ? `/users/${auth.user.id}` : '/login'" :class="{active:auth.user && selectedKey===`/users/${auth.user.id}`}" ><UserAvatar v-if="auth.user" :src="auth.user.avatar_url" :name="auth.user.display_name" :size="22" /><AppIcon v-else name="user" size="22" /><span>我的</span></RouterLink></nav>
    <button v-if="isMobile && mobileFABVisible" type="button" class="rf-mobile-compose-fab" :class="{ 'rf-mobile-compose-fab--navless': !mobileBottomNavVisible }" :aria-label="auth.user ? '打开发布菜单' : '打开账号菜单'" @click="mobileActionsOpen = true"><AppIcon :name="auth.user ? 'edit' : 'user'" size="23" /></button>
    <nut-popup v-if="isMobile" v-model:visible="mobileActionsOpen" position="bottom" round closeable pop-class="rf-mobile-actions-popup">
      <section class="rf-mobile-actions" :aria-label="auth.user ? '发布菜单' : '账号菜单'">
        <header><h2>{{ auth.user ? '创建' : '加入社区' }}</h2></header>
        <template v-if="auth.user">
          <button type="button" @click="runMobileAction('/posts/new')"><span><AppIcon name="edit" size="20" /></span><div><strong>发布帖子</strong><small>分享动态、攻略或组队信息</small></div><AppIcon name="arrow" size="17" /></button>
          <button type="button" @click="runMobileAction('/resources')"><span><AppIcon name="shop" size="20" /></span><div><strong>投稿资源</strong><small>上传并提交资源审核</small></div><AppIcon name="arrow" size="17" /></button>
          <button type="button" @click="runMobileAction('/messages')"><span><AppIcon name="people" size="20" /></span><div><strong>创建群聊</strong><small>邀请社区用户开始交流</small></div><AppIcon name="arrow" size="17" /></button>
        </template>
        <template v-else>
          <button type="button" @click="runMobileAction('/login')"><span><AppIcon name="user" size="20" /></span><div><strong>登录</strong><small>继续使用收藏、消息和发布功能</small></div><AppIcon name="arrow" size="17" /></button>
          <button type="button" @click="runMobileAction('/register')"><span><AppIcon name="people" size="20" /></span><div><strong>创建账号</strong><small>加入 Roblox 中文玩家社区</small></div><AppIcon name="arrow" size="17" /></button>
        </template>
      </section>
    </nut-popup>
  </div>
</template>
