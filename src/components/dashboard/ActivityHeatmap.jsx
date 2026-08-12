import { Bot, CalendarDays, Flame, Gauge, Timer } from 'lucide-react'
import { formatShortDuration, normalizeItems } from './dashboardUtils.js'

const parseDate = (value) => {
  const [year, month, day] = String(value || '')
    .split('-')
    .map(Number)
  if (!year || !month || !day) return null
  return new Date(year, month - 1, day)
}

const dateKey = (date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const addDays = (date, amount) => {
  const next = new Date(date)
  next.setDate(next.getDate() + amount)
  return next
}

const startOfWeek = (date) => addDays(date, -((date.getDay() + 6) % 7))
const endOfWeek = (date) => addDays(date, 6 - ((date.getDay() + 6) % 7))

const buildCalendar = (activityDays) => {
  const days = normalizeItems(activityDays)
    .filter((day) => day?.date)
    .sort((left, right) => left.date.localeCompare(right.date))

  if (!days.length) return { cells: [], monthLabels: [], weekCount: 0 }

  const byDate = new Map(days.map((day) => [day.date, day]))
  const firstDate = parseDate(days[0].date)
  const lastDate = parseDate(days[days.length - 1].date)
  if (!firstDate || !lastDate) return { cells: [], monthLabels: [], weekCount: 0 }

  const calendarStart = startOfWeek(firstDate)
  const calendarEnd = endOfWeek(lastDate)
  const cells = []
  const monthLabels = []
  let lastMonth = ''

  for (let cursor = calendarStart; cursor <= calendarEnd; cursor = addDays(cursor, 1)) {
    const key = dateKey(cursor)
    const day = byDate.get(key)
    const inRange = cursor >= firstDate && cursor <= lastDate
    const weekIndex = Math.floor(cells.length / 7)
    const month = cursor.toLocaleDateString('en-US', { month: 'short' })

    if (inRange && month !== lastMonth) {
      monthLabels.push({ label: month, weekIndex })
      lastMonth = month
    }

    cells.push({
      date: key,
      seconds: Number(day?.total_seconds) || 0,
      intensity: Number(day?.intensity) || 0,
      inRange,
    })
  }

  return { cells, monthLabels, weekCount: Math.ceil(cells.length / 7) }
}

const intensityClass = (intensity, inRange) => {
  if (!inRange) return 'bg-transparent'
  if (intensity <= 0) return 'bg-zinc-900 ring-1 ring-inset ring-zinc-800/60'
  if (intensity === 1) return 'bg-sky-950 ring-1 ring-inset ring-sky-900/70'
  if (intensity === 2) return 'bg-sky-900'
  if (intensity === 3) return 'bg-sky-600'
  return 'bg-sky-300'
}

const formatDate = (value) => {
  if (!value) return '—'
  return new Date(`${value}T00:00:00`).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  })
}

const Metric = ({ icon: Icon, label, value, note }) => (
  <div className="min-w-0 border-t border-zinc-900 p-4 lg:border-t-0 lg:border-l lg:first:border-l-0">
    <div className="mb-3 flex items-center justify-between gap-3">
      <span className="text-[10px] tracking-[0.18em] text-zinc-600 uppercase">{label}</span>
      <Icon size={14} className="text-sky-300" />
    </div>
    <div className="truncate font-mono text-lg font-medium text-zinc-100">{value}</div>
    <div className="mt-1 truncate text-[10px] text-zinc-600" title={note}>
      {note}
    </div>
  </div>
)

const ActivityHeatmap = ({ data = {}, loading = false }) => {
  const days = normalizeItems(data.days)
  const calendar = buildCalendar(days)
  const year = Number(data.year) || new Date().getFullYear()
  const activeDays = Number(data.active_days) || 0
  const calendarDays = Number(data.calendar_days) || 365
  const periodDays = Number(data.period_days) || days.length
  const activityCoverage = calendarDays > 0 ? Math.min(100, (activeDays / calendarDays) * 100) : 0
  const peakDay = data.peak_day || {}
  const longestTask = data.longest_task || {}
  const streakRange = data.longest_streak_start
    ? `${formatDate(data.longest_streak_start)} → ${formatDate(data.longest_streak_end)}`
    : 'No streak recorded'
  const taskLabel = longestTask.agent_name || longestTask.project || 'No AI session recorded'

  return (
    <section className="mb-6 border border-zinc-900 bg-zinc-950/80">
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-zinc-900 px-5 py-4">
        <div>
          <div className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 bg-sky-300" />
            <h2 className="text-xs font-semibold tracking-[0.22em] text-zinc-200 uppercase">
              ACTIVITY_MATRIX // {year}
            </h2>
          </div>
          <p className="mt-2 text-[11px] text-zinc-600">
            Daily focus density compiled from heartbeat intervals in {data.timezone || 'UTC'}.
          </p>
        </div>
        <div className="flex items-center gap-2 font-mono text-[10px] tracking-[0.16em] text-zinc-600 uppercase">
          <span>{loading ? 'Compiling annual signal' : 'Server compiled'}</span>
          <span
            className={`h-1.5 w-1.5 ${loading ? 'animate-pulse bg-sky-300' : 'bg-emerald-400'}`}
          />
        </div>
      </div>

      <div className="grid lg:grid-cols-[minmax(0,1fr)_17rem]">
        <div className="min-w-0 p-5">
          {!days.length ? (
            <div className="flex min-h-52 items-center justify-center border border-dashed border-zinc-800 text-sm text-zinc-600">
              {loading
                ? 'Compiling activity matrix...'
                : `No heartbeat activity recorded for ${year}.`}
            </div>
          ) : (
            <div className="overflow-x-auto pb-1">
              <div className="min-w-[760px]">
                <div className="relative ml-9 h-6 font-mono text-[9px] text-zinc-600 uppercase">
                  {calendar.monthLabels.map((month, index) => (
                    <span
                      key={`${month.label}-${month.weekIndex}-${index}`}
                      className="absolute top-0"
                      style={{ left: `${(month.weekIndex / calendar.weekCount) * 100}%` }}
                    >
                      {month.label}
                    </span>
                  ))}
                </div>
                <div className="flex gap-2">
                  <div className="grid w-7 shrink-0 grid-rows-7 gap-[3px] font-mono text-[9px] text-zinc-700">
                    {['Mon', '', 'Wed', '', 'Fri', '', 'Sun'].map((label, index) => (
                      <span key={`${label}-${index}`} className="flex items-center">
                        {label}
                      </span>
                    ))}
                  </div>
                  <div
                    className="grid flex-1 grid-flow-col grid-rows-7 gap-[3px]"
                    style={{
                      gridTemplateColumns: `repeat(${calendar.weekCount}, minmax(10px, 1fr))`,
                    }}
                    role="img"
                    aria-label={`${year} daily activity matrix`}
                  >
                    {calendar.cells.map((cell) => (
                      <span
                        key={cell.date}
                        className={`aspect-square min-h-2.5 min-w-2.5 ${intensityClass(cell.intensity, cell.inRange)}`}
                        title={
                          cell.inRange
                            ? `${cell.date}: ${formatShortDuration(cell.seconds)}`
                            : undefined
                        }
                      />
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}

          <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-zinc-900 pt-4 text-[10px] text-zinc-600">
            <span className="font-mono uppercase">
              Window {data.range_start || `${year}-01-01`} → {data.range_end || '—'} ·{' '}
              {data.writes_only ? 'writes only' : 'all heartbeats'}
            </span>
            <div className="flex items-center gap-1.5">
              <span className="mr-1 uppercase">Idle</span>
              {['bg-zinc-900', 'bg-sky-950', 'bg-sky-900', 'bg-sky-600', 'bg-sky-300'].map(
                (color) => (
                  <span key={color} className={`h-2.5 w-2.5 ${color}`} />
                )
              )}
              <span className="ml-1 uppercase">Peak</span>
            </div>
          </div>
        </div>

        <aside className="border-t border-zinc-900 bg-zinc-950/50 p-5 lg:border-t-0 lg:border-l">
          <div className="mb-6 flex items-center justify-between">
            <span className="text-[10px] tracking-[0.2em] text-zinc-600 uppercase">
              Year signal
            </span>
            <Gauge size={15} className="text-sky-300" />
          </div>
          <div className="font-mono text-4xl font-medium tracking-tight text-zinc-100">
            {activeDays}
            <span className="ml-1 text-base text-zinc-600">/{calendarDays}</span>
          </div>
          <div className="mt-2 text-[10px] tracking-[0.18em] text-zinc-500 uppercase">
            Active days · {activityCoverage.toFixed(1)}%
          </div>
          <div className="mt-4 h-1 bg-zinc-900">
            <div className="h-full bg-sky-300" style={{ width: `${activityCoverage}%` }} />
          </div>

          <dl className="mt-7 space-y-4 border-t border-zinc-900 pt-5 text-[11px]">
            <div className="flex items-center justify-between gap-4">
              <dt className="text-zinc-600">Focus accumulated</dt>
              <dd className="font-mono text-zinc-300">{formatShortDuration(data.total_seconds)}</dd>
            </div>
            <div className="flex items-center justify-between gap-4">
              <dt className="text-zinc-600">Days compiled</dt>
              <dd className="font-mono text-zinc-300">{periodDays}</dd>
            </div>
            <div className="flex items-center justify-between gap-4">
              <dt className="text-zinc-600">Timeout model</dt>
              <dd className="font-mono text-zinc-300">{Number(data.timeout_minutes) || 15}m</dd>
            </div>
          </dl>
        </aside>
      </div>

      <div className="grid border-t border-zinc-900 sm:grid-cols-2 lg:grid-cols-4">
        <Metric
          icon={Flame}
          label="Longest streak"
          value={`${Number(data.longest_streak_days) || 0} days`}
          note={streakRange}
        />
        <Metric
          icon={CalendarDays}
          label="Peak focus day"
          value={formatShortDuration(peakDay.total_seconds)}
          note={formatDate(peakDay.date)}
        />
        <Metric
          icon={Bot}
          label="Longest AI run"
          value={formatShortDuration(longestTask.total_seconds)}
          note={taskLabel}
        />
        <Metric
          icon={Timer}
          label="Active-day average"
          value={formatShortDuration(data.average_active_day_seconds)}
          note={`${activeDays} active days in ${year}`}
        />
      </div>
    </section>
  )
}

export default ActivityHeatmap
