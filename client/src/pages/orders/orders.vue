<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="tabs">
      <text :class="{ on: tab === 'pay' }" @tap="switchTab('pay')">支付订单</text>
      <text :class="{ on: tab === 'txn' }" @tap="switchTab('txn')">金币流水</text>
      <text :class="{ on: tab === 'item' }" @tap="switchTab('item')">道具记录</text>
    </view>

    <sk-list v-if="loading" :rows="5" />
    <block v-else>
      <!-- 支付订单 -->
      <block v-if="tab === 'pay'">
        <view v-for="o in orders" :key="o.order_no" class="row">
          <view class="ic pay">💳</view>
          <view class="bd">
            <view class="tt">充值 {{ o.coins }} 金币</view>
            <view class="sub">订单 {{ o.order_no.slice(-8) }}</view>
            <view class="tm">{{ fmtTime(o.created_at) }}</view>
          </view>
          <view class="right">
            <view class="price">¥{{ fmtYuan(o.price_fen) }}</view>
            <view class="status" :class="o.status">{{ statusLabel(o.status) }}</view>
          </view>
        </view>
        <empty-state v-if="!orders.length" icon="📋" title="暂无支付订单" desc="充值后可在这里查看记录" />
      </block>

      <!-- 金币流水 -->
      <block v-if="tab === 'txn'">
        <view v-for="t in txns" :key="t.txn_id" class="row">
          <view class="ic" :class="t.direction">{{ sceneIcon(t.scene) }}</view>
          <view class="bd">
            <view class="tt">{{ sceneLabel(t.scene) }}</view>
            <view class="tm">{{ fmtTime(t.created_at) }}</view>
          </view>
          <view class="amt" :class="t.direction">
            {{ t.direction === 'credit' ? '+' : '-' }}{{ t.coins }}
          </view>
        </view>
        <empty-state v-if="!txns.length" icon="💰" title="暂无流水记录" desc="" />
      </block>

      <!-- 道具记录 -->
      <block v-if="tab === 'item'">
        <view v-for="t in itemTxns" :key="t.txn_id" class="row">
          <view class="ic credit">🎁</view>
          <view class="bd">
            <view class="tt">获得道具</view>
            <view class="sub">花费 {{ t.coins }} 金币</view>
            <view class="tm">{{ fmtTime(t.created_at) }}</view>
          </view>
          <view class="amt credit">+1</view>
        </view>
        <empty-state v-if="!itemTxns.length" icon="🎁" title="暂无道具记录" desc="购买道具后可在这里查看" />
      </block>

      <view v-if="loadingMore" class="more">加载中…</view>
      <view v-else-if="!more[src()] && (tab === 'pay' ? orders.length : txns.length)" class="more">没有更多了</view>
      <ad-slot slot-key="banner_orders" type="banner" />
    </block>
    <ad-slot slot-key="banner_orders" type="banner" />
    </block>
  </view>
</template>

<script>
import { walletApi, payApi } from '../../api/index'
import pagesCover from '../../mixins/pagesCover'

const LABELS = { recharge: '充值金币', reward: '注册奖励', chat: '开聊消费', unlock: '解锁回信', gift: '购买道具', checkin: '签到奖励', msg: '消息消费', share: '分享奖励' }
const ICONS  = { recharge: '💰', reward: '🎁', chat: '💬', unlock: '🔓', gift: '🛍️', checkin: '📅', msg: '✉️', share: '🔗' }
const STATUS_LABEL = { pending: '待支付', paid: '已支付', failed: '失败', refunded: '已退款' }
const SIZE = 20

export default {
  mixins: [pagesCover],
  data() {
    return {
      tab: 'pay',
      orders: [], txns: [],
      // 分页状态:pay 用 orders 源,txn/item 共用 txns 源
      page: { pay: 1, txn: 1 },
      more: { pay: true, txn: true },
      loading: true,   // 首屏骨架
      loadingMore: false
    }
  },
  computed: {
    itemTxns() { return this.txns.filter(t => t.scene === 'gift') }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    // 首次进入按当前 tab 拉第一页
    if (!this._inited) { this._inited = true; this.refresh() }
  },
  onPullDownRefresh() {
    // 下拉刷新:只刷当前选中的 tab
    this.refresh().finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    // 列表向上滑到底:加载当前 tab 下一页
    this.loadMore()
  },
  methods: {
    src() { return this.tab === 'pay' ? 'pay' : 'txn' }, // txn/item 共用 txns 源
    async switchTab(t) {
      if (this.tab === t) return
      this.tab = t
      const s = this.src()
      // 该源还没数据则拉首页
      const empty = s === 'pay' ? !this.orders.length : !this.txns.length
      if (empty) this.refresh()
    },
    async fetch(s, page) {
      if (s === 'pay') return await payApi.orders({ page, size: SIZE }) || []
      return await walletApi.txns({ page, size: SIZE }) || []
    },
    async refresh() {
      const s = this.src()
      this.loading = !((s === 'pay' ? this.orders : this.txns).length)
      try {
        const list = await this.fetch(s, 1)
        if (s === 'pay') this.orders = list; else this.txns = list
        this.page[s] = 1
        this.more[s] = list.length >= SIZE
      } catch (e) {} finally { this.loading = false }
    },
    async loadMore() {
      const s = this.src()
      if (this.loadingMore || !this.more[s]) return
      this.loadingMore = true
      try {
        const next = this.page[s] + 1
        const list = await this.fetch(s, next)
        if (list.length) {
          if (s === 'pay') this.orders = this.orders.concat(list)
          else this.txns = this.txns.concat(list)
          this.page[s] = next
        }
        if (list.length < SIZE) this.more[s] = false
      } catch (e) {} finally { this.loadingMore = false }
    },
    fmtTime(t) { return t ? String(t).slice(0, 16).replace('T', ' ') : '' },
    fmtYuan(fen) { return fen % 100 === 0 ? fen / 100 : (fen / 100).toFixed(2) },
    sceneLabel(s) { return LABELS[s] || '其他' },
    sceneIcon(s)  { return ICONS[s] || '📋' },
    statusLabel(s) { return STATUS_LABEL[s] || s }
  }
}
</script>

<style lang="scss">
.page { min-height: 100vh; background: $ground; }
.tabs {
  display: flex; background: #fff; border-bottom: 1rpx solid $line;
  text { flex: 1; text-align: center; padding: 24rpx 0; font-size: 28rpx; color: $ink-faint; font-weight: 600;
    &.on { color: $coral; border-bottom: 4rpx solid $coral; }
  }
}
.row {
  display: flex; align-items: center; gap: 20rpx;
  background: $card; border-bottom: 1rpx solid $line; padding: 24rpx 28rpx;
}
.ic {
  width: 72rpx; height: 72rpx; border-radius: $r-avatar; flex: none;
  display: flex; align-items: center; justify-content: center; font-size: 34rpx;
  &.pay    { background: rgba(91,208,224,0.16); }
  &.credit { background: rgba(79,201,122,0.14); }
  &.debit  { background: rgba(255,107,91,0.12); }
}
.bd { flex: 1; min-width: 0; }
.tt { font-size: 28rpx; font-weight: 600; color: $ink-900; }
.sub { font-size: 22rpx; color: $ink-faint; margin-top: 2rpx; }
.tm { font-size: 22rpx; color: $ink-faint; margin-top: 4rpx; }
.right { text-align: right; }
.price { font-size: 32rpx; font-weight: 700; color: $ink; }
.status {
  font-size: 20rpx; margin-top: 6rpx; font-weight: 600;
  &.paid    { color: #26a65b; }
  &.pending { color: #E8901A; }
  &.failed  { color: $coral; }
  &.refunded{ color: $ink-soft; }
}
.amt { font-size: 32rpx; font-weight: 700; &.credit { color: #26a65b; } &.debit { color: $coral; } }
.more { text-align: center; color: $ink-faint; font-size: 24rpx; padding: 28rpx 0 40rpx; }
</style>
