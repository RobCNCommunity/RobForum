<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchMe, fetchMembership, subscribeMembership, type MembershipSummary, type MembershipTier } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMembershipStore } from '@/stores/membership'
import AppIcon from '@/components/AppIcon.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import PageContainer from '@/components/PageContainer.vue'

type PlanKey = 'monthly' | 'quarterly' | 'yearly'
type PurchaseOption = { tier: MembershipTier; plan: PlanKey; title: string; months: number; price: number }

const router = useRouter()
const auth = useAuthStore()
const membershipStore = useMembershipStore()
const loading = ref(true)
const summary = ref<MembershipSummary | null>(null)
const pending = ref<PurchaseOption | null>(null)
const buying = ref(false)

const visibleTiers = computed(() => (summary.value?.config.tiers || []).filter((tier) => tier.enabled))
const currentPolicy = computed(() => {
  if (summary.value?.active && summary.value.current_tier) return summary.value.current_tier
  return {
    withdrawal_fee_bps: summary.value?.config.default_withdrawal_fee_bps || 300,
    service_fee_bps: summary.value?.config.default_service_fee_bps || 500,
  }
})
const money = (cents: number) => `¥${(Number(cents || 0) / 100).toFixed(2)}`
const percent = (bps: number) => `${(Number(bps || 0) / 100).toFixed(2).replace(/\.00$/, '')}%`
const formatDate = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const planLabel = (value: string) => ({ monthly: '月度', quarterly: '季度', yearly: '年度' } as Record<string, string>)[value] || value

function plansFor(tier: MembershipTier): PurchaseOption[] {
  return [
    { tier, plan: 'monthly', title: '月度', months: 1, price: tier.monthly_price_cents },
    { tier, plan: 'quarterly', title: '季度', months: 3, price: tier.quarterly_price_cents },
    { tier, plan: 'yearly', title: '年度', months: 12, price: tier.yearly_price_cents },
  ].filter((item) => item.price > 0)
}

async function load() {
  loading.value = true
  try {
    summary.value = await fetchMembership()
    membershipStore.setConfig(summary.value.config)
  } catch (error) {
    Notify.danger(errorMessage(error, '会员信息加载失败'))
  } finally {
    loading.value = false
  }
}

function choosePlan(option: PurchaseOption) {
  if (!summary.value?.config.enabled) {
    Notify.warn('会员暂未开放购买')
    return
  }
  if (summary.value.active && summary.value.current_tier?.id !== option.tier.id) {
    Notify.warn('当前会员未到期，只能续费同一阶层')
    return
  }
  if ((summary.value?.available_cents || 0) < option.price) {
    Notify.warn('余额不足，请先充值')
    router.push('/wallet')
    return
  }
  pending.value = option
}

async function confirmPurchase() {
  if (!pending.value) return
  buying.value = true
  try {
    const updated = await subscribeMembership(pending.value.tier.id, pending.value.plan)
    summary.value = updated
    membershipStore.setConfig(updated.config)
    auth.setUser(await fetchMe())
    pending.value = null
    Notify.success(`会员已开通，有效期至 ${formatDate(updated.expires_at)}`)
  } catch (error) {
    Notify.danger(errorMessage(error, '会员购买失败'))
  } finally {
    buying.value = false
  }
}

onMounted(load)
</script>

<template>
  <PageContainer title="会员中心">
    <div v-if="loading" class="rf-loading-block">正在加载会员信息…</div>
    <div v-else-if="summary" class="rf-membership-page">
      <section class="rf-membership-status">
        <div class="rf-membership-mark" :style="{ color: summary.current_tier?.badge_color || 'var(--primary)' }">
          <img v-if="summary.current_tier?.badge_url" :src="summary.current_tier.badge_url" alt="" />
          <AppIcon v-else name="crown" size="29" :stroke-width="2" />
        </div>
        <div class="rf-membership-status-copy">
          <span>{{ summary.active ? '当前身份' : '当前身份：普通用户' }}</span>
          <h2 v-if="summary.active && summary.current_tier">
            {{ summary.current_tier.name }}
            <MembershipBadge active :tier-id="summary.current_tier.id" />
          </h2>
          <h2 v-else>选择适合你的会员阶层</h2>
          <p v-if="summary.active">有效期至 {{ formatDate(summary.expires_at) }}。同阶层续费会从当前到期时间继续累加。</p>
          <p v-else>{{ summary.config.enabled ? '使用钱包余额即时开通，价格与权益由每个阶层单独配置。' : '管理员暂未开放会员购买。' }}</p>
        </div>
        <div class="rf-membership-balance">
          <span>钱包余额</span>
          <strong>{{ money(summary.available_cents) }}</strong>
          <RouterLink to="/wallet">充值与兑换 <AppIcon name="arrow" size="15" /></RouterLink>
        </div>
      </section>

      <section class="rf-membership-benefits" aria-label="当前费率">
        <div><AppIcon name="payout" size="19" /><span><strong>提现手续费 {{ percent(currentPolicy.withdrawal_fee_bps) }}</strong><small>提交申请时锁定费率和实付金额</small></span></div>
        <div><AppIcon name="wallet" size="19" /><span><strong>资源服务费 {{ percent(currentPolicy.service_fee_bps) }}</strong><small>资源成交时按创作者当前阶层计算</small></span></div>
        <div><AppIcon name="document" size="19" /><span><strong>{{ summary.current_tier?.post_review_exempt ? '发帖免人工审核' : '遵循普通审核流程' }}</strong><small>资源投稿始终进入人工审核队列</small></span></div>
      </section>

      <section class="rf-membership-section">
        <header><div><h2>会员阶层</h2><p>每个阶层有独立徽标、价格、费率和推流权益。</p></div></header>
        <div v-if="summary.config.enabled && visibleTiers.length" class="rf-membership-tier-list">
          <article v-for="tier in visibleTiers" :key="tier.id" class="rf-membership-tier">
            <div class="rf-membership-tier-head">
              <span class="rf-membership-tier-badge" :style="{ color: tier.badge_color }"><img v-if="tier.badge_url" :src="tier.badge_url" alt="" /><AppIcon v-else name="crown" size="20" /></span>
              <span><strong>{{ tier.name }} <MembershipBadge active :tier-id="tier.id" /></strong><small>{{ tier.badge_label }} · 提现 {{ percent(tier.withdrawal_fee_bps) }} · 服务费 {{ percent(tier.service_fee_bps) }}</small></span>
              <span class="rf-membership-tier-rights"><em v-if="tier.feed_priority">信息流优先</em><em v-if="tier.post_review_exempt">发帖免审</em></span>
            </div>
            <div class="rf-membership-tier-plans">
              <button v-for="option in plansFor(tier)" :key="option.plan" type="button" :disabled="buying" @click="choosePlan(option)">
                <span>{{ option.title }} <small>{{ option.months }} 个月</small></span><strong>{{ money(option.price) }}</strong><AppIcon name="chevron" size="17" />
              </button>
            </div>
          </article>
        </div>
        <div v-else class="rf-empty">当前没有可购买的会员阶层</div>
      </section>

      <section class="rf-membership-section">
        <header><div><h2>购买记录</h2><p>订单会保留购买时的阶层名称、价格和有效期。</p></div><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></header>
        <div v-if="summary.orders.length" class="rf-membership-orders">
          <article v-for="order in summary.orders" :key="order.id">
            <div><strong>{{ order.tier_name || '会员' }} · {{ planLabel(order.plan) }}</strong><small>{{ order.order_no }}</small></div>
            <b>-{{ money(order.amount_cents) }}</b>
            <span>{{ formatDate(order.started_at) }} 至 {{ formatDate(order.expires_at) }}</span>
          </article>
        </div>
        <div v-else class="rf-empty">暂无会员购买记录</div>
      </section>
    </div>

    <div v-if="pending" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="!buying && (pending = null)">
      <section class="rf-dialog rf-membership-confirm" role="dialog" aria-modal="true" aria-labelledby="membership-confirm-title">
        <header><div><h2 id="membership-confirm-title">确认余额支付</h2><p>{{ pending.tier.name }} · {{ pending.title }} · {{ pending.months }} 个月</p></div><button type="button" class="rf-icon-button" aria-label="关闭" :disabled="buying" @click="pending = null"><AppIcon name="close" size="18" /></button></header>
        <div class="rf-membership-confirm-body"><span>本次支付</span><strong>{{ money(pending.price) }}</strong><small>支付后余额 {{ money(summary.available_cents - pending.price) }}</small></div>
        <footer><button type="button" class="rf-secondary-button" :disabled="buying" @click="pending = null">取消</button><nut-button type="primary" :loading="buying" @click="confirmPurchase">确认支付</nut-button></footer>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-membership-page { min-width: 0; }
.rf-membership-status { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 18px; padding: 28px 20px; border-bottom: 1px solid var(--rf-line); }
.rf-membership-mark { display: grid; width: 58px; height: 58px; place-items: center; border: 1px solid color-mix(in srgb, currentColor 28%, var(--rf-line)); border-radius: 16px; background: color-mix(in srgb, currentColor 8%, var(--rf-bg)); }
.rf-membership-mark img, .rf-membership-tier-badge img { width: 38px; height: 38px; object-fit: contain; }
.rf-membership-status-copy { min-width: 0; }
.rf-membership-status-copy > span, .rf-membership-balance > span { color: var(--rf-muted); font-size: 12px; }
.rf-membership-status-copy h2 { display: flex; align-items: center; gap: 5px; margin: 3px 0 0; font-size: 24px; letter-spacing: -.02em; }
.rf-membership-status-copy p { margin: 7px 0 0; color: var(--rf-muted); font-size: 13px; line-height: 1.55; }
.rf-membership-balance { display: flex; min-width: 138px; flex-direction: column; align-items: flex-end; }
.rf-membership-balance strong { margin-top: 3px; font-size: 23px; font-variant-numeric: tabular-nums; }
.rf-membership-balance a { display: inline-flex; align-items: center; gap: 3px; margin-top: 6px; color: var(--primary); font-size: 12px; font-weight: 700; }
.rf-membership-benefits { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-bottom: 1px solid var(--rf-line); }
.rf-membership-benefits > div { display: flex; min-width: 0; align-items: center; gap: 10px; padding: 18px 20px; }
.rf-membership-benefits > div + div { border-left: 1px solid var(--rf-line); }
.rf-membership-benefits svg { flex: 0 0 auto; color: var(--primary); }
.rf-membership-benefits span { display: flex; min-width: 0; flex-direction: column; }
.rf-membership-benefits strong { font-size: 13px; }
.rf-membership-benefits small { margin-top: 2px; overflow: hidden; color: var(--rf-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.rf-membership-section { border-bottom: 1px solid var(--rf-line); }
.rf-membership-section > header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 20px 20px 13px; }
.rf-membership-section h2 { margin: 0; font-size: 18px; }
.rf-membership-section header p { margin: 4px 0 0; color: var(--rf-muted); font-size: 12px; }
.rf-membership-tier-list, .rf-membership-orders { border-top: 1px solid var(--rf-line); }
.rf-membership-tier { border-bottom: 1px solid var(--rf-line); }
.rf-membership-tier:last-child { border-bottom: 0; }
.rf-membership-tier-head { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 17px 20px 12px; }
.rf-membership-tier-badge { display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid color-mix(in srgb, currentColor 25%, var(--rf-line)); border-radius: 12px; background: color-mix(in srgb, currentColor 7%, var(--rf-bg)); }
.rf-membership-tier-badge img { width: 27px; height: 27px; }
.rf-membership-tier-head > span:nth-child(2) { display: flex; min-width: 0; flex-direction: column; }
.rf-membership-tier-head strong { display: flex; align-items: center; gap: 4px; font-size: 15px; }
.rf-membership-tier-head small { margin-top: 3px; color: var(--rf-muted); font-size: 11px; }
.rf-membership-tier-rights { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }
.rf-membership-tier-rights em { border: 1px solid var(--rf-line); border-radius: 999px; padding: 3px 7px; color: var(--rf-muted); font-size: 10px; font-style: normal; }
.rf-membership-tier-plans { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-top: 1px solid var(--rf-line); }
.rf-membership-tier-plans button { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 8px; min-height: 58px; border: 0; background: transparent; padding: 10px 16px; color: var(--rf-text); text-align: left; cursor: pointer; }
.rf-membership-tier-plans button + button { border-left: 1px solid var(--rf-line); }
.rf-membership-tier-plans button:hover { background: var(--rf-hover); }
.rf-membership-tier-plans button:disabled { cursor: not-allowed; opacity: .55; }
.rf-membership-tier-plans button span { display: flex; flex-direction: column; font-size: 13px; font-weight: 700; }
.rf-membership-tier-plans button small { color: var(--rf-muted); font-size: 10px; font-weight: 400; }
.rf-membership-tier-plans button strong { font-size: 14px; font-variant-numeric: tabular-nums; }
.rf-membership-orders article { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(190px, auto); align-items: center; gap: 15px; min-height: 66px; padding: 10px 20px; border-bottom: 1px solid var(--rf-line); }
.rf-membership-orders article:last-child { border-bottom: 0; }
.rf-membership-orders article > div { display: flex; min-width: 0; flex-direction: column; }
.rf-membership-orders small, .rf-membership-orders span { color: var(--rf-muted); font-size: 11px; }
.rf-membership-orders article > b { color: var(--rf-danger); font-size: 16px; font-variant-numeric: tabular-nums; }
.rf-membership-orders article > span { text-align: right; }
.rf-membership-confirm-body { display: flex; flex-direction: column; align-items: center; padding: 24px 20px 20px; }
.rf-membership-confirm-body span, .rf-membership-confirm-body small { color: var(--rf-muted); font-size: 12px; }
.rf-membership-confirm-body strong { margin: 7px 0; font-size: 34px; font-variant-numeric: tabular-nums; }
.rf-membership-confirm footer { display: flex; justify-content: flex-end; gap: 8px; padding: 0 20px 20px; }
@media (max-width: 640px) {
  .rf-membership-status { grid-template-columns: auto minmax(0, 1fr); gap: 13px; padding: 22px 12px; }
  .rf-membership-mark { width: 50px; height: 50px; border-radius: 14px; }
  .rf-membership-status-copy h2 { font-size: 20px; }
  .rf-membership-balance { grid-column: 1 / -1; align-items: flex-start; padding-top: 14px; border-top: 1px solid var(--rf-line); }
  .rf-membership-benefits { grid-template-columns: 1fr; }
  .rf-membership-benefits > div { padding: 14px 12px; }
  .rf-membership-benefits > div + div { border-top: 1px solid var(--rf-line); border-left: 0; }
  .rf-membership-section > header { padding-inline: 12px; }
  .rf-membership-tier-head { grid-template-columns: auto minmax(0, 1fr); padding-inline: 12px; }
  .rf-membership-tier-rights { grid-column: 1 / -1; justify-content: flex-start; }
  .rf-membership-tier-plans { grid-template-columns: 1fr; }
  .rf-membership-tier-plans button { padding-inline: 12px; }
  .rf-membership-tier-plans button + button { border-top: 1px solid var(--rf-line); border-left: 0; }
  .rf-membership-orders article { grid-template-columns: minmax(0, 1fr) auto; padding-inline: 12px; }
  .rf-membership-orders article > span { grid-column: 1 / -1; text-align: left; }
  .rf-membership-confirm footer { padding: 0 14px calc(18px + env(safe-area-inset-bottom)); }
}
</style>
