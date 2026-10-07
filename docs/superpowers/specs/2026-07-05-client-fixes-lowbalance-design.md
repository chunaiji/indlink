# 小程序端修复(捞瓶/快捷回复/聊天横幅) + 余额不足系统消息 — 设计文档

- 日期：2026-07-05
- 分支：main
- 范围：4 项（2 bug + 2 功能），均围绕客户端 + 后台配置：
  1. #1 首页点浮动瓶子绕过捞瓶配额（bug）
  2. #2 后台快捷回复/UI文案对老用户不生效（bug，根因在 profile 接口）
  3. #3 聊天页"缘起一只漂流瓶 · 友善聊天"写死 → 可配
  4. #4 余额 < N 时聊天页自动推后台可配的系统消息（新功能）

## 0. 现状与根因（已核实）

- **#1**：`client/src/pages/ocean/ocean.vue`——`loadFloating()` 用 `bottleApi.scoop`（批量预览、**不计次**）填充浮动瓶；模板 `@tap="openBottle(b)"`→`showBottle` 直接展示，**绕过配额**。「捞瓶子」按钮走 `scoop()`→`scoopOne`（计次+去重+3003）。
- **#2**：`server/internal/user/handler.go` **登录响应**下发 `chat_quicks/reply_quicks` 等；但 `store/user.js silentLogin()` 对老用户走 `fetchProfile()`（GET `/user/profile`）即 return，而该接口**不返回**这些字段、客户端 `fetchProfile` 也不读 → 老用户永远拿不到后台配置。已确认租户100 `chat_quicks` 改过但线上老用户仍显示写死默认。`reply_quicks` 同理。
- **#3**：`client/src/pages/chat/chat.vue:6` 文案写死。
- **#4**：聊天消息模型有 `type`（text/image/gift/system…）；`chat.vue` 目前只渲染 text/image 气泡，无 system 渲染。

## 1. #1 点瓶子=捞一次

- `ocean.vue`：把浮动瓶点击从"免费预览"改为**与捞瓶按钮同一动作**：
  - `openBottle(b)` 改为调用 `this.scoop()`（即 `scoopOne`：计入每日次数 + 去重 + 配额 0 时弹「次数用完」引导购买）。被点的具体瓶子内容不再直接展示，统一由 scoopOne 返回一条（"点击=捞一次"）。
  - `openShared(id)`（分享链接进入）保持免费展示，不受影响（不同意图）。
- 结果：两个入口配额一致，无法绕过。

## 2. #2 后台配置对老用户生效（profile 补齐配置块）

### 后端
- 在 `server/internal/user` 抽一个共享函数 `clientConfig(tenantID int64) gin.H`，聚合所有客户端要用的 sysconfig 字段：`ios_recharge_off / push_subscribe_prompt / price_chat / share_title / share_image / ws_url / ui_text_anon_sender / ui_text_anon_friend / ui_text_some_friend / ui_text_nav_title / ui_text_chat_banner(新, 见#3) / chat_quicks / reply_quicks / low_balance_threshold(新,见#4) / low_balance_msg(新,见#4)`。
- **登录响应**与 **`/user/profile` 响应**都并入 `clientConfig(u.TenantID)`（登录用它替换现有内联字段，避免两处漂移）。profile 响应保持 `{"user": u, ...clientConfig}`。

### 客户端
- `store/user.js` 抽 `applyClientConfig(res)`：集中设置 `chatQuicks/replyQuicks/uiText.*/chatPrice/shareTitle/shareImage/wsUrl/iosRechargeOff/pushSubscribePrompt/lowBalance*`（沿用现有 parseQuicks 等）。
- `login()` 与 `fetchProfile()` **都调用** `applyClientConfig(res)`，老用户带 token 走 profile 也能拿到最新配置。
- `fetchProfile` 目前 `res.user ?? res`：后端 profile 返回顶层带 user + 配置字段，客户端 `applyClientConfig(res)` 读顶层字段。

## 3. #3 聊天横幅可配

- 新配置键 `KeyUITextChatBanner = "ui_text_chat_banner"`，默认 `"缘起一只漂流瓶 · 友善聊天"`。
- 并入 `clientConfig` 块（#2）；client `uiText.chatBanner` 默认同值，`applyClientConfig` 从 `res.ui_text_chat_banner` 设置。
- `chat.vue`：`data`/`computed` 增加 `chatBanner`（读 `useUserStore().uiText.chatBanner`），模板 `<view class="day">{{ chatBanner }}</view>`。
- admin `meta.go` 加 ConfigField（分组「UI 文案」）。

## 4. #4 余额不足系统消息

### 配置键（新）
- `KeyLowBalanceThreshold = "low_balance_threshold"`，默认 `"10"`（金币；`0`=关闭该功能）。
- `KeyLowBalanceMsg = "low_balance_msg"`，默认 `"余额不足啦,充值后就能继续畅聊咯~"`。
- 两者并入 `clientConfig` 块下发（前端暂不用，但便于后台统一管理；核心逻辑在后端）。admin `meta.go` 加两条 ConfigField（分组「聊天」）。

### 触发与频率
- 触发点：真人在聊天页**发送消息**、后端扣费后检查余额。
- 频率：**仅首次跌破阈值**——用 Redis 标记 `lowbal_notified:{tenant}:{user}`：
  - 发送后余额 `< threshold` 且标记不存在 → 插入系统消息 + 置标记（`SetNX`，TTL 如 7 天兜底）。
  - 发送后余额 `>= threshold` → 删除标记（余额回升后下次跌破可再次触发）。

### 系统消息落地
- 后端在 chat 发送链路（`server/internal/chat` 的 SendMessage/handler，扣费之后）插入一条 `type='system'` 的消息（`sender_id=0` 作为系统），写入 `messages` 表并**经现有 WS 广播**给该用户，保证实时显示且刷新可见。
- 实现时先读 `chat.Service.SendMessage` 与扣费/广播逻辑，确认在扣费成功后拿到最新余额的位置插入；系统消息不计费、不触发机器人回复。

### 客户端渲染
- `chat.vue`：消息循环里对 `m.type === 'system'` 渲染为**居中系统提示**（复用/类似 `.day` 样式），不显示头像/气泡。可在系统消息后放一个"去充值"文案（点击跳充值页，可选）。

## 5. 涉及文件

- `client/src/pages/ocean/ocean.vue`（#1）
- `client/src/store/user.js`（#2/#3：applyClientConfig）
- `client/src/pages/chat/chat.vue`（#3 横幅 + #4 system 渲染）
- `server/internal/user/handler.go`（#2/#3：clientConfig 用于 login + profile）
- `server/internal/chat/*`（#4：发送后余额检查 + 插入系统消息）
- `server/internal/sysconfig/sysconfig.go`（#3/#4 新键 + defaults）
- `server/internal/admin/meta.go`（#3/#4 ConfigField）

## 6. 测试与验证

- 后端单测（纯逻辑可测的部分）：`clientConfig` 含全部键；低余额判定/标记逻辑若可纯函数化则单测（跨阈值：<N 且未标记→发、>=N→清标记）。
- `go build/vet/test ./...`；`npm run build`（client 用 HBuilderX/uni 构建，或至少 lint/构建通过）。
- 生产验证：老用户 profile 返回 chat_quicks（curl /user/profile 带 token）；聊天横幅随配置变化；余额压到 <N 发消息出现一条系统消息且同会话当日不重复。

## 7. 非目标（YAGNI）

- 不做余额不足的强弹窗/拦截发送（只插一条软提示系统消息）。
- 不做系统消息的多语言/富文本。
- #1 不改后端 scoop 接口，仅前端统一入口。
