<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <sk-list v-if="loading" :rows="4" />
    <block v-else>
      <view v-for="u in users" :key="u.user_id" class="row">
        <user-avatar :name="u.nickname" :src="u.avatar" :size="88" />
        <view class="info">
          <view class="nm">{{ u.nickname }}</view>
          <view class="ct">{{ u.city || '未知' }}</view>
        </view>
        <view class="btn" @tap="unblock(u)">解除</view>
      </view>
      <empty-state v-if="!users.length" icon="🚫" title="黑名单是空的" desc="在用户卡片长按可拉黑骚扰者" />
    </block>
    </block>
  </view>
</template>

<script>
import { moderationApi } from '../../api/index'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return { users: [], loading: true }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.reload()
  },
  methods: {
    async reload() {
      this.loading = true
      try { this.users = await moderationApi.blockList() || [] } catch (e) {} finally { this.loading = false }
    },
    unblock(u) {
      uni.showModal({
        title: '解除拉黑', content: `不再屏蔽「${u.nickname}」?`,
        success: async (r) => {
          if (!r.confirm) return
          try {
            await moderationApi.unblock(u.user_id)
            this.users = this.users.filter((x) => x.user_id !== u.user_id)
            uni.showToast({ title: '已解除', icon: 'none' })
          } catch (e) {}
        }
      })
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
.ct { font-size: 24rpx; color: $ink-400; margin-top: 6rpx; }
.btn {
  flex: none; padding: 0 28rpx; height: 64rpx; line-height: 64rpx; border-radius: $r-sm;
  border: 1rpx solid $line; color: $ink-600; font-size: 26rpx; font-weight: 600;
}
</style>
