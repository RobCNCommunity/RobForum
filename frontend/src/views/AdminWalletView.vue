<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { createRedeemCode, errorMessage, fetchRedeemCodes, revokeRedeemCode, type RedeemCode } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const codes = ref<RedeemCode[]>([])
const loading = ref(true)
const creating = ref(false)
const revokingID = ref<number | null>(null)
const generatedCode = ref('')
const form = reactive({ amount_yuan: 10, expires_on: '' })

const money = (cents: number) => `¥${(Number(cents || 0) / 100).toFixed(2)}`
const statusLabel = (value: string) => ({ active: '可用', redeemed: '已兑换', revoked: '已停用', expired: '已过期' } as Record<string, string>)[value] || value

async function load() {
  loading.value = true
  try {
    codes.value = await fetchRedeemCodes()
  } catch (error) {
    Notify.danger(errorMessage(error, '兑换码加载失败'))
  } finally {
    loading.value = false
  }
}

async function submit() {
  const amount = Math.round(Number(form.amount_yuan || 0) * 100)
  if (!Number.isInteger(amount) || amount < 1) {
    Notify.warn('请输入有效金额')
    return
  }
  let expiresAt: string | undefined
  if (form.expires_on) {
    const date = new Date(`${form.expires_on}T23:59:59`)
    if (Number.isNaN(date.getTime()) || date.getTime() <= Date.now()) {
      Notify.warn('有效期必须晚于当前时间')
      return
    }
    expiresAt = date.toISOString()
  }
  creating.value = true
  generatedCode.value = ''
  try {
    const item = await createRedeemCode({ amount_cents: amount, expires_at: expiresAt })
    generatedCode.value = item.code || ''
    Notify.success('兑换码已创建')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '兑换码创建失败'))
  } finally {
    creating.value = false
  }
}

async function revoke(item: RedeemCode) {
  if (item.status !== 'active' || revokingID.value !== null) return
  if (!window.confirm(`停用兑换码 ${item.code_hint}？停用后不能恢复。`)) return
  revokingID.value = item.id
  try {
    await revokeRedeemCode(item.id)
    Notify.success('兑换码已停用')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '兑换码停用失败'))
  } finally {
    revokingID.value = null
  }
}

onMounted(load)
</script>

<template>
  <PageContainer title="钱包与兑换码">
    <template #extra><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></template>

    <section class="rf-admin-wallet-create">
      <header><span><AppIcon name="code" size="20" /></span><div><h2>创建兑换码</h2><p>兑换码以哈希形式保存，生成后的完整内容只显示这一次。</p></div></header>
      <form @submit.prevent="submit">
        <label>面额（元）<input v-model.number="form.amount_yuan" type="number" min="0.01" max="10000" step="0.01" inputmode="decimal" /></label>
        <label>有效期（可选）<input v-model="form.expires_on" type="date" /></label>
        <button type="submit" class="rf-primary-button" :disabled="creating">{{ creating ? '创建中…' : '生成兑换码' }}</button>
      </form>
      <div v-if="generatedCode" class="rf-admin-wallet-code" role="status"><span>请立即保存</span><code>{{ generatedCode }}</code><button type="button" @click="generatedCode = ''">已保存</button></div>
    </section>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
    <div v-else-if="!codes.length" class="rf-empty"><AppIcon name="code" size="28" /><strong>还没有兑换码</strong><span>创建后会在这里显示兑换和停用状态。</span></div>
    <div v-else class="rf-data-table rf-admin-wallet-table" role="table" aria-label="兑换码列表">
      <div class="rf-data-row rf-data-head" role="row"><span>兑换码</span><span>面额</span><span>状态</span><span>兑换用户</span><span>有效期</span><span>操作</span></div>
      <article v-for="item in codes" :key="item.id" class="rf-data-row" role="row">
        <div data-label="兑换码"><strong>{{ item.code_hint }}</strong><small>创建于 {{ new Date(item.created_at).toLocaleString('zh-CN') }}</small></div>
        <div data-label="面额"><b class="rf-money">{{ money(item.amount_cents) }}</b></div>
        <div data-label="状态"><span class="rf-status-chip" :class="item.status === 'active' ? 'is-on' : item.status === 'redeemed' ? 'is-warning' : 'is-danger'">{{ statusLabel(item.status) }}</span></div>
        <div data-label="兑换用户"><span>{{ item.redeemed_by_name || '—' }}</span><small v-if="item.redeemed_at">{{ new Date(item.redeemed_at).toLocaleString('zh-CN') }}</small></div>
        <div data-label="有效期"><time>{{ item.expires_at ? new Date(item.expires_at).toLocaleString('zh-CN') : '永久有效' }}</time></div>
        <div data-label="操作" class="rf-row-actions"><button v-if="item.status === 'active'" type="button" class="rf-danger-button rf-button-small" :disabled="revokingID === item.id" @click="revoke(item)">{{ revokingID === item.id ? '停用中…' : '停用' }}</button><small v-else>不可操作</small></div>
      </article>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-admin-wallet-create { padding: 20px; border-bottom: 1px solid var(--rf-line); }.rf-admin-wallet-create > header { display: flex; align-items: center; gap: 10px; }.rf-admin-wallet-create > header > span { display: inline-grid; width: 38px; height: 38px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, var(--rf-bg)); }.rf-admin-wallet-create h2 { margin: 0; font-size: 18px; }.rf-admin-wallet-create p { margin: 2px 0 0; color: var(--rf-muted); font-size: 12px; }.rf-admin-wallet-create form { display: grid; grid-template-columns: minmax(0, 180px) minmax(0, 210px) auto; align-items: end; gap: 12px; margin-top: 16px; }.rf-admin-wallet-create label { display: flex; flex-direction: column; gap: 6px; color: var(--rf-muted); font-size: 12px; font-weight: 600; }.rf-admin-wallet-create input { min-height: 40px; padding: 0 10px; border: 1px solid var(--rf-line); border-radius: 6px; outline: 0; color: var(--rf-text); background: var(--rf-bg); }.rf-admin-wallet-create input:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent); }.rf-admin-wallet-create .rf-primary-button { min-height: 40px; }.rf-admin-wallet-code { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 14px; padding: 12px; border-radius: 7px; color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); }.rf-admin-wallet-code span { font-size: 12px; font-weight: 700; }.rf-admin-wallet-code code { padding: 3px 7px; border-radius: 4px; color: var(--rf-text); background: var(--rf-bg); font-size: 14px; user-select: all; }.rf-admin-wallet-code button { margin-left: auto; min-height: 30px; padding: 0 9px; border-radius: var(--rf-pill); color: var(--primary); background: transparent; font-size: 12px; font-weight: 700; }.rf-admin-wallet-code button:hover { background: color-mix(in srgb, var(--primary) 10%, transparent); }
@media (max-width: 760px) { .rf-admin-wallet-create { padding-inline: 12px; }.rf-admin-wallet-create form { grid-template-columns: 1fr; }.rf-admin-wallet-create .rf-primary-button { width: 100%; }.rf-admin-wallet-code button { margin-left: 0; }.rf-admin-wallet-table .rf-data-row > div.rf-row-actions { padding-left: 94px; } }
</style>
