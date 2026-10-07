<template>
  <view class="ocean" :class="theme === 'night' ? 'night' : 'day'">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <!-- 星空层(夜间)-->
    <view v-if="theme === 'night'" class="stars">
      <view v-for="(s, i) in stars" :key="i" class="star" :style="starStyle(s)"></view>
    </view>
    <!-- 月亮 / 太阳 -->
    <view class="sun" :class="theme"></view>

    <!-- 状态栏占位 -->
    <view :style="{ height: statusBarHeight + 'px' }"></view>

    <!-- 顶部导航 -->
    <view class="nav">
      <text class="nav-l" @tap="openFilter($event)">≡ 筛选</text>
      <text class="nav-t">{{ uiNavTitle }}</text>
      <view class="nav-r">
        <text class="dots" @tap="onToggleTheme($event)">•••</text>
        <user-avatar class="me" :name="myName" :src="myAvatar" :size="88" shape="circle" />
      </view>
    </view>

    <!-- 情绪钩子 -->
    <view class="hook">今日已有 <text class="hot">{{ todayReplies }}</text> 条 回应</view>

    <!-- 深夜场徽标 -->
    <view v-if="nightOn" class="night-badge">🌙 深夜场进行中 · 此刻的心事只漂给深夜的人</view>

    <!-- 邀请悬浮胶囊 -->
    <button class="invite" open-type="share">
      <image class="gift" src="/static/icons/liwu.png" mode="aspectFit" />
      <view class="it"><view>邀请好友</view><view>领回应</view></view>
    </button>

    <!-- 关联小程序入口(后台可配最多4个,缩起胶囊/展开面板) -->
    <view v-if="linkMP.on" class="link-mp" :class="{ open: mpOpen }" @tap="mpOpen = !mpOpen">
      <template v-if="!mpOpen">
        <text class="lm-ic">🚀</text>
        <text class="lm-t">{{ linkMP.title || '更多好玩' }}</text>
      </template>
      <view v-else class="lm-panel" @tap.stop>
        <view class="lm-hd">
          <text>{{ linkMP.title || '更多好玩' }}</text>
          <text class="lm-x" @tap.stop="mpOpen = false">✕</text>
        </view>
        <view v-for="(m, i) in linkMP.list" :key="i" class="lm-item" @tap.stop="goLinkMP(m)">
          <image v-if="m.icon" class="lm-item-ic" :src="m.icon" mode="aspectFill" />
          <text v-else class="lm-item-emoji">🎮</text>
          <text class="lm-item-t">{{ m.title || '小程序' }}</text>
          <text class="lm-item-go">›</text>
        </view>
      </view>
    </view>

    <!-- 灯塔 + 海岛 -->
    <view class="scene">
      <view class="island"></view>
      <view class="lighthouse">
        <view class="beam"></view>
        <view class="lamp"></view>
        <view class="tower"></view>
        <view class="base"></view>
      </view>
      <view class="hill"></view>
    </view>

    <!-- 海面 + 漂浮瓶子 -->
    <view class="sea">
      <view
        v-for="(b, i) in floating"
        :key="i"
        class="fbottle"
        :style="bottleStyle(i)"
        @tap="openBottle(b)"
      >
        <image class="bimg" :src="i % 2 === 0 ? bottleL : bottleR" mode="widthFix" />
        <text v-if="b.reply_count" class="num">{{ b.reply_count }}</text>
      </view>
    </view>

    <!-- 沙滩三件事 -->
    <view class="beach">
      <view class="act" @tap="openCompose($event)">
        <text class="qbadge">剩 {{ throwLeft }}</text>
        <view class="ic"><image class="icimg" src="/static/icons/piaoliuping.png" mode="aspectFit" /></view><text class="lb">扔瓶子</text>
      </view>
      <view class="act big" @tap="scoop($event)">
        <text class="qbadge">剩 {{ scoopLeft }}</text>
        <view class="ic"><image class="icimg" src="/static/icons/yuwang.png" mode="aspectFit" /></view><text class="lb">捞瓶子</text>
      </view>
      <view class="act" @tap="goMyBottles($event)">
        <view class="ic"><image class="icimg" src="/static/icons/tong.png" mode="aspectFit" /></view><text class="lb">我的瓶子</text>
      </view>
    </view>

    <!-- 打捞动画 -->
    <view v-if="scooping" class="scoop-fx">
      <view class="sf-stage">
        <view class="ripple r1"></view>
        <view class="ripple r2"></view>
        <view class="ripple r3"></view>
        <text class="sf-hook">🎣</text>
        <text class="sf-rise">🍾</text>
      </view>
      <view class="sf-tip">正在打捞海里的瓶子<text class="dot">…</text></view>
    </view>

    <!-- 扔瓶子:写信纸 → 卷起 → 塞瓶 → 投海 -->
    <view v-if="composeVisible" class="compose">
      <view class="c-mask" @tap="closeCompose($event)"></view>

      <!-- 写纸条 -->
      <view v-if="phase === 'write'" class="c-paper">
        <view class="c-clip">📎</view>
        <view class="c-hd">写一张漂流纸条</view>
        <textarea
          v-model="composeText"
          class="c-ta"
          :maxlength="500"
          :focus="composeVisible"
          placeholder="今天想说点什么?这里没人认识你,却有人懂你。"
          placeholder-class="c-ph"
        />
        <view class="c-tags">
          <text v-for="t in tags" :key="t" class="c-tg" :class="{ on: selTags.includes(t) }" @tap="toggleTag(t)">#{{ t }}</text>
        </view>
        <view v-if="nightOn" class="c-opt night">
          <text class="c-anon">🌙 深夜瓶(只在深夜被捞起)</text>
          <switch :checked="nightPick" color="#6741d9" style="transform:scale(0.8)" @change="onNight($event)" />
        </view>
        <view class="c-opt">
          <text class="c-anon">匿名漂流</text>
          <switch :checked="anon" color="#FF6B5B" style="transform:scale(0.8)" @change="onAnon($event)" />
          <text class="c-count">{{ composeText.length }}/500</text>
        </view>
        <button class="c-send" :loading="sending" @tap="seal($event)">📜 封瓶投海</button>
      </view>

      <!-- 封瓶 / 投海 动画 -->
      <view v-else class="c-story">
        <view class="story-paper" :class="phase"></view>
        <view class="story-bottle" :class="phase">🍾</view>
        <view v-if="phase === 'toss'" class="story-splash">💦</view>
        <view class="c-tip">{{ phase === 'toss' ? '用力扔进大海…' : '把纸条卷起来,塞进瓶子…' }}</view>
      </view>
    </view>

    <!-- 捞到的瓶子:信纸弹框 -->
    <view v-if="popupVisible" class="modal">
      <view class="mask" @tap="closePopup($event)"></view>
      <view class="letter-wrap">
        <view class="paper-bg b1"></view>
        <view class="paper-bg b2"></view>
        <view class="letter">
          <view class="l-head">
            <user-avatar class="l-ava" :name="popName" :size="72" shape="circle" />
            <view class="l-id">
              <view class="l-name">{{ popName }}</view>
              <view class="l-meta">
                <text class="l-gp" :class="popup && popup.author_gender === 1 ? 'male' : 'female'">{{ popGender }} {{ (popup && popup.author_age) || '?' }}</text>
                <text class="l-lv">Lv {{ popLevel }}</text>
              </view>
            </view>
            <!-- 收藏 / 转发:标题区图标 -->
            <view class="l-tools">
              <view class="l-tool" :class="{ on: collected }" @tap="toggleCollect($event)">
                <image class="ti-img" src="/static/icons/shoucang.png" mode="aspectFit" />
              </view>
              <button class="l-tool" open-type="share">
                <image class="ti-img" src="/static/icons/fenxiang.png" mode="aspectFit" />
              </button>
            </view>
          </view>
          <scroll-view scroll-y class="l-body">
            <view class="l-paper"><text class="l-text">{{ popup && popup.content }}</text></view>
          </scroll-view>
          <view class="l-foot">
            <text class="l-time">{{ popTime }} · {{ popup && popup.city || '远方' }}</text>
            <text v-if="isNightBottle" class="l-tag night">🌙 深夜瓶</text>
            <text v-else-if="popTag" class="l-tag">#{{ popTag }}</text>
          </view>
        </view>
      </view>
      <view class="m-actions">
        <view class="m-btn ghost" @tap="nextBottle($event)">换一个</view>
        <view class="m-btn cta" @tap="viewDetail($event)">查看详情</view>
      </view>
    </view>

    <!-- 筛选(#7)-->
    <view v-if="filterVisible" class="filter-modal">
      <view class="f-mask" @tap="closeFilter($event)"></view>
      <view class="f-panel">
        <view class="f-hd">筛选漂流瓶</view>
        <view class="f-row">
          <text class="f-lb">漂向</text>
          <view class="f-seg">
            <text :class="{ on: filter.scope === '' }" @tap="setF('scope','')">不限</text>
            <text :class="{ on: filter.scope === 'local' }" @tap="setF('scope','local')">仅同城</text>
            <text :class="{ on: filter.scope === 'national' }" @tap="setF('scope','national')">全国</text>
          </view>
        </view>
        <view class="f-row">
          <text class="f-lb">性别</text>
          <view class="f-seg">
            <text :class="{ on: filter.gender === 0 }" @tap="setF('gender',0)">不限</text>
            <text :class="{ on: filter.gender === 2 }" @tap="setF('gender',2)">女生</text>
            <text :class="{ on: filter.gender === 1 }" @tap="setF('gender',1)">男生</text>
          </view>
        </view>
        <view class="f-acts">
          <view class="f-btn ghost" @tap="resetFilter($event)">重置</view>
          <view class="f-btn cta" @tap="applyFilter($event)">应用筛选</view>
        </view>
      </view>
    </view>
    </block>
  </view>
  <!-- 首页公告弹窗：必须在 .ocean 外，否则 overflow:hidden 阻断触摸事件 -->
  <view v-if="noticeVisible" class="notice-modal">
    <view class="nm-mask" @tap="dismissNotice($event)"></view>
    <view class="nm-panel">
      <view class="nm-hd">{{ noticeTitle }}</view>
      <view class="nm-bd">{{ noticeBody }}</view>
      <view class="nm-btn" @tap="dismissNotice($event)">好的，我知道了</view>
    </view>
  </view>
  <ad-slot v-if="!showCover" slot-key="banner_ocean" type="banner" />
  <tab-bar :current="0" />
</template>

<script>
import { bottleApi, collectionApi, sysApi, pushApi, shareApi } from '../../api/index'
import { getTheme, toggleTheme } from '../../utils/theme'
import { useUserStore } from '../../store/user'
import { useFeaturesStore } from '../../store/features'
import { showInterstitial } from '../../utils/ad'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return {
      statusBarHeight: 20,
      theme: 'night',
      todayReplies: '0',
      floating: [],
      stars: [],
      bottleL: '/static/bottle-l.png',
      bottleR: '/static/bottle-r.png',
      pool: [],          // 捞瓶子缓冲池
      popup: null,       // 当前展示的瓶子
      popupVisible: false,
      scooping: false,   // 打捞动画进行中
      // 扔瓶子(写纸条)
      composeVisible: false,
      phase: 'write',    // write -> roll -> stuff -> toss
      composeText: '',
      tags: ['失眠', '情感', '交友', '吐槽', '学习', '树洞'],
      selTags: [],
      anon: true,
      sending: false,
      nightPick: false,  // 深夜瓶勾选(仅夜场时段显示)
      mpOpen: false,     // 关联小程序面板展开
      // 筛选(#7)
      filterVisible: false,
      filter: { scope: '', gender: 0 },
      collected: false,
      // 每日次数(#5)
      throwLeft: '-',
      scoopLeft: '-',
      // 公告
      noticeVisible: false,
      noticeTitle: '温馨提示',
      noticeBody: '',
      // 分享奖励待告知(领奖在分享面板弹出时,需回到页面后再提示)
      pendingShareReward: 0
    }
  },
  computed: {
    myName() { return (useUserStore().profile || {}).nickname || '我' },
    myAvatar() { return (useUserStore().profile || {}).avatar || '' },
    uiNavTitle() { return useUserStore().uiText.navTitle },
    nightOn() { return useFeaturesStore().inNightWindow },
    linkMP() { return useFeaturesStore().linkMP },
    popName() {
      if (!this.popup) return ''
      const t = useUserStore().uiText
      return this.popup.is_anonymous ? t.anonSender : t.anonFriend
    },
    popLevel() {
      const b = this.popup
      if (!b) return 1
      return Math.max(1, Math.min(99, (b.reply_count || 0) + (b.like_count || 0) + 1))
    },
    popTime() {
      const t = this.popup && this.popup.created_at
      if (!t) return '刚刚'
      return String(t).slice(5, 16).replace('T', ' ')
    },
    isNightBottle() {
      return !!(this.popup && String(this.popup.tags || '').split(',').includes('night'))
    },
    popTag() {
      const t = this.popup && this.popup.tags
      return t ? String(t).split(',')[0] : ''
    },
    popGender() {
      const g = this.popup && this.popup.author_gender
      return g === 1 ? '♂' : g === 2 ? '♀' : '·'
    }
  },
  onLoad(q) {
    const info = uni.getSystemInfoSync()
    this.statusBarHeight = info.statusBarHeight || 20
    this.theme = getTheme()
    this.buildStars()
    // 分享落地:带 bottle 参数则直接打开那只瓶子
    if (q && q.bottle) this.openShared(q.bottle)
  },
  // 转发好友(imageUrl 设置后微信展示大图卡片;建议 5:4)
  onShareAppMessage() {
    this.claimShareReward()
    const u = useUserStore()
    const img = u.shareImage
    if (this.popupVisible && this.popup) {
      const txt = String(this.popup.content || '').slice(0, 18)
      const r = { title: txt || u.shareTitle, path: '/pages/ocean/ocean?bottle=' + this.popup.bottle_id }
      if (img) r.imageUrl = img
      return r
    }
    const r = { title: u.shareTitle, path: '/pages/ocean/ocean' }
    if (img) r.imageUrl = img
    return r
  },
  // 分享到朋友圈
  onShareTimeline() {
    this.claimShareReward()
    const u = useUserStore()
    const img = u.shareImage
    if (this.popupVisible && this.popup) {
      const r = { title: String(this.popup.content || '').slice(0, 18) || u.shareTitle, query: 'bottle=' + this.popup.bottle_id }
      if (img) r.imageUrl = img
      return r
    }
    const r = { title: u.shareTitle }
    if (img) r.imageUrl = img
    return r
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    useFeaturesStore().fetch()
    this.flushShareReward()  // 分享回到页面后告知奖励
    this.loadFloating()
    this.loadHookCount()
    this.loadQuota()
    this.checkNotice()
    this.requestPushSubscription()
    // 新用户首次进入:引导完善资料(只引导一次)
    if (uni.getStorageSync('needProfile')) {
      uni.removeStorageSync('needProfile')
      uni.navigateTo({ url: '/pages/profile-edit/profile-edit?first=1' })
      return
    }
    // 来自「我的瓶子」等入口的「去扔一个」:自动打开写纸条弹框
    if (uni.getStorageSync('pendingCompose')) {
      uni.removeStorageSync('pendingCompose')
      this.$nextTick(() => this.openCompose())
    }
  },
  methods: {
    // 分享触发后领奖(后端按每日上限兜量;触发即领,不依赖发送成功回调)
    // 领奖发生在分享面板弹出瞬间,此时弹提示会被面板盖住,故暂存,回到页面(onShow)后再告知;
    // 部分机型分享后不触发 onShow,故再加 1.5s 延迟兜底。
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
    buildStars() {
      const arr = []
      // 固定布点,避免每次渲染抖动
      const seed = [7, 19, 31, 43, 58, 71, 12, 24, 38, 52, 66, 80, 5, 17, 29, 47, 61, 74, 88, 15, 35, 55]
      for (let i = 0; i < seed.length; i++) {
        arr.push({
          left: (seed[i] * 1.13) % 96 + 2,
          top: (seed[i] * 0.9) % 38 + 2,
          size: (i % 3) + 2,
          op: 0.4 + (i % 5) * 0.12
        })
      }
      this.stars = arr
    },
    starStyle(s) {
      return `left:${s.left}%;top:${s.top}%;width:${s.size}rpx;height:${s.size}rpx;opacity:${s.op};`
    },
    scoopParams() {
      const p = {}
      if (this.filter.scope) p.scope = this.filter.scope
      if (this.filter.gender) p.gender = this.filter.gender
      return p
    },
    async openShared(id) {
      try { const b = await bottleApi.detail(id); if (b) this.showBottle(b) } catch (e) {}
    },
    async loadFloating() {
      try {
        const list = await bottleApi.scoop(this.scoopParams())
        this.pool = list || []
        this.floating = (list || []).slice(0, 5)
      } catch (e) {}
    },
    async loadHookCount() {
      try {
        const r = await sysApi.hookCount()
        if (r && typeof r.count === 'number') this.todayReplies = r.count.toLocaleString()
      } catch (e) {}
    },
    bottleStyle(i) {
      const pos = [
        { left: '14%', top: '50%' },
        { left: '52%', top: '38%' },
        { left: '70%', top: '60%' },
        { left: '30%', top: '72%' },
        { left: '60%', top: '82%' }
      ]
      const p = pos[i % pos.length]
      return `left:${p.left};top:${p.top};animation-delay:${-i * 0.6}s;`
    },
    async loadQuota() {
      try {
        const q = await bottleApi.quota()
        if (q) { this.throwLeft = q.throw_left; this.scoopLeft = q.scoop_left }
      } catch (e) {}
    },
    // 次数用完:引导购买次数包道具(文案后台可配)
    quotaExceeded() {
      const t = useUserStore().uiText
      uni.showModal({
        title: t.quotaTitle,
        content: t.quotaMsg,
        confirmText: '去购买',
        cancelText: '知道了',
        success: (r) => { if (r.confirm) uni.navigateTo({ url: '/pages/items/items' }) }
      })
    },
    async scoop() {
      if (this.scooping || this.popupVisible) return
      this.scooping = true
      const minDelay = new Promise((r) => setTimeout(r, 1600))
      let bottle = null, quotaErr = false
      try {
        bottle = await bottleApi.scoopOne(this.scoopParams()) // 计入每日次数 + 去重已看过
      } catch (e) {
        if (e && e.code === 3003) quotaErr = true
      }
      await minDelay
      this.scooping = false
      this.loadQuota()
      if (quotaErr) { this.quotaExceeded('scoop'); return }
      if (!bottle) { uni.showToast({ title: '海面暂时很安静,稍后再来', icon: 'none' }); return }
      this.showBottle(bottle)
    },
    // 点浮动瓶子与「捞瓶子」按钮同一动作:计次+去重+配额0拦截
    openBottle() {
      this.scoop()
    },
    showBottle(b) {
      this.popup = b
      this.collected = false
      this.popupVisible = true
    },
    closePopup() { this.popupVisible = false; showInterstitial('inter_scoop') },
    // 换一个:把当前瓶收回海里(关弹框),重播打捞动画再捞新的一条
    nextBottle() {
      this.popupVisible = false
      this.popup = null
      this.scoop()
    },
    viewDetail() {
      if (!this.popup) return
      const id = this.popup.bottle_id
      this.popupVisible = false
      uni.navigateTo({ url: '/pages/detail/detail?id=' + id })
    },
    goMyBottles() { uni.navigateTo({ url: '/pages/mybottles/mybottles' }) },
    goLinkMP(m) {
      if (!m || !m.appid) return
      uni.navigateToMiniProgram({
        appId: m.appid,
        path: m.path || undefined,
        fail: () => {} // 用户取消跳转弹窗时静默
      })
    },
    openCompose() {
      if (this.scooping || this.popupVisible) return
      this.phase = 'write'
      this.composeText = ''
      this.selTags = []
      this.anon = true
      this.nightPick = this.nightOn // 夜场时段默认勾选深夜瓶
      this.composeVisible = true
    },
    closeCompose() {
      if (this.phase !== 'write') return // 动画进行中不可关
      this.composeVisible = false
    },
    toggleTag(t) {
      if (this.selTags.includes(t)) this.selTags = this.selTags.filter((x) => x !== t)
      else if (this.selTags.length < 3) this.selTags.push(t)
      else uni.showToast({ title: '最多选 3 个标签', icon: 'none' })
    },
    onAnon(e) { this.anon = e.detail.value },
    onNight(e) { this.nightPick = e.detail.value },
    async seal() {
      if (this.sending) return
      if (!this.composeText.trim()) { uni.showToast({ title: '写点什么再扔吧', icon: 'none' }); return }
      this.sending = true
      try {
        await bottleApi.create({
          content: this.composeText,
          content_type: 'text',
          tags: this.selTags,
          is_anonymous: this.anon,
          scope: 'national',
          visibility: '30d',
          night: this.nightOn && this.nightPick
        })
      } catch (e) {
        this.sending = false
        if (e && e.code === 2003) uni.showToast({ title: '请先完成真人认证', icon: 'none' })
        else if (e && e.code === 3003) { this.composeVisible = false; this.quotaExceeded('throw') }
        return
      }
      this.sending = false
      this.loadQuota()
      this.runSeal()
    },
    // 卷纸 -> 塞瓶 -> 投海 三段动画
    runSeal() {
      this.phase = 'roll'
      setTimeout(() => { this.phase = 'stuff' }, 750)
      setTimeout(() => { this.phase = 'toss' }, 1350)
      setTimeout(() => {
        this.composeVisible = false
        this.phase = 'write'
        this.composeText = ''
        this.selTags = []
        uni.showToast({ title: '已扔进大海 🌊' })
        this.loadFloating()
      }, 2350)
    },
    openFilter() { this.filterVisible = true },
    closeFilter() { this.filterVisible = false },
    setF(key, val) { this.filter[key] = val },
    resetFilter() { this.filter = { scope: '', gender: 0 } },
    applyFilter() {
      this.filterVisible = false
      this.pool = []
      this.loadFloating()
      const on = this.filter.scope || this.filter.gender
      uni.showToast({ title: on ? '已按筛选刷新海面' : '已恢复全部', icon: 'none' })
    },
    async toggleCollect() {
      if (!this.popup) return
      const id = this.popup.bottle_id
      try {
        if (this.collected) {
          await collectionApi.remove(id, 'bottle')
          this.collected = false
          uni.showToast({ title: '已取消收藏', icon: 'none' })
        } else {
          await collectionApi.add(id, 'bottle')
          this.collected = true
          uni.showToast({ title: '已收藏', icon: 'none' })
        }
      } catch (e) {}
    },
    onToggleTheme() { this.theme = toggleTheme() },
    async checkNotice() {
      try {
        const n = await sysApi.notice()
        if (!n || !n.body) return
        const seen = uni.getStorageSync('noticeSeen')
        if (seen === n.body) return
        this.noticeTitle = n.title || '温馨提示'
        this.noticeBody = n.body
        this.noticeVisible = true
      } catch (e) {}
    },
    dismissNotice() {
      uni.setStorageSync('noticeSeen', this.noticeBody)
      this.noticeVisible = false
    },
    async requestPushSubscription() {
      // #ifdef MP-WEIXIN
      const user = useUserStore()
      if (!user.pushSubscribePrompt) return   // 管理台开关未开
      if (uni.getStorageSync('pushAsked')) return  // 已经询问过，不再打扰
      let templates = []
      try { templates = await pushApi.templates() } catch (e) {}
      if (!templates || !templates.length) return
      // 先弹说明框，用户点「开启」再走系统授权（微信要求用户手势触发）
      uni.showModal({
        title: '开启消息提醒',
        content: '开启后，有人给你发消息、到签到时间时，会第一时间通知你。',
        confirmText: '开启提醒',
        cancelText: '暂不',
        success: (m) => {
          uni.setStorageSync('pushAsked', 1)
          if (!m.confirm) return
          wx.requestSubscribeMessage({
            tmplIds: templates.map(t => t.id),
            success: (res) => {
              templates.forEach(t => {
                if (res[t.id] === 'accept') {
                  pushApi.subscribe({ template_id: t.id, scene: t.scene }).catch(() => {})
                }
              })
            }
          })
        }
      })
      // #endif
    }
  }
}
</script>

<style lang="scss">
.ocean {
  min-height: 100vh; position: relative; overflow: hidden;
  &.night {
    background: linear-gradient(180deg, #0A1733 0%, #122A4E 30%, #173963 48%, #14304F 60%, #2B3A3E 74%, #3C342A 100%);
  }
  &.day {
    background: linear-gradient(180deg, #BFE9F4 0%, #9BDCEB 34%, #6FC6DD 52%, #5AB6CE 62%, #E7DFC4 76%, #EFE6CC 100%);
  }
}

/* 星空 */
.stars { position: absolute; top: 0; left: 0; right: 0; height: 46%; z-index: 0; }
.star { position: absolute; background: #fff; border-radius: 50%; box-shadow: 0 0 6rpx rgba(255,255,255,0.8); }

/* 月亮 / 太阳 */
.sun {
  position: absolute; top: 120rpx; right: 160rpx; width: 120rpx; height: 120rpx; border-radius: 50%; z-index: 0;
  &.night { background: #EAF1FF; box-shadow: 0 0 50rpx 14rpx rgba(234,241,255,0.4); }
  &.day { background: #FFE489; box-shadow: 0 0 70rpx 22rpx rgba(255,217,107,0.55); }
}

/* 顶部导航 */
.nav {
  position: relative; z-index: 5;
  display: flex; align-items: center; justify-content: space-between; padding: 100rpx 40rpx;
  .nav-l { font-size: 34rpx; font-weight: 700; }
  .nav-t { font-size: 34rpx; font-weight: 800; letter-spacing: 4rpx; }
  .nav-r { display: flex; align-items: center; gap: 18rpx; }
  .dots { font-size: 32rpx; letter-spacing: 2rpx; }
  .me { border: 3rpx solid rgba(255,255,255,0.8); margin-top: 16rpx; }
}
.night .nav { color: #EAF1FF; }
.day .nav { color: #0C2A33; }

/* 情绪钩子 */
.hook {
  position: relative; z-index: 5; text-align: center; margin-top: -70rpx;
  font-size: 26rpx; font-weight: 600;
  .hot { color: #5BE0F0; font-weight: 800; }
}
.day .hook { color: #0C2A33; .hot { color: #0E7FA6; } }
.night .hook { color: #CFE0F5; }
.night-badge {
  position: relative; z-index: 5; margin: 16rpx auto 0; width: fit-content;
  font-size: 22rpx; font-weight: 700; color: #E5DBFF;
  background: rgba(103,65,217,0.45); border: 1rpx solid rgba(190,167,255,0.5);
  padding: 8rpx 24rpx; border-radius: 999rpx; backdrop-filter: blur(4rpx);
}
.day .night-badge { color: #6741d9; background: rgba(103,65,217,0.10); border-color: rgba(103,65,217,0.25); }
.qbadge {
  position: absolute; top: -18rpx; left: 50%; transform: translateX(-50%); white-space: nowrap;
  font-size: 24rpx; font-weight: 800; color: #fff; background: rgba(255,107,91,0.92);
  padding: 2rpx 18rpx; border-radius: 999rpx; box-shadow: 0 4rpx 10rpx rgba(255,107,91,0.4); z-index: 6;
}

/* 邀请胶囊 */
.invite {
  position: absolute; right: 0; top: 340rpx; z-index: 6;width: 250rpx;
  display: flex; align-items: center; gap: 10rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B);
  color: #fff; padding: 16rpx 22rpx 16rpx 20rpx; border-radius: 999rpx 0 0 999rpx;
  box-shadow: 0 8rpx 24rpx rgba(255,107,91,0.45);
  .gift { width: 62rpx; height: 62rpx; }
  .it { font-size: 24rpx; font-weight: 700; line-height: 1.2; width: 140rpx;}
}

/* 关联小程序悬浮胶囊(邀请胶囊下方);展开为竖列面板 */
.link-mp {
  position: absolute; right: 0; top: 470rpx; z-index: 7;
  display: flex; align-items: center; gap: 8rpx;
  background: linear-gradient(135deg, #5C7CFA, #4263EB);
  color: #fff; padding: 14rpx 22rpx 14rpx 18rpx; border-radius: 999rpx 0 0 999rpx;
  box-shadow: 0 8rpx 24rpx rgba(66,99,235,0.4);
  .lm-ic { font-size: 32rpx; }
  .lm-t { font-size: 24rpx; font-weight: 700; max-width: 160rpx; }
  &.open {
    background: #fff; color: $ink-900; border-radius: 20rpx 0 0 20rpx;
    padding: 0; box-shadow: 0 12rpx 36rpx rgba(20,40,60,0.18);
  }
}
.lm-panel { width: 340rpx; padding: 20rpx 22rpx 12rpx; }
.lm-hd {
  display: flex; align-items: center; font-size: 26rpx; font-weight: 800; color: $ink-900;
  padding-bottom: 12rpx; border-bottom: 1rpx solid $line;
  .lm-x { margin-left: auto; font-size: 26rpx; color: $ink-400; padding: 0 6rpx; }
}
.lm-item {
  display: flex; align-items: center; gap: 14rpx; padding: 16rpx 0;
  border-bottom: 1rpx solid rgba(12,42,51,0.05);
  &:last-child { border-bottom: none; }
  .lm-item-ic { width: 56rpx; height: 56rpx; border-radius: 12rpx; flex: none; }
  .lm-item-emoji { font-size: 40rpx; flex: none; }
  .lm-item-t { flex: 1; min-width: 0; font-size: 26rpx; font-weight: 600; color: $ink-900;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .lm-item-go { font-size: 30rpx; color: $ink-400; }
}

/* 灯塔场景 */
.scene { position: absolute; top: 250rpx; left: 0; right: 0; height: 360rpx; z-index: 1; }
.island {
  position: absolute; left: -40rpx; bottom: 0; width: 360rpx; height: 200rpx;
  background: radial-gradient(120% 100% at 40% 100%, #16324a 0%, #0e2436 70%);
  border-radius: 50% 50% 0 0 / 100% 100% 0 0;
}
.hill {
  position: absolute; right: -60rpx; bottom: 0; width: 320rpx; height: 120rpx;
  background: radial-gradient(120% 100% at 60% 100%, #12283c 0%, #0c1e2e 70%);
  border-radius: 50% 50% 0 0 / 100% 100% 0 0;
}
.lighthouse {
  position: absolute; left: 70rpx; bottom: 120rpx; width: 60rpx; height: 180rpx;
  .tower {
    position: absolute; left: 14rpx; bottom: 0; width: 32rpx; height: 140rpx;
    background: repeating-linear-gradient(180deg, #E4534A 0 24rpx, #F4F0E8 24rpx 48rpx);
    border-radius: 8rpx 8rpx 4rpx 4rpx;
    clip-path: polygon(18% 0, 82% 0, 100% 100%, 0 100%);
  }
  .base { position: absolute; left: 6rpx; bottom: -6rpx; width: 48rpx; height: 18rpx; background: #2a2118; border-radius: 6rpx; }
  .lamp {
    position: absolute; left: 20rpx; bottom: 140rpx; width: 20rpx; height: 22rpx; border-radius: 6rpx 6rpx 0 0;
    background: #FFE07A; box-shadow: 0 0 30rpx 10rpx rgba(255,224,122,0.6); z-index: 2;
  }
  .beam {
    position: absolute; left: -90rpx; bottom: 96rpx; width: 0; height: 0; z-index: 1;
    border-top: 70rpx solid transparent; border-bottom: 70rpx solid transparent;
    border-right: 240rpx solid rgba(255,232,160,0.16);
    transform-origin: right center; transform: rotate(-6deg);
  }
}
.day .island, .day .hill { background: #6FA9B8; }
.day .lighthouse .beam { border-right-color: rgba(255,255,255,0.28); }

/* 海面 + 瓶子 */
.sea { position: relative; z-index: 2; height: 500rpx; margin-top: 130rpx; }
.fbottle {
  position: absolute; width: 132rpx; animation: bob 4.5s ease-in-out infinite;
  .bimg {
    width: 132rpx; height: auto; display: block; border-radius: 14rpx;
    box-shadow: 0 8rpx 18rpx rgba(0,0,0,0.3);
  }
  .num {
    position: absolute; top: -12rpx; right: -8rpx;
    font-size: 18rpx; background: #FF6B5B; color: #fff; border-radius: 999rpx; padding: 2rpx 10rpx; font-weight: 700;
    box-shadow: 0 2rpx 6rpx rgba(0,0,0,0.25);
  }
}
@keyframes bob {
  0%, 100% { transform: translateY(0) rotate(-2deg); }
  50% { transform: translateY(-14rpx) rotate(2deg); }
}

/* 沙滩三件事 */
.beach {
  position: absolute; left: 0; right: 0; bottom: 160rpx; z-index: 4;
  display: flex; justify-content: space-around; align-items: flex-end;
  padding: 0 24rpx;
  padding-bottom: calc(0rpx + constant(safe-area-inset-bottom));
  padding-bottom: calc(0rpx + env(safe-area-inset-bottom));
}
.act {
  position: relative;
  text-align: center;
  .ic {
    width: 135rpx; height: 135rpx; line-height: 135rpx; font-size: 52rpx; margin: 0 auto 10rpx;
    background: rgba(255,255,255,0.14); border: 1rpx solid rgba(255,255,255,0.22);
    border-radius: 36rpx; backdrop-filter: blur(2px);
  }
  .lb { font-size: 24rpx; font-weight: 700; }
  .ic .icimg {
    width: 126rpx; height: 126rpx; vertical-align: middle;
  }
  &.big .ic {
    width: 161rpx; height: 161rpx; line-height: 161rpx; font-size: 60rpx;
    background: linear-gradient(135deg, #29C7E6, #1F9FD1);
    border-color: rgba(255,255,255,0.5); box-shadow: 0 10rpx 26rpx rgba(31,159,209,0.5);
    .icimg { width: 149rpx; height: 149rpx; }
  }
}
.night .act { color: #EAF1FF; }
.day .act { color: #0C2A33; .ic { background: rgba(255,255,255,0.78); border-color: rgba(255,255,255,0.9); } }

/* 打捞动画 */
.scoop-fx {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 1001;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  background: rgba(6,16,34,0.5);
}
.sf-stage { position: relative; width: 300rpx; height: 300rpx; }
.ripple {
  position: absolute; left: 50%; top: 64%; width: 40rpx; height: 40rpx; margin: -20rpx 0 0 -20rpx;
  border: 3rpx solid rgba(120,220,245,0.8); border-radius: 50%;
  animation: ripple 1.6s ease-out infinite;
}
.ripple.r2 { animation-delay: 0.5s; }
.ripple.r3 { animation-delay: 1s; }
@keyframes ripple {
  0% { transform: scale(0.3); opacity: 0.9; }
  100% { transform: scale(4.2); opacity: 0; }
}
.sf-hook {
  position: absolute; left: 50%; top: 0; margin-left: -28rpx; font-size: 56rpx;
  animation: cast 1.6s ease-in-out infinite;
}
@keyframes cast {
  0% { transform: translateY(-40rpx) rotate(-8deg); }
  45% { transform: translateY(150rpx) rotate(6deg); }
  100% { transform: translateY(-40rpx) rotate(-8deg); }
}
.sf-rise {
  position: absolute; left: 50%; top: 60%; margin-left: -26rpx; font-size: 52rpx; opacity: 0;
  animation: rise 1.6s ease-in-out infinite;
}
@keyframes rise {
  0%, 50% { transform: translateY(60rpx) rotate(-12deg); opacity: 0; }
  80% { transform: translateY(-10rpx) rotate(8deg); opacity: 1; }
  100% { transform: translateY(-30rpx) rotate(-6deg); opacity: 0.9; }
}
.sf-tip { margin-top: 20rpx; color: #EAF1FF; font-size: 28rpx; font-weight: 700; }
.sf-tip .dot { animation: blink 1.2s steps(3) infinite; }
@keyframes blink { 0% { opacity: 0.2; } 50% { opacity: 1; } 100% { opacity: 0.2; } }

/* 扔瓶子:写纸条 + 封瓶动画 */
.compose { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 1001; display: flex; align-items: center; justify-content: center; }
.c-mask { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(6,16,34,0.6); }
.c-paper {
  position: relative; z-index: 1; width: 600rpx; background: #FFFDF6; border-radius: 14rpx;
  padding: 42rpx 34rpx 30rpx; box-shadow: 0 18rpx 44rpx rgba(0,0,0,0.32);
  background-image: repeating-linear-gradient(180deg, transparent 0 47rpx, rgba(120,140,160,0.16) 47rpx 48rpx);
}
.c-clip { position: absolute; top: -18rpx; right: 56rpx; font-size: 48rpx; transform: rotate(18deg); }
.c-hd { font-size: 30rpx; font-weight: 800; color: #3A3A3A; margin-bottom: 12rpx; }
.c-ta { width: 100%; min-height: 230rpx; font-size: 30rpx; line-height: 48rpx; color: #3A3A3A; background: transparent; box-sizing: border-box; }
.c-ph { color: #B7B2A2; }
.c-tags { display: flex; flex-wrap: wrap; gap: 14rpx; margin-top: 10rpx; }
.c-tg {
  font-size: 22rpx; font-weight: 600; color: #9A8E6E; background: #F2ECD9; padding: 8rpx 18rpx; border-radius: 999rpx;
  &.on { background: rgba(255,107,91,0.14); color: #E0533F; }
}
.c-opt { display: flex; align-items: center; gap: 12rpx; margin-top: 16rpx; font-size: 24rpx; color: #6A6A5E; }
.c-anon { font-weight: 600; }
.c-count { margin-left: auto; color: #B7B2A2; font-size: 22rpx; }
.c-send {
  margin-top: 22rpx; height: 88rpx; line-height: 88rpx; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF5B4B); color: #fff; font-weight: 800; font-size: 30rpx;
  box-shadow: 0 10rpx 26rpx rgba(255,91,75,0.45);
}
.c-story { position: relative; z-index: 1; width: 500rpx; height: 500rpx; }
.story-paper {
  position: absolute; left: 50%; top: 46%; margin-left: -26rpx; width: 52rpx; height: 120rpx; border-radius: 999rpx;
  background: linear-gradient(180deg, #F3E6C4, #E4D2A6); box-shadow: 0 4rpx 10rpx rgba(0,0,0,0.25);
}
.story-paper.roll { animation: curl 0.75s ease-in; }
.story-paper.stuff { animation: intoBottle 0.6s ease-in forwards; }
.story-paper.toss { opacity: 0; }
@keyframes curl {
  0% { width: 300rpx; height: 200rpx; margin-left: -150rpx; border-radius: 10rpx; background: #FFFDF6; }
  60% { width: 120rpx; height: 150rpx; margin-left: -60rpx; border-radius: 40rpx; background: #F6ECD0; }
  100% { width: 52rpx; height: 120rpx; margin-left: -26rpx; border-radius: 999rpx; background: linear-gradient(180deg, #F3E6C4, #E4D2A6); }
}
@keyframes intoBottle {
  0% { transform: translate(0,0) rotate(0) scale(1); opacity: 1; }
  100% { transform: translate(116rpx, 40rpx) rotate(40deg) scale(0.25); opacity: 0; }
}
.story-bottle { position: absolute; left: 50%; top: 42%; margin-left: -44rpx; font-size: 96rpx; opacity: 0; }
.story-bottle.stuff { opacity: 1; animation: wiggle 0.6s ease; }
.story-bottle.toss { opacity: 1; animation: toss 1s cubic-bezier(0.4,0,0.7,1) forwards; }
@keyframes wiggle { 0%,100% { transform: rotate(-6deg); } 50% { transform: rotate(6deg) scale(1.05); } }
@keyframes toss {
  0% { left: 50%; top: 42%; transform: rotate(0) scale(1.05); opacity: 1; }
  50% { left: 64%; top: 6%; transform: rotate(220deg) scale(0.9); opacity: 1; }
  100% { left: 82%; top: 54%; transform: rotate(400deg) scale(0.45); opacity: 0; }
}
.story-splash { position: absolute; left: 76%; top: 50%; font-size: 52rpx; opacity: 0; animation: cstory-splash 0.7s ease-out forwards; animation-delay: 0.85s; }
@keyframes cstory-splash { 0% { transform: translateY(16rpx) scale(0.4); opacity: 0; } 50% { opacity: 1; } 100% { transform: translateY(-12rpx) scale(1.1); opacity: 0; } }
.c-tip { position: absolute; bottom: 20rpx; left: 0; right: 0; text-align: center; color: #EAF1FF; font-size: 26rpx; font-weight: 700; }

/* 信纸弹框 */
.modal { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 1001; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.mask { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(6,16,34,0.6); }
.letter-wrap { position: relative; z-index: 1; width: 600rpx; }
.paper-bg {
  position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: #FBFAF5; border-radius: 14rpx;
  box-shadow: 0 10rpx 30rpx rgba(0,0,0,0.25);
  &.b1 { transform: rotate(-4deg); }
  &.b2 { transform: rotate(3deg); background: #F4F2EA; }
}
.letter {
  position: relative; z-index: 2; background: #FFFDF8; border-radius: 14rpx; padding: 36rpx 34rpx 30rpx;
  box-shadow: 0 16rpx 40rpx rgba(0,0,0,0.3);
}
.l-head { display: flex; align-items: center; gap: 18rpx; }
.l-ava { box-shadow: 0 0 0 4rpx #fff, 0 4rpx 10rpx rgba(0,0,0,0.15); }
.l-id { flex: 1; min-width: 0; }
.l-name { font-size: 28rpx; font-weight: 800; color: #2B2B2B; }
.l-meta { display: flex; align-items: center; gap: 10rpx; margin-top: 6rpx; }
.l-gp {
  font-size: 18rpx; font-weight: 800; padding: 2rpx 12rpx; border-radius: 999rpx;
  &.female { color: #E0533F; background: rgba(255,107,91,0.16); }
  &.male { color: #1F9FD1; background: rgba(79,201,240,0.18); }
}
.l-lv {
  display: inline-block; font-size: 18rpx; font-weight: 800; color: #fff;
  background: linear-gradient(135deg, #FF8A7A, #FF5B6B); padding: 2rpx 12rpx; border-radius: 999rpx;
}
.l-tools { display: flex; align-items: center; gap: 12rpx; flex: none; }
.l-tool {
  width: 64rpx; height: 64rpx; padding: 0; margin: 0; border-radius: 50%; line-height: 64rpx;
  display: flex; align-items: center; justify-content: center;
  background: #F2EEE4; border: none;
}
.l-tool::after { border: none; }
.l-tool .ti { font-size: 36rpx; color: #8A8270; line-height: 1; }
.l-tool .ti.on { color: #FFB13C; }
.l-tool .ti-img { width: 36rpx; height: 36rpx; }
.l-tool.on { background: #FFF1D6; }
.l-body { margin-top: 22rpx; max-height: 44vh; }
.l-paper {
  min-height: 200rpx; padding: 1rpx 6rpx 16rpx;
  background-image: repeating-linear-gradient(180deg, transparent 0 47rpx, rgba(120,140,160,0.28) 47rpx 48rpx);
}
.l-text { font-size: 30rpx; line-height: 48rpx; color: #3A3A3A; word-break: break-word; white-space: pre-wrap; }
.l-foot { display: flex; align-items: center; justify-content: space-between; margin-top: 18rpx; }
.l-time { font-size: 22rpx; color: #9A9A8E; }
.l-tag { font-size: 22rpx; font-weight: 700; color: #1F9FD1;
  &.night { color: #6741d9; } }
.c-opt.night .c-anon { color: #6741d9; font-weight: 700; }
.m-actions { position: relative; z-index: 1; width: 600rpx; display: flex; gap: 24rpx; margin-top: 48rpx; }
.m-btn {
  height: 92rpx; line-height: 92rpx; text-align: center; border-radius: 999rpx;
  font-size: 30rpx; font-weight: 800;
  &.ghost { flex: 1; background: rgba(255,255,255,0.92); color: #3A3A3A; }
  &.cta { flex: 1.7; background: linear-gradient(135deg, #2FD0E6, #18A7CE); color: #fff; box-shadow: 0 10rpx 26rpx rgba(24,167,206,0.5); }
}
button.invite { margin: 0; line-height: normal; color: #fff; font-weight: 700; }
button.invite::after { border: none; }

/* 公告弹窗 */
.notice-modal { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 1000; display: flex; align-items: center; justify-content: center; }
.nm-mask { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(6,16,34,0.6); }
.nm-panel {
  position: relative; z-index: 1; width: 580rpx; background: #fff; border-radius: 24rpx;
  padding: 48rpx 40rpx 36rpx; box-shadow: 0 20rpx 60rpx rgba(0,0,0,0.3);
}
.nm-hd { font-size: 34rpx; font-weight: 800; color: #1A2C38; text-align: center; margin-bottom: 24rpx; }
.nm-bd { font-size: 28rpx; line-height: 1.8; color: #3A5060; text-align: center; }
.nm-btn {
  margin-top: 40rpx; height: 88rpx; line-height: 88rpx; text-align: center; border-radius: 999rpx;
  background: linear-gradient(135deg, #5BD0E0, #3BA9CC); color: #fff; font-weight: 800; font-size: 30rpx;
}

/* 筛选面板(#7)*/
.filter-modal { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 1001; display: flex; align-items: flex-end; }
.f-mask { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(6,16,34,0.55); }
.f-panel {
  position: relative; z-index: 1; width: 100%; background: #fff;
  border-radius: 28rpx 28rpx 0 0; padding: 36rpx 36rpx;
  padding-bottom: calc(36rpx + constant(safe-area-inset-bottom));
  padding-bottom: calc(36rpx + env(safe-area-inset-bottom));
}
.f-hd { font-size: 32rpx; font-weight: 800; color: #2B2B2B; margin-bottom: 24rpx; }
.f-row { display: flex; align-items: center; margin-bottom: 24rpx; }
.f-lb { width: 100rpx; font-size: 28rpx; font-weight: 600; color: #3A3A3A; flex: none; }
.f-seg {
  flex: 1; display: flex; gap: 16rpx;
  text {
    flex: 1; text-align: center; font-size: 26rpx; font-weight: 600; padding: 16rpx 0; border-radius: 14rpx;
    background: #EEF4F6; color: #5B6B7B;
    &.on { background: linear-gradient(135deg, #2FD0E6, #18A7CE); color: #fff; }
  }
}
.f-acts { display: flex; gap: 20rpx; margin-top: 12rpx; }
.f-btn {
  flex: 1; height: 88rpx; line-height: 88rpx; text-align: center; border-radius: 999rpx; font-size: 30rpx; font-weight: 800;
  &.ghost { background: #F0F4F6; color: #3A3A3A; }
  &.cta { background: linear-gradient(135deg, #FF8A7A, #FF5B4B); color: #fff; }
}
</style>
