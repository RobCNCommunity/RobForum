<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, createRobloxMusicSubmission, fetchMyRobloxMusicSubmissions, fetchRobloxMusic, type RobloxMusic, type RobloxMusicSubmission } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const query = ref('')
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const notice = ref('')
const library = ref<RobloxMusic[]>([])
const favorites = ref<RobloxMusic[]>([])
const submissions = ref<RobloxMusicSubmission[]>([])
const form = reactive({ asset_id: '', name: '', image_url: '' })

async function load(q = query.value.trim()) {
  loading.value = true
  errorMessage.value = ''
  try {
    const [music, favoriteResponse, ownSubmissions] = await Promise.all([
      fetchRobloxMusic(q),
      api.get('/me/roblox/music'),
      fetchMyRobloxMusicSubmissions(),
    ])
    favorites.value = favoriteResponse.data.data || []
    submissions.value = ownSubmissions
    library.value = music.map((item) => ({ ...item, favorited: favorites.value.some((favorite) => favorite.asset_id === item.asset_id) }))
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '音乐库加载失败'
  } finally {
    loading.value = false
  }
}

async function search() { await load(query.value.trim()) }

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

async function submit() {
  const assetID = Number(form.asset_id.trim())
  if (!Number.isSafeInteger(assetID) || assetID <= 0) { errorMessage.value = '请输入有效的 Roblox 音乐 ID'; return }
  submitting.value = true
  errorMessage.value = ''
  notice.value = ''
  try {
    await createRobloxMusicSubmission({ asset_id: assetID, name: form.name.trim(), image_url: form.image_url.trim() })
    Object.assign(form, { asset_id: '', name: '', image_url: '' })
    notice.value = '投稿已提交，审核通过后会显示在音乐库中'
    await load()
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '投稿提交失败'
  } finally {
    submitting.value = false
  }
}

function statusLabel(status: RobloxMusicSubmission['status']) {
  return status === 'approved' ? '已通过' : status === 'rejected' ? '已驳回' : '审核中'
}

function applyAssetFromURL() {
  const asset = new URLSearchParams(window.location.search).get('asset')
  if (asset) { query.value = asset; void load(asset) }
}

onMounted(() => { void load().then(applyAssetFromURL) })
</script>

<template>
  <section class="rf-page music-page">
    <header class="rf-page-header"><div class="rf-page-heading"><p class="rf-kicker">COMMUNITY ROBLOX AUDIO</p><h1>Roblox 音乐库</h1><p>浏览社区分享的 Roblox 音乐，收藏常用曲目并分享给队友。</p></div></header>
    <form class="music-search" @submit.prevent="search"><AppIcon name="search" size="19" /><input v-model="query" autocomplete="off" placeholder="搜索音乐名称或 Roblox 音乐 ID" aria-label="搜索音乐名称或 Roblox 音乐 ID" /><button class="rf-primary-button" type="submit" :disabled="loading"><AppIcon name="search" size="16" />{{ loading ? '搜索中' : '搜索' }}</button></form>
    <p class="music-note"><AppIcon name="info" size="15" />音乐由社区投稿并经审核后展示，暂不支持试听。</p>
    <p v-if="errorMessage" class="music-error" role="alert">{{ errorMessage }}</p>
    <p v-if="notice" class="music-notice" role="status">{{ notice }}</p>

    <section class="music-section library-section"><div class="section-title"><h2>{{ query.trim() ? '搜索结果' : '已审核音乐' }}</h2><span>{{ library.length }} 首</span></div><div v-if="library.length" class="music-grid"><article v-for="item in library" :key="item.asset_id" class="music-card"><div class="music-card-cover"><img v-if="item.thumbnail_url" :src="item.thumbnail_url" alt="" /><AppIcon v-else name="soundOn" size="34" /></div><div class="music-card-copy"><span class="music-id">ROBLOX ID {{ item.asset_id }}</span><h3>{{ item.name }}</h3><small>社区投稿 · 暂无试听</small></div><footer><button class="rf-icon-text-button" type="button" :aria-pressed="item.favorited" @click="toggleFavorite(item)"><AppIcon name="star" size="16" />{{ item.favorited ? '已收藏' : '收藏' }}</button><button class="rf-icon-text-button" type="button" @click="share(item)"><AppIcon name="share" size="16" />分享</button></footer></article></div><nut-empty v-else :description="query.trim() ? '没有匹配的已审核音乐' : '暂无已审核音乐，来提交第一首吧'" /></section>

    <section class="music-section submit-section"><div class="section-title"><div><h2>投稿音乐</h2><p>填写音乐 ID、名称和图片，审核通过后会公开展示。</p></div></div><form class="submission-form" @submit.prevent="submit"><label>Roblox 音乐 ID<input v-model.trim="form.asset_id" inputmode="numeric" placeholder="例如 1843529605" required /></label><label>音乐名称<input v-model.trim="form.name" maxlength="120" placeholder="给这首音乐起个名字" required /></label><label class="full">图片 URL<input v-model.trim="form.image_url" type="url" placeholder="https://..." required /></label><button class="rf-primary-button" type="submit" :disabled="submitting"><AppIcon name="upload" size="16" />{{ submitting ? '提交中' : '提交审核' }}</button></form></section>

    <section class="music-section"><div class="section-title"><h2>我的投稿</h2><span>{{ submissions.length }} 条</span></div><div v-if="submissions.length" class="submission-list"><article v-for="item in submissions" :key="item.id"><div><strong>{{ item.name }}</strong><small>ID {{ item.asset_id }} · {{ new Date(item.created_at).toLocaleString() }}</small></div><span :class="['submission-status', item.status]">{{ statusLabel(item.status) }}</span><small v-if="item.review_note" class="review-note">{{ item.review_note }}</small></article></div><nut-empty v-else description="还没有投稿记录" /></section>
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
.music-card h3 { margin: 4px 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 15px; }
.music-card-copy small { color: var(--rf-muted); font-size: 11px; }
.music-card footer { display: flex; grid-column: 1 / -1; gap: 7px; }
.submission-form { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; max-width: 680px; }
.submission-form label { display: flex; flex-direction: column; gap: 5px; color: var(--rf-muted); font-size: 12px; }
.submission-form label.full { grid-column: 1 / -1; }
.submission-form input { min-height: 38px; padding: 8px 10px; border: 1px solid var(--rf-line); border-radius: 6px; color: var(--rf-text); background: var(--rf-bg); }
.submission-form button { justify-self: start; }
.submission-list { border-top: 1px solid var(--rf-line); }
.submission-list article { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 5px 12px; padding: 12px 0; border-bottom: 1px solid var(--rf-line); }
.submission-list article > div { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.submission-list small { color: var(--rf-muted); font-size: 11px; }
.submission-status { align-self: start; font-size: 12px; font-weight: 700; }
.submission-status.pending { color: var(--rf-muted); }.submission-status.approved { color: var(--rf-success); }.submission-status.rejected { color: var(--rf-danger); }
.review-note { grid-column: 1 / -1; }
@media (max-width: 650px) { .submission-form { grid-template-columns: 1fr; }.submission-form label.full { grid-column: auto; }.music-section { padding-inline: 14px; }.music-search { margin-inline: 14px; } }
</style>
