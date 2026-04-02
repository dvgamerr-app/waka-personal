import { MoonStar, SunMedium } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/stores/theme'

export default function ThemeToggle() {
  const { isDark, toggle } = useTheme()

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className="gap-2 px-3 font-semibold tracking-[0.3em] uppercase"
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
    </Button>
  )
}
