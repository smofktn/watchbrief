import MarkdownIt from 'markdown-it'
import createDOMPurify from 'dompurify'

// This is a display-time policy, not InformationSource input validation or #20
// fetch/SSRF protection. Never infer a scheme or resolve against the app URL.
export const isReportLink = (value: string): boolean => {
  if (!/^https?:\/\//i.test(value) || /[\p{White_Space}\p{Cc}\\]/u.test(value)
    || /%(?:0[0-9a-f]|1[0-9a-f]|7f|5c)/i.test(value) || /%(?![0-9a-f]{2})/i.test(value)) { return false }
  try {
    if (/[\p{Cc}\\]/u.test(decodeURIComponent(value))) { return false }
    const authority = /^https?:\/\/([^/?#]*)/i.exec(value)?.[1]
    if (!authority || authority.includes('@')) { return false }
    const url = new URL(value)
    return (url.protocol === 'http:' || url.protocol === 'https:') && Boolean(url.hostname)
      && !url.username && !url.password
  } catch { return false }
}

// Only this pipeline may produce HTML for ReportMarkdown. Return null on any
// unsupported sanitizer or conversion failure; callers must use text interpolation.
export const renderReportMarkdown = (markdown: string): string | null => {
  try {
    const purifier = createDOMPurify(window)
    if (!purifier.isSupported) { return null }
    const parser = new MarkdownIt('commonmark', { html: false, linkify: false, typographer: false })
    parser.disable('blockquote')
    const validate = parser.validateLink.bind(parser)
    const normalize = parser.normalizeLink.bind(parser)
    // Check before normalization can encode whitespace/backslashes, then check
    // the decoded/normalized destination again during parsing and sanitization.
    parser.normalizeLink = value => isReportLink(value) ? normalize(value) : ''
    parser.validateLink = value => validate(value) && isReportLink(value)
    parser.renderer.rules.image = (tokens, index) => parser.utils.escapeHtml(tokens[index]!.content)
    parser.renderer.rules.fence = (tokens, index) => `<pre><code>${parser.utils.escapeHtml(tokens[index]!.content)}</code></pre>\n`
    purifier.addHook('afterSanitizeAttributes', node => {
      const allowed = node.nodeName === 'A' ? ['href', 'rel', 'referrerpolicy'] : node.nodeName === 'OL' ? ['start'] : []
      for (const attribute of Array.from(node.attributes)) {
        if (!allowed.includes(attribute.name)) { node.removeAttribute(attribute.name) }
      }
      if (node.nodeName !== 'A') { return }
      node.removeAttribute('rel')
      node.removeAttribute('referrerpolicy')
      const href = node.getAttribute('href')
      if (!href || !isReportLink(href)) { node.removeAttribute('href') }
      else {
        node.setAttribute('rel', 'noopener noreferrer')
        node.setAttribute('referrerpolicy', 'no-referrer')
      }
    })
    const html = purifier.sanitize(parser.render(markdown), {
      ALLOWED_TAGS: ['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'strong', 'em', 'ul', 'ol', 'li', 'pre', 'code', 'a', 'br', 'hr'],
      ALLOWED_ATTR: ['href', 'rel', 'referrerpolicy', 'start'],
      ALLOW_DATA_ATTR: false,
      ALLOW_ARIA_ATTR: false,
      RETURN_TRUSTED_TYPE: false,
    })
    return typeof html === 'string' ? html : null
  } catch { return null }
}
