<template>
  <!-- 微信已将 Banner 广告升级为原生模板广告,统一用 ad-custom 渲染(banner 组件已停用) -->
  <ad-custom v-if="visible" :unit-id="unit" @error="onErr" />
</template>

<script>
import { useAdsStore } from '../../store/ads'

export default {
  props: {
    slotKey: { type: String, required: true },
    type: { type: String, default: 'banner' }
  },
  data() { return { err: false } },
  computed: {
    visible() { return !this.err && useAdsStore().canShow(this.slotKey) },
    unit() { return useAdsStore().slot(this.slotKey).unit }
  },
  methods: { onErr() { this.err = true } }
}
</script>
