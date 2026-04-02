import { useEffect, useMemo, useState } from 'react'
import { format } from 'date-fns'
import { CalendarDays, Check, ChevronDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'

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

const formatApiDate = (date) => format(date, 'yyyy-MM-dd')

const formatRangeLabel = (range) => {
  if (!range?.from) return 'Custom Range'
  if (!range.to) return format(range.from, 'MMM d, yyyy')
  return `${format(range.from, 'MMM d, yyyy')} - ${format(range.to, 'MMM d, yyyy')}`
}

export default function DateRangePicker({ value = 'Last 7 Days', onChange }) {
  const [open, setOpen] = useState(false)
  const [customRange, setCustomRange] = useState()
  const [draftRange, setDraftRange] = useState()

  useEffect(() => {
    if (value !== 'Custom Range') {
      return
    }

    setDraftRange((current) => current ?? customRange)
  }, [customRange, value])

  useEffect(() => {
    if (!open) {
      return
    }

    setDraftRange((current) => current ?? customRange)
  }, [customRange, open])

  const selectPreset = (preset) => {
    if (preset === 'Custom Range') {
      setDraftRange((current) => current ?? customRange)
      return
    }

    setOpen(false)
    onChange?.({ range: preset, start: null, end: null })
  }

  const applyCustom = () => {
    if (!draftRange?.from || !draftRange?.to) return

    const nextRange = {
      from: draftRange.from,
      to: draftRange.to,
    }

    setCustomRange(nextRange)
    setOpen(false)
    onChange?.({
      range: null,
      start: formatApiDate(nextRange.from),
      end: formatApiDate(nextRange.to),
    })
  }

  const resetDraft = () => {
    setDraftRange(customRange)
  }

  const clearDraft = () => {
    setDraftRange(undefined)
  }

  const displayLabel = useMemo(() => {
    if (value !== 'Custom Range') {
      return value
    }

    return formatRangeLabel(customRange)
  }, [customRange, value])

  const draftLabel = useMemo(() => formatRangeLabel(draftRange), [draftRange])
  const hasCompleteRange = Boolean(draftRange?.from && draftRange?.to)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            className="min-w-[220px] justify-between gap-3 px-3 font-semibold tracking-[0.24em] uppercase"
          />
        }
      >
        <span className="flex min-w-0 items-center gap-2">
          <CalendarDays className="size-4" />
          <span className="truncate">{displayLabel}</span>
        </span>
        <ChevronDown className="size-4 opacity-60" />
      </PopoverTrigger>

      <PopoverContent align="end" sideOffset={10} className="w-[min(96vw,760px)] gap-0 p-0">
        <div className="grid gap-0 md:grid-cols-[230px_minmax(0,1fr)]">
          <div className="border-border border-b p-4 md:border-r md:border-b-0">
            <PopoverHeader className="mb-4 gap-2">
              <PopoverTitle className="text-sm tracking-[0.24em] uppercase">
                Range Presets
              </PopoverTitle>
              <PopoverDescription>
                Use a quick preset or lock a custom date window from the calendar.
              </PopoverDescription>
            </PopoverHeader>

            <div className="grid gap-2">
              {PRESETS.map((preset) => {
                const isActive =
                  preset === 'Custom Range' ? value === 'Custom Range' : value === preset

                return (
                  <Button
                    key={preset}
                    type="button"
                    variant={isActive ? 'secondary' : 'ghost'}
                    className="justify-between px-3 py-2 text-left text-xs font-semibold tracking-[0.22em] uppercase"
                    onClick={() => selectPreset(preset)}
                  >
                    <span className="truncate">{preset}</span>
                    {isActive && <Check className="size-4" />}
                  </Button>
                )
              })}
            </div>
          </div>

          <div className="flex min-w-0 flex-col">
            <div className="border-border border-b p-4">
              <p className="text-muted-foreground text-[10px] font-semibold tracking-[0.35em] uppercase">
                Custom Range
              </p>
              <p className="mt-2 text-sm font-medium">{draftLabel}</p>
              <p className="text-muted-foreground mt-1 text-xs">
                Pick a start and end date, then apply the range.
              </p>
            </div>

            <div className="overflow-x-auto p-2">
              <Calendar
                mode="range"
                numberOfMonths={2}
                selected={draftRange}
                defaultMonth={draftRange?.from || customRange?.from || new Date()}
                onSelect={(range) => setDraftRange(range)}
                className="mx-auto"
              />
            </div>

            <div className="border-border grid gap-3 border-t p-4 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-center">
              <div className="text-muted-foreground min-w-0 text-xs">
                {hasCompleteRange ? (
                  <>
                    <span className="text-foreground font-medium">
                      {format(draftRange.from, 'MMM d, yyyy')}
                    </span>
                    <span className="mx-2">to</span>
                    <span className="text-foreground font-medium">
                      {format(draftRange.to, 'MMM d, yyyy')}
                    </span>
                  </>
                ) : (
                  'Select both dates to apply a custom range.'
                )}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={customRange ? resetDraft : clearDraft}
              >
                {customRange ? 'Reset' : 'Clear'}
              </Button>
              <Button type="button" size="sm" onClick={applyCustom} disabled={!hasCompleteRange}>
                Apply Range
              </Button>
            </div>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
