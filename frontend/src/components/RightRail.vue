<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { fetchBoards, fetchHotUsers, type Board, type UserSearchResult } from '@/api'
import AppIcon from './AppIcon.vue'
import UserAvatar from './UserAvatar.vue'
import VerifiedBadge from './VerifiedBadge.vue'
import MembershipBadge from './MembershipBadge.vue'

const router = useRouter()
const search = ref('')
const boards = ref<Board[]>([])
const users = ref<UserSearchResult[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    const [nextBoards, nextUsers] = await Promise.all([fetchBoards(), fetchHotUsers()])
    boards.value = nextBoards.slice(0, 5)
    users.value = nextUsers.slice(0, 3)
  } catch {
    boards.value = []
    users.value = []
  } finally {
    loading.value = false
  }
}

function submitSearch() {
  const q = search.value.trim()
  router.push({ path: '/search', query: q ? { q } : {} })
}

function viewUser(id: number) {
  router.push(`/users/${id}`)
}

function formatHeat(value: number) {
  if (value >= 10000) return `${Math.floor(value / 1000) / 10}k`
  return String(value || 0)
}

onMounted(load)
</script>

<template>
  <aside class="rf-right-rail" aria-label="发现">
    <form class="rf-rail-search" role="search" @submit.prevent="submitSearch">
      <AppIcon name="search" size="18" />
      <input v-model="search" type="search" autocomplete="off" placeholder="搜索 RobForum" aria-label="搜索帖子、攻略和资源" />
    </form>

    <section class="rf-rail-panel rf-rail-trends" aria-labelledby="rail-boards-title">
      <h2 id="rail-boards-title">热门板块</h2>
      <div v-if="loading" class="rf-rail-skeleton"><nut-skeleton v-for="n in 4" :key="n" animated row="1" height="14px" /></div>
      <RouterLink v-for="(board, index) in boards" v-else :key="board.id" :to="`/boards/${board.slug}`" class="rf-trend">
        <span>社区讨论 · {{ index + 1 }}</span>
        <strong>#{{ board.name }}</strong>
        <small>{{ board.post_count }} 篇帖子</small>
      </RouterLink>
      <RouterLink v-if="boards.length" to="/boards/guides" class="rf-show-more">探索更多板块</RouterLink>
    </section>

    <section class="rf-rail-panel rf-rail-users" aria-labelledby="rail-users-title">
      <h2 id="rail-users-title">值得关注</h2>
      <div v-if="loading" class="rf-rail-skeleton"><nut-skeleton v-for="n in 3" :key="n" animated avatar row="1" height="14px" /></div>
      <button v-for="user in users" v-else :key="user.id" type="button" class="rf-user-suggestion" @click="viewUser(user.id)">
        <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="40" :frame="user.avatar_frame" />
        <span>
          <strong>{{ user.display_name }}<VerifiedBadge :verified="user.blue_verified" :label="user.verification_label" /><MembershipBadge :active="user.member_active" :tier-id="user.membership_tier_id" /></strong>
          <small>{{ user.post_count }} 帖子 · 热度 {{ formatHeat(user.hot_score) }}</small>
        </span>
        <AppIcon name="arrow" size="16" />
      </button>
      <RouterLink v-if="users.length" to="/users" class="rf-show-more">查看热门用户</RouterLink>
    </section>

    <section class="rf-rail-note">
      <strong>RobForum</strong>
      <p>面向 Roblox 中文玩家的独立社区。</p>
      <div><RouterLink to="/">关于</RouterLink><span>·</span><RouterLink to="/resources">资源</RouterLink><span>·</span><span>隐私</span></div>
    </section>
  </aside>
</template>
