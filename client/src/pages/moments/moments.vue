<template>
  <view class="page">
    <page-cover v-if="showCover" :image="cover.image" />
    <block v-else>
    <!-- 发布框 -->
    <view class="compose">
      <textarea
        v-model="draft"
        class="cta"
        :maxlength="500"
        placeholder="分享你此刻的心情…"
        placeholder-class="cph"
        :focus="composeFocus"
        @blur="composeFocus = false"
      />
      <view class="cimgs">
        <image v-for="(img, i) in draftImgs" :key="i" class="cimg" :src="img" mode="aspectFill" @tap="removeDraftImg(i)" />
        <view v-if="draftImgs.length < 9" class="cimg-add" @tap="pickDraftImgs()">＋</view>
      </view>
      <view class="cfoot">
        <text class="cnt">{{ draft.length }}/500</text>
        <view class="seg">
          <text :class="{ on: visible === 'public' }" @tap="visible = 'public'">公开</text>
          <text :class="{ on: visible === 'self' }" @tap="visible = 'self'">仅自己</text>
        </view>
        <view class="pub" :class="{ on: draft.trim() || draftImgs.length }" @tap="publish">发布</view>
      </view>
    </view>

    <!-- 列表 -->
    <sk-list v-if="loading" :rows="5" />
    <block v-else>
      <view v-for="m in list" :key="m.moment_id" class="mcard" @longpress="onLongPress(m)">
        <view class="mhead">
          <user-avatar class="mava" :name="myNickname" :src="myAvatar" :size="64" shape="circle" />
          <view class="mmeta">
            <view class="mname">{{ myNickname }}</view>
            <view class="mtime">{{ shortTime(m.created_at) }}{{ m.visible === 'self' ? ' · 仅自己可见' : '' }}</view>
          </view>
        </view>
        <view v-if="m.content" class="mbody">{{ m.content }}</view>
        <view v-if="imgs(m).length" class="mimgs">
          <image
            v-for="(img, i) in imgs(m)" :key="i" class="mimg" :src="img"
            :mode="imgs(m).length === 1 ? 'widthFix' : 'aspectFill'"
            @tap="previewImgs(m, i)"
          />
        </view>
        <view class="mfoot">
          <view class="mlike" @tap="like(m)">
            <text :class="{ liked: liked[m.moment_id] }">{{ liked[m.moment_id] ? '♥' : '♡' }}</text>
            <text class="lcnt">{{ m.like_count || 0 }}</text>
          </view>
        </view>
      </view>
      <empty-state v-if="!list.length" icon="📸" title="还没有动态" desc="发布你的第一条动态吧" />
    </block>
    </block>
  </view>
</template>

<script>
import { momentApi } from '../../api/index'
import { useUserStore } from '../../store/user'
import { chooseAndUploadImages } from '../../utils/upload'
import pagesCover from '../../mixins/pagesCover'

export default {
  mixins: [pagesCover],
  data() {
    return { draft: '', draftImgs: [], visible: 'public', composeFocus: false, list: [], liked: {}, loading: true }
  },
  computed: {
    myNickname() { return (useUserStore().profile || {}).nickname || '我' },
    myAvatar() { return (useUserStore().profile || {}).avatar || '' }
  },
  async onShow() {
    await this.loadCover()
    if (this.showCover) return
    this.reload()
  },
  methods: {
    async reload() {
      this.loading = true
      try { this.list = await momentApi.mine({ page: 1, size: 50 }) || [] } catch (e) {} finally { this.loading = false }
    },
    async publish() {
      const text = this.draft.trim()
      if (!text && !this.draftImgs.length) return
      try {
        const m = await momentApi.create(text, this.visible, this.draftImgs)
        this.list.unshift(m)
        this.draft = ''
        this.draftImgs = []
        uni.showToast({ title: '发布成功', icon: 'none' })
      } catch (e) {
        uni.showToast({ title: (e && e.message) || '发布失败', icon: 'none' })
      }
    },
    async pickDraftImgs() {
      try {
        const urls = await chooseAndUploadImages(9 - this.draftImgs.length)
        this.draftImgs = this.draftImgs.concat(urls)
      } catch (e) {}
    },
    removeDraftImg(i) { this.draftImgs.splice(i, 1) },
    imgs(m) {
      try { const a = JSON.parse(m.images || '[]'); return Array.isArray(a) ? a : [] } catch (e) { return [] }
    },
    previewImgs(m, i) {
      const urls = this.imgs(m)
      uni.previewImage({ urls, current: urls[i] })
    },
    async like(m) {
      try {
        const r = await momentApi.like(m.moment_id)
        const wasLiked = this.liked[m.moment_id]
        this.liked = { ...this.liked, [m.moment_id]: r.liked }
        m.like_count = Math.max(0, (m.like_count || 0) + (r.liked ? 1 : -1))
        if (!wasLiked && r.liked) uni.showToast({ title: '已点赞', icon: 'none' })
      } catch (e) {}
    },
    onLongPress(m) {
      uni.showActionSheet({
        itemList: ['删除这条动态'],
        success: async (res) => {
          if (res.tapIndex !== 0) return
          try {
            await momentApi.remove(m.moment_id)
            this.list = this.list.filter((x) => x.moment_id !== m.moment_id)
            uni.showToast({ title: '已删除', icon: 'none' })
          } catch (e) {}
        }
      })
    },
    shortTime(t) {
      if (!t) return ''
      return String(t).slice(0, 16).replace('T', ' ')
    }
  }
}
</script>

<style lang="scss">
.page { padding: 24rpx 28rpx; padding-bottom: 60rpx; }
.cimgs { display: flex; flex-wrap: wrap; gap: 10rpx; padding: 0 8rpx 8rpx; }
.cimg { width: 140rpx; height: 140rpx; border-radius: 10rpx; }
.cimg-add {
  width: 140rpx; height: 140rpx; border-radius: 10rpx; border: 2rpx dashed $line;
  display: flex; align-items: center; justify-content: center; font-size: 48rpx; color: $ink-faint;
}
.mimgs { display: flex; flex-wrap: wrap; gap: 8rpx; margin-top: 14rpx; }
.mimg { width: calc(33.33% - 6rpx); height: 200rpx; border-radius: 10rpx;
  &:only-child { width: 400rpx; height: auto; } }
.compose {
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 24rpx; margin-bottom: 24rpx; box-shadow: $shadow-card;
}
.cta {
  width: 100%; min-height: 120rpx; font-size: 28rpx; line-height: 1.7;
  color: $ink; background: transparent; box-sizing: border-box;
}
.cph { color: $ink-faint; }
.cfoot { display: flex; align-items: center; gap: 16rpx; margin-top: 16rpx; }
.cnt { font-size: 22rpx; color: $ink-faint; }
.seg {
  display: flex; gap: 8rpx; margin-left: auto;
  text {
    font-size: 22rpx; padding: 6rpx 18rpx; border-radius: 999rpx;
    background: $ink-50; color: $ink-soft; font-weight: 600;
    &.on { background: rgba(91,208,224,0.18); color: $sea-deep; }
  }
}
.pub {
  font-size: 26rpx; font-weight: 700; color: $ink-faint;
  padding: 10rpx 28rpx; border-radius: 999rpx; background: $ink-50;
  &.on { background: linear-gradient(135deg, $coral, $coral-deep); color: #fff; box-shadow: 0 4rpx 12rpx rgba(255,107,91,0.35); }
}
.mcard {
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 24rpx; margin-bottom: 16rpx; box-shadow: $shadow-card;
}
.mhead { display: flex; align-items: center; gap: 16rpx; margin-bottom: 16rpx; }
.mava { flex: none; }
.mmeta { flex: 1; min-width: 0; }
.mname { font-size: 28rpx; font-weight: 700; color: $ink; }
.mtime { font-size: 22rpx; color: $ink-faint; margin-top: 4rpx; }
.mbody { font-size: 30rpx; line-height: 1.7; color: $ink; word-break: break-word; white-space: pre-wrap; }
.mfoot { display: flex; align-items: center; margin-top: 20rpx; }
.mlike {
  display: flex; align-items: center; gap: 8rpx;
  font-size: 28rpx; color: $ink-soft;
  text { font-size: 32rpx; &.liked { color: $coral; } }
  .lcnt { font-size: 24rpx; }
}
</style>
