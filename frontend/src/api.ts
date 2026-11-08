export const FAILURE = '処理に失敗しました。時間をおいて再度お試しください'
export const NETWORK_FAILURE = '接続できません。再度お試しください'
export const errorCode = (body: unknown): unknown => {
  if (typeof body !== 'object' || body === null || !('error' in body)
    || typeof body.error !== 'object' || body.error === null || !('code' in body.error)) {
    return undefined
  }
  return body.error.code
}
export const request = (path: string, options: RequestInit = {}): Promise<Response> => fetch(path, {
  ...options, credentials: 'same-origin', cache: 'no-store',
})
export const jsonBody = (response: Response): Promise<unknown> => response.json().catch(() => null)
