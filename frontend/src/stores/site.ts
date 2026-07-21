import { ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchSiteSettings, type SiteSettings } from '@/api'

export const useSiteStore = defineStore('site', () => {
  const settings = ref<SiteSettings | null>(null)
  const loading = ref(false)
  let pending: Promise<SiteSettings> | null = null

  function applyTheme(value: SiteSettings) {
    if (typeof document !== 'undefined' && /^#[0-9a-f]{6}$/i.test(value.primary_color)) {
      document.documentElement.style.setProperty('--primary', value.primary_color)
    }
  }

  function setSettings(value: SiteSettings) {
    settings.value = value
    applyTheme(value)
  }

  async function load(force = false) {
    if (settings.value && !force) return settings.value
    if (pending && !force) return pending
    loading.value = true
    pending = fetchSiteSettings()
      .then((value) => {
        setSettings(value)
        return value
      })
      .finally(() => {
        loading.value = false
        pending = null
      })
    return pending
  }

  return { settings, loading, load, setSettings }
})
