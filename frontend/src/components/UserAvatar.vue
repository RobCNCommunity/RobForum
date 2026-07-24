<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{
  src?: string
  name?: string
  size?: number
}>(), {
  src: '',
  name: '',
  size: 42,
})

const style = computed(() => ({
  '--rf-avatar-size': `${props.size}px`,
  '--rf-avatar-icon-size': `${Math.max(13, Math.round(props.size * 0.46))}px`,
}))
</script>

<template>
  <span class="rf-user-avatar" :style="style" :aria-label="props.name ? `${props.name}的头像` : '用户头像'" role="img">
    <img v-if="props.src" :src="props.src" alt="" />
    <AppIcon v-else name="user" :size="Math.max(13, Math.round(props.size * 0.46))" />
  </span>
</template>

<style scoped>
.rf-user-avatar {
  display: inline-grid;
  width: var(--rf-avatar-size);
  height: var(--rf-avatar-size);
  min-width: var(--rf-avatar-size);
  min-height: var(--rf-avatar-size);
  max-width: var(--rf-avatar-size);
  max-height: var(--rf-avatar-size);
  flex: 0 0 var(--rf-avatar-size);
  aspect-ratio: 1 / 1;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  color: var(--rf-muted);
  background: var(--rf-bg-subtle);
  box-shadow: inset 0 0 0 1px var(--rf-line);
  vertical-align: middle;
}
.rf-user-avatar img {
  display: block;
  width: 100%;
  height: 100%;
  aspect-ratio: 1 / 1;
  object-fit: cover;
}
.rf-user-avatar :deep(svg) {
  display: block;
  width: var(--rf-avatar-icon-size);
  height: var(--rf-avatar-icon-size);
}
</style>
