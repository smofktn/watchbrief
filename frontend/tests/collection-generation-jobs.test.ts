import { afterEach, expect, test, vi } from 'vitest'
import { fetchCollectionJobResource } from '../src/features/collection-generation-jobs/api'
import { decodeCollectionJobResource } from '../src/features/collection-generation-jobs/data'

const id = '01ARZ3NDEKTSV4RRFFQ69G5FAV'
const other = '01ARZ3NDEKTSV4RRFFQ69G5FAW'
const rows = [
  { id, name: '架空Collection', enabled: false, display_order: 0 },
  { id: other, name: '別の架空Collection', enabled: true, display_order: 1 },
]
const job = { id, collection_name: '履歴名', scheduled_at: '2026-11-27T03:04:05.123456Z', status: 'succeeded', report_id: other }
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

afterEach(() => vi.unstubAllGlobals())

test('selector requires only existing owned Collection summaries', async () => {
  const fetch = vi.fn(async (_path: string) => response({ collections: rows }))
  vi.stubGlobal('fetch', fetch)
  const result = await fetchCollectionJobResource('/reports/collections', new AbortController().signal)
  expect(result.status).toBe(200)
  expect(await result.json()).toEqual({ collections: rows, selected: null, generation_jobs: null, next_cursor: null })
  expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/collections'])
})

test('selected Collection history fetch follows candidates and is attached to the selected identity', async () => {
  const fetch = vi.fn(async (path: string) => path === '/api/collections'
    ? response({ collections: rows })
    : response({ generation_jobs: [job], next_cursor: null }))
  vi.stubGlobal('fetch', fetch)
  const path = `/reports/collections/${id}`
  const result = await fetchCollectionJobResource(path, new AbortController().signal)
  const body = await result.json()
  expect(fetch.mock.calls.map(([url]) => url)).toEqual(['/api/collections', `/api/collections/${id}/generation-jobs`])
  expect(decodeCollectionJobResource(body, path)).toEqual({ collections: rows, selected: rows[0], items: [job], nextCursor: null })
})

test('verified candidate responses replace the option list before selected history completes', async () => {
  let resolveHistory!: (response: Response) => void
  const history = new Promise<Response>(resolve => { resolveHistory = resolve })
  const fetch = vi.fn(async (path: string) => path === '/api/collections'
    ? response({ collections: rows })
    : history)
  let markCandidates!: () => void
  const candidatesReady = new Promise<void>(resolve => { markCandidates = resolve })
  const candidates = vi.fn(() => markCandidates())
  vi.stubGlobal('fetch', fetch)
  const pending = fetchCollectionJobResource(`/reports/collections/${id}`, new AbortController().signal, candidates)
  await candidatesReady
  expect(candidates).toHaveBeenCalledWith(rows)
  expect(fetch.mock.calls.map(([url]) => url)).toEqual(['/api/collections', `/api/collections/${id}/generation-jobs`])
  resolveHistory(response({ generation_jobs: [], next_cursor: null }))
  expect((await pending).status).toBe(200)
})

test('stale/non-owned Collection is rejected before requesting history', async () => {
  const fetch = vi.fn(async (_path: string) => response({ collections: [rows[1]] }))
  vi.stubGlobal('fetch', fetch)
  const result = await fetchCollectionJobResource(`/reports/collections/${id}`, new AbortController().signal)
  expect(result.status).toBe(404)
  expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/collections'])
})

test('a deleted Collection is removed from verified options before returning 404', async () => {
  const fetch = vi.fn(async (path: string) => path === '/api/collections'
    ? response({ collections: rows })
    : response({ error: { code: 'not_found', message: '見つかりません。' } }, 404))
  const candidates = vi.fn()
  vi.stubGlobal('fetch', fetch)
  const result = await fetchCollectionJobResource(`/reports/collections/${id}`, new AbortController().signal, candidates)
  expect(result.status).toBe(404)
  expect(await result.json()).toEqual({ error: { code: 'not_found', message: '見つかりません。' } })
  expect(candidates.mock.calls).toEqual([[rows], [[rows[1]]]])
})

test('a missing candidate endpoint clears the previously verified options', async () => {
  const fetch = vi.fn(async () => response({ error: { code: 'not_found' } }, 404))
  const candidates = vi.fn()
  vi.stubGlobal('fetch', fetch)
  const result = await fetchCollectionJobResource('/reports/collections', new AbortController().signal, candidates)
  expect(result.status).toBe(404)
  expect(candidates).toHaveBeenCalledWith([])
})

test('a late candidate response after abort cannot restore options or request history', async () => {
  let resolve!: (response: Response) => void
  const pending = new Promise<Response>(done => { resolve = done })
  const fetch = vi.fn(() => pending)
  const candidates = vi.fn()
  vi.stubGlobal('fetch', fetch)
  const controller = new AbortController()
  const result = fetchCollectionJobResource('/reports/collections', controller.signal, candidates)
  controller.abort()
  resolve(response({ collections: rows }))
  const fetched = await result
  expect(fetched.status).toBe(200)
  expect(candidates).not.toHaveBeenCalled()
  expect(fetch).toHaveBeenCalledTimes(1)
})

test('malformed collection summaries and job pages cannot decode as a successful resource', () => {
  expect(decodeCollectionJobResource({ collections: [rows[0], rows[0]], selected: null, generation_jobs: null, next_cursor: null }, '/reports/collections')).toBeNull()
  expect(decodeCollectionJobResource({ collections: rows, selected: rows[0], generation_jobs: [job, job], next_cursor: null }, `/reports/collections/${id}`)).toBeNull()
})
