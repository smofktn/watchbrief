import assert from 'node:assert/strict'
import { test } from 'vitest'
import { isCollection, isCollectionSummary } from '../src/features/collections/data.ts'
import { collectionIDFromPath, loginDestination, safeReturnPath } from '../src/navigation.ts'

const id = '01ARZ3NDEKTSV4RRFFQ69G5FAV'
const summary = { id, name: '架空の設定', enabled: false, display_order: 0 }
const detail = { ...summary, version: '1', research_instruction: '', initial_lookback_mode: 'ai', initial_lookback_days: null, schedule: null, targets: [] }

test('Collection routes permit only canonical ULID paths inside the application', () => {
  assert.equal(safeReturnPath('/collections'), '/collections')
  assert.equal(collectionIDFromPath(`/collections/${id}`), id)
  assert.equal(loginDestination(`?return_to=${encodeURIComponent(`/collections/${id}`)}`), `/collections/${id}`)
  for (const invalid of ['/collections/invalid', `/collections/${id.toLowerCase()}`, `/collections/${id}/`, `/collections/${id}?x=y`, `/collections/${id}#x`, '/collections/../login']) {
    assert.equal(collectionIDFromPath(invalid), null)
    assert.equal(safeReturnPath(invalid), '/me')
  }
})

test('Malformed successful responses are rejected before rendering nested settings', () => {
  assert.equal(isCollectionSummary(summary), true)
  assert.equal(isCollection(detail), true)
  for (const bad of [null, {}, { ...summary, id: 'invalid' }, { ...detail, targets: [null] }, { ...detail, targets: [{ id, name: 'x', enabled: true, instruction: '', display_order: 0, sources: null }] }, { ...detail, schedule: {} }, { ...detail, initial_lookback_mode: 'days', initial_lookback_days: null }]) {
    assert.equal(isCollection(bad), false)
  }
})

test('Collection revision is a canonical positive bigint string', () => {
  for (const version of ['', '0', '01', '+1', '1.0', '9223372036854775808', 1, null, undefined]) {
    assert.equal(isCollection({ ...detail, version }), false)
  }
  assert.equal(isCollection({ ...detail, version: '9223372036854775807' }), true)
})
