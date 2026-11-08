import { request } from '../../api'
export const getCurrentUser = () => request('/api/me')
export const login = (loginId: string, password: string) => request('/api/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ login_id: loginId, password }),
})
export const logout = (signal: AbortSignal) => request('/api/logout', { method: 'POST', signal })
export const isCurrentUser = (body: unknown): body is { login_id: string } => {
  return typeof body === 'object' && body !== null && 'login_id' in body
    && typeof body.login_id === 'string' && /^[a-z0-9_-]{1,64}$/.test(body.login_id)
}
