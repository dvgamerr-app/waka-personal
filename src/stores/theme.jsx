import { createContext, useContext, useState, useEffect, useCallback } from 'react'

const THEME_KEY = 'theme'

const ThemeContext = createContext(null)

export const ThemeProvider = ({ children }) => {
  // SSR-safe: default false on server, sync on client via useEffect
  const [isDark, setIsDark] = useState(false)

  useEffect(() => {
    const stored = localStorage.getItem(THEME_KEY)
    const initial =
      stored !== null ? stored === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
    setIsDark(initial)
  }, [])

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    localStorage.setItem(THEME_KEY, isDark ? 'dark' : 'light')
  }, [isDark])

  const toggle = useCallback(() => setIsDark((prev) => !prev), [])

  return <ThemeContext.Provider value={{ isDark, toggle }}>{children}</ThemeContext.Provider>
}

export const useTheme = () => {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
