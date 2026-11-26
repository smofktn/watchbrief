<script setup lang="ts">
import { computed } from 'vue'
import { renderReportMarkdown } from './markdown'
const props = defineProps<{ markdown: string }>()
// No separate HTML state/cache: resource clearing unmounts this component.
const html = computed(() => renderReportMarkdown(props.markdown))
</script>
<template>
  <!-- The only HTML sink accepts only the final limited renderer result. -->
  <div v-if="html !== null" class="report-body" v-html="html" />
  <template v-else>
    <p role="status">整形表示を利用できないため、保存済み本文をテキストで表示します。</p>
    <pre class="report-body report-text">{{ markdown }}</pre>
  </template>
</template>
