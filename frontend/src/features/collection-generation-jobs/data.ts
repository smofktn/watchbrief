import { isCollectionSummary, type CollectionSummary } from '../collections/data'
import { decodeJobs } from '../generation-jobs/api'
import type { GenerationJob } from '../generation-jobs/data'

export interface CollectionJobResource {
  collections: CollectionSummary[]
  selected: CollectionSummary | null
  items: GenerationJob[]
  nextCursor: string | null
}
const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null

export const decodeCollectionJobResource = (body: unknown, path: string): CollectionJobResource | null => {
  if (!object(body) || !Array.isArray(body.collections) || !body.collections.every(isCollectionSummary)) {
    return null
  }
  const collections = body.collections
  if (new Set(collections.map(row => row.id)).size !== collections.length || collections.some((row, index) => index > 0 && (collections[index - 1]!.display_order > row.display_order || collections[index - 1]!.display_order === row.display_order && collections[index - 1]!.id > row.id))) {
    return null
  }
  if (path === '/reports/collections') {
    if (body.selected !== null || body.generation_jobs !== null || body.next_cursor !== null) { return null }
    return { collections, selected: null, items: [], nextCursor: null }
  }
  const id = /^\/reports\/collections\/([0-7][0-9A-HJKMNP-TV-Z]{25})$/.exec(path)?.[1]
  if (!id || !object(body.selected) || !isCollectionSummary(body.selected) || body.selected.id !== id || !collections.some(row => row.id === id)) {
    return null
  }
  const page = decodeJobs({ generation_jobs: body.generation_jobs, next_cursor: body.next_cursor })
  if (!page) { return null }
  return { collections, selected: body.selected, items: page.items, nextCursor: page.nextCursor }
}
