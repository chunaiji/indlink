<template>
  <view class="chat">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <scroll-view scroll-y class="scroll" :scroll-top="scrollTop" :scroll-with-animation="true">
      <ad-slot slot-key="banner_chat" type="banner" />
      <view class="day"><text class="day-chip">{{ chatBanner }}</text></view>
      <template v-for="m in messages">
        <!-- 系统消息:居中提示 -->
        <view v-if="m.type === 'system'" :key="m.message_id" class="sys-tip"><text class="sys-chip">{{ m.content }}</text></view>
        <!-- 礼物消息:居中礼物卡片 -->
        <view v-else-if="m.type === 'gift'" :key="m.message_id" class="gift-msg">
          <view class="gift-msg-card">
            <image v-if="giftIcon(m)" :src="giftIcon(m)" class="gift-msg-ic" mode="aspectFit" />
            <text v-else class="gift-msg-emoji">🎁</text>
            <text class="gift-msg-txt">{{ mine(m) ? '你送出' : '对方送出' }} {{ giftName(m) }}</text>
          </view>
        </view>
        <view
          v-else
          :key="m.message_id"
          class="msg"
          :class="mine(m) ? 'me' : 'them'"
        >
          <!-- 对方:头像在左 / 自己:row-reverse 下第一个子元素排到最右 -->
          <user-avatar v-if="!mine(m)" class="av" :name="partnerNickname || '对方'" :src="partnerAvatar" :size="76" shape="circle" />
          <user-avatar v-if="mine(m)" class="av" :name="myNickname" :src="myAvatar" :size="76" shape="circle" />
          <view class="col">
            <text v-if="!mine(m)" class="nm">{{ partnerNickname || '对方' }}</text>
            <view class="bub" :class="m.type === 'image' ? 'img' : (mine(m) ? 'me-bub' : 'them-bub')">
              <image v-if="m.type === 'image'" :src="m.content" mode="widthFix" class="bub-img" @tap="preview(m.content)" />
              <text v-else>{{ m.content }}</text>
            </view>
          </view>
        </view>
      </template>
    </scroll-view>

    <view class="footer" :style="keyboardH ? `bottom: ${keyboardH}px` : ''">
      <view class="quick">
        <text v-for="q in quicks" :key="q" @tap="quickSend(q)">{{ q }}</text>
      </view>
      <view class="inputbar">
        <text class="ic" @tap="sendImage($event)">📷</text>
        <text class="ic" @tap="sendGift($event)">🎁</text>
        <input v-model="draft" class="field" placeholder="说点什么…" confirm-type="send"
               :adjust-position="false" cursor-spacing="20"
               @confirm="send($event)" @focus="e => { keyboardH = e.detail.height || 0; scrollToBottom() }" @blur="keyboardH = 0" />
        <view class="sendbtn" :class="{ on: draft.trim() }" @tap="send($event)">发送</view>
      </view>
    </view>

    <!-- 礼物面板 -->
    <view v-if="giftPanel" class="gp-mask" @tap="giftPanel = false"></view>
    <view v-if="giftPanel" class="gp">
      <swiper class="gp-swiper" :indicator-dots="giftPages.length > 1" indicator-active-color="#3BA9CC" indicator-color="rgba(0,0,0,0.12)">
        <swiper-item v-for="(page, pi) in giftPages" :key="pi">
          <view class="gp-grid">
            <view v-for="g in page" :key="g.item_id" class="gp-cell" :class="{ on: selGift === g.item_id }" @tap="selGift = g.item_id">
              <text v-if="ownedCount(g)" class="gp-own">×{{ ownedCount(g) }}</text>
              <image v-if="isImg(g.icon)" :src="g.icon" class="gp-ic" mode="aspectFit" />
              <text v-else class="gp-emoji">🎁</text>
              <text class="gp-name">{{ g.name }}</text>
              <view class="gp-price"><image class="gp-coin" src="/static/icons/coin.png" mode="aspectFit" /><text>{{ g.price_coin }}</text></view>
            </view>
          </view>
        </swiper-item>
      </swiper>
      <view class="gp-cap">赠送礼物，对方可增加魅力值，数量与金币相同</view>
      <view class="gp-bar">
        <image class="gp-coin" src="/static/icons/coin.png" mode="aspectFit" />
        <text class="gp-bal">{{ balance }}</text>
        <view class="gp-recharge" @tap="goRecharge($event)">充值</view>
        <view class="gp-send" :class="{ on: selGift }" @tap="doSendGift($event)">赠送</view>
      </view>
    </view>
    </block>
  </view>
</template>


<script>
import { chatApi, itemApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { useWalletStore } from '../../store/wallet'
import { onWSMessage, onWSReconnect } from '../../utils/ws'
import { chooseAndUploadImage } from '../../utils/upload'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return {
      chatId: '',
      messages: [],
      draft: '',
      scrollTop: 0,
      myId: 0,
      myNickname: '',
      myAvatar: '',
      partnerNickname: '',
      partnerAvatar: '',
      quicks: useUserStore().chatQuicks,
      chatBanner: useUserStore().uiText.chatBanner,
      off: null,
      offReconnect: null,
      keyboardH: 0,
      giftPanel: false,
      gifts: [],
      owned: {},
      selGift: 0
    }
  },
  computed: {
    balance() { return useWalletStore().balance },
    // 礼物按 8 个一页分组(参考图分页)
    giftPages() {
      const pages = []
      for (let i = 0; i < this.gifts.length; i += 8) pages.push(this.gifts.slice(i, i + 8))
      return pages
    }
  },
  async onLoad(q) {
    this.chatId = q.id
    await this.loadCover()
    if (this.showCover) return
    const store = useUserStore()
    this.myId = store.userId
    this.myNickname = (store.profile && store.profile.nickname) || '我'
    this.myAvatar = (store.profile && store.profile.avatar) || ''
    if (q.name) {
      this.partnerNickname = decodeURIComponent(q.name)
      uni.setNavigationBarTitle({ title: this.partnerNickname })
    }
    if (q.avatar) this.partnerAvatar = decodeURIComponent(q.avatar)
    this.load()
    this.off = onWSMessage((msg) => {
      if (msg && msg.event === 'message' && String(msg.chat_id) === this.chatId) {
        this.messages.push(msg.message)
        this.scrollToBottom()
      }
    })
    this.offReconnect = onWSReconnect(() => {
      // WS 断线重连后补拉最新消息
      this.load()
    })
  },
  onUnload() {
    if (this.off) this.off()
    if (this.offReconnect) this.offReconnect()
  },
  methods: {
    // int64 ID 在 JS 端为字符串,统一转字符串比较,避免类型不一致导致全部判为对方
    mine(m) { return String(m.sender_id) === String(this.myId) },
    async load() {
      try {
        this.messages = await chatApi.messages(this.chatId, { page: 1, size: 30 }) || []
        this.scrollToBottom()
      } catch (e) {}
    },
    async send() {
      const text = this.draft.trim()
      if (!text) return
      this.draft = ''
      try {
        const msg = await chatApi.send(this.chatId, text, 'text', { showError: false })
        this.messages.push(msg)
        this.scrollToBottom()
      } catch (e) {
        this.draft = text // 失败回填,避免用户重输
        this.handleSendError(e)
      }
    },
    // 余额不足(每条消息扣费)弹充值,其余错误给通用提示
    handleSendError(e) {
      if (e && e.code === 5001) {
        uni.showModal({
          title: '金币不足', content: '余额不足,前往充值?',
          success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
        })
      } else if (e && e.msg) {
        uni.showToast({ title: e.msg, icon: 'none' })
      }
    },
    quickSend(q) { this.draft = q; this.send() },
    async sendImage() {
      try {
        const url = await chooseAndUploadImage()
        const msg = await chatApi.send(this.chatId, url, 'image', { showError: false })
        this.messages.push(msg)
        this.scrollToBottom()
      } catch (e) { this.handleSendError(e) }
    },
    preview(url) { uni.previewImage({ urls: [url] }) },
    // 打开礼物面板:拉礼物(gift/vip) + 自己各礼物的拥有数量 + 余额
    async sendGift() {
      try {
        const [items, owned] = await Promise.all([itemApi.list(), itemApi.myItems()])
        this.gifts = (items || []).filter((i) => i.type === 'gift' || i.type === 'vip')
        this.owned = owned || {}
      } catch (e) { this.gifts = []; this.owned = {} }
      if (!this.gifts.length) { uni.showToast({ title: '暂无礼物', icon: 'none' }); return }
      useWalletStore().fetchBalance()
      this.selGift = 0
      this.giftPanel = true
    },
    ownedCount(g) { return Number(this.owned[String(g.item_id)] || 0) },
    isImg(s) { return typeof s === 'string' && /^(https?:)?\/\//.test(s) },
    giftInfo(m) { try { return JSON.parse(m.content) } catch (e) { return null } },
    giftIcon(m) { const g = this.giftInfo(m); return g && this.isImg(g.icon) ? g.icon : '' },
    giftName(m) { const g = this.giftInfo(m); return g ? g.name : String(m.content || '礼物') },
    goRecharge() { this.giftPanel = false; uni.navigateTo({ url: '/pages/recharge/recharge' }) },
    async doSendGift() {
      if (!this.selGift) { uni.showToast({ title: '请选择礼物', icon: 'none' }); return }
      // 库存制:选中礼物数量不足 → 跳转充值页(充值后去道具商城购买)
      const have = Number(this.owned[String(this.selGift)] || 0)
      if (have < 1) {
        this.giftPanel = false
        uni.showToast({ title: '礼物数量不足，先去充值购买', icon: 'none' })
        setTimeout(() => uni.navigateTo({ url: '/pages/recharge/recharge' }), 800)
        return
      }
      try {
        const msg = await chatApi.gift(this.chatId, this.selGift, { showError: false })
        this.messages.push(msg)
        this.owned[String(this.selGift)] = have - 1
        this.giftPanel = false
        this.selGift = 0
        this.scrollToBottom()
      } catch (e) { this.handleSendError(e) }
    },
    scrollToBottom() {
      this._seq = (this._seq || 0) + 1
      const seq = this._seq
      this.$nextTick(() => {
        this.$nextTick(() => { this.scrollTop = 999999 + seq })
      })
    }
  }
}
</script>

<style lang="scss">
/* 微信风格:浅灰底 + 左右气泡带小尾巴 */
.chat { min-height: 100vh; background: #EDEDED; }
/* 底部留足固定 footer(快捷回复+输入栏)+ 安全区的高度,避免最新消息被输入栏挡住 */
.scroll { position: fixed; left: 0; right: 0; top: 0; bottom: 0; padding: 24rpx 24rpx 320rpx; padding-bottom: calc(320rpx + constant(safe-area-inset-bottom)); padding-bottom: calc(320rpx + env(safe-area-inset-bottom)); box-sizing: border-box; }
.footer { position: fixed; left: 0; right: 0; bottom: 0; z-index: 10; }
/* 顶部横幅:居中小胶囊 */
.day { display: flex; justify-content: center; margin: 16rpx 0 28rpx; }
.day-chip {
  font-size: 22rpx; color: #a6a6a6; letter-spacing: 0.5rpx;
  background: rgba(0, 0, 0, 0.04); padding: 6rpx 22rpx; border-radius: 22rpx;
}
/* 系统消息:居中胶囊(与顶部横幅同风格) */
.sys-tip { display: flex; justify-content: center; margin: 28rpx 0; }
.sys-chip {
  max-width: 82%;
  padding: 14rpx 26rpx;
  text-align: center;
  font-size: 26rpx;
  line-height: 1.5;
  color: #8a939d;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 24rpx;
}

.msg { display: flex; align-items: flex-start; margin-bottom: 32rpx; }
.msg.them { flex-direction: row; }
.msg.me { flex-direction: row-reverse; }
.av {
  width: 76rpx; height: 76rpx; border-radius: 50%; flex: none;
  background: linear-gradient(135deg, #BFE6F0, #8FD3E4);
  display: flex; align-items: center; justify-content: center; font-size: 32rpx;
}
.col { display: flex; flex-direction: column; max-width: 64%; margin: 0 20rpx; }
.msg.me .col { align-items: flex-end; }
.nm { font-size: 22rpx; color: #9b9b9b; margin: 2rpx 0 8rpx 6rpx; }

.bub {
  position: relative; padding: 20rpx 24rpx; border-radius: 10rpx;
  font-size: 31rpx; line-height: 1.45; color: #111; word-break: break-word; white-space: pre-wrap;
}
/* 对方:白底,尾巴在左上 */
.them-bub { background: #fff; }
.them-bub::after {
  content: ''; position: absolute; left: -14rpx; top: 24rpx;
  border: 8rpx solid transparent; border-right-color: #fff; border-left-width: 6rpx;
}
/* 自己:微信绿,尾巴在右上 */
.me-bub { background: #95EC69; }
.me-bub::after {
  content: ''; position: absolute; right: -14rpx; top: 24rpx;
  border: 8rpx solid transparent; border-left-color: #95EC69; border-right-width: 6rpx;
}
/* 图片消息:无气泡背景 */
.bub.img { padding: 0; background: transparent; }
.bub-img { width: 320rpx; border-radius: 12rpx; display: block; background: #e6e6e6; }
.quick {
  display: flex; gap: 16rpx; padding: 14rpx 24rpx; overflow-x: auto; white-space: nowrap;
  background: rgba(255,255,255,0.92); border-top: 1rpx solid $line;
  text {
    flex: none; font-size: 24rpx; color: $sea-deep; background: $ink-50;
    border: 1rpx solid $line; padding: 12rpx 24rpx; border-radius: 999rpx;
  }
}
.inputbar {
  display: flex; align-items: center; gap: 16rpx; padding: 16rpx 24rpx;
  padding-bottom: calc(16rpx + constant(safe-area-inset-bottom));
  padding-bottom: calc(16rpx + env(safe-area-inset-bottom));
  background: #fff; border-top: 1rpx solid $line;
  .ic { font-size: 40rpx; flex: none; }
  .field { flex: 1; background: $ink-50; border-radius: $r-sm; padding: 16rpx 24rpx; font-size: 28rpx; }
  .sendbtn {
    flex: none; background: $ink-200; color: #fff; font-weight: 600; padding: 14rpx 30rpx;
    border-radius: $r-sm; font-size: 26rpx; transition: background .15s;
    &.on { background: $coral; }
  }
}

/* 礼物消息卡片(居中) */
.gift-msg { display: flex; justify-content: center; margin: 24rpx 0; }
.gift-msg-card {
  display: flex; align-items: center; gap: 12rpx;
  background: rgba(255,145,60,0.12); border: 1rpx solid rgba(255,145,60,0.3);
  padding: 12rpx 24rpx; border-radius: 999rpx;
}
.gift-msg-ic { width: 48rpx; height: 48rpx; }
.gift-msg-emoji { font-size: 40rpx; }
.gift-msg-txt { font-size: 26rpx; color: #B8600A; font-weight: 600; }

/* 礼物面板 */
.gp-mask { position: fixed; inset: 0; background: rgba(0,0,0,0.35); z-index: 40; }
.gp {
  position: fixed; left: 0; right: 0; bottom: 0; z-index: 50; background: #fff;
  border-radius: 28rpx 28rpx 0 0; padding: 24rpx 0;
  padding-bottom: calc(24rpx + constant(safe-area-inset-bottom));
  padding-bottom: calc(24rpx + env(safe-area-inset-bottom));
}
.gp-swiper { height: 460rpx; }
.gp-grid { display: flex; flex-wrap: wrap; padding: 8rpx 12rpx; }
.gp-cell {
  position: relative;
  width: 25%; box-sizing: border-box; padding: 16rpx 8rpx; text-align: center;
  border: 2rpx solid transparent; border-radius: 18rpx;
  &.on { border-color: $coral; background: rgba(255,107,91,0.06); }
}
.gp-own {
  position: absolute; top: 6rpx; right: 10rpx;
  font-size: 20rpx; font-weight: 700; color: #fff;
  background: #FF913C; padding: 0 10rpx; border-radius: 999rpx; line-height: 30rpx;
}
.gp-ic { width: 96rpx; height: 96rpx; }
.gp-emoji { font-size: 80rpx; line-height: 96rpx; }
.gp-name { display: block; font-size: 24rpx; color: $ink-900; margin-top: 8rpx; }
.gp-price { display: flex; align-items: center; justify-content: center; gap: 4rpx; margin-top: 4rpx; font-size: 22rpx; color: #E8901A; font-weight: 600; }
.gp-coin { width: 30rpx; height: 30rpx; }
.gp-cap { text-align: center; font-size: 22rpx; color: $ink-faint; padding: 12rpx 24rpx; }
.gp-bar { display: flex; align-items: center; gap: 12rpx; padding: 16rpx 28rpx; border-top: 1rpx solid $line; }
.gp-bal { font-size: 30rpx; font-weight: 700; color: $ink-900; margin-right: auto; }
.gp-recharge { font-size: 26rpx; color: $sea-deep; border: 1rpx solid $sea-deep; padding: 10rpx 26rpx; border-radius: 999rpx; }
.gp-send { font-size: 28rpx; color: #fff; font-weight: 700; background: $ink-200; padding: 12rpx 44rpx; border-radius: 999rpx; }
.gp-send.on { background: linear-gradient(135deg, #5BD0E0, #3BA9CC); }
</style>
