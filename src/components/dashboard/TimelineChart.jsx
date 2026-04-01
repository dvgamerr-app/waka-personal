import { normalizeItems, formatShortDuration } from './dashboardUtils.js'

const HOUR_MARKS = Array.from({ length: 12 }, (_, i) => i)
const GRID_DIVIDERS = Array.from({ length: 12 })

export default function TimelineChart({ title = '', subtitle = '', rows = [] }) {
  const items = normalizeItems(rows)

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
            <div className="grid grid-cols-12 gap-0">
              {HOUR_MARKS.map((i) => (
                <span key={i}>{String(i * 2).padStart(2, '0')}</span>
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
                <div className="absolute inset-0 grid grid-cols-12">
                  {GRID_DIVIDERS.map((_, i) => (
                    <div key={i} className="border-border/60 border-r last:border-r-0" />
                  ))}
                </div>
                {row.segments.map((segment, i) => (
                  <div
                    key={i}
                    className="border-background/30 absolute top-2 bottom-2 border"
                    style={{
                      left: `${segment.left}%`,
                      width: `${segment.width}%`,
                      background: segment.color,
                    }}
                    title={`${row.name} · ${formatShortDuration(segment.duration)}`}
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
