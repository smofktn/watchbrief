import { useResource } from '../../composables/useResource'
import { decodeCollections, fetchCollections } from './api'
export const useCollections = () => useResource(fetchCollections, decodeCollections)
