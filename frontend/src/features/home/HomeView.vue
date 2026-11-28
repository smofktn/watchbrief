<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { RouterLink } from 'vue-router'
import Button from 'primevue/button'
import { errorCode, FAILURE, jsonBody, NETWORK_FAILURE, request } from '../../api'
import { useSession } from '../auth/session'
import { decodeHome, type HomeCollection, type HomeData } from './data'
import { useHome } from './useHome'

const { state, message, data, load } = useHome()
const route = useRoute()
const session = useSession()
const editing = ref(false)
const draftIDs = ref<string[]>([])
const expected = ref<Array<{ id: string, version: string }>>([])
const saveState = ref<'idle' | 'saving' | 'unknown' | 'conflict'>('idle')
const saveMessage = ref('')
const conflictCode = ref('')
const conflictHome = ref<HomeData | null>(null)
const confirmedHome = ref<HomeData | null>(null)
const conflictLoadMessage = ref('')
const conflictLoading = ref(false)
let generation = 0
let saveController: AbortController | undefined
let conflictController: AbortController | undefined
let intent: { key: string, body: string } | null = null

const hasTodayJobs = (collection: HomeCollection): boolean => {
  const counts = collection.today_job_counts
  return counts.queued > 0 || counts.running > 0 || counts.succeeded > 0 || counts.failed > 0
}

const visibleHome = computed(() => confirmedHome.value ?? data.value)
const displayedCollections = computed(() => {
  const collections = visibleHome.value?.collections ?? []
  if (!editing.value) { return collections }
  const byID = new Map(collections.map(collection => [collection.id, collection]))
  return draftIDs.value.flatMap(id => {
    const collection = byID.get(id)
    return collection ? [collection] : []
  })
})

const clearEditor = () => {
  ++generation
  saveController?.abort()
  conflictController?.abort()
  saveController = undefined
  conflictController = undefined
  editing.value = false
  draftIDs.value = []
  expected.value = []
  saveState.value = 'idle'
  saveMessage.value = ''
  conflictCode.value = ''
  conflictHome.value = null
  confirmedHome.value = null
  conflictLoadMessage.value = ''
  conflictLoading.value = false
  intent = null
}

watch([() => route.path, session.state, session.epoch], clearEditor, { flush: 'sync' })
onScopeDispose(clearEditor)

const positions = computed(() => new Map(draftIDs.value.map((id, index) => [id, index])))
const matchingConflictSet = computed(() => {
  const latest = conflictHome.value?.collections.map(collection => collection.id) ?? []
  return latest.length === draftIDs.value.length && latest.every(id => positions.value.has(id))
})

const startEditing = () => {
  if (!data.value || data.value.collections.length === 0 || route.path !== '/home' || session.state.value !== 'authenticated') { return }
  expected.value = data.value.collections.map(collection => ({ id: collection.id, version: collection.version }))
  draftIDs.value = data.value.collections.map(collection => collection.id)
  saveState.value = 'idle'
  saveMessage.value = ''
  conflictHome.value = null
  confirmedHome.value = null
  intent = null
  editing.value = true
}

const move = (id: string, offset: -1 | 1) => {
  if (saveState.value === 'saving' || saveState.value === 'unknown' || saveState.value === 'conflict') { return }
  const from = positions.value.get(id)
  if (from === undefined) { return }
  const to = from + offset
  if (to < 0 || to >= draftIDs.value.length) { return }
  const next = [...draftIDs.value]
  const displaced = next[to]
  const current = next[from]
  if (displaced === undefined || current === undefined) { return }
  next[to] = current
  next[from] = displaced
  draftIDs.value = next
  saveMessage.value = ''
  intent = null
}

const active = (requestID: number, epoch: number) => requestID === generation && epoch === session.epoch.value
  && session.state.value === 'authenticated' && route.path === '/home'
const activeConflictRequest = (controller: AbortController, requestID: number, epoch: number) =>
  conflictController === controller && !controller.signal.aborted && active(requestID, epoch)
const activeConflictGeneration = (controller: AbortController, requestID: number, epoch: number) =>
  conflictController === controller && active(requestID, epoch)

const save = async () => {
  if (saveState.value === 'saving' || saveState.value === 'conflict' || !editing.value) { return }
  if (intent === null) {
    intent = {
      key: crypto.randomUUID(),
      body: JSON.stringify({ expected: expected.value, ordered_ids: draftIDs.value }),
    }
  }
  const requestIntent = intent
  if (requestIntent === null) { return }
  const requestID = ++generation
  const epoch = session.epoch.value
  const controller = new AbortController()
  saveController = controller
  const timeout = window.setTimeout(() => controller.abort(), 15_000)
  saveState.value = 'saving'
  saveMessage.value = ''
  try {
    const response = await request('/api/collections/order', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': requestIntent.key },
      body: requestIntent.body,
      signal: controller.signal,
    })
    if (!active(requestID, epoch)) { return }
    if (response.status === 401) {
      const body = await jsonBody(response)
      if (!active(requestID, epoch)) { return }
      if (errorCode(body) === 'unauthenticated') {
        await session.expire(epoch, '/home')
      } else {
        saveState.value = 'unknown'
        saveMessage.value = '認証応答を確認できません。編集内容は保持しています。ページを再確認してください。'
      }
      return
    }
    if (response.status === 204) {
      editing.value = false
      draftIDs.value = []
      expected.value = []
      intent = null
      saveState.value = 'idle'
      conflictHome.value = null
      confirmedHome.value = null
      ++generation
      await load()
      return
    }
    const body = await jsonBody(response)
    if (!active(requestID, epoch)) { return }
    const code = errorCode(body)
    if (response.status === 409 && (code === 'order_conflict' || code === 'order_request_conflict')) {
      saveState.value = 'conflict'
      conflictCode.value = String(code)
      saveMessage.value = code === 'order_conflict'
        ? 'Collectionの構成または設定が更新されています。下書きを残したまま最新状態を確認してください。'
        : '同じ更新要求が別の内容で使われています。再送せず、現在の順序を確認してください。'
      return
    }
    if (response.status === 400 && code === 'invalid_request') {
      intent = null
      saveState.value = 'idle'
      saveMessage.value = '順序を保存できませんでした。下書きは残しています。内容を確認してください。'
      return
    }
    saveState.value = 'unknown'
    saveMessage.value = '保存結果を確認できません。同じ内容で再試行するか、キャンセルして現在の順序を確認してください。'
  } catch {
    if (active(requestID, epoch)) {
      saveState.value = 'unknown'
      saveMessage.value = NETWORK_FAILURE + ' 保存結果が不明な場合は同じ内容で再試行してください。'
    }
  } finally {
    window.clearTimeout(timeout)
    if (requestID === generation) {
      saveController = undefined
      if (saveState.value === 'saving') { saveState.value = 'idle' }
    }
  }
}

const refreshConflict = async () => {
  if (saveState.value !== 'conflict' || conflictLoading.value) { return }
  conflictController?.abort()
  const controller = new AbortController()
  conflictController = controller
  conflictLoading.value = true
  const requestID = generation
  const epoch = session.epoch.value
  const timeout = window.setTimeout(() => controller.abort(), 15_000)
  conflictHome.value = null
  conflictLoadMessage.value = ''
  try {
    const response = await request('/api/home', { signal: controller.signal })
    if (!activeConflictRequest(controller, requestID, epoch)) { return }
    const body = await jsonBody(response)
    if (!activeConflictRequest(controller, requestID, epoch)) { return }
    if (response.status === 401 && errorCode(body) === 'unauthenticated') {
      await session.expire(epoch, '/home')
      return
    }
    const current = response.status === 200 ? decodeHome(body, '/home') : null
    if (current === null) {
      conflictLoadMessage.value = FAILURE
      return
    }
    conflictHome.value = current
  } catch {
    if (activeConflictGeneration(controller, requestID, epoch)) { conflictLoadMessage.value = NETWORK_FAILURE }
  } finally {
    window.clearTimeout(timeout)
    if (conflictController === controller) {
      conflictController = undefined
      conflictLoading.value = false
    }
  }
}

const confirmSameSet = () => {
  const current = conflictHome.value
  if (!current || !matchingConflictSet.value) { return }
  expected.value = current.collections.map(collection => ({ id: collection.id, version: collection.version }))
  confirmedHome.value = current
  conflictHome.value = null
  conflictCode.value = ''
  saveState.value = 'idle'
  saveMessage.value = '最新状態を確認しました。この順序で保存するには、もう一度「順序を保存」を押してください。'
  intent = null
}

const discardAndReload = () => {
  clearEditor()
  void load()
}

const cancelEditing = () => {
  clearEditor()
  void load()
}
</script>

<template>
  <section aria-labelledby="home-title">
    <h2 id="home-title">ホーム</h2>
    <p v-if="state === 'loading'" role="status">Collectionを読み込んでいます。</p>
    <template v-else-if="state === 'error'">
      <p role="alert">{{ message }}</p>
      <Button label="ホームを再試行" @click="load" />
    </template>
    <template v-else-if="state === 'ready' && data">
      <Button v-if="!editing && data.collections.length > 0" label="表示順を変更" @click="startEditing" />
      <Button v-if="!editing" label="更新" @click="load" />
      <p v-if="visibleHome?.collections.length === 0">表示できるコレクションはありません</p>
      <ol v-else>
        <li v-for="collection in displayedCollections" :key="collection.id" class="home-collection" :data-collection-id="collection.id">
          <h3>{{ collection.name }}</h3>
          <template v-if="editing">
            <span class="home-order-position">{{ (positions.get(collection.id) ?? 0) + 1 }}</span>
            <Button :aria-label="`${collection.name}を上へ`" label="上へ" :disabled="saveState !== 'idle' || positions.get(collection.id) === 0" @click="move(collection.id, -1)" />
            <Button :aria-label="`${collection.name}を下へ`" label="下へ" :disabled="saveState !== 'idle' || positions.get(collection.id) === draftIDs.length - 1" @click="move(collection.id, 1)" />
          </template>
          <p v-if="hasTodayJobs(collection)" class="home-today-status">
            本日（{{ visibleHome?.as_of_date }} JST）：待機中 {{ collection.today_job_counts.queued }}件、実行中 {{ collection.today_job_counts.running }}件、成功 {{ collection.today_job_counts.succeeded }}件、失敗 {{ collection.today_job_counts.failed }}件
          </p>
          <RouterLink v-if="collection.latest_report_id" :to="`/reports/${collection.latest_report_id}`">最新のレポートを開く</RouterLink>
          <p v-else>まだレポートはありません</p>
        </li>
      </ol>
      <template v-if="editing">
        <p v-if="saveMessage" role="status">{{ saveMessage }}</p>
        <Button v-if="saveState !== 'conflict'" :label="saveState === 'unknown' ? '同じ内容で再試行' : '順序を保存'" :disabled="saveState === 'saving'" @click="save" />
        <Button label="キャンセル" :disabled="saveState === 'saving'" @click="cancelEditing" />
        <section v-if="saveState === 'conflict'" aria-label="最新順序との競合">
          <Button label="最新状態を確認" :disabled="conflictLoading" @click="refreshConflict" />
          <p v-if="conflictLoadMessage" role="alert">{{ conflictLoadMessage }}</p>
          <ol v-if="conflictHome" aria-label="現在保存されている順序">
            <li v-for="collection in conflictHome.collections" :key="collection.id">{{ collection.name }}</li>
          </ol>
          <Button v-if="conflictHome && matchingConflictSet" label="最新状態を確認して下書きを維持" @click="confirmSameSet" />
          <Button v-if="conflictHome && !matchingConflictSet" label="下書きを破棄して最新順を再読込" @click="discardAndReload" />
        </section>
      </template>
    </template>
  </section>
</template>
