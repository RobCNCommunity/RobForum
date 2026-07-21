<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage, fetchAds, fetchBoards, fetchFollowingPosts, fetchNotices, fetchPosts, type AdSlot, type Board, type Notice, type Post } from '@/api'
import { useAuthStore } from '@/stores/auth'
import PageContainer from '@/components/PageContainer.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import AppIcon from '@/components/AppIcon.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { postTypeLabel } from '@/postTypes'

const route = useRoute()
const auth = useAuthStore()
const boards = ref<Board[]>([])
const posts = ref<Post[]>([])
const ads = ref<AdSlot[]>([])
const notices = ref<Notice[]>([])
const loading = ref(true)
const error = ref('')
const search = ref('')
const noticeOpen = ref(false)
const feedMode = ref<'for-you' | 'following'>('for-you')
const activeAdIndex = ref(0)
const carouselPaused = ref(false)
let adTimer: number | undefined

const activeBoard = computed(() => boards.value.find((item) => item.slug === String(route.params.slug || '')))
const pageTitle = computed(() => activeBoard.value?.name || (route.params.slug ? '板块内容' : '首页'))
const isHome = computed(() => !route.params.slug)
const pinnedPosts = computed(() => posts.value.filter((post) => post.pinned))
const normalPosts = computed(() => posts.value.filter((post) => !post.pinned))
const activeAd = computed(() => ads.value[activeAdIndex.value] || null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const tasks: Promise<unknown>[] = [
      fetchBoards().then((value) => { boards.value = value }),
      (isHome.value && feedMode.value === 'following' && auth.user
        ? fetchFollowingPosts()
        : fetchPosts({ board: String(route.params.slug || ''), q: search.value.trim() || undefined })).then((value) => { posts.value = value }),
    ]
    if (isHome.value) {
      tasks.push(fetchAds().then((value) => { ads.value = value }))
      tasks.push(fetchNotices().then((value) => { notices.value = value }))
    } else {
      ads.value = []
      notices.value = []
    }
    await Promise.all(tasks)
  } catch (e) {
    error.value = errorMessage(e, '社区内容加载失败')
  } finally {
    loading.value = false
  }
}

function submitSearch() { load() }
function formatDate(value: string) { try { return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) } catch { return value } }
function excerpt(value: string) { const text = (value || '').replace(/\s+/g, ' ').trim(); return text.length > 120 ? text.slice(0, 120) + '…' : text }
function openAd(ad: AdSlot) { if (ad.link_url) window.open(ad.link_url, '_blank', 'noopener,noreferrer') }
function selectAd(index: number) { activeAdIndex.value = index }
function stopAdTimer() { if (adTimer !== undefined) window.clearInterval(adTimer); adTimer = undefined }
function startAdTimer() {
  stopAdTimer()
  if (ads.value.length < 2) return
  adTimer = window.setInterval(() => {
    if (!carouselPaused.value) activeAdIndex.value = (activeAdIndex.value + 1) % ads.value.length
  }, 4500)
}

watch(() => route.params.slug, () => { search.value = ''; load() })
watch(feedMode, load)
watch(ads, () => { activeAdIndex.value = 0; startAdTimer() })
onMounted(load)
onBeforeUnmount(stopAdTimer)
</script>

<template>
  <PageContainer :title="route.params.slug ? pageTitle : undefined">
    <nut-tabs v-if="isHome && auth.user" v-model="feedMode" class="rf-feed-tabs" type="line" color="var(--primary)">
      <nut-tabpane title="为你推荐" pane-key="for-you" />
      <nut-tabpane title="正在关注" pane-key="following" />
    </nut-tabs>

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

    <div class="rf-feed-search">
      <button v-if="isHome" type="button" class="rf-notice-button" aria-label="打开社区公告" @click="noticeOpen = true">
        <AppIcon name="notice" size="19" />
        <span v-if="notices.length" aria-hidden="true">{{ notices.length > 99 ? '99+' : notices.length }}</span>
      </button>
      <nut-searchbar v-model="search" clearable :placeholder="isHome ? '搜索帖子、攻略、资源' : `在“${pageTitle}”中搜索`" @search="submitSearch" />
      <nut-button type="primary" size="small" @click="submitSearch">搜索</nut-button>
    </div>

    <section v-if="auth.user && isHome" class="rf-composer">
      <UserAvatar :src="auth.user.avatar_url" :name="auth.user.display_name" :size="42" />
      <RouterLink to="/posts/new">有什么新鲜事？分享给社区</RouterLink>
      <nut-button type="primary" size="small" @click="$router.push('/posts/new')">发布</nut-button>
    </section>

    <div v-if="error" class="rf-inline-error">{{ error }}<button type="button" @click="load">重试</button></div>
    <section v-if="pinnedPosts.length" class="rf-pinned"><div class="rf-section-label"><span>置顶帖子</span><AppIcon name="notice" size="16" /></div><RouterLink v-for="post in pinnedPosts" :key="`p-${post.id}`" :to="`/posts/${post.id}`" class="rf-pinned-row"><span class="rf-pin-label">置顶</span><strong>{{ post.title }}</strong><small>{{ post.board_name }}</small></RouterLink></section>

    <div class="rf-section-label rf-feed-label"><span>{{ route.params.slug ? '板块帖子' : '最新动态' }}</span><small v-if="!loading">{{ posts.length }} 条</small></div>
    <div v-if="loading" class="rf-feed-loading"><nut-skeleton v-for="n in 4" :key="n" animated avatar title row="2" height="14px" /></div>
    <section v-else class="rf-timeline">
      <RouterLink v-for="post in normalPosts" :key="post.id" :to="`/posts/${post.id}`" class="rf-post-row">
        <UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" />
        <div class="rf-post-body"><div class="rf-post-meta"><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" /><span>@{{ post.author_id }}</span><span>·</span><time>{{ formatDate(post.created_at) }}</time><span class="rf-post-more"><AppIcon name="more" size="17" /></span></div><div class="rf-post-type"><span v-if="post.featured">精华</span><span>{{ postTypeLabel(post.post_type) }}</span><em>{{ post.board_name }}</em></div><h2>{{ post.title }}</h2><p v-if="excerpt(post.content)">{{ excerpt(post.content) }}</p><PostMediaGrid v-if="post.media?.length" :media="post.media" compact :preview="false" /><div class="rf-post-actions"><span><AppIcon name="message" size="17" />{{ post.comment_count }}</span><span :class="{ reposted: post.reposted }"><AppIcon name="repost" size="17" />{{ post.repost_count || 0 }}</span><span :class="{ liked: post.liked }"><AppIcon name="heart" size="17" />{{ post.like_count || 0 }}</span><span><AppIcon name="share" size="17" /></span></div></div>
      </RouterLink>
      <div v-if="!normalPosts.length" class="rf-empty"><nut-empty description="这里还没有内容" /><span>成为第一个分享想法的人吧。</span><nut-button v-if="auth.user" type="primary" @click="$router.push('/posts/new')">发布第一篇帖子</nut-button></div>
    </section>

    <section class="rf-boards"><div class="rf-section-label"><span>社区板块</span><AppIcon name="people" size="16" /></div><RouterLink v-for="board in boards" :key="board.id" :to="`/boards/${board.slug}`" class="rf-board-row" :class="{ active: board.slug === route.params.slug }"><span class="rf-board-icon"><AppIcon :name="board.icon === 'shop' ? 'shop' : board.icon === 'team' ? 'people' : 'message'" size="19" /></span><span><strong>{{ board.name }}</strong><small>{{ board.description }}</small></span><b>{{ board.post_count }}</b></RouterLink></section>

    <nut-popup v-model:visible="noticeOpen" position="right" :style="{ width: 'min(420px, 100vw)', height: '100%' }" closeable round>
      <section class="rf-notice-sheet"><header><h2>社区公告</h2></header><div v-if="!notices.length" class="rf-empty">暂无公告</div><article v-for="item in notices" :key="item.id"><div><b v-if="item.pinned">置顶</b><strong>{{ item.title }}</strong></div><p>{{ item.content }}</p><time>{{ formatDate(item.updated_at || item.created_at) }}</time></article></section>
    </nut-popup>
  </PageContainer>
</template>

<style scoped>
.rf-feed-tabs { border-top: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); }
.rf-feed-tabs :deep(.nut-tabs__content) { display: none; }
.rf-feed-tabs :deep(.nut-tabs__titles) { background: var(--rf-bg); }
.rf-ad-strip { position: relative; display: flex; width: 100%; min-height: 100px; max-height: 180px; align-items: center; justify-content: center; margin: 0 0 4px; overflow: hidden; border-bottom: 1px solid var(--rf-line); background: var(--rf-bg-subtle); }
.rf-ad-carousel { min-height: 148px; max-height: 148px; padding: 0; }
.rf-ad-slide { position: absolute; inset: 0; display: block; width: 100%; height: 100%; padding: 0; color: inherit; background: var(--rf-bg-subtle); text-align: left; cursor: default; }
.rf-ad-slide.linked { cursor: pointer; }
.rf-ad-slide img { width: 100%; height: 100%; object-fit: cover; }
.rf-ad-slide > span { display: flex; height: 100%; flex-direction: column; align-items: center; justify-content: center; gap: 4px; }
.rf-ad-slide small, .rf-ad-slide em { color: var(--rf-muted); font-size: 11px; font-style: normal; }
.rf-ad-slide em { position: absolute; top: 9px; left: 12px; padding: 2px 6px; border-radius: 999px; color: #fff; background: rgba(0,0,0,.5); }
.rf-ad-pagination { position: absolute; z-index: 2; right: 0; bottom: 10px; left: 0; display: flex; align-items: center; justify-content: center; gap: 6px; pointer-events: none; }
.rf-ad-pagination button { width: 7px; height: 7px; padding: 0; border: 1px solid rgba(15,20,25,.25); border-radius: 50%; background: rgba(255,255,255,.72); box-shadow: 0 1px 3px rgba(15,20,25,.18); pointer-events: auto; transition: width 180ms ease-out, border-radius 180ms ease-out, background-color 140ms ease-out; }
.rf-ad-pagination button.active { width: 18px; border-radius: 999px; background: var(--primary); }
.rf-ad-fade-enter-active { transition: opacity 220ms ease-out; }
.rf-ad-fade-leave-active { transition: opacity 140ms ease-in; }
.rf-ad-fade-enter-from, .rf-ad-fade-leave-to { opacity: 0; }
.rf-feed-search { display: flex; align-items: center; gap: 8px; margin: 12px 16px; }
.rf-notice-button { position: relative; display: grid; width: 40px; height: 40px; flex: 0 0 40px; place-items: center; border: 1px solid var(--rf-line); border-radius: 50%; color: var(--rf-text); background: var(--rf-bg); transition: color .16s ease, background-color .16s ease; }
.rf-notice-button:hover { color: var(--primary); background: var(--rf-bg-hover); }
.rf-notice-button > span { position: absolute; top: -4px; right: -4px; display: grid; min-width: 18px; height: 18px; padding: 0 4px; place-items: center; border: 2px solid var(--rf-bg); border-radius: 999px; color: #fff; background: var(--primary); font-size: 10px; font-variant-numeric: tabular-nums; line-height: 1; }
.rf-feed-search :deep(.nut-searchbar) { flex: 1; min-width: 0; padding: 0; border-radius: 999px; background: var(--rf-bg-subtle); }
.rf-feed-search :deep(.nut-searchbar__search-input) { border-radius: 999px; }
.rf-feed-search > .nut-button { flex: 0 0 auto; }
.rf-composer { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-top: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); }.rf-composer > a { flex: 1; color: var(--rf-muted); }.rf-composer > a:hover { color: var(--primary); }.rf-composer .nut-button { flex: 0 0 auto; }
.rf-section-label { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 8px; color: var(--rf-muted); font-size: 13px; font-weight: 700; }.rf-section-label small { font-weight: 400; }.rf-feed-label { border-top: 1px solid var(--rf-line); }
.rf-inline-error { margin: 12px 16px; padding: 12px 14px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }.rf-inline-error button { margin-left: 8px; color: var(--primary); background: transparent; font-weight: 700; }
.rf-pinned { border-top: 1px solid var(--rf-line); }.rf-pinned-row { display: flex; align-items: center; gap: 10px; min-height: 46px; padding: 8px 16px; border-top: 1px solid var(--rf-line); }.rf-pinned-row:hover { background: var(--rf-bg-hover); }.rf-pinned-row strong { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-pinned-row small { color: var(--rf-muted); }.rf-pin-label { padding: 2px 7px; border-radius: 999px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 10%, transparent); font-size: 11px; }
.rf-feed-loading { display: flex; flex-direction: column; gap: 16px; padding: 18px 16px; }.rf-feed-loading :deep(.nut-skeleton) { padding: 0; }
.rf-timeline { border-top: 1px solid var(--rf-line); }.rf-post-row { display: flex; gap: 12px; padding: 15px 16px 14px; border-bottom: 1px solid var(--rf-line); color: inherit; transition: background .16s ease; }.rf-post-row:hover { color: inherit; background: var(--rf-bg-hover); }.rf-post-body { min-width: 0; flex: 1; }.rf-post-meta { display: flex; align-items: center; gap: 5px; min-height: 22px; color: var(--rf-muted); font-size: 13px; }.rf-post-meta strong { color: var(--rf-text); font-size: 15px; }.rf-post-meta time { white-space: nowrap; }.rf-post-more { margin-left: auto; }.rf-post-type { display: flex; align-items: center; gap: 6px; margin: 4px 0; color: var(--rf-muted); font-size: 12px; }.rf-post-type span:first-child { color: var(--rf-danger); }.rf-post-type em { padding: 1px 6px; border-radius: 999px; background: var(--rf-bg-subtle); font-style: normal; }.rf-post-body h2 { margin: 3px 0 4px; font-family: var(--rf-font-display); font-size: 17px; line-height: 1.35; }.rf-post-body p { display: -webkit-box; margin: 0 0 9px; overflow: hidden; color: var(--rf-muted); line-height: 1.55; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }.rf-post-actions { display: flex; align-items: center; justify-content: space-between; max-width: 440px; color: var(--rf-muted); font-size: 12px; }.rf-post-actions span { display: inline-flex; align-items: center; gap: 5px; }.rf-post-actions span:hover, .rf-post-actions .liked { color: var(--primary); }
.rf-post-actions .reposted { color: var(--rf-success); }
.rf-empty { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 42px 20px; color: var(--rf-muted); text-align: center; }.rf-empty > span { color: var(--rf-muted); }.rf-empty .nut-empty { padding: 0; }.rf-empty .nut-button { margin-top: 8px; }
.rf-boards { border-top: 1px solid var(--rf-line); }.rf-board-row { display: flex; align-items: center; gap: 10px; padding: 11px 16px; border-top: 1px solid var(--rf-line); }.rf-board-row:hover, .rf-board-row.active { background: var(--rf-bg-hover); }.rf-board-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 12%, transparent); }.rf-board-row > span:nth-child(2) { display: flex; min-width: 0; flex: 1; flex-direction: column; }.rf-board-row strong { font-size: 14px; }.rf-board-row small { overflow: hidden; color: var(--rf-muted); text-overflow: ellipsis; white-space: nowrap; }.rf-board-row > b { color: var(--rf-muted); font-size: 12px; }
.rf-notice-sheet { height: 100%; overflow-y: auto; padding: 22px 20px; background: var(--rf-bg); }.rf-notice-sheet header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }.rf-notice-sheet h2 { margin: 0; font-size: 20px; }.rf-notice-sheet article { padding: 16px 0; border-bottom: 1px solid var(--rf-line); }.rf-notice-sheet article div { display: flex; align-items: center; gap: 8px; }.rf-notice-sheet article b { color: var(--rf-danger); font-size: 12px; }.rf-notice-sheet article p { color: var(--rf-muted); white-space: pre-wrap; line-height: 1.6; }.rf-notice-sheet article time { color: var(--rf-faint); font-size: 12px; }
@media (max-width: 560px) { .rf-ad-carousel { min-height: 108px; height: 108px; }.rf-composer { display: none; }.rf-post-row { padding: 13px 12px; }.rf-post-meta time { display: none; }.rf-post-body h2 { font-size: 16px; }.rf-pinned-row { padding-inline: 12px; }.rf-feed-search { margin-inline: 12px; }.rf-section-label { padding-inline: 12px; } }
</style>
