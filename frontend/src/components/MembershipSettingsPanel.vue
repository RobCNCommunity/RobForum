<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  createMembershipTier,
  deleteMembershipTier,
  deleteMembershipTierBadge,
  errorMessage,
  fetchAdminMembership,
  updateAdminMembership,
  updateMembershipTier,
  type MembershipSettings,
  type MembershipTier,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import { useMembershipStore } from '@/stores/membership'

type PriceKey = 'monthly_price_cents' | 'quarterly_price_cents' | 'yearly_price_cents'
type FeeKey = 'withdrawal_fee_bps' | 'service_fee_bps'

const membershipStore = useMembershipStore()
const loading = ref(true)
const savingGlobal = ref(false)
const creating = ref(false)
const newTierOpen = ref(false)
const savingTierIDs = ref<number[]>([])
const membership = reactive<MembershipSettings>({
  enabled: false,
  default_withdrawal_fee_bps: 300,
  default_service_fee_bps: 500,
  tiers: [],
  updated_at: '',
})
const badgeFiles = reactive<Record<number, any[]>>({})
const uploadHeaders = computed(() => ({ 'X-CSRF-Token': readCookie('roblox_csrf') }))
const draft = reactive<MembershipTier>(blankTier())

function blankTier(): MembershipTier {
  return {
    id: 0,
    enabled: true,
    name: '',
    badge_label: '',
    badge_url: '',
    badge_color: '#f59e0b',
    monthly_price_cents: 0,
    quarterly_price_cents: 0,
    yearly_price_cents: 0,
    post_review_exempt: false,
    feed_priority: false,
    withdrawal_fee_bps: 300,
    service_fee_bps: 500,
    sort_order: 10,
    created_at: '',
    updated_at: '',
  }
}

function readCookie(name: string) {
  if (typeof document === 'undefined') return ''
  const value = document.cookie.split('; ').find((item) => item.startsWith(`${name}=`))
  return value ? decodeURIComponent(value.slice(name.length + 1)) : ''
}

function syncBadgeFiles() {
  for (const key of Object.keys(badgeFiles)) delete badgeFiles[Number(key)]
  for (const tier of membership.tiers) {
    badgeFiles[tier.id] = tier.badge_url ? [{ uid: `membership-tier-${tier.id}`, name: `${tier.name}徽标`, url: tier.badge_url, status: 'success', type: 'image/png' }] : []
  }
}

async function load() {
  loading.value = true
  try {
    const result = await fetchAdminMembership()
    Object.assign(membership, result, { tiers: result.tiers.map((tier) => ({ ...tier })) })
    membershipStore.setConfig(result)
    syncBadgeFiles()
  } catch (error) {
    Notify.danger(errorMessage(error, '会员设置加载失败'))
  } finally {
    loading.value = false
  }
}

function setGlobalFee(key: 'default_withdrawal_fee_bps' | 'default_service_fee_bps', event: Event) {
  membership[key] = percentToBPS((event.target as HTMLInputElement).value)
}

function setTierFee(tier: MembershipTier, key: FeeKey, event: Event) {
  tier[key] = percentToBPS((event.target as HTMLInputElement).value)
}

function setTierPrice(tier: MembershipTier, key: PriceKey, event: Event) {
  const amount = Number((event.target as HTMLInputElement).value || 0)
  tier[key] = Number.isFinite(amount) ? Math.max(0, Math.min(1_000_000, Math.round(amount * 100))) : 0
}

function percentToBPS(value: string) {
  const percent = Number(value || 0)
  return Number.isFinite(percent) ? Math.max(0, Math.min(10_000, Math.round(percent * 100))) : 0
}

const percentValue = (bps: number) => Number((Number(bps || 0) / 100).toFixed(2))
const priceValue = (cents: number) => Number((Number(cents || 0) / 100).toFixed(2))

async function saveGlobal() {
  savingGlobal.value = true
  try {
    const updated = await updateAdminMembership({
      enabled: membership.enabled,
      default_withdrawal_fee_bps: membership.default_withdrawal_fee_bps,
      default_service_fee_bps: membership.default_service_fee_bps,
      tiers: [],
      updated_at: membership.updated_at,
    })
    Object.assign(membership, updated, { tiers: updated.tiers.map((tier) => ({ ...tier })) })
    membershipStore.setConfig(updated)
    syncBadgeFiles()
    Notify.success('会员全局设置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, '会员全局设置保存失败'))
  } finally {
    savingGlobal.value = false
  }
}

async function saveTier(tier: MembershipTier) {
  if (!tier.name.trim() || !tier.badge_label.trim()) {
    Notify.warn('阶层名称和徽标提示文字不能为空')
    return
  }
  savingTierIDs.value = [...savingTierIDs.value, tier.id]
  try {
    const updated = await updateMembershipTier(tier.id, { ...tier })
    const index = membership.tiers.findIndex((item) => item.id === tier.id)
    if (index >= 0) membership.tiers[index] = updated
    membershipStore.setConfig({ ...membership, tiers: membership.tiers.map((item) => ({ ...item })) })
    Notify.success(`${updated.name} 已保存`)
  } catch (error) {
    Notify.danger(errorMessage(error, '会员阶层保存失败'))
  } finally {
    savingTierIDs.value = savingTierIDs.value.filter((id) => id !== tier.id)
  }
}

async function createTier() {
  if (!draft.name.trim() || !draft.badge_label.trim()) {
    Notify.warn('请填写阶层名称和徽标提示文字')
    return
  }
  creating.value = true
  try {
    const created = await createMembershipTier({
      enabled: draft.enabled,
      name: draft.name,
      badge_label: draft.badge_label,
      badge_url: '',
      badge_color: draft.badge_color,
      monthly_price_cents: draft.monthly_price_cents,
      quarterly_price_cents: draft.quarterly_price_cents,
      yearly_price_cents: draft.yearly_price_cents,
      post_review_exempt: draft.post_review_exempt,
      feed_priority: draft.feed_priority,
      withdrawal_fee_bps: draft.withdrawal_fee_bps,
      service_fee_bps: draft.service_fee_bps,
      sort_order: draft.sort_order,
    })
    membership.tiers.push(created)
    membership.tiers.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
    badgeFiles[created.id] = []
    Object.assign(draft, blankTier())
    newTierOpen.value = false
    membershipStore.setConfig({ ...membership, tiers: membership.tiers.map((item) => ({ ...item })) })
    Notify.success('会员阶层已创建')
  } catch (error) {
    Notify.danger(errorMessage(error, '会员阶层创建失败'))
  } finally {
    creating.value = false
  }
}

async function removeTier(tier: MembershipTier) {
  if (!window.confirm(`确定删除“${tier.name}”吗？有有效会员时系统会拒绝删除。`)) return
  try {
    await deleteMembershipTier(tier.id)
    membership.tiers = membership.tiers.filter((item) => item.id !== tier.id)
    delete badgeFiles[tier.id]
    membershipStore.setConfig({ ...membership, tiers: membership.tiers.map((item) => ({ ...item })) })
    Notify.success('会员阶层已删除')
  } catch (error) {
    Notify.danger(errorMessage(error, '会员阶层删除失败'))
  }
}

function parseTierUpload(payload: any): MembershipTier {
  const parsed = typeof payload?.responseText === 'string' ? JSON.parse(payload.responseText) : payload
  const tier = parsed?.data
  if (!tier?.id) throw new Error('会员徽标上传响应无效')
  return tier
}

function handleTierBadgeSuccess(payload: any) {
  try {
    const updated = parseTierUpload(payload)
    const index = membership.tiers.findIndex((item) => item.id === updated.id)
    if (index >= 0) membership.tiers[index] = updated
    membershipStore.setConfig({ ...membership, tiers: membership.tiers.map((item) => ({ ...item })) })
    Notify.success('会员徽标已上传')
  } catch (error) {
    Notify.danger(errorMessage(error, '会员徽标上传响应无效'))
  }
}

function handleTierBadgeFailure(payload: any) {
  let message = '会员徽标上传失败'
  try {
    const parsed = typeof payload?.responseText === 'string' ? JSON.parse(payload.responseText) : payload
    message = parsed?.error?.message || parsed?.data?.error?.message || message
  } catch { /* keep safe fallback */ }
  Notify.danger(message)
}

async function clearTierBadge(tier: MembershipTier) {
  if (!tier.badge_url) return
  try {
    const updated = await deleteMembershipTierBadge(tier.id)
    const index = membership.tiers.findIndex((item) => item.id === tier.id)
    if (index >= 0) membership.tiers[index] = updated
    badgeFiles[tier.id] = []
    membershipStore.setConfig({ ...membership, tiers: membership.tiers.map((item) => ({ ...item })) })
    Notify.success('会员徽标已移除')
  } catch (error) {
    Notify.danger(errorMessage(error, '会员徽标移除失败'))
  }
}

onMounted(load)
</script>

<template>
  <section class="setting-card wide-card rf-membership-admin">
    <header class="rf-panel-heading">
      <div><h3><AppIcon name="crown" size="18" />会员身份组</h3><p>创建多个阶层，并分别设置徽标、价格、推流权益和交易费率。</p></div>
      <span class="rf-status-chip" :class="membership.enabled ? 'is-on' : ''">{{ membership.enabled ? '开放购买' : '暂停购买' }}</span>
    </header>

    <div v-if="loading" class="rf-settings-loading"><span v-for="n in 3" :key="n" /></div>
    <template v-else>
      <form class="rf-editor-form rf-membership-global" @submit.prevent="saveGlobal">
        <div class="rf-form-section"><div><strong>普通用户费率</strong><p>无有效会员时使用。300 基点等于 3%，500 基点等于 5%。</p></div><label class="rf-switch-row"><input v-model="membership.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>允许余额购买会员</span></label></div>
        <div class="rf-form-grid rf-form-grid--two">
          <label class="rf-field-label"><span>普通用户提现手续费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(membership.default_withdrawal_fee_bps)" @input="setGlobalFee('default_withdrawal_fee_bps', $event)" /></label>
          <label class="rf-field-label"><span>普通用户资源服务费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(membership.default_service_fee_bps)" @input="setGlobalFee('default_service_fee_bps', $event)" /></label>
        </div>
        <div class="rf-inline-note rf-inline-note--info"><AppIcon name="wallet" size="16" /><span>资源服务费在成交事务中扣除；提现手续费在提交申请时计算。订单和提现记录都会保存当时的费率快照。</span></div>
        <div class="rf-form-actions"><nut-button type="primary" :loading="savingGlobal" @click="saveGlobal">保存全局设置</nut-button></div>
      </form>

      <div class="rf-membership-tier-toolbar"><div><strong>已配置阶层</strong><small>{{ membership.tiers.length }} 个</small></div><button type="button" class="rf-secondary-button" @click="newTierOpen = !newTierOpen"><AppIcon :name="newTierOpen ? 'close' : 'plus'" size="16" />{{ newTierOpen ? '取消新增' : '新增阶层' }}</button></div>

      <form v-if="newTierOpen" class="rf-membership-tier-editor is-new" @submit.prevent="createTier">
        <div class="rf-membership-tier-title"><span class="rf-membership-tier-preview" :style="{ color: draft.badge_color }"><AppIcon name="crown" size="20" /></span><div><strong>新会员阶层</strong><small>创建后即可上传自定义徽标</small></div></div>
        <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>阶层名称</span><input v-model="draft.name" class="rf-control" maxlength="80" placeholder="例如：RobForum Pro" /></label><label class="rf-field-label"><span>徽标提示文字</span><input v-model="draft.badge_label" class="rf-control" maxlength="40" placeholder="例如：高级会员" /></label></div>
        <div class="rf-form-grid rf-form-grid--three"><label class="rf-field-label"><span>月卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(draft.monthly_price_cents)" @input="setTierPrice(draft, 'monthly_price_cents', $event)" /></label><label class="rf-field-label"><span>季卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(draft.quarterly_price_cents)" @input="setTierPrice(draft, 'quarterly_price_cents', $event)" /></label><label class="rf-field-label"><span>年卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(draft.yearly_price_cents)" @input="setTierPrice(draft, 'yearly_price_cents', $event)" /></label></div>
        <div class="rf-form-grid rf-form-grid--three"><label class="rf-field-label"><span>提现手续费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(draft.withdrawal_fee_bps)" @input="setTierFee(draft, 'withdrawal_fee_bps', $event)" /></label><label class="rf-field-label"><span>资源服务费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(draft.service_fee_bps)" @input="setTierFee(draft, 'service_fee_bps', $event)" /></label><label class="rf-field-label"><span>排序</span><input v-model.number="draft.sort_order" class="rf-control" type="number" min="-10000" max="10000" /></label></div>
        <div class="rf-choice-row"><label class="rf-switch-row"><input v-model="draft.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>允许购买</span></label><label class="rf-switch-row"><input v-model="draft.feed_priority" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>信息流优先</span></label><label class="rf-switch-row"><input v-model="draft.post_review_exempt" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>发帖免审</span></label></div>
        <div class="rf-form-actions"><nut-button type="primary" :loading="creating" @click="createTier">创建阶层</nut-button></div>
      </form>

      <div v-if="membership.tiers.length" class="rf-membership-admin-list">
        <form v-for="tier in membership.tiers" :key="tier.id" class="rf-membership-tier-editor" @submit.prevent="saveTier(tier)">
          <div class="rf-membership-tier-title">
            <span class="rf-membership-tier-preview" :style="{ color: tier.badge_color }"><img v-if="tier.badge_url" :src="tier.badge_url" alt="" /><AppIcon v-else name="crown" size="20" /></span>
            <div><strong>{{ tier.name || '未命名阶层' }} <MembershipBadge active :tier-id="tier.id" /></strong><small>ID {{ tier.id }} · 排序 {{ tier.sort_order }}</small></div>
            <span class="rf-status-chip" :class="tier.enabled ? 'is-on' : ''">{{ tier.enabled ? '可购买' : '已停用' }}</span>
          </div>
          <div class="rf-field-label rf-verification-upload"><span>自定义徽标</span><small>透明 PNG 会保留透明背景；支持 PNG、JPG，最大 2 MB。</small><nut-uploader v-model:file-list="badgeFiles[tier.id]" :url="`/api/v1/admin/membership/tiers/${tier.id}/badge`" accept="image/png,image/jpeg" list-type="list" maximum="1" :maximize="2 * 1024 * 1024" :headers="uploadHeaders" with-credentials @success="handleTierBadgeSuccess" @failure="handleTierBadgeFailure" @oversize="Notify.warn('会员徽标不能超过 2 MB')" @delete="clearTierBadge(tier)"><nut-button type="primary" size="small" native-type="button">上传徽标</nut-button></nut-uploader></div>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>阶层名称</span><input v-model="tier.name" class="rf-control" maxlength="80" /></label><label class="rf-field-label"><span>徽标提示文字</span><input v-model="tier.badge_label" class="rf-control" maxlength="40" /></label></div>
          <label class="rf-field-label"><span>徽标备用颜色</span><span class="rf-color-control"><input v-model="tier.badge_color" class="rf-control" placeholder="#f59e0b" /><input v-model="tier.badge_color" type="color" aria-label="选择会员徽标颜色" /></span></label>
          <div class="rf-form-grid rf-form-grid--three"><label class="rf-field-label"><span>月卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(tier.monthly_price_cents)" @input="setTierPrice(tier, 'monthly_price_cents', $event)" /></label><label class="rf-field-label"><span>季卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(tier.quarterly_price_cents)" @input="setTierPrice(tier, 'quarterly_price_cents', $event)" /></label><label class="rf-field-label"><span>年卡（元）</span><input class="rf-control" type="number" min="0" max="10000" step="0.01" :value="priceValue(tier.yearly_price_cents)" @input="setTierPrice(tier, 'yearly_price_cents', $event)" /></label></div>
          <div class="rf-form-grid rf-form-grid--three"><label class="rf-field-label"><span>提现手续费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(tier.withdrawal_fee_bps)" @input="setTierFee(tier, 'withdrawal_fee_bps', $event)" /></label><label class="rf-field-label"><span>资源服务费（%）</span><input class="rf-control" type="number" min="0" max="100" step="0.01" :value="percentValue(tier.service_fee_bps)" @input="setTierFee(tier, 'service_fee_bps', $event)" /></label><label class="rf-field-label"><span>排序</span><input v-model.number="tier.sort_order" class="rf-control" type="number" min="-10000" max="10000" /></label></div>
          <div class="rf-choice-row"><label class="rf-switch-row"><input v-model="tier.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>允许购买</span></label><label class="rf-switch-row"><input v-model="tier.feed_priority" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>信息流优先</span></label><label class="rf-switch-row"><input v-model="tier.post_review_exempt" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>发帖免审</span></label></div>
          <div class="rf-form-actions rf-membership-tier-actions"><button type="button" class="rf-text-button rf-danger-text" @click="removeTier(tier)">删除阶层</button><nut-button type="primary" :loading="savingTierIDs.includes(tier.id)" @click="saveTier(tier)">保存阶层</nut-button></div>
        </form>
      </div>
      <div v-else class="rf-empty">还没有会员阶层，请先新增一个。</div>
    </template>
  </section>
</template>

<style scoped>
.rf-membership-global { border-bottom: 1px solid var(--rf-line); }
.rf-membership-tier-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 20px; border-bottom: 1px solid var(--rf-line); }
.rf-membership-tier-toolbar > div { display: flex; flex-direction: column; }
.rf-membership-tier-toolbar strong { font-size: 14px; }
.rf-membership-tier-toolbar small { margin-top: 2px; color: var(--rf-muted); font-size: 11px; }
.rf-membership-admin-list { min-width: 0; }
.rf-membership-tier-editor { padding: 20px; border-bottom: 1px solid var(--rf-line); }
.rf-membership-tier-editor:last-child { border-bottom: 0; }
.rf-membership-tier-editor.is-new { background: color-mix(in srgb, var(--primary) 4%, var(--rf-bg)); }
.rf-membership-tier-title { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 11px; margin-bottom: 17px; }
.rf-membership-tier-title > div { display: flex; min-width: 0; flex-direction: column; }
.rf-membership-tier-title strong { display: flex; align-items: center; gap: 4px; font-size: 15px; }
.rf-membership-tier-title small { margin-top: 2px; color: var(--rf-muted); font-size: 11px; }
.rf-membership-tier-preview { display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid color-mix(in srgb, currentColor 25%, var(--rf-line)); border-radius: 12px; background: color-mix(in srgb, currentColor 7%, var(--rf-bg)); }
.rf-membership-tier-preview img { width: 27px; height: 27px; object-fit: contain; }
.rf-membership-tier-actions { justify-content: space-between; }
.rf-danger-text { color: var(--rf-danger); }
@media (max-width: 640px) {
  .rf-membership-tier-toolbar, .rf-membership-tier-editor { padding-inline: 12px; }
  .rf-membership-tier-title { grid-template-columns: auto minmax(0, 1fr); }
  .rf-membership-tier-title > .rf-status-chip { grid-column: 1 / -1; justify-self: start; }
}
</style>
