<script setup lang="ts">
import Button from 'primevue/button'
defineProps<{ message: string }>()
defineEmits<{ retry: [] }>()
</script>
<template><section><p role="alert">{{ message }}</p><Button label="再試行" @click="$emit('retry')" /></section></template>
