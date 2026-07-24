import { ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchSiteSettings, type SiteSettings } from '@/api'

const siteSettingsCacheKey = 'robforum.site-settings.v1'

function readCachedSettings() {
  if (typeof window === 'undefined') return null
  try {
    const value = JSON.parse(window.localStorage.getItem(siteSettingsCacheKey) || '') as SiteSettings
    return typeof value?.site_name === 'string' ? value : null
  } catch {
    return null
  }
}

function cacheSettings(value: SiteSettings) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(siteSettingsCacheKey, JSON.stringify(value))
  } catch {
    // A blocked or full storage area must not prevent the site from rendering.
  }
}

export const useSiteStore = defineStore('site', () => {
  // This is read synchronously so the initial loading screen can use the last
  // configured community name and logo instead of a compiled-in fallback.
  const settings = ref<SiteSettings | null>(readCachedSettings())
  const loading = ref(false)
  let loaded = false
  let pending: Promise<SiteSettings> | null = null

  function applyTheme(value: SiteSettings) {
    if (typeof document !== 'undefined' && /^#[0-9a-f]{6}$/i.test(value.primary_color)) {
      document.documentElement.style.setProperty('--primary', value.primary_color)
    }
  }

  function setSettings(value: SiteSettings) {
    settings.value = value
    applyTheme(value)
    cacheSettings(value)
    loaded = true
  }

  async function load(force = false) {
    if (loaded && settings.value && !force) return settings.value
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
