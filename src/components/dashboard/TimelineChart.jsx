import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const HOURLY_MARKS = Array.from({ length: 12 }, (_, i) => ({
  label: String(i * 2).padStart(2, '0'),
  pct: (i / 12) * 100,
}))

const getAxisMarks = (axisLabels) => {
  const n = axisLabels.length
  const step = n > 12 ? Math.ceil(n / 12) : 1
  return axisLabels
    .map((label, i) => ({ label, pct: (i / n) * 100 }))
    .filter((_, i) => i % step === 0)
}

export default function TimelineChart({ title = '', subtitle = '', rows = [], axisLabels = null }) {
  const items = normalizeItems(rows)
  const isDailyMode = Array.isArray(axisLabels) && axisLabels.length > 0
  const marks = isDailyMode ? getAxisMarks(axisLabels) : HOURLY_MARKS
  const gridCols = isDailyMode ? axisLabels.length : 12

  return (
    <section className="border-border bg-background/70 border p-5 backdrop-blur-sm">
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
            {title}
          </p>
          {subtitle && <p className="text-foreground/65 mt-2 text-sm">{subtitle}</p>}
        </div>
      </div>

      {items.length === 0 ? (
        <div className="border-border text-foreground/55 border border-dashed p-8 text-sm">
          No sessions for this view.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="text-foreground/45 grid grid-cols-[220px_minmax(0,1fr)] gap-3 text-[10px] font-semibold tracking-[0.3em] uppercase">
            <div>Slice</div>
            <div className="relative h-4 overflow-hidden">
              {marks.map((m, i) => (
                <span
                  key={i}
                  className="absolute whitespace-nowrap"
                  style={{ left: `${m.pct}%` }}
                >
                  {m.label}
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
                <div
                  className="absolute inset-0 flex"
                  style={{ '--grid-cols': gridCols }}
                >
                  {Array.from({ length: gridCols }).map((_, i) => (
                    <div
                      key={i}
                      className="border-border/40 border-r last:border-r-0"
                      style={{ flex: '1 1 0' }}
                    />
                  ))}
                </div>
                {row.segments.map((segment, i) => (
                  <div
                    key={i}
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
    </section>
  )
}
