import { inject, onMounted, onUnmounted, provide, readonly, ref, type InjectionKey } from 'vue'
import type { Router } from 'vue-router'
import { canonicalLoginPath, DEFAULT_PROTECTED_PATH, isProtectedPath, loginDestination, loginPathFor, LOGIN_PATH } from '../../navigation'
import { errorCode, FAILURE, jsonBody, NETWORK_FAILURE } from '../../api'
import * as api from './api'

type State = 'checking' | 'anonymous' | 'authenticated' | 'error' | 'logging-out' | 'logout-error' | 'not-found'
export const createSession = (router: Router) => {
  const state = ref<State>('checking')
  const currentLoginId = ref('')
  const loginId = ref('')
  const password = ref('')
  const message = ref('')
  const inputError = ref('')
  const submitting = ref(false)
  const epoch = ref(0)
  const internalPaths = new Set<string>()
  let navigationGeneration = 0
  let disposed = false
  let logoutController: AbortController | undefined
  const invalidate = () => {
    // Stop resource consumers before changing the generation; synchronous watchers must not start a fresh request.
    state.value = 'checking'
    ++epoch.value
    currentLoginId.value = ''
    loginId.value = ''
    password.value = ''
    inputError.value = ''
    message.value = ''
    submitting.value = false
  }
  const active = (generation: number) => !disposed && generation === epoch.value
  const replace = async (path: string) => {
    internalPaths.add(path)
    try {
      await router.replace(path)
    } finally {
      internalPaths.delete(path)
    }
  }
  const loadCurrentUser = async () => {
    if (disposed || state.value === 'logging-out' || state.value === 'logout-error') {
      return
    }
    invalidate()
    const generation = epoch.value
    state.value = 'checking'
    try {
      const response = await api.getCurrentUser()
      const body = await jsonBody(response)
      if (!active(generation)) {
        return
      }
      const path = router.currentRoute.value.path
      if (response.status === 401 && errorCode(body) === 'unauthenticated') {
        if (isProtectedPath(path)) {
          await replace(loginPathFor(path))
        }
        if (active(generation)) {
          state.value = 'anonymous'
        }
        return
      }
      if (response.status === 200 && api.isCurrentUser(body)) {
        if (path === LOGIN_PATH) {
          await replace(loginDestination(router.currentRoute.value.fullPath.split('?')[1] ?? ''))
        }
        if (active(generation)) {
          currentLoginId.value = body.login_id
          state.value = 'authenticated'
        }
        return
      }
      message.value = FAILURE
    } catch {
      if (!active(generation)) {
        return
      }
      message.value = NETWORK_FAILURE
    }
    state.value = 'error'
  }
  const handleNavigation = async () => {
    const navigationId = ++navigationGeneration
    if (state.value === 'logging-out' || state.value === 'logout-error') {
      await replace(LOGIN_PATH)
      return
    }
    invalidate()
    state.value = 'checking'
    const route = router.currentRoute.value
    if (route.path === '/') {
      await replace(DEFAULT_PROTECTED_PATH)
    } else if (route.path === LOGIN_PATH) {
      await replace(canonicalLoginPath(route.fullPath.slice(LOGIN_PATH.length).split('#')[0] ?? ''))
    } else if (isProtectedPath(route.path)) {
      await replace(route.path)
    } else {
      state.value = 'not-found'
      return
    }
    if (navigationId === navigationGeneration && !disposed) {
      await loadCurrentUser()
    }
  }
  // Resource features report a confirmed API expiry through this boundary only.
  const expire = async (generation: number, path: string) => {
    if (!active(generation) || state.value !== 'authenticated') {
      return
    }
    invalidate()
    const nextGeneration = epoch.value
    state.value = 'checking'
    await replace(loginPathFor(path))
    if (active(nextGeneration)) {
      state.value = 'anonymous'
    }
  }
  const submit = async () => {
    if (submitting.value || state.value !== 'anonymous') {
      return
    }
    submitting.value = true
    const generation = ++epoch.value
    message.value = ''
    inputError.value = ''
    try {
      const response = await api.login(loginId.value, password.value)
      if (!active(generation)) {
        return
      }
      if (response.status === 204) {
        password.value = ''
        submitting.value = false
        await loadCurrentUser()
        return
      }
      const body = await jsonBody(response)
      if (!active(generation)) {
        return
      }
      const code = errorCode(body)
      if (response.status === 400 && code === 'invalid_request') {
        inputError.value = 'ログインIDは英数字・_・-の1〜64文字、パスワードは15〜128文字で入力してください。'
      } else if (response.status === 401 && code === 'invalid_credentials') {
        message.value = 'ログインIDまたはパスワードが違います'
      } else if (response.status === 403 && code === 'csrf_rejected') {
        message.value = '送信元を確認できません。画面を再読み込みしてください。'
      } else {
        message.value = FAILURE
      }
    } catch {
      if (active(generation)) {
        message.value = NETWORK_FAILURE
      }
    } finally {
      if (active(generation)) {
        submitting.value = false
      }
    }
  }
  const logout = async () => {
    if (state.value !== 'authenticated' && state.value !== 'logout-error') {
      return
    }
    invalidate()
    const generation = epoch.value
    state.value = 'logging-out'
    const navigation = replace(LOGIN_PATH)
    const controller = new AbortController()
    logoutController = controller
    const timeout = window.setTimeout(() => controller.abort(), 15_000)
    try {
      const response = await api.logout(controller.signal)
      await navigation
      if (active(generation)) {
        state.value = response.status === 204 ? 'anonymous' : 'logout-error'
      }
    } catch {
      await navigation
      if (active(generation)) {
        state.value = 'logout-error'
      }
    } finally {
      window.clearTimeout(timeout)
    }
  }
  const handlePageShow = (event: PageTransitionEvent) => {
    if (event.persisted) {
      void handleNavigation()
    }
  }
  let removeNavigation: (() => void) | undefined
  onMounted(async () => {
    await router.isReady()
    if (disposed) {
      return
    }
    removeNavigation = router.afterEach((_to, _from, failure) => {
      if (!failure && !internalPaths.has(_to.fullPath)) {
        void handleNavigation()
      }
    })
    window.addEventListener('pageshow', handlePageShow)
    void handleNavigation()
  })
  onUnmounted(() => {
    disposed = true
    invalidate()
    logoutController?.abort()
    removeNavigation?.()
    window.removeEventListener('pageshow', handlePageShow)
  })
  return { state: readonly(state), currentLoginId: readonly(currentLoginId), epoch: readonly(epoch),
    loginId, password, message: readonly(message), inputError: readonly(inputError), submitting: readonly(submitting),
    submit, logout, loadCurrentUser, expire }
}
export type Session = ReturnType<typeof createSession>
const key: InjectionKey<Session> = Symbol('session')
export const provideSession = (router: Router) => {
  const session = createSession(router)
  provide(key, session)
  return session
}
export const useSession = (): Session => {
  const session = inject(key)
  if (!session) {
    throw new Error('Session provider is required')
  }
  return session
}
