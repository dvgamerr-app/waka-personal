import { useEffect, useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'

export default function YearPicker() {
  const currentYear = new Date().getFullYear()
  const [year, setYear] = useState(() => {
    if (typeof window === 'undefined') return currentYear
    const param = new URLSearchParams(window.location.search).get('year')
    return Number(param) || currentYear
  })

  const navigate = (next) => {
    const url = new URL(window.location.href)
    url.searchParams.set('year', String(next))
    history.pushState(null, '', url.toString())
    window.dispatchEvent(new CustomEvent('year-change', { detail: next }))
    setYear(next)
  }

  useEffect(() => {
    const handler = (e) => setYear(e.detail)
    window.addEventListener('year-change', handler)
    return () => window.removeEventListener('year-change', handler)
  }, [])

  return (
    <div className="flex items-center border border-zinc-800 text-[11px] tracking-[0.2em] text-zinc-400 uppercase">
      <button
        type="button"
        className="flex h-8 w-8 items-center justify-center transition-colors hover:text-sky-300"
        title="Previous year"
        onClick={() => navigate(year - 1)}
      >
        <ChevronLeft size={14} />
      </button>
      <span className="border-x border-zinc-800 px-3 py-1.5 font-mono text-zinc-300">{year}</span>
      <button
        type="button"
        className="flex h-8 w-8 items-center justify-center transition-colors hover:text-sky-300 disabled:cursor-not-allowed disabled:text-zinc-700"
        title={year >= currentYear ? `${year} is the latest year` : 'Next year'}
        disabled={year >= currentYear}
        onClick={() => navigate(Math.min(currentYear, year + 1))}
      >
        <ChevronRight size={14} />
      </button>
    </div>
  )
}
