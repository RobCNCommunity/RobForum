<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchNotifications, markNotificationsRead, type NotificationItem } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import { useNotificationsStore } from '@/stores/notifications'
import UserAvatar from '@/components/UserAvatar.vue'

const items = ref<NotificationItem[]>([])
const loading = ref(true)
const notifications = useNotificationsStore()

function message(item: NotificationItem) {
  return ({ follow: '关注了你', like: '赞了你的帖子', comment: '评论了你的帖子', comment_like: '赞了你的评论', repost: '转发了你的帖子', group_invite: '邀请你加入群聊', group_invite_reminder: '再次提醒你确认群聊邀请', group_joined: '通过邀请链接加入了群聊', group_left: '退出了群聊', group_removed: '将你移出了群聊', group_dissolved: '解散了群聊', report_accepted: '已处理你的举报，确认内容违规', report_rejected: '已处理你的举报，未发现违规', content_report_accepted: '已处理被举报内容，相关内容已隐藏' } as Record<string, string>)[item.kind] || '与你有新的互动'
}
function target(item: NotificationItem) {
  if (item.kind === 'report_accepted' || item.kind === 'report_rejected' || item.kind === 'content_report_accepted') return '/notifications'
  if (item.conversation_id && !['group_removed', 'group_dissolved'].includes(item.kind)) return { path: '/messages', query: { conversation: String(item.conversation_id) } }
  if (item.post_id) return `/posts/${item.post_id}`
  return `/users/${item.actor_id}`
}
function date(value: string) { return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
async function load() {
  loading.value = true
  try { items.value = await fetchNotifications(); await markNotificationsRead(); notifications.markRead() } catch (e) { Notify.danger(errorMessage(e, '通知加载失败')) } finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <PageContainer title="通知">
    <div v-if="loading" class="rf-loading-block">正在加载通知…</div>
    <nut-empty v-else-if="!items.length" description="暂时没有新通知" />
    <section v-else class="rf-notification-list">
      <RouterLink v-for="item in items" :key="item.id" :to="target(item)" class="rf-notification-row" :class="{ unread: !item.read }">
        <UserAvatar :src="item.actor_avatar" :name="item.actor_name" :size="42" />
        <div><p><strong>{{ item.actor_name }}</strong> {{ message(item) }}</p><small v-if="item.post_title || item.conversation_name">{{ item.post_title || item.conversation_name }}</small><time>{{ date(item.created_at) }}</time></div>
        <AppIcon name="arrow" size="17" />
      </RouterLink>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-notification-list { border-top: 1px solid var(--rf-line); }.rf-notification-row { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); }.rf-notification-row:hover, .rf-notification-row.unread { background: color-mix(in srgb, var(--primary) 5%, var(--rf-bg)); }.rf-notification-row > div { min-width: 0; flex: 1; }.rf-notification-row p { margin: 0; line-height: 1.45; }.rf-notification-row p strong { color: var(--rf-text); }.rf-notification-row small, .rf-notification-row time { display: block; overflow: hidden; color: var(--rf-muted); text-overflow: ellipsis; white-space: nowrap; }.rf-notification-row time { margin-top: 3px; color: var(--rf-faint); font-size: 12px; }
</style>
