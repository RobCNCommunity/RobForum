<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
  reportComment,
  reportPost,
  toggleCommentLike,
  togglePostBookmark,
  togglePostLike,
  togglePostRepost,
  type Comment,
  type Post,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import CommentComposer from '@/components/CommentComposer.vue'
import CommentThreadItem from '@/components/CommentThreadItem.vue'
import ContentReportDialog from '@/components/ContentReportDialog.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PageContainer from '@/components/PageContainer.vue'
import PostImageViewer from '@/components/PostImageViewer.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import PostTagList from '@/components/PostTagList.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import { buildCommentTree } from '@/commentTree'

interface CommentComposerHandle {
  focus: () => void
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const post = ref<Post | null>(null)
const comments = ref<Comment[]>([])
const comment = ref('')
const commentFiles = ref<File[]>([])
const commentPreviewURLs = ref<string[]>([])
const replyComposer = ref<CommentComposerHandle | null>(null)
const viewerReplyComposer = ref<CommentComposerHandle | null>(null)
const replyingTo = ref<Comment | null>(null)
const loading = ref(true)
const pinning = ref(false)
const sending = ref(false)
const likingPost = ref(false)
const bookmarking = ref(false)
const reposting = ref(false)
const busyCommentID = ref(0)
const postMenuOpen = ref(false)
const reportTarget = ref<{ type: 'post' | 'comment'; id: number; label: string } | null>(null)
const reporting = ref(false)
const viewerOpen = ref(false)
const viewerIndex = ref(0)

const canPin = computed(() => auth.isAdmin)
const authorHandle = computed(() => post.value ? `@user_${post.value.author_id}` : '')
const reportOpen = computed(() => reportTarget.value !== null)
const commentTree = computed(() => buildCommentTree(comments.value))
const viewerMedia = computed(() => (post.value?.media || []).filter((item) => item.mime_type.startsWith('image/')))

function requestedViewerIndex() {
  const value = Array.isArray(route.query.media) ? route.query.media[0] : route.query.media
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return null
  const index = Number(value)
  return Number.isSafeInteger(index) ? index : null
}

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

function formatCount(value: number) {
  return new Intl.NumberFormat('zh-CN', { notation: value >= 10000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(value || 0)
}

async function load() {
  loading.value = true
  postMenuOpen.value = false
  reportTarget.value = null
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
    const mediaIndex = requestedViewerIndex()
    if (mediaIndex !== null) openImageViewer(mediaIndex)
  } catch (error) {
    Notify.danger(errorMessage(error, '帖子加载失败'))
    router.push('/')
  } finally {
    loading.value = false
    await nextTick()
    const targetID = route.hash === '#reply-composer'
      ? 'reply-composer'
      : /^#reply-\d+$/.test(route.hash) ? route.hash.slice(1) : ''
    if (targetID) {
      document.getElementById(targetID)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
      if (targetID === 'reply-composer' && auth.user) replyComposer.value?.focus()
    }
  }
}

function requireLogin() {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

async function focusReply() {
  if (!auth.user) {
    requireLogin()
    return
  }
  replyingTo.value = null
  await nextTick()
  if (viewerOpen.value) {
    viewerReplyComposer.value?.focus()
    return
  }
  document.querySelector('#reply-composer')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  replyComposer.value?.focus()
}

async function replyToComment(item: Comment) {
  if (!auth.user) {
    requireLogin()
    return
  }
  replyingTo.value = item
  await nextTick()
  if (viewerOpen.value) {
    viewerReplyComposer.value?.focus()
    return
  }
  document.querySelector('#reply-composer')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  replyComposer.value?.focus()
}

function cancelCommentReply() {
  replyingTo.value = null
}

async function submitComment() {
  const content = comment.value.trim()
  if (!post.value || (!content && !commentFiles.value.length) || sending.value) return
  sending.value = true
  try {
    comments.value.push(await createComment(post.value.id, content, commentFiles.value, replyingTo.value?.id))
    comment.value = ''
    clearCommentFiles()
    replyingTo.value = null
    post.value.comment_count += 1
    Notify.success('回复已发布')
  } catch (error) {
    Notify.danger(errorMessage(error, '回复发布失败'))
  } finally {
    sending.value = false
  }
}

function chooseCommentFiles(files: File[]) {
  const remaining = 4 - commentFiles.value.length
  if (files.length > remaining) Notify.warn('每条回复最多上传 4 个媒体文件')
  const candidates = files.slice(0, Math.max(0, remaining))
  const accepted = candidates.filter((file) => {
    const image = ['image/png', 'image/jpeg'].includes(file.type) && file.size <= 5 * 1024 * 1024
    const video = ['video/mp4', 'video/webm'].includes(file.type) && file.size <= 50 * 1024 * 1024
    return image || video
  })
  if (accepted.length !== candidates.length) Notify.warn('仅支持 PNG/JPG（5 MB 内）或 MP4/WebM（50 MB 内）')
  commentFiles.value.push(...accepted)
  commentPreviewURLs.value.push(...accepted.map((file) => URL.createObjectURL(file)))
}

function removeCommentFile(index: number) {
  URL.revokeObjectURL(commentPreviewURLs.value[index])
  commentFiles.value.splice(index, 1)
  commentPreviewURLs.value.splice(index, 1)
}

function clearCommentFiles() {
  commentPreviewURLs.value.forEach((url) => URL.revokeObjectURL(url))
  commentFiles.value = []
  commentPreviewURLs.value = []
}

function openImageViewer(index: number) {
  if (!viewerMedia.value.length) return
  viewerIndex.value = Math.min(Math.max(index, 0), viewerMedia.value.length - 1)
  viewerOpen.value = true
}

function closeImageViewer() {
  viewerOpen.value = false
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

function openPostReport() {
  if (!post.value) return
  postMenuOpen.value = false
  if (!auth.user) {
    requireLogin()
    return
  }
  if (auth.user.id === post.value.author_id) return
  reportTarget.value = { type: 'post', id: post.value.id, label: `帖子：${post.value.title || post.value.content.slice(0, 40)}` }
}

function openCommentReport(item: Comment) {
  if (!auth.user) {
    requireLogin()
    return
  }
  if (auth.user.id === item.author_id) return
  if (viewerOpen.value) closeImageViewer()
  reportTarget.value = { type: 'comment', id: item.id, label: `回复：${item.author_name}` }
}

async function submitReport(reason: string) {
  const target = reportTarget.value
  if (!target || reporting.value) return
  reporting.value = true
  try {
    if (target.type === 'post') await reportPost(target.id, reason)
    else await reportComment(target.id, reason)
    reportTarget.value = null
    Notify.success('举报已提交，审核结果会通知你')
  } catch (error) {
    Notify.danger(errorMessage(error, '举报提交失败'))
  } finally {
    reporting.value = false
  }
}

async function removeComment(item: Comment) {
  if (!window.confirm('确定删除这条回复吗？')) return
  busyCommentID.value = item.id
  try {
    await deleteComment(item.id)
    comments.value = comments.value.filter((value) => value.id !== item.id)
    if (replyingTo.value?.id === item.id) replyingTo.value = null
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
      await navigator.share({ title: post.value.title || post.value.content.slice(0, 80), url: window.location.href })
      return
    }
    await copyLink()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    Notify.warn('分享失败，请复制浏览器地址栏链接')
  }
}

function shareComment(item: Comment) {
  return copyLink(`${window.location.origin}${route.path}#reply-${item.id}`)
}

function openModeration() {
  if (!post.value) return
  postMenuOpen.value = false
  router.push({ path: '/admin/posts', query: { post_id: post.value.id } })
}

watch(() => route.params.id, () => {
  if (viewerOpen.value) closeImageViewer()
  load()
})
onMounted(load)
onBeforeUnmount(() => {
  clearCommentFiles()
})
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
              <span><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" /><MembershipBadge :active="post.author_member" :tier-id="post.author_membership_tier_id" /></span>
              <small>{{ authorHandle }}</small>
            </RouterLink>
            <div class="rf-x-status-menu">
              <button type="button" class="rf-x-status-round" aria-label="更多帖子操作" :aria-expanded="postMenuOpen" @click="postMenuOpen = !postMenuOpen">
                <AppIcon name="more" size="20" />
              </button>
              <Transition name="rf-x-menu">
                <div v-if="postMenuOpen" class="rf-x-status-menu-panel">
                  <button type="button" @click="postMenuOpen = false; copyLink()">复制帖子链接</button>
                  <button v-if="!auth.user || auth.user.id !== post.author_id" type="button" class="danger" @click="openPostReport">举报帖子</button>
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
          </div>
          <PostTagList :tags="post.tags" class="rf-x-status-tags" />
          <h1 v-if="post.title">{{ post.title }}</h1>
          <MarkdownContent v-if="post.content" :source="post.content" class="rf-x-status-content" />
          <PostMediaGrid v-if="post.media?.length" :media="post.media" custom-preview @open-preview="openImageViewer" />

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

        <CommentComposer
          v-if="auth.user"
          id="reply-composer"
          ref="replyComposer"
          v-model="comment"
          :files="commentFiles"
          :preview-urls="commentPreviewURLs"
          :sending="sending"
          :replying-to="replyingTo"
          @submit="submitComment"
          @select-files="chooseCommentFiles"
          @remove-file="removeCommentFile"
          @cancel-reply="cancelCommentReply"
        />
        <button v-else id="reply-composer" type="button" class="rf-x-reply-login" @click="requireLogin">登录后发布回复</button>

        <section class="rf-x-replies" aria-label="帖子回复">
          <CommentThreadItem
            v-for="node in commentTree"
            :key="node.item.id"
            :node="node"
            :viewer-id="auth.user?.id || 0"
            :viewer-is-admin="auth.isAdmin"
            :busy-id="busyCommentID"
            :replying-to-id="replyingTo?.id || 0"
            @reply="replyToComment"
            @like="likeComment"
            @delete="removeComment"
            @report="openCommentReport"
            @share="shareComment"
          />
          <div v-if="!comments.length" class="rf-x-replies-empty"><nut-empty description="还没有回复" /></div>
        </section>
      </template>
    </section>
    <ContentReportDialog :open="reportOpen" :target-label="reportTarget?.label || ''" :submitting="reporting" @close="reportTarget = null" @submit="submitReport" />
    <PostImageViewer v-if="post" v-model:open="viewerOpen" :media="viewerMedia" :start-index="viewerIndex">
      <header class="rf-viewer-author">
        <RouterLink :to="`/users/${post.author_id}`" @click="closeImageViewer"><UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" /></RouterLink>
        <div>
          <RouterLink :to="`/users/${post.author_id}`" @click="closeImageViewer">
            <strong>{{ post.author_name }}</strong>
            <VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" />
            <MembershipBadge :active="post.author_member" :tier-id="post.author_membership_tier_id" />
          </RouterLink>
          <small>{{ authorHandle }} · {{ post.board_name }}</small>
        </div>
      </header>

      <section class="rf-viewer-post-copy">
        <PostTagList :tags="post.tags" compact class="rf-viewer-tags" />
        <h2 v-if="post.title">{{ post.title }}</h2>
        <MarkdownContent v-if="post.content" :source="post.content" class="rf-viewer-markdown" />
        <time>{{ formatPostDate(post.created_at) }}</time>
      </section>

      <div class="rf-viewer-stats">
        <span><strong>{{ formatCount(post.comment_count) }}</strong> 回复</span>
        <span><strong>{{ formatCount(post.repost_count) }}</strong> 转发</span>
        <span><strong>{{ formatCount(post.like_count) }}</strong> 喜欢</span>
      </div>
      <div class="rf-viewer-actions" role="group" aria-label="帖子操作">
        <button type="button" aria-label="回复" @click="focusReply"><AppIcon name="message" size="20" /></button>
        <button type="button" aria-label="转发" :class="{ reposted: post.reposted }" :disabled="reposting" :aria-pressed="post.reposted" @click="repostPost"><AppIcon name="repost" size="20" /></button>
        <button type="button" aria-label="点赞" :class="{ liked: post.liked }" :disabled="likingPost" :aria-pressed="post.liked" @click="likePost"><AppIcon name="heart" size="20" /></button>
        <button type="button" aria-label="收藏" :class="{ bookmarked: post.bookmarked }" :disabled="bookmarking" :aria-pressed="post.bookmarked" @click="bookmarkPost"><AppIcon name="star" size="20" /></button>
        <button type="button" aria-label="分享" @click="sharePost"><AppIcon name="share" size="20" /></button>
      </div>

      <CommentComposer
        v-if="auth.user"
        ref="viewerReplyComposer"
        v-model="comment"
        compact
        :files="commentFiles"
        :preview-urls="commentPreviewURLs"
        :sending="sending"
        :replying-to="replyingTo"
        @submit="submitComment"
        @select-files="chooseCommentFiles"
        @remove-file="removeCommentFile"
        @cancel-reply="cancelCommentReply"
      />
      <button v-else type="button" class="rf-viewer-login" @click="requireLogin">登录后发布回复</button>

      <section class="rf-viewer-comments" aria-label="帖子回复">
        <CommentThreadItem
          v-for="node in commentTree"
          :key="node.item.id"
          :node="node"
          compact
          :viewer-id="auth.user?.id || 0"
          :viewer-is-admin="auth.isAdmin"
          :busy-id="busyCommentID"
          :replying-to-id="replyingTo?.id || 0"
          @reply="replyToComment"
          @like="likeComment"
          @delete="removeComment"
          @report="openCommentReport"
          @share="shareComment"
        />
        <div v-if="!comments.length" class="rf-viewer-comments-empty">还没有回复</div>
      </section>
    </PostImageViewer>
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
.rf-x-status-tags { margin-top: 9px; }
.rf-x-status-post h1 { margin: 8px 0 8px; font-size: 23px; line-height: 1.3; overflow-wrap: anywhere; }
.rf-x-status-content { font-size: 18px; line-height: 1.55; }
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
.rf-x-reply-login { width: 100%; min-height: 52px; border-bottom: 1px solid var(--rf-line); color: var(--primary); background: transparent; font-weight: 650; }
.rf-x-reply-login:hover { background: var(--rf-bg-hover); }
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
.rf-viewer-author { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 10px; padding: 15px 16px 12px; border-bottom: 1px solid var(--rf-line); }
.rf-viewer-author > div, .rf-viewer-author a { min-width: 0; }
.rf-viewer-author > div > a { display: flex; align-items: center; gap: 5px; }
.rf-viewer-author strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rf-viewer-author small { display: block; overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rf-viewer-post-copy { padding: 13px 16px; border-bottom: 1px solid var(--rf-line); }
.rf-viewer-tags { margin-bottom: 8px; }
.rf-viewer-post-copy h2 { margin: 0 0 7px; font-size: 18px; line-height: 1.35; overflow-wrap: anywhere; }
.rf-viewer-markdown { margin: 0 0 10px; line-height: 1.55; }
.rf-viewer-post-copy time { color: var(--rf-muted); font-size: 12px; }
.rf-viewer-stats { display: flex; flex-wrap: wrap; gap: 13px; min-height: 44px; align-items: center; padding: 0 16px; border-bottom: 1px solid var(--rf-line); color: var(--rf-muted); font-size: 13px; }
.rf-viewer-stats strong { color: var(--rf-text); font-variant-numeric: tabular-nums; }
.rf-viewer-actions { display: grid; min-height: 48px; grid-template-columns: repeat(5, minmax(0, 1fr)); border-bottom: 1px solid var(--rf-line); }
.rf-viewer-actions button { display: inline-grid; width: 38px; height: 38px; place-self: center; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; transition: color 150ms ease-out, background-color 150ms ease-out, transform 100ms ease-out; }
.rf-viewer-actions button:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-viewer-actions button:active { transform: scale(.92); }
.rf-viewer-actions button:disabled { cursor: wait; opacity: .5; }
.rf-viewer-actions button.reposted { color: var(--rf-success); }
.rf-viewer-actions button.liked { color: #f91880; }
.rf-viewer-actions button.bookmarked { color: var(--primary); }
.rf-viewer-login { min-height: 46px; border-bottom: 1px solid var(--rf-line); color: var(--primary); background: transparent; font-weight: 650; }
.rf-viewer-login:hover { background: var(--rf-bg-hover); }
.rf-viewer-comments { display: flex; flex-direction: column; min-height: 0; }
.rf-viewer-comments-empty { padding: 26px 16px; color: var(--rf-muted); text-align: center; }

@media (max-width: 560px) {
  .rf-x-status-header { padding-inline: 8px; }
  .rf-x-status-post { padding-inline: 12px; }
  .rf-x-status-author { grid-template-columns: 44px minmax(0, 1fr) 40px; }
  .rf-x-status-author :deep(.rf-user-avatar) { width: 44px !important; height: 44px !important; flex-basis: 44px !important; }
  .rf-x-status-post h1 { font-size: 21px; }
  .rf-x-status-content { font-size: 17px; }
  .rf-x-status-stats { gap: 13px; font-size: 13px; }
}

@media (prefers-reduced-motion: reduce) {
  .rf-x-status-loading span, .rf-x-status-loading i, .rf-x-status-loading b { animation: none; }
}
</style>
