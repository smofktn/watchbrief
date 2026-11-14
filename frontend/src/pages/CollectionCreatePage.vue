<script setup lang="ts">
import Button from 'primevue/button'
import CollectionFields from '../features/collections/CollectionFields.vue'
import { RouterLink } from 'vue-router'
import { useCollectionCreation } from '../features/collections/useCollectionCreation'
const { form, fields, submitting, uncertain, conflict, message, submit } = useCollectionCreation()
</script>

<template>
  <section aria-labelledby="create-title">
    <h2 id="create-title" tabindex="-1">コレクションの新規作成</h2>
    <p>停止中で保存します。ウォッチ対象と生成スケジュールの設定が必要なため、まだONにはできません。</p>
    <form novalidate :aria-busy="submitting" @submit.prevent="submit">
      <CollectionFields :form="form" :fields="fields" :disabled="submitting || uncertain" />
      <p v-if="message" role="alert">{{ message }}</p>
      <p v-if="submitting" role="status">保存結果を確認しています。</p>
      <Button type="submit" :label="uncertain ? '同じ内容で再試行' : '保存'" :disabled="submitting || conflict" />
      <RouterLink to="/collections">一覧へ戻る</RouterLink>
    </form>
  </section>
</template>
