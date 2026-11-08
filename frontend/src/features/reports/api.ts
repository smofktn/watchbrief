import { request } from '../../api'
import { reportIDFromPath } from '../../navigation'
import { isReport, isReportSummary, type Report, type ReportSummary } from './data'
export type Resource = { items: ReportSummary[]; selected: Report | null }
export const fetchReports = (path: string, signal: AbortSignal) => request(`/api${path}`, { signal })
export const decodeReports = (body: unknown, path: string): Resource | null => {
  if (typeof body !== 'object' || body === null) {
    return null
  }
  if (path === '/reports' && 'reports' in body && Array.isArray(body.reports) && body.reports.every(isReportSummary)) {
    return { items: body.reports, selected: null }
  }
  if (isReport(body) && body.id === reportIDFromPath(path)) {
    return { items: [], selected: body }
  }
  return null
}
