<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="bal">
      <text class="bl">金币余额</text>
      <view class="bv">{{ balance }}<image class="coin-bal" src="/static/icons/coin.png" mode="aspectFit" /></view>
      <text class="recharge" @tap="goRecharge($event)">充值 ›</text>
    </view>

    <sk-list v-if="loading" :rows="4" />
    <block v-else>
      <view class="grid">
        <view v-for="it in items" :key="it.item_id" class="card" @tap="buy(it)">
          <view class="ic">
            <image v-if="isImg(it.icon)" :src="it.icon" class="ic-img" mode="aspectFit" />
            <text v-else>{{ iconOf(it) }}</text>
          </view>
          <view class="nm">{{ it.name }}</view>
          <view class="pr">{{ it.price_coin }}<image class="coin-pr" src="/static/icons/coin.png" mode="aspectFit" /></view>
          <view v-if="myItems[String(it.item_id)]" class="own">已有 {{ myItems[String(it.item_id)] }}</view>
        </view>
      </view>
      <empty-state v-if="!items.length" icon="🎁" title="暂无道具" desc="过会儿再来看看" />
    </block>
    </block>
  </view>
</template>

<script>
import { itemApi } from '../../api/index'
import { useWalletStore } from '../../store/wallet'
import pagesCover from '../../mixins/pagesCover'

const ICONS = { boost: '🚀', top: '⬆️', superlike: '💖', gift: '🎁' }

export default {
  mixins: [pagesCover],
  data() {
    return { items: [], myItems: {}, loading: true }
  },
  computed: {
    balance() { return useWalletStore().balance }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.loading = true
    try {
      await useWalletStore().fetchBalance()
      const [items, my] = await Promise.all([itemApi.list(), itemApi.myItems()])
      this.items = items || []
      this.myItems = my || {}
    } catch (e) {} finally { this.loading = false }
  },
  methods: {
    isImg(s) { return typeof s === 'string' && /^(https?:)?\/\//.test(s) },
    iconOf(it) { return it.icon || ICONS[it.type] || '🎁' },
    goRecharge() { uni.navigateTo({ url: '/pages/recharge/recharge' }) },
    buy(it) {
      uni.showModal({
        title: '购买道具',
        content: `用 ${it.price_coin} 金币购买「${it.name}」?`,
        confirmText: '购买',
        success: async (r) => {
          if (!r.confirm) return
          try {
            await itemApi.buy(it.item_id, 0)
            await useWalletStore().fetchBalance()
            uni.showToast({ title: '购买成功 🎉' })
          } catch (e) {
            if (e && e.code === 5001) {
              uni.showModal({
                title: '金币不足', content: '前往充值?',
                success: (rr) => { if (rr.confirm) this.goRecharge() }
              })
            }
          }
        }
      })
    }
  }
}
</script>

<style lang="scss">
.page { padding: 24rpx 32rpx; }
.bal {
  display: flex; align-items: center; background: linear-gradient(135deg, #FFD96B, #FFB13C);
  color: #7A4A00; border-radius: $r-lg; padding: 28rpx 32rpx; margin-bottom: 28rpx;
  .bl { font-size: 26rpx; font-weight: 600; opacity: 0.9; }
  .bv { font-size: 40rpx; font-weight: 700; margin-left: 16rpx; flex: 1; display: flex; align-items: flex-end; gap: 8rpx; }
  .coin-bal { width: 54rpx; height: 54rpx; flex-shrink: 0; }
  .recharge { font-size: 26rpx; font-weight: 600; }
}
.grid { display: flex; flex-wrap: wrap; justify-content: space-between; }
.card {
  width: 31.5%; box-sizing: border-box; background: $card; border: 1rpx solid $line;
  border-radius: $r-lg; box-shadow: $shadow-card; padding: 28rpx 12rpx; margin-bottom: 22rpx; text-align: center;
  .ic { font-size: 64rpx; height: 96rpx; display: flex; align-items: center; justify-content: center; }
  .ic-img { width: 96rpx; height: 96rpx; }
  .nm { font-size: 26rpx; font-weight: 600; color: $ink-900; margin-top: 12rpx; }
  .pr  { font-size: 34rpx; color: $coral-deep; font-weight: 600; margin-top: 8rpx; display: flex; align-items: flex-end; justify-content: center; gap: 4rpx; }
  .coin-pr { width: 50rpx; height: 50rpx; flex-shrink: 0; }
  .own { font-size: 20rpx; color: #26a65b; font-weight: 600; margin-top: 4rpx; }
}
</style>
