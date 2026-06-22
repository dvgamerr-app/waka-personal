import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
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
  <h3 className="text-sm font-medium text-zinc-400 mb-6 flex items-center gap-2">
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
        label: new Date(`${data.year}-${month}-01T00:00:00`).toLocaleDateString('en-US', { month: 'short' }),
        seconds: buckets.get(month) || 0,
      }
    })
  }, [summaries, data.year])

  const maxMonth = useMemo(() => Math.max(1, ...months.map((m) => m.seconds)), [months])
  const peakMonthIdx = useMemo(
    () => months.reduce((best, m, i) => (m.seconds > months[best].seconds ? i : best), 0),
    [months]
  )

  const aiTotal = (Number(rangeStats?.aiAdditions) || 0) + (Number(rangeStats?.aiDeletions) || 0)
  const humanTotal = (Number(rangeStats?.humanAdditions) || 0) + (Number(rangeStats?.humanDeletions) || 0)
  const aiMultiplier = humanTotal > 0 ? (aiTotal / humanTotal).toFixed(1) : null
  const totalHours = Math.round((rangeStats?.totalSeconds || 0) / 3600)
  const linesPerDay = rangeStats?.activeDays > 0 ? Math.round(aiTotal / rangeStats.activeDays) : 0
  const machineCount = rangeStats?.machines?.length || 0

  // Compute tokens from summaries (more reliable than stats.ai_input_tokens)
  const totalInputTokens = useMemo(
    () => summaries.reduce((sum, d) => sum + (Number(d.grand_total?.ai_input_tokens) || 0), 0),
    [summaries]
  )
  const totalOutputTokens = useMemo(
    () => summaries.reduce((sum, d) => sum + (Number(d.grand_total?.ai_output_tokens) || 0), 0),
    [summaries]
  )
  const totalTokens = totalInputTokens + totalOutputTokens
  // Claude Sonnet pricing: $3/$15 per MTok = 0.0003/0.0015 cents per token
  const spendCents = Math.round(totalInputTokens * 0.0003 + totalOutputTokens * 0.0015)

  // Longest streak
  const longestStreak = useMemo(() => {
    let max = 0, cur = 0, streakStart = null, bestStart = null, bestEnd = null
    const sorted = [...summaries].sort((a, b) =>
      (a.range?.date || '').localeCompare(b.range?.date || '')
    )
    for (const day of sorted) {
      if ((Number(day.grand_total?.total_seconds) || 0) > 0) {
        if (cur === 0) streakStart = day.range?.date
        cur++
        if (cur > max) { max = cur; bestStart = streakStart; bestEnd = day.range?.date }
      } else {
        cur = 0
      }
    }
    return { days: max, start: bestStart, end: bestEnd }
  }, [summaries])

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
    const totalLines = Math.max(1, models.reduce((s, m) => s + m.lines, 0))
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
        value: rangeStats?.bestDay?.date ? fmtDateLong(rangeStats.bestDay.date) : '—',
        note: rangeStats?.bestDay?.text || 'No data',
      },
      {
        key: '[LONGEST_STREAK]',
        value: longestStreak.days > 0 ? `${longestStreak.days} days` : '—',
        note:
          longestStreak.start
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
        note: topLanguage ? `${formatPercent(topLanguage.percent)} of focus time` : 'No language data',
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
    [rangeStats, longestStreak, topProject, topLanguage, months, peakMonthIdx, firstActiveDay, selectedYear]
  )

  // Milestones
  const milestones = useMemo(
    () => [
      {
        label: '1M Lines Club',
        unlocked: aiTotal >= 1_000_000,
        date: aiTotal >= 1_000_000 ? fmtDate(rangeStats?.bestDay?.date) || '✓' : '—',
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
    [aiTotal, longestStreak, languages, totalTokens, totalHours, rangeStats]
  )

  return (
    <>
      {/* Hero banner */}
      <section className="relative mb-6 overflow-hidden border border-sky-300/20 bg-gradient-to-br from-sky-300/5 to-transparent p-8">
        <div className="pointer-events-none absolute inset-0 select-none overflow-hidden break-all font-mono text-[9px] leading-3 text-sky-300 opacity-[0.03] whitespace-pre-wrap">
          {'01001000 01100101 01101100 01101100 01101111 '.repeat(600)}
        </div>
        <div className="relative">
          <div className="mb-2 text-[10px] uppercase tracking-[0.3em] text-sky-300">
            // SYNTAX_OPS · WRAPPED · {selectedYear}
          </div>
          <h2 className="mb-2 text-4xl font-medium tracking-tight text-zinc-100 md:text-5xl">
            You shipped <span className="text-sky-300">{formatCount(aiTotal)}</span> lines this year.
          </h2>
          <p className="max-w-[60ch] text-sm text-zinc-400">
            That's roughly{' '}
            <span className="text-zinc-200">{linesPerDay.toLocaleString()} lines per day</span>,
            sustained across{' '}
            <span className="text-zinc-200">{projects.length} projects</span>
            {machineCount > 0 && (
              <> and <span className="text-zinc-200">{machineCount} agent stations</span></>
            )}
            .
            {aiMultiplier && (
              <> Your AI multiplier landed at <span className="text-sky-300">{aiMultiplier}x</span>.</>
            )}
          </p>
        </div>
      </section>

      {data.errors.length > 0 && (
        <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-400" />
          <div>{data.errors.map((e) => <p key={e}>{e}</p>)}</div>
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
            <section key={label} className="ring-1 ring-zinc-900 bg-zinc-950/80 p-4">
              <span className="text-[10px] uppercase tracking-wider text-zinc-500">{label}</span>
              <div className="mt-1 text-3xl font-medium leading-none text-zinc-100">{value}</div>
              <div className="mt-2 text-[11px] text-zinc-600">{note}</div>
            </section>
          ))}
        </div>

        {/* Bottom grid */}
        <div className="grid grid-cols-12 gap-6">
          {/* Left column */}
          <div className="col-span-12 lg:col-span-8 space-y-6">
            {/* Monthly trace */}
            <section className={PANEL}>
              <SectionTitle>
                MONTHLY_TRACE // {formatCount(totalHours)}H
              </SectionTitle>
              <div className="flex h-48 items-stretch gap-2">
                {months.map((month, i) => {
                  const pct = Math.max(4, (month.seconds / maxMonth) * 100)
                  const isPeak = i === peakMonthIdx && month.seconds > 0
                  return (
                    <div key={month.label} className="flex min-w-0 flex-1 flex-col items-center gap-2 group">
                      <span className={`text-[10px] tabular-nums ${isPeak ? 'text-sky-300' : 'text-zinc-600'}`}>
                        {Math.round(month.seconds / 3600)}h
                      </span>
                      <div className="w-full flex-1 bg-zinc-900/40 relative overflow-hidden rounded-sm">
                        <div
                          className={`absolute bottom-0 left-0 right-0 transition-all ${isPeak ? 'bg-sky-300' : 'bg-sky-300/30'}`}
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
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {superlatives.map(({ key, value, note }) => (
                  <div
                    key={key}
                    className="p-3 ring-1 ring-zinc-900 hover:ring-sky-300/30 transition-colors rounded-none bg-zinc-900/20"
                  >
                    <div className="text-[10px] text-sky-300 uppercase tracking-widest mb-1">{key}</div>
                    <div className="text-lg text-zinc-100 font-medium">{value}</div>
                    <div className="text-[10px] text-zinc-500 mt-0.5">{note}</div>
                  </div>
                ))}
              </div>
            </section>

            {/* Language distribution */}
            <section className={PANEL}>
              <SectionTitle>LANGUAGE_DISTRIBUTION</SectionTitle>
              <div className="flex h-8 ring-1 ring-zinc-800 overflow-hidden mb-3 rounded-sm">
                {langBar.map((lang, i) => (
                  <div
                    key={lang.name}
                    className={`h-full flex items-center justify-center text-[10px] uppercase ${LANG_BAR_CLASSES[i] || 'bg-zinc-800 text-zinc-500'}`}
                    style={{ width: `${lang.percent}%` }}
                  >
                    {lang.percent >= 8 ? lang.name : ''}
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-3 md:grid-cols-6 gap-2 text-[10px] text-zinc-500 uppercase">
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
          <div className="col-span-12 lg:col-span-4 space-y-6">
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
                      <div className="flex justify-between text-[11px] mb-1">
                        <span className="text-zinc-300">
                          <span className="text-zinc-600 mr-2 tabular-nums">#{i + 1}</span>
                          {model.name}
                        </span>
                        <span className="text-sky-300 tabular-nums">{model.pct}%</span>
                      </div>
                      <div className="h-1.5 bg-zinc-900 overflow-hidden">
                        <div className="h-full bg-sky-300" style={{ width: `${model.pct}%` }} />
                      </div>
                      <div className="text-[10px] text-zinc-600 mt-0.5 tabular-nums">
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
                    className="flex justify-between items-center border-b border-zinc-900/60 pb-1.5"
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
              <div className="text-center py-2">
                <div className="text-[10px] text-zinc-500 uppercase tracking-widest mb-4">
                  Share your wrapped
                </div>
                <div className="flex gap-2 justify-center">
                  <button
                    type="button"
                    className="text-[10px] py-1.5 px-3 ring-1 ring-sky-300 bg-sky-300/10 text-sky-300 uppercase tracking-widest hover:bg-sky-300/20 transition-colors"
                  >
                    Export PNG
                  </button>
                  <button
                    type="button"
                    className="text-[10px] py-1.5 px-3 ring-1 ring-zinc-800 text-zinc-400 uppercase tracking-widest hover:ring-zinc-700 transition-colors"
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
