import { computed, readonly, ref } from 'vue'

export type ColorTheme = 'light' | 'dark'

const storageKey = 'robforum:color-theme'
const darkScheme = window.matchMedia('(prefers-color-scheme: dark)')
const theme = ref<ColorTheme>('light')
const transitioning = ref(false)
let initialized = false

interface ThemeTransitionOrigin {
  x: number
  y: number
}

interface ViewTransitionDocument extends Document {
  startViewTransition?: (update: () => void) => {
    ready: Promise<void>
    finished: Promise<void>
  }
}

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

function applyThemeWithFallback(value: ColorTheme): void {
  const root = document.documentElement
  root.classList.add('rf-theme-transition-fallback')
  window.requestAnimationFrame(() => {
    applyTheme(value)
    window.setTimeout(() => {
      root.classList.remove('rf-theme-switching', 'rf-theme-transition-fallback')
      transitioning.value = false
    }, 360)
  })
}

function applyThemeWithTransition(value: ColorTheme, origin?: ThemeTransitionOrigin): void {
  const root = document.documentElement
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const startViewTransition = (document as ViewTransitionDocument).startViewTransition?.bind(document)
  if (reducedMotion) {
    applyTheme(value)
    transitioning.value = false
    return
  }

  root.classList.add('rf-theme-switching')
  if (!startViewTransition) {
    applyThemeWithFallback(value)
    return
  }

  const x = Math.min(Math.max(origin?.x ?? window.innerWidth / 2, 0), window.innerWidth)
  const y = Math.min(Math.max(origin?.y ?? window.innerHeight / 2, 0), window.innerHeight)
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  const transition = startViewTransition(() => applyTheme(value))
  transition.ready.then(() => {
    root.animate(
      { clipPath: [`circle(0 at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      {
        duration: 520,
        easing: 'cubic-bezier(.22, 1, .36, 1)',
        pseudoElement: '::view-transition-new(root)',
      } as KeyframeAnimationOptions,
    )
  }).catch(() => undefined)
  transition.finished.then(() => {
    root.classList.remove('rf-theme-switching')
    transitioning.value = false
  }, () => {
    root.classList.remove('rf-theme-switching')
    transitioning.value = false
  })
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

  function toggleTheme(origin?: ThemeTransitionOrigin): void {
    if (transitioning.value) return
    const nextTheme: ColorTheme = isDark.value ? 'light' : 'dark'
    try {
      window.localStorage.setItem(storageKey, nextTheme)
    } catch (error) {
      if (!(error instanceof DOMException)) throw error
    }
    transitioning.value = true
    applyThemeWithTransition(nextTheme, origin)
  }

  return { theme: readonly(theme), isDark, transitioning: readonly(transitioning), toggleTheme }
}
