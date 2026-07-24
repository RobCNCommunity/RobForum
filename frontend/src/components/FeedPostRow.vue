<script setup lang="ts">
import type { Post } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import PostTagList from '@/components/PostTagList.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'

withDefaults(defineProps<{
  post: Post
  liking?: boolean
  reposting?: boolean
  commenting?: boolean
}>(), {
  liking: false,
  reposting: false,
  commenting: false,
})

const emit = defineEmits<{
  comment: [post: Post]
  like: [post: Post]
  openMedia: [post: Post, index: number]
  quote: [post: Post]
  repost: [post: Post]
  share: [post: Post]
}>()

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(date)
}

</script>

<template>
  <article class="rf-feed-post">
    <RouterLink :to="`/users/${post.author_id}`" :aria-label="`查看 ${post.author_name} 的个人主页`" class="rf-feed-post-avatar">
      <UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" />
    </RouterLink>
    <div class="rf-feed-post-body">
      <div class="rf-feed-post-meta">
        <RouterLink :to="`/users/${post.author_id}`"><strong>{{ post.author_name }}</strong></RouterLink>
        <VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" />
        <MembershipBadge :active="post.author_member" :tier-id="post.author_membership_tier_id" />
        <span>@{{ post.author_id }}</span><span>·</span><time>{{ formatDate(post.created_at) }}</time>
      </div>
      <div class="rf-feed-post-context"><span v-if="post.featured">精华</span><em>{{ post.board_name }}</em></div>
      <PostTagList :tags="post.tags" compact class="rf-feed-post-tags" />
      <RouterLink :to="`/posts/${post.id}`" class="rf-feed-post-copy">
        <h2 v-if="post.title">{{ post.title }}</h2>
        <MarkdownContent v-if="post.content.trim()" :source="post.content" compact :class="{ primary: !post.title }" />
      </RouterLink>
      <PostMediaGrid v-if="post.media?.length" :media="post.media" compact custom-preview @open-preview="emit('openMedia', post, $event)" />
      <div class="rf-feed-post-actions" aria-label="帖子互动操作">
        <button type="button" class="comment" :class="{ active: commenting }" :aria-expanded="commenting" :aria-label="`评论帖子，当前 ${post.comment_count} 条`" @click="emit('comment', post)"><AppIcon name="message" size="17" /><span>{{ post.comment_count }}</span></button>
        <button type="button" class="repost" :class="{ reposted: post.reposted }" :disabled="reposting" :aria-pressed="post.reposted" :aria-label="post.reposted ? '取消转发' : '转发帖子'" @click="emit('repost', post)"><AppIcon name="repost" size="17" /><span>{{ post.repost_count || 0 }}</span></button>
        <button type="button" class="quote" aria-label="引用帖子" @click="emit('quote', post)"><AppIcon name="quote" size="17" /></button>
        <button type="button" class="like" :class="{ liked: post.liked }" :disabled="liking" :aria-pressed="post.liked" :aria-label="post.liked ? '取消点赞' : '点赞帖子'" @click="emit('like', post)"><AppIcon name="heart" size="17" /><span>{{ post.like_count || 0 }}</span></button>
        <button type="button" class="share" aria-label="分享帖子" @click="emit('share', post)"><AppIcon name="share" size="17" /></button>
      </div>
    </div>
  </article>
</template>

<style scoped>
.rf-feed-post { display: grid; min-width: 0; grid-template-columns: 42px minmax(0, 1fr); gap: 12px; padding: 15px 16px 14px; border-bottom: 1px solid var(--rf-line); color: var(--rf-text); transition: background-color 150ms ease-out; }
.rf-feed-post:hover { background: var(--rf-bg-hover); }
.rf-feed-post-avatar { align-self: start; }
.rf-feed-post-body { min-width: 0; }
.rf-feed-post-meta { display: flex; min-width: 0; align-items: center; gap: 5px; color: var(--rf-muted); font-size: 13px; }
.rf-feed-post-meta > a { min-width: 0; }
.rf-feed-post-meta strong { display: block; overflow: hidden; color: var(--rf-text); font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.rf-feed-post-meta > span, .rf-feed-post-meta time { flex: 0 0 auto; white-space: nowrap; }
.rf-feed-post-context { display: flex; align-items: center; gap: 6px; margin: 4px 0; color: var(--rf-muted); font-size: 12px; }
.rf-feed-post-context > span { color: var(--rf-danger); font-weight: 650; }
.rf-feed-post-context em { padding: 1px 6px; border-radius: var(--rf-pill); background: var(--rf-bg-subtle); font-style: normal; }
.rf-feed-post-tags { margin: 6px 0 4px; }
.rf-feed-post-copy { display: block; color: inherit; }
.rf-feed-post-copy:hover { color: inherit; }
.rf-feed-post-copy h2 { margin: 3px 0 4px; font-family: var(--rf-font-display); font-size: 17px; line-height: 1.35; overflow-wrap: anywhere; }
.rf-feed-post-copy :deep(.rf-markdown) { margin: 0 0 9px; color: var(--rf-muted); text-wrap: pretty; }
.rf-feed-post-copy :deep(.rf-markdown.primary) { color: var(--rf-text); font-size: 16px; text-wrap: balance; }
.rf-feed-post-actions { display: grid; max-width: 460px; min-height: var(--rf-touch); grid-template-columns: repeat(5, minmax(var(--rf-touch), 1fr)); align-items: center; color: var(--rf-muted); font-size: 12px; }
.rf-feed-post-actions > button { display: inline-flex; width: max-content; min-width: var(--rf-touch); min-height: var(--rf-touch); align-items: center; gap: 5px; padding: 0 7px; border-radius: var(--rf-pill); color: inherit; background: transparent; }
.rf-feed-post-actions > button.comment:hover, .rf-feed-post-actions > button.comment.active, .rf-feed-post-actions > button.quote:hover, .rf-feed-post-actions > button.share:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 9%, transparent); }
.rf-feed-post-actions > button.like:hover, .rf-feed-post-actions > button.like.liked { color: #f91880; background: rgba(249, 24, 128, .08); }
.rf-feed-post-actions > button.repost:hover, .rf-feed-post-actions > button.repost.reposted { color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 9%, transparent); }
.rf-feed-post-actions > button:disabled { cursor: wait; opacity: .55; }
@media (max-width: 560px) {
  .rf-feed-post { grid-template-columns: 38px minmax(0, 1fr); gap: 9px; padding: 13px 12px; }
  .rf-feed-post-avatar :deep(.rf-user-avatar) { width: 38px !important; height: 38px !important; flex-basis: 38px !important; }
  .rf-feed-post-meta time { display: none; }
  .rf-feed-post-copy h2 { font-size: 16px; }
}
</style>
