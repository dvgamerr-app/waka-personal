import { useState, useRef, useEffect } from 'react'
import { CalendarDays } from 'lucide-react'

const PRESETS = [
  'Today',
  'Yesterday',
  'Last 7 Days',
  'Last 7 Days from Yesterday',
  'Last 14 Days',
  'Last 30 Days',
  'This Week',
  'Last Week',
  'This Month',
  'Last Month',
  'Last Year',
  'Custom Range',
]

export default function DateRangePicker({ value = 'Last 7 Days', onChange }) {
  const [open, setOpen] = useState(false)
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const [awaitingCustom, setAwaitingCustom] = useState(value === 'Custom Range')
  const ref = useRef(null)

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.key === 'Escape') setOpen(false)
    }
    const onClickOutside = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setOpen(false)
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('mousedown', onClickOutside)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('mousedown', onClickOutside)
    }
  }, [])

  const selectPreset = (preset) => {
    if (preset === 'Custom Range') {
      setAwaitingCustom(true)
      return
    }
    setAwaitingCustom(false)
    setCustomStart('')
    setCustomEnd('')
    setOpen(false)
    onChange?.({ range: preset, start: null, end: null })
  }

  const applyCustom = () => {
    if (!customStart || !customEnd) return
    setOpen(false)
    onChange?.({ range: null, start: customStart, end: customEnd })
  }

  const cancelCustom = () => {
    setAwaitingCustom(false)
    setCustomStart('')
    setCustomEnd('')
  }

  const displayLabel = value === 'Custom Range' && customStart && customEnd
    ? `${customStart} → ${customEnd}`
    : value

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        className="border-border bg-background text-foreground hover:bg-foreground hover:text-background inline-flex items-center gap-2 border px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase transition"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <CalendarDays size={14} />
        <span>{displayLabel}</span>
      </button>

      {open && (
        <div
          className="border-border bg-background absolute right-0 top-full z-50 mt-1 w-64 border shadow-xl"
          role="listbox"
        >
          <div className="p-1">
            {PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                role="option"
                aria-selected={value === preset || (preset === 'Custom Range' && awaitingCustom)}
                className={`w-full px-3 py-2 text-left text-sm transition ${
                  value === preset && preset !== 'Custom Range'
                    ? 'bg-foreground text-background'
                    : preset === 'Custom Range' && awaitingCustom
                      ? 'bg-foreground text-background'
                      : 'text-foreground hover:bg-foreground hover:text-background'
                }`}
                onClick={() => selectPreset(preset)}
              >
                {preset}
              </button>
            ))}
          </div>

          {awaitingCustom && (
            <div className="border-border border-t p-3">
              <p className="text-foreground/55 mb-2 text-[10px] font-semibold tracking-[0.3em] uppercase">
                Custom Range
              </p>
              <div className="space-y-2">
                <input
                  type="date"
                  value={customStart}
                  onChange={(e) => setCustomStart(e.target.value)}
                  className="border-border bg-background text-foreground w-full border px-2 py-1.5 text-xs"
                  placeholder="Start date"
                />
                <input
                  type="date"
                  value={customEnd}
                  onChange={(e) => setCustomEnd(e.target.value)}
                  className="border-border bg-background text-foreground w-full border px-2 py-1.5 text-xs"
                  placeholder="End date"
                />
              </div>
              <div className="mt-3 flex gap-2">
                <button
                  type="button"
                  className="bg-foreground text-background flex-1 px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase disabled:opacity-40"
                  onClick={applyCustom}
                  disabled={!customStart || !customEnd}
                >
                  Apply
                </button>
                <button
                  type="button"
                  className="border-border text-foreground hover:bg-foreground hover:text-background flex-1 border px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase transition"
                  onClick={cancelCustom}
                >
                  Clear
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
