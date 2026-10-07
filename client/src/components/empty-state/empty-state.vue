<template>
  <view class="empty">
    <view class="empty-ic">{{ icon }}</view>
    <view class="empty-tt">{{ title }}</view>
    <view v-if="desc" class="empty-ds">{{ desc }}</view>
    <view v-if="action" class="empty-btn" @tap="$emit('action')">{{ action }}</view>
  </view>
</template>

<script>
export default {
  name: 'empty-state',
  props: {
    icon: { type: String, default: '🌊' },
    title: { type: String, default: '这里还很安静' },
    desc: { type: String, default: '' },
    action: { type: String, default: '' }
  }
}
</script>

<style lang="scss">
.empty { padding: 96rpx 48rpx; text-align: center; }
.empty-ic { font-size: 88rpx; opacity: 0.5; }
.empty-tt { font-size: 28rpx; font-weight: 600; color: $ink-600; margin-top: 20rpx; }
.empty-ds { font-size: 24rpx; color: $ink-400; margin-top: 10rpx; line-height: 1.6; }
.empty-btn {
  display: inline-block; margin-top: 28rpx; padding: 0 36rpx; height: 72rpx; line-height: 72rpx;
  background: linear-gradient(135deg, $coral, $coral-deep); color: #fff;
  font-size: 26rpx; font-weight: 600; border-radius: $r-sm;
}
</style>
