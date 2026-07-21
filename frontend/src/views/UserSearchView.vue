<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createConversation,
  errorMessage,
  fetchHotUsers,
  searchUsers,
  type UserSearchResult,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import PageContainer from '@/components/PageContainer.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const router = useRouter()
const auth = useAuthStore()
const query = ref('')
const submittedQuery = ref('')
const loading = ref(true)
const searching = ref(false)
const viewMode = ref<'hot' | 'results'>('hot')
const users = ref<UserSearchResult[]>([])
const hotUsers = ref<UserSearchResult[]>([])

const visibleUsers = computed(() => viewMode.value === 'hot' ? hotUsers.value : users.value)
const listTitle = computed(() => viewMode.value === 'hot' ? '热门排行' : `“${submittedQuery.value}”的搜索结果`)

async function loadHotUsers() {
  loading.value = true
  try {
    hotUsers.value = await fetchHotUsers()
  } catch (e) {
    Notify.danger(errorMessage(e, '热门用户加载失败'))
  } finally {
    loading.value = false
  }
}

async function submitSearch() {
  const keyword = query.value.trim()
  if (!keyword) {
    viewMode.value = 'hot'
    return
  }

  searching.value = true
  try {
    users.value = await searchUsers(keyword)
    submittedQuery.value = keyword
    viewMode.value = 'results'
  } catch (e) {
    Notify.danger(errorMessage(e, '用户搜索失败'))
  } finally {
    searching.value = false
  }
}

function clearSearch() {
  query.value = ''
  submittedQuery.value = ''
  users.value = []
  viewMode.value = 'hot'
}

async function directMessage(user: UserSearchResult) {
  if (!auth.user) {
    await router.push('/login')
    return
  }
  if (auth.user.id === user.id) return

  try {
    const conversation = await createConversation({ kind: 'direct', member_ids: [user.id] })
    await router.push({ path: '/messages', query: { conversation: String(conversation.id) } })
  } catch (e) {
    Notify.danger(errorMessage(e, '私信创建失败'))
  }
}

onMounted(loadHotUsers)
</script>

<template>
  <PageContainer title="用户发现">
    <div class="rf-discovery">
      <form class="rf-discovery-search" role="search" @submit.prevent="submitSearch">
        <AppIcon name="search" size="20" />
        <input
          v-model="query"
          type="search"
          autocomplete="off"
          aria-label="搜索社区用户"
          placeholder="搜索昵称或 Roblox 名称"
        />
        <button
          v-if="query"
          type="button"
          class="rf-discovery-clear"
          aria-label="清除搜索"
          title="清除搜索"
          @click="clearSearch"
        >
          <AppIcon name="close" size="16" />
        </button>
        <button type="submit" class="rf-discovery-submit" :disabled="searching">
          {{ searching ? '搜索中' : '搜索' }}
        </button>
      </form>

      <div class="rf-discovery-tabs" role="tablist" aria-label="用户列表类型">
        <button
          type="button"
          role="tab"
          :aria-selected="viewMode === 'hot'"
          :class="{ active: viewMode === 'hot' }"
          @click="viewMode = 'hot'"
        >
          热门排行
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="viewMode === 'results'"
          :class="{ active: viewMode === 'results' }"
          :disabled="!submittedQuery"
          @click="viewMode = 'results'"
        >
          搜索结果<span v-if="submittedQuery"> {{ users.length }}</span>
        </button>
      </div>

      <div class="rf-discovery-heading">
        <strong>{{ listTitle }}</strong>
        <span v-if="!loading">{{ visibleUsers.length }} 位用户</span>
      </div>

      <div v-if="loading" class="rf-discovery-loading" aria-label="正在加载用户">
        <nut-skeleton v-for="n in 5" :key="n" animated avatar title row="2" height="13px" />
      </div>

      <section v-else-if="visibleUsers.length" class="rf-discovery-list" :aria-label="listTitle">
        <article
          v-for="(user, index) in visibleUsers"
          :key="`${viewMode}-${user.id}`"
          class="rf-discovery-user"
          :class="{ 'has-rank': viewMode === 'hot' }"
        >
          <span v-if="viewMode === 'hot'" class="rf-discovery-rank" :class="{ top: index < 3 }">
            {{ index + 1 }}
          </span>
          <RouterLink :to="`/users/${user.id}`" class="rf-discovery-identity">
            <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="48" />
            <span class="rf-discovery-copy">
              <span class="rf-discovery-name">
                <strong>{{ user.display_name }}</strong>
                <VerifiedBadge :verified="user.blue_verified" :label="user.verification_label" />
              </span>
              <small v-if="user.roblox_name">@{{ user.roblox_name }}</small>
              <span class="rf-discovery-bio">{{ user.bio || '这个用户还没有填写简介。' }}</span>
              <span class="rf-discovery-stats">
                <span><b>{{ user.post_count }}</b> 帖子</span>
                <span><b>{{ user.resource_count }}</b> 资源</span>
              </span>
            </span>
          </RouterLink>
          <button
            v-if="auth.user?.id !== user.id"
            type="button"
            class="rf-discovery-message"
            :aria-label="`给 ${user.display_name} 发私信`"
            :title="`给 ${user.display_name} 发私信`"
            @click="directMessage(user)"
          >
            <AppIcon name="message" size="19" />
          </button>
        </article>
      </section>

      <div v-else class="rf-empty rf-discovery-empty">
        <AppIcon :name="viewMode === 'hot' ? 'people' : 'search'" size="30" />
        <strong>{{ viewMode === 'hot' ? '暂时没有热门用户' : '没有找到相关用户' }}</strong>
        <span>{{ viewMode === 'hot' ? '社区有新动态后会生成排行。' : '检查关键词，或尝试搜索 Roblox 名称。' }}</span>
        <button v-if="viewMode === 'results'" type="button" @click="clearSearch">返回热门排行</button>
      </div>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-discovery { min-width: 0; }
.rf-discovery-search { display: flex; align-items: center; gap: 10px; min-height: 46px; margin: 14px 16px; padding: 0 5px 0 15px; border: 1px solid transparent; border-radius: var(--rf-pill); color: var(--rf-muted); background: var(--rf-bg-subtle); transition: border-color 150ms ease-out, box-shadow 150ms ease-out; }
.rf-discovery-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 15%, transparent); }
.rf-discovery-search input { min-width: 0; height: 42px; flex: 1; border: 0; outline: 0; color: var(--rf-text); background: transparent; }
.rf-discovery-search input::-webkit-search-cancel-button { display: none; }
.rf-discovery-clear { display: inline-grid; width: 32px; height: 32px; flex: 0 0 32px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }
.rf-discovery-clear:hover { color: var(--rf-text); background: var(--rf-bg-hover); }
.rf-discovery-submit { min-width: 64px; min-height: 36px; padding: 0 15px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 13px; font-weight: 700; }
.rf-discovery-submit:hover { background: var(--primary-hover); }
.rf-discovery-submit:disabled { opacity: .55; cursor: wait; }
.rf-discovery-tabs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); border-top: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); }
.rf-discovery-tabs button { position: relative; min-height: 52px; color: var(--rf-muted); background: transparent; font-weight: 600; }
.rf-discovery-tabs button:hover:not(:disabled) { color: var(--rf-text); background: var(--rf-bg-hover); }
.rf-discovery-tabs button::after { position: absolute; right: 28%; bottom: 0; left: 28%; height: 4px; border-radius: 4px 4px 0 0; background: var(--primary); content: ''; opacity: 0; transform: scaleX(.35); transition: opacity 150ms ease-out, transform 200ms cubic-bezier(.22, 1, .36, 1); }
.rf-discovery-tabs button.active { color: var(--rf-text); font-weight: 700; }
.rf-discovery-tabs button.active::after { opacity: 1; transform: scaleX(1); }
.rf-discovery-tabs button:disabled { color: var(--rf-faint); cursor: default; }
.rf-discovery-heading { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 16px; color: var(--rf-muted); font-size: 13px; }
.rf-discovery-heading strong { color: var(--rf-text); font-weight: 700; }
.rf-discovery-heading span { font-variant-numeric: tabular-nums; }
.rf-discovery-loading { display: flex; flex-direction: column; gap: 18px; padding: 16px; border-top: 1px solid var(--rf-line); }
.rf-discovery-loading :deep(.nut-skeleton) { padding: 0; }
.rf-discovery-list { border-top: 1px solid var(--rf-line); }
.rf-discovery-user { display: grid; grid-template-columns: minmax(0, 1fr) 44px; align-items: center; gap: 10px; min-height: 92px; padding: 13px 16px; border-bottom: 1px solid var(--rf-line); transition: background-color 140ms ease-out; }
.rf-discovery-user.has-rank { grid-template-columns: 24px minmax(0, 1fr) 44px; }
.rf-discovery-user:hover { background: var(--rf-bg-hover); }
.rf-discovery-rank { color: var(--rf-faint); font-size: 14px; font-variant-numeric: tabular-nums; font-weight: 700; text-align: center; }
.rf-discovery-rank.top { color: var(--primary); font-size: 17px; }
.rf-discovery-identity { display: grid; min-width: 0; grid-template-columns: 48px minmax(0, 1fr); align-items: center; gap: 11px; color: inherit; }
.rf-discovery-identity:hover { color: inherit; }
.rf-discovery-identity > .rf-user-avatar { color: var(--rf-muted); background: var(--rf-bg-subtle); }
.rf-discovery-copy { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; }
.rf-discovery-name { display: flex; max-width: 100%; align-items: center; gap: 5px; }
.rf-discovery-name strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.rf-discovery-copy > small { max-width: 100%; overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rf-discovery-bio { display: -webkit-box; max-width: 100%; margin-top: 3px; overflow: hidden; color: var(--rf-muted); font-size: 13px; line-height: 1.35; -webkit-box-orient: vertical; -webkit-line-clamp: 1; }
.rf-discovery-stats { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 5px; color: var(--rf-muted); font-size: 12px; }
.rf-discovery-stats b { color: var(--rf-text); font-variant-numeric: tabular-nums; }
.rf-discovery-message { display: inline-grid; width: 40px; height: 40px; place-items: center; border: 1px solid var(--rf-line); border-radius: 50%; color: var(--rf-text); background: var(--rf-bg); transition: color 140ms ease-out, background-color 140ms ease-out, transform 100ms ease-out; }
.rf-discovery-message:hover { color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, var(--rf-bg)); }
.rf-discovery-message:active { transform: scale(.94); }
.rf-discovery-empty { min-height: 260px; border-top: 1px solid var(--rf-line); }
.rf-discovery-empty button { min-height: 40px; margin-top: 8px; padding: 0 16px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-weight: 700; }

@media (max-width: 560px) {
  .rf-discovery-search { margin: 12px; }
  .rf-discovery-heading { padding-inline: 12px; }
  .rf-discovery-user { min-height: 88px; padding: 12px; }
  .rf-discovery-user.has-rank { grid-template-columns: 20px minmax(0, 1fr) 42px; }
  .rf-discovery-identity { grid-template-columns: 44px minmax(0, 1fr); gap: 9px; }
  .rf-discovery-identity > .rf-user-avatar { width: 44px !important; height: 44px !important; flex-basis: 44px !important; }
  .rf-discovery-message { width: 40px; height: 40px; }
}
</style>
