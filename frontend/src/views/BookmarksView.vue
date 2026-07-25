<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchBookmarkedPosts, type Post } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import AppIcon from '@/components/AppIcon.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import PostTagList from '@/components/PostTagList.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const posts = ref<Post[]>([])
const loading = ref(true)
async function load() { loading.value = true; try { posts.value = await fetchBookmarkedPosts() } catch (e) { Notify.danger(errorMessage(e, '收藏加载失败')) } finally { loading.value = false } }
onMounted(load)
</script>

<template>
  <PageContainer title="收藏">
    <div v-if="loading" class="rf-loading-block">正在加载收藏…</div>
    <nut-empty v-else-if="!posts.length" description="还没有收藏帖子" />
    <section v-else class="rf-timeline rf-bookmark-timeline"><RouterLink v-for="post in posts" :key="post.id" :to="`/posts/${post.id}`" class="rf-post-row"><UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" :frame="post.author_avatar_frame" /><div class="rf-post-body"><div class="rf-post-meta"><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" /><MembershipBadge :active="post.author_member" :tier-id="post.author_membership_tier_id" /><span>·</span><time>{{ new Date(post.created_at).toLocaleDateString('zh-CN') }}</time></div><PostTagList :tags="post.tags" compact /><h2 v-if="post.title">{{ post.title }}</h2><MarkdownContent :source="post.content" compact :class="{ 'rf-post-primary-copy': !post.title }" /><PostMediaGrid v-if="post.media?.length" :media="post.media" compact :preview="false" /><div class="rf-post-actions"><span><AppIcon name="message" size="17" />{{ post.comment_count }}</span><span><AppIcon name="repost" size="17" />{{ post.repost_count || 0 }}</span><span><AppIcon name="heart" size="17" />{{ post.like_count || 0 }}</span></div></div></RouterLink></section>
  </PageContainer>
</template>

<style scoped>
.rf-post-primary-copy { color: var(--rf-text); font-size: 16px; }
</style>
