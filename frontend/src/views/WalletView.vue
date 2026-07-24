<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  createWalletTopUp,
  errorMessage,
  fetchCreatorPayouts,
  fetchWallet,
  redeemWalletCode,
  requestCreatorPayout,
  type CreatorPayout,
  type WalletSummary,
} from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'

const loading = ref(true)
const wallet = ref<WalletSummary | null>(null)
const payouts = ref<CreatorPayout[]>([])
const topUpYuan = ref<number | null>(10)
const topUpLoading = ref(false)
const redeemCode = ref('')
const redeeming = ref(false)
const withdrawOpen = ref(false)
const withdrawing = ref(false)
const payoutForm = reactive({ amount_yuan: 0, payout_method: 'alipay', payout_account: '', account_name: '', note: '' })

const topUpPresets = [5, 10, 30, 50, 100]
const availableCents = computed(() => wallet.value?.available_cents || 0)
const withdrawableCents = computed(() => wallet.value?.withdrawable_cents || 0)
const withdrawalFeeBPS = computed(() => wallet.value?.fee_policy?.withdrawal_fee_bps ?? 300)
const serviceFeeBPS = computed(() => wallet.value?.fee_policy?.service_fee_bps ?? 500)
const requestedPayoutCents = computed(() => Math.max(0, Math.round(Number(payoutForm.amount_yuan || 0) * 100)))
const estimatedWithdrawalFee = computed(() => Math.min(requestedPayoutCents.value, Math.ceil(requestedPayoutCents.value * withdrawalFeeBPS.value / 10_000)))
const estimatedNetPayout = computed(() => Math.max(0, requestedPayoutCents.value - estimatedWithdrawalFee.value))
const pendingTopUps = computed(() => wallet.value?.top_ups.filter((item) => item.status === 'pending') || [])
const yuan = (cents: number) => (Number(cents || 0) / 100).toFixed(2)
const money = (cents: number) => `¥${yuan(cents)}`
const percent = (bps: number) => `${(Number(bps || 0) / 100).toFixed(2).replace(/\.00$/, '')}%`

function entryLabel(value: string) {
  return ({ wallet_top_up: '钱包充值', redeem_code: '兑换码到账', resource_purchase: '资源购买', resource_sale: '资源销售', payout: '创作者提现' } as Record<string, string>)[value] || value
}

function topUpStatus(value: string) {
  return ({ pending: '待支付', paid: '已到账', cancelled: '已取消' } as Record<string, string>)[value] || value
}

function payoutStatus(value: string) {
  return ({ pending: '审核中', paid: '已打款', rejected: '已驳回' } as Record<string, string>)[value] || value
}

async function load() {
  loading.value = true
  try {
    const [walletResult, payoutResult] = await Promise.all([fetchWallet(), fetchCreatorPayouts()])
    wallet.value = walletResult
    payouts.value = payoutResult
  } catch (error) {
    Notify.danger(errorMessage(error, '钱包加载失败'))
  } finally {
    loading.value = false
  }
}

function selectTopUp(amount: number) {
  topUpYuan.value = amount
}

async function startTopUp() {
  const amount = Math.round(Number(topUpYuan.value || 0) * 100)
  if (!Number.isInteger(amount) || amount < 100 || amount > 500_000) {
    Notify.warn('单次充值金额需在 1 到 5,000 元之间')
    return
  }
  topUpLoading.value = true
  try {
    const order = await createWalletTopUp(amount)
    if (!order.payment_url) throw new Error('支付跳转地址为空')
    window.location.assign(order.payment_url)
  } catch (error) {
    Notify.danger(errorMessage(error, '充值订单创建失败'))
  } finally {
    topUpLoading.value = false
  }
}

async function submitRedeem() {
  const code = redeemCode.value.trim()
  if (!code) {
    Notify.warn('请输入兑换码')
    return
  }
  redeeming.value = true
  try {
    const item = await redeemWalletCode(code)
    redeemCode.value = ''
    Notify.success(`已到账 ${money(item.amount_cents)}`)
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '兑换失败'))
  } finally {
    redeeming.value = false
  }
}

function openWithdraw() {
  payoutForm.amount_yuan = Number((withdrawableCents.value / 100).toFixed(2))
  withdrawOpen.value = true
}

async function submitWithdraw() {
  const amount = Math.round(Number(payoutForm.amount_yuan || 0) * 100)
  if (!Number.isInteger(amount) || amount < 1000) {
    Notify.warn('单次提现最低 10 元')
    return
  }
  if (amount > withdrawableCents.value) {
    Notify.warn('提现金额超过可提现收益')
    return
  }
  if (!payoutForm.payout_account.trim() || !payoutForm.account_name.trim()) {
    Notify.warn('请填写收款账户和收款人姓名')
    return
  }
  withdrawing.value = true
  try {
    await requestCreatorPayout({
      amount_cents: amount,
      payout_method: payoutForm.payout_method,
      payout_account: payoutForm.payout_account.trim(),
      account_name: payoutForm.account_name.trim(),
      note: payoutForm.note.trim(),
    })
    withdrawOpen.value = false
    payoutForm.amount_yuan = 0
    payoutForm.payout_account = ''
    payoutForm.account_name = ''
    payoutForm.note = ''
    Notify.success('提现申请已提交')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '提现申请失败'))
  } finally {
    withdrawing.value = false
  }
}

onMounted(load)
</script>

<template>
  <PageContainer title="我的钱包">
    <div v-if="loading" class="rf-loading-block">正在加载钱包…</div>
    <div v-else class="rf-wallet-page">
      <section class="rf-wallet-balance" aria-label="余额概览">
        <div class="rf-wallet-balance-main">
          <span><AppIcon name="wallet" size="18" />可用余额</span>
          <strong>{{ money(availableCents) }}</strong>
          <small>可用于购买社区中的付费资源。</small>
        </div>
        <div class="rf-wallet-balance-side">
          <span>可提现创作者收益</span>
          <strong>{{ money(withdrawableCents) }}</strong>
          <small>提现手续费 {{ percent(withdrawalFeeBPS) }} · 资源服务费 {{ percent(serviceFeeBPS) }}</small>
          <button type="button" class="rf-text-button" :disabled="withdrawableCents < 1000" @click="openWithdraw">
            <AppIcon name="payout" size="17" />申请提现
          </button>
        </div>
      </section>

      <section class="rf-wallet-actions" aria-label="钱包操作">
        <form class="rf-wallet-action" @submit.prevent="startTopUp">
          <header><span class="rf-wallet-action-icon"><AppIcon name="topup" size="20" /></span><div><h2>充值</h2><p>安全跳转到已配置的支付渠道。</p></div></header>
          <div class="rf-wallet-presets" role="group" aria-label="选择充值金额">
            <button v-for="amount in topUpPresets" :key="amount" type="button" :class="{ active: topUpYuan === amount }" @click="selectTopUp(amount)">¥{{ amount }}</button>
          </div>
          <label>其他金额<input v-model.number="topUpYuan" type="number" min="1" max="5000" step="0.01" inputmode="decimal" /></label>
          <button type="submit" class="rf-primary-button" :disabled="topUpLoading">{{ topUpLoading ? '创建订单中…' : '去充值' }}</button>
        </form>

        <form class="rf-wallet-action" @submit.prevent="submitRedeem">
          <header><span class="rf-wallet-action-icon"><AppIcon name="code" size="20" /></span><div><h2>兑换码</h2><p>兑换成功后，余额会立即到账。</p></div></header>
          <label>兑换码<input v-model="redeemCode" maxlength="64" autocomplete="off" placeholder="XXXX-XXXX-XXXX-XXXX" /></label>
          <button type="submit" class="rf-secondary-button" :disabled="redeeming">{{ redeeming ? '兑换中…' : '确认兑换' }}</button>
        </form>
      </section>

      <section v-if="pendingTopUps.length" class="rf-wallet-section">
        <header class="rf-wallet-section-heading"><div><h2>待完成充值</h2><span>完成支付后余额会自动到账。</span></div><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></header>
        <div class="rf-wallet-rows">
          <article v-for="order in pendingTopUps" :key="order.id" class="rf-wallet-row">
            <div><strong>{{ money(order.amount_cents) }}</strong><small>订单 {{ order.order_no }}</small></div>
            <span class="rf-wallet-status is-pending">{{ topUpStatus(order.status) }}</span>
            <time>{{ new Date(order.created_at).toLocaleString('zh-CN') }}</time>
          </article>
        </div>
      </section>

      <section class="rf-wallet-section">
        <header class="rf-wallet-section-heading"><div><h2>资金流水</h2><span>充值、消费、兑换和创作者收益都会记录在这里。</span></div><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></header>
        <div v-if="wallet?.entries.length" class="rf-wallet-rows">
          <article v-for="entry in wallet.entries" :key="entry.id" class="rf-wallet-row">
            <div><strong>{{ entryLabel(entry.entry_type) }}</strong><small>{{ entry.note || '钱包账本记录' }}</small></div>
            <b :class="{ positive: entry.amount_cents >= 0 }">{{ entry.amount_cents >= 0 ? '+' : '-' }}{{ money(Math.abs(entry.amount_cents)) }}</b>
            <time>{{ new Date(entry.created_at).toLocaleString('zh-CN') }}</time>
          </article>
        </div>
        <div v-else class="rf-empty">暂无流水记录</div>
      </section>

      <section v-if="payouts.length" class="rf-wallet-section">
        <header class="rf-wallet-section-heading"><div><h2>提现记录</h2><span>创作者收益由管理员审核后人工打款。</span></div></header>
        <div class="rf-wallet-rows">
          <article v-for="item in payouts" :key="item.id" class="rf-wallet-row">
            <div><strong>到账 {{ money(item.net_amount_cents || item.amount_cents) }}</strong><small>申请 {{ money(item.amount_cents) }} · 手续费 {{ money(item.withdrawal_fee_cents || 0) }} · {{ item.payout_method }} · {{ item.account_name }}</small></div>
            <span class="rf-wallet-status" :class="item.status === 'paid' ? 'is-paid' : item.status === 'rejected' ? 'is-rejected' : 'is-pending'">{{ payoutStatus(item.status) }}</span>
            <time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time>
          </article>
        </div>
      </section>
    </div>

    <div v-if="withdrawOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="!withdrawing && (withdrawOpen = false)">
      <section class="rf-dialog rf-wallet-withdraw-dialog" role="dialog" aria-modal="true" aria-labelledby="withdraw-title">
        <header><div><h2 id="withdraw-title">申请提现</h2><p>可提现 {{ money(withdrawableCents) }}，当前手续费 {{ percent(withdrawalFeeBPS) }}，最低 10 元。</p></div><button type="button" class="rf-icon-button" aria-label="关闭" :disabled="withdrawing" @click="withdrawOpen = false"><AppIcon name="close" size="19" /></button></header>
        <form class="rf-editor-form" @submit.prevent="submitWithdraw">
          <label>提现金额（元）<input v-model.number="payoutForm.amount_yuan" type="number" min="10" :max="withdrawableCents / 100" step="0.01" inputmode="decimal" /></label>
          <label>收款方式<select v-model="payoutForm.payout_method"><option value="alipay">支付宝</option><option value="bank">银行卡</option><option value="paypal">PayPal</option><option value="other">其他</option></select></label>
          <label>收款账户<input v-model="payoutForm.payout_account" maxlength="180" placeholder="账号或银行卡号" /></label>
          <label>收款人姓名<input v-model="payoutForm.account_name" maxlength="80" /></label>
          <label>备注（可选）<textarea v-model="payoutForm.note" rows="2" maxlength="500" /></label>
          <div class="rf-withdraw-estimate"><span>手续费 {{ money(estimatedWithdrawalFee) }}</span><strong>预计到账 {{ money(estimatedNetPayout) }}</strong></div>
          <footer><button type="button" class="rf-secondary-button" :disabled="withdrawing" @click="withdrawOpen = false">取消</button><button type="submit" class="rf-primary-button" :disabled="withdrawing">{{ withdrawing ? '提交中…' : '提交申请' }}</button></footer>
        </form>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-wallet-page { min-width: 0; }.rf-wallet-balance { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(190px, .8fr); border-bottom: 1px solid var(--rf-line); }.rf-wallet-balance-main, .rf-wallet-balance-side { display: flex; min-height: 176px; flex-direction: column; justify-content: center; padding: 25px 20px; }.rf-wallet-balance-main > span { display: flex; align-items: center; gap: 7px; color: var(--rf-muted); font-size: 13px; }.rf-wallet-balance strong { margin-top: 7px; font-family: var(--rf-font-display); font-size: 36px; font-variant-numeric: tabular-nums; line-height: 1; }.rf-wallet-balance small { margin-top: 8px; color: var(--rf-muted); font-size: 12px; }.rf-wallet-balance-side { align-items: flex-start; border-left: 1px solid var(--rf-line); }.rf-wallet-balance-side > span { color: var(--rf-muted); font-size: 13px; }.rf-wallet-balance-side strong { font-size: 25px; }.rf-wallet-balance-side .rf-text-button { min-height: 35px; margin: 10px -10px 0; gap: 6px; padding-inline: 10px; color: var(--primary); font-size: 13px; }.rf-wallet-balance-side .rf-text-button:disabled { cursor: not-allowed; opacity: .45; }
.rf-wallet-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); border-bottom: 1px solid var(--rf-line); }.rf-wallet-action { display: flex; min-width: 0; flex-direction: column; gap: 13px; padding: 20px; }.rf-wallet-action + .rf-wallet-action { border-left: 1px solid var(--rf-line); }.rf-wallet-action header { display: flex; align-items: center; gap: 10px; }.rf-wallet-action-icon { display: inline-grid; width: 38px; height: 38px; flex: 0 0 38px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, var(--rf-bg)); }.rf-wallet-action h2 { margin: 0; font-size: 17px; }.rf-wallet-action p { margin: 2px 0 0; color: var(--rf-muted); font-size: 12px; }.rf-wallet-action label { display: flex; flex-direction: column; gap: 6px; color: var(--rf-muted); font-size: 12px; font-weight: 600; }.rf-wallet-action input { width: 100%; min-height: 40px; padding: 0 11px; border: 1px solid var(--rf-line); border-radius: 6px; outline: 0; color: var(--rf-text); background: var(--rf-bg); }.rf-wallet-action input:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent); }.rf-wallet-action > .rf-primary-button, .rf-wallet-action > .rf-secondary-button { align-self: flex-start; min-height: 38px; padding-inline: 17px; }.rf-wallet-presets { display: flex; flex-wrap: wrap; gap: 7px; }.rf-wallet-presets button { min-width: 46px; min-height: 34px; padding: 0 10px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); color: var(--rf-text); background: var(--rf-bg); font-size: 13px; font-variant-numeric: tabular-nums; }.rf-wallet-presets button:hover { background: var(--rf-bg-hover); }.rf-wallet-presets button.active { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 7%, transparent); }
.rf-wallet-section { border-bottom: 1px solid var(--rf-line); }.rf-wallet-section-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 17px 20px 11px; }.rf-wallet-section-heading h2 { margin: 0; font-size: 17px; }.rf-wallet-section-heading span { display: block; margin-top: 3px; color: var(--rf-muted); font-size: 12px; }.rf-wallet-section-heading .rf-icon-text-button { flex: 0 0 auto; }.rf-wallet-rows { border-top: 1px solid var(--rf-line); }.rf-wallet-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 16px; min-height: 62px; padding: 10px 20px; border-bottom: 1px solid var(--rf-line); }.rf-wallet-row:last-child { border-bottom: 0; }.rf-wallet-row > div { display: flex; min-width: 0; flex-direction: column; gap: 2px; }.rf-wallet-row strong, .rf-wallet-row small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-wallet-row strong { font-size: 14px; }.rf-wallet-row small, .rf-wallet-row time { color: var(--rf-muted); font-size: 11px; }.rf-wallet-row > b { color: var(--rf-danger); font-variant-numeric: tabular-nums; }.rf-wallet-row > b.positive { color: var(--rf-success); }.rf-wallet-status { min-width: 48px; color: var(--rf-muted); font-size: 12px; text-align: right; }.rf-wallet-status.is-pending { color: #a56a00; }.rf-wallet-status.is-paid { color: var(--rf-success); }.rf-wallet-status.is-rejected { color: var(--rf-danger); }.rf-wallet-withdraw-dialog > form { padding: 18px 20px 22px; }.rf-wallet-withdraw-dialog > header p { margin: 3px 0 0; color: var(--rf-muted); font-size: 12px; }.rf-withdraw-estimate { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 3px 0 7px; padding: 11px 12px; border-radius: 8px; background: color-mix(in srgb, var(--rf-text) 4%, var(--rf-bg)); }.rf-withdraw-estimate span { color: var(--rf-muted); font-size: 12px; }.rf-withdraw-estimate strong { font-size: 13px; font-variant-numeric: tabular-nums; }.rf-wallet-withdraw-dialog footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 5px; }
@media (max-width: 560px) { .rf-wallet-balance, .rf-wallet-actions { grid-template-columns: 1fr; }.rf-wallet-balance-main, .rf-wallet-balance-side { min-height: auto; padding: 21px 12px; }.rf-wallet-balance-side { border-top: 1px solid var(--rf-line); border-left: 0; }.rf-wallet-balance strong { font-size: 31px; }.rf-wallet-action { padding: 18px 12px; }.rf-wallet-action + .rf-wallet-action { border-top: 1px solid var(--rf-line); border-left: 0; }.rf-wallet-action > .rf-primary-button, .rf-wallet-action > .rf-secondary-button { align-self: stretch; }.rf-wallet-section-heading { padding-inline: 12px; }.rf-wallet-row { grid-template-columns: minmax(0, 1fr) auto; gap: 7px 12px; padding-inline: 12px; }.rf-wallet-row time { grid-column: 1 / -1; }.rf-wallet-withdraw-dialog > form { padding: 16px 14px calc(20px + env(safe-area-inset-bottom)); } }
</style>
