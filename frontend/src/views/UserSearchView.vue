<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchHotUsers, searchUsers, type UserSearchResult } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const query = ref('')
const submittedQuery = ref('')
const loading = ref(true)
const searching = ref(false)
const users = ref<UserSearchResult[]>([])
const hotUsers = ref<UserSearchResult[]>([])

const showingResults = computed(() => Boolean(submittedQuery.value))
const visibleUsers = computed(() => showingResults.value ? users.value : hotUsers.value)
const heading = computed(() => showingResults.value ? `“${submittedQuery.value}”的用户` : '推荐关注')
const helper = computed(() => showingResults.value ? `${users.value.length} 个匹配结果` : '活跃创作者与社区玩家')

async function loadHotUsers() {
  loading.value = true
  try {
    hotUsers.value = await fetchHotUsers()
  } catch (error) {
    Notify.danger(errorMessage(error, '推荐用户加载失败'))
  } finally {
    loading.value = false
  }
}

async function submitSearch() {
  const keyword = query.value.trim()
  if (!keyword) {
    submittedQuery.value = ''
    users.value = []
    return
  }
  searching.value = true
  try {
    users.value = await searchUsers(keyword)
    submittedQuery.value = keyword
  } catch (error) {
    Notify.danger(errorMessage(error, '用户搜索失败'))
  } finally {
    searching.value = false
  }
}

function clearSearch() {
  query.value = ''
  submittedQuery.value = ''
  users.value = []
}

function handle(user: UserSearchResult) {
  const name = (user.custom_uid || user.roblox_name)?.trim().replace(/\s+/g, '_')
  return `@${name || `user_${user.id}`}`
}

onMounted(loadHotUsers)
</script>

<template>
  <PageContainer>
    <div class="rf-explore">
      <header class="rf-explore-searchbar">
        <form role="search" @submit.prevent="submitSearch">
          <AppIcon name="search" size="20" />
          <input
            v-model="query"
            type="search"
            autocomplete="off"
            aria-label="搜索社区用户"
            placeholder="搜索用户"
            @keydown.esc="clearSearch"
          />
          <button v-if="query" type="button" class="rf-explore-clear" aria-label="清除搜索" title="清除搜索" @click="clearSearch">
            <AppIcon name="close" size="16" />
          </button>
        </form>
        <button type="button" class="rf-explore-submit" :disabled="searching" @click="submitSearch">
          {{ searching ? '搜索中' : '搜索' }}
        </button>
      </header>

      <section class="rf-explore-section" :aria-label="heading">
        <header class="rf-explore-heading">
          <div>
            <h1>{{ heading }}</h1>
            <span>{{ helper }}</span>
          </div>
          <button v-if="showingResults" type="button" class="rf-explore-reset" @click="clearSearch">查看推荐</button>
        </header>

        <div v-if="loading" class="rf-explore-loading" aria-label="正在加载用户">
          <nut-skeleton v-for="n in 5" :key="n" animated avatar title row="2" height="13px" />
        </div>

        <div v-else-if="visibleUsers.length" class="rf-explore-list">
          <RouterLink v-for="user in visibleUsers" :key="user.id" :to="`/users/${user.id}`" class="rf-explore-user">
            <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="48" :frame="user.avatar_frame" />
            <span class="rf-explore-copy">
              <span class="rf-explore-name">
                <strong>{{ user.display_name }}</strong>
                <VerifiedBadge :verified="user.blue_verified" :label="user.verification_label" /><MembershipBadge :active="user.member_active" :tier-id="user.membership_tier_id" />
              </span>
              <small>{{ handle(user) }}</small>
              <span v-if="user.bio" class="rf-explore-bio">{{ user.bio }}</span>
              <span class="rf-explore-stats"><b>{{ user.post_count }}</b> 动态 <i>·</i> <b>{{ user.resource_count }}</b> 资源</span>
            </span>
            <AppIcon name="chevron" size="19" />
          </RouterLink>
        </div>

        <div v-else class="rf-empty rf-explore-empty">
          <AppIcon :name="showingResults ? 'search' : 'people'" size="30" />
          <strong>{{ showingResults ? '没有找到相关用户' : '暂时没有推荐用户' }}</strong>
          <span>{{ showingResults ? '换一个昵称、Roblox 名称或关键词再试。' : '社区产生新动态后，推荐会在这里出现。' }}</span>
          <button v-if="showingResults" type="button" class="rf-primary-button" @click="clearSearch">查看推荐</button>
        </div>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-explore { min-width: 0; }
.rf-explore-searchbar { position: sticky; top: var(--rf-header); z-index: 6; display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(14px); }
.rf-explore-searchbar form { display: flex; min-width: 0; flex: 1; align-items: center; gap: 9px; min-height: 44px; padding: 0 13px; border: 1px solid transparent; border-radius: var(--rf-pill); color: var(--rf-muted); background: var(--rf-bg-subtle); transition: border-color 150ms ease-out, box-shadow 150ms ease-out, background-color 150ms ease-out; }
.rf-explore-searchbar form:focus-within { border-color: var(--primary); color: var(--primary); background: var(--rf-bg); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.rf-explore-searchbar input { min-width: 0; height: 42px; flex: 1; border: 0; outline: 0; color: var(--rf-text); background: transparent; }
.rf-explore-searchbar input::-webkit-search-cancel-button { display: none; }
.rf-explore-clear { display: inline-grid; width: 28px; height: 28px; flex: 0 0 28px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }
.rf-explore-clear:hover { color: var(--rf-text); background: var(--rf-bg-hover); }
.rf-explore-submit { min-width: 58px; min-height: 36px; padding: 0 12px; border-radius: var(--rf-pill); color: var(--rf-text); background: transparent; font-size: 13px; font-weight: 700; }
.rf-explore-submit:hover { background: var(--rf-bg-hover); }.rf-explore-submit:disabled { cursor: wait; opacity: .55; }
.rf-explore-section { min-width: 0; }
.rf-explore-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 14px; padding: 20px 16px 12px; }
.rf-explore-heading h1 { margin: 0; font-family: var(--rf-font-display); font-size: 22px; line-height: 1.2; }.rf-explore-heading span { display: block; margin-top: 4px; color: var(--rf-muted); font-size: 13px; }
.rf-explore-reset { min-height: 34px; padding: 0 10px; border-radius: var(--rf-pill); color: var(--primary); background: transparent; font-size: 13px; font-weight: 700; }.rf-explore-reset:hover { background: color-mix(in srgb, var(--primary) 9%, transparent); }
.rf-explore-loading { display: flex; flex-direction: column; gap: 18px; padding: 16px; border-top: 1px solid var(--rf-line); }.rf-explore-loading :deep(.nut-skeleton) { padding: 0; }
.rf-explore-list { border-top: 1px solid var(--rf-line); }
.rf-explore-user { display: grid; grid-template-columns: 48px minmax(0, 1fr) 22px; align-items: center; gap: 11px; min-height: 80px; padding: 12px 16px; border-bottom: 1px solid var(--rf-line); color: var(--rf-text); transition: background-color 140ms ease-out; }.rf-explore-user:hover { color: var(--rf-text); background: var(--rf-bg-hover); }.rf-explore-user > :deep(.rf-user-avatar) { color: var(--rf-muted); background: var(--rf-bg-subtle); }.rf-explore-user > svg { color: var(--rf-faint); }
.rf-explore-copy { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; }.rf-explore-name { display: flex; max-width: 100%; align-items: center; gap: 5px; }.rf-explore-name strong, .rf-explore-copy > small { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-explore-name strong { font-size: 15px; }.rf-explore-copy > small { color: var(--rf-muted); font-size: 12px; }.rf-explore-bio { display: -webkit-box; max-width: 100%; margin-top: 3px; overflow: hidden; color: var(--rf-muted); font-size: 13px; line-height: 1.35; -webkit-box-orient: vertical; -webkit-line-clamp: 1; }.rf-explore-stats { margin-top: 4px; color: var(--rf-muted); font-size: 12px; }.rf-explore-stats b { color: var(--rf-text); font-variant-numeric: tabular-nums; }.rf-explore-stats i { margin: 0 4px; color: var(--rf-faint); font-style: normal; }
.rf-explore-empty { min-height: 280px; border-top: 1px solid var(--rf-line); }.rf-explore-empty .rf-primary-button { margin-top: 6px; }
@media (max-width: 560px) { .rf-explore-searchbar { padding: 10px 12px; }.rf-explore-heading { padding-inline: 12px; }.rf-explore-user { padding: 12px; }.rf-explore-heading h1 { font-size: 20px; } }
</style>
