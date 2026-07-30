import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchMe, login as loginRequest, logout as logoutRequest, register as registerRequest, type User } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(false)
  const ready = ref(false)
  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function bootstrap() {
    if (ready.value) return
    try { user.value = await fetchMe() } catch { user.value = null } finally { ready.value = true }
  }
  async function login(email: string, password: string, captchaToken = '') {
    loading.value = true
    try {
      const result = await loginRequest({ email, password, captcha_token: captchaToken })
      if (result.requires_2fa && result.challenge_token) return { requires2FA: true, challengeToken: result.challenge_token }
      if (!result.user) throw new Error('login_response_invalid')
      user.value = result.user
      return { requires2FA: false, challengeToken: '' }
    } finally { loading.value = false }
  }
  async function register(email: string, password: string, displayName: string, captchaToken = '', emailCode = '') { loading.value = true; try { user.value = (await registerRequest({ email, password, display_name: displayName, captcha_token: captchaToken, ...(emailCode ? { email_code: emailCode } : {}) })).user } finally { loading.value = false } }
  async function logout() { await logoutRequest().catch(() => {}); user.value = null }
  function setUser(nextUser: User) { user.value = nextUser }
  function invalidateSession() { user.value = null; ready.value = true }
  return { user, loading, ready, isAuthenticated, isAdmin, bootstrap, login, register, logout, setUser, invalidateSession }
})
