<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type PointSummary, type PointOrder } from '@/api'

const summary = ref<PointSummary | null>(null)
const orders = ref<PointOrder[]>([])
const loading = ref(true)
async function load() { loading.value = true; try { const [points, list] = await Promise.all([api.get('/me/points'), api.get('/me/point-orders')]); summary.value = points.data.data; orders.value = list.data.data } finally { loading.value = false } }
async function complete(order: PointOrder) { try { await api.patch(`/me/point-orders/${order.id}/complete`); await load() } catch (error: any) { window.alert(error?.response?.data?.error?.message || '确认收货失败') } }
async function tracking(order: PointOrder) { try { const response = await api.get(`/me/point-orders/${order.id}/tracking`); window.open(response.data.data.query_url, '_blank', 'noopener,noreferrer') } catch (error: any) { window.alert(error?.response?.data?.error?.message || '物流信息暂不可用') } }
onMounted(load)
</script>

<template>
  <section class="rf-page points-page">
    <header class="rf-page-header"><div class="rf-page-heading"><h1>积分中心</h1><p>记录每一次获得与使用，余额实时更新。</p></div></header>
    <div v-if="loading" class="points-loading">正在加载积分账户…</div>
    <template v-else-if="summary">
      <div class="points-hero"><span>当前积分</span><strong>{{ summary.account.balance.toLocaleString() }}</strong><small>累计获得 {{ summary.account.total_earned.toLocaleString() }} · 累计消耗 {{ summary.account.total_spent.toLocaleString() }}</small></div>
      <div class="points-actions"><RouterLink class="rf-primary-button" to="/points/store">去积分商城</RouterLink><RouterLink class="rf-text-button" to="/lottery">参加抽奖</RouterLink></div>
      <section class="points-section"><div class="section-title"><h2>积分流水</h2><span>最近 100 条</span></div><div class="points-list"><article v-for="entry in summary.entries" :key="entry.id" class="points-row"><div><strong>{{ entry.note || entry.type }}</strong><small>{{ new Date(entry.created_at).toLocaleString() }}</small></div><b :class="entry.amount > 0 ? 'positive' : 'negative'">{{ entry.amount > 0 ? '+' : '' }}{{ entry.amount }}</b><small>余额 {{ entry.balance_after }}</small></article><nut-empty v-if="!summary.entries.length" description="暂无积分流水" /></div></section>
      <section class="points-section"><div class="section-title"><h2>兑换订单</h2><RouterLink to="/points/store">商城</RouterLink></div><div class="points-list"><article v-for="order in orders.slice(0, 5)" :key="order.id" class="points-row"><div><strong>{{ order.product_name }}</strong><small>{{ order.order_no }} · {{ new Date(order.ordered_at).toLocaleDateString() }}</small></div><span>{{ order.status === 'pending' ? '待处理' : order.status === 'shipped' ? '已发货' : '已完成' }}</span><div class="order-actions"><button v-if="order.status === 'shipped' && order.physical" class="rf-text-button" @click="complete(order)">确认收货</button><button v-if="order.carrier" class="rf-text-button" @click="tracking(order)">查看物流</button></div><b class="negative">-{{ order.points_spent + order.shipping_points }}</b></article><nut-empty v-if="!orders.length" description="还没有兑换订单" /></div></section>
    </template>
  </section>
</template>

<style scoped>
.points-page { padding-bottom: 48px; }.points-loading { padding: 48px 20px; color: var(--rf-muted); }.points-hero { display: flex; flex-direction: column; gap: 8px; padding: 32px 24px; color: #fff; background: linear-gradient(135deg, #0f1419, #25313d); }.points-hero span { font-size: 14px; opacity: .75; }.points-hero strong { font-size: 52px; line-height: 1; }.points-hero small { opacity: .72; }.points-actions { display: flex; gap: 12px; padding: 18px 20px; border-bottom: 1px solid var(--rf-line); }.points-section { padding: 20px; border-bottom: 1px solid var(--rf-line); }.section-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }.section-title h2 { margin: 0; font-size: 18px; }.section-title span, .section-title a { color: var(--rf-muted); font-size: 12px; }.points-list { display: flex; flex-direction: column; gap: 2px; }.points-row { display: grid; grid-template-columns: minmax(0,1fr) auto auto auto; align-items: center; gap: 14px; min-height: 62px; border-bottom: 1px solid var(--rf-line); }.points-row > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.points-row strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.points-row small { color: var(--rf-muted); font-size: 12px; }.points-row b { font-size: 16px; }.positive { color: #16a34a; }.negative { color: #dc2626; }.points-row > span { color: var(--rf-muted); font-size: 12px; }.order-actions { display: flex !important; flex-direction: row !important; gap: 8px !important; }
</style>
