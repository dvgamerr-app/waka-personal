import { useMemo } from 'react'
import { ComposedChart, Bar, Line, XAxis, YAxis, CartesianGrid } from 'recharts'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart'
import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const LONG_RANGES = new Set(['last year', 'last_year'])

export default function DailyTrendChart({
  title = 'Daily Trend',
  subtitle = '',
  days = [],
  range = '',
}) {
  const entries = normalizeItems(days)
  const showLine = !LONG_RANGES.has((range || '').toLowerCase()) && entries.length < 355

  const { chartData, chartConfig, categoryKeys } = useMemo(() => {
    if (!entries.length) return { chartData: [], chartConfig: {}, categoryKeys: [] }

    const allCategories = []
    const seen = new Set()

    entries.forEach((entry) => {
      entry.segments?.forEach((segment) => {
        if (!seen.has(segment.name)) {
          seen.add(segment.name)
          allCategories.push({ name: segment.name, color: segment.color })
        }
      })
    })

    const config = { __total: { label: 'Total', color: '#e2e8f0' } }
    allCategories.forEach((category) => {
      config[category.name] = { label: category.name, color: category.color }
    })

    const data = entries.map((entry) => {
      const flat = { label: entry.label, __total: entry.totalSeconds || 0 }
      entry.segments?.forEach((segment) => {
        flat[segment.name] = segment.seconds || 0
      })
      return flat
    })

    return { chartData: data, chartConfig: config, categoryKeys: allCategories }
  }, [entries])

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
                tickFormatter={(seconds) => formatShortDuration(seconds)}
                width={48}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name) => (
                      <>
                        <span className="text-muted-foreground">{name}</span>
                        <span className="text-foreground ml-auto font-mono tabular-nums">
                          {formatShortDuration(value)}
                        </span>
                      </>
                    )}
                  />
                }
              />
              {categoryKeys.map((category) => (
                <Bar
                  key={category.name}
                  dataKey={category.name}
                  stackId="a"
                  fill={category.color}
                  radius={0}
                  maxBarSize={36}
                />
              ))}
              {showLine && (
                <Line
                  dataKey="__total"
                  type="linear"
                  stroke="#e2e8f0"
                  strokeWidth={2}
                  dot={{ fill: '#f8fafc', r: 3, strokeWidth: 0 }}
                />
              )}
            </ComposedChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
