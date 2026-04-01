import { useMemo } from 'react'
import { PieChart, Pie, Cell, Tooltip } from 'recharts'
import { ChartContainer } from '@/components/ui/chart'
import { enrichItemsWithColor, formatPercent, formatShortDuration } from './dashboardUtils.js'

const CHART_CONFIG = {}

const CustomTooltip = ({ active, payload }) => {
  if (!active || !payload?.length) return null
  const item = payload[0]
  return (
    <div className="border-border/50 bg-background grid gap-1 border px-2.5 py-1.5 text-xs shadow-xl">
      <p className="font-medium">{item.name}</p>
      <p className="text-muted-foreground">
        {formatShortDuration(item.payload.total_seconds)} · {formatPercent(item.payload.percent)}
      </p>
    </div>
  )
}

export default function BreakdownCard({
  title = '',
  subtitle = '',
  items = [],
  emptyLabel = 'No data available.',
}) {
  const segments = useMemo(() => enrichItemsWithColor(items, 6), [items])
  const totalPercent = useMemo(
    () => segments.reduce((sum, item) => sum + (Number(item.percent) || 0), 0) || 1,
    [segments]
  )

  const pieData = useMemo(
    () =>
      segments.map((item) => ({
        name: item.name,
        value: Number(item.percent) || 0,
        color: item.color,
        total_seconds: item.total_seconds,
        percent: item.percent,
      })),
    [segments]
  )

  const pieConfig = useMemo(() => {
    const cfg = {}
    segments.forEach((item) => {
      cfg[item.name] = { label: item.name, color: item.color }
    })
    return cfg
  }, [segments])

  return (
    <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
      <div className="mb-4">
        <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
          {title}
        </p>
        {subtitle && <p className="text-foreground/65 mt-2 text-sm">{subtitle}</p>}
      </div>

      {segments.length === 0 ? (
        <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
          {emptyLabel}
        </div>
      ) : (
        <div className="grid gap-6 lg:grid-cols-[160px_minmax(0,1fr)]">
          <div className="flex items-center justify-center">
            <ChartContainer config={pieConfig} className="h-40 w-40">
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  innerRadius={38}
                  outerRadius={58}
                  startAngle={90}
                  endAngle={-270}
                  strokeWidth={0}
                >
                  {pieData.map((entry, index) => (
                    <Cell key={index} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip content={<CustomTooltip />} />
              </PieChart>
            </ChartContainer>
          </div>

          <div className="space-y-3">
            {segments.map((item) => (
              <div
                key={item.name}
                className="border-border/50 grid grid-cols-[14px_minmax(0,1fr)_auto] items-start gap-3 border-b pb-3 last:border-b-0 last:pb-0"
              >
                <span
                  className="border-border mt-1 h-3 w-3 border"
                  style={{ background: item.color }}
                />
                <div className="min-w-0">
                  <p className="text-foreground truncate text-sm font-medium">{item.name}</p>
                  <p className="text-foreground/60 mt-1 text-xs">
                    {formatShortDuration(item.total_seconds)}
                  </p>
                </div>
                <span className="text-foreground/70 text-sm font-medium">
                  {formatPercent(item.percent)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}
