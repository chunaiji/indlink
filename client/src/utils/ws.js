// WebSocket 封装：连接、分梯次自动重连、消息分发。token 走 query，url 由登录接口下发。
import { WS_URL } from './config'

// 重连延迟表（秒）：1-4 次 60s，5-8 次 900s（15min），9-10 次 1800s（30min）
const RETRY_DELAYS = [60, 60, 60, 60, 900, 900, 900, 900, 1800, 1800]
const MAX_RETRIES = RETRY_DELAYS.length // 10 次后停止

let socketTask = null
let listeners = []
let reconnectListeners = []
let reconnectTimer = null
let heartbeatTimer = null
let manualClosed = false
let _connected = false
let retryCount = 0
let currentToken = null
let currentUrl = null

function startHeartbeat() {
  stopHeartbeat()
  heartbeatTimer = setInterval(() => {
    if (socketTask) {
      try {
        socketTask.send({ data: JSON.stringify({ event: 'ping' }) })
        console.log('[WS] ❤️ heartbeat sent')
      } catch (e) {
        console.warn('[WS] heartbeat send error', e)
      }
    }
  }, 20000)
}

function stopHeartbeat() {
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
}

export function connectWS(token, wsUrl) {
  manualClosed = false
  if (!token) { console.warn('[WS] connectWS: no token, skip'); return }
  if (socketTask) { console.log('[WS] connectWS: already has socketTask, skip'); return }
  currentToken = token
  currentUrl = wsUrl || WS_URL
  const url = currentUrl + '?token=' + token.slice(0, 8) + '...'
  console.log('[WS] connecting →', url, '| retryCount:', retryCount)
  socketTask = uni.connectSocket({
    url: currentUrl + '?token=' + token,
    complete: () => {}
  })
  socketTask.onOpen(() => {
    console.log('[WS] ✅ onOpen — connected')
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
    if (_connected) {
      console.log('[WS] reconnect success, notifying listeners')
      reconnectListeners.forEach((fn) => fn())
    }
    _connected = true
    retryCount = 0
    startHeartbeat()
  })
  socketTask.onMessage((res) => {
    let payload = res.data
    try { payload = JSON.parse(res.data) } catch (e) {}
    console.log('[WS] onMessage event:', payload && payload.event, payload)
    if (payload && payload.event === 'message') {
      try { uni.vibrateShort({ type: 'light' }) } catch (e) {}
    }
    listeners.forEach((fn) => fn(payload))
  })
  socketTask.onClose((res) => {
    console.log('[WS] ❌ onClose — code:', res && res.code, 'reason:', res && res.reason, '| manualClosed:', manualClosed)
    stopHeartbeat()
    socketTask = null
    if (!manualClosed) scheduleReconnect()
  })
  socketTask.onError((res) => {
    console.error('[WS] onError', res)
    stopHeartbeat()
    socketTask = null
    if (!manualClosed) scheduleReconnect()
  })
}

function scheduleReconnect() {
  if (reconnectTimer) { console.log('[WS] scheduleReconnect: timer already set, skip'); return }
  if (retryCount >= MAX_RETRIES) { console.warn('[WS] max retries reached, give up'); return }
  const delay = RETRY_DELAYS[retryCount] * 1000
  retryCount++
  console.log('[WS] scheduleReconnect — attempt', retryCount, '/ delay', delay / 1000, 's')
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectWS(currentToken, currentUrl)
  }, delay)
}

export function onWSMessage(fn) {
  listeners.push(fn)
  return () => { listeners = listeners.filter((f) => f !== fn) }
}

// 重连成功后触发（首次连接不触发）
export function onWSReconnect(fn) {
  reconnectListeners.push(fn)
  return () => { reconnectListeners = reconnectListeners.filter((f) => f !== fn) }
}

export function ensureConnected(token, wsUrl) {
  const state = { socketTask: !!socketTask, reconnectTimer: !!reconnectTimer, manualClosed, retryCount }
  console.log('[WS] ensureConnected state:', JSON.stringify(state))
  if (!token || socketTask || reconnectTimer || manualClosed) return
  if (retryCount >= MAX_RETRIES) { console.warn('[WS] ensureConnected: max retries, skip'); return }
  connectWS(token, wsUrl)
}

export function resetAndReconnect(token, wsUrl) {
  console.log('[WS] resetAndReconnect — reset retryCount, force reconnect')
  retryCount = 0
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (socketTask) { socketTask.close(); socketTask = null }
  connectWS(token, wsUrl)
}

export function closeWS() {
  console.log('[WS] closeWS — manual close')
  manualClosed = true
  _connected = false
  retryCount = 0
  stopHeartbeat()
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (socketTask) socketTask.close()
  socketTask = null
}
