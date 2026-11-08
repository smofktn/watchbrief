import { afterEach, describe, expect, test, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import PrimeVue from 'primevue/config'
import App from '../src/App.vue'
import { createAppRouter } from '../src/router'

const id = '01ARZ3NDEKTSV4RRFFQ69G5FAV'
const otherId = '01ARZ3NDEKTSV4RRFFQ69G5FAW'
const summary = { id, name: '架空の設定', enabled: false, display_order: 0 }
const collection = { ...summary, research_instruction: '', initial_lookback_mode: 'ai', initial_lookback_days: null, schedule: null, targets: [] }
const report = { id, collection_name: '架空レポート', scheduled_at: '2026-01-02T00:00:00Z', generated_at: '2026-01-02T00:00:04Z', body_markdown: '<script>bad()</script><img src=x onerror=bad()>', summary_markdown: '', updates_started_at: '2026-01-01T00:00:00Z', updates_ended_at: '2026-01-02T00:00:02Z' }
const response = (body: unknown, status = 200) => new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const unauthenticated = () => response({ error: { code: 'unauthenticated' } }, 401)
const me = () => response({ login_id: 'fictional_user' })
const deferred = <T>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
let wrapper: VueWrapper | undefined
const start = async (path = '/me', responder: (path: string, options: RequestInit) => Response | Promise<Response> = () => me()) => {
  const fetch = vi.fn((path: string, options: RequestInit) => Promise.resolve(responder(path, options)))
  vi.stubGlobal('fetch', fetch)
  const router = createAppRouter(createMemoryHistory())
  await router.push(path)
  await router.isReady()
  wrapper = mount(App, { attachTo: document.body, global: { plugins: [router, [PrimeVue, { unstyled: true }]] } })
  await flushPromises()
  return { router, fetch, app: wrapper }
}
const click = async (label: string) => {
  const button = wrapper!.findAll('button').find((item) => item.text() === label)
  expect(button, label).toBeDefined()
  await button!.trigger('click')
  await flushPromises()
}
const fillAndSubmit = async () => {
  await wrapper!.get('#login-id').setValue('fictional_user')
  await wrapper!.get('#password').setValue('fictional-password')
  await wrapper!.get('form').trigger('submit')
  await flushPromises()
}
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('real router and login form', () => {
  test('protected direct URL logs in and returns to the same resource; all requests use private same-origin policy', async () => {
    let authenticated = false
    const { app, router, fetch } = await start(`/collections/${id}`, (path) => {
      if (path === '/api/me') { return authenticated ? me() : unauthenticated() }
      if (path === '/api/login') { authenticated = true; return response(null, 204) }
      return response(collection)
    })
    expect(router.currentRoute.value.fullPath).toBe(`/login?return_to=%2Fcollections%2F${id}`)
    await fillAndSubmit()
    expect(router.currentRoute.value.path).toBe(`/collections/${id}`)
    expect(app.text()).toContain('架空の設定')
    expect(app.find('#password').exists()).toBe(false)
    for (const [, options] of fetch.mock.calls) {
      expect(options.credentials).toBe('same-origin')
      expect(options.cache).toBe('no-store')
    }
    expect(JSON.parse(fetch.mock.calls.find(([path]) => path === '/api/login')![1].body as string)).toEqual({ login_id: 'fictional_user', password: 'fictional-password' })
  })
  test.each([
    [400, 'invalid_request', 'ログインIDは英数字'],
    [401, 'invalid_credentials', 'ログインIDまたはパスワードが違います'],
    [403, 'csrf_rejected', '送信元を確認できません'],
    [500, 'internal_error', '処理に失敗しました'],
    [200, '', '処理に失敗しました'],
  ])('login %s keeps anonymous form with appropriate feedback', async (status, code, text) => {
    const { app, router } = await start('/login', (path) => path === '/api/me' ? unauthenticated() : response({ error: { code } }, status))
    await fillAndSubmit()
    expect(app.text()).toContain(text)
    expect(router.currentRoute.value.path).toBe('/login')
    expect(app.get('button').attributes('disabled')).toBeUndefined()
  })
  test('network failure differs from credential refusal; duplicate submission is blocked', async () => {
    const pending = deferred<Response>()
    let calls = 0
    const { app } = await start('/login', (path) => {
      if (path === '/api/me') { return unauthenticated() }
      ++calls
      return pending.promise.then(() => { throw new Error('private internal error') })
    })
    await fillAndSubmit()
    await app.get('form').trigger('submit')
    expect(calls).toBe(1)
    expect(app.get('#password').attributes('disabled')).toBeDefined()
    pending.resolve(response(null, 204))
    await flushPromises()
    expect(app.text()).toContain('接続できません')
    expect(app.text()).not.toContain('private internal error')
  })
  test.each(['/missing', '/collections/invalid', '/reports/invalid', '/ME', '/me/'])('unknown route %s has no authentication fetch', async (path) => {
    const { app, fetch } = await start(path)
    expect(app.text()).toContain('ページが見つかりません。')
    expect(fetch).not.toHaveBeenCalled()
    await app.get('a').trigger('click')
    await flushPromises()
    expect(app.text()).toContain('fictional_user')
  })
  test('root canonicalizes; authenticated login return rejects unsafe destination', async () => {
    const { router, app } = await start('/')
    expect(router.currentRoute.value.path).toBe('/me')
    await router.push('/login?return_to=https://example.com&password=hidden#secret')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/me')
    expect(app.text()).toContain('fictional_user')
  })
  test('500 on me retains URL and retries; network and malformed success remain errors', async () => {
    let outcome = 0
    const { app, router } = await start('/reports', () => {
      ++outcome
      if (outcome === 1) { return response({}, 500) }
      if (outcome === 2) { throw new Error('network') }
      if (outcome === 3) { return response({ login_id: 'BAD ID' }) }
      return me()
    })
    expect(router.currentRoute.value.path).toBe('/reports')
    expect(app.text()).toContain('処理に失敗しました')
    await click('再試行')
    expect(app.text()).toContain('接続できません')
    await click('再試行')
    expect(app.text()).toContain('処理に失敗しました')
    await router.push('/me')
    await flushPromises()
    expect(app.text()).toContain('fictional_user')
  })
})

describe('resources, retry, and rendering', () => {
  test.each([
    ['/collections', { collections: [] }, 'コレクションはありません。'],
    ['/reports', { reports: [] }, '保存済みReportはありません。'],
    [`/collections/${id}`, collection, 'ウォッチ対象はありません。'],
  ])('renders %s empty states', async (path, body, text) => {
    const { app } = await start(path, (url) => url === '/api/me' ? me() : response(body))
    expect(app.text()).toContain(text)
  })
  test.each([
    ['/collections', { collections: [summary] }, '架空の設定', `/collections/${id}`, collection],
    ['/reports', { reports: [report] }, '架空レポート', `/reports/${id}`, report],
  ])('list %s links to detail through router', async (path, list, text, target, detail) => {
    const { app, router } = await start(path, (url) => url === '/api/me' ? me() : response(url === `/api${path}` ? list : detail))
    await app.get(`a[href="${target}"]`).trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(target)
    expect(app.get('article').text()).toContain(text)
  })
  test('report HTML is displayed only as pre text', async () => {
    const { app } = await start(`/reports/${id}`, (url) => url === '/api/me' ? me() : response(report))
    expect(app.get('pre').text()).toBe(report.body_markdown)
    expect(app.find('pre script').exists()).toBe(false)
    expect(app.find('pre img').exists()).toBe(false)
  })
  test('collection renders nested data and optional settings as plain text', async () => {
    const rich = { ...collection, enabled: true, research_instruction: '<b>plain</b>', initial_lookback_mode: 'days', initial_lookback_days: 7, schedule: { id, cron_expression: '0 9 * * *' }, targets: [
      { id, name: '対象', enabled: true, instruction: '', display_order: 0, sources: [] },
      { id: otherId, name: '対象2', enabled: false, instruction: '補足', display_order: 1, sources: [{ id, name: '情報源', url: 'https://example.com', instruction: '<img>' }] },
    ] }
    const { app } = await start(`/collections/${id}`, (url) => url === '/api/me' ? me() : response(rich))
    expect(app.text()).toContain('7日前から')
    expect(app.text()).toContain('0 9 * * *')
    expect(app.text()).toContain('登録情報源はありません。')
    expect(app.text()).toContain('https://example.com')
    expect(app.find('article b').exists()).toBe(false)
  })
  test.each(['/collections', '/reports'])('%s errors, malformed 200, 404 and retry have distinct states', async (path) => {
    const outcomes = [response({}, 500), response({}), response({ error: { code: 'not_found' } }, 404)]
    const { app } = await start(path, (url) => url === '/api/me' ? me() : outcomes.shift()!)
    const label = path === '/reports' ? 'Reportを再試行' : '設定を再試行'
    expect(app.text()).toContain('処理に失敗しました')
    await click(label)
    expect(app.text()).toContain('処理に失敗しました')
    await click(label)
    expect(app.text()).toContain('見つかりません。')
    expect(app.find('article').exists()).toBe(false)
  })
  test('resource network error retries, 401 expires shared session and clears identity', async () => {
    let count = 0
    const { app, router } = await start('/collections', (url) => {
      if (url === '/api/me') { return me() }
      if (++count === 1) { throw new Error('network') }
      return unauthenticated()
    })
    expect(app.text()).toContain('接続できません')
    await click('設定を再試行')
    expect(router.currentRoute.value.fullPath).toBe('/login?return_to=%2Fcollections')
    expect(app.find('form').exists()).toBe(true)
    expect(app.text()).not.toContain('fictional_user')
  })
  test.each([`/collections/${id}`, `/reports/${id}`])('other user or missing detail %s returns only not-found', async (path) => {
    const { app } = await start(path, (url) => url === '/api/me' ? me() : response({ error: { code: 'not_found' } }, 404))
    expect(app.text()).toContain('見つかりません。')
    expect(app.find('article').exists()).toBe(false)
    expect(app.text()).not.toContain('架空の設定')
    expect(app.text()).not.toContain('架空レポート')
  })
  test('wrong detail ID in 200 is rejected', async () => {
    const { app } = await start(`/reports/${id}`, (url) => url === '/api/me' ? me() : response({ ...report, id: otherId }))
    expect(app.text()).toContain('処理に失敗しました')
    expect(app.find('pre').exists()).toBe(false)
  })
})

describe('logout and stale response isolation', () => {
  test('logout removes personal data immediately, confirms only 204 and focuses empty form', async () => {
    const pending = deferred<Response>()
    const { app } = await start('/me', (url) => url === '/api/logout' ? pending.promise : me())
    await click('ログアウト')
    expect(app.text()).toContain('ログアウトしています。')
    expect(app.text()).not.toContain('fictional_user')
    expect(document.activeElement?.textContent).toContain('ログアウトしています。')
    pending.resolve(response(null, 204))
    await flushPromises()
    expect(app.get('#login-id').element).toHaveProperty('value', '')
    expect(app.get('#password').element).toHaveProperty('value', '')
    expect(document.activeElement?.textContent).toBe('ログイン')
  })
  test('failed logout blocks route/history/BFCache identity reload until explicit logout retry', async () => {
    let logoutCalls = 0
    const { app, router, fetch } = await start('/me', (url) => {
      if (url === '/api/logout') { return response(null, ++logoutCalls === 1 ? 500 : 204) }
      return me()
    })
    await click('ログアウト')
    const initialCalls = fetch.mock.calls.length
    await router.push('/reports')
    await flushPromises()
    router.back()
    await flushPromises()
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
    await flushPromises()
    expect(fetch.mock.calls.length).toBe(initialCalls)
    expect(app.text()).not.toContain('fictional_user')
    expect(app.text()).toContain('ログアウトを確認できません')
    await click('再試行')
    expect(fetch.mock.calls.at(-1)![0]).toBe('/api/logout')
    expect(app.find('form').exists()).toBe(true)
  })
  test('logout timeout retains barrier and permits retry', async () => {
    const { app } = await start('/me', (url, options) => {
      if (url !== '/api/logout') { return me() }
      return new Promise((_resolve, reject) => options.signal!.addEventListener('abort', () => reject(new Error('abort'))))
    })
    vi.useFakeTimers()
    await click('ログアウト')
    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()
    expect(app.text()).toContain('ログアウトを確認できません')
    expect(app.text()).not.toContain('fictional_user')
  })
  test('resource response after logout is discarded even if transport ignores abort', async () => {
    const pending = deferred<Response>()
    const { app, fetch } = await start('/reports', (url) => url === '/api/me' ? me() : url === '/api/logout' ? response(null, 204) : pending.promise)
    await click('ログアウト')
    expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/me', '/api/reports', '/api/logout'])
    pending.resolve(response({ reports: [report] }))
    await flushPromises()
    expect(app.text()).not.toContain('架空レポート')
    expect(app.find('form').exists()).toBe(true)
  })
  test('old resource after route change cannot replace new detail', async () => {
    const pending = deferred<Response>()
    const { app, router } = await start('/reports', (url) => url === '/api/me' ? me() : url === '/api/reports' ? pending.promise : response(collection))
    await router.push(`/collections/${id}`)
    await flushPromises()
    pending.resolve(response({ reports: [report] }))
    await flushPromises()
    expect(app.text()).toContain('架空の設定')
    expect(app.text()).not.toContain('架空レポート')
  })
  test('late me response after route change and logout cannot restore identity', async () => {
    const pending = deferred<Response>()
    let meCalls = 0
    const { app, router } = await start('/me', (url) => url === '/api/logout' ? response(null, 204) : ++meCalls === 1 ? pending.promise : me())
    await router.push('/login')
    await flushPromises()
    await click('ログアウト')
    pending.resolve(me())
    await flushPromises()
    expect(app.find('form').exists()).toBe(true)
    expect(app.text()).not.toContain('fictional_user')
  })
  test('late login response after navigating away cannot authenticate new screen', async () => {
    const pending = deferred<Response>()
    const { app, router, fetch } = await start('/login', (url) => url === '/api/login' ? pending.promise : unauthenticated())
    await fillAndSubmit()
    await router.push('/missing')
    await flushPromises()
    const calls = fetch.mock.calls.length
    pending.resolve(response(null, 204))
    await flushPromises()
    expect(fetch.mock.calls.length).toBe(calls)
    expect(app.text()).toContain('ページが見つかりません。')
  })
  test('BFCache and history recheck identity and discard old data', async () => {
    let authenticated = true
    const { app, router } = await start('/me', () => authenticated ? me() : unauthenticated())
    await router.push('/collections')
    await flushPromises()
    authenticated = false
    router.back()
    await flushPromises()
    expect(app.find('form').exists()).toBe(true)
    authenticated = true
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
    await flushPromises()
    expect(app.text()).toContain('fictional_user')
  })
  test('changing detail ID clears old content and waits for identity before fetching the new resource', async () => {
    const identity = deferred<Response>()
    let identities = 0
    const { app, router, fetch } = await start(`/reports/${id}`, (url) => {
      if (url === '/api/me') { return ++identities === 1 ? me() : identity.promise }
      return response({ ...report, id: url.endsWith(otherId) ? otherId : id })
    })
    expect(app.get('pre').text()).toBe(report.body_markdown)
    await router.push(`/reports/${otherId}`)
    await flushPromises()
    expect(app.find('pre').exists()).toBe(false)
    expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/me', `/api/reports/${id}`, '/api/me'])
    identity.resolve(me())
    await flushPromises()
    expect(fetch.mock.calls.at(-1)![0]).toBe(`/api/reports/${otherId}`)
    expect(app.find('pre').exists()).toBe(true)
  })
  test('external navigation crossing an internal authentication redirect invalidates late identity', async () => {
    const identity = deferred<Response>()
    const redirect = deferred<void>()
    const { app, router, fetch } = await start('/me', () => identity.promise)
    router.beforeEach((to) => to.path === '/login' ? redirect.promise : undefined)
    identity.resolve(unauthenticated())
    await flushPromises()
    await router.push('/missing')
    await flushPromises()
    redirect.resolve()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/missing')
    expect(app.text()).toContain('ページが見つかりません。')
    expect(fetch.mock.calls.length).toBe(1)
  })
  test('resource timeout shows network feedback and retry succeeds', async () => {
    let calls = 0
    const { app } = await start('/collections', (url, options) => {
      if (url === '/api/me') { return me() }
      if (++calls === 1) {
        return response({}, 500)
      }
      if (calls === 2) {
        return new Promise((_resolve, reject) => options.signal!.addEventListener('abort', () => reject(new Error('timeout'))))
      }
      return response({ collections: [] })
    })
    vi.useFakeTimers()
    await click('設定を再試行')
    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()
    expect(app.text()).toContain('接続できません')
    await click('設定を再試行')
    expect(app.text()).toContain('コレクションはありません。')
  })
  test('logout transport failure remains blocked and nonpersisted pageshow has no effect', async () => {
    const { app, fetch } = await start('/me', (url) => {
      if (url === '/api/logout') { throw new Error('network') }
      return me()
    })
    window.dispatchEvent(new PageTransitionEvent('pageshow'))
    await flushPromises()
    expect(fetch.mock.calls.length).toBe(1)
    await click('ログアウト')
    expect(app.text()).toContain('ログアウトを確認できません')
    expect(app.find('form').exists()).toBe(false)
  })
  test('malformed JSON success is not treated as authenticated', async () => {
    const { app } = await start('/me', () => new Response('invalid json', { status: 200 }))
    expect(app.text()).toContain('処理に失敗しました')
    expect(app.text()).not.toContain('ログインID：')
  })
  test('unmount aborts resource transport and removes pageshow listener', async () => {
    const pending = deferred<Response>()
    let signal: AbortSignal | undefined
    const { app, fetch } = await start('/reports', (url, options) => {
      if (url === '/api/me') { return me() }
      signal = options.signal as AbortSignal
      return pending.promise
    })
    app.unmount()
    wrapper = undefined
    expect(signal?.aborted).toBe(true)
    const calls = fetch.mock.calls.length
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
    pending.resolve(response({ reports: [report] }))
    await flushPromises()
    expect(fetch.mock.calls.length).toBe(calls)
  })
})
