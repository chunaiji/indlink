<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="mrow" @tap="goExpand($event)">
      <view class="mic wall">💗</view>
      <view class="mtxt">
        <view class="t">扩列墙</view>
        <view class="d">发现有趣的灵魂,扩列交友</view>
      </view>
      <text class="arrow">›</text>
    </view>

    <view class="mrow" @tap="openNotify($event)">
      <view class="mic notify">💌</view>
      <view class="mtxt">
        <view class="t">互动通知</view>
        <view class="d">谁回信了你的漂流瓶,都在这里</view>
      </view>
      <view class="right">
        <text v-if="unread > 0" class="badge">{{ unread > 99 ? '99+' : unread }}</text>
        <text v-else class="arrow">›</text>
      </view>
    </view>

    <view class="mrow" @tap="openSystem($event)">
      <view class="mic sys">🔔</view>
      <view class="mtxt">
        <view class="t">系统消息 <text class="official">官方</text></view>
        <view class="d">注册奖励已到账,快去捞瓶子吧</view>
      </view>
      <text class="arrow">›</text>
    </view>

    <view class="divider">最近聊天</view>

    <view v-for="c in visibleChats" :key="c.chat_id" class="swipe">
      <view class="swipe-del" @tap="deleteChat(c)">删除</view>
      <view
        class="mrow swipe-content"
        :style="{ transform: 'translateX(' + (swipeId === String(c.chat_id) ? -160 : 0) + 'rpx)' }"
        @touchstart="onTouchStart($event)"
        @touchend="onTouchEnd($event, c)"
        @tap="onRowTap(c)"
      >
        <view class="ava-wrap">
          <image v-if="c.partner_verified" class="vbadge" src="/static/icons/a-qiye_yirenzhengzhankai.png" mode="aspectFit" />
          <user-avatar :name="c.partner_nickname || '匿名好友'" :src="c.partner_avatar" :size="84" shape="circle" />
        </view>
        <view class="mtxt">
          <view class="t">{{ c.partner_nickname || '匿名好友' }}</view>
          <view class="d">{{ fmtMsg(c.last_message) }}</view>
        </view>
        <view class="chat-meta">
          <text class="time">{{ shortTime(c.updated_at) }}</text>
          <view v-if="c.unread_count > 0" class="unread-dot">{{ c.unread_count > 99 ? '99+' : c.unread_count }}</view>
        </view>
      </view>
    </view>
    <empty-state v-if="!visibleChats.length" icon="🐚" title="还没有聊天" desc="去同城开聊,或回信你捞到的瓶子" />
    <ad-slot slot-key="banner_message" type="banner" />
    </block>
  </view>
  <ad-slot v-if="!showCover" slot-key="banner_message" type="banner" />
  <tab-bar :current="3" :badge="tabBadge" />
</template>

<script>
import { chatApi, notifyApi } from '../../api/index'
import { onWSMessage, onWSReconnect } from '../../utils/ws'
import pagesCover from '../../mixins/pagesCover'

const HIDDEN_KEY = 'hiddenChats'

export default {
  mixins: [pagesCover],
  data() {
    return { chats: [], unread: 0, chatUnread: 0, off: null, offReconnect: null, _pollTimer: null, hidden: new Set(), swipeId: '', _sx: 0, _swiped: false }
  },
  computed: {
    tabBadge() {
      const total = this.unread + this.chatUnread
      return total > 0 ? { message: total } : {}
    },
    visibleChats() {
      return this.chats.filter((c) => !this.hidden.has(String(c.chat_id)))
    }
  },
  onPullDownRefresh() {
    Promise.all([this.reload(), this.loadUnread()]).finally(() => uni.stopPullDownRefresh())
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this._loadHidden()
    this.reload()
    this.loadUnread()
    // WS：实时推送
    if (!this.off) {
      this.off = onWSMessage((msg) => {
        if (!msg) return
        if (msg.event === 'message') {
          if (msg.chat_id) { this.hidden.delete(String(msg.chat_id)); this._saveHidden() }
          this.reload()
          this.loadUnread()
          uni.showToast({ title: '收到新消息', icon: 'none', duration: 2000 })
        }
        if (msg.type === 'notify') { this.unread++; this.loadUnread() }
      })
    }
    // WS 重连后补拉
    if (!this.offReconnect) {
      this.offReconnect = onWSReconnect(() => { this.reload(); this.loadUnread() })
    }
    // HTTP 轮询兜底：每 10s 拉一次，WS 断线期间保证消息不漏
    if (!this._pollTimer) {
      this._pollTimer = setInterval(() => { this.reload(); this.loadUnread() }, 10000)
    }
  },
  onHide() { this._stopPoll() },
  onUnload() {
    this._stopPoll()
    if (this.off) { this.off(); this.off = null }
    if (this.offReconnect) { this.offReconnect(); this.offReconnect = null }
  },
  methods: {
    _loadHidden() {
      try {
        const raw = uni.getStorageSync(HIDDEN_KEY)
        this.hidden = new Set(raw ? JSON.parse(raw) : [])
      } catch (e) { this.hidden = new Set() }
    },
    _saveHidden() {
      try { uni.setStorageSync(HIDDEN_KEY, JSON.stringify([...this.hidden])) } catch (e) {}
    },
    onTouchStart(e) {
      this._sx = e.changedTouches[0].clientX
      this._swiped = false
    },
    onTouchEnd(e, c) {
      const dx = e.changedTouches[0].clientX - this._sx
      const id = String(c.chat_id)
      if (dx <= -40) { this.swipeId = id; this._swiped = true }       // 左滑:带出删除
      else if (dx >= 40) { this.swipeId = ''; this._swiped = true }    // 右滑:收起
    },
    onRowTap(c) {
      if (this._swiped) { this._swiped = false; return }   // 刚滑动过,不当点击
      if (this.swipeId) { this.swipeId = ''; return }       // 有打开的删除按钮,先收起
      this.openChat(c)
    },
    deleteChat(c) {
      this.hidden.add(String(c.chat_id))
      this._saveHidden()
      this.hidden = new Set(this.hidden) // 触发 visibleChats 重算
      this.swipeId = ''
      uni.showToast({ title: '已删除', icon: 'none' })
    },
    _stopPoll() {
      if (this._pollTimer) { clearInterval(this._pollTimer); this._pollTimer = null }
    },
    async reload() {
      try {
        this.chats = await chatApi.list({ page: 1, size: 30 }) || []
        // 有未读消息的聊天自动从隐藏集合里移出（WS 推送时页面不在前台无法即时移出）
        let changed = false
        this.chats.forEach(c => {
          if (c.unread_count > 0 && this.hidden.has(String(c.chat_id))) {
            this.hidden.delete(String(c.chat_id))
            changed = true
          }
        })
        if (changed) this._saveHidden()
      } catch (e) {}
    },
    async loadUnread() {
      try {
        const [n, c] = await Promise.all([
          notifyApi.unread().catch(() => ({ count: 0 })),
          chatApi.unread().catch(() => ({ count: 0 }))
        ])
        this.unread = (n && n.count) || 0      // 互动通知未读(用于互动通知行红点)
        this.chatUnread = (c && c.count) || 0  // 未读聊天消息
      } catch (e) {}
    },
    openNotify() {
      this.unread = 0
      uni.navigateTo({ url: '/pages/notifications/notifications' })
    },
    openChat(c) {
      const name = encodeURIComponent(c.partner_nickname || '匿名好友')
      const avatar = encodeURIComponent(c.partner_avatar || '')
      uni.navigateTo({ url: `/pages/chat/chat?id=${c.chat_id}&name=${name}&avatar=${avatar}` })
    },
    goExpand() { uni.switchTab({ url: '/pages/expand/expand' }) },
    openSystem() {
      uni.showModal({
        title: '系统消息',
        content: '🎉 注册奖励 50 金币已到账,快去海洋捞个瓶子吧~',
        showCancel: false,
        confirmText: '去捞瓶'
      })
    },
    fmtMsg(msg) {
      if (!msg) return '打个招呼吧~'
      if (/^https?:\/\//.test(msg) || msg.startsWith('wxfile://')) return '[图片]'
      return msg
    },
    shortTime(t) {
      if (!t) return ''
      return String(t).slice(11, 16)
    },
  }

}
</script>

<style lang="scss">
.page { padding: 20rpx 28rpx 200rpx; }
.mrow {
  display: flex; align-items: center; gap: 24rpx;
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 26rpx 24rpx; margin-bottom: 20rpx;
}
.ava-wrap { position: relative; flex: none; }
.vbadge {
  position: absolute; top: -36rpx; left: 50%;
  width: 84rpx; height: 84rpx; z-index: 3; pointer-events: none;
  transform: translateX(-50%);
  animation: vbeat 1.3s ease-in-out infinite;
}
@keyframes vbeat {
  0%   { transform: translateX(-50%) scale(1); }
  12%  { transform: translateX(-50%) scale(1.18); }
  24%  { transform: translateX(-50%) scale(1); }
  36%  { transform: translateX(-50%) scale(1.12); }
  50%  { transform: translateX(-50%) scale(1); }
  100% { transform: translateX(-50%) scale(1); }
}
/* 左滑删除:卡片外观移到 wrapper,内层行扁平且不透明,滑动时整体一块露出底层删除,无接缝 */
.swipe {
  position: relative; margin-bottom: 20rpx; overflow: hidden; border-radius: $r-lg;
  border: 1rpx solid $line; box-shadow: $shadow-card; background: $card;
}
.swipe .mrow {
  margin-bottom: 0; border: none; border-radius: 0; box-shadow: none; background: $card;
}
.swipe-content { position: relative; z-index: 2; transition: transform 0.25s cubic-bezier(0.25,0.1,0.25,1); will-change: transform; }
.swipe-del {
  position: absolute; top: 0; right: 0; bottom: 0; z-index: 1; width: 160rpx;
  display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #FF7A6B, #F2543F); color: #fff; font-weight: 700; font-size: 28rpx;
}
.mic {
  width: 84rpx; height: 84rpx; border-radius: $r-avatar; flex: none;
  display: flex; align-items: center; justify-content: center; font-size: 40rpx; color: #fff;
  &.wall { background: linear-gradient(135deg, #7C6FF0, #5B4FD6); }
  &.notify { background: linear-gradient(135deg, #FF8A7A, #FF6B5B); }
  &.sys { background: linear-gradient(135deg, #FF8A6B, #F2543F); }
  &.chat { background: linear-gradient(135deg, #5BD0E0, #3BA9CC); }
}
.mtxt { flex: 1; min-width: 0; }
.t { font-weight: 700; font-size: 28rpx; }
.official {
  font-size: 18rpx; background: rgba(79,201,240,0.16); color: $sea-deep;
  padding: 2rpx 10rpx; border-radius: 6rpx; font-weight: 700;
}
.d { font-size: 24rpx; color: $ink-soft; margin-top: 6rpx; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.right { text-align: right; }
.chat-meta { display: flex; flex-direction: column; align-items: flex-end; gap: 8rpx; flex-shrink: 0; }
.time { font-size: 22rpx; color: $ink-faint; }
.unread-dot {
  min-width: 32rpx; height: 32rpx; line-height: 32rpx; text-align: center;
  background: #FF6B5B; color: #fff; border-radius: 999rpx;
  font-size: 18rpx; font-weight: 700; padding: 0 8rpx;
}
.arrow { font-size: 40rpx; color: $ink-faint; }
.badge {
  display: block; margin-top: 8rpx; background: $coral; color: #fff;
  font-size: 20rpx; font-weight: 700; border-radius: 999rpx; padding: 2rpx 12rpx;
}
.divider { font-size: 24rpx; color: $ink-soft; padding: 16rpx 8rpx; font-weight: 700; }
.empty { text-align: center; color: $ink-faint; padding: 80rpx 0; font-size: 26rpx; }
</style>
