<script setup lang="ts">
import { computed } from 'vue'
import type { LotteryPrize } from '@/api'

const props = defineProps<{ prizes: LotteryPrize[]; rotation: number; animating: boolean }>()
const emit = defineEmits<{ end: [] }>()
const colors = ['#2563eb', '#e11d48', '#059669', '#d97706', '#7c3aed', '#0891b2', '#db2777', '#4f46e5']
const items = computed(() => {
  const result = props.prizes.slice(0, 8).map((item) => ({ name: item.name, image_url: item.image_url }))
  while (result.length < 8) result.push({ name: '谢谢参与', image_url: '' })
  return result
})
const segment = computed(() => 360 / items.value.length)
const wheelBackground = computed(() => {
  const stops = items.value.map((_, index) => `${colors[index % colors.length]} ${index * segment.value}deg ${(index + 1) * segment.value}deg`)
  return `conic-gradient(from -90deg, ${stops.join(', ')})`
})

function labelStyle(index: number) {
  const angle = (index + 0.5) * segment.value
  return { transform: `translate(-50%, -50%) rotate(${angle}deg) translateY(calc(var(--wheel-radius) * -1)) rotate(${-angle}deg)` }
}

function transitionEnded(event: TransitionEvent) {
  if (event.propertyName === 'transform' && props.animating) emit('end')
}
</script>

<template>
  <div class="wheel-shell">
    <span class="wheel-pointer" aria-hidden="true" />
    <div class="wheel" :class="{ animating }" :style="{ background: wheelBackground, transform: `rotate(${rotation}deg)` }" @transitionend="transitionEnded">
      <div v-for="(item, index) in items" :key="`${index}-${item.name}`" class="wheel-label" :style="labelStyle(index)">
        <img v-if="item.image_url" :src="item.image_url" alt="" />
        <span>{{ item.name }}</span>
      </div>
      <div class="wheel-hub"><span>LUOBO</span></div>
    </div>
  </div>
</template>

<style scoped>
.wheel-shell { --wheel-radius: clamp(88px, 27vw, 126px); position: relative; width: min(100%, 360px); aspect-ratio: 1; padding: 12px; }
.wheel-pointer { position: absolute; z-index: 3; top: 0; left: 50%; width: 0; height: 0; border-right: 14px solid transparent; border-left: 14px solid transparent; border-top: 28px solid var(--rf-text); filter: drop-shadow(0 2px 2px rgba(0,0,0,.2)); transform: translateX(-50%); }
.wheel { position: relative; width: 100%; height: 100%; overflow: hidden; border: 8px solid var(--rf-bg); border-radius: 50%; box-shadow: 0 0 0 1px var(--rf-line), 0 14px 32px rgba(15,23,42,.18); }
.wheel.animating { transition: transform 4.2s cubic-bezier(.12,.72,.12,1); }
.wheel::after { position: absolute; inset: 0; border: 1px solid rgba(255,255,255,.4); border-radius: 50%; content: ''; pointer-events: none; }
.wheel-label { position: absolute; z-index: 1; top: 50%; left: 50%; display: flex; width: 76px; align-items: center; flex-direction: column; gap: 3px; color: #fff; text-align: center; text-shadow: 0 1px 2px rgba(0,0,0,.45); }
.wheel-label img { width: 28px; height: 28px; border: 2px solid rgba(255,255,255,.75); border-radius: 50%; object-fit: cover; }
.wheel-label span { display: -webkit-box; overflow: hidden; font-size: 10px; font-weight: 700; line-height: 1.2; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.wheel-hub { position: absolute; z-index: 2; top: 50%; left: 50%; display: grid; width: 72px; height: 72px; place-items: center; border: 7px solid rgba(255,255,255,.82); border-radius: 50%; background: var(--rf-text); box-shadow: 0 4px 12px rgba(0,0,0,.3); transform: translate(-50%, -50%); }
.wheel-hub span { color: #fff; font-size: 10px; font-weight: 800; }
@media (max-width: 390px) { .wheel-shell { --wheel-radius: 25vw; }.wheel-label { width: 64px; }.wheel-hub { width: 62px; height: 62px; } }
</style>
