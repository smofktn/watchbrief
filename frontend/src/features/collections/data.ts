export interface CollectionSummary { id: string; name: string; enabled: boolean; display_order: number }
export interface Source { id: string; name: string; url: string; instruction: string }
export interface Target { id: string; name: string; instruction: string; enabled: boolean; display_order: number; sources: Source[] }
export interface Collection extends CollectionSummary {
  research_instruction: string
  initial_lookback_mode: 'ai' | 'days'
  initial_lookback_days: number | null
  schedule: { id: string; cron_expression: string } | null
  targets: Target[]
}

const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null
const id = (value: unknown): value is string => typeof value === 'string' && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value)
const source = (value: unknown): value is Source => {
  return object(value) && id(value.id) && typeof value.name === 'string'
    && typeof value.url === 'string' && typeof value.instruction === 'string'
}
const target = (value: unknown): value is Target => {
  return object(value) && id(value.id) && typeof value.name === 'string'
    && typeof value.instruction === 'string' && typeof value.enabled === 'boolean'
    && Number.isInteger(value.display_order) && Array.isArray(value.sources) && value.sources.every(source)
}
export const isCollectionSummary = (value: unknown): value is CollectionSummary => {
  return object(value) && id(value.id) && typeof value.name === 'string'
    && typeof value.enabled === 'boolean' && Number.isInteger(value.display_order)
}
export const isCollection = (value: unknown): value is Collection => {
  if (!isCollectionSummary(value) || !object(value)) {
    return false
  }
  const lookback = value.initial_lookback_mode === 'ai' && value.initial_lookback_days === null
    || value.initial_lookback_mode === 'days' && typeof value.initial_lookback_days === 'number'
    && Number.isInteger(value.initial_lookback_days) && value.initial_lookback_days >= 1 && value.initial_lookback_days <= 365
  const schedule = value.schedule === null || object(value.schedule) && id(value.schedule.id)
    && typeof value.schedule.cron_expression === 'string'
  return typeof value.research_instruction === 'string' && lookback && schedule
    && Array.isArray(value.targets) && value.targets.every(target)
}
