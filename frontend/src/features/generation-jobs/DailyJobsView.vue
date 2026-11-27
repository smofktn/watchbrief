<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import Button from 'primevue/button'
import { useGenerationJobs } from './useGenerationJobs'
import { groupJobs, jobTime, statusLabel } from './data'
const { state, message, data, load, nextCursor, moreBusy, moreMessage, loadMore } = useGenerationJobs()
const groups = computed(() => groupJobs(data.value?.items ?? []))
</script>
<template>
  <section aria-labelledby="daily-title">
    <h2 id="daily-title">日付別の予定・結果</h2>
    <p>予定時刻は日本時間です。</p>
    <RouterLink to="/reports">保存済みReport一覧へ</RouterLink>
    <Button label="更新" :disabled="state === 'loading'" @click="load()" />
    <p v-if="state === 'loading'" role="status">予定・結果を読み込んでいます。</p>
    <template v-else-if="state === 'error'">
      <p role="alert">{{ message }}</p><Button label="予定・結果を再試行" @click="load()" />
    </template>
    <template v-else-if="state === 'ready' && data">
      <p v-if="!data.items.length">保存済みの予定・結果はありません。</p>
      <section v-for="group in groups" :key="group.day" class="job-day">
        <h3>{{ group.day }}</h3>
        <ul><li v-for="job in group.items" :key="job.id" class="daily-job">
          <time :datetime="job.scheduled_at">{{ jobTime(job.scheduled_at) }}</time>
          {{ job.collection_name }} — <span class="job-status">{{ statusLabel[job.status] }}</span>
          <RouterLink v-if="job.status === 'succeeded' && job.report_id" :to="`/reports/${job.report_id}`">Reportを開く</RouterLink>
        </li></ul>
      </section>
      <p v-if="moreMessage" role="alert">{{ moreMessage }}</p>
      <p v-if="moreBusy" role="status">続きを読み込んでいます。</p>
      <Button v-if="nextCursor" label="もっと読む" :disabled="moreBusy" @click="loadMore" />
      <p v-else-if="data.items.length">すべての保存済み予定・結果を表示しました。</p>
    </template>
  </section>
</template>
