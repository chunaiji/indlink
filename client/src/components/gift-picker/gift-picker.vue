<template>
  <view v-if="visible" class="gp-mask">
    <view class="gp-bg" @tap="close()"></view>
    <view class="gp-panel">
      <view class="gp-hd">
        <text>🎁 选个礼物送 TA</text>
        <text class="gp-bal">余额 {{ balance }} 🪙</text>
      </view>
      <view v-if="loading" class="gp-tip">加载中…</view>
      <view v-else-if="!gifts.length" class="gp-tip">暂无可送的礼物</view>
      <view v-else class="gp-grid">
        <view
          v-for="g in gifts" :key="g.item_id"
          class="gp-item" :class="{ on: picked && picked.item_id === g.item_id }"
          @tap="pick(g)"
        >
          <image v-if="g.icon" class="gp-ic" :src="g.icon" mode="aspectFit" />
          <text v-else class="gp-emoji">🎁</text>
          <text class="gp-name">{{ g.name }}</text>
          <text class="gp-price">{{ g.price_coin }} 🪙</text>
        </view>
      </view>
      <button class="gp-send" :class="{ dis: !picked }" :loading="sending" @tap="send()">
        {{ picked ? `送出「${picked.name}」(${picked.price_coin}金币)` : '选一个礼物' }}
      </button>
    </view>
  </view>
</template>

<script>
import { itemApi } from '../../api/index'
import { useWalletStore } from '../../store/wallet'

// 礼物选择半屏面板:show() 打开,选中后点赠送 emit('send', item),由父页面调具体送礼接口。
export default {
  data() {
    return { visible: false, loading: false, gifts: [], picked: null, sending: false }
  },
  computed: {
    balance() { return useWalletStore().balance }
  },
  methods: {
    async show() {
      this.visible = true
      this.picked = null
      this.sending = false
      useWalletStore().fetchBalance()
      if (!this.gifts.length) {
        this.loading = true
        try {
          const list = await itemApi.list() || []
          this.gifts = list.filter((x) => x.type === 'gift')
        } catch (e) {} finally { this.loading = false }
      }
    },
    close() { this.visible = false },
    pick(g) { this.picked = g },
    send() {
      if (!this.picked || this.sending) return
      this.sending = true
      this.$emit('send', this.picked)
    },
    // 父页面处理完(成功/失败)后调用,复位状态
    done(ok) {
      this.sending = false
      if (ok) this.visible = false
    }
  }
}
</script>

<style lang="scss" scoped>
/* z-index 高于 tabBar(999)与评论弹层(1001),可叠加在评论弹层之上 */
.gp-mask { position: fixed; inset: 0; z-index: 1002; display: flex; align-items: flex-end; }
.gp-bg { position: absolute; inset: 0; background: rgba(8,30,40,0.5); }
.gp-panel {
  position: relative; width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0;
  padding: 30rpx 30rpx calc(30rpx + env(safe-area-inset-bottom));
}
.gp-hd { display: flex; align-items: baseline; font-size: 30rpx; font-weight: 800; color: $ink-900;
  .gp-bal { margin-left: auto; font-size: 24rpx; font-weight: 600; color: #d9480f; } }
.gp-tip { text-align: center; padding: 60rpx 0; font-size: 24rpx; color: $ink-faint; }
.gp-grid { display: flex; flex-wrap: wrap; gap: 14rpx; margin-top: 24rpx; max-height: 46vh; overflow-y: auto; }
.gp-item {
  width: calc(25% - 11rpx); text-align: center; padding: 18rpx 6rpx 14rpx;
  border: 2rpx solid $line; border-radius: 16rpx; box-sizing: border-box;
  transition: transform 0.15s ease;
  .gp-ic { width: 72rpx; height: 72rpx; }
  .gp-emoji { font-size: 60rpx; display: block; }
  .gp-name { display: block; font-size: 22rpx; color: $ink-900; margin-top: 8rpx;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .gp-price { display: block; font-size: 20rpx; font-weight: 800; color: #d9480f; margin-top: 2rpx; }
  &.on { border-color: $coral; background: #FFF6F4; transform: scale(1.06); box-shadow: 0 6rpx 16rpx rgba(255,107,91,0.25); }
}
.gp-send {
  margin-top: 26rpx; height: 84rpx; line-height: 84rpx; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; font-size: 28rpx; font-weight: 800;
  &.dis { opacity: 0.5; }
}
.gp-send::after { border: none; }
</style>
