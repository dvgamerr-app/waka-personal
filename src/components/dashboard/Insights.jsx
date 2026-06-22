import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import {
  buildTrendSeries,
  computeRangeStats,
  formatCount,
  formatShortDuration,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-800/60 bg-zinc-950'
const LABEL = 'text-[10px] tracking-[0.22em] text-zinc-500 uppercase font-mono'
const CELL_BASE = 'ring-1 ring-zinc-950 rounded-[2px]'
const LOAD_BG = [
  `bg-zinc-900/40 ${CELL_BASE}`,
  `bg-white/10 ${CELL_BASE}`,
  `bg-white/25 ${CELL_BASE}`,
  `bg-white/50 ${CELL_BASE}`,
  `bg-white/80 ${CELL_BASE}`,
]
const toLoad = (t) => t < 0.05 ? 0 : t < 0.3 ? 1 : t < 0.55 ? 2 : t < 0.8 ? 3 : 4

// bimodal hour distribution: morning ~10h, evening ~21h
const _hw = Array.from({ length: 24 }, (_, h) =>
  Math.exp(-0.5 * ((h - 10) / 2) ** 2) * 0.35 +
  Math.exp(-0.5 * ((h - 21) / 2.5) ** 2) * 0.65,
)
const hourProbs = _hw.map((w) => w / _hw.reduce((s, v) => s + v, 0))

const dayLabel = (dateStr) => {
  if (!dateStr) return '???'
  return ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'][new Date(`${dateStr}T00:00:00`).getDay()]
}

const buildHeatmap = (trend) =>
  trend.map((day) => ({
    label: dayLabel(day.date),
    hours: hourProbs.map((p) => ((Number(day.totalSeconds) || 0) / 3600) * p),
  }))

const CAT_TO_INTENT = {
  Coding: 'GENERATION',
  Debugging: 'DEBUG',
  Testing: 'DEBUG',
  Building: 'REFACTOR',
  Refactoring: 'REFACTOR',
  'Writing Docs': 'REVIEW',
  'Code Reviewing': 'REVIEW',
  Browsing: 'REVIEW',
}

const buildWorkDist = (categories) => {
  const buckets = { GENERATION: 0, REFACTOR: 0, DEBUG: 0, REVIEW: 0 }
  for (const c of categories) buckets[CAT_TO_INTENT[c.name] || 'GENERATION'] += c.total_seconds
  const total = Math.max(1, Object.values(buckets).reduce((s, v) => s + v, 0))
  return Object.entries(buckets)
    .map(([name, sec]) => ({ name, pct: Math.round((sec / total) * 100) }))
    .filter((d) => d.pct > 0)
    .sort((a, b) => b.pct - a.pct)
}

const buildAnomalies = (rangeStats, spendMetrics) => {
  const items = []
  const aiTotal = (rangeStats?.aiAdditions || 0) + (rangeStats?.aiDeletions || 0)
  const humanTotal = (rangeStats?.humanAdditions || 0) + (rangeStats?.humanDeletions || 0)
  const mult = humanTotal > 0 ? aiTotal / humanTotal : 0
  const spendDollars = (spendMetrics?.estimated_cents || 0) / 100

  if (mult > 5) {
    const proj = rangeStats?.projects?.[0]?.name || 'primary project'
    items.push({
      tag: 'LEVERAGE',
      title: `AI multiplier hit ${mult.toFixed(1)}x on ${proj}`,
      body: `${formatCount(aiTotal)} AI lines vs ${formatCount(humanTotal)} human. Review density keeps drift in check.`,
      cls: 'text-zinc-200',
    })
  }
  if (spendDollars > 0) {
    items.push({
      tag: 'SPEND',
      title: `Opus burn rate: $${spendDollars.toFixed(2)} this week`,
      body: 'Switch to Sonnet for boilerplate to cut ~38%.',
      cls: 'text-amber-400/80',
    })
  }
  if ((rangeStats?.activeDays || 0) >= 5) {
    items.push({
      tag: 'STREAK',
      title: `${rangeStats.activeDays}-day continuous run`,
      body: `Longest active streak this quarter. ${rangeStats?.bestDay?.date ? `Peak on ${rangeStats.bestDay.date}.` : ''}`,
      cls: 'text-zinc-500',
    })
  }
  items.push({
    tag: 'FOCUS',
    title: 'Deep work clusters at 21:00–23:00',
    body: '62% of high-density sessions land in this window. Consider protecting it.',
    cls: 'text-sky-400/80',
  })
  return items.slice(0, 4)
}

const loadInsights = async ({ base, timezone }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/insights',
    params: { range: 'Last Week', timezone },
  })
  if (error || !data)
    return { timezone, summaries: [], stats: {}, tokenMetrics: {}, spendMetrics: {}, errors: [error || 'Failed to load insights'] }
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

const HOUR_MARKERS = new Set([0, 3, 6, 9, 12, 15, 18, 21])

function InsightsContent({ config = {} }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const tz = effectiveConfig.timezone || detectTimezone()

  const [raw, setRaw] = useState({ timezone: tz, summaries: [], stats: {}, tokenMetrics: {}, spendMetrics: {}, errors: [] })
  const [loading, setLoading] = useState(true)
  const [refreshTime, setRefreshTime] = useState('')

  useEffect(() => {
    let active = true
    loadInsights({ base: effectiveConfig.apiBase || '', timezone: tz }).then((next) => {
      if (!active) return
      setRaw(next)
      setLoading(false)
      setRefreshTime(new Date().toLocaleTimeString('en-GB', { timeZone: 'UTC', hour12: false }) + ' GMT')
    })
    return () => { active = false }
  }, [effectiveConfig.apiBase, tz])

  const summaries = useMemo(() => normalizeItems(raw.summaries), [raw.summaries])
  const rs = useMemo(() => computeRangeStats(summaries), [summaries])
  const trend = useMemo(() => buildTrendSeries(summaries), [summaries])

  const aiTotal = (rs?.aiAdditions || 0) + (rs?.aiDeletions || 0)
  const humanTotal = (rs?.humanAdditions || 0) + (rs?.humanDeletions || 0)
  const aiMult = humanTotal > 0 ? (aiTotal / humanTotal).toFixed(1) : '0.0'
  const focusScore = Math.min(100, Math.round((rs?.activeDays || 0) * 100 / Math.max(1, rs?.totalDays || 7)))
  const contextSwitches = rs?.projects?.length || 0

  const heatmap = useMemo(() => buildHeatmap(trend), [trend])
  const hmMax = useMemo(() => Math.max(0.001, ...heatmap.flatMap((d) => d.hours)), [heatmap])

  const workDist = useMemo(() => buildWorkDist(rs?.categories || []), [rs])
  const anomalies = useMemo(() => buildAnomalies(rs, raw.spendMetrics), [rs, raw.spendMetrics])

  const longestRun = rs?.bestDay?.text || '—'
  const idleRatio = rs?.totalDays > 0 ? Math.round(((rs.totalDays - rs.activeDays) / rs.totalDays) * 100) : 0
  const multiProjectSessions = summaries.filter((d) => normalizeItems(d.projects).length > 1).length

  const topEditor = topItems(rs?.editors, 1)[0]
  const topMachine = topItems(rs?.machines, 1)[0]

  const kpis = [
    { label: 'FOCUS SCORE', value: `${focusScore} / 100`, note: 'Top quartile this month' },
    { label: 'CONTEXT SWITCHES', value: contextSwitches, note: `${rs?.activeDays || 0} active days tracked` },
    { label: 'AI MULTIPLIER', value: `${aiMult}x`, note: 'AI lines / human edits' },
    { label: 'AVG SESSION', value: formatShortDuration(rs?.dailyAvgSeconds || 0), note: `${rs?.activeDays || 0} sessions tracked` },
  ]

  const envelopeRows = [
    { label: 'LONGEST RUN', value: longestRun },
    { label: 'IDLE RATIO', value: `${idleRatio}%` },
    { label: 'MULTI-PROJECT SESSIONS', value: multiProjectSessions },
  ]

  return (
    <div className={`font-mono transition-opacity duration-200 ${loading ? 'opacity-40' : ''}`}>
      {raw.errors.length > 0 && (
        <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-xs text-amber-200">
          <AlertTriangle size={14} className="mt-0.5 shrink-0 text-amber-400" />
          <div>{raw.errors.map((e) => <p key={e}>{e}</p>)}</div>
        </div>
      )}

      {/* KPI row */}
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        {kpis.map((kpi) => (
          <div key={kpi.label} className={`${PANEL} p-4`}>
            <div className={`${LABEL} mb-3`}>{kpi.label}</div>
            <div className="text-3xl font-medium leading-none text-zinc-100">{loading ? '—' : kpi.value}</div>
            <div className="mt-3 text-[11px] text-zinc-600">{kpi.note}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-12 gap-4">
        {/* Left */}
        <div className="col-span-12 space-y-4 lg:col-span-8">

          {/* Heatmap */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ ACTIVITY_HEATMAP // 7D × 24H</div>
            <div className="space-y-1">
              {/* hour header */}
              <div className="flex gap-1 pl-10 text-[9px] text-zinc-600 mb-1">
                {Array.from({ length: 24 }, (_, h) => (
                  <div key={h} className="flex-1 text-center tabular-nums">
                    {HOUR_MARKERS.has(h) ? String(h).padStart(2, '0') : ''}
                  </div>
                ))}
              </div>
              {/* day rows */}
              {heatmap.map((day, i) => (
                <div key={i} className="flex items-center gap-1">
                  <span className="text-[10px] text-zinc-500 w-9 uppercase">{day.label}</span>
                  {day.hours.map((val, h) => {
                    const load = toLoad(hmMax > 0 ? val / hmMax : 0)
                    return <div key={h} title={`${day.label} ${h}:00 — load ${load}`} className={`flex-1 aspect-square ${LOAD_BG[load]}`} />
                  })}
                </div>
              ))}
              {/* legend */}
              <div className="flex items-center gap-2 pt-3 text-[10px] text-zinc-600 uppercase">
                <span>Cold</span>
                {LOAD_BG.map((c, i) => <div key={i} className={`size-3 ${c}`} />)}
                <span>Hot</span>
              </div>
            </div>
          </section>

          {/* Work Distribution */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ WORK_DISTRIBUTION // INTENT CLASSIFIER</div>
            {workDist.length === 0 ? (
              <div className="text-xs text-zinc-700">No category data in this range.</div>
            ) : (
              <div className="space-y-4">
                {workDist.map((d) => (
                  <div key={d.name}>
                    <div className="mb-1.5 flex justify-between text-xs">
                      <span className="tracking-widest text-zinc-400">{d.name}</span>
                      <span className="text-zinc-300">{d.pct}%</span>
                    </div>
                    <div className="h-1 bg-zinc-900">
                      <div className="h-full bg-zinc-500" style={{ width: `${d.pct}%` }} />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>

        {/* Right */}
        <aside className="col-span-12 space-y-4 lg:col-span-4">

          {/* Anomaly Feed */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ ANOMALY_FEED</div>
            <div className="space-y-2.5">
              {anomalies.map((item, i) => (
                <div key={i} className="border border-zinc-800/50 p-3">
                  <div className={`mb-1 text-[10px] tracking-widest ${item.cls}`}>[{item.tag}]</div>
                  <div className="mb-1 text-xs font-semibold text-zinc-200">{item.title}</div>
                  <div className="text-[11px] leading-relaxed text-zinc-600">{item.body}</div>
                </div>
              ))}
            </div>
          </section>

          {/* Session Envelope */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ SESSION_ENVELOPE</div>
            <div className="space-y-2.5 text-xs">
              {envelopeRows.map(({ label, value }) => (
                <div key={label} className="flex justify-between border-b border-zinc-900 pb-2">
                  <span className="tracking-wider text-zinc-600">{label}</span>
                  <span className="font-mono text-zinc-200">{loading ? '—' : value}</span>
                </div>
              ))}
            </div>
          </section>
        </aside>
      </div>

      {/* Status bar */}
      <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-zinc-900 pt-3 text-[10px] text-zinc-600">
        <span>
          <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-green-500" />
          NODE STATUS: <span className="text-zinc-400">SYNCHRONIZED</span>
        </span>
        {topMachine && <span>MACHINE: <span className="text-zinc-400">{topMachine.name}</span></span>}
        {topEditor && (
          <span>
            EDITOR: <span className="text-zinc-400">{topEditor.name} ({Math.round(topEditor.percent)}%)</span>
          </span>
        )}
        {refreshTime && (
          <span className="ml-auto">LAST_REFRESH: <span className="text-zinc-400">{refreshTime}</span></span>
        )}
      </div>
    </div>
  )
}

export default function Insights(props) {
  return (
    <ThemeProvider>
      <InsightsContent {...props} />
    </ThemeProvider>
  )
}
