<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <view class="searchbar">
      <input v-model="keyword" class="kw" placeholder="搜索昵称、省、城市" placeholder-class="ph" />
      <view class="go" @tap="reload($event)">搜索</view>
    </view>

    <view class="filter">
      <view class="seg">
        <text :class="{ on: gender === 0 }" @tap="setGender(0)">全部</text>
        <text :class="{ on: gender === 2 }" @tap="setGender(2)">女生</text>
        <text :class="{ on: gender === 1 }" @tap="setGender(1)">男生</text>
      </view>
      <view class="seg sort">
        <text :class="{ on: sort === 'active' }" @tap="setSort('active')">活跃</text>
        <text :class="{ on: sort === 'new' }" @tap="setSort('new')">新人</text>
      </view>
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
            <view class="rel">
              被关注人数 <text class="n">{{ u.fans_count || 0 }}</text>　魅力 <text class="n">{{ u.charm || 0 }}</text>　注册天数 <text class="n">{{ regDays(u.created_at) }}</text>
            </view>
          </view>
          <view class="ract">
            <text v-if="!paidUsers[u.user_id]" class="cost">−{{ chatPrice }}金币</text>
            <view class="chatbtn" :class="{ free: paidUsers[u.user_id] }" @tap.stop="startChat(u)">
              开聊<image class="hearts" :src="paidUsers[u.user_id] ? '/static/icons/xindong_1.png' : '/static/icons/xindong.png'" mode="aspectFit" />
            </view>
          </view>
        </view>
        <empty-state v-if="!users.length" icon="🧭" :title="keyword ? '没搜到匹配的人' : '附近还没有人'" desc="换个关键词或筛选再看看" />
        <ad-slot slot-key="banner_city" type="banner" />
      </block>
    </scroll-view>
    </block>
    <user-card ref="ucard" @chat="onCardChat($event)" @like="onCardLike($event)" />
  </view>
  <ad-slot v-if="!showCover" slot-key="banner_city" type="banner" />
  <tab-bar :current="1" />
</template>

<script>
import { matchApi, chatApi, moderationApi, relationApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { useFeaturesStore } from '../../store/features'
import { isIOS } from '../../utils/platform'
import pagesCover from '../../mixins/pagesCover'
import UserCard from '../../components/user-card/user-card.vue'

export default {
  components: { UserCard },
  mixins: [pagesCover],
  data() {
    return { keyword: '', gender: 0, sort: 'active', users: [], loading: true, paidUsers: {} }
  },
  computed: {
    chatPrice() { return useUserStore().chatPrice },
    cardOn() { return useFeaturesStore().userCard },
    // 覆盖图只在非 iOS 生效:iOS 审核不需要屏蔽,恒显示原内容
    showCover() { return this.cover.on && !isIOS() }
  },
  async onShow() {
    this.paidUsers = uni.getStorageSync('paid_city_users') || {}
    await this.loadCover()
    if (this.showCover) return
    const fs = useFeaturesStore()
    if (!fs.loaded) fs.fetch()
    this.reload()
  },
  onPullDownRefresh() {
    this.loadCover().then(() => { if (!this.showCover) return this.reload() }).finally(() => uni.stopPullDownRefresh())
  },
  methods: {
    async reload() {
      this.loading = true
      try {
        this.users = await matchApi.cityUsers({
          keyword: this.keyword.trim(),
          gender: this.gender,
          sort: this.sort,
          page: 1,
          size: 30
        }) || []
      } catch (e) {} finally { this.loading = false }
    },
    setGender(g) { this.gender = g; this.reload() },
    setSort(s) { this.sort = s; this.reload() },
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
              await moderationApi.report(u.user_id, 'user', '列表举报')
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
    genderText(g) { return g === 1 ? '♂' : g === 2 ? '♀' : '·' },
    avatarText(u) { return (u.nickname || '?').slice(0, 1) },
    async startChat(u) {
      try {
        const chat = await chatApi.start(u.user_id, 0)
        this.paidUsers = { ...this.paidUsers, [u.user_id]: true }
        uni.setStorageSync('paid_city_users', this.paidUsers)
        uni.navigateTo({ url: `/pages/chat/chat?id=${chat.chat_id}&name=${encodeURIComponent(u.nickname)}` })
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
      if (!this.cardOn) return // 资料卡未开启保持原状
      this.$refs.ucard.show(u.user_id)
    },
    onCardChat(userId) {
      const u = this.users.find((x) => String(x.user_id) === String(userId))
      if (u) this.startChat(u)
    },
    async onCardLike(userId) {
      try { await relationApi.like(userId); uni.showToast({ title: '已喜欢 ❤️', icon: 'none' }) } catch (e) {}
    }
  }
}
</script>

<style lang="scss">
.page { padding: 20rpx 28rpx; }
.searchbar {
  display: flex; align-items: center; gap: 16rpx;
  background: #EEF7F9; border-radius: 24rpx; padding: 16rpx 24rpx;
  .kw { flex: 1; font-size: 26rpx; color: $ink; }
  .ph { color: $ink-faint; }
  .go {
    background: linear-gradient(135deg, $sea-2, $sea-3); color: #fff;
    font-weight: 700; padding: 10rpx 24rpx; border-radius: $r-md; font-size: 24rpx;
  }
}
.filter {
  display: flex; align-items: center; justify-content: space-between; padding: 24rpx 8rpx 8rpx;
  .seg { display: flex; gap: 28rpx; font-size: 28rpx; color: $ink-faint;
    .on { color: $ink; font-weight: 700; }
  }
  .seg.sort { gap: 18rpx; font-size: 24rpx;
    text { padding: 6rpx 18rpx; border-radius: 999rpx; background: #EEF4F6; }
    .on { color: #fff; background: linear-gradient(135deg, $sea-2, $sea-3); }
  }
}
.list { height: calc(100vh - 380rpx); }
.ucard {
  display: flex; align-items: center; gap: 24rpx;
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 16rpx 20rpx; margin-bottom: 14rpx;
  box-shadow: $shadow-card;
}
/* 尺寸/形状由 user-avatar 组件自身控制(:size + shape),此处只占位,避免与内联样式冲突产生重叠痕迹 */
.ava { flex: none; }
.ava-wrap { position: relative; flex: none; }
.vbadge {
  position: absolute; top: -56rpx; left: 50%;
  width: 104rpx; height: 104rpx; z-index: 3; pointer-events: none;
  transform: translateX(-50%);
  animation: vbeat 1.3s ease-in-out infinite;
}
/* 心跳：两次快速收缩 + 间歇,模拟脉搏 */
@keyframes vbeat {
  0%   { transform: translateX(-50%) scale(1); }
  12%  { transform: translateX(-50%) scale(1.18); }
  24%  { transform: translateX(-50%) scale(1); }
  36%  { transform: translateX(-50%) scale(1.12); }
  50%  { transform: translateX(-50%) scale(1); }
  100% { transform: translateX(-50%) scale(1); }
}
.info { flex: 1; min-width: 0; padding-left: 4rpx; }
.name { font-weight: 700; font-size: 29rpx; color: $ink-900; }
.pills { display: flex; flex-wrap: wrap; align-items: center; gap: 10rpx; margin-top: 7rpx; }
.pill {
  font-size: 20rpx; color: $ink-soft; background: $ink-50;
  padding: 3rpx 12rpx; border-radius: 999rpx; line-height: 1.5;
}
.pill.g.female { color: #E0533F; background: rgba(255,107,91,0.14); }
.pill.g.male { color: $sea-deep; background: rgba(79,201,240,0.16); }
.rel {
  margin-top: 7rpx; font-size: 21rpx; color: $ink-faint;
  .n { color: $ink-soft; font-weight: 700; }
}
/* 开聊按钮与扩列墙保持一致 */
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
.empty { text-align: center; color: $ink-faint; padding: 80rpx 0; font-size: 26rpx; }
</style>
