<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { createRobloxMusicSubmission, fetchMusicCategories, fetchMyRobloxMusicSubmissions, type MusicCategory, type RobloxMusicSubmission } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const categories = ref<MusicCategory[]>([])
const submissions = ref<RobloxMusicSubmission[]>([])
const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const notice = ref('')
const form = reactive({ asset_id: '', name: '', image_url: '', category_id: 0 })

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [categoryItems, submissionItems] = await Promise.all([fetchMusicCategories(), fetchMyRobloxMusicSubmissions()])
    categories.value = categoryItems
    submissions.value = submissionItems
    if (!categoryItems.some((item) => item.id === form.category_id)) form.category_id = categoryItems[0]?.id || 0
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '投稿页面加载失败'
  } finally {
    loading.value = false
  }
}

async function submit() {
  const assetID = Number(form.asset_id.trim())
  if (!Number.isSafeInteger(assetID) || assetID <= 0) { errorMessage.value = '请输入有效的 Roblox 音乐 ID'; return }
  if (!form.category_id) { errorMessage.value = '请选择音乐分区'; return }
  submitting.value = true
  errorMessage.value = ''
  notice.value = ''
  try {
    await createRobloxMusicSubmission({ asset_id: assetID, name: form.name.trim(), image_url: form.image_url.trim(), category_id: form.category_id })
    Object.assign(form, { asset_id: '', name: '', image_url: '', category_id: categories.value[0]?.id || 0 })
    notice.value = '投稿已提交，审核通过后会显示在全部音乐中'
    submissions.value = await fetchMyRobloxMusicSubmissions()
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '投稿提交失败'
  } finally {
    submitting.value = false
  }
}

function statusLabel(status: RobloxMusicSubmission['status']) {
  return status === 'approved' ? '已通过' : status === 'rejected' ? '已驳回' : '审核中'
}

onMounted(load)
</script>

<template>
  <section class="rf-page music-submit-page">
    <header class="rf-page-header"><div class="rf-page-heading"><p class="rf-kicker">MUSIC SUBMISSION</p><h1>投稿音乐</h1><p>填写 Roblox 音乐信息并提交审核，审核通过后会进入全部音乐。</p></div><RouterLink to="/music" class="rf-secondary-button"><AppIcon name="back" size="16" />返回音乐库</RouterLink></header>
    <p v-if="errorMessage" class="music-message error" role="alert">{{ errorMessage }}</p>
    <p v-if="notice" class="music-message success" role="status">{{ notice }}</p>

    <section class="submit-panel">
      <form class="submission-form" @submit.prevent="submit">
        <label>Roblox 音乐 ID<input v-model.trim="form.asset_id" inputmode="numeric" placeholder="例如 1843529605" required /></label>
        <label>音乐名称<input v-model.trim="form.name" maxlength="120" placeholder="给这首音乐起个名字" required /></label>
        <label>音乐分区<select v-model.number="form.category_id" required><option :value="0" disabled>请选择分区</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option></select></label>
        <label>图片 URL<input v-model.trim="form.image_url" type="url" placeholder="https://..." required /></label>
        <button class="rf-primary-button" type="submit" :disabled="submitting || loading || !categories.length"><AppIcon name="upload" size="16" />{{ submitting ? '提交中' : '提交审核' }}</button>
      </form>
    </section>

    <section class="submission-history"><div class="section-title"><h2>我的投稿</h2><span>{{ submissions.length }} 条</span></div><div v-if="loading" class="rf-list-loading"><span v-for="n in 3" :key="n" /></div><div v-else-if="submissions.length" class="submission-list"><article v-for="item in submissions" :key="item.id"><div><strong>{{ item.name }}</strong><small>ID {{ item.asset_id }} · {{ item.category_name || '未分区' }} · {{ new Date(item.created_at).toLocaleString('zh-CN') }}</small></div><span :class="['submission-status', item.status]">{{ statusLabel(item.status) }}</span><small v-if="item.review_note" class="review-note">{{ item.review_note }}</small></article></div><nut-empty v-else description="还没有投稿记录" /></section>
  </section>
</template>

<style scoped>
.music-submit-page { padding-bottom: 48px; }
.rf-kicker { margin: 0 0 5px; color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.music-message { margin: 12px 20px 0; padding: 10px 12px; font-size: 12px; }
.music-message.error { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.music-message.success { color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 8%, transparent); }
.submit-panel, .submission-history { padding: 20px; border-top: 1px solid var(--rf-line); }
.submission-form { display: grid; max-width: 720px; grid-template-columns: 1fr 1fr; gap: 14px; }
.submission-form label { display: flex; flex-direction: column; gap: 6px; color: var(--rf-muted); font-size: 12px; }
.submission-form input, .submission-form select { min-height: 40px; padding: 8px 10px; border: 1px solid var(--rf-line); border-radius: 6px; color: var(--rf-text); background: var(--rf-bg); }
.submission-form button { justify-self: start; }
.section-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.section-title h2 { margin: 0; font-size: 18px; }
.section-title span { color: var(--rf-muted); font-size: 12px; }
.submission-list { border-top: 1px solid var(--rf-line); }
.submission-list article { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 5px 12px; padding: 13px 0; border-bottom: 1px solid var(--rf-line); }
.submission-list article > div { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.submission-list small { color: var(--rf-muted); font-size: 11px; }
.submission-status { align-self: start; font-size: 12px; font-weight: 700; }
.submission-status.pending { color: var(--rf-muted); }.submission-status.approved { color: var(--rf-success); }.submission-status.rejected { color: var(--rf-danger); }
.review-note { grid-column: 1 / -1; }
@media (max-width: 650px) { .music-submit-page .rf-page-header { align-items: flex-start; flex-direction: column; }.submit-panel, .submission-history { padding-inline: 14px; }.submission-form { grid-template-columns: 1fr; } }
</style>
