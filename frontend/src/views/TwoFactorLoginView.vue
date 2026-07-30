<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Notify, NumberKeyboard } from '@nutui/nutui'
import { errorMessage, verifyTwoFactorLogin } from '@/api'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const code = ref('')
const visible = ref(true)
const submitting = ref(false)
const challenge = sessionStorage.getItem('robforum:2fa-challenge') || ''
const digits = computed(() => Array.from({ length: 6 }, (_, index) => code.value[index] || ''))

async function submit() {
  if (submitting.value || code.value.length !== 6 || !challenge) return
  submitting.value = true
  try {
    const result = await verifyTwoFactorLogin(challenge, code.value)
    auth.setUser(result.user)
    const destination = sessionStorage.getItem('robforum:2fa-return-to') || '/'
    sessionStorage.removeItem('robforum:2fa-challenge')
    sessionStorage.removeItem('robforum:2fa-return-to')
    Notify.success('登录成功')
    await router.replace(destination.startsWith('/') && !destination.startsWith('//') ? destination : '/')
  } catch (cause) {
    code.value = ''
    Notify.danger(errorMessage(cause, '动态验证码不正确或已过期'))
  } finally { submitting.value = false }
}

watch(code, () => { if (code.value.length === 6) void submit() })
onMounted(() => { if (!challenge) router.replace('/login') })
</script>

<template>
  <main class="rf-two-factor-page">
    <section class="rf-two-factor-panel" aria-labelledby="two-factor-title">
      <h1 id="two-factor-title">两步验证</h1>
      <p>请输入身份验证器显示的 6 位动态验证码</p>
      <div class="rf-two-factor-digits" aria-label="6 位动态验证码">
        <span v-for="(digit, index) in digits" :key="index" :class="{ filled: digit }">{{ digit }}</span>
      </div>
      <p v-if="submitting" class="rf-two-factor-state">正在验证...</p>
      <button type="button" class="rf-two-factor-back" @click="router.replace('/login')">返回密码登录</button>
      <NumberKeyboard v-model:value="code" :visible="visible" :maxlength="6" :overlay="false" teleport-disable />
    </section>
  </main>
</template>

<style scoped>
.rf-two-factor-page { min-height: 100vh; display: grid; place-items: start center; padding: 11vh 16px 320px; background: var(--rf-bg); }
.rf-two-factor-panel { width: min(100%, 420px); text-align: center; }
.rf-two-factor-panel h1 { margin: 0; font-size: 28px; }
.rf-two-factor-panel > p { margin: 12px 0 28px; color: var(--rf-muted); }
.rf-two-factor-digits { display: grid; grid-template-columns: repeat(6, 48px); justify-content: center; gap: 10px; }
.rf-two-factor-digits span { display: grid; width: 48px; height: 56px; place-items: center; border: 1px solid var(--rf-line); border-radius: 6px; background: var(--rf-bg-subtle); font-size: 24px; font-variant-numeric: tabular-nums; }
.rf-two-factor-digits span.filled { border-color: var(--primary); }
.rf-two-factor-state { min-height: 24px; margin: 18px 0 0 !important; }
.rf-two-factor-back { margin-top: 16px; color: var(--primary); background: transparent; }
@media (max-width: 420px) { .rf-two-factor-digits { grid-template-columns: repeat(6, 42px); gap: 6px; } .rf-two-factor-digits span { width: 42px; height: 52px; } }
</style>
