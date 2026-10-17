import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import {
  buildTrendSeries,
  computeRangeStats,
  formatCount,
  formatShortDuration,
  formatSpend,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-800/60 bg-zinc-950'
const LABEL = 'text-[10px] tracking-[0.22em] text-zinc-500 uppercase font-mono'

const dayLabel = (dateStr) => {
  if (!dateStr) return '???'
  return ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'][new Date(`${dateStr}T00:00:00`).getDay()]
}

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
  const total = Math.max(
    1,
    Object.values(buckets).reduce((s, v) => s + v, 0)
  )
  return Object.entries(buckets)
    .map(([name, sec]) => ({ name, pct: Math.round((sec / total) * 100) }))
    .filter((d) => d.pct > 0)
    .sort((a, b) => b.pct - a.pct)
}

// Longest run of consecutive days that have tracked time.
const longestActiveRun = (summaries) => {
  let longest = 0
  let run = 0
  normalizeItems(summaries).forEach((day) => {
    if ((Number(day.grand_total?.total_seconds) || 0) > 0) {
      run += 1
      longest = Math.max(longest, run)
    } else {
      run = 0
    }
  })
  return longest
}

const rangeDescription = (summaries) => {
  const days = normalizeItems(summaries)
  const first = days[0]?.range?.date
  const last = days[days.length - 1]?.range?.date
  return first && last ? `${first} → ${last}` : 'no data'
}

// Every item below is derived from the fetched range; nothing is invented.
const buildAnomalies = (rangeStats, spendMetrics, tokenMetrics, summaries) => {
  const items = []
  const aiTotal = (rangeStats?.aiAdditions || 0) + (rangeStats?.aiDeletions || 0)
  const humanTotal = (rangeStats?.humanAdditions || 0) + (rangeStats?.humanDeletions || 0)
  const spendCents = Number(spendMetrics?.estimated_cents) || 0
  const totalTokens = Number(tokenMetrics?.total_tokens) || 0
  const window = rangeDescription(summaries)

  if (humanTotal > 0 && aiTotal / humanTotal > 5) {
    const proj = rangeStats?.projects?.[0]?.name || 'primary project'
    items.push({
      tag: 'LEVERAGE',
      title: `AI wrote ${(aiTotal / humanTotal).toFixed(1)}x more lines than you (top project: ${proj})`,
      body: `${formatCount(aiTotal)} AI lines vs ${formatCount(humanTotal)} human lines in ${window}.`,
      cls: 'text-zinc-200',
    })
  } else if (humanTotal === 0 && aiTotal > 0) {
    items.push({
      tag: 'LEVERAGE',
      title: 'All tracked line changes came from AI',
      body: `${formatCount(aiTotal)} AI lines and no human line changes in ${window}.`,
      cls: 'text-zinc-200',
    })
  }
  if (spendCents > 0) {
    items.push({
      tag: 'SPEND',
      title: `AI spend: ${formatSpend(spendCents)} for ${window}`,
      body: `${formatCount(totalTokens)} tokens, estimated from the model pricing table.`,
      cls: 'text-amber-400/80',
    })
  }
  const run = longestActiveRun(summaries)
  if (run >= 5) {
    items.push({
      tag: 'STREAK',
      title: `${run} consecutive active days`,
      body: `Longest unbroken run inside ${window}.${rangeStats?.bestDay?.date ? ` Peak on ${rangeStats.bestDay.date}.` : ''}`,
      cls: 'text-zinc-500',
    })
  }
  return items.slice(0, 4)
}

const loadInsights = async ({ base, timezone }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/insights',
    params: { range: 'Last Week', timezone },
  })
  if (error || !data)
    return {
      timezone,
      summaries: [],
      stats: {},
      tokenMetrics: {},
      spendMetrics: {},
      errors: [error || 'Failed to load insights'],
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
  const tz = effectiveConfig.timezone || detectTimezone()

  const [raw, setRaw] = useState({
    timezone: tz,
    summaries: [],
    stats: {},
    tokenMetrics: {},
    spendMetrics: {},
    errors: [],
  })
  const [loading, setLoading] = useState(true)
  const [refreshTime, setRefreshTime] = useState('')

  useEffect(() => {
    let active = true
    loadInsights({ base: effectiveConfig.apiBase || '', timezone: tz }).then((next) => {
      if (!active) return
      setRaw(next)
      setLoading(false)
      setRefreshTime(
        new Date().toLocaleTimeString('en-GB', { timeZone: 'UTC', hour12: false }) + ' GMT'
      )
    })
    return () => {
      active = false
    }
  }, [effectiveConfig.apiBase, tz])

  const summaries = useMemo(() => normalizeItems(raw.summaries), [raw.summaries])
  const rs = useMemo(() => computeRangeStats(summaries), [summaries])
  const trend = useMemo(() => buildTrendSeries(summaries), [summaries])

  const aiTotal = (rs?.aiAdditions || 0) + (rs?.aiDeletions || 0)
  const humanTotal = (rs?.humanAdditions || 0) + (rs?.humanDeletions || 0)
  const aiMult = humanTotal > 0 ? `${(aiTotal / humanTotal).toFixed(1)}x` : '—'
  const activeDays = rs?.activeDays || 0
  const totalDays = rs?.totalDays || 0
  const projectCount = rs?.projects?.length || 0

  const maxDaySeconds = useMemo(
    () => Math.max(1, ...trend.map((day) => Number(day.totalSeconds) || 0)),
    [trend]
  )

  const workDist = useMemo(() => buildWorkDist(rs?.categories || []), [rs])
  const anomalies = useMemo(
    () => buildAnomalies(rs, raw.spendMetrics, raw.tokenMetrics, summaries),
    [rs, raw.spendMetrics, raw.tokenMetrics, summaries]
  )

  const bestDay = rs?.bestDay?.text || '—'
  const idleRatio = totalDays > 0 ? Math.round(((totalDays - activeDays) / totalDays) * 100) : 0
  const multiProjectDays = summaries.filter((d) => normalizeItems(d.projects).length > 1).length

  const topEditor = topItems(rs?.editors, 1)[0]
  const topMachine = topItems(rs?.machines, 1)[0]

  const kpis = [
    {
      label: 'ACTIVE DAYS',
      value: `${activeDays} / ${totalDays}`,
      note: 'Days with tracked time',
    },
    {
      label: 'PROJECTS',
      value: projectCount,
      note: 'Distinct projects worked on',
    },
    { label: 'AI MULTIPLIER', value: aiMult, note: 'AI lines / human lines' },
    {
      label: 'AVG ACTIVE DAY',
      value: formatShortDuration(rs?.dailyAvgSeconds || 0),
      note: `Across ${activeDays} active days`,
    },
  ]

  const envelopeRows = [
    { label: 'BEST DAY', value: bestDay },
    { label: 'IDLE RATIO', value: `${idleRatio}%` },
    { label: 'MULTI-PROJECT DAYS', value: multiProjectDays },
  ]

  const signalCount = aiTotal + humanTotal

  useEffect(() => {
    const el = document.getElementById('shell-meta')
    if (!el) return
    el.innerHTML = `<div>Signals: <span class="text-zinc-300">${formatCount(signalCount)} events</span></div><div>Anomalies: <span class="text-zinc-300">${anomalies.length} detected</span></div>`
  }, [signalCount, anomalies.length])

  return (
    <div className={`font-mono transition-opacity duration-200 ${loading ? 'opacity-40' : ''}`}>
      {raw.errors.length > 0 && (
        <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-xs text-amber-200">
          <AlertTriangle size={14} className="mt-0.5 shrink-0 text-amber-400" />
          <div>
            {raw.errors.map((e) => (
              <p key={e}>{e}</p>
            ))}
          </div>
        </div>
      )}

      {/* KPI row */}
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        {kpis.map((kpi) => (
          <div key={kpi.label} className={`${PANEL} p-4`}>
            <div className={`${LABEL} mb-3`}>{kpi.label}</div>
            <div className="text-3xl leading-none font-medium text-zinc-100">
              {loading ? '—' : kpi.value}
            </div>
            <div className="mt-3 text-[11px] text-zinc-600">{kpi.note}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-12 gap-4">
        {/* Left */}
        <div className="col-span-12 space-y-4 lg:col-span-8">
          {/* Daily focus */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ DAILY_FOCUS // {rangeDescription(summaries)}</div>
            {trend.length === 0 ? (
              <div className="text-xs text-zinc-700">No activity in this range.</div>
            ) : (
              <div className="space-y-2">
                {trend.map((day) => {
                  const seconds = Number(day.totalSeconds) || 0
                  return (
                    <div key={day.date} className="flex items-center gap-3 text-xs">
                      <span className="w-9 text-[10px] text-zinc-500 uppercase">
                        {dayLabel(day.date)}
                      </span>
                      <div className="h-3 flex-1 bg-zinc-900">
                        <div
                          className="h-full bg-zinc-400"
                          style={{ width: `${(seconds / maxDaySeconds) * 100}%` }}
                        />
                      </div>
                      <span className="w-16 text-right text-zinc-400 tabular-nums">
                        {formatShortDuration(seconds)}
                      </span>
                    </div>
                  )
                })}
              </div>
            )}
          </section>

          {/* Work Distribution */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ WORK_DISTRIBUTION // CATEGORY GROUPS</div>
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
            {anomalies.length === 0 ? (
              <div className="text-xs text-zinc-700">Nothing unusual in this range.</div>
            ) : (
              <div className="space-y-2.5">
                {anomalies.map((item) => (
                  <div key={item.tag} className="border border-zinc-800/50 p-3">
                    <div className={`mb-1 text-[10px] tracking-widest ${item.cls}`}>
                      [{item.tag}]
                    </div>
                    <div className="mb-1 text-xs font-semibold text-zinc-200">{item.title}</div>
                    <div className="text-[11px] leading-relaxed text-zinc-600">{item.body}</div>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Session Envelope */}
          <section className={`${PANEL} p-5`}>
            <div className={`${LABEL} mb-4`}>▪ RANGE_ENVELOPE</div>
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
          <span className="mr-1.5 inline-block h-1.5 w-1.5 bg-green-500" />
          NODE STATUS: <span className="text-zinc-400">SYNCHRONIZED</span>
        </span>
        {topMachine && (
          <span>
            MACHINE: <span className="text-zinc-400">{topMachine.name}</span>
          </span>
        )}
        {topEditor && (
          <span>
            EDITOR:{' '}
            <span className="text-zinc-400">
              {topEditor.name} ({Math.round(topEditor.percent)}%)
            </span>
          </span>
        )}
        {refreshTime && (
          <span className="ml-auto">
            LAST_REFRESH: <span className="text-zinc-400">{refreshTime}</span>
          </span>
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
