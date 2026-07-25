<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppIcon from './AppIcon.vue'
import IdentityInfoDialog from './IdentityInfoDialog.vue'
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
const open = ref(false)
const membershipDescription = computed(() => {
  const benefits = []
  if (tier.value?.feed_priority) benefits.push('动态优先展示')
  if (tier.value?.post_review_exempt) benefits.push('发帖免人工预审')
  const suffix = benefits.length ? ` 当前阶层包含${benefits.join('、')}等权益。` : ''
  return `该用户当前拥有有效的“${tier.value?.name || '社区会员'}”会员身份。${suffix}`
})

onMounted(() => membership.load())
</script>

<template>
  <button v-if="active" type="button" class="membership-badge" :class="{ 'membership-badge--custom': tier?.badge_url }" :title="badgeTitle" :aria-label="`查看会员信息：${badgeTitle}`" :style="{ '--membership-badge-color': badgeColor }" @click.stop.prevent="open = true">
    <img v-if="tier?.badge_url" :src="tier.badge_url" alt="" />
    <AppIcon v-else name="crown" size="14" :stroke-width="2.2" />
  </button>
  <IdentityInfoDialog :open="open" kind="membership" title="会员身份" :label="badgeTitle" :description="membershipDescription" :image-url="tier?.badge_url" :color="badgeColor" @close="open = false" />
</template>
