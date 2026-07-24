<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import { useTheme } from '@/theme'

const { isDark, toggleTheme } = useTheme()
</script>

<template>
  <button
    type="button"
    class="rf-theme-toggle"
    :aria-label="isDark ? '切换到亮色模式' : '切换到深色模式'"
    :aria-pressed="isDark"
    :title="isDark ? '切换到亮色模式' : '切换到深色模式'"
    @click="toggleTheme"
  >
    <AppIcon :name="isDark ? 'sun' : 'moon'" size="20" />
  </button>
</template>
