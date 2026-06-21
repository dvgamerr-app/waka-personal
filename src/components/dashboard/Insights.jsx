import { useEffect, useMemo, useState } from 'react'
import { Activity, GitBranch, Search, Zap } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import {
  buildTrendSeries,
  computeRangeStats,
  formatCount,
  formatPercent,
  formatShortDuration,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-900 bg-zinc-950/80'
const LABEL = 'text-[10px] tracking-[0.24em] text-zinc-500 uppercase'

const Kpi = ({ label, value, note, icon: Icon }) => (
  <section className={`${PANEL} p-4`}>
    <div className="mb-3 flex items-center justify-between gap-3">
      <span className={LABEL}>{label}</span>
      {Icon && <Icon size={15} className="text-sky-300" />}
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
    <div
      className="h-full bg-sky-300/70"
      style={{ width: `${Math.max(0, Math.min(100, value))}%` }}
    />
  </div>
)

const loadInsights = async ({ base, timezone }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/insights',
    params: { range: 'Last 7 Days', timezone },
  })
  if (error || !data) {
    return {
      timezone,
      summaries: [],
      stats: {},
      tokenMetrics: {},
      spendMetrics: {},
      errors: [error || 'Failed to load insights'],
    }
  }
  return {
    timezone: data.timezone || timezone,
    summaries: data.summaries || [],
    stats: data.stats || {},
    tokenMetrics: data.token_metrics || {},
    spendMetrics: data.spend_metrics || {},
    generatedAt: data.generated_at || '',
    errors: [],
  }
}

function InsightsContent({ config = {} }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const fallbackTimezone = effectiveConfig.timezone || detectTimezone()
  const [data, setData] = useState({
    timezone: fallbackTimezone,
    summaries: [],
    stats: {},
    tokenMetrics: {},
    spendMetrics: {},
    errors: [],
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    loadInsights({
      base: effectiveConfig.apiBase || '',
      timezone: fallbackTimezone,
    }).then((next) => {
      if (!active) return
      setData(next)
      setLoading(false)
    })
    return () => {
      active = false
    }
  }, [effectiveConfig.apiBase, fallbackTimezone])

  const summaries = useMemo(() => normalizeItems(data.summaries), [data.summaries])
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])
  const trend = useMemo(() => buildTrendSeries(summaries), [summaries])
  const projects = useMemo(() => topItems(rangeStats?.projects, 6), [rangeStats])
  const languages = useMemo(() => topItems(rangeStats?.languages, 6), [rangeStats])
  const aiTotal = Number(rangeStats?.aiAdditions) + Number(rangeStats?.aiDeletions)
  const humanTotal = Number(rangeStats?.humanAdditions) + Number(rangeStats?.humanDeletions)
  const changeTotal = Math.max(1, aiTotal + humanTotal)
  const aiPercent = (aiTotal / changeTotal) * 100
  const bestDay = rangeStats?.bestDay
  const maxDay = Math.max(1, ...trend.map((day) => Number(day.totalSeconds) || 0))

  return (
    <>
      {data.errors.length > 0 && (
        <div className="mb-6 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          {data.errors.map((error) => (
            <p key={error}>{error}</p>
          ))}
        </div>
      )}

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
        <Kpi
          label="Focus Score"
          value={
            loading ? '-' : `${Math.min(100, Math.round((rangeStats?.activeDays || 0) * 14))} / 100`
          }
          note={`${rangeStats?.activeDays || 0} active days in this range`}
          icon={Search}
        />
        <Kpi
          label="Active Time"
          value={rangeStats?.humanReadableTotal || '-'}
          note={`Average ${rangeStats?.humanReadableDailyAvg || '0s'} / active day`}
          icon={Activity}
        />
        <Kpi
          label="AI Share"
          value={formatPercent(aiPercent)}
          note={`${formatCount(aiTotal)} AI changes / ${formatCount(humanTotal)} human`}
          icon={Zap}
        />
        <Kpi
          label="AI Tokens"
          value={formatCount(data.tokenMetrics?.total_tokens || 0)}
          note={`${formatCount(data.tokenMetrics?.input_tokens || 0)} in`}
          icon={Zap}
        />
        <Kpi
          label="Best Day"
          value={bestDay?.text || '-'}
          note={bestDay?.date || 'No peak day yet'}
          icon={GitBranch}
        />
      </div>

      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-12 space-y-6 lg:col-span-8">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>ACTIVITY_HEATMAP // 7D</SectionTitle>
            <div className="flex h-52 items-end gap-2 md:gap-3">
              {trend.map((day) => {
                const pct = Math.max(4, ((Number(day.totalSeconds) || 0) / maxDay) * 100)
                return (
                  <div
                    key={day.date || day.label}
                    className="flex min-w-0 flex-1 flex-col items-center gap-2"
                  >
                    <span className="font-mono text-[10px] text-zinc-600 tabular-nums">
                      {formatShortDuration(day.totalSeconds)}
                    </span>
                    <div className="relative h-36 w-full overflow-hidden bg-zinc-900/60">
                      <div
                        className="absolute inset-x-0 bottom-0 bg-sky-300/60"
                        style={{ height: `${pct}%` }}
                      />
                    </div>
                    <span className="truncate text-[10px] text-zinc-500 uppercase">
                      {day.label}
                    </span>
                  </div>
                )
              })}
            </div>
          </section>

          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>PROJECT_LEVERAGE</SectionTitle>
            <div className="space-y-4">
              {projects.map((project) => (
                <div key={project.name}>
                  <div className="mb-1 flex justify-between gap-4 text-[11px]">
                    <span className="truncate tracking-[0.18em] text-zinc-300 uppercase">
                      {project.name}
                    </span>
                    <span className="font-mono text-sky-300 tabular-nums">
                      {formatCount(project.ai_changes)} AI / {formatCount(project.human_changes)}{' '}
                      human
                    </span>
                  </div>
                  <ProgressLine value={project.ai_percent} />
                </div>
              ))}
            </div>
          </section>
        </div>

        <aside className="col-span-12 space-y-6 lg:col-span-4">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>LANGUAGE_INDEX</SectionTitle>
            <div className="space-y-3 text-[11px]">
              {languages.map((language) => (
                <div key={language.name}>
                  <div className="mb-1 flex justify-between gap-3">
                    <span className="truncate text-zinc-300">{language.name}</span>
                    <span className="font-mono text-zinc-500 tabular-nums">{language.text}</span>
                  </div>
                  <ProgressLine value={language.percent} />
                </div>
              ))}
            </div>
          </section>
        </aside>
      </div>
    </>
  )
}

export default function Insights(props) {
  return (
    <ThemeProvider>
      <InsightsContent {...props} />
    </ThemeProvider>
  )
}
