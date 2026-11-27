<script setup lang="ts">
import Button from 'primevue/button'
import { RouterLink } from 'vue-router'
import { useHome } from './useHome'

const { state, message, data, load } = useHome()
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
      <Button label="更新" @click="load" />
      <p v-if="data.collections.length === 0">表示できるコレクションはありません</p>
      <ul v-else>
        <li v-for="collection in data.collections" :key="collection.id" class="home-collection">
          <h3>{{ collection.name }}</h3>
          <RouterLink v-if="collection.latest_report_id" :to="`/reports/${collection.latest_report_id}`">最新のレポートを開く</RouterLink>
          <p v-else>まだレポートはありません</p>
        </li>
      </ul>
    </template>
  </section>
</template>
