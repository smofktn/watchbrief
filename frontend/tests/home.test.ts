import { expect, test } from 'vitest'
import { decodeHome } from '../src/features/home/data'

const collectionID = '01ARZ3NDEKTSV4RRFFQ69G5FAV'
const reportID = '01ARZ3NDEKTSV4RRFFQ69G5FAW'

test('Home decoder accepts only current Collection summaries and optional Report references', () => {
  expect(decodeHome({ collections: [{ id: collectionID, name: 'current', latest_report_id: reportID }, { id: '01ARZ3NDEKTSV4RRFFQ69G5FAX', name: 'empty', latest_report_id: null }] }, '/home'))
    .toEqual({ collections: [{ id: collectionID, name: 'current', latest_report_id: reportID }, { id: '01ARZ3NDEKTSV4RRFFQ69G5FAX', name: 'empty', latest_report_id: null }] })
  expect(decodeHome({ collections: [] }, '/home')).toEqual({ collections: [] })
})

test.each([
  { collections: [{ id: collectionID, name: 'current', latest_report_id: reportID, body_markdown: 'private' }] },
  { collections: [{ id: collectionID, name: 'current', latest_report_id: 'bad' }] },
  { collections: [{ id: collectionID, name: 'current', latest_report_id: null }, { id: collectionID, name: 'duplicate', latest_report_id: null }] },
  { collections: null },
])('Home decoder rejects malformed or expanded payloads', body => {
  expect(decodeHome(body, '/home')).toBeNull()
})

test('Home decoder rejects a valid payload for an unrelated route', () => {
  expect(decodeHome({ collections: [] }, '/me')).toBeNull()
})
