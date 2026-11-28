import { describe, expect, test } from 'vitest'
import { validSourceURL } from '../src/features/collections/sourceForm'
describe('saved source URL boundary without normalization or network access', () => {
  test.each(['https://example.com/news?q=日本語#発表', 'HTTP://EXAMPLE.COM/a%20b', 'http://localhost:8080/', 'https://127.0.0.1', 'https://[::1]:443/', 'https://xn--wgv71a119e.jp/', 'https://example.com./'])('accepts %s', value => { expect(validSourceURL(value)).toBe(true) })
  test.each(['', '/relative', '//example.com', 'https:example.com', 'ftp://example.com', 'javascript:alert(1)', 'https://u:p@example.com', 'https://@example.com', 'https://example.com\\x', 'https://example.com/ a', 'https://example.com/\n', 'https://example.com/\u0085', 'https://example.com/%xx', 'https://example.com/?x=%', 'https://example.com/#%xx', 'https://example.com:', 'https://example.com:0', 'https://example.com:65536', 'https://例.jp/', 'https://-bad.com', 'https://a..b', 'https://[v1.test]', 'https://[fe80::1%25eth0]', 'https://example.com/%+1', 'https://example.com/'+'x'.repeat(2048)])('rejects %s', value => { expect(validSourceURL(value)).toBe(false) })
})
