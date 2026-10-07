<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="hd">
      <view class="hd-t">✨ 魅力周榜</view>
      <view class="hd-d">本周收到礼物越多,魅力越高 · 每周一重新计算</view>
    </view>

    <sk-list v-if="loading" :rows="6" />
    <block v-else>
      <view v-for="(u, i) in list" :key="u.user_id" class="row" :class="{ top1: i === 0 }" @tap="openCard(u)">
        <text class="no" :class="'r' + (i + 1)">{{ i < 3 ? ['👑', '🥈', '🥉'][i] : i + 1 }}</text>
        <user-avatar :name="u.nickname" :src="u.avatar" :size="80" shape="circle" />
        <view class="info">
          <view class="name">
            {{ u.nickname || '神秘朋友' }}
            <image v-if="u.is_verified" class="v" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
          </view>
          <view class="sub">{{ i === 0 ? '本周魅力之星' : '魅力值靠礼物累积' }}</view>
        </view>
        <view class="charm">
          <text class="cn">{{ u.week_charm }}</text>
          <text class="cl">周魅力</text>
        </view>
      </view>
      <empty-state v-if="!list.length" icon="🎁" title="本周还没人上榜" desc="给喜欢的人送份礼物,TA 就会出现在这里" />
    </block>

    <user-card ref="ucard" @chat="startChat($event)" @like="likeUser($event)" />
    </block>
  </view>
</template>

<script>
import { rankApi, chatApi, relationApi } from '../../api/index'
import UserCard from '../../components/user-card/user-card.vue'
import pagesCover from '../../mixins/pagesCover'

export default {
  components: { UserCard },
  mixins: [pagesCover],
  data() {
    return { list: [], loading: true }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.loading = true
    try { this.list = await rankApi.charmWeek() || [] } catch (e) {}
    this.loading = false
  },
  methods: {
    openCard(u) { this.$refs.ucard.show(u.user_id) },
    async startChat(userId) {
      try {
        const chat = await chatApi.start(userId, 0)
        if (chat && chat.chat_id) uni.navigateTo({ url: '/pages/chat/chat?id=' + chat.chat_id })
      } catch (e) {
        if (e && e.message) uni.showToast({ title: e.message, icon: 'none' })
      }
    },
    async likeUser(userId) {
      try { await relationApi.like(userId); uni.showToast({ title: '已喜欢 ❤️', icon: 'none' }) } catch (e) {}
    }
  }
}
</script>

<style lang="scss">
.page { padding: 24rpx 28rpx 60rpx; min-height: 100vh; background: linear-gradient(180deg, #FFF4E6, #FDFBF7 30%); }
.hd { padding: 16rpx 8rpx 26rpx; }
.hd-t { font-size: 40rpx; font-weight: 800; color: #7A4B12; }
.hd-d { font-size: 22rpx; color: #A57A3C; margin-top: 8rpx; }

.row {
  display: flex; align-items: center; gap: 20rpx; background: #fff;
  border: 1rpx solid $line; border-radius: $r-lg; padding: 22rpx 24rpx; margin-bottom: 18rpx;
  box-shadow: $shadow-card;
  &.top1 { background: linear-gradient(135deg, #FFF6E0, #FFE9C7); border-color: #FFD79A; }
}
.no { width: 56rpx; text-align: center; font-size: 30rpx; font-weight: 800; color: $ink-400;
  &.r1, &.r2, &.r3 { font-size: 40rpx; } }
.info { flex: 1; min-width: 0; }
.name { font-size: 28rpx; font-weight: 700; color: $ink-900; display: flex; align-items: center; gap: 8rpx;
  .v { width: 28rpx; height: 28rpx; } }
.sub { font-size: 20rpx; color: $ink-400; margin-top: 6rpx; }
.charm { text-align: right;
  .cn { display: block; font-size: 32rpx; font-weight: 800; color: #d9480f; }
  .cl { display: block; font-size: 20rpx; color: $ink-faint; } }
</style>
