import { useState, useEffect, useMemo } from 'react'
import { Activity } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ThemeProvider } from '@/stores/theme'
import ThemeToggle from './ThemeToggle'
import StatCard from './StatCard'
import DailyTrendChart from './DailyTrendChart'
import TimelineChart from './TimelineChart'
import DeltaBars from './DeltaBars'
import BreakdownCard from './BreakdownCard'
import DateRangePicker from './DateRangePicker'
import {
  buildDailyBreakdownRows,
  buildDeltaSeries,
  buildTimelineRows,
  buildTrendSeries,
  computeRangeStats,
  formatDayLabel,
  normalizeItems,
  palette,
  topItems,
} from './dashboardUtils.js'

const detectTimezone = (fallback = 'UTC') => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || fallback
  } catch (_) {
    return fallback
  }
}

const normalizeDashboardData = (data = {}, fallbackTimezone = 'UTC') => ({
  timezone: data.timezone || fallbackTimezone,
  stats: data.stats || {},
  summaries: data.summaries || [],
  today: data.today || {},
  projectDurations: data.projectDurations || [],
  languageDurations: data.languageDurations || [],
  errors: Array.isArray(data.errors) ? data.errors : [],
})

const buildApiUrl = (base, path, params = {}, apiKey = '') => {
  const root = base || window.location.origin
  const url = new URL(path, root)
  for (const [key, value] of Object.entries(params)) {
    if (value != null && value !== '') {
      url.searchParams.set(key, String(value))
    }
  }
  if (apiKey) {
    url.searchParams.set('api_key', apiKey)
  }
  return base ? url.toString() : `${url.pathname}${url.search}`
}

const fetchJson = async ({ base, path, params, apiKey }) => {
  try {
    const res = await fetch(buildApiUrl(base, path, params, apiKey))
    if (!res.ok) {
      return { data: null, error: `${path} returned ${res.status}` }
    }
    return { data: await res.json(), error: null }
  } catch (error) {
    return {
      data: null,
      error: `${path} failed: ${error instanceof Error ? error.message : 'request failed'}`,
    }
  }
}

const fetchDashboardData = async ({ base, apiKey, timezone, range, start, end }) => {
  const selectedRange = range || 'Last 7 Days'
  const params = start && end ? { start, end, timezone } : { range: selectedRange, timezone }

  const { data, error } = await fetchJson({
    base,
    path: '/api/v1/users/current/dashboard',
    params,
    apiKey,
  })

  if (error || !data) {
    return {
      timezone,
      stats: {},
      summaries: [],
      today: {},
      projectDurations: [],
      languageDurations: [],
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
    errors: Array.isArray(data.errors) ? data.errors : [],
  }
}

// Wrap with ThemeProvider so the React island owns its own theme context
export default function Dashboard({ data = {}, config = {} }) {
  return (
    <ThemeProvider>
      <DashboardContent data={data} config={config} />
    </ThemeProvider>
  )
}

function DashboardContent({ data, config }) {
  const fallbackTimezone = config.timezone || detectTimezone()
  const hasInitialData =
    Object.keys(data.stats || {}).length > 0 ||
    (data.summaries || []).length > 0 ||
    Object.keys(data.today || {}).length > 0 ||
    (data.projectDurations || []).length > 0 ||
    (data.languageDurations || []).length > 0

  const [dashData, setDashData] = useState(() => normalizeDashboardData(data, fallbackTimezone))
  const [loading, setLoading] = useState(false)
  const [selectedRange, setSelectedRange] = useState('Last 7 Days')

  const fetchDashboard = async ({ range, start, end }) => {
    setLoading(true)
    try {
      const timezone = dashData.timezone || fallbackTimezone
      const nextData = await fetchDashboardData({
        base: config.apiBase || '',
        apiKey: config.apiKey || '',
        timezone,
        range,
        start,
        end,
      })
      setDashData(nextData)
    } catch (error) {
      setDashData((prev) => ({
        ...prev,
        errors: [error instanceof Error ? error.message : 'Failed to load dashboard data'],
      }))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (hasInitialData) {
      return
    }
    fetchDashboard({ range: selectedRange })
  }, [])

  const stats = dashData.stats || {}
  const summaries = useMemo(() => normalizeItems(dashData.summaries), [dashData.summaries])
  const today = dashData.today || {}
  const errors = useMemo(() => normalizeItems(dashData.errors), [dashData.errors])
  const todayRange = today.range || {}

  const isMultiDay = summaries.length > 7
  const trendSeries = useMemo(() => buildTrendSeries(summaries), [summaries])
  const deltaSeries = useMemo(() => buildDeltaSeries(summaries), [summaries])
  const hasDeltaData = useMemo(
    () =>
      deltaSeries.some(
        (d) =>
          d.aiAdditions > 0 || d.aiDeletions > 0 || d.humanAdditions > 0 || d.humanDeletions > 0
      ),
    [deltaSeries]
  )
  const projectRows = useMemo(
    () =>
      isMultiDay
        ? buildDailyBreakdownRows(summaries, 'projects')
        : buildTimelineRows(dashData.projectDurations, 'project', todayRange.start, todayRange.end),
    [isMultiDay, summaries, dashData.projectDurations, todayRange.start, todayRange.end]
  )
  const languageRows = useMemo(
    () =>
      isMultiDay
        ? buildDailyBreakdownRows(summaries, 'languages')
        : buildTimelineRows(
            dashData.languageDurations,
            'language',
            todayRange.start,
            todayRange.end
          ),
    [isMultiDay, summaries, dashData.languageDurations, todayRange.start, todayRange.end]
  )
  const timelineAxisLabels = useMemo(
    () => (isMultiDay ? summaries.map((day) => formatDayLabel(day.range?.date)) : null),
    [isMultiDay, summaries]
  )
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])

  const topProjects = useMemo(() => topItems(rangeStats?.projects, 6), [rangeStats])
  const topLanguages = useMemo(() => topItems(rangeStats?.languages, 6), [rangeStats])
  const topMachines = useMemo(() => topItems(rangeStats?.machines, 6), [rangeStats])
  const topCategories = useMemo(
    () =>
      topItems(rangeStats?.categories, 6).map((item, index) => ({
        ...item,
        color: palette[index % palette.length],
      })),
    [rangeStats]
  )

  const topProject = topProjects[0]
  const topLanguage = topLanguages[0]
  const topMachine = topMachines[0]
  const rangeLabel = selectedRange === 'Custom Range' ? 'custom range' : selectedRange.toLowerCase()

  const handleRangeChange = ({ range, start, end }) => {
    const nextRange = range || (start && end ? 'Custom Range' : selectedRange)
    setSelectedRange(nextRange)
    fetchDashboard({ range, start, end })
  }

  return (
    <div className="bg-background text-foreground relative min-h-screen">
      <div className="dashboard-grid pointer-events-none fixed inset-0 opacity-70" />

      <div className="relative mx-auto flex min-h-screen w-full max-w-[1500px] flex-col gap-6 px-4 py-6 md:px-6 lg:px-8">
        <header className="border-border bg-background/80 relative z-10 border p-5 backdrop-blur-sm">
          {loading && (
            <div className="absolute inset-x-0 top-0 h-[2px] overflow-hidden">
              <div className="animate-loading-bar h-full w-1/3 bg-sky-400" />
            </div>
          )}
          <div className="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
            <div className="max-w-4xl">
              <div className="text-foreground/55 mb-4 flex flex-wrap items-center gap-3 text-[10px] font-semibold tracking-[0.35em] uppercase">
                <span>Waka Personal</span>
                <span className="bg-border h-px w-8" />
                <span>{todayRange.timezone || dashData.timezone || fallbackTimezone}</span>
              </div>

              <h1 className="text-foreground max-w-4xl text-4xl font-semibold tracking-tight md:text-6xl">
                {rangeStats?.humanReadableTotal ||
                  stats.human_readable_total_including_other_language ||
                  '—'}
                <span className="block text-base font-medium tracking-[0.25em] text-sky-400 uppercase md:mt-3 md:text-lg">
                  over {rangeLabel}
                </span>
              </h1>

              <div className="text-foreground/65 mt-5 flex flex-wrap gap-3 text-sm">
                {topProject && (
                  <span className="border-border border px-3 py-2 text-xs tracking-wide">
                    <span className="text-foreground/40 mr-1.5">Project</span>
                    {topProject.name}
                  </span>
                )}
                {topLanguage && (
                  <span className="border-border border px-3 py-2 text-xs tracking-wide">
                    <span className="text-foreground/40 mr-1.5">Language</span>
                    {topLanguage.name}
                  </span>
                )}
                {topMachine && (
                  <span className="border-border border px-3 py-2 text-xs tracking-wide">
                    <span className="text-foreground/40 mr-1.5">Machine</span>
                    {topMachine.name}
                  </span>
                )}
              </div>
            </div>

            <div className="flex flex-col items-start gap-3 xl:items-end">
              <div className="flex items-center gap-3">
                {loading && (
                  <span className="text-foreground/40 flex items-center gap-1.5 text-[10px] tracking-[0.25em] uppercase">
                    <span className="inline-block h-1.5 w-1.5 animate-pulse bg-sky-400" />
                    Updating
                  </span>
                )}
                <ThemeToggle />
                <DateRangePicker value={selectedRange} onChange={handleRangeChange} />
              </div>
              <p className="text-foreground/40 max-w-sm text-xs xl:text-right">
                {rangeStats
                  ? `${rangeStats.activeDays} active days · ${rangeStats.humanReadableDailyAvg} avg/day`
                  : 'Local WakaTime-compatible stats'}
              </p>
            </div>
          </div>

          {errors.length > 0 && (
            <div className="mt-5 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
              {errors.map((error, i) => (
                <p key={i}>{error}</p>
              ))}
            </div>
          )}
        </header>

        <div
          className={`flex flex-col gap-6 transition-opacity duration-300 ${loading ? 'pointer-events-none opacity-50' : 'opacity-100'}`}
        >
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <StatCard
              label="Daily Average"
              value={
                rangeStats?.humanReadableDailyAvg ||
                stats.human_readable_daily_average_including_other_language ||
                '—'
              }
              note={`${rangeStats?.activeDays ?? stats.days_minus_holidays ?? 0} active days in this range`}
              accent="#38bdf8"
            />
            <StatCard
              label="Best Day"
              value={rangeStats?.bestDay?.text || stats.best_day?.text || '—'}
              note={
                rangeStats?.bestDay?.date || stats.best_day?.date
                  ? formatDayLabel(rangeStats?.bestDay?.date || stats.best_day?.date)
                  : 'No peak day yet'
              }
              accent="#22c55e"
            />
            <StatCard
              label="Today"
              value={today.grand_total?.text || '0 secs'}
              note={
                today.projects?.[0]
                  ? `Leading project: ${today.projects[0].name}`
                  : 'No focused project yet'
              }
              accent="#818cf8"
            />
            <StatCard
              label="AI vs Human"
              value={`${(rangeStats?.aiAdditions ?? stats.ai_additions ?? 0).toLocaleString()} / ${(rangeStats?.humanAdditions ?? stats.human_additions ?? 0).toLocaleString()}`}
              note={`AI / human additions over ${rangeLabel}`}
              accent="#f59e0b"
            />
          </section>

          <section className="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(340px,1fr)]">
            <DailyTrendChart
              title="Daily Activity"
              subtitle="Stacked by top categories with total activity overlaid."
              days={trendSeries}
              range={selectedRange}
            />

            <Card className="border-border/80 bg-background/75 shadow-none backdrop-blur-sm">
              <CardHeader className="flex-row items-center gap-2 p-5 pb-0">
                <Activity size={16} className="text-sky-400" />
                <CardTitle className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
                  Momentum Mix
                </CardTitle>
              </CardHeader>

              <CardContent className="p-5">
                {topCategories.length === 0 ? (
                  <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
                    No category data yet.
                  </div>
                ) : (
                  <div className="space-y-4">
                    {topCategories.map((item) => (
                      <div key={item.name}>
                        <div className="mb-2 flex items-center justify-between gap-3 text-sm">
                          <span className="text-foreground truncate font-medium">{item.name}</span>
                          <span className="text-foreground/60">{item.text}</span>
                        </div>
                        <div className="border-border bg-foreground/[0.04] h-3 border">
                          <div
                            className="h-full"
                            style={{
                              width: `${Math.max(4, item.percent || 0)}%`,
                              background: item.color,
                            }}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </section>

          <section className="grid gap-4 xl:grid-cols-2">
            <TimelineChart
              title="Project Timeline"
              subtitle={
                isMultiDay
                  ? `Daily breakdown by project over ${rangeLabel}.`
                  : 'Today sliced by project.'
              }
              rows={projectRows}
              axisLabels={timelineAxisLabels}
            />
            <TimelineChart
              title="Language Timeline"
              subtitle={
                isMultiDay
                  ? `Daily breakdown by language over ${rangeLabel}.`
                  : 'Today sliced by language.'
              }
              rows={languageRows}
              axisLabels={timelineAxisLabels}
            />
          </section>

          {hasDeltaData && (
            <DeltaBars
              title="AI vs Human Line Changes"
              subtitle="Positive bars are additions, negative bars are deletions inferred from heartbeat line-change totals."
              series={deltaSeries}
            />
          )}

          <section className="grid gap-4 xl:grid-cols-3">
            <BreakdownCard
              title="Projects"
              subtitle="Where the week was spent."
              items={topProjects}
              emptyLabel="No project breakdown available."
            />
            <BreakdownCard
              title="Languages"
              subtitle="What you actually wrote."
              items={topLanguages}
              emptyLabel="No language breakdown available."
            />
            <BreakdownCard
              title="Machines"
              subtitle="Which machines carried the load."
              items={topMachines}
              emptyLabel="No machine data available."
            />
          </section>

          <footer className="border-border/40 border-t pt-6 pb-2">
            <div className="text-foreground/40 flex flex-wrap items-center justify-between gap-4 text-xs">
              <div className="flex flex-wrap items-center gap-4">
                <span>
                  Built by{' '}
                  <a
                    href="https://dvgamerr.app/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-foreground/60 hover:text-foreground underline underline-offset-2 transition"
                  >
                    dvgamerr
                  </a>
                </span>
                <span className="bg-border h-3 w-px" />
                <a
                  href="https://github.com/dvgamerr/waka-personal"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-foreground/60 hover:text-foreground inline-flex items-center gap-1.5 transition"
                >
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                  waka-personal
                </a>
                <span className="bg-border h-3 w-px" />
                <span>
                  {rangeStats?.activeDays ?? 0}/{rangeStats?.totalDays ?? 0} active days in range
                </span>
              </div>
              <span className="text-foreground/25 text-[10px] tracking-widest uppercase">
                WakaTime-compatible · self-hosted
              </span>
            </div>
          </footer>
        </div>
      </div>

      <style>{`
        .dashboard-grid {
          background-image:
            linear-gradient(to right, rgba(148, 163, 184, 0.08) 1px, transparent 1px),
            linear-gradient(to bottom, rgba(148, 163, 184, 0.08) 1px, transparent 1px),
            radial-gradient(circle at top left, rgba(56, 189, 248, 0.12), transparent 28%),
            radial-gradient(circle at bottom right, rgba(34, 197, 94, 0.1), transparent 24%);
          background-size: 48px 48px, 48px 48px, 100% 100%, 100% 100%;
        }
        @keyframes loading-bar {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(400%); }
        }
        .animate-loading-bar {
          animation: loading-bar 1.2s ease-in-out infinite;
        }
      `}</style>
    </div>
  )
}
