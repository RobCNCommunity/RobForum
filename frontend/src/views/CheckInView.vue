<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { createCheckin, errorMessage, fetchCheckin, type CheckinSummary } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import BadgeGrid from '@/components/BadgeGrid.vue'
import PageContainer from '@/components/PageContainer.vue'

const loading = ref(true)
const checking = ref(false)
const summary = ref<CheckinSummary | null>(null)

const progress = computed(() => summary.value?.progress)
const historyMap = computed(() => new Map((summary.value?.history || []).map((item) => [item.date, item])))
const nextExperience = computed(() => {
  const value = progress.value
  if (!value?.next_level_experience) return 0
  return Math.max(0, value.next_level_experience - value.experience)
})

function dateKey(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const calendarTitle = computed(() => new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long' }).format(new Date()))
const calendarDays = computed(() => {
  const now = new Date()
  const year = now.getFullYear()
  const month = now.getMonth()
  const first = new Date(year, month, 1)
  const leading = (first.getDay() + 6) % 7
  const count = new Date(year, month + 1, 0).getDate()
  const result: Array<{ day: number; key: string; checked: boolean; today: boolean; future: boolean } | null> = []
  for (let index = 0; index < leading; index++) result.push(null)
  const today = dateKey(now)
  for (let day = 1; day <= count; day++) {
    const key = dateKey(new Date(year, month, day))
    result.push({ day, key, checked: historyMap.value.has(key), today: key === today, future: key > today })
  }
  return result
})

const levels = [
  { level: 1, name: '新芽', experience: 0 },
  { level: 2, name: '熟面孔', experience: 100 },
  { level: 3, name: '活跃玩家', experience: 300 },
  { level: 4, name: '资深玩家', experience: 800 },
  { level: 5, name: '社区达人', experience: 1600 },
  { level: 6, name: '社区元老', experience: 3000 },
]

async function load() {
  loading.value = true
  try { summary.value = await fetchCheckin() }
  catch (error) { Notify.danger(errorMessage(error, '签到信息加载失败')) }
  finally { loading.value = false }
}

async function checkin() {
  if (checking.value || progress.value?.checked_in_today) return
  checking.value = true
  const before = progress.value?.experience || 0
  try {
    summary.value = await createCheckin()
    const gained = Math.max(0, (summary.value.progress.experience || 0) - before)
    Notify.success(gained ? `签到成功，获得 ${gained} 经验` : '今天已经签到')
    if (summary.value.newly_awarded.length) Notify.success(`获得徽章：${summary.value.newly_awarded.map((item) => item.name).join('、')}`)
  } catch (error) { Notify.danger(errorMessage(error, '签到失败')) }
  finally { checking.value = false }
}

onMounted(load)
</script>

<template>
  <PageContainer title="签到与资历">
    <div v-if="loading" class="rf-checkin-loading"><nut-skeleton animated title row="5" /></div>
    <div v-else-if="summary && progress" class="rf-checkin-page">
      <section class="rf-checkin-overview">
        <div class="rf-checkin-level-mark"><span>LV.{{ progress.level }}</span><strong>{{ progress.level_name }}</strong></div>
        <div class="rf-checkin-level-copy">
          <div><span>当前资历</span><strong>{{ progress.experience }} 经验</strong></div>
          <div class="rf-checkin-progress" role="progressbar" :aria-valuenow="progress.level_progress" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${progress.level_progress}%` }" /></div>
          <small v-if="progress.next_level_experience">距离下一级还需 {{ nextExperience }} 经验</small><small v-else>已达到当前最高资历等级</small>
        </div>
        <nut-button type="primary" size="large" :loading="checking" :disabled="progress.checked_in_today" @click="checkin"><AppIcon :name="progress.checked_in_today ? 'success' : 'calendar'" size="18" />{{ progress.checked_in_today ? '今日已签到' : '立即签到' }}</nut-button>
      </section>

      <section class="rf-checkin-stats" aria-label="签到统计">
        <div><AppIcon name="calendar" size="19" /><span>累计签到</span><strong>{{ progress.total_checkins }} 天</strong></div>
        <div><AppIcon name="flame" size="19" /><span>连续签到</span><strong>{{ progress.current_streak }} 天</strong></div>
        <div><AppIcon name="award" size="19" /><span>最长连续</span><strong>{{ progress.longest_streak }} 天</strong></div>
      </section>

      <section class="rf-checkin-section">
        <header><div><h2>{{ calendarTitle }}</h2><p>每日签到获得 10 经验，连续签到最多额外获得 12 经验。</p></div></header>
        <div class="rf-checkin-weekdays"><span v-for="day in ['一', '二', '三', '四', '五', '六', '日']" :key="day">{{ day }}</span></div>
        <div class="rf-checkin-calendar">
          <span v-for="(item, index) in calendarDays" :key="item?.key || `empty-${index}`" :class="{ empty: !item, checked: item?.checked, today: item?.today, future: item?.future }"><template v-if="item"><b>{{ item.day }}</b><AppIcon v-if="item.checked" name="plainCheck" size="13" /></template></span>
        </div>
      </section>

      <section class="rf-checkin-section">
        <header><div><h2>我的徽章</h2><p>签到和参与社区会自动解锁成就徽章。</p></div><span>{{ summary.badges.length }} 枚</span></header>
        <BadgeGrid v-if="summary.badges.length" :badges="summary.badges" />
        <div v-else class="rf-empty">完成首次签到即可获得第一枚签到徽章。</div>
      </section>

      <section class="rf-checkin-section rf-level-list">
        <header><div><h2>资历等级</h2><p>经验会永久累计，等级不会因签到中断而下降。</p></div></header>
        <div v-for="item in levels" :key="item.level" :class="{ active: item.level === progress.level, reached: item.level <= progress.level }"><span>LV.{{ item.level }}</span><strong>{{ item.name }}</strong><small>{{ item.experience }} 经验</small></div>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-checkin-page { min-width: 0; padding-bottom: 30px; }
.rf-checkin-loading { padding: 24px 18px; }
.rf-checkin-overview { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 18px; padding: 24px 20px; border-bottom: 1px solid var(--rf-line); }
.rf-checkin-level-mark { display: grid; width: 84px; height: 84px; place-items: center; align-content: center; border: 1px solid color-mix(in srgb, var(--primary) 28%, var(--rf-line)); border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, var(--rf-bg)); }
.rf-checkin-level-mark span { font-size: 12px; font-weight: 800; }
.rf-checkin-level-mark strong { margin-top: 2px; font-size: 16px; }
.rf-checkin-level-copy { min-width: 0; }
.rf-checkin-level-copy > div:first-child { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.rf-checkin-level-copy span, .rf-checkin-level-copy small { color: var(--rf-muted); font-size: 12px; }
.rf-checkin-level-copy strong { font-size: 19px; font-variant-numeric: tabular-nums; }
.rf-checkin-progress { height: 8px; margin: 10px 0 7px; overflow: hidden; border-radius: 4px; background: var(--rf-bg-subtle); }
.rf-checkin-progress i { display: block; height: 100%; border-radius: inherit; background: var(--primary); transition: width 300ms ease-out; }
.rf-checkin-stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-bottom: 10px solid var(--rf-bg-subtle); }
.rf-checkin-stats div { display: grid; min-width: 0; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 3px 8px; padding: 18px 20px; border-right: 1px solid var(--rf-line); }
.rf-checkin-stats div:last-child { border-right: 0; }
.rf-checkin-stats svg { grid-row: 1 / span 2; color: var(--primary); }
.rf-checkin-stats span { color: var(--rf-muted); font-size: 11px; }
.rf-checkin-stats strong { overflow: hidden; font-size: 17px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.rf-checkin-section { padding: 20px; border-bottom: 10px solid var(--rf-bg-subtle); }
.rf-checkin-section > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; margin-bottom: 16px; }
.rf-checkin-section h2 { margin: 0; font-size: 19px; }
.rf-checkin-section header p { margin: 4px 0 0; color: var(--rf-muted); font-size: 12px; }
.rf-checkin-section header > span { flex: 0 0 auto; color: var(--rf-muted); font-size: 12px; }
.rf-checkin-weekdays, .rf-checkin-calendar { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); }
.rf-checkin-weekdays span { padding: 5px 0 8px; color: var(--rf-muted); font-size: 11px; text-align: center; }
.rf-checkin-calendar { gap: 5px; }
.rf-checkin-calendar > span { position: relative; display: grid; width: 100%; aspect-ratio: 1; place-items: center; border: 1px solid var(--rf-line); border-radius: 8px; color: var(--rf-text); background: var(--rf-bg); font-size: 12px; }
.rf-checkin-calendar > span.empty { border-color: transparent; background: transparent; }
.rf-checkin-calendar > span.future { color: var(--rf-faint); background: var(--rf-bg-subtle); }
.rf-checkin-calendar > span.today { border-color: var(--primary); }
.rf-checkin-calendar > span.checked { border-color: color-mix(in srgb, var(--primary) 38%, var(--rf-line)); color: var(--primary); background: color-mix(in srgb, var(--primary) 9%, var(--rf-bg)); font-weight: 800; }
.rf-checkin-calendar > span svg { position: absolute; right: 4px; bottom: 4px; }
.rf-level-list > div { display: grid; min-height: 52px; grid-template-columns: 58px minmax(0, 1fr) auto; align-items: center; gap: 10px; border-top: 1px solid var(--rf-line); }
.rf-level-list > div > span { color: var(--rf-faint); font-size: 12px; font-weight: 800; }
.rf-level-list > div > small { color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-level-list > div.reached > span, .rf-level-list > div.active strong { color: var(--primary); }
.rf-level-list > div.active { background: color-mix(in srgb, var(--primary) 5%, transparent); }
@media (max-width: 560px) {
  .rf-checkin-overview { grid-template-columns: 68px minmax(0, 1fr); gap: 13px; padding: 18px 12px; }
  .rf-checkin-level-mark { width: 68px; height: 68px; }
  .rf-checkin-level-mark strong { font-size: 14px; }
  .rf-checkin-overview > .nut-button { grid-column: 1 / -1; width: 100%; min-height: 46px; }
  .rf-checkin-stats div { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; padding: 14px 10px; }
  .rf-checkin-stats svg { grid-row: auto; }
  .rf-checkin-stats strong { font-size: 15px; }
  .rf-checkin-section { padding: 18px 12px; }
  .rf-checkin-calendar { gap: 3px; }
  .rf-checkin-calendar > span { border-radius: 6px; }
  .rf-checkin-calendar > span svg { right: 2px; bottom: 2px; }
}
</style>
