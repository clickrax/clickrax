import {defineConfig, type Plugin} from 'vite'
import vue from '@vitejs/plugin-vue'

/** Wails asset server has no CORS headers; Vite's crossorigin breaks WebView2 loads. */
function removeCrossorigin(): Plugin {
  return {
    name: 'remove-crossorigin',
    transformIndexHtml(html) {
      return html
        .replace(/<script([^>]*?) crossorigin(?:="[^"]*")?([^>]*)>/gi, '<script$1$2>')
        .replace(/<link([^>]*?) crossorigin(?:="[^"]*")?([^>]*)>/gi, '<link$1$2>')
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), removeCrossorigin()],
  // Wails embeds frontend/dist; absolute /assets/... paths break in production.
  base: './',
  build: {
    modulePreload: {polyfill: false},
  },
})
