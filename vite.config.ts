import {defineConfig, type Plugin} from 'vite'
import vue from '@vitejs/plugin-vue'

/** Wails WebView2: no CORS on embedded assets; Vite 7 crossorigin breaks production loads. */
function removeCrossorigin(): Plugin {
  return {
    name: 'remove-crossorigin',
    transformIndexHtml: {
      order: 'post',
      handler(html) {
        return html
          .replace(/<script([^>]*?) crossorigin(?:="[^"]*")?([^>]*)>/gi, '<script$1$2>')
          .replace(/<link([^>]*?) crossorigin(?:="[^"]*")?([^>]*)>/gi, '<link$1$2>')
      },
    },
  }
}

export default defineConfig({
  plugins: [vue(), removeCrossorigin()],
  base: './',
  build: {
    modulePreload: false,
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        inlineDynamicImports: true,
      },
    },
  },
})
