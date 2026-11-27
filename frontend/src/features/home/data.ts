export interface HomeCollection {
  id: string
  name: string
  latest_report_id: string | null
  today_job_counts: HomeJobCounts
}

export interface HomeJobCounts {
  queued: number
  running: number
  succeeded: number
  failed: number
}

export interface HomeData {
  as_of_date: string
  collections: HomeCollection[]
}

const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null
const identifier = (value: unknown): value is string => typeof value === 'string' && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value)
const exactKeys = (value: Record<string, unknown>, expected: string[]): boolean => {
  return Object.keys(value).length === expected.length && expected.every(key => key in value)
}
const isDate = (value: unknown): value is string => {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) { return false }
  const [year, month, day] = value.split('-').map(Number)
  const parsed = new Date(Date.UTC(year!, month! - 1, day!))
  return parsed.getUTCFullYear() === year && parsed.getUTCMonth() === month! - 1 && parsed.getUTCDate() === day
}
const isCount = (value: unknown): value is number => typeof value === 'number' && Number.isInteger(value) && value >= 0
const isHomeJobCounts = (value: unknown): value is HomeJobCounts => object(value)
  && exactKeys(value, ['queued', 'running', 'succeeded', 'failed'])
  && isCount(value.queued) && isCount(value.running) && isCount(value.succeeded) && isCount(value.failed)

const isHomeCollection = (value: unknown): value is HomeCollection => {
  return object(value) && exactKeys(value, ['id', 'name', 'latest_report_id', 'today_job_counts']) && identifier(value.id)
    && typeof value.name === 'string' && value.name.length > 0
    && (value.latest_report_id === null || identifier(value.latest_report_id))
    && isHomeJobCounts(value.today_job_counts)
}

export const decodeHome = (body: unknown, path: string): HomeData | null => {
  if (path !== '/home' || !object(body) || !exactKeys(body, ['as_of_date', 'collections']) || !isDate(body.as_of_date) || !Array.isArray(body.collections)
    || !body.collections.every(isHomeCollection)) { return null }
  const ids = body.collections.map(collection => collection.id)
  if (new Set(ids).size !== ids.length) { return null }
  return { as_of_date: body.as_of_date, collections: body.collections }
}
