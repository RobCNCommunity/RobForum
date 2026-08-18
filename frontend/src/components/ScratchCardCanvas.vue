<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

const props = withDefaults(defineProps<{ content: string; ratio?: number }>(), { ratio: 0.3 })
const emit = defineEmits<{ revealed: [] }>()
const root = ref<HTMLElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
const drawing = ref(false)
const opened = ref(false)
let previousPoint: { x: number; y: number } | null = null
let resizeObserver: ResizeObserver | null = null
let moveCheckCounter = 0
let interacted = false

function paintCover() {
  const element = canvas.value
  const host = root.value
  if (!element || !host || opened.value || interacted) return
  const rect = host.getBoundingClientRect()
  if (!rect.width || !rect.height) return
  const ratio = Math.min(window.devicePixelRatio || 1, 2)
  element.width = Math.round(rect.width * ratio)
  element.height = Math.round(rect.height * ratio)
  const context = element.getContext('2d', { willReadFrequently: true })
  if (!context) return
  context.setTransform(ratio, 0, 0, ratio, 0, 0)
  context.globalCompositeOperation = 'source-over'
  context.fillStyle = '#728096'
  context.fillRect(0, 0, rect.width, rect.height)
  context.strokeStyle = 'rgba(255,255,255,.14)'
  context.lineWidth = 1
  for (let x = -rect.height; x < rect.width + rect.height; x += 22) {
    context.beginPath()
    context.moveTo(x, 0)
    context.lineTo(x + rect.height, rect.height)
    context.stroke()
  }
  context.fillStyle = '#ffffff'
  context.font = '700 16px MiSans, sans-serif'
  context.textAlign = 'center'
  context.textBaseline = 'middle'
  context.fillText('刮开查看结果', rect.width / 2, rect.height / 2)
}

function pointFromEvent(event: PointerEvent) {
  const rect = canvas.value?.getBoundingClientRect()
  if (!rect) return null
  return { x: event.clientX - rect.left, y: event.clientY - rect.top }
}

function eraseTo(point: { x: number; y: number }) {
  const element = canvas.value
  if (!element) return
  const context = element.getContext('2d', { willReadFrequently: true })
  if (!context) return
  const ratio = Math.min(window.devicePixelRatio || 1, 2)
  context.setTransform(ratio, 0, 0, ratio, 0, 0)
  context.globalCompositeOperation = 'destination-out'
  context.strokeStyle = 'rgba(0,0,0,1)'
  context.lineWidth = 34
  context.lineCap = 'round'
  context.lineJoin = 'round'
  context.beginPath()
  if (previousPoint) context.moveTo(previousPoint.x, previousPoint.y)
  else context.moveTo(point.x, point.y)
  context.lineTo(point.x, point.y)
  context.stroke()
  previousPoint = point
}

function revealIfReady() {
  const element = canvas.value
  if (!element || opened.value) return
  const context = element.getContext('2d', { willReadFrequently: true })
  if (!context) return
  const pixels = context.getImageData(0, 0, element.width, element.height).data
  let transparent = 0
  let sampled = 0
  for (let index = 3; index < pixels.length; index += 4 * 16) {
    sampled += 1
    if (pixels[index] < 32) transparent += 1
  }
  if (sampled && transparent / sampled >= props.ratio) {
    opened.value = true
    context.clearRect(0, 0, element.width, element.height)
    emit('revealed')
  }
}

function pointerDown(event: PointerEvent) {
  if (opened.value) return
  event.preventDefault()
  interacted = true
  try { canvas.value?.setPointerCapture(event.pointerId) } catch { /* Continue without capture on older browsers. */ }
  drawing.value = true
  moveCheckCounter = 0
  previousPoint = null
  const point = pointFromEvent(event)
  if (point) {
    eraseTo(point)
    moveCheckCounter += 1
    if (moveCheckCounter % 10 === 0) revealIfReady()
  }
}

function pointerMove(event: PointerEvent) {
  if (!drawing.value || opened.value) return
  event.preventDefault()
  const point = pointFromEvent(event)
  if (point) {
    eraseTo(point)
    moveCheckCounter += 1
    if (moveCheckCounter % 10 === 0) revealIfReady()
  }
}

function pointerUp(event: PointerEvent) {
  if (!drawing.value) return
  drawing.value = false
  previousPoint = null
  try {
    if (canvas.value?.hasPointerCapture(event.pointerId)) canvas.value.releasePointerCapture(event.pointerId)
  } catch {
    // Releasing capture is optional; reveal calculation must still run.
  }
  revealIfReady()
}

onMounted(async () => {
  await nextTick()
  paintCover()
  resizeObserver = new ResizeObserver(paintCover)
  if (root.value) resizeObserver.observe(root.value)
})
onBeforeUnmount(() => resizeObserver?.disconnect())
</script>

<template>
  <div ref="root" class="scratch-card">
    <div class="scratch-result"><span>本次结果</span><strong>{{ content }}</strong></div>
    <canvas ref="canvas" aria-label="刮刮卡覆盖层" @pointerdown="pointerDown" @pointermove="pointerMove" @pointerup="pointerUp" @pointercancel="pointerUp" />
  </div>
</template>

<style scoped>
.scratch-card { position: relative; width: min(100%, 360px); aspect-ratio: 8 / 5; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg); box-shadow: 0 10px 24px rgba(15,23,42,.12); }
.scratch-result { position: absolute; inset: 0; display: flex; align-items: center; flex-direction: column; justify-content: center; gap: 8px; padding: 20px; color: var(--rf-text); background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--primary) 9%, var(--rf-bg)) 0 12px, var(--rf-bg) 12px 24px); text-align: center; }
.scratch-result span { color: var(--rf-muted); font-size: 12px; }
.scratch-result strong { font-size: 28px; overflow-wrap: anywhere; }
canvas { position: absolute; inset: 0; width: 100%; height: 100%; cursor: crosshair; touch-action: none; }
@media (max-width: 390px) { .scratch-result strong { font-size: 22px; } }
</style>
