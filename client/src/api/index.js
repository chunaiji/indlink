// 按模块封装的接口集合。页面只调这里,不直接拼 url。
import { get, post } from '../utils/request'

export const bottleApi = {
  create: (data) => post('/bottle/create', data),
  scoop: (params) => get('/bottle/scoop', params),
  scoopOne: (params) => get('/bottle/scoop-one', params),
  quota: () => get('/bottle/quota'),
  mine: (params) => get('/bottle/mine', params),
  detail: (id) => get(`/bottle/${id}`),
  replies: (id) => get(`/bottle/${id}/replies`),
  reply: (id, content) => post(`/bottle/${id}/reply`, { content }),
  unlock: (rid) => post(`/bottle/reply/${rid}/unlock`),
  like: (id) => post(`/bottle/${id}/like`),
  skip: (id) => post(`/bottle/${id}/skip`),
  trace: (id) => get(`/bottle/${id}/trace`) // 漂流轨迹(仅瓶主)
}

export const matchApi = {
  cityUsers: (params) => get('/city/users', params),
  expandWall: (params) => get('/expand/wall', params)
}

export const chatApi = {
  // ID 一律按字符串传,避免 int64 在 JS 端精度丢失;source_bottle_id 无值时省略
  start: (targetId, sourceBottleId = 0) => {
    const body = { target_id: String(targetId) }
    if (sourceBottleId) body.source_bottle_id = String(sourceBottleId)
    return post('/chat/start', body)
  },
  list: (params) => get('/chat/list', params),
  unread: () => get('/chat/unread'),
  messages: (id, params) => get(`/chat/${id}/messages`, params),
  // opt 透传给 request(如 showError:false),便于调用方自行处理余额不足等业务错误
  send: (id, content, type = 'text', opt = {}) => post(`/chat/${id}/send`, { content, type }, opt),
  // 聊天内送礼:扣币+收礼方魅力值+,后端插入 gift 消息
  gift: (id, itemId, opt = {}) => post(`/chat/${id}/gift`, { item_id: itemId }, opt)
}

export const relationApi = {
  like: (targetId) => post('/relation/like', { target_id: String(targetId) }),
  stats: () => get('/relation/stats'),
  list: (params) => get('/relation/list', params),
  iViewedList: (params) => get('/relation/i-viewed', params)
}

export const walletApi = {
  balance: () => get('/wallet/balance'),
  txns: (params) => get('/wallet/txns', params)
}

export const payApi = {
  packages: () => get('/pay/packages'),
  createOrder: (packageId) => post('/pay/order', { package_id: packageId }),
  orders: (params) => get('/pay/orders', params)
}

export const itemApi = {
  list: () => get('/item/list'),
  myItems: () => get('/item/my'),
  buy: (itemId, targetId = 0) => {
    const body = { item_id: itemId }
    if (targetId) body.target_id = String(targetId)
    return post('/item/buy', body)
  }
}

export const userApi = {
  verify: () => post('/user/verify'),
  card: (id) => get(`/user/card/${id}`) // 用户资料卡(公开资料+礼物墙)
}

export const rankApi = {
  charmWeek: () => get('/rank/charm') // 魅力周榜 Top20
}

export const moderationApi = {
  report: (targetId, targetType, reason) => post('/report', { target_id: String(targetId), target_type: targetType, reason }),
  block: (targetId) => post('/block', { target_id: String(targetId) }),
  blockList: () => get('/block/list'),
  unblock: (targetId) => post('/block/remove', { target_id: String(targetId) })
}

export const collectionApi = {
  add: (targetId, targetType) => post('/collection/add', { target_id: String(targetId), target_type: targetType }),
  remove: (targetId, targetType) => post('/collection/remove', { target_id: String(targetId), target_type: targetType }),
  list: (params) => get('/collection/list', params)
}

export const notifyApi = {
  list: (params) => get('/notify/list', params),
  unread: () => get('/notify/unread'),
  read: (id) => post('/notify/read', id ? { id: String(id) } : {})
}

export const sysApi = {
  notice: () => get('/notice'),
  tabs: () => get('/tabs'),
  mineFunctions: () => get('/mine-functions'),
  hookCount: () => get('/hook-count'),
  cityConfig: () => get('/city-config'),
  pagesConfig: () => get('/pages-config'),
  features: () => get('/features') // 留存功能开关(轨迹/深夜瓶/资料卡/周榜)
}

export const pushApi = {
  templates: () => get('/push/templates'),
  subscribe: (data) => post('/push/subscribe', data)
}

export const checkinApi = {
  status: () => get('/checkin/status'),
  sign: () => post('/checkin', {}),
  makeup: () => post('/checkin/makeup', {}) // 看完激励视频补签昨天
}

export const shareApi = {
  // 分享触发后领奖;达上限/未开启返回 rewarded=false,不弹错误 toast
  reward: () => post('/share/reward', {}, { showError: false })
}

export const momentApi = {
  create: (content, visible = 'public', images = []) => post('/moment', { content, visible, images }),
  feed: (params) => get('/moment/feed', params), // 动态广场公开流
  detail: (id) => get(`/moment/${id}`), // 单条动态(通知/分享落点)
  mine: (params) => get('/moment/mine', params),
  remove: (id) => post(`/moment/${id}/remove`, {}),
  like: (id) => post(`/moment/${id}/like`, {}),
  comments: (id, params) => get(`/moment/${id}/comments`, params),
  comment: (id, content, replyTo = 0) => post(`/moment/${id}/comment`, { content, reply_to: replyTo ? String(replyTo) : '' }),
  removeComment: (cid) => post(`/moment/comment/${cid}/remove`, {}),
  gift: (id, itemId, opt = {}) => post(`/moment/${id}/gift`, { item_id: itemId }, opt) // 给动态作者送礼
}

export const geoApi = {
  regeo: (lat, lng) => get('/geo/regeo', { lat, lng }) // 逆地理编码(水印地址,服务端持 key)
}

export const adApi = {
  config: () => get('/ads'),
  // 激励视频看完领奖;达上限/未开启返回 rewarded=false,不弹错误 toast
  reward: () => post('/ad/reward', {}, { showError: false })
}
