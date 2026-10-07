# 机器人内容多样化 + 同城注入 + 主动引导 — 设计文档

- 日期：2026-07-05
- 分支：main
- 范围：三个相互独立的子课题，一起实现
  1. #1 漂流瓶内容多样化（修复同质化）
  2. #2 同城用户列表机器人注入（前 M 含 N，随机位置）
  3. #3 机器人主动引导聊天（四种行为）

## 0. 背景与证据

- 生产库现状（`ai_message`）：3397 个瓶子仅 **64 种内容**，机器人造 **3352** 个，最高一条重复 **1973 次**，top6 ≈ 96%。
- 根因：`robot/service.go throwBottles` → `genAIText(contentType="bottle")`，缓存键 `cacheKey("bottle", personaRole)`；`replycache.go maxVariants=5`。即"每人格角色最多 5 条变体 + 固定提示词"，反复命中同几条。
- 同城 `match/service.go CityUsers`：仅 `WHERE city=? ORDER BY last_active_at`，机器人无注入逻辑。
- 私聊机器人回复：`robot/bot_worker.go BotWorkerPool`（`MessageSender` 接口含 `EnsureRobotChat`/`SendMessage`/`HistoryRecent`），#3 复用它。

## 1. #1 漂流瓶内容多样化

### 机制：成长型内容池 + 随机取
- 造瓶不再走 5 条封顶的 reply-cache，改为维护 `robot_contents`(type='bottle') 作为**成长池**：
  - `throwBottles` 每次投瓶：
    - 若池条数 < 目标 `KeyRobotBottlePoolTarget`(默认 80) 且 AI 开启：调 LLM 生成新文案（**随机主题种子**，见下），入库前按 `text` 精确去重，写入 `robot_contents`(type='bottle', weight=1)，并用作本次瓶子内容；本次 tick 生成数上限 = 本次 throwN（避免突发）。
    - 否则：从池中**随机取**一条。
  - AI 关闭：直接从池随机取（含静态兜底）。
- **随机主题种子**：内置 ~20 个场景词（加班/失眠/旅行/美食/宠物/深夜/清晨/职场/校园/搬家/emo/晚风…）+ ~5 个语气词，随机组合进 prompt：
  `"以「{theme}」为主题，用{mood}的语气写一条真实、有温度的漂流瓶，50字以内，不要打招呼，不要加引号。"`
- **静态兜底池**：`SeedContent` 的 bottle 从 10 条扩到 ~40 条（多样化文案）。
- 效果：池大且随机取，投放彻底去同质；运营可在「机器人内容池」页查看/增删这些 AI 文案。

### 存量清理（一次性，可逆）
- 把机器人历史瓶子全部软删：`UPDATE bottles SET status='expired' WHERE user_id IN (SELECT user_id FROM users WHERE is_robot=1)`。
- 通过 DB 脚本执行（部署后运行一次），非代码路径；`status` 改回 `active` 即可恢复。

## 2. #2 同城用户列表机器人注入

### 机制：每页前 M 保 N，随机位置
- `CityUsers` 出 `cards` 后调用 `injectRobots(cards, ...)`：
  - `M = KeyCityRobotTopM`(默认 10)，`N = KeyCityRobotTopN`(默认 3)；`N<=0` 直接返回（关闭）。
  - `window = 前 min(M, len(cards))`；统计其中已有机器人数 `have`；`need = N - have`；`need<=0` 返回。
  - 取候选机器人：本租户、active、非自己、未拉黑、**不在当前 cards 中**；**同城优先**，随机序，取 `need` 个；不足时用**其他城市**机器人补齐。
  - 每个候选**插入到 `[0, M)` 内随机且互不重复的位置**（插入不替换，真人不丢；页长度可能为 size+need）。
  - 跨城补进来的机器人，其卡片 `city` 字段**显示为请求城市**（营造同城人气，符合功能意图）。
- 分页说明（已知限制）：不同页可能重复出现某些机器人（每页随机）；本期接受，后续可用会话/游标去重。

## 3. #3 机器人主动引导聊天

四种行为；**(b)(c)(d) 统一受总开关 `KeyRobotProactive`（默认关，运营灰度开启）约束**，(a) 随 AI 开启即生效。全部复用 `BotWorkerPool` 的打字延迟、Tier0 身份拦截、AI 自白兜底。

### (a) 多轮提问（提示词强化，风险最低）
- `bot_worker.go buildPrompt`（私聊）与 `service.go genAIText` 的 bottle/reply 提示词，追加：
  `"回复自然地以一个问题或话题钩子结尾，引导对方继续聊，但不要生硬、不要每句都问。"`

### (b) 回信后引导加聊
- `maybeReply`：机器人回真人瓶后，按 `KeyRobotBottle2ChatRatio`(默认 20，%) 概率：
  `EnsureRobotChat(robot, author)` → 发一句私聊开场（AI 生成，引用瓶子内容；失败用模板）。
- 受 `KeyRobotProactive` 总开关约束；每真人每次回信最多触发一次。

### (c) 主动破冰（新用户 + 沉默用户）
- `Tick` 新子任务 `proactiveOutreach`（在 `engage.go`）：
  - 目标真人：`is_robot=0 AND status=active AND tenant`，且满足 **新用户**(`created_at > now-KeyRobotOutreachNewHours`，默认 24h) **或 沉默用户**(`last_active_at < now-KeyRobotOutreachSilentDays`，默认 7d)；
  - 且**当前无任何机器人会话**（`chats` 中不存在与任一机器人的会话）→ 避免重复骚扰；
  - 每租户每日上限 `KeyRobotOutreachDailyCap`(默认 20)，用 Redis 计数键 `robot_outreach:{tenant}:{yyyymmdd}` 控量；
  - 每个目标：随机机器人 → `EnsureRobotChat` → 发开场私聊（AI 生成温暖开场，失败用模板）。

### (d) 沉默追问
- `Tick` 新子任务 `nudgeStalledChats`（`engage.go`）：
  - 候选会话：本租户、`updated_at ∈ [now - silentMin - 窗口, now - silentMin]`（`silentMin = KeyRobotNudgeSilentMin`，默认 30min；窗口如 60min，只抓"刚沉默"的，不反复扫历史）、且一方是机器人；
  - 用 `HistoryRecent` 计算**末尾连续机器人消息数** `trailing`；当 `1 <= trailing <= KeyRobotNudgeMax`(默认 1) 且用户确实是最后发言后沉默 → 追一句（AI 生成，引用上下文）；
  - `trailing > KeyRobotNudgeMax` 则停（靠消息历史天然限次，无需新列）。

### 发送通道
- `robot.Service` 在 `InitWorkerPool` 时获得 `MessageSender` 引用（新增 `Service.SetSender`），`engage.go` 用它建会话/发消息；(c)(d) 的发送同样经打字延迟。

## 4. 新增配置键（sysconfig + admin/meta）

| Key 常量 | 字符串键 | 默认 | 说明 |
|---|---|---|---|
| KeyRobotBottlePoolTarget | robot_bottle_pool_target | 80 | AI 瓶子池目标条数 |
| KeyCityRobotTopM | city_robot_top_m | 10 | 同城前 M 个位置 |
| KeyCityRobotTopN | city_robot_top_n | 3 | 其中机器人数(0=关) |
| KeyRobotProactive | robot_proactive_enabled | 0 | 主动引导总开关(b/c/d) |
| KeyRobotBottle2ChatRatio | robot_bottle2chat_ratio | 20 | 回信后引导加聊概率(%) |
| KeyRobotOutreachDailyCap | robot_outreach_daily_cap | 20 | 破冰每租户每日上限 |
| KeyRobotOutreachNewHours | robot_outreach_new_hours | 24 | 新用户窗口(小时) |
| KeyRobotOutreachSilentDays | robot_outreach_silent_days | 7 | 沉默用户阈值(天) |
| KeyRobotNudgeSilentMin | robot_nudge_silent_min | 30 | 会话沉默追问阈值(分) |
| KeyRobotNudgeMax | robot_nudge_max_per_chat | 1 | 每会话最多追问次数 |

- admin `meta.go` 加对应 ConfigField（分组「机器人」/「流量/同城」），后台配置页自动出现，可分租户调。

## 5. 涉及文件

- `server/internal/robot/service.go`：`throwBottles` 重写(成长池+随机主题)、`SeedContent` 扩池、`maybeReply` 加 (b)、`Tick` 挂 (c)(d)、`SetSender`。
- `server/internal/robot/engage.go`（新增）：`proactiveOutreach`、`nudgeStalledChats`、随机主题种子表、开场/追问模板。
- `server/internal/robot/bot_worker.go`：`buildPrompt` 加提问钩子；`InitWorkerPool` 调 `SetSender`。
- `server/internal/match/service.go`：`CityUsers` 加 `injectRobots`。
- `server/internal/sysconfig/sysconfig.go`：新增 Key 常量 + defaults。
- `server/internal/admin/meta.go`：新增 ConfigField。
- 一次性 DB 脚本：软删机器人旧瓶。

## 6. 测试与验证

- **单测**（纯逻辑，脱离 DB/LLM）：
  - `injectRobots` 位置注入（给定 cards + 机器人集，验证前 M 内机器人数=N、位置在 [0,M)、真人不丢、不重复）；
  - 随机主题种子生成（非空、含主题词）；
  - `trailing` 末尾连续机器人消息计数；
  - 破冰目标筛选的边界（新/沉默判定）——可对纯函数化的判定单测。
- **集成**：`go build ./... && go vet && go test ./...`；交叉编译部署；生产小流量：
  - #1：投放后查 `SELECT COUNT(DISTINCT content)` 应快速上升；
  - #2：`/city/users` 前 M 内机器人数=N；
  - #3：开总开关后观察会话新增/追问日志，确认不刷屏。
- **灰度**：`KeyRobotProactive` 默认关；上线后先小 `top_n`/`ratio`/`daily_cap`，观察后再放量。

## 7. 非目标（YAGNI）

- 同城机器人跨页去重（本期接受重复）。
- 机器人会话的情绪/记忆增强（沿用现有 MemoryStore）。
- 破冰/追问的多语言、AB 实验框架。
