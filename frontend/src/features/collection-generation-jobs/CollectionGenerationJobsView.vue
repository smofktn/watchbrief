<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import { useCollectionGenerationJobs } from './useCollectionGenerationJobs'
import { jobDay, jobTime, statusLabel } from '../generation-jobs/data'
import { collectionJobsIDFromPath } from '../../navigation'
import type { CollectionSummary } from '../collections/data'

const route = useRoute()
const router = useRouter()
const { state, message, data, load, nextCursor, moreBusy, moreMessage, loadMore } = useCollectionGenerationJobs()
const selectedID = computed(() => collectionJobsIDFromPath(route.path) ?? '')
const knownCollections = ref<CollectionSummary[]>([])
const selectCollection = (event: Event) => {
  const id = (event.target as HTMLSelectElement).value
  void router.push(id ? `/reports/collections/${id}` : '/reports/collections')
}
watch(() => route.path, path => {
  if (path === '/reports/collections' || collectionJobsIDFromPath(path)) { void load() }
}, { flush: 'post' })
watch(data, value => { if (value) { knownCollections.value = value.collections.map(collection => ({ ...collection })) } }, { immediate: true })
</script>

<template>
  <section aria-labelledby="collection-jobs-title">
    <h2 id="collection-jobs-title">Collection別の予定・結果</h2>
    <RouterLink to="/reports">保存済みReport一覧へ</RouterLink>
    <p>
      <label for="collection-jobs-choice">Collection</label>
      <select id="collection-jobs-choice" :value="selectedID" :disabled="knownCollections.length === 0" @change="selectCollection">
        <option value="">選択してください</option>
        <option v-for="collection in data?.collections ?? knownCollections" :key="collection.id" :value="collection.id">{{ collection.name }}</option>
      </select>
    </p>
    <p v-if="state === 'loading'" role="status">Collectionと予定・結果を読み込んでいます。</p>
    <template v-else-if="state === 'error'">
      <p role="alert">{{ message }}</p><Button label="再試行" @click="load()" />
    </template>
    <p v-else-if="state === 'not-found'" role="alert">見つかりません。</p>
    <template v-else-if="state === 'ready' && data">
      <p v-if="!data.selected && data.collections.length === 0">表示できるCollectionはありません。</p>
      <article v-else-if="data.selected">
        <h3>{{ data.selected.name }}</h3>
        <Button label="更新" @click="load()" />
        <p v-if="data.items.length === 0 && !nextCursor">まだレポートはありません</p>
        <ul v-else>
          <li v-for="job in data.items" :key="job.id" class="collection-job">
            <time :datetime="job.scheduled_at">{{ jobDay(job.scheduled_at) }} {{ jobTime(job.scheduled_at) }}</time>
            — {{ job.collection_name }} — <span>{{ statusLabel[job.status] }}</span>
            <RouterLink v-if="job.status === 'succeeded' && job.report_id" :to="`/reports/${job.report_id}`">Reportを開く</RouterLink>
          </li>
        </ul>
        <p v-if="moreMessage" role="alert">{{ moreMessage }}</p>
        <p v-if="moreBusy" role="status">続きを読み込んでいます。</p>
        <Button v-if="nextCursor" label="もっと読む" :disabled="moreBusy" @click="loadMore" />
        <p v-else-if="data.items.length">すべての予定・結果を表示しました。</p>
      </article>
      <p v-else>Collectionを選択してください。</p>
    </template>
  </section>
</template>
