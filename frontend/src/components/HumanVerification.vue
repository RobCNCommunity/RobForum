<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchPublicCaptcha, type CaptchaConfig } from '@/api'

type GeetestInstance = {
  appendTo: (target: HTMLElement) => void
  onSuccess: (callback: () => void) => void
  onError: (callback: () => void) => void
  onClose: (callback: () => void) => void
  getValidate: () => Record<string, string> | undefined
  reset: () => void
  destroy?: () => void
}

declare global {
  interface Window {
    initGeetest4?: (options: Record<string, unknown>, callback: (captcha: GeetestInstance) => void) => void
  }
}

const emit = defineEmits<{ token: [value: string]; required: [value: boolean] }>()
const container = ref<HTMLElement | null>(null)
const loading = ref(true)
const failed = ref(false)
let instance: GeetestInstance | null = null

function loadScript() {
  if (window.initGeetest4) return Promise.resolve()
  const existing = document.querySelector<HTMLScriptElement>('script[data-geetest-v4]')
  if (existing) return new Promise<void>((resolve, reject) => {
    existing.addEventListener('load', () => resolve(), { once: true })
    existing.addEventListener('error', () => reject(new Error('GT4 script load failed')), { once: true })
  })
  return new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = 'https://static.geetest.com/v4/gt4.js'
    script.async = true
    script.dataset.geetestV4 = 'true'
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('GT4 script load failed'))
    document.head.appendChild(script)
  })
}

async function initialize(config: CaptchaConfig) {
  await loadScript()
  await nextTick()
  if (!window.initGeetest4 || !container.value) throw new Error('GT4 unavailable')
  window.initGeetest4({ captchaId: config.site_key, product: 'float', language: 'zho' }, (captcha) => {
    instance = captcha
    captcha.appendTo(container.value!)
    captcha.onSuccess(() => emit('token', JSON.stringify(captcha.getValidate() || {})))
    captcha.onError(() => emit('token', ''))
    captcha.onClose(() => emit('token', ''))
    loading.value = false
  })
}

function reset() {
  emit('token', '')
  instance?.reset()
}

defineExpose({ reset })

onMounted(async () => {
  try {
    const config = await fetchPublicCaptcha()
    const required = config.enabled && config.provider === 'gt4' && config.adapter_status === 'ready'
    emit('required', required)
    if (!required) { loading.value = false; return }
    await initialize(config)
  } catch {
    failed.value = true
    loading.value = false
    emit('required', true)
  }
})

onBeforeUnmount(() => instance?.destroy?.())
</script>

<template>
  <div v-if="loading || failed || instance" class="human-verification">
    <div v-if="loading" class="rf-verification-loading">正在加载人机验证…</div>
    <div v-else-if="failed" class="rf-verification-error">人机验证加载失败，请刷新页面重试</div>
    <div ref="container" class="human-verification-container" />
  </div>
</template>
