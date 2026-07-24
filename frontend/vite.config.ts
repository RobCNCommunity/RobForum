import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

const nutUICspCompatibility = {
  name: 'nutui-csp-compatibility',
  enforce: 'pre' as const,
  transform(code: string, id: string) {
    const normalizedID = id.replace(/\\/g, '/')
    if (normalizedID.endsWith('/@nutui/nutui/dist/packages/_es/Notify.js')) {
      return code.replace('unmount: new Function()', 'unmount: () => {}')
    }
    if (normalizedID.endsWith('/@nutui/nutui/dist/style.css')) {
      return code.replace(
        /@font-face\{font-family:nutui-iconfont;src:url\(https:\/\/storage\.360buyimg\.com\/nutui\/3x\/static\/iconfont\.woff2\?t=1668762221765\) format\("woff2"\),url\(https:\/\/storage\.360buyimg\.com\/nutui\/3x\/static\/iconfont\.woff\?t=1668762221765\) format\("woff"\),url\(https:\/\/storage\.360buyimg\.com\/nutui\/3x\/static\/iconfont\.ttf\?t=1668762221765\) format\("truetype"\)\}/,
        '@font-face{font-family:nutui-iconfont;src:url("/fonts/nutui-iconfont.woff2") format("woff2");font-display:block}',
      )
    }
  },
}

export default defineConfig({
  plugins: [nutUICspCompatibility, vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY || 'http://127.0.0.1:8088',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: { sourcemap: false }
})
