<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import { useSiteStore } from '@/stores/site'

defineProps<{ verified?: boolean; label?: string }>()
const site = useSiteStore()
</script>

<template>
  <span v-if="verified" class="verified-badge" :title="label || '已认证用户'">
    <img v-if="site.settings?.verification_badge_url" :src="site.settings.verification_badge_url" alt="" />
    <AppIcon v-else name="check" size="13" />
  </span>
</template>
