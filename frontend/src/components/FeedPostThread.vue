<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createComment,
  errorMessage,
  togglePostLike,
  togglePostRepost,
  type Post,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import CommentComposer from '@/components/CommentComposer.vue'
import FeedPostRow from '@/components/FeedPostRow.vue'

const props = defineProps<{ post: Post }>()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const liking = ref(false)
const reposting = ref(false)
const commenting = ref(false)
const sending = ref(false)
const comment = ref('')
const commentFiles = ref<File[]>([])
const commentPreviewURLs = ref<string[]>([])
const composer = ref<{ focus: () => void } | null>(null)

function requireLogin() {
  return router.push({ path: '/login', query: { redirect: route.fullPath } })
}

async function likePost() {
  if (!auth.user) {
    await requireLogin()
    return
  }
  if (liking.value) return
  liking.value = true
  try {
    Object.assign(props.post, await togglePostLike(props.post.id))
  } catch (cause) {
    Notify.danger(errorMessage(cause, '点赞失败'))
  } finally {
    liking.value = false
  }
}

async function repostPost() {
  if (!auth.user) {
    await requireLogin()
    return
  }
  if (reposting.value) return
  reposting.value = true
  try {
    Object.assign(props.post, await togglePostRepost(props.post.id))
    Notify.success(props.post.reposted ? '已转发' : '已取消转发')
  } catch (cause) {
    Notify.danger(errorMessage(cause, '转发失败'))
  } finally {
    reposting.value = false
  }
}

async function toggleCommentComposer() {
  if (!auth.user) {
    await requireLogin()
    return
  }
  commenting.value = !commenting.value
  if (commenting.value) window.requestAnimationFrame(() => composer.value?.focus())
}

async function quotePost() {
  if (!auth.user) {
    await requireLogin()
    return
  }
  await router.push({ path: '/posts/new', query: { quote: String(props.post.id) } })
}

async function sharePost() {
  const url = new URL(`/posts/${props.post.id}`, window.location.origin).href
  try {
    if (navigator.share) {
      await navigator.share({ title: props.post.title || props.post.content.slice(0, 80), url })
      return
    }
    await navigator.clipboard.writeText(url)
    Notify.success('帖子链接已复制')
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    Notify.warn('分享失败，请打开帖子后复制地址')
  }
}

async function submitComment() {
  const content = comment.value.trim()
  if ((!content && !commentFiles.value.length) || sending.value) return
  sending.value = true
  try {
    await createComment(props.post.id, content, commentFiles.value)
    props.post.comment_count += 1
    comment.value = ''
    clearCommentFiles()
    commenting.value = false
    Notify.success('回复已发布')
  } catch (cause) {
    Notify.danger(errorMessage(cause, '回复发布失败'))
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
  const url = commentPreviewURLs.value[index]
  if (url) URL.revokeObjectURL(url)
  commentFiles.value.splice(index, 1)
  commentPreviewURLs.value.splice(index, 1)
}

function clearCommentFiles() {
  commentPreviewURLs.value.forEach((url) => URL.revokeObjectURL(url))
  commentFiles.value = []
  commentPreviewURLs.value = []
}

function openPostMedia(_: Post, index: number) {
  router.push({ path: `/posts/${props.post.id}`, query: { media: String(index) } })
}

onBeforeUnmount(clearCommentFiles)
</script>

<template>
  <div class="rf-feed-thread">
    <FeedPostRow
      :post="post"
      :liking="liking"
      :reposting="reposting"
      :commenting="commenting"
      @comment="toggleCommentComposer"
      @like="likePost"
      @open-media="openPostMedia"
      @quote="quotePost"
      @repost="repostPost"
      @share="sharePost"
    />
    <CommentComposer
      v-if="commenting"
      ref="composer"
      v-model="comment"
      compact
      :files="commentFiles"
      :preview-urls="commentPreviewURLs"
      :sending="sending"
      @submit="submitComment"
      @select-files="chooseCommentFiles"
      @remove-file="removeCommentFile"
    />
  </div>
</template>

<style scoped>
.rf-feed-thread { min-width: 0; }
.rf-feed-thread :deep(.rf-comment-composer) { background: var(--rf-bg-subtle); }
</style>
