<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppIcon from './AppIcon.vue'
import { useMembershipStore } from '@/stores/membership'

const props = defineProps<{ active?: boolean; tierId?: number }>()
const membership = useMembershipStore()
const tier = computed(() => {
  if (!props.active) return undefined
  if (props.tierId) return membership.config.tiers.find((item) => item.id === props.tierId)
  return membership.config.tiers[0]
})
const badgeTitle = computed(() => tier.value?.badge_label || tier.value?.name || '社区会员')
const badgeColor = computed(() => /^#[0-9a-f]{6}$/i.test(tier.value?.badge_color || '') ? tier.value!.badge_color : '#f59e0b')

onMounted(() => membership.load())
</script>

<template>
  <span v-if="active" class="membership-badge" :class="{ 'membership-badge--custom': tier?.badge_url }" :title="badgeTitle" :style="{ '--membership-badge-color': badgeColor }">
    <img v-if="tier?.badge_url" :src="tier.badge_url" alt="" />
    <AppIcon v-else name="crown" size="14" :stroke-width="2.2" />
  </span>
</template>
