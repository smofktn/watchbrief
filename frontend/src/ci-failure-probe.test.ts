import { expect, test } from 'vitest'

// Temporary probe for Issue #70; remove after observing the failed PR check.
test('CI reports a failing test', () => {
  expect(true).toBe(false)
})
