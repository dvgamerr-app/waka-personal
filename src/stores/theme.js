import { writable } from 'svelte/store'

const THEME_KEY = 'payme-theme'
const browser = typeof window !== 'undefined'

function createThemeStore() {
  const getInitialTheme = () => {
    if (!browser) return false

    const stored = localStorage.getItem(THEME_KEY)
    if (stored !== null) {
      return stored === 'dark'
    }

    return window.matchMedia('(prefers-color-scheme: dark)').matches
  }

  const { subscribe, set, update } = writable(getInitialTheme())

  return {
    subscribe,

    toggle: () => {
      update((isDark) => {
        const newValue = !isDark
        if (browser) {
          localStorage.setItem(THEME_KEY, newValue ? 'dark' : 'light')
          if (newValue) {
            document.documentElement.classList.add('dark')
          } else {
            document.documentElement.classList.remove('dark')
          }
        }
        return newValue
      })
    },

    set: (isDark) => {
      if (browser) {
        localStorage.setItem(THEME_KEY, isDark ? 'dark' : 'light')
        if (isDark) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
      }
      set(isDark)
    },

    init: () => {
      if (browser) {
        const isDark = getInitialTheme()
        if (isDark) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
        set(isDark)
      }
    },
  }
}

export const theme = createThemeStore()
