export const errorMessages = {
  invalid_input: 'ログインIDは英数字・_・-の1〜64文字、パスワードは15〜128文字で入力してください。',
  invalid_credentials: 'ログインIDまたはパスワードを確認してください。',
  csrf_rejected: '安全に送信できませんでした。ページを開き直してください。',
  internal_error: '処理に失敗しました。時間をおいて再試行してください。',
  unauthenticated: 'ログインしてください。',
}

export function createClient(fetcher = fetch) {
  async function request(path, options) {
    let response
    try {
      response = await fetcher(path, { credentials: 'same-origin', cache: 'no-store', ...options })
    } catch {
      throw new Error('通信に失敗しました。接続を確認して再試行してください。')
    }
    if (!response.ok) {
      const body = await response.json().catch(() => ({}))
      const error = new Error(errorMessages[body.error] || errorMessages.internal_error)
      error.status = response.status
      throw error
    }
    return response.json()
  }
  return {
    me: () => request('/api/me'),
    login: (loginID, password) => request('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-WatchBrief-CSRF': '1' },
      body: JSON.stringify({ login_id: loginID, password }),
    }),
  }
}
