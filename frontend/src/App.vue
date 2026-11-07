<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Card from 'primevue/card'
import InputText from 'primevue/inputtext'

const loginId = ref('')
const password = ref('')
const message = ref('')
const submitting = ref(false)
const inputError = ref('')
const currentLoginId = ref('')
const state = ref<'checking' | 'anonymous' | 'authenticated' | 'error' | 'logging-out' | 'logout-error'>('checking')
const logoutStatus = ref<HTMLElement | null>(null)
const loginTitle = ref<HTMLElement | null>(null)
let currentUserRequest = 0

const loadCurrentUser = async () => {
  const requestId = ++currentUserRequest
  state.value = 'checking'
  currentLoginId.value = ''
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
      state.value = 'anonymous'
      return
    }
    if (response.status === 200 && typeof body === 'object' && body !== null
      && 'login_id' in body && typeof body.login_id === 'string'
      && /^[a-z0-9_-]{1,64}$/.test(body.login_id)) {
      currentLoginId.value = body.login_id
      state.value = 'authenticated'
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

onMounted(loadCurrentUser)

const submit = async () => {
  if (submitting.value || state.value !== 'anonymous') {
    return
  }
  submitting.value = true
  message.value = ''
  inputError.value = ''
  try {
    const response = await fetch('/api/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login_id: loginId.value, password: password.value }),
    })
    if (response.status === 204) {
      password.value = ''
      await loadCurrentUser()
      return
    }
    const body: unknown = await response.json().catch(() => null)
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
    message.value = '接続できません。再度お試しください'
  } finally {
    submitting.value = false
  }
}

const logout = async () => {
  if (state.value !== 'authenticated' && state.value !== 'logout-error') {
    return
  }
  ++currentUserRequest
  currentLoginId.value = ''
  loginId.value = ''
  password.value = ''
  inputError.value = ''
  message.value = ''
  state.value = 'logging-out'
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
        <p v-if="state === 'checking'" role="status">本人情報を確認しています。</p>
        <section v-else-if="state === 'authenticated'" aria-labelledby="current-user-title">
          <h2 id="current-user-title">本人情報</h2>
          <p>ログインID：<span>{{ currentLoginId }}</span></p>
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
