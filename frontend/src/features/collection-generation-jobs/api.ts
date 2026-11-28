import { errorCode, jsonBody, request } from '../../api'
import { decodeCollections } from '../collections/api'
import { collectionJobsIDFromPath } from '../../navigation'
import type { CollectionSummary } from '../collections/data'
import { decodeCollectionJobResource } from './data'

const syntheticResponse = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
  status,
  headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' },
})

export const fetchCollectionJobResource = async (
  path: string,
  signal: AbortSignal,
  onCandidates?: (collections: CollectionSummary[]) => void,
): Promise<Response> => {
  const listResponse = await request('/api/collections', { signal })
  if (listResponse.status !== 200) {
    if (listResponse.status === 404) {
      const body = await jsonBody(listResponse)
      if (!signal.aborted && errorCode(body) === 'not_found') { onCandidates?.([]) }
      return syntheticResponse(body, 404)
    }
    return listResponse
  }
  const list = decodeCollections(await jsonBody(listResponse), '/collections')
  if (!list || list.selected !== null || new Set(list.items.map(row => row.id)).size !== list.items.length || list.items.some((row, index) => index > 0 && (list.items[index - 1]!.display_order > row.display_order || list.items[index - 1]!.display_order === row.display_order && list.items[index - 1]!.id > row.id))) { return syntheticResponse(null) }
  if (signal.aborted) { return syntheticResponse(null) }
  onCandidates?.(list.items)
  const id = collectionJobsIDFromPath(path)
  if (!id) {
    return syntheticResponse({ collections: list.items, selected: null, generation_jobs: null, next_cursor: null })
  }
  const selected: CollectionSummary | undefined = list.items.find(row => row.id === id)
  if (!selected) {
    return syntheticResponse({ error: { code: 'not_found', message: '見つかりません。' } }, 404)
  }
  const response = await request(`/api/collections/${id}/generation-jobs`, { signal })
  if (response.status !== 200) {
    if (response.status === 404) {
      const body = await jsonBody(response)
      if (!signal.aborted && errorCode(body) === 'not_found') {
        onCandidates?.(list.items.filter(collection => collection.id !== id))
      }
      return syntheticResponse(body, 404)
    }
    return response
  }
  const page = await jsonBody(response)
  if (!decodeCollectionJobResource({ collections: list.items, selected, ...(typeof page === 'object' && page !== null ? page : {}) }, path)) {
    return syntheticResponse(null)
  }
  return syntheticResponse({ collections: list.items, selected, ...(page as Record<string, unknown>) })
}

export const decodeCollectionJobs = decodeCollectionJobResource
