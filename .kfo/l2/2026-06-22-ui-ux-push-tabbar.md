# UI/UX 修复 + 自定义 TabBar + 推送模块

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-22 |
| 状态 | COMPLETED(后端已部署至 `https://ambertu.com/message`) |
| 触发 | 8 项 UI/UX 问题 + 4 项新功能需求 |
| 关联 | L0 `l0/architecture.md` · L1 `l1/chat-ws.md` · `l1/notify.md` · `l2/2026-06-22-p2-p3-features.md` |

---

## 一、8 项 UI/UX 修复

### 1. 捞瓶弹框操作区改为图标布局

`client/src/pages/ocean/ocean.vue` — `.m-actions` 内四个操作(收藏/换一个/分享/查看详情)从文字按钮改为图标+标签的 `.m-ico` 布局;分享改用 `<button open-type="share">` 触发微信分享面板。

### 2. 聊天礼物列表显示持有数量

`sendGift()` 同时调 `itemApi.list()` 和 `itemApi.myItems()`,若用户持有数量 > 0 则展示 `名称 ×N（X金币）`,否则仅展示价格。

### 3. 邀请「领回应」按钮无响应

`<view @tap="onInvite">` → `<button open-type="share">`;`uni.showShareMenu()` 只配置"…"菜单项,无法主动触发分享面板。

### 4. 道具订单记录改为 +1 展示

`pages/orders/orders.vue` 道具 Tab 将金额列改为 `+1`(获得道具),副标题改为「花费 X 金币」,视觉上与金币流水方向一致。

### 5. 编辑资料 — 头像/昵称接入微信官方 API

- 头像:改用 `<button open-type="chooseAvatar" @chooseavatar="onChooseAvatar">`,上传 `e.detail.avatarUrl` 到后端;弃用已下线的 `uni.getUserProfile()`。
- 昵称:input 加 `type="nickname"` 触发微信昵称自动填写弹框(条件编译 `#ifdef MP-WEIXIN`)。

### 6. 「完善资料」文本改为跑马灯动画

`pages/mine/mine.vue` 文本容器 `.completebar` 使用 `overflow:hidden` + `white-space:nowrap`;内部 `.marquee-inner` 使用 `@keyframes marquee-run` CSS 动画(12s linear infinite)。

### 7. 「我的」用户信息区间距调整

`.id` padding-top 由 `24rpx` 增至 `48rpx`(向下推);底部 spacer 高度由 `140rpx` 增至保留自定义 TabBar 空间。

### 8. 下拉刷新加载状态不停止

**根因:**`onPullDownRefresh` 生命周期钩子写在 `methods: {}` 内 — uni-app 不识别,导致钩子永不触发、`uni.stopPullDownRefresh()` 永不调用。

**修复:**将 `onPullDownRefresh` 从 `methods` 移出到组件 options 顶层(与 `onShow`/`onLoad` 并列)。涉及文件:`city.vue`、`expand.vue`、`message.vue`。

---

## 二、4 项新功能

### 1. 聊天消息列表显示双方头像和昵称

`pages/chat/chat.vue`:
- 对方消息左侧显示 `<user-avatar>` + 昵称(`partnerNickname/partnerAvatar`);我方消息右侧显示自己头像 + 昵称。
- 头像/昵称从 URL 参数 `q.name`/`q.avatar`(编码)读入;`message.vue` 的 `openChat()` 跳转时写入。

### 2. 首页公告弹框(后台维护内容)

- `pages.json`:「海洋」Tab 重命名为「首页」。
- `ocean.vue`:进入首页调 `GET /api/notice` 取公告标题/正文;用户点「好的我知道了」后写 `localStorage` 不再弹出(同版本内)。
- 后端 `sysconfig`:新增 `notice_title`/`notice_body` 配置键,管理台可热改。

### 3. 自定义 TabBar — 5 个 Tab 可后台配置显示/隐藏

- `pages.json`:加 `"custom": true` 启用自定义 tabBar。
- 新增组件 `components/tab-bar/tab-bar.vue`:从 `GET /api/tabs` 拉取显示配置(每次 onShow 拉取,结果缓存 localStorage);5 个 Tab(首页/同城/扩列/消息/我的)按配置过滤渲染。
- 所有 Tab 页各自引用 `<tab-bar :current="N" />`;scroll-view 高度相应缩小以避免被固定 TabBar 遮挡。
- 后端 `sysconfig`:新增 `tab_show_ocean`/`tab_show_city`/`tab_show_expand`/`tab_show_message`/`tab_show_mine` 五个布尔配置键,默认全 true。

### 4. 微信订阅消息推送模块

**后端 `internal/push/`(NEW):**

- `model.PushSubscription`(table `push_subscriptions`):记录用户订阅的模板 ID 与场景。
- `service.go`:
  - `Subscribe()` — upsert 订阅记录
  - `GetTemplates()` — 从 sysconfig 读推送模板 ID
  - `Send(scene, userID, data)` — 异步 goroutine 逐条调微信 `subscribeMessage.send` API
  - `getAccessToken()` — mutex 保护的 access_token 缓存(7200s 有效期,提前 60s 刷新)
- `handler.go`:
  - `GET /api/push/templates`(无需登录) — 返回订阅所需模板 ID 列表
  - `POST /api/push/subscribe`(需登录) — 前端用户授权后保存订阅

**前端 `ocean.vue`:**
- 首次进入首页时调 `pushApi.templates()` + `uni.requestSubscribeMessage()` 触发授权弹框,成功后调 `pushApi.subscribe()` 存服务端。

**sysconfig 新键:**`push_tpl_reply`/`push_tpl_chat`/`wx_appid`/`wx_secret`。

---

## 三、后端部署

```
服务器: 43.136.54.189
部署路径: /usr/jack/deploy/go_workspace/
生产地址: https://ambertu.com/message
部署命令: node deploy.js (根目录)
编译命令: cd server && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o driftbottle-linux ./cmd/api/main.go
```

---

## 四、关键踩坑

| 坑 | 原因 | 修复 |
|---|---|---|
| `onPullDownRefresh` 永不触发 | 写在 `methods:{}` 内,uni-app 只识别 options 顶层生命周期钩子 | 移出 methods |
| 邀请按钮无响应 | `uni.showShareMenu()` 只配置"…"菜单,不能主动触发分享面板 | 改用 `<button open-type="share">` |
| 头像 API 不可用 | `uni.getUserProfile()` 已于 2023-04 被微信下线 | 改用 `open-type="chooseAvatar"` |
| Linux 二进制 Exec format error | Bash 内用 `set GOOS=linux`(Windows CMD 语法),实际编译出 Windows PE32+ | 改用 Bash inline env:`GOOS=linux GOARCH=amd64 go build` |
