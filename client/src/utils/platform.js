// 平台差异封装层:登录、支付、系统信息。
// 业务代码只调这里的函数,永不直接出现 wx.* / my.*。

// 获取登录 code(微信 wx.login / 支付宝 my.getAuthCode)。
export function getLoginCode() {
  return new Promise((resolve, reject) => {
    // #ifdef MP-WEIXIN
    uni.login({
      provider: 'weixin',
      success: (res) => resolve(res.code),
      fail: reject
    })
    return
    // #endif
    // #ifdef MP-ALIPAY
    my.getAuthCode({
      scopes: 'auth_base',
      success: (res) => resolve(res.authCode),
      fail: reject
    })
    return
    // #endif
    // #ifndef MP-WEIXIN || MP-ALIPAY
    // H5/开发态:用时间戳模拟 code
    resolve('devcode-' + Date.now())
    // #endif
  })
}

// 拉起支付。params 为后端返回的 pay_params。
export function requestPay(params) {
  return new Promise((resolve, reject) => {
    // 开发态 mock:直接成功(本地联调时后端 mock driver 配合)
    if (params && params.mock) {
      resolve({ mock: true })
      return
    }
    // #ifdef MP-WEIXIN
    uni.requestPayment({
      provider: 'wxpay',
      timeStamp: params.timeStamp,
      nonceStr: params.nonceStr,
      package: params.package,
      signType: params.signType || 'RSA',
      paySign: params.paySign,
      success: resolve,
      fail: reject
    })
    return
    // #endif
    // #ifdef MP-ALIPAY
    my.tradePay({
      tradeNO: params.tradeNO,
      success: (res) => {
        // 9000 表示支付成功
        if (res.resultCode === '9000') resolve(res)
        else reject(res)
      },
      fail: reject
    })
    return
    // #endif
    // #ifndef MP-WEIXIN || MP-ALIPAY
    resolve({ mock: true })
    // #endif
  })
}

// 获取当前小程序自身 appid(多租户登录用,前端运行时自取)。
export function getAppId() {
  // #ifdef MP-WEIXIN
  try {
    const info = uni.getAccountInfoSync()
    return (info && info.miniProgram && info.miniProgram.appId) || ''
  } catch (e) { return '' }
  // #endif
  // #ifdef MP-ALIPAY
  try {
    return (my.getAppIdSync && my.getAppIdSync().appId) || ''
  } catch (e) { return '' }
  // #endif
  // #ifndef MP-WEIXIN || MP-ALIPAY
  return ''
  // #endif
}

// 获取定位(会触发系统授权弹窗)。resolve({latitude, longitude}),拒绝则引导去设置。
export function getLocationOnce() {
  return new Promise((resolve, reject) => {
    uni.getLocation({
      type: 'gcj02',
      success: (res) => resolve({ latitude: res.latitude, longitude: res.longitude }),
      fail: (err) => {
        uni.showModal({
          title: '需要位置权限',
          content: '开启位置权限后才能看到附近的人',
          confirmText: '去设置',
          success: (r) => { if (r.confirm) uni.openSetting() }
        })
        reject(err)
      }
    })
  })
}

// 系统信息(uni 已抹平),返回 { platform: 'ios'|'android'|..., ... }
export function getSystemInfo() {
  return uni.getSystemInfoSync()
}

// 是否为 iOS 设备(用于充值入口合规控制)
export function isIOS() {
  const info = getSystemInfo()
  return (info.platform || '').toLowerCase() === 'ios'
}
