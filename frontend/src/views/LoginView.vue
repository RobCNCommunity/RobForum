<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchPublicOAuth, type PublicOAuthConfig } from '@/api'
import { useAuthStore } from '@/stores/auth'
import HumanVerification from '@/components/HumanVerification.vue'
import AppIcon from '@/components/AppIcon.vue'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const verification = ref<InstanceType<typeof HumanVerification> | null>(null)
const formRef = ref<{ validate: () => Promise<unknown> } | null>(null)
const visible = ref(true)
const captchaToken = ref('')
const captchaRequired = ref(false)
const form = reactive({ email: '', password: '' })
const oauth = ref<PublicOAuthConfig>({ enabled: false, provider_name: 'OAuth' })
const isMobile = ref(typeof window !== 'undefined' && window.innerWidth <= 560)

const rules = {
  email: [
    { required: true, message: '请输入邮箱' },
    { regex: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '请输入有效的邮箱地址' },
  ],
  password: [{ required: true, message: '请输入密码' }],
}

function goHome() {
  if (router.currentRoute.value.path !== '/') router.push('/')
}

async function submit() {
  if (captchaRequired.value && !captchaToken.value) {
    Notify.warn('请先完成验证')
    return
  }
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  try {
    await auth.login(form.email.trim(), form.password, captchaToken.value)
    Notify.success('登录成功')
    const requested = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    const destination = requested.startsWith('/') && !requested.startsWith('//') && !requested.includes('\\') ? requested : '/'
    await router.push(destination)
  } catch (error) {
    Notify.danger(errorMessage(error, '登录失败，请稍后重试'))
    verification.value?.reset()
  }
}

function openRegister() {
  visible.value = false
  router.push('/register')
}

function updateViewport() {
  isMobile.value = window.innerWidth <= 560
}

function oauthLogin() {
  const requested = typeof route.query.return_to === 'string' ? route.query.return_to : '/'
  const returnTo = requested.startsWith('/') && !requested.startsWith('//') ? requested : '/'
  window.location.assign(`/api/v1/oauth/start?return_to=${encodeURIComponent(returnTo)}`)
}

onMounted(async () => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
  try { oauth.value = await fetchPublicOAuth() } catch { oauth.value.enabled = false }
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
})

onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))
</script>

<template>
  <div class="auth-page auth-page--popup" @keyup.enter="submit">
    <div class="auth-backdrop-content" aria-hidden="true">
      <AppIcon name="people" size="30" />
      <p>加入社区，分享你的 Roblox 世界</p>
    </div>

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
      <section class="rf-login-dialog" aria-labelledby="login-title">
        <div class="rf-login-mark"><AppIcon name="people" size="24" /></div>
        <header class="auth-heading">
          <h1 id="login-title">欢迎回来</h1>
          <p>登录社区，继续你的 Roblox 旅程。</p>
        </header>

        <template v-if="oauth.enabled">
          <nut-button class="rf-oauth-button" block plain type="default" size="large" @click="oauthLogin"><AppIcon name="link" size="19" />使用 {{ oauth.provider_name }} 继续</nut-button>
          <nut-divider content-position="center">或使用邮箱登录</nut-divider>
        </template>

        <nut-form ref="formRef" :model-value="form" :rules="rules" class="rf-auth-nut-form">
          <nut-form-item prop="email" label="邮箱">
            <input v-model="form.email" class="rf-auth-field-input" type="email" autocomplete="email" placeholder="name@example.com" />
          </nut-form-item>
          <nut-form-item prop="password" label="密码">
            <input v-model="form.password" class="rf-auth-field-input" type="password" autocomplete="current-password" placeholder="请输入密码" />
          </nut-form-item>
        </nut-form>

        <div class="auth-forgot"><RouterLink to="/forgot-password">忘记密码？</RouterLink></div>
        <HumanVerification ref="verification" @token="captchaToken = $event" @required="captchaRequired = $event" />
        <nut-button type="primary" block size="large" :loading="auth.loading" :disabled="captchaRequired && !captchaToken" @click="submit">登录</nut-button>

        <nut-divider content-position="center">还没有账号？</nut-divider>
        <nut-button block plain type="default" size="large" @click="openRegister">创建账号</nut-button>
      </section>
    </nut-popup>
  </div>
</template>
