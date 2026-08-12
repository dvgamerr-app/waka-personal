import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import ActivityHeatmap from './ActivityHeatmap'
import {
  computeRangeStats,
  formatCount,
  formatPercent,
  formatSpend,
  formatShortDuration,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'ring-1 ring-zinc-900 bg-zinc-950/80 p-6 rounded-none'

const SectionTitle = ({ children }) => (
  <h3 className="mb-6 flex items-center gap-2 text-sm font-medium text-zinc-400">
    <div className="size-1 bg-zinc-600" />
    {children}
  </h3>
)

const formatTokens = (n) => {
  const num = Number(n) || 0
  if (num >= 1e9) return `${(num / 1e9).toFixed(1)}B`
  if (num >= 1e6) return `${(num / 1e6).toFixed(1)}M`
  if (num >= 1e3) return `${(num / 1e3).toFixed(1)}K`
  return num > 0 ? String(num) : '—'
}

const fmtDate = (d) => {
  if (!d) return '—'
  return new Date(`${d}T00:00:00`).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

const fmtDateLong = (d) => {
  if (!d) return '—'
  return new Date(`${d}T00:00:00`).toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  })
}

const LANG_BAR_CLASSES = [
  'bg-sky-300 text-black',
  'bg-sky-300/60 text-black',
  'bg-sky-300/40 text-zinc-800',
  'bg-sky-300/25 text-zinc-500',
  'bg-zinc-800 text-zinc-500',
  'bg-zinc-800 text-zinc-500',
]

const loadWrapped = async ({ base, timezone, year }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/wrapped',
    params: { year, timezone },
  })
  if (error || !data) {
    return {
      timezone,
      year,
      stats: {},
      summaries: [],
      tokenMetrics: {},
      spendMetrics: {},
      activity: {},
      totalDays: 0,
      errors: [error || 'Failed to load wrapped'],
    }
  }
  return {
    timezone: data.timezone || timezone,
    year: data.year || year,
    stats: data.stats || {},
    summaries: data.summaries || [],
    tokenMetrics: data.token_metrics || {},
    spendMetrics: data.spend_metrics || {},
    activity: data.activity || {},
    totalDays: data.total_days || 0,
    generatedAt: data.generated_at || '',
    errors: data.errors || [],
  }
}

function WrappedContent({ config = {}, year }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const fallbackTimezone = effectiveConfig.timezone || detectTimezone()
  const currentYear = new Date().getFullYear()

  const getYearFromUrl = () => {
    if (typeof window === 'undefined') return Number(year) || currentYear
    const param = new URLSearchParams(window.location.search).get('year')
    return Number(param) || Number(year) || currentYear
  }

  const [selectedYear, setSelectedYear] = useState(getYearFromUrl)
  const [data, setData] = useState({
    timezone: fallbackTimezone,
    year: selectedYear,
    stats: {},
    summaries: [],
    tokenMetrics: {},
    spendMetrics: {},
    activity: {},
    totalDays: 0,
    errors: [],
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const handler = (e) => setSelectedYear(e.detail)
    window.addEventListener('year-change', handler)
    return () => window.removeEventListener('year-change', handler)
  }, [])

  useEffect(() => {
    let active = true
    setLoading(true)
    loadWrapped({
      base: effectiveConfig.apiBase || '',
      timezone: fallbackTimezone,
      year: selectedYear,
    }).then((next) => {
      if (!active) return
      setData(next)
      setLoading(false)
      const cmdEl = document.getElementById('shell-command')
      if (cmdEl) cmdEl.textContent = `wrapped --year=${selectedYear}`
      const subtitleEl = document.getElementById('shell-subtitle')
      if (subtitleEl)
        subtitleEl.textContent = `Annual Telemetry Compilation · ${selectedYear} Year-in-Code Report`
      const metaEl = document.getElementById('shell-meta')
      if (metaEl) {
        const compiledDate =
          selectedYear < currentYear
            ? `${selectedYear}-12-31 23:59:59`
            : new Date().toISOString().slice(0, 19).replace('T', ' ')
        metaEl.innerHTML = `<div>Compiled: ${compiledDate}</div><div class="text-zinc-500">Data points: <span class="text-sky-300">${next.totalDays.toLocaleString()}</span></div>`
      }
    })
    return () => {
      active = false
    }
  }, [effectiveConfig.apiBase, fallbackTimezone, selectedYear])

  const summaries = useMemo(() => normalizeItems(data.summaries), [data.summaries])
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])
  const projects = useMemo(() => topItems(rangeStats?.projects, 6), [rangeStats])
  const languages = useMemo(() => topItems(rangeStats?.languages, 8), [rangeStats])

  const months = useMemo(() => {
    const buckets = new Map()
    summaries.forEach((day) => {
      const key = (day.range?.date || '').slice(5, 7)
      if (!key) return
      buckets.set(key, (buckets.get(key) || 0) + (Number(day.grand_total?.total_seconds) || 0))
    })
    return Array.from({ length: 12 }, (_, i) => {
      const month = String(i + 1).padStart(2, '0')
      return {
        label: new Date(`${data.year}-${month}-01T00:00:00`).toLocaleDateString('en-US', {
          month: 'short',
        }),
        seconds: buckets.get(month) || 0,
      }
    })
  }, [summaries, data.year])

  const maxMonth = useMemo(() => Math.max(1, ...months.map((m) => m.seconds)), [months])
  const peakMonthIdx = useMemo(
    () => months.reduce((best, m, i) => (m.seconds > months[best].seconds ? i : best), 0),
    [months]
  )

  const aiTotal =
    (Number(rangeStats?.aiAdditions) || Number(data.stats?.ai_additions) || 0) +
    (Number(rangeStats?.aiDeletions) || Number(data.stats?.ai_deletions) || 0)
  const humanTotal =
    (Number(rangeStats?.humanAdditions) || Number(data.stats?.human_additions) || 0) +
    (Number(rangeStats?.humanDeletions) || Number(data.stats?.human_deletions) || 0)
  const aiMultiplier = humanTotal > 0 ? (aiTotal / humanTotal).toFixed(1) : null
  const totalHours = Math.round((rangeStats?.totalSeconds || 0) / 3600)
  const linesPerDay = rangeStats?.activeDays > 0 ? Math.round(aiTotal / rangeStats.activeDays) : 0
  const machineCount = rangeStats?.machines?.length || 0

  const activity = data.activity || {}
  const tokenUsage = activity.token_usage || {}
  const totalInputTokens = Number(tokenUsage.input_tokens) || 0
  const totalOutputTokens = Number(tokenUsage.output_tokens) || 0
  const totalTokens = totalInputTokens + totalOutputTokens
  const spendCents = data.spendMetrics?.estimated_cents || 0

  const longestStreak = {
    days: Number(activity.longest_streak_days) || 0,
    start: activity.longest_streak_start || null,
    end: activity.longest_streak_end || null,
  }

  // First active day
  const firstActiveDay = useMemo(() => {
    const active = summaries
      .filter((d) => (Number(d.grand_total?.total_seconds) || 0) > 0)
      .sort((a, b) => (a.range?.date || '').localeCompare(b.range?.date || ''))
    return active[0]?.range?.date || null
  }, [summaries])

  // Language bar (top 5 + other)
  const langBar = useMemo(() => {
    const top = languages.slice(0, 5)
    const usedPct = top.reduce((sum, l) => sum + l.percent, 0)
    const other = 100 - usedPct
    return [...top, ...(other > 0.5 ? [{ name: 'Other', percent: other }] : [])]
  }, [languages])

  // AI model leaderboard from rangeStats
  const aiModelLeaderboard = useMemo(() => {
    const models = normalizeItems(rangeStats?.aiModels)
    if (!models.length) return []
    const totalLines = Math.max(
      1,
      models.reduce((s, m) => s + m.lines, 0)
    )
    return models.slice(0, 4).map((m) => ({
      name: m.name,
      lines: m.lines,
      pct: Math.round((m.lines / totalLines) * 100),
    }))
  }, [rangeStats])

  // Superlatives
  const topProject = projects[0]
  const topLanguage = languages[0]
  const superlatives = useMemo(
    () => [
      {
        key: '[MOST_PROLIFIC_DAY]',
        value: activity.peak_day?.date ? fmtDateLong(activity.peak_day.date) : '—',
        note: activity.peak_day?.total_seconds
          ? formatShortDuration(activity.peak_day.total_seconds)
          : 'No data',
      },
      {
        key: '[LONGEST_STREAK]',
        value: longestStreak.days > 0 ? `${longestStreak.days} days` : '—',
        note: longestStreak.start
          ? `${fmtDate(longestStreak.start)} → ${fmtDate(longestStreak.end)}`
          : 'No streak data',
      },
      {
        key: '[TOP_PROJECT]',
        value: topProject?.name || '—',
        note: topProject
          ? `${topProject.text} · ${formatCount(topProject.ai_additions + topProject.ai_deletions)} AI lines`
          : 'No project data',
      },
      {
        key: '[FAVORITE_LANGUAGE]',
        value: topLanguage?.name || '—',
        note: topLanguage
          ? `${formatPercent(topLanguage.percent)} of focus time`
          : 'No language data',
      },
      {
        key: '[PEAK_MONTH]',
        value: months[peakMonthIdx]?.seconds > 0 ? months[peakMonthIdx]?.label || '—' : '—',
        note:
          months[peakMonthIdx]?.seconds > 0
            ? `${Math.round(months[peakMonthIdx].seconds / 3600)}h coded`
            : 'No data',
      },
      {
        key: '[FIRST_ACTIVE_DAY]',
        value: firstActiveDay ? fmtDate(firstActiveDay) : '—',
        note: firstActiveDay ? `First session of ${selectedYear}` : 'No data',
      },
    ],
    [
      activity,
      longestStreak,
      topProject,
      topLanguage,
      months,
      peakMonthIdx,
      firstActiveDay,
      selectedYear,
    ]
  )

  // Milestones
  const milestones = useMemo(
    () => [
      {
        label: '1M Lines Club',
        unlocked: aiTotal >= 1_000_000,
        date: aiTotal >= 1_000_000 ? fmtDate(activity.peak_day?.date) || '✓' : '—',
      },
      {
        label: `${longestStreak.days || 0}-Day Streak`,
        unlocked: longestStreak.days >= 7,
        date: longestStreak.start ? fmtDate(longestStreak.start) : '—',
      },
      {
        label: `Polyglot · ${languages.length} langs`,
        unlocked: languages.length >= 3,
        date: languages.length >= 3 ? '✓' : '—',
      },
      {
        label: `Token Tycoon · ${formatTokens(totalTokens)}`,
        unlocked: totalTokens >= 1_000_000,
        date: totalTokens >= 1_000_000 ? '✓' : '—',
      },
      {
        label: 'Marathon · 2000h',
        unlocked: totalHours >= 2000,
        date: totalHours >= 2000 ? `${totalHours}h` : '—',
      },
      {
        label: 'Zero Bug Week',
        unlocked: false,
        date: '—',
      },
    ],
    [aiTotal, longestStreak, languages, totalTokens, totalHours, activity]
  )

  return (
    <>
      {/* Hero banner */}
      <section className="relative mb-6 overflow-hidden border border-sky-300/20 bg-gradient-to-br from-sky-300/5 to-transparent p-8">
        <div className="pointer-events-none absolute inset-0 overflow-hidden font-mono text-[9px] leading-3 break-all whitespace-pre-wrap text-sky-300 opacity-[0.03] select-none">
          {'01001000 01100101 01101100 01101100 01101111 '.repeat(600)}
        </div>
        <div className="relative">
          <div className="mb-2 text-[10px] tracking-[0.3em] text-sky-300 uppercase">
            // SYNTAX_OPS · WRAPPED · {selectedYear}
          </div>
          <h2 className="mb-2 text-4xl font-medium tracking-tight text-zinc-100 md:text-5xl">
            {aiTotal > 0 ? (
              <>
                You shipped <span className="text-sky-300">{formatCount(aiTotal)}</span> lines this
                year.
              </>
            ) : (
              <>
                You coded <span className="text-sky-300">{totalHours.toLocaleString()}h</span> this
                year.
              </>
            )}
          </h2>
          <p className="max-w-[60ch] text-sm text-zinc-400">
            {aiTotal > 0 ? (
              <>
                That's roughly{' '}
                <span className="text-zinc-200">{linesPerDay.toLocaleString()} lines per day</span>,
                sustained across <span className="text-zinc-200">{projects.length} projects</span>
                {machineCount > 0 && (
                  <>
                    {' '}
                    and <span className="text-zinc-200">{machineCount} agent stations</span>
                  </>
                )}
                .
                {aiMultiplier && (
                  <>
                    {' '}
                    Your AI multiplier landed at{' '}
                    <span className="text-sky-300">{aiMultiplier}x</span>.
                  </>
                )}
              </>
            ) : (
              <>
                Across{' '}
                <span className="text-zinc-200">{rangeStats?.activeDays || 0} active days</span> and{' '}
                <span className="text-zinc-200">{projects.length} projects</span>
                {machineCount > 0 && (
                  <>
                    {' '}
                    on <span className="text-zinc-200">{machineCount} machines</span>
                  </>
                )}
                . AI line tracking was not available for {selectedYear}.
              </>
            )}
          </p>
        </div>
      </section>

      {data.errors.length > 0 && (
        <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-400" />
          <div>
            {data.errors.map((e) => (
              <p key={e}>{e}</p>
            ))}
          </div>
        </div>
      )}

      <div className={`transition-opacity duration-200 ${loading ? 'opacity-50' : ''}`}>
        {/* Stat cards */}
        <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
          {[
            [
              'TOTAL_HOURS',
              totalHours > 0 ? `${totalHours.toLocaleString()}h` : '—',
              `≈ ${Math.round(totalHours / 24)} days at the keyboard`,
            ],
            [
              'AI_LINES',
              formatCount(aiTotal),
              humanTotal > 0
                ? `vs ${formatCount(humanTotal)} human lines`
                : `${rangeStats?.activeDays || 0} active days`,
            ],
            [
              'TOKENS_BURNED',
              formatTokens(totalTokens),
              totalTokens > 0
                ? `${machineCount > 0 ? `across ${machineCount} agent stations` : 'total tokens'}`
                : 'not tracked in heartbeats',
            ],
            [
              'TOTAL_SPEND',
              totalTokens > 0 ? formatSpend(spendCents) : '—',
              totalTokens > 0
                ? `${formatTokens(totalInputTokens)} in · ${formatTokens(totalOutputTokens)} out`
                : 'not tracked in heartbeats',
            ],
          ].map(([label, value, note]) => (
            <section key={label} className="bg-zinc-950/80 p-4 ring-1 ring-zinc-900">
              <span className="text-[10px] tracking-wider text-zinc-500 uppercase">{label}</span>
              <div className="mt-1 text-3xl leading-none font-medium text-zinc-100">{value}</div>
              <div className="mt-2 text-[11px] text-zinc-600">{note}</div>
            </section>
          ))}
        </div>

        <ActivityHeatmap data={activity} loading={loading} />

        {/* Bottom grid */}
        <div className="grid grid-cols-12 gap-6">
          {/* Left column */}
          <div className="col-span-12 space-y-6 lg:col-span-8">
            {/* Monthly trace */}
            <section className={PANEL}>
              <SectionTitle>MONTHLY_TRACE // {formatCount(totalHours)}H</SectionTitle>
              <div className="flex h-48 items-stretch gap-2">
                {months.map((month, i) => {
                  const pct = month.seconds > 0 ? Math.max(4, (month.seconds / maxMonth) * 100) : 0
                  const isPeak = i === peakMonthIdx && month.seconds > 0
                  return (
                    <div
                      key={month.label}
                      className="group flex min-w-0 flex-1 flex-col items-center gap-2"
                    >
                      <span
                        className={`text-[10px] tabular-nums ${isPeak ? 'text-sky-300' : 'text-zinc-600'}`}
                      >
                        {Math.round(month.seconds / 3600)}h
                      </span>
                      <div className="relative w-full flex-1 overflow-hidden rounded-sm bg-zinc-900/40">
                        <div
                          className={`absolute right-0 bottom-0 left-0 transition-all ${isPeak ? 'bg-sky-300' : 'bg-sky-300/30'}`}
                          style={{ height: `${pct}%` }}
                        />
                      </div>
                      <span className="text-[10px] text-zinc-500 uppercase">{month.label}</span>
                    </div>
                  )
                })}
              </div>
            </section>

            {/* Superlatives */}
            <section className={PANEL}>
              <SectionTitle>SUPERLATIVES</SectionTitle>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                {superlatives.map(({ key, value, note }) => (
                  <div
                    key={key}
                    className="rounded-none bg-zinc-900/20 p-3 ring-1 ring-zinc-900 transition-colors hover:ring-sky-300/30"
                  >
                    <div className="mb-1 text-[10px] tracking-widest text-sky-300 uppercase">
                      {key}
                    </div>
                    <div className="text-lg font-medium text-zinc-100">{value}</div>
                    <div className="mt-0.5 text-[10px] text-zinc-500">{note}</div>
                  </div>
                ))}
              </div>
            </section>

            {/* Language distribution */}
            <section className={PANEL}>
              <SectionTitle>LANGUAGE_DISTRIBUTION</SectionTitle>
              <div className="mb-3 flex h-8 overflow-hidden rounded-sm ring-1 ring-zinc-800">
                {langBar.map((lang, i) => (
                  <div
                    key={lang.name}
                    className={`flex h-full items-center justify-center text-[10px] uppercase ${LANG_BAR_CLASSES[i] || 'bg-zinc-800 text-zinc-500'}`}
                    style={{ width: `${lang.percent}%` }}
                  >
                    {lang.percent >= 8 ? lang.name : ''}
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-3 gap-2 text-[10px] text-zinc-500 uppercase md:grid-cols-6">
                {langBar.map((lang) => (
                  <div key={lang.name} className="flex justify-between">
                    <span className="truncate">{lang.name}</span>
                    <span className="text-zinc-300 tabular-nums">{Math.round(lang.percent)}%</span>
                  </div>
                ))}
              </div>
            </section>
          </div>

          {/* Right column */}
          <div className="col-span-12 space-y-6 lg:col-span-4">
            {/* Agent leaderboard */}
            <section className={PANEL}>
              <SectionTitle>AGENT_LEADERBOARD</SectionTitle>
              {aiModelLeaderboard.length === 0 ? (
                <div className="border border-dashed border-zinc-800 p-6 text-xs text-zinc-600">
                  No AI model data for {selectedYear}.
                </div>
              ) : (
                <div className="space-y-3">
                  {aiModelLeaderboard.map((model, i) => (
                    <div key={model.name}>
                      <div className="mb-1 flex justify-between text-[11px]">
                        <span className="text-zinc-300">
                          <span className="mr-2 text-zinc-600 tabular-nums">#{i + 1}</span>
                          {model.name}
                        </span>
                        <span className="text-sky-300 tabular-nums">{model.pct}%</span>
                      </div>
                      <div className="h-1.5 overflow-hidden bg-zinc-900">
                        <div className="h-full bg-sky-300" style={{ width: `${model.pct}%` }} />
                      </div>
                      <div className="mt-0.5 text-[10px] text-zinc-600 tabular-nums">
                        {formatCount(model.lines)} lines
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>

            {/* Milestones */}
            <section className={PANEL}>
              <SectionTitle>MILESTONES_UNLOCKED</SectionTitle>
              <div className="space-y-2 text-[11px]">
                {milestones.map(({ label, unlocked, date }) => (
                  <div
                    key={label}
                    className="flex items-center justify-between border-b border-zinc-900/60 pb-1.5"
                  >
                    <div className="flex items-center gap-2">
                      <span
                        className={`size-1.5 rounded-full ${unlocked ? 'bg-sky-300' : 'bg-zinc-700'}`}
                      />
                      <span className={unlocked ? 'text-zinc-200' : 'text-zinc-600 line-through'}>
                        {label}
                      </span>
                    </div>
                    <span className="text-[10px] text-zinc-500 tabular-nums">{date}</span>
                  </div>
                ))}
              </div>
            </section>

            {/* Share */}
            <section className={PANEL}>
              <div className="py-2 text-center">
                <div className="mb-4 text-[10px] tracking-widest text-zinc-500 uppercase">
                  Share your wrapped
                </div>
                <div className="flex justify-center gap-2">
                  <button
                    type="button"
                    className="bg-sky-300/10 px-3 py-1.5 text-[10px] tracking-widest text-sky-300 uppercase ring-1 ring-sky-300 transition-colors hover:bg-sky-300/20"
                  >
                    Export PNG
                  </button>
                  <button
                    type="button"
                    className="px-3 py-1.5 text-[10px] tracking-widest text-zinc-400 uppercase ring-1 ring-zinc-800 transition-colors hover:ring-zinc-700"
                  >
                    Copy Link
                  </button>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </>
  )
}

export default function Wrapped(props) {
  return (
    <ThemeProvider>
      <WrappedContent {...props} />
    </ThemeProvider>
  )
}
