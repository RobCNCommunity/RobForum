<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchPublicOAuth, type PublicOAuthConfig } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import HumanVerification from '@/components/HumanVerification.vue'
import AppIcon from '@/components/AppIcon.vue'

type LoginStep = 'email' | 'password'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const site = useSiteStore()
const verification = ref<InstanceType<typeof HumanVerification> | null>(null)
const emailInput = ref<HTMLInputElement | null>(null)
const passwordInput = ref<HTMLInputElement | null>(null)
const visible = ref(true)
const step = ref<LoginStep>('email')
const emailError = ref('')
const passwordError = ref('')
const showPassword = ref(false)
const submitting = ref(false)
const form = reactive({ email: '', password: '' })
const oauthProviders = ref<PublicOAuthConfig[]>([])
const isMobile = ref(typeof window !== 'undefined' && window.innerWidth <= 560)

const siteName = computed(() => site.settings?.site_name || '罗布玩家社区')
const brandImage = computed(() => site.settings?.logo_url || site.settings?.avatar_url || '')
const brandInitial = computed(() => siteName.value.trim().charAt(0).toUpperCase() || 'R')
const headline = computed(() => step.value === 'email' ? '欢迎登录' : '输入你的密码')

function safeDestination() {
  const requested = typeof route.query.redirect === 'string'
    ? route.query.redirect
    : typeof route.query.return_to === 'string'
      ? route.query.return_to
      : '/'
  return requested.startsWith('/') && !requested.startsWith('//') && !requested.includes('\\') ? requested : '/'
}

function goHome() {
  if (router.currentRoute.value.path === '/login') router.push('/')
}

function validateEmail() {
  const email = form.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    emailError.value = '请输入有效的邮箱地址'
    return false
  }
  form.email = email
  emailError.value = ''
  return true
}

async function continueWithEmail() {
  if (!validateEmail()) {
    await nextTick()
    emailInput.value?.focus()
    return
  }
  step.value = 'password'
  verification.value?.reset()
  await nextTick()
  passwordInput.value?.focus()
}

function returnToEmail() {
  step.value = 'email'
  passwordError.value = ''
  verification.value?.reset()
  nextTick(() => emailInput.value?.focus())
}

async function submit() {
  if (submitting.value) return
  if (step.value === 'email') {
    await continueWithEmail()
    return
  }
  if (!validateEmail()) {
    returnToEmail()
    return
  }
  if (!form.password) {
    passwordError.value = '请输入密码'
    await nextTick()
    passwordInput.value?.focus()
    return
  }
  passwordError.value = ''
  submitting.value = true
  try {
    const captchaToken = await verification.value?.verify() || ''
    const result = await auth.login(form.email, form.password, captchaToken)
    if (result?.requires2FA) {
      sessionStorage.setItem('robforum:2fa-challenge', result.challengeToken)
      sessionStorage.setItem('robforum:2fa-return-to', safeDestination())
      await router.push('/login/2fa')
      return
    }
    Notify.success('登录成功')
    await router.push(safeDestination())
  } catch (cause) {
    if (cause instanceof Error && cause.message === 'captcha_cancelled') return
    if (cause instanceof Error && cause.message.startsWith('captcha_')) {
      Notify.danger('人机验证暂时无法完成，请稍后重试')
      return
    }
    Notify.danger(errorMessage(cause, '邮箱或密码不正确，请重试'))
    verification.value?.reset()
  } finally {
    submitting.value = false
  }
}

function openRegister() {
  visible.value = false
  router.push('/register')
}

function updateViewport() {
  isMobile.value = window.innerWidth <= 560
}

function oauthLogin(providerKey: string) {
  window.location.assign(`/api/v1/oauth/start?provider=${encodeURIComponent(providerKey)}&return_to=${encodeURIComponent(safeDestination())}`)
}

onMounted(async () => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
  try { oauthProviders.value = await fetchPublicOAuth() } catch { oauthProviders.value = [] }
  const failure = typeof route.query.oauth_error === 'string' ? route.query.oauth_error : ''
  if (failure) {
    const messages: Record<string, string> = {
      denied: '已取消第三方登录',
      email_unverified: '第三方账号邮箱尚未验证，无法安全登录',
      registration_disabled: '站点当前暂停新用户注册',
      unavailable: '第三方登录当前未启用',
    }
    Notify.danger(messages[failure] || '第三方登录失败，请重试')
  }
  await nextTick()
  emailInput.value?.focus()
})

onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))
</script>

<template>
  <div class="auth-page auth-page--popup">
    <nut-popup
      v-model:visible="visible"
      :position="isMobile ? 'bottom' : 'center'"
      :transition="isMobile ? 'rf-login-sheet' : ''"
      :duration="isMobile ? 0.3 : 0.2"
      round
      closeable
      destroy-on-close
      :close-on-click-overlay="false"
      pop-class="rf-login-popup"
      @closed="goHome"
    >
      <section class="rf-login-dialog" aria-labelledby="login-title" @keyup.enter="submit">
        <div class="rf-login-brand-center" aria-hidden="true">
          <img v-if="brandImage" :src="brandImage" :alt="siteName" />
          <span v-else class="rf-login-brand-fallback">{{ brandInitial }}</span>
        </div>
        <header class="auth-heading rf-login-heading">
          <h1 id="login-title">{{ headline }}</h1>
        </header>

        <template v-if="step === 'email'">
          <nut-form class="rf-auth-nut-form" aria-label="邮箱登录">
            <nut-form-item label="邮箱">
              <input
                ref="emailInput"
                v-model="form.email"
                class="rf-auth-field-input"
                type="email"
                autocomplete="email"
                inputmode="email"
                placeholder="name@example.com"
                :aria-invalid="Boolean(emailError)"
                :aria-describedby="emailError ? 'login-email-error' : undefined"
                @input="emailError = ''"
                @blur="form.email && validateEmail()"
              />
            </nut-form-item>
          </nut-form>
          <p v-if="emailError" id="login-email-error" class="rf-auth-field-error" role="alert">{{ emailError }}</p>
          <button type="button" class="rf-auth-action rf-auth-action--primary" @click="continueWithEmail">继续</button>

          <template v-if="oauthProviders.length">
            <nut-divider content-position="center">或使用其他方式</nut-divider>
            <button v-for="provider in oauthProviders" :key="provider.id" type="button" class="rf-auth-action rf-auth-action--secondary" @click="oauthLogin(provider.provider_key)">
              <AppIcon name="link" size="19" />
              使用 {{ provider.provider_name }} 继续
            </button>
          </template>
        </template>

        <template v-else>
          <div class="rf-login-email-summary">
            <span>{{ form.email }}</span>
            <button type="button" @click="returnToEmail">修改</button>
          </div>
          <nut-form class="rf-auth-nut-form" aria-label="密码登录">
            <nut-form-item label="密码">
              <span class="rf-password-field">
                <input
                  ref="passwordInput"
                  v-model="form.password"
                  class="rf-auth-field-input"
                  :type="showPassword ? 'text' : 'password'"
                  autocomplete="current-password"
                  placeholder="请输入密码"
                  :aria-invalid="Boolean(passwordError)"
                  :aria-describedby="passwordError ? 'login-password-error' : undefined"
                  @input="passwordError = ''"
                />
                <button type="button" class="rf-password-toggle" :aria-label="showPassword ? '隐藏密码' : '显示密码'" :title="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
                  <AppIcon name="eye" size="18" />
                </button>
              </span>
            </nut-form-item>
          </nut-form>
          <p v-if="passwordError" id="login-password-error" class="rf-auth-field-error" role="alert">{{ passwordError }}</p>
          <div class="auth-forgot"><RouterLink to="/forgot-password">忘记密码？</RouterLink></div>
          <HumanVerification ref="verification" />
          <button
            type="button"
            class="rf-auth-action rf-auth-action--primary"
            :disabled="submitting || auth.loading"
            @click="submit"
          >
            <span v-if="submitting || auth.loading" class="rf-auth-action-spinner" aria-hidden="true" />
            {{ submitting || auth.loading ? '正在登录' : '登录' }}
          </button>
        </template>

        <footer class="rf-login-footer">
          <span>还没有账号？</span>
          <button type="button" @click="openRegister">创建账号</button>
        </footer>
        <p class="rf-login-consent">登录即代表您同意我们的<a v-if="site.settings?.user_agreement_url" :href="site.settings.user_agreement_url" target="_blank" rel="noopener noreferrer">用户协议</a><span v-else>用户协议</span>和<a v-if="site.settings?.cookies_policy_url" :href="site.settings.cookies_policy_url" target="_blank" rel="noopener noreferrer">cookies政策</a><span v-else>cookies政策</span></p>
      </section>
    </nut-popup>
  </div>
</template>
