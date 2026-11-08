<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { useSession } from './session'
const { loginId, password, inputError, submitting, message, submit } = useSession()
</script>
<template>
  <h2 data-login-title tabindex="-1">ログイン</h2>
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
