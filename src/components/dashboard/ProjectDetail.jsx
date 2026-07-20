import { useEffect, useMemo, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  Bot,
  CalendarCheck,
  CircleDollarSign,
  Clock3,
  ExternalLink,
  FileCode2,
  GitBranch,
  MessageSquareText,
  Users,
} from 'lucide-react'
import { ThemeProvider } from '@/stores/theme'
import DateRangePicker from './DateRangePicker'
import {
  computeRangeStats,
  formatCount,
  formatDayLabel,
  formatPercent,
  formatShortDuration,
  normalizeItems,
} from './dashboardUtils.js'
import { detectTimezone, fetchJson, readRuntimeConfig } from './apiClient.js'

const PANEL = 'border border-zinc-900 bg-zinc-950/80'
const LABEL = 'text-[11px] tracking-[0.22em] text-zinc-500 uppercase'

const getPageParams = () => {
  if (typeof window === 'undefined') return {}
  const params = new URLSearchParams(window.location.search)
  return {
    project: params.get('name') || '',
    range: params.get('range') || '',
    start: params.get('start') || '',
    end: params.get('end') || '',
  }
}

const aggregateItems = (summaries, key) => {
  const totals = new Map()
  normalizeItems(summaries).forEach((day) => {
    normalizeItems(day[key]).forEach((item) => {
      const name = item.name || 'Unknown'
      totals.set(name, (totals.get(name) || 0) + (Number(item.total_seconds) || 0))
    })
  })
  const total = Math.max(
    1,
    Array.from(totals.values()).reduce((sum, value) => sum + value, 0)
  )
  return Array.from(totals.entries())
    .map(([name, totalSeconds]) => ({
      name,
      totalSeconds,
      percent: (totalSeconds / total) * 100,
    }))
    .sort((a, b) => b.totalSeconds - a.totalSeconds)
}

const projectTokens = (summaries) =>
  normalizeItems(summaries).reduce(
    (totals, day) => ({
      input: totals.input + (Number(day.grand_total?.ai_input_tokens) || 0),
      output: totals.output + (Number(day.grand_total?.ai_output_tokens) || 0),
    }),
    { input: 0, output: 0 }
  )

const projectAIMetrics = (summaries) => {
  const models = new Map()
  const metrics = normalizeItems(summaries).reduce(
    (totals, day) => {
      const grandTotal = day.grand_total || {}
      totals.prompts += Number(grandTotal.ai_prompt_count) || 0
      totals.promptChars += Number(grandTotal.ai_prompt_chars) || 0
      totals.sessions += Number(grandTotal.ai_session_count) || 0

      normalizeItems(day.ai_models).forEach((model) => {
        const name = model.name || 'Unknown model'
        const current = models.get(name) || { name, spendCents: 0 }
        current.spendCents += Number(model.spend_cents) || 0
        models.set(name, current)
      })
      return totals
    },
    { prompts: 0, promptChars: 0, sessions: 0 }
  )

  metrics.models = Array.from(models.values()).sort((a, b) => b.spendCents - a.spendCents)
  metrics.spendCents = metrics.models.reduce((sum, model) => sum + model.spendCents, 0)
  return metrics
}

const formatMoney = (cents) =>
  new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: cents >= 10000 ? 0 : 2,
    maximumFractionDigits: cents >= 10000 ? 0 : 2,
  }).format((Number(cents) || 0) / 100)

const formatPrecisePercent = (value) => {
  const numeric = Number(value) || 0
  if (numeric > 0 && numeric < 1) return `${numeric.toFixed(2).replace(/0+$/, '')}%`
  return formatPercent(numeric)
}

const aiPercent = (stats) => {
  const ai = (Number(stats?.aiAdditions) || 0) + (Number(stats?.aiDeletions) || 0)
  const human = (Number(stats?.humanAdditions) || 0) + (Number(stats?.humanDeletions) || 0)
  return ai + human > 0 ? (ai / (ai + human)) * 100 : 0
}

const rangeText = (summaries) => {
  const days = normalizeItems(summaries)
  if (!days.length) return 'No activity in selected range'
  const first = days[0]?.range?.date
  const last = days[days.length - 1]?.range?.date
  return first === last ? first : `${first} → ${last}`
}

const Kpi = ({ icon: Icon, label, value, note }) => (
  <section className={`${PANEL} p-4`}>
    <div className="mb-4 flex items-center justify-between gap-3">
      <span className={LABEL}>{label}</span>
      <Icon size={15} className="text-sky-300" />
    </div>
    <div className="font-mono text-2xl font-medium text-zinc-100">{value}</div>
    <div className="mt-2 text-xs text-zinc-600">{note}</div>
  </section>
)

const SectionTitle = ({ children }) => (
  <h2 className="mb-5 flex items-center gap-2 text-xs font-semibold tracking-[0.18em] text-zinc-400 uppercase">
    <span className="h-1.5 w-1.5 bg-sky-300" />
    {children}
  </h2>
)

const ActivityTrend = ({ summaries }) => {
  const days = normalizeItems(summaries).map((day) => ({
    date: day.range?.date,
    seconds: Number(day.grand_total?.total_seconds) || 0,
  }))
  const max = Math.max(1, ...days.map((day) => day.seconds))

  return (
    <section className={`${PANEL} p-5 lg:p-6`}>
      <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
        <SectionTitle>DAILY_PROJECT_TREND</SectionTitle>
        <span className="text-[11px] text-zinc-600 uppercase">Focused time per day</span>
      </div>
      {days.length === 0 ? (
        <EmptyState label="No project activity in this range." />
      ) : (
        <div className="flex h-56 items-end gap-1.5 sm:gap-2">
          {days.map((day) => {
            const height = day.seconds > 0 ? Math.max(3, (day.seconds / max) * 100) : 0
            const isPeak = day.seconds === max
            return (
              <div key={day.date} className="group flex min-w-0 flex-1 flex-col items-center gap-2">
                <span className="hidden font-mono text-[10px] text-zinc-600 group-hover:block xl:block">
                  {day.seconds > 0 ? formatShortDuration(day.seconds) : '—'}
                </span>
                <div className="relative h-40 w-full bg-zinc-900/60">
                  <div
                    className={`absolute inset-x-0 bottom-0 transition-colors ${isPeak ? 'bg-sky-300' : 'bg-sky-300/35 group-hover:bg-sky-300/60'}`}
                    style={{ height: `${height}%` }}
                    title={`${day.date}: ${formatShortDuration(day.seconds)}`}
                  />
                </div>
                <span className="max-w-full truncate text-[10px] text-zinc-600 uppercase">
                  {formatDayLabel(day.date)}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

const RankedBreakdown = ({ title, items, emptyLabel = 'No data available.' }) => {
  const rows = normalizeItems(items).slice(0, 8)
  return (
    <section className={`${PANEL} p-5`}>
      <SectionTitle>{title}</SectionTitle>
      {rows.length === 0 ? (
        <EmptyState label={emptyLabel} />
      ) : (
        <div className="space-y-4">
          {rows.map((item) => (
            <div key={item.name}>
              <div className="mb-1.5 flex items-center justify-between gap-4 text-xs">
                <span className="truncate text-zinc-300">{item.name}</span>
                <span className="shrink-0 font-mono text-zinc-500">
                  {formatShortDuration(item.totalSeconds)} · {formatPercent(item.percent)}
                </span>
              </div>
              <div className="h-1.5 bg-zinc-900">
                <div className="h-full bg-sky-300/60" style={{ width: `${item.percent}%` }} />
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

const FileActivity = ({ files }) => (
  <section className={`${PANEL} p-5`}>
    <SectionTitle>ACTIVE_FILES</SectionTitle>
    {files.length === 0 ? (
      <EmptyState label="No file activity available." />
    ) : (
      <div className="divide-y divide-zinc-900/80">
        {files.slice(0, 10).map((file, index) => (
          <div
            key={file.name}
            className="grid grid-cols-[28px_minmax(0,1fr)_auto] gap-3 py-3 text-xs"
          >
            <span className="font-mono text-zinc-700">{String(index + 1).padStart(2, '0')}</span>
            <span className="truncate text-zinc-300" title={file.name}>
              {file.name}
            </span>
            <span className="font-mono text-zinc-500">
              {formatShortDuration(file.totalSeconds)}
            </span>
          </div>
        ))}
      </div>
    )}
  </section>
)

const EmptyState = ({ label }) => (
  <div className="border border-dashed border-zinc-800 p-6 text-sm text-zinc-600">{label}</div>
)

const ProjectDetail = ({ config = {} }) => {
  return (
    <ThemeProvider>
      <ProjectDetailContent config={config} />
    </ThemeProvider>
  )
}

const ProjectDetailContent = ({ config }) => {
  const pageParams = useMemo(() => getPageParams(), [])
  const runtimeConfig = readRuntimeConfig()
  const effectiveConfig = { ...config, ...runtimeConfig }
  const timezone = effectiveConfig.timezone || detectTimezone()
  const project = pageParams.project
  const hasCustomRange = Boolean(pageParams.start && pageParams.end)
  const [selectedRange, setSelectedRange] = useState(
    hasCustomRange ? 'Custom Range' : pageParams.range || 'Last 30 Days'
  )
  const [summaries, setSummaries] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [initialCustomRange] = useState(() =>
    hasCustomRange
      ? {
          from: new Date(`${pageParams.start}T00:00:00`),
          to: new Date(`${pageParams.end}T00:00:00`),
        }
      : undefined
  )

  const loadProject = async ({ range, start, end }) => {
    if (!project) {
      setError('Project name is missing from the URL.')
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    const { data, error: requestError } = await fetchJson({
      base: effectiveConfig.apiBase || '',
      path: '/api/v2/project',
      params: { project, timezone, range, start, end },
    })
    if (requestError || !data) {
      setError(requestError || 'Failed to load project details.')
      setSummaries([])
    } else {
      setSummaries(normalizeItems(data.summaries))
    }
    setLoading(false)
  }

  useEffect(() => {
    loadProject(
      hasCustomRange
        ? { start: pageParams.start, end: pageParams.end }
        : { range: pageParams.range || 'Last 30 Days' }
    )
  }, [])

  const stats = useMemo(() => computeRangeStats(summaries), [summaries])
  const languages = useMemo(() => aggregateItems(summaries, 'languages'), [summaries])
  const editors = useMemo(() => aggregateItems(summaries, 'editors'), [summaries])
  const branches = useMemo(() => aggregateItems(summaries, 'branches'), [summaries])
  const files = useMemo(() => aggregateItems(summaries, 'entities'), [summaries])
  const tokens = useMemo(() => projectTokens(summaries), [summaries])
  const aiMetrics = useMemo(() => projectAIMetrics(summaries), [summaries])
  const contribution = aiPercent(stats)
  const aiChanges = (Number(stats?.aiAdditions) || 0) + (Number(stats?.aiDeletions) || 0)
  const humanChanges = (Number(stats?.humanAdditions) || 0) + (Number(stats?.humanDeletions) || 0)
  const changeTotal = Math.max(1, aiChanges + humanChanges)
  const humanContribution = (humanChanges / changeTotal) * 100
  const averagePromptChars = aiMetrics.prompts
    ? Math.round(aiMetrics.promptChars / aiMetrics.prompts)
    : 0
  const averagePrompts = aiMetrics.sessions ? aiMetrics.prompts / aiMetrics.sessions : 0
  const spendBreakdown = aiMetrics.models
    .filter((model) => model.spendCents > 0)
    .slice(0, 2)
    .map((model) => `${model.name} ${formatMoney(model.spendCents)}`)
    .join(' · ')
  const wakaUrl = `https://wakatime.com/projects/${encodeURIComponent(project)}`

  const handleRangeChange = ({ range, start, end }) => {
    const nextRange = range || 'Custom Range'
    setSelectedRange(nextRange)
    loadProject({ range, start, end })
    const params = new URLSearchParams({ name: project })
    if (start && end) {
      params.set('start', start)
      params.set('end', end)
    } else {
      params.set('range', nextRange)
    }
    history.replaceState(null, '', `/project?${params}`)
  }

  return (
    <div className={`transition-opacity duration-200 ${loading ? 'opacity-55' : ''}`}>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4 border border-zinc-900 bg-zinc-950 p-4">
        <div className="flex min-w-0 items-center gap-3">
          <a
            href="/"
            className="flex h-9 w-9 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-sky-300/50 hover:text-sky-300"
            aria-label="Back to dashboard"
          >
            <ArrowLeft size={16} />
          </a>
          <div className="min-w-0">
            <p className={LABEL}>Project Detail</p>
            <h2 className="truncate text-lg font-medium text-zinc-100">
              {project || 'Unknown project'}
            </h2>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <a
            href={wakaUrl}
            target="_blank"
            rel="noreferrer"
            className="flex h-9 items-center gap-2 border border-zinc-800 px-3 text-[11px] tracking-[0.14em] text-zinc-500 uppercase transition-colors hover:border-sky-300/50 hover:text-sky-300"
          >
            WakaTime <ExternalLink size={13} />
          </a>
          <DateRangePicker
            value={selectedRange}
            onChange={handleRangeChange}
            initialCustomRange={initialCustomRange}
          />
        </div>
      </div>

      {error && (
        <div className="mb-6 flex gap-3 border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-400" />
          {error}
        </div>
      )}

      <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Kpi
          icon={Clock3}
          label="Focused Time"
          value={stats?.humanReadableTotal || '0s'}
          note={`${stats?.humanReadableDailyAvg || '0s'} daily average · ${rangeText(summaries)}`}
        />
        <Kpi
          icon={Bot}
          label="AI Changes"
          value={formatCount(aiChanges)}
          note={`${formatPrecisePercent(contribution)} of recorded changes`}
        />
        <Kpi
          icon={Activity}
          label="Human Changes"
          value={formatCount(humanChanges)}
          note={`${formatPrecisePercent(humanContribution)} of recorded changes`}
        />
        <Kpi
          icon={MessageSquareText}
          label="AI Prompts"
          value={formatCount(aiMetrics.prompts)}
          note={`${formatCount(averagePromptChars)} average characters`}
        />
        <Kpi
          icon={Users}
          label="AI Sessions"
          value={formatCount(aiMetrics.sessions)}
          note={`${averagePrompts.toFixed(1).replace(/\.0$/, '')} average prompts`}
        />
        <Kpi
          icon={FileCode2}
          label="Tokens"
          value={formatCount(tokens.input + tokens.output)}
          note={`${formatCount(tokens.input)} in · ${formatCount(tokens.output)} out`}
        />
        <Kpi
          icon={CircleDollarSign}
          label="AI Spend"
          value={formatMoney(aiMetrics.spendCents)}
          note={spendBreakdown || 'No priced model usage'}
        />
        <Kpi
          icon={CalendarCheck}
          label="Active Days"
          value={`${stats?.activeDays || 0}/${stats?.totalDays || 0}`}
          note={`Peak ${stats?.bestDay?.date || '—'} · ${stats?.bestDay?.text || '0s'}`}
        />
      </div>

      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-12 space-y-6 xl:col-span-8">
          <ActivityTrend summaries={summaries} />
          <FileActivity files={files} />
        </div>
        <aside className="col-span-12 grid gap-6 md:grid-cols-2 xl:col-span-4 xl:grid-cols-1">
          <RankedBreakdown title="LANGUAGE_MIX" items={languages} />
          <RankedBreakdown title="EDITOR_USAGE" items={editors} />
          <RankedBreakdown
            title="BRANCH_ACTIVITY"
            items={branches}
            emptyLabel="No branch information available."
          />
        </aside>
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-2 border border-zinc-900 p-4 text-[11px] tracking-[0.16em] text-zinc-600 uppercase">
        <span className="flex items-center gap-2 text-zinc-400">
          <GitBranch size={13} /> {branches.length} branches
        </span>
        <span>{files.length} tracked files</span>
        <span>{languages.length} languages</span>
        <span className="ml-auto">Timezone: {timezone}</span>
      </div>
    </div>
  )
}

export default ProjectDetail
