import { defineStore } from 'pinia'
import { sysApi } from '../api/index'

// 留存功能开关(后台可配,默认全关):漂流轨迹/深夜瓶/资料卡/魅力周榜。
export const useFeaturesStore = defineStore('features', {
  state: () => ({
    loaded: false,
    bottleTrace: false,
    nightBottle: false,
    nightStart: 22,
    nightEnd: 2,
    userCard: false,
    charmRank: false,
    square: false, // 动态广场:开=扩列tab变朋友圈,关=旧用户列表
    linkMP: { on: false, title: '', list: [] }, // 首页关联小程序(最多4个:{appid,title,icon,path})
    cameraWM: { text: '小纸条', color: 'rgba(255,255,255,0.16)', size: 26 } // 水印相机平铺水印样式
  }),
  getters: {
    // 当前是否处于深夜场时段(跨零点区间,如 22 → 2)
    inNightWindow: (s) => {
      if (!s.nightBottle) return false
      const h = new Date().getHours()
      return s.nightStart <= s.nightEnd
        ? h >= s.nightStart && h < s.nightEnd
        : h >= s.nightStart || h < s.nightEnd
    }
  },
  actions: {
    async fetch() {
      try {
        const f = await sysApi.features()
        if (f) {
          this.bottleTrace = !!f.bottle_trace
          this.nightBottle = !!f.night_bottle
          this.nightStart = Number.isInteger(f.night_start) ? f.night_start : 22
          this.nightEnd = Number.isInteger(f.night_end) ? f.night_end : 2
          this.userCard = !!f.user_card
          this.charmRank = !!f.charm_rank
          this.square = !!f.square
          if (f.link_mp) this.linkMP = { on: !!f.link_mp.on, title: f.link_mp.title || '', list: Array.isArray(f.link_mp.list) ? f.link_mp.list : [] }
          if (f.camera_wm) {
            this.cameraWM = {
              text: f.camera_wm.text || '小纸条',
              color: f.camera_wm.color || 'rgba(255,255,255,0.16)',
              size: f.camera_wm.size > 0 ? f.camera_wm.size : 26
            }
          }
          this.loaded = true
        }
      } catch (e) {}
    }
  }
})
