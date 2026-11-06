// Local-only static entry and same-origin /api proxy. Production uses nginx (#21).
import { createServer, request } from 'node:http'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const root = fileURLToPath(new URL('../dist/', import.meta.url))
const assets = new Map([
  ['/', ['index.html', 'text/html; charset=utf-8']], ['/app.js', ['app.js', 'text/javascript']],
  ['/client.mjs', ['client.mjs', 'text/javascript']], ['/vue.js', ['vue.js', 'text/javascript']],
  ['/style.css', ['style.css', 'text/css']],
])
const server = createServer(async (req, res) => {
  // No access log: URLs, headers, bodies and proxy errors may contain secrets.
  if (req.headers.host !== '127.0.0.1:5173' && req.headers.host !== 'localhost:5173') {
    res.writeHead(403).end(); return
  }
  if (req.url.startsWith('/api/')) {
    const upstream = request({ hostname:'127.0.0.1', port:8080, path:req.url, method:req.method,
      headers:{ ...req.headers, host:'127.0.0.1:8080' }, timeout:15000 }, response => {
      res.writeHead(response.statusCode, response.headers)
      response.pipe(res)
    })
    upstream.on('timeout', () => upstream.destroy())
    upstream.on('error', () => {
      if (!res.headersSent) res.writeHead(502, { 'Content-Type':'application/json', 'Cache-Control':'no-store' })
      res.end('{"error":"internal_error"}')
    })
    req.on('aborted', () => upstream.destroy())
    req.pipe(upstream)
    return
  }
  const asset = assets.get(req.url)
  if (!asset || (req.method !== 'GET' && req.method !== 'HEAD')) { res.writeHead(404).end(); return }
  try {
    const body = await readFile(`${root}${asset[0]}`)
    res.writeHead(200, { 'Content-Type':asset[1], 'Cache-Control':'no-store',
      'Content-Security-Policy':"default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'",
      'X-Content-Type-Options':'nosniff', 'Referrer-Policy':'no-referrer' })
    res.end(req.method === 'HEAD' ? undefined : body)
  } catch { res.writeHead(503).end('Run npm run build first.') }
})
server.on('error', () => {
  console.error(JSON.stringify({ level:'ERROR', operation:'frontend_start', result:'listen_failed' }))
  process.exitCode = 1
})
server.listen(5173, '127.0.0.1', () => { console.log('WatchBrief: http://127.0.0.1:5173') })
