<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { RouterLink } from 'vue-router'
import { useCollectionCreation } from '../features/collections/useCollectionCreation'
const { form, fields, submitting, uncertain, conflict, message, submit } = useCollectionCreation()
</script>

<template>
  <section aria-labelledby="create-title">
    <h2 id="create-title" tabindex="-1">コレクションの新規作成</h2>
    <p>停止中で保存します。ウォッチ対象と生成スケジュールの設定が必要なため、まだONにはできません。</p>
    <form novalidate :aria-busy="submitting" @submit.prevent="submit">
      <fieldset :disabled="submitting || uncertain">
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
      <p v-if="message" role="alert">{{ message }}</p>
      <p v-if="submitting" role="status">保存結果を確認しています。</p>
      <Button type="submit" :label="uncertain ? '同じ内容で再試行' : '保存'" :disabled="submitting || conflict" />
      <RouterLink to="/collections">一覧へ戻る</RouterLink>
    </form>
  </section>
</template>
