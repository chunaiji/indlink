// 后端地址配置。开发期指向本地,生产由打包环境替换。
// 用条件编译给变量赋值(而非重复声明),uni 编译时按平台保留对应分支。
// let _base = 'http://127.0.0.1:8980/api'
// let _ws = 'ws://127.0.0.1:8980/ws'
// let _platform = 'wx'

// // #ifdef MP-WEIXIN
// _base = 'http://localhost:8980/api'
// _ws = 'ws://localhost:8980/ws'
// _platform = 'wx'
// // #endif

// // #ifdef MP-ALIPAY
// _base = 'http://localhost:8980/api'
// _ws = 'ws://localhost:8980/ws'
// _platform = 'alipay'
// // #endif

// export const BASE_URL = _base
// export const WS_URL = _ws

// // 当前运行平台标识(传给后端 /auth/login)
// export function currentPlatform() {
//   return _platform
// }


let _base = 'https://ambertu.com/message/api'
let _ws = 'wss://ambertu.com/message/ws'
let _platform = 'wx'

// #ifdef MP-WEIXIN
_base = 'https://ambertu.com/message/api'
_ws = 'wss://ambertu.com/message/ws'
_platform = 'wx'
// #endif

// #ifdef MP-ALIPAY
_base = 'https://ambertu.com/message/api'
_ws = 'wss://ambertu.com/message/ws'
_platform = 'alipay'
// #endif

export const BASE_URL = _base
export const WS_URL = _ws

// 当前运行平台标识(传给后端 /auth/login)
export function currentPlatform() {
  return _platform
}
