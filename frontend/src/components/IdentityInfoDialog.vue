<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{
  open: boolean
  kind: 'verification' | 'membership'
  title: string
  label: string
  description: string
  imageUrl?: string
  color?: string
}>(), { imageUrl: '', color: '#1d9bf0' })

const emit = defineEmits<{ close: [] }>()
function close() { emit('close') }
function handleKeydown(event: KeyboardEvent) { if (event.key === 'Escape' && props.open) close() }
onMounted(() => window.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="close">
      <section class="rf-dialog rf-identity-dialog" role="dialog" aria-modal="true" :aria-label="title">
        <header><div><h2>{{ title }}</h2><p>{{ label }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="close"><AppIcon name="close" size="18" /></button></header>
        <div class="rf-identity-dialog-body">
          <span class="rf-identity-dialog-mark" :style="{ '--identity-color': color }"><img v-if="imageUrl" :src="imageUrl" alt="" /><AppIcon v-else :name="kind === 'verification' ? 'check' : 'crown'" size="29" /></span>
          <div><strong>{{ label }}</strong><p>{{ description }}</p></div>
        </div>
        <footer><button type="button" class="rf-primary-button" @click="close">知道了</button></footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.rf-identity-dialog { width: min(100%, 420px); }
.rf-identity-dialog-body { display: grid; grid-template-columns: 58px minmax(0, 1fr); align-items: center; gap: 14px; padding: 22px 20px 8px; }
.rf-identity-dialog-mark { display: grid; width: 58px; height: 58px; place-items: center; overflow: hidden; border: 1px solid color-mix(in srgb, var(--identity-color) 28%, var(--rf-line)); border-radius: 50%; color: var(--identity-color); background: color-mix(in srgb, var(--identity-color) 9%, var(--rf-bg)); }
.rf-identity-dialog-mark img { width: 36px; height: 36px; object-fit: contain; }
.rf-identity-dialog-body strong { font-size: 17px; }
.rf-identity-dialog-body p { margin: 5px 0 0; color: var(--rf-muted); font-size: 13px; line-height: 1.55; }
@media (max-width: 560px) { .rf-identity-dialog-body { padding-inline: 16px; } .rf-identity-dialog > footer .rf-primary-button { width: 100%; } }
</style>
