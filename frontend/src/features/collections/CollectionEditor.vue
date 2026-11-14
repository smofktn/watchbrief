<script setup lang="ts">
import type { DeepReadonly } from 'vue'
import Button from 'primevue/button'
import CollectionFields from './CollectionFields.vue'
import type { Collection } from './data'
import { useCollectionEditing } from './useCollectionEditing'
const props = defineProps<{ collection: DeepReadonly<Collection> }>()
const emit = defineEmits<{ saved: []; cancel: [] }>()
const { form, fields, busy, uncertain, conflict, terminal, latest, message, submit, checkLatest, restart } = useCollectionEditing(props.collection, () => emit('saved'))
</script>
<template>
  <section aria-labelledby="edit-title">
    <h3 id="edit-title" tabindex="-1">コレクション本体の編集</h3>
    <p>名前・調査方針・初回の更新確認期間を保存します。</p>
    <form novalidate :aria-busy="busy" @submit.prevent="submit">
      <CollectionFields :form="form" :fields="fields" :disabled="busy || uncertain || conflict || terminal" />
      <p v-if="message" role="alert">{{ message }}</p>
      <p v-if="busy" role="status">結果を確認しています。</p>
      <Button type="submit" :label="uncertain ? '同じ内容で再試行' : '保存'" :disabled="busy || conflict || terminal" />
      <Button v-if="conflict && !terminal" label="最新値を確認" :disabled="busy" @click="checkLatest" />
      <section v-if="latest" aria-labelledby="latest-title" class="latest-settings">
        <h4 id="latest-title">最新の保存値</h4>
        <p>名前：{{ latest.name }}</p>
        <p>調査方針：{{ latest.research_instruction || '未設定' }}</p>
        <p>初回の更新確認：{{ latest.initial_lookback_mode === 'ai' ? 'AIに任せる' : `${latest.initial_lookback_days}日前から` }}</p>
        <p v-if="latest.enabled">現在はONのため、この画面からは保存できません。</p>
        <p v-else>下書きと見比べてください。「最新値で編集し直す」で入力欄を最新値に置き換えます。</p>
        <Button label="最新値で編集し直す" :disabled="busy || latest.enabled || terminal" @click="restart" />
      </section>
      <Button label="詳細へ戻る" @click="emit('cancel')" />
    </form>
  </section>
</template>
