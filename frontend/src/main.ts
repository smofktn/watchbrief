import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import App from './App.vue'
import './base.css'

createApp(App)
  .use(PrimeVue, { theme: { preset: Aura } })
  .mount('#app')
