<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { User } from '@/api'
import UserAvatar from './UserAvatar.vue'
import VerifiedBadge from './VerifiedBadge.vue'
import MembershipBadge from './MembershipBadge.vue'

const props = withDefaults(defineProps<{ user: User; busy?: boolean }>(), { busy: false })
const emit = defineEmits<{ logout: [] }>()
</script>

<template>
  <div class="rf-sidebar-account">
    <RouterLink :to="`/users/${props.user.id}`" class="rf-sidebar-account-profile" :aria-label="`查看 ${props.user.display_name} 的个人主页`">
      <UserAvatar :src="props.user.avatar_url" :name="props.user.display_name" :size="38" />
      <span class="rf-sidebar-account-name">
        <strong>{{ props.user.display_name }}</strong>
        <VerifiedBadge :verified="props.user.blue_verified" :label="props.user.verification_label" /><MembershipBadge :active="props.user.member_active" :tier-id="props.user.membership_tier_id" />
      </span>
    </RouterLink>
    <button type="button" class="rf-sidebar-account-logout" :disabled="props.busy" @click="emit('logout')">
      {{ props.busy ? '退出中…' : '退出登录' }}
    </button>
  </div>
</template>
