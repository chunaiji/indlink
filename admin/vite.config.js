import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// base 用相对路径,hash 路由,构建产物可部署到任意子路径(如 /admin)。
export default defineConfig({
  base: './',
  plugins: [vue()],
  server: {
    port: 5174,
    proxy: {
      // 开发期把后台 API 代理到 Go 服务,避免跨域
      '/admin/api': {
        target: 'http://localhost:8980',
        changeOrigin: true
      }
    }
  }
})
