import { useEffect, useMemo, useState } from 'react'
import { Activity, Code2, Zap } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import {
  buildTrendSeries,
  computeRangeStats,
  formatCount,
  formatPercent,
  formatShortDuration,
  formatSpend,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-900 bg-zinc-950/80'
const LABEL = 'text-[10px] tracking-[0.24em] text-zinc-500 uppercase'

const Kpi = ({ label, value, note, icon: Icon, accent = '#7dd3fc' }) => (
  <section className={`${PANEL} p-4`}>
    <div className="mb-3 flex items-center justify-between gap-3">
      <span className={LABEL}>{label}</span>
      {Icon && <Icon size={15} style={{ color: accent }} />}
    </div>
    <div className="font-mono text-3xl leading-none font-medium text-zinc-100">{value}</div>
    <div className="mt-3 text-[11px] text-zinc-600">{note}</div>
  </section>
)

const SectionTitle = ({ children }) => (
  <h2 className="mb-6 flex items-center gap-2 text-sm font-medium text-zinc-400">
    <span className="h-1.5 w-1.5 bg-zinc-600" />
    {children}
  </h2>
)

const ProgressLine = ({ value = 0 }) => (
  <div className="h-2 bg-zinc-900">
    <div className="h-full bg-sky-300/70" style={{ width: `${Math.max(0, Math.min(100, value))}%` }} />
  </div>
)

const loadReports = async ({ base, timezone, range }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/dashboard',
    params: { range, timezone },
  })
  if (error || !data) {
    return {
      timezone,
      range,
      summaries: [],
      stats: {},
      projectDurations: [],
      tokenMetrics: {},
      spendMetrics: {},
      errors: [error || 'Failed to load reports'],
    }
  }
  return {
    timezone: data.timezone || timezone,
    range: range,
    summaries: data.summaries || [],
    stats: data.stats || {},
    projectDurations: data.project_durations || [],
    tokenMetrics: data.token_metrics || {},
    spendMetrics: data.spend_metrics || {},
    errors: Array.isArray(data.errors) ? data.errors : [],
  }
}

function ReportsContent({ config = {} }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const fallbackTimezone = effectiveConfig.timezone || detectTimezone()
  const [selectedRange, setSelectedRange] = useState('Last 30 Days')
  const [data, setData] = useState({
    timezone: fallbackTimezone,
    range: selectedRange,
    summaries: [],
    stats: {},
    projectDurations: [],
    tokenMetrics: {},
    spendMetrics: {},
    errors: [],
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    setLoading(true)
    loadReports({
      base: effectiveConfig.apiBase || '',
      timezone: fallbackTimezone,
      range: selectedRange,
    }).then((next) => {
      if (active) {
        setData(next)
        setLoading(false)
      }
    })
    return () => {
      active = false
    }
  }, [effectiveConfig.apiBase, fallbackTimezone, selectedRange])

  const summaries = useMemo(() => normalizeItems(data.summaries), [data.summaries])
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])
  const projects = useMemo(() => topItems(rangeStats?.projects, 10), [rangeStats])
  const editors = useMemo(
    () => topItems(data.projectDurations?.filter((p) => p.editor) || [], 6),
    [data.projectDurations]
  )
  const aiTotal = Number(rangeStats?.aiAdditions) + Number(rangeStats?.aiDeletions)
  const humanTotal = Number(rangeStats?.humanAdditions) + Number(rangeStats?.humanDeletions)
  const changeTotal = Math.max(1, aiTotal + humanTotal)
  const aiPercent = (aiTotal / changeTotal) * 100

  return (
    <>
      <div className="mb-8 flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-medium tracking-tight text-zinc-100 md:text-3xl">
            <span className="text-sky-300">$</span> comprehensive_reports
            <span className="ml-2 inline-block h-5 w-2 animate-pulse bg-sky-300 align-middle" />
          </h1>
          <p className="mt-1 text-xs tracking-[0.22em] text-zinc-500 uppercase">
            Detailed metrics and insights — timezone: {data.timezone}
          </p>
        </div>

        <select
          value={selectedRange}
          onChange={(e) => setSelectedRange(e.target.value)}
          className="border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-300 focus:border-sky-300 focus:outline-none"
        >
          <option>Last 7 Days</option>
          <option>Last 14 Days</option>
          <option>Last 30 Days</option>
          <option>Last 90 Days</option>
          <option>Last Year</option>
        </select>
      </div>

      {data.errors.length > 0 && (
        <div className="mb-6 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          {data.errors.map((error, i) => (
            <p key={i}>{error}</p>
          ))}
        </div>
      )}

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Kpi
          label="Total Active Time"
          value={rangeStats?.humanReadableTotal || '-'}
          note={`Avg: ${rangeStats?.humanReadableDailyAvg || '0s'} / active day`}
          icon={Activity}
        />
        <Kpi
          label="AI Tokens Consumed"
          value={formatCount(data.tokenMetrics?.total_tokens || 0)}
          note={`Input: ${formatCount(data.tokenMetrics?.input_tokens || 0)}`}
          icon={Code2}
        />
        <Kpi
          label="Estimated Spend"
          value={formatSpend(data.spendMetrics?.estimated_cents || 0)}
          note={`${formatCount(data.spendMetrics?.token_count || 0)} tokens`}
          icon={Zap}
          accent="#fbbf24"
        />
        <Kpi
          label="AI Contribution"
          value={formatPercent(aiPercent)}
          note={`${formatCount(aiTotal)} changes — ${formatCount(humanTotal)} human`}
          icon={Zap}
          accent="#bae6fd"
        />
      </div>

      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-12 space-y-6 lg:col-span-8">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>PROJECT_BREAKDOWN</SectionTitle>
            {projects.length === 0 ? (
              <div className="border border-dashed border-zinc-800 p-8 text-sm text-zinc-600">
                No project data available.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <div className="min-w-[980px]">
                  <div className="grid grid-cols-12 gap-4 border-b border-zinc-900 pb-2 text-[10px] tracking-[0.18em] text-zinc-600 uppercase">
                    <div className="col-span-4">Project</div>
                    <div className="col-span-3 text-right">Time</div>
                    <div className="col-span-3 text-right">AI %</div>
                    <div className="col-span-2 text-right">Activity Share</div>
                  </div>
                  <div className="divide-y divide-zinc-900/70">
                    {projects.map((project) => (
                      <div
                        key={project.name}
                        className="grid grid-cols-12 items-center gap-4 py-3 text-[11px]"
                      >
                        <div className="col-span-4 flex min-w-0 items-center gap-2 text-zinc-300">
                          <span className="text-zinc-700">&gt;</span>
                          <span className="truncate">{project.name}</span>
                        </div>
                        <div className="col-span-3 text-right font-mono text-zinc-400 tabular-nums">
                          {project.text || formatShortDuration(project.total_seconds)}
                        </div>
                        <div className="col-span-3 text-right font-mono text-sky-300 tabular-nums">
                          {formatPercent(project.ai_percent)}
                        </div>
                        <div className="col-span-2 grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                          <ProgressLine value={project.percent} />
                          <span className="text-right font-mono text-[10px] text-zinc-500">
                            {formatPercent(project.percent)}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </section>
        </div>

        <aside className="col-span-12 space-y-6 lg:col-span-4">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>EDITOR_DISTRIBUTION</SectionTitle>
            {editors.length === 0 ? (
              <div className="border border-dashed border-zinc-800 p-8 text-sm text-zinc-600">
                No editor data available.
              </div>
            ) : (
              <div className="space-y-3">
                {editors.map((editor) => {
                  const total = editors.reduce((sum, e) => sum + (Number(e.total_seconds) || 0), 0)
                  return (
                    <div key={editor.name} className="text-[11px]">
                      <div className="mb-1 flex items-center justify-between gap-3">
                        <span className="truncate text-zinc-300">{editor.name}</span>
                        <span className="font-mono text-zinc-500 tabular-nums">
                          {formatPercent((Number(editor.total_seconds) / total) * 100)}
                        </span>
                      </div>
                      <ProgressLine value={(Number(editor.total_seconds) / total) * 100} />
                    </div>
                  )
                })}
              </div>
            )}
          </section>
        </aside>
      </div>
    </>
  )
}

export default function Reports(props) {
  return (
    <ThemeProvider>
      <ReportsContent {...props} />
    </ThemeProvider>
  )
}
