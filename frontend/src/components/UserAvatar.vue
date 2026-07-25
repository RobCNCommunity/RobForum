<script setup lang="ts">
import { computed } from 'vue'
import type { AvatarFrame } from '@/api'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{
  src?: string
  name?: string
  size?: number
  frame?: AvatarFrame | null
}>(), {
  src: '',
  name: '',
  size: 42,
})

const style = computed(() => ({
  '--rf-avatar-size': `${props.size}px`,
  '--rf-avatar-icon-size': `${Math.max(13, Math.round(props.size * 0.46))}px`,
  '--rf-frame-primary': props.frame?.primary_color || 'transparent',
  '--rf-frame-secondary': props.frame?.secondary_color || 'transparent',
}))
</script>

<template>
  <span class="rf-user-avatar" :style="style" :aria-label="props.name ? `${props.name}的头像` : '用户头像'" role="img">
    <span class="rf-user-avatar-image">
      <img v-if="props.src" :src="props.src" alt="" />
      <AppIcon v-else name="user" :size="Math.max(13, Math.round(props.size * 0.46))" />
    </span>
    <span v-if="props.frame" class="rf-avatar-frame" :class="`is-${props.frame.style}`" :title="props.frame.name" aria-hidden="true" />
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
  position: relative;
  vertical-align: middle;
}
.rf-user-avatar-image {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  color: var(--rf-muted);
  background: var(--rf-bg-subtle);
  box-shadow: inset 0 0 0 1px var(--rf-line);
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
.rf-avatar-frame {
  position: absolute;
  z-index: 1;
  inset: -3px;
  pointer-events: none;
  border-radius: 50%;
}
.rf-avatar-frame.is-ring {
  border: 3px solid var(--rf-frame-primary);
  box-shadow: 0 0 0 1px var(--rf-frame-secondary);
}
.rf-avatar-frame.is-double {
  border: 2px solid var(--rf-frame-primary);
  box-shadow: inset 0 0 0 2px var(--rf-frame-secondary), 0 0 0 2px var(--rf-frame-secondary);
}
.rf-avatar-frame.is-glow {
  border: 2px solid var(--rf-frame-primary);
  box-shadow: 0 0 5px var(--rf-frame-primary), 0 0 10px var(--rf-frame-secondary);
}
.rf-avatar-frame.is-pixel {
  inset: -2px;
  border: 3px solid var(--rf-frame-primary);
  border-radius: 4px;
  box-shadow: 3px 3px 0 var(--rf-frame-secondary), -2px -2px 0 var(--rf-frame-secondary);
}
.rf-avatar-frame.is-halo {
  inset: -4px;
  border: 2px solid var(--rf-frame-primary);
  box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--rf-frame-secondary) 60%, transparent), 0 0 8px var(--rf-frame-secondary);
}
.rf-avatar-frame.is-halo::before {
  position: absolute;
  top: -5px;
  right: 15%;
  left: 15%;
  height: 4px;
  border: 2px solid var(--rf-frame-secondary);
  border-radius: 50%;
  content: '';
}
</style>
