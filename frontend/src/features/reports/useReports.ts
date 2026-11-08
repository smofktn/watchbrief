import { useResource } from '../../composables/useResource'
import { decodeReports, fetchReports } from './api'
export const useReports = () => useResource(fetchReports, decodeReports)
