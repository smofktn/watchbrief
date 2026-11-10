// Private fixture path is the only argument. Never print browser bodies or credentials.
import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { readFile, writeFile, mkdtemp, rm, readdir } from 'node:fs/promises'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

const root = new URL('../', import.meta.url).pathname
const fixture = JSON.parse(await readFile(process.argv[2], 'utf8'))
const { origin, api_port: apiPort, project } = fixture
const port = Number(new URL(origin).port)
const createVite = async () => {
  const { createServer } = await import(pathToFileURL(join(root, 'frontend/node_modules/vite/dist/node/index.js')).href)
  return createServer({ root: join(root, 'frontend'), configFile: join(root, 'frontend/vite.config.ts'), logLevel: 'silent', server: { host: '127.0.0.1', port, strictPort: true, proxy: { '/api': { target: `http://127.0.0.1:${apiPort}`, changeOrigin: false } } } })
}
let vite
let browser
let browserExited
let profile
let timeout
let phase = 'startup'
const issued = []
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))
// Only the detached process group created by this runner is signalled.
const waitForBrowserExit = async (exited, milliseconds) => {
  let timer
  try {
    return await Promise.race([exited.then(() => true), new Promise(resolve => { timer = setTimeout(() => resolve(false), milliseconds) })])
  } finally {
    clearTimeout(timer)
  }
}
const terminateBrowser = async (child, exited) => {
  if (!child?.pid) { return }
  const signalGroup = signal => {
    try { process.kill(-child.pid, signal) } catch (error) { if (error.code !== 'ESRCH') { throw error } }
  }
  signalGroup('SIGTERM')
  await waitForBrowserExit(exited, 2000)
  // Also reap descendants if the main browser has already exited.
  signalGroup('SIGKILL')
  if (!await waitForBrowserExit(exited, 2000)) {
    child.unref()
    for (const stream of child.stdio) { stream?.destroy() }
    throw new Error('Browser cleanup did not finish')
  }
}
const db = action => {
  const result = spawnSync('docker', ['compose', ...action], { encoding: 'utf8', timeout: 120000 })
  assert.equal(result.status, 0, 'Disposable database operation failed')
}
const stopDB = () => db(['stop', 'postgres'])
const startDB = () => db(['up', '-d', '--wait', 'postgres'])
try {
  phase = 'project-identity'
  assert(/^watchbrief-verify-[a-f0-9]{16}$/.test(project))
  assert.equal(process.env.COMPOSE_PROJECT_NAME, project)
  phase = 'vite-startup'
  vite = await createVite()
  await vite.listen()
  let binary = process.env.CHROMIUM_BIN
  if (!binary) {
    for (const command of ['chromium', 'chromium-browser', 'google-chrome']) {
      const found = spawnSync('which', [command], { encoding: 'utf8' })
      if (found.status === 0) { binary = found.stdout.trim(); break }
    }
  }
  if (!binary) {
    const cache = join(process.env.HOME, '.cache/ms-playwright')
    const directories = await readdir(cache).catch(() => [])
    const directory = directories.filter(name => /^chromium-\d+$/.test(name)).sort().at(-1)
    if (directory) { binary = join(cache, directory, 'chrome-linux64/chrome') }
  }
  assert(binary, 'Install Chromium or set CHROMIUM_BIN before verification')
  profile = await mkdtemp('/tmp/watchbrief-issue17-browser-')
  browser = spawn(binary, ['--headless', '--no-sandbox', '--disable-gpu', '--remote-debugging-pipe', `--user-data-dir=${profile}`], { detached: true, stdio: ['ignore', 'ignore', 'ignore', 'pipe', 'pipe'], env: { ...process.env, ...(process.env.BROWSER_LIBRARY_PATH ? { LD_LIBRARY_PATH: process.env.BROWSER_LIBRARY_PATH } : {}) } })
  browserExited = new Promise(resolve => { browser.once('exit', resolve); browser.once('error', resolve) })
  const pending = new Map()
  const runtimeErrors = []
  let browserFailed = false
  const failBrowser = () => {
    browserFailed = true
    for (const callbacks of pending.values()) { callbacks.reject(new Error('Browser transport closed')) }
    pending.clear()
  }
  browser.stdio[3].on('error', failBrowser)
  browser.stdio[4].on('error', failBrowser)
  browser.on('exit', failBrowser)
  browser.on('error', failBrowser)
  let counter = 0
  let loadCount = 0
  let buffer = ''
  browser.stdio[4].on('data', data => {
    buffer += data
    let index
    while ((index = buffer.indexOf('\0')) >= 0) {
      const message = JSON.parse(buffer.slice(0, index))
      buffer = buffer.slice(index + 1)
      if (message.id) {
        const callbacks = pending.get(message.id)
        pending.delete(message.id)
        if (!callbacks) { continue }
        if (message.error) { callbacks.reject(new Error('CDP command failed')) } else { callbacks.resolve(message.result) }
      } else if (message.method === 'Page.loadEventFired') {
        loadCount++
      } else if (message.method === 'Runtime.exceptionThrown') {
        runtimeErrors.push(true)
      }
    }
  })
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    if (browserFailed) { reject(new Error('Browser transport closed')); return }
    const id = ++counter
    pending.set(id, { resolve, reject })
    browser.stdio[3].write(JSON.stringify({ id, method, params, sessionId }) + '\0')
  })
  timeout = setTimeout(failBrowser, 180000)
  const { targetId } = await send('Target.createTarget', { url: 'about:blank' })
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
  const call = (method, params) => send(method, params, sessionId)
  const evaluate = async expression => {
    const result = await call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
    assert(!result.exceptionDetails, 'Browser evaluation failed')
    return result.result.value
  }
  const waitFor = async condition => {
    await evaluate(`new Promise((resolve,reject)=>{let n=0;const timer=setInterval(()=>{if(${condition}){clearInterval(timer);resolve(true)}else if(++n>350){clearInterval(timer);reject('timeout')}},50)})`)
  }
  const reload = async () => {
    const before = loadCount
    await call('Page.reload')
    for (let n = 0; n < 100 && loadCount === before; n++) { await delay(50) }
    assert(loadCount > before, 'Page reload timed out')
  }
  const navigate = async path => {
    const before = loadCount
    await call('Page.navigate', { url: origin + path })
    for (let n = 0; n < 100 && loadCount === before; n++) { await delay(50) }
    assert(loadCount > before, 'Page navigation timed out')
  }
  const click = async text => evaluate(`[...document.querySelectorAll('button')].find(button=>button.textContent===${JSON.stringify(text)}).click()`)
  const hasButton = text => `[...document.querySelectorAll('button')].some(button=>button.textContent===${JSON.stringify(text)})`
  const cookies = async () => (await call('Network.getCookies', { urls: [origin] })).cookies
  const token = async () => (await cookies()).find(cookie => cookie.name === 'watchbrief_session')?.value
  const submit = async () => evaluate(`for(const [id,value] of [['login-id',${JSON.stringify(fixture.login_id)}],['password',${JSON.stringify(fixture.password)}]]){const element=document.getElementById(id);element.value=value;element.dispatchEvent(new Event('input',{bubbles:true}));}document.querySelector('form').requestSubmit()`)
  const clearPrivate = async () => {
    assert(await evaluate(`!document.querySelector('main').textContent.includes(${JSON.stringify(fixture.login_id)}) && !document.querySelector('.report-body') && !document.querySelector('#current-user-title')`), 'Private data retained after failed authentication/logout')
  }
  await call('Page.enable')
  await call('Runtime.enable')
  await call('Network.enable')
  phase = 'protected-return'
  await navigate('/reports/' + fixture.ids.report)
  await waitFor("document.querySelector('#password') !== null")
  assert.equal(await evaluate('location.pathname+location.search'), '/login?return_to=' + encodeURIComponent('/reports/' + fixture.ids.report))
  phase = 'login-outage'
  stopDB()
  await submit()
  await waitFor("document.querySelector('form [role=status]')?.textContent === '処理に失敗しました。時間をおいて再度お試しください'")
  assert(!(await token()), 'Failed login created Cookie')
  phase = 'login-recovery'
  startDB()
  await submit()
  await waitFor("document.querySelector('.report-body') !== null")
  assert.equal(await evaluate('location.pathname'), '/reports/' + fixture.ids.report)
  assert(await evaluate("document.querySelector('.report-body').textContent.includes('<script>window.reportExecuted = true</script>') && window.reportExecuted === undefined && !document.querySelector('.report-body script')"))
  issued.push(await token())
  phase = 'settings-outage'
  // Hold a real successful identity response, stop DB, then release it so the
  // subsequent real settings SQL fails. No response status/body is fabricated.
  await evaluate("window.baseFetch=window.fetch.bind(window);window.pauseNextMe=true;window.fetch=async(...args)=>{const response=await window.baseFetch(...args);if(args[0]==='/api/me'&&window.pauseNextMe){window.pauseNextMe=false;return new Promise(resolve=>{window.releaseMe=()=>resolve(response)})}return response}")
  await evaluate("document.querySelector('nav a[href=\"/collections\"]').click()")
  await waitFor("typeof window.releaseMe === 'function'")
  stopDB()
  await evaluate("window.releaseMe();window.releaseMe=undefined")
  await waitFor(hasButton('設定を再試行'))
  assert(!(await evaluate("document.querySelector('main').textContent.includes('架空コレクション')")))
  startDB()
  await click('設定を再試行')
  await waitFor("document.querySelector('main').textContent.includes('架空コレクション')")
  phase = 'report-outage'
  await evaluate("window.pauseNextMe=true;document.querySelector('nav a[href=\"/reports\"]').click()")
  await waitFor("typeof window.releaseMe === 'function'")
  stopDB()
  await evaluate("window.releaseMe();window.releaseMe=undefined")
  await waitFor(hasButton('Reportを再試行'))
  assert(!(await evaluate("document.querySelector('main').textContent.includes('保存Report確認用（架空）')")))
  startDB()
  await click('Reportを再試行')
  await waitFor("document.querySelector('main').textContent.includes('保存Report確認用（架空）')")
  phase = 'me-outage'
  stopDB()
  await reload()
  await waitFor(hasButton('再試行'))
  await clearPrivate()
  startDB()
  await click('再試行')
  await waitFor("document.querySelector('#current-user-title') !== null")
  const oldToken = await token()
  assert(oldToken)
  phase = 'logout-outage'
  stopDB()
  await click('ログアウト')
  await waitFor("document.querySelector('[role=alert]')?.textContent === 'ログアウトを確認できません。再試行してください。'")
  await clearPrivate()
  assert.equal(await token(), oldToken, 'Failed logout changed Cookie')
  startDB()
  await click('再試行')
  await waitFor("document.querySelector('#password') !== null")
  assert(!(await token()), 'Successful retry retained Cookie')
  const rejected = await fetch(`http://127.0.0.1:${apiPort}/api/me`, { headers: { Cookie: 'watchbrief_session=' + oldToken } })
  assert.equal(rejected.status, 401)
  phase = 'unsafe-return'
  await navigate('/login?return_to=https%3A%2F%2Fexternal.invalid%2F')
  await waitFor("document.querySelector('#password') !== null")
  await submit()
  await waitFor("document.querySelector('#current-user-title') !== null")
  assert.equal(await evaluate('location.pathname'), '/me')
  issued.push(await token())
  await click('ログアウト')
  await waitFor("document.querySelector('#password') !== null")
  if (fixture.verify_creation) {
    phase = 'collection-creation'
    const { verifyCreation } = await import('./verify-collection-creation-browser.mjs')
    issued.push(await verifyCreation({ call, evaluate, waitFor, navigate, click, submit, fixture, stopDB, startDB, token }))
  }
  assert.equal(runtimeErrors.length, 0)
  assert(await evaluate('localStorage.length===0 && sessionStorage.length===0'))
  await writeFile(process.argv[2], JSON.stringify({ ...fixture, browser_tokens: issued }))
  console.log('Browser: protected Report return, safe fallback, own settings/plain text Report, real DB outage and retry for login/me/settings/reports/logout: passed')
} catch (error) {
  if (/^[A-Z_0-9]+$/.test(error.code ?? '')) { console.error('browser_code=' + error.code) }
  console.error('browser_stage=' + phase)
  process.exitCode = 1
} finally {
  clearTimeout(timeout)
  try {
    await terminateBrowser(browser, browserExited)
  } finally {
    try {
      if (vite) { await vite.close() }
    } finally {
      if (profile) { await rm(profile, { recursive: true, force: true }) }
    }
  }
}
