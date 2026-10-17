// Writes PUBLIC_TRACEWAY_DSN into the built page's meta tag at deploy time.
// Without the variable the tag stays empty and the page loads no telemetry.
const META = '<meta name="traceway-dsn" content="" />'

export const injectDsn = (html, dsn) => {
  if (!html.includes(META)) throw new Error('traceway-dsn meta tag not found')
  if (!dsn) return html
  if (!dsn.includes('@')) throw new Error('PUBLIC_TRACEWAY_DSN must look like <token>@<url>')
  const escaped = dsn.replace(/[&"<>]/g, (c) => `&#${c.charCodeAt(0)};`)
  // A replacer function keeps `$&`, `$1` and similar sequences in the DSN literal.
  return html.replace(META, () => `<meta name="traceway-dsn" content="${escaped}" />`)
}

if (import.meta.main) {
  const file = process.argv[2]
  const html = await Bun.file(file).text()
  await Bun.write(file, injectDsn(html, (process.env.PUBLIC_TRACEWAY_DSN || '').trim()))
}
