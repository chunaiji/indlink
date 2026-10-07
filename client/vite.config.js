import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// uni-app CLI 构建配置。微信:npm run dev:mp-weixin / build:mp-weixin
export default defineConfig({
  plugins: [uni()]
})
