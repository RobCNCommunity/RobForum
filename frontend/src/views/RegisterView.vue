<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchSiteSettings, sendRegistrationVerification, type SiteSettings } from '@/api'
import { useAuthStore } from '@/stores/auth'
import HumanVerification from '@/components/HumanVerification.vue'
import AppIcon from '@/components/AppIcon.vue'

type RegisterStep = 'identity' | 'code' | 'password'

const auth = useAuthStore()
const router = useRouter()
const verification = ref<InstanceType<typeof HumanVerification> | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)
const emailInput = ref<HTMLInputElement | null>(null)
const codeInput = ref<HTMLInputElement | null>(null)
const passwordInput = ref<HTMLInputElement | null>(null)
const visible = ref(true)
const isMobile = ref(typeof window !== 'undefined' && window.innerWidth <= 560)
const site = ref<SiteSettings | null>(null)
const siteLoading = ref(true)
const step = ref<RegisterStep>('identity')
const name = ref('')
const email = ref('')
const password = ref('')
const confirm = ref('')
const emailCode = ref('')
const cooldown = ref(0)
const loading = ref(false)
const sending = ref(false)
let timer: number | undefined
let cooldownUntil = 0

const emailVerificationEnabled = computed(() => site.value?.require_email_verification === true)
const totalSteps = computed(() => emailVerificationEnabled.value ? 3 : 2)
const stepNumber = computed(() => {
  if (step.value === 'identity') return 1
  if (step.value === 'code') return 2
  return emailVerificationEnabled.value ? 3 : 2
})
const siteName = computed(() => site.value?.site_name || '罗布玩家社区')
const brandImage = computed(() => site.value?.logo_url || site.value?.avatar_url || '')
const brandInitial = computed(() => siteName.value.trim().charAt(0).toUpperCase() || 'R')
const heading = computed(() => {
  if (step.value === 'code') return '输入验证码'
  if (step.value === 'password') return '设置密码'
  return '创建你的账号'
})

function normalizedEmail() {
  return email.value.trim().toLowerCase()
}

function cooldownKey() {
  return `robforum.registration-cooldown:${normalizedEmail()}`
}

function clearTimer() {
  if (timer) window.clearInterval(timer)
  timer = undefined
}

function updateCooldown() {
  cooldown.value = Math.max(0, Math.ceil((cooldownUntil - Date.now()) / 1000))
  if (!cooldown.value) {
    clearTimer()
    try { window.sessionStorage.removeItem(cooldownKey()) } catch { /* storage is optional */ }
  }
}

function startCooldown() {
  cooldownUntil = Date.now() + 60_000
  try { window.sessionStorage.setItem(cooldownKey(), String(cooldownUntil)) } catch { /* storage is optional */ }
  updateCooldown()
  clearTimer()
  timer = window.setInterval(updateCooldown, 500)
}

function restoreCooldown() {
  let value = 0
  try { value = Number(window.sessionStorage.getItem(cooldownKey()) || 0) } catch { /* storage is optional */ }
  cooldownUntil = Number.isFinite(value) ? value : 0
  updateCooldown()
  if (cooldown.value && !timer) timer = window.setInterval(updateCooldown, 500)
}

function updateViewport() {
  isMobile.value = window.innerWidth <= 560
}

function goHome() {
  if (router.currentRoute.value.path === '/register') router.push('/')
}

function validateIdentity() {
  const displayName = name.value.trim()
  const address = normalizedEmail()
  if (displayName.length < 1) {
    Notify.warn('请输入昵称')
    nextTick(() => nameInput.value?.focus())
    return false
  }
  if (displayName.length > 80) {
    Notify.warn('昵称不能超过 80 个字符')
    nextTick(() => nameInput.value?.focus())
    return false
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(address)) {
    Notify.warn('请输入有效的邮箱地址')
    nextTick(() => emailInput.value?.focus())
    return false
  }
  name.value = displayName
  email.value = address
  return true
}

function captchaWasCancelled(cause: unknown) {
  return cause instanceof Error && cause.message === 'captcha_cancelled'
}

function captchaFailed(cause: unknown) {
  return cause instanceof Error && cause.message.startsWith('captcha_')
}

async function requestCode() {
  if (sending.value || cooldown.value || !validateIdentity()) return
  sending.value = true
  try {
    const captchaToken = await verification.value?.verify() || ''
    await sendRegistrationVerification(email.value, captchaToken)
    startCooldown()
    emailCode.value = ''
    step.value = 'code'
    Notify.success('验证码已发送')
    await nextTick()
    codeInput.value?.focus()
  } catch (cause) {
    if (captchaWasCancelled(cause)) return
    if (captchaFailed(cause)) Notify.danger('人机验证暂时无法完成，请稍后重试')
    else Notify.danger(errorMessage(cause, '验证码发送失败'))
    verification.value?.reset()
  } finally {
    sending.value = false
  }
}

async function resendCode() {
  await requestCode()
}

function continueFromCode() {
  if (!/^\d{6}$/.test(emailCode.value.trim())) {
    Notify.warn('请输入 6 位邮箱验证码')
    codeInput.value?.focus()
    return
  }
  step.value = 'password'
  nextTick(() => passwordInput.value?.focus())
}

function continueFromIdentity() {
  if (!validateIdentity()) return
  if (emailVerificationEnabled.value) {
    requestCode()
    return
  }
  step.value = 'password'
  nextTick(() => passwordInput.value?.focus())
}

function editIdentity() {
  step.value = 'identity'
  emailCode.value = ''
  password.value = ''
  confirm.value = ''
  nextTick(() => emailInput.value?.focus())
}

async function submit() {
  if (loading.value || step.value !== 'password') return
  if (password.value.length < 10) {
    Notify.warn('密码至少需要 10 个字符')
    passwordInput.value?.focus()
    return
  }
  if (password.value !== confirm.value) {
    Notify.warn('两次密码不一致')
    return
  }
  loading.value = true
  try {
    // With email verification enabled, the code sent through the CAPTCHA-gated
    // endpoint is the registration proof. A second GT4 challenge would consume
    // a new token and make the flow unnecessarily fail closed.
    const captchaToken = emailVerificationEnabled.value ? '' : await verification.value?.verify() || ''
    await auth.register(email.value, password.value, name.value, captchaToken, emailCode.value)
    Notify.success('注册成功')
    await router.push('/')
  } catch (cause) {
    if (captchaWasCancelled(cause)) return
    if (captchaFailed(cause)) Notify.danger('人机验证暂时无法完成，请稍后重试')
    else Notify.danger(errorMessage(cause, '注册失败'))
    verification.value?.reset()
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
  try {
    site.value = await fetchSiteSettings()
    restoreCooldown()
  } catch {
    Notify.danger('站点配置加载失败，请刷新页面重试')
  } finally {
    siteLoading.value = false
  }
  await nextTick()
  nameInput.value?.focus()
})

onBeforeUnmount(() => {
  clearTimer()
  window.removeEventListener('resize', updateViewport)
})
</script>

<template>
  <div class="auth-page auth-page--popup">
    <nut-popup
      v-model:visible="visible"
      :position="isMobile ? 'bottom' : 'center'"
      :transition="isMobile ? 'rf-login-sheet' : ''"
      :duration="isMobile ? 0.3 : 0.2"
      destroy-on-close
      :close-on-click-overlay="false"
      pop-class="rf-auth-popup rf-register-popup"
      @closed="goHome"
    >
      <section class="rf-register-dialog" aria-labelledby="register-title">
        <header class="rf-register-topbar">
          <button type="button" class="rf-auth-close" aria-label="关闭" title="关闭" @click="visible = false">
            <AppIcon name="close" size="20" />
          </button>
          <div class="rf-login-brand-center" aria-hidden="true">
            <img v-if="brandImage" :src="brandImage" :alt="siteName" />
            <span v-else class="rf-login-brand-fallback">{{ brandInitial }}</span>
          </div>
          <span class="rf-register-step">第 {{ stepNumber }} 步，共 {{ totalSteps }} 步</span>
        </header>

        <header class="auth-heading rf-register-heading">
          <h1 id="register-title">{{ heading }}</h1>
        </header>

        <HumanVerification ref="verification" />

        <div v-if="siteLoading" class="rf-auth-loading" aria-live="polite">加载中…</div>

        <form v-else-if="step === 'identity'" class="rf-auth-form rf-register-form" @submit.prevent="continueFromIdentity" @keydown.enter.prevent="continueFromIdentity">
          <label>昵称
            <div class="rf-field">
              <AppIcon name="user" size="18" />
              <input ref="nameInput" v-model="name" maxlength="80" autocomplete="nickname" placeholder="社区中显示的名字" required />
            </div>
          </label>
          <label>邮箱
            <div class="rf-field">
              <AppIcon name="mail" size="18" />
              <input ref="emailInput" v-model="email" type="email" autocomplete="email" placeholder="name@example.com" required @input="restoreCooldown" />
            </div>
          </label>
          <nut-button type="primary" block size="large" :loading="sending" :disabled="sending || (emailVerificationEnabled && cooldown > 0)" @click="continueFromIdentity">
            {{ emailVerificationEnabled && cooldown > 0 ? `${cooldown}s 后重发` : emailVerificationEnabled ? '获取验证码' : '继续' }}
          </nut-button>
        </form>

        <form v-else-if="step === 'code'" class="rf-auth-form rf-register-form" @submit.prevent="continueFromCode" @keydown.enter.prevent="continueFromCode">
          <div class="rf-register-email-summary">
            <span>{{ email }}</span>
            <button type="button" @click="editIdentity">修改</button>
          </div>
          <label>邮箱验证码
            <div class="rf-code-field">
              <input ref="codeInput" v-model="emailCode" maxlength="6" inputmode="numeric" autocomplete="one-time-code" placeholder="6 位验证码" />
              <button type="button" :disabled="cooldown > 0 || sending" @click="resendCode">
                {{ cooldown > 0 ? `${cooldown}s 后重发` : '重新获取' }}
              </button>
            </div>
          </label>
          <nut-button type="primary" block size="large" @click="continueFromCode">继续</nut-button>
        </form>

        <form v-else class="rf-auth-form rf-register-form" @submit.prevent="submit" @keydown.enter.prevent="submit">
          <div class="rf-register-email-summary">
            <span>{{ email }}</span>
            <button type="button" @click="editIdentity">修改</button>
          </div>
          <label>密码
            <div class="rf-field">
              <AppIcon name="lock" size="18" />
              <input ref="passwordInput" v-model="password" type="password" autocomplete="new-password" placeholder="至少 10 个字符" required />
            </div>
          </label>
          <label>确认密码
            <div class="rf-field">
              <AppIcon name="lock" size="18" />
              <input v-model="confirm" type="password" autocomplete="new-password" placeholder="再次输入密码" required />
            </div>
          </label>
          <nut-button type="primary" block size="large" :loading="loading || auth.loading" :disabled="loading" @click="submit">创建账号</nut-button>
        </form>

        <footer class="rf-login-footer">
          <span>已有账号？</span>
          <RouterLink to="/login">登录</RouterLink>
        </footer>
      </section>
    </nut-popup>
  </div>
</template>
