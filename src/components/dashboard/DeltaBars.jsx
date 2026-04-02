import { useMemo } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, ReferenceLine, Cell } from 'recharts'
import { ChartContainer } from '@/components/ui/chart'
import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const LEGEND = [
  { key: 'humanAdditions', label: 'Human add', color: '#84cc16' },
  { key: 'aiAdditions', label: 'AI add', color: '#38bdf8' },
  { key: 'aiDeletions', label: 'AI del', color: '#a78bfa' },
  { key: 'humanDeletions', label: 'Human del', color: '#f43f5e' },
]

const CHART_CONFIG = {
  humanAdditions: { label: 'Human Additions', color: '#84cc16' },
  aiAdditions: { label: 'AI Additions', color: '#38bdf8' },
  aiDeletions: { label: 'AI Deletions', color: '#a78bfa' },
  humanDeletions: { label: 'Human Deletions', color: '#f43f5e' },
}

const CustomTooltip = ({ active, payload, label }) => {
  if (!active || !payload?.length) return null
  return (
    <div className="border-border/50 bg-background grid min-w-[9rem] gap-1.5 border px-2.5 py-1.5 text-xs shadow-xl">
      <p className="font-medium">{label}</p>
      {payload.map((p) => (
        <div key={p.dataKey} className="flex items-center gap-2">
          <span className="h-2 w-2 shrink-0" style={{ backgroundColor: p.fill }} />
          <span className="text-muted-foreground">{CHART_CONFIG[p.dataKey]?.label || p.name}</span>
          <span className="text-foreground ml-auto font-mono tabular-nums">{Math.abs(p.value).toLocaleString()}</span>
        </div>
      ))}
    </div>
  )
}

export default function DeltaBars({ title = 'AI vs Human', subtitle = '', series = [] }) {
  const items = normalizeItems(series)

  const chartData = useMemo(
    () =>
      items.map((item) => ({
        label: item.label,
        aiAdditions: item.aiAdditions,
        aiDeletions: -item.aiDeletions,
        humanAdditions: item.humanAdditions,
        humanDeletions: -item.humanDeletions,
      })),
    [items]
  )

  return (
    <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
            {title}
          </p>
          {subtitle && <p className="text-foreground/65 mt-2 text-sm">{subtitle}</p>}
        </div>
        <div className="text-foreground/70 flex flex-wrap items-center gap-4 text-xs">
          {LEGEND.map((item) => (
            <span key={item.key} className="flex items-center gap-2">
              <span className="border-border h-3 w-3 border" style={{ background: item.color }} />
              {item.label}
            </span>
          ))}
        </div>
      </div>

      {items.length === 0 ? (
        <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
          No line change data yet.
        </div>
      ) : (
        <ChartContainer config={CHART_CONFIG} className="h-[280px] w-full">
          <BarChart data={chartData} barCategoryGap="30%" barGap={2} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
            <CartesianGrid vertical={false} stroke="currentColor" strokeOpacity={0.08} />
            <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 11, opacity: 0.65 }} />
            <YAxis tickLine={false} axisLine={false} tick={{ fontSize: 10, opacity: 0.5 }} width={48} />
            <ReferenceLine y={0} stroke="currentColor" strokeOpacity={0.3} />
            <Bar dataKey="humanAdditions" stackId="add" fill="#84cc16" maxBarSize={20} radius={0} />
            <Bar dataKey="aiAdditions" stackId="add" fill="#38bdf8" maxBarSize={20} radius={0} />
            <Bar dataKey="aiDeletions" stackId="del" fill="#a78bfa" maxBarSize={20} radius={0} />
            <Bar dataKey="humanDeletions" stackId="del" fill="#f43f5e" maxBarSize={20} radius={0} />
          </BarChart>
        </ChartContainer>
      )}
    </section>
  )
}
