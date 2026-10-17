import { createContext, useContext, useState, useEffect, useCallback } from 'react'

const THEME_KEY = 'theme'

const ThemeContext = createContext(null)

// Storage can be blocked (private windows, strict settings); never let it break rendering.
const readStoredTheme = () => {
  try {
    return localStorage.getItem(THEME_KEY)
  } catch (_) {
    return null
  }
}

const writeStoredTheme = (value) => {
  try {
    localStorage.setItem(THEME_KEY, value)
  } catch (_) {
    // ignore: the theme just will not persist
  }
}

export const ThemeProvider = ({ children }) => {
  const [isDark, setIsDark] = useState(() => {
    if (typeof window === 'undefined') return false
    const stored = readStoredTheme()
    return stored !== null
      ? stored === 'dark'
      : window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    writeStoredTheme(isDark ? 'dark' : 'light')
  }, [isDark])

  const toggle = useCallback(() => setIsDark((prev) => !prev), [])

  return <ThemeContext.Provider value={{ isDark, toggle }}>{children}</ThemeContext.Provider>
}

export const useTheme = () => {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
