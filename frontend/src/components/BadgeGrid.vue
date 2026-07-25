<script setup lang="ts">
import type { Badge } from '@/api'
import AppIcon from './AppIcon.vue'

withDefaults(defineProps<{ badges: Badge[]; compact?: boolean }>(), { compact: false })

function awardDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'short', day: 'numeric' }).format(date)
}
</script>

<template>
  <div class="rf-badge-grid" :class="{ 'is-compact': compact }">
    <article v-for="badge in badges" :key="badge.id" :style="{ '--badge-color': badge.color || '#1d9bf0' }">
      <span><AppIcon :name="badge.icon || 'badge'" :size="compact ? 18 : 23" /></span>
      <div><strong>{{ badge.name }}</strong><p v-if="!compact">{{ badge.description }}</p><small v-if="!compact && awardDate(badge.awarded_at)">{{ awardDate(badge.awarded_at) }} 获得</small></div>
    </article>
  </div>
</template>

<style scoped>
.rf-badge-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.rf-badge-grid article { display: grid; min-width: 0; grid-template-columns: 42px minmax(0, 1fr); align-items: start; gap: 10px; padding: 12px; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg); }
.rf-badge-grid article > span { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; color: var(--badge-color); background: color-mix(in srgb, var(--badge-color) 10%, var(--rf-bg)); }
.rf-badge-grid article > div { min-width: 0; }
.rf-badge-grid strong { display: block; overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.rf-badge-grid p { margin: 3px 0 0; color: var(--rf-muted); font-size: 12px; line-height: 1.45; }
.rf-badge-grid small { display: block; margin-top: 5px; color: var(--rf-faint); font-size: 10px; }
.rf-badge-grid.is-compact { display: flex; flex-wrap: wrap; gap: 7px; }
.rf-badge-grid.is-compact article { display: inline-flex; width: auto; grid-template-columns: none; align-items: center; gap: 6px; padding: 4px 8px 4px 5px; border-radius: var(--rf-pill); }
.rf-badge-grid.is-compact article > span { width: 26px; height: 26px; }
.rf-badge-grid.is-compact strong { max-width: 140px; font-size: 12px; }
@media (max-width: 560px) { .rf-badge-grid { grid-template-columns: 1fr; } }
</style>
