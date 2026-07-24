import { computed, readonly, ref } from 'vue'

export type ColorTheme = 'light' | 'dark'

const storageKey = 'robforum:color-theme'
const darkScheme = window.matchMedia('(prefers-color-scheme: dark)')
const theme = ref<ColorTheme>('light')
let initialized = false

function storedTheme(): ColorTheme | null {
  try {
    const value = window.localStorage.getItem(storageKey)
    return value === 'light' || value === 'dark' ? value : null
  } catch (error) {
    if (error instanceof DOMException) return null
    throw error
  }
}

function resolveTheme(): ColorTheme {
  return storedTheme() ?? (darkScheme.matches ? 'dark' : 'light')
}

function applyTheme(value: ColorTheme): void {
  theme.value = value
  document.documentElement.dataset.theme = value
  document.documentElement.classList.toggle('nut-theme-dark', value === 'dark')
  document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute('content', value === 'dark' ? '#000000' : '#ffffff')
}

function handleSystemThemeChange(): void {
  if (storedTheme() === null) applyTheme(resolveTheme())
}

function handleStoredThemeChange(event: StorageEvent): void {
  if (event.key === storageKey) applyTheme(resolveTheme())
}

export function initializeTheme(): void {
  applyTheme(resolveTheme())
  if (initialized) return
  initialized = true
  darkScheme.addEventListener('change', handleSystemThemeChange)
  window.addEventListener('storage', handleStoredThemeChange)
}

export function useTheme() {
  const isDark = computed(() => theme.value === 'dark')

  function toggleTheme(): void {
    const nextTheme: ColorTheme = isDark.value ? 'light' : 'dark'
    try {
      window.localStorage.setItem(storageKey, nextTheme)
    } catch (error) {
      if (!(error instanceof DOMException)) throw error
    }
    applyTheme(nextTheme)
  }

  return { theme: readonly(theme), isDark, toggleTheme }
}
