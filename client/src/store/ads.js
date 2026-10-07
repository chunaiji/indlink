import { defineStore } from 'pinia'
import { adApi } from '../api/index'

// 广告配置 store:启动时拉一次 /ads,各页用 canShow(slotKey) 判断是否展示。
export const useAdsStore = defineStore('ads', {
  state: () => ({ enabled: false, interGapSec: 180, slots: {}, loaded: false }),
  actions: {
    async fetchConfig() {
      try {
        const r = await adApi.config()
        if (r) {
          this.enabled = !!r.enabled
          this.interGapSec = r.inter_gap_sec || 180
          this.slots = r.slots || {}
          this.loaded = true
        }
      } catch (e) {}
    },
    slot(key) { return this.slots[key] || { on: false, unit: '' } },
    canShow(key) { const s = this.slot(key); return this.enabled && s.on && !!s.unit }
  }
})
