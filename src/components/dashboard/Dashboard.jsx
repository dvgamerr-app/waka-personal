import { useState, useEffect, useMemo } from 'react'
import { Activity, AlertTriangle, Code2, Cpu, Terminal, Zap } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import ThemeToggle from './ThemeToggle'
import DateRangePicker from './DateRangePicker'
import {
  buildTrendSeries,
  computeRangeStats,
  formatCount,
  formatPercent,
  formatShortDuration,
  formatSpend,
  formatTableDuration,
  normalizeItems,
  normalizeWakaTime,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const ACCENT = '#7dd3fc'
const PANEL = 'border border-zinc-900 bg-zinc-950/80'
const LABEL = 'text-[11px] tracking-[0.24em] text-zinc-500 uppercase'

const normalizeDashboardData = (data = {}, fallbackTimezone = 'UTC') => ({
  timezone: data.timezone || fallbackTimezone,
  stats: data.stats || {},
  summaries: data.summaries || [],
  today: data.today || {},
  projectDurations: data.projectDurations || data.project_durations || [],
  languageDurations: data.languageDurations || data.language_durations || [],
  editorDurations: data.editorDurations || data.editor_durations || [],
  tokenMetrics: data.tokenMetrics || data.token_metrics || {},
  spendMetrics: data.spendMetrics || data.spend_metrics || {},
  errors: Array.isArray(data.errors) ? data.errors : [],
})

const normalizeLiveData = (data = {}) => ({
  cachedAt: data.cached_at || '',
  status: data.status || 'synchronized',
  today: data.today || {},
  projectDurations: data.project_durations || [],
  languageDurations: data.language_durations || [],
  editorDurations: data.editor_durations || [],
  errors: Array.isArray(data.errors) ? data.errors : [],
})

const fetchDashboardData = async ({ base, timezone, range, start, end }) => {
  const selectedRange = range || 'Last 7 Days'
  const params = start && end ? { start, end, timezone } : { range: selectedRange, timezone }

  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/dashboard',
    params,
  })

  if (error || !data) {
    return {
      timezone,
      stats: {},
      summaries: [],
      today: {},
      projectDurations: [],
      languageDurations: [],
      editorDurations: [],
      tokenMetrics: {},
      spendMetrics: {},
      errors: [error || 'Failed to load dashboard data'],
    }
  }

  return {
    timezone,
    stats: data.stats || {},
    summaries: data.summaries || [],
    today: data.today || {},
    projectDurations: data.project_durations || [],
    languageDurations: data.language_durations || [],
    editorDurations: data.editor_durations || [],
    tokenMetrics: data.token_metrics || {},
    spendMetrics: data.spend_metrics || {},
    errors: Array.isArray(data.errors) ? data.errors : [],
  }
}

const fetchLiveData = async ({ base, timezone }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/live',
    params: { timezone },
  })

  if (error || !data) {
    return {
      cachedAt: new Date().toISOString(),
      status: 'degraded',
      today: {},
      projectDurations: [],
      languageDurations: [],
      errors: [error || 'Failed to load live data'],
    }
  }

  return normalizeLiveData(data)
}

const clampPercent = (value, fallback = 0) => Math.max(0, Math.min(100, Number(value) || fallback))

const aiShare = (stats) => {
  const ai = Number(stats?.aiAdditions) || 0
  const human = Number(stats?.humanAdditions) || 0
  const total = ai + human
  return total > 0 ? (ai / total) * 100 : 0
}

const metricText = (value, fallback = '-') => (value == null || value === '' ? fallback : value)

const dayName = (date) => {
  if (!date) return '--'
  return new Date(`${date}T00:00:00`).toLocaleDateString('en-US', { weekday: 'short' })
}

const formatBestDay = (date) => {
  if (!date) return '--'
  const d = new Date(`${date}T00:00:00`)
  const dow = d.toLocaleDateString('en-US', { weekday: 'short' })
  const mon = d.toLocaleDateString('en-US', { month: 'short' })
  const day = d.getDate()
  const n = day % 10
  const suffix = day > 10 && day < 14 ? 'th' : n === 1 ? 'st' : n === 2 ? 'nd' : n === 3 ? 'rd' : 'th'
  return `${dow} ${mon} ${day}${suffix}`
}

const rangeDateText = (summaries) => {
  const days = normalizeItems(summaries)
  if (!days.length) return 'NO_RANGE'
  const first = days[0]?.range?.date
  const last = days[days.length - 1]?.range?.date
  return `${first || '----'} -> ${last || '----'}`
}

const buildMonthlyTraceSeries = (summaries) => {
  const buckets = new Map()

  normalizeItems(summaries).forEach((day) => {
    const date = day.range?.date || ''
    const month = date.slice(0, 7)
    if (!month) return

    const current = buckets.get(month) || {
      date: `${month}-01`,
      label: new Date(`${month}-01T00:00:00`).toLocaleDateString('en-US', {
        month: 'short',
      }),
      totalSeconds: 0,
    }
    current.totalSeconds += Number(day.grand_total?.total_seconds) || 0
    buckets.set(month, current)
  })

  return Array.from(buckets.values()).map((month) => ({
    ...month,
    totalText: formatShortDuration(month.totalSeconds),
  }))
}

const buildHourlyTraceSeries = (durations, timezone) => {
  const hours = Array.from({ length: 24 }, (_, hour) => ({
    date: `hour-${hour}`,
    label: String(hour).padStart(2, '0'),
    totalSeconds: 0,
  }))

  normalizeItems(durations).forEach((item) => {
    const start = Number(item.time) || 0
    const duration = Number(item.duration) || 0
    if (start <= 0 || duration <= 0) return

    const hour = Number(
      new Date(start * 1000).toLocaleTimeString('en-US', {
        hour: '2-digit',
        hour12: false,
        hourCycle: 'h23',
        timeZone: timezone || 'UTC',
      })
    )
    if (Number.isNaN(hour) || !hours[hour]) return
    hours[hour].totalSeconds += duration
  })

  return hours.map((hour) => ({
    ...hour,
    totalText: formatShortDuration(hour.totalSeconds),
  }))
}

const isSingleDayRange = ({ range, summaries }) => {
  if (range === 'Today' || range === 'Yesterday') return true
  const days = normalizeItems(summaries).filter((day) => day.range?.date)
  return days.length === 1
}

const refreshTimeText = (value) => {
  const date = value ? new Date(value) : new Date()
  if (Number.isNaN(date.getTime())) {
    return '--:--:--'
  }
  return date.toLocaleTimeString('en-US', { hour12: false })
}

const hasObjectData = (value) => value && Object.keys(value).length > 0

const KpiPanel = ({ label, value, note, icon: Icon, accent = ACCENT }) => (
  <section className={`${PANEL} p-4`}>
    <div className="mb-4 flex items-center justify-between gap-3">
      <span className={LABEL}>{label}</span>
      {Icon && <Icon size={15} style={{ color: accent }} />}
    </div>
    <div className="font-mono text-3xl leading-none font-medium text-zinc-100">{value}</div>
    {note && <div className="mt-3 text-xs text-zinc-600">{note}</div>}
  </section>
)

const SectionTitle = ({ children }) => (
  <h2 className="mb-5 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-zinc-400/80">
    <span className="h-1.5 w-1.5 bg-zinc-600" />
    {children}
  </h2>
)

const ProgressLine = ({ value = 0, accent = ACCENT }) => (
  <div className="h-1.5 overflow-hidden bg-zinc-900">
    <div className="h-full" style={{ width: `${clampPercent(value)}%`, background: accent }} />
  </div>
)

const DailyTrace = ({ series, title = 'DAILY_ACTIVITY_TRACE', showWeekday = true }) => {
  const maxSeconds = Math.max(1, ...series.map((day) => Number(day.totalSeconds) || 0))

  return (
    <section className={`${PANEL} p-5 lg:p-6`}>
      <SectionTitle>{title}</SectionTitle>
      {series.length === 0 ? (
        <EmptyState label="No activity trace in this range." />
      ) : (
        <div className="flex h-48 items-end gap-2 md:gap-3">
          {series.map((day) => {
            const height = Math.max(3, ((Number(day.totalSeconds) || 0) / maxSeconds) * 100)
            const isPeak = height >= 99
            return (
              <div
                key={day.date || day.label}
                className="flex min-w-0 flex-1 flex-col items-center gap-2"
              >
                <span
                  className={`font-mono text-[11px] tabular-nums ${isPeak ? 'text-sky-300' : 'text-zinc-600'}`}
                >
                  {formatShortDuration(day.totalSeconds)}
                </span>
                <div className="relative h-32 w-full overflow-hidden bg-zinc-900/60">
                  <div
                    className={`absolute inset-x-0 bottom-0 ${isPeak ? 'bg-sky-300' : 'bg-sky-300/30'}`}
                    style={{ height: `${height}%` }}
                  />
                </div>
                <span className="truncate text-[11px] text-zinc-500 uppercase">
                  {showWeekday ? dayName(day.date) || day.label : day.label}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

const AiSplit = ({ stats, topProject, rangeStats }) => {
  const percent = aiShare(stats)
  const humanPercent = Math.max(0, 100 - percent)
  const totalChanges = (Number(stats?.aiAdditions) || 0) + (Number(stats?.humanAdditions) || 0)
  const totalHours = (Number(rangeStats?.totalSeconds) || Number(stats?.total_seconds) || 0) / 3600
  const locPerHour = totalHours > 0 ? Math.round(totalChanges / totalHours) : 0

  return (
    <section className={`${PANEL} p-5 lg:p-6`}>
      <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="text-lg font-medium text-zinc-100">AI vs Human Logic Split</h2>
          <p className="mt-1 max-w-[60ch] text-sm text-zinc-500">
            Distribution of line additions across the selected range, aligned with WakaTime
            heartbeat telemetry.
          </p>
        </div>
        <div className="text-right">
          <span className="block text-[11px] tracking-[0.24em] text-zinc-500 uppercase">
            Current Peak
          </span>
          <span className="font-mono text-sm font-medium text-sky-300">
            {formatCount(locPerHour)} LOC/hr
          </span>
        </div>
      </div>

      <div className="relative flex h-12 overflow-hidden border border-zinc-900 bg-zinc-900">
        <div className="h-full" style={{ width: `${clampPercent(percent)}%`, background: ACCENT }} />
        <div
          className="flex h-full items-center justify-end bg-zinc-800 px-3 text-[11px] text-zinc-500 uppercase"
          style={{ width: `${clampPercent(humanPercent)}%` }}
        >
          Human
        </div>
        <div className="pointer-events-none absolute inset-0 flex items-center px-4 text-xs font-semibold tracking-[0.18em] uppercase">
          <span className={percent >= 15 ? 'text-black' : 'text-sky-300'}>
            AI [{formatPercent(percent)}]
          </span>
        </div>
      </div>

      <div className="mt-6 grid gap-5 border-t border-zinc-900 pt-5 sm:grid-cols-2 xl:grid-cols-4">
        <MicroMetric
          label="Review Density"
          value={`${formatPercent(percent)} · ${formatCount(totalChanges)} sessions`}
        />
        <MicroMetric
          label="Human Follow-up"
          value={`${formatPercent(humanPercent)} · ${formatCount(stats?.humanAdditions)} edits`}
        />
        <MicroMetric
          label="Top Project"
          value={
            topProject
              ? `${topProject.name} (${formatShortDuration(topProject.total_seconds)})`
              : '-'
          }
        />
        <MicroMetric label="Active Day" value={formatBestDay(rangeStats?.bestDay?.date)} />
      </div>
    </section>
  )
}

const MicroMetric = ({ label, value }) => (
  <div>
    <span className="mb-1 block text-[11px] text-zinc-500 uppercase">{label}</span>
    <span className="text-sm text-zinc-300">{metricText(value)}</span>
  </div>
)

const buildProjectUrl = (name, summaries) => {
  const days = normalizeItems(summaries)
  const start = days[0]?.range?.date
  const end = days[days.length - 1]?.range?.date
  const params = new URLSearchParams()
  if (start) params.set('start', start)
  if (end) params.set('end', end)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return `https://wakatime.com/projects/${encodeURIComponent(name)}${suffix}`
}

const ProjectMetrics = ({ projects, summaries }) => {
  const rows = topItems(projects, 8)
  const hasTokens = rows.some((p) => (p.token_count || 0) > 0)
  const hasSpend = rows.some((p) => (p.spend_cents || 0) > 0)

  return (
    <section className={`${PANEL} p-5 lg:p-6`}>
      <SectionTitle>PROJECT_ENVIRONMENT_METRICS</SectionTitle>
      {rows.length === 0 ? (
        <EmptyState label="No project metrics available." />
      ) : (
        <div className="overflow-x-auto">
          <div className="min-w-[600px]">
            <div className="grid grid-cols-12 gap-3 border-b border-zinc-900 pb-2 text-[11px] tracking-[0.18em] text-zinc-500 uppercase">
              <div className="col-span-3">Project</div>
              <div className="col-span-4">AI Density</div>
              <div className="col-span-2 text-right">Time</div>
              {hasTokens && <div className="col-span-1 text-right">Tokens</div>}
              {hasSpend && <div className={hasTokens ? 'col-span-2 text-right' : 'col-span-3 text-right'}>Spend</div>}
              {!hasTokens && !hasSpend && <div className="col-span-3 text-right">Activity</div>}
            </div>
            <div className="divide-y divide-zinc-900/70">
              {rows.map((project) => {
                const url = buildProjectUrl(project.name, summaries)
                return (
                  <a
                    key={project.name}
                    href={url}
                    className="grid grid-cols-12 items-center gap-3 py-3 text-xs transition-colors hover:bg-zinc-900/35"
                  >
                    <div className="col-span-3 flex min-w-0 items-center gap-2 text-zinc-300">
                      <span className="text-zinc-600">&gt;</span>
                      <span className="truncate">{project.name}</span>
                    </div>
                    <div className="col-span-4 flex items-center gap-2">
                      <div className="h-1 flex-1 overflow-hidden bg-zinc-900">
                        <div
                          className="h-full bg-sky-300"
                          style={{ width: `${clampPercent(project.ai_percent)}%` }}
                        />
                      </div>
                      <span className="shrink-0 font-mono text-[11px] text-sky-300 tabular-nums">
                        {formatPercent(project.ai_percent)}
                      </span>
                    </div>
                    <div className="col-span-2 text-right font-mono text-zinc-500 tabular-nums">
                      {formatTableDuration(project.total_seconds)}
                    </div>
                    {hasTokens && (
                      <div className="col-span-1 text-right font-mono text-zinc-500 tabular-nums">
                        {project.token_count > 0 ? formatCount(project.token_count) : '-'}
                      </div>
                    )}
                    {hasSpend && (
                      <div className={`${hasTokens ? 'col-span-2' : 'col-span-3'} text-right font-mono text-zinc-500 tabular-nums`}>
                        {project.spend_cents > 0 ? formatSpend(project.spend_cents) : '-'}
                      </div>
                    )}
                    {!hasTokens && !hasSpend && (
                      <div className="col-span-3 flex items-center justify-end gap-2">
                        <ProgressLine value={project.percent} />
                      </div>
                    )}
                  </a>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </section>
  )
}

const RankedList = ({ title, items, emptyLabel = 'No data available.' }) => {
  const rows = topItems(items, 8)
  const max = Math.max(1, ...rows.map((row) => Number(row.total_seconds) || 0))

  return (
    <section className={`${PANEL} p-5`}>
      <SectionTitle>{title}</SectionTitle>
      {rows.length === 0 ? (
        <EmptyState label={emptyLabel} />
      ) : (
        <div className="space-y-3">
          {rows.map((item) => (
            <div key={item.name} className="text-xs">
              <div className="mb-1 flex items-center justify-between gap-4">
                <span className="truncate text-zinc-300">{item.name}</span>
                <span className="shrink-0 font-mono text-zinc-500 tabular-nums">
                  {item.text || formatShortDuration(item.total_seconds)}
                </span>
              </div>
              <ProgressLine
                value={((Number(item.total_seconds) || 0) / max) * 100}
                accent="#7dd3fc99"
              />
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

const AgentStations = ({ aiModels }) => {
  const rows = topItems(aiModels, 4)

  const getStatus = (index) => {
    if (index === 0)
      return { label: 'Active', textColor: 'text-sky-300', dotClass: 'animate-pulse bg-sky-300' }
    if (index === 1)
      return { label: 'Standby', textColor: 'text-zinc-500', dotClass: 'bg-zinc-500' }
    return { label: 'Idle', textColor: 'text-zinc-600', dotClass: 'bg-zinc-700' }
  }

  return (
    <section className={`${PANEL} p-5`}>
      <SectionTitle>AGENT_STATIONS</SectionTitle>
      {rows.length === 0 ? (
        <EmptyState label="No AI agent activity in this range." />
      ) : (
        <div className="space-y-3">
          {rows.map((station, index) => {
            const { label, textColor, dotClass } = getStatus(index)
            return (
              <div
                key={station.name}
                className={`border p-3 ${index === 0 ? 'border-sky-300/40 bg-sky-300/5' : 'border-zinc-900 bg-zinc-900/30'}`}
              >
                <div className="mb-2 flex items-center justify-between gap-3">
                  <span className="truncate text-xs font-semibold text-zinc-100">
                    {station.name}
                  </span>
                  <span
                    className={`flex items-center gap-1.5 text-[11px] tracking-[0.18em] uppercase ${textColor}`}
                  >
                    <span className={`h-1.5 w-1.5 ${dotClass}`} />
                    {label}
                  </span>
                </div>
                <div className="grid grid-cols-2 text-[11px] text-zinc-500 uppercase">
                  <div>{station.lines > 0 ? `${formatCount(station.lines)} Lines` : formatShortDuration(station.total_seconds)}</div>
                  <div className="text-right">{formatPercent(station.percent)} Load</div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

const osAbbr = (name) => {
  if (name === 'macOS') return 'Mac'
  if (name === 'Windows') return 'Win'
  return name
}

const EditorUsage = ({ editors, operatingSystems, machineCount }) => {
  const normalized = normalizeItems(editors).map((e) => ({
    name: e.editor || e.name || 'Unknown',
    total_seconds: e.duration || e.total_seconds || 0,
    percent: e.percent,
  }))
  const rows = topItems(normalized, 4)
  const total = rows.reduce((sum, e) => sum + (Number(e.total_seconds) || 0), 0)
  const osSplit = topItems(normalizeItems(operatingSystems), 3)
    .map((os) => `${osAbbr(os.name)} ${formatPercent(os.percent)}`)
    .join(' · ')

  return (
    <section className={`${PANEL} p-5`}>
      <SectionTitle>EDITOR_USAGE</SectionTitle>
      {rows.length === 0 ? (
        <EmptyState label="No editor data available." />
      ) : (
        <>
          <div className="space-y-3">
            {rows.map((editor) => {
              const pct = editor.percent ?? (total > 0 ? ((Number(editor.total_seconds) || 0) / total) * 100 : 0)
              return (
                <div key={editor.name} className="text-xs">
                  <div className="mb-1 flex items-center gap-2">
                    <span className="min-w-0 flex-1 truncate text-zinc-300">{editor.name}</span>
                    <span className="shrink-0 font-mono text-zinc-500 tabular-nums">
                      {formatShortDuration(editor.total_seconds)}
                    </span>
                    <span className="w-10 shrink-0 text-right font-mono text-zinc-500 tabular-nums">
                      {formatPercent(pct)}
                    </span>
                  </div>
                  <ProgressLine value={pct} accent="#7dd3fc99" />
                </div>
              )
            })}
          </div>
          {(osSplit || machineCount != null) && (
            <div className="mt-4 grid grid-cols-2 gap-3 border-t border-zinc-900 pt-4 text-[11px] text-zinc-500">
              <div>
                <div className="mb-1 tracking-[0.18em] uppercase">OS Split</div>
                <div>{osSplit || '–'}</div>
              </div>
              <div>
                <div className="mb-1 tracking-[0.18em] uppercase">Machines</div>
                <div>{machineCount != null ? `${machineCount} node${machineCount === 1 ? '' : 's'} online` : '–'}</div>
              </div>
            </div>
          )}
        </>
      )}
    </section>
  )
}

const EmptyState = ({ label }) => (
  <div className="border border-dashed border-zinc-800 p-8 text-sm text-zinc-600">
    {label}
  </div>
)

export default function Dashboard({ config = {} }) {
  return (
    <ThemeProvider>
      <DashboardContent config={config} />
    </ThemeProvider>
  )
}

function DashboardContent({ config }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const fallbackTimezone = effectiveConfig.timezone || detectTimezone()

  const [dashData, setDashData] = useState(() => normalizeDashboardData({}, fallbackTimezone))
  const [liveData, setLiveData] = useState(() => normalizeLiveData({}))
  const [loading, setLoading] = useState(false)
  const [liveLoading, setLiveLoading] = useState(false)
  const [selectedRange, setSelectedRange] = useState(() => {
    if (typeof window === 'undefined') return 'Last 7 Days'
    const p = new URLSearchParams(window.location.search)
    const start = p.get('start')
    const end = p.get('end')
    if (start && end) return 'Custom Range'
    return p.get('range') || 'Last 7 Days'
  })
  const [initialCustomRange] = useState(() => {
    if (typeof window === 'undefined') return undefined
    const p = new URLSearchParams(window.location.search)
    const start = p.get('start')
    const end = p.get('end')
    if (!start || !end) return undefined
    return { from: new Date(`${start}T00:00:00`), to: new Date(`${end}T00:00:00`) }
  })

  const fetchDashboard = async ({ range, start, end }) => {
    setLoading(true)
    try {
      const timezone = dashData.timezone || fallbackTimezone
      const nextData = await fetchDashboardData({
        base: effectiveConfig.apiBase || '',
        timezone,
        range,
        start,
        end,
      })
      setDashData(nextData)
      setLiveData((prev) => ({
        ...prev,
        today: nextData.today,
        projectDurations: nextData.projectDurations,
        languageDurations: nextData.languageDurations,
        errors: [],
      }))
    } catch (error) {
      setDashData((prev) => ({
        ...prev,
        errors: [error instanceof Error ? error.message : 'Failed to load dashboard data'],
      }))
    } finally {
      setLoading(false)
    }
  }

  const refreshLive = async () => {
    setLiveLoading(true)
    try {
      const timezone = dashData.timezone || fallbackTimezone
      const nextLive = await fetchLiveData({
        base: effectiveConfig.apiBase || '',
        timezone,
      })
      setLiveData(nextLive)
    } catch (error) {
      setLiveData((prev) => ({
        ...prev,
        cachedAt: new Date().toISOString(),
        status: 'degraded',
        errors: [error instanceof Error ? error.message : 'Failed to load live data'],
      }))
    } finally {
      setLiveLoading(false)
    }
  }

  useEffect(() => {
    fetchDashboard(
      initialCustomRange
        ? { range: null, start: initialCustomRange.from.toISOString().slice(0, 10), end: initialCustomRange.to.toISOString().slice(0, 10) }
        : { range: selectedRange }
    )
  }, [])

  useEffect(() => {
    refreshLive()
    const intervalID = window.setInterval(refreshLive, 60_000)
    return () => window.clearInterval(intervalID)
  }, [dashData.timezone, fallbackTimezone, effectiveConfig.apiBase])

  const stats = dashData.stats || {}
  const summaries = useMemo(() => normalizeItems(dashData.summaries), [dashData.summaries])
  const today = hasObjectData(liveData.today) ? liveData.today : dashData.today || {}
  const liveProjects = liveData.projectDurations.length
    ? liveData.projectDurations
    : dashData.projectDurations || []
  const liveLanguages = liveData.languageDurations.length
    ? liveData.languageDurations
    : dashData.languageDurations || []
  const errors = useMemo(
    () => normalizeItems([...(dashData.errors || []), ...(liveData.errors || [])]),
    [dashData.errors, liveData.errors]
  )
  const todayRange = today.range || {}

  const trendSeries = useMemo(() => buildTrendSeries(summaries), [summaries])
  const monthlyTraceSeries = useMemo(() => buildMonthlyTraceSeries(summaries), [summaries])
  const hourlyTraceSeries = useMemo(
    () =>
      buildHourlyTraceSeries(
        dashData.projectDurations,
        todayRange.timezone || dashData.timezone || fallbackTimezone
      ),
    [dashData.projectDurations, todayRange.timezone, dashData.timezone, fallbackTimezone]
  )
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])

  const topProjects = useMemo(() => topItems(rangeStats?.projects, 8), [rangeStats])
  const topLanguages = useMemo(() => topItems(rangeStats?.languages, 8), [rangeStats])
  const topMachines = useMemo(() => topItems(rangeStats?.machines, 6), [rangeStats])
  const topEditors = useMemo(() => {
    const items = rangeStats?.editors?.length ? rangeStats.editors : stats.editors
    return topItems(items, 6)
  }, [rangeStats, stats.editors])
  const topOperatingSystems = useMemo(() => topItems(rangeStats?.operatingSystems, 5), [rangeStats])
  const topAiModels = useMemo(() => {
    if (rangeStats?.aiModels?.length) return topItems(rangeStats.aiModels, 4)
    const models = normalizeItems(stats.ai_models)
    const total = models.reduce((s, m) => s + (Number(m.ai_additions) || 0) + (Number(m.ai_deletions) || 0), 0)
    return topItems(
      models.map((m) => ({
        ...m,
        lines: (Number(m.ai_additions) || 0) + (Number(m.ai_deletions) || 0),
        percent: total > 0 ? (((Number(m.ai_additions) || 0) + (Number(m.ai_deletions) || 0)) / total) * 100 : 0,
      })),
      4
    )
  }, [rangeStats, stats.ai_models])

  const totalAi = Number(rangeStats?.aiAdditions ?? stats.ai_additions) || 0
  const totalHuman = Number(rangeStats?.humanAdditions ?? stats.human_additions) || 0
  const aiPercent = aiShare({ aiAdditions: totalAi, humanAdditions: totalHuman })
  const activeDays = rangeStats?.activeDays ?? stats.days_minus_holidays ?? 0
  const totalDays = rangeStats?.totalDays ?? stats.days_including_holidays ?? summaries.length
  const rangeLabel = selectedRange === 'Custom Range' ? 'custom range' : selectedRange.toLowerCase()
  const traceIsMonthly = selectedRange === 'Last Year'
  const traceIsHourly = isSingleDayRange({ range: selectedRange, summaries })
  const traceSeries = traceIsHourly
    ? hourlyTraceSeries
    : traceIsMonthly
      ? monthlyTraceSeries
      : trendSeries
  const traceTitle = traceIsHourly
    ? 'HOURLY_ACTIVITY_TRACE'
    : traceIsMonthly
      ? 'MONTHLY_ACTIVITY_TRACE'
      : 'DAILY_ACTIVITY_TRACE'
  const liveProjectName = today.projects?.[0]?.name || liveProjects[0]?.project || ''
  const liveLanguageName = liveLanguages[0]?.language || topLanguages[0]?.name || ''
  const liveStatus = (liveData.status || 'synchronized').toUpperCase()

  const handleRangeChange = ({ range, start, end }) => {
    const nextRange = range || (start && end ? 'Custom Range' : selectedRange)
    setSelectedRange(nextRange)
    fetchDashboard({ range, start, end })
    const p = new URLSearchParams()
    if (start && end) {
      p.set('start', start)
      p.set('end', end)
    } else {
      p.set('range', nextRange)
    }
    history.replaceState(null, '', `?${p}`)
  }

  return (
    <div className="min-h-screen bg-zinc-950 font-mono text-zinc-100 selection:bg-sky-300/30">
      <nav className="sticky top-0 z-50 border-b border-zinc-900 bg-zinc-950/85 backdrop-blur-sm">
        <div className="md:hidden flex gap-5 border-b border-zinc-900/50 px-4 py-1.5 text-[11px] tracking-[0.24em] text-zinc-500 uppercase">
          <span className="text-sky-300">Dashboard</span>
          <a href="/insights" className="transition-colors hover:text-zinc-100">Insights</a>
          <a href="/wrapped" className="transition-colors hover:text-zinc-100">Wrapped</a>
        </div>
        <div className="mx-auto flex h-12 max-w-[1600px] items-center justify-between gap-4 px-4">
          <div className="flex min-w-0 items-center gap-6">
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 animate-pulse bg-sky-300" />
              <span className="text-sm font-semibold tracking-tight text-zinc-100">
                WAKA_PERSONAL v2
              </span>
            </div>
            <div className="hidden items-center gap-4 text-xs tracking-[0.24em] text-zinc-500 uppercase md:flex">
              <span className="text-sky-300">Dashboard</span>
              <a href="/insights" className="transition-colors hover:text-zinc-100">
                Insights
              </a>
              <a href="/wrapped" className="transition-colors hover:text-zinc-100">
                Wrapped
              </a>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {(loading || liveLoading) && (
              <span className="hidden items-center gap-2 text-[11px] tracking-[0.22em] text-zinc-500 uppercase sm:flex">
                <span className="h-1.5 w-1.5 animate-pulse bg-sky-300" />
                {loading ? 'Syncing' : 'Live tick'}
              </span>
            )}
            <ThemeToggle />
            <DateRangePicker value={selectedRange} onChange={handleRangeChange} initialCustomRange={initialCustomRange} />
          </div>
        </div>
      </nav>

      <main className={`mx-auto max-w-[1600px] p-4 md:p-6 lg:p-8 transition-opacity duration-200 ${loading ? 'opacity-50' : ''}`}>
        <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-2xl font-medium tracking-tight text-zinc-100 md:text-3xl">
              <span className="text-sky-300">$</span> activity_overview
              <span className="ml-2 inline-block h-5 w-2 animate-pulse bg-sky-300 align-middle" />
            </h1>
            <p className="mt-1 text-xs tracking-[0.22em] text-zinc-500 uppercase">
              {rangeLabel} - daily refresh - timezone:{' '}
              {todayRange.timezone || dashData.timezone || fallbackTimezone}
            </p>
          </div>
          <div className="text-right text-[11px] tracking-[0.22em] text-zinc-500 uppercase">
            <div>Range: {rangeDateText(summaries)}</div>
            <div className="text-zinc-500">
              Today: {normalizeWakaTime(today.grand_total?.text) || '0s'}{' '}
              <span className="text-sky-300">LIVE 60s</span>
            </div>
          </div>
        </header>

        {errors.length > 0 && (
          <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
            <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-400" />
            <div>{errors.map((error, i) => <p key={i}>{error}</p>)}</div>
          </div>
        )}

        <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-6">
          <KpiPanel
            label="Active Time"
            value={normalizeWakaTime(
              rangeStats?.humanReadableTotal ||
              stats.human_readable_total_including_other_language
            ) || '-'}
            note={`Avg: ${normalizeWakaTime(rangeStats?.humanReadableDailyAvg || stats.human_readable_daily_average_including_other_language) || '0m'} / active day`}
            icon={Activity}
          />
          <KpiPanel
            label="AI Contribution"
            value={formatPercent(aiPercent)}
            note={`${formatCount(totalAi)} AI - ${formatCount(totalHuman)} Human additions`}
            icon={Zap}
            accent="#bae6fd"
          />
          <KpiPanel
            label="Active Days"
            value={`${activeDays}/${totalDays || 0}`}
            note={`Best: ${normalizeWakaTime(rangeStats?.bestDay?.text || stats.best_day?.text) || 'No peak day yet'}`}
            icon={Terminal}
          />
          <KpiPanel
            label="AI Tokens"
            value={formatCount(dashData.tokenMetrics?.total_tokens || 0)}
            note={`${formatCount(dashData.tokenMetrics?.input_tokens || 0)} in · ${formatCount(dashData.tokenMetrics?.output_tokens || 0)} out`}
            icon={Code2}
          />
          <KpiPanel
            label="AI Spend"
            value={formatSpend(dashData.spendMetrics?.estimated_cents || 0)}
            note={`${formatCount(dashData.spendMetrics?.token_count || 0)} tokens consumed`}
            icon={Zap}
            accent="#fbbf24"
          />
          <KpiPanel
            label="Today"
            value={normalizeWakaTime(today.grand_total?.text) || '0s'}
            note={liveProjectName ? `Project: ${liveProjectName}` : 'No focused project yet'}
            icon={Cpu}
          />
        </div>

        <div className="grid grid-cols-12 gap-6">
          <div className="col-span-12 space-y-6 lg:col-span-8">
            <AiSplit
              stats={{
                aiAdditions: totalAi,
                humanAdditions: totalHuman,
                aiDeletions: rangeStats?.aiDeletions ?? stats.ai_deletions,
              }}
              topProject={topProjects[0]}
              rangeStats={rangeStats}
            />
            <DailyTrace
              series={traceSeries}
              title={traceTitle}
              showWeekday={!traceIsMonthly && !traceIsHourly}
            />
            <ProjectMetrics projects={topProjects} summaries={summaries} />
          </div>

          <aside className="col-span-12 space-y-6 lg:col-span-4">
            <AgentStations aiModels={topAiModels} />
            <EditorUsage
              editors={topEditors.length ? topEditors : dashData.editorDurations || []}
              operatingSystems={topOperatingSystems}
              machineCount={topMachines.length}
            />
            <RankedList
              title="LANGUAGE_INDEX"
              items={topLanguages}
              emptyLabel="No language index yet."
            />
          </aside>
        </div>

        <footer className="mt-8 flex flex-wrap items-center gap-x-8 gap-y-2 border border-zinc-900 bg-zinc-950 p-4 text-[11px] text-zinc-500">
          <div className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 animate-pulse bg-sky-300" />
            <span className="text-zinc-500 uppercase">Node Status:</span>
            <span className="text-sky-300 uppercase">{liveStatus}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-zinc-500 uppercase">Live Project:</span>
            <span className="text-zinc-300">{liveProjectName || 'local'}</span>
          </div>
          <div className="flex items-center gap-2">
            <Code2 size={12} />
            <span className="text-zinc-300">{liveLanguageName || 'No language'}</span>
          </div>
          <div className="ml-auto flex gap-4">
            <span>API</span>
            <span>CONFIG</span>
            <span className="text-border">-</span>
            <span>LAST_REFRESH: {refreshTimeText(liveData.cachedAt)}</span>
          </div>
        </footer>
      </main>
    </div>
  )
}
