<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { InfiniteLoading, Notify } from '@nutui/nutui'
import {
  errorMessage,
  fetchAds,
  fetchBoards,
  fetchFollowingPostsPage,
  fetchNotices,
  fetchPostsPage,
  fetchRecommendedPostsPage,
  togglePostLike,
  type AdSlot,
  type Board,
  type Notice,
  type Post,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import FeedPostRow from '@/components/FeedPostRow.vue'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const boards = ref<Board[]>([])
const posts = ref<Post[]>([])
const ads = ref<AdSlot[]>([])
const notices = ref<Notice[]>([])
const loading = ref(true)
const loadingMore = ref(false)
const nextOffset = ref(0)
const hasMore = ref(true)
const error = ref('')
const search = ref('')
const noticeOpen = ref(false)
const feedMode = ref<'for-you' | 'following'>('for-you')
const activeAdIndex = ref(0)
const carouselPaused = ref(false)
const likingPostIDs = ref<number[]>([])
let adTimer: number | undefined
let feedRequestVersion = 0

const activeBoard = computed(() => boards.value.find((item) => item.slug === String(route.params.slug || '')))
const pageTitle = computed(() => activeBoard.value?.name || (route.params.slug ? '板块内容' : '首页'))
const isHome = computed(() => !route.params.slug)
const viewingFollowing = computed(() => isHome.value && feedMode.value === 'following')
const requiresLoginForFeed = computed(() => viewingFollowing.value && !auth.user)
const activeAd = computed(() => ads.value[activeAdIndex.value] || null)
const pinnedPosts = computed(() => posts.value.filter((post) => post.pinned))
const normalPosts = computed(() => posts.value.filter((post) => !post.pinned))
const feedLabel = computed(() => {
  if (requiresLoginForFeed.value) return '正在关注'
  if (route.params.slug) return `${pageTitle.value}的帖子`
  return viewingFollowing.value ? '正在关注' : '为你推荐'
})

async function fetchFeedPage(offset: number) {
  if (requiresLoginForFeed.value) return { items: [] as Post[], next_offset: 0, has_more: false }
  if (isHome.value && !viewingFollowing.value) return fetchRecommendedPostsPage(offset, 30)
  return viewingFollowing.value
    ? fetchFollowingPostsPage(offset, 30)
    : fetchPostsPage({ board: String(route.params.slug || ''), offset, limit: 30 })
}

async function load() {
  const requestVersion = ++feedRequestVersion
  loading.value = true
  error.value = ''
  nextOffset.value = 0
  hasMore.value = true

  try {
    const tasks: Promise<unknown>[] = [
      fetchBoards().then((value) => { if (requestVersion === feedRequestVersion) boards.value = value }),
      fetchFeedPage(0).then((page) => {
        if (requestVersion !== feedRequestVersion) return
        posts.value = page.items
        nextOffset.value = page.next_offset
        hasMore.value = page.has_more
      }),
    ]

    if (isHome.value) {
      tasks.push(fetchAds().then((value) => { if (requestVersion === feedRequestVersion) ads.value = value }))
      tasks.push(fetchNotices().then((value) => { if (requestVersion === feedRequestVersion) notices.value = value }))
    } else {
      if (requestVersion === feedRequestVersion) {
        ads.value = []
        notices.value = []
      }
    }

    await Promise.all(tasks)
  } catch (cause) {
    if (requestVersion !== feedRequestVersion) return
    error.value = errorMessage(cause, '社区内容加载失败')
  } finally {
    if (requestVersion !== feedRequestVersion) return
    loading.value = false
  }
}

async function loadMore(done?: () => void) {
  if (loading.value || loadingMore.value || !hasMore.value || requiresLoginForFeed.value) {
    done?.()
    return
  }
  const requestVersion = feedRequestVersion
  loadingMore.value = true
  try {
    const page = await fetchFeedPage(nextOffset.value)
    if (requestVersion !== feedRequestVersion) return
    const known = new Set(posts.value.map((item) => item.id))
    posts.value = [...posts.value, ...page.items.filter((item) => !known.has(item.id))]
    nextOffset.value = page.next_offset
    hasMore.value = page.has_more
  } catch (cause) {
    if (requestVersion !== feedRequestVersion) return
    error.value = errorMessage(cause, '更多内容加载失败')
  } finally {
    if (requestVersion === feedRequestVersion) loadingMore.value = false
    done?.()
  }
}

async function refreshFeed(done?: () => void) {
  try {
    await load()
  } finally {
    done?.()
  }
}

function submitSearch() {
  const q = search.value.trim()
  router.push({ path: '/search', query: q ? { q } : {} })
}

function openAd(ad: AdSlot) {
  if (ad.link_url) window.open(ad.link_url, '_blank', 'noopener,noreferrer')
}

function selectAd(index: number) {
  activeAdIndex.value = index
}

function stopAdTimer() {
  if (adTimer !== undefined) window.clearInterval(adTimer)
  adTimer = undefined
}

function startAdTimer() {
  stopAdTimer()
  if (ads.value.length < 2) return
  adTimer = window.setInterval(() => {
    if (!carouselPaused.value) activeAdIndex.value = (activeAdIndex.value + 1) % ads.value.length
  }, 4500)
}

function formatDate(value: string) {
  try {
    return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(new Date(value))
  } catch {
    return value
  }
}

function excerpt(value: string) {
  const text = (value || '').replace(/\s+/g, ' ').trim()
  return text.length > 150 ? `${text.slice(0, 150)}...` : text
}

async function likePost(item: Post) {
  if (!auth.user) {
    await router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (likingPostIDs.value.includes(item.id)) return
  likingPostIDs.value = [...likingPostIDs.value, item.id]
  try {
    Object.assign(item, await togglePostLike(item.id))
  } catch (cause) {
    Notify.danger(errorMessage(cause, '点赞失败'))
  } finally {
    likingPostIDs.value = likingPostIDs.value.filter((id) => id !== item.id)
  }
}

function openPostMedia(item: Post, index: number) {
  router.push({ path: `/posts/${item.id}`, query: { media: String(index) } })
}

async function sharePost(item: Post) {
  const url = new URL(`/posts/${item.id}`, window.location.origin).href
  try {
    if (navigator.share) {
      await navigator.share({ title: item.title || excerpt(item.content), url })
      return
    }
    await navigator.clipboard.writeText(url)
    Notify.success('帖子链接已复制')
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    Notify.warn('分享失败，请打开帖子后复制地址')
  }
}

watch(() => route.params.slug, () => { load() })
watch(feedMode, load)
watch(() => auth.user?.id, () => {
  if (isHome.value && feedMode.value === 'following') load()
})
watch(ads, () => {
  activeAdIndex.value = 0
  startAdTimer()
})
onMounted(load)
onBeforeUnmount(stopAdTimer)
</script>

<template>
  <PageContainer>
    <div class="rf-home-sticky">
      <header class="rf-home-titlebar">
        <div>
          <h1>{{ isHome ? '首页' : pageTitle }}</h1>
          <small v-if="route.params.slug">{{ activeBoard?.description }}</small>
        </div>
        <div class="rf-home-header-actions">
          <button v-if="isHome" type="button" class="rf-home-notice" aria-label="查看社区公告" @click="noticeOpen = true">
            <AppIcon name="notice" size="19" />
            <span v-if="notices.length" aria-hidden="true">{{ notices.length > 99 ? '99+' : notices.length }}</span>
          </button>
          <template v-if="!auth.user">
            <RouterLink to="/login" class="rf-text-button">登录</RouterLink>
            <RouterLink to="/register" class="rf-primary-button">注册</RouterLink>
          </template>
          <RouterLink v-else to="/posts/new" class="rf-home-write-link">
            <AppIcon name="edit" size="17" />
            <span>发布</span>
          </RouterLink>
        </div>
      </header>

      <nut-tabs v-if="isHome" v-model="feedMode" class="rf-feed-tabs" type="line" color="var(--primary)">
        <nut-tabpane title="为你推荐" pane-key="for-you" />
        <nut-tabpane title="正在关注" pane-key="following" />
      </nut-tabs>
    </div>

    <form class="rf-home-mobile-search" role="search" @submit.prevent="submitSearch">
      <button v-if="isHome" type="button" class="rf-home-notice" aria-label="查看社区公告" @click="noticeOpen = true">
        <AppIcon name="notice" size="19" />
        <span v-if="notices.length" aria-hidden="true">{{ notices.length > 99 ? '99+' : notices.length }}</span>
      </button>
      <nut-searchbar v-model="search" clearable placeholder="搜索帖子、攻略、资源" @search="submitSearch" @keyup.enter.prevent="submitSearch" />
      <nut-button type="primary" size="small" native-type="button" @click="submitSearch">搜索</nut-button>
    </form>

    <section v-if="auth.user && isHome" class="rf-composer">
      <UserAvatar :src="auth.user.avatar_url" :name="auth.user.display_name" :size="42" />
      <RouterLink to="/posts/new">有什么新鲜事？</RouterLink>
      <nut-button type="primary" size="small" @click="router.push('/posts/new')">发布</nut-button>
    </section>

    <section
      v-if="isHome && activeAd"
      class="rf-ad-strip rf-ad-carousel"
      aria-label="社区广告"
      aria-roledescription="轮播图"
      @mouseenter="carouselPaused = true"
      @mouseleave="carouselPaused = false"
      @focusin="carouselPaused = true"
      @focusout="carouselPaused = false"
    >
      <Transition name="rf-ad-fade" mode="out-in">
        <button
          :key="activeAd.id"
          type="button"
          class="rf-ad-slide"
          :class="{ linked: activeAd.link_url }"
          :disabled="!activeAd.link_url"
          :aria-label="activeAd.link_url ? `打开广告：${activeAd.title || '社区广告'}` : undefined"
          @click="openAd(activeAd)"
        >
          <img v-if="activeAd.image_url" :src="activeAd.image_url" :alt="activeAd.title || '社区广告'" />
          <span v-else><small>广告</small><strong>{{ activeAd.title || '社区广告位' }}</strong></span>
          <em>广告</em>
        </button>
      </Transition>
      <div v-if="ads.length > 1" class="rf-ad-pagination" role="tablist" aria-label="选择广告">
        <button
          v-for="(ad, index) in ads"
          :key="`dot-${ad.id}`"
          type="button"
          role="tab"
          :aria-label="`显示第 ${index + 1} 个广告`"
          :aria-selected="index === activeAdIndex"
          :class="{ active: index === activeAdIndex }"
          @click="selectAd(index)"
        />
      </div>
    </section>

    <div v-if="error" class="rf-inline-error" role="alert">
      {{ error }}
      <button type="button" @click="load">重试</button>
    </div>

    <section v-if="pinnedPosts.length && !requiresLoginForFeed" class="rf-pinned">
      <div class="rf-section-label"><span>置顶帖子</span><AppIcon name="notice" size="16" /></div>
      <RouterLink v-for="post in pinnedPosts" :key="`p-${post.id}`" :to="`/posts/${post.id}`" class="rf-pinned-row">
        <span class="rf-pin-label">置顶</span>
        <strong>{{ post.title || excerpt(post.content) }}</strong>
        <small>{{ post.board_name }}</small>
      </RouterLink>
    </section>

    <div class="rf-section-label rf-feed-label">
      <span>{{ feedLabel }}</span>
      <small v-if="!loading && !requiresLoginForFeed">{{ posts.length }} 条</small>
    </div>

    <section v-if="requiresLoginForFeed" class="rf-feed-gate">
      <div class="rf-feed-gate-mark"><AppIcon name="people" size="23" /></div>
      <h2>看看你关注的人</h2>
      <p>登录后，关注的创作者和玩家的最新动态会出现在这里。</p>
      <RouterLink to="/login" class="rf-primary-button">登录查看</RouterLink>
    </section>

    <div v-else-if="loading" class="rf-feed-loading" aria-label="正在加载动态">
      <nut-skeleton v-for="n in 4" :key="n" animated avatar title row="2" height="14px" />
    </div>

    <InfiniteLoading
      v-else
      class="rf-feed-infinite"
      :has-more="hasMore"
      :is-open-refresh="true"
      :threshold="260"
      pull-icon="refresh"
      load-icon="loading"
      pull-txt="下拉刷新"
      load-txt="正在加载更多内容"
      load-more-txt="已经看到这里了"
      @load-more="loadMore"
      @refresh="refreshFeed"
    >
      <section class="rf-timeline">
        <FeedPostRow
          v-for="post in normalPosts"
          :key="post.id"
          :post="post"
          :liking="likingPostIDs.includes(post.id)"
          @like="likePost"
          @open-media="openPostMedia"
          @share="sharePost"
        />

        <div v-if="!normalPosts.length" class="rf-empty">
          <nut-empty description="这里还没有内容" />
          <span>成为第一个分享想法的人吧。</span>
          <nut-button v-if="auth.user" type="primary" @click="router.push('/posts/new')">发布第一篇帖子</nut-button>
        </div>
      </section>
      <template #loading><div class="rf-feed-more"><span class="rf-inline-spinner" aria-hidden="true" />加载下一批 30 条内容</div></template>
      <template #finished><div v-if="normalPosts.length" class="rf-feed-finished">没有更多内容了</div></template>
    </InfiniteLoading>

    <nut-popup v-model:visible="noticeOpen" position="right" :style="{ width: 'min(420px, 100vw)', height: '100%' }" closeable round>
      <section class="rf-notice-sheet"><header><h2>社区公告</h2></header><div v-if="!notices.length" class="rf-empty">暂无公告</div><article v-for="item in notices" :key="item.id"><div><b v-if="item.pinned">置顶</b><strong>{{ item.title }}</strong></div><p>{{ item.content }}</p><time>{{ formatDate(item.updated_at || item.created_at) }}</time></article></section>
    </nut-popup>
  </PageContainer>
</template>

<style scoped>
.rf-home-sticky { position: sticky; top: 0; z-index: 8; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 90%, transparent); backdrop-filter: blur(14px); }
.rf-home-titlebar { display: flex; min-height: 56px; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 16px 6px; }
.rf-home-titlebar > div:first-child { display: flex; min-width: 0; flex-direction: column; gap: 1px; }
.rf-home-titlebar h1 { margin: 0; font-family: var(--rf-font-display); font-size: 20px; line-height: 1.2; }
.rf-home-titlebar small { overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rf-home-header-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }
.rf-home-write-link { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 0 14px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 13px; font-weight: 700; }
.rf-home-write-link:hover { color: #fff; background: var(--primary-hover); }
.rf-home-notice { position: relative; display: inline-grid; width: 38px; height: 38px; flex: 0 0 38px; place-items: center; border-radius: 50%; color: var(--rf-text); background: transparent; }
.rf-home-notice:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-home-notice > span { position: absolute; top: -3px; right: -3px; display: grid; min-width: 17px; height: 17px; padding: 0 4px; place-items: center; border: 2px solid var(--rf-bg); border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 10px; font-variant-numeric: tabular-nums; line-height: 1; }
.rf-feed-tabs { margin: 0 -1px -1px; }
.rf-feed-tabs :deep(.nut-tabs__content) { display: none; }
.rf-feed-tabs :deep(.nut-tabs__titles) { min-height: 44px; background: transparent; }
.rf-feed-tabs :deep(.nut-tabs__titles-item) { min-height: 44px; font-weight: 600; }
.rf-home-mobile-search { display: none; }
.rf-composer { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); }
.rf-composer > a { min-width: 0; flex: 1; color: var(--rf-muted); }.rf-composer > a:hover { color: var(--primary); }.rf-composer .nut-button { flex: 0 0 auto; }
.rf-ad-strip { position: relative; overflow: hidden; border-bottom: 1px solid var(--rf-line); background: var(--rf-bg-subtle); }
.rf-ad-carousel { height: 132px; }
.rf-ad-slide { position: absolute; inset: 0; display: block; width: 100%; height: 100%; padding: 0; color: inherit; background: var(--rf-bg-subtle); text-align: left; cursor: default; }
.rf-ad-slide.linked { cursor: pointer; }.rf-ad-slide img { width: 100%; height: 100%; object-fit: cover; }
.rf-ad-slide > span { display: flex; height: 100%; flex-direction: column; align-items: center; justify-content: center; gap: 4px; }.rf-ad-slide small, .rf-ad-slide em { color: var(--rf-muted); font-size: 11px; font-style: normal; }.rf-ad-slide em { position: absolute; top: 9px; left: 12px; padding: 2px 6px; border-radius: var(--rf-pill); color: #fff; background: rgba(0,0,0,.5); }
.rf-ad-pagination { position: absolute; z-index: 2; right: 0; bottom: 10px; left: 0; display: flex; align-items: center; justify-content: center; gap: 6px; pointer-events: none; }.rf-ad-pagination button { width: 7px; height: 7px; padding: 0; border: 1px solid rgba(15,20,25,.25); border-radius: 50%; background: rgba(255,255,255,.72); box-shadow: 0 1px 3px rgba(15,20,25,.18); pointer-events: auto; transition: width 180ms ease-out, border-radius 180ms ease-out, background-color 140ms ease-out; }.rf-ad-pagination button.active { width: 18px; border-radius: var(--rf-pill); background: var(--primary); }.rf-ad-fade-enter-active { transition: opacity 220ms ease-out; }.rf-ad-fade-leave-active { transition: opacity 140ms ease-in; }.rf-ad-fade-enter-from, .rf-ad-fade-leave-to { opacity: 0; }
.rf-section-label { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 8px; color: var(--rf-muted); font-size: 13px; font-weight: 700; }.rf-section-label small { font-weight: 400; }.rf-feed-label { border-top: 1px solid var(--rf-line); }
.rf-inline-error { margin: 12px 16px; padding: 12px 14px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }.rf-inline-error button { margin-left: 8px; color: var(--primary); background: transparent; font-weight: 700; }
.rf-pinned { border-top: 1px solid var(--rf-line); }.rf-pinned-row { display: flex; min-height: 46px; align-items: center; gap: 10px; padding: 8px 16px; border-top: 1px solid var(--rf-line); }.rf-pinned-row:hover { background: var(--rf-bg-hover); }.rf-pinned-row strong { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-pinned-row small { color: var(--rf-muted); }.rf-pin-label { padding: 2px 7px; border-radius: var(--rf-pill); color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 10%, transparent); font-size: 11px; }
.rf-feed-gate { display: flex; min-height: 300px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 36px 26px; text-align: center; }.rf-feed-gate-mark { display: grid; width: 48px; height: 48px; margin-bottom: 5px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 11%, transparent); }.rf-feed-gate h2 { margin: 0; font-size: 21px; }.rf-feed-gate p { max-width: 32ch; margin: 0 0 12px; color: var(--rf-muted); line-height: 1.55; }
.rf-feed-loading { display: flex; flex-direction: column; gap: 16px; padding: 18px 16px; }.rf-feed-loading :deep(.nut-skeleton) { padding: 0; }
.rf-feed-infinite :deep(.nut-infinite-top) { color: var(--rf-muted); background: var(--rf-bg); }.rf-feed-infinite :deep(.nut-infinite-top .nut-icon) { color: var(--primary); }.rf-feed-more, .rf-feed-finished { display: flex; min-height: 54px; align-items: center; justify-content: center; gap: 8px; color: var(--rf-muted); font-size: 13px; }.rf-feed-finished { border-top: 1px solid var(--rf-line); }.rf-feed-more .rf-inline-spinner { width: 16px; height: 16px; border: 2px solid color-mix(in srgb, var(--primary) 25%, transparent); border-top-color: var(--primary); border-radius: 50%; animation: rf-feed-spin .7s linear infinite; }@keyframes rf-feed-spin { to { transform: rotate(360deg); } }
.rf-timeline { border-top: 1px solid var(--rf-line); }
.rf-empty { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 42px 20px; color: var(--rf-muted); text-align: center; }.rf-empty > span { color: var(--rf-muted); }.rf-empty .nut-empty { padding: 0; }.rf-empty .nut-button { margin-top: 8px; }
.rf-notice-sheet { height: 100%; overflow-y: auto; padding: 22px 20px; background: var(--rf-bg); }.rf-notice-sheet header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }.rf-notice-sheet h2 { margin: 0; font-size: 20px; }.rf-notice-sheet article { padding: 16px 0; border-bottom: 1px solid var(--rf-line); }.rf-notice-sheet article div { display: flex; align-items: center; gap: 8px; }.rf-notice-sheet article b { color: var(--rf-danger); font-size: 12px; }.rf-notice-sheet article p { color: var(--rf-muted); white-space: pre-wrap; line-height: 1.6; }.rf-notice-sheet article time { color: var(--rf-faint); font-size: 12px; }
@media (max-width: 1019px) { .rf-home-sticky { top: var(--rf-header); } .rf-home-titlebar { display: none; } .rf-home-mobile-search { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--rf-line); } .rf-home-mobile-search :deep(.nut-searchbar) { min-width: 0; flex: 1; padding: 0; border-radius: var(--rf-pill); } .rf-home-mobile-search :deep(.nut-searchbar__search-input) { border-radius: var(--rf-pill); } .rf-home-mobile-search > .nut-button { flex: 0 0 auto; } }
@media (max-width: 560px) { .rf-ad-carousel { height: 104px; }.rf-composer { display: none; }.rf-pinned-row { padding-inline: 12px; }.rf-section-label { padding-inline: 12px; }.rf-home-mobile-search { padding-inline: 10px; }.rf-feed-gate { min-height: 260px; }.rf-home-mobile-search .rf-home-notice { width: 36px; height: 36px; flex-basis: 36px; } }
</style>
