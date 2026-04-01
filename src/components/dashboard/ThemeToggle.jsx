import { MoonStar, SunMedium } from 'lucide-react'
import { useTheme } from '@/stores/theme'

export default function ThemeToggle() {
  const { isDark, toggle } = useTheme()

  return (
    <button
      type="button"
      className="border-border bg-background text-foreground hover:bg-foreground hover:text-background inline-flex items-center gap-2 border px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase transition"
      onClick={toggle}
      aria-label="Toggle theme"
    >
      {isDark ? (
        <>
          <SunMedium size={14} />
          <span>Light</span>
        </>
      ) : (
        <>
          <MoonStar size={14} />
          <span>Dark</span>
        </>
      )}
    </button>
  )
}
