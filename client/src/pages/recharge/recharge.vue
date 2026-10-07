<template>
  <view class="rech">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="bal">
      <view class="shine"></view>
      <view class="bl">当前余额</view>
      <view class="bv">{{ balance }}<image class="coin-lg" src="/static/icons/coin.png" mode="aspectFit" /></view>
    </view>

    <!-- iOS 合规:命中开关时隐藏档位与支付,给出提示 -->
    <block v-if="iosBlocked">
      <view class="ios-block">
        <text class="ic">🍎</text>
        <view class="t">当前为 iOS 设备</view>
        <view class="d">按平台规定暂不支持在 iOS 充值,可在安卓端或网页端充值,金币全平台通用。</view>
      </view>
    </block>

    <block v-else>
      <view v-if="packages.length" class="pkg-grid">
        <view
          v-for="p in packages"
          :key="p.package_id"
          class="pkg"
          :class="{ on: selected === p.package_id }"
          @tap="selectPkg(p.package_id)"
        >
          <text v-if="p.bonus_coins" class="bonus">送 {{ p.bonus_coins }}</text>
          <view class="coins">{{ p.coins + p.bonus_coins }}<image class="coin-sm" src="/static/icons/coin.png" mode="aspectFit" /></view>
          <view class="price">¥{{ p.price_fen % 100 === 0 ? p.price_fen / 100 : (p.price_fen / 100).toFixed(2) }}</view>
          <view v-if="p.name" class="pname">{{ p.name }}</view>
        </view>
      </view>
      <view v-else class="pkg-empty">
        <text class="pe-ic">🛒</text>
        <view class="pe-t">暂无充值档位</view>
        <view class="pe-d">充值档位维护中，请稍后再来</view>
      </view>

      <!-- 无档位时不显示支付方式与支付按钮 -->
      <block v-if="packages.length">
        <view class="pay-h">支付方式</view>
        <view class="payrow" :class="{ on: payMethod === platform }">
          <view class="pic" :style="{ background: platform === 'alipay' ? '#1677FF' : '#07C160' }">
            {{ platform === 'alipay' ? '支' : '✓' }}
          </view>
          <text>{{ platform === 'alipay' ? '支付宝' : '支付' }}</text>
          <view class="radio on"></view>
        </view>

        <button class="paybtn" :loading="paying" @tap="pay($event)">{{ payText }}</button>
      </block>
    </block>
    </block>
  </view>
</template>

<script>
import { payApi } from '../../api/index'
import { useWalletStore } from '../../store/wallet'
import { useUserStore } from '../../store/user'
import { requestPay } from '../../utils/platform'
import { isIOS } from '../../utils/platform'
import { currentPlatform } from '../../utils/config'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return { packages: [], selected: 0, paying: false, platform: 'wx', payMethod: 'wx', iosBlocked: false }
  },
  computed: {
    balance() { return useWalletStore().balance },
    payText() {
      const p = this.packages.find((x) => x.package_id === this.selected)
      const yuan = p ? (p.price_fen % 100 === 0 ? p.price_fen / 100 : (p.price_fen / 100).toFixed(2)) : 0
      return p ? `¥${yuan} 立即支付` : '请选择档位'
    }
  },
  async onLoad() {
    await this.loadCover()
    if (this.showCover) return
    this.platform = currentPlatform()
    this.payMethod = this.platform
    // iOS + 微信 + 后台开关 => 隐藏充值
    const user = useUserStore()
    this.iosBlocked = this.platform === 'wx' && isIOS() && user.iosRechargeOff
    try {
      this.packages = await payApi.packages() || []
      if (this.packages.length) this.selected = this.packages[Math.min(2, this.packages.length - 1)].package_id
      await useWalletStore().fetchBalance()
    } catch (e) {}
  },
  methods: {
    selectPkg(id) { this.selected = id },
    async pay() {
      if (!this.selected) return
      this.paying = true
      try {
        const res = await payApi.createOrder(this.selected)
        await requestPay(res.pay_params)
        // 充值入账以后端异步回调为准,这里轮询余额刷新
        uni.showLoading({ title: '确认中…' })
        await this.pollBalance(res.order_no)
        uni.hideLoading()
        uni.showToast({ title: '充值成功' })
        setTimeout(() => uni.navigateBack(), 800)
      } catch (e) {
        uni.hideLoading()
        if (e && e.errMsg && e.errMsg.indexOf('cancel') >= 0) {
          uni.showToast({ title: '已取消支付', icon: 'none' })
        }
      } finally {
        this.paying = false
      }
    },
    // 余额以回调入账为准,前端轮询几次确认到账
    async pollBalance(_orderNo) {
      const before = useWalletStore().balance
      for (let i = 0; i < 5; i++) {
        await new Promise((r) => setTimeout(r, 800))
        const now = await useWalletStore().fetchBalance()
        if (now > before) return
      }
    }
  }
}
</script>

<style lang="scss">
.rech { min-height: 100vh; background: linear-gradient(180deg, #FFF6E2, #EAF7FB 40%); padding: 28rpx; }
.bal {
  position: relative; overflow: hidden; padding: 36rpx; border-radius: $r-lg;
  background: linear-gradient(135deg, #FFD96B, #FFB13C); color: #7A4A00;
  .shine { position: absolute; right: -40rpx; top: -40rpx; width: 180rpx; height: 180rpx; border-radius: 50%; background: rgba(255,255,255,0.3); }
  .bl { font-size: 24rpx; font-weight: 700; opacity: 0.85; }
  .bv { font-size: 64rpx; font-weight: 700; margin-top: 4rpx; display: flex; align-items: flex-end; gap: 10rpx; }
  .coin-lg { width: 78rpx; height: 78rpx; flex-shrink: 0; }
}
.pkg-grid { display: flex; flex-wrap: wrap; justify-content: space-between; margin-top: 28rpx; }
.pkg {
  width: 31%; box-sizing: border-box; margin-bottom: 20rpx;
  border: 3rpx solid $line; border-radius: 24rpx; padding: 28rpx 8rpx; text-align: center;
  background: #fff; position: relative;
  &.on { border-color: $coral; background: rgba(255,107,91,0.06); }
  .coins { font-size: 34rpx; font-weight: 700; color: $ink; display: flex; align-items: flex-end; justify-content: center; gap: 4rpx; }
  .coin-sm { width: 52rpx; height: 52rpx; flex-shrink: 0; }
  .price { font-size: 24rpx; color: $ink-soft; margin-top: 10rpx; font-weight: 600; }
  .pname { font-size: 22rpx; color: $ink-faint; margin-top: 6rpx; }
  .bonus {
    position: absolute; top: -16rpx; left: 50%; transform: translateX(-50%); white-space: nowrap;
    font-size: 18rpx; font-weight: 700; background: $coral; color: #fff; padding: 2rpx 14rpx; border-radius: 999rpx;
  }
}
.pay-h { font-size: 26rpx; font-weight: 700; margin: 28rpx 4rpx 16rpx; }
.payrow {
  display: flex; align-items: center; gap: 20rpx; padding: 24rpx;
  border: 1rpx solid $line; border-radius: 24rpx; background: #fff; font-weight: 700; font-size: 28rpx;
  .pic { width: 52rpx; height: 52rpx; border-radius: 14rpx; color: #fff; display: flex; align-items: center; justify-content: center; font-size: 30rpx; }
  .radio { margin-left: auto; width: 36rpx; height: 36rpx; border-radius: 50%; border: 4rpx solid $ink-faint;
    &.on { border-color: $coral; background: radial-gradient(circle, #{$coral} 42%, transparent 46%); }
  }
}
.paybtn {
  margin-top: 48rpx; height: 100rpx; line-height: 100rpx; border-radius: $r-lg;
  background: linear-gradient(135deg, $coral, $coral-deep); color: #fff; font-weight: 700; font-size: 32rpx;
  box-shadow: $shadow-pop;
}
.ios-block {
  margin-top: 60rpx; text-align: center; padding: 60rpx 40rpx;
  background: #fff; border-radius: $r-lg;
  .ic { font-size: 80rpx; }
  .t { font-size: 32rpx; font-weight: 700; margin-top: 20rpx; }
  .d { font-size: 26rpx; color: $ink-soft; margin-top: 16rpx; line-height: 1.6; }
}
.pkg-empty {
  margin-top: 60rpx; text-align: center; padding: 60rpx 40rpx;
  background: #fff; border-radius: $r-lg;
  .pe-ic { font-size: 80rpx; }
  .pe-t { font-size: 32rpx; font-weight: 700; margin-top: 20rpx; }
  .pe-d { font-size: 26rpx; color: $ink-soft; margin-top: 16rpx; line-height: 1.6; }
}
</style>
