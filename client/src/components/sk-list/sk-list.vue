<template>
  <view class="skl">
    <view v-for="i in rows" :key="i" class="skl-row">
      <view class="skl-ava sk"></view>
      <view class="skl-info">
        <view class="skl-line sk" style="width:40%"></view>
        <view class="skl-line sk" style="width:66%;margin-top:14rpx"></view>
      </view>
      <view class="skl-btn sk"></view>
    </view>
  </view>
</template>

<script>
export default {
  name: 'sk-list',
  props: { rows: { type: Number, default: 5 } }
}
</script>

<style lang="scss">
.skl-row {
  display: flex; align-items: center; gap: 24rpx;
  background: $card; border: 1rpx solid $line; border-radius: $r-lg;
  padding: 24rpx; margin-bottom: 20rpx;
}
.skl-ava { width: 96rpx; height: 96rpx; border-radius: $r-avatar; flex: none; }
.skl-info { flex: 1; }
.skl-line { height: 26rpx; border-radius: $r-xs; }
.skl-btn { width: 120rpx; height: 64rpx; border-radius: $r-sm; flex: none; }
.sk {
  background: linear-gradient(90deg, #EDF2F4 25%, #F6F9FA 37%, #EDF2F4 63%);
  background-size: 400% 100%;
  animation: sk-shimmer 1.3s ease infinite;
}
@keyframes sk-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}
@media (prefers-reduced-motion: reduce) {
  .sk { animation: none; }
}
</style>
