<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { equipAvatarFrame, errorMessage, fetchAvatarFrames, fetchMe, fetchWallet, purchaseAvatarFrame, type AvatarFrame } from '@/api'
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
    const [items, wallet] = await Promise.all([fetchAvatarFrames(), fetchWallet()])
    frames.value = items
    balance.value = wallet.available_cents
  } catch (error) {
    Notify.danger(errorMessage(error, '头像框市场加载失败'))
  } finally {
    loading.value = false
  }
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
.rf-frame-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.rf-frame-card { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 14px; padding: 20px; border-right: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); }.rf-frame-card:nth-child(2n) { border-right: 0; }.rf-frame-card.is-equipped { background: color-mix(in srgb, var(--primary) 5%, var(--rf-bg)); }
.rf-frame-preview { display: grid; min-height: 92px; place-items: center; border-radius: 8px; background: var(--rf-bg-subtle); }.rf-frame-main { min-width: 0; }.rf-frame-title { display: flex; min-width: 0; align-items: center; gap: 7px; }.rf-frame-title h2 { min-width: 0; margin: 0; overflow: hidden; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }.rf-frame-title > span { flex: 0 0 auto; padding: 2px 6px; border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); font-size: 10px; font-weight: 700; }.rf-frame-main p { min-height: 36px; margin: 5px 0 8px; color: var(--rf-muted); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }.rf-frame-main > small { display: block; margin-top: 8px; color: var(--rf-faint); font-size: 11px; }.rf-frame-audience { display: flex; flex-wrap: wrap; gap: 5px; }.rf-frame-audience span { padding: 2px 6px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); font-size: 10px; }
.rf-frame-card footer { display: flex; grid-column: 1 / -1; align-items: center; justify-content: space-between; gap: 10px; }.rf-frame-card footer > strong { font-size: 16px; }.rf-frame-card footer button { display: inline-flex; min-width: 98px; align-items: center; justify-content: center; gap: 5px; }.rf-frame-current { display: flex; align-items: center; gap: 7px; padding: 13px 20px; color: var(--primary); font-size: 12px; }
@media (max-width: 680px) { .rf-frame-market-head { padding: 16px 12px; }.rf-frame-grid { grid-template-columns: minmax(0, 1fr); }.rf-frame-card, .rf-frame-card:nth-child(2n) { padding: 16px 12px; border-right: 0; }.rf-frame-card { grid-template-columns: 82px minmax(0, 1fr); }.rf-frame-preview { min-height: 82px; }.rf-frame-card footer button { min-width: 106px; }.rf-frame-current { padding-inline: 12px; } }
</style>
