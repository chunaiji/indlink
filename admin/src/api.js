// 极简 fetch 封装:自动带 admin token,401 跳登录,统一解包 {code,msg,data}
import { currentLocale, gt } from './i18n'
import { tenantStore } from './tenant.js'

const BASE = (location.hostname === 'localhost' || location.hostname === '127.0.0.1')
  ? '/admin/api'
  : '/message/admin/api'

export function getToken() {
  return localStorage.getItem('admin_token') || ''
}
export function setToken(t) {
  if (t) localStorage.setItem('admin_token', t)
  else localStorage.removeItem('admin_token')
}

async function req(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const tk = getToken()
  if (tk) headers.Authorization = 'Bearer ' + tk
  if (tenantStore.currentTenantID) headers['X-Tenant-ID'] = String(tenantStore.currentTenantID)
  // 后端按此头下发错误消息与配置项标签(中间件只挂 /admin/api)
  headers['Accept-Language'] = currentLocale()
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined
  })
  if (res.status === 401) {
    setToken('')
    if (location.hash !== '#/login') location.hash = '#/login'
    throw new Error(gt('api.notLoggedIn'))
  }
  const json = await res.json().catch(() => ({ code: -1, msg: gt('api.parseFailed') }))
  if (json.code !== 0) throw new Error(json.msg || gt('api.requestFailed'))
  return json.data
}

// 图片上传(multipart,不走 JSON 封装)
async function uploadFile(file) {
  const fd = new FormData()
  fd.append('file', file)
  const headers = { 'Accept-Language': currentLocale() }
  const tk = getToken()
  if (tk) headers.Authorization = 'Bearer ' + tk
  const res = await fetch(BASE + '/upload', { method: 'POST', headers, body: fd })
  const json = await res.json().catch(() => ({ code: -1, msg: gt('api.parseFailed') }))
  if (json.code !== 0) throw new Error(json.msg || gt('api.uploadFailed'))
  return json.data // { url }
}

export const api = {
  login: (username, password) => req('POST', '/login', { username, password }),
  me: () => req('GET', '/me'),
  changePassword: (oldp, newp) => req('POST', '/password', { old_password: oldp, new_password: newp }),
  stats: () => req('GET', '/stats'),
  statsOverview: () => req('GET', '/stats/overview'), // 当期KPI+近30天趋势,租户走 X-Tenant-ID header
  getConfig: () => req('GET', '/config'),
  // clear=true 显式清空;机密项写空会被服务端当成「保持不变」
  setConfig: (key, value, clear = false) => req('PUT', '/config', { key, value: String(value), clear }),
  upload: uploadFile,
  // 机器人内容池
  getRobotContent: (type) => req('GET', '/robot/content' + (type ? '?type=' + type : '')),
  createRobotContent: (data) => req('POST', '/robot/content', data),
  updateRobotContent: (id, data) => req('PUT', '/robot/content/' + id, data),
  deleteRobotContent: (id) => req('DELETE', '/robot/content/' + id),
  // 凭证管理
  getCredentials: () => req('GET', '/credentials'),
  updateCredential: (id, data) => req('PUT', '/credentials/' + id, data),
  createCredential: (data) => req('POST', '/credentials', data),
  getProviders: (kind) => req('GET', '/providers/' + kind),
  saveProvider: (kind, provider, data) => req('PUT', `/providers/${kind}/${provider}`, data),
  probeProvider: (kind, provider) => req('POST', `/providers/${kind}/${provider}/test`),
  // 人格库
  listPersonas: () => req('GET', '/persona'),
  createPersona: (data) => req('POST', '/persona', data),
  updatePersona: (id, data) => req('PUT', '/persona/' + id, data),
  deletePersona: (id) => req('DELETE', '/persona/' + id),
  reloadPersona: () => req('POST', '/persona/reload'),
  personaBatchInfo: () => req('GET', '/persona/batch-info'),
  batchCreatePersonas: (count) => req('POST', '/persona/batch', { count }),
  // 机器人档案
  listRobotProfiles: () => req('GET', '/robot/profiles'),
  // 按语言批量新增机器人 {count, language: zh|en, gender: 0|1|2, city}
  createRobots: (body) => req('POST', '/robot/profiles', body),
  updateRobotProfile: (id, data) => req('PUT', '/robot/profiles/' + id, data),
  // 关键字规则
  listKeywordRules: () => req('GET', '/keyword-rules'),
  createKeywordRule: (data) => req('POST', '/keyword-rules', data),
  updateKeywordRule: (id, data) => req('PUT', '/keyword-rules/' + id, data),
  deleteKeywordRule: (id) => req('DELETE', '/keyword-rules/' + id),
  toggleKeywordRule: (id, status) => req('PUT', '/keyword-rules/' + id + '/toggle', { status }),
  // 回复缓存
  listReplyCache: (params) => req('GET', '/reply-cache?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  updateReplyCache: (id, data) => req('PUT', '/reply-cache/' + id, data),
  toggleReplyCache: (id, status) => req('PUT', '/reply-cache/' + id + '/toggle', { status }),
  deleteReplyCache: (id) => req('DELETE', '/reply-cache/' + id),
  // 租户
  updateTenant: (id, data) => req('PUT', `/tenants/${id}`, data),
  listTenants: () => req('GET', '/tenants'),
  createTenant: (body) => req('POST', '/tenants', body),
  // 用户管理
  listUsers: (params) => req('GET', '/users?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  listMessages: (params) => req('GET', '/messages?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  // 瓶子管理(审核)
  listBottles: (params) => req('GET', '/bottles?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  bottleReplies: (id) => req('GET', `/bottles/${id}/replies`),
  deleteBottle: (id) => req('DELETE', `/bottles/${id}`),
  deleteBottleReply: (id) => req('DELETE', `/bottle-replies/${id}`),
  // 外部接口调用日志
  listApiLogs: (params) => req('GET', '/apilogs?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  listEmailLogs: (params) => req('GET', '/email-logs?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  // 动态管理(审核)
  listMoments: (params) => req('GET', '/moments?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  momentComments: (id) => req('GET', `/moments/${id}/comments`),
  deleteMoment: (id) => req('DELETE', `/moments/${id}`),
  deleteMomentComment: (id) => req('DELETE', `/moment-comments/${id}`),
  // 会话式浏览
  listChats: (params) => req('GET', '/chats?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  chatDetail: (chatId, limit = 100) => req('GET', `/chats/${chatId}?limit=${limit}`),
  banUser: (id, ban) => req('PUT', `/users/${id}/ban`, { ban }),
  muteUser: (id, mute) => req('PUT', `/users/${id}/mute`, { mute }),
  updateUserTags: (id, tags) => req('PUT', `/users/${id}/tags`, { tags }),
  // 金币调账:delta 正加负扣,remark 记原因进流水
  adjustUserCoins: (id, delta, remark) => req('POST', `/users/${id}/coins`, { delta, remark }),
  startRobotChat: (userId, botUserIdStr) => req('POST', `/users/${userId}/robot-chat`, { bot_user_id: botUserIdStr }),
  pushUser: (id, body) => req('POST', `/users/${id}/push`, body),
  // LLM 测试
  testLLM: (prompt) => req('POST', '/llm/test', { prompt }),
  // 机器人对话（手动回复）
  listRobotChats: (page, size) => req('GET', `/robot/chats?page=${page}&size=${size}`),
  robotChatMessages: (chatId, limit) => req('GET', `/robot/chats/${chatId}/messages?limit=${limit}`),
  sendAsRobot: (chatId, content) => req('POST', `/robot/chats/${chatId}/send`, { content }),
  // WS 在线状态
  getOnlineUsers: () => req('GET', '/online'),
  // 财务流水
  listPayOrders: (params) => req('GET', '/orders?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  listWalletTxns: (params) => req('GET', '/wallet-txns?' + new URLSearchParams(Object.fromEntries(Object.entries(params).filter(([,v]) => v !== '' && v != null)))),
  // 充值档位
  listPackages: () => req('GET', '/packages'),
  createPackage: (data) => req('POST', '/packages', data),
  updatePackage: (id, data) => req('PUT', '/packages/' + id, data),
  deletePackage: (id) => req('DELETE', '/packages/' + id),
  // 道具/礼物
  listItems: () => req('GET', '/items'),
  createItem: (data) => req('POST', '/items', data),
  updateItem: (id, data) => req('PUT', '/items/' + id, data),
  deleteItem: (id) => req('DELETE', '/items/' + id),
}
