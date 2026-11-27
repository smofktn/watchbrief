<script setup lang="ts">
import Button from 'primevue/button'
import { RouterLink } from 'vue-router'
import { useHome } from './useHome'

const { state, message, data, load } = useHome()
const hasTodayJobs = (collection: { today_job_counts: { queued: number, running: number, succeeded: number, failed: number } }): boolean => {
  const counts = collection.today_job_counts
  return counts.queued > 0 || counts.running > 0 || counts.succeeded > 0 || counts.failed > 0
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
      <Button label="更新" @click="load" />
      <p v-if="data.collections.length === 0">表示できるコレクションはありません</p>
      <ul v-else>
        <li v-for="collection in data.collections" :key="collection.id" class="home-collection">
          <h3>{{ collection.name }}</h3>
          <p v-if="hasTodayJobs(collection)" class="home-today-status">
            本日（{{ data.as_of_date }} JST）：待機中 {{ collection.today_job_counts.queued }}件、実行中 {{ collection.today_job_counts.running }}件、成功 {{ collection.today_job_counts.succeeded }}件、失敗 {{ collection.today_job_counts.failed }}件
          </p>
          <RouterLink v-if="collection.latest_report_id" :to="`/reports/${collection.latest_report_id}`">最新のレポートを開く</RouterLink>
          <p v-else>まだレポートはありません</p>
        </li>
      </ul>
    </template>
  </section>
</template>
