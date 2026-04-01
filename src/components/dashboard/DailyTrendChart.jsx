import { useMemo } from 'react'
import {
  ComposedChart,
  Bar,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from 'recharts'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from '@/components/ui/chart'
import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const CustomTooltip = ({ active, payload, label }) => {
  if (!active || !payload?.length) return null
  const total = payload.find((p) => p.dataKey === '__total')
  const bars = payload.filter((p) => p.dataKey !== '__total')

  return (
    <div className="border-border/50 bg-background grid min-w-[8rem] gap-1.5 border px-2.5 py-1.5 text-xs shadow-xl">
      <p className="font-medium">{label}</p>
      {total && (
        <p className="text-foreground/70">
          Total: <span className="text-foreground font-medium">{formatShortDuration(total.value)}</span>
        </p>
      )}
      {bars.map((p) => (
        <div key={p.dataKey} className="flex items-center gap-2">
          <span className="h-2 w-2 shrink-0" style={{ backgroundColor: p.fill || p.color }} />
          <span className="text-muted-foreground">{p.name}</span>
          <span className="text-foreground ml-auto font-mono tabular-nums">
            {formatShortDuration(p.value)}
          </span>
        </div>
      ))}
    </div>
  )
}

export default function DailyTrendChart({ title = 'Daily Trend', subtitle = '', days = [] }) {
  const entries = normalizeItems(days)

  const { chartData, chartConfig, categoryKeys } = useMemo(() => {
    if (!entries.length) return { chartData: [], chartConfig: {}, categoryKeys: [] }

    const allCategories = []
    const seen = new Set()
    entries.forEach((d) => {
      d.segments?.forEach((seg) => {
        if (!seen.has(seg.name)) {
          seen.add(seg.name)
          allCategories.push({ name: seg.name, color: seg.color })
        }
      })
    })

    const cfg = { __total: { label: 'Total', color: '#e2e8f0' } }
    allCategories.forEach((cat) => {
      cfg[cat.name] = { label: cat.name, color: cat.color }
    })

    const data = entries.map((d) => {
      const flat = { label: d.label, __total: d.totalSeconds || 0 }
      d.segments?.forEach((seg) => {
        flat[seg.name] = seg.seconds || 0
      })
      return flat
    })

    return { chartData: data, chartConfig: cfg, categoryKeys: allCategories }
  }, [entries])

  return (
    <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
      <div className="mb-4">
        <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
          {title}
        </p>
        {subtitle && <p className="text-foreground/65 mt-2 text-sm">{subtitle}</p>}
      </div>

      {entries.length === 0 ? (
        <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
          No activity in this range.
        </div>
      ) : (
        <ChartContainer config={chartConfig} className="h-[290px] w-full">
          <ComposedChart data={chartData} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
            <CartesianGrid vertical={false} stroke="currentColor" strokeOpacity={0.08} />
            <XAxis
              dataKey="label"
              tickLine={false}
              axisLine={false}
              tick={{ fontSize: 11, opacity: 0.65 }}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tick={{ fontSize: 10, opacity: 0.5 }}
              tickFormatter={(v) => formatShortDuration(v)}
              width={48}
            />
            <Tooltip content={<CustomTooltip />} />
            {categoryKeys.map((cat) => (
              <Bar
                key={cat.name}
                dataKey={cat.name}
                stackId="a"
                fill={cat.color}
                radius={0}
                maxBarSize={36}
              />
            ))}
            <Line
              dataKey="__total"
              type="linear"
              stroke="#e2e8f0"
              strokeWidth={2}
              dot={{ fill: '#f8fafc', r: 3, strokeWidth: 0 }}
            />
          </ComposedChart>
        </ChartContainer>
      )}
    </section>
  )
}
