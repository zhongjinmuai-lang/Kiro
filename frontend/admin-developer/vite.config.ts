import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': resolve(__dirname, 'src') } },
  server: {
    port: 3000,
    proxy: {
      // API 接口 → API Server (8080)
      '/api':    { target: 'http://localhost:8080', changeOrigin: true },
      // 管理后台 → Admin Server (8081)
      '/admin':  { target: 'http://localhost:8081', changeOrigin: true },
      // 智能体引擎 → Agent Engine (8082)
      '/agent':  { target: 'http://localhost:8082', changeOrigin: true },
      // 通用接口
      '/version':{ target: 'http://localhost:8080', changeOrigin: true },
      '/health': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
