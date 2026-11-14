<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import CollectionEditor from './CollectionEditor.vue'
import Button from 'primevue/button'
import { RouterLink, useRoute } from 'vue-router'
import { useCollections } from './useCollections'
const editing = ref(false)
const startEditing = async () => {
  editing.value = true
  await nextTick()
  document.getElementById('edit-title')?.focus()
}
const finishEditing = async () => {
  editing.value = false
  await load()
  await nextTick()
  document.getElementById('detail-title')?.focus()
}
const route = useRoute()
const { state, message, data, load } = useCollections()
watch(() => route.path, () => { editing.value = false }, { flush: 'sync' })
</script>

<template>
  <section class="collection-settings" aria-labelledby="settings-title">
    <h2 id="settings-title">コレクション設定</h2>
    <p v-if="route.path === '/collections'"><RouterLink to="/collections/new">新規作成</RouterLink></p>
    <p v-if="state === 'loading'" role="status">設定を読み込んでいます。</p>
    <template v-else-if="state === 'error'">
      <p role="alert">{{ message }}</p><Button label="設定を再試行" @click="load()" />
    </template>
    <p v-else-if="state === 'not-found'" role="alert">見つかりません。</p>
    <template v-else-if="state === 'ready' && data && route.path === '/collections'">
      <p v-if="data.items.length === 0">コレクションはありません。</p>
      <ul v-else><li v-for="collection in data.items" :key="collection.id">
        <RouterLink :to="`/collections/${collection.id}`">{{ collection.name }}</RouterLink>
        <span>（{{ collection.enabled ? 'ON' : 'OFF' }}）</span>
      </li></ul>
    </template>
    <CollectionEditor v-else-if="editing && state === 'ready' && data?.selected" :collection="data.selected" @saved="finishEditing" @cancel="finishEditing" />
    <article v-else-if="state === 'ready' && data && data.selected">
      <h3 id="detail-title" tabindex="-1">{{ data.selected.name }}</h3>
      <Button v-if="!data.selected.enabled" label="編集" @click="startEditing" />
      <p>状態：{{ data.selected.enabled ? 'ON' : 'OFF' }}</p>
      <p>調査方針：{{ data.selected.research_instruction || '未設定' }}</p>
      <p>初回の更新確認：{{ data.selected.initial_lookback_mode === 'ai' ? 'AIに任せる' : `${data.selected.initial_lookback_days}日前から` }}</p>
      <p>生成予定（日本時間）：{{ data.selected.schedule?.cron_expression ?? '未設定' }}</p>
      <p v-if="data.selected.targets.length === 0">ウォッチ対象はありません。</p>
      <section v-for="target in data.selected.targets" :key="target.id">
        <h4>{{ target.name }}（{{ target.enabled ? 'ON' : 'OFF' }}）</h4>
        <p>補足指示：{{ target.instruction || '未設定' }}</p>
        <p v-if="target.sources.length === 0">登録情報源はありません。</p>
        <ul v-else><li v-for="source in target.sources" :key="source.id">
          {{ source.name }}：{{ source.url }}<p>{{ source.instruction }}</p>
        </li></ul>
      </section>
    </article>
  </section>
</template>
