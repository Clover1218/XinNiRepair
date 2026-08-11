import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    proxy: {
      // 本地联调：/api/* 转发到本地后端，避免浏览器 CORS
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
