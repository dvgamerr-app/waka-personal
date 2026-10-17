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

  if (hours > 0) return `${hours}h ${String(minutes).padStart(2, '0')}m`
  if (minutes > 0) return `${minutes}m`
  return `${total % 60}s`
}

export const formatTableDuration = (seconds) => {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  return `${String(hours).padStart(2, '0')}h ${String(minutes).padStart(2, '0')}m`
}

export const normalizeWakaTime = (text) => {
  if (!text) return text
  return text
    .replace(/(\d+)\s*hrs?/gi, (_, n) => `${n}h`)
    .replace(/(\d+)\s*mins?/gi, (_, n) => `${n}m`)
    .replace(/\s+/g, ' ')
    .trim()
}

export const formatPercent = (value) => `${Math.round(Number(value) || 0)}%`

const compactCount = (n, unit, divisor) => {
  const compact = n / divisor
  return `${compact >= 10 ? Math.round(compact) : compact.toFixed(1).replace(/.0$/, '')}${unit}`
}

export const formatCount = (value) => {
  const n = Math.round(Number(value) || 0)
  if (n >= 999_500_000) return compactCount(n, 'B', 1e9)
  if (n >= 999_500) return compactCount(n, 'M', 1e6)
  if (n >= 1000) return compactCount(n, 'K', 1e3)
  return String(n)
}

export const formatSpend = (cents) => {
  const dollars = (cents || 0) / 100
  if (dollars === 0) return '$0'
  if (dollars < 0.01) return '<$0.01'
  return `$${dollars.toFixed(2)}`
}

export const formatLocalDate = (date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export const formatDayLabel = (value) => {
  if (!value) return ''
  const date = new Date(`${value}T00:00:00`)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

export const topItems = (items, limit = 5) => normalizeItems(items).slice(0, limit)

export const buildTrendSeries = (summaries) => {
  const days = normalizeItems(summaries)
  const topProjects = []
  const totalsByProject = new Map()

  days.forEach((day) => {
    normalizeItems(day.projects).forEach((item) => {
      const next = (totalsByProject.get(item.name) || 0) + (Number(item.total_seconds) || 0)
      totalsByProject.set(item.name, next)
    })
  })

  Array.from(totalsByProject.entries())
    .sort((left, right) => right[1] - left[1])
    .slice(0, 5)
    .forEach(([name], index) => {
      topProjects.push({ name, color: palette[index % palette.length] })
    })

  return days.map((day) => {
    const projectMap = new Map()
    normalizeItems(day.projects).forEach((item) => {
      projectMap.set(item.name, Number(item.total_seconds) || 0)
    })

    const segments = topProjects.map((project) => ({
      name: project.name,
      color: project.color,
      seconds: projectMap.get(project.name) || 0,
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
  const editorMap = new Map()
  const categoryMap = new Map()
  const osMap = new Map()
  const aiModelMap = new Map()

  const accumulate = (map, items) => {
    normalizeItems(items).forEach((item) => {
      const name = item.name || item.machine_name_id || 'Unknown'
      const sec = Number(item.total_seconds) || 0
      map.set(name, (map.get(name) || 0) + sec)
    })
  }

  const accumulateProject = (items) => {
    normalizeItems(items).forEach((item) => {
      const name = item.name || 'Unknown'
      const current = projectMap.get(name) || {
        total_seconds: 0,
        ai_additions: 0,
        ai_deletions: 0,
        human_additions: 0,
        human_deletions: 0,
        token_count: 0,
        spend_cents: 0,
      }
      current.total_seconds += Number(item.total_seconds) || 0
      current.ai_additions += Number(item.ai_additions) || 0
      current.ai_deletions += Number(item.ai_deletions) || 0
      current.human_additions += Number(item.human_additions) || 0
      current.human_deletions += Number(item.human_deletions) || 0
      current.token_count += Number(item.token_count) || 0
      current.spend_cents += Number(item.spend_cents) || 0
      projectMap.set(name, current)
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

    accumulateProject(day.projects)
    accumulate(languageMap, day.languages)
    accumulate(machineMap, day.machines)
    accumulate(editorMap, day.editors)
    accumulate(categoryMap, day.categories)
    accumulate(osMap, day.operating_systems)
    normalizeItems(day.ai_models).forEach((item) => {
      const name = item.name || 'Unknown'
      const cur = aiModelMap.get(name) || { total_seconds: 0, ai_additions: 0, ai_deletions: 0 }
      cur.total_seconds += Number(item.total_seconds) || 0
      cur.ai_additions += Number(item.ai_additions) || 0
      cur.ai_deletions += Number(item.ai_deletions) || 0
      aiModelMap.set(name, cur)
    })
  }

  const activeDays = days.filter((d) => (Number(d.grand_total?.total_seconds) || 0) > 0).length
  const dailyAvgSeconds = activeDays > 0 ? Math.round(totalSeconds / activeDays) : 0
  const categoryTotal = Math.max(
    1,
    Array.from(categoryMap.values()).reduce((s, v) => s + v, 0)
  )

  const toRanked = (map, grandTotal) => {
    const total =
      grandTotal ||
      Math.max(
        1,
        Array.from(map.values()).reduce((s, v) => s + v, 0)
      )
    return Array.from(map.entries())
      .map(([name, total_seconds]) => ({
        name,
        total_seconds,
        percent: (total_seconds / total) * 100,
        text: formatShortDuration(total_seconds),
      }))
      .sort((a, b) => b.total_seconds - a.total_seconds)
  }

  const toProjectRanked = (map, grandTotal) => {
    const total = Math.max(1, grandTotal || 0)
    return Array.from(map.entries())
      .map(([name, item]) => {
        const totalChanges =
          Number(item.ai_additions) +
          Number(item.ai_deletions) +
          Number(item.human_additions) +
          Number(item.human_deletions)
        const aiChanges = Number(item.ai_additions) + Number(item.ai_deletions)
        const humanChanges = Number(item.human_additions) + Number(item.human_deletions)
        const changeTotal = Math.max(1, aiChanges + humanChanges)
        return {
          name,
          total_seconds: item.total_seconds,
          percent: (item.total_seconds / total) * 100,
          text: formatShortDuration(item.total_seconds),
          ai_additions: item.ai_additions,
          ai_deletions: item.ai_deletions,
          human_additions: item.human_additions,
          human_deletions: item.human_deletions,
          ai_changes: aiChanges,
          human_changes: humanChanges,
          total_changes: totalChanges,
          ai_percent: (aiChanges / changeTotal) * 100,
          human_percent: (humanChanges / changeTotal) * 100,
          token_count: item.token_count || 0,
          spend_cents: item.spend_cents || 0,
        }
      })
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
    projects: toProjectRanked(projectMap, totalSeconds),
    languages: toRanked(languageMap, totalSeconds),
    machines: toRanked(machineMap, totalSeconds),
    editors: toRanked(editorMap, totalSeconds),
    operatingSystems: toRanked(osMap, totalSeconds),
    aiModels: (() => {
      const entries = Array.from(aiModelMap.entries())
      const totalLines = entries.reduce((s, [, m]) => s + m.ai_additions + m.ai_deletions, 0)
      const totalAiSecs = entries.reduce((s, [, m]) => s + m.total_seconds, 0)
      return entries
        .map(([name, m]) => {
          const lines = m.ai_additions + m.ai_deletions
          const percent =
            totalLines > 0
              ? (lines / totalLines) * 100
              : totalAiSecs > 0
                ? (m.total_seconds / totalAiSecs) * 100
                : 0
          return {
            name,
            total_seconds: m.total_seconds,
            lines,
            ai_additions: m.ai_additions,
            ai_deletions: m.ai_deletions,
            percent,
          }
        })
        .sort((a, b) => (totalLines > 0 ? b.lines - a.lines : b.total_seconds - a.total_seconds))
    })(),
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
