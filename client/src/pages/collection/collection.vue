<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <sk-list v-if="loading" :rows="5" />
    <block v-else>
      <view v-for="b in list" :key="b.collection_id" class="row" @tap="open(b)">
        <view class="icon">{{ typeIcon(b) }}</view>
        <view class="info">
          <view class="txt">{{ b.content }}</view>
          <view class="meta">
            <text class="tag" v-if="firstTag(b)">#{{ firstTag(b) }}</text>
            <text class="city">{{ b.city || '远方' }}</text>
            <text class="time">{{ shortTime(b.collected_at) }}</text>
          </view>
        </view>
      </view>
      <empty-state v-if="!list.length" icon="⭐" title="还没有收藏" desc="捞到好瓶子，点收藏留存" />
      <ad-slot slot-key="banner_collection" type="banner" />
    </block>
    <ad-slot slot-key="banner_collection" type="banner" />
    </block>
  </view>
</template>

<script>
import { collectionApi } from '../../api/index'
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
      try { this.list = await collectionApi.list({ page: 1, size: 50 }) || [] } catch (e) {} finally { this.loading = false }
    },
    typeIcon(b) { return b.content_type === 'image' ? '🖼️' : '💌' },
    firstTag(b) { return b.tags ? String(b.tags).split(',')[0] : '' },
    open(b) {
      uni.navigateTo({ url: '/pages/detail/detail?id=' + b.bottle_id })
    },
    shortTime(t) {
      if (!t) return ''
      return String(t).slice(0, 10)
    }
  }
}
</script>

<style lang="scss">
.page { padding: 24rpx 32rpx; }
.row {
  display: flex; align-items: flex-start; gap: 20rpx; background: $card; border: 1rpx solid $line;
  border-radius: $r-lg; box-shadow: $shadow-card; padding: 24rpx; margin-bottom: 18rpx;
}
.icon {
  width: 80rpx; height: 80rpx; border-radius: $r-avatar; flex: none;
  display: flex; align-items: center; justify-content: center; font-size: 36rpx;
  background: linear-gradient(135deg, #FFF4D6, #FFE199);
}
.info { flex: 1; min-width: 0; }
.txt {
  font-size: 28rpx; color: $ink-900; line-height: 1.5;
  overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
}
.meta { display: flex; align-items: center; gap: 16rpx; margin-top: 10rpx; }
.tag { font-size: 22rpx; color: $sea-deep; font-weight: 600; }
.city { font-size: 22rpx; color: $ink-soft; }
.time { font-size: 22rpx; color: $ink-faint; margin-left: auto; }
</style>
