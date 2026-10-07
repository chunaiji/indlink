<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <sk-list v-if="loading" :rows="5" />
    <block v-else>
      <view v-for="u in users" :key="u.user_id" class="row">
        <user-avatar :name="u.nickname" :src="u.avatar" :size="88" />
        <view class="info">
          <view class="nm">{{ u.nickname }}</view>
          <view class="ct">{{ u.city || '未知城市' }}</view>
        </view>
        <view class="time">{{ shortTime(u.viewed_at) }}</view>
      </view>
      <empty-state v-if="!users.length" icon="🕘" title="还没有浏览记录" desc="逛逛同城和扩列，留下你的足迹" />
      <ad-slot slot-key="banner_viewed" type="banner" />
    </block>
    <ad-slot slot-key="banner_viewed" type="banner" />
    </block>
  </view>
</template>

<script>
import { relationApi } from '../../api/index'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return { users: [], loading: true, page: 1 }
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
        this.users = await relationApi.iViewedList({ page: 1, size: 50 }) || []
      } catch (e) {
      } finally {
        this.loading = false
      }
    },
    shortTime(t) {
      if (!t) return ''
      const d = new Date(t)
      const now = new Date()
      const diff = (now - d) / 1000
      if (diff < 60) return '刚刚'
      if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前'
      if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前'
      return String(t).slice(0, 10)
    }
  }
}
</script>

<style lang="scss">
.page { padding: 24rpx 32rpx; }
.row {
  display: flex; align-items: center; gap: 24rpx; background: $card; border: 1rpx solid $line;
  border-radius: $r-lg; box-shadow: $shadow-card; padding: 24rpx; margin-bottom: 20rpx;
}
.info { flex: 1; min-width: 0; }
.nm { font-size: 30rpx; font-weight: 600; }
.ct { font-size: 24rpx; color: $ink-soft; margin-top: 6rpx; }
.time { font-size: 22rpx; color: $ink-faint; flex: none; }
</style>
