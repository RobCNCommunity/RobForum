<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { errorMessage, forgotPassword } from '@/api'
import HumanVerification from '@/components/HumanVerification.vue'
import AppIcon from '@/components/AppIcon.vue'
import { Notify } from '@nutui/nutui'
const verification = ref<InstanceType<typeof HumanVerification> | null>(null); const email = ref(''); const loading = ref(false); const sent = ref(false); const captchaToken = ref(''); const captchaRequired = ref(false)
async function submit() { if (captchaRequired.value && !captchaToken.value) { Notify.warn('请先完成人机验证'); return }; loading.value = true; try { await forgotPassword(email.value, captchaToken.value); sent.value = true } catch (e) { Notify.danger(errorMessage(e, '请求失败')); verification.value?.reset() } finally { loading.value = false } }
</script>
<template><div class="auth-page"><div class="auth-card"><div class="auth-heading"><h1>找回密码</h1><p>输入注册邮箱，我们会发送一封安全的重置邮件。</p></div><div v-if="sent" class="rf-success-box">如果邮箱已注册，重置邮件已经发送，请检查收件箱和垃圾邮件。</div><form v-else class="rf-auth-form" @submit.prevent="submit"><label>注册邮箱<div class="rf-field"><AppIcon name="message" size="18" /><input v-model="email" type="email" placeholder="name@example.com" required /></div></label><HumanVerification ref="verification" @token="captchaToken = $event" @required="captchaRequired = $event" /><button type="submit" class="rf-primary-button rf-auth-submit" :disabled="loading || (captchaRequired && !captchaToken)">{{ loading ? '发送中…' : '发送重置邮件' }}</button></form><div class="auth-footer"><RouterLink to="/login">返回登录</RouterLink></div></div></div></template>
