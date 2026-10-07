# 主动匹配（火花）设计

> 日期：2026-10-07
> 范围：Go 后端新包 `internal/spark`、两个 Flutter 工程（`app/bottles_zh` 与 `app/bottles`）、`/config` 新分组
> 前置：`internal/robot/outreach.go`（机器人主动触达，本设计与它当天互斥）、`internal/chat`（会话与 WS Hub）

---

## 一、目标与现状

### 1.1 要解决什么

真实用户在线时，系统每隔一段随机时间主动给他配一个人——按概率是真人或机器人——在 App 上弹窗「XX 与你碰撞出了火花」，点进去直接进聊天，开聊不扣币。目的是让在线用户不必自己找人，降低冷启动的空场感。

### 1.2 现状

- `internal/robot/outreach.go` 已有一个很接近的东西：定时 tick、挑近期活跃真人、抽样 N%、让随机机器人 `EnsureRobotChat` + 发开场白。开关 `outreach_enabled` 默认关。它**只做机器人单向发消息，没有弹窗、不配真人、不涉及计费**。
- `internal/chat`：`Hub.OnlineUserIDs()` / `Hub.IsOnline()` / `Hub.PushTo(userID, payload)` 已有；`EnsureRobotChat(tenantID, botUserID, userID)` 建会话且**不扣费**；`StartChat` 走 `wallet.Debit` 扣 `price_chat`。
- `internal/match` 的 `injectRobots(cards, robots, m, n, pick)` 是请求驱动的比例混合，与本功能无关但思路可参照。
- 聊天扣费两笔：开聊 `price_chat`、每条 `price_msg`（默认 0）。

### 1.3 不在本设计范围

- 离线补推送（App 推送通道尚未接入）。不在线就不匹配。
- 按性别 / 兴趣 / 地理的匹配偏好。本版随机。
- 小程序端。本功能仅 App。
- 匹配效果的后台统计报表。

---

## 二、决策记录

| # | 决策 | 理由 |
|---|---|---|
| D1 | 新包 `internal/spark`，`outreach.go` 原样不动 | 触发条件（在线 vs 近期活跃）、送达方式（弹窗 vs 静默消息）、计费（免开聊费）、受众（能配两个真人）都不同；真人配真人不属于 robot 包 |
| D2 | 真人配对**双向弹窗** | 用户选定。两边都看到，谁先点谁开会话 |
| D3 | 仅 WS 在线时弹，不在线不匹配 | 用户选定，与「真实用户在线时候」的原话一致；App 推送通道未接 |
| D4 | 只免**开聊费**（`price_chat`），`price_msg` 照常 | 用户选定。`price_msg` 默认 0，等于现状全免；日后调价时这批会话不会成为收入缺口 |
| D5 | 机器人配对**预建会话并先发一条开场白**；真人配对**不预建会话** | 用户选定机器人先说话，开场白得有地方放；真人那边预建会让两人的消息列表各多一个空会话 |
| D6 | 真人配对点击后走 `POST /spark/accept`，**服务端校验配对记录存在才免费建会话** | 没有这道校验，任何人都能用这个接口与任意人免费开聊，`price_chat` 形同虚设 |
| D7 | 与 `outreach` **当天互斥**：共用现有计数键 `outreach:<租户>:<uid>:<日期>`，各自按自己的 cap 判断 | 用户选定。一方消耗的额度另一方看得见，用户不会被两套机制轮番打扰；沿用现有键名，不动 outreach 代码 |
| D8 | 状态全放 Redis，不加数据表 | 三类键都是当天/短时效的调度状态，不是业务记录；`outreach` 也是这么做的 |
| D9 | 每人的下次匹配时刻独立随机（`spark:next:<租户>:<uid>`，TTL = rand(N..M) 分钟），调度器固定每分钟扫一次 | 「每隔 N–M 分钟」是对**每个用户**而言；固定 tick + 每人 TTL 比给每人起一个定时器省得多 |
| D10 | 首次见到某在线用户只设 TTL、不匹配 | 否则用户一打开 App 就被弹窗糊脸 |
| D11 | 真人候选为空时**退回机器人**，不放弃这次机会 | 冷启动期在线真人就是少；放弃等于功能在最需要它的时候不工作 |
| D12 | 弹窗文案后台可配，支持 `{nickname}` 占位符，中英各一条 | 用户要求「文案是管理后端配置」；两端都要，沿用 App 文案键「默认空串 = 用内置 ARB」的惯例 |
| D13 | 两个 Flutter 工程同时实现 | 用户选定 |

---

## 三、服务端设计

### 3.1 `internal/spark` 包

```go
// Deps 外部依赖,全部以接口注入(便于测试不起真实 Hub / DB)
type Deps struct {
    DB        *gorm.DB
    Online    func() []int64                                            // chat.Hub.OnlineUserIDs
    Push      func(userID int64, payload interface{})                   // chat.Hub.PushTo
    EnsureChat func(tenantID, u1, u2 int64) (int64, error)              // chat.Service.EnsureFreeChat
    SendAs    func(tenantID, fromUserID, chatID int64, text, kind string) (int64, error) // chat.Service.SendMessage
    Opening   func(tenantID, botUserID int64) string                    // robot 开场白
    ClaimDaily func(tenantID, userID int64, cap int) bool               // robot.ClaimDailyReach(共用 outreach 计数键)
    IsBlocked func(a, b int64) bool                                     // moderation.Service.IsBlocked
}

func Start(d Deps, defaultTenant int64)       // 起常驻 goroutine
func (s *Service) Accept(tenantID, userID, peerID int64) (int64, error) // /spark/accept
```

纯函数（可测，不碰 DB / Redis / 时钟）：

```go
// pickKind 掷骰子:真人还是机器人。ratio 是真人概率(%),越界夹到 [0,100]。
func pickKind(ratio int, roll func(int) int) Kind            // KindReal / KindRobot

// nextInterval 下次匹配的间隔。min > max 时交换;都 <= 0 时回落 defaultInterval。
func nextInterval(minMin, maxMin int, roll func(int) int) time.Duration

// inWindow 当前小时是否在时段内(跨零点按 outreach 同款语义)
func inWindow(hour, start, end int) bool

// eligiblePeers 真人候选过滤:排除自己、机器人、非 active、已拉黑、当日已配过、配额满的。
// capLeft 是**只读**检查,真正占额度在选中之后的 ClaimDaily;两者之间的窗口里对方可能被
// 另一个 tick 占满——所以选中后 ClaimDaily 失败要换下一个候选,而不是直接放弃。
func eligiblePeers(self int64, online []candidate, pairedToday func(a, b int64) bool,
    blocked func(a, b int64) bool, capLeft func(int64) bool) []candidate

// pairKey 当日去重键,与顺序无关
func pairKey(tenantID, a, b int64) string
```

### 3.2 调度循环

```
每 60s:
  关 / 不在时段 → 跳过
  online := Online()  →  按租户查出这批 user(排除 is_robot / status != active / is_muted)
  for 每个在线真人 u:
      if Redis EXISTS spark:next:<t>:<u> → 跳过(还没到点)
      SET spark:next:<t>:<u> = 1 EX rand(N..M)*60          // 先设,再匹配:匹配失败也不要原地重试
      if SET spark:seen:<t>:<u> = 1 EX 24h NX 成功 → 本轮不匹配(首次见到,D10)
      if !ClaimDaily(t, u, spark_user_daily_cap) → 跳过
      kind := pickKind(spark_real_ratio, rand.Intn)
      if kind == Real:
          peers := eligiblePeers(...)
          if len(peers) == 0 → kind = Robot                  // D11
      分派到 matchRobot / matchReal
```

> ⚠️ 「首次见到」不能用 `spark:next` 的 `SET NX` 返回值判断：那个键每次到期都会重新被创建，「原先不存在」在第二次之后同样成立。所以另立一个长 TTL 的 `spark:seen:<t>:<u>`（24h）：`SET NX` 成功即本人今天头一回被扫到，本轮只设键不匹配。

### 3.3 机器人配对

```
bot := 随机取一个 is_robot=1 AND status=active 的同租户用户(排除当日已配过的)
chatID := EnsureChat(t, bot, u)            // 不扣费
text := Opening(t, bot)                     // 复用 robot 的开场白生成
SendAs(t, bot, chatID, text, "text")        // 机器人先说话
标记 pairKey(t, bot, u) 当日已配
Push(u, sparkPayload{ChatID: chatID, Peer: bot 资料, Title, Text})
```

### 3.4 真人配对

```
peer := 从候选里随机一个
ClaimDaily(t, peer, cap)  失败则换一个候选
标记 pairKey(t, u, peer) 当日已配(TTL 到当天 24:00)
Push(u,    sparkPayload{PeerID: peer, Peer: peer 资料, Title, Text})    // 无 chat_id
Push(peer, sparkPayload{PeerID: u,    Peer: u    资料, Title, Text})
```

### 3.5 `POST /api/spark/accept`（鉴权）

```
请求 {peer_id: "..."}  →  响应 {chat_id: "..."}

1. peerID 必须与自己同租户、存在、status=active
2. **校验 Redis EXISTS pairKey(tenantID, 自己, peerID)** —— 不存在直接 403「匹配已过期」
   这是防止任何人拿这个接口与任意人免费开聊的唯一闸门(D6)
3. EnsureChat(tenantID, 自己, peerID) → chatID(已存在则直接返回,天然幂等)
4. 返回 chat_id
```

机器人那路不调这个接口（弹窗载荷里已有 `chat_id`）。

### 3.6 `chat.Service` 的改动（加法）

`EnsureRobotChat` 泛化为 `EnsureFreeChat(tenantID, u1, u2) (int64, error)`——实现与现在逐字相同，只是名字不再暗示「必须有一方是机器人」。保留 `EnsureRobotChat` 作为薄包装，`outreach.go` 不改。

### 3.7 WS 载荷

```json
{
  "type": "spark",
  "chat_id": "123",            // 机器人配对才有;真人配对为空
  "peer_id": "456",
  "peer": {"id":"456","nickname":"…","avatar":"…"},
  "title": "有人和你对上眼了",
  "text": "小鱼 与你碰撞出了火花",
  "title_en": "Someone caught your eye",
  "text_en": "You and 小鱼 just sparked"
}
```

`title` / `text` 已在服务端把 `{nickname}` 替换完毕，客户端直接显示。语种按该连接登录态的 `Accept-Language`？WS 没有这个头——改为**推两份**（`title`/`text` 为中文，`title_en`/`text_en` 为英文），客户端按自己的 locale 选。空串表示后台没配，客户端用内置 ARB 文案。

### 3.8 配置（`/config` 新分组「主动匹配」）

`GroupSpark = "spark"`，`Section: SectionGrowth`，`Platform: PlatformApp`。全部进 `defaults` 与 `meta.go` 白名单（中英标签各一条）。

| 键 | 默认 | 类型 | 含义 |
|---|---|---|---|
| `spark_enabled` | `0` | bool | 总开关 |
| `spark_interval_min` | `20` | int | 每人下次匹配的最小间隔(分钟) |
| `spark_interval_max` | `60` | int | 最大间隔(分钟) |
| `spark_real_ratio` | `30` | int | 配到真人的概率 A(%) |
| `spark_window_start` | `10` | int | 时段开始小时 |
| `spark_window_end` | `23` | int | 时段结束小时(不含) |
| `spark_user_daily_cap` | `3` | int | 每人每日上限(与 outreach 共用计数) |
| `spark_title` / `spark_title_en` | `""` | text | 弹窗标题，空=用 App 内置文案 |
| `spark_text` / `spark_text_en` | `""` | text | 弹窗正文，支持 `{nickname}`，空=用内置 |

### 3.9 Redis 键

| 键 | TTL | 用途 |
|---|---|---|
| `spark:next:<租户>:<uid>` | rand(N..M) 分钟 | 下次匹配时刻 |
| `spark:seen:<租户>:<uid>` | 24h | 已见过(首轮不弹) |
| `spark:pair:<租户>:<小 uid>:<大 uid>` | 到当天 24:00 | 当日该对已配过；`/spark/accept` 的校验依据 |
| `outreach:<租户>:<uid>:<日期>` | 到当天 24:00 | **与 outreach 共用**的每日次数(D7) |

---

## 四、客户端设计（`app/bottles_zh` 与 `app/bottles` 各落一遍）

- `core/network/chat_socket.dart`：新增 `spark` 消息类型的解析与回调。
- 新文件 `lib/features/spark/spark_overlay.dart`：用全局 `navigatorKey` 在任意页面 `showDialog`——头像 + 昵称 + 标题 + 正文 + 「去聊聊」/「以后再说」。同一时刻只允许一个火花弹窗（已有弹窗时新的丢弃，不排队）。
- 点击「去聊聊」：
  - 载荷有 `chat_id` → 直接 `context.push(Routes.chat(chatId))`
  - 没有 → 调 `POST /spark/accept` 拿 `chat_id` 再跳；失败（如匹配已过期）toast 提示
- 文案：服务端给空串时用 ARB 内置 `sparkTitle` / `sparkBody`（`{nickname}` 占位）。
- 新增 `SparkRepository.accept(peerId)` 到 `data/repositories.dart` 三件套（接口 / remote / mock）。

---

## 五、要改的既有代码

**server**
- 新增 `internal/spark/{service.go,schedule.go,handler.go,*_test.go}`
- `internal/chat/service.go`：`EnsureFreeChat`（`EnsureRobotChat` 改为包装）
- `internal/robot/outreach.go`：`claimOutreach` 导出为 `ClaimDailyReach`（供 spark 共用计数键）
- `internal/sysconfig/sysconfig.go`：10 个新键 + defaults
- `internal/admin/meta.go`：10 条白名单 + `GroupSpark`
- `cmd/api/main.go`：装配 `spark.Start(...)` 与路由

**app（两个工程各一遍）**
- `lib/core/network/chat_socket.dart`、`lib/features/spark/spark_overlay.dart`（新）、`lib/app/router.dart`（navigatorKey 若未暴露）、`lib/data/repositories.dart` + `remote/` + `mock/`、`lib/l10n/*.arb`

---

## 六、测试策略

- `spark`：`pickKind`（0/100/越界/中间值，注入确定性 roll）、`nextInterval`（min>max 交换、全 0 回落、边界）、`inWindow`（跨零点）、`eligiblePeers`（逐条排除规则各一例）、`pairKey`（顺序无关）。
- `/spark/accept`：**未匹配过的 peer 必须 403**（防白嫖闸门）；已匹配 → 返回 chatID；重复调用幂等返回同一 chatID。
- 调度：注入假 `Online` / 假时钟 / 假 roll，断言「首次见到不弹」「到点才弹」「真人候选为空退回机器人」「配额用尽跳过」。
- `chat`：`EnsureFreeChat` 不产生钱包流水（现有 `EnsureRobotChat` 行为不变）。
- `sysconfig` / `admin`：新键默认值与白名单（`meta_test.go` 自动覆盖）。
- app：`spark_overlay_test.dart`（载荷 → 弹窗内容、有/无 chat_id 的两条点击路径、同时来两条只弹一个）；`flutter analyze` 零告警。

---

## 七、遗留

- 离线用户的补推送（等 App 推送通道接入）。
- 匹配偏好（性别 / 兴趣 / 距离）。
- 后台的匹配效果统计（弹出数 / 点击率 / 转化成聊天数）。
- 小程序端。
