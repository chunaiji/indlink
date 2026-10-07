# L1 设计 — 实时聊天 WebSocket(chat)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-10-07 |
| 覆盖模块 | `internal/chat/hub.go`、`service.go`、`handler.go` · `client/src/utils/ws.js` · `client/src/App.vue` |
| 关联 | L0 `l0/architecture.md` · L1 `l1/pay-wallet.md`(开聊扣币) · L2 `l2/2026-06-23-ws-reconnect-online-status.md` |

---

> **多租户(已实现)**:`chat`/`message` 落 `tenant_id`;`StartChat/SendMessage/ListChats/History` 均按 `tenant_id` 过滤会话。见 `l1/tenant-saas.md`。

## 一、整体结构

```
Client ──(WS /ws?token=JWT)── Go WebSocket Hub(单实例内存)
                                  │
发消息 POST /chat/:id/send  ──→ 落库一次 + 更新会话 + Hub.PushTo(对方)
```

- 走 `gorilla/websocket`。在线连接维护在内存 `Hub`(单实例版)。
- **消息收发用 HTTP**(`/chat/:id/send`、`/chat/:id/messages`),WS 只做**服务端 → 客户端推送**通道。这样消息可靠落库、历史可分页,WS 断连不丢消息(重连后拉历史补齐)。

## 二、Hub(在线连接管理)

```go
type Hub struct {
  mu    sync.RWMutex
  conns map[int64]map[*Conn]struct{}   // userId → 多端连接集合
}
add/remove/IsOnline(userId)
OnlineUserIDs() []int64   // 返回当前所有在线用户 ID（供管理台使用）
PushTo(userId, payload)   // 向该用户所有连接非阻塞推送
```

- 一个用户可多端在线(多个 `*Conn`)。
- `PushTo` 序列化后写各连接的 `send chan`(缓冲 32);**channel 满则丢弃**(不阻塞 Hub)——客户端可通过历史接口补齐,推送只是"尽力而为的实时性"。

## 三、连接生命周期

```
ws handler:
  token = query("token") → jwtutil.Parse(校验,失败 401)
  upgrade → Conn{userId, ws, send chan(32)}
  hub.add(conn); go writePump(); readPump()(阻塞至断开)
```

- **readPump**：读超时 **60s**；收到 pong 重置 deadline；任何读错误 → `hub.remove` + 关闭 + close(send)。
- **writePump**：**20s** 定时 `ping`；从 `send` chan 取消息 `WriteMessage`；chan 关闭则发 close frame 退出。

> 鉴权走 **query token**(`/ws?token=`)，因为小程序 WS 不便设自定义 header。

### 客户端保活（微信小程序专项）

微信小程序 WS 空闲约 25s 会被微信侧强制关闭，原服务端 30s Ping 无法在超时前到达。

**解决方案**（`client/src/utils/ws.js`）：
- onOpen 时启动 **20s 客户端心跳**：`setInterval(() => socketTask.send({event:'ping'}), 20000)`
- onClose/onError/closeWS 时清除心跳定时器

**App.vue onShow**：每次回到前台调 `ensureConnected(token, wsUrl)`，应对 WeChat 后台静默杀 WS（不触发 `onClose`）的情况。

### 重连机制（分梯次退避）

```
第 1-4 次    60s 后重试
第 5-8 次    900s (15min) 后重试
第 9-10 次   1800s (30min) 后重试
> 10 次      放弃重连
```

- `connectWS(token, wsUrl)` — 首次连接或重连执行体，有 socketTask 则跳过
- `ensureConnected(token, wsUrl)` — 回前台时调用，仅在"无 socket + 无重连计划 + 未主动关闭"时触发
- `resetAndReconnect(token, wsUrl)` — 新登录时调用，重置 retryCount 立即重连
- `closeWS()` — 登出时调用，设 `manualClosed=true` 阻止自动重连

### WS URL 动态配置

登录接口 `/api/auth/login` 响应中带 `ws_url` 字段（来自 sysconfig `ws_url` key，空则客户端使用默认值）。客户端持久化到 storage 并在每次重连时使用，允许后台无需发版修改 WS 地址。

> 鉴权走 **query token**，因为小程序 WS 不便设自定义 header。

## 四、开聊(StartChat,扣币)

```
StartChat(userId, targetId, sourceBottle):
  认证开关校验 → 拉黑校验(任一方向 block 即拒,code 4001)
  已存在会话? → 直接返回(不重复扣费)        // 幂等
  否则 wallet.Debit(userId, price_chat, scene=chat, biz) { 事务内 chat.Create }
```

- 价格 `config.price_chat`(默认 5 金币);余额不足 code 5001。
- 会话双方按 `(min,max)` 归一存 `user_a/user_b`,`findChat` 用归一键查 → 同两人只一条会话。

## 五、发消息(SendMessage)

```
校验:会话存在 + 发送者是成员 + 未被拉黑 + 文本审核(敏感词/防引流)
单事务{ Message 落库一次 + 更新 chat.last_message/updated_at }
Hub.PushTo(对方, {event:'message', chat_id, message})
```

- 历史 `/chat/:id/messages` 倒序取再翻正序返回,并把对方发来的未读标已读。

## 六、关键约束

| 项 | 说明 |
|---|---|
| 消息只落库一次 | DB 是真相源;WS 推送失败不影响持久化 |
| 推送尽力而为 | send chan 满则丢,靠历史接口补齐(对齐 L0:聊天系统要轻) |
| int64 ID | `chat_id`/`message_id`/`sender_id` 均字符串序列化(见 `l4/cross-platform-api-patterns.md`),前端 `chat.vue` 勿 `Number()` |

## 七、待办(多实例扩展点)

当前 Hub 是**单实例内存**:多实例部署时,A 连实例1、B 连实例2,`PushTo(B)` 在实例1 找不到 B 的连接 → 推送丢失。扩展方案:在 `PushTo` 之上接 **Redis Pub/Sub** 广播,各实例订阅后再投递本地连接(L0 已预留此层)。

## 八、扣费规则三键(2026-10-04)

后台「价格」分组:

| 键 | 含义 | 默认 |
|---|---|---|
| `price_chat` | 开聊扣 M | 5 |
| `price_msg` | 每条扣 N | 0(不扣) |
| `chat_free_msgs` | 每个会话**发送方**前 L 条免费 | 0 |

真扣在 `chat/service.go`:`StartChat` / `SendMessage` + `withinFreeMsgs`(按会话内该发送方的
非系统消息条数判定)。`/app-config` 的 `pricing` 段下发给 App,打招呼按钮标价与聊天页顶部提示
都从这里取(**之前写死 5**)。

⚠️ **低余额系统提示只在 N>0 时发** —— 按条不收费却弹「余额不足」,用户点了没扣钱,提示是假的。
⚠️ App 与小程序是两个租户,改规则要改 App 那个租户。

## 九、免费开聊入口(2026-10-07)

`EnsureRobotChat` 更名 **`EnsureFreeChat`**(旧名留作薄包装),成为不扣费建会话的唯一入口,
由机器人主动搭讪与主动匹配(火花)共用。**只免开聊费,`price_msg` 照常收。**
两者还共用每人每日计数键 `outreach:<租户>:<uid>:<日期>`(`ClaimDailyReach`),当天互斥。
详见 `l1/spark.md`。
