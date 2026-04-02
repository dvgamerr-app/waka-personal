import { useMemo } from 'react'
import { PieChart, Pie, Cell } from 'recharts'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart'
import { enrichItemsWithColor, formatPercent, formatShortDuration } from './dashboardUtils.js'

export default function BreakdownCard({
  title = '',
  subtitle = '',
  items = [],
  emptyLabel = 'No data available.',
}) {
  const segments = useMemo(() => enrichItemsWithColor(items, 6), [items])
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
    <Card className="border-border/80 bg-background/75 shadow-none backdrop-blur-sm">
      <CardHeader className="p-5">
        <CardTitle className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
          {title}
        </CardTitle>
        {subtitle && (
          <CardDescription className="text-foreground/65 mt-2">{subtitle}</CardDescription>
        )}
      </CardHeader>

      <CardContent className="px-5 pb-5">
        {segments.length === 0 ? (
          <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
            {emptyLabel}
          </div>
        ) : (
          <div className="grid gap-6 lg:grid-cols-[160px_minmax(0,1fr)]">
            <ChartContainer config={pieConfig} className="mx-auto h-40 w-40">
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
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      hideIndicator
                      formatter={(_, name, item) => (
                        <>
                          <span className="text-foreground">{name}</span>
                          <span className="text-muted-foreground ml-auto">
                            {formatShortDuration(item.payload.total_seconds)} ·{' '}
                            {formatPercent(item.payload.percent)}
                          </span>
                        </>
                      )}
                    />
                  }
                />
              </PieChart>
            </ChartContainer>

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
      </CardContent>
    </Card>
  )
}
