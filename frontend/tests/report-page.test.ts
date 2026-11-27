import { expect, test } from 'vitest'
import { reportTimeKey, reportPrecedes } from '../src/features/reports/data'
import { decodeReports } from '../src/features/reports/api'
const row = (id: string, at: string) => ({ id, collection_name: '架空', scheduled_at: at, generated_at: '2026-04-01T01:00:00Z' })
const a = row('00000000000000000000000001', '2026-04-01T00:00:00.000002Z')
const b = row('00000000000000000000000002', '2026-04-01T00:00:00.000001Z')
test('microseconds oppose ID order and compare before ID; exact ties use ID', () => {
  expect(Date.parse(a.scheduled_at)).toBe(Date.parse(b.scheduled_at))
  expect(reportPrecedes(a,b)).toBe(true)
  expect(reportPrecedes(b,a)).toBe(false)
  expect(reportPrecedes({ ...a, scheduled_at: b.scheduled_at },b)).toBe(false)
  expect(decodeReports({ reports: [a,b], next_cursor: null }, '/reports')).not.toBeNull()
  expect(decodeReports({ reports: [b,a], next_cursor: null }, '/reports')).toBeNull()
})
test.each(['2026-02-30T00:00:00Z','2026-04-01T00:00:00.0000001Z','0000-01-01T00:00:00Z','2026-04-01T00:00:00+00:00','bad'])('rejects noncanonical list time %s', at => { expect(reportTimeKey(at)).toBeNull() })
test('page shape/count/uniqueness/cursor consistency and exact-time ties', () => {
  expect(decodeReports({ reports: [], next_cursor: null }, '/reports')).not.toBeNull()
  for (const body of [{ reports: [] },{ reports: [],next_cursor: 'cursor' },{ reports: [a,a],next_cursor: null },{ reports: [a],next_cursor: '' },{ reports: [a],next_cursor: 1 }]) {
    expect(decodeReports(body,'/reports')).toBeNull()
  }
})
