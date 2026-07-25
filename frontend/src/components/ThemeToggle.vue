<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import { useTheme } from '@/theme'

const { isDark, transitioning, toggleTheme } = useTheme()

function handleToggle(event: MouseEvent) {
  toggleTheme({ x: event.clientX, y: event.clientY })
}
</script>

<template>
  <button
    type="button"
    class="rf-theme-toggle"
    :aria-label="isDark ? '切换到亮色模式' : '切换到深色模式'"
    :aria-pressed="isDark"
    :title="isDark ? '切换到亮色模式' : '切换到深色模式'"
    :disabled="transitioning"
    @click="handleToggle"
  >
    <AppIcon :name="isDark ? 'sun' : 'moon'" size="20" />
  </button>
</template>
