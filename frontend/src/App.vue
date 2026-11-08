<script setup lang="ts">
import { useRouter } from 'vue-router'
import AppLayout from './layouts/AppLayout.vue'
import { provideSession } from './features/auth/session'
provideSession(useRouter())
</script>
<template><AppLayout /></template>
