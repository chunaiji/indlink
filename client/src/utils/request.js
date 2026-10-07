// 统一请求封装:自动带 token、统一解包 {code,msg,data}、错误提示。
// 鉴权就绪门:无 token 且非登录接口的请求,先等待 silentLogin 完成再发,避免首屏竞争 2001。
import { BASE_URL } from './config'

let tokenGetter = () => uni.getStorageSync('token') || ''
let onUnauthorized = () => {}
let waitReady = null

export function setupRequest({ getToken, onAuthFail, onReady }) {
  if (getToken) tokenGetter = getToken
  if (onAuthFail) onUnauthorized = onAuthFail
  if (onReady) waitReady = onReady
}

export async function request(options) {
  const { url, method = 'GET', data, header = {}, showError = true } = options
  // 登录接口本身不等待;已有 token 也不等待(避免登录流程内自我死锁)
  if (url !== '/auth/login' && !tokenGetter() && waitReady) {
    try { await waitReady() } catch (e) {}
  }
  const token = tokenGetter()
  return new Promise((resolve, reject) => {
    uni.request({
      url: BASE_URL + url,
      method,
      data,
      timeout: 15000,
      header: {
        'Content-Type': 'application/json',
        Authorization: token ? 'Bearer ' + token : '',
        ...header
      },
      success: (res) => {
        const body = res.data || {}
        if (res.statusCode === 401 || body.code === 2001) {
          onUnauthorized()
          reject(body)
          return
        }
        if (body.code === 0) {
          resolve(body.data)
        } else {
          if (showError) uni.showToast({ title: body.msg || '请求失败', icon: 'none' })
          reject(body)
        }
      },
      fail: (err) => {
        if (showError) uni.showToast({ title: '网络异常', icon: 'none' })
        reject(err)
      }
    })
  })
}

export const get = (url, data, opt = {}) => request({ url, method: 'GET', data, ...opt })
export const post = (url, data, opt = {}) => request({ url, method: 'POST', data, ...opt })
