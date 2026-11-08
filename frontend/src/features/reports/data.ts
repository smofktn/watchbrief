export interface ReportSummary { id: string; collection_name: string; scheduled_at: string; generated_at: string }
export interface Report extends ReportSummary {
  body_markdown: string
  summary_markdown: string
  updates_started_at: string
  updates_ended_at: string
}
const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null
const timestamp = (value: unknown): value is string => typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(value) && Number.isFinite(Date.parse(value))
export const isReportSummary = (value: unknown): value is ReportSummary => {
  return object(value) && typeof value.id === 'string' && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value.id)
    && typeof value.collection_name === 'string' && timestamp(value.scheduled_at) && timestamp(value.generated_at)
}
export const isReport = (value: unknown): value is Report => {
  return isReportSummary(value) && object(value) && typeof value.body_markdown === 'string'
    && typeof value.summary_markdown === 'string' && timestamp(value.updates_started_at) && timestamp(value.updates_ended_at)
}
