export default function StatCard({ label = '', value = '', note = '', accent = '#38bdf8' }) {
  return (
    <article className="border-border bg-background/70 border p-4 backdrop-blur-sm">
      <div className="mb-3 h-1 w-16" style={{ background: accent }}></div>
      <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
        {label}
      </p>
      <p className="text-foreground mt-3 text-2xl font-semibold tracking-tight">{value}</p>
      {note && <p className="text-foreground/65 mt-2 text-sm">{note}</p>}
    </article>
  )
}
