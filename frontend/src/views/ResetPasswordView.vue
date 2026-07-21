<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { errorMessage, resetPassword } from '@/api'
import { Notify } from '@nutui/nutui'
const route = useRoute(); const password = ref(''); const confirm = ref(''); const loading = ref(false); const done = ref(false)
async function submit() { if (password.value !== confirm.value) { Notify.warn('两次密码不一致'); return }; loading.value = true; try { await resetPassword(String(route.query.token || ''), password.value); done.value = true } catch (e) { Notify.danger(errorMessage(e, '密码重置失败')) } finally { loading.value = false } }
</script>
<template><div class="auth-page"><div class="auth-card"><div class="auth-heading"><h1>设置新密码</h1><p>请设置一个新的社区登录密码。</p></div><div v-if="done" class="rf-success-box">密码已更新，请使用新密码登录。</div><form v-else class="rf-auth-form" @submit.prevent="submit"><label>新密码<div class="rf-field"><input v-model="password" type="password" placeholder="至少 10 个字符" required /></div></label><label>确认新密码<div class="rf-field"><input v-model="confirm" type="password" placeholder="再次输入新密码" required /></div></label><button type="submit" class="rf-primary-button rf-auth-submit" :disabled="loading">{{ loading ? '保存中…' : '保存新密码' }}</button></form><div class="auth-footer"><RouterLink to="/login">返回登录</RouterLink></div></div></div></template>
