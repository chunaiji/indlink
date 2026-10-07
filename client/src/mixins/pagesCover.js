import { sysApi } from '../api/index'

const CACHE_KEY = 'pages_cover_cfg'

// 多页面图片覆盖(首页/消息/充值/道具/流水/订单/收藏):onShow/onLoad 里先 await loadCover(),
// showCover 为真时页面只显示覆盖图,并 return 跳过原有数据加载与副作用。
// 与同城页不同:所有端都生效(不判断 iOS)。
// 防闪:data() 用上次缓存同步初始化,避免进页面时先闪一下原内容再切覆盖图。
export default {
  data() {
    const cached = uni.getStorageSync(CACHE_KEY) || {}
    return { cover: { on: !!cached.on, image: cached.image || '' } }
  },
  computed: {
    showCover() { return this.cover.on }
  },
  methods: {
    // 拉覆盖图配置并写缓存;失败时保留缓存值(不翻转),避免误闪。
    async loadCover() {
      try {
        const cfg = await sysApi.pagesConfig()
        this.cover = { on: !!cfg.cover_on, image: cfg.cover_image || '' }
        uni.setStorageSync(CACHE_KEY, { on: this.cover.on, image: this.cover.image })
      } catch (e) {}
    }
  }
}
