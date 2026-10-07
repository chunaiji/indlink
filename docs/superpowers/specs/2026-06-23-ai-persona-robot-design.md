# 机器人 AI 人格化对话引擎 · 集成设计

> 版本：V1 草案 · 日期：2026-06-23  
> 范围：在现有 `robot/` 模块基础上叠加 AI 人格层，实现三条触发路径的智能回复；引入两级回复缓存（关键字规则 + LLM 自积累缓存）降低 60%+ 高频问题的 LLM 调用；后台参数维护复用现有 Admin 管理台。

---

## 0. 背景与现状

现有机器人系统（`robot/service.go`）已具备基础骨架：

| 现有能力 | 实现方式 |
|---|---|
| 机器人账号管理 | `EnsureRobots()` 按配置数量创建 `is_robot=true` 的 User |
| 定时投放瓶子 | `Tick()` 每分钟运行，速率累加器换算投放数 |
| 被动回信 | `maybeReply()` 对真人瓶按比例随机回一条 |
| 内容池 | `robot_content` 表，Admin 已有 CRUD 页面 |
| 调度器 | `scheduler.go` 支持单/多租户 |

**缺失：**
- 所有内容均为静态文本随机取，无语境感知
- 无人格差异——所有机器人说话方式完全相同
- 用户在聊天框回复机器人后无响应（断链）
- 无记忆——每次回复对同一用户都像第一次见面

---

## 1. 核心决策

| 维度 | 结论 |
|---|---|
| 改造方式 | 叠加，不替换。现有静态池保留为 AI 失败时的 fallback |
| AI 触发开关 | `ai_bot_enabled`（全局）+ `ai_chat_enabled`（聊天独立）两级开关 |
| LLM 接口 | OpenAI 兼容接口，通过 sysconfig 配置 endpoint/key/model |
| 人格绑定粒度 | 每个机器人 User 可绑定一个 PersonaConfig（允许空=使用默认） |
| 记忆归属 | 记忆属于用户，不属于机器人。同一用户与不同机器人的记忆相互独立 |
| 并发控制 | semaphore 限制同时进行的 LLM 调用数（防费用突刺），通过 `llm_concurrency` sysconfig 配置 |
| **回复缓存** | **两级缓存：Tier1 关键字规则（Admin 预设）+ Tier2 LLM 回复自积累。预计覆盖 60%+ 高频问题，显著减少 LLM 调用** |
| **缓存 Key 粒度** | **`persona_role`（不是 `persona_id`）+ 归一化消息 hash。同一角色的所有机器人共享缓存，命中率最大化** |
| chat/service.go 改动量 | 仅在 `SendMessage()` 末尾增加 ~4 行（is_robot 检测 + 投队列） |

### 1.1 V1 不做

- 多 Persona 混合权重（幽默 + 专业叠加）
- Persona A/B 测试框架
- 用户画像向量化（User Embedding）
- 机器人"自动转接"到人工客服
- 对话质量评分 / 效果统计面板

---

## 2. 三条触发路径

```
路径 A · 主动投瓶（Cron）
  Tick() → throwBottles() → [AI 生成内容 | fallback 静态池] → 写 bottles 表

路径 B · 被动回信（Event）
  Tick() → maybeReply() → [AI 生成回复 | fallback 静态池] → 写 bottle_replies 表

路径 C · 聊天续接（全新）
  用户发消息 → SendMessage() → 检测 is_robot → BotReplyQueue → Worker
    → [Tier1 关键字规则命中? → 直接返回]
    → [Tier2 缓存命中? → 直接返回]
    → LLM → 异步写缓存 → 存消息 + hub 推送
```

三条路径相互独立，各有开关，出问题互不影响。  
路径 A/B 同样先走两级缓存查找，未命中才调 LLM。

---

## 3. 数据模型变更

### 3.1 新增表：`persona_configs`

```sql
CREATE TABLE persona_configs (
    persona_id        BIGINT PRIMARY KEY,
    tenant_id         BIGINT NOT NULL,
    bot_user_id       BIGINT,          -- 绑定到具体机器人，NULL=模板
    name              VARCHAR(64) NOT NULL,
    relationship_role VARCHAR(32),     -- friend/partner/companion/mentor
    affective_style   VARCHAR(32),     -- warm_soft/calm/energetic/playful/dominant
    voice_style       VARCHAR(32),     -- short_sentence/casual/structured/expressive
    rules_json        TEXT,            -- {"do":["..."],"dont":["..."]}
    status            VARCHAR(16) DEFAULT 'active',
    created_at        DATETIME,
    updated_at        DATETIME,
    INDEX idx_tenant (tenant_id),
    INDEX idx_bot (bot_user_id)
);
```

### 3.2 新增表：`robot_memory`

```sql
CREATE TABLE robot_memory (
    id                BIGINT PRIMARY KEY,
    tenant_id         BIGINT NOT NULL,
    user_id           BIGINT NOT NULL,
    bot_user_id       BIGINT NOT NULL,
    familiarity       FLOAT DEFAULT 0.3,   -- 0~1，驱动语气亲密度
    preferences_json  TEXT,                -- {"likes_short":true, "tone":"soft"}
    session_summary   TEXT,                -- 上次会话压缩摘要（<512 tokens）
    updated_at        DATETIME,
    UNIQUE INDEX uk_user_bot (user_id, bot_user_id)
);
```

**设计要点：** 一个用户对一个机器人只有一条记录。familiarity 随每次对话微增（+0.01），不同机器人的 familiarity 独立累计。

### 3.3 修改表：`users` 增加一列

```sql
ALTER TABLE users ADD COLUMN persona_id BIGINT NULL COMMENT '绑定人格，NULL=使用默认';
```

### 3.4 不变：`robot_content`

角色从"主力内容"降级为"AI fallback 备用池"，Admin 已有的 CRUD 界面继续使用，无需修改。

### 3.5 新增表：`robot_keyword_rules`（Tier 1 — Admin 预设规则）

Admin 精确维护的高频问题→答案映射，优先级最高，命中直接返回，**不消耗 LLM**。

```sql
CREATE TABLE robot_keyword_rules (
    rule_id        BIGINT PRIMARY KEY,
    tenant_id      BIGINT NOT NULL,
    priority       INT DEFAULT 0,          -- 越大越优先，同优先级按 rule_id 排
    match_type     VARCHAR(16) NOT NULL,   -- contains / exact / prefix
    keywords_json  TEXT NOT NULL,          -- ["你好","hi","hello","嗨"] 任意一个命中即触发
    responses_json TEXT NOT NULL,          -- {"warm_soft":["回复A","回复B"],"calm":["回复C"],"default":["通用回复"]}
                                           -- 按 persona_role 存变体；没有对应角色时取 "default"
    hit_count      INT DEFAULT 0,
    status         VARCHAR(16) DEFAULT 'active',
    created_at     DATETIME,
    updated_at     DATETIME,
    INDEX idx_tenant_status (tenant_id, status)
);
```

**设计要点：**
- `keywords_json` 是 OR 关系，任意一个关键词命中该条规则即触发
- `responses_json` 按 `persona_role` 存各自的回复变体数组，取时随机选一条；无对应角色时取 `"default"` 键
- `match_type = contains`：消息中包含关键词即命中（适合"你好""谢谢"等）
- `match_type = exact`：归一化后完全相等（适合"你是AI吗"等精确问句）
- `match_type = prefix`：消息以关键词开头（适合"帮我""请问"等引导语）
- 规则按 `priority DESC, rule_id ASC` 排序，取第一条命中结果

### 3.6 新增表：`robot_reply_cache`（Tier 2 — LLM 自积累缓存）

每次 LLM 生成回复后异步写入，下次相同问题直接命中缓存，无需再调 LLM。

```sql
CREATE TABLE robot_reply_cache (
    cache_id        BIGINT PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    persona_role    VARCHAR(32) NOT NULL,  -- 按角色共享，不按具体机器人
    question_hash   CHAR(16) NOT NULL,     -- SHA256(normalize(text) + "|" + persona_role)[:8] hex
    question_sample VARCHAR(200),          -- 原始问题示例（admin 查看用，不参与匹配）
    responses_json  TEXT NOT NULL,         -- ["变体1","变体2",...] 最多 5 条，随机取一条
    hit_count       INT DEFAULT 0,
    source          VARCHAR(16) DEFAULT 'ai_generated',  -- ai_generated / manual（admin 编辑后改为 manual）
    status          VARCHAR(16) DEFAULT 'active',        -- active / disabled
    created_at      DATETIME,
    updated_at      DATETIME,
    UNIQUE INDEX uk_hash_role (tenant_id, question_hash, persona_role),
    INDEX idx_tenant_hits (tenant_id, hit_count)
);
```

**设计要点：**
- **缓存 Key = `question_hash`（16位hex）+ `persona_role`**，同一角色的所有机器人共享。100 个 warm_soft 机器人面对同一个问题，只需调一次 LLM，后续全部命中缓存。
- **归一化规则：** 转小写 → 去除标点/空格 → 保留汉字+字母+数字 → 合并空白。"你好！" / "你好~" / "你 好" 归一化后相同。
- **变体积累：** 每次 LLM 生成新内容，若该 hash 已有缓存且变体 < 5 条，则 append；达到 5 条后不再追加。随着时间积累，同一问题会有多个变体，避免用户每次收到相同回复。
- **命中时：** `hit_count++`（异步），随机取一条变体返回。
- **Admin 操作：** 可编辑变体文案（source 改为 manual）、禁用条目、一键"升级为关键字规则"（将高频缓存转为精确控制的规则）。

---

## 4. 新增 Go 文件清单

| 文件 | 职责 |
|---|---|
| `robot/registry.go` | BotRegistry：map[userID]PersonaConfig，提供 Load/Get/Reload |
| `robot/memory.go` | MemoryStore：GetOrCreate / UpdateAsync，操作 robot_memory 表 |
| `robot/llmclient.go` | HTTP 调用 LLM API（OpenAI 兼容），semaphore 并发控制，sysconfig 读配置 |
| `robot/promptbuilder.go` | Build(persona, memory, history) → LLMRequest，familiarity 驱动亲密度 |
| `robot/keymatcher.go` | KeywordMatcher：加载 robot_keyword_rules，Match(text, personaRole)，内存缓存规则列表，支持热重载 |
| `robot/replycache.go` | ReplyCache：Get(hash, role) / AddVariant(hash, role, text)，先查 DB，后续可加 Redis 层 |
| `robot/normalize.go` | normalizeText(text) string：小写+去标点+合并空白；cacheKey(text, role) string：SHA256 截 8 字节 hex |
| `robot/bot_worker.go` | BotReplyQueue chan + Worker goroutine pool，路径 C 完整调用链（含两级缓存查找） |

### 4.1 修改已有文件

| 文件 | 修改内容 | 改动量 |
|---|---|---|
| `robot/service.go` | throwBottles/maybeReply 加 AI 分支，失败 fallback 静态池 | ~30 行 |
| `chat/service.go` | SendMessage 末尾：if partner.IsRobot → BotReplyQueue <- job | ~4 行 |
| `sysconfig/sysconfig.go` | 增加 10 个 AI 引擎 Key 常量 + 默认值 | ~15 行 |
| `model/model.go` | PersonaConfig、RobotMemory、RobotKeywordRule、RobotReplyCache 结构体；User 加 PersonaID | ~50 行 |
| `bootstrap/` | Migration SQL | 新增文件 |

---

## 5. 路径 C 详细调用链（含两级缓存）

用户发消息后，HTTP 立即返回，AI 回复异步完成。两级缓存查找在进入 LLM 之前，预计拦截 60%+ 请求：

```
用户 POST /api/chat/:id/send
  └─ chat.Service.SendMessage()
       ├─ 存 UserMessage（现有逻辑不变）
       ├─ hub.PushTo(userID, userMsg)            ← 用户自己的消息实时显示
       └─ if partner.IsRobot:
            └─ BotReplyQueue <- BotJob{...}      ← 4 行新增，HTTP 立即返回

                   ↓（BotReplyWorker 异步消费）

            BotRegistry.Get(botUserID)           → PersonaConfig{role, ...}

            ──── Tier 1：关键字规则查找 ────────────────────────────────
            KeywordMatcher.Match(userMsg, personaRole)
              ├─ 命中 → 随机取变体 → responseText    ← 直接跳到"发送"，不调 LLM
              └─ 未命中 ↓

            ──── Tier 2：LLM 缓存查找 ──────────────────────────────────
            key = cacheKey(normalize(userMsg), personaRole)
            ReplyCache.Get(tenantID, key, personaRole)
              ├─ 命中 → 随机取变体 → responseText    ← 直接跳到"发送"，不调 LLM
              │          go ReplyCache.IncrHit(key)  ← 异步更新命中计数
              └─ 未命中 ↓

            ──── Tier 3：LLM 调用 ──────────────────────────────────────
            MemoryStore.GetOrCreate(userID, botUserID)  → RobotMemory
            chat.Service.History(chatID, limit=5)       → []Message
            PromptBuilder.Build(persona, memory, history) → LLMRequest
            LLMClient.Chat(ctx, req)                    → responseText
              ├─ 成功 → go ReplyCache.AddVariant(tenantID, key, personaRole, responseText)
              └─ 失败 → fallback：随机取 robot_content 表一条（不写缓存）

            ──── 发送 ──────────────────────────────────────────────────
            time.Sleep(rand(ai_reply_delay_min, ai_reply_delay_max))  ← 模拟打字
            moderation.CheckText(responseText)
              └─ 不通过 → fallback 静态池（不写缓存）
            chat.Service.SendMessage(botID, chatID, responseText)
            hub.PushTo(userID, botMsg)                  ← 用户收到机器人消息

            go MemoryStore.UpdateAsync(userID, botUserID, signals)    ← 异步更新记忆
```

**缓存命中时的延迟：** Tier 1/2 命中同样要执行 `time.Sleep` 延迟，避免缓存回复比 LLM 回复快得不自然。

**并发保护：** BotReplyQueue 容量 1000，Worker goroutine 数 = `llm_concurrency`（默认 10）。队列满则丢弃（极端情况下机器人不回，不影响用户正常发消息）。

---

## 5a. 消息归一化与缓存 Key 算法

```go
// robot/normalize.go

// normalizeText 用于匹配和 hash，不影响发送给 LLM 的原始文本。
func normalizeText(text string) string {
    text = strings.ToLower(strings.TrimSpace(text))
    // 只保留：汉字 + 字母 + 数字 + 空格，去掉所有标点/表情
    re := regexp.MustCompile(`[^\p{Han}a-z0-9\s]`)
    text = re.ReplaceAllString(text, "")
    // 合并多余空白
    text = strings.Join(strings.Fields(text), " ")
    return text
}

// cacheKey 生成 16 位 hex，作为 robot_reply_cache.question_hash
func cacheKey(text, personaRole string) string {
    normalized := normalizeText(text)
    sum := sha256.Sum256([]byte(normalized + "|" + personaRole))
    return hex.EncodeToString(sum[:8]) // 8 bytes = 16 hex chars
}
```

**归一化示例（相同 hash）：**

| 用户输入 | 归一化结果 |
|---|---|
| `你好！` | `你好` |
| `你好~` | `你好` |
| `你 好 啊` | `你好啊` |
| `Hi！` | `hi` |
| `你是谁呀？` | `你是谁呀` |
| `你是谁～～` | `你是谁` |

**归一化示例（不同 hash）：**

| 输入 A | 输入 B | 原因 |
|---|---|---|
| `你是谁` | `你叫什么` | 表达不同，语义匹配不在此层做 |
| `warm_soft 的 你好` | `calm 的 你好` | persona_role 不同 |

> **注：** 归一化只做字面层面简化，不做语义相似度计算（避免引入向量 DB 依赖）。语义层的合并通过 Tier 1 关键字规则来覆盖（Admin 把"你是谁"/"你叫什么"/"介绍一下自己"归入同一条规则）。

---

## 6. Prompt 构建规则

System Prompt 由三部分拼接：

```
【人格】
你是一个对话陪伴助手，当前人格：{persona.name}
关系定位：{relationship_role}
情绪风格：{affective_style}
表达方式：{voice_style}

【互动规则】
必须做：{rules.do[]}
不能做：{rules.dont[]}
当前亲密度：{familiarity}（0=陌生，1=高度亲密）

【用户记忆】
{preferences_summary}
{session_summary}

【历史对话（最近5轮）】
{history}
```

**familiarity 与语气映射：**

| familiarity | 对应语气 |
|---|---|
| 0.0–0.3 | 礼貌、有距离感 |
| 0.4–0.6 | 轻度关心，自然口语 |
| 0.7–0.9 | 轻撒娇、情绪表达增多 |
| 1.0 | 高亲密，仍克制不过度 |

---

## 7. 后台参数维护位置

### 7.1 sysconfig 配置页（现有页面新增分组"AI 引擎"）

| Key | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `ai_bot_enabled` | bool | 0 | AI 回复总开关，关闭则 fallback 静态池 |
| `ai_chat_enabled` | bool | 0 | 聊天续接 AI 独立开关（路径 C） |
| `llm_api_endpoint` | text | — | LLM API 地址（OpenAI 兼容）|
| `llm_api_key` | text | — | API Key，写入前 AES 加密（复用 crypto 包） |
| `llm_model` | text | gpt-4o-mini | 模型名 |
| `llm_temperature` | int | 8 | 实际温度 = value/10，即 0.8 |
| `llm_max_tokens` | int | 200 | 每次回复 token 上限 |
| `ai_reply_delay_min` | int | 1500 | 模拟打字延迟最小值（ms） |
| `ai_reply_delay_max` | int | 4000 | 模拟打字延迟最大值（ms） |
| `llm_concurrency` | int | 10 | 同时进行的 LLM 调用上限 |

**注：** `llm_api_key` 在 Admin 前端展示为 `****`，通过 PUT 写入时先加密再存 DB，读取时解密后注入 LLMClient，**不在 GET /admin/api/config 接口中返回明文**。

### 7.2 新增管理页：人格库 `/admin/persona`

后端路由：`/admin/api/persona/*`（新增）

功能：
- 列出所有 PersonaConfig（支持按 tenant 筛选）
- 创建/编辑：name / relationship_role / affective_style / voice_style / rules_json
- 保存后触发 `BotRegistry.Reload()`，无需重启服务
- Prompt 预览：输入测试文本，展示 Build() 输出的 System Prompt 结构（不调 LLM）

### 7.3 新增管理页：机器人档案 `/admin/robot/profiles`

后端路由：`/admin/api/robot/profile/*`（新增）

功能：
- 列出所有 `is_robot=true` 的 User，显示昵称/性别/城市/状态/绑定人格名
- 编辑昵称、头像、城市，绑定/切换 PersonaConfig（下拉选人格模板）
- 单个机器人启用/暂停（`User.Status = paused`）
- 统计：累计投瓶数、回信数、发起聊天数（从 bottles/bottle_replies/chats 表 count）

### 7.4 新增管理页：关键字规则 `/admin/keyword-rules`

后端路由：`/admin/api/keyword-rules/*`（新增）

功能：
- 列表视图：触发关键词摘要 / 匹配类型 / 覆盖角色数 / 命中次数 / 状态
- 创建/编辑规则：
  - **关键词组**（tag 输入框，多个关键词 OR 关系，例：`你好`、`hi`、`hello`、`嗨`）
  - **匹配方式**：包含 / 完全匹配 / 前缀
  - **优先级**：数字，越大越先匹配
  - **各角色回复变体**（按 persona_role 分 tab：warm_soft / calm / educator / 默认通用）
  - 每个 tab 可添加 2–5 条变体，运行时随机取一条
- 启用/禁用单条规则
- 从"缓存管理"一键导入高频缓存条目（question_sample 变为规则关键词，responses 变为变体）

**运营工作流：** 上线初期 Admin 预设 20–30 条最高频规则（打招呼/问身份/问你是AI/问年龄等），后续通过缓存管理页的命中统计发现新的高频问题，逐步补充。

### 7.5 新增管理页：回复缓存 `/admin/reply-cache`

后端路由：`/admin/api/reply-cache/*`（新增）

功能：
- 列表视图（默认按 `hit_count DESC` 排序）：问题样本 / 角色 / 变体数 / 命中次数 / 来源 / 状态
- 筛选：按 persona_role / 日期范围 / 来源（ai_generated/manual）/ 状态
- 编辑：修改/增删变体文案，source 自动改为 `manual`
- 一键"升级为关键字规则"：将高频缓存转为精确控制的规则（弹窗输入关键词组）
- 批量禁用 hit_count < N 的条目（清理低价值缓存）
- 手动新建缓存条目（直接为某类消息预设机器人回复，不等待 LLM 自然积累）

**推荐运营节奏：** 上线 3 天后查看 hit_count TOP 50，把其中有规律的批量升级为关键字规则；每周清理 hit_count < 3 的僵尸缓存。

### 7.6 保留不变

- **内容池管理页**（现有 `/admin/api/robot/content/*`）：继续维护 fallback 静态文案
- **sysconfig 机器人分组**（robot_enabled / robot_count / robot_throw_per_hour / robot_reply_ratio）：保持不动

---

## 8. 错误处理与降级策略

| 场景 | 处理 |
|---|---|
| LLM 调用超时（>15s） | 放弃，fallback 静态池随机取一条（不写缓存） |
| LLM 返回内容不通过审核 | fallback 静态池（不写缓存，避免污染） |
| BotReplyQueue 满 | 丢弃该次任务，记 log，不影响 HTTP 响应 |
| PersonaConfig 未找到 | 使用内置默认人格（warm_soft 角色）继续 |
| RobotMemory 创建失败 | 使用空 memory 继续（不中断对话） |
| ReplyCache.Get 失败（DB 异常） | 跳过缓存直接调 LLM，记 log，不阻断流程 |
| ReplyCache.AddVariant 失败 | 丢弃写入，LLM 响应正常发送 |
| KeywordMatcher 规则加载失败 | 跳过 Tier 1，继续走 Tier 2 和 LLM |
| ai_bot_enabled=0 | 所有三条路径均走静态池，与现有行为完全一致 |

---

## 9. 实施优先级

```
P0（基础设施，后续全部依赖）
  ├─ model/model.go 新增 PersonaConfig / RobotMemory / RobotKeywordRule / RobotReplyCache；User 加 PersonaID
  ├─ bootstrap Migration SQL（4 张表）
  ├─ robot/normalize.go（normalizeText + cacheKey）
  ├─ robot/registry.go（BotRegistry）
  └─ robot/memory.go（MemoryStore）

P1（缓存层，上线前先把高频规则填好）
  ├─ robot/keymatcher.go（KeywordMatcher，加载 robot_keyword_rules）
  ├─ robot/replycache.go（ReplyCache，Get + AddVariant + IncrHit）
  └─ 手动向 robot_keyword_rules 写入 20-30 条核心规则（打招呼/问身份等）

P2（LLM 层，P1 完成后接入）
  ├─ robot/llmclient.go
  ├─ robot/promptbuilder.go
  ├─ robot/bot_worker.go（含两级缓存 + LLM 完整调用链）
  ├─ chat/service.go 增加 4 行
  ├─ robot/service.go 加 AI 分支
  └─ sysconfig/sysconfig.go 增加 10 个 Key

P3（Admin 界面，运营可自助维护）
  ├─ admin/handler.go 新增人格 + 机器人档案 + 关键字规则 + 缓存管理路由
  ├─ admin/src/views/Persona.vue
  ├─ admin/src/views/RobotProfile.vue
  ├─ admin/src/views/KeywordRules.vue
  └─ admin/src/views/ReplyCache.vue
```

**P1 先于 P2 的理由：** 关键字规则是纯内存匹配，零 LLM 成本，P1 完成后即可覆盖 60% 高频问题；P2 的 LLM 接入覆盖剩余 40% 的长尾对话。两个阶段都有效果，风险分散。

---

## 10. 开放问题（待确认）

1. **LLM 供应商**：使用 OpenAI 还是国内模型（通义/文心）？影响 `llm_api_endpoint` 默认值和 token 计费方式。
2. **API Key 加密**：复用现有 `crypto` 包的 AES-GCM，还是另行处理？（建议复用）
3. **机器人聊天是否扣金币**：用户向机器人发起聊天时，`StartChat()` 会扣费。机器人发起的场景是否豁免？
4. **familiarity 上限**：是否需要设置上限（如最高 0.9），避免语气过于亲密引发用户误解？
5. **路径 A/B 的 AI 内容质量**：投放的瓶子/回信是否需要二次审核流程，还是通过模板约束 prompt 保证质量？
6. **关键字规则初始内容**：首批 20–30 条规则由谁来写？建议由产品/运营提供"高频问题清单 + 各人格对应回复样本"，开发照单写入 DB，不进入代码。
7. **缓存变体数量**：每个缓存 Key 最多 5 条变体，是否足够？还是需要按角色单独设上限？
8. **缓存跨租户隔离**：`robot_reply_cache` 的 UNIQUE INDEX 包含 `tenant_id`，多租户下不共享缓存——这是否符合预期？（若多个租户角色定义相同，共享缓存可降成本，但增加复杂度）
