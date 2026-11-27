<script setup lang="ts">
import Button from 'primevue/button'
import { RouterLink, useRoute } from 'vue-router'
import ReportMarkdown from './ReportMarkdown.vue'
import { useReports } from './useReports'
const route = useRoute()
const { state, message, data, load, nextCursor, moreBusy, moreMessage, loadMore } = useReports()
</script>

<template>
  <section aria-labelledby="reports-title">
    <h2 id="reports-title">保存済みReport（読み取り専用）</h2>
    <RouterLink v-if="route.path === '/reports'" to="/reports/daily">日付別の予定・結果</RouterLink>
    <p v-if="state === 'loading'" role="status">Reportを読み込んでいます。</p>
    <template v-else-if="state === 'error'">
      <p role="alert">{{ message }}</p><Button label="Reportを再試行" @click="load()" />
    </template>
    <p v-else-if="state === 'not-found'" role="alert">見つかりません。</p>
    <template v-else-if="state === 'ready' && data && route.path === '/reports'">
      <p v-if="data.items.length === 0">保存済みReportはありません。</p>
      <ul v-else><li v-for="report in data.items" :key="report.id">
        <RouterLink :to="`/reports/${report.id}`">{{ report.collection_name }}（予定：{{ report.scheduled_at }}）</RouterLink>
      </li></ul>
      <p v-if="moreMessage" role="alert">{{ moreMessage }}</p>
      <p v-if="moreBusy" role="status">続きを読み込んでいます。</p>
      <Button v-if="nextCursor" label="もっと読む" :disabled="moreBusy" @click="loadMore" />
      <p v-else-if="data.items.length">すべての保存済みReportを表示しました。</p>
    </template>
    <article v-else-if="state === 'ready' && data && data.selected">
      <h3>{{ data.selected.collection_name }}</h3>
      <p>確認期間：{{ data.selected.updates_started_at }} 〜 {{ data.selected.updates_ended_at }}</p>
      <p>生成完了：{{ data.selected.generated_at }}</p>
      <ReportMarkdown :markdown="data.selected.body_markdown" />
    </article>
  </section>
</template>
