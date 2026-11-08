<script setup lang="ts">
import Card from 'primevue/card'
import Button from 'primevue/button'
import { RouterLink, RouterView } from 'vue-router'
import { useSession } from '../features/auth/session'
import ErrorNotice from '../components/ErrorNotice.vue'
import { focusSession } from './focusSession'
const session = useSession()
const { state, currentLoginId, message, logout, loadCurrentUser } = session
focusSession(session)
</script>
<template>
  <main><Card>
    <template #title><h1>WatchBrief</h1></template>
    <template #content>
      <p v-if="state === 'checking'" role="status">本人情報を確認しています。</p>
      <section v-else-if="state === 'authenticated'" aria-labelledby="current-user-title">
        <h2 id="current-user-title">本人情報</h2>
        <p>ログインID：<span>{{ currentLoginId }}</span></p>
        <nav aria-label="確認する情報">
          <RouterLink to="/me">本人情報</RouterLink>
          <span> / </span><RouterLink to="/collections">コレクション設定</RouterLink>
          <span> / </span><RouterLink to="/reports">保存済みReport</RouterLink>
        </nav>
        <RouterView />
        <Button label="ログアウト" @click="logout" />
      </section>
      <section v-else-if="state === 'logging-out'" aria-busy="true">
        <p data-logout-status tabindex="-1" role="status">ログアウトしています。</p>
      </section>
      <section v-else-if="state === 'logout-error'">
        <p data-logout-status tabindex="-1" role="alert">ログアウトを確認できません。再試行してください。</p>
        <Button label="再試行" @click="logout" />
      </section>
      <ErrorNotice v-else-if="state === 'error'" :message="message" @retry="loadCurrentUser" />
      <RouterView v-else />
    </template>
  </Card></main>
</template>
