import { onScopeDispose, readonly, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSession } from '../features/auth/session'
import { errorCode, FAILURE, jsonBody, NETWORK_FAILURE } from '../api'

export const useResource = <T>(fetchResource: (path: string, signal: AbortSignal) => Promise<Response>, decode: (body: unknown, path: string) => T | null) => {
  const session = useSession()
  const route = useRoute()
  const state = ref<'idle' | 'loading' | 'ready' | 'error' | 'not-found'>('idle')
  const message = ref('')
  const data = shallowRef<T | null>(null)
  let generation = 0
  let controller: AbortController | undefined
  let requestTimeout: number | undefined
  const clear = () => {
    ++generation
    controller?.abort()
    window.clearTimeout(requestTimeout)
    data.value = null
    message.value = ''
    state.value = 'idle'
  }
  const load = async () => {
    clear()
    if (session.state.value !== 'authenticated') {
      return
    }
    const requestId = generation
    const epoch = session.epoch.value
    const path = route.path
    const requestController = new AbortController()
    controller = requestController
    const signal = requestController.signal
    const timeout = window.setTimeout(() => requestController.abort(), 15_000)
    requestTimeout = timeout
    const active = () => requestId === generation && epoch === session.epoch.value && session.state.value === 'authenticated'
    state.value = 'loading'
    try {
      const response = await fetchResource(path, signal)
      const body = await jsonBody(response)
      if (!active()) {
        return
      }
      if (response.status === 401 && errorCode(body) === 'unauthenticated') {
        await session.expire(epoch, path)
        return
      }
      if (response.status === 404 && errorCode(body) === 'not_found') {
        state.value = 'not-found'
        return
      }
      const value = response.status === 200 ? decode(body, path) : null
      if (value !== null) {
        data.value = value
        state.value = 'ready'
        return
      }
      message.value = FAILURE
    } catch {
      if (!active()) {
        return
      }
      message.value = NETWORK_FAILURE
    } finally {
      window.clearTimeout(timeout)
    }
    state.value = 'error'
  }
  // Route changes clear immediately; only a confirmed session starts the next resource request.
  watch(() => route.path, clear, { flush: 'sync' })
  watch([session.state, session.epoch], () => { void load() }, { immediate: true, flush: 'sync' })
  onScopeDispose(clear)
  return { state: readonly(state), message: readonly(message), data: readonly(data), load }
}
