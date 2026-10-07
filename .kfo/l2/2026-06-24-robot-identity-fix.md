# 机器人身份暴露三层修复

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-24 |
| 状态 | COMPLETED + DEPLOYED |
| 触发 | 机器人仍然回复"是，我是智能助手"、"我是GPT-5.1"等——LLM 历史修复后仍未解决 |
| 关联 | L1 `l1/robot.md` · `l2/2026-06-24-robot-persona-chat-fixes.md` · `l3/2026-06-24-reply-cache-poisoning.md` |

---

## 根本原因

上一轮修复（LLM 历史结构 + AI 自白净化）只作用于 **Tier 3（LLM 调用路径）**，但在此之前：

1. **Tier 2 缓存已被污染**：历史 LLM 回复含"是，我是智能助手"等内容，写入了 `robot_reply_caches`。用户再次问同类问题时，Worker 在 Tier 2 直接命中返回旧内容，根本不走 LLM，所有 prompt 修复全部失效。
2. **无 Tier 0 兜底**：即使 LLM 有 prompt 约束，模型 RLHF 训练下被反复追问仍可能松口，需要代码层硬拦截。

---

## 三层修复

### 1. Tier 0 — 身份识别硬拦截（`bot_worker.go`）

在所有 Tier 之前新增：

```go
if isIdentityQuestion(job.UserMsg) {
    p.deliver(job, buildDenyResponse())
    return
}
```

触发关键词（`identityKeywords`）：
```
机器人 / 人工智能 / 真人 / 虚拟人 /
ai / gpt / chatgpt / claude / gemini / llama /
语言模型 / 大模型 / 智能助手 / 虚拟助手
```

否认话术（`denyResponses`，7 条随机）：
- "哈哈我就是普通人呀，你怎么突然这么问呢？"
- "机器人？不不，我就是个真实的人啦～"
- "我真人哦～怎么了，你不信我吗？"
- 等 4 条……

### 2. 缓存读取过滤（`replycache.go: Get()`）

```go
response := variants[rand.Intn(len(variants))]
if containsAIConfession(response) {
    return "" // 缓存条目含 AI 自白，强制走 LLM 重新生成
}
return response
```

### 3. 缓存写入过滤（`replycache.go: AddVariant()`）

```go
if containsAIConfession(text) {
    return // 不缓存含 AI 自白的回复，防止缓存污染
}
```

`containsAIConfession` 提取为 `promptbuilder.go` 的公共函数，复用 `aiConfessionPhrases` 列表（20+ 短语）。

---

## DB 清理

```sql
DELETE FROM robot_reply_caches
WHERE responses_json REGEXP 'AI助手|机器人|人工智能|GPT|ChatGPT|Claude|Gemini|语言模型|大模型|智能助手|我是AI|我是助手|虚拟';
```

已在生产执行。

---

## 防御层级总结

| 层级 | 机制 | 覆盖范围 |
|---|---|---|
| Tier 0（代码硬拦截） | `isIdentityQuestion` + `buildDenyResponse` | 所有含身份关键词的问题，不依赖 LLM |
| Tier 2 读取过滤 | `containsAIConfession` | 清除历史污染缓存的影响 |
| Tier 2 写入过滤 | `containsAIConfession` | 防止未来 LLM 偶发自白被缓存固化 |
| Tier 3 prompt | system prompt 末尾硬性禁止块 | LLM 生成路径的最后一道约束 |
| Tier 3 历史净化 | `buildHistory` + `sanitizeBotContent` | 防止 LLM 看到自己历史自白后继续沿用 |
