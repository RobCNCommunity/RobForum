<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createConversation,
  errorMessage,
  fetchBlockStatus,
  fetchFollowStatus,
  fetchUserProfile,
  reportUserProfile,
  setUserBlocked,
  setUserFollowing,
  type FollowStatus,
  type PublicUser,
  type UserProfile,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import ContentReportDialog from '@/components/ContentReportDialog.vue'
import PageContainer from '@/components/PageContainer.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import PostTagList from '@/components/PostTagList.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import BadgeGrid from '@/components/BadgeGrid.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const profile = ref<UserProfile | null>(null)
const follow = ref<FollowStatus>({ following: false, follower_count: 0, following_count: 0 })
const loading = ref(true)
const blocked = ref(false)
const acting = ref(false)
const userMenuOpen = ref(false)
const profileTab = ref<'posts' | 'resources'>('posts')
const reportOpen = ref(false)
const reporting = ref(false)

const isMe = computed(() => !!profile.value && auth.user?.id === profile.value.user.id)
const postCountLabel = computed(() => `${profile.value?.posts.length || 0} 条动态`)
const profileHandle = computed(() => profile.value ? userHandle(profile.value.user) : '')

function userHandle(user: Pick<PublicUser, 'id' | 'roblox_name'>) {
  const candidate = (user.roblox_name || '').trim().replace(/\s+/g, '_')
  return `@${candidate || `user_${user.id}`}`
}

function formatJoinDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long' }).format(date)}加入`
}

function formatTimelineDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const sameYear = date.getFullYear() === now.getFullYear()
  return new Intl.DateTimeFormat('zh-CN', sameYear ? { month: 'numeric', day: 'numeric' } : { year: 'numeric', month: 'numeric', day: 'numeric' }).format(date)
}

const formatMoney = (value: number) => value > 0 ? `¥${(value / 100).toFixed(2)}` : '免费'

function resourceExcerpt(value: string) {
  const text = (value || '').replace(/\s+/g, ' ').trim()
  return text.length > 180 ? `${text.slice(0, 180)}…` : text
}

async function load() {
  loading.value = true
  userMenuOpen.value = false
  try {
    const id = Number(route.params.userID)
    if (!Number.isInteger(id) || id <= 0) throw new Error('invalid user id')
    profile.value = await fetchUserProfile(id)
    follow.value = {
      following: profile.value.following,
      follower_count: profile.value.follower_count,
      following_count: profile.value.following_count,
    }
    if (auth.user && auth.user.id !== id) {
      const [block, social] = await Promise.all([fetchBlockStatus(id), fetchFollowStatus(id)])
      blocked.value = block.blocked
      follow.value = social
    } else {
      blocked.value = false
    }
  } catch (error) {
    Notify.danger(errorMessage(error, '个人主页加载失败'))
    router.push('/')
  } finally {
    loading.value = false
  }
}

async function startMessage() {
  if (!profile.value) return
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  acting.value = true
  try {
    const conversation = await createConversation({ kind: 'direct', member_ids: [profile.value.user.id] })
    router.push({ path: '/messages', query: { conversation: String(conversation.id) } })
  } catch (error) {
    Notify.danger(errorMessage(error, '私信创建失败'))
  } finally {
    acting.value = false
  }
}

async function toggleFollow() {
  if (!profile.value) return
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  acting.value = true
  try {
    follow.value = await setUserFollowing(profile.value.user.id, !follow.value.following)
    Notify.success(follow.value.following ? '已关注' : '已取消关注')
  } catch (error) {
    Notify.danger(errorMessage(error, '关注操作失败'))
  } finally {
    acting.value = false
  }
}

async function toggleBlock() {
  if (!profile.value || !auth.user) return
  acting.value = true
  userMenuOpen.value = false
  try {
    const wasFollowing = follow.value.following
    blocked.value = (await setUserBlocked(profile.value.user.id, !blocked.value)).blocked
    if (blocked.value && wasFollowing) {
      follow.value = {
        ...follow.value,
        following: false,
        follower_count: Math.max(0, follow.value.follower_count - 1),
      }
    }
    Notify.success(blocked.value ? '已拉黑该用户' : '已解除拉黑')
  } catch (error) {
    Notify.danger(errorMessage(error, '拉黑操作失败'))
  } finally {
    acting.value = false
  }
}

function openProfileReport() {
  if (!profile.value || isMe.value) return
  userMenuOpen.value = false
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  reportOpen.value = true
}

async function submitProfileReport(reason: string) {
  if (!profile.value || reporting.value) return
  reporting.value = true
  try {
    await reportUserProfile(profile.value.user.id, reason)
    reportOpen.value = false
    Notify.success('举报已提交，审核结果会通知你')
  } catch (error) {
    Notify.danger(errorMessage(error, '举报提交失败'))
  } finally {
    reporting.value = false
  }
}

watch(() => route.params.userID, () => {
  profileTab.value = 'posts'
  load()
})
onMounted(load)
</script>

<template>
  <PageContainer>
    <div v-if="loading" class="rf-x-profile-loading" aria-label="正在加载个人主页">
      <div class="rf-x-profile-loading-header"><span /><div><i /><i /></div></div>
      <div class="rf-x-profile-loading-cover" />
      <div class="rf-x-profile-loading-body"><span /><i /><i /><i /></div>
    </div>

    <div v-else-if="profile" class="rf-x-profile">
      <header class="rf-x-profile-header">
        <button type="button" class="rf-x-round-button" aria-label="返回" @click="router.back()">
          <AppIcon name="back" size="21" />
        </button>
        <div>
          <strong>{{ profile.user.display_name }}</strong>
          <span>{{ postCountLabel }}</span>
        </div>
      </header>

      <section class="rf-x-profile-summary">
        <div class="rf-x-profile-cover" aria-label="个人主页封面">
          <img v-if="profile.user.cover_url" :src="profile.user.cover_url" alt="" />
        </div>
        <div class="rf-x-profile-action-row">
          <UserAvatar class="rf-x-profile-avatar" :src="profile.user.avatar_url" :name="profile.user.display_name" :size="104" />
          <div class="rf-x-profile-actions">
            <RouterLink v-if="isMe" to="/settings/profile" class="rf-x-outline-button">编辑个人资料</RouterLink>
            <template v-else>
              <button type="button" class="rf-x-round-button rf-x-outline-control" aria-label="发送私信" title="发送私信" :disabled="acting || blocked" @click="startMessage">
                <AppIcon name="message" size="20" />
              </button>
              <div class="rf-x-user-menu">
                <button type="button" class="rf-x-round-button rf-x-outline-control" aria-label="更多用户操作" :aria-expanded="userMenuOpen" @click="userMenuOpen = !userMenuOpen">
                  <AppIcon name="more" size="20" />
                </button>
                <Transition name="rf-x-menu">
                  <div v-if="userMenuOpen" class="rf-x-user-menu-panel">
                    <button type="button" :disabled="acting" @click="toggleBlock">{{ blocked ? '解除拉黑' : '拉黑此用户' }}</button>
                    <button type="button" class="danger" :disabled="reporting" @click="openProfileReport">举报此用户</button>
                  </div>
                </Transition>
              </div>
              <button type="button" class="rf-x-follow-button" :class="{ following: follow.following }" :disabled="acting || blocked" @click="toggleFollow">
                {{ follow.following ? '正在关注' : '关注' }}
              </button>
            </template>
          </div>
        </div>

        <div class="rf-x-profile-copy">
          <div class="rf-x-profile-name">
            <h1>{{ profile.user.display_name }}</h1>
            <VerifiedBadge :verified="profile.user.blue_verified" :label="profile.user.verification_label" /><MembershipBadge :active="profile.user.member_active" :tier-id="profile.user.membership_tier_id" />
          </div>
          <span class="rf-x-profile-handle">{{ profileHandle }}</span>
          <div class="rf-x-profile-level"><span>LV.{{ profile.user.progress.level }}</span><strong>{{ profile.user.progress.level_name }}</strong><small>{{ profile.user.progress.experience }} 经验</small></div>
          <p v-if="profile.user.bio" class="rf-x-profile-bio">{{ profile.user.bio }}</p>
          <div class="rf-x-profile-details">
            <span v-if="formatJoinDate(profile.user.created_at)"><AppIcon name="calendar" size="18" />{{ formatJoinDate(profile.user.created_at) }}</span>
          </div>
          <div class="rf-x-profile-stats">
            <span><strong>{{ follow.following_count }}</strong> 正在关注</span>
            <span><strong>{{ follow.follower_count }}</strong> 关注者</span>
          </div>
          <div v-if="profile.user.badges.length" class="rf-x-profile-badges"><span>成就徽章</span><BadgeGrid :badges="profile.user.badges" compact /></div>
        </div>

        <div v-if="blocked" class="rf-x-blocked-note">你已拉黑该用户，关注和私信功能已关闭。</div>
      </section>

      <nav class="rf-x-profile-tabs" aria-label="个人主页内容">
        <button type="button" :class="{ active: profileTab === 'posts' }" @click="profileTab = 'posts'"><span>动态</span></button>
        <button type="button" :class="{ active: profileTab === 'resources' }" @click="profileTab = 'resources'"><span>资源</span></button>
      </nav>

      <Transition name="rf-profile-tab" mode="out-in">
        <section :key="profileTab" class="rf-x-profile-content">
          <template v-if="profileTab === 'posts'">
            <div v-if="profile.posts.length" class="rf-x-timeline">
              <RouterLink v-for="post in profile.posts" :key="post.id" :to="`/posts/${post.id}`" class="rf-x-post-row">
                <UserAvatar :src="profile.user.avatar_url" :name="profile.user.display_name" :size="42" />
                <div class="rf-x-post-body">
                  <div class="rf-x-post-meta">
                    <strong>{{ profile.user.display_name }}</strong>
                    <VerifiedBadge :verified="profile.user.blue_verified" :label="profile.user.verification_label" /><MembershipBadge :active="profile.user.member_active" :tier-id="profile.user.membership_tier_id" />
                    <span class="rf-x-post-handle">{{ profileHandle }}</span><span>·</span><time>{{ formatTimelineDate(post.created_at) }}</time>
                    <span class="rf-x-post-more"><AppIcon name="more" size="18" /></span>
                  </div>
                  <div class="rf-x-post-context">{{ post.board_name }}</div>
                  <PostTagList :tags="post.tags" compact class="rf-x-profile-post-tags" />
                  <h2 v-if="post.title">{{ post.title }}</h2>
                  <MarkdownContent v-if="post.content" :source="post.content" compact :class="{ primary: !post.title }" />
                  <PostMediaGrid v-if="post.media?.length" :media="post.media" compact :preview="false" />
                  <div class="rf-x-post-actions" aria-label="帖子互动数据">
                    <span><AppIcon name="message" size="18" />{{ post.comment_count || 0 }}</span>
                    <span :class="{ active: post.reposted }"><AppIcon name="repost" size="18" />{{ post.repost_count || 0 }}</span>
                    <span :class="{ active: post.liked }"><AppIcon name="heart" size="18" />{{ post.like_count || 0 }}</span>
                    <span><AppIcon name="eye" size="18" />{{ post.views || 0 }}</span>
                    <span aria-label="分享"><AppIcon name="share" size="18" /></span>
                  </div>
                </div>
              </RouterLink>
            </div>
            <div v-else class="rf-empty"><nut-empty description="还没有公开动态" /></div>
          </template>

          <template v-else>
            <div v-if="profile.resources.length" class="rf-x-resource-list">
              <article v-for="item in profile.resources" :key="item.id" class="rf-x-resource-row">
                <UserAvatar :src="profile.user.avatar_url" :name="profile.user.display_name" :size="42" />
                <div>
                  <header>
                    <span><strong>{{ profile.user.display_name }}</strong><VerifiedBadge :verified="profile.user.blue_verified" :label="profile.user.verification_label" /><MembershipBadge :active="profile.user.member_active" :tier-id="profile.user.membership_tier_id" /><small>{{ profileHandle }}</small></span>
                    <b>{{ formatMoney(item.price_cents) }}</b>
                  </header>
                  <div class="rf-x-post-context">{{ item.resource_type || '资源' }} · {{ item.game || 'Roblox' }}</div>
                  <h2>{{ item.title }}</h2>
                  <p v-if="resourceExcerpt(item.description)">{{ resourceExcerpt(item.description) }}</p>
                  <footer>
                    <span v-if="item.version">版本 {{ item.version }}</span>
                    <span>{{ item.download_count }} 次下载</span>
                    <span v-if="item.sales_count">{{ item.sales_count }} 次购买</span>
                  </footer>
                </div>
              </article>
            </div>
            <div v-else class="rf-empty"><nut-empty description="还没有公开资源" /></div>
          </template>
        </section>
      </Transition>
    </div>
    <ContentReportDialog :open="reportOpen" :target-label="profile ? `用户：${profile.user.display_name}` : ''" :submitting="reporting" @close="reportOpen = false" @submit="submitProfileReport" />
  </PageContainer>
</template>

<style scoped>
.rf-x-profile { min-width: 0; min-height: 100vh; background: var(--rf-bg); }
.rf-x-profile-header { position: sticky; top: 0; z-index: 12; display: grid; min-height: 56px; grid-template-columns: 44px minmax(0, 1fr); align-items: center; gap: 12px; padding: 4px 16px; border-bottom: 1px solid color-mix(in srgb, var(--rf-line) 82%, transparent); background: color-mix(in srgb, var(--rf-bg) 88%, transparent); backdrop-filter: blur(12px); }
.rf-x-profile-header > div { display: flex; min-width: 0; flex-direction: column; }
.rf-x-profile-header strong { overflow: hidden; font-size: 20px; line-height: 1.15; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-profile-header span { color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-x-round-button { display: inline-grid; width: 44px; height: 44px; flex: 0 0 44px; place-items: center; border-radius: 50%; color: var(--rf-text); background: transparent; transition: background-color 150ms ease-out, transform 100ms ease-out; touch-action: manipulation; }
.rf-x-round-button:hover { background: var(--rf-bg-hover); }
.rf-x-round-button:active { transform: scale(.94); }
.rf-x-round-button:disabled { cursor: not-allowed; opacity: .45; }
.rf-x-profile-cover { width: 100%; aspect-ratio: 3 / 1; overflow: hidden; background: #cfd9de; }
.rf-x-profile-cover img { display: block; width: 100%; height: 100%; object-fit: cover; }
.rf-x-profile-action-row { position: relative; display: flex; min-height: 64px; align-items: flex-start; justify-content: flex-end; padding: 12px 16px 0; }
.rf-x-profile-avatar { position: absolute; bottom: 10px; left: 16px; width: 104px !important; height: 104px !important; flex-basis: 104px !important; border: 4px solid var(--rf-bg); background: var(--rf-bg-subtle); box-shadow: none !important; }
.rf-x-profile-actions { display: flex; min-height: 44px; align-items: center; justify-content: flex-end; gap: 8px; }
.rf-x-outline-button, .rf-x-follow-button { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; padding: 0 17px; border: 1px solid var(--rf-faint); border-radius: var(--rf-pill); color: var(--rf-text); background: var(--rf-bg); font-size: 14px; font-weight: 700; transition: background-color 150ms ease-out, border-color 150ms ease-out, transform 100ms ease-out; }
.rf-x-outline-button:hover, .rf-x-follow-button.following:hover { color: var(--rf-text); background: var(--rf-bg-hover); }
.rf-x-outline-button:active, .rf-x-follow-button:active { transform: scale(.97); }
.rf-x-follow-button { min-width: 82px; border-color: var(--rf-text); color: var(--rf-bg); background: var(--rf-text); }
.rf-x-follow-button:hover { background: color-mix(in srgb, var(--rf-text) 88%, transparent); }
.rf-x-follow-button.following { color: var(--rf-text); background: var(--rf-bg); }
.rf-x-follow-button:disabled { cursor: not-allowed; opacity: .45; }
.rf-x-outline-control { width: 36px; height: 36px; flex-basis: 36px; border: 1px solid var(--rf-faint); }
.rf-x-user-menu { position: relative; }
.rf-x-user-menu-panel { position: absolute; z-index: 20; top: 42px; right: 0; width: max-content; min-width: 154px; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 12px; background: var(--rf-bg); box-shadow: 0 8px 30px rgba(15, 20, 25, .16); }
.rf-x-user-menu-panel button { width: 100%; min-height: 44px; padding: 0 16px; color: var(--rf-text); background: transparent; font-weight: 700; text-align: left; }
.rf-x-user-menu-panel button:hover { background: var(--rf-bg-hover); }
.rf-x-user-menu-panel button.danger { color: var(--rf-danger); }
.rf-x-user-menu-panel button.danger:hover { background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.rf-x-menu-enter-active { transition: opacity 160ms ease-out, transform 180ms cubic-bezier(.22, 1, .36, 1); }
.rf-x-menu-leave-active { transition: opacity 100ms ease-in, transform 100ms ease-in; }
.rf-x-menu-enter-from, .rf-x-menu-leave-to { opacity: 0; transform: translateY(-5px) scale(.97); }
.rf-x-profile-copy { padding: 0 16px 16px; }
.rf-x-profile-name { display: flex; min-width: 0; align-items: center; gap: 5px; }
.rf-x-profile-name h1 { min-width: 0; margin: 0; overflow: hidden; font-size: 21px; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-profile-handle { display: block; color: var(--rf-muted); font-size: 14px; }
.rf-x-profile-level { display: flex; align-items: center; gap: 6px; margin-top: 10px; }
.rf-x-profile-level span { padding: 2px 7px; border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, var(--rf-bg)); font-size: 11px; font-weight: 800; }
.rf-x-profile-level strong { font-size: 13px; }
.rf-x-profile-level small { color: var(--rf-muted); font-size: 11px; }
.rf-x-profile-badges { margin-top: 14px; }
.rf-x-profile-badges > span { display: block; margin-bottom: 7px; color: var(--rf-muted); font-size: 12px; }
.rf-x-profile-bio { margin: 14px 0 0; line-height: 1.45; white-space: pre-wrap; overflow-wrap: anywhere; }
.rf-x-profile-details { display: flex; flex-wrap: wrap; gap: 6px 16px; margin-top: 13px; color: var(--rf-muted); font-size: 14px; }
.rf-x-profile-details span { display: inline-flex; align-items: center; gap: 5px; }
.rf-x-profile-stats { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 12px; color: var(--rf-muted); font-size: 14px; }
.rf-x-profile-stats strong { color: var(--rf-text); font-variant-numeric: tabular-nums; }
.rf-x-blocked-note { margin: 0 16px 16px; padding: 11px 13px; border: 1px solid color-mix(in srgb, var(--rf-danger) 28%, var(--rf-line)); border-radius: 8px; color: var(--rf-danger); font-size: 13px; }
.rf-x-profile-tabs { display: grid; min-height: 53px; grid-template-columns: repeat(2, minmax(0, 1fr)); border-bottom: 1px solid var(--rf-line); }
.rf-x-profile-tabs button { position: relative; display: grid; min-width: 0; min-height: 53px; place-items: center; color: var(--rf-muted); background: transparent; font-weight: 600; transition: color 150ms ease-out, background-color 150ms ease-out; }
.rf-x-profile-tabs button:hover { background: var(--rf-bg-hover); }
.rf-x-profile-tabs button.active { color: var(--rf-text); font-weight: 700; }
.rf-x-profile-tabs button.active span::after { position: absolute; right: 0; bottom: -16px; left: 0; height: 4px; border-radius: 4px; background: var(--primary); content: ''; }
.rf-x-profile-tabs button span { position: relative; }
.rf-x-profile-content { min-width: 0; min-height: 240px; }
.rf-x-post-row, .rf-x-resource-row { display: flex; min-width: 0; gap: 11px; padding: 13px 16px 10px; border-bottom: 1px solid var(--rf-line); color: inherit; transition: background-color 150ms ease-out; }
.rf-x-post-row:hover, .rf-x-resource-row:hover { color: inherit; background: var(--rf-bg-hover); }
.rf-x-post-body, .rf-x-resource-row > div { min-width: 0; flex: 1; }
.rf-x-post-meta { display: flex; min-width: 0; min-height: 21px; align-items: center; gap: 4px; color: var(--rf-muted); font-size: 14px; }
.rf-x-post-meta strong { overflow: hidden; color: var(--rf-text); text-overflow: ellipsis; white-space: nowrap; }
.rf-x-post-meta > span:not(.rf-x-post-more):not(.rf-x-post-handle), .rf-x-post-meta time { flex: 0 0 auto; white-space: nowrap; }
.rf-x-post-handle { min-width: 0; flex: 1 1 auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-post-more { display: inline-flex; margin-left: auto; }
.rf-x-post-context { margin: 1px 0 4px; color: var(--rf-muted); font-size: 12px; }
.rf-x-profile-post-tags { margin: 5px 0; }
.rf-x-post-body h2, .rf-x-resource-row h2 { margin: 0 0 3px; font-size: 16px; line-height: 1.35; overflow-wrap: anywhere; }
.rf-x-post-body :deep(.rf-markdown), .rf-x-resource-row p { margin: 0 0 10px; line-height: 1.5; overflow-wrap: anywhere; }
.rf-x-resource-row p { white-space: pre-wrap; }
.rf-x-post-body :deep(.rf-markdown.primary) { font-size: 16px; }
.rf-x-post-actions { display: flex; max-width: 455px; align-items: center; justify-content: space-between; margin-top: 10px; color: var(--rf-muted); font-size: 12px; }
.rf-x-post-actions span { display: inline-flex; min-width: 34px; align-items: center; gap: 6px; }
.rf-x-post-actions span.active { color: var(--primary); }
.rf-x-resource-row header { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 12px; }
.rf-x-resource-row header > span { display: flex; min-width: 0; align-items: center; gap: 5px; }
.rf-x-resource-row header strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-resource-row header small { flex: 0 0 auto; color: var(--rf-muted); font-size: 13px; }
.rf-x-resource-row header b { flex: 0 0 auto; color: var(--primary); font-size: 14px; font-variant-numeric: tabular-nums; }
.rf-x-resource-row footer { display: flex; flex-wrap: wrap; gap: 5px 16px; margin-top: 10px; color: var(--rf-muted); font-size: 12px; }
.rf-profile-tab-enter-active { transition: opacity 190ms ease-out, transform 210ms cubic-bezier(.22, 1, .36, 1); }
.rf-profile-tab-leave-active { transition: opacity 110ms ease-in, transform 110ms ease-in; }
.rf-profile-tab-enter-from { opacity: 0; transform: translateX(8px); }
.rf-profile-tab-leave-to { opacity: 0; transform: translateX(-5px); }
.rf-x-profile-loading { min-height: 620px; overflow: hidden; }
.rf-x-profile-loading-header { display: flex; height: 56px; align-items: center; gap: 12px; padding: 0 16px; }
.rf-x-profile-loading-header > span, .rf-x-profile-loading-header i, .rf-x-profile-loading-body > span, .rf-x-profile-loading-body i { display: block; border-radius: var(--rf-pill); background: var(--rf-bg-subtle); animation: rf-x-pulse 1.3s ease-in-out infinite alternate; }
.rf-x-profile-loading-header > span { width: 34px; height: 34px; }
.rf-x-profile-loading-header > div { display: flex; flex-direction: column; gap: 6px; }
.rf-x-profile-loading-header i:first-child { width: 132px; height: 16px; }
.rf-x-profile-loading-header i:last-child { width: 68px; height: 10px; }
.rf-x-profile-loading-cover { width: 100%; aspect-ratio: 3 / 1; background: var(--rf-bg-subtle); animation: rf-x-pulse 1.3s ease-in-out infinite alternate; }
.rf-x-profile-loading-body { display: flex; flex-direction: column; gap: 10px; padding: 0 16px 28px; }
.rf-x-profile-loading-body > span { width: 104px; height: 104px; margin-top: -52px; border: 4px solid var(--rf-bg); border-radius: 50%; }
.rf-x-profile-loading-body i { width: 55%; height: 13px; }
.rf-x-profile-loading-body i:nth-of-type(2) { width: 82%; }
.rf-x-profile-loading-body i:nth-of-type(3) { width: 38%; }
@keyframes rf-x-pulse { from { opacity: .55; } to { opacity: 1; } }

@media (max-width: 560px) {
  .rf-x-profile-header { padding-inline: 10px; }
  .rf-x-profile-cover { aspect-ratio: 3 / 1; }
  .rf-x-profile-action-row { min-height: 54px; padding: 10px 12px 0; }
  .rf-x-profile-avatar { bottom: 7px; left: 12px; width: 80px !important; height: 80px !important; flex-basis: 80px !important; border-width: 3px; }
  .rf-x-profile-loading-body > span { width: 80px; height: 80px; margin-top: -40px; border-width: 3px; }
  .rf-x-profile-actions { gap: 6px; }
  .rf-x-profile-copy { padding: 0 12px 15px; }
  .rf-x-profile-name h1 { font-size: 20px; }
  .rf-x-outline-button, .rf-x-follow-button { min-height: 44px; padding-inline: 14px; font-size: 13px; }
  .rf-x-outline-control { width: 44px; height: 44px; flex-basis: 44px; }
  .rf-x-blocked-note { margin-inline: 12px; }
  .rf-x-post-row, .rf-x-resource-row { gap: 9px; padding: 12px 11px 9px; }
  .rf-x-post-meta { font-size: 13px; }
  .rf-x-post-meta time { display: none; }
  .rf-x-post-body h2, .rf-x-resource-row h2 { font-size: 15px; }
  .rf-x-resource-row header small { display: none; }
  .rf-x-post-actions span { min-width: 28px; gap: 4px; }
}

@media (prefers-reduced-motion: reduce) {
  .rf-x-profile-loading-header > span, .rf-x-profile-loading-header i, .rf-x-profile-loading-cover, .rf-x-profile-loading-body > span, .rf-x-profile-loading-body i { animation: none; }
}
</style>
