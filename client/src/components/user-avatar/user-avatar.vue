<template>
  <view class="ua" :style="boxStyle">
    <image v-if="src" :src="src" mode="aspectFill" class="ua-img" :style="rStyle" />
    <view v-else class="ua-fb" :style="fbStyle">{{ initial }}</view>
  </view>
</template>

<script>
// 企业级默认头像:有图显示图,无图用昵称首字 + 由名字确定性派生的柔和渐变。
const GRADIENTS = [
  ['#FFD8C2', '#FFB59B'], ['#BFE6F0', '#8FD3E4'], ['#E7D8FF', '#C9B6F5'],
  ['#C9F0DC', '#9BDCBE'], ['#FFE3B3', '#FFC97A'], ['#D6E2FF', '#A9C2F5'],
  ['#FAD1E0', '#F4A8C6'], ['#CDEFF0', '#97D9DC']
]
export default {
  name: 'user-avatar',
  props: {
    name: { type: String, default: '' },
    src: { type: String, default: '' },
    size: { type: Number, default: 96 }, // rpx
    shape: { type: String, default: 'square' } // square | circle
  },
  computed: {
    initial() {
      const n = (this.name || '').trim()
      if (!n) return '友'
      const c = n[0]
      return /[a-zA-Z]/.test(c) ? c.toUpperCase() : c
    },
    grad() {
      const n = this.name || ''
      let h = 0
      for (let i = 0; i < n.length; i++) h = (h * 31 + n.charCodeAt(i)) >>> 0
      return GRADIENTS[h % GRADIENTS.length]
    },
    radius() {
      return this.shape === 'circle' ? '50%' : '24rpx'
    },
    boxStyle() {
      return `width:${this.size}rpx;height:${this.size}rpx;border-radius:${this.radius};`
    },
    rStyle() {
      return `width:100%;height:100%;border-radius:${this.radius};`
    },
    fbStyle() {
      const [a, b] = this.grad
      return `border-radius:${this.radius};background:linear-gradient(135deg,${a},${b});font-size:${Math.round(this.size * 0.42)}rpx;`
    }
  }
}
</script>

<style lang="scss">
.ua { overflow: hidden; flex: none; box-shadow: inset 0 0 0 1rpx rgba(12,42,51,0.05); }
.ua-img { display: block; }
.ua-fb {
  width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
  color: #fff; font-weight: 600;
}
</style>
