<template>
  <view class="detail">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="letter" v-if="bottle">
      <view class="who">
        <user-avatar class="ava" :name="bottle.is_anonymous ? '匿' : '友'" :size="76" shape="circle" />
        <view>
          <view class="name">{{ senderName }}</view>
          <view class="sub">
            <text class="gp" :class="bottle.author_gender === 1 ? 'male' : 'female'">{{ genderText(bottle.author_gender) }} {{ bottle.author_age || '?' }}</text>
            {{ shortTime(bottle.created_at) }} · {{ bottle.city || '远方' }}
          </view>
        </view>
      </view>
      <view class="topic" v-if="bottle.tags">#{{ firstTag }}</view>
      <view class="qa">{{ bottle.content }}</view>
    </view>

    <view class="greet">
      <view class="greet-btn" @tap="openReply($event)">
        <text>回应一下</text>
      </view>
      <button class="greet-share" open-type="share">
        <image class="gs-ic-img" src="/static/icons/fenxiang.png" mode="aspectFit" />
        <text>分享</text>
      </button>
    </view>

    <ad-slot slot-key="banner_detail" type="banner" />
    <view class="cmt-h">全部回应 · {{ replies.length }}</view>
    <view v-for="r in replies" :key="r.reply_id" class="cmt">
      <user-avatar class="ci" :name="r.nickname || '友'" :src="r.avatar" :size="60" shape="circle" />
      <view class="cb">
        <view class="cmeta">{{ r.nickname || '匿名' }} · {{ shortTime(r.created_at) }}</view>
        <view v-if="!r.locked" class="content">{{ r.content }}</view>
        <view v-else class="locked" @tap="unlock(r)">
          <text class="masked">{{ r.content }}</text>
          <text class="u">🔒 {{ r.unlock_cost }}金币解锁</text>
        </view>
      </view>
      <view v-if="!isSelf(r)" class="cchat" @tap="goChat(r)">💬 开聊</view>
    </view>
    <empty-state v-if="!replies.length" icon="💬" title="还没有人回应" desc="来当第一个回信的人吧" />

    <!-- 回信输入(背景层与内容框分离,点框内不会误关)-->
    <view v-if="showReply" class="reply-mask">
      <view class="reply-bg" @tap="closeReply($event)"></view>
      <view class="reply-box" :style="keyboardH ? { bottom: keyboardH + 'px', paddingBottom: '32rpx' } : {}">
        <view class="reply-hd">
          <text>写回信</text>
          <text class="reply-x" @tap="closeReply($event)">✕</text>
        </view>
        <textarea v-model="replyContent" class="rta" placeholder="友善评论,说点温暖的话"
          :focus="showReply" adjust-position="false" :cursor-spacing="20"
          @focus="e => { keyboardH = e.detail.height || 0 }"
          @blur="keyboardH = 0" />
        <view class="quick">
          <text v-for="q in quicks" :key="q" @tap="pickQuick(q)">{{ q }}</text>
        </view>
        <button class="send" @tap="sendReply($event)">发送回信</button>
      </view>
    </view>
    </block>
  </view>
</template>

<script>
import { bottleApi, shareApi, chatApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { showInterstitial } from '../../utils/ad'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return {
      bottleId: 0,
      bottle: null,
      replies: [],
      showReply: false,
      replyContent: '',
      keyboardH: 0,
      quicks: useUserStore().replyQuicks,
      pendingShareReward: 0
    }
  },
  computed: {
    firstTag() { return (this.bottle && this.bottle.tags || '').split(',')[0] },
    senderName() {
      if (!this.bottle) return ''
      const t = useUserStore().uiText
      return this.bottle.is_anonymous ? t.anonSender : t.someFriend
    }
  },
  async onLoad(q) {
    this.bottleId = q.id
    await this.loadCover()
    if (this.showCover) return
    this.load()
    showInterstitial('inter_detail')
  },
  onShow() {
    this.flushShareReward()  // 分享回到页面后告知奖励
  },
  onShareAppMessage() {
    this.claimShareReward()
    const u = useUserStore()
    const r = { title: this.bottle ? String(this.bottle.content || '').slice(0, 18) || u.shareTitle : u.shareTitle, path: '/pages/detail/detail?id=' + this.bottleId }
    if (u.shareImage) r.imageUrl = u.shareImage
    return r
  },
  methods: {
    // 领奖在分享面板弹出瞬间发生,此时提示会被面板盖住,故暂存,回到页面(onShow)后再告知;
    // 部分机型分享后不触发 onShow,再加 1.5s 延迟兜底。
    claimShareReward() {
      shareApi.reward().then((r) => {
        if (r && r.rewarded && r.coins) {
          this.pendingShareReward = r.coins
          setTimeout(() => this.flushShareReward(), 1500)
        }
      }).catch(() => {})
    },
    flushShareReward() {
      const coins = this.pendingShareReward
      if (!coins) return
      this.pendingShareReward = 0
      uni.showModal({
        title: '分享成功 🎉',
        content: `恭喜获得 ${coins} 金币奖励，已存入钱包`,
        showCancel: false,
        confirmText: '收下'
      })
    },
    async load() {
      try {
        this.bottle = await bottleApi.detail(this.bottleId)
        this.replies = await bottleApi.replies(this.bottleId) || []
      } catch (e) {}
    },
    async unlock(r) {
      uni.showModal({
        title: '解锁回应', content: `消耗 ${r.unlock_cost} 金币查看完整内容?`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            const data = await bottleApi.unlock(r.reply_id)
            r.content = data.content
            r.locked = false
          } catch (e) {
            if (e && e.code === 5001) {
              uni.showModal({
                title: '金币不足', content: '前往充值?',
                success: (rr) => { if (rr.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
              })
            }
          }
        }
      })
    },
    async sendReply() {
      if (!this.replyContent.trim()) return
      try {
        await bottleApi.reply(this.bottleId, this.replyContent)
        this.replyContent = ''
        this.showReply = false
        uni.showToast({ title: '回应已送出', icon: 'none' })
        this.load()
      } catch (e) {}
    },
    genderText(g) { return g === 1 ? '♂' : g === 2 ? '♀' : '·' },
    openReply() { this.showReply = true },
    closeReply() { this.showReply = false },
    isSelf(r) { return String(r.user_id) === String(useUserStore().userId) },
    // 与扩列开聊同一后端接口:未有会话时扣开聊币,已有会话直接进入不重复扣
    async goChat(r) {
      try {
        const chat = await chatApi.start(r.user_id, this.bottleId)
        uni.navigateTo({ url: `/pages/chat/chat?id=${chat.chat_id}&name=${encodeURIComponent(r.nickname || '')}&avatar=${encodeURIComponent(r.avatar || '')}` })
      } catch (e) {
        if (e && e.code === 5001) {
          uni.showModal({
            title: '金币不足', content: '开聊需要消耗金币,余额不足,前往充值?',
            success: (res) => { if (res.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
          })
        } else if (e && e.message) {
          uni.showToast({ title: e.message, icon: 'none' })
        }
      }
    },
    pickQuick(q) { this.replyContent = q },
    shortTime(t) { return t ? String(t).slice(5, 16).replace('T', ' ') : '刚刚' }
  }
}
</script>

<style lang="scss">
.detail { min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #DDEFF3); padding-bottom: 40rpx; }
.letter {
  margin: 24rpx 28rpx 0; padding: 32rpx; background: $card; border-radius: $r-lg;
  box-shadow: $shadow-card;
}
.who { display: flex; gap: 20rpx; align-items: center; margin-bottom: 20rpx; }
.ava {
  width: 76rpx; height: 76rpx; border-radius: $r-avatar;
  background: linear-gradient(135deg, #FFD8C2, #FFB59B);
  display: flex; align-items: center; justify-content: center; font-size: 36rpx;
}
.name { font-weight: 700; font-size: 30rpx; }
.sub { font-size: 22rpx; color: $ink-soft; margin-top: 6rpx; }
.gp {
  font-size: 20rpx; font-weight: 700; padding: 2rpx 12rpx; border-radius: 999rpx; margin-right: 8rpx;
  &.female { color: #E0533F; background: rgba(255,107,91,0.14); }
  &.male { color: $sea-deep; background: rgba(79,201,240,0.16); }
}
.topic { font-size: 24rpx; color: $coral-deep; font-weight: 700; margin-bottom: 12rpx; }
.qa { font-size: 30rpx; line-height: 1.7; color: $ink; }
.greet { margin: 28rpx; display: flex; gap: 20rpx; justify-content: center;
  .greet-btn {
    display: inline-flex; align-items: center; gap: 12rpx;
    background: linear-gradient(135deg, #5BD0E0, #3BA9CC); color: #fff; font-weight: 700;
    font-size: 30rpx; padding: 22rpx 56rpx; border-radius: $r-lg;
    box-shadow: $shadow-pop;
  }
}
.greet-share {
  display: inline-flex; align-items: center; gap: 8rpx;
  background: rgba(91,208,224,0.12); border: 1rpx solid rgba(91,208,224,0.4);
  color: $sea-deep; font-weight: 700; font-size: 28rpx;
  padding: 0 32rpx; border-radius: $r-lg; margin: 0; line-height: normal;
  .gs-ic { font-size: 32rpx; }
  .gs-ic-img { width: 32rpx; height: 32rpx; }
}
.greet-share::after { border: none; }
.cmt-h { font-size: 26rpx; font-weight: 700; padding: 8rpx 32rpx 16rpx; }
.cmt { display: flex; gap: 20rpx; padding: 16rpx 32rpx; }
.ci { width: 60rpx; height: 60rpx; flex: none; }
.cb { flex: 1; min-width: 0; }
.cchat {
  flex: none; align-self: center; font-size: 20rpx; font-weight: 700; color: $coral-deep;
  border: 1rpx solid $coral; border-radius: 999rpx; padding: 8rpx 18rpx;
}
.cmeta { font-size: 20rpx; color: $ink-faint; margin-bottom: 8rpx; }
.content { font-size: 28rpx; color: $ink; line-height: 1.5; word-break: break-word; }
.locked {
  font-size: 28rpx; color: $ink-soft; background: #EDF4F6; border-radius: 16rpx;
  padding: 16rpx 20rpx; display: flex; align-items: center; gap: 12rpx;
  .masked { flex: 1; min-width: 0; color: $ink; }
  .u { flex: none; color: $coral-deep; font-weight: 700; font-size: 24rpx; }
}
.empty { text-align: center; color: $ink-faint; padding: 60rpx 0; font-size: 26rpx; }
.reply-mask {
  position: fixed; top: 0; right: 0; bottom: 0; left: 0; display: flex; align-items: flex-end; z-index: 99;
}
.reply-bg { position: absolute; top: 0; right: 0; bottom: 0; left: 0; background: rgba(12,42,51,0.45); }
.reply-box {
  position: absolute; bottom: 0; left: 0; right: 0; z-index: 1; background: #fff;
  border-radius: $r-lg $r-lg 0 0; padding: 32rpx;
  padding-bottom: calc(32rpx + constant(safe-area-inset-bottom));
  padding-bottom: calc(32rpx + env(safe-area-inset-bottom));
}
.reply-hd { display: flex; justify-content: space-between; align-items: center; font-size: 30rpx; font-weight: 600; margin-bottom: 20rpx; }
.reply-x { font-size: 32rpx; color: $ink-400; padding: 0 8rpx; }
.rta { width: 100%; height: 160rpx; background: #F2F8FA; border-radius: 20rpx; padding: 20rpx; font-size: 28rpx; box-sizing: border-box; }
.quick { display: flex; gap: 16rpx; flex-wrap: wrap; margin: 20rpx 0;
  text { font-size: 24rpx; color: $sea-deep; background: rgba(79,201,240,0.12); padding: 10rpx 20rpx; border-radius: 999rpx; }
}
.send {
  height: 88rpx; line-height: 88rpx; border-radius: 24rpx;
  background: linear-gradient(135deg, $coral, $coral-deep); color: #fff; font-weight: 700; font-size: 30rpx;
}
</style>
