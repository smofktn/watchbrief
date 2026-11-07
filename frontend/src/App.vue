<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import Button from 'primevue/button'
import Card from 'primevue/card'
import InputText from 'primevue/inputtext'
import { isCollection, isCollectionSummary, type Collection, type CollectionSummary } from './collection-data'
import { collectionIDFromPath, canonicalLoginPath, DEFAULT_PROTECTED_PATH, isProtectedPath, loginDestination, loginPathFor, LOGIN_PATH } from './navigation'

const loginId = ref('')
const password = ref('')
const message = ref('')
const submitting = ref(false)
const inputError = ref('')
const currentLoginId = ref('')
const state = ref<'checking' | 'anonymous' | 'authenticated' | 'error' | 'logging-out' | 'logout-error' | 'not-found'>('checking')
const routePath = ref(window.location.pathname)
const logoutStatus = ref<HTMLElement | null>(null)
const loginTitle = ref<HTMLElement | null>(null)
let currentUserRequest = 0
const collections = ref<CollectionSummary[]>([])
const selectedCollection = ref<Collection | null>(null)
const settingsState = ref<'idle' | 'loading' | 'ready' | 'error' | 'not-found'>('idle')
const settingsMessage = ref('')
const clearSettings = () => {
  collections.value = []
  selectedCollection.value = null
  settingsMessage.value = ''
  settingsState.value = 'idle'
}
const collectionPath = (id: string) => `/collections/${id}`
const navigateTo = (path: string) => {
  if (!isProtectedPath(path)) {
    return
  }
  window.history.pushState(null, '', path)
  handleNavigation()
}
const loadSettings = async (requestId = currentUserRequest) => {
  clearSettings()
  const path = routePath.value
  if (path !== '/collections' && collectionIDFromPath(path) === null) {
    return
  }
  settingsState.value = 'loading'
  try {
    const response = await fetch(`/api${path}`, { credentials: 'same-origin', cache: 'no-store' })
    const body: unknown = await response.json().catch(() => null)
    if (requestId !== currentUserRequest) {
      return
    }
    const code = typeof body === 'object' && body !== null && 'error' in body
      && typeof body.error === 'object' && body.error !== null && 'code' in body.error
      ? body.error.code : undefined
    if (response.status === 401 && code === 'unauthenticated') {
      ++currentUserRequest
      clearSettings()
      currentLoginId.value = ''
      replaceLocation(loginPathFor(path))
      state.value = 'anonymous'
      return
    }
    if (response.status === 404 && code === 'not_found') {
      settingsState.value = 'not-found'
      return
    }
    if (response.status === 200 && typeof body === 'object' && body !== null) {
      if (path === '/collections' && 'collections' in body && Array.isArray(body.collections) && body.collections.every(isCollectionSummary)) {
        collections.value = body.collections
        settingsState.value = 'ready'
        return
      }
      if (isCollection(body) && body.id === collectionIDFromPath(path)) {
        selectedCollection.value = body
        settingsState.value = 'ready'
        return
      }
    }
    settingsMessage.value = '処理に失敗しました。時間をおいて再度お試しください'
  } catch {
    if (requestId !== currentUserRequest) {
      return
    }
    settingsMessage.value = '接続できません。再度お試しください'
  }
  settingsState.value = 'error'
}

const replaceLocation = (path: string) => {
  window.history.replaceState(null, '', path)
  routePath.value = window.location.pathname
}

const loadCurrentUser = async () => {
  const requestId = ++currentUserRequest
  state.value = 'checking'
  currentLoginId.value = ''
  clearSettings()
  message.value = ''
  try {
    const response = await fetch('/api/me', { credentials: 'same-origin', cache: 'no-store' })
    const body: unknown = await response.json().catch(() => null)
    // A late identity response must not restore data after logout starts.
    if (requestId !== currentUserRequest) {
      return
    }
    if (response.status === 401 && typeof body === 'object' && body !== null
      && 'error' in body && typeof body.error === 'object' && body.error !== null
      && 'code' in body.error && body.error.code === 'unauthenticated') {
      if (isProtectedPath(routePath.value)) {
        replaceLocation(loginPathFor(routePath.value))
      }
      state.value = 'anonymous'
      return
    }
    if (response.status === 200 && typeof body === 'object' && body !== null
      && 'login_id' in body && typeof body.login_id === 'string'
      && /^[a-z0-9_-]{1,64}$/.test(body.login_id)) {
      if (routePath.value === LOGIN_PATH) {
        replaceLocation(loginDestination(window.location.search))
      }
      currentLoginId.value = body.login_id
      state.value = 'authenticated'
      await loadSettings(requestId)
      return
    }
    message.value = '処理に失敗しました。時間をおいて再度お試しください'
  } catch {
    if (requestId !== currentUserRequest) {
      return
    }
    message.value = '接続できません。再度お試しください'
  }
  state.value = 'error'
}

const handleNavigation = () => {
  // Browser history must not bypass an unconfirmed logout or restore private data.
  if (state.value === 'logging-out' || state.value === 'logout-error') {
    replaceLocation(LOGIN_PATH)
    return
  }
  ++currentUserRequest
  clearSettings()
  loginId.value = ''
  password.value = ''
  currentLoginId.value = ''
  inputError.value = ''
  submitting.value = false
  routePath.value = window.location.pathname
  if (routePath.value === '/') {
    replaceLocation(DEFAULT_PROTECTED_PATH)
  } else if (routePath.value === LOGIN_PATH) {
    replaceLocation(canonicalLoginPath(window.location.search))
  } else if (isProtectedPath(routePath.value)) {
    // This screen currently has no supported query parameters or fragments.
    replaceLocation(routePath.value)
  } else {
    state.value = 'not-found'
    return
  }
  void loadCurrentUser()
}

const handlePageShow = (event: PageTransitionEvent) => {
  if (event.persisted) {
    handleNavigation()
  }
}

onMounted(() => {
  window.addEventListener('popstate', handleNavigation)
  window.addEventListener('pageshow', handlePageShow)
  handleNavigation()
})

onUnmounted(() => {
  ++currentUserRequest
  window.removeEventListener('popstate', handleNavigation)
  window.removeEventListener('pageshow', handlePageShow)
})

const submit = async () => {
  if (submitting.value || state.value !== 'anonymous') {
    return
  }
  submitting.value = true
  const requestId = ++currentUserRequest
  message.value = ''
  inputError.value = ''
  try {
    const response = await fetch('/api/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login_id: loginId.value, password: password.value }),
    })
    if (requestId !== currentUserRequest) {
      return
    }
    if (response.status === 204) {
      password.value = ''
      submitting.value = false
      await loadCurrentUser()
      return
    }
    const body: unknown = await response.json().catch(() => null)
    if (requestId !== currentUserRequest) {
      return
    }
    const code = typeof body === 'object' && body !== null && 'error' in body
      && typeof body.error === 'object' && body.error !== null && 'code' in body.error
      ? body.error.code : undefined
    if (response.status === 400 && code === 'invalid_request') {
      inputError.value = 'ログインIDは英数字・_・-の1〜64文字、パスワードは15〜128文字で入力してください。'
    } else if (response.status === 401 && code === 'invalid_credentials') {
      message.value = 'ログインIDまたはパスワードが違います'
    } else if (response.status === 403 && code === 'csrf_rejected') {
      // Provisional guidance until the CSRF error wording is agreed.
      message.value = '送信元を確認できません。画面を再読み込みしてください。'
    } else {
      message.value = '処理に失敗しました。時間をおいて再度お試しください'
    }
  } catch {
    if (requestId === currentUserRequest) {
      message.value = '接続できません。再度お試しください'
    }
  } finally {
    if (requestId === currentUserRequest) {
      submitting.value = false
    }
  }
}

const logout = async () => {
  if (state.value !== 'authenticated' && state.value !== 'logout-error') {
    return
  }
  ++currentUserRequest
  currentLoginId.value = ''
  clearSettings()
  loginId.value = ''
  password.value = ''
  inputError.value = ''
  message.value = ''
  state.value = 'logging-out'
  replaceLocation(LOGIN_PATH)
  await nextTick()
  logoutStatus.value?.focus()

  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 15_000)
  try {
    const response = await fetch('/api/logout', {
      method: 'POST',
      credentials: 'same-origin',
      cache: 'no-store',
      signal: controller.signal,
    })
    // Only a confirmed 204 completes logout. Do not reload identity on failure.
    state.value = response.status === 204 ? 'anonymous' : 'logout-error'
  } catch {
    state.value = 'logout-error'
  } finally {
    window.clearTimeout(timeout)
    await nextTick()
    if (state.value === 'anonymous') {
      loginTitle.value?.focus()
    } else {
      logoutStatus.value?.focus()
    }
  }
}
</script>

<template>
  <main>
    <Card>
      <template #title><h1>WatchBrief</h1></template>
      <template #content>
        <section v-if="state === 'not-found'">
          <h2>ページが見つかりません。</h2>
          <a href="/me">本人情報へ</a>
        </section>
        <p v-else-if="state === 'checking'" role="status">本人情報を確認しています。</p>
        <section v-else-if="state === 'authenticated'" aria-labelledby="current-user-title">
          <h2 id="current-user-title">本人情報</h2>
          <p>ログインID：<span>{{ currentLoginId }}</span></p>
          <nav aria-label="確認する情報">
            <a href="/me" @click.prevent="navigateTo('/me')">本人情報</a>
            <span> / </span><a href="/collections" @click.prevent="navigateTo('/collections')">コレクション設定</a>
          </nav>
          <section v-if="routePath === '/collections' || collectionIDFromPath(routePath) !== null" aria-labelledby="settings-title">
            <h2 id="settings-title">コレクション設定（読み取り専用）</h2>
            <p v-if="settingsState === 'loading'" role="status">設定を読み込んでいます。</p>
            <template v-else-if="settingsState === 'error'">
              <p role="alert">{{ settingsMessage }}</p><Button label="設定を再試行" @click="loadSettings()" />
            </template>
            <p v-else-if="settingsState === 'not-found'" role="alert">見つかりません。</p>
            <template v-else-if="settingsState === 'ready' && routePath === '/collections'">
              <p v-if="collections.length === 0">コレクションはありません。</p>
              <ul v-else><li v-for="collection in collections" :key="collection.id">
                <a :href="collectionPath(collection.id)" @click.prevent="navigateTo(collectionPath(collection.id))">{{ collection.name }}</a>
                <span>（{{ collection.enabled ? 'ON' : 'OFF' }}）</span>
              </li></ul>
            </template>
            <article v-else-if="settingsState === 'ready' && selectedCollection">
              <h3>{{ selectedCollection.name }}</h3>
              <p>状態：{{ selectedCollection.enabled ? 'ON' : 'OFF' }}</p>
              <p>調査方針：{{ selectedCollection.research_instruction || '未設定' }}</p>
              <p>初回の更新確認：{{ selectedCollection.initial_lookback_mode === 'ai' ? 'AIに任せる' : `${selectedCollection.initial_lookback_days}日前から` }}</p>
              <p>生成予定（日本時間）：{{ selectedCollection.schedule?.cron_expression ?? '未設定' }}</p>
              <p v-if="selectedCollection.targets.length === 0">ウォッチ対象はありません。</p>
              <section v-for="target in selectedCollection.targets" :key="target.id">
                <h4>{{ target.name }}（{{ target.enabled ? 'ON' : 'OFF' }}）</h4>
                <p>補足指示：{{ target.instruction || '未設定' }}</p>
                <p v-if="target.sources.length === 0">登録情報源はありません。</p>
                <ul v-else><li v-for="source in target.sources" :key="source.id">
                  {{ source.name }}：{{ source.url }}<p>{{ source.instruction }}</p>
                </li></ul>
              </section>
            </article>
          </section>
          <Button label="ログアウト" @click="logout" />
        </section>
        <section v-else-if="state === 'logging-out'" aria-busy="true">
          <p ref="logoutStatus" tabindex="-1" role="status">ログアウトしています。</p>
        </section>
        <section v-else-if="state === 'logout-error'">
          <p ref="logoutStatus" tabindex="-1" role="alert">ログアウトを確認できません。再試行してください。</p>
          <Button label="再試行" @click="logout" />
        </section>
        <section v-else-if="state === 'error'">
          <p role="alert">{{ message }}</p>
          <Button label="再試行" @click="loadCurrentUser" />
        </section>
        <template v-else>
        <h2 ref="loginTitle" tabindex="-1">ログイン</h2>
        <p id="login-note">ログインIDとパスワードを入力してください。</p>
        <form aria-describedby="login-note" @submit.prevent="submit">
          <div class="field">
            <label for="login-id">ログインID</label>
            <InputText id="login-id" v-model="loginId" name="username" autocomplete="username" :disabled="submitting" :aria-describedby="inputError ? 'input-error' : undefined" required />
          </div>
          <div class="field">
            <label for="password">パスワード</label>
            <InputText id="password" v-model="password" name="password" type="password" autocomplete="current-password" :disabled="submitting" :aria-describedby="inputError ? 'password-hint input-error' : 'password-hint'" required />
            <small id="password-hint">15〜128文字。空白・大文字・小文字は区別します。</small>
            <p v-if="inputError" id="input-error" role="alert">{{ inputError }}</p>
          </div>
          <Button type="submit" label="ログイン" :disabled="submitting" />
          <p role="status">{{ submitting ? '送信中です。' : message }}</p>
        </form>
        </template>
      </template>
    </Card>
  </main>
</template>
