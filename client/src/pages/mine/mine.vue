<template>
  <view class="mine">
    <!-- 注:mine 是工具形态保留的 tab,不接整页覆盖;社交入口显隐由 mineFunctions 细粒度控制 -->
    <!-- 头图 -->
    <view class="top" :style="{ paddingTop: statusBarHeight + 'px' }">
      <view class="id" @tap="editProfile($event)">
        <user-avatar class="ava" :name="profile.nickname || '我'" :src="profile.avatar" :size="180" shape="circle" />
        <view class="meta">
          <view class="name-row">
            <text class="name">{{ profile.nickname || '用户' }}</text>
            <text class="edit">编辑资料</text>
          </view>
          <view class="sub">ID {{ profile.user_id || '-' }}</view>
          <view class="sub">{{ profile.gender === 1 ? '♂ 男' : profile.gender === 2 ? '♀ 女' : '保密' }} · {{ profile.age || '?' }}岁 · {{ profile.city || '未填城市' }}</view>
          <view class="sub2" v-if="fn.verify">
            <text class="verify" @tap.stop="goVerify($event)">{{ profile.is_verified ? '已认证 ✓' : '认证中心 ›' }}</text>
          </view>
        </view>
      </view>
      <view v-if="fn.complete_tip && (!profile.city || !profile.gender)" class="completebar" @tap="editProfile($event)">
        <view class="marquee-inner">{{ marqueeText }}　　　{{ marqueeText }}　　　</view>
      </view>
      <view class="stats">
        <view class="cell"><view class="n">{{ stats.i_like }}</view><view class="l">我喜欢</view></view>
        <view class="cell"><view class="n">{{ stats.like_me }}</view><view class="l">喜欢我</view></view>
        <view class="cell"><view class="n">{{ stats.viewed_me }}</view><view class="l">看过我</view></view>
        <view class="cell"><view class="n">{{ profile.charm || 0 }}</view><view class="l">魅力值</view></view>
      </view>
    </view>

    <!-- 钱包 -->
    <view v-if="fn.wallet" class="wallet">
      <view class="wcell" @tap="onWalletTap($event)">
        <view class="wl">我的钱包</view>
        <view class="wv">{{ balance }}<image class="coin-icon" src="/static/icons/coin.png" mode="aspectFit" /></view>
        <view v-if="fn.recharge" class="recharge">充值金币 ›</view>
      </view>
      <view class="wcell" v-if="fn.moments" @tap="goMoments($event)">
        <view class="wl">📸 我的动态</view>
        <view class="wv">—</view>
        <view class="recharge soft">发布动态 ›</view>
      </view>
    </view>

    <!-- 每日签到:7天阶梯 -->
    <view v-if="checkin.enabled" class="checkin ladder" :class="{ done: checkin.signed_today }">
      <view class="ck-hd" @tap="doCheckin($event)">
        <text class="ci">{{ checkin.signed_today ? '🎉' : '📅' }}</text>
        <text class="ct">{{ checkin.signed_today ? `已连签 ${checkin.streak} 天` : (checkin.streak ? `连签 ${checkin.streak} 天 · 今日第 ${checkin.today_index} 天` : '每日签到') }}</text>
        <text class="cb">{{ checkin.signed_today ? '明天再来 ✓' : `领 ${checkin.today_coins} 金币 ›` }}</text>
      </view>
      <view class="ck-days">
        <view
          v-for="(c, i) in checkin.ladder" :key="i"
          class="ck-day" :class="{ got: dayGot(i), cur: !checkin.signed_today && i === curSlot }"
        >
          <text class="ck-coin">{{ dayGot(i) ? '✓' : c }}</text>
          <text class="ck-lb">{{ i === checkin.ladder.length - 1 ? '🎁' : 'D' + (i + 1) }}</text>
        </view>
      </view>
      <view v-if="checkin.can_makeup" class="ck-makeup" @tap="doMakeup($event)">
        🎬 昨天忘了签？看视频补签（本月 {{ checkin.makeup_used }}/{{ checkin.makeup_limit }}）›
      </view>
    </view>
    <view v-if="adReward" class="checkin" @tap="onWatchAd($event)">
      <text class="ci">🎬</text>
      <text class="ct">看视频领金币</text>
      <text class="cb">去观看 ›</text>
    </view>

    <view class="sec">其它功能</view>
    <view class="fn-grid">
      <view v-if="fn.verify" class="fn" @tap="goVerify($event)"><image class="fimg" src="/static/icons/yirenzheng_1.png" />真人认证</view>
      <view v-if="fn.avatar" class="fn" @tap="changeAvatar($event)"><image class="fimg" src="/static/icons/paizhao.png" />换头像</view>
      <view v-if="fn.viewed" class="fn" @tap="goViewed($event)"><image class="fimg" src="/static/icons/hudong.png" />浏览记录</view>
      <view v-if="fn.items" class="fn" @tap="goItems($event)"><image class="fimg" src="/static/icons/liwu.png" />道具商城</view>
      <view v-if="fn.collection" class="fn" @tap="goCollection($event)"><image class="fimg" src="/static/icons/shoucang.png" />我的收藏</view>
      <view v-if="fn.walletLog" class="fn" @tap="goWalletLog($event)"><image class="fimg" src="/static/icons/liushuisel.png" />金币流水</view>
      <view v-if="fn.orders" class="fn" @tap="goOrders($event)"><image class="fimg" src="/static/icons/dingdan.png" />我的订单</view>
      <view v-if="fn.moments" class="fn" @tap="goMoments($event)"><image class="fimg" src="/static/icons/xinghudong.png" />我的动态</view>
      <view v-if="fn.blocklist" class="fn" @tap="goBlocklist($event)"><image class="fimg" src="/static/icons/heimingdan.png" />黑名单</view>
      <view v-if="fn.contact" class="fn" @tap="contact($event)"><image class="fimg" src="/static/icons/zaixiankefu.png" />客服</view>
      <view v-if="fn.settings" class="fn" @tap="openSettings($event)"><image class="fimg" src="/static/icons/shezhi.png" />设置</view>
    </view>
    <ad-slot slot-key="banner_mine" type="banner" />
    <view style="height: 140rpx;"></view>

    <!-- 联系客服弹层(文案+图片后台可配,图片一般放客服微信二维码) -->
    <view v-if="contactShow" class="cs-mask" @tap="contactShow = false">
      <view class="cs-panel" @tap.stop>
        <view class="cs-hd">联系客服</view>
        <image
          v-if="fn.contact_image" class="cs-img" :src="fn.contact_image" mode="widthFix"
          @tap="previewContactImg()"
        />
        <view class="cs-text">{{ contactText }}</view>
        <view v-if="fn.contact_image" class="cs-tip">点击图片可放大,长按保存</view>
        <view class="cs-btn" @tap="contactShow = false">知道了</view>
      </view>
    </view>
  </view>
  <tab-bar :current="4" />
</template>

<script>
import { useUserStore } from '../../store/user'
import { useWalletStore } from '../../store/wallet'
import { useAdsStore } from '../../store/ads'
import { relationApi, userApi, sysApi, checkinApi } from '../../api/index'
import { chooseAndUploadImage } from '../../utils/upload'
import { showRewarded } from '../../utils/ad'

// 默认全部隐藏，拉到 /mine-functions 接口后再按配置显示，避免接口返回前闪出功能项
const FN_DEFAULT = {
  verify: false, avatar: false, viewed: false, items: false, collection: false,
  wallet: false, orders: false, blocklist: false, contact: false, settings: false, moments: false,
  recharge: false, walletLog: false
}

export default {
  data() {
    return {
      statusBarHeight: 20, profile: {}, stats: { i_like: 0, like_me: 0, viewed_me: 0 }, fn: { ...FN_DEFAULT },
      checkin: { enabled: false, coins: 0, signed_today: false, streak: 0, today_index: 1, today_coins: 0, ladder: [], can_makeup: false, makeup_used: 0, makeup_limit: 0 },
      contactShow: false
    }
  },
  computed: {
    balance() { return useWalletStore().balance },
    adReward() { return useAdsStore().canShow('reward_coin') },
    contactText() { return this.fn.contact_text || '客服微信:chunj008 \n工作时间 10:00-22:00' },
    marqueeText() { return this.fn.complete_tip_text || '✨ 完善资料，更容易被同城的人看到 ›' },
    // 今天在 7 格中的位置(0-based)
    curSlot() {
      const len = (this.checkin.ladder || []).length || 1
      return (this.checkin.today_index - 1) % len
    }
  },
  onLoad() {
    this.statusBarHeight = uni.getSystemInfoSync().statusBarHeight || 20
  },
  async onShow() {
    useAdsStore().fetchConfig()
    const user = useUserStore()
    if (!user.isLogin) await user.silentLogin()
    this.profile = user.profile || {}
    try {
      await useWalletStore().fetchBalance()
      this.stats = await relationApi.stats() || this.stats
    } catch (e) {}
    try {
      const f = await sysApi.mineFunctions()
      if (f) this.fn = { ...FN_DEFAULT, ...f }
    } catch (e) {}
    await this.loadCheckin()
  },
  methods: {
    onWatchAd() { showRewarded('reward_coin') },
    async loadCheckin() {
      try {
        const c = await checkinApi.status()
        if (c) {
          this.checkin = {
            enabled: !!c.enabled, coins: c.coins || 0, signed_today: !!c.signed_today,
            streak: c.streak || 0, today_index: c.today_index || 1, today_coins: c.today_coins || c.coins || 0,
            ladder: Array.isArray(c.ladder) ? c.ladder : [], can_makeup: !!c.can_makeup,
            makeup_used: c.makeup_used || 0, makeup_limit: c.makeup_limit || 0
          }
        }
      } catch (e) {}
    },
    // 档位 i(0-based)在本轮是否已签
    dayGot(i) {
      const len = (this.checkin.ladder || []).length || 1
      const prog = this.checkin.signed_today
        ? ((this.checkin.today_index - 1) % len) + 1
        : (this.checkin.today_index - 1) % len
      return i < prog
    },
    async doCheckin() {
      if (!this.checkin.enabled || this.checkin.signed_today) return
      try {
        const r = await checkinApi.sign()
        if (typeof r.balance === 'number') useWalletStore().setBalance(r.balance)
        if (r.already_signed) {
          uni.showToast({ title: '今日已签到', icon: 'none' })
        } else {
          // icon:'none' 避免微信带图标时标题被截断(>7字)
          uni.showToast({ title: `连签第 ${r.day_index} 天 +${r.coins} 金币`, icon: 'none' })
        }
        this.loadCheckin()
      } catch (e) {
        uni.showToast({ title: '签到失败,请重试', icon: 'none' })
      }
    },
    doMakeup() {
      if (!this.checkin.can_makeup) return
      showRewarded('reward_coin', async () => {
        try {
          const r = await checkinApi.makeup()
          if (typeof r.balance === 'number') useWalletStore().setBalance(r.balance)
          uni.showToast({ title: `补签成功 +${r.coins} 金币`, icon: 'none' })
          this.loadCheckin()
        } catch (e) {
          uni.showToast({ title: (e && e.message) || '补签失败', icon: 'none' })
        }
      })
    },
    editProfile() { uni.navigateTo({ url: '/pages/profile-edit/profile-edit' }) },
    goRecharge() { uni.navigateTo({ url: '/pages/recharge/recharge' }) },
    onWalletTap() { if (this.fn.recharge) this.goRecharge() }, // 充值入口隐藏时点击钱包卡片不跳转
    goWalletLog() { uni.navigateTo({ url: '/pages/wallet-log/wallet-log' }) },
    goOrders() { uni.navigateTo({ url: '/pages/orders/orders' }) },
    goItems() { uni.navigateTo({ url: '/pages/items/items' }) },
    goBlocklist() { uni.navigateTo({ url: '/pages/blocklist/blocklist' }) },
    goViewed() { uni.navigateTo({ url: '/pages/viewed/viewed' }) },
    goCollection() { uni.navigateTo({ url: '/pages/collection/collection' }) },
    goMoments() { uni.navigateTo({ url: '/pages/moments/moments' }) },
    async goVerify() {
      if (this.profile.is_verified) { uni.showToast({ title: '已认证', icon: 'none' }); return }
      uni.showModal({
        title: '真人认证', content: '完成实名+真人头像认证,提升信任与曝光',
        confirmText: '去认证',
        success: async (r) => {
          if (r.confirm) {
            try { await userApi.verify(); this.profile.is_verified = true; uni.showToast({ title: '认证成功' }) } catch (e) {}
          }
        }
      })
    },
    async changeAvatar() {
      try {
        const url = await chooseAndUploadImage()
        await useUserStore().updateProfile({ avatar: url })
        this.profile = useUserStore().profile || {}
        uni.showToast({ title: '头像已更新' })
      } catch (e) {}
    },
    contact() { this.contactShow = true },
    previewContactImg() {
      if (this.fn.contact_image) uni.previewImage({ urls: [this.fn.contact_image] })
    },
    openSettings() {
      uni.showActionSheet({
        itemList: ['消息通知设置', '隐私设置', '退出登录'],
        success: (res) => {
          if (res.tapIndex === 2) {
            uni.showModal({
              title: '退出登录', content: '确定退出当前账号?',
              success: (r) => { if (r.confirm) { useUserStore().logout(); uni.showToast({ title: '已退出', icon: 'none' }) } }
            })
          } else {
            this.soon('该设置项')
          }
        }
      })
    },
    soon(name) { uni.showToast({ title: (name || '功能') + '开发中', icon: 'none' }) },
    onWatchAd() { showRewarded('reward_coin') }
  }
}
</script>

<style lang="scss">
.mine { min-height: 100vh; background: $ground; }
.top {
  background: linear-gradient(160deg, #7FD4E3, #3BA9CC 80%);
  padding: 0 32rpx 60rpx; color: #fff;
}
.id { display: flex; align-items: center; gap: 28rpx; padding-top: 88rpx; }
.ava {
  width: 180rpx; height: 180rpx; border-radius: 50%;
  background: linear-gradient(135deg, #FFE3C2, #FFB59B);
  display: flex; align-items: center; justify-content: center; font-size: 88rpx;
  border: 5rpx solid rgba(255,255,255,0.7);
}
.meta { flex: 1; min-width: 0; }
.name-row { display: flex; align-items: center; gap: 16rpx; }
.name { font-size: 38rpx; font-weight: 700; max-width: 320rpx; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.edit {
  flex: none; font-size: 21rpx; font-weight: 600;
  padding: 5rpx 18rpx; border-radius: 999rpx;
  background: rgba(255,255,255,0.22); border: 1rpx solid rgba(255,255,255,0.35);
}
.sub { font-size: 24rpx; opacity: 0.92; margin-top: 8rpx; }
.sub2 { font-size: 24rpx; opacity: 0.92; margin-top: 6rpx; }
.verify { text-decoration: underline; }
.completebar {
  margin-top: 20rpx; background: rgba(255,255,255,0.18); border-radius: 999rpx;
  padding: 14rpx 24rpx; font-size: 24rpx; font-weight: 600; overflow: hidden; white-space: nowrap;
}
.marquee-inner {
  display: inline-block; white-space: nowrap;
  animation: marquee-run 12s linear infinite;
}
@keyframes marquee-run {
  0% { transform: translateX(0); }
  100% { transform: translateX(-50%); }
}
.stats { display: flex; margin-top: 32rpx; text-align: center; }
.cell { flex: 1; .n { font-size: 38rpx; font-weight: 700; } .l { font-size: 24rpx; opacity: 0.9; } }
.wallet {
  display: flex; gap: 20rpx; margin: -40rpx 24rpx 0; position: relative; z-index: 2;
}
.wcell {
  flex: 1; padding: 26rpx; border-radius: $r-lg; background: $card; border: 1rpx solid $line;
  box-shadow: $shadow-card;
  .wl { font-size: 24rpx; color: $ink-soft; font-weight: 600; }
  .wv { font-size: 46rpx; font-weight: 700; margin-top: 8rpx; display: flex; align-items: center; gap: 8rpx; }
  .coin-icon { width: 55rpx; height: 55rpx; flex-shrink: 0; }
  .recharge { font-size: 22rpx; color: $coral; font-weight: 700; &.soft { color: $sea-deep; } }
}
.checkin {
  display: flex; align-items: center; gap: 16rpx; margin: 20rpx 24rpx 0;
  padding: 26rpx 28rpx; border-radius: $r-lg; box-shadow: $shadow-card;
  background: linear-gradient(135deg, #FFE9C7, #FFD79A);
  .ci { font-size: 40rpx; flex: none; }
  .ct { font-size: 28rpx; font-weight: 700; color: #7A4B12; }
  .cb { margin-left: auto; font-size: 24rpx; font-weight: 700; color: #B5621C; }
  &.done { background: $ink-50; border: 1rpx solid $line;
    .ct { color: $ink-soft; } .cb { color: $ink-faint; } }
}
/* 7天阶梯版签到卡:头行 + 7格 + 补签行 */
.checkin.ladder {
  display: block;
  .ck-hd { display: flex; align-items: center; gap: 16rpx; }
  .ck-days { display: flex; gap: 8rpx; margin-top: 20rpx; }
  .ck-day {
    flex: 1; text-align: center; padding: 12rpx 0 8rpx; border-radius: 12rpx;
    background: rgba(255,255,255,0.55);
    .ck-coin { display: block; font-size: 26rpx; font-weight: 800; color: #B5621C; }
    .ck-lb { display: block; font-size: 20rpx; color: #A57A3C; margin-top: 2rpx; }
    &.got { background: rgba(255,255,255,0.9);
      .ck-coin { color: #2b8a3e; } }
    &.cur { background: #fff; box-shadow: 0 0 0 3rpx #FF9F43 inset;
      .ck-coin { color: #E8590C; } }
  }
  .ck-makeup { margin-top: 18rpx; font-size: 24rpx; font-weight: 600; color: #7A4B12; }
  &.done {
    .ck-day { background: rgba(255,255,255,0.35); }
    .ck-day.got { background: rgba(255,255,255,0.7); }
  }
}
/* 联系客服弹层 */
.cs-mask { position: fixed; inset: 0; background: rgba(8,30,40,0.5); z-index: 99; display: flex; align-items: center; justify-content: center; }
.cs-panel { width: 560rpx; background: #fff; border-radius: 24rpx; padding: 36rpx 32rpx 28rpx; }
.cs-hd { font-size: 32rpx; font-weight: 800; color: $ink-900; text-align: center; }
.cs-img { width: 320rpx; display: block; margin: 24rpx auto 0; border-radius: 12rpx; }
.cs-text { font-size: 28rpx; color: $ink-soft; line-height: 1.7; text-align: center; margin-top: 20rpx; white-space: pre-wrap; }
.cs-tip { font-size: 20rpx; color: $ink-faint; text-align: center; margin-top: 10rpx; }
.cs-btn {
  margin-top: 28rpx; height: 80rpx; line-height: 80rpx; text-align: center; border-radius: 999rpx;
  background: linear-gradient(135deg, #FF8A7A, #FF6B5B); color: #fff; font-size: 28rpx; font-weight: 800;
}

.sec { font-size: 26rpx; font-weight: 700; padding: 32rpx 32rpx 12rpx; }
.fn-grid { display: flex; flex-wrap: wrap; padding: 0 16rpx; }
.fn {
  width: 25%; text-align: center; font-size: 22rpx; color: $ink-soft; margin-bottom: 28rpx;
  .fi { font-size: 48rpx; display: block; margin-bottom: 8rpx; }
  .fimg { width: 56rpx; height: 56rpx; display: block; margin: 0 auto 8rpx; }
}
</style>
