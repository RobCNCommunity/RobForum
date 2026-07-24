<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { fetchPublicCaptcha, type CaptchaConfig } from '@/api'

type GeetestInstance = {
  onReady?: (callback: () => void) => void
  onSuccess: (callback: () => void) => void
  onError: (callback: () => void) => void
  onClose: (callback: () => void) => void
  getValidate: () => Record<string, string> | undefined
  showCaptcha: () => boolean | void
  reset: () => void
  destroy?: () => void
}

declare global {
  interface Window {
    initGeetest4?: (options: Record<string, unknown>, callback: (captcha: GeetestInstance) => void) => void
  }
}

const emit = defineEmits<{ token: [value: string]; required: [value: boolean] }>()

let instance: GeetestInstance | null = null
let initialization: Promise<boolean> | null = null
let disposed = false
let pending: { resolve: (token: string) => void; reject: (error: Error) => void } | null = null

const geetestScriptSelector = 'script[data-geetest-v4]'
const geetestScriptURL = 'https://static.geetest.com/v4/gt4.js'

function loadScript() {
  if (window.initGeetest4) return Promise.resolve()
  return new Promise<void>((resolve, reject) => {
    let script = document.querySelector<HTMLScriptElement>(geetestScriptSelector)
    let settled = false
    const timeout = window.setTimeout(() => finish(new Error('captcha_load_failed')), 15000)

    const cleanup = () => {
      window.clearTimeout(timeout)
      script?.removeEventListener('load', onLoad)
      script?.removeEventListener('error', onError)
    }
    const finish = (error?: Error) => {
      if (settled) return
      settled = true
      cleanup()
      if (error) {
        script?.remove()
        reject(error)
      } else {
        resolve()
      }
    }
    const onLoad = () => {
      if (window.initGeetest4) finish()
      else finish(new Error('captcha_load_failed'))
    }
    const onError = () => finish(new Error('captcha_load_failed'))

    if (!script || script.dataset.geetestV4State === 'failed') {
      script?.remove()
      script = document.createElement('script')
      script.src = geetestScriptURL
      script.async = true
      script.dataset.geetestV4 = 'true'
      script.dataset.geetestV4State = 'loading'
      script.addEventListener('load', onLoad, { once: true })
      script.addEventListener('error', onError, { once: true })
      document.head.appendChild(script)
      return
    }

    script.addEventListener('load', onLoad, { once: true })
    script.addEventListener('error', onError, { once: true })
    if (window.initGeetest4) finish()
  })
}

function settle(error?: Error, token = '') {
  const current = pending
  if (!current) return
  pending = null
  emit('token', token)
  if (error) current.reject(error)
  else current.resolve(token)
}

async function initialize(config: CaptchaConfig) {
  await loadScript()
  if (!window.initGeetest4) throw new Error('captcha_load_failed')

  await new Promise<void>((resolve, reject) => {
    let settled = false
    const timeout = window.setTimeout(() => finish(new Error('captcha_load_failed')), 15000)
    const finish = (error?: Error) => {
      if (settled) return
      settled = true
      window.clearTimeout(timeout)
      if (error) reject(error)
      else resolve()
    }

    try {
      window.initGeetest4!({ captchaId: config.site_key, product: 'bind', language: 'zho' }, (captcha) => {
        if (disposed) {
          captcha.destroy?.()
          finish(new Error('captcha_unmounted'))
          return
        }
        instance = captcha
        captcha.onSuccess(() => {
          const payload = captcha.getValidate()
          if (!payload || Object.keys(payload).length === 0) {
            settle(new Error('captcha_invalid'))
            return
          }
          settle(undefined, JSON.stringify(payload))
        })
        captcha.onError(() => settle(new Error('captcha_failed')))
        captcha.onClose(() => settle(new Error('captcha_cancelled')))
        if (captcha.onReady) captcha.onReady(() => finish())
        else finish()
      })
    } catch {
      finish(new Error('captcha_load_failed'))
    }
  })
}

async function ensureReady() {
  if (initialization) return initialization
  initialization = (async () => {
    const config = await fetchPublicCaptcha()
    const required = config.enabled && config.provider === 'gt4' && config.adapter_status === 'ready'
    emit('required', required)
    if (!required) return false
    await initialize(config)
    return true
  })().catch((error) => {
    initialization = null
    emit('required', true)
    throw error
  })
  return initialization
}

async function verify() {
  const required = await ensureReady()
  if (!required) return ''
  if (!instance) throw new Error('captcha_load_failed')
  if (pending) throw new Error('captcha_in_progress')

  return new Promise<string>((resolve, reject) => {
    try {
      instance!.reset()
      pending = { resolve, reject }
      const shown = instance!.showCaptcha()
      if (shown === false) {
        settle(new Error('captcha_load_failed'))
      }
    } catch {
      settle(new Error('captcha_load_failed'))
    }
  })
}

function reset() {
  if (pending) settle(new Error('captcha_cancelled'))
  else emit('token', '')
  instance?.reset()
}

defineExpose({ verify, reset })

onMounted(() => {
  void ensureReady().catch(() => undefined)
})

onBeforeUnmount(() => {
  disposed = true
  if (pending) settle(new Error('captcha_unmounted'))
  instance?.destroy?.()
})
</script>

<template>
  <span class="rf-human-verification-anchor" aria-hidden="true" />
</template>
