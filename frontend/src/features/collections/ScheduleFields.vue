<script setup lang="ts">
import InputText from 'primevue/inputtext'
defineProps<{ form: { cron: string }; fields: { cron: string }; disabled: boolean; existing: boolean }>()
</script>
<template>
  <fieldset :disabled="disabled">
    <legend>生成スケジュール（日本時間）</legend>
    <label for="schedule-cron">cron式</label>
    <InputText id="schedule-cron" v-model="form.cron" :aria-invalid="Boolean(fields.cron)" aria-describedby="schedule-help schedule-error" autocomplete="off" />
    <p id="schedule-help">例：0 7 * * * は毎日7時、0 7,19 * * * は毎日7時と19時です。分・時・日・月・曜日の5欄を半角空白で区切ります。数値・*・列挙（,）・範囲（-）・間隔（*/n、範囲/n）が使えます。曜日は0/7が日曜、1が月曜です。256文字以内。祝日除外はありません。</p>
    <p v-if="existing">登録済みの式を空欄にして削除することはできません。</p>
    <p v-else>空欄のままならスケジュールを追加しません。</p>
    <p>保存してもコレクションは停止中のままで、生成は始まりません。</p>
    <p id="schedule-error" role="alert" v-if="fields.cron">{{ fields.cron }}</p>
  </fieldset>
</template>
