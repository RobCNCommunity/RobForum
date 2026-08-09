<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, type LotteryActivity, type LotteryDrawResult, type LotteryWin } from '@/api'

const activities = ref<LotteryActivity[]>([])
const wins = ref<LotteryWin[]>([])
const selected = ref<LotteryActivity | null>(null)
const spinning = ref(false)
const result = ref<LotteryDrawResult | null>(null)
const errorMessage = ref('')
const claimWin = ref<LotteryWin | null>(null)
const address = reactive({ name: '', phone: '', province: '', city: '', district: '', detail: '' })

async function load() {
  const [activityResponse, winsResponse] = await Promise.all([api.get('/lottery/activities'), api.get('/me/lottery-wins')])
  activities.value = activityResponse.data.data
  wins.value = winsResponse.data.data
  if (!selected.value || !activities.value.some((item) => item.id === selected.value?.id)) selected.value = activities.value[0] || null
}
async function draw() {
  if (!selected.value || spinning.value) return
  spinning.value = true
  result.value = null
  errorMessage.value = ''
  try {
    const response = await api.post(`/lottery/activities/${selected.value.id}/draw`)
    await new Promise((resolve) => setTimeout(resolve, 1100))
    result.value = response.data.data
    await load()
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '抽奖失败'
  } finally {
    spinning.value = false
  }
}
async function claim() {
  if (!claimWin.value) return
  try {
    await api.post(`/me/lottery-wins/${claimWin.value.id}/claim`, { address })
    claimWin.value = null
    Object.assign(address, { name: '', phone: '', province: '', city: '', district: '', detail: '' })
    await load()
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '领取失败'
  }
}
onMounted(load)
</script>

<template>
  <section class="rf-page lottery-page">
    <header class="rf-page-header"><div class="rf-page-heading"><h1>抽奖活动</h1><p>转动好运转盘，积分奖品即时到账。</p></div></header>
    <div class="activity-tabs"><button v-for="activity in activities" :key="activity.id" :class="{ active: selected?.id === activity.id }" @click="selected = activity">{{ activity.name }}</button><nut-empty v-if="!activities.length" description="暂无进行中的活动" /></div>
    <template v-if="selected">
      <div class="lottery-layout">
        <div class="wheel-stage">
          <template v-if="selected.style === 'scratch'">
            <div class="scratch-wrap"><nut-scratch-card :content="result?.prize?.name || '刮开查看结果'" :height="220" :width="320" cover-color="#94a3b8" @open="draw" /></div>
            <small>刮开卡片后提交一次抽奖</small>
          </template>
          <template v-else>
            <div class="wheel" :class="{ spinning }"><div v-for="(prize, index) in selected.prizes" :key="prize.id" class="wheel-label" :style="{ transform: `rotate(${index * 360 / Math.max(selected.prizes.length, 1)}deg)` }"><span>{{ prize.name }}</span></div><div class="wheel-center">GO</div></div>
            <button class="rf-primary-button draw-button" :disabled="spinning" @click="draw">{{ spinning ? '抽奖中…' : `消耗 ${selected.cost_points} 积分` }}</button>
          </template>
        </div>
        <div class="lottery-info"><h2>{{ selected.name }}</h2><p>{{ selected.description }}</p><div class="prize-list"><div v-for="prize in selected.prizes" :key="prize.id"><span>{{ prize.name }}</span><small>{{ (prize.probability_bp / 100).toFixed(2) }}%</small></div></div></div>
      </div>
      <div v-if="result" class="result-banner"><strong>{{ result.won ? '恭喜你抽中了' : '再接再厉' }}</strong><span>{{ result.prize?.name || '谢谢参与' }}</span></div>
    </template>
    <p v-if="errorMessage" class="lottery-error" role="alert">{{ errorMessage }}</p>
    <section class="wins"><div class="section-title"><h2>我的中奖记录</h2><span>最近 100 条</span></div><div v-for="win in wins" :key="win.id" class="win-row"><strong>{{ win.prize_name }}</strong><span>{{ win.activity_name }}</span><small>{{ new Date(win.won_at).toLocaleString() }}</small><button v-if="win.physical && win.status === 'pending'" class="rf-text-button" @click="claimWin = win">填写地址</button></div><nut-empty v-if="!wins.length" description="暂无中奖记录" /></section>
    <div v-if="claimWin" class="claim-overlay" @click.self="claimWin = null"><form class="claim-panel" @submit.prevent="claim"><h2>领取实物奖品</h2><p>{{ claimWin.prize_name }}</p><div class="claim-grid"><label>收货人<input v-model.trim="address.name" required /></label><label>手机号<input v-model.trim="address.phone" required /></label><label>省份<input v-model.trim="address.province" required /></label><label>城市<input v-model.trim="address.city" required /></label><label>区县<input v-model.trim="address.district" /></label><label class="full">详细地址<textarea v-model.trim="address.detail" required rows="3" /></label></div><button class="rf-primary-button" type="submit">确认领取</button></form></div>
  </section>
</template>

<style scoped>
.lottery-page{padding-bottom:48px}.activity-tabs{display:flex;gap:8px;padding:16px 20px;overflow:auto;border-bottom:1px solid var(--rf-line)}.activity-tabs button{flex:0 0 auto;padding:9px 14px;border-radius:999px;color:var(--rf-muted);background:var(--rf-bg-subtle)}.activity-tabs button.active{color:#fff;background:var(--primary);font-weight:700}.lottery-layout{display:grid;grid-template-columns:minmax(300px,1fr) minmax(260px,1fr);gap:28px;padding:28px 20px}.wheel-stage{display:flex;align-items:center;flex-direction:column;gap:20px}.scratch-wrap{padding:12px;border:1px solid var(--rf-line);background:var(--rf-bg-subtle)}.wheel{position:relative;width:min(360px,80vw);aspect-ratio:1;border:12px solid #0f1419;border-radius:50%;background:conic-gradient(#f97316 0 25%,#facc15 25% 50%,#22c55e 50% 75%,#38bdf8 75%);transition:transform 1.3s cubic-bezier(.2,.8,.2,1)}.wheel.spinning{transform:rotate(1080deg)}.wheel-center{position:absolute;inset:35%;display:grid;place-items:center;border:5px solid #fff;border-radius:50%;color:#fff;background:var(--primary);font-weight:800}.wheel-label{position:absolute;inset:12px;transform-origin:center}.wheel-label span{position:absolute;top:8px;left:50%;color:#0f1419;font-size:12px;font-weight:800;transform:translateX(-50%);white-space:nowrap}.draw-button{min-width:220px}.lottery-info h2{margin:0 0 8px;font-size:24px}.lottery-info p{margin:0 0 18px;color:var(--rf-muted);line-height:1.6}.prize-list{display:flex;flex-direction:column;border-top:1px solid var(--rf-line)}.prize-list div{display:flex;justify-content:space-between;padding:11px 0;border-bottom:1px solid var(--rf-line)}.prize-list small{color:var(--rf-muted)}.result-banner{display:flex;align-items:center;justify-content:center;gap:12px;margin:0 20px;padding:16px;color:#fff;background:#16a34a}.lottery-error{margin:12px 20px;padding:10px;color:#b91c1c;background:#fee2e2}.wins{padding:20px;border-top:1px solid var(--rf-line)}.section-title{display:flex;justify-content:space-between;margin-bottom:10px}.section-title h2{margin:0;font-size:18px}.section-title span{color:var(--rf-muted);font-size:12px}.win-row{display:grid;grid-template-columns:1fr auto auto auto;gap:12px;padding:12px 0;border-bottom:1px solid var(--rf-line)}.win-row span,.win-row small{color:var(--rf-muted);font-size:12px}.claim-overlay{position:fixed;z-index:2000;inset:0;display:grid;place-items:center;padding:20px;background:rgba(0,0,0,.48)}.claim-panel{width:min(520px,100%);padding:20px;border-radius:8px;background:var(--rf-bg)}.claim-panel h2{margin:0}.claim-panel p{color:var(--rf-muted)}.claim-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:16px 0}.claim-grid label{display:flex;flex-direction:column;gap:5px;color:var(--rf-muted);font-size:12px}.claim-grid input,.claim-grid textarea{padding:9px;border:1px solid var(--rf-line);border-radius:6px;background:var(--rf-bg);color:var(--rf-text)}.claim-grid .full{grid-column:1/-1}@media(max-width:760px){.lottery-layout{grid-template-columns:1fr}.wheel{width:min(320px,82vw)}.win-row{grid-template-columns:1fr auto}}
</style>
