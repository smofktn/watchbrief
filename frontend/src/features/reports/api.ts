import { request } from '../../api'
import { reportIDFromPath } from '../../navigation'
import { isReport, isReportSummary, reportTimeKey, reportPrecedes, type Report, type ReportSummary } from './data'
export type Resource = { items: ReportSummary[]; selected: Report | null; nextCursor: string | null }
export const fetchReports = (path: string, signal: AbortSignal) => request(`/api${path}`, { signal })
export const decodeReports = (body: unknown, path: string): Resource | null => {
  if (typeof body !== 'object' || body === null) {
    return null
  }
  if (path === '/reports' && 'reports' in body && Array.isArray(body.reports) && body.reports.every(isReportSummary) && 'next_cursor' in body) {
    const rows = body.reports
    const cursor = body.next_cursor
    if (!(cursor === null || typeof cursor === 'string' && /^[A-Za-z0-9_-]{1,256}$/.test(cursor)) || rows.length > 30 || cursor !== null && rows.length !== 30) { return null }
    if (new Set(rows.map(row => row.id)).size !== rows.length || rows.some((row, i) => reportTimeKey(row.scheduled_at) === null || i > 0 && !reportPrecedes(rows[i - 1]!, row))) { return null }
    return { items: rows, selected: null, nextCursor: cursor }
  }
  if (isReport(body) && body.id === reportIDFromPath(path)) {
    return { items: [], selected: body, nextCursor: null }
  }
  return null
}
