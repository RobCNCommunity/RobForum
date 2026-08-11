<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { api, type LotteryActivity, type LotteryDrawResult, type LotteryWin } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const activities = ref<LotteryActivity[]>([])
const wins = ref<LotteryWin[]>([])
const selected = ref<LotteryActivity | null>(null)
const spinning = ref(false)
const result = ref<LotteryDrawResult | null>(null)
const errorMessage = ref('')
const claimWin = ref<LotteryWin | null>(null)
const marqueeRoot = ref<HTMLElement | null>(null)
const marqueeTarget = ref(-1)
const scratchKey = ref(0)
const scratchRevealed = ref(false)
const address = reactive({ name: '', phone: '', province: '', city: '', district: '', detail: '' })

const marqueePrizes = computed(() => {
  const prizes = (selected.value?.prizes || []).slice(0, 8).map((prize) => ({ text: prize.name, imagePath: prize.image_url || '' }))
  while (prizes.length < 8) prizes.push({ text: '谢谢参与', imagePath: '' })
  return prizes
})

async function load() {
  const [activityResponse, winsResponse] = await Promise.all([api.get('/lottery/activities'), api.get('/me/lottery-wins')])
  activities.value = activityResponse.data.data
  wins.value = winsResponse.data.data
  if (!selected.value || !activities.value.some((item) => item.id === selected.value?.id)) selected.value = activities.value[0] || null
}

async function startMarquee() {
  if (!selected.value || spinning.value) return
  spinning.value = true
  result.value = null
  errorMessage.value = ''
  try {
    const response = await api.post(`/lottery/activities/${selected.value.id}/draw`)
    result.value = response.data.data
    const prizeID = result.value?.prize?.id
    const index = marqueePrizes.value.findIndex((_, prizeIndex) => selected.value?.prizes.slice(0, 8)[prizeIndex]?.id === prizeID)
    marqueeTarget.value = index >= 0 ? index : Math.min(7, marqueePrizes.value.length - 1)
    await nextTick()
    marqueeRoot.value?.querySelector<HTMLElement>('.start')?.click()
  } catch (error: any) {
    spinning.value = false
    errorMessage.value = error?.response?.data?.error?.message || '抽奖失败'
  }
}

function marqueeEnded() {
  spinning.value = false
  void load()
}

async function startScratch() {
	if (!selected.value || spinning.value) return
	spinning.value = true
	result.value = null
	scratchRevealed.value = false
  errorMessage.value = ''
  try {
		const response = await api.post(`/lottery/activities/${selected.value.id}/draw`)
		result.value = response.data.data
		scratchKey.value += 1
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.error?.message || '抽奖失败'
  } finally {
    spinning.value = false
  }
}

function scratchOpened() {
	scratchRevealed.value = true
	void load()
}

function selectActivity(activity: LotteryActivity) {
	selected.value = activity
	result.value = null
	scratchRevealed.value = false
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
    <header class="rf-page-header"><div class="rf-page-heading"><p class="lottery-kicker">COMMUNITY REWARDS</p><h1>抽奖活动</h1><p>使用积分参与活动，中奖后积分奖品即时到账。</p></div></header>
    <div class="activity-tabs"><button v-for="activity in activities" :key="activity.id" :class="{ active: selected?.id === activity.id }" type="button" @click="selectActivity(activity)">{{ activity.name }}</button><nut-empty v-if="!activities.length" description="暂无进行中的活动" /></div>
    <template v-if="selected">
      <div class="lottery-layout">
        <section class="lottery-stage" aria-label="抽奖区域">
          <div v-if="selected.style === 'scratch'" class="scratch-stage"><button v-if="!result" class="rf-primary-button draw-button" type="button" :disabled="spinning" @click="startScratch"><AppIcon name="code" size="17" />{{ spinning ? '准备刮刮卡中' : `消耗 ${selected.cost_points} 积分开始` }}</button><nut-scratch-card v-else :key="scratchKey" :content="result.prize?.name || '谢谢参与'" :height="220" :width="320" :ratio="0.3" cover-color="#a5b4c4" @open="scratchOpened" /><small>{{ result ? '刮开覆盖层查看本次结果' : '开始后生成本次刮刮卡' }}</small></div>
          <template v-else>
            <div ref="marqueeRoot" class="marquee-frame"><nut-marquee :prize-list="marqueePrizes" :prize-index="marqueeTarget" :speed="120" :circle="26" @end-turns="marqueeEnded" /></div>
            <button class="rf-primary-button draw-button" type="button" :disabled="spinning" @click="startMarquee"><AppIcon name="code" size="17" />{{ spinning ? '滚动开奖中' : `消耗 ${selected.cost_points} 积分开始` }}</button>
            <small class="stage-hint">滚动停止后显示本次结果</small>
          </template>
        </section>
        <aside class="lottery-info"><div class="lottery-info-head"><div><span class="lottery-label">ACTIVE EVENT</span><h2>{{ selected.name }}</h2></div><span class="cost-badge">{{ selected.cost_points }} 积分</span></div><p>{{ selected.description || '社区限时抽奖活动' }}</p><dl class="lottery-stats"><div><dt>每日上限</dt><dd>{{ selected.daily_limit || '不限' }}</dd></div><div><dt>免费次数</dt><dd>{{ selected.daily_free_attempts }}</dd></div><div><dt>奖品数量</dt><dd>{{ selected.prizes.length }}</dd></div></dl><div class="prize-list"><div v-for="prize in selected.prizes" :key="prize.id"><span>{{ prize.name }}</span><small>{{ (prize.probability_bp / 100).toFixed(2) }}%</small></div></div></aside>
      </div>
      <div v-if="result && (selected.style !== 'scratch' || scratchRevealed)" class="result-banner"><strong>{{ result.won ? '恭喜你抽中了' : '再接再厉' }}</strong><span>{{ result.prize?.name || '谢谢参与' }}</span><small>当前积分 {{ result.balance.toLocaleString() }}</small></div>
    </template>
    <p v-if="errorMessage" class="lottery-error" role="alert">{{ errorMessage }}</p>
    <section class="wins"><div class="section-title"><h2>我的中奖记录</h2><span>最近 100 条</span></div><div v-for="win in wins" :key="win.id" class="win-row"><strong>{{ win.prize_name }}</strong><span>{{ win.activity_name }}</span><small>{{ new Date(win.won_at).toLocaleString() }}</small><button v-if="win.physical && win.status === 'pending'" class="rf-text-button" type="button" @click="claimWin = win">填写地址</button></div><nut-empty v-if="!wins.length" description="暂无中奖记录" /></section>
    <div v-if="claimWin" class="claim-overlay" @click.self="claimWin = null"><form class="claim-panel" @submit.prevent="claim"><h2>领取实物奖品</h2><p>{{ claimWin.prize_name }}</p><div class="claim-grid"><label>收货人<input v-model.trim="address.name" required /></label><label>手机号<input v-model.trim="address.phone" required /></label><label>省份<input v-model.trim="address.province" required /></label><label>城市<input v-model.trim="address.city" required /></label><label>区县<input v-model.trim="address.district" /></label><label class="full">详细地址<textarea v-model.trim="address.detail" required rows="3" /></label></div><button class="rf-primary-button" type="submit">确认领取</button></form></div>
  </section>
</template>

<style scoped>
.lottery-page { padding-bottom: 48px; }
.lottery-kicker { margin: 0 0 5px; color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.activity-tabs { display: flex; gap: 8px; padding: 0 20px 16px; overflow: auto; border-bottom: 1px solid var(--rf-line); }
.activity-tabs button { flex: 0 0 auto; padding: 9px 14px; border: 1px solid var(--rf-line); border-radius: 7px; color: var(--rf-muted); background: var(--rf-bg); }
.activity-tabs button.active { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); font-weight: 700; }
.lottery-layout { display: grid; grid-template-columns: minmax(330px, 1fr) minmax(280px, 1fr); gap: 28px; padding: 24px 20px 28px; }
.lottery-stage { display: flex; min-width: 0; align-items: center; flex-direction: column; justify-content: center; gap: 14px; padding: 22px 16px; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg-subtle); }
.marquee-frame { width: min(100%, 420px); min-height: 340px; display: grid; place-items: center; overflow: hidden; }
.marquee-frame :deep(.nutbig-marquee) { transform: scale(.86); transform-origin: center; }
.marquee-frame :deep(.nutbig-marquee .start) { pointer-events: none; }
.scratch-stage { display: flex; align-items: center; flex-direction: column; gap: 12px; }
.scratch-stage :deep(.nutbig-scratch-card), .scratch-stage :deep(.nut-cover) { touch-action: none; }
.lottery-stage small, .stage-hint { color: var(--rf-muted); font-size: 12px; }
.draw-button { min-width: 220px; }
.lottery-info { min-width: 0; padding: 2px 0; }
.lottery-info-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.lottery-label { color: var(--primary); font-size: 10px; font-weight: 700; letter-spacing: .08em; }
.lottery-info h2 { margin: 4px 0 8px; font-size: 24px; }
.lottery-info > p { margin: 0 0 18px; color: var(--rf-muted); line-height: 1.6; }
.cost-badge { padding: 5px 8px; border-radius: 5px; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); font-size: 12px; font-weight: 700; white-space: nowrap; }
.lottery-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin: 0 0 18px; }
.lottery-stats div { padding: 10px; border: 1px solid var(--rf-line); border-radius: 7px; background: var(--rf-bg-subtle); }
.lottery-stats dt { color: var(--rf-muted); font-size: 11px; }
.lottery-stats dd { margin: 4px 0 0; font-weight: 700; }
.prize-list { display: flex; flex-direction: column; border-top: 1px solid var(--rf-line); }
.prize-list div { display: flex; justify-content: space-between; gap: 12px; padding: 11px 0; border-bottom: 1px solid var(--rf-line); }
.prize-list small { color: var(--rf-muted); }
.result-banner { display: flex; align-items: center; justify-content: center; gap: 12px; margin: 0 20px 20px; padding: 14px 16px; border: 1px solid color-mix(in srgb, var(--rf-success) 30%, transparent); color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 8%, transparent); }
.result-banner small { color: var(--rf-muted); }
.lottery-error { margin: 0 20px 16px; padding: 10px 12px; color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
.wins { padding: 20px; border-top: 1px solid var(--rf-line); }
.section-title { display: flex; justify-content: space-between; margin-bottom: 10px; }
.section-title h2 { margin: 0; font-size: 18px; }
.section-title span { color: var(--rf-muted); font-size: 12px; }
.win-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto auto; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--rf-line); }
.win-row span, .win-row small { color: var(--rf-muted); font-size: 12px; }
.claim-overlay { position: fixed; z-index: 2000; inset: 0; display: grid; place-items: center; padding: 20px; background: rgba(0,0,0,.48); }
.claim-panel { width: min(520px,100%); padding: 20px; border-radius: 8px; background: var(--rf-bg); }
.claim-panel h2 { margin: 0; }
.claim-panel p { color: var(--rf-muted); }
.claim-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin: 16px 0; }
.claim-grid label { display: flex; flex-direction: column; gap: 5px; color: var(--rf-muted); font-size: 12px; }
.claim-grid input, .claim-grid textarea { padding: 9px; border: 1px solid var(--rf-line); border-radius: 6px; background: var(--rf-bg); color: var(--rf-text); }
.claim-grid .full { grid-column: 1 / -1; }
@media (max-width: 760px) { .lottery-layout { grid-template-columns: 1fr; padding-inline: 14px; } .lottery-stats { gap: 5px; } .win-row { grid-template-columns: 1fr auto; } .win-row small { grid-column: 1; } }
</style>
