<script setup lang="ts">
import Button from 'primevue/button'
import SourceFields from './SourceFields.vue'
import type { TargetDraft } from './targetForm'
defineProps<{ targets: TargetDraft[]; disabled: boolean }>()
const emit = defineEmits<{ add: []; remove: [index: number]; addSource: [index: number]; removeSource: [targetIndex: number, sourceIndex: number] }>()
</script>
<template>
  <section aria-labelledby="targets-title">
    <h4 id="targets-title" tabindex="-1">ウォッチ対象</h4>
    <p>新しい対象はONで保存します。コレクションは停止中のままです。</p>
    <p>1回に保存できる対象の追加・変更は20行までです。</p>
    <p v-if="targets.length === 0">ウォッチ対象はありません。</p>
    <fieldset v-for="(target, index) in targets" :key="target.id ?? `new-${index}`" :disabled="disabled">
      <legend>対象 {{ index + 1 }}{{ target.id === null ? '（未保存）' : '' }}</legend>
      <label :for="`target-name-${index}`">対象名（必須・200文字以内）</label>
      <input :id="`target-name-${index}`" v-model="target.name" type="text" :aria-invalid="!!target.error" :aria-describedby="target.error ? `target-error-${index}` : undefined" />
      <label :for="`target-instruction-${index}`">補足指示（任意・10,000文字以内）</label>
      <textarea :id="`target-instruction-${index}`" v-model="target.instruction" :aria-invalid="!!target.error" :aria-describedby="target.error ? `target-error-${index}` : undefined" />
      <p v-if="target.error" :id="`target-error-${index}`" role="alert">{{ target.error }}</p>
      <SourceFields v-if="target.id !== null" :sources="target.sources" :target-index="index" :disabled="disabled" @add="emit('addSource', index)" @remove="emit('removeSource', index, $event)" />
      <p v-else>対象を保存してから情報源を追加してください。</p>
      <Button v-if="target.id === null" type="button" label="追加を取り消す" @click="emit('remove', index)" />
    </fieldset>
    <Button type="button" label="対象を追加" :disabled="disabled" @click="emit('add')" />
  </section>
</template>
<style scoped>
fieldset { margin-block: 1rem; min-width: 0; }
label, input, textarea { display: block; }
input, textarea { box-sizing: border-box; width: 100%; margin-block: .4rem .8rem; font: inherit; }
textarea { min-height: 6rem; }
</style>
