// Only implemented, protected application routes may be used as login destinations.
// Extend this allowlist together with the corresponding screen and authorization tests.
export const DEFAULT_PROTECTED_PATH = '/me'
export const LOGIN_PATH = '/login'

export const collectionIDFromPath = (path: string): string | null => {
  const match = /^\/collections\/([0-7][0-9A-HJKMNP-TV-Z]{25})$/.exec(path)
  return match?.[1] ?? null
}

export const reportIDFromPath = (path: string): string | null => {
  const match = /^\/reports\/([0-7][0-9A-HJKMNP-TV-Z]{25})$/.exec(path)
  return match?.[1] ?? null
}

export const isProtectedPath = (path: string): boolean => {
  return path === DEFAULT_PROTECTED_PATH || path === '/collections' || path === '/collections/new' || collectionIDFromPath(path) !== null || path === '/reports' || reportIDFromPath(path) !== null
}

export const safeReturnPath = (candidate: string | null): string => {
  return candidate !== null && isProtectedPath(candidate) ? candidate : DEFAULT_PROTECTED_PATH
}

export const loginDestination = (search: string): string => {
  const parameters = new URLSearchParams(search)
  const values = parameters.getAll('return_to')
  return values.length === 1 ? safeReturnPath(values[0] ?? null) : DEFAULT_PROTECTED_PATH
}

export const loginPathFor = (destination: string): string => {
  return `${LOGIN_PATH}?${new URLSearchParams({ return_to: safeReturnPath(destination) })}`
}

export const canonicalLoginPath = (search: string): string => {
  const parameters = new URLSearchParams(search)
  const values = parameters.getAll('return_to')
  if (values.length === 1 && values[0] !== null && isProtectedPath(values[0] ?? '')) {
    return loginPathFor(values[0] ?? DEFAULT_PROTECTED_PATH)
  }
  return LOGIN_PATH
}
