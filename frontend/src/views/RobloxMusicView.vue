<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, fetchMusicCategories, fetchRobloxMusic, type MusicCategory, type RobloxMusic } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const query = ref('')
const loading = ref(false)
const errorMessage = ref('')
const notice = ref('')
const library = ref<RobloxMusic[]>([])
const favorites = ref<RobloxMusic[]>([])
const categories = ref<MusicCategory[]>([])
const categoryID = ref(0)

async function load(q = query.value.trim()) {
  loading.value = true
  errorMessage.value = ''
  try {
    const [music, favoriteResponse] = await Promise.all([
      fetchRobloxMusic(q, categoryID.value),
      api.get('/me/roblox/music'),
    ])
    favorites.value = favoriteResponse.data.data || []
    library.value = music.map((item) => ({ ...item, favorited: favorites.value.some((favorite) => favorite.asset_id === item.asset_id) }))
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '音乐库加载失败'
  } finally {
    loading.value = false
  }
}

async function search() { await load(query.value.trim()) }

async function selectCategory(id: number) {
  categoryID.value = id
  await load()
}

async function toggleFavorite(item: RobloxMusic) {
  try {
    if (item.favorited) {
      await api.delete(`/me/roblox/music/${item.asset_id}/favorite`)
      favorites.value = favorites.value.filter((favorite) => favorite.asset_id !== item.asset_id)
      item.favorited = false
    } else {
      const response = await api.post(`/me/roblox/music/${item.asset_id}/favorite`)
      const saved = response.data.data as RobloxMusic
      favorites.value = [saved, ...favorites.value.filter((favorite) => favorite.asset_id !== item.asset_id)]
      item.favorited = true
    }
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '收藏操作失败'
  }
}

async function share(item: RobloxMusic) {
  const url = `${window.location.origin}/music?asset=${item.asset_id}`
  try {
    if (navigator.share) await navigator.share({ title: item.name, text: `Roblox 音乐 ID：${item.asset_id}`, url })
    else { await navigator.clipboard.writeText(`${item.name}（Roblox 音乐 ID：${item.asset_id}）\n${url}`); notice.value = '音乐链接已复制' }
  } catch (error: any) {
    if (error?.name !== 'AbortError') errorMessage.value = '分享失败，请稍后重试'
  }
}

function applyAssetFromURL() {
  const asset = new URLSearchParams(window.location.search).get('asset')
  if (asset) { query.value = asset; void load(asset) }
}

onMounted(async () => {
  try {
    categories.value = await fetchMusicCategories()
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '音乐分区加载失败'
  }
  await load()
  applyAssetFromURL()
})
</script>

<template>
  <section class="rf-page music-page">
    <header class="rf-page-header"><div class="rf-page-heading"><p class="rf-kicker">COMMUNITY ROBLOX AUDIO</p><h1>Roblox 音乐库</h1><p>浏览社区分享的 Roblox 音乐，收藏常用曲目并分享给队友。</p></div></header>
    <div class="music-toolbar"><form class="music-search" @submit.prevent="search"><AppIcon name="search" size="19" /><input v-model="query" autocomplete="off" placeholder="搜索音乐名称或 Roblox 音乐 ID" aria-label="搜索音乐名称或 Roblox 音乐 ID" /><button class="rf-primary-button" type="submit" :disabled="loading"><AppIcon name="search" size="16" />{{ loading ? '搜索中' : '搜索' }}</button></form><RouterLink to="/music/submit" class="rf-primary-button music-submit-link"><AppIcon name="upload" size="16" />投稿音乐</RouterLink></div>
    <nav class="music-categories" aria-label="音乐分区"><button type="button" :class="{ active: categoryID === 0 }" @click="selectCategory(0)">全部</button><button v-for="category in categories" :key="category.id" type="button" :class="{ active: categoryID === category.id }" @click="selectCategory(category.id)">{{ category.name }}</button></nav>
    <p class="music-note"><AppIcon name="info" size="15" />音乐由社区投稿并经审核后展示，暂不支持试听。</p>
    <p v-if="errorMessage" class="music-error" role="alert">{{ errorMessage }}</p>
    <p v-if="notice" class="music-notice" role="status">{{ notice }}</p>

    <section class="music-section library-section"><div class="section-title"><h2>{{ query.trim() ? '搜索结果' : '全部音乐' }}</h2><span>{{ library.length }} 首</span></div><div v-if="library.length" class="music-grid"><article v-for="item in library" :key="item.asset_id" class="music-card"><div class="music-card-cover"><img v-if="item.thumbnail_url" :src="item.thumbnail_url" alt="" /><AppIcon v-else name="soundOn" size="34" /></div><div class="music-card-copy"><span class="music-id">ROBLOX ID {{ item.asset_id }}</span><div class="music-title-row"><h3>{{ item.name }}</h3><span class="music-category-tag">{{ item.category_name || '未分区' }}</span></div><small>社区投稿 · 暂无试听</small></div><footer><button class="rf-icon-text-button" type="button" :aria-pressed="item.favorited" @click="toggleFavorite(item)"><AppIcon name="star" size="16" />{{ item.favorited ? '已收藏' : '收藏' }}</button><button class="rf-icon-text-button" type="button" @click="share(item)"><AppIcon name="share" size="16" />分享</button></footer></article></div><nut-empty v-else :description="query.trim() ? '没有匹配的公开音乐' : '暂无公开音乐，来提交第一首吧'" /></section>

  </section>
</template>

<style scoped>
.music-page { padding-bottom: 48px; }
.music-page .rf-page-header { padding-bottom: 16px; }
.rf-kicker { margin: 0 0 5px; color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.music-toolbar { display: flex; align-items: center; gap: 10px; padding: 0 20px; }
.music-search { display: flex; min-width: 0; flex: 1; align-items: center; gap: 10px; margin: 0; padding: 8px 9px 8px 13px; border: 1px solid var(--rf-line); border-radius: 9px; background: var(--rf-bg-subtle); color: var(--rf-muted); }
.music-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent); }
.music-search input { min-width: 0; flex: 1; border: 0; outline: 0; color: var(--rf-text); background: transparent; }
.music-search .rf-primary-button { min-height: 36px; padding-inline: 13px; }
.music-submit-link { min-height: 44px; flex: 0 0 auto; }
.music-categories { display: flex; gap: 7px; padding: 12px 20px 2px; overflow-x: auto; }
.music-categories button { flex: 0 0 auto; min-height: 30px; padding: 0 11px; border: 1px solid var(--rf-line); border-radius: 5px; color: var(--rf-muted); background: var(--rf-bg); font-size: 12px; }
.music-categories button.active { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); font-weight: 700; }
.music-note, .music-error, .music-notice { margin: 10px 20px; font-size: 12px; }
.music-note { display: flex; align-items: center; gap: 6px; color: var(--rf-muted); }
.music-error, .music-notice { padding: 10px 12px; }
.music-error { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.music-notice { color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 8%, transparent); }
.music-section { margin-top: 24px; padding: 20px; border-top: 1px solid var(--rf-line); }
.section-title { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 12px; }
.section-title h2 { margin: 0; font-size: 18px; }
.section-title p { margin: 5px 0 0; color: var(--rf-muted); font-size: 12px; }
.section-title > span { color: var(--rf-muted); font-size: 12px; }
.music-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 12px; }
.music-card { display: grid; grid-template-columns: 64px minmax(0, 1fr); gap: 12px; padding: 13px; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg); }
.music-card-cover { width: 64px; height: 64px; display: grid; place-items: center; overflow: hidden; border-radius: 7px; color: var(--primary); background: var(--rf-bg-subtle); }
.music-card-cover img { width: 100%; height: 100%; object-fit: cover; }
.music-card-copy { min-width: 0; }
.music-id { color: var(--primary); font-size: 10px; font-weight: 700; letter-spacing: .04em; }
.music-title-row { display: flex; min-width: 0; align-items: center; gap: 7px; margin: 4px 0; }
.music-card h3 { min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 15px; }
.music-category-tag { flex: 0 0 auto; max-width: 86px; overflow: hidden; padding: 2px 6px; border-radius: 4px; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); font-size: 10px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.music-card-copy small { color: var(--rf-muted); font-size: 11px; }
.music-card footer { display: flex; grid-column: 1 / -1; gap: 7px; }
@media (max-width: 650px) { .music-page .rf-page-header { align-items: flex-start; flex-direction: column; }.music-toolbar { align-items: stretch; flex-direction: column; padding-inline: 14px; }.music-submit-link { min-width: 116px; min-height: 42px; align-self: flex-start; }.music-section { padding-inline: 14px; }.music-categories { padding-inline: 14px; } }
</style>
