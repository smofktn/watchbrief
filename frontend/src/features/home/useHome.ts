import { useResource } from '../../composables/useResource'
import { decodeHome } from './data'
import { fetchHome } from './api'

export const useHome = () => useResource(fetchHome, decodeHome)
