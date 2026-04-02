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

export const buildTimelineRows = (durations, key, start, end) => {  const windowStart = new Date(start || 0).getTime() / 1000
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

export const buildDailyBreakdownRows = (summaries, key) => {
  const days = normalizeItems(summaries)
  const totalDays = Math.max(1, days.length)
  const grouped = new Map()

  days.forEach((day, dayIndex) => {
    normalizeItems(day[key]).forEach((item) => {
      const name = item.name || 'Unknown'
      const seconds = Number(item.total_seconds) || 0
      if (!grouped.has(name)) grouped.set(name, { name, totalSeconds: 0, rawSegments: [] })
      const row = grouped.get(name)
      row.totalSeconds += seconds
      if (seconds > 0) {
        row.rawSegments.push({
          dayIndex,
          left: (dayIndex / totalDays) * 100,
          width: Math.max(0.8, (1 / totalDays) * 100 - 0.4),
          duration: seconds,
          date: day.range?.date,
        })
      }
    })
  })

  return Array.from(grouped.values())
    .sort((a, b) => b.totalSeconds - a.totalSeconds)
    .slice(0, 7)
    .map((row, i) => ({
      name: row.name,
      totalSeconds: row.totalSeconds,
      segments: row.rawSegments.map((seg) => ({ ...seg, color: palette[i % palette.length] })),
    }))
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

export const computeRangeStats = (summaries) => {
  const days = normalizeItems(summaries)
  if (days.length === 0) return null

  let totalSeconds = 0
  let aiAdditions = 0
  let aiDeletions = 0
  let humanAdditions = 0
  let humanDeletions = 0
  let bestDay = null
  const projectMap = new Map()
  const languageMap = new Map()
  const machineMap = new Map()
  const categoryMap = new Map()

  const accumulate = (map, items) => {
    normalizeItems(items).forEach((item) => {
      const name = item.name || item.machine_name_id || 'Unknown'
      const sec = Number(item.total_seconds) || 0
      map.set(name, (map.get(name) || 0) + sec)
    })
  }

  for (const day of days) {
    const ts = Number(day.grand_total?.total_seconds) || 0
    totalSeconds += ts
    aiAdditions += Number(day.grand_total?.ai_additions) || 0
    aiDeletions += Number(day.grand_total?.ai_deletions) || 0
    humanAdditions += Number(day.grand_total?.human_additions) || 0
    humanDeletions += Number(day.grand_total?.human_deletions) || 0

    if (!bestDay || ts > bestDay.totalSeconds) {
      bestDay = {
        date: day.range?.date,
        text: day.grand_total?.text || formatShortDuration(ts),
        totalSeconds: ts,
      }
    }

    accumulate(projectMap, day.projects)
    accumulate(languageMap, day.languages)
    accumulate(machineMap, day.machines)
    accumulate(categoryMap, day.categories)
  }

  const activeDays = days.filter((d) => (Number(d.grand_total?.total_seconds) || 0) > 0).length
  const dailyAvgSeconds = activeDays > 0 ? Math.round(totalSeconds / activeDays) : 0
  const categoryTotal = Math.max(1, Array.from(categoryMap.values()).reduce((s, v) => s + v, 0))

  const toRanked = (map, grandTotal) => {
    const total = grandTotal || Math.max(1, Array.from(map.values()).reduce((s, v) => s + v, 0))
    return Array.from(map.entries())
      .map(([name, total_seconds]) => ({
        name,
        total_seconds,
        percent: (total_seconds / total) * 100,
        text: formatShortDuration(total_seconds),
      }))
      .sort((a, b) => b.total_seconds - a.total_seconds)
  }

  return {
    totalSeconds,
    humanReadableTotal: formatShortDuration(totalSeconds),
    dailyAvgSeconds,
    humanReadableDailyAvg: formatShortDuration(dailyAvgSeconds),
    activeDays,
    totalDays: days.length,
    bestDay,
    aiAdditions,
    aiDeletions,
    humanAdditions,
    humanDeletions,
    projects: toRanked(projectMap, totalSeconds),
    languages: toRanked(languageMap, totalSeconds),
    machines: toRanked(machineMap, totalSeconds),
    categories: Array.from(categoryMap.entries())
      .map(([name, total_seconds]) => ({
        name,
        total_seconds,
        percent: (total_seconds / categoryTotal) * 100,
        text: formatShortDuration(total_seconds),
      }))
      .sort((a, b) => b.total_seconds - a.total_seconds),
  }
}
