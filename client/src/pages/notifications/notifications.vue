<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view v-for="n in list" :key="n.id" class="row" hover-class="row-press" @tap="open(n)">
      <view class="ic" :class="n.type">{{ icon(n.type) }}</view>
      <view class="bd">
        <view class="t">{{ n.title }}</view>
        <view class="d">{{ n.body }}</view>
        <view class="time">{{ shortTime(n.created_at) }}</view>
      </view>
      <text v-if="!n.read" class="dot"></text>
      <text v-if="jumpable(n)" class="go">›</text>
    </view>
    <empty-state v-if="!loading && !list.length" icon="💬" title="还没有互动通知" desc="有人回信你的瓶子时,会在这里提醒你" />
    </block>
  </view>
</template>

<script>
import { notifyApi } from '../../api/index'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return { list: [], loading: true }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.reload()
  },
  methods: {
    async reload() {
      this.loading = true
      try {
        this.list = await notifyApi.list({ page: 1, size: 50 }) || []
        // 进入页面标记全部已读并清除 tab badge
        await notifyApi.read()
        uni.removeTabBarBadge({ index: 3 })
      } catch (e) {} finally { this.loading = false }
    },
    icon(t) {
      return {
        reply: '💌', scoop: '🌊', moment_comment: '💬', moment_gift: '🎁', like: '❤️'
      }[t] || '🔔'
    },
    jumpable(n) {
      return n.ref_id && n.ref_id !== '0' &&
        ['reply', 'scoop', 'moment_comment', 'moment_gift'].includes(n.type)
    },
    // 按类型分发跳转:回信→收件详情;被捞→我的瓶子(自动弹轨迹);动态评论/礼物→动态详情
    open(n) {
      if (!this.jumpable(n)) return
      if (n.type === 'reply') {
        uni.navigateTo({ url: '/pages/detail/detail?id=' + n.ref_id })
      } else if (n.type === 'scoop') {
        uni.navigateTo({ url: '/pages/mybottles/mybottles?trace_id=' + n.ref_id })
      } else {
        uni.navigateTo({ url: '/pages/moment-detail/moment-detail?id=' + n.ref_id })
      }
    },
    shortTime(t) { return t ? String(t).slice(5, 16).replace('T', ' ') : '刚刚' }
  }
}
</script>

<style lang="scss">
.page { padding: 20rpx 28rpx; min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #DDEFF3); }
.row {
  display: flex; align-items: center; gap: 22rpx; background: $card; border: 1rpx solid $line;
  border-radius: $r-lg; box-shadow: $shadow-card; padding: 26rpx 24rpx; margin-bottom: 18rpx;
  transition: transform 0.15s ease;
}
.row-press { transform: scale(0.98); opacity: 0.85; }
.ic {
  width: 80rpx; height: 80rpx; border-radius: $r-avatar; flex: none;
  display: flex; align-items: center; justify-content: center; font-size: 38rpx; color: #fff;
  background: linear-gradient(135deg, #5BD0E0, #3BA9CC);
  &.reply { background: linear-gradient(135deg, #FF8A7A, #FF6B5B); }
  &.scoop { background: linear-gradient(135deg, #4DABF7, #1971C2); }
  &.moment_comment { background: linear-gradient(135deg, #9775FA, #6741D9); }
  &.moment_gift { background: linear-gradient(135deg, #FFB84D, #E8590C); }
}
.bd { flex: 1; min-width: 0; }
.t { font-size: 30rpx; font-weight: 700; color: $ink-900; }
.d { font-size: 25rpx; color: $ink-soft; margin-top: 6rpx; }
.time { font-size: 22rpx; color: $ink-faint; margin-top: 8rpx; }
.dot { width: 18rpx; height: 18rpx; border-radius: 50%; background: $coral; flex: none; }
.go { font-size: 34rpx; color: $ink-faint; flex: none; }
</style>
