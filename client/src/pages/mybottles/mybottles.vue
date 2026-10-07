<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <!-- 筛选 -->
    <view class="tabs">
      <text :class="{ on: filter === 'all' }" @tap="setFilter('all')">全部</text>
      <text :class="{ on: filter === 'replied' }" @tap="setFilter('replied')">
        有回应<text v-if="repliedCount" class="tabnum">{{ repliedCount }}</text>
      </text>
    </view>

    <sk-list v-if="loading" :rows="4" />
    <block v-else>
      <view v-for="b in shownList" :key="b.bottle_id" class="card">
        <!-- 卡片主体 -->
        <view class="main" @tap="toggle(b)">
          <view class="row1">
            <text class="status" :class="bStatus(b).cls">{{ bStatus(b).text }}</text>
            <text class="time">{{ shortTime(b.created_at) }} · {{ b.city || '远方' }}</text>
          </view>
          <view class="content">{{ b.content || (b.content_type === 'image' ? '[图片漂流瓶]' : '') }}</view>
          <view class="meta">
            <text v-if="firstTag(b)" class="tag">#{{ firstTag(b) }}</text>
            <text class="m-sp"></text>
            <text class="rc" :class="{ hot: b.reply_count > 0 }" @tap.stop="toggle(b)">💬 {{ b.reply_count || 0 }} 回应</text>
            <text class="lk">❤️ {{ b.like_count || 0 }}</text>
            <text class="chev">{{ opened[b.bottle_id] ? '收起 ▴' : '展开 ▾' }}</text>
          </view>
          <view v-if="traceOn" class="trace-line" @tap.stop="openTrace(b)">
            🌊 {{ b.scoop_count ? `漂过 ${b.city_count || 0} 座城 · 被捞 ${b.scoop_count} 次` : '还在海上漂着,等人捞起' }}
            <text class="t-go">轨迹 ›</text>
          </view>
        </view>

        <!-- 展开的回应 -->
        <view v-if="opened[b.bottle_id]" class="replies">
          <view v-if="loadingId === b.bottle_id" class="r-loading">加载回应中…</view>
          <block v-else>
            <view v-for="r in (repliesMap[b.bottle_id] || [])" :key="r.reply_id" class="reply">
              <user-avatar :name="r.nickname || '回信者'" :src="r.avatar" :size="56" shape="circle" />
              <view class="r-cb">
                <view class="r-meta">{{ r.nickname || '匿名' }} · {{ shortTime(r.created_at) }}</view>
                <view class="r-text">{{ r.content }}</view>
              </view>
              <view class="r-chat" @tap.stop="goChat(b, r)">💬 开聊</view>
            </view>
            <view v-if="!(repliesMap[b.bottle_id] || []).length" class="r-empty">还没有人回应这只瓶子</view>
          </block>
        </view>
      </view>

      <empty-state
        v-if="!shownList.length"
        :icon="filter === 'replied' ? '💬' : '🌊'"
        :title="filter === 'replied' ? '还没有人回应你的瓶子' : '你还没扔过瓶子'"
        :desc="filter === 'replied' ? '再扔几只,等待海那头的回信' : '写下心事,扔进大海漂给有缘人'"
        :action="filter === 'replied' ? '' : '去扔一个'"
        @action="goThrow"
      />
    </block>

    <!-- 漂流轨迹弹层(背景层与内容框分离,@tap.stop 不可靠) -->
    <view v-if="traceShow" class="tr-mask">
      <view class="tr-bg" @tap="traceShow = false"></view>
      <view class="tr-panel">
        <view class="tr-hd">
          <text class="tr-title">🌊 漂流轨迹</text>
          <text class="tr-sub" v-if="trace">漂过 {{ trace.city_count || 0 }} 座城 · 被捞 {{ trace.scoop_count || 0 }} 次</text>
          <text class="tr-close" @tap="traceShow = false">✕</text>
        </view>
        <scroll-view scroll-y class="tr-body">
          <view v-if="traceLoading" class="tr-empty">加载中…</view>
          <block v-else-if="trace && trace.events && trace.events.length">
            <view class="tr-item">
              <text class="tr-dot start">●</text>
              <view class="tr-cb"><text class="tr-act">你把瓶子扔进了大海</text></view>
            </view>
            <view v-for="(ev, i) in trace.events" :key="i" class="tr-item">
              <text class="tr-dot" :class="ev.action">●</text>
              <view class="tr-cb">
                <text class="tr-act">{{ evText(ev) }}</text>
                <text class="tr-time">{{ shortTime(ev.created_at) }}</text>
              </view>
            </view>
          </block>
          <view v-else class="tr-empty">还没有漂流记录,等风来 🍃</view>
        </scroll-view>
      </view>
    </view>
    </block>
  </view>
</template>

<script>
import { bottleApi, chatApi } from '../../api/index'
import { useFeaturesStore } from '../../store/features'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return {
      list: [],
      loading: true,
      filter: 'all',
      opened: {},       // bottle_id -> bool(是否展开)
      repliesMap: {},    // bottle_id -> [reply]
      loadingId: 0,
      traceShow: false,  // 轨迹弹层
      trace: null,
      traceLoading: false
    }
  },
  computed: {
    shownList() {
      if (this.filter === 'replied') return this.list.filter((b) => b.reply_count > 0)
      return this.list
    },
    repliedCount() {
      return this.list.filter((b) => b.reply_count > 0).length
    },
    traceOn() { return useFeaturesStore().bottleTrace }
  },
  onLoad(opt) {
    // 通知「瓶子被捞」跳转带 trace_id → 列表加载完自动弹出该瓶轨迹
    this.pendingTraceId = (opt && opt.trace_id) || ''
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    const fs = useFeaturesStore()
    if (!fs.loaded) fs.fetch()
    this.reload()
  },
  methods: {
    async reload() {
      this.loading = true
      try {
        this.list = await bottleApi.mine({ page: 1, size: 50 }) || []
      } catch (e) {} finally { this.loading = false }
      if (this.pendingTraceId) {
        const b = this.list.find((x) => String(x.bottle_id) === String(this.pendingTraceId))
        this.pendingTraceId = ''
        if (b) this.openTrace(b)
      }
    },
    setFilter(f) { this.filter = f },
    async toggle(b) {
      const id = b.bottle_id
      const next = !this.opened[id]
      this.opened = { ...this.opened, [id]: next }
      // 展开且尚未拉过回应 -> 拉取(作者可见已解锁内容)
      if (next && b.reply_count > 0 && !this.repliesMap[id]) {
        this.loadingId = id
        try {
          const rs = await bottleApi.replies(id) || []
          this.repliesMap = { ...this.repliesMap, [id]: rs }
        } catch (e) {
          this.repliesMap = { ...this.repliesMap, [id]: [] }
        } finally {
          this.loadingId = 0
        }
      } else if (next && b.reply_count === 0) {
        this.repliesMap = { ...this.repliesMap, [id]: [] }
      }
    },
    bStatus(b) {
      const expired = b.status === 'expired' || (b.expire_at && new Date(b.expire_at).getTime() < Date.now())
      if (b.status === 'deleted') return { text: '已删除', cls: 'gone' }
      return expired ? { text: '已过期', cls: 'gone' } : { text: '漂流中', cls: 'live' }
    },
    firstTag(b) { return b.tags ? String(b.tags).split(',')[0] : '' },
    shortTime(t) { return t ? String(t).slice(5, 16).replace('T', ' ') : '刚刚' },
    async openTrace(b) {
      this.traceShow = true
      this.traceLoading = true
      this.trace = null
      try { this.trace = await bottleApi.trace(b.bottle_id) } catch (e) {}
      this.traceLoading = false
    },
    evText(ev) {
      const city = ev.city ? `${ev.city}的` : '远方的'
      if (ev.action === 'view') return `被${city}朋友捞起`
      if (ev.action === 'reply') return `${city}朋友留下了回应 💌`
      if (ev.action === 'like') return `${city}朋友点了赞 ❤️`
      return '继续漂流中'
    },
    goThrow() {
      // 统一走海洋页的「写纸条」弹框:置标记 -> 切到海洋 Tab,由其 onShow 打开
      uni.setStorageSync('pendingCompose', 1)
      uni.switchTab({ url: '/pages/ocean/ocean' })
    },
    // 与扩列开聊同一后端接口:未有会话时扣开聊币,已有会话直接进入不重复扣
    async goChat(b, r) {
      try {
        const chat = await chatApi.start(r.user_id, b.bottle_id)
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
    }
  }
}
</script>

<style lang="scss">
.page { padding: 16rpx 28rpx 40rpx; min-height: 100vh; background: linear-gradient(180deg, #EAF7FB, #DDEFF3); }

.tabs {
  display: flex; gap: 36rpx; padding: 12rpx 8rpx 20rpx; font-size: 28rpx; color: $ink-400; font-weight: 600;
  .on { color: $ink-900; font-weight: 800; }
  .tabnum {
    margin-left: 8rpx; font-size: 20rpx; font-weight: 800; color: #fff;
    background: $coral; border-radius: 999rpx; padding: 0 10rpx;
  }
}

.card {
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  box-shadow: $shadow-card; margin-bottom: 22rpx; overflow: hidden;
}
.main { padding: 26rpx 26rpx 22rpx; }
.row1 { display: flex; align-items: center; justify-content: space-between; }
.status {
  font-size: 20rpx; font-weight: 800; padding: 4rpx 14rpx; border-radius: 999rpx;
  &.live { color: #0E7FA6; background: rgba(43,176,214,0.14); }
  &.gone { color: $ink-400; background: $ink-50; }
}
.time { font-size: 22rpx; color: $ink-400; }
.content {
  font-size: 30rpx; line-height: 1.6; color: $ink-900; margin-top: 16rpx;
  display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; overflow: hidden;
}
.meta { display: flex; align-items: center; gap: 18rpx; margin-top: 18rpx; }
.tag { font-size: 22rpx; font-weight: 700; color: $coral-deep; }
.m-sp { flex: 1; }
.rc {
  font-size: 22rpx; color: $ink-400; font-weight: 600;
  &.hot { color: $coral-deep; font-weight: 800; }
}
.lk { font-size: 22rpx; color: $ink-400; }
.chev { font-size: 22rpx; color: $sea-deep; font-weight: 700; }

.trace-line {
  margin-top: 14rpx; padding-top: 14rpx; border-top: 1rpx dashed $line;
  font-size: 22rpx; color: $sea-deep; display: flex; align-items: center;
  .t-go { margin-left: auto; font-weight: 700; }
}

/* 轨迹弹层 */
.tr-mask { position: fixed; inset: 0; z-index: 99; display: flex; align-items: flex-end; }
.tr-bg { position: absolute; inset: 0; background: rgba(8,30,40,0.45); }
.tr-panel { position: relative; width: 100%; max-height: 68vh; background: #fff; border-radius: 28rpx 28rpx 0 0; display: flex; flex-direction: column; }
.tr-hd { display: flex; align-items: baseline; gap: 16rpx; padding: 28rpx 30rpx 18rpx; border-bottom: 1rpx solid $line; }
.tr-title { font-size: 30rpx; font-weight: 800; color: $ink-900; }
.tr-sub { font-size: 22rpx; color: $ink-400; }
.tr-close { margin-left: auto; font-size: 30rpx; color: $ink-400; padding: 0 8rpx; }
.tr-body { flex: 1; padding: 20rpx 30rpx 40rpx; box-sizing: border-box; }
.tr-item { display: flex; gap: 18rpx; padding: 14rpx 0; }
.tr-dot { font-size: 20rpx; color: $sea-deep; flex: none; line-height: 1.7;
  &.start { color: $coral; } &.reply { color: $coral-deep; } &.like { color: #e5484d; } }
.tr-cb { flex: 1; display: flex; align-items: baseline; gap: 14rpx; border-bottom: 1rpx solid rgba(12,42,51,0.05); padding-bottom: 12rpx; }
.tr-act { font-size: 26rpx; color: $ink-900; }
.tr-time { margin-left: auto; font-size: 20rpx; color: $ink-faint; flex: none; }
.tr-empty { text-align: center; padding: 60rpx 0; font-size: 24rpx; color: $ink-400; }

.replies { background: #F4F9FB; border-top: 1rpx dashed $line; padding: 12rpx 26rpx 18rpx; }
.r-loading, .r-empty { font-size: 24rpx; color: $ink-400; padding: 18rpx 4rpx; text-align: center; }
.reply { display: flex; gap: 18rpx; padding: 16rpx 0; border-bottom: 1rpx solid rgba(12,42,51,0.05); align-items: center; }
.r-chat {
  flex: none; font-size: 20rpx; font-weight: 700; color: $coral-deep;
  border: 1rpx solid $coral; border-radius: 999rpx; padding: 8rpx 16rpx;
}
.reply:last-child { border-bottom: none; }
.reply .ua { flex: none; }
.r-cb { flex: 1; min-width: 0; }
.r-meta { font-size: 20rpx; color: $ink-faint; margin-bottom: 6rpx; }
.r-text { font-size: 28rpx; color: $ink-900; line-height: 1.55; word-break: break-word; }
</style>
