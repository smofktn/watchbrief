<script setup lang="ts">
import Button from 'primevue/button'
import type { SourceDraft } from './sourceForm'
defineProps<{ sources: SourceDraft[]; targetIndex: number; disabled: boolean }>()
const emit = defineEmits<{ add: []; remove: [index: number] }>()
</script>
<template>
  <section :aria-labelledby="`sources-title-${targetIndex}`">
    <h5 :id="`sources-title-${targetIndex}`" tabindex="-1">情報源</h5>
    <p>1回の保存で情報源を20行まで追加・変更できます。URLのページ取得は行いません。</p>
    <p>URLにユーザー名・パスワードなどの認証情報を含めないでください。</p>
    <p v-if="sources.length === 0">登録情報源はありません。</p>
    <fieldset v-for="(source, index) in sources" :key="source.key" :disabled="disabled">
      <legend>情報源 {{ index + 1 }}{{ source.id === null ? '（未保存）' : '' }}</legend>
      <label :for="`source-name-${source.key}`">表示名（必須・200文字以内）</label>
      <input :id="`source-name-${source.key}`" v-model="source.name" type="text" :aria-invalid="source.errorField === 'name'" :aria-describedby="source.error ? `source-error-${source.key}` : undefined" />
      <label :for="`source-url-${source.key}`">URL（必須・HTTP/HTTPS・2,048文字以内）</label>
      <input :id="`source-url-${source.key}`" v-model="source.url" type="text" inputmode="url" :aria-invalid="source.errorField === 'url'" :aria-describedby="source.error ? `source-error-${source.key}` : undefined" />
      <label :for="`source-instruction-${source.key}`">補足指示（任意・10,000文字以内）</label>
      <textarea :id="`source-instruction-${source.key}`" v-model="source.instruction" :aria-invalid="source.errorField === 'instruction'" :aria-describedby="source.error ? `source-error-${source.key}` : undefined" />
      <p v-if="source.error" :id="`source-error-${source.key}`" role="alert">{{ source.error }}</p>
      <Button v-if="source.id === null" type="button" label="情報源の追加を取り消す" @click="emit('remove', index)" />
    </fieldset>
    <Button type="button" label="情報源を追加" :disabled="disabled" @click="emit('add')" />
  </section>
</template>
<style scoped>
fieldset { margin-block: 1rem; min-width: 0; }
label, input, textarea { display: block; }
input, textarea { box-sizing: border-box; width: 100%; margin-block: .4rem .8rem; font: inherit; }
textarea { min-height: 6rem; }
</style>
