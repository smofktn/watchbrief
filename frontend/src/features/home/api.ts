import { request } from '../../api'
export const fetchHome = (path: string, signal: AbortSignal) => request(`/api${path}`, { signal })
