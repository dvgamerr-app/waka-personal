import { useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import {
  computeRangeStats,
  formatCount,
  formatPercent,
  normalizeItems,
  topItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-900 bg-zinc-950/80'

const SectionTitle = ({ children }) => (
  <h2 className="mb-6 flex items-center gap-2 text-sm font-medium text-zinc-400">
    <span className="h-1.5 w-1.5 bg-zinc-600" />
    {children}
  </h2>
)

const loadWrapped = async ({ base, timezone, year }) => {
  const { data, error } = await fetchJson({
    base,
    path: '/api/v2/wrapped',
    params: { year, timezone },
  })
  if (error || !data) {
    return { timezone, year, stats: {}, summaries: [], errors: [error || 'Failed to load wrapped'] }
  }
  return {
    timezone: data.timezone || timezone,
    year: data.year || year,
    stats: data.stats || {},
    summaries: data.summaries || [],
    generatedAt: data.generated_at || '',
    errors: [],
  }
}

function WrappedContent({ config = {}, year }) {
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const fallbackTimezone = effectiveConfig.timezone || detectTimezone()
  const initialYear = Number(year) || new Date().getFullYear()
  const currentYear = new Date().getFullYear()
  const [selectedYear, setSelectedYear] = useState(initialYear)
  const [data, setData] = useState({
    timezone: fallbackTimezone,
    year: selectedYear,
    stats: {},
    summaries: [],
    errors: [],
  })

  useEffect(() => {
    let active = true
    loadWrapped({
      base: effectiveConfig.apiBase || '',
      timezone: fallbackTimezone,
      year: selectedYear,
    }).then((next) => {
      if (active) setData(next)
    })
    return () => {
      active = false
    }
  }, [effectiveConfig.apiBase, fallbackTimezone, selectedYear])

  const summaries = useMemo(() => normalizeItems(data.summaries), [data.summaries])
  const rangeStats = useMemo(() => computeRangeStats(summaries), [summaries])
  const projects = useMemo(() => topItems(rangeStats?.projects, 6), [rangeStats])
  const languages = useMemo(() => topItems(rangeStats?.languages, 6), [rangeStats])
  const months = useMemo(() => {
    const buckets = new Map()
    summaries.forEach((day) => {
      const date = day.range?.date || ''
      const key = date.slice(5, 7)
      if (!key) return
      buckets.set(key, (buckets.get(key) || 0) + (Number(day.grand_total?.total_seconds) || 0))
    })
    return Array.from({ length: 12 }, (_, index) => {
      const month = String(index + 1).padStart(2, '0')
      return {
        label: new Date(`${data.year}-${month}-01T00:00:00`).toLocaleDateString('en-US', {
          month: 'short',
        }),
        seconds: buckets.get(month) || 0,
      }
    })
  }, [summaries, data.year])
  const maxMonth = Math.max(1, ...months.map((item) => item.seconds))
  const topProject = projects[0]
  const topLanguage = languages[0]
  const aiTotal = Number(rangeStats?.aiAdditions) + Number(rangeStats?.aiDeletions)
  const canGoNext = selectedYear < currentYear

  return (
    <>
      <section className="relative mb-6 overflow-hidden border border-sky-300/30 bg-sky-300/5 p-8">
        <div className="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-4xl">
            <div className="mb-4 text-[10px] tracking-[0.32em] text-sky-300 uppercase">
              Annual telemetry package
            </div>
            <h2 className="text-4xl font-medium tracking-tight text-zinc-100 md:text-6xl">
              {data.year} in code, compiled from your local archive.
            </h2>
            <p className="mt-5 max-w-2xl text-sm leading-6 text-zinc-500">
              Active time, language gravity, AI-assisted momentum, and project peaks from the
              Waka-compatible backend.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="flex h-9 w-9 items-center justify-center border border-zinc-800 text-zinc-400 transition-colors hover:border-sky-300/40 hover:text-sky-300"
              title="Previous year"
              onClick={() => setSelectedYear((value) => value - 1)}
            >
              <ChevronLeft size={16} />
            </button>
            <div className="border border-zinc-800 bg-zinc-950 px-4 py-2 font-mono text-xs tracking-[0.24em] text-zinc-300 uppercase">
              wrapped --year={selectedYear}
            </div>
            <button
              type="button"
              className="flex h-9 w-9 items-center justify-center border border-zinc-800 text-zinc-400 transition-colors hover:border-sky-300/40 hover:text-sky-300 disabled:cursor-not-allowed disabled:border-zinc-900 disabled:text-zinc-700"
              title="Next year"
              disabled={!canGoNext}
              onClick={() => setSelectedYear((value) => Math.min(currentYear, value + 1))}
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      </section>

      {data.errors.length > 0 && (
        <div className="mb-6 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          {data.errors.map((error) => (
            <p key={error}>{error}</p>
          ))}
        </div>
      )}

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        {[
          [
            'Total Active Time',
            rangeStats?.humanReadableTotal || '-',
            `${rangeStats?.activeDays || 0} active days`,
          ],
          ['AI Assisted Changes', formatCount(aiTotal), 'Additions and deletions combined'],
          ['Top Language', topLanguage?.name || '-', topLanguage?.text || 'No language data'],
          ['Top Project', topProject?.name || '-', topProject?.text || 'No project data'],
        ].map(([label, value, note]) => (
          <section key={label} className={`${PANEL} p-4`}>
            <span className="text-[10px] tracking-[0.24em] text-zinc-500 uppercase">{label}</span>
            <div className="mt-1 font-mono text-3xl leading-none font-medium text-zinc-100">
              {value}
            </div>
            <div className="mt-3 text-[11px] text-zinc-600">{note}</div>
          </section>
        ))}
      </div>

      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-12 space-y-6 lg:col-span-8">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>MONTHLY_ACTIVITY_CURVE</SectionTitle>
            <div className="flex h-52 items-end gap-2 md:gap-3">
              {months.map((month) => {
                const pct = Math.max(4, (month.seconds / maxMonth) * 100)
                return (
                  <div
                    key={month.label}
                    className="flex min-w-0 flex-1 flex-col items-center gap-2"
                  >
                    <span className="font-mono text-[10px] text-zinc-600 tabular-nums">
                      {Math.round(month.seconds / 3600)}h
                    </span>
                    <div className="relative h-36 w-full overflow-hidden bg-zinc-900/60">
                      <div
                        className="absolute inset-x-0 bottom-0 bg-sky-300/50"
                        style={{ height: `${pct}%` }}
                      />
                    </div>
                    <span className="text-[10px] text-zinc-500 uppercase">{month.label}</span>
                  </div>
                )
              })}
            </div>
          </section>

          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>PROJECTS</SectionTitle>
            <div className="space-y-3">
              {projects.map((project) => (
                <div
                  key={project.name}
                  className="grid grid-cols-[minmax(0,1fr)_72px_64px] items-center gap-3 text-[11px]"
                >
                  <span className="truncate text-zinc-300">{project.name}</span>
                  <span className="text-right font-mono text-zinc-500">{project.text}</span>
                  <span className="text-right font-mono text-sky-300">
                    {formatPercent(project.ai_percent)}
                  </span>
                </div>
              ))}
            </div>
          </section>
        </div>

        <aside className="col-span-12 space-y-6 lg:col-span-4">
          <section className={`${PANEL} p-5 lg:p-6`}>
            <SectionTitle>LANGUAGE_DISTRIBUTION</SectionTitle>
            <div className="space-y-3">
              {languages.map((language) => (
                <div key={language.name} className="text-[11px]">
                  <div className="mb-1 flex justify-between gap-3">
                    <span className="truncate text-zinc-300">{language.name}</span>
                    <span className="font-mono text-zinc-500">
                      {formatPercent(language.percent)}
                    </span>
                  </div>
                  <div className="h-1.5 bg-zinc-900">
                    <div className="h-full bg-sky-300" style={{ width: `${language.percent}%` }} />
                  </div>
                </div>
              ))}
            </div>
          </section>
        </aside>
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
