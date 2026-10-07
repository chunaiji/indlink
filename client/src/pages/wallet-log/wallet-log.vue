<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="tabs">
      <text :class="{ on: tab === 'all' }" @tap="tab = 'all'">全部</text>
      <text :class="{ on: tab === 'credit' }" @tap="tab = 'credit'">收入</text>
      <text :class="{ on: tab === 'debit' }" @tap="tab = 'debit'">支出</text>
    </view>

    <view class="summary">
      <view class="sc"><view class="sv">{{ totalIn }}</view><view class="sl">累计收入</view></view>
      <view class="sc"><view class="sv out">{{ totalOut }}</view><view class="sl">累计支出</view></view>
    </view>

    <sk-list v-if="loading" :rows="6" />
    <block v-else>
      <view v-for="t in filtered" :key="t.txn_id" class="row">
        <view class="ic" :class="t.direction">{{ sceneIcon(t.scene) }}</view>
        <view class="bd">
          <view class="tt">{{ sceneLabel(t.scene) }}</view>
          <view class="tm">{{ fmtTime(t.created_at) }}</view>
        </view>
        <view class="amt" :class="t.direction">
          {{ t.direction === 'credit' ? '+' : '-' }}{{ t.coins }}
        </view>
        <view class="bal">余 {{ t.balance_after }}</view>
      </view>
      <empty-state v-if="!filtered.length" icon="💰" title="暂无流水" desc="充值或消费后可在这里查看记录" />
      <ad-slot slot-key="banner_walletlog" type="banner" />
    </block>
    <ad-slot slot-key="banner_walletlog" type="banner" />
    </block>
  </view>
</template>

<script>
import { walletApi } from '../../api/index'
import pagesCover from '../../mixins/pagesCover'

const LABELS = { recharge: '充值金币', reward: '注册奖励', chat: '开聊消费', unlock: '解锁回信', gift: '购买道具', checkin: '签到奖励', msg: '消息消费', share: '分享奖励' }
const ICONS  = { recharge: '💰', reward: '🎁', chat: '💬', unlock: '🔓', gift: '🛍️', checkin: '📅', msg: '✉️', share: '🔗' }

export default {
  mixins: [pagesCover],
  data() {
    return { list: [], loading: true, tab: 'all' }
  },
  computed: {
    filtered() {
      if (this.tab === 'all') return this.list
      return this.list.filter(t => t.direction === this.tab)
    },
    totalIn()  { return this.list.filter(t => t.direction === 'credit').reduce((s, t) => s + t.coins, 0) },
    totalOut() { return this.list.filter(t => t.direction === 'debit').reduce((s, t) => s + t.coins, 0) }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.loading = true
    try { this.list = await walletApi.txns({ page: 1, size: 50 }) || [] } catch (e) {} finally { this.loading = false }
  },
  methods: {
    sceneLabel(s) { return LABELS[s] || '其他' },
    sceneIcon(s)  { return ICONS[s] || '📋' },
    fmtTime(t) { return t ? String(t).slice(0, 16).replace('T', ' ') : '' }
  }
}
</script>

<style lang="scss">
.page { min-height: 100vh; background: $ground; padding: 0 0 40rpx; }
.tabs {
  display: flex; gap: 0; background: #fff; border-bottom: 1rpx solid $line;
  text { flex: 1; text-align: center; padding: 24rpx 0; font-size: 28rpx; color: $ink-faint; font-weight: 600;
    &.on { color: $coral; border-bottom: 4rpx solid $coral; }
  }
}
.summary {
  display: flex; background: linear-gradient(135deg, #5BD0E0, #3BA9CC); color: #fff;
  padding: 28rpx 40rpx; margin-bottom: 16rpx;
  .sc { flex: 1; text-align: center; }
  .sv { font-size: 48rpx; font-weight: 700; &.out { opacity: 0.85; } }
  .sl { font-size: 22rpx; opacity: 0.9; margin-top: 4rpx; }
}
.row {
  display: flex; align-items: center; gap: 20rpx;
  background: $card; border-bottom: 1rpx solid $line; padding: 24rpx 28rpx;
}
.ic {
  width: 72rpx; height: 72rpx; border-radius: $r-avatar; flex: none;
  display: flex; align-items: center; justify-content: center; font-size: 34rpx;
  &.credit { background: rgba(79,201,122,0.14); }
  &.debit  { background: rgba(255,107,91,0.12); }
}
.bd { flex: 1; min-width: 0; }
.tt { font-size: 28rpx; font-weight: 600; color: $ink-900; }
.tm { font-size: 22rpx; color: $ink-faint; margin-top: 4rpx; }
.amt { font-size: 32rpx; font-weight: 700; &.credit { color: #26a65b; } &.debit { color: $coral; } }
.bal { font-size: 20rpx; color: $ink-faint; margin-left: 8rpx; white-space: nowrap; }
</style>
