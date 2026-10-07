<template>
  <view v-if="visible" class="uc-mask">
    <view class="uc-bg" @tap="close()"></view>
    <view class="uc-panel">
      <view class="uc-close" @tap="close()">✕</view>
      <view v-if="loading" class="uc-loading">加载中…</view>
      <block v-else-if="card">
        <view class="uc-head">
          <user-avatar :name="card.nickname" :src="card.avatar" :size="120" shape="circle" />
          <view class="uc-id">
            <view class="uc-name">
              {{ card.nickname || '神秘朋友' }}
              <image v-if="card.is_verified" class="uc-v" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
            </view>
            <view class="uc-pills">
              <text class="uc-pill" :class="card.gender === 1 ? 'male' : 'female'">{{ genderText }} {{ card.age || '' }}</text>
              <text class="uc-pill">{{ card.city || '远方' }}</text>
            </view>
          </view>
        </view>
        <view v-if="card.bio" class="uc-bio">{{ card.bio }}</view>
        <view class="uc-stats">
          <view class="uc-stat"><text class="n">{{ card.charm || 0 }}</text><text class="l">魅力</text></view>
          <view class="uc-stat"><text class="n">{{ card.fans_count || 0 }}</text><text class="l">粉丝</text></view>
          <view class="uc-stat"><text class="n">{{ regDays }}</text><text class="l">来这 {{ regDays }} 天</text></view>
        </view>
        <view class="uc-sec">🎁 礼物墙</view>
        <view v-if="(card.gifts || []).length" class="uc-gifts">
          <view v-for="g in card.gifts" :key="g.item_id" class="uc-gift">
            <image v-if="g.icon" class="gi" :src="g.icon" mode="aspectFit" />
            <text v-else class="gi-emoji">🎁</text>
            <text class="gn">{{ g.name }}</text>
            <text class="gc">×{{ g.count }}</text>
          </view>
        </view>
        <view v-else class="uc-gifts-empty" @tap="emitChat()">还没有礼物,做 TA 的第一个送礼人 ›</view>
        <view class="uc-actions">
          <view class="uc-btn ghost" @tap="emitLike()">❤️ 喜欢</view>
          <view class="uc-btn cta" @tap="emitChat()">开聊</view>
        </view>
      </block>
      <view v-else class="uc-loading">资料获取失败</view>
    </view>
  </view>
</template>

<script>
import { userApi } from '../../api/index'
import UserAvatar from '../user-avatar/user-avatar.vue'

// 用户资料半屏卡:show(userId) 打开;开聊/喜欢交给页面处理(emit chat/like)。
export default {
  components: { UserAvatar },
  data() {
    return { visible: false, loading: false, card: null, userId: '' }
  },
  computed: {
    genderText() { return this.card && this.card.gender === 1 ? '♂' : this.card && this.card.gender === 2 ? '♀' : '' },
    regDays() {
      if (!this.card || !this.card.created_at) return 1
      return Math.max(1, Math.ceil((Date.now() - new Date(this.card.created_at).getTime()) / 86400000))
    }
  },
  methods: {
    async show(userId) {
      this.userId = String(userId)
      this.visible = true
      this.loading = true
      this.card = null
      try { this.card = await userApi.card(this.userId) } catch (e) {}
      this.loading = false
    },
    close() { this.visible = false },
    emitChat() { this.$emit('chat', this.userId); this.close() },
    emitLike() { this.$emit('like', this.userId) }
  }
}
</script>

<style lang="scss" scoped>
/* z-index 高于自定义 tabBar(999);bg 层负责点击关闭(@tap.stop 不可靠,勿用) */
.uc-mask { position: fixed; inset: 0; z-index: 1001; display: flex; align-items: flex-end; }
.uc-bg { position: absolute; inset: 0; background: rgba(8,30,40,0.45); }
.uc-panel { position: relative; width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0; padding: 36rpx 32rpx calc(40rpx + env(safe-area-inset-bottom)); }
.uc-close { position: absolute; right: 26rpx; top: 22rpx; font-size: 30rpx; color: $ink-400; padding: 8rpx; }
.uc-loading { text-align: center; padding: 80rpx 0; font-size: 26rpx; color: $ink-400; }

.uc-head { display: flex; align-items: center; gap: 22rpx; }
.uc-id { flex: 1; min-width: 0; }
.uc-name { font-size: 34rpx; font-weight: 800; color: $ink-900; display: flex; align-items: center; gap: 10rpx;
  .uc-v { width: 34rpx; height: 34rpx; } }
.uc-pills { display: flex; gap: 12rpx; margin-top: 12rpx; }
.uc-pill {
  font-size: 20rpx; font-weight: 700; color: $ink-soft; background: $ink-50;
  padding: 4rpx 16rpx; border-radius: 999rpx;
  &.male { color: #1c7ed6; background: rgba(28,126,214,0.10); }
  &.female { color: #e64980; background: rgba(230,73,128,0.10); }
}
.uc-bio { margin-top: 20rpx; font-size: 26rpx; color: $ink-soft; line-height: 1.55; }

.uc-stats { display: flex; margin-top: 26rpx; background: #F6FAFC; border-radius: 16rpx; padding: 20rpx 0; }
.uc-stat { flex: 1; text-align: center;
  .n { display: block; font-size: 32rpx; font-weight: 800; color: $ink-900; }
  .l { display: block; font-size: 20rpx; color: $ink-400; margin-top: 4rpx; } }

.uc-sec { margin-top: 28rpx; font-size: 26rpx; font-weight: 800; color: $ink-900; }
.uc-gifts { display: flex; flex-wrap: wrap; gap: 16rpx; margin-top: 16rpx; }
.uc-gift {
  width: 140rpx; text-align: center; background: #FFF8F0; border: 1rpx solid #FFE4C4;
  border-radius: 14rpx; padding: 14rpx 6rpx;
  .gi { width: 56rpx; height: 56rpx; }
  .gi-emoji { font-size: 48rpx; display: block; }
  .gn { display: block; font-size: 20rpx; color: $ink-soft; margin-top: 6rpx; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .gc { display: block; font-size: 20rpx; font-weight: 800; color: #d9480f; margin-top: 2rpx; }
}
.uc-gifts-empty {
  margin-top: 16rpx; font-size: 24rpx; color: #d9480f; font-weight: 600;
  background: #FFF8F0; border: 1rpx dashed #FFC078; border-radius: 14rpx; padding: 22rpx; text-align: center;
}

.uc-actions { display: flex; gap: 20rpx; margin-top: 32rpx; }
.uc-btn {
  flex: 1; height: 84rpx; line-height: 84rpx; text-align: center; border-radius: 999rpx;
  font-size: 28rpx; font-weight: 800;
  &.ghost { background: $ink-50; color: $ink-900; }
  &.cta { background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; box-shadow: 0 8rpx 20rpx rgba(255,107,91,0.35); }
}
</style>
