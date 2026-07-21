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
  return ({ follow: '关注了你', like: '赞了你的帖子', comment: '评论了你的帖子', comment_like: '赞了你的评论', repost: '转发了你的帖子' } as Record<string, string>)[item.kind] || '与你有新的互动'
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
      <RouterLink v-for="item in items" :key="item.id" :to="item.post_id ? `/posts/${item.post_id}` : `/users/${item.actor_id}`" class="rf-notification-row" :class="{ unread: !item.read }">
        <UserAvatar :src="item.actor_avatar" :name="item.actor_name" :size="42" />
        <div><p><strong>{{ item.actor_name }}</strong> {{ message(item) }}</p><small v-if="item.post_title">{{ item.post_title }}</small><time>{{ date(item.created_at) }}</time></div>
        <AppIcon name="arrow" size="17" />
      </RouterLink>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-notification-list { border-top: 1px solid var(--rf-line); }.rf-notification-row { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); }.rf-notification-row:hover, .rf-notification-row.unread { background: color-mix(in srgb, var(--primary) 5%, var(--rf-bg)); }.rf-notification-row > div { min-width: 0; flex: 1; }.rf-notification-row p { margin: 0; line-height: 1.45; }.rf-notification-row p strong { color: var(--rf-text); }.rf-notification-row small, .rf-notification-row time { display: block; overflow: hidden; color: var(--rf-muted); text-overflow: ellipsis; white-space: nowrap; }.rf-notification-row time { margin-top: 3px; color: var(--rf-faint); font-size: 12px; }
</style>
