import { Card, CardContent } from '@/components/ui/card'

export default function StatCard({ label = '', value = '', note = '', accent = '#38bdf8' }) {
  return (
    <Card className="border-border/80 bg-background/75 shadow-none backdrop-blur-sm">
      <CardContent className="grid gap-3 p-4">
        <div className="h-1 w-16" style={{ background: accent }} />
        <p className="text-foreground/55 text-[10px] font-semibold tracking-[0.35em] uppercase">
          {label}
        </p>
        <p className="text-foreground text-2xl font-semibold tracking-tight">{value}</p>
        {note && <p className="text-foreground/65 mt-2 text-sm">{note}</p>}
      </CardContent>
    </Card>
  )
}
