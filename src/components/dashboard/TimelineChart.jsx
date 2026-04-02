import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const HOURLY_MARKS = Array.from({ length: 12 }, (_, i) => ({
  label: String(i * 2).padStart(2, '0'),
  pct: (i / 12) * 100,
}))

const getAxisMarks = (axisLabels) => {
  const totalLabels = axisLabels.length
  const step = totalLabels > 12 ? Math.ceil(totalLabels / 12) : 1

  return axisLabels
    .map((label, index) => ({ label, pct: (index / totalLabels) * 100 }))
    .filter((_, index) => index % step === 0)
}

export default function TimelineChart({ title = '', subtitle = '', rows = [], axisLabels = null }) {
  const items = normalizeItems(rows)
  const isDailyMode = Array.isArray(axisLabels) && axisLabels.length > 0
  const marks = isDailyMode ? getAxisMarks(axisLabels) : HOURLY_MARKS
  const gridCols = isDailyMode ? axisLabels.length : 12

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
        {items.length === 0 ? (
          <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
            No sessions for this view.
          </div>
        ) : (
          <div className="space-y-4">
            <div className="text-foreground/45 grid grid-cols-[220px_minmax(0,1fr)] gap-3 text-[10px] font-semibold tracking-[0.3em] uppercase">
              <div>Slice</div>
              <div className="relative h-4 overflow-hidden">
                {marks.map((mark, index) => (
                  <span
                    key={index}
                    className="absolute whitespace-nowrap"
                    style={{ left: `${mark.pct}%` }}
                  >
                    {mark.label}
                  </span>
                ))}
              </div>
            </div>

            {items.map((row) => (
              <div key={row.name} className="grid grid-cols-[220px_minmax(0,1fr)] gap-3">
                <div className="pr-3">
                  <p className="text-foreground truncate text-lg font-medium">{row.name}</p>
                  <p className="text-foreground/60 mt-1 text-sm">
                    {formatShortDuration(row.totalSeconds)}
                  </p>
                </div>
                <div className="border-border bg-foreground/[0.03] relative h-14 border">
                  <div className="absolute inset-0 flex">
                    {Array.from({ length: gridCols }).map((_, index) => (
                      <div
                        key={index}
                        className="border-border/40 border-r last:border-r-0"
                        style={{ flex: '1 1 0' }}
                      />
                    ))}
                  </div>
                  {row.segments.map((segment, index) => (
                    <div
                      key={index}
                      className="absolute top-2 bottom-2"
                      style={{
                        left: `${segment.left}%`,
                        width: `${segment.width}%`,
                        background: segment.color,
                      }}
                      title={`${row.name}${segment.date ? ' · ' + segment.date : ''} · ${formatShortDuration(segment.duration)}`}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
