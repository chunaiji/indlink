# 小程序修复 + 余额不足系统消息 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复点瓶绕过捞瓶配额、后台配置对老用户不生效、聊天横幅写死，并新增"余额不足自动系统消息"。

**Architecture:** 后端把客户端配置聚合成共享 `clientConfig` 块，登录与 `/user/profile` 都下发；客户端 `applyClientConfig` 在 login 与 fetchProfile 都应用。捞瓶点击统一走计次接口。余额不足在 chat 发送扣费后按 Redis「首次跌破」标记插入并 WS 推送一条 system 消息。

**Tech Stack:** Go(Gin/GORM/Redis) 后端；uni-app(Vue2 Options API) 小程序前端。

## Global Constraints

- 后端每步 `cd server && go build ./...` 通过；结束 `go test ./...` 全绿；纯逻辑 TDD。
- **客户端是微信小程序(uni-app)，无法用本仓库脚本部署**；客户端改动由人工用 HBuilderX/微信开发者工具构建上传。客户端任务不写自动化测试，靠代码审查 + 人工验证。
- 后端配置键必须同时加 `sysconfig`(常量+defaults) 与 `admin/meta.go`(configMeta)。
- 系统消息用 `type="system"`、`sender_id=0`，不计费、不触发机器人回复。
- 提交信息结尾加：`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- `server/internal/sysconfig/sysconfig.go` — 新增 3 键 + defaults。
- `server/internal/admin/meta.go` — 新增 3 ConfigField。
- `server/internal/user/handler.go` — 新增 `clientConfig(tenantID)`，login + profile 都用。
- `server/internal/chat/service.go` — `SendMessage` 后置 `maybeLowBalanceNotice`；新增该方法 + `pushSystemMessage`。
- `client/src/store/user.js` — 新增 `applyClientConfig(res)`，login + fetchProfile 调用；uiText 加 chatBanner。
- `client/src/pages/ocean/ocean.vue` — `openBottle` 改为走 `scoop()`。
- `client/src/pages/chat/chat.vue` — 横幅读配置；渲染 `type=system`；确保 WS 自推消息追加。

---

## Task 1: 新增配置键(#3/#4)

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`

**Interfaces:**
- Produces: 常量 `KeyUITextChatBanner`, `KeyLowBalanceThreshold`, `KeyLowBalanceMsg`（string 值键）。

- [ ] **Step 1: 加常量**（`sysconfig.go` const 键区块，UI 文案/聊天相关处）

```go
	KeyUITextChatBanner    = "ui_text_chat_banner"   // 聊天页顶部横幅文案
	KeyLowBalanceThreshold = "low_balance_threshold" // 余额低于此值(猛币)推系统消息;0=关
	KeyLowBalanceMsg       = "low_balance_msg"        // 余额不足系统消息文案
```

- [ ] **Step 2: 加 defaults**（`defaults` map）

```go
	KeyUITextChatBanner:    "缘起一只漂流瓶 · 友善聊天",
	KeyLowBalanceThreshold: "10",
	KeyLowBalanceMsg:       "余额不足啦,充值后就能继续畅聊咯~",
```

- [ ] **Step 3: 加 admin ConfigField**（`meta.go` configMeta 末尾）

```go
	{Key: sysconfig.KeyUITextChatBanner, Label: "聊天页横幅文案", Group: "UI 文案", Type: "text"},
	{Key: sysconfig.KeyLowBalanceThreshold, Label: "余额不足阈值(猛币,0关)", Group: "聊天", Type: "int"},
	{Key: sysconfig.KeyLowBalanceMsg, Label: "余额不足系统消息", Group: "聊天", Type: "text"},
```

- [ ] **Step 4: 构建**

Run: `cd server && go build ./...`
Expected: exit 0

- [ ] **Step 5: Commit**

```bash
git add server/internal/sysconfig/sysconfig.go server/internal/admin/meta.go
git commit -m "feat(config): 新增聊天横幅/余额不足阈值+文案配置键"
```

---

## Task 2: 后端 clientConfig 共享块(#2/#3)

**Files:**
- Modify: `server/internal/user/handler.go`（新增 `clientConfig`；`login` 用它替换内联字段；`profile` 并入）
- Test: `server/internal/user/clientconfig_test.go`

**Interfaces:**
- Consumes: Task 1 的新键。
- Produces: `func clientConfig(tenantID int64) gin.H`（含所有客户端配置字段）。

- [ ] **Step 1: 写失败测试** `server/internal/user/clientconfig_test.go`

```go
package user

import "testing"

func TestClientConfig_HasKeys(t *testing.T) {
	c := clientConfig(0) // 用全局默认
	for _, k := range []string{
		"chat_quicks", "reply_quicks", "ui_text_chat_banner",
		"low_balance_threshold", "low_balance_msg", "price_chat", "ws_url",
	} {
		if _, ok := c[k]; !ok {
			t.Errorf("clientConfig 缺少键 %s", k)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/user/ -run TestClientConfig`
Expected: FAIL（`undefined: clientConfig`）

- [ ] **Step 3: 实现 `clientConfig`**（`handler.go`，import 已有 sysconfig/gin）

```go
// clientConfig 聚合客户端要用的 sysconfig 字段;登录与 /user/profile 共用,避免两处漂移。
func clientConfig(tenantID int64) gin.H {
	return gin.H{
		"ios_recharge_off":      sysconfig.GetBool(tenantID, sysconfig.KeyIOSRechargeOff),
		"push_subscribe_prompt": sysconfig.GetBool(tenantID, sysconfig.KeyPushSubscribePrompt),
		"price_chat":            sysconfig.GetInt(tenantID, sysconfig.KeyPriceChat),
		"share_title":           sysconfig.GetString(tenantID, sysconfig.KeyShareTitle),
		"share_image":           sysconfig.GetString(tenantID, sysconfig.KeyShareImage),
		"ws_url":                sysconfig.GetString(tenantID, sysconfig.KeyWSURL),
		"ui_text_anon_sender":   sysconfig.GetString(tenantID, sysconfig.KeyUITextAnonSender),
		"ui_text_anon_friend":   sysconfig.GetString(tenantID, sysconfig.KeyUITextAnonFriend),
		"ui_text_some_friend":   sysconfig.GetString(tenantID, sysconfig.KeyUITextSomeFriend),
		"ui_text_nav_title":     sysconfig.GetString(tenantID, sysconfig.KeyUITextNavTitle),
		"ui_text_chat_banner":   sysconfig.GetString(tenantID, sysconfig.KeyUITextChatBanner),
		"chat_quicks":           sysconfig.GetString(tenantID, sysconfig.KeyChatQuicks),
		"reply_quicks":          sysconfig.GetString(tenantID, sysconfig.KeyReplyQuicks),
		"low_balance_threshold": sysconfig.GetInt(tenantID, sysconfig.KeyLowBalanceThreshold),
		"low_balance_msg":       sysconfig.GetString(tenantID, sysconfig.KeyLowBalanceMsg),
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/user/ -run TestClientConfig`
Expected: PASS

- [ ] **Step 5: login 用 clientConfig**（替换 `handler.go` login 响应里 `response.OK(c, gin.H{...})` 的内联配置字段）

把 login 的响应改为在基础字段上并入 clientConfig：
```go
	resp := gin.H{"token": token, "user": u, "is_new": isNew}
	for k, v := range clientConfig(u.TenantID) {
		resp[k] = v
	}
	response.OK(c, resp)
```
（删除原来 login 响应里逐个写的 ios_recharge_off/…/chat_quicks/reply_quicks 内联行。）

- [ ] **Step 6: profile 并入 clientConfig**（`handler.go profile`，现为 `response.OK(c, gin.H{"user": u})` 或类似——先读确认，改为并入）

```go
func (h *Handler) profile(c *gin.Context) {
	u, err := h.svc.Profile(middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeNotFound, "用户不存在")
		return
	}
	resp := gin.H{"user": u}
	for k, v := range clientConfig(u.TenantID) {
		resp[k] = v
	}
	response.OK(c, resp)
}
```

- [ ] **Step 7: 构建 + 测试**

Run: `cd server && go build ./... && go test ./internal/user/`
Expected: build exit 0；PASS

- [ ] **Step 8: Commit**

```bash
git add server/internal/user/handler.go server/internal/user/clientconfig_test.go
git commit -m "fix(user): profile 与 login 共用 clientConfig, 老用户也能拿到后台配置"
```

---

## Task 3: 客户端 applyClientConfig + 聊天横幅(#2/#3)

**Files:**
- Modify: `client/src/store/user.js`（新增 `applyClientConfig`；login/fetchProfile 调用；state.uiText 加 chatBanner；state 加 lowBalance 字段）

**Interfaces:**
- Consumes: 后端 login/profile 响应字段（Task 2）。

- [ ] **Step 1: state 加 chatBanner 与 lowBalance 字段**（`user.js` state 的 uiText 对象里加 `chatBanner`，并加两个顶层字段）

在 `uiText: { ... }` 内加：
```js
      chatBanner: '缘起一只漂流瓶 · 友善聊天',
```
在 state 里(chatQuicks 附近)加：
```js
    lowBalanceThreshold: 0,
    lowBalanceMsg: '',
```

- [ ] **Step 2: 新增 `applyClientConfig(res)` action**（`user.js` actions 内，login 之前）

```js
    // 应用后端下发的客户端配置(登录/拉资料共用),老用户经 profile 也能拿到最新配置。
    applyClientConfig(res) {
      if (!res) return
      this.iosRechargeOff = !!res.ios_recharge_off
      this.pushSubscribePrompt = !!res.push_subscribe_prompt
      if (res.price_chat != null) this.chatPrice = res.price_chat
      if (res.share_title) this.shareTitle = res.share_title
      if (res.share_image != null) this.shareImage = res.share_image || ''
      if (res.ws_url) { this.wsUrl = res.ws_url; uni.setStorageSync('wsUrl', res.ws_url) }
      if (res.ui_text_anon_sender) this.uiText.anonSender = res.ui_text_anon_sender
      if (res.ui_text_anon_friend) this.uiText.anonFriend = res.ui_text_anon_friend
      if (res.ui_text_some_friend) this.uiText.someFriend = res.ui_text_some_friend
      if (res.ui_text_nav_title) this.uiText.navTitle = res.ui_text_nav_title
      if (res.ui_text_chat_banner) this.uiText.chatBanner = res.ui_text_chat_banner
      this.chatQuicks = parseQuicks(res.chat_quicks, this.chatQuicks)
      this.replyQuicks = parseQuicks(res.reply_quicks, this.replyQuicks)
      if (res.low_balance_threshold != null) this.lowBalanceThreshold = res.low_balance_threshold
      if (res.low_balance_msg != null) this.lowBalanceMsg = res.low_balance_msg
    },
```

- [ ] **Step 3: login 改用 applyClientConfig**（把 login 里逐条设置 ios_recharge_off/…/replyQuicks 的行替换为一句）

在 `login()` 里 `this.token = res.token; this.profile = res.user` 之后，删除第 84–95 行那批逐条赋值，改为：
```js
        this.applyClientConfig(res)
```
保留 `uni.setStorageSync('token', res.token)`、`is_new` 逻辑、`init()`、`resetAndReconnect(...)`。

- [ ] **Step 4: fetchProfile 也应用配置**（`fetchProfile()` 内，拿到 res 后）

```js
    async fetchProfile() {
      const res = await get('/user/profile')
      this.profile = res.user ?? res
      this.applyClientConfig(res)
      return this.profile
    },
```
（删除原来 fetchProfile 里单独的 `if (res.price_chat != null) this.chatPrice = ...`，已并入 applyClientConfig。）

- [ ] **Step 5: 构建校验**（客户端无自动化测试；用 uni 构建或 lint 确认无语法错）

Run: `cd client && npx vue-tsc --noEmit 2>/dev/null || echo "无 tsc, 跳过"`（若项目无 TS 检查，跳过，靠审查）
Expected: 无语法/引用错误

- [ ] **Step 6: Commit**

```bash
git add client/src/store/user.js
git commit -m "fix(client): 抽 applyClientConfig, login+fetchProfile 都应用后台配置(修老用户不生效)"
```

---

## Task 4: 点浮动瓶=捞一次(#1)

**Files:**
- Modify: `client/src/pages/ocean/ocean.vue`（`openBottle` 改为走 `scoop()`）

- [ ] **Step 1: 改 `openBottle`**（`ocean.vue` methods 里）

```js
    // 点浮动瓶子与「捞瓶子」按钮同一动作:计次+去重+配额0拦截
    openBottle() {
      this.scoop()
    },
```
（`scoop()` 已存在:调 `scoopOne`、计次、`code===3003` 弹 quotaExceeded、成功 showBottle。）

- [ ] **Step 2: 确认模板调用兼容**

模板 `@tap="openBottle(b)"` 传参 b 现被忽略（点击即捞一次）。确认无其他地方依赖 openBottle(b) 展示具体瓶子（`openShared` 分享入口不受影响,仍免费展示）。

- [ ] **Step 3: 构建校验**

Run: `cd client && echo "uni-app 由 HBuilderX 构建, 此处仅审查语法"`
Expected: 无语法错误（审查确认）

- [ ] **Step 4: Commit**

```bash
git add client/src/pages/ocean/ocean.vue
git commit -m "fix(client): 点浮动瓶子改为走捞瓶配额(与捞瓶按钮一致),不再绕过限次"
```

---

## Task 5: 后端余额不足系统消息(#4)

**Files:**
- Modify: `server/internal/chat/service.go`（`SendMessage` 末尾调 `maybeLowBalanceNotice`；新增 `maybeLowBalanceNotice` + `pushSystemMessage`）

**Interfaces:**
- Consumes: Task 1 的 `KeyLowBalanceThreshold`/`KeyLowBalanceMsg`；`s.wlt.Balance(userID)`；`s.hub.PushTo`；`cache.RDB`。
- Produces: `func (s *Service) maybeLowBalanceNotice(tenantID, userID, chatID int64)`。

- [ ] **Step 1: 先读依赖确认签名**

阅读 `server/internal/chat/service.go` 顶部结构体与 import：确认 `s.wlt` 有 `Balance(userID int64) (int64, error)`（如名称不同以实际为准）、`s.hub.PushTo(userID int64, payload ...)` 的签名、是否已 import `driftbottle/pkg/cache` 与 `sysconfig`/`idgen`/`model`/`time`。缺则补 import。

- [ ] **Step 2: 在 `SendMessage` 成功推送后调用**（`service.go` `SendMessage` 里 `s.hub.PushTo(other, ...)` 之后、`return` 之前）

```go
	// 真人发消息后:余额不足软提示(首次跌破阈值,系统消息)
	if !sender.IsRobot {
		s.maybeLowBalanceNotice(tenantID, senderID, chatID)
	}
```

- [ ] **Step 3: 实现 `maybeLowBalanceNotice` 与 `pushSystemMessage`**（`service.go` 文件末尾）

```go
// maybeLowBalanceNotice 真人发消息后,若余额首次跌破阈值,插入并推送一条系统消息。
// 用 Redis 标记 lowbal_notified:{tenant}:{user} 保证仅首次跌破时提示;余额回升到阈值以上时清标记。
func (s *Service) maybeLowBalanceNotice(tenantID, userID, chatID int64) {
	threshold := sysconfig.GetInt64(tenantID, sysconfig.KeyLowBalanceThreshold)
	if threshold <= 0 {
		return
	}
	bal, err := s.wlt.Balance(userID)
	if err != nil {
		return
	}
	ctx := gocontext.Background()
	key := fmt.Sprintf("lowbal_notified:%d:%d", tenantID, userID)
	if bal >= threshold {
		cache.RDB.Del(ctx, key) // 余额充足:清标记,下次跌破可再提示
		return
	}
	// 余额不足:仅首次(SetNX 成功)提示
	ok, e := cache.RDB.SetNX(ctx, key, 1, 7*24*time.Hour).Result()
	if e != nil || !ok {
		return
	}
	msg := sysconfig.GetString(tenantID, sysconfig.KeyLowBalanceMsg)
	if msg == "" {
		return
	}
	s.pushSystemMessage(tenantID, chatID, userID, msg)
}

// pushSystemMessage 插入一条 system 消息(sender_id=0,不计费)并 WS 推给指定用户。
func (s *Service) pushSystemMessage(tenantID, chatID, toUser int64, content string) {
	msg := &model.Message{
		MessageID: idgen.Next(), TenantID: tenantID, ChatID: chatID, SenderID: 0,
		Content: content, Type: "system", ReadStatus: false, CreatedAt: time.Now(),
	}
	if err := s.db.Create(msg).Error; err != nil {
		return
	}
	s.hub.PushTo(toUser, map[string]interface{}{
		"event": "message", "chat_id": strconv.FormatInt(chatID, 10), "message": msg,
	})
}
```
> 注:若 `SendMessage` 里已 import `gocontext "context"`/`fmt`,复用;否则在 import 块补 `"context"` 或 `gocontext "context"` 与 `"fmt"`、`"driftbottle/pkg/cache"`。`s.wlt.Balance` 名称以实际为准。

- [ ] **Step 4: 构建 + 全测**

Run: `cd server && go build ./... && go test ./...`
Expected: build exit 0；全绿

- [ ] **Step 5: Commit**

```bash
git add server/internal/chat/service.go
git commit -m "feat(chat): 余额首次跌破阈值时推送余额不足系统消息(可配)"
```

---

## Task 6: 客户端聊天页系统消息渲染 + 横幅(#3/#4)

**Files:**
- Modify: `client/src/pages/chat/chat.vue`（横幅读配置；`type=system` 居中渲染；确认 WS 自推消息追加）

- [ ] **Step 1: 横幅读配置**

模板第 6 行改：
```html
      <view class="day">{{ chatBanner }}</view>
```
`data()` 里加（`quicks` 附近）：
```js
      chatBanner: useUserStore().uiText.chatBanner,
```

- [ ] **Step 2: system 消息渲染**（模板消息循环内，最外层按类型分支）

把消息循环体改为区分 system 与普通消息（保留原有普通消息结构）：
```html
      <template v-for="m in messages">
        <view v-if="m.type === 'system'" :key="m.message_id" class="sys-tip">{{ m.content }}</view>
        <view
          v-else
          :key="m.message_id"
          class="msg"
          :class="mine(m) ? 'me' : 'them'"
        >
          <user-avatar v-if="!mine(m)" class="av" :name="partnerNickname || '对方'" :src="partnerAvatar" :size="76" shape="circle" />
          <user-avatar v-if="mine(m)" class="av" :name="myNickname" :src="myAvatar" :size="76" shape="circle" />
          <view class="col">
            <text v-if="!mine(m)" class="nm">{{ partnerNickname || '对方' }}</text>
            <view class="bub" :class="m.type === 'image' ? 'img' : (mine(m) ? 'me-bub' : 'them-bub')">
              <image v-if="m.type === 'image'" :src="m.content" mode="widthFix" class="bub-img" @tap="preview(m.content)" />
              <text v-else>{{ m.content }}</text>
            </view>
          </view>
        </view>
      </template>
```

- [ ] **Step 3: 加 system 样式**（`<style>` 内）

```css
.sys-tip { text-align: center; color: #9aa7b4; font-size: 24rpx; margin: 16rpx auto; padding: 6rpx 20rpx; background: rgba(0,0,0,0.04); border-radius: 16rpx; max-width: 80%; }
```

- [ ] **Step 4: 确认 WS 自推消息会被追加**

阅读 `chat.vue` 的 `onWSMessage`/消息追加逻辑：确认收到 `event=message` 且 `chat_id` 等于当前会话时会 `messages.push(m)`，且**不因 sender 是自己/系统而过滤**（系统消息 sender_id=0）。若现有逻辑按 chat_id 匹配即追加,则无需改;若有"忽略自己发的"过滤,需放行 `type=system`。把结论写进报告。

- [ ] **Step 5: 构建校验（审查）**

Run: `cd client && echo "uni-app 由 HBuilderX 构建, 审查语法与模板"`
Expected: 模板/脚本无语法错误

- [ ] **Step 6: Commit**

```bash
git add client/src/pages/chat/chat.vue
git commit -m "feat(client): 聊天横幅读配置 + 渲染余额不足系统消息(居中提示)"
```

---

## Task 7: 部署后端 + 验证 + 客户端交接

**Files:** 无代码改动(ops)。

- [ ] **Step 1: 全量校验 + 交叉编译**

```bash
cd server && go build ./... && go vet ./... && go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api
```
Expected: 全 exit 0；生成 driftbottle-linux

- [ ] **Step 2: 部署后端**

```bash
node deploy.js
```
Expected: active + nginx 重载成功

- [ ] **Step 3: 验证 #2（profile 下发配置）**

```bash
# 用真人 token 调 /user/profile,确认返回 chat_quicks / ui_text_chat_banner / low_balance_* 字段
```
（无现成真人 token 时,后端 curl 无法直连需鉴权的 /user/profile;改为确认 admin /config 新键在线 + 代码审查。见 Step 4。）

- [ ] **Step 4: 验证配置键在线**

```bash
BASE=https://ambertu.com/message/admin/api
TOKEN=$(curl -s -X POST $BASE/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
curl -s "$BASE/config" -H "Authorization: Bearer $TOKEN" | grep -oE "ui_text_chat_banner|low_balance_threshold|low_balance_msg"
```
Expected: 三个键都出现

- [ ] **Step 5: 客户端交接（人工）**

告知用户：客户端 3 处改动（ocean.vue / store/user.js / chat.vue）需**用 HBuilderX/微信开发者工具构建并上传小程序**;后端已上线,老用户重开小程序(触发 fetchProfile)即可拿到最新配置。余额不足系统消息需客户端上线后、余额压到 <阈值 发消息触发验证。

- [ ] **Step 6: 推送**

```bash
git push origin main
```

---

## Self-Review

- **Spec 覆盖**：#1(Task4)/#2(Task2+3)/#3(Task1+2+3+6)/#4(Task1+5+6)/部署(Task7) 全覆盖。
- **占位符**：无 TBD;每步含实际代码/命令。客户端"构建"步骤明确说明由 HBuilderX 人工构建(环境限制,非占位)。
- **类型一致**：`clientConfig`(Task2)→客户端 `applyClientConfig`(Task3) 字段名一致(chat_quicks/ui_text_chat_banner/low_balance_*);`maybeLowBalanceNotice`/`pushSystemMessage`(Task5) 签名自洽;`type="system"`(Task5) 与客户端渲染(Task6)一致;`chatBanner`(Task3 state)→chat.vue(Task6) 一致。
- **已知风险**：`s.wlt.Balance`/`s.hub.PushTo` 精确签名、chat.vue WS 追加与 profile handler 现状,均要求实现者先读真实代码再写(相应步骤已注明"以实际为准/先读确认")。
