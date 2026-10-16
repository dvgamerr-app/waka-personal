import { describe, expect, test } from 'bun:test'

import { injectDsn } from './inject-dsn.js'

const page = '<head><meta name="traceway-dsn" content="" /></head>'

describe('injectDsn', () => {
  test('leaves the page untouched without a DSN', () => {
    expect(injectDsn(page, '')).toBe(page)
  })

  test('writes the DSN into the meta tag', () => {
    expect(injectDsn(page, 'abc@https://otel.example.com/api/report')).toContain(
      'content="abc@https://otel.example.com/api/report"'
    )
  })

  test('escapes markup characters', () => {
    expect(injectDsn(page, 'a"b@https://x/<y>')).toContain(
      'content="a&#34;b@https://x/&#60;y&#62;"'
    )
  })

  test('rejects a value without a token part', () => {
    expect(() => injectDsn(page, 'https://otel.example.com/api/report')).toThrow()
  })

  test('fails loudly when the meta tag is missing', () => {
    expect(() => injectDsn('<head></head>', 'abc@https://x')).toThrow()
  })
})
