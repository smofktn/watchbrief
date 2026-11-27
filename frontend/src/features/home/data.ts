export interface HomeCollection {
  id: string
  name: string
  latest_report_id: string | null
}

export interface HomeData {
  collections: HomeCollection[]
}

const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null
const identifier = (value: unknown): value is string => typeof value === 'string' && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value)
const exactKeys = (value: Record<string, unknown>, expected: string[]): boolean => {
  return Object.keys(value).length === expected.length && expected.every(key => key in value)
}

const isHomeCollection = (value: unknown): value is HomeCollection => {
  return object(value) && exactKeys(value, ['id', 'name', 'latest_report_id']) && identifier(value.id)
    && typeof value.name === 'string' && value.name.length > 0
    && (value.latest_report_id === null || identifier(value.latest_report_id))
}

export const decodeHome = (body: unknown, path: string): HomeData | null => {
  if (path !== '/home' || !object(body) || !exactKeys(body, ['collections']) || !Array.isArray(body.collections)
    || !body.collections.every(isHomeCollection)) { return null }
  const ids = body.collections.map(collection => collection.id)
  if (new Set(ids).size !== ids.length) { return null }
  return { collections: body.collections }
}
