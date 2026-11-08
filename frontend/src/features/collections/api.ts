import { request } from '../../api'
import { collectionIDFromPath } from '../../navigation'
import { isCollection, isCollectionSummary, type Collection, type CollectionSummary } from './data'
export type Resource = { items: CollectionSummary[]; selected: Collection | null }
export const fetchCollections = (path: string, signal: AbortSignal) => request(`/api${path}`, { signal })
export const decodeCollections = (body: unknown, path: string): Resource | null => {
  if (typeof body !== 'object' || body === null) {
    return null
  }
  if (path === '/collections' && 'collections' in body && Array.isArray(body.collections) && body.collections.every(isCollectionSummary)) {
    return { items: body.collections, selected: null }
  }
  if (isCollection(body) && body.id === collectionIDFromPath(path)) {
    return { items: [], selected: body }
  }
  return null
}
