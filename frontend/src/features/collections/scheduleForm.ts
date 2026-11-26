// Keep this grammar aligned with domain.ValidCron and the shared fixture.
export const validCron = (value: string): boolean => {
  if (!value.length || value.length > 256 || /[^0-9*,/\- \t]/.test(value)) { return false }
  const fields = value.trim().split(/[ \t]+/)
  if (fields.length !== 5) { return false }
  const bounds = [[0, 59], [0, 23], [1, 31], [1, 12], [0, 7]] as const
  return fields.every((field, i) => field.split(',').every(atom => {
    const parts = /^(\*|[0-9]+(?:-[0-9]+)?)(?:\/([0-9]+))?$/.exec(atom)
    if (!parts) { return false }
    const base = parts[1]!
    const [lo, hi] = bounds[i]!
    if (parts[2] !== undefined && (Number(parts[2]) < 1 || Number(parts[2]) > hi || base !== '*' && !base.includes('-'))) { return false }
    if (base === '*') { return true }
    const ends = base.split('-').map(Number)
    return ends[0]! >= lo && ends[0]! <= hi && (ends.length === 1 || ends[1]! >= ends[0]! && ends[1]! <= hi)
  }))
}
