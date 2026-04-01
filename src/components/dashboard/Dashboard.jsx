import { useState, useEffect, useMemo } from 'react'
import { Activity, CalendarDays, BrainCircuit } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import ThemeToggle from './ThemeToggle'
import StatCard from './StatCard'
import DailyTrendChart from './DailyTrendChart'
import TimelineChart from './TimelineChart'
import DeltaBars from './DeltaBars'
import BreakdownCard from './BreakdownCard'
import DateRangePicker from './DateRangePicker'
import {
  buildDeltaSeries,
  buildTimelineRows,
  buildTrendSeries,
  formatDayLabel,
  normalizeItems,
  palette,
  topItems,
} from './dashboardUtils.js'

// Wrap with ThemeProvider so the React island owns its own theme context
export default function Dashboard({ data = {} }) {
  return (
    <ThemeProvider>
      <DashboardContent data={data} />
    </ThemeProvider>
  )
}

function DashboardContent({ data }) {
  const [dashData, setDashData] = useState(data)
  const [loading, setLoading] = useState(false)
  const [selectedRange, setSelectedRange] = useState('Last 7 Days')

  const fetchDashboard = async ({ range, start, end }) => {
    setLoading(true)
    try {
      const timezone = dashData.timezone || 'UTC'
      const params = new URLSearchParams({ timezone })
      if (range) params.set('range', range)
      if (start) params.set('start', start)
      if (end) params.set('end', end)
      const res = await fetch(`/api/v1/users/current/dashboard?${params}`)
      if (res.ok) {
        const json = await res.json()
        setDashData((prev) => ({
          timezone: prev.timezone,
          stats: json.stats || {},
          summaries: json.summaries || [],
          today: json.today || {},
          projectDurations: json.project_durations || [],
          languageDurations: json.language_durations || [],
          errors: json.errors || [],
        }))
      }
    } catch (_) {}
    setLoading(false)
  }

  const stats = dashData.stats || {}
  const summaries = useMemo(() => normalizeItems(dashData.summaries), [dashData.summaries])
  const today = dashData.today || {}
  const errors = useMemo(() => normalizeItems(dashData.errors), [dashData.errors])
  const todayRange = today.range || {}

  const trendSeries = useMemo(() => buildTrendSeries(summaries), [summaries])
  const deltaSeries = useMemo(() => buildDeltaSeries(summaries), [summaries])
  const projectRows = useMemo(
    () =>
      buildTimelineRows(
        dashData.projectDurations,
        'project',
        todayRange.start,
        todayRange.end
      ),
    [dashData.projectDurations, todayRange.start, todayRange.end]
  )
  const languageRows = useMemo(
    () =>
      buildTimelineRows(
        dashData.languageDurations,
        'language',
        todayRange.start,
        todayRange.end
      ),
    [dashData.languageDurations, todayRange.start, todayRange.end]
  )
  const topProjects = useMemo(() => topItems(stats.projects, 6), [stats.projects])
  const topLanguages = useMemo(() => topItems(stats.languages, 6), [stats.languages])
  const topMachines = useMemo(() => topItems(stats.machines, 6), [stats.machines])
  const topCategories = useMemo(
    () =>
      topItems(stats.categories, 6).map((item, index) => ({
        ...item,
        color: palette[index % palette.length],
      })),
    [stats.categories]
  )

  const topProject = topProjects[0]
  const topLanguage = topLanguages[0]
  const topMachine = topMachines[0]
  const rangeLabel =
    selectedRange === 'Custom Range' ? 'custom range' : selectedRange.toLowerCase()

  const handleRangeChange = ({ range, start, end }) => {
    if (range) setSelectedRange(range)
    fetchDashboard({ range, start, end })
  }

  return (
    <div className="dashboard-shell bg-background text-foreground min-h-screen">
      <div className="dashboard-grid pointer-events-none fixed inset-0 opacity-70" />

      <div className="relative mx-auto flex min-h-screen w-full max-w-[1500px] flex-col gap-6 px-4 py-6 md:px-6 lg:px-8">
        <header className="border-border bg-background/80 border p-5 backdrop-blur-sm">
          <div className="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
            <div className="max-w-4xl">
              <div className="text-foreground/55 mb-4 flex flex-wrap items-center gap-3 text-[10px] font-semibold tracking-[0.35em] uppercase">
                <span>Waka Personal</span>
                <span className="bg-border h-px w-8" />
                <span>{todayRange.timezone || timezone}</span>
              </div>

              <h1 className="text-foreground max-w-4xl text-4xl font-semibold tracking-tight md:text-6xl">
                {stats.human_readable_total_including_other_language || '0 secs'}
                <span className="block text-base font-medium tracking-[0.25em] text-sky-400 uppercase md:mt-3 md:text-lg">
                  over {rangeLabel}
                </span>
              </h1>

              <div className="text-foreground/65 mt-5 flex flex-wrap gap-3 text-sm">
                {topProject && (
                  <span className="border-border border px-3 py-2">
                    Top project: {topProject.name}
                  </span>
                )}
                {topLanguage && (
                  <span className="border-border border px-3 py-2">
                    Top language: {topLanguage.name}
                  </span>
                )}
                {topMachine && (
                  <span className="border-border border px-3 py-2">
                    Main machine: {topMachine.name}
                  </span>
                )}
              </div>
            </div>

            <div className="flex flex-col items-start gap-4 xl:items-end">
              <div className="flex items-center gap-3">
                <ThemeToggle />
                <DateRangePicker value={selectedRange} onChange={handleRangeChange} />
              </div>
              {loading ? (
                <p className="text-foreground/40 text-xs tracking-[0.25em] uppercase">
                  Loading…
                </p>
              ) : (
                <p className="text-foreground/60 max-w-sm text-sm xl:text-right">
                  Mirrors WakaTime-style stats from your local heartbeats, then pushes the most
                  useful signals up front.
                </p>
              )}
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

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatCard
            label="Daily Average"
            value={stats.human_readable_daily_average_including_other_language || '0 secs'}
            note={`${stats.days_minus_holidays || 0} active days in this range`}
            accent="#38bdf8"
          />
          <StatCard
            label="Best Day"
            value={stats.best_day?.text || '0 secs'}
            note={stats.best_day?.date ? formatDayLabel(stats.best_day.date) : 'No peak day yet'}
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
            value={`${(stats.ai_additions || 0).toLocaleString()} / ${(stats.human_additions || 0).toLocaleString()}`}
            note="Additions over the last 7 days"
            accent="#f59e0b"
          />
        </section>

        <section className="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(340px,1fr)]">
          <DailyTrendChart
            title="Daily Activity"
            subtitle="Stacked by top categories with total activity overlaid."
            days={trendSeries}
          />

          <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
            <div className="mb-4 flex items-center gap-2">
              <Activity size={16} className="text-sky-400" />
              <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
                Momentum Mix
              </p>
            </div>

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
          </section>
        </section>

        <section className="grid gap-4 xl:grid-cols-2">
          <TimelineChart
            title="Project Timeline"
            subtitle="Today sliced by project."
            rows={projectRows}
          />
          <TimelineChart
            title="Language Timeline"
            subtitle="Today sliced by language."
            rows={languageRows}
          />
        </section>

        <DeltaBars
          title="AI vs Human Line Changes"
          subtitle="Positive bars are additions, negative bars are deletions inferred from heartbeat line-change totals."
          series={deltaSeries}
        />

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

        <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
          <div className="mb-4 flex flex-wrap gap-6 text-sm text-foreground/65">
            <div className="flex items-start gap-3">
              <CalendarDays size={16} className="mt-0.5 shrink-0 text-sky-400" />
              <span>
                {stats.days_minus_holidays || 0} of {stats.days_including_holidays || 0} days had
                measurable activity.
              </span>
            </div>
            <div className="flex items-start gap-3">
              <BrainCircuit size={16} className="mt-0.5 shrink-0 text-cyan-300" />
              <span>
                AI and human additions/deletions are inferred from heartbeat net line-change values
                stored locally.
              </span>
            </div>
          </div>
        </section>
      </div>

      <style>{`
        .dashboard-shell {
          position: relative;
        }
        .dashboard-grid {
          background-image:
            linear-gradient(to right, rgba(148, 163, 184, 0.08) 1px, transparent 1px),
            linear-gradient(to bottom, rgba(148, 163, 184, 0.08) 1px, transparent 1px),
            radial-gradient(circle at top left, rgba(56, 189, 248, 0.12), transparent 28%),
            radial-gradient(circle at bottom right, rgba(34, 197, 94, 0.1), transparent 24%);
          background-size: 48px 48px, 48px 48px, 100% 100%, 100% 100%;
        }
      `}</style>
    </div>
  )
}
