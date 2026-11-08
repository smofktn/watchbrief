import { nextTick, watch } from 'vue'
import type { Session } from '../features/auth/session'
export const focusSession = (session: Session) => {
  watch(session.state, async (state, previous) => {
    if (state !== 'logging-out' && state !== 'logout-error' && !(state === 'anonymous' && previous === 'logging-out')) {
      return
    }
    await nextTick()
    document.querySelector<HTMLElement>(state === 'anonymous' ? '[data-login-title]' : '[data-logout-status]')?.focus()
  })
}
