<template>
  <view class="tb" :style="{ paddingBottom: safeB + 'px' }">
    <view
      v-for="item in visibleItems"
      :key="item.key"
      class="tb-item"
      :class="{ on: item.index === current }"
      @tap="go(item)"
    >
      <view class="tb-ic">
        <image
          v-if="item.img"
          class="tb-img"
          :src="item.index === current ? item.imgActive : item.img"
          mode="aspectFit"
        />
        <text v-else class="tb-emoji">{{ item.icon }}</text>
        <view v-if="badge[item.key]" class="tb-dot">{{ badge[item.key] > 99 ? '99+' : badge[item.key] }}</view>
      </view>
      <text class="tb-lb">{{ item.text }}</text>
    </view>
  </view>
</template>

<script>
import { sysApi } from '../../api/index'

const IC = '/static/icons/'
const ALL_TABS = [
  { index: 5, key: 'privacy', text: '相机', img: IC + 'paizhao.png', imgActive: IC + 'paizhao_active.png', page: '/pages/privacy/privacy', isExtra: true },
  { index: 0, key: 'home',    text: '首页', img: IC + 'shouye.png',  imgActive: IC + 'shouye_active.png', page: '/pages/ocean/ocean' },
  { index: 1, key: 'city',    text: '同城', img: IC + 'weizhi.png',  imgActive: IC + 'weizhi_active.png', page: '/pages/city/city' },
  { index: 2, key: 'expand',  text: '扩列', img: IC + 'xindong.png', imgActive: IC + 'xindong_1.png',     page: '/pages/expand/expand' },
  { index: 3, key: 'message', text: '消息', img: IC + 'xiaoxi.png',  imgActive: IC + 'xiaoxi_active.png', page: '/pages/message/message' },
  { index: 4, key: 'mine',    text: '我的', img: IC + 'wode.png',    imgActive: IC + 'wode_active.png',   page: '/pages/mine/mine' },
]

export default {
  props: {
    current: { type: Number, default: 0 },
    badge:   { type: Object,  default: () => ({}) }
  },
  data() {
    return {
      safeB: 0,
      // 默认全部隐藏，拉到 /tabs 接口后再按配置显示，避免接口返回前闪出不该显示的 Tab
      visibility: uni.getStorageSync('tab_visibility') || { privacy: false, home: false, city: false, expand: false, message: false, mine: false }
    }
  },
  computed: {
    visibleItems() {
      return ALL_TABS.filter(t => this.visibility[t.key] !== false)
    }
  },
  created() {
    const info = uni.getSystemInfoSync()
    this.safeB = (info.safeAreaInsets && info.safeAreaInsets.bottom) || 0
    this.redirectIfHidden()  // 用缓存立即检查，避免首屏闪烁
    this.fetchVisibility()   // 立即拉服务器配置
  },
  methods: {
    async fetchVisibility() {
      try {
        const res = await sysApi.tabs()
        if (!res) return
        if (JSON.stringify(res) === JSON.stringify(this.visibility)) return
        uni.setStorageSync('tab_visibility', res)
        this.visibility = res
        // 配置可能把当前页对应 tab 隐藏（如相机页是启动页但后台关了相机 tab），需跳到可见页
        this.redirectIfHidden()
      } catch (e) {}
    },
    redirectIfHidden() {
      const visible = this.visibleItems
      if (!visible.length) return
      if (visible.some(t => t.index === this.current)) return
      // 当前页对应的 tab 被隐藏（含相机这类 isExtra 启动页）→ 跳到第一个可见 tab
      const first = visible[0]
      if (first.isExtra) {
        uni.reLaunch({ url: first.page })
      } else {
        uni.switchTab({ url: first.page })
      }
    },
    go(item) {
      if (item.index === this.current) return
      if (item.isExtra) {
        uni.reLaunch({ url: item.page })
        return
      }
      uni.switchTab({ url: item.page })
    }
  }
}
</script>

<style lang="scss">
.tb {
  position: fixed; left: 0; right: 0; bottom: 0; z-index: 999;
  display: flex; background: #fff;
  border-top: 1rpx solid #EDF0F2;
  box-shadow: 0 -4rpx 20rpx rgba(12,42,51,0.07);
}
.tb-item {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  padding: 14rpx 0 8rpx;
}
.tb-ic {
  position: relative; width: 52rpx; height: 52rpx;
  display: flex; align-items: center; justify-content: center;
}
.tb-emoji { font-size: 44rpx; }
.tb-img { width: 48rpx; height: 48rpx; }
.tb-lb { font-size: 20rpx; font-weight: 600; color: #9DBDC8; margin-top: 4rpx; }
.tb-item.on .tb-lb { color: #FF6B5B; }
.tb-dot {
  position: absolute; top: -8rpx; right: -10rpx; min-width: 32rpx; height: 32rpx;
  line-height: 32rpx; text-align: center; background: #FF6B5B; color: #fff;
  border-radius: 999rpx; font-size: 18rpx; font-weight: 700; padding: 0 6rpx;
  border: 3rpx solid #fff;
}
</style>
