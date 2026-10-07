<template>
  <view class="page" :class="{ square: squareOn }">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view v-if="rankOn" class="rank-banner" @tap="goRank()">
      ✨ 魅力周榜 · 看看本周谁最受欢迎 <text class="rb-go">›</text>
    </view>

    <!-- ════ 动态广场(square 开启) ════ -->
    <block v-if="squareOn">
      <sk-list v-if="feedLoading && !feed.length" :rows="5" />
      <block v-else>
        <view v-for="m in feed" :key="m.moment_id" class="mcard">
          <view class="m-hd">
            <user-avatar :name="m.nickname" :src="m.avatar" :size="76" shape="circle" @tap="openCardById(m.user_id)" />
            <view class="m-id" @tap="openCardById(m.user_id)">
              <view class="m-name">
                {{ m.nickname || '神秘朋友' }}
                <image v-if="m.is_verified" class="m-v" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
              </view>
              <view class="m-time">{{ shortTime(m.created_at) }}</view>
            </view>
            <text v-if="isMine(m)" class="m-del" @tap="delMoment(m)">删除</text>
          </view>
          <view v-if="m.content" class="m-text">{{ m.content }}</view>
          <view v-if="imgs(m).length" class="m-grid" :class="'g' + Math.min(imgs(m).length, 3)">
            <image
              v-for="(img, i) in imgs(m)" :key="i" class="m-img" :src="img"
              :mode="imgs(m).length === 1 ? 'widthFix' : 'aspectFill'"
              @tap="previewImgs(m, i)"
            />
          </view>
          <view class="m-acts">
            <text
              class="m-act" :class="{ on: m.liked, pop: likePopId === m.moment_id }"
              hover-class="act-press" @tap="toggleLike(m)"
            >{{ m.liked ? '❤️' : '🤍' }} {{ m.like_count || 0 }}</text>
            <text class="m-act" hover-class="act-press" @tap="openComments(m)">💬 {{ m.comment_count || 0 }}</text>
            <text class="m-act gift" hover-class="act-press" @tap="openGift(m)">🎁 送礼</text>
          </view>
          <view v-if="(m.previews || []).length" class="m-cmts" @tap="openComments(m)">
            <view v-for="p in m.previews" :key="p.comment_id" class="m-cmt" :class="{ 'is-gift': p.type === 'gift' }">
              <text class="c-nick">{{ p.nickname || '匿名' }}</text>
              <template v-if="p.type === 'gift'">
                <text class="c-gift">送出了「{{ giftInfo(p).name }}」🎁</text>
              </template>
              <template v-else>
                <text v-if="p.reply_to_nick" class="c-reply">回复 {{ p.reply_to_nick }}</text>
                <text class="c-body">：{{ p.content }}</text>
              </template>
            </view>
            <view v-if="m.comment_count > m.previews.length" class="m-more">查看全部 {{ m.comment_count }} 条评论 ›</view>
          </view>
        </view>
        <view v-if="feedLoading && feed.length" class="feed-tip">加载中…</view>
        <view v-else-if="feedEnd && feed.length" class="feed-tip">— 到底啦 —</view>
        <empty-state v-if="!feed.length" icon="🌈" title="还没有动态" desc="点右下角 ＋ 发第一条动态,让大家认识你" />
        <ad-slot slot-key="banner_expand" type="banner" />
      </block>

      <!-- 发布浮动按钮 -->
      <view class="post-fab" @tap="openPost()">＋</view>

      <!-- 发布弹层(mask 不做点击关闭:chooseImage 返回瞬间的穿透 tap 会误关面板丢草稿,仅 ✕ 关闭) -->
      <view v-if="postShow" class="po-mask">
        <view class="po-panel">
          <view class="po-hd">发动态 <text class="po-x" @tap="postShow = false">✕</text></view>
          <textarea v-model="postText" class="po-ta" :maxlength="500" placeholder="分享此刻的心情…" />
          <view class="po-grid">
            <image v-for="(img, i) in postImgs" :key="i" class="po-img" :src="img" mode="aspectFill" @tap="removePostImg(i)" />
            <view v-if="postImgs.length < 9" class="po-add" @tap="pickPostImgs()">＋</view>
          </view>
          <view class="po-tip">{{ postImgs.length ? '点图片可移除 · ' : '' }}{{ postText.length }}/500</view>
          <button class="po-send" :loading="posting" @tap="submitPost()">发布</button>
        </view>
      </view>

      <!-- 评论弹层(背景层与内容框分离:@tap.stop 在本 uni-app 版本不可靠,点框内会误关) -->
      <view v-if="cmtShow" class="cm-mask">
        <view class="cm-bg" @tap="closeComments()"></view>
        <view class="cm-panel">
          <view class="cm-hd">评论 {{ cmtMoment ? cmtMoment.comment_count || 0 : 0 }} <text class="cm-x" @tap="closeComments()">✕</text></view>
          <scroll-view scroll-y class="cm-list">
            <view v-if="cmtLoading" class="cm-tip">加载中…</view>
            <block v-else>
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
              <view v-if="!cmts.length" class="cm-tip">还没有评论,来抢沙发</view>
            </block>
          </scroll-view>
          <view class="cm-input-row">
            <text class="cm-gift-btn" hover-class="act-press" @tap="openGift(cmtMoment)">🎁</text>
            <input
              v-model="cmtText" class="cm-input" :placeholder="replyTo ? `回复 ${replyTo.nickname}…` : '友善评论…'"
              confirm-type="send" :cursor-spacing="16" @confirm="sendComment()"
            />
            <text v-if="replyTo" class="cm-cancel" @tap="replyTo = null">✕</text>
            <view class="cm-send" hover-class="act-press" @tap="sendComment()">发送</view>
          </view>
        </view>
      </view>

      <!-- 送礼成功动效:礼物从中心放大上飘 -->
      <view v-if="giftFx" class="gift-fx">
        <image v-if="giftFx.icon" class="gf-img" :src="giftFx.icon" mode="aspectFit" />
        <text v-else class="gf-emoji">🎁</text>
        <text class="gf-name">「{{ giftFx.name }}」已送出</text>
      </view>

      <gift-picker ref="gpicker" @send="onGiftSend($event)" />
    </block>

    <!-- ════ 旧扩列用户列表(square 关闭时兜底) ════ -->
    <block v-else>
      <view class="segtabs">
        <text :class="{ on: type === 'active' }" @tap="setType('active')">活跃</text>
        <text :class="{ on: type === 'new' }" @tap="setType('new')">新人</text>
        <text :class="{ on: type === 'nearby' }" @tap="setType('nearby')">附近</text>
      </view>

      <scroll-view scroll-y class="list">
        <sk-list v-if="loading" :rows="6" />
        <block v-else>
          <view v-for="u in users" :key="u.user_id" class="ucard" @tap="openCard(u)" @longpress="moreActions(u)">
            <view class="ava-wrap">
              <image v-if="u.is_verified" class="vbadge" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
              <user-avatar class="ava" :name="u.nickname" :src="u.avatar" :size="84" shape="circle" />
            </view>
            <view class="info">
              <view class="name">{{ u.nickname }}</view>
              <view class="pills">
                <text class="pill g" :class="u.gender === 1 ? 'male' : 'female'">{{ genderText(u.gender) }} {{ u.age || '' }}</text>
                <text class="pill">{{ u.city || '未知' }}</text>
                <text class="pill">{{ u.online_hint || '刚刚' }}</text>
              </view>
              <view class="rel">粉丝 <text class="n">{{ u.fans_count || 0 }}</text>　魅力 <text class="n">{{ u.charm || 0 }}</text>　注册 <text class="n">{{ regDays(u.created_at) }}</text> 天</view>
            </view>
            <view class="ract">
              <text v-if="!paidUsers[u.user_id]" class="cost">−{{ chatPrice }}金币</text>
              <view class="chatbtn" :class="{ free: paidUsers[u.user_id] }" @tap.stop="startChat(u)">
                开聊<image class="hearts" :src="paidUsers[u.user_id] ? '/static/icons/xindong_1.png' : '/static/icons/xindong.png'" mode="aspectFit" />
              </view>
            </view>
          </view>
          <empty-state v-if="!users.length" icon="💗" title="还没有人" desc="过会儿再来逛逛扩列墙" />
          <ad-slot slot-key="banner_expand" type="banner" />
        </block>
      </scroll-view>
    </block>
    </block>
    <user-card ref="ucard" @chat="onCardChat($event)" @like="onCardLike($event)" />
  </view>
  <ad-slot v-if="!showCover" slot-key="banner_expand" type="banner" />
  <tab-bar :current="2" />
</template>

<script>
import { matchApi, chatApi, moderationApi, relationApi, momentApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { useFeaturesStore } from '../../store/features'
import { getLocationOnce } from '../../utils/platform'
import { chooseAndUploadImages } from '../../utils/upload'
import pagesCover from '../../mixins/pagesCover'
import UserCard from '../../components/user-card/user-card.vue'
import GiftPicker from '../../components/gift-picker/gift-picker.vue'
import { useWalletStore } from '../../store/wallet'

export default {
  components: { UserCard, GiftPicker },
  mixins: [pagesCover],
  data() {
    return {
      type: 'active', users: [], paidUsers: {}, located: false, loading: true,
      // 动态广场
      feed: [], feedPage: 1, feedLoading: false, feedEnd: false,
      postShow: false, postText: '', postImgs: [], posting: false,
      cmtShow: false, cmtMoment: null, cmts: [], cmtLoading: false, cmtText: '', replyTo: null,
      // 送礼
      giftMoment: null,  // 正在送礼的动态
      giftFx: null,      // 送礼成功动效 {name, icon}
      likePopId: null    // 点赞弹跳动画中的 moment_id
    }
  },
  computed: {
    chatPrice() { return useUserStore().chatPrice },
    cardOn() { return useFeaturesStore().userCard },
    rankOn() { return useFeaturesStore().charmRank },
    squareOn() { return useFeaturesStore().square },
    myId() { return useUserStore().userId }
  },
  async onShow() {
    this.paidUsers = uni.getStorageSync('paid_expand_users') || {}
    await this.loadCover()
    if (this.showCover) return
    const fs = useFeaturesStore()
    if (!fs.loaded) await fs.fetch()
    if (this.squareOn) this.reloadFeed()
    else this.reload()
  },
  onPullDownRefresh() {
    const p = this.squareOn ? this.reloadFeed() : this.reload()
    p.finally(() => uni.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.squareOn && !this.feedLoading && !this.feedEnd) this.loadFeed()
  },
  methods: {
    /* ── 动态广场 ─────────────────────────── */
    async reloadFeed() {
      this.feedPage = 1
      this.feedEnd = false
      this.feed = []
      await this.loadFeed()
    },
    async loadFeed() {
      this.feedLoading = true
      try {
        const list = await momentApi.feed({ page: this.feedPage, size: 20 }) || []
        if (this.feedPage === 1) this.feed = list
        else this.feed = this.feed.concat(list)
        if (list.length < 20) this.feedEnd = true
        this.feedPage++
      } catch (e) {} finally { this.feedLoading = false }
    },
    imgs(m) {
      try { const a = JSON.parse(m.images || '[]'); return Array.isArray(a) ? a : [] } catch (e) { return [] }
    },
    previewImgs(m, i) {
      const urls = this.imgs(m)
      uni.previewImage({ urls, current: urls[i] })
    },
    isMine(m) { return String(m.user_id) === String(this.myId) },
    shortTime(t) { return t ? String(t).slice(5, 16).replace('T', ' ') : '刚刚' },
    giftInfo(cm) {
      try { return JSON.parse(cm.content || '{}') || {} } catch (e) { return {} }
    },
    async toggleLike(m) {
      // 先做弹跳动画(即时反馈),接口异步跟上
      this.likePopId = m.moment_id
      setTimeout(() => { if (this.likePopId === m.moment_id) this.likePopId = null }, 450)
      try {
        const r = await momentApi.like(m.moment_id)
        m.liked = !!(r && r.liked)
        m.like_count = Math.max(0, (m.like_count || 0) + (m.liked ? 1 : -1))
      } catch (e) {}
    },
    /* 送礼 */
    openGift(m) {
      if (!m) return
      if (this.isMine(m)) { uni.showToast({ title: '不能送给自己哦', icon: 'none' }); return }
      this.giftMoment = m
      this.$refs.gpicker.show()
    },
    async onGiftSend(item) {
      const m = this.giftMoment
      if (!m) { this.$refs.gpicker.done(false); return }
      try {
        const cv = await momentApi.gift(m.moment_id, item.item_id, { showError: false })
        this.$refs.gpicker.done(true)
        m.comment_count = (m.comment_count || 0) + 1
        if (cv) {
          if (this.cmtShow && this.cmtMoment && this.cmtMoment.moment_id === m.moment_id) this.cmts.push(cv)
          m.previews = (m.previews || []).concat(cv).slice(-2)
        }
        useWalletStore().fetchBalance()
        // 成功动效:礼物中心放大上飘
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
    },
    delMoment(m) {
      uni.showModal({
        title: '删除动态', content: '删除后不可恢复,确定?',
        success: async (r) => {
          if (!r.confirm) return
          try { await momentApi.remove(m.moment_id); this.feed = this.feed.filter((x) => x.moment_id !== m.moment_id) } catch (e) {}
        }
      })
    },
    /* 发布 */
    openPost() { this.postShow = true },
    async pickPostImgs() {
      try {
        const urls = await chooseAndUploadImages(9 - this.postImgs.length)
        this.postImgs = this.postImgs.concat(urls)
      } catch (e) {}
    },
    removePostImg(i) { this.postImgs.splice(i, 1) },
    async submitPost() {
      if (this.posting) return
      if (!this.postText.trim() && !this.postImgs.length) { uni.showToast({ title: '写点什么或配张图吧', icon: 'none' }); return }
      this.posting = true
      try {
        await momentApi.create(this.postText.trim(), 'public', this.postImgs)
        this.postShow = false
        this.postText = ''
        this.postImgs = []
        uni.showToast({ title: '已发布 🎉', icon: 'none' })
        this.reloadFeed()
      } catch (e) {
        uni.showToast({ title: (e && e.message) || '发布失败', icon: 'none' })
      } finally { this.posting = false }
    },
    /* 评论 */
    async openComments(m) {
      this.cmtMoment = m
      this.cmtShow = true
      this.cmtText = ''
      this.replyTo = null
      this.cmtLoading = true
      try { this.cmts = await momentApi.comments(m.moment_id, { page: 1, size: 50 }) || [] }
      catch (e) { this.cmts = [] } finally { this.cmtLoading = false }
    },
    closeComments() { this.cmtShow = false; this.cmtMoment = null },
    setReplyTo(cm) {
      if (String(cm.user_id) === String(this.myId)) return
      this.replyTo = cm
    },
    canDelCmt(cm) {
      return String(cm.user_id) === String(this.myId) ||
        (this.cmtMoment && String(this.cmtMoment.user_id) === String(this.myId))
    },
    async sendComment() {
      const text = this.cmtText.trim()
      if (!text || !this.cmtMoment) return
      try {
        const cv = await momentApi.comment(this.cmtMoment.moment_id, text, this.replyTo ? this.replyTo.user_id : 0)
        if (cv) this.cmts.push(cv)
        this.cmtText = ''
        this.replyTo = null
        this.cmtMoment.comment_count = (this.cmtMoment.comment_count || 0) + 1
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
            if (this.cmtMoment) this.cmtMoment.comment_count = Math.max(0, (this.cmtMoment.comment_count || 0) - 1)
          } catch (e) {}
        }
      })
    },
    openCardById(userId) {
      if (!this.cardOn) return
      this.$refs.ucard.show(userId)
    },
    /* ── 旧扩列列表 ─────────────────────────── */
    async reload() {
      this.loading = true
      try {
        this.users = await matchApi.expandWall({ type: this.type, page: 1, size: 30 }) || []
      } catch (e) {} finally { this.loading = false }
    },
    async setType(t) {
      if (t === 'nearby' && !this.located) {
        try { await getLocationOnce(); this.located = true } catch (e) { return }
      }
      this.type = t
      this.reload()
    },
    genderText(g) { return g === 1 ? '♂' : g === 2 ? '♀' : '·' },
    regDays(t) {
      if (!t) return 0
      const d = new Date(String(t).replace(' ', 'T'))
      if (isNaN(d.getTime())) return 0
      return Math.max(0, Math.floor((Date.now() - d.getTime()) / 86400000))
    },
    moreActions(u) {
      uni.showActionSheet({
        itemList: ['举报', '拉黑'],
        success: async (res) => {
          try {
            if (res.tapIndex === 0) {
              await moderationApi.report(u.user_id, 'user', '扩列举报')
              uni.showToast({ title: '已举报', icon: 'none' })
            } else {
              await moderationApi.block(u.user_id)
              this.users = this.users.filter((x) => x.user_id !== u.user_id)
              uni.showToast({ title: '已拉黑', icon: 'none' })
            }
          } catch (e) {}
        }
      })
    },
    async startChat(u) {
      try {
        const chat = await chatApi.start(u.user_id, 0)
        this.paidUsers = { ...this.paidUsers, [u.user_id]: true }
        uni.setStorageSync('paid_expand_users', this.paidUsers)
        uni.navigateTo({ url: `/pages/chat/chat?id=${chat.chat_id}&name=${encodeURIComponent(u.nickname)}&avatar=${encodeURIComponent(u.avatar || '')}` })
      } catch (e) {
        if (e && e.code === 5001) {
          uni.showModal({
            title: '金币不足', content: '余额不足,前往充值?',
            success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
          })
        }
      }
    },
    openCard(u) {
      if (!this.cardOn) return // 资料卡未开启保持原状(卡片主体无点击行为)
      this.$refs.ucard.show(u.user_id)
    },
    onCardChat(userId) {
      const u = this.users.find((x) => String(x.user_id) === String(userId))
      if (u) this.startChat(u)
      else { // 广场形态下 users 为空,直接按 id 开聊
        chatApi.start(userId, 0)
          .then((chat) => uni.navigateTo({ url: '/pages/chat/chat?id=' + chat.chat_id }))
          .catch((e) => {
            if (e && e.code === 5001) {
              uni.showModal({
                title: '金币不足', content: '余额不足,前往充值?',
                success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/recharge/recharge' }) }
              })
            }
          })
      }
    },
    async onCardLike(userId) {
      try { await relationApi.like(userId); uni.showToast({ title: '已喜欢 ❤️', icon: 'none' }) } catch (e) {}
    },
    goRank() { uni.navigateTo({ url: '/pages/rank/rank' }) }
  }
}
</script>

<style lang="scss">
.page { padding: 8rpx 28rpx; }
.page.square { padding-bottom: 160rpx; min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #F6FAFC 20%); }
.rank-banner {
  margin-top: 16rpx; padding: 18rpx 24rpx; border-radius: $r-lg;
  background: linear-gradient(135deg, #FFF6E0, #FFE9C7); border: 1rpx solid #FFD79A;
  font-size: 24rpx; font-weight: 700; color: #7A4B12; display: flex; align-items: center;
  .rb-go { margin-left: auto; font-size: 28rpx; color: #B5621C; }
}

/* ── 动态广场 ── */
.mcard {
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 24rpx; margin-top: 18rpx; box-shadow: $shadow-card;
}
.m-hd { display: flex; align-items: center; gap: 16rpx; }
.m-id { flex: 1; min-width: 0; }
.m-name { font-size: 34rpx; font-weight: 700; color: $ink-900; display: flex; align-items: center; gap: 8rpx;
  .m-v { width: 32rpx; height: 32rpx; } }
.m-time { font-size: 24rpx; color: $ink-faint; margin-top: 4rpx; }
.m-del { flex: none; font-size: 24rpx; color: $ink-faint; padding: 6rpx 8rpx; }
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
/* 操作按钮按压反馈(hover-class) */
.act-press { transform: scale(0.88); opacity: 0.75; background: rgba(12,42,51,0.06); }
@keyframes like-pop {
  0% { transform: scale(1); }
  35% { transform: scale(1.45); }
  60% { transform: scale(0.9); }
  100% { transform: scale(1); }
}
.m-cmts { background: #F4F9FB; border-radius: 12rpx; padding: 14rpx 18rpx; margin-top: 14rpx; }
.m-cmt { font-size: 30rpx; line-height: 1.6; word-break: break-word;
  .c-nick { color: $sea-deep; font-weight: 700; }
  .c-reply { color: $ink-faint; margin-left: 6rpx; }
  .c-body { color: $ink-900; }
  .c-gift { color: #d9480f; font-weight: 700; margin-left: 6rpx; }
  &.is-gift { background: #FFF8F0; border-radius: 8rpx; padding: 4rpx 10rpx; margin: 4rpx 0; } }
.m-more { font-size: 26rpx; color: $sea-deep; margin-top: 6rpx; font-weight: 600; }

/* 送礼成功动效:中心放大上飘淡出 */
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
.feed-tip { text-align: center; font-size: 22rpx; color: $ink-faint; padding: 24rpx 0; }

.post-fab {
  position: fixed; right: 36rpx; bottom: 220rpx; z-index: 20;
  width: 100rpx; height: 100rpx; line-height: 96rpx; text-align: center;
  border-radius: 50%; background: linear-gradient(135deg, #FF8A7A, #FF6B5B);
  color: #fff; font-size: 56rpx; font-weight: 700;
  box-shadow: 0 12rpx 30rpx rgba(255,107,91,0.45);
}

/* 发布弹层(z-index 必须高于自定义 tabBar 的 999,否则底部按钮被挡无法点击) */
.po-mask { position: fixed; inset: 0; background: rgba(8,30,40,0.5); z-index: 1001; display: flex; align-items: flex-end; }
.po-panel {
  width: 100%; max-height: 80vh; overflow-y: auto; box-sizing: border-box;
  background: #fff; border-radius: 28rpx 28rpx 0 0;
  padding: 30rpx 30rpx calc(30rpx + env(safe-area-inset-bottom));
}
.po-hd { font-size: 30rpx; font-weight: 800; color: $ink-900; display: flex;
  .po-x { margin-left: auto; font-size: 28rpx; color: $ink-400; padding: 0 8rpx; } }
.po-ta { width: 100%; height: 200rpx; margin-top: 20rpx; font-size: 28rpx; line-height: 1.6; }
.po-grid { display: flex; flex-wrap: wrap; gap: 12rpx; margin-top: 12rpx; }
.po-img { width: 156rpx; height: 156rpx; border-radius: 12rpx; }
.po-add {
  width: 156rpx; height: 156rpx; border-radius: 12rpx; border: 2rpx dashed $line;
  display: flex; align-items: center; justify-content: center; font-size: 56rpx; color: $ink-faint;
}
.po-tip { font-size: 20rpx; color: $ink-faint; margin-top: 10rpx; }
.po-send {
  margin-top: 24rpx; height: 84rpx; line-height: 84rpx; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; font-size: 28rpx; font-weight: 800;
}
.po-send::after { border: none; }

/* 评论弹层(同发布弹层,压过 tabBar;bg 层负责点击关闭,panel 为其兄弟不受冒泡影响) */
.cm-mask { position: fixed; inset: 0; z-index: 1001; display: flex; align-items: flex-end; }
.cm-bg { position: absolute; inset: 0; background: rgba(8,30,40,0.5); }
.cm-panel { position: relative; width: 100%; height: 68vh; background: #fff; border-radius: 28rpx 28rpx 0 0; display: flex; flex-direction: column; }
.cm-hd { padding: 26rpx 30rpx 16rpx; font-size: 28rpx; font-weight: 800; color: $ink-900; display: flex; border-bottom: 1rpx solid $line;
  .cm-x { margin-left: auto; font-size: 28rpx; color: $ink-400; padding: 0 8rpx; } }
.cm-list { flex: 1; padding: 10rpx 30rpx; box-sizing: border-box; }
.cm-item { display: flex; gap: 16rpx; padding: 18rpx 0; border-bottom: 1rpx solid rgba(12,42,51,0.05); }
.cm-body { flex: 1; min-width: 0; }
.cm-meta { font-size: 22rpx; color: $sea-deep; font-weight: 700;
  .cm-rp { color: $ink-faint; font-weight: 400; margin-left: 8rpx; }
  .cm-time { color: $ink-faint; font-weight: 400; margin-left: 12rpx; font-size: 20rpx; } }
.cm-text { font-size: 26rpx; color: $ink-900; line-height: 1.55; margin-top: 6rpx; word-break: break-word; }
.cm-gift { font-size: 26rpx; color: #d9480f; font-weight: 700; margin-top: 6rpx; display: flex; align-items: center; gap: 8rpx;
  .cm-gift-ic { width: 44rpx; height: 44rpx; } }
.cm-del { flex: none; align-self: center; font-size: 22rpx; color: #e5484d; padding: 8rpx; }
.cm-gift-btn { flex: none; font-size: 40rpx; padding: 0 4rpx; border-radius: 12rpx; transition: transform 0.15s ease; }
.cm-tip { text-align: center; font-size: 24rpx; color: $ink-faint; padding: 40rpx 0; }
.cm-input-row {
  display: flex; align-items: center; gap: 14rpx;
  padding: 16rpx 24rpx calc(16rpx + env(safe-area-inset-bottom)); border-top: 1rpx solid $line;
}
.cm-input { flex: 1; height: 68rpx; background: #F4F7FA; border-radius: 999rpx; padding: 0 24rpx; font-size: 26rpx; }
.cm-cancel { font-size: 24rpx; color: $ink-400; }
.cm-send {
  flex: none; height: 68rpx; line-height: 68rpx; padding: 0 30rpx; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; font-size: 26rpx; font-weight: 700;
}

/* ── 旧扩列列表 ── */
.segtabs {
  display: flex; gap: 36rpx; padding: 24rpx 4rpx; font-size: 30rpx; color: $ink-faint;
  .on { color: $ink; font-weight: 700; }
}
.list { height: calc(100vh - 260rpx); }
.ucard {
  display: flex; align-items: center; gap: 24rpx;
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 16rpx 20rpx; margin-bottom: 14rpx; box-shadow: $shadow-card;
}
.ava { flex: none; }
.ava-wrap { position: relative; flex: none; }
.vbadge { position: absolute; bottom: -4rpx; right: -4rpx; width: 32rpx; height: 32rpx; z-index: 1; }
.info { flex: 1; min-width: 0; }
.name { font-weight: 700; font-size: 30rpx; }
.pills { display: flex; flex-wrap: wrap; gap: 8rpx; margin: 8rpx 0; }
.pill {
  font-size: 20rpx; background: #EEF7F9; color: $sea-deep;
  padding: 4rpx 14rpx; border-radius: 8rpx; font-weight: 600;
  &.g.male { background: rgba(79,201,240,0.18); color: $sea-deep; }
  &.g.female { background: rgba(255,107,91,0.14); color: $coral-deep; }
}
.rel { font-size: 22rpx; color: $ink-soft; .n { font-weight: 700; color: $ink; } }
.ract { display: flex; flex-direction: column; align-items: center; gap: 8rpx; flex: none; }
.cost { font-size: 20rpx; color: $ink-faint; }
.chatbtn {
  display: flex; align-items: center; gap: 6rpx;
  background: linear-gradient(135deg, $coral, $coral-deep); color: #fff;
  font-size: 24rpx; font-weight: 700; padding: 14rpx 20rpx; border-radius: $r-md;
  box-shadow: 0 4rpx 12rpx rgba(255,107,91,0.4);
  &.free { background: linear-gradient(135deg, #5BD0E0, #3BA9CC); box-shadow: 0 4rpx 12rpx rgba(91,208,224,0.4); }
  .hearts { width: 28rpx; height: 28rpx; }
}
</style>
