<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorMessage, fetchCommunitySearch, type CommunitySearchResult, type Post, type Resource, type UserSearchResult } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PostTagList from '@/components/PostTagList.vue'

type SearchTab = 'all' | 'posts' | 'resources' | 'users'

const route = useRoute()
const router = useRouter()
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
const activeTab = ref<SearchTab>('all')
const loading = ref(false)
const error = ref('')
const activeQuery = ref('')
const emptyResults = (): CommunitySearchResult => ({ query: '', posts: [], resources: [], users: [] })
const results = ref<CommunitySearchResult>(emptyResults())
let searchVersion = 0

const total = computed(() => results.value.posts.length + results.value.resources.length + results.value.users.length)
const hasQuery = computed(() => Boolean(activeQuery.value))
const tabs = computed(() => [
  { key: 'all' as const, label: '全部', count: total.value },
  { key: 'posts' as const, label: '帖子', count: results.value.posts.length },
  { key: 'resources' as const, label: '资源', count: results.value.resources.length },
  { key: 'users' as const, label: '用户', count: results.value.users.length },
])

function excerpt(value: string, max = 140) {
  const text = (value || '').replace(/\s+/g, ' ').trim()
  return text.length > max ? `${text.slice(0, max)}...` : text
}

function handle(user: UserSearchResult) {
  const name = user.roblox_name?.trim().replace(/\s+/g, '_')
  return `@${name || `user_${user.id}`}`
}

function money(cents: number) {
  return cents > 0 ? `¥${(cents / 100).toFixed(2)}` : '免费'
}

async function load(query: string) {
  const current = query.trim()
  const version = ++searchVersion
  activeQuery.value = current
  error.value = ''
  if (!current) {
    results.value = emptyResults()
    loading.value = false
    return
  }

  loading.value = true
  try {
    const next = await fetchCommunitySearch(current)
    if (version !== searchVersion) return
    results.value = next
  } catch (cause) {
    if (version !== searchVersion) return
    results.value = emptyResults()
    error.value = errorMessage(cause, '搜索暂时不可用')
  } finally {
    if (version === searchVersion) loading.value = false
  }
}

function submitSearch() {
  const q = search.value.trim()
  if (q === activeQuery.value) {
    void load(q)
    return
  }
  void router.push({ path: '/search', query: q ? { q } : {} })
}

function clearSearch() {
  search.value = ''
  activeTab.value = 'all'
  void router.push({ path: '/search' })
}

function showTab(tab: SearchTab) {
  activeTab.value = tab
}

watch(
  () => route.query.q,
  (value) => {
    search.value = typeof value === 'string' ? value : ''
    void load(search.value)
  },
  { immediate: true },
)
</script>

<template>
  <PageContainer>
    <section class="rf-search-view">
      <div class="rf-search-sticky">
        <header class="rf-search-top">
          <button type="button" class="rf-search-back" aria-label="返回首页" title="返回首页" @click="router.back()">
            <AppIcon name="back" size="20" />
          </button>
          <form class="rf-search-form" role="search" @submit.prevent="submitSearch">
            <AppIcon name="search" size="19" />
            <input v-model="search" type="search" autocomplete="off" maxlength="80" aria-label="搜索帖子、资源和用户" placeholder="搜索帖子、资源和用户" @keydown.esc="clearSearch" />
            <button v-if="search" type="button" class="rf-search-clear" aria-label="清除搜索" title="清除搜索" @click="clearSearch">
              <AppIcon name="close" size="16" />
            </button>
            <button type="submit" class="rf-search-submit" aria-label="搜索" title="搜索" :disabled="loading">
              <AppIcon name="search" size="18" />
            </button>
          </form>
        </header>

        <nav v-if="hasQuery" class="rf-search-tabs" role="tablist" aria-label="搜索结果分类">
          <button v-for="tab in tabs" :key="tab.key" type="button" role="tab" :aria-selected="activeTab === tab.key" :class="{ active: activeTab === tab.key }" @click="showTab(tab.key)">
            <span>{{ tab.label }}</span><small>{{ tab.count }}</small>
          </button>
        </nav>
      </div>

      <template v-if="hasQuery">
        <div v-if="error" class="rf-search-error" role="alert">
          <span>{{ error }}</span>
          <button type="button" @click="load(activeQuery)">重试</button>
        </div>

        <div v-else-if="loading" class="rf-search-loading" aria-label="正在搜索">
          <nut-skeleton v-for="n in 5" :key="n" animated avatar title row="2" height="13px" />
        </div>

        <div v-else-if="!total" class="rf-search-empty">
          <AppIcon name="search" size="31" />
          <strong>没有找到相关结果</strong>
          <span>换一个更具体的关键词再试试。</span>
        </div>

        <div v-else class="rf-search-results">
          <section v-if="(activeTab === 'all' || activeTab === 'posts') && results.posts.length" class="rf-search-group" aria-labelledby="search-posts-title">
            <header><h1 id="search-posts-title">帖子</h1><span>{{ results.posts.length }} 条结果</span></header>
            <RouterLink v-for="post in results.posts" :key="post.id" :to="`/posts/${post.id}`" class="rf-search-post">
              <UserAvatar :src="post.author_avatar" :name="post.author_name" :size="42" />
              <div class="rf-search-copy">
                <span class="rf-search-author"><strong>{{ post.author_name }}</strong><VerifiedBadge :verified="post.author_verified" :label="post.author_verification_label" /><MembershipBadge :active="post.author_member" :tier-id="post.author_membership_tier_id" /><small>@user_{{ post.author_id }}</small></span>
                <span class="rf-search-post-meta"><em>{{ post.board_name }}</em></span>
                <PostTagList :tags="post.tags" compact class="rf-search-tags" />
                <strong v-if="post.title" class="rf-search-title">{{ post.title }}</strong>
                <MarkdownContent v-if="post.content.trim()" :source="post.content" compact class="rf-search-excerpt" :class="{ primary: !post.title }" />
                <span class="rf-search-stats"><span><AppIcon name="message" size="15" />{{ post.comment_count }}</span><span><AppIcon name="heart" size="15" />{{ post.like_count }}</span></span>
              </div>
              <AppIcon name="chevron" size="18" />
            </RouterLink>
          </section>

          <section v-if="(activeTab === 'all' || activeTab === 'resources') && results.resources.length" class="rf-search-group" aria-labelledby="search-resources-title">
            <header><h1 id="search-resources-title">资源</h1><span>{{ results.resources.length }} 条结果</span></header>
            <RouterLink v-for="resource in results.resources" :key="resource.id" to="/resources" class="rf-search-resource">
              <span class="rf-search-resource-mark"><AppIcon name="shop" size="20" /></span>
              <span class="rf-search-copy">
                <span class="rf-search-author"><strong>{{ resource.creator_name }}</strong><VerifiedBadge :verified="resource.creator_verified" :label="resource.creator_verification_label" /><MembershipBadge :active="resource.creator_member" :tier-id="resource.creator_membership_tier_id" /></span>
                <strong class="rf-search-title">{{ resource.title }}</strong>
                <span v-if="excerpt(resource.description, 120)" class="rf-search-excerpt">{{ excerpt(resource.description, 120) }}</span>
                <span class="rf-search-resource-meta"><span>{{ resource.game }}{{ resource.version ? ` · ${resource.version}` : '' }}</span><b>{{ money(resource.price_cents) }}</b></span>
              </span>
              <AppIcon name="chevron" size="18" />
            </RouterLink>
          </section>

          <section v-if="(activeTab === 'all' || activeTab === 'users') && results.users.length" class="rf-search-group" aria-labelledby="search-users-title">
            <header><h1 id="search-users-title">用户</h1><span>{{ results.users.length }} 位用户</span></header>
            <RouterLink v-for="user in results.users" :key="user.id" :to="`/users/${user.id}`" class="rf-search-user">
              <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="48" />
              <span class="rf-search-copy">
                <span class="rf-search-author"><strong>{{ user.display_name }}</strong><VerifiedBadge :verified="user.blue_verified" :label="user.verification_label" /><MembershipBadge :active="user.member_active" :tier-id="user.membership_tier_id" /></span>
                <small>{{ handle(user) }}</small>
                <span v-if="user.bio" class="rf-search-excerpt">{{ excerpt(user.bio, 96) }}</span>
                <span class="rf-search-user-stats"><b>{{ user.post_count }}</b> 动态 <i>·</i> <b>{{ user.resource_count }}</b> 资源</span>
              </span>
              <AppIcon name="chevron" size="18" />
            </RouterLink>
          </section>

          <div v-if="activeTab !== 'all' && !tabs.find((item) => item.key === activeTab)?.count" class="rf-search-empty rf-search-empty--tab">
            <AppIcon name="search" size="28" />
            <strong>这个分类没有结果</strong>
          </div>
        </div>
      </template>

      <div v-else class="rf-search-empty rf-search-empty--start">
        <AppIcon name="search" size="31" />
        <strong>搜索社区</strong>
        <span>帖子、资源和用户会在同一处呈现。</span>
      </div>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-search-view { min-width: 0; }
.rf-search-sticky { position: sticky; top: 0; z-index: 10; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(14px); }
.rf-search-top { display: grid; grid-template-columns: 44px minmax(0, 1fr); align-items: center; gap: 8px; min-height: 60px; padding: 8px 12px 4px; }
.rf-search-back { display: inline-grid; width: 44px; height: 44px; place-items: center; border-radius: 50%; color: var(--rf-text); background: transparent; }.rf-search-back:hover { background: var(--rf-bg-hover); }
.rf-search-form { display: flex; min-width: 0; min-height: 42px; align-items: center; gap: 8px; padding: 0 9px 0 13px; border: 1px solid transparent; border-radius: var(--rf-pill); color: var(--rf-muted); background: var(--rf-bg-subtle); transition: border-color 150ms ease-out, box-shadow 150ms ease-out, background-color 150ms ease-out; }.rf-search-form:focus-within { border-color: var(--primary); color: var(--primary); background: var(--rf-bg); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }.rf-search-form input { min-width: 0; height: 40px; flex: 1; border: 0; outline: 0; color: var(--rf-text); background: transparent; }.rf-search-form input::-webkit-search-cancel-button { display: none; }
.rf-search-clear, .rf-search-submit { display: inline-grid; width: 30px; height: 30px; flex: 0 0 30px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }.rf-search-clear:hover { color: var(--rf-text); background: var(--rf-bg-hover); }.rf-search-submit { color: #fff; background: var(--primary); }.rf-search-submit:hover { background: var(--primary-hover); }.rf-search-submit:disabled { cursor: wait; opacity: .6; }
.rf-search-tabs { display: flex; min-height: 52px; padding: 6px 8px 0; overflow-x: auto; scrollbar-width: none; }.rf-search-tabs::-webkit-scrollbar { display: none; }.rf-search-tabs button { position: relative; display: inline-flex; min-width: max-content; min-height: 46px; flex: 1; align-items: center; justify-content: center; gap: 5px; padding: 0 15px; color: var(--rf-muted); background: transparent; font-size: 14px; font-weight: 600; transition: color 150ms ease-out, background-color 150ms ease-out; }.rf-search-tabs button:hover { background: var(--rf-bg-hover); }.rf-search-tabs button.active { color: var(--rf-text); font-weight: 700; }.rf-search-tabs button.active::after { position: absolute; right: 14px; bottom: 0; left: 14px; height: 3px; border-radius: var(--rf-pill); background: var(--primary); content: ''; }.rf-search-tabs small { color: var(--rf-faint); font-size: 11px; font-weight: 600; }.rf-search-tabs button.active small { color: var(--primary); }
.rf-search-error { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 12px; padding: 11px 13px; border-radius: 10px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }.rf-search-error button { min-height: 32px; padding: 0 8px; border-radius: var(--rf-pill); color: var(--primary); background: transparent; font-weight: 700; }
.rf-search-loading { display: flex; flex-direction: column; gap: 18px; padding: 18px 16px; }.rf-search-loading :deep(.nut-skeleton) { padding: 0; }
.rf-search-results { min-width: 0; }.rf-search-group + .rf-search-group { margin-top: 10px; border-top: 10px solid var(--rf-bg-subtle); }.rf-search-group > header { display: flex; min-height: 56px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 16px; border-bottom: 1px solid var(--rf-line); }.rf-search-group h1 { margin: 0; font-family: var(--rf-font-display); font-size: 20px; }.rf-search-group > header span { color: var(--rf-muted); font-size: 12px; }
.rf-search-post, .rf-search-resource, .rf-search-user { display: grid; min-width: 0; grid-template-columns: 42px minmax(0, 1fr) 20px; align-items: start; gap: 11px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); color: var(--rf-text); transition: background-color 140ms ease-out; }.rf-search-post:hover, .rf-search-resource:hover, .rf-search-user:hover { color: var(--rf-text); background: var(--rf-bg-hover); }.rf-search-post > svg, .rf-search-resource > svg, .rf-search-user > svg { align-self: center; color: var(--rf-faint); }
.rf-search-copy { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; }.rf-search-author { display: flex; max-width: 100%; align-items: center; gap: 5px; line-height: 1.2; }.rf-search-author strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }.rf-search-author small, .rf-search-copy > small { overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.rf-search-post-meta { display: flex; align-items: center; gap: 6px; margin-top: 4px; color: var(--rf-muted); font-size: 12px; }.rf-search-post-meta em { padding: 1px 6px; border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 9%, transparent); font-style: normal; }.rf-search-title { display: block; max-width: 100%; margin-top: 5px; font-family: var(--rf-font-display); font-size: 16px; line-height: 1.35; }.rf-search-excerpt { display: -webkit-box; max-width: 100%; margin-top: 3px; overflow: hidden; color: var(--rf-muted); font-size: 13px; line-height: 1.45; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }.rf-search-stats { display: flex; align-items: center; gap: 17px; margin-top: 8px; color: var(--rf-muted); font-size: 12px; }.rf-search-stats span { display: inline-flex; align-items: center; gap: 4px; }
.rf-search-tags { margin-top: 5px; }.rf-search-excerpt.primary { color: var(--rf-text); font-size: 14px; }
.rf-search-resource { grid-template-columns: 42px minmax(0, 1fr) 20px; align-items: center; }.rf-search-resource-mark { display: inline-grid; width: 42px; height: 42px; place-items: center; border-radius: 12px; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }.rf-search-resource-meta { display: flex; width: 100%; align-items: center; justify-content: space-between; gap: 10px; margin-top: 7px; color: var(--rf-muted); font-size: 12px; }.rf-search-resource-meta span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-search-resource-meta b { flex: 0 0 auto; color: var(--rf-text); }
.rf-search-user { grid-template-columns: 48px minmax(0, 1fr) 20px; align-items: center; }.rf-search-user-stats { margin-top: 5px; color: var(--rf-muted); font-size: 12px; }.rf-search-user-stats b { color: var(--rf-text); font-variant-numeric: tabular-nums; }.rf-search-user-stats i { margin: 0 4px; color: var(--rf-faint); font-style: normal; }
.rf-search-empty { display: flex; min-height: 280px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 40px 24px; color: var(--rf-muted); text-align: center; }.rf-search-empty > svg { margin-bottom: 5px; color: var(--primary); }.rf-search-empty strong { color: var(--rf-text); font-size: 20px; }.rf-search-empty span { max-width: 30ch; line-height: 1.55; }.rf-search-empty--start { min-height: calc(100vh - 160px); }.rf-search-empty--tab { min-height: 200px; }
@media (max-width: 1019px) { .rf-search-empty--start { min-height: calc(100vh - 160px); } }
@media (max-width: 560px) { .rf-search-top { grid-template-columns: 40px minmax(0, 1fr); min-height: 56px; padding: 6px 8px 2px; }.rf-search-back { width: 40px; height: 40px; }.rf-search-tabs { min-height: 50px; padding: 5px 6px 0; justify-content: flex-start; }.rf-search-tabs button { min-height: 45px; flex: 0 0 auto; padding-inline: 12px; }.rf-search-group > header { min-height: 52px; padding-inline: 12px; }.rf-search-post, .rf-search-resource, .rf-search-user { padding: 13px 12px; } }
</style>
