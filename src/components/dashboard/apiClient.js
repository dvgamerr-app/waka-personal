export const detectTimezone = (fallback = 'UTC') => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || fallback
  } catch (_) {
    return fallback
  }
}

export const readRuntimeConfig = () => {
  if (typeof window === 'undefined') {
    return {}
  }
  const runtime = window.__WAKA_DASHBOARD_CONFIG__ || {}
  return Object.fromEntries(
    Object.entries(runtime).filter(([, value]) => value != null && value !== '')
  )
}

export const buildApiUrl = (base, path, params = {}) => {
  const root = base || window.location.origin
  const url = new URL(path, root)
  for (const [key, value] of Object.entries(params)) {
    if (value != null && value !== '') {
      url.searchParams.set(key, String(value))
    }
  }
  return base ? url.toString() : `${url.pathname}${url.search}`
}

export const fetchJson = async ({ base, path, params }) => {
  try {
    const res = await fetch(buildApiUrl(base, path, params))
    if (!res.ok) {
      return { data: null, error: `${path} returned ${res.status}` }
    }
    return { data: await res.json(), error: null }
  } catch (error) {
    return {
      data: null,
      error: `${path} failed: ${error instanceof Error ? error.message : 'request failed'}`,
    }
  }
}
