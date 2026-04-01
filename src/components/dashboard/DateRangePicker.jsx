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
  'Custom Range',
]

export default function DateRangePicker({ value = 'Last 7 Days', onChange }) {
  const [open, setOpen] = useState(false)
  const [pending, setPending] = useState(value)
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const ref = useRef(null)

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.key === 'Escape') cancel()
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
    setPending(preset)
    if (preset !== 'Custom Range') {
      setCustomStart('')
      setCustomEnd('')
    }
  }

  const apply = () => {
    setOpen(false)
    if (pending === 'Custom Range') {
      onChange?.({ range: null, start: customStart, end: customEnd })
    } else {
      onChange?.({ range: pending, start: null, end: null })
    }
  }

  const cancel = () => {
    setPending(value)
    setOpen(false)
  }

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
        <span>{value}</span>
      </button>

      {open && (
        <div
          className="border-border bg-background absolute right-0 top-full z-50 mt-1 w-56 border shadow-xl"
          role="listbox"
        >
          <div className="p-1">
            {PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                role="option"
                aria-selected={pending === preset}
                className={`w-full px-3 py-2 text-left text-sm transition ${
                  pending === preset
                    ? 'bg-foreground text-background'
                    : 'text-foreground hover:bg-foreground hover:text-background'
                }`}
                onClick={() => selectPreset(preset)}
              >
                {preset}
              </button>
            ))}
          </div>

          {pending === 'Custom Range' && (
            <div className="border-border space-y-2 border-t p-3">
              <input
                type="date"
                value={customStart}
                onChange={(e) => setCustomStart(e.target.value)}
                className="border-border bg-background text-foreground w-full border px-2 py-1 text-xs"
              />
              <input
                type="date"
                value={customEnd}
                onChange={(e) => setCustomEnd(e.target.value)}
                className="border-border bg-background text-foreground w-full border px-2 py-1 text-xs"
              />
            </div>
          )}

          <div className="border-border flex gap-2 border-t p-3">
            <button
              type="button"
              className="bg-foreground text-background flex-1 px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase"
              onClick={apply}
            >
              Apply
            </button>
            <button
              type="button"
              className="border-border text-foreground hover:bg-foreground hover:text-background flex-1 border px-3 py-2 text-xs font-semibold tracking-[0.3em] uppercase transition"
              onClick={cancel}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
