<script>
import { useUserStore } from './store/user'
import { useAdsStore } from './store/ads'
import { initTheme } from './utils/theme'
import { ensureConnected } from './utils/ws'

export default {
  onLaunch() {
    // 启动即按本地时间选主题(白天/夜间),并尝试静默登录
    try { initTheme() } catch (e) {}
    try { useAdsStore().fetchConfig() } catch (e) {}
    try {
      const user = useUserStore()
      // 不 await,且吞掉异常,绝不让启动流程因登录失败而中断
      Promise.resolve(user.silentLogin()).catch(() => {})
    } catch (e) {}
    // 预拉广告配置(失败静默)
    try { useAdsStore().fetchConfig() } catch (e) {}
    // 微信隐私授权框架（2023年9月起强制）：注册回调，否则 chooseAvatar 等 API 静默失败
    // #ifdef MP-WEIXIN
    if (typeof wx !== 'undefined' && wx.onNeedPrivacyAuthorization) {
      wx.onNeedPrivacyAuthorization((resolve) => {
        uni.showModal({
          title: '用户隐私保护提示',
          content: '我们需要获取你的头像信息用于个人资料设置，请阅读并同意《隐私保护指引》。',
          confirmText: '同意',
          cancelText: '暂不使用',
          success(res) {
            if (res.confirm) {
              wx.requirePrivacyAuthorize({
                success: () => resolve({ event: 'agree', buttonId: 'agree-btn' }),
                fail: () => resolve({ event: 'disagree' })
              })
            } else {
              resolve({ event: 'disagree' })
            }
          }
        })
      })
    }
    // #endif
  },
  onShow() {
    console.log('[App] onShow')
    try {
      const user = useUserStore()
      console.log('[App] onShow token:', user.token ? user.token.slice(0, 8) + '...' : 'none')
      if (user.token) ensureConnected(user.token, user.wsUrl)
    } catch (e) { console.error('[App] onShow error', e) }
  },
  onError(err) {
    // 把运行时错误显形,便于联调定位(而非静默失效)
    console.error('[AppError]', err)
  }
}
</script>

<style>
/* 全局基础样式 */
page {
  background-color: #F4F8F9;
  color: #0C2A33;
  font-family: -apple-system, 'SF Pro Text', 'PingFang SC', 'HarmonyOS Sans SC', 'Microsoft YaHei', sans-serif;
  font-size: 28rpx;
  line-height: 1.5;
}
/* 夜间主题:页面加 dark 类时覆盖 */
.theme-dark page,
page.theme-dark {
  background-color: #070E1C;
  color: #EAF2FF;
}
</style>
