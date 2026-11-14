import { nextTick, onScopeDispose, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { errorCode, jsonBody, request } from '../../api'
import { validateCollectionForm } from './form'
import { useSession } from '../auth/session'
import { collectionIDFromPath } from '../../navigation'

export const useCollectionCreation = () => {
  const session = useSession()
  const route = useRoute()
  const router = useRouter()
  const form = reactive({ name: '', instruction: '', mode: 'ai', days: '' })
  const fields = reactive({ name: '', instruction: '', days: '' })
  const submitting = ref(false)
  const uncertain = ref(false)
  const conflict = ref(false)
  const message = ref('')
  let key = ''
  let payload = ''
  let generation = 0
  let controller: AbortController | undefined
  let requestTimeout: number | undefined
  const clear = () => {
    ++generation
    controller?.abort()
    window.clearTimeout(requestTimeout)
    Object.assign(form, { name: '', instruction: '', mode: 'ai', days: '' })
    Object.assign(fields, { name: '', instruction: '', days: '' })
    key = ''
    payload = ''
    submitting.value = false
    uncertain.value = false
    conflict.value = false
    message.value = ''
  }
  const validate = () => validateCollectionForm(form, fields)
  const submit = async () => {
    if (submitting.value || conflict.value || session.state.value !== 'authenticated') {
      return
    }
    const retrying = uncertain.value
    if (!retrying) {
      if (!validate()) {
        await nextTick()
        document.getElementById(fields.name ? 'collection-name' : fields.instruction ? 'collection-instruction' : 'collection-days')?.focus()
        return
      }
      payload = JSON.stringify({ name: form.name, research_instruction: form.instruction, initial_lookback_mode: form.mode, initial_lookback_days: form.mode === 'days' ? Number(form.days) : null })
      try {
        key = crypto.randomUUID()
      } catch {
        message.value = '保存を準備できません。画面を再読み込みしてください。'
        return
      }
    }
    const requestID = ++generation
    const epoch = session.epoch.value
    const active = () => requestID === generation && epoch === session.epoch.value && session.state.value === 'authenticated' && route.path === '/collections/new'
    const requestController = new AbortController()
    controller = requestController
    const timeout = window.setTimeout(() => requestController.abort(), 15_000)
    requestTimeout = timeout
    submitting.value = true
    message.value = ''
    // Every ambiguous result retries this exact payload/key, with editing locked.
    uncertain.value = true
    try {
      const response = await request('/api/collections', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key }, body: payload, signal: controller.signal })
      const body = await jsonBody(response)
      if (!active()) {
        return
      }
      if (response.status === 401 && errorCode(body) === 'unauthenticated') {
        await session.expire(epoch, '/collections/new')
        return
      }
      if ((response.status === 200 || response.status === 201) && typeof body === 'object' && body !== null && 'id' in body && typeof body.id === 'string' && collectionIDFromPath(`/collections/${body.id}`) !== null) {
        await router.replace(`/collections/${body.id}`)
        return
      }
      if (response.status === 400 && errorCode(body) === 'invalid_request') {
        if (!retrying) {
          uncertain.value = false
          key = ''
          payload = ''
        }
        message.value = retrying ? '要求が拒否されましたが、最初の保存結果は未確認です。一覧で確認するか、同じ内容で再試行してください。' : '入力内容を確認してください。名前は1〜200文字、調査方針は10,000文字以内、日数は1〜365日です。'
      } else if (response.status === 403 && errorCode(body) === 'csrf_rejected') {
        uncertain.value = retrying
        message.value = retrying ? '送信元を確認できません。一覧で保存結果を確認するか、同じ内容で再試行してください。' : '送信元を確認できません。画面を再読み込みしてください。'
      } else if (response.status === 409 && errorCode(body) === 'creation_conflict') {
        conflict.value = true
        message.value = '作成要求を確認できません。一覧で保存結果を確認してください。'
      } else {
        message.value = '保存結果を確認できません。同じ内容で再試行してください。'
      }
    } catch {
      if (active()) {
        message.value = '接続できず保存結果を確認できません。同じ内容で再試行してください。'
      }
    } finally {
      window.clearTimeout(timeout)
      if (active()) {
        submitting.value = false
      }
    }
  }
  watch(() => route.path, clear, { flush: 'sync' })
  watch([session.state, session.epoch], clear, { flush: 'sync' })
  onScopeDispose(clear)
  return { form, fields, submitting, uncertain, conflict, message, submit }
}
