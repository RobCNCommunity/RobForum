<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  createAdminAvatarFrame,
  deleteAdminAvatarFrame,
  errorMessage,
  fetchAdminAvatarFrames,
  fetchAdminAvatarFrameSubmissions,
  fetchAdminAvatarFrameUploadSettings,
  reviewAdminAvatarFrameSubmission,
  uploadAdminAvatarFrameImage,
  updateAdminAvatarFrameUploadSettings,
  updateAdminAvatarFrame,
  type AvatarFrame,
  type AvatarFrameSubmission,
  type AvatarFrameUploadSettings,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useAuthStore } from '@/stores/auth'

type FrameInput = Parameters<typeof createAdminAvatarFrame>[0]
const auth = useAuthStore()
const items = ref<AvatarFrame[]>([])
const submissions = ref<AvatarFrameSubmission[]>([])
const uploadSettings = reactive<AvatarFrameUploadSettings>({ allow_regular_upload: false, allow_member_upload: false, can_upload: true, updated_at: '' })
const loading = ref(true)
const saving = ref(false)
const settingsSaving = ref(false)
const reviewingID = ref(0)
const imageUploading = ref(false)
const imageInput = ref<HTMLInputElement | null>(null)
const editingID = ref(0)
const emptyForm = (): FrameInput => ({
  name: '', description: '', style: 'ring', image_url: '', primary_color: '#ff3b30', secondary_color: '#ffd60a',
  price_cents: 0, allowed_regular: true, allowed_member: true, allowed_admin: true, enabled: true, sort_order: 0,
})
const form = reactive<FrameInput>(emptyForm())
const priceYuan = ref(0)
const preview = (): AvatarFrame => ({ ...form, id: editingID.value, sales_count: 0, owned: false, equipped: false, can_use: true, created_at: '', updated_at: '' })

async function load() {
  loading.value = true
  try {
    const [frames, pendingSubmissions, settings] = await Promise.all([fetchAdminAvatarFrames(), fetchAdminAvatarFrameSubmissions(), fetchAdminAvatarFrameUploadSettings()])
    items.value = frames
    submissions.value = pendingSubmissions
    Object.assign(uploadSettings, settings)
  }
  catch (error) { Notify.danger(errorMessage(error, '头像框列表加载失败')) }
  finally { loading.value = false }
}

async function toggleUploadSetting(kind: 'regular' | 'member') {
  if (settingsSaving.value) return
  const next = {
    allow_regular_upload: kind === 'regular' ? !uploadSettings.allow_regular_upload : uploadSettings.allow_regular_upload,
    allow_member_upload: kind === 'member' ? !uploadSettings.allow_member_upload : uploadSettings.allow_member_upload,
  }
  settingsSaving.value = true
  try {
    Object.assign(uploadSettings, await updateAdminAvatarFrameUploadSettings(next))
    Notify.success('头像框上传权限已更新')
  } catch (error) { Notify.danger(errorMessage(error, '上传权限保存失败')) }
  finally { settingsSaving.value = false }
}

function submissionFrame(item: AvatarFrameSubmission): AvatarFrame {
  return {
    ...preview(), id: item.approved_frame_id || 0, name: item.name, description: item.description,
    style: 'image', image_url: item.image_url, enabled: false,
  }
}

async function reviewSubmission(item: AvatarFrameSubmission, status: 'approved' | 'rejected') {
  let note = ''
  if (status === 'approved') {
    if (!window.confirm(`确定通过“${item.name}”并加入头像框市场吗？`)) return
  } else {
    const value = window.prompt(`请输入驳回“${item.name}”的原因`)
    if (value === null) return
    note = value.trim()
    if (!note) { Notify.warn('请填写驳回原因'); return }
  }
  reviewingID.value = item.id
  try {
    await reviewAdminAvatarFrameSubmission(item.id, status, note)
    Notify.success(status === 'approved' ? '审核通过，头像框已加入市场' : '头像框投稿已驳回')
    await load()
  } catch (error) { Notify.danger(errorMessage(error, '头像框审核失败')) }
  finally { reviewingID.value = 0 }
}

function reset() {
  editingID.value = 0
  Object.assign(form, emptyForm())
  priceYuan.value = 0
}

function edit(item: AvatarFrame) {
  editingID.value = item.id
  Object.assign(form, {
    name: item.name, description: item.description, style: item.style, image_url: item.image_url || '', primary_color: item.primary_color,
    secondary_color: item.secondary_color, price_cents: item.price_cents, allowed_regular: item.allowed_regular,
    allowed_member: item.allowed_member, allowed_admin: item.allowed_admin, enabled: item.enabled, sort_order: item.sort_order,
  })
  priceYuan.value = item.price_cents / 100
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function save() {
  form.price_cents = Math.max(0, Math.round(Number(priceYuan.value || 0) * 100))
  if (!form.name.trim()) { Notify.warn('请输入头像框名称'); return }
  if (!/^#[0-9a-f]{6}$/i.test(form.primary_color) || !/^#[0-9a-f]{6}$/i.test(form.secondary_color)) { Notify.warn('颜色代码需使用 #RRGGBB 格式'); return }
  if (form.style === 'image' && !form.image_url) { Notify.warn('请先上传头像框图片'); return }
  if (!form.allowed_regular && !form.allowed_member && !form.allowed_admin) { Notify.warn('至少选择一类可用用户'); return }
  saving.value = true
  try {
    if (editingID.value) await updateAdminAvatarFrame(editingID.value, { ...form })
    else await createAdminAvatarFrame({ ...form })
    Notify.success(editingID.value ? '头像框已更新' : '头像框已创建')
    reset()
    await load()
  } catch (error) { Notify.danger(errorMessage(error, '保存失败')) }
  finally { saving.value = false }
}

async function chooseFrameImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/gif'].includes(file.type)) { Notify.warn('头像框图片仅支持 PNG、JPG 和 GIF'); return }
  if (file.size > 5 * 1024 * 1024) { Notify.warn('头像框图片不能超过 5 MB'); return }
  imageUploading.value = true
  try {
    form.image_url = (await uploadAdminAvatarFrameImage(file)).image_url
    form.style = 'image'
    Notify.success('头像框图片已上传')
  } catch (error) {
    Notify.danger(errorMessage(error, '头像框图片上传失败'))
  } finally {
    imageUploading.value = false
  }
}

async function disable(item: AvatarFrame) {
  if (!window.confirm(`确定停用“${item.name}”吗？正在使用的用户会自动卸下。`)) return
  try { await deleteAdminAvatarFrame(item.id); Notify.success('头像框已停用'); if (editingID.value === item.id) reset(); await load() }
  catch (error) { Notify.danger(errorMessage(error, '停用失败')) }
}

onMounted(load)
</script>

<template>
  <PageContainer title="头像框管理">
    <section class="rf-frame-upload-settings" aria-label="用户头像框上传权限">
      <div><strong>用户上传权限</strong><span>用户投稿后必须由管理员人工审核，通过后才会上架。</span></div>
      <button type="button" role="switch" :aria-checked="uploadSettings.allow_regular_upload" :class="{ active: uploadSettings.allow_regular_upload }" :disabled="settingsSaving" @click="toggleUploadSetting('regular')"><span />普通用户上传</button>
      <button type="button" role="switch" :aria-checked="uploadSettings.allow_member_upload" :class="{ active: uploadSettings.allow_member_upload }" :disabled="settingsSaving" @click="toggleUploadSetting('member')"><span />会员用户上传</button>
    </section>

    <section v-if="submissions.length" class="rf-frame-review-queue">
      <header><div><strong>待审核头像框</strong><span>{{ submissions.length }} 个投稿</span></div><AppIcon name="review" size="18" /></header>
      <article v-for="item in submissions" :key="item.id">
        <UserAvatar :src="item.user_avatar" :name="item.user_name" :size="58" :frame="submissionFrame(item)" />
        <div><strong>{{ item.name }}</strong><span>{{ item.description || '无说明' }}</span><small>{{ item.user_name }} · {{ new Date(item.created_at).toLocaleString('zh-CN') }}</small></div>
        <footer><button type="button" class="rf-primary-button rf-button-small" :disabled="reviewingID !== 0" @click="reviewSubmission(item, 'approved')">通过</button><button type="button" class="rf-danger-button rf-button-small" :disabled="reviewingID !== 0" @click="reviewSubmission(item, 'rejected')">驳回</button></footer>
      </article>
    </section>

    <form class="rf-frame-admin-form" @submit.prevent="save">
      <div class="rf-frame-admin-preview"><UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="76" :frame="preview()" /><small>实时预览</small></div>
      <div class="rf-frame-fields">
        <label><span>名称</span><input v-model.trim="form.name" class="rf-control" maxlength="80" required /></label>
        <label><span>类型与样式</span><select v-model="form.style" class="rf-control"><option value="ring">颜色框 · 单环</option><option value="double">颜色框 · 双环</option><option value="glow">颜色框 · 发光</option><option value="pixel">颜色框 · 像素</option><option value="halo">颜色框 · 光环</option><option value="image">上传图片框</option></select></label>
        <label class="rf-frame-color-field"><span>主色代码</span><span class="rf-frame-color-control"><input v-model.trim="form.primary_color" class="rf-control" maxlength="7" placeholder="#ff3b30" /><input v-model="form.primary_color" type="color" aria-label="选择主色" /></span></label>
        <label class="rf-frame-color-field"><span>辅色代码</span><span class="rf-frame-color-control"><input v-model.trim="form.secondary_color" class="rf-control" maxlength="7" placeholder="#ffd60a" /><input v-model="form.secondary_color" type="color" aria-label="选择辅色" /></span></label>
        <label><span>价格（元）</span><input v-model.number="priceYuan" class="rf-control" type="number" min="0" max="100000" step="0.01" /></label>
        <label><span>排序</span><input v-model.number="form.sort_order" class="rf-control" type="number" min="-10000" max="10000" /></label>
        <label class="rf-frame-description"><span>说明</span><textarea v-model.trim="form.description" class="rf-control rf-textarea" rows="2" maxlength="240" /></label>
        <div v-if="form.style === 'image'" class="rf-frame-image-field">
          <span>头像框图片</span>
          <button type="button" class="rf-frame-image-upload" :disabled="imageUploading" @click="imageInput?.click()"><AppIcon name="upload" size="17" />{{ imageUploading ? '上传中…' : form.image_url ? '更换图片' : '上传图片' }}</button>
          <small>建议使用透明背景的正方形 PNG；支持 JPG、PNG、GIF，最大 5 MB。</small>
          <input ref="imageInput" type="file" accept="image/png,image/jpeg,image/gif" class="rf-frame-file-input" @change="chooseFrameImage" />
        </div>
      </div>
      <fieldset><legend>可用身份</legend><label><input v-model="form.allowed_regular" type="checkbox" />普通用户</label><label><input v-model="form.allowed_member" type="checkbox" />会员用户</label><label><input v-model="form.allowed_admin" type="checkbox" />管理员</label><label><input v-model="form.enabled" type="checkbox" />上架</label></fieldset>
      <footer><button v-if="editingID" type="button" class="rf-secondary-button" @click="reset">取消编辑</button><button type="submit" class="rf-primary-button" :disabled="saving"><AppIcon name="check" size="16" />{{ saving ? '保存中…' : editingID ? '保存修改' : '创建头像框' }}</button></footer>
    </form>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 4" :key="n" /></div>
    <div v-else-if="!items.length" class="rf-empty"><AppIcon name="badge" size="28" /><strong>还没有头像框</strong></div>
    <section v-else class="rf-admin-frame-list">
      <article v-for="item in items" :key="item.id" :class="{ disabled: !item.enabled }">
        <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="52" :frame="item" />
        <div><strong>{{ item.name }}</strong><span>{{ item.description || '无说明' }}</span><small>¥{{ (item.price_cents / 100).toFixed(2) }} · 售出 {{ item.sales_count }} · 排序 {{ item.sort_order }}</small></div>
        <div class="rf-admin-frame-tags"><span v-if="item.allowed_regular">普通</span><span v-if="item.allowed_member">会员</span><span v-if="item.allowed_admin">管理</span><b>{{ item.enabled ? '已上架' : '已停用' }}</b></div>
        <footer class="rf-row-actions"><button type="button" class="rf-secondary-button rf-button-small" @click="edit(item)">编辑</button><button v-if="item.enabled" type="button" class="rf-danger-button rf-button-small" @click="disable(item)">停用</button></footer>
      </article>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-frame-upload-settings { display: flex; align-items: center; gap: 10px; padding: 15px 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-upload-settings > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.rf-frame-upload-settings > div span { color: var(--rf-muted); font-size: 11px; }.rf-frame-upload-settings > button { display: inline-flex; min-height: 38px; align-items: center; gap: 7px; padding: 0 11px; border: 1px solid var(--rf-line); border-radius: 6px; color: var(--rf-muted); background: var(--rf-bg); font-size: 12px; font-weight: 700; }.rf-frame-upload-settings > button > span { position: relative; width: 30px; height: 18px; border-radius: var(--rf-pill); background: var(--rf-faint); transition: background-color 150ms ease-out; }.rf-frame-upload-settings > button > span::after { position: absolute; top: 3px; left: 3px; width: 12px; height: 12px; border-radius: 50%; background: #fff; content: ''; transition: transform 150ms ease-out; }.rf-frame-upload-settings > button.active { border-color: color-mix(in srgb, var(--primary) 40%, var(--rf-line)); color: var(--primary); }.rf-frame-upload-settings > button.active > span { background: var(--primary); }.rf-frame-upload-settings > button.active > span::after { transform: translateX(12px); }.rf-frame-upload-settings > button:disabled { cursor: wait; opacity: .6; }.rf-frame-review-queue { border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--primary) 3%, var(--rf-bg)); }.rf-frame-review-queue > header { display: flex; align-items: center; justify-content: space-between; padding: 12px 20px; border-bottom: 1px solid var(--rf-line); color: var(--primary); }.rf-frame-review-queue > header > div { display: flex; align-items: baseline; gap: 7px; }.rf-frame-review-queue > header span { color: var(--rf-muted); font-size: 11px; }.rf-frame-review-queue article { display: grid; grid-template-columns: 64px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 12px 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-review-queue article:last-child { border-bottom: 0; }.rf-frame-review-queue article > div { display: flex; min-width: 0; flex-direction: column; gap: 2px; }.rf-frame-review-queue article > div span, .rf-frame-review-queue article small { overflow: hidden; color: var(--rf-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.rf-frame-review-queue article footer { display: flex; gap: 6px; }
.rf-frame-admin-form { display: grid; grid-template-columns: 112px minmax(0, 1fr); gap: 16px; padding: 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-admin-preview { display: flex; min-height: 112px; flex-direction: column; align-items: center; justify-content: center; gap: 10px; border-radius: 8px; background: var(--rf-bg-subtle); }.rf-frame-admin-preview small { color: var(--rf-muted); font-size: 11px; }.rf-frame-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }.rf-frame-fields label { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.rf-frame-fields label > span, .rf-frame-image-field > span { color: var(--rf-muted); font-size: 11px; font-weight: 600; }.rf-frame-description, .rf-frame-image-field { grid-column: 1 / -1; }.rf-frame-color-control { display: grid; grid-template-columns: minmax(0, 1fr) 42px; gap: 6px; }.rf-frame-color-control input[type='color'] { width: 42px; height: 42px; padding: 3px; border: 1px solid var(--rf-line); border-radius: 6px; background: var(--rf-bg); cursor: pointer; }.rf-frame-image-field { display: grid; grid-template-columns: minmax(130px, max-content) minmax(0, 1fr); align-items: center; gap: 5px 10px; }.rf-frame-image-field > span { grid-column: 1 / -1; }.rf-frame-image-field small { color: var(--rf-muted); font-size: 11px; }.rf-frame-image-upload { display: inline-flex; min-height: 42px; align-items: center; justify-content: center; gap: 7px; padding: 0 13px; border: 1px dashed var(--rf-faint); border-radius: 6px; color: var(--primary); background: var(--rf-bg-subtle); font-weight: 700; }.rf-frame-image-upload:disabled { cursor: wait; opacity: .6; }.rf-frame-file-input { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); }.rf-frame-admin-form fieldset { display: flex; grid-column: 2; flex-wrap: wrap; gap: 14px; padding: 0; border: 0; }.rf-frame-admin-form legend { margin-bottom: 7px; color: var(--rf-muted); font-size: 11px; font-weight: 600; }.rf-frame-admin-form fieldset label { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; }.rf-frame-admin-form footer { display: flex; grid-column: 2; justify-content: flex-end; gap: 8px; }.rf-frame-admin-form footer button { display: inline-flex; align-items: center; gap: 5px; }
.rf-admin-frame-list article { display: grid; grid-template-columns: 56px minmax(0, 1fr) auto auto; align-items: center; gap: 14px; padding: 14px 20px; border-bottom: 1px solid var(--rf-line); }.rf-admin-frame-list article.disabled { opacity: .58; }.rf-admin-frame-list article > div:nth-child(2) { display: flex; min-width: 0; flex-direction: column; gap: 3px; }.rf-admin-frame-list article > div:nth-child(2) strong, .rf-admin-frame-list article > div:nth-child(2) span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-admin-frame-list article > div:nth-child(2) span, .rf-admin-frame-list article small { color: var(--rf-muted); font-size: 11px; }.rf-admin-frame-tags { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }.rf-admin-frame-tags span, .rf-admin-frame-tags b { padding: 3px 6px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); font-size: 10px; font-weight: 600; }.rf-admin-frame-tags b { color: var(--primary); }.rf-admin-frame-list footer { display: flex; gap: 6px; }
@media (max-width: 760px) { .rf-frame-admin-form { grid-template-columns: 1fr; padding: 16px 12px; }.rf-frame-admin-preview { min-height: 108px; }.rf-frame-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }.rf-frame-fields .rf-frame-description { grid-column: 1 / -1; }.rf-frame-admin-form fieldset, .rf-frame-admin-form footer { grid-column: 1; }.rf-frame-admin-form footer button { flex: 1; justify-content: center; }.rf-admin-frame-list article { grid-template-columns: 52px minmax(0, 1fr); gap: 10px; padding-inline: 12px; }.rf-admin-frame-tags, .rf-admin-frame-list article footer { grid-column: 2; justify-content: flex-start; } }
@media (max-width: 760px) { .rf-frame-upload-settings { flex-wrap: wrap; padding: 13px 12px; }.rf-frame-upload-settings > div { flex-basis: 100%; }.rf-frame-upload-settings > button { flex: 1; justify-content: center; }.rf-frame-review-queue > header, .rf-frame-review-queue article { padding-inline: 12px; }.rf-frame-review-queue article { grid-template-columns: 58px minmax(0, 1fr); }.rf-frame-review-queue article footer { grid-column: 2; } }
@media (max-width: 460px) { .rf-frame-fields { grid-template-columns: 1fr; }.rf-frame-fields > *, .rf-frame-image-field { grid-column: 1; }.rf-frame-image-field { grid-template-columns: 1fr; }.rf-frame-image-field > span { grid-column: 1; } }
</style>
