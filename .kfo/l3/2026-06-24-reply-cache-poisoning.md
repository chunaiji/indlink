# 踩坑：LLM 回复缓存污染导致 prompt 修复失效

| 字段 | 值 |
|---|---|
| KFO 层级 | L3 — 踩坑/决策记录 |
| 日期 | 2026-06-24 |
| 关联 | L2 `l2/2026-06-24-robot-identity-fix.md` · `l3/2026-06-24-llm-history-in-systemprompt.md` |

---

## 现象

修复了 LLM 历史结构（见 L3 `llm-history-in-systemprompt.md`），重新部署后机器人仍然回复"是，我是智能助手"、"我是 GPT-5.1"，与修复前毫无差异。

---

## 根本原因

`robot_reply_caches` 表（Tier 2）存储了历史 LLM 生成的回复。修复 LLM prompt 之前，"你是机器人吗"等问题触发了 AI 自白，这些回复已被写入缓存。

**Worker 处理顺序：Tier 1 → Tier 2 → Tier 3（LLM）**

用户再次询问时，Tier 2 直接命中旧缓存并返回，Tier 3（修复后的 LLM）根本不会被调用。  
→ 所有 prompt 层面的修复对已缓存的问题**完全无效**。

---

## 规律

> **任何 LLM prompt 修复，只对尚未缓存的问题有效。**  
> 已缓存的"有毒"回复会无限期复现，直到缓存被清理或过期。

---

## 修复方案

**三合一**，缺一不可：

1. **清理 DB 存量**：`DELETE FROM robot_reply_caches WHERE responses_json REGEXP '...'`
2. **读取过滤**：`Get()` 命中后检查 `containsAIConfession`，含自白则返回 "" 强制走 LLM
3. **写入过滤**：`AddVariant()` 入口检查，含自白则拒绝写入

只做 DB 清理不加代码过滤：下次 LLM 偶发自白会再次写入缓存，问题复现。  
只加代码过滤不清 DB：存量污染条目仍会在 `Get()` 中被选中（旧逻辑），直到读取过滤生效。

---

## 预防

- LLM 自积累缓存引入时，就应在 `AddVariant` 入口加内容校验（至少过滤明显的合规风险内容）
- 每次修改 prompt / 模型后，检查缓存是否需要清理（命中率高的 key 最危险）
- Admin 管理台的缓存页（`/admin/reply-cache`）应支持按关键词搜索 + 批量删除，方便运营清理
