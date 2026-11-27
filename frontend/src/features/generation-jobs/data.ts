import { reportTimeKey } from '../reports/data'
export interface GenerationJob { id: string; collection_name: string; scheduled_at: string; status: 'queued' | 'running' | 'succeeded' | 'failed'; report_id: string | null }
const id = (value: unknown): value is string => typeof value === 'string' && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value)
export const isGenerationJob = (value: unknown): value is GenerationJob => {
  if (typeof value !== 'object' || value === null) { return false }
  const row = value as Record<string, unknown>
  return id(row.id) && typeof row.collection_name === 'string' && typeof row.scheduled_at === 'string' && reportTimeKey(row.scheduled_at) !== null
    && ['queued', 'running', 'succeeded', 'failed'].includes(row.status as string)
    && (row.status === 'succeeded' ? id(row.report_id) : row.report_id === null)
}
export const jobPrecedes = (first: GenerationJob, next: GenerationJob): boolean => {
  const a = reportTimeKey(first.scheduled_at)
  const b = reportTimeKey(next.scheduled_at)
  return a !== null && b !== null && (a > b || a === b && first.id > next.id)
}
const dayFormat = new Intl.DateTimeFormat('ja-JP', { timeZone: 'Asia/Tokyo', year: 'numeric', month: '2-digit', day: '2-digit' })
const timeFormat = new Intl.DateTimeFormat('ja-JP', { timeZone: 'Asia/Tokyo', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' })
export const jobDay = (at: string): string => dayFormat.format(new Date(at))
export const jobTime = (at: string): string => timeFormat.format(new Date(at))
export const statusLabel = { queued: '待機中', running: '実行中', succeeded: '成功', failed: '失敗' }
export const groupJobs = (rows: readonly GenerationJob[]) => {
  const groups: { day: string; items: GenerationJob[] }[] = []
  for (const row of rows) {
    const day = jobDay(row.scheduled_at)
    if (groups.at(-1)?.day !== day) { groups.push({ day, items: [] }) }
    groups.at(-1)!.items.push(row)
  }
  return groups
}
