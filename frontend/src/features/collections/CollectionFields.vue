<script setup lang="ts">
import InputText from 'primevue/inputtext'
import type { CollectionForm, CollectionFields } from './form'
defineProps<{ form: CollectionForm; fields: CollectionFields; disabled: boolean }>()
</script>
<template>
  <fieldset :disabled="disabled">
    <legend>コレクションの設定</legend>
    <div class="field">
      <label for="collection-name">名前（必須）</label>
      <InputText id="collection-name" v-model="form.name" :aria-invalid="!!fields.name" aria-describedby="collection-name-hint collection-name-error" required />
      <small id="collection-name-hint">1〜200文字。空白だけでは保存できません。</small>
      <p v-if="fields.name" id="collection-name-error" role="alert">{{ fields.name }}</p>
    </div>
    <div class="field">
      <label for="collection-instruction">調査方針（任意）</label>
      <textarea id="collection-instruction" v-model="form.instruction" rows="5" :aria-invalid="!!fields.instruction" aria-describedby="collection-instruction-hint collection-instruction-error" />
      <small id="collection-instruction-hint">10,000文字以内。例：新モデルや研究発表を重視する。</small>
      <p v-if="fields.instruction" id="collection-instruction-error" role="alert">{{ fields.instruction }}</p>
    </div>
    <div class="field">
      <label for="collection-mode">初回の更新確認期間</label>
      <select id="collection-mode" v-model="form.mode">
        <option value="ai">AIにおまかせ</option>
        <option value="days">日数を指定</option>
      </select>
      <small>初回だけに使う設定です。2回目以降は前回成功分から確認します。</small>
    </div>
    <div v-if="form.mode === 'days'" class="field">
      <label for="collection-days">遡る日数（必須）</label>
      <InputText id="collection-days" v-model="form.days" inputmode="numeric" :aria-invalid="!!fields.days" aria-describedby="collection-days-hint collection-days-error" required />
      <small id="collection-days-hint">1〜365日の整数で指定してください。</small>
      <p v-if="fields.days" id="collection-days-error" role="alert">{{ fields.days }}</p>
    </div>
  </fieldset>
</template>
