import assert from 'node:assert/strict'
import test from 'node:test'
import { isReport, isReportSummary } from '../src/report-data.ts'
const value = { id: '01ARZ3NDEKTSV4RRFFQ69G5FAV', collection_name: '架空', scheduled_at: '2026-01-02T00:00:00Z', generated_at: '2026-01-02T00:00:04Z', body_markdown: '<script>bad()</script>', summary_markdown: '', updates_started_at: '2026-01-01T00:00:00Z', updates_ended_at: '2026-01-02T00:00:02Z' }
test('Report success bodies require all visible fields with valid types', () => {
  assert(isReport(value))
  assert(isReportSummary(value))
  for (const key of Object.keys(value)) {
    assert.equal(isReport({ ...value, [key]: null }), false)
    const missing = { ...value }; delete missing[key]
    assert.equal(isReport(missing), false)
  }
  for (const invalid of [null, [], {}, { ...value, id: 'invalid' }, { ...value, generated_at: 'not a time' }]) {
    assert.equal(isReport(invalid), false)
  }
})
