<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'
import { useSiteStore } from '@/stores/site'

defineProps<{ verified?: boolean; label?: string }>()
const site = useSiteStore()
const hasCustomBadge = computed(() => Boolean(site.settings?.verification_badge_url))
</script>

<template>
  <span v-if="verified" class="verified-badge" :class="{ 'verified-badge--custom': hasCustomBadge }" :title="label || '已认证用户'">
    <img v-if="hasCustomBadge" :src="site.settings!.verification_badge_url" alt="" />
    <AppIcon v-else name="check" size="13" />
  </span>
</template>
