import { useAdsStore } from '../store/ads'
import { adApi } from '../api/index'
import { useWalletStore } from '../store/wallet'

// 插屏:受 interGapSec 节流;激励视频:看完(isEnded)回调后端领奖。
let _rewarded = null

export function showInterstitial(slotKey) {
  const ads = useAdsStore()
  if (!ads.canShow(slotKey)) return
  const last = uni.getStorageSync('ad_inter_last') || 0
  if (Date.now() - last < (ads.interGapSec || 180) * 1000) return
  // #ifdef MP-WEIXIN
  try {
    const adObj = wx.createInterstitialAd({ adUnitId: ads.slot(slotKey).unit })
    adObj.onError(() => {})
    adObj.show().catch(() => adObj.load().then(() => adObj.show()).catch(() => {}))
    uni.setStorageSync('ad_inter_last', Date.now())
  } catch (e) {}
  // #endif
}

// 本次展示的自定义"看完"回调(补签等场景);为空走默认领币逻辑
let _onEnded = null

export function showRewarded(slotKey, onEnded = null) {
  const ads = useAdsStore()
  if (!ads.canShow(slotKey)) { uni.showToast({ title: '暂不可用', icon: 'none' }); return }
  // #ifdef MP-WEIXIN
  try {
    _onEnded = onEnded
    if (!_rewarded) {
      _rewarded = wx.createRewardedVideoAd({ adUnitId: ads.slot(slotKey).unit })
      _rewarded.onError(() => {})
      _rewarded.onClose((res) => {
        if (res && res.isEnded) {
          if (_onEnded) { const cb = _onEnded; _onEnded = null; cb(); return }
          adApi.reward().then((r) => {
            if (r && r.rewarded) {
              uni.showToast({ title: `+${r.coins} 金币`, icon: 'none' })
              useWalletStore().fetchBalance()
            } else {
              uni.showToast({ title: '今日已领完', icon: 'none' })
            }
          }).catch(() => {})
        } else {
          _onEnded = null // 未看完不触发,清掉本次回调
        }
      })
    }
    _rewarded.show().catch(() => _rewarded.load().then(() => _rewarded.show()).catch(() => {}))
  } catch (e) {}
  // #endif
}
