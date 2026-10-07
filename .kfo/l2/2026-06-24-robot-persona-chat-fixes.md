# 机器人 AI 人格实装 + 聊天体验修复

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-24 |
| 状态 | COMPLETED |
| 触发 | AI 承认自己是机器人；聊天头像反向；无法自动滚底；admin 机器人对话不可见 |
| 关联 | L1 `l1/robot.md` · `l1/chat-ws.md` · `l1/admin-platform.md` |

---

## 1. LLM 历史消息结构修复（根本性修复）

### 问题

`LLMClient.Chat()` 只发两条消息：`{system: 全部 prompt 含历史文本}` + `{user: 当前消息}`。历史对话以**文本形式嵌入 system prompt**：

```
system: "你是真实的人…【硬性禁止】不能承认是AI…
         你：是的，我是一个AI助手。"   ← 历史文本
user: "你是什么？"
```

LLM 看到自己上一轮说了"我是AI"，会继续维持方向——system 禁令被自己的历史输出覆盖。

### 修复

历史对话改为 **独立的 user/assistant 消息**，system prompt 只保留身份规则，不嵌入历史：

```go
// llmclient.go — 新签名
func (c *LLMClient) Chat(ctx context.Context, systemPrompt string, history []llmMessage, userText string) (string, error)

// 构造顺序：system → history(user/assistant交替) → 当前user
messages = append([]llmMessage{{Role:"system", Content:systemPrompt}}, history...)
messages = append(messages, llmMessage{Role:"user", Content:userText})
```

```go
// promptbuilder.go — buildPrompt 不再接收 history 参数
func buildPrompt(persona model.PersonaConfig, memory model.RobotMemory) string

// 新增 buildHistory：转换并净化历史记录
func buildHistory(history []model.Message, botUserID int64) []llmMessage
```

### AI Confession 净化（buildHistory）

若 bot 的历史消息包含 AI 自白短语（"AI助手"、"GPT"、"机器人"、"语言模型" 等 20+ 关键词），将该条消息内容替换为 `"嗯，你说呢～"`，防止 LLM 看到后继续沿用。

```go
var aiConfessionPhrases = []string{
    "AI助手", "人工智能", "语言模型", "GPT", "ChatGPT", "Claude",
    "我是机器人", "我是AI", "我是助手", "作为一个AI", ...
}

func sanitizeBotContent(content string) string {
    for _, phrase := range aiConfessionPhrases {
        if strings.Contains(strings.ToLower(content), strings.ToLower(phrase)) {
            return "嗯，你说呢～"
        }
    }
    return content
}
```

### 受影响文件

- `server/internal/robot/llmclient.go` — Chat 签名增加 `history []llmMessage`
- `server/internal/robot/promptbuilder.go` — 移除 history 参数，新增 `buildHistory` + `sanitizeBotContent`
- `server/internal/robot/bot_worker.go` — 分别传入 systemPrompt 和 historyMsgs
- `server/internal/robot/service.go` — genAIText 中 `llm.Chat` 传 nil history
- `server/internal/robot/bot_worker.go` — TestLLM 传 nil history

---

## 2. System Prompt 身份设定加固

| 位置 | 改动 |
|---|---|
| 开头 | `"你是一个真实的人，名字叫{name}"` 替代 `"你是一个对话陪伴助手"` |
| 末尾（权重最高） | 4 条硬性禁止：不承认AI、直接否认并转移话题、不解释运作机制、反复追问也不松口 |

---

## 3. 管理后台机器人对话租户修复

### 问题链路

1. `startRobotChat` 用 `defaultTenantID()=1` 创建聊天 → chat.tenant_id=1
2. 小程序用户 tenant_id=100，按 tenant_id=100 查询消息列表 → 看不到
3. Admin `ListRobotChats` 也按 tenant_id=1 查 → 后台看不到

### 修复

- `startRobotChat` 改用 `GetUserTenant(userID)` 取目标用户的租户
- `ListRobotChats` SQL 加条件过滤（tenant_id=0 时全量，>0 时按租户）
- `RobotChatMessages`、`SendAsRobot` 移除 tenantID 参数，直接按 chat 记录的 tenant_id 操作
- DB 补丁：已存在的错误租户聊天手动更新 `UPDATE chats SET tenant_id=100 WHERE ...`

---

## 4. 小程序聊天体验修复（client/src/pages/chat/chat.vue）

### 头像方向修复

`flex-direction: row-reverse` 下，子元素排列右→左，第一个子元素在最右。

```html
<!-- 正确：avatar 在 col 之前，row-reverse 时 avatar 在最右（自己的消息） -->
<user-avatar v-if="mine(m)" class="av" ... />
<view class="col">...</view>
```

### 自动滚底修复

用单调递增计数器替代随机数，确保 `scroll-top` 每次都有新值触发滚动：

```js
scrollToBottom() {
  this._seq = (this._seq || 0) + 1
  const seq = this._seq
  this.$nextTick(() => {
    this.$nextTick(() => { this.scrollTop = 999999 + seq })
  })
}
```

双 `$nextTick` 保证 DOM 先渲染完再设置滚动位置。

---

## 5. Admin 机器人对话 5 秒轮询（admin/src/views/RobotChats.vue）

用户在小程序回复后，admin 面板需轮询才能看到（无 WS 推送到 admin）。

```js
let msgTimer = null

async function select(c) {
  if (msgTimer) clearInterval(msgTimer)
  await loadMessages()
  msgTimer = setInterval(loadMessages, 5000)
}

// loadMessages 优化：只有消息数量增加时才强制滚底
const hadNew = msgs.length > messages.value.length
if (hadNew) { await nextTick(); msgsEl.value.scrollTop = msgsEl.value.scrollHeight }
```

切换对话或组件卸载时清除计时器。
