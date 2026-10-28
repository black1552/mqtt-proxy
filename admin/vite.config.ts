import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 后端默认监听 :8080。在 `npm run dev` 期间，API 与监控 WebSocket 请求
// 会被代理到那里，以便 SPA 可以跑在 5173 端口。
export default defineConfig({
  base: './',
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1600,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
