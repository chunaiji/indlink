# WS 重连机制重设计 + 管理台在线状态

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-23 |
| 状态 | COMPLETED |
| 触发 | WS 频繁断线、重连失效；管理台无法感知用户在线状态 |
| 关联 | L1 `l1/chat-ws.md` · `l1/admin-platform.md` · `l1/user-auth.md` |

---

## 根本原因诊断

### WS 连接只持续 20-27 秒

GIN 日志可见连接每次约 20-27s 就关闭，随后等待约 60s 才重连。

**原因链：**
1. 服务端 `writePump` Ping 间隔 = 30s
2. 微信小程序 WS 空闲超时 ≈ 25s（无流量时微信侧强制关闭）
3. 连接在第一个 Ping 到达之前就被微信关闭
4. `onClose` 触发 → `scheduleReconnect` 等 60s 再重连
5. 每 80s 周期里用户只有 ~25s 在线，管理台轮询大概率落在离线窗口

### App.vue 无 onShow

WeChat 将小程序切入后台时会 OS 级静默杀掉 WS，不触发 `onClose`。导致 `socketTask` 仍非 null，`ensureConnected` 检查时误判为"已连接"，永不重连。

### 管理台 /admin/api/online 始终返回空

`admin.Service` 有 `chatSvc *chat.Service` 字段，但 `SetChatService(chatSvc)` 虽已在 `main.go` 中调用，`/online` 接口之前未实现，后来补充后一并部署。

---

## 改动清单

### 1. WS 分梯次重连机制（client/src/utils/ws.js 完整重写）

```
重试次数   延迟
1-4 次    60s
5-8 次    900s (15min)
9-10 次   1800s (30min)
>10 次    停止重连
```

新增模块级状态：`retryCount`、`currentToken`、`currentUrl`、`RETRY_DELAYS`。

**新增函数：**
- `ensureConnected(token, wsUrl)` — 回前台时调用，有 socketTask/reconnectTimer/manualClosed 则跳过
- `resetAndReconnect(token, wsUrl)` — 新登录时调用，重置 retryCount 立即重连

**onOpen** 重置 retryCount；**onClose/onError** 触发 `scheduleReconnect`（含 retryCount 上限判断）。

### 2. WS URL 由后台配置下发（server + client）

**服务端：**
- `server/internal/sysconfig/sysconfig.go`：新增常量 `KeyWSURL = "ws_url"`，默认值 `""`
- `server/internal/admin/meta.go`：configMeta 新增 `{Key:"ws_url", Label:"小程序 WebSocket 地址(空=客户端默认)", Group:"通用", Type:"text"}`
- `server/internal/user/handler.go`：login 响应新增字段 `"ws_url": sysconfig.GetString(sysconfig.KeyWSURL)`

**客户端：**
- `client/src/store/user.js`：state 新增 `wsUrl: uni.getStorageSync('wsUrl') || ''`；login 时若 `res.ws_url` 非空则持久化；`silentLogin` 传 `wsUrl` 给 `connectWS`；login 调 `resetAndReconnect`

### 3. App.vue 新增 onShow 钩子

```js
onShow() {
  // WeChat 后台切换可能静默断开 WS（不触发 onClose），回前台时主动检查
  const user = useUserStore()
  if (user.token) ensureConnected(user.token, user.wsUrl)
}
```

### 4. 客户端 20s 心跳（client/src/utils/ws.js）

onOpen 时启动 `setInterval(20s)`，每次发送 `{event:"ping"}`，防止微信 25s 空闲超时。onClose/onError/closeWS 时清除定时器。

### 5. 服务端 Ping/ReadDeadline 调整（server/internal/chat/handler.go）

| 参数 | 旧值 | 新值 |
|------|------|------|
| writePump Ping 间隔 | 30s | 20s |
| readPump ReadDeadline | 90s | 60s |

原来 30s Ping 比微信 25s 超时晚，连接总在 Ping 前就被关闭。现在 20s Ping < 25s 超时，服务端 Ping 可作为兜底。客户端 20s 心跳是主要保活手段。

### 6. Hub.OnlineUserIDs() + 管理台在线状态

**服务端：**
- `server/internal/chat/hub.go`：新增 `OnlineUserIDs() []int64` — 遍历 `h.conns` 返回所有在线 userID
- `server/internal/admin/service.go`：新增 `chatSvc *chat.Service` 字段 + `SetChatService(svc)` 方法
- `server/internal/admin/handler.go`：新增路由 `GET /admin/api/online`，调 `chatSvc.Hub().OnlineUserIDs()`
- `server/cmd/api/main.go`：在 `admin.New` 后调 `adminSvc.SetChatService(chatSvc)`

**Admin 前端：**
- `admin/src/api.js`：新增 `getOnlineUsers: () => req('GET', '/online')`
- `admin/src/views/RobotChats.vue`：`onlineSet`(Set of string userID) + `loadOnline()` 每 8s 轮询 + WS 状态绿/灰圆点 + "● WS 在线 / ○ WS 离线" badge

### 7. removeTabBarBadge 错误修复

`client/src/pages/message/message.vue` 中移除 `uni.setTabBarBadge` / `uni.removeTabBarBadge` 调用。原因：自定义 TabBar 组件不兼容原生 Badge API（报 `removeTabBarBadge:fail custom Tabbar`）。Tab 红点已由 `tabBadge` computed prop 驱动自定义组件。

### 8. WS 调试日志

`client/src/utils/ws.js` 全面加 `console.log`：连接 URL（token 截断）、onOpen、onClose（带 code/reason）、onError、心跳发送、scheduleReconnect（显示 attempt/delay）、ensureConnected state dump。`App.vue onShow` 也加日志。

---

## 验证要点

1. 微信开发者工具 Console 可见 `[WS] ✅ onOpen` → `[WS] ❤️ heartbeat sent`（每 20s）
2. 管理台 `/robot-chats` 打开小程序时用户旁出现绿点「● WS 在线」
3. 小程序切后台再切回，Console 出现 `[App] onShow` 并自动重连（若连接已断）
4. 连续断线 10 次后不再重连（Console 出现 `max retries reached`）
5. 后台修改 `ws_url` 配置后，重新登录小程序即使用新地址
