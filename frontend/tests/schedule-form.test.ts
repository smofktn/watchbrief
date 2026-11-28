import fixture from './fixtures/cron.json?raw'
import { expect, test } from 'vitest'
import { validCron } from '../src/features/collections/scheduleForm'
const cases = JSON.parse(fixture) as { expression: string; valid: boolean }[]
test.each(cases)('shared cron grammar: $expression', ({ expression, valid }) => {
  expect(validCron(expression)).toBe(valid)
})
