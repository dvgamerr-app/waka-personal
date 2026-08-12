export const dashboardConfig = {
  apiBase: (import.meta.env.PUBLIC_API_BASE || '').replace(/\/$/, ''),
  timezone: import.meta.env.PUBLIC_APP_TIMEZONE || import.meta.env.APP_TIMEZONE || 'UTC',
}
