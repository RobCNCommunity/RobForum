<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createComment,
  deleteComment,
  errorMessage,
  fetchBookmarkStatus,
  fetchCommentLike,
  fetchComments,
  fetchPost,
  fetchPostLike,
  fetchRepostStatus,
  pinPost,
  toggleCommentLike,
  togglePostBookmark,
  togglePostLike,
  togglePostRepost,
  type Comment,
  type Post,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import { postTypeLabel } from '@/postTypes'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const post = ref<Post | null>(null)
const comments = ref<Comment[]>([])
const comment = ref('')
const replyInput = ref<HTMLTextAreaElement | null>(null)
const loading = ref(true)
const pinning = ref(false)
const sending = ref(false)
const likingPost = ref(false)
const bookmarking = ref(false)
const reposting = ref(false)
const busyCommentID = ref(0)
const postMenuOpen = ref(false)
const commentMenuID = ref(0)

const canPin = computed(() => auth.isAdmin)
const authorHandle = computed(() => post.value ? `@user_${post.value.author_id}` : '')

function formatPostDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function formatReplyDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const options: Intl.DateTimeFormatOptions = date.getFullYear() === now.getFullYear()
    ? { month: 'numeric', day: 'numeric' }
    : { year: 'numeric', month: 'numeric', day: 'numeric' }
  return new Intl.DateTimeFormat('zh-CN', options).format(date)
}

function formatCount(value: number) {
  return new Intl.NumberFormat('zh-CN', { notation: value >= 10000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(value || 0)
}

async function load() {
  loading.value = true
  postMenuOpen.value = false
  commentMenuID.value = 0
  try {
    const id = Number(route.params.id)
    if (!Number.isInteger(id) || id < 1) throw new Error('invalid post id')
    const [item, list] = await Promise.all([fetchPost(id), fetchComments(id)])
    post.value = item
    comments.value = list
    if (auth.user) {
      const [like, bookmark, repost] = await Promise.all([
        fetchPostLike(id).catch(() => null),
        fetchBookmarkStatus(id).catch(() => null),
        fetchRepostStatus(id).catch(() => null),
      ])
      if (like && post.value) Object.assign(post.value, like)
      if (bookmark && post.value) post.value.bookmarked = bookmark.bookmarked
      if (repost && post.value) Object.assign(post.value, repost)
      await Promise.all(comments.value.map(async (item) => {
        const state = await fetchCommentLike(item.id).catch(() => null)
        if (state) Object.assign(item, state)
      }))
    }
  } catch (error) {
    Notify.danger(errorMessage(error, '帖子加载失败'))
    router.push('/')
  } finally {
    loading.value = false
  }
}

function requireLogin() {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

function focusReply() {
  if (!auth.user) {
    requireLogin()
    return
  }
  replyInput.value?.focus()
  replyInput.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

async function submitComment() {
  const content = comment.value.trim()
  if (!post.value || !content || sending.value) return
  sending.value = true
  try {
    comments.value.push(await createComment(post.value.id, content))
    comment.value = ''
    post.value.comment_count += 1
    Notify.success('回复已发布')
  } catch (error) {
    Notify.danger(errorMessage(error, '回复发布失败'))
  } finally {
    sending.value = false
  }
}

async function togglePin() {
  if (!post.value) return
  pinning.value = true
  postMenuOpen.value = false
  try {
    post.value = await pinPost(post.value.id, !post.value.pinned)
    Notify.success(post.value.pinned ? '已置顶' : '已取消置顶')
  } catch (error) {
    Notify.danger(errorMessage(error, '置顶操作失败'))
  } finally {
    pinning.value = false
  }
}

async function likePost() {
  if (!post.value) return
  if (!auth.user) {
    requireLogin()
    return
  }
  likingPost.value = true
  try {
    Object.assign(post.value, await togglePostLike(post.value.id))
  } catch (error) {
    Notify.danger(errorMessage(error, '点赞失败'))
  } finally {
    likingPost.value = false
  }
}

async function bookmarkPost() {
  if (!post.value) return
  if (!auth.user) {
    requireLogin()
    return
  }
  bookmarking.value = true
  try {
    post.value.bookmarked = (await togglePostBookmark(post.value.id)).bookmarked
    Notify.success(post.value.bookmarked ? '已收藏' : '已取消收藏')
  } catch (error) {
    Notify.danger(errorMessage(error, '收藏失败'))
  } finally {
    bookmarking.value = false
  }
}

async function repostPost() {
  if (!post.value) return
  if (!auth.user) {
    requireLogin()
    return
  }
  reposting.value = true
  try {
    Object.assign(post.value, await togglePostRepost(post.value.id))
    Notify.success(post.value.reposted ? '已转发' : '已取消转发')
  } catch (error) {
    Notify.danger(errorMessage(error, '转发失败'))
  } finally {
    reposting.value = false
  }
}

async function likeComment(item: Comment) {
  if (!auth.user) {
    requireLogin()
    return
  }
  busyCommentID.value = item.id
  try {
    Object.assign(item, await toggleCommentLike(item.id))
  } catch (error) {
    Notify.danger(errorMessage(error, '点赞失败'))
  } finally {
    busyCommentID.value = 0
  }
}

function canDelete(item: Comment) {
  return !!auth.user && (auth.isAdmin || auth.user.id === item.author_id)
}

async function removeComment(item: Comment) {
  commentMenuID.value = 0
  if (!window.confirm('确定删除这条回复吗？')) return
  busyCommentID.value = item.id
  try {
    await deleteComment(item.id)
    comments.value = comments.value.filter((value) => value.id !== item.id)
    if (post.value) post.value.comment_count = Math.max(0, post.value.comment_count - 1)
    Notify.success('回复已删除')
  } catch (error) {
    Notify.danger(errorMessage(error, '删除回复失败'))
  } finally {
    busyCommentID.value = 0
  }
}

async function copyLink(url = window.location.href) {
  try {
    await navigator.clipboard.writeText(url)
    Notify.success('链接已复制')
  } catch {
    Notify.warn('请复制浏览器地址栏链接')
  }
}

async function sharePost() {
  if (!post.value) return
  try {
    if (navigator.share) {
      await navigator.share({ title: post.value.title, url: window.location.href })
      return
    }
    await copyLink()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    Notify.warn('分享失败，请复制浏览器地址栏链接')
  }
}

function openModeration() {
  if (!post.value) return
  postMenuOpen.value = false
  router.push({ path: '/admin/posts', query: { post_id: post.value.id } })
}

watch(() => route.params.id, load)
onMounted(load)
</script>

<template>
  <PageContainer>
    <section class="rf-x-status-page">
      <header class="rf-x-status-header">
        <button type="button" class="rf-x-status-round" aria-label="返回" @click="router.back()">
          <AppIcon name="back" size="21" />
        </button>
        <strong>帖子</strong>
      </header>

      <div v-if="loading" class="rf-x-status-loading" aria-label="正在加载帖子">
        <div><span /><i /><i /></div>
        <b /><b /><b />
      </div>

      <template v-else-if="post">
        <article class="rf-x-status-post">
          <header class="rf-x-status-author">
            <RouterLink :to="`/users/${post.author_id}`" :aria-label="`查看 ${post.author_name} 的个人主页`">
              <UserAvatar :src="post.author_avatar" :name="post.author_name" :size="48" />
            </RouterLink>
            <RouterLink :to="`/users/${post.author_id}`" class="rf-x-status-author-copy">
              <span><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" /></span>
              <small>{{ authorHandle }}</small>
            </RouterLink>
            <div class="rf-x-status-menu">
              <button type="button" class="rf-x-status-round" aria-label="更多帖子操作" :aria-expanded="postMenuOpen" @click="postMenuOpen = !postMenuOpen">
                <AppIcon name="more" size="20" />
              </button>
              <Transition name="rf-x-menu">
                <div v-if="postMenuOpen" class="rf-x-status-menu-panel">
                  <button type="button" @click="postMenuOpen = false; copyLink()">复制帖子链接</button>
                  <button v-if="canPin" type="button" :disabled="pinning" @click="togglePin">{{ post.pinned ? '取消置顶' : '置顶帖子' }}</button>
                  <button v-if="canPin" type="button" class="danger" @click="openModeration">审核或删除</button>
                </div>
              </Transition>
            </div>
          </header>

          <div class="rf-x-status-context">
            <span v-if="post.pinned">置顶</span>
            <span v-if="post.featured">精华</span>
            <span>{{ post.board_name }}</span>
            <span>·</span>
            <span>{{ postTypeLabel(post.post_type) }}</span>
          </div>
          <h1>{{ post.title }}</h1>
          <div v-if="post.content" class="rf-x-status-content">{{ post.content }}</div>
          <PostMediaGrid v-if="post.media?.length" :media="post.media" />

          <div class="rf-x-status-date">
            <time>{{ formatPostDate(post.created_at) }}</time><span>·</span><strong>{{ formatCount(post.views) }}</strong><span>次浏览</span>
          </div>
          <div class="rf-x-status-stats" aria-label="帖子互动统计">
            <span><strong>{{ formatCount(post.comment_count) }}</strong> 回复</span>
            <span><strong>{{ formatCount(post.repost_count) }}</strong> 转发</span>
            <span><strong>{{ formatCount(post.like_count) }}</strong> 喜欢</span>
          </div>
          <div class="rf-x-status-actions" role="group" aria-label="帖子操作">
            <button type="button" aria-label="回复" @click="focusReply"><AppIcon name="message" size="21" /></button>
            <button type="button" aria-label="转发" :class="{ reposted: post.reposted }" :disabled="reposting" :aria-pressed="post.reposted" @click="repostPost"><AppIcon name="repost" size="21" /></button>
            <button type="button" aria-label="点赞" :class="{ liked: post.liked }" :disabled="likingPost" :aria-pressed="post.liked" @click="likePost"><AppIcon name="heart" size="21" /></button>
            <button type="button" aria-label="收藏" :class="{ bookmarked: post.bookmarked }" :disabled="bookmarking" :aria-pressed="post.bookmarked" @click="bookmarkPost"><AppIcon name="star" size="21" /></button>
            <button type="button" aria-label="分享" @click="sharePost"><AppIcon name="share" size="21" /></button>
          </div>
        </article>

        <form v-if="auth.user" class="rf-x-reply-composer" @submit.prevent="submitComment">
          <UserAvatar :src="auth.user.avatar_url" :name="auth.user.display_name" :size="42" />
          <div>
            <textarea ref="replyInput" v-model="comment" rows="2" maxlength="5000" placeholder="发布你的回复" @keydown.ctrl.enter.prevent="submitComment" />
            <footer>
              <span>{{ comment.length }}/5000</span>
              <button type="submit" :disabled="!comment.trim() || sending">{{ sending ? '发布中' : '回复' }}</button>
            </footer>
          </div>
        </form>
        <button v-else type="button" class="rf-x-reply-login" @click="requireLogin">登录后发布回复</button>

        <section class="rf-x-replies" aria-label="帖子回复">
          <article v-for="item in comments" :id="`reply-${item.id}`" :key="item.id" class="rf-x-reply-row">
            <RouterLink :to="`/users/${item.author_id}`" :aria-label="`查看 ${item.author_name} 的个人主页`">
              <UserAvatar :src="item.author_avatar" :name="item.author_name" :size="40" />
            </RouterLink>
            <div class="rf-x-reply-body">
              <div class="rf-x-reply-meta">
                <RouterLink :to="`/users/${item.author_id}`"><strong>{{ item.author_name }}</strong></RouterLink>
                <VerifiedBadge :verified="item.author_verified" :label="item.author_verification_label" />
                <span>@user_{{ item.author_id }}</span><span>·</span><time>{{ formatReplyDate(item.created_at) }}</time>
                <div v-if="canDelete(item)" class="rf-x-reply-menu">
                  <button type="button" aria-label="更多回复操作" :aria-expanded="commentMenuID === item.id" @click="commentMenuID = commentMenuID === item.id ? 0 : item.id"><AppIcon name="more" size="18" /></button>
                  <Transition name="rf-x-menu">
                    <div v-if="commentMenuID === item.id"><button type="button" :disabled="busyCommentID === item.id" @click="removeComment(item)">删除回复</button></div>
                  </Transition>
                </div>
              </div>
              <p>{{ item.content }}</p>
              <div class="rf-x-reply-actions">
                <button type="button" aria-label="点赞回复" :class="{ liked: item.liked }" :disabled="busyCommentID === item.id" @click="likeComment(item)"><AppIcon name="heart" size="17" /><span>{{ formatCount(item.like_count) }}</span></button>
                <button type="button" aria-label="复制回复链接" @click="copyLink(`${window.location.origin}${route.path}#reply-${item.id}`)"><AppIcon name="share" size="17" /></button>
              </div>
            </div>
          </article>
          <div v-if="!comments.length" class="rf-x-replies-empty"><nut-empty description="还没有回复" /></div>
        </section>
      </template>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-x-status-page { min-width: 0; min-height: 100vh; background: var(--rf-bg); }
.rf-x-status-header { position: sticky; top: 0; z-index: 12; display: grid; min-height: 56px; grid-template-columns: 44px minmax(0, 1fr) 44px; align-items: center; gap: 8px; padding: 4px 12px; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 88%, transparent); backdrop-filter: blur(12px); }
.rf-x-status-header strong { font-size: 20px; }
.rf-x-status-round { display: inline-grid; width: 40px; height: 40px; place-items: center; border-radius: 50%; color: var(--rf-text); background: transparent; transition: background-color 150ms ease-out, transform 100ms ease-out; }
.rf-x-status-round:hover { background: var(--rf-bg-hover); }
.rf-x-status-round:active { transform: scale(.94); }
.rf-x-status-post { padding: 12px 16px 0; border-bottom: 1px solid var(--rf-line); }
.rf-x-status-author { position: relative; display: grid; grid-template-columns: 48px minmax(0, 1fr) 40px; align-items: center; gap: 10px; }
.rf-x-status-author-copy { display: flex; min-width: 0; flex-direction: column; color: inherit; }
.rf-x-status-author-copy > span { display: flex; min-width: 0; align-items: center; gap: 5px; }
.rf-x-status-author-copy strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-status-author-copy small { overflow: hidden; color: var(--rf-muted); font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-status-menu { position: relative; }
.rf-x-status-menu-panel { position: absolute; z-index: 20; top: 42px; right: 0; width: max-content; min-width: 176px; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 12px; background: var(--rf-bg); box-shadow: 0 8px 30px rgba(15, 20, 25, .16); }
.rf-x-status-menu-panel button { display: block; width: 100%; min-height: 44px; padding: 0 16px; color: var(--rf-text); background: transparent; font-weight: 650; text-align: left; }
.rf-x-status-menu-panel button:hover { background: var(--rf-bg-hover); }
.rf-x-status-menu-panel button.danger { color: var(--rf-danger); }
.rf-x-status-context { display: flex; flex-wrap: wrap; align-items: center; gap: 5px; margin-top: 17px; color: var(--rf-muted); font-size: 13px; }
.rf-x-status-context span:first-child { color: var(--primary); font-weight: 650; }
.rf-x-status-post h1 { margin: 8px 0 8px; font-size: 23px; line-height: 1.3; overflow-wrap: anywhere; }
.rf-x-status-content { font-size: 18px; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }
.rf-x-status-date { display: flex; flex-wrap: wrap; align-items: center; gap: 5px; margin-top: 15px; padding-bottom: 14px; color: var(--rf-muted); font-size: 14px; }
.rf-x-status-date strong { color: var(--rf-text); font-variant-numeric: tabular-nums; }
.rf-x-status-stats { display: flex; flex-wrap: wrap; gap: 18px; min-height: 50px; align-items: center; border-top: 1px solid var(--rf-line); color: var(--rf-muted); font-size: 14px; }
.rf-x-status-stats strong { color: var(--rf-text); font-variant-numeric: tabular-nums; }
.rf-x-status-actions { display: grid; min-height: 52px; grid-template-columns: repeat(5, minmax(0, 1fr)); align-items: center; border-top: 1px solid var(--rf-line); }
.rf-x-status-actions button { display: inline-grid; width: 40px; height: 40px; place-self: center; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; transition: color 150ms ease-out, background-color 150ms ease-out, transform 100ms ease-out; }
.rf-x-status-actions button:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-x-status-actions button:active { transform: scale(.92); }
.rf-x-status-actions button:disabled { cursor: wait; opacity: .5; }
.rf-x-status-actions button.reposted { color: var(--rf-success); }
.rf-x-status-actions button.liked { color: #f91880; }
.rf-x-status-actions button.bookmarked { color: var(--primary); }
.rf-x-reply-composer { display: flex; gap: 11px; padding: 13px 16px 10px; border-bottom: 1px solid var(--rf-line); }
.rf-x-reply-composer > div { min-width: 0; flex: 1; }
.rf-x-reply-composer textarea { display: block; width: 100%; min-height: 58px; padding: 8px 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 18px; line-height: 1.45; resize: vertical; }
.rf-x-reply-composer footer { display: flex; min-height: 38px; align-items: center; justify-content: flex-end; gap: 12px; border-top: 1px solid var(--rf-line); }
.rf-x-reply-composer footer span { color: var(--rf-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.rf-x-reply-composer footer button { min-width: 68px; min-height: 34px; padding: 0 16px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-weight: 700; }
.rf-x-reply-composer footer button:disabled { cursor: not-allowed; opacity: .45; }
.rf-x-reply-login { width: 100%; min-height: 52px; border-bottom: 1px solid var(--rf-line); color: var(--primary); background: transparent; font-weight: 650; }
.rf-x-reply-login:hover { background: var(--rf-bg-hover); }
.rf-x-reply-row { display: flex; min-width: 0; gap: 10px; padding: 12px 16px 9px; border-bottom: 1px solid var(--rf-line); transition: background-color 150ms ease-out; }
.rf-x-reply-row:hover { background: var(--rf-bg-hover); }
.rf-x-reply-body { min-width: 0; flex: 1; }
.rf-x-reply-meta { position: relative; display: flex; min-width: 0; min-height: 22px; align-items: center; gap: 4px; color: var(--rf-muted); font-size: 13px; }
.rf-x-reply-meta > a { min-width: 0; }
.rf-x-reply-meta strong { display: block; overflow: hidden; color: var(--rf-text); text-overflow: ellipsis; white-space: nowrap; }
.rf-x-reply-meta > span, .rf-x-reply-meta time { flex: 0 0 auto; white-space: nowrap; }
.rf-x-reply-body > p { margin: 2px 0 8px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.rf-x-reply-menu { position: relative; margin-left: auto; }
.rf-x-reply-menu > button { display: inline-grid; width: 30px; height: 30px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }
.rf-x-reply-menu > button:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-x-reply-menu > div { position: absolute; z-index: 10; top: 32px; right: 0; width: max-content; min-width: 136px; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg); box-shadow: 0 8px 24px rgba(15, 20, 25, .15); }
.rf-x-reply-menu > div button { width: 100%; min-height: 42px; padding: 0 14px; color: var(--rf-danger); background: transparent; font-weight: 650; text-align: left; }
.rf-x-reply-menu > div button:hover { background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.rf-x-reply-actions { display: flex; max-width: 220px; align-items: center; justify-content: space-between; color: var(--rf-muted); }
.rf-x-reply-actions button { display: inline-flex; min-width: 38px; min-height: 32px; align-items: center; gap: 5px; padding: 0 7px; border-radius: var(--rf-pill); color: inherit; background: transparent; font-size: 12px; }
.rf-x-reply-actions button:hover, .rf-x-reply-actions button.liked { color: #f91880; background: rgba(249, 24, 128, .08); }
.rf-x-reply-actions button:last-child:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 9%, transparent); }
.rf-x-replies-empty { padding: 32px 16px; }
.rf-x-menu-enter-active { transition: opacity 150ms ease-out, transform 170ms cubic-bezier(.22, 1, .36, 1); }
.rf-x-menu-leave-active { transition: opacity 90ms ease-in, transform 90ms ease-in; }
.rf-x-menu-enter-from, .rf-x-menu-leave-to { opacity: 0; transform: translateY(-4px) scale(.98); }
.rf-x-status-loading { padding-bottom: 28px; overflow: hidden; }
.rf-x-status-loading > div { display: grid; grid-template-columns: 48px minmax(0, 1fr); gap: 10px; padding: 16px; }
.rf-x-status-loading span, .rf-x-status-loading i, .rf-x-status-loading b { display: block; border-radius: var(--rf-pill); background: var(--rf-bg-subtle); animation: rf-x-status-pulse 1.2s ease-in-out infinite alternate; }
.rf-x-status-loading span { width: 48px; height: 48px; grid-row: 1 / 3; border-radius: 50%; }
.rf-x-status-loading i { width: 42%; height: 13px; }
.rf-x-status-loading i:last-child { width: 27%; }
.rf-x-status-loading b { height: 14px; margin: 8px 16px; }
.rf-x-status-loading b:nth-of-type(2) { width: 86%; }
.rf-x-status-loading b:nth-of-type(3) { width: 64%; }
@keyframes rf-x-status-pulse { from { opacity: .52; } to { opacity: 1; } }

@media (max-width: 560px) {
  .rf-x-status-header { padding-inline: 8px; }
  .rf-x-status-post { padding-inline: 12px; }
  .rf-x-status-author { grid-template-columns: 44px minmax(0, 1fr) 40px; }
  .rf-x-status-author :deep(.rf-user-avatar) { width: 44px !important; height: 44px !important; flex-basis: 44px !important; }
  .rf-x-status-post h1 { font-size: 21px; }
  .rf-x-status-content { font-size: 17px; }
  .rf-x-status-stats { gap: 13px; font-size: 13px; }
  .rf-x-reply-composer, .rf-x-reply-row { padding-inline: 12px; }
  .rf-x-reply-composer textarea { font-size: 16px; }
  .rf-x-reply-meta time { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .rf-x-status-loading span, .rf-x-status-loading i, .rf-x-status-loading b { animation: none; }
}
</style>
