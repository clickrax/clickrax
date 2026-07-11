import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  // Wails embeds frontend/dist; absolute /assets/... paths break in production.
  base: './',
})
