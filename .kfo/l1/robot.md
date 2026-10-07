# L1 设计 — 运营机器人(robot)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-24 |
| 覆盖 | `server/internal/robot/*` · `model.RobotContent/PersonaConfig/RobotMemory/RobotKeywordRule/RobotReplyCache` · `model.User.IsRobot/PersonaID` |
| 关联 | `l1/admin-platform.md`(配置) · `l1/bottle-feed.md`(机器人降权) · `l1/chat-ws.md`(hub.PushTo 复用) · `l2/2026-06-23-ai-persona-robot-design.md`(AI 层执行计划) · `l2/2026-06-24-robot-persona-chat-fixes.md` · `l2/2026-06-24-robot-identity-fix.md` · `l3/2026-06-24-llm-history-in-systemprompt.md` · `l3/2026-06-24-reply-cache-poisoning.md` |

---

## 一、定位

注入运营机器人，让海洋/同城在真人不足时也"有人气"：按管理台配置**定时投放瓶子**、**对真人瓶概率回信**、**接聊天自动续接**（PROPOSED）。机器人即 `IsRobot=true` 的 `User`，内容取自 `robot_content` 静态池（已实现）或 AI 人格引擎（PROPOSED）。

---

## 二、数据

### 已实现

- `User.IsRobot`：标记机器人；`feed.go` 打分对其 **−wRobot 降权**（避免满屏机器人）。
- `RobotContent(tenant_id, type[bottle/reply], text, tags, weight)`：静态内容池；空时 `SeedContent` 灌默认文案，运营在管理台维护。
- 机器人资料：`EnsureRobots` 按昵称前后缀池 + 城市池随机生成，头像留空（前端渐变默认头像），注册时间打散。

### PROPOSED — AI 人格层新增字段/表

- `User.PersonaID`（nullable bigint）：绑定人格配置，NULL = 使用默认人格。
- `persona_configs`：人格配置模板。每个机器人绑定一个 PersonaConfig，可多个机器人共用同一模板。

  | 字段 | 说明 |
  |---|---|
  | persona_id | PK |
  | tenant_id | 租户隔离 |
  | bot_user_id | 绑定机器人 User，NULL = 模板 |
  | name | 人格名称 |
  | relationship_role | friend / partner / companion / mentor |
  | affective_style | warm_soft / calm / energetic / playful / dominant |
  | voice_style | short_sentence / casual / structured / expressive |
  | rules_json | `{"do":["..."],"dont":["..."]}` |
  | status | active / paused |

- `robot_memory`：用户×机器人关系记忆，属于用户不属于机器人，同一用户对不同机器人独立记录。

  | 字段 | 说明 |
  |---|---|
  | user_id + bot_user_id | 唯一索引 |
  | familiarity | 0.0–1.0，每次对话 +0.01，驱动语气亲密度 |
  | preferences_json | 用户稳定偏好摘要 |
  | session_summary | 上次会话压缩摘要（<512 tokens） |

- `robot_keyword_rules`：Tier 1 关键字规则，Admin 精确维护，命中直接返回，**不调 LLM**。

  | 字段 | 说明 |
  |---|---|
  | priority | 越大越先匹配 |
  | match_type | contains / exact / prefix |
  | keywords_json | `["你好","hi","hello"]`，OR 关系 |
  | responses_json | `{"warm_soft":["变体A","变体B"],"default":["通用"]}` |
  | hit_count | 命中计数，供运营分析 |

- `robot_reply_cache`：Tier 2 LLM 自积累缓存，每次 LLM 生成后异步写入，下次相同问题直接命中。

  | 字段 | 说明 |
  |---|---|
  | question_hash | SHA256(normalize(text)\|persona_role)[:8] hex，16 位 |
  | persona_role | 缓存按角色级别共享，100 个同角色机器人共用 |
  | question_sample | 原始问题示例，admin 查看用 |
  | responses_json | 最多 5 条变体，随时间积累，随机取一条 |
  | source | ai_generated / manual（admin 编辑后改为 manual） |
  | hit_count | 命中计数 |

---

## 三、调度(scheduler.go) — 已实现

- 启动后 +10s 首跑，之后每 `intervalMin`（=1min）一次；多租户则遍历 `tenant` 表，单租户只跑 default；每租户 tick 包 `recover`，互不影响。
- **总开关 `sysconfig.robot_enabled`**（默认 0 关闭），关闭时 `Tick` 立即返回，调度器空转无副作用。

---

## 四、Tick 逻辑 — 已实现（AI 分支 PROPOSED）

1. 读配置：`robot_count` / `robot_throw_per_hour` / `robot_reply_ratio`。
2. `SeedContent` + `EnsureRobots(count)`（每次最多新建 10 个）。
3. **投放**（`throwBottles`）：速率累加器精确还原"每小时 N 个"。每个瓶子随机机器人 + 内容。
   - 已实现：随机取 `robot_content` 静态池
   - PROPOSED：若 `ai_bot_enabled=1` → 先查两级缓存 → 未命中则调 LLM 生成，失败 fallback 静态池
4. **回信**（`maybeReply`）：取最近真人 active、`reply_count<3` 的瓶子最多 5 条，各按 `reply_ratio%` 概率回一条。
   - 已实现：随机取 `robot_content` 静态池
   - PROPOSED：若 `ai_bot_enabled=1` → 先查两级缓存 → 未命中则调 LLM，失败 fallback 静态池

---

## 四·5、聊天续接 — PROPOSED（路径 C，全新）

用户给机器人发消息后，机器人自动回复。调用链（HTTP 立即返回，AI 异步完成）：

```
chat.Service.SendMessage()
  ├─ 存消息 + hub 推送（不变）
  └─ if partner.IsRobot → BotReplyQueue <- job（4 行新增）

      Worker 异步：
      Tier 0: isIdentityQuestion(text)                   → 命中直接返回否认话术（不走缓存/LLM）
      Tier 1: KeywordMatcher.Match(text, personaRole)    → 命中直接返回
      Tier 2: ReplyCache.Get(hash, personaRole)          → 命中且无AI自白直接返回
      Tier 3: PromptBuilder.Build + LLMClient.Chat       → LLM 生成
              → go ReplyCache.AddVariant(...)            → 异步写缓存（AI自白拒绝写入）
      time.Sleep(ai_reply_delay_ms)                      → 模拟打字
      chat.Service.SendMessage(botID, chatID, text)      → 存消息
      hub.PushTo(userID)                                 → 实时推送
      go MemoryStore.UpdateAsync(...)                    → 更新记忆
```

BotReplyQueue 容量 1000，Worker 并发 = `llm_concurrency`（默认 10）。队列满则丢弃，不影响用户正常发消息。

---

## 四·6、两级回复缓存 — PROPOSED

**消息归一化：** 转小写 → 去标点符号 → 保留汉字/字母/数字 → 合并空白。"你好！" / "你好~" 归一化后相同，命中同一缓存。

**Tier 0 身份硬拦截（最高优先级，2026-06-24 新增）：**
- 代码硬编码，不走 DB / 缓存 / LLM，零延迟
- 触发词：机器人 / 人工智能 / 真人 / AI / GPT / Claude / 语言模型 / 大模型 / 智能助手 / 虚拟助手 等
- 随机返回 7 条否认话术之一（`buildDenyResponse`）
- 目的：彻底堵死"你是机器人吗"类问题，不依赖 LLM prompt 遵循性

**Tier 1 关键字规则（次高优先）：**
- Admin 预设，支持 contains / exact / prefix 三种匹配
- 按 `persona_role` 存多条变体，运行时随机选一条
- 命中直接返回，零 LLM 消耗
- 覆盖目标：打招呼 / 问年龄 / 情感安慰等高频场景

**Tier 2 LLM 自积累缓存（次优先）：**
- 每次 LLM 响应后异步写入，下次命中跳过 LLM
- 缓存 Key = SHA256(normalized_text | persona_role)[:8] hex
- 同一 Key 最多积累 5 条变体（多次 LLM 响应追加），随机选取
- 同角色所有机器人共享同一缓存桶，命中率最大化
- **读取时过滤**：命中条目含 AI 自白短语则返回 "" 强制走 LLM（见 `containsAIConfession`）
- **写入时过滤**：LLM 回复含 AI 自白短语拒绝写入，防止缓存污染
- Admin 可查看/编辑/升级为关键字规则

---

## 五、配置项

### 已实现（管理台「机器人」组）

`robot_enabled`（开关） · `robot_count`（数量） · `robot_throw_per_hour`（投放速率） · `robot_reply_ratio`（回信概率%）。改动即时生效（sysconfig 热刷新）。

### PROPOSED — 管理台新增「AI 引擎」组

| Key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `ai_bot_enabled` | bool | 0 | AI 回复总开关，关闭则 fallback 静态池 |
| `ai_chat_enabled` | bool | 0 | 聊天续接 AI 独立开关（路径 C） |
| `llm_api_endpoint` | text | — | LLM API 地址（OpenAI 兼容）|
| `llm_api_key` | text | — | API Key，AES-GCM 加密存储，不在 GET 接口返回明文 |
| `llm_model` | text | gpt-4o-mini | 模型名 |
| `llm_temperature` | int | 8 | 实际 = value/10 |
| `llm_max_tokens` | int | 200 | 单次回复 token 上限 |
| `ai_reply_delay_min` | int | 1500 | 模拟打字最小延迟（ms） |
| `ai_reply_delay_max` | int | 4000 | 模拟打字最大延迟（ms） |
| `llm_concurrency` | int | 10 | 同时 LLM 调用上限 |

---

## 六、Go 文件（已实现 ✅）

| 文件 | 职责 |
|---|---|
| `robot/normalize.go` | normalizeText + cacheKey（SHA256 截 8 字节） |
| `robot/registry.go` | BotRegistry：map[userID]PersonaConfig，Load/Get/Reload |
| `robot/memory.go` | MemoryStore：GetOrCreate / UpdateAsync |
| `robot/keymatcher.go` | KeywordMatcher：加载 robot_keyword_rules，Match(text, role) |
| `robot/replycache.go` | ReplyCache：Get / AddVariant / IncrHit；读写均过滤 AI 自白（`containsAIConfession`） |
| `robot/llmclient.go` | `Chat(ctx, systemPrompt, history []llmMessage, userText)` — 历史以 user/assistant 消息结构传入，不嵌入 system |
| `robot/promptbuilder.go` | `buildPrompt(persona, memory)` — 不含历史；`buildHistory()` 净化 AI 自白；`containsAIConfession()` 共用判断函数 |
| `robot/bot_worker.go` | BotReplyQueue chan + Worker pool；Tier 0 身份拦截（`isIdentityQuestion` + `buildDenyResponse`） |

已有文件修改：`service.go`（throwBottles/maybeReply 加 AI 分支） · `chat/service.go`（+4 行） · `sysconfig/sysconfig.go`（+10 Key） · `model/model.go`（+4 结构体）

### LLM 调用结构（关键约束）

```
messages = [
  {role: "system",    content: 身份规则+禁止，不含历史},
  {role: "user",      content: 历史消息1},    ← buildHistory 转换
  {role: "assistant", content: 历史回复1},    ← AI自白短语已净化
  ...
  {role: "user",      content: 当前消息},
]
```

> ⚠️ 禁止将历史对话嵌入 system prompt 文本 → 会导致 LLM 优先维持历史方向，覆盖身份禁令。见 L3 `l3/2026-06-24-llm-history-in-systemprompt.md`。

---

## 七、管理台新增页面（PROPOSED）

| 路由 | 后端 | 功能 |
|---|---|---|
| `/admin/persona` | `/admin/api/persona/*` | 人格库 CRUD，保存后触发 BotRegistry.Reload() |
| `/admin/robot/profiles` | `/admin/api/robot/profile/*` | 机器人档案，绑定人格，启用/暂停，统计 |
| `/admin/keyword-rules` | `/admin/api/keyword-rules/*` | 关键字规则 CRUD，批量导入，优先级排序 |
| `/admin/reply-cache` | `/admin/api/reply-cache/*` | 缓存管理，按命中排序，一键升级为规则，清理 |

---

## 八、验证（已实现部分，2026-06-21）

开启后 1 个 tick：投放 10 瓶 + 回信 5 条（600/h、reply=100% 测试值）；用户数 +5 机器人；C 端捞瓶可见机器人瓶且被降权排后。已重置为安全默认（关闭）。

---

## 八·6、回复去重与素人化（2026-07-21）

- **回复雷同根因**：三层生成(关键字→缓存→LLM)键只含(消息, personaRole)不含机器人个体，且缓存命中即返回、变体池长期停留 1 条 → 同瓶/跨机器人/跨会话一字不差。
- **修复**：`GetVariants`+`pickVariant(exclude)` 取代 `Get`；变体 <5 时按 `cacheRegenProb=40%` 穿透 LLM 积累变体；聊天防复读扩到近 5 条(`lastBotMessages`)；瓶子同瓶查重(`genAIText` 带 exclude)；opener/破冰/追问文案变体化。
- **素人化**：部分机器人改 `用户XXXXXX` 昵称 + 空头像 + 未认证，仿真实用户。配套：`EnsureRobots`/`ReassignAvatarsByGender` 补头像仅对 `is_verified=1` 生效，否则调度每分钟把手动清空的头像补回。
- 详见 [l2/2026-07-21](../l2/2026-07-21-admin-audit-tags-coins-mobile.md)。

## 九、待办 / 状态

- ✅ 管理台内容池维护页（增删改 `robot_content`）
- ✅ 机器人 AI 人格引擎：人格绑定 + 两级缓存 + 聊天续接（路径 C）
- ✅ 管理台 Persona / RobotProfiles / KeywordRules / ReplyCache / RobotChats 页面
- ✅ 系统 prompt 身份加固：以真实人类自居 + 末尾硬性禁止块
- ✅ LLM 历史结构修复：history 改为 user/assistant 独立消息 + AI 自白净化
- ✅ Tier 0 身份硬拦截：`isIdentityQuestion` + `buildDenyResponse`，彻底堵死"你是机器人吗"类追问
- ✅ 缓存污染清除：`ReplyCache.Get/AddVariant` 双向过滤 AI 自白；历史污染条目已从 DB 删除
- **OPEN** 投放时间分布拟人化（白天多、深夜少，按 sysconfig 配置时间段权重）
- **OPEN** LLM 供应商选型 / 机器人聊天是否豁免金币扣费 / familiarity 上限值
