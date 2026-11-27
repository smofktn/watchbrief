import { expect, test } from 'vitest'
import { decodeHome } from '../src/features/home/data'

const collectionID = '01ARZ3NDEKTSV4RRFFQ69G5FAV'
const reportID = '01ARZ3NDEKTSV4RRFFQ69G5FAW'
const zeroCounts = { queued: 0, running: 0, succeeded: 0, failed: 0 }
const homeBody = { as_of_date: '2026-11-27', collections: [
  { id: collectionID, name: 'current', latest_report_id: reportID, today_job_counts: { queued: 1, running: 2, succeeded: 3, failed: 4 } },
  { id: '01ARZ3NDEKTSV4RRFFQ69G5FAX', name: 'empty', latest_report_id: null, today_job_counts: zeroCounts },
] }

test('Home decoder accepts the server date and exact four-state counts', () => {
  expect(decodeHome(homeBody, '/home')).toEqual(homeBody)
  expect(decodeHome({ as_of_date: '2026-11-27', collections: [] }, '/home')).toEqual({ as_of_date: '2026-11-27', collections: [] })
})

test.each([
  { ...homeBody, collections: [{ ...homeBody.collections[0], body_markdown: 'private' }] },
  { ...homeBody, collections: [{ ...homeBody.collections[0], latest_report_id: 'bad' }] },
  { ...homeBody, collections: [{ ...homeBody.collections[0], today_job_counts: { ...zeroCounts, failed: -1 } }] },
  { ...homeBody, collections: [{ ...homeBody.collections[0], today_job_counts: { queued: 0, running: 0, succeeded: 0 } }] },
  { ...homeBody, as_of_date: '2026-02-30' },
  { ...homeBody, collections: [{ ...homeBody.collections[0] }, { ...homeBody.collections[0], name: 'duplicate' }] },
  { ...homeBody, collections: null },
])('Home decoder rejects malformed or expanded payloads', body => {
  expect(decodeHome(body, '/home')).toBeNull()
})

test('Home decoder rejects a valid payload for an unrelated route', () => {
  expect(decodeHome({ as_of_date: '2026-11-27', collections: [] }, '/me')).toBeNull()
})
