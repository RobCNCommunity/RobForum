<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchPost, type Post } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import { markdownToPlainText } from '@/lib/markdown'

const emit = defineEmits<{ loaded: [content: string] }>()
const route = useRoute()
const quotedPost = ref<Post | null>(null)

function excerpt(item: Post) {
  const text = item.title.trim() || markdownToPlainText(item.content)
  return text.length > 120 ? `${text.slice(0, 120)}...` : text
}

function requestedQuoteID() {
  const value = Array.isArray(route.query.quote) ? route.query.quote[0] : route.query.quote
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return 0
  const id = Number(value)
  return Number.isSafeInteger(id) ? id : 0
}

onMounted(() => {
  const quoteID = requestedQuoteID()
  if (!quoteID) return
  fetchPost(quoteID).then((item) => {
    quotedPost.value = item
    emit('loaded', `引用 @${item.author_name} 的帖子：\n${excerpt(item)}\n${window.location.origin}/posts/${item.id}\n\n`)
  }).catch((cause: unknown) => {
    Notify.warn(errorMessage(cause, '引用的帖子无法加载'))
  })
})
</script>

<template>
  <RouterLink v-if="quotedPost" :to="`/posts/${quotedPost.id}`" class="rf-quote-preview">
    <span><AppIcon name="quote" size="16" />引用 @{{ quotedPost.author_name }}</span>
    <strong>{{ excerpt(quotedPost) }}</strong>
  </RouterLink>
</template>

<style scoped>
.rf-quote-preview { display: grid; gap: 6px; margin: 8px 0 12px; padding: 12px; border: 1px solid var(--rf-line); border-radius: var(--rf-radius); color: var(--rf-text); background: var(--rf-bg-subtle); }
.rf-quote-preview:hover { border-color: var(--primary); color: var(--rf-text); }
.rf-quote-preview span { display: inline-flex; align-items: center; gap: 6px; color: var(--primary); font-size: 12px; font-weight: 650; }
.rf-quote-preview strong { overflow: hidden; font-size: 14px; font-weight: 500; line-height: 1.45; text-overflow: ellipsis; white-space: nowrap; }
</style>
