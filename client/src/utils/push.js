import { pushApi } from '../api/index'

/**
 * 向微信请求订阅消息授权并上报已授权的模板。
 * templates 来自 GET /push/templates，格式 [{id, scene}, ...]。
 * 每天只弹一次（local storage 记录日期）。
 */
export async function trySubscribePush(templates) {
  if (!templates || templates.length === 0) return

  // 今天已经弹过了，跳过
  const today = new Date().toISOString().slice(0, 10) // "2026-06-24"
  if (uni.getStorageSync('push_subscribed_date') === today) return

  // #ifdef MP-WEIXIN
  const tmplIds = templates.map(t => t.id)
  await new Promise(resolve => {
    wx.requestSubscribeMessage({
      tmplIds,
      success(res) {
        const accepted = templates.filter(t => res[t.id] === 'accept')
        if (accepted.length === 0) { resolve(); return }
        Promise.all(
          accepted.map(t => pushApi.subscribe({ template_id: t.id, scene: t.scene }).catch(() => {}))
        ).finally(resolve)
      },
      fail: resolve
    })
  })
  // #endif

  uni.setStorageSync('push_subscribed_date', today)
}
