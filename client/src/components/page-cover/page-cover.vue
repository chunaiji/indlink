<template>
  <scroll-view scroll-y class="cover-scroll">
    <image :src="image || '/static/city-cover.png'" mode="widthFix" class="cover-img" />
  </scroll-view>
</template>

<script>
// 覆盖图展示:整页可滚动大图,空图回退内置默认图。
export default {
  props: {
    image: { type: String, default: '' }
  }
}
</script>

<style scoped>
.cover-scroll { height: calc(100vh - 100rpx); }
.cover-img { width: 100%; display: block; }
</style>
