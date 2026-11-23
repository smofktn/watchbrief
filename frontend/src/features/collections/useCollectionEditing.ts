import type { DeepReadonly } from 'vue'
import { nextTick, onScopeDispose, reactive, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorCode, jsonBody, request } from '../../api'
import { useSession } from '../auth/session'
import { isCollection, isCollectionVersion, type Collection } from './data'
import { validateCollectionForm } from './form'
import { newSource, sourceChanges, validateSources } from './sourceForm'
import { targetChanges, targetDrafts, validateTargets, type TargetDraft } from './targetForm'

export const useCollectionEditing = (original: DeepReadonly<Collection>, saved: () => void) => {
  const session = useSession()
  const route = useRoute()
  const form = reactive({ name: '', instruction: '', mode: 'ai', days: '' })
  const targets = reactive<TargetDraft[]>([])
  let baseline = original
  const fields = reactive({ name: '', instruction: '', days: '' })
  const busy = ref(false)
  const uncertain = ref(false)
  const conflict = ref(false)
  const terminal = ref(false)
  const latest = shallowRef<Collection | null>(null)
  const message = ref('')
  let version = original.version
  let key = ''
  let payload = ''
  let generation = 0
  let controller: AbortController | undefined
  let timer: number | undefined
  const assign = (value: DeepReadonly<Collection>) => {
    Object.assign(form, { name: value.name, instruction: value.research_instruction, mode: value.initial_lookback_mode, days: value.initial_lookback_days === null ? '' : String(value.initial_lookback_days) })
    Object.assign(fields, { name: '', instruction: '', days: '' })
    version = value.version
    baseline = value
    targets.splice(0, targets.length, ...targetDrafts(value))
  }
  assign(original)
  const clear = () => {
    ++generation
    controller?.abort()
    window.clearTimeout(timer)
    Object.assign(form, { name: '', instruction: '', mode: 'ai', days: '' })
    Object.assign(fields, { name: '', instruction: '', days: '' })
    targets.splice(0)
    version = ''; key = ''; payload = ''
    latest.value = null
    message.value = ''
    busy.value = false
    uncertain.value = false
    conflict.value = false
    terminal.value = true
  }
  const run = async (checking: boolean) => {
    if (busy.value || terminal.value || session.state.value !== 'authenticated' || (!checking && conflict.value)) {
      return
    }
    const retrying = uncertain.value
    if (!checking && !retrying) {
      const bodyValid = validateCollectionForm(form, fields)
      const targetsValid = validateTargets(targets)
      const allSources = targets.flatMap(target => target.sources)
      // Legacy stored values are preserved unless the user edits that source.
      for (const source of allSources) { source.error = ''; source.errorField = '' }
      const sourcesValid = validateSources(allSources.filter(source => sourceChanges([source], baseline).length > 0))
      if (!bodyValid || !targetsValid || !sourcesValid) {
        await nextTick()
        document.getElementById(!bodyValid ? (fields.name ? 'collection-name' : fields.instruction ? 'collection-instruction' : 'collection-days') : !targetsValid ? `target-name-${targets.findIndex(target => target.error)}` : `source-${allSources.find(source => source.error)?.errorField}-${allSources.find(source => source.error)?.key}`)?.focus()
        return
      }
      const changes = targetChanges(targets, baseline)
      if (changes.length > 20) {
        message.value = '1回に保存できる対象の追加・変更は20行までです。変更を減らして保存してください。'
        return
      }
      const sources = sourceChanges(targets.flatMap(target => target.sources), baseline)
      if (sources.length > 20) {
        message.value = '1回に保存できる情報源の追加・変更は20行までです。変更を減らして保存してください。'
        return
      }
      try {
        key = crypto.randomUUID()
      } catch {
        message.value = '保存を準備できません。詳細へ戻って再度お試しください。'
        return
      }
      payload = JSON.stringify({ name: form.name, research_instruction: form.instruction, initial_lookback_mode: form.mode, initial_lookback_days: form.mode === 'days' ? Number(form.days) : null, version, ...(changes.length ? { targets: changes } : {}), ...(sources.length ? { sources } : {}) })
      if (new TextEncoder().encode(payload).length > 1048576) {
        key = ''; payload = ''
        message.value = '保存する内容が大きすぎます。入力を減らしてください。'
        return
      }
    }
    const requestID = ++generation
    const epoch = session.epoch.value
    const path = route.path
    const active = () => requestID === generation && epoch === session.epoch.value && session.state.value === 'authenticated' && path === route.path
    const requestController = new AbortController()
    controller = requestController
    const timeout = window.setTimeout(() => requestController.abort(), 15_000)
    timer = timeout
    busy.value = true
    message.value = ''
    if (checking) {
      latest.value = null
    } else {
      uncertain.value = true
    }
    try {
      const response = await request(`/api/collections/${original.id}`, checking ? { signal: controller.signal } : { method: 'PATCH', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key }, body: payload, signal: controller.signal })
      const body = await jsonBody(response)
      if (!active()) {
        return
      }
      if (response.status === 401 && errorCode(body) === 'unauthenticated') {
        await session.expire(epoch, path)
        return
      }
      if (response.status === 404 && errorCode(body) === 'not_found') {
        terminal.value = true
        message.value = '見つかりません。詳細へ戻って確認してください。'
      } else if (checking) {
        if (response.status === 200 && isCollection(body) && body.id === original.id) {
          latest.value = body
        } else {
          message.value = '最新値を取得できません。入力は保持しています。再試行してください。'
        }
      } else if (response.status === 200 && typeof body === 'object' && body !== null && 'id' in body && body.id === original.id && 'version' in body && isCollectionVersion(body.version) && BigInt(body.version) === BigInt(version) + 1n) {
        saved()
      } else if (response.status === 409 && (errorCode(body) === 'update_conflict' || errorCode(body) === 'collection_not_stopped')) {
        conflict.value = true
        message.value = '別の更新が保存されているか、停止中ではありません。入力は保持しています。最新値を確認してください。'
      } else if (response.status === 409 && errorCode(body) === 'update_request_conflict') {
        terminal.value = true
        message.value = '更新要求を確認できません。詳細へ戻って保存結果を確認してください。'
      } else if ((response.status === 400 && errorCode(body) === 'invalid_request') || (response.status === 403 && errorCode(body) === 'csrf_rejected')) {
        if (!retrying) {
          uncertain.value = false
          key = ''; payload = ''
        }
        message.value = retrying ? '要求が拒否されましたが、最初の保存結果は未確認です。同じ内容で再試行するか、詳細で確認してください。' : response.status === 400 ? '入力内容と各欄の条件を確認してください。情報源URLはHTTP/HTTPSで、認証情報を含めず入力してください。' : '送信元を確認できません。詳細へ戻って確認してください。'
      } else {
        message.value = '保存結果を確認できません。同じ内容で再試行してください。'
      }
    } catch {
      if (active()) {
        message.value = checking ? '最新値を取得できません。入力は保持しています。再試行してください。' : '接続できず保存結果を確認できません。同じ内容で再試行してください。'
      }
    } finally {
      window.clearTimeout(timeout)
      if (active()) {
        busy.value = false
      }
    }
  }
  const restart = () => {
    if (busy.value || terminal.value || !latest.value || latest.value.enabled) {
      return
    }
    assign(latest.value)
    latest.value = null
    uncertain.value = false
    conflict.value = false
    key = ''; payload = ''
    message.value = '最新値から編集を再開しました。必要な変更を入力してください。'
  }
  watch(() => route.path, clear, { flush: 'sync' })
  watch([session.state, session.epoch], clear, { flush: 'sync' })
  onScopeDispose(clear)
  const addTarget = async () => {
    if (busy.value || uncertain.value || conflict.value || terminal.value) { return }
    if (targetChanges(targets, baseline).length >= 20) {
      message.value = '1回に保存できる対象の追加・変更は20行までです。先に保存してください。'
      return
    }
    targets.push({ id: null, name: '', instruction: '', error: '', sources: [] })
    await nextTick()
    document.getElementById(`target-name-${targets.length - 1}`)?.focus()
  }
  const removeTarget = async (index: number) => {
    if (busy.value || uncertain.value || conflict.value || terminal.value || targets[index]?.id !== null) { return }
    targets.splice(index, 1)
    await nextTick()
    document.getElementById('targets-title')?.focus()
  }
  const addSource = async (index: number) => {
    const target = targets[index]
    if (busy.value || uncertain.value || conflict.value || terminal.value || !target?.id) { return }
    if (sourceChanges(targets.flatMap(item => item.sources), baseline).length >= 20) {
      message.value = '1回に保存できる情報源の追加・変更は20行までです。先に保存してください。'
      return
    }
    const source = newSource(target.id)
    target.sources.push(source)
    await nextTick()
    document.getElementById(`source-name-${source.key}`)?.focus()
  }
  const removeSource = async (targetIndex: number, sourceIndex: number) => {
    const target = targets[targetIndex]
    if (busy.value || uncertain.value || conflict.value || terminal.value || target?.sources[sourceIndex]?.id !== null) { return }
    target.sources.splice(sourceIndex, 1)
    await nextTick()
    document.getElementById(`sources-title-${targetIndex}`)?.focus()
  }
  return { targets, addSource, removeSource, addTarget, removeTarget, form, fields, busy, uncertain, conflict, terminal, latest, message, submit: () => run(false), checkLatest: () => run(true), restart }
}
