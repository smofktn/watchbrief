import { computed, onScopeDispose, readonly, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useResource } from '../../composables/useResource'
import { errorCode, jsonBody, request } from '../../api'
import { useSession } from '../auth/session'
import { decodeJobs, fetchJobs } from './api'
import { jobPrecedes, type GenerationJob } from './data'

export const useGenerationJobs = () => {
  const resource = useResource(fetchJobs, decodeJobs)
  const session = useSession()
  const route = useRoute()
  const extra = shallowRef<GenerationJob[]>([])
  const nextCursor = ref<string | null>(null)
  const moreBusy = ref(false)
  const moreMessage = ref('')
  const usedCursors = new Set<string>()
  let generation = 0
  let controller: AbortController | undefined
  let timer: number | undefined
  const clearMore = () => {
    ++generation
    controller?.abort()
    window.clearTimeout(timer)
    extra.value = []
    nextCursor.value = resource.data.value?.nextCursor ?? null
    usedCursors.clear()
    moreBusy.value = false
    moreMessage.value = ''
  }
  // Initial fetch, identity recheck and route/logout clearing remain owned by
  // useResource. Its data replacement invalidates all continuation requests.
  watch(resource.data, clearMore, { immediate: true, flush: 'sync' })
  const data = computed(() => resource.data.value ? { ...resource.data.value, items: [...resource.data.value.items, ...extra.value], nextCursor: nextCursor.value } : null)
  const loadMore = async () => {
    if (moreBusy.value || resource.state.value !== 'ready' || route.path !== '/reports/daily' || session.state.value !== 'authenticated' || !nextCursor.value || !data.value) { return }
    const cursor = nextCursor.value
    const epoch = session.epoch.value
    const requestID = ++generation
    const active = () => requestID === generation && epoch === session.epoch.value && session.state.value === 'authenticated' && route.path === '/reports/daily'
    const requestController = new AbortController()
    controller = requestController
    const timeout = window.setTimeout(() => requestController.abort(), 15_000)
    timer = timeout
    moreBusy.value = true
    moreMessage.value = ''
    try {
      const response = await request('/api/generation-jobs?' + new URLSearchParams({ cursor }), { signal: requestController.signal })
      const body = await jsonBody(response)
      if (!active()) { return }
      if (response.status === 401 && errorCode(body) === 'unauthenticated') { await session.expire(epoch, '/reports/daily'); return }
      const page = response.status === 200 ? decodeJobs(body) : null
      const items = data.value?.items ?? []
      const ids = new Set(items.map(item => item.id))
      if (!page || page.items.some(item => ids.has(item.id)) || page.items.length > 0 && items.length > 0 && !jobPrecedes(items.at(-1)!, page.items[0]!) || page.nextCursor !== null && (page.nextCursor === cursor || usedCursors.has(page.nextCursor))) {
        moreMessage.value = '続きを取得できません。表示済みの項目は保持しています。同じ続きから再試行してください。'
        return
      }
      extra.value = [...extra.value, ...page.items]
      usedCursors.add(cursor)
      nextCursor.value = page.nextCursor
    } catch {
      if (active()) { moreMessage.value = '続きを取得できません。表示済みの項目は保持しています。同じ続きから再試行してください。' }
    } finally {
      window.clearTimeout(timeout)
      if (active()) { moreBusy.value = false }
    }
  }
  onScopeDispose(clearMore)
  return { ...resource, data, nextCursor: readonly(nextCursor), moreBusy: readonly(moreBusy), moreMessage: readonly(moreMessage), loadMore }
}
