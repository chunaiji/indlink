# AI 人格化对话引擎 — 设计与规划

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层(规划) |
| 日期 | 2026-06-23 |
| 状态 | COMPLETED（P0–P3 全部实现；待接真实 LLM 配置后验证） |
| 触发 | 在静态内容池机器人基础上叠加 AI 人格层：人格绑定 + 两级回复缓存 + 聊天续接，减少 LLM 调用成本 |
| 原始设计文档 | `docs/superpowers/specs/2026-06-23-ai-persona-robot-design.md` |
| 关联 | L1 `l1/robot.md` · `l1/chat-ws.md`(hub 复用) · `l1/admin-platform.md`(新管理页) · `l2/2026-06-21-feature-roadmap.md`(#1 机器人待办) |

---

## 目标

在现有 `robot/` 模块（`Tick()` 静态池随机投放/回信）基础上叠加三层能力：

1. **人格层**：每个机器人绑定 PersonaConfig（关系角色 × 情绪风格 × 表达方式），同角色机器人说话一致
2. **两级缓存**：60%+ 高频问题不调 LLM——Tier 1 关键字规则（Admin 预设）+ Tier 2 LLM 回复自积累
3. **聊天续接**：用户向机器人发消息后，机器人自动回复（现有系统断链，不响应聊天消息）

---

## 改造原则

- **叠加不替换**：现有静态池保留为 AI 失败时的 fallback，`ai_bot_enabled=0` 行为与现在完全相同
- **最小侵入**：`chat/service.go` 仅增加 ~4 行；调度器框架不动，只在 throwBottles/maybeReply 内加分支
- **缓存先于 LLM**：P1 阶段先把关键字规则填好（零 LLM 成本），P2 阶段再接 LLM 覆盖长尾

---

## 三条触发路径

| 路径 | 触发方式 | 修改文件 | 状态 |
|---|---|---|---|
| A 主动投瓶 | Cron `Tick()→throwBottles()` | `robot/service.go` | PROPOSED |
| B 被动回信 | Cron `Tick()→maybeReply()` | `robot/service.go` | PROPOSED |
| C 聊天续接 | 用户发消息 → SendMessage() → BotReplyQueue | `chat/service.go` + 新增 bot_worker | PROPOSED |

---

## 两级回复缓存

```
用户消息
  ↓ normalize(text)
  Tier 1  KeywordMatcher.Match(text, personaRole)   → 命中直接返回（零 LLM）
  Tier 2  ReplyCache.Get(hash, personaRole)         → 命中直接返回（零 LLM）
  Tier 3  LLMClient.Chat()                          → LLM 生成
          go ReplyCache.AddVariant(...)              → 异步写缓存
```

**缓存 Key 设计：** `SHA256(normalize(text) + "|" + persona_role)[:8]` hex（16 字符）。按 `persona_role` 而非 `persona_id` 共享，同角色 100 个机器人共用同一缓存桶，命中率最大化。

---

## 数据模型变更

| 变更 | 类型 | 说明 |
|---|---|---|
| `users.persona_id` | ALTER TABLE | nullable bigint，绑定人格 |
| `persona_configs` | 新建表 | 人格配置模板（role / style / rules_json） |
| `robot_memory` | 新建表 | 用户×机器人关系记忆（familiarity 0~1 / 偏好摘要） |
| `robot_keyword_rules` | 新建表 | Tier 1 规则（关键词 + 按角色的回复变体） |
| `robot_reply_cache` | 新建表 | Tier 2 LLM 自积累缓存（最多 5 条变体/Key） |

---

## 新增 Go 文件

`robot/normalize.go` · `robot/registry.go` · `robot/memory.go` · `robot/keymatcher.go` · `robot/replycache.go` · `robot/llmclient.go` · `robot/promptbuilder.go` · `robot/bot_worker.go`

---

## 新增 Admin 管理页

| 页面路由 | 功能 |
|---|---|
| `/admin/persona` | 人格库 CRUD + Prompt 预览 + 热重载 |
| `/admin/robot/profiles` | 机器人档案 + 绑定人格 + 统计 |
| `/admin/keyword-rules` | 关键字规则 CRUD + 优先级 + 批量导入 |
| `/admin/reply-cache` | 缓存查看/编辑 + 命中统计 + 升级为规则 |

---

## 实施优先级

```
P0  基础设施（模型 + Migration + normalize + registry + memory）
P1  缓存层（keymatcher + replycache + 填写 20-30 条首批规则）   ← 零 LLM 成本即可生效
P2  LLM 层（llmclient + promptbuilder + bot_worker + chat 4 行 + sysconfig 10 Key）
P3  Admin 界面（4 个新管理页）
```

---

## 开放问题（待确认后方可实施）

1. LLM 供应商：OpenAI 还是国内模型（通义/文心）？
2. 机器人聊天是否豁免金币扣费（`StartChat()` 目前会扣费）？
3. `familiarity` 上限：建议封顶 0.9，避免语气过于亲密引起用户误解。
4. 首批关键字规则（20–30 条）由运营提供问题清单 + 各人格回复样本，开发写入 DB。
5. 缓存跨租户是否共享（现设计按 tenant_id 隔离）？

---

## 验证要点（实施后补充）

- [ ] `ai_bot_enabled=0` 时行为与现有完全相同
- [ ] 关键字规则命中后不调 LLM，`hit_count` 递增
- [ ] 相同消息第二次发送命中 Tier 2 缓存，不再调 LLM
- [ ] 聊天消息发送后 1.5–4s 内收到机器人回复（实时 WS 推送）
- [ ] LLM 超时/失败时 fallback 静态池，对话不中断
- [ ] BotReplyQueue 满时丢弃，不阻塞 HTTP /chat/:id/send 响应
