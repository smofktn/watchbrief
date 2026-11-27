import { request } from '../../api'
import { isGenerationJob, jobPrecedes, type GenerationJob } from './data'
export type Resource = { items: GenerationJob[]; nextCursor: string | null }
export const fetchJobs = (_path: string, signal: AbortSignal) => request('/api/generation-jobs', { signal })
export const decodeJobs = (body: unknown): Resource | null => {
  if (typeof body !== 'object' || body === null || !('generation_jobs' in body) || !Array.isArray(body.generation_jobs) || !body.generation_jobs.every(isGenerationJob) || !('next_cursor' in body)) { return null }
  const rows = body.generation_jobs
  const cursor = body.next_cursor
  if (!(cursor === null || typeof cursor === 'string' && /^[A-Za-z0-9_-]{1,256}$/.test(cursor)) || rows.length > 30 || cursor !== null && rows.length !== 30) { return null }
  if (new Set(rows.map(row => row.id)).size !== rows.length || rows.some((row, i) => i > 0 && !jobPrecedes(rows[i - 1]!, row))) { return null }
  return { items: rows, nextCursor: cursor }
}
