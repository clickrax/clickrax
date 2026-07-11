import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import { bootstrapLocale, i18n } from './i18n/bootstrap'
import { router } from './router'

async function main() {
  createApp(App).use(i18n).use(router).mount('#app')
  void bootstrapLocale()
}

main()
