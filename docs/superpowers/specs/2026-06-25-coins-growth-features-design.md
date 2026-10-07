# 金币增长功能设计（签到 / 消息扣费 / 分享 / 活跃时间 / 机器人主动触达）

- 日期：2026-06-25
- 范围：server（Go）为主，admin 配置项白名单，client（uni-app）触点列表
- 状态：待审核

## 背景与目标

在现有漂流瓶 + AI 人格化机器人体系上，新增 5 项围绕「金币消耗/发放」和「机器人主动拉活」的运营增长能力。全部由管理后台开关 + 数值控制，沿用既有模式：

- 钱包：`wallet.Credit` / `wallet.Debit`（带 `biz` 回调，扣费与业务同事务），流水落 `WalletTxn`（含 `scene` / `biz_no` / `balance_after`，可对账）。
- 配置：`sysconfig` 内存缓存 + `config` 表热调；管理后台只允许改 `admin/meta.go` 白名单 `configMeta` 中的键，前端 `Config.vue` 按 `group` 自动渲染表单。
- 定时任务：`push.StartCheckinCron` 的 goroutine + `time.Sleep(until next)` 模式。
- 每日计数：`quota` 包用 Redis 按天 key（48h 过期，跨天自动失效）。
- 机器人：`bot_worker` 走 Tier0~3，最终 `MessageSender.SendMessage(botUserID, ...)` 投递；`LLMClient.Chat` 按 persona system prompt 生成回复。

---

## 功能 1：每日签到领 N 金币

### 配置（sysconfig + configMeta）
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `checkin_enabled` | bool | `0` | 签到总开关 |
| `checkin_coins` | int | `10` | 每日签到发放金币 N（固定值，非连续累加） |

### 数据模型（新增表）
```go
// CheckinLog 每日签到记录（唯一索引保证一人一天一次，资金可对账）。
type CheckinLog struct {
    ID        int64     `gorm:"primaryKey;autoIncrement"`
    TenantID  int64     `gorm:"index"`
    UserID    int64     `gorm:"uniqueIndex:uk_user_date"`
    Date      string    `gorm:"size:8;uniqueIndex:uk_user_date"` // 20060102
    Coins     int64
    CreatedAt time.Time
}
```
加入 `model.AllModels()` 参与 AutoMigrate。

### 钱包场景
新增 `wallet.SceneCheckin = "checkin"`（`Scene` 字段 size:24，足够）。

### 服务与接口（新增 `internal/checkin` 包 或并入 user 模块，建议独立包）
- `POST /api/checkin`：
  - 关闭则返回业务错误「签到未开启」。
  - 同事务：`INSERT CheckinLog`（唯一键命中 → 已签到，返回 already_signed=true，不重复发币）→ `wallet.CreditTx(tx, ..., coins, SceneCheckin, biz_no)`。
  - `biz_no = "checkin:{userID}:{date}"`。
  - 返回 `{ coins, balance, already_signed }`。
- `GET /api/checkin/status`：返回 `{ enabled, coins, signed_today }`，供客户端渲染按钮态。

### 边界
- 唯一索引兜底并发重复签到。
- `checkin_coins<=0` 视为不发币（仍记录签到态）。

---

## 功能 2：聊天每条消息扣 A 金币（记流水）

> 现状：开聊 `StartChat` 已按 `price_chat` 扣一次并记流水。本功能**新增「每条消息」扣费**，与开聊费用独立。

### 配置
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `price_msg` | int | `0` | 每条消息消耗金币 A；`0` = 关闭（不破坏现有免费行为） |

### 扣费规则
- 仅**真人 sender** 扣费；`is_robot` 的 sender 一律免扣（机器人回复/主动触达都不扣）。
- 与接收方是否机器人无关：真人发出每条消息都扣 A（这是核心变现点）。
- `price_msg<=0` 时完全走原有免费路径。

### 实现（chat.Service.SendMessage 改造）
当前 `SendMessage` 在一个事务里「建消息 + 更新会话」。改为：
- 取 `price := sysconfig.GetInt64(KeyPriceMsg)`，查 sender `is_robot`。
- 若 `price>0 && !sender.IsRobot`：用 `wlt.Debit(tenantID, senderID, price, SceneMsg, biz_no, biz)`，把「建消息 + 更新会话」放进 `biz` 回调，保证扣费与落库原子。`biz_no = "msg:{messageID}"`。
- 否则走原有无扣费事务。
- WS 推送、`onMsgPush`、`botEnqueue` 逻辑保持在扣费成功之后。
- 余额不足返回既有 `errs.ErrInsufficient`，HTTP 与 WS 两条发送路径都要把该错误透传给客户端（前端弹充值，与开聊一致）。

### 边界
- WS 发送路径（chat handler）此前可能未处理扣费失败，需要补：扣费失败时通过 WS 回一条 error 事件而非静默丢弃。
- 机器人主动触达（功能5）经 `SendMessage(botUserID)` 不扣费，符合规则。

---

## 功能 3：分享领 M 金币（控制每日次数）

### 配置
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `share_reward_enabled` | bool | `0` | 分享奖励总开关 |
| `share_reward_coins` | int | `5` | 单次分享奖励 M |
| `share_reward_daily_limit` | int | `3` | 每日最多领奖次数 |

### 计数与发放
- 复用 `quota` 按天计数思路：Redis key `q:{tenant}:{user}:share:{date}`，`TryConsume(action="share", freeLimit=share_reward_daily_limit)`。
  - 该 action 不发售「次数包」，pack 余额恒为 0，所以 `TryConsume` 实际就是「每日上限 N 次」的纯计数器，达上限返回 false。
- 成功占用一次后 `wallet.Credit(..., M, SceneShare, biz_no)`，`biz_no = "share:{user}:{date}:{count}"`。
- 新增 `wallet.SceneShare = "share"`。

### 接口
- `POST /api/share/reward`：客户端在 `onShareAppMessage` 回调成功后调用。
  - 返回 `{ rewarded(bool), coins, balance, count_today, limit }`。达上限返回 `rewarded=false` + 文案，不报错。

### 说明（已知限制）
微信小程序无法可靠探测「分享是否真的发送成功」，采用**信任客户端触发 + 每日上限**兜量，刷量风险由 daily_limit 控制。后续可加分享落地页回流校验，本期不做。

---

## 功能 4：记录用户最新登录时间 + 最近活跃时间

### 数据模型
`User` 已有 `LastActiveAt`。新增：
```go
LastLoginAt time.Time `json:"last_login_at"` // 最近一次登录(code2session)时间
```
（`LastActiveAt` 复用为「最近活跃」，由请求刷新，供功能5。）

### 写入时机
- **登录时间**：`user.Login` 命中老用户/新建用户时，置 `last_login_at = now`（与现有 `last_active_at` 更新同处）。
- **最近活跃**：在鉴权中间件解析出 `userID` 后刷新 `last_active_at`。直接每请求写库代价高，做 Redis 去抖：
  - key `active:{user}`，TTL 60s；若不存在则更新 DB `last_active_at = now` 并 set key。
  - 即「每个用户最多每 60s 写一次 last_active_at」。
- WS 连接 `add` 时也刷新一次（保证在线即算活跃）。

### 边界
- 机器人账号不刷活跃（机器人无 HTTP 请求/WS，天然不影响）。

---

## 功能 5：机器人按概率主动触达近期活跃用户

### 配置
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `outreach_enabled` | bool | `0` | 主动触达总开关 |
| `outreach_window_start` | int | `19` | 触达时间段开始小时（0-23，含） |
| `outreach_window_end` | int | `22` | 触达时间段结束小时（0-23，不含） |
| `outreach_probability` | int | `10` | 从近期活跃用户中抽样的比例 N（%） |
| `outreach_lookback_min` | int | `30` | 回溯活跃窗口 M（分钟） |
| `outreach_tick_min` | int | `30` | 窗口内调度器执行间隔（分钟） |
| `outreach_user_daily_cap` | int | `1` | 同一用户每日最多被触达次数（防骚扰） |

### 调度器（新增 `internal/robot/outreach.go`）
仿 `StartCheckinCron`：goroutine 每 `outreach_tick_min` 分钟 tick 一次。每 tick：
1. `outreach_enabled` 关 → skip。
2. 当前小时不在 `[window_start, window_end)` → skip。
3. 查候选用户：`is_robot=false AND status='active' AND is_muted=false AND last_active_at >= now - M 分钟`。
4. 排除「今日已达 `outreach_user_daily_cap`」的用户（Redis `outreach:{tenant}:{user}:{date}` 计数）。
5. 抽样：`pick = floor(len(候选) * N / 100)`，随机选 `pick` 个（可为 0）。
6. 对每个选中用户：
   - 选机器人：若该用户已有与某机器人的会话，复用该机器人；否则从 `BotRegistry` 随机取一个 active 机器人。
   - `EnsureRobotChat(tenant, botUserID, userID)`：找/建 robot↔user 会话（**不扣机器人金币**，独立于 `StartChat`）。
   - 生成开场白：`LLMClient.Chat(persona system prompt, 空历史, 触发指令)`，触发指令形如「请你主动发起聊天，发一句自然、符合人设的开场问候，不要透露 AI 身份」。
   - 经 `MessageSender.SendMessage(botUserID, chatID, text, "text")` 投递（复用 `bot_worker.deliver` 的打字延迟更自然，可选）。
   - Redis 触达计数 +1（TTL 48h）。
7. 失败兜底：`ai_chat_enabled`/`ai_bot_enabled` 关闭或 LLM 报错 → 从静态内容池 `RobotContent(type=reply)` 随机取一条；池空则跳过该用户。

### 接口改造
扩展 robot 包既有 `MessageSender` 接口，新增：
```go
EnsureRobotChat(tenantID, botUserID, userID int64) (chatID int64, err error)
```
由 `chat.Service` 实现：找已有会话则返回，否则建会话（不走 `Debit`）。

### 触达可达性
近期活跃用户大概率 WS 在线，`SendMessage` 内 `hub.PushTo` 实时下发即可。机器人消息不触发微信订阅推送（沿用现状）。

### 边界 / 防骚扰
- `outreach_user_daily_cap` 限制单用户每日被触达上限。
- 抽样向下取整，小流量时可能为 0，符合「概率」语义。
- 时间窗 + tick 间隔避免全天高频打扰。

---

## 配置白名单（admin/meta.go 新增组）
新增 `Group: "增长运营"`：
- `checkin_enabled`(bool)、`checkin_coins`(int)
- `price_msg`(int)
- `share_reward_enabled`(bool)、`share_reward_coins`(int)、`share_reward_daily_limit`(int)
- `outreach_enabled`(bool)、`outreach_window_start`(int)、`outreach_window_end`(int)、`outreach_probability`(int)、`outreach_lookback_min`(int)、`outreach_tick_min`(int)、`outreach_user_daily_cap`(int)

前端 `Config.vue` 按 group 自动渲染，无需新增管理页。

## 客户端触点（uni-app，后续单独实现）
- **签到**：「我的」页或首页入口按钮 → `GET /api/checkin/status` 渲染态 → `POST /api/checkin` 领币 toast。
- **分享**：`onShareAppMessage` 成功回调后 `POST /api/share/reward`，按 `rewarded` toast。
- **消息扣费**：发消息收到 `ErrInsufficient` → 复用开聊的充值引导弹框（HTTP + WS 两路）。

## 主程序接线（cmd/api/main.go）
- 注册 `CheckinLog` 迁移、checkin handler 路由、share reward 路由。
- 启动 `robot.StartOutreachCron(...)`（注入 db、registry、llm、MessageSender）。
- 鉴权中间件加 last_active_at 去抖刷新。

## 数据流小结
```
签到:   POST /api/checkin → tx{INSERT CheckinLog(uk) + CreditTx} → WalletTxn(checkin)
消息:   SendMessage(真人) → Debit(price_msg, biz=建消息+更新会话) → WalletTxn(msg) → WS/push/bot
分享:   POST /api/share/reward → quota.TryConsume("share",limit) → Credit(M) → WalletTxn(share)
活跃:   请求 → 鉴权中间件 → Redis去抖 → UPDATE users.last_active_at
触达:   Cron(tick) → 窗口/开关校验 → 查近M分钟活跃真人 → 抽N% → EnsureRobotChat + LLM开场白 → SendMessage(bot)
```

## 不在本期范围（YAGNI）
- 连续签到累加奖励、补签。
- 分享回流落地页校验。
- 触达效果统计看板（点击率/回复率）。
- 多实例部署下触达去重的跨实例协调（当前单实例 Hub + Redis 计数已够）。
