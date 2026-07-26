<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  createAdminAvatarFrame,
  deleteAdminAvatarFrame,
  errorMessage,
  fetchAdminAvatarFrames,
  updateAdminAvatarFrame,
  type AvatarFrame,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useAuthStore } from '@/stores/auth'

type FrameInput = Parameters<typeof createAdminAvatarFrame>[0]
const auth = useAuthStore()
const items = ref<AvatarFrame[]>([])
const loading = ref(true)
const saving = ref(false)
const editingID = ref(0)
const emptyForm = (): FrameInput => ({
  name: '', description: '', style: 'ring', primary_color: '#ff3b30', secondary_color: '#ffd60a',
  price_cents: 0, allowed_regular: true, allowed_member: true, allowed_admin: true, enabled: true, sort_order: 0,
})
const form = reactive<FrameInput>(emptyForm())
const priceYuan = ref(0)
const preview = (): AvatarFrame => ({ ...form, id: editingID.value, sales_count: 0, owned: false, equipped: false, can_use: true, created_at: '', updated_at: '' })

async function load() {
  loading.value = true
  try { items.value = await fetchAdminAvatarFrames() }
  catch (error) { Notify.danger(errorMessage(error, '头像框列表加载失败')) }
  finally { loading.value = false }
}

function reset() {
  editingID.value = 0
  Object.assign(form, emptyForm())
  priceYuan.value = 0
}

function edit(item: AvatarFrame) {
  editingID.value = item.id
  Object.assign(form, {
    name: item.name, description: item.description, style: item.style, primary_color: item.primary_color,
    secondary_color: item.secondary_color, price_cents: item.price_cents, allowed_regular: item.allowed_regular,
    allowed_member: item.allowed_member, allowed_admin: item.allowed_admin, enabled: item.enabled, sort_order: item.sort_order,
  })
  priceYuan.value = item.price_cents / 100
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function save() {
  form.price_cents = Math.max(0, Math.round(Number(priceYuan.value || 0) * 100))
  if (!form.name.trim()) { Notify.warn('请输入头像框名称'); return }
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

async function disable(item: AvatarFrame) {
  if (!window.confirm(`确定停用“${item.name}”吗？正在使用的用户会自动卸下。`)) return
  try { await deleteAdminAvatarFrame(item.id); Notify.success('头像框已停用'); if (editingID.value === item.id) reset(); await load() }
  catch (error) { Notify.danger(errorMessage(error, '停用失败')) }
}

onMounted(load)
</script>

<template>
  <PageContainer title="头像框管理">
    <form class="rf-frame-admin-form" @submit.prevent="save">
      <div class="rf-frame-admin-preview"><UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="76" :frame="preview()" /><small>实时预览</small></div>
      <div class="rf-frame-fields">
        <label><span>名称</span><input v-model.trim="form.name" class="rf-control" maxlength="80" required /></label>
        <label><span>样式</span><select v-model="form.style" class="rf-control"><option value="ring">单环</option><option value="double">双环</option><option value="glow">发光</option><option value="pixel">像素</option><option value="halo">光环</option></select></label>
        <label><span>主色</span><input v-model="form.primary_color" type="color" class="rf-frame-color" /></label>
        <label><span>辅色</span><input v-model="form.secondary_color" type="color" class="rf-frame-color" /></label>
        <label><span>价格（元）</span><input v-model.number="priceYuan" class="rf-control" type="number" min="0" max="100000" step="0.01" /></label>
        <label><span>排序</span><input v-model.number="form.sort_order" class="rf-control" type="number" min="-10000" max="10000" /></label>
        <label class="rf-frame-description"><span>说明</span><textarea v-model.trim="form.description" class="rf-control rf-textarea" rows="2" maxlength="240" /></label>
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
.rf-frame-admin-form { display: grid; grid-template-columns: 112px minmax(0, 1fr); gap: 16px; padding: 20px; border-bottom: 1px solid var(--rf-line); }.rf-frame-admin-preview { display: flex; min-height: 112px; flex-direction: column; align-items: center; justify-content: center; gap: 10px; border-radius: 8px; background: var(--rf-bg-subtle); }.rf-frame-admin-preview small { color: var(--rf-muted); font-size: 11px; }.rf-frame-fields { display: grid; grid-template-columns: minmax(120px, 1.4fr) minmax(110px, 1fr) 72px 72px minmax(110px, .8fr) 80px; gap: 10px; }.rf-frame-fields label { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.rf-frame-fields label > span { color: var(--rf-muted); font-size: 11px; font-weight: 600; }.rf-frame-description { grid-column: 1 / -1; }.rf-frame-color { width: 100%; min-height: 42px; padding: 4px; border: 1px solid var(--rf-line); border-radius: 6px; background: var(--rf-bg); }.rf-frame-admin-form fieldset { display: flex; grid-column: 2; flex-wrap: wrap; gap: 14px; padding: 0; border: 0; }.rf-frame-admin-form legend { margin-bottom: 7px; color: var(--rf-muted); font-size: 11px; font-weight: 600; }.rf-frame-admin-form fieldset label { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; }.rf-frame-admin-form footer { display: flex; grid-column: 2; justify-content: flex-end; gap: 8px; }.rf-frame-admin-form footer button { display: inline-flex; align-items: center; gap: 5px; }
.rf-admin-frame-list article { display: grid; grid-template-columns: 56px minmax(0, 1fr) auto auto; align-items: center; gap: 14px; padding: 14px 20px; border-bottom: 1px solid var(--rf-line); }.rf-admin-frame-list article.disabled { opacity: .58; }.rf-admin-frame-list article > div:nth-child(2) { display: flex; min-width: 0; flex-direction: column; gap: 3px; }.rf-admin-frame-list article > div:nth-child(2) strong, .rf-admin-frame-list article > div:nth-child(2) span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-admin-frame-list article > div:nth-child(2) span, .rf-admin-frame-list article small { color: var(--rf-muted); font-size: 11px; }.rf-admin-frame-tags { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }.rf-admin-frame-tags span, .rf-admin-frame-tags b { padding: 3px 6px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); font-size: 10px; font-weight: 600; }.rf-admin-frame-tags b { color: var(--primary); }.rf-admin-frame-list footer { display: flex; gap: 6px; }
@media (max-width: 760px) { .rf-frame-admin-form { grid-template-columns: 1fr; padding: 16px 12px; }.rf-frame-admin-preview { min-height: 108px; }.rf-frame-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }.rf-frame-fields .rf-frame-description { grid-column: 1 / -1; }.rf-frame-admin-form fieldset, .rf-frame-admin-form footer { grid-column: 1; }.rf-frame-admin-form footer button { flex: 1; justify-content: center; }.rf-admin-frame-list article { grid-template-columns: 52px minmax(0, 1fr); gap: 10px; padding-inline: 12px; }.rf-admin-frame-tags, .rf-admin-frame-list article footer { grid-column: 2; justify-content: flex-start; } }
</style>
