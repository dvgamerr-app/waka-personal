export const palette = [
  '#38bdf8',
  '#60a5fa',
  '#818cf8',
  '#22c55e',
  '#f59e0b',
  '#ef4444',
  '#14b8a6',
  '#eab308',
]

export const normalizeItems = (items) => (Array.isArray(items) ? items : [])

export const formatShortDuration = (seconds) => {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m`
  return `${total % 60}s`
}

export const formatPercent = (value) => `${Math.round(Number(value) || 0)}%`

export const formatDayLabel = (value) => {
  if (!value) return ''
  const date = new Date(`${value}T00:00:00`)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

export const topItems = (items, limit = 5) => normalizeItems(items).slice(0, limit)

export const enrichItemsWithColor = (items, limit = 6) =>
  topItems(items, limit).map((item, index) => ({
    ...item,
    color: palette[index % palette.length],
  }))

export const buildTrendSeries = (summaries) => {
  const days = normalizeItems(summaries)
  const topCategories = []
  const totalsByCategory = new Map()

  days.forEach((day) => {
    normalizeItems(day.categories).forEach((item) => {
      const next = (totalsByCategory.get(item.name) || 0) + (Number(item.total_seconds) || 0)
      totalsByCategory.set(item.name, next)
    })
  })

  Array.from(totalsByCategory.entries())
    .sort((left, right) => right[1] - left[1])
    .slice(0, 4)
    .forEach(([name], index) => {
      topCategories.push({ name, color: palette[index % palette.length] })
    })

  return days.map((day) => {
    const categoryMap = new Map()
    normalizeItems(day.categories).forEach((item) => {
      categoryMap.set(item.name, Number(item.total_seconds) || 0)
    })

    const segments = topCategories.map((category) => ({
      name: category.name,
      color: category.color,
      seconds: categoryMap.get(category.name) || 0,
    }))

    const usedSeconds = segments.reduce((sum, segment) => sum + segment.seconds, 0)
    const totalSeconds = Number(day.grand_total?.total_seconds) || 0
    if (totalSeconds > usedSeconds) {
      segments.push({
        name: 'Other',
        color: '#334155',
        seconds: totalSeconds - usedSeconds,
      })
    }

    return {
      date: day.range?.date,
      label: formatDayLabel(day.range?.date),
      totalSeconds,
      totalText: day.grand_total?.text || formatShortDuration(totalSeconds),
      segments,
    }
  })
}

export const buildTimelineRows = (durations, key, start, end) => {
  const windowStart = new Date(start || 0).getTime() / 1000
  const windowEnd = new Date(end || 0).getTime() / 1000
  const windowSize = Math.max(1, windowEnd - windowStart)
  const grouped = new Map()
  normalizeItems(durations).forEach((item) => {
    const name = item[key] || 'Unknown'
    const time = Number(item.time) || 0
    const duration = Number(item.duration) || 0
    const left = Math.max(0, Math.min(100, ((time - windowStart) / windowSize) * 100))
    const width = Math.max(0.6, (duration / windowSize) * 100)
    const current = grouped.get(name) || { name, totalSeconds: 0, segments: [] }
    current.totalSeconds += duration
    current.segments.push({
      left,
      width,
      duration,
      color: palette[grouped.size % palette.length],
    })
    grouped.set(name, current)
  })

  return Array.from(grouped.values())
    .sort((left, right) => right.totalSeconds - left.totalSeconds)
    .slice(0, 7)
}

export const buildDeltaSeries = (summaries) =>
  normalizeItems(summaries).map((day) => ({
    date: day.range?.date,
    label: formatDayLabel(day.range?.date),
    aiAdditions: Number(day.grand_total?.ai_additions) || 0,
    aiDeletions: Number(day.grand_total?.ai_deletions) || 0,
    humanAdditions: Number(day.grand_total?.human_additions) || 0,
    humanDeletions: Number(day.grand_total?.human_deletions) || 0,
  }))

export const maxDeltaValue = (series) =>
  Math.max(
    1,
    ...normalizeItems(series).flatMap((item) => [
      item.aiAdditions,
      item.aiDeletions,
      item.humanAdditions,
      item.humanDeletions,
    ])
  )
