<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PostMedia } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const props = withDefaults(defineProps<{
  open: boolean
  media: readonly PostMedia[]
  startIndex?: number
}>(), {
  startIndex: 0,
})

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const dialog = ref<HTMLElement | null>(null)
const canvas = ref<HTMLElement | null>(null)
const image = ref<HTMLImageElement | null>(null)
const index = ref(0)
const scale = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const dragging = ref(false)
let pointerID: number | undefined
let lastX = 0
let lastY = 0
let bodyOverflow = ''
let bodyLocked = false

const current = computed(() => props.media[index.value])
const imageTransform = computed(() => `translate3d(${offsetX.value}px, ${offsetY.value}px, 0) scale(${scale.value})`)

function clampIndex(value: number) {
  if (!props.media.length) return 0
  return Math.min(Math.max(value, 0), props.media.length - 1)
}

function resetTransform() {
  scale.value = 1
  offsetX.value = 0
  offsetY.value = 0
  dragging.value = false
  pointerID = undefined
}

function clampOffsets() {
  const viewport = canvas.value
  const mediaImage = image.value
  if (!viewport || !mediaImage) return
  const maxX = Math.max(0, (mediaImage.clientWidth * scale.value - viewport.clientWidth) / 2)
  const maxY = Math.max(0, (mediaImage.clientHeight * scale.value - viewport.clientHeight) / 2)
  offsetX.value = Math.min(maxX, Math.max(-maxX, offsetX.value))
  offsetY.value = Math.min(maxY, Math.max(-maxY, offsetY.value))
}

function zoomTo(value: number, clientX?: number, clientY?: number) {
  const nextScale = Math.min(5, Math.max(1, value))
  const viewport = canvas.value
  if (viewport && clientX !== undefined && clientY !== undefined && scale.value > 0) {
    const bounds = viewport.getBoundingClientRect()
    const cursorX = clientX - bounds.left - bounds.width / 2
    const cursorY = clientY - bounds.top - bounds.height / 2
    const ratio = nextScale / scale.value
    offsetX.value = cursorX - (cursorX - offsetX.value) * ratio
    offsetY.value = cursorY - (cursorY - offsetY.value) * ratio
  }
  scale.value = Number(nextScale.toFixed(3))
  if (scale.value === 1) {
    offsetX.value = 0
    offsetY.value = 0
  } else {
    clampOffsets()
  }
}

function handleWheel(event: WheelEvent) {
  event.preventDefault()
  const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 18 : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? window.innerHeight : 1
  const factor = Math.exp(-event.deltaY * unit * 0.0014)
  zoomTo(scale.value * factor, event.clientX, event.clientY)
}

function startDrag(event: PointerEvent) {
  if (scale.value <= 1 || pointerID !== undefined) return
  pointerID = event.pointerId
  dragging.value = true
  lastX = event.clientX
  lastY = event.clientY
  if (event.currentTarget instanceof HTMLElement) event.currentTarget.setPointerCapture(event.pointerId)
}

function moveDrag(event: PointerEvent) {
  if (!dragging.value || pointerID !== event.pointerId) return
  offsetX.value += event.clientX - lastX
  offsetY.value += event.clientY - lastY
  lastX = event.clientX
  lastY = event.clientY
  clampOffsets()
}

function endDrag(event: PointerEvent) {
  if (pointerID !== event.pointerId) return
  dragging.value = false
  pointerID = undefined
}

function toggleZoom(event: MouseEvent) {
  zoomTo(scale.value > 1 ? 1 : 2.4, event.clientX, event.clientY)
}

function show(nextIndex: number) {
  if (!props.media.length) return
  index.value = (nextIndex + props.media.length) % props.media.length
  resetTransform()
}

function close() {
  emit('update:open', false)
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.open) return
  if (event.key === 'Escape') close()
  if (event.key === 'ArrowLeft') show(index.value - 1)
  if (event.key === 'ArrowRight') show(index.value + 1)
}

watch(() => props.open, async (open) => {
  if (open) {
    index.value = clampIndex(props.startIndex)
    resetTransform()
    bodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    bodyLocked = true
    await nextTick()
    dialog.value?.focus()
    return
  }
  if (bodyLocked) document.body.style.overflow = bodyOverflow
  bodyLocked = false
  resetTransform()
})

watch(() => props.startIndex, (value) => {
  if (props.open) show(clampIndex(value))
})

watch(() => props.media.length, () => {
  index.value = clampIndex(index.value)
  resetTransform()
})

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', clampOffsets)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', clampOffsets)
  if (bodyLocked) document.body.style.overflow = bodyOverflow
})
</script>

<template>
  <Teleport to="body">
    <Transition name="rf-image-viewer">
      <div v-if="open && current" ref="dialog" class="rf-image-viewer" role="dialog" aria-modal="true" aria-label="图片查看器" tabindex="-1">
        <main class="rf-image-viewer-stage" @wheel="handleWheel">
          <button type="button" class="rf-image-viewer-close" aria-label="关闭图片查看器" @click="close"><AppIcon name="close" size="22" /></button>
          <button v-if="media.length > 1" type="button" class="rf-image-viewer-nav prev" aria-label="上一张图片" @click="show(index - 1)"><AppIcon name="back" size="25" /></button>
          <figure
            ref="canvas"
            class="rf-image-viewer-canvas"
            :class="{ zoomed: scale > 1, dragging }"
            @pointerdown="startDrag"
            @pointermove="moveDrag"
            @pointerup="endDrag"
            @pointercancel="endDrag"
            @dblclick="toggleZoom"
          >
            <img ref="image" :src="current.url" alt="帖子图片" draggable="false" :style="{ transform: imageTransform }" @load="clampOffsets" />
          </figure>
          <button v-if="media.length > 1" type="button" class="rf-image-viewer-nav next" aria-label="下一张图片" @click="show(index + 1)"><AppIcon name="arrow" size="25" /></button>
          <div v-if="media.length > 1" class="rf-image-viewer-thumbs" aria-label="选择图片">
            <button v-for="(item, mediaIndex) in media" :key="item.id || item.url" type="button" :class="{ active: mediaIndex === index }" :aria-label="`查看第 ${mediaIndex + 1} 张图片`" @click="show(mediaIndex)"><img :src="item.url" alt="" /></button>
          </div>
          <div class="rf-image-viewer-zoom" aria-live="polite">{{ Math.round(scale * 100) }}%</div>
        </main>
        <aside class="rf-image-viewer-detail" aria-label="帖子详情和评论"><slot /></aside>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.rf-image-viewer { position: fixed; inset: 0; z-index: 1200; display: grid; grid-template-columns: minmax(0, 1fr) minmax(340px, 420px); overflow: hidden; color: #e7e9ea; background: #050505; outline: 0; }
.rf-image-viewer-stage { position: relative; display: grid; min-width: 0; min-height: 0; place-items: center; overflow: hidden; user-select: none; touch-action: none; }
.rf-image-viewer-close, .rf-image-viewer-nav { position: absolute; z-index: 4; display: inline-grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .62); transition: background-color 150ms ease-out, transform 120ms cubic-bezier(.22, 1, .36, 1); }
.rf-image-viewer-close { top: 18px; left: 18px; }
.rf-image-viewer-close:hover, .rf-image-viewer-nav:hover { background: rgba(15, 20, 25, .84); }
.rf-image-viewer-close:active { transform: scale(.94); }
.rf-image-viewer-nav { top: 50%; transform: translateY(-50%); }
.rf-image-viewer-nav:active { transform: translateY(-50%) scale(.94); }
.rf-image-viewer-nav.prev { left: 18px; }
.rf-image-viewer-nav.next { right: 18px; }
.rf-image-viewer-canvas { display: grid; width: 100%; height: 100%; margin: 0; place-items: center; cursor: zoom-in; }
.rf-image-viewer-canvas.zoomed { cursor: grab; }
.rf-image-viewer-canvas.dragging { cursor: grabbing; }
.rf-image-viewer-canvas img { display: block; max-width: min(100%, 1200px); max-height: 100%; object-fit: contain; will-change: transform; transition: transform 280ms cubic-bezier(.16, 1, .3, 1); }
.rf-image-viewer-canvas.dragging img { transition-duration: 45ms; transition-timing-function: cubic-bezier(.2, .8, .2, 1); }
.rf-image-viewer-thumbs { position: absolute; z-index: 4; right: 18px; bottom: 18px; left: 18px; display: flex; justify-content: center; gap: 8px; pointer-events: none; }
.rf-image-viewer-thumbs button { width: 52px; height: 52px; overflow: hidden; padding: 0; border: 2px solid transparent; border-radius: 8px; background: rgba(255, 255, 255, .12); opacity: .65; pointer-events: auto; transition: border-color 150ms ease-out, opacity 150ms ease-out, transform 180ms cubic-bezier(.22, 1, .36, 1); }
.rf-image-viewer-thumbs button:hover, .rf-image-viewer-thumbs button.active { opacity: 1; transform: translateY(-2px); }
.rf-image-viewer-thumbs button.active { border-color: var(--primary); }
.rf-image-viewer-thumbs img { width: 100%; height: 100%; object-fit: cover; }
.rf-image-viewer-zoom { position: absolute; z-index: 4; top: 18px; right: 18px; min-width: 58px; padding: 5px 9px; border-radius: var(--rf-pill); color: #fff; background: rgba(15, 20, 25, .62); font-size: 12px; font-variant-numeric: tabular-nums; text-align: center; }
.rf-image-viewer-detail { min-width: 0; min-height: 0; overflow-y: auto; overscroll-behavior: contain; border-left: 1px solid #2f3336; color: var(--rf-text); background: var(--rf-bg); }
.rf-image-viewer-enter-active { transition: opacity 180ms ease-out; }
.rf-image-viewer-leave-active { transition: opacity 120ms ease-in; }
.rf-image-viewer-enter-from, .rf-image-viewer-leave-to { opacity: 0; }
@media (max-width: 860px) {
  .rf-image-viewer { grid-template-columns: 1fr; grid-template-rows: minmax(250px, 58dvh) minmax(0, 42dvh); }
  .rf-image-viewer-stage { border-bottom: 1px solid #2f3336; }
  .rf-image-viewer-detail { border-left: 0; }
  .rf-image-viewer-close { top: 12px; left: 12px; }
  .rf-image-viewer-zoom { top: 12px; right: 12px; }
  .rf-image-viewer-nav.prev { left: 12px; }
  .rf-image-viewer-nav.next { right: 12px; }
  .rf-image-viewer-thumbs { right: 12px; bottom: 12px; left: 12px; }
  .rf-image-viewer-thumbs button { width: 44px; height: 44px; }
}
@media (prefers-reduced-motion: reduce) {
  .rf-image-viewer-canvas img, .rf-image-viewer-enter-active, .rf-image-viewer-leave-active { transition: none; }
}
</style>
