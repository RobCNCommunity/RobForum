<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Comment } from '@/api'
import type { CommentNode } from '@/commentTree'
import AppIcon from '@/components/AppIcon.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'

const props = withDefaults(defineProps<{
  node: CommentNode
  depth?: number
  viewerId?: number
  viewerIsAdmin?: boolean
  busyId?: number
  replyingToId?: number
  compact?: boolean
}>(), {
  depth: 0,
  viewerId: 0,
  viewerIsAdmin: false,
  busyId: 0,
  replyingToId: 0,
  compact: false,
})

const emit = defineEmits<{
  reply: [item: Comment]
  like: [item: Comment]
  delete: [item: Comment]
  report: [item: Comment]
  share: [item: Comment]
}>()

const menuOpen = ref(false)
const item = computed(() => props.node.item)
const canDelete = computed(() => props.viewerId > 0 && (props.viewerIsAdmin || props.viewerId === item.value.author_id))
const canReport = computed(() => props.viewerId === 0 || props.viewerId !== item.value.author_id)

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const options: Intl.DateTimeFormatOptions = date.getFullYear() === now.getFullYear()
    ? { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }
    : { year: 'numeric', month: 'numeric', day: 'numeric' }
  return new Intl.DateTimeFormat('zh-CN', options).format(date)
}

function formatCount(value: number) {
  return new Intl.NumberFormat('zh-CN', { notation: value >= 10000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(value || 0)
}

function runMenuAction(action: 'delete' | 'report') {
  menuOpen.value = false
  emit(action, item.value)
}
</script>

<template>
  <div class="rf-comment-node" :class="{ compact, deep: depth >= 3, replying: replyingToId === item.id }">
    <article :id="`reply-${item.id}`" class="rf-comment-row">
      <RouterLink :to="`/users/${item.author_id}`" :aria-label="`查看 ${item.author_name} 的个人主页`">
        <UserAvatar :src="item.author_avatar" :name="item.author_name" :size="compact ? 34 : 40" />
      </RouterLink>
      <div class="rf-comment-body">
        <header>
          <RouterLink :to="`/users/${item.author_id}`"><strong>{{ item.author_name }}</strong></RouterLink>
          <VerifiedBadge :verified="item.author_verified" :label="item.author_verification_label" />
          <MembershipBadge :active="item.author_member" :tier-id="item.author_membership_tier_id" />
          <span>@user_{{ item.author_id }}</span><span>·</span><time>{{ formatDate(item.created_at) }}</time>
          <div v-if="canDelete || canReport" class="rf-comment-menu">
            <button type="button" aria-label="更多回复操作" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen"><AppIcon name="more" size="17" /></button>
            <Transition name="rf-comment-menu">
              <div v-if="menuOpen">
                <button v-if="canReport" type="button" class="danger" @click="runMenuAction('report')">举报回复</button>
                <button v-if="canDelete" type="button" class="danger" :disabled="busyId === item.id" @click="runMenuAction('delete')">删除回复</button>
              </div>
            </Transition>
          </div>
        </header>
        <p v-if="item.content">{{ item.content }}</p>
        <PostMediaGrid v-if="item.media?.length" :media="item.media" compact />
        <footer>
          <button v-if="item.parent_id == null" type="button" aria-label="回复这条评论" :class="{ active: replyingToId === item.id }" @click="emit('reply', item)"><AppIcon name="message" size="16" /><span>回复</span></button>
          <button type="button" aria-label="点赞回复" :class="{ liked: item.liked }" :disabled="busyId === item.id" @click="emit('like', item)"><AppIcon name="heart" size="16" /><span>{{ formatCount(item.like_count) }}</span></button>
          <button type="button" aria-label="复制回复链接" @click="emit('share', item)"><AppIcon name="share" size="16" /></button>
        </footer>
      </div>
    </article>
    <div v-if="node.children.length" class="rf-comment-children">
      <CommentThreadItem
        v-for="child in node.children"
        :key="child.item.id"
        :node="child"
        :depth="depth + 1"
        :viewer-id="viewerId"
        :viewer-is-admin="viewerIsAdmin"
        :busy-id="busyId"
        :replying-to-id="replyingToId"
        :compact="compact"
        @reply="emit('reply', $event)"
        @like="emit('like', $event)"
        @delete="emit('delete', $event)"
        @report="emit('report', $event)"
        @share="emit('share', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.rf-comment-node { min-width: 0; border-bottom: 1px solid var(--rf-line); }
.rf-comment-node.replying > .rf-comment-row { background: color-mix(in srgb, var(--primary) 6%, var(--rf-bg)); }
.rf-comment-row { display: grid; min-width: 0; grid-template-columns: 40px minmax(0, 1fr); gap: 10px; padding: 12px 16px 10px; transition: background-color 150ms ease-out; }
.rf-comment-row:hover { background: var(--rf-bg-hover); }
.rf-comment-body { min-width: 0; }
.rf-comment-body > header { position: relative; display: flex; min-width: 0; min-height: 22px; align-items: center; gap: 4px; color: var(--rf-muted); font-size: 12px; }
.rf-comment-body > header > a { min-width: 0; }
.rf-comment-body > header strong { display: block; overflow: hidden; color: var(--rf-text); text-overflow: ellipsis; white-space: nowrap; }
.rf-comment-body > header > span, .rf-comment-body > header time { flex: 0 0 auto; white-space: nowrap; }
.rf-comment-body > p { margin: 3px 0 8px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.rf-comment-body :deep(.rf-post-media-grid) { max-width: 480px; margin: 6px 0 8px; }
.rf-comment-body > footer { display: flex; max-width: 245px; align-items: center; justify-content: space-between; color: var(--rf-muted); }
.rf-comment-body > footer button { display: inline-flex; min-width: 36px; min-height: 32px; align-items: center; gap: 5px; padding: 0 7px; border-radius: var(--rf-pill); color: inherit; background: transparent; font-size: 12px; }
.rf-comment-body > footer button:hover, .rf-comment-body > footer button.active { color: var(--primary); background: color-mix(in srgb, var(--primary) 9%, transparent); }
.rf-comment-body > footer button.liked { color: #f91880; background: rgba(249, 24, 128, .08); }
.rf-comment-menu { position: relative; margin-left: auto; }
.rf-comment-menu > button { display: inline-grid; width: 30px; height: 30px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }
.rf-comment-menu > button:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-comment-menu > div { position: absolute; z-index: 20; top: 32px; right: 0; width: max-content; min-width: 136px; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg); box-shadow: 0 8px 24px rgba(15, 20, 25, .15); }
.rf-comment-menu > div button { display: block; width: 100%; min-height: 42px; padding: 0 14px; color: var(--rf-danger); background: transparent; font-weight: 650; text-align: left; }
.rf-comment-menu > div button:hover { background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.rf-comment-children { margin-left: 34px; border-left: 2px solid var(--rf-line); }
.rf-comment-node.deep > .rf-comment-children { margin-left: 0; border-left-width: 1px; }
.rf-comment-node.compact > .rf-comment-row { grid-template-columns: 34px minmax(0, 1fr); padding: 11px 14px; }
.rf-comment-node.compact .rf-comment-children { margin-left: 20px; }
.rf-comment-node.compact :deep(.rf-post-media-grid) { max-width: 270px; }
.rf-comment-menu-enter-active { transition: opacity 140ms ease-out, transform 170ms cubic-bezier(.22, 1, .36, 1); }
.rf-comment-menu-leave-active { transition: opacity 90ms ease-in; }
.rf-comment-menu-enter-from, .rf-comment-menu-leave-to { opacity: 0; transform: translateY(-4px); }
@media (max-width: 560px) {
  .rf-comment-row { grid-template-columns: 36px minmax(0, 1fr); gap: 8px; padding-inline: 12px; }
  .rf-comment-row :deep(.rf-user-avatar) { width: 36px !important; height: 36px !important; flex-basis: 36px !important; }
  .rf-comment-body > header > span { display: none; }
  .rf-comment-children { margin-left: 18px; }
}
</style>
