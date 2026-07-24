<script setup lang="ts">
import { ref } from 'vue'
import type { Comment } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  files?: readonly File[]
  previewUrls?: readonly string[]
  sending?: boolean
  replyingTo?: Comment | null
  compact?: boolean
}>(), {
  files: () => [],
  previewUrls: () => [],
  sending: false,
  replyingTo: null,
  compact: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  submit: []
  'select-files': [files: File[]]
  'remove-file': [index: number]
  'cancel-reply': []
}>()

const auth = useAuthStore()
const input = ref<HTMLTextAreaElement | null>(null)

function updateContent(event: Event) {
  if (event.target instanceof HTMLTextAreaElement) emit('update:modelValue', event.target.value)
}

function chooseFiles(event: Event) {
  if (!(event.target instanceof HTMLInputElement)) return
  emit('select-files', Array.from(event.target.files || []))
  event.target.value = ''
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || (!event.ctrlKey && !event.metaKey)) return
  event.preventDefault()
  emit('submit')
}

function isVideo(index: number) {
  return props.files[index]?.type.startsWith('video/') || false
}

function focus() {
  input.value?.focus()
}

defineExpose({ focus })
</script>

<template>
  <form class="rf-comment-composer" :class="{ compact }" @submit.prevent="emit('submit')">
    <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="compact ? 34 : 42" />
    <div>
      <div v-if="replyingTo" class="rf-comment-replying">
        <span>回复 <strong>@{{ replyingTo.author_name }}</strong></span>
        <button type="button" aria-label="取消回复指定评论" @click="emit('cancel-reply')"><AppIcon name="close" size="15" /></button>
      </div>
      <textarea
        ref="input"
        :value="modelValue"
        rows="2"
        maxlength="5000"
        :placeholder="replyingTo ? `回复 @${replyingTo.author_name}` : '发布你的回复'"
        @input="updateContent"
        @keydown="handleKeydown"
      />
      <div v-if="previewUrls.length" class="rf-comment-previews">
        <figure v-for="(url, index) in previewUrls" :key="url">
          <video v-if="isVideo(index)" :src="url" controls playsinline preload="metadata" />
          <img v-else :src="url" alt="待发送图片" />
          <button type="button" aria-label="移除媒体" @click="emit('remove-file', index)"><AppIcon name="close" size="15" /></button>
        </figure>
      </div>
      <footer>
        <label aria-label="添加图片或视频" title="添加图片或视频">
          <AppIcon name="photo" size="19" />
          <input type="file" accept="image/png,image/jpeg,video/mp4,video/webm" multiple @change="chooseFiles" />
        </label>
        <span>{{ modelValue.length }}/5000</span>
        <button type="submit" :disabled="(!modelValue.trim() && !files.length) || sending">{{ sending ? '发布中' : '回复' }}</button>
      </footer>
    </div>
  </form>
</template>

<style scoped>
.rf-comment-composer { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 11px; padding: 13px 16px 10px; border-bottom: 1px solid var(--rf-line); }
.rf-comment-composer > div { min-width: 0; }
.rf-comment-composer textarea { display: block; width: 100%; min-height: 58px; padding: 8px 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font: inherit; font-size: 17px; line-height: 1.45; resize: vertical; }
.rf-comment-replying { display: flex; min-height: 30px; align-items: center; justify-content: space-between; gap: 8px; color: var(--rf-muted); font-size: 12px; }
.rf-comment-replying strong { color: var(--primary); }
.rf-comment-replying button { display: inline-grid; width: 28px; height: 28px; place-items: center; border-radius: 50%; color: var(--rf-muted); background: transparent; }
.rf-comment-replying button:hover { color: var(--primary); background: var(--rf-bg-hover); }
.rf-comment-previews { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; padding: 5px 0 10px; }
.rf-comment-previews figure { position: relative; aspect-ratio: 1 / 1; margin: 0; overflow: hidden; border-radius: 8px; background: #000; }
.rf-comment-previews img, .rf-comment-previews video { display: block; width: 100%; height: 100%; object-fit: contain; }
.rf-comment-previews figure > button { position: absolute; top: 4px; right: 4px; display: grid; width: 26px; height: 26px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .74); }
.rf-comment-composer footer { display: flex; min-height: 38px; align-items: center; justify-content: flex-end; gap: 12px; border-top: 1px solid var(--rf-line); }
.rf-comment-composer footer label { display: inline-grid; width: 34px; height: 34px; margin-right: auto; place-items: center; border-radius: 50%; color: var(--primary); cursor: pointer; }
.rf-comment-composer footer label:hover { background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-comment-composer footer input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }
.rf-comment-composer footer span { color: var(--rf-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.rf-comment-composer footer > button { min-width: 68px; min-height: 34px; padding: 0 16px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-weight: 700; }
.rf-comment-composer footer > button:disabled { cursor: not-allowed; opacity: .45; }
.rf-comment-composer.compact { grid-template-columns: 34px minmax(0, 1fr); gap: 9px; padding: 11px 14px; }
.rf-comment-composer.compact textarea { min-height: 50px; padding-block: 6px; font-size: 15px; }
.rf-comment-composer.compact footer { min-height: 34px; gap: 9px; }
.rf-comment-composer.compact footer > button { min-width: 60px; min-height: 32px; padding-inline: 14px; font-size: 13px; }
@media (max-width: 560px) {
  .rf-comment-composer { grid-template-columns: 36px minmax(0, 1fr); gap: 8px; padding-inline: 12px; }
  .rf-comment-composer textarea { font-size: 16px; }
}
</style>
