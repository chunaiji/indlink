import { defineStore } from 'pinia'
import { getLoginCode, getAppId } from '../utils/platform'
import { currentPlatform } from '../utils/config'
import { post, get, setupRequest } from '../utils/request'
import { connectWS, closeWS, resetAndReconnect } from '../utils/ws'

// 登录就绪门:silentLogin 完成(无论成败)后 resolve,放行被挂起的鉴权请求。
// 保险:即使 silentLogin 卡住(如后端不可达),最多 8s 也强制放行,避免请求/页面永久挂起。
let _readyResolve
const _ready = new Promise((r) => { _readyResolve = r })
let _readyDone = false
const _markReady = () => { if (!_readyDone) { _readyDone = true; _readyResolve() } }
setTimeout(_markReady, 8000)

// 逗号分隔的快捷回复字符串 → 数组;空/无效则回退默认。
function parseQuicks(s, fallback) {
  if (!s) return fallback
  const arr = String(s).split(',').map((x) => x.trim()).filter(Boolean)
  return arr.length ? arr : fallback
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: uni.getStorageSync('token') || '',
    wsUrl: uni.getStorageSync('wsUrl') || '',
    profile: null,
    iosRechargeOff: true,
    chatPrice: 5,
    shareTitle: '我在漂流瓶捞到一句话,你也来看看~',
    shareImage: '',
    logging: false,
    pushSubscribePrompt: false,
    // UI 文案(后台可配,登录时下发)
    uiText: {
      anonSender: '匿名漂流瓶',
      anonFriend: '海上的朋友',
      someFriend: '某位朋友',
      navTitle: '漂流瓶',
      chatBanner: '缘起一只漂流瓶 · 友善聊天',
      quotaTitle: '次数已用完',
      quotaMsg: '可前往道具商城购买次数包继续,或明天免费次数恢复。',
    },
    // 快捷回复(后台可配,登录时下发)
    chatQuicks: ['我懂你', '我也经历过', '说说细节?', '继续聊'],
    replyQuicks: ['我懂你', '我也经历过', '说说细节?'],
    // 余额不足系统消息(后台可配,前端仅参考)
    lowBalanceThreshold: 0,
    lowBalanceMsg: ''
  }),
  getters: {
    isLogin: (s) => !!s.token,
    userId: (s) => (s.profile ? s.profile.user_id : 0)
  },
  actions: {
    init() {
      setupRequest({
        getToken: () => this.token || uni.getStorageSync('token') || '',
        onAuthFail: () => {
          this.token = ''
          uni.removeStorageSync('token')
        },
        onReady: () => _ready
      })
    },
    // 静默登录:已有 token 拉资料;否则走 code 登录。完成后放行就绪门。
    async silentLogin() {
      this.init()
      try {
        if (this.token) {
          try {
            await this.fetchProfile()
            connectWS(this.token, this.wsUrl)
            return
          } catch (e) { /* token 失效则重新登录 */ }
        }
        await this.login()
      } finally {
        _markReady()
      }
    },
    // 应用后端下发的客户端配置(登录/拉资料共用),老用户经 profile 也能拿到最新配置。
    applyClientConfig(res) {
      if (!res) return
      this.iosRechargeOff = !!res.ios_recharge_off
      this.pushSubscribePrompt = !!res.push_subscribe_prompt
      if (res.price_chat != null) this.chatPrice = res.price_chat
      if (res.share_title) this.shareTitle = res.share_title
      if (res.share_image != null) this.shareImage = res.share_image || ''
      if (res.ws_url) { this.wsUrl = res.ws_url; uni.setStorageSync('wsUrl', res.ws_url) }
      if (res.ui_text_anon_sender) this.uiText.anonSender = res.ui_text_anon_sender
      if (res.ui_text_anon_friend) this.uiText.anonFriend = res.ui_text_anon_friend
      if (res.ui_text_some_friend) this.uiText.someFriend = res.ui_text_some_friend
      if (res.ui_text_nav_title) this.uiText.navTitle = res.ui_text_nav_title
      if (res.ui_text_chat_banner) this.uiText.chatBanner = res.ui_text_chat_banner
      if (res.ui_text_quota_title) this.uiText.quotaTitle = res.ui_text_quota_title
      if (res.ui_text_quota_msg) this.uiText.quotaMsg = res.ui_text_quota_msg
      this.chatQuicks = parseQuicks(res.chat_quicks, this.chatQuicks)
      this.replyQuicks = parseQuicks(res.reply_quicks, this.replyQuicks)
      if (res.low_balance_threshold != null) this.lowBalanceThreshold = res.low_balance_threshold
      if (res.low_balance_msg != null) this.lowBalanceMsg = res.low_balance_msg
    },
    async login() {
      if (this.logging) return
      this.logging = true
      try {
        const code = await getLoginCode()
        // 多租户:带上本小程序 appid;单租户后端忽略此字段
        const res = await post('/auth/login', { platform: currentPlatform(), appid: getAppId(), code })
        this.token = res.token
        this.profile = res.user
        this.applyClientConfig(res)
        uni.setStorageSync('token', res.token)
        // 新用户:打标记,首次进首页时引导完善资料
        if (res.is_new) uni.setStorageSync('needProfile', 1)
        this.init()
        resetAndReconnect(this.token, this.wsUrl) // 新登录，重置重连计数并立即连接
      } finally {
        this.logging = false
      }
    },
    async fetchProfile() {
      const res = await get('/user/profile')
      this.profile = res.user ?? res
      this.applyClientConfig(res)
      return this.profile
    },
    async updateProfile(data) {
      await post('/user/update', data)
      await this.fetchProfile()
    },
    logout() {
      this.token = ''
      this.profile = null
      uni.removeStorageSync('token')
      closeWS()
    }
  }
})
