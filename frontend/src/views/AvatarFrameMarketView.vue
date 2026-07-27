<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  createAvatarFrameSubmission,
  equipAvatarFrame,
  errorMessage,
  fetchAvatarFrames,
  fetchAvatarFrameUploadSettings,
  fetchMe,
  fetchMyAvatarFrameSubmissions,
  fetchWallet,
  purchaseAvatarFrame,
  uploadAvatarFrameSubmissionImage,
  type AvatarFrame,
  type AvatarFrameSubmission,
  type AvatarFrameUploadSettings,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const frames = ref<AvatarFrame[]>([])
const balance = ref(0)
const loading = ref(true)
const pendingID = ref(0)
const money = (cents: number) => cents === 0 ? '免费' : `¥${(cents / 100).toFixed(2)}`
const balanceMoney = (cents: number) => `¥${(cents / 100).toFixed(2)}`
const equipped = computed(() => frames.value.find((item) => item.equipped))
const uploadSettings = ref<AvatarFrameUploadSettings | null>(null)
const submissions = ref<AvatarFrameSubmission[]>([])
const submitting = ref(false)
const submissionImageUploading = ref(false)
const submissionImageInput = ref<HTMLInputElement | null>(null)
const submissionForm = reactive({ name: '', description: '', image_url: '' })
const submissionPreview = computed<AvatarFrame>(() => ({
  id: 0, name: submissionForm.name || '头像框预览', description: submissionForm.description, style: 'image', image_url: submissionForm.image_url,
  primary_color: '#1d9bf0', secondary_color: '#8b5cf6', price_cents: 0, allowed_regular: true, allowed_member: true,
  allowed_admin: true, enabled: true, sort_order: 0, sales_count: 0, owned: false, equipped: false, can_use: true, created_at: '', updated_at: '',
}))

function audience(frame: AvatarFrame) {
  const items: string[] = []
  if (frame.allowed_regular) items.push('普通用户')
  if (frame.allowed_member) items.push('会员')
  if (frame.allowed_admin) items.push('管理员')
  return items
}

async function load() {
  loading.value = true
  try {
    const [items, wallet, settings, ownSubmissions] = await Promise.all([fetchAvatarFrames(), fetchWallet(), fetchAvatarFrameUploadSettings(), fetchMyAvatarFrameSubmissions()])
    frames.value = items
    balance.value = wallet.available_cents
    uploadSettings.value = settings
    submissions.value = ownSubmissions
  } catch (error) {
    Notify.danger(errorMessage(error, '头像框市场加载失败'))
  } finally {
    loading.value = false
  }
}

async function chooseSubmissionImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/gif'].includes(file.type)) { Notify.warn('头像框图片仅支持 PNG、JPG 和 GIF'); return }
  if (file.size > 5 * 1024 * 1024) { Notify.warn('头像框图片不能超过 5 MB'); return }
  submissionImageUploading.value = true
  try { submissionForm.image_url = (await uploadAvatarFrameSubmissionImage(file)).image_url }
  catch (error) { Notify.danger(errorMessage(error, '头像框图片上传失败')) }
  finally { submissionImageUploading.value = false }
}

async function submitAvatarFrame() {
  if (!submissionForm.name.trim() || !submissionForm.image_url || submitting.value) return
  submitting.value = true
  try {
    await createAvatarFrameSubmission({ name: submissionForm.name.trim(), description: submissionForm.description.trim(), image_url: submissionForm.image_url })
    Object.assign(submissionForm, { name: '', description: '', image_url: '' })
    submissions.value = await fetchMyAvatarFrameSubmissions()
    Notify.success('头像框已提交，等待管理员审核')
  } catch (error) { Notify.danger(errorMessage(error, '头像框投稿失败')) }
  finally { submitting.value = false }
}

function submissionStatus(value: AvatarFrameSubmission['status']) {
  return ({ pending: '待审核', approved: '已通过', rejected: '已驳回' } as const)[value]
}

function submissionFrame(item: AvatarFrameSubmission): AvatarFrame {
  return { ...submissionPreview.value, name: item.name, image_url: item.image_url }
}

async function buy(item: AvatarFrame) {
  pendingID.value = item.id
  try {
    await purchaseAvatarFrame(item.id)
    Notify.success(`已获得“${item.name}”`)
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '购买失败'))
  } finally {
    pendingID.value = 0
  }
}

async function equip(item: AvatarFrame | null) {
  pendingID.value = item?.id || -1
  try {
    await equipAvatarFrame(item?.id || 0)
    auth.setUser(await fetchMe())
    await load()
    Notify.success(item ? `已装备“${item.name}”` : '已卸下头像框')
  } catch (error) {
    Notify.danger(errorMessage(error, '头像框设置失败'))
  } finally {
    pendingID.value = 0
  }
}

onMounted(load)
</script>

<template>
  <PageContainer title="头像框市场">
    <header class="rf-frame-market-head">
      <div><span>钱包余额</span><strong>{{ balanceMoney(balance) }}</strong></div>
      <button type="button" class="rf-secondary-button" @click="$router.push('/wallet')"><AppIcon name="wallet" size="16" />钱包</button>
    </header>

    <section v-if="uploadSettings?.can_upload" class="rf-frame-submit-band">
      <div class="rf-frame-submit-preview">
        <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="72" :frame="submissionForm.image_url ? submissionPreview : undefined" />
      </div>
      <form @submit.prevent="submitAvatarFrame">
        <header><strong>投稿头像框</strong><span>提交后由管理员人工审核</span></header>
        <div class="rf-frame-submit-fields">
          <label><span>名称</span><input v-model="submissionForm.name" class="rf-control" maxlength="80" required /></label>
          <label><span>说明</span><input v-model="submissionForm.description" class="rf-control" maxlength="240" /></label>
        </div>
        <footer>
          <button type="button" class="rf-secondary-button" :disabled="submissionImageUploading" @click="submissionImageInput?.click()"><AppIcon name="upload" size="16" />{{ submissionImageUploading ? '上传中…' : submissionForm.image_url ? '更换图片' : '上传头像框' }}</button>
          <button type="submit" class="rf-primary-button" :disabled="submitting || !submissionForm.name.trim() || !submissionForm.image_url">{{ submitting ? '提交中…' : '提交审核' }}</button>
          <input ref="submissionImageInput" class="rf-frame-submit-file" type="file" accept="image/png,image/jpeg,image/gif" @change="chooseSubmissionImage" />
        </footer>
      </form>
    </section>

    <section v-if="submissions.length" class="rf-frame-submission-history">
      <header><strong>我的投稿</strong><span>{{ submissions.length }}</span></header>
      <article v-for="item in submissions" :key="item.id">
        <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="40" :frame="submissionFrame(item)" />
        <div><strong>{{ item.name }}</strong><small>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</small><p v-if="item.review_note">{{ item.review_note }}</p></div>
        <span :class="`is-${item.status}`">{{ submissionStatus(item.status) }}</span>
      </article>
    </section>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 6" :key="n" /></div>
    <div v-else-if="!frames.length" class="rf-empty"><AppIcon name="badge" size="28" /><strong>暂无可选头像框</strong></div>
    <section v-else class="rf-frame-grid">
      <article v-for="item in frames" :key="item.id" class="rf-frame-card" :class="{ 'is-equipped': item.equipped }">
        <div class="rf-frame-preview">
          <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="72" :frame="item" />
        </div>
        <div class="rf-frame-main">
          <div class="rf-frame-title"><h2>{{ item.name }}</h2><span v-if="item.equipped">使用中</span></div>
          <p>{{ item.description || '个性头像框' }}</p>
          <div class="rf-frame-audience"><span v-for="label in audience(item)" :key="label">{{ label }}</span></div>
          <small>{{ item.sales_count }} 人已购买</small>
        </div>
        <footer>
          <strong>{{ money(item.price_cents) }}</strong>
          <button v-if="!item.can_use" type="button" class="rf-secondary-button" disabled>当前身份不可用</button>
          <button v-else-if="!item.owned" type="button" class="rf-primary-button" :disabled="pendingID !== 0 || balance < item.price_cents" @click="buy(item)">
            <AppIcon name="shop" size="16" />{{ pendingID === item.id ? '购买中…' : balance < item.price_cents ? '余额不足' : '购买' }}
          </button>
          <button v-else-if="item.equipped" type="button" class="rf-secondary-button" :disabled="pendingID !== 0" @click="equip(null)">卸下</button>
          <button v-else type="button" class="rf-primary-button" :disabled="pendingID !== 0" @click="equip(item)">装备</button>
        </footer>
      </article>
    </section>

    <div v-if="equipped" class="rf-frame-current"><AppIcon name="check" size="16" />当前使用：{{ equipped.name }}</div>
  </PageContainer>
</template>

<style scoped>
.rf-frame-market-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 20px; border-bottom: 1px solid var(--rf-line); }
.rf-frame-market-head > div { display: flex; min-width: 0; flex-direction: column; gap: 3px; }.rf-frame-market-head span { color: var(--rf-muted); font-size: 12px; }.rf-frame-market-head strong { font-size: 27px; font-variant-numeric: tabular-nums; }.rf-frame-market-head button { display: inline-flex; align-items: center; gap: 6px; }
.rf-frame-submit-band { display: grid; grid-template-columns: 96px minmax(0, 1fr); gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-submit-preview { display: grid; min-height: 96px; place-items: center; border-radius: 8px; background: var(--rf-bg-subtle); }.rf-frame-submit-band form { min-width: 0; }.rf-frame-submit-band form > header { display: flex; align-items: baseline; gap: 8px; margin-bottom: 10px; }.rf-frame-submit-band form > header span { color: var(--rf-muted); font-size: 11px; }.rf-frame-submit-fields { display: grid; grid-template-columns: minmax(120px, .8fr) minmax(180px, 1.2fr); gap: 8px; }.rf-frame-submit-fields label { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.rf-frame-submit-fields label span { color: var(--rf-muted); font-size: 11px; }.rf-frame-submit-band form > footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 10px; }.rf-frame-submit-band button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; }.rf-frame-submit-file { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); }.rf-frame-submission-history { border-bottom: 1px solid var(--rf-line); }.rf-frame-submission-history > header { display: flex; align-items: center; gap: 6px; padding: 11px 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-submission-history > header span { color: var(--rf-muted); font-size: 11px; }.rf-frame-submission-history article { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 10px 20px; }.rf-frame-submission-history article > div { display: flex; min-width: 0; flex-direction: column; }.rf-frame-submission-history article small, .rf-frame-submission-history article p { margin: 0; color: var(--rf-muted); font-size: 11px; }.rf-frame-submission-history article > span { padding: 3px 7px; border-radius: var(--rf-pill); font-size: 10px; font-weight: 700; }.rf-frame-submission-history article > span.is-pending { color: #a56a00; background: color-mix(in srgb, #a56a00 10%, transparent); }.rf-frame-submission-history article > span.is-approved { color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 10%, transparent); }.rf-frame-submission-history article > span.is-rejected { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 10%, transparent); }
.rf-frame-submit-band button:disabled { cursor: not-allowed; opacity: .45; }
.rf-frame-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.rf-frame-card { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 14px; padding: 20px; border-right: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); }.rf-frame-card:nth-child(2n) { border-right: 0; }.rf-frame-card.is-equipped { background: color-mix(in srgb, var(--primary) 5%, var(--rf-bg)); }
.rf-frame-preview { display: grid; min-height: 92px; place-items: center; border-radius: 8px; background: var(--rf-bg-subtle); }.rf-frame-main { min-width: 0; }.rf-frame-title { display: flex; min-width: 0; align-items: center; gap: 7px; }.rf-frame-title h2 { min-width: 0; margin: 0; overflow: hidden; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }.rf-frame-title > span { flex: 0 0 auto; padding: 2px 6px; border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); font-size: 10px; font-weight: 700; }.rf-frame-main p { min-height: 36px; margin: 5px 0 8px; color: var(--rf-muted); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }.rf-frame-main > small { display: block; margin-top: 8px; color: var(--rf-faint); font-size: 11px; }.rf-frame-audience { display: flex; flex-wrap: wrap; gap: 5px; }.rf-frame-audience span { padding: 2px 6px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); font-size: 10px; }
.rf-frame-card footer { display: flex; grid-column: 1 / -1; align-items: center; justify-content: space-between; gap: 10px; }.rf-frame-card footer > strong { font-size: 16px; }.rf-frame-card footer button { display: inline-flex; min-width: 98px; align-items: center; justify-content: center; gap: 5px; }.rf-frame-current { display: flex; align-items: center; gap: 7px; padding: 13px 20px; color: var(--primary); font-size: 12px; }
@media (max-width: 680px) { .rf-frame-market-head { padding: 16px 12px; }.rf-frame-grid { grid-template-columns: minmax(0, 1fr); }.rf-frame-card, .rf-frame-card:nth-child(2n) { padding: 16px 12px; border-right: 0; }.rf-frame-card { grid-template-columns: 82px minmax(0, 1fr); }.rf-frame-preview { min-height: 82px; }.rf-frame-card footer button { min-width: 106px; }.rf-frame-current { padding-inline: 12px; } }
@media (max-width: 680px) { .rf-frame-submit-band { grid-template-columns: 72px minmax(0, 1fr); gap: 10px; padding: 15px 12px; }.rf-frame-submit-preview { min-height: 76px; }.rf-frame-submit-fields { grid-template-columns: 1fr; }.rf-frame-submit-band form > header { flex-direction: column; gap: 1px; }.rf-frame-submit-band form > footer { flex-wrap: wrap; }.rf-frame-submit-band form > footer button { flex: 1; }.rf-frame-submission-history > header, .rf-frame-submission-history article { padding-inline: 12px; } }
</style>
