<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Card from 'primevue/card'
import InputText from 'primevue/inputtext'

const loginId = ref('')
const password = ref('')
const message = ref('')
const submitting = ref(false)
const inputError = ref('')
const currentLoginId = ref('')
const state = ref<'checking' | 'anonymous' | 'authenticated' | 'error'>('checking')

const loadCurrentUser = async () => {
  state.value = 'checking'
  currentLoginId.value = ''
  message.value = ''
  try {
    const response = await fetch('/api/me', { credentials: 'same-origin', cache: 'no-store' })
    const body: unknown = await response.json().catch(() => null)
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
    message.value = '接続できません。再度お試しください'
  }
  state.value = 'error'
}

onMounted(loadCurrentUser)

const submit = async () => {
  if (submitting.value) {
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
        </section>
        <section v-else-if="state === 'error'">
          <p role="alert">{{ message }}</p>
          <Button label="再試行" @click="loadCurrentUser" />
        </section>
        <template v-else>
        <h2>ログイン</h2>
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
