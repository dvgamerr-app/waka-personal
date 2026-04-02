import { useMemo } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, ReferenceLine } from 'recharts'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart'
import { normalizeItems } from './dashboardUtils.js'

const LEGEND = [
  { key: 'humanAdditions', label: 'Human add', color: '#84cc16' },
  { key: 'humanDeletions', label: 'Human del', color: '#f43f5e' },
  { key: 'aiAdditions', label: 'AI add', color: '#38bdf8' },
  { key: 'aiDeletions', label: 'AI del', color: '#a78bfa' },
]

const CHART_CONFIG = {
  humanAdditions: { label: 'Human Additions', color: '#84cc16' },
  aiAdditions: { label: 'AI Additions', color: '#38bdf8' },
  aiDeletions: { label: 'AI Deletions', color: '#a78bfa' },
  humanDeletions: { label: 'Human Deletions', color: '#f43f5e' },
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
    <Card className="border-border/80 bg-background/75 shadow-none backdrop-blur-sm">
      <CardHeader className="gap-4 p-5 xl:flex-row xl:items-end xl:justify-between">
        <div className="min-w-0">
          <CardTitle className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
            {title}
          </CardTitle>
          {subtitle && (
            <CardDescription className="text-foreground/65 mt-2">{subtitle}</CardDescription>
          )}
        </div>
        <div className="text-foreground/70 flex flex-wrap items-center gap-4 text-xs">
          {LEGEND.map((item) => (
            <span key={item.key} className="flex items-center gap-2">
              <span className="border-border h-3 w-3 border" style={{ background: item.color }} />
              {item.label}
            </span>
          ))}
        </div>
      </CardHeader>

      <CardContent className="px-5 pb-5">
        {items.length === 0 ? (
          <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
            No line change data yet.
          </div>
        ) : (
          <ChartContainer config={CHART_CONFIG} className="h-[280px] w-full">
            <BarChart
              data={chartData}
              stackOffset="sign"
              barCategoryGap="30%"
              barGap={0}
              margin={{ top: 8, right: 8, left: -20, bottom: 0 }}
            >
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
                width={48}
              />
              <ReferenceLine y={0} stroke="currentColor" strokeOpacity={0.3} />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name) => (
                      <>
                        <span className="text-muted-foreground">
                          {CHART_CONFIG[name]?.label || name}
                        </span>
                        <span className="text-foreground ml-auto font-mono tabular-nums">
                          {Math.abs(value).toLocaleString()}
                        </span>
                      </>
                    )}
                  />
                }
              />
              <Bar
                dataKey="humanAdditions"
                stackId="human"
                fill="#84cc16"
                maxBarSize={20}
                radius={0}
              />
              <Bar
                dataKey="humanDeletions"
                stackId="human"
                fill="#f43f5e"
                maxBarSize={20}
                radius={0}
              />
              <Bar dataKey="aiAdditions" stackId="ai" fill="#38bdf8" maxBarSize={20} radius={0} />
              <Bar dataKey="aiDeletions" stackId="ai" fill="#a78bfa" maxBarSize={20} radius={0} />
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
