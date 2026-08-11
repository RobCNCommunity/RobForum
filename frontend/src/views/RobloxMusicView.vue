<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type RobloxMusic } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const query = ref('')
const loading = ref(false)
const errorMessage = ref('')
const result = ref<RobloxMusic | null>(null)
const favorites = ref<RobloxMusic[]>([])

async function loadFavorites() {
  const response = await api.get('/me/roblox/music')
  favorites.value = response.data.data || []
}

async function search() {
  const value = query.value.trim()
  if (!/^\d+$/.test(value)) {
    errorMessage.value = '请输入纯数字的 Roblox 音乐 ID'
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await api.get('/roblox/music', { params: { query: value } })
    result.value = { ...response.data.data, favorited: favorites.value.some((item) => item.asset_id === response.data.data.asset_id) }
  } catch (error: any) {
    result.value = null
    errorMessage.value = error?.response?.data?.error?.message || '暂时无法查询该音乐 ID'
  } finally {
    loading.value = false
  }
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
    else {
      await navigator.clipboard.writeText(`${item.name}（Roblox 音乐 ID：${item.asset_id}）\n${url}`)
      window.alert('音乐链接已复制')
    }
  } catch (error: any) {
    if (error?.name !== 'AbortError') errorMessage.value = '分享失败，请稍后重试'
  }
}

function applyAssetFromURL() {
  const asset = new URLSearchParams(window.location.search).get('asset')
  if (asset && /^\d+$/.test(asset)) { query.value = asset; void search() }
}

onMounted(async () => {
  try { await loadFavorites(); applyAssetFromURL() } catch (error: any) { errorMessage.value = error?.response?.data?.error?.message || '收藏加载失败' }
})
</script>

<template>
  <section class="rf-page music-page">
    <header class="rf-page-header">
      <div class="rf-page-heading"><p class="rf-kicker">ROBLOX AUDIO INDEX</p><h1>Roblox 音乐库</h1><p>查询 Roblox 音乐 ID，收藏常用曲目并分享给队友。</p></div>
    </header>
    <form class="music-search" @submit.prevent="search">
      <AppIcon name="search" size="19" />
      <input v-model="query" inputmode="numeric" autocomplete="off" placeholder="输入 Roblox 音乐 ID，例如 1843529605" aria-label="Roblox 音乐 ID" />
      <button class="rf-primary-button" type="submit" :disabled="loading"><AppIcon name="search" size="16" />{{ loading ? '查询中' : '查询' }}</button>
    </form>
    <p class="music-note"><AppIcon name="info" size="15" />当前仅提供 ID 与元数据查询，暂不支持试听。</p>
    <p v-if="errorMessage" class="music-error" role="alert">{{ errorMessage }}</p>
    <section v-if="result" class="music-result">
      <div class="music-cover"><img v-if="result.thumbnail_url" :src="result.thumbnail_url" alt="" /><AppIcon v-else name="soundOn" size="42" /></div>
      <div class="music-copy"><span class="music-id">ASSET {{ result.asset_id }}</span><h2>{{ result.name || `Roblox 音乐 ${result.asset_id}` }}</h2><p>{{ result.description || '暂无描述' }}</p><small>创作者：{{ result.creator_name || '未知' }}</small></div>
      <div class="music-actions"><button class="rf-icon-text-button" type="button" :aria-pressed="result.favorited" @click="toggleFavorite(result)"><AppIcon name="star" size="17" />{{ result.favorited ? '已收藏' : '收藏' }}</button><button class="rf-icon-text-button" type="button" @click="share(result)"><AppIcon name="share" size="17" />分享</button></div>
    </section>
    <section class="music-section"><div class="section-title"><h2>我的收藏</h2><span>{{ favorites.length }} / 100</span></div><div v-if="favorites.length" class="music-list"><article v-for="item in favorites" :key="item.asset_id" class="music-row"><div class="music-row-cover"><img v-if="item.thumbnail_url" :src="item.thumbnail_url" alt="" /><AppIcon v-else name="soundOn" size="20" /></div><div class="music-row-copy"><strong>{{ item.name || `Roblox 音乐 ${item.asset_id}` }}</strong><small>ID {{ item.asset_id }} · {{ item.creator_name || '未知创作者' }}</small></div><button class="rf-icon-button" type="button" aria-label="分享音乐" @click="share(item)"><AppIcon name="share" size="17" /></button><button class="rf-icon-button" type="button" aria-label="取消收藏" @click="toggleFavorite(item)"><AppIcon name="star" size="17" /></button></article></div><nut-empty v-else description="还没有收藏音乐" /></section>
  </section>
</template>

<style scoped>
.music-page { padding-bottom: 48px; }
.music-page .rf-page-header { padding-bottom: 16px; }
.rf-kicker { margin: 0 0 5px; color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.music-search { display: flex; align-items: center; gap: 10px; margin: 0 20px; padding: 8px 9px 8px 13px; border: 1px solid var(--rf-line); border-radius: 9px; background: var(--rf-bg-subtle); color: var(--rf-muted); }
.music-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent); }
.music-search input { min-width: 0; flex: 1; border: 0; outline: 0; color: var(--rf-text); background: transparent; }
.music-search .rf-primary-button { min-height: 36px; padding-inline: 13px; }
.music-note { display: flex; align-items: center; gap: 6px; margin: 10px 20px; color: var(--rf-muted); font-size: 12px; }
.music-error { margin: 12px 20px; padding: 10px 12px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.music-result { display: grid; grid-template-columns: 88px minmax(0, 1fr) auto; gap: 16px; align-items: center; margin: 18px 20px 0; padding: 16px; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg); }
.music-cover, .music-row-cover { display: grid; place-items: center; overflow: hidden; color: var(--primary); background: var(--rf-bg-subtle); }
.music-cover { width: 88px; aspect-ratio: 1; border-radius: 8px; }
.music-cover img, .music-row-cover img { width: 100%; height: 100%; object-fit: cover; }
.music-copy { min-width: 0; }
.music-id { color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .06em; }
.music-copy h2 { margin: 4px 0 4px; overflow-wrap: anywhere; font-size: 18px; }
.music-copy p { margin: 0 0 6px; color: var(--rf-muted); font-size: 13px; line-height: 1.5; }
.music-copy small { color: var(--rf-muted); font-size: 12px; }
.music-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 7px; }
.music-section { margin-top: 24px; padding: 20px; border-top: 1px solid var(--rf-line); }
.section-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.section-title h2 { margin: 0; font-size: 18px; }
.section-title span { color: var(--rf-muted); font-size: 12px; }
.music-list { border-top: 1px solid var(--rf-line); }
.music-row { display: grid; grid-template-columns: 42px minmax(0, 1fr) auto auto; gap: 11px; align-items: center; min-height: 62px; border-bottom: 1px solid var(--rf-line); }
.music-row-cover { width: 42px; height: 42px; border-radius: 6px; }
.music-row-copy { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.music-row-copy strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.music-row-copy small { color: var(--rf-muted); font-size: 11px; }
@media (max-width: 650px) { .music-result { grid-template-columns: 64px minmax(0, 1fr); } .music-cover { width: 64px; } .music-actions { grid-column: 1 / -1; justify-content: flex-start; } }
</style>
