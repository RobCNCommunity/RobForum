<script setup lang="ts">
import { computed } from 'vue'
import { ImagePreview } from '@nutui/nutui'
import type { PostMedia } from '@/api'

const props = withDefaults(defineProps<{
  media?: PostMedia[]
  compact?: boolean
  preview?: boolean
  customPreview?: boolean
}>(), {
  media: () => [],
  compact: false,
  preview: true,
  customPreview: false,
})

const emit = defineEmits<{
  'open-preview': [index: number]
}>()

const items = computed(() => props.media.slice(0, 4))
const imageItems = computed(() => items.value.filter((item) => isImage(item)))
const layoutClass = computed(() => `count-${items.value.length}`)

function isImage(item: PostMedia) {
  return item.mime_type.startsWith('image/')
}

function isVideo(item: PostMedia) {
  return item.mime_type.startsWith('video/')
}

function ratio(item: PostMedia) {
  if (!item.width || !item.height) return '16 / 9'
  return String(Math.min(1.9, Math.max(0.8, item.width / item.height)))
}

function openPreview(event: MouseEvent, index: number) {
  const item = items.value[index]
  if (!item || !isImage(item) || !props.preview) return
  event.preventDefault()
  event.stopPropagation()
  const imageIndex = imageItems.value.findIndex((candidate) => candidate.id === item.id && candidate.url === item.url)
  if (imageIndex < 0) return
  if (props.customPreview) {
    emit('open-preview', imageIndex)
    return
  }
  ImagePreview({
    show: true,
    images: imageItems.value.map((candidate) => ({ src: candidate.url })),
    initNo: imageIndex,
    contentClose: true,
    closeable: true,
    isLoop: imageItems.value.length > 1,
    maxZoom: 4,
  })
}
</script>

<template>
  <div
    v-if="items.length"
    class="rf-post-media-grid"
    :class="[layoutClass, { compact, interactive: preview && imageItems.length }]"
    :aria-label="`帖子媒体，共 ${items.length} 个`"
  >
    <component
      :is="isImage(item) && preview ? 'button' : 'div'"
      v-for="(item, index) in items"
      :key="item.id || item.url"
      class="rf-post-media-item"
      :type="isImage(item) && preview ? 'button' : undefined"
      :aria-label="isImage(item) && preview ? `预览第 ${index + 1} 张图片` : isVideo(item) ? `第 ${index + 1} 个视频` : undefined"
      :style="items.length === 1 ? { aspectRatio: ratio(item) } : undefined"
      @click="openPreview($event, index)"
    >
      <video v-if="isVideo(item)" :src="item.url" controls playsinline preload="metadata" @click.stop @pointerdown.stop />
      <nut-image v-else :src="item.url" fit="cover" position="center" lazy-load>
        <template #loading><span class="rf-media-loading" /></template>
        <template #error><span class="rf-media-error">图片加载失败</span></template>
      </nut-image>
    </component>
  </div>
</template>

<style scoped>
.rf-post-media-grid {
  display: grid;
  width: 100%;
  margin-top: 11px;
  overflow: hidden;
  border: 1px solid var(--rf-line);
  border-radius: 16px;
  background: var(--rf-bg-subtle);
  gap: 2px;
}
.rf-post-media-item {
  position: relative;
  display: block;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  padding: 0;
  color: var(--rf-muted);
  background: var(--rf-bg-subtle);
}
.rf-post-media-grid.interactive .rf-post-media-item {
  cursor: zoom-in;
}
.rf-post-media-item :deep(.nut-image),
.rf-post-media-item :deep(.nut-img),
.rf-post-media-item video {
  width: 100%;
  height: 100%;
}
.rf-post-media-item video { display: block; object-fit: contain; background: #000; }
.rf-post-media-item :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: filter 150ms ease-out, transform 180ms ease-out;
}
.rf-post-media-grid.interactive .rf-post-media-item:hover :deep(img) {
  filter: brightness(.94);
  transform: scale(1.01);
}
.count-1 {
  grid-template-columns: minmax(0, 1fr);
}
.count-1 .rf-post-media-item {
  min-height: 220px;
  max-height: 520px;
}
.count-2,
.count-4 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.count-2 .rf-post-media-item {
  aspect-ratio: 1 / 1;
}
.count-3 {
  grid-template: repeat(2, 190px) / minmax(0, 1.18fr) minmax(0, 1fr);
}
.count-3 .rf-post-media-item:first-child {
  grid-row: 1 / 3;
}
.count-4 {
  grid-template-rows: repeat(2, 180px);
}
.compact.count-1 .rf-post-media-item {
  min-height: 190px;
  max-height: 420px;
}
.compact.count-3 {
  grid-template-rows: repeat(2, 142px);
}
.compact.count-4 {
  grid-template-rows: repeat(2, 138px);
}
.rf-media-loading,
.rf-media-error {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
}
.rf-media-loading {
  background: linear-gradient(90deg, var(--rf-bg-subtle), var(--rf-bg-hover), var(--rf-bg-subtle));
  background-size: 200% 100%;
  animation: rf-media-loading 1.2s ease-in-out infinite;
}
.rf-media-error {
  min-height: 110px;
  color: var(--rf-muted);
  font-size: 12px;
}
@keyframes rf-media-loading {
  from { background-position: 100% 0; }
  to { background-position: -100% 0; }
}
@media (max-width: 560px) {
  .rf-post-media-grid { border-radius: 14px; }
  .count-1 .rf-post-media-item,
  .compact.count-1 .rf-post-media-item { min-height: 180px; max-height: 430px; }
  .count-3,
  .compact.count-3 { grid-template-rows: repeat(2, 112px); }
  .count-4,
  .compact.count-4 { grid-template-rows: repeat(2, 108px); }
}
@media (prefers-reduced-motion: reduce) {
  .rf-post-media-item :deep(img) { transition: none; }
  .rf-media-loading { animation: none; }
}
</style>
