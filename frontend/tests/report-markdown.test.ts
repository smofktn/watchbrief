import { afterEach, describe, expect, test, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MarkdownIt from 'markdown-it'
import createDOMPurify from 'dompurify'
import ReportMarkdown from '../src/features/reports/ReportMarkdown.vue'
import { isReportLink } from '../src/features/reports/markdown'

vi.mock('dompurify', async importOriginal => {
  const actual = await importOriginal<typeof import('dompurify')>()
  return { ...actual, default: vi.fn(actual.default) }
})
afterEach(() => { vi.restoreAllMocks() })

const render = (markdown: string) => mount(ReportMarkdown, { props: { markdown } })

describe('saved complete Markdown (distinct from AI fragment validation)', () => {
  test('headings, paragraphs, emphasis, lists, code and source links; no language attributes', () => {
    const app = render('# 見出し\n\n段落 **太字** *斜体* `inline <x>`\n\n- 項目\n\n3. 順序\n\n```html\n<img src=x onerror=bad()>\n```\n\n    indented <code>\n\n---\n\n[出典](https://example.com/news "任意の題")')
    expect(app.get('h1').text()).toBe('見出し')
    expect(app.get('strong').text()).toBe('太字')
    expect(app.get('em').text()).toBe('斜体')
    expect(app.get('ul li').text()).toBe('項目')
    expect(app.get('ol').attributes()).toEqual({ start: '3' })
    expect(app.findAll('code').map(node => node.element.textContent)).toEqual(['inline <x>', '<img src=x onerror=bad()>\n', 'indented <code>\n'])
    expect(app.findAll('code').every(node => Object.keys(node.attributes()).length === 0)).toBe(true)
    expect(app.get('a').attributes()).toEqual({ href: 'https://example.com/news', rel: 'noopener noreferrer', referrerpolicy: 'no-referrer' })
    expect(app.find('hr').exists()).toBe(true)
    expect(app.find('img').exists()).toBe(false)
  })
  test('HTML stays literal, images keep alt text without elements or links, no extension tags', () => {
    const html = '<script>window.reportExecuted=true</script><iframe src="https://external.invalid/x"></iframe><svg onload=bad()><a href="javascript:bad()">x</a></svg>'
    const app = render(html+'\n\n![架空画像](https://external.invalid/image)\n\n![data画像](data:image/png;base64,AAAA)\n\n> 引用\n\n|表|列|\n|-|-|\n|値|値|\n\n~~取消~~\n\nhttps://example.com/plain')
    expect(app.text()).toContain(html)
    expect(app.text()).toContain('架空画像')
    expect(app.find('script,iframe,img,svg,math,object,embed,video,audio,style,table,blockquote,del').exists()).toBe(false)
    expect(app.find('a').exists()).toBe(false)
  })
  test.each(['javascript:alert(1)', 'JaVaScRiPt:alert(1)', 'javascript&#58;alert(1)', 'jav&#x61;script:alert(1)', 'java&#x09;script:alert(1)', 'data:text/html,bad', 'vbscript:bad', 'file:///etc/passwd', 'mailto:a@example.com', 'blob:https://example.com/a', '/relative', '//example.com/path', '#fragment', 'https://user:pass@example.com', 'https://@example.com', 'https://user%40name@example.com', 'https:\\example.com', 'https://example.com/%0A', 'https://example.com/%5cpath', 'https://example.com/%C2%85', 'https://example.com/%zz'])('keeps refused destination as non-clickable text: %s', url => {
    const app = render(`[拒否した出典](<${url}>)`)
    expect(app.text()).toContain('拒否した出典')
    expect(app.find('a[href]').exists()).toBe(false)
  })
  test.each(['HTTPS://EXAMPLE.COM/path?q=日本語#part', 'http://localhost:8080/', 'https://[::1]/', 'https://例.jp/発表', 'https://example.com/a%20b', 'https://example.com/'+ 'x'.repeat(3000)])('accepts absolute HTTP(S) without transferring source-input length rules', url => {
    const app = render(`[許可した出典](${url})`)
    const link = app.get('a')
    expect(isReportLink(link.attributes('href') ?? '')).toBe(true)
    expect(link.attributes('target')).toBeUndefined()
    expect(link.attributes('ping')).toBeUndefined()
    expect(link.attributes('referrerpolicy')).toBe('no-referrer')
  })
  test('sanitizer independently restricts tags, attributes and URLs even if parser output changes', () => {
    vi.spyOn(MarkdownIt.prototype, 'render').mockReturnValue('<p id="location" name="x" href="https://example.com" start="2" data-x="x" aria-label="x" style="color:red" onclick="bad()">内容<img src="https://external.invalid/i"></p><ol start="3" href="https://example.com"><li>x</li></ol><a href="javascript:bad()" ping="https://external.invalid/p" target="_blank">拒否</a><a href="https://example.com/" rel="opener" referrerpolicy="unsafe-url" start="2">許可</a><svg><a>x</a></svg>')
    const app = render('ignored')
    expect(app.get('p').attributes()).toEqual({})
    expect(app.get('ol').attributes()).toEqual({ start: '3' })
    expect(app.find('img,svg,script').exists()).toBe(false)
    const anchors = app.findAll('a')
    expect(anchors[0].attributes()).toEqual({})
    expect(anchors[1].attributes()).toEqual({ href: 'https://example.com/', rel: 'noopener noreferrer', referrerpolicy: 'no-referrer' })
    expect(app.text()).toContain('拒否')
  })
  test.each(['unsupported', 'factory', 'render', 'sanitize'])('fails closed with interpolation for %s', mode => {
    if (mode === 'unsupported') { vi.mocked(createDOMPurify).mockReturnValueOnce({ isSupported: false } as ReturnType<typeof createDOMPurify>) }
    if (mode === 'factory') { vi.mocked(createDOMPurify).mockImplementationOnce(() => { throw new Error('private detail') }) }
    if (mode === 'render') { vi.spyOn(MarkdownIt.prototype, 'render').mockImplementationOnce(() => { throw new Error('private detail') }) }
    if (mode === 'sanitize') { vi.mocked(createDOMPurify).mockReturnValueOnce({ isSupported: true, addHook: vi.fn(), sanitize: () => { throw new Error('private detail') } } as unknown as ReturnType<typeof createDOMPurify>) }
    const markdown = '<img src=x onerror=bad()>\n# 保存本文'
    const app = render(markdown)
    expect(app.get('.report-text').text()).toBe(markdown)
    expect(app.find('img,h1').exists()).toBe(false)
    expect(app.text()).toContain('整形表示を利用できない')
    expect(app.text()).not.toContain('private detail')
  })
  test('replaces derived HTML when its only input changes', async () => {
    const app = render('# 古い本文')
    await app.setProps({ markdown: '新しい本文' })
    expect(app.find('h1').exists()).toBe(false)
    expect(app.text()).toBe('新しい本文')
    await app.setProps({ markdown: '' })
    expect(app.text()).toBe('')
  })
})
