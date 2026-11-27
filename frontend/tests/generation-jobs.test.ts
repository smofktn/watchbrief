import { expect, test } from 'vitest'
import { decodeJobs } from '../src/features/generation-jobs/api'
import { groupJobs } from '../src/features/generation-jobs/data'
const row = { id: '00000000000000000000000001', collection_name: '架空', scheduled_at: '2026-04-01T00:00:00.123456Z', status: 'queued' as const, report_id: null }
test('rejects invalid status, report mismatch, malformed time/ID and invalid page contracts', () => {
  expect(decodeJobs({ generation_jobs: [row], next_cursor: null })).not.toBeNull()
  for (const bad of [{ ...row, status:'other' },{ ...row, status:'succeeded' },{ ...row, report_id:row.id },{ ...row, scheduled_at:'2026-02-30T00:00:00Z' },{ ...row, id:'bad' }]) {
    expect(decodeJobs({ generation_jobs:[bad], next_cursor:null })).toBeNull()
  }
  for (const bad of [{ generation_jobs:[row,row], next_cursor:null },{ generation_jobs:[], next_cursor:'x' },{ generation_jobs:[] },{ generation_jobs:[row], next_cursor:'' }]) {
    expect(decodeJobs(bad)).toBeNull()
  }
})
test('same JST day across pagination groups once, independent of local timezone', () => {
  const rows = Array.from({length:35},(_,i)=>({...row,id:String(35-i).padStart(26,'0')}))
  expect(groupJobs(rows)).toHaveLength(1)
  expect(groupJobs(rows)[0]?.items).toHaveLength(35)
})
