<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchBookmarkedPosts, type Post } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import AppIcon from '@/components/AppIcon.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const posts = ref<Post[]>([])
const loading = ref(true)
function excerpt(value: string) { const text = (value || '').replace(/\s+/g, ' ').trim(); return text.length > 150 ? `${text.slice(0, 150)}…` : text }
async function load() { loading.value = true; try { posts.value = await fetchBookmarkedPosts() } catch (e) { Notify.danger(errorMessage(e, '收藏加载失败')) } finally { loading.value = false } }
onMounted(load)
</script>

<template>
  <PageContainer title="收藏">
    <div v-if="loading" class="rf-loading-block">正在加载收藏…</div>
    <nut-empty v-else-if="!posts.length" description="还没有收藏帖子" />
    <section v-else class="rf-timeline rf-bookmark-timeline"><RouterLink v-for="post in posts" :key="post.id" :to="`/posts/${post.id}`" class="rf-post-row"><UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" /><div class="rf-post-body"><div class="rf-post-meta"><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" /><span>·</span><time>{{ new Date(post.created_at).toLocaleDateString('zh-CN') }}</time></div><h2>{{ post.title }}</h2><p>{{ excerpt(post.content) }}</p><PostMediaGrid v-if="post.media?.length" :media="post.media" compact :preview="false" /><div class="rf-post-actions"><span><AppIcon name="message" size="17" />{{ post.comment_count }}</span><span><AppIcon name="repost" size="17" />{{ post.repost_count || 0 }}</span><span><AppIcon name="heart" size="17" />{{ post.like_count || 0 }}</span></div></div></RouterLink></section>
  </PageContainer>
</template>
