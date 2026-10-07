<template>
  <view class="cam-page">
    <view class="sec-hd">水印工具</view>

    <!-- 添加水印（点上半部大图触发选图/拍照；@tap 必须带括号调用，否则本 uni-app 版本编译后不调用）-->
    <view class="cam-card">
      <!-- 顶部图像区（点击区域）-->
      <view class="card-banner add-banner" hover-class="card-banner-press" @tap="pickAdd()">
        <view v-if="left.state === 'idle'" class="banner-idle">
          <text class="banner-label">添加小纸条官方水印</text>
          <text class="banner-sub">拍照，自动打印时间戳</text>
        </view>
        <view v-else-if="left.state === 'loading'" class="banner-spin">
          <view class="spinner add-spin" />
          <text class="spin-txt">正在打印水印…</text>
        </view>
        <image v-else class="banner-img" :src="left.result" mode="aspectFill" />
      </view>
      <!-- 底部信息栏（不触发，仅保存按钮可点）-->
      <view class="card-foot">
        <view class="foot-info">
          <text class="foot-t">添加时间水印</text>
          <text class="foot-d">{{ left.state === 'done' ? '点击重拍' : '拍照 / 相册' }}</text>
        </view>
        <view v-if="left.state === 'done'" class="save-tag add-tag" @tap.stop="doSave(left.result)">保存</view>
        <view v-else class="arr">›</view>
      </view>
    </view>

    <!-- 删除水印 -->
    <view class="cam-card">
      <view class="card-banner rm-banner" hover-class="card-banner-press" @tap="pickRemove()">
        <view v-if="right.state === 'idle'" class="banner-idle">
          <text class="banner-label">删除水印</text>
          <text class="banner-sub">选择图片，智能去除水印</text>
        </view>
        <view v-else-if="right.state === 'loading'" class="banner-spin">
          <view class="spinner rm-spin" />
          <text class="spin-txt">正在删除水印…</text>
        </view>
        <image v-else class="banner-img" :src="right.result" mode="aspectFill" />
      </view>
      <view class="card-foot">
        <view class="foot-info">
          <text class="foot-t">去除图片水印</text>
          <text class="foot-d">{{ right.state === 'done' ? '点击重选' : '从相册选图' }}</text>
        </view>
        <view v-if="right.state === 'done'" class="save-tag rm-tag" @tap.stop="doSave(right.result)">保存</view>
        <view v-else class="arr">›</view>
      </view>
    </view>

    <!-- 仅绘图时挂载 canvas，避免原生组件常驻拦截点击事件 -->
    <canvas v-if="left.state === 'loading'" canvas-id="wm-canvas" :style="canvasStyle" />
    <ad-slot slot-key="banner_privacy" type="banner" />
    <view style="height: 160rpx;" />
  </view>
  <tab-bar :current="5" />
</template>

<script>
import TabBar from '../../components/tab-bar/tab-bar.vue'
import { geoApi } from '../../api/index'
import { uploadFilePath } from '../../utils/upload'
import { useFeaturesStore } from '../../store/features'

export default {
  components: { TabBar },
  onShow() {
    const fs = useFeaturesStore()
    if (!fs.loaded) fs.fetch() // 拉取水印样式等远程配置(失败用默认值)
  },
  data() {
    return {
      left:  { state: 'idle', result: '' },
      right: { state: 'idle', result: '' },
      canvasW: 600,
      canvasH: 800,
      addrText: '',   // 地址水印(定位+逆地理;拒绝授权/失败则为空,仅画时间)
    }
  },
  computed: {
    canvasStyle() {
      return `position:absolute;left:-9999px;top:0;width:${this.canvasW}px;height:${this.canvasH}px;pointer-events:none;`
    }
  },
  methods: {
    /* ── 添加水印 ─────────────────────────── */
    pickAdd() {
      console.log('[camera] pickAdd 触发, state=', this.left.state)
      if (this.left.state === 'loading') return
      uni.chooseMedia({
        count: 1,
        mediaType: ['image'],
        sourceType: ['camera', 'album'],
        success: async (res) => {
          console.log('[camera] chooseMedia success', JSON.stringify(res))
          const path = res.tempFiles[0] && res.tempFiles[0].tempFilePath
          if (!path) { console.log('[camera] 未取到 path'); return }
          this.left.state = 'loading'
          this.secCheckUpload(path) // 内容安全:静默上传原图触发微信 mediaCheckAsync 检测留档
          this.addrText = await this.fetchAddress() // 先拿地址(拒绝授权/失败为空,不阻断)
          this.drawWatermark(path)
            .then(out => { console.log('[camera] 水印完成', out); this.left.result = out; this.left.state = 'done' })
            .catch((e) => {
              console.log('[camera] drawWatermark fail', JSON.stringify(e))
              uni.showToast({ title: '处理失败', icon: 'none' })
              this.left.state = 'idle'
            })
        },
        fail: (e) => { console.log('[camera] chooseMedia fail', JSON.stringify(e)) }
      })
    },

    // 静默定位:拒绝授权/失败不弹窗,水印降级为仅时间
    getLocationSilent() {
      return new Promise((resolve) => {
        uni.getLocation({
          type: 'gcj02',
          success: (res) => resolve({ lat: res.latitude, lng: res.longitude }),
          fail: () => resolve(null)
        })
      })
    },
    // 定位 → 服务端逆地理(后台配腾讯位置服务 key);无 key/失败降级为经纬度文字
    async fetchAddress() {
      const loc = await this.getLocationSilent()
      if (!loc) { this.nudgeLocationOnce(); return '' }
      try {
        const r = await geoApi.regeo(loc.lat, loc.lng)
        if (r && r.address) return r.address
      } catch (e) {}
      return `N${loc.lat.toFixed(4)}° E${loc.lng.toFixed(4)}°`
    },
    // 曾拒绝过授权时微信不再弹系统窗,轻引导一次去设置开启(每次会话最多一次)
    nudgeLocationOnce() {
      if (this._locNudged) return
      this._locNudged = true
      uni.getSetting({
        success: (s) => {
          if (s.authSetting && s.authSetting['scope.userLocation'] === false) {
            uni.showModal({
              title: '开启位置权限',
              content: '开启后,水印会自动带上拍摄地点',
              confirmText: '去开启',
              success: (r) => { if (r.confirm) uni.openSetting() }
            })
          }
        }
      })
    },

    drawWatermark(filePath) {
      return new Promise((resolve, reject) => {
        uni.getImageInfo({
          src: filePath,
          success: (info) => {
            const maxW = 750
            const scale = Math.min(1, maxW / info.width)
            const W = Math.round(info.width  * scale)
            const H = Math.round(info.height * scale)
            this.canvasW = W
            this.canvasH = H

            this.$nextTick(() => {
              const ctx = uni.createCanvasContext('wm-canvas', this)
              ctx.drawImage(filePath, 0, 0, W, H)

              // 品牌水印:低透明度斜向平铺全图(先画,底部时间条压在其上);文字/颜色/字号后台可配
              const wm = useFeaturesStore().cameraWM
              ctx.save()
              ctx.rotate(-30 * Math.PI / 180)
              ctx.setFontSize(wm.size)
              ctx.setFillStyle(wm.color)
              const stepY = Math.max(100, wm.size * 5.5)  // 间距随字号缩放
              const stepX = Math.max(160, wm.size * 8.5)
              const span = W + H // 旋转后需覆盖的对角范围
              for (let y = -span; y < span; y += stepY) {
                // 隔行错位,平铺更自然
                const offset = (Math.round(y / stepY) % 2) * (stepX / 2)
                for (let x = -span; x < span; x += stepX) {
                  ctx.fillText(wm.text, x + offset, y)
                }
              }
              ctx.restore()

              // 底部黑色半透明条(有地址时加高一行)
              const addr = this.addrText
              const barH = addr ? 112 : 80
              ctx.setFillStyle('rgba(0,0,0,0.52)')
              ctx.fillRect(0, H - barH, W, barH)

              // 日期（黄色大字）
              const now = new Date()
              ctx.setFontSize(28)
              ctx.setFillStyle('#FFD700')
              ctx.fillText(this.fmtDate(now), 18, H - barH + 34)

              // 星期+时间（白色小字）
              ctx.setFontSize(20)
              ctx.setFillStyle('rgba(255,255,255,0.88)')
              ctx.fillText(this.fmtTime(now), 18, H - barH + (addr ? 64 : 66))

              // 地址（白色小字,过长截断）
              if (addr) {
                let a = addr
                if (a.length > 26) a = a.slice(0, 25) + '…'
                ctx.setFontSize(18)
                ctx.setFillStyle('rgba(255,255,255,0.8)')
                ctx.fillText(a, 18, H - 14)
              }

              ctx.draw(false, () => {
                uni.canvasToTempFilePath({
                  canvasId: 'wm-canvas',
                  success: r => resolve(r.tempFilePath),
                  fail: reject
                }, this)
              })
            })
          },
          fail: reject
        })
      })
    },

    // 内容安全合规:把用户选的图静默上传服务器,由后端提交微信 mediaCheckAsync 异步检测。
    // 不阻塞水印流程,失败静默;后台图片检测开关关闭时后端自动跳过。
    secCheckUpload(filePath) {
      console.log('[seccheck] 图片安全检测:开始上传', filePath)
      try {
        uploadFilePath(filePath)
          .then((url) => console.log('[seccheck] 上传成功,已提交检测', url))
          .catch((e) => console.log('[seccheck] 上传失败', e && e.message))
      } catch (e) {
        console.log('[seccheck] 异常', e && e.message)
      }
    },

    fmtDate(d) {
      const y = d.getFullYear()
      const m = String(d.getMonth() + 1).padStart(2, '0')
      const day = String(d.getDate()).padStart(2, '0')
      return `${y}-${m}-${day}`
    },
    fmtTime(d) {
      const weeks = ['日','一','二','三','四','五','六']
      const h = String(d.getHours()).padStart(2, '0')
      const min = String(d.getMinutes()).padStart(2, '0')
      return `星期${weeks[d.getDay()]}  ${h}:${min}`
    },

    /* ── 删除水印 ─────────────────────────── */
    pickRemove() {
      console.log('[camera] pickRemove 触发, state=', this.right.state)
      if (this.right.state === 'loading') return
      uni.chooseMedia({
        count: 1,
        mediaType: ['image'],
        sourceType: ['album'],
        success: (res) => {
          console.log('[camera] pickRemove chooseMedia success', JSON.stringify(res))
          const path = res.tempFiles[0] && res.tempFiles[0].tempFilePath
          if (!path) { console.log('[camera] pickRemove 未取到 path'); return }
          this.right.state = 'loading'
          this.secCheckUpload(path) // 内容安全:同上,检测留档
          uni.showLoading({ title: '正在删除水印…', mask: true })
          setTimeout(() => {
            uni.hideLoading()
            this.right.result = path
            this.right.state = 'done'
          }, 2200)
        },
        fail: (e) => { console.log('[camera] pickRemove chooseMedia fail', JSON.stringify(e)) }
      })
    },

    /* ── 保存到相册 ───────────────────────── */
    doSave(filePath) {
      if (!filePath) return
      uni.saveImageToPhotosAlbum({
        filePath,
        success: () => uni.showToast({ title: '已保存到相册', icon: 'success' }),
        fail: (e) => {
          const msg = (e && e.errMsg) || ''
          if (msg.indexOf('auth') >= 0 || msg.indexOf('deny') >= 0) {
            uni.showModal({
              title: '需要相册权限',
              content: '请在设置中允许访问相册',
              confirmText: '去设置',
              success: r => { if (r.confirm) uni.openSetting() }
            })
          } else {
            uni.showToast({ title: '保存失败', icon: 'none' })
          }
        }
      })
    }
  }
}
</script>

<style lang="scss">
/* 整体：对齐消息页的 $ground 浅色背景 */
.cam-page {
  min-height: 100vh;
  background: $ground;
  padding: 0 28rpx;
  position: relative;
}

/* 分区标题，同消息页 .divider 风格 */
.sec-hd {
  font-size: 24rpx; font-weight: 700; color: $ink-soft;
  padding: 32rpx 8rpx 20rpx;
}

/* 主卡片 */
.cam-card {
  width: 100%;
  margin: 0 0 20rpx;
  background: $card;
  border: 1rpx solid $line;
  border-radius: $r-lg;
  box-shadow: $shadow-card;
  overflow: hidden;
}
.card-banner-press { opacity: 0.88; }

/* 图像区 banner */
.card-banner {
  width: 100%; height: 320rpx;
  display: flex; align-items: center; justify-content: center;
  overflow: hidden;
}
.add-banner { background: linear-gradient(135deg, #FF8A7A, #FF6B5B); }
.rm-banner  { background: linear-gradient(135deg, #5BD0E0, #3BA9CC); }

.banner-idle {
  display: flex; flex-direction: column; align-items: center; gap: 12rpx;
}
.banner-label { font-size: 36rpx; font-weight: 800; color: #fff; }
.banner-sub   { font-size: 24rpx; color: rgba(255,255,255,0.8); }

.banner-spin {
  display: flex; flex-direction: column; align-items: center; gap: 20rpx;
}
.spinner {
  width: 52rpx; height: 52rpx; border-radius: 50%;
  border: 6rpx solid rgba(255,255,255,0.3);
  animation: wm-spin 0.85s linear infinite;
}
.add-spin { border-top-color: #FFD700; }
.rm-spin  { border-top-color: #fff; }
@keyframes wm-spin { to { transform: rotate(360deg); } }
.spin-txt { font-size: 24rpx; color: rgba(255,255,255,0.85); font-weight: 600; }

.banner-img { width: 100%; height: 100%; display: block; }

/* 底部操作栏，同 .mrow 内部行样式 */
.card-foot {
  display: flex; align-items: center; gap: 20rpx;
  padding: 24rpx 28rpx;
}
.foot-info { flex: 1; min-width: 0; }
.foot-t { font-size: 28rpx; font-weight: 700; color: $ink; display: block; }
.foot-d { font-size: 22rpx; color: $ink-soft; margin-top: 6rpx; display: block; }

/* 保存标签 */
.save-tag {
  flex-shrink: 0; height: 56rpx; line-height: 56rpx;
  padding: 0 28rpx; border-radius: $r-full;
  font-size: 24rpx; font-weight: 700; color: #fff;
}
.add-tag { background: linear-gradient(135deg, #FF8A7A, #FF6B5B); }
.rm-tag  { background: linear-gradient(135deg, #5BD0E0, #3BA9CC); }

/* 箭头 */
.arr { font-size: 40rpx; color: $ink-faint; flex-shrink: 0; }
</style>
