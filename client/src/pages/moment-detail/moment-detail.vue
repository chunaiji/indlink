<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <sk-list v-if="loading" :rows="3" />
    <empty-state v-else-if="!m" icon="🍃" title="动态已经不在了" desc="可能已被作者删除" />
    <block v-else>
      <!-- 动态主体(布局同广场卡片) -->
      <view class="mcard">
        <view class="m-hd">
          <user-avatar :name="m.nickname" :src="m.avatar" :size="76" shape="circle" @tap="openCard()" />
          <view class="m-id" @tap="openCard()">
            <view class="m-name">
              {{ m.nickname || '神秘朋友' }}
              <image v-if="m.is_verified" class="m-v" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
            </view>
            <view class="m-time">{{ shortTime(m.created_at) }}</view>
          </view>
        </view>
        <view v-if="m.content" class="m-text">{{ m.content }}</view>
        <view v-if="imgList.length" class="m-grid" :class="'g' + Math.min(imgList.length, 3)">
          <image
            v-for="(img, i) in imgList" :key="i" class="m-img" :src="img"
            :mode="imgList.length === 1 ? 'widthFix' : 'aspectFill'"
            @tap="previewImgs(i)"
          />
        </view>
        <view class="m-acts">
          <text class="m-act" :class="{ on: m.liked, pop: likePop }" hover-class="act-press" @tap="toggleLike()">{{ m.liked ? '❤️' : '🤍' }} {{ m.like_count || 0 }}</text>
          <text class="m-act gift" hover-class="act-press" @tap="openGift()">🎁 送礼</text>
        </view>
      </view>

      <!-- 评论区(页面内嵌) -->
      <view class="cmt-h">全部评论 · {{ m.comment_count || 0 }}</view>
      <view v-for="cm in cmts" :key="cm.comment_id" class="cm-item">
        <user-avatar :name="cm.nickname" :src="cm.avatar" :size="56" shape="circle" />
        <view class="cm-body" @tap="setReplyTo(cm)">
          <view class="cm-meta">
            {{ cm.nickname || '匿名' }}
            <text v-if="cm.type !== 'gift' && cm.reply_to_nick" class="cm-rp">▸ {{ cm.reply_to_nick }}</text>
            <text class="cm-time">{{ shortTime(cm.created_at) }}</text>
          </view>
          <view v-if="cm.type === 'gift'" class="cm-gift">
            🎁 送出了「{{ giftInfo(cm).name }}」
            <image v-if="giftInfo(cm).icon" class="cm-gift-ic" :src="giftInfo(cm).icon" mode="aspectFit" />
          </view>
          <view v-else class="cm-text">{{ cm.content }}</view>
        </view>
        <text v-if="canDelCmt(cm) && cm.type !== 'gift'" class="cm-del" @tap="delComment(cm)">删</text>
      </view>
      <view v-if="!cmts.length" class="cm-empty">还没有评论,来抢沙发</view>
      <view style="height: 140rpx;" />

      <!-- 底部常驻评论输入 -->
      <view class="input-bar">
        <text class="ib-gift" hover-class="act-press" @tap="openGift()">🎁</text>
        <input
          v-model="cmtText" class="ib-input" :placeholder="replyTo ? `回复 ${replyTo.nickname}…` : '友善评论…'"
          confirm-type="send" :cursor-spacing="16" @confirm="sendComment()"
        />
        <text v-if="replyTo" class="ib-cancel" @tap="replyTo = null">✕</text>
        <view class="ib-send" hover-class="act-press" @tap="sendComment()">发送</view>
      </view>

      <!-- 送礼成功动效 -->
      <view v-if="giftFx" class="gift-fx">
        <image v-if="giftFx.icon" class="gf-img" :src="giftFx.icon" mode="aspectFit" />
        <text v-else class="gf-emoji">🎁</text>
        <text class="gf-name">「{{ giftFx.name }}」已送出</text>
      </view>
    </block>

    <user-card ref="ucard" @chat="onCardChat($event)" @like="onCardLike($event)" />
    <gift-picker ref="gpicker" @send="onGiftSend($event)" />
    </block>
  </view>
</template>

<script>
import { momentApi, chatApi, relationApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { useWalletStore } from '../../store/wallet'
import UserCard from '../../components/user-card/user-card.vue'
import GiftPicker from '../../components/gift-picker/gift-picker.vue'
import pagesCover from '../../mixins/pagesCover'

export default {
  components: { UserCard, GiftPicker },
  mixins: [pagesCover],
  data() {
    return {
      id: '', m: null, loading: true,
      cmts: [], cmtText: '', replyTo: null,
      giftFx: null, likePop: false
    }
  },
  computed: {
    myId() { return useUserStore().userId },
    imgList() {
      try { const a = JSON.parse((this.m && this.m.images) || '[]'); return Array.isArray(a) ? a : [] } catch (e) { return [] }
    }
  },
  async onLoad(opt) {
    this.id = opt.id || ''
    await this.loadCover()
    if (this.showCover) return
    this.reload()
  },
  methods: {
    async reload() {
      if (!this.id) { this.loading = false; return }
      this.loading = true
      try {
        this.m = await momentApi.detail(this.id)
        this.cmts = await momentApi.comments(this.id, { page: 1, size: 50 }) || []
      } catch (e) { this.m = null } finally { this.loading = false }
    },
    shortTime(t) { return t ? String(t).slice(5, 16).replace('T', ' ') : '刚刚' },
    giftInfo(cm) {
      try { return JSON.parse(cm.content || '{}') || {} } catch (e) { return {} }
    },
    previewImgs(i) { uni.previewImage({ urls: this.imgList, current: this.imgList[i] }) },
    async toggleLike() {
      this.likePop = true
      setTimeout(() => { this.likePop = false }, 450)
      try {
        const r = await momentApi.like(this.m.moment_id)
        this.m.liked = !!(r && r.liked)
        this.m.like_count = Math.max(0, (this.m.like_count || 0) + (this.m.liked ? 1 : -1))
      } catch (e) {}
    },
    openCard() {
      this.$refs.ucard.show(this.m.user_id)
    },
    async onCardChat(userId) {
      try {
        const chat = await chatApi.start(userId, 0)
        uni.navigateTo({ url: '/pages/chat/chat?id=' + chat.chat_id })
      } catch (e) {
        if (e && e.code === 5001) {
          uni.showModal({
            title: '金币不足', content: '余额不足,前往充值?',
            success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
          })
        }
      }
    },
    async onCardLike(userId) {
      try { await relationApi.like(userId); uni.showToast({ title: '已喜欢 ❤️', icon: 'none' }) } catch (e) {}
    },
    setReplyTo(cm) {
      if (String(cm.user_id) === String(this.myId)) return
      this.replyTo = cm
    },
    canDelCmt(cm) {
      return String(cm.user_id) === String(this.myId) ||
        (this.m && String(this.m.user_id) === String(this.myId))
    },
    async sendComment() {
      const text = this.cmtText.trim()
      if (!text || !this.m) return
      try {
        const cv = await momentApi.comment(this.m.moment_id, text, this.replyTo ? this.replyTo.user_id : 0)
        if (cv) this.cmts.push(cv)
        this.cmtText = ''
        this.replyTo = null
        this.m.comment_count = (this.m.comment_count || 0) + 1
      } catch (e) {
        uni.showToast({ title: (e && e.message) || '评论失败', icon: 'none' })
      }
    },
    delComment(cm) {
      uni.showModal({
        title: '删除评论', content: '确定删除这条评论?',
        success: async (r) => {
          if (!r.confirm) return
          try {
            await momentApi.removeComment(cm.comment_id)
            this.cmts = this.cmts.filter((x) => x.comment_id !== cm.comment_id)
            this.m.comment_count = Math.max(0, (this.m.comment_count || 0) - 1)
          } catch (e) {}
        }
      })
    },
    openGift() {
      if (!this.m) return
      if (String(this.m.user_id) === String(this.myId)) { uni.showToast({ title: '不能送给自己哦', icon: 'none' }); return }
      this.$refs.gpicker.show()
    },
    async onGiftSend(item) {
      if (!this.m) { this.$refs.gpicker.done(false); return }
      try {
        const cv = await momentApi.gift(this.m.moment_id, item.item_id, { showError: false })
        this.$refs.gpicker.done(true)
        if (cv) this.cmts.push(cv)
        this.m.comment_count = (this.m.comment_count || 0) + 1
        useWalletStore().fetchBalance()
        this.giftFx = { name: item.name, icon: item.icon }
        setTimeout(() => { this.giftFx = null }, 1200)
      } catch (e) {
        this.$refs.gpicker.done(false)
        if (e && e.code === 5001) {
          uni.showModal({
            title: '金币不足', content: '余额不够送这个礼物,前往充值?',
            success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
          })
        } else {
          uni.showToast({ title: (e && e.message) || '赠送失败', icon: 'none' })
        }
      }
    }
  }
}
</script>

<style lang="scss">
.page { padding: 20rpx 28rpx 40rpx; min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #F6FAFC 20%); }

.mcard {
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 24rpx; box-shadow: $shadow-card;
}
.m-hd { display: flex; align-items: center; gap: 16rpx; }
.m-id { flex: 1; min-width: 0; }
.m-name { font-size: 34rpx; font-weight: 700; color: $ink-900; display: flex; align-items: center; gap: 8rpx;
  .m-v { width: 32rpx; height: 32rpx; } }
.m-time { font-size: 24rpx; color: $ink-faint; margin-top: 4rpx; }
.m-text { font-size: 32rpx; line-height: 1.6; color: $ink-900; margin-top: 16rpx; word-break: break-word; white-space: pre-wrap; }
.m-grid { display: flex; flex-wrap: wrap; gap: 8rpx; margin-top: 16rpx;
  &.g1 .m-img { width: 420rpx; height: auto; }
  &.g2 .m-img { width: calc(50% - 4rpx); height: 300rpx; }
  &.g3 .m-img { width: calc(33.33% - 6rpx); height: 205rpx; }
}
.m-img { border-radius: 12rpx; }
.m-acts { display: flex; gap: 36rpx; margin-top: 20rpx; }
.m-act {
  font-size: 32rpx; color: $ink-soft; font-weight: 600; padding: 8rpx 12rpx; border-radius: 12rpx;
  transition: transform 0.15s ease;
  &.on { color: $coral-deep; }
  &.gift { color: #d9480f; }
  &.pop { animation: like-pop 0.45s ease; }
}
.act-press { transform: scale(0.88); opacity: 0.75; background: rgba(12,42,51,0.06); }
@keyframes like-pop {
  0% { transform: scale(1); }
  35% { transform: scale(1.45); }
  60% { transform: scale(0.9); }
  100% { transform: scale(1); }
}

.cmt-h { font-size: 28rpx; font-weight: 800; color: $ink-900; padding: 28rpx 8rpx 8rpx; }
.cm-item { display: flex; gap: 16rpx; padding: 18rpx 8rpx; border-bottom: 1rpx solid rgba(12,42,51,0.05); }
.cm-body { flex: 1; min-width: 0; }
.cm-meta { font-size: 22rpx; color: $sea-deep; font-weight: 700;
  .cm-rp { color: $ink-faint; font-weight: 400; margin-left: 8rpx; }
  .cm-time { color: $ink-faint; font-weight: 400; margin-left: 12rpx; font-size: 20rpx; } }
.cm-text { font-size: 26rpx; color: $ink-900; line-height: 1.55; margin-top: 6rpx; word-break: break-word; }
.cm-gift { font-size: 26rpx; color: #d9480f; font-weight: 700; margin-top: 6rpx; display: flex; align-items: center; gap: 8rpx;
  .cm-gift-ic { width: 44rpx; height: 44rpx; } }
.cm-del { flex: none; align-self: center; font-size: 22rpx; color: #e5484d; padding: 8rpx; }
.cm-empty { text-align: center; font-size: 24rpx; color: $ink-faint; padding: 50rpx 0; }

.input-bar {
  position: fixed; left: 0; right: 0; bottom: 0; z-index: 50;
  display: flex; align-items: center; gap: 14rpx;
  background: #fff; border-top: 1rpx solid $line;
  padding: 16rpx 24rpx calc(16rpx + env(safe-area-inset-bottom));
}
.ib-gift { flex: none; font-size: 40rpx; padding: 0 4rpx; border-radius: 12rpx; transition: transform 0.15s ease; }
.ib-input { flex: 1; height: 68rpx; background: #F4F7FA; border-radius: 999rpx; padding: 0 24rpx; font-size: 26rpx; }
.ib-cancel { font-size: 24rpx; color: $ink-400; }
.ib-send {
  flex: none; height: 68rpx; line-height: 68rpx; padding: 0 30rpx; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; font-size: 26rpx; font-weight: 700;
}

.gift-fx {
  position: fixed; left: 50%; top: 42%; z-index: 1005; transform: translate(-50%, -50%);
  display: flex; flex-direction: column; align-items: center; gap: 12rpx;
  animation: gift-float 1.2s ease forwards; pointer-events: none;
  .gf-img { width: 140rpx; height: 140rpx; }
  .gf-emoji { font-size: 120rpx; }
  .gf-name { font-size: 28rpx; font-weight: 800; color: #d9480f; background: rgba(255,255,255,0.92);
    padding: 8rpx 24rpx; border-radius: 999rpx; box-shadow: 0 6rpx 20rpx rgba(0,0,0,0.12); }
}
@keyframes gift-float {
  0% { opacity: 0; transform: translate(-50%, -30%) scale(0.4); }
  25% { opacity: 1; transform: translate(-50%, -50%) scale(1.15); }
  45% { transform: translate(-50%, -50%) scale(1); }
  100% { opacity: 0; transform: translate(-50%, -110%) scale(0.9); }
}
</style>
