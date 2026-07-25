<script setup lang="ts">
import { computed, ref } from 'vue'
import AppIcon from './AppIcon.vue'
import IdentityInfoDialog from './IdentityInfoDialog.vue'
import { useSiteStore } from '@/stores/site'

const props = defineProps<{ verified?: boolean; label?: string }>()
const site = useSiteStore()
const hasCustomBadge = computed(() => Boolean(site.settings?.verification_badge_url))
const open = ref(false)
const badgeLabel = computed(() => props.label || '已认证用户')
</script>

<template>
  <button v-if="verified" type="button" class="verified-badge" :class="{ 'verified-badge--custom': hasCustomBadge }" :title="badgeLabel" :aria-label="`查看认证信息：${badgeLabel}`" @click.stop.prevent="open = true">
    <img v-if="hasCustomBadge" :src="site.settings!.verification_badge_url" alt="" />
    <AppIcon v-else name="check" size="13" />
  </button>
  <IdentityInfoDialog :open="open" kind="verification" title="蓝微认证" :label="badgeLabel" :description="`该账号已通过社区认证审核，认证身份为“${badgeLabel}”。认证图标只代表社区身份核验，不代表 Roblox 官方背书。`" :image-url="site.settings?.verification_badge_url" color="var(--primary)" @close="open = false" />
</template>
