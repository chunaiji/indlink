# 机器人重复回复修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除机器人在瓶子回复、聊天回复、引导加聊、破冰/追问各场景发出相同文案的问题。

**Architecture:** 统一机制——回复缓存从「取一条」改为「取全部变体」，选取时排除近期已用文本（`pickVariant` 纯函数）；变体未满 5 条时按 40% 概率穿透缓存走 LLM 让变体池真正积累；瓶子侧对同瓶已有回复查重兜底；两处硬编码/小池文案扩为变体数组。

**Tech Stack:** Go 1.22（`slices.Contains` 可用）+ GORM（MySQL）。

**根因结论（本对话 systematic-debugging 产出）:** 三层回复生成（关键字规则→缓存→LLM）的键只含（消息内容, personaRole），不含机器人个体；且缓存命中即返回、只有 Tier 3 之后才 `AddVariant`，变体池长期停留在 1 条 → 同瓶/跨机器人/跨会话回复一字不差。

## Global Constraints

- 不改缓存键结构（保持跨机器人复用缓存以控制 LLM 成本）
- `maxVariants = 5`（既有常量，replycache.go:16）不变
- 穿透概率为常量 `cacheRegenProb = 40`（百分比），不做 sysconfig 可配（YAGNI；若未来做可配，必须同步加 sysconfig defaults——项目已有空串反转开关的教训）
- 防 AI 自白过滤（`containsAIConfession`）语义不得丢失
- 测试风格：纯函数单测，无 DB 测试基建；DB 交互靠构建验证
- 运行 go 命令的目录：`server/`（module root）

---

### Task 1: ReplyCache.GetVariants + pickVariant 纯函数

**Files:**
- Modify: `server/internal/robot/replycache.go`
- Create: `server/internal/robot/replycache_test.go`

**Interfaces:**
- Produces:
  - `(rc *ReplyCache) GetVariants(tenantID int64, hash, personaRole string) []string` — 返回缓存条目全部变体（已过滤 AI 自白），未命中返回 nil
  - `pickVariant(variants, exclude []string) string` — 随机取一条不在 exclude 里的变体，无可用返回 ""
  - `const cacheRegenProb = 40`
- 旧方法 `Get` 本任务保留（bot_worker.go、service.go 仍在用），Task 3 删除。

- [ ] **Step 1: Write the failing test**

创建 `server/internal/robot/replycache_test.go`：

```go
package robot

import (
	"slices"
	"testing"
)

func TestPickVariant(t *testing.T) {
	// 空变体 → ""
	if got := pickVariant(nil, nil); got != "" {
		t.Fatalf("empty variants: got %q, want empty", got)
	}
	// 单变体无排除 → 返回它
	if got := pickVariant([]string{"a"}, nil); got != "a" {
		t.Fatalf("single: got %q, want a", got)
	}
	// 两变体排除一条 → 必返回另一条(确定性)
	if got := pickVariant([]string{"a", "b"}, []string{"a"}); got != "b" {
		t.Fatalf("exclude a: got %q, want b", got)
	}
	// 全部被排除 → ""
	if got := pickVariant([]string{"a", "b"}, []string{"a", "b"}); got != "" {
		t.Fatalf("all excluded: got %q, want empty", got)
	}
	// 多变体无排除 → 返回值必在变体集内
	vs := []string{"x", "y", "z"}
	for i := 0; i < 20; i++ {
		if got := pickVariant(vs, nil); !slices.Contains(vs, got) {
			t.Fatalf("multi: got %q not in variants", got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/robot/ -run TestPickVariant -v`
Expected: FAIL（`pickVariant` 未定义，编译错误）

- [ ] **Step 3: Write minimal implementation**

`replycache.go` 顶部常量区（`const maxVariants = 5` 处）改为：

```go
const (
	maxVariants = 5
	// cacheRegenProb 变体未满时穿透缓存走 LLM 的百分比概率,用于让变体池逐步积累。
	cacheRegenProb = 40
)
```

`replycache.go` 的 `Get` 方法之后新增：

```go
// GetVariants 返回缓存条目的全部可用变体(过滤 AI 自白);未命中或解析失败返回 nil。
func (rc *ReplyCache) GetVariants(tenantID int64, hash, personaRole string) []string {
	var row model.RobotReplyCache
	err := rc.db.Where("tenant_id = ? AND question_hash = ? AND persona_role = ? AND status = ?",
		tenantID, hash, personaRole, "active").First(&row).Error
	if err != nil {
		return nil
	}
	var variants []string
	if e := json.Unmarshal([]byte(row.ResponsesJSON), &variants); e != nil {
		return nil
	}
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		if !containsAIConfession(v) {
			out = append(out, v)
		}
	}
	return out
}

// pickVariant 从 variants 随机取一条不在 exclude 里的;全被排除或为空返回 ""。
func pickVariant(variants, exclude []string) string {
	pool := make([]string, 0, len(variants))
	for _, v := range variants {
		if !slices.Contains(exclude, v) {
			pool = append(pool, v)
		}
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}
```

import 块加 `"slices"`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/robot/ -run TestPickVariant -v`
Expected: PASS

- [ ] **Step 5: 构建 + 包内全量测试**

Run: `go build ./... && go test ./internal/robot/`
Expected: 编译通过、全部 PASS

- [ ] **Step 6: Commit**

```bash
git add server/internal/robot/replycache.go server/internal/robot/replycache_test.go
git commit -m "feat(robot): reply cache GetVariants + pickVariant with exclusion"
```

---

### Task 2: 聊天侧接入 — 近 5 条防复读 + 概率穿透积累变体

**Files:**
- Modify: `server/internal/robot/bot_worker.go:98-168`（handle）、`bot_worker.go:224-233`（lastBotMessage）

**Interfaces:**
- Consumes: `GetVariants` / `pickVariant` / `cacheRegenProb`（Task 1）
- Produces: `lastBotMessages(chatID, botUserID int64, n int) []string`（替换原 `lastBotMessage`，唯一调用方在同文件）

- [ ] **Step 1: lastBotMessage 改为取近 n 条**

`bot_worker.go:224-233` 整体替换为：

```go
// lastBotMessages 返回该会话最近 n 条由该机器人发送的消息内容(用于防复读)。
func (p *BotWorkerPool) lastBotMessages(chatID, botUserID int64, n int) []string {
	var contents []string
	p.db.Model(&model.Message{}).Where("chat_id = ? AND sender_id = ?", chatID, botUserID).
		Order("created_at desc").Limit(n).Pluck("content", &contents)
	return contents
}
```

- [ ] **Step 2: handle 的 Tier 1/2 改造**

`bot_worker.go` `handle` 中 106-132 行（`lastBot := ...` 到 Tier 2 块结束）替换为：

```go
	// ── Tier 0：身份识别硬拦截（最高优先级，不走缓存也不走 LLM）──────
	if isIdentityQuestion(job.UserMsg) {
		p.deliver(job, buildDenyResponse())
		return
	}

	// 防复读:该机器人在该会话最近 5 条消息,canned 回复(规则/缓存)与其任一相同则不用。
	recent := p.lastBotMessages(job.ChatID, job.BotUserID, 5)

	// ── Tier 1：关键字规则 ─────────────────────────────────────
	if reply, ruleID := p.matcher.Match(job.UserMsg, personaRole); reply != "" && !slices.Contains(recent, reply) {
		p.matcher.IncrHit(ruleID)
		p.deliver(job, reply)
		return
	}

	// ── Tier 2：LLM 缓存(变体未满时按概率穿透,让变体池逐步积累) ────
	hash := cacheKey(job.UserMsg, personaRole)
	variants := p.cache.GetVariants(job.TenantID, hash, personaRole)
	regen := len(variants) < maxVariants && rand.Intn(100) < cacheRegenProb
	if !regen {
		if picked := pickVariant(variants, recent); picked != "" {
			p.cache.IncrHit(job.TenantID, hash, personaRole)
			p.deliver(job, picked)
			return
		}
	}
```

说明：原 `cached != lastBot` 与「历史坏缓存 skip」分支被吸收——AI 自白过滤已在 `GetVariants` 内完成，近 5 条排除由 `pickVariant` 完成。Tier 3 及之后（135 行起）不动，`AddVariant` 既有调用继续负责入池。

import 块加 `"slices"`。

- [ ] **Step 3: 构建 + 测试**

Run: `go build ./... && go test ./internal/robot/`
Expected: 编译通过、PASS

- [ ] **Step 4: Commit**

```bash
git add server/internal/robot/bot_worker.go
git commit -m "fix(robot): chat anti-repeat over last 5 bot messages + variant pool growth"
```

---

### Task 3: 瓶子侧接入 — genAIText 带排除集 + 同瓶查重 + 删旧 Get

**Files:**
- Modify: `server/internal/robot/service.go:331-397`（maybeReply）、`service.go:399-440`（genAIText）
- Modify: `server/internal/robot/replycache.go:25-42`（删除旧 Get）

**Interfaces:**
- Consumes: `GetVariants` / `pickVariant` / `cacheRegenProb`（Task 1）
- Produces: `genAIText(tenantID int64, personaRole, contentType, msgCtx string, exclude []string) string`（唯一调用方 maybeReply 同步更新）
- Removes: `(rc *ReplyCache) Get`（Task 2 后无调用方）

- [ ] **Step 1: genAIText 加 exclude 参数**

`service.go:399-421`（注释、签名到 Tier 2 块）替换为：

```go
// genAIText 为 Path A/B 生成 AI 文本；失败返回 ""（调用方 fallback 静态池）。
// contentType: "bottle"(主动投放) or "reply"(回信)；msgCtx 为要回复的瓶子内容（回信时非空）。
// exclude: 不希望复用的文本(如该瓶已有回复),Tier 1/2 的 canned 结果命中其一时跳过。
func (s *Service) genAIText(tenantID int64, personaRole, contentType, msgCtx string, exclude []string) string {
	matcher := s.getMatcher(tenantID)

	// Tier 1：关键字规则（Path A/B 仅在有 msgCtx 时匹配）
	if msgCtx != "" {
		if reply, ruleID := matcher.Match(msgCtx, personaRole); reply != "" && !slices.Contains(exclude, reply) {
			matcher.IncrHit(ruleID)
			return reply
		}
	}

	// Tier 2：LLM 缓存(变体未满时按概率穿透,让变体池逐步积累)
	hashKey := msgCtx
	if hashKey == "" {
		hashKey = contentType
	}
	hash := cacheKey(hashKey, personaRole)
	variants := s.cache.GetVariants(tenantID, hash, personaRole)
	regen := len(variants) < maxVariants && rand.Intn(100) < cacheRegenProb
	if !regen {
		if picked := pickVariant(variants, exclude); picked != "" {
			s.cache.IncrHit(tenantID, hash, personaRole)
			return picked
		}
	}
```

Tier 3（423 行起）不动。`service.go` import 块加 `"slices"`。

- [ ] **Step 2: maybeReply 同瓶查重**

`service.go` maybeReply 内 351-363 行（选机器人到 fallback 静态池）替换为：

```go
		r := robots[rand.Intn(len(robots))]

		// 同瓶查重:已有回复文本不复用(跨机器人也不允许一字不差)
		var prior []string
		s.db.Model(&model.BottleReply{}).Where("bottle_id = ?", b.BottleID).Pluck("content", &prior)

		var text string
		if aiEnabled {
			persona := s.registry.Get(r.UserID)
			text = s.genAIText(tenantID, persona.AffectiveStyle, "reply", b.Content, prior)
		}
		if text == "" {
			if len(pool) == 0 {
				continue
			}
			text = strings.TrimSpace(pool[rand.Intn(len(pool))].Text)
		}
		if slices.Contains(prior, text) {
			continue // 静态池撞车或 LLM 罕见重复:放弃本条,宁缺毋滥
		}
```

- [ ] **Step 3: 删除旧 Get**

删除 `replycache.go` 的 `Get` 方法（25-42 行，含注释）。此时全仓已无调用方。

- [ ] **Step 4: 构建 + 测试**

Run: `go build ./... && go test ./internal/robot/`
Expected: 编译通过、PASS。若编译报 `Get` 仍被引用，说明有遗漏调用方——回 Task 2 检查，不得保留旧方法。

- [ ] **Step 5: Commit**

```bash
git add server/internal/robot/service.go server/internal/robot/replycache.go
git commit -m "fix(robot): bottle reply dedup per bottle + exclude-aware genAIText"
```

---

### Task 4: 回瓶引导加聊 opener 去硬编码

**Files:**
- Modify: `server/internal/robot/service.go:386-389`（maybeReply 内 opener）
- Modify: `server/internal/robot/engage.go:44`（openers 数组之后加新数组）

**Interfaces:**
- Consumes: `randLine`（engage.go:52，既有）
- Produces: `var bottleChatOpeners []string`

- [ ] **Step 1: 定义变体数组**

`engage.go` `openers` 数组（39-44 行）之后插入：

```go
// bottleChatOpeners 回信后引导加聊的开场白变体(maybeReply 用)。
var bottleChatOpeners = []string{
	"刚看到你的瓶子,挺有共鸣的,想和你多聊两句~",
	"你那个瓶子我看了好几遍,想跟你聊聊。",
	"捞到你的瓶子啦,感觉我们会聊得来~",
	"看了你写的东西,有点想认识你。",
	"你的瓶子写得真好,方便聊聊吗?",
}
```

- [ ] **Step 2: maybeReply 里替换硬编码**

`service.go:386-389`：

```go
					if chatID, err := sender.EnsureRobotChat(tenantID, r.UserID, b.UserID); err == nil {
						sender.SendMessage(tenantID, r.UserID, chatID, randLine(bottleChatOpeners), "text")
					}
```

（删除原 `opener := "刚看到你的瓶子..."` 局部变量。）

- [ ] **Step 3: 构建验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add server/internal/robot/service.go server/internal/robot/engage.go
git commit -m "fix(robot): vary bottle-to-chat opener lines"
```

---

### Task 5: 破冰/追问文案扩池

**Files:**
- Modify: `server/internal/robot/engage.go:39-50`（openers、nudges）

**Interfaces:**
- Consumes/Produces: 无签名变化，纯文案扩充（4→8、3→8）

- [ ] **Step 1: 扩充数组**

`openers`（39-44 行）追加 4 条：

```go
	"路过你的主页,想跟你说说话~",
	"今天心情不错,想找个人分享一下,你呢?",
	"感觉你是个有故事的人,想听你讲讲。",
	"嘿,好巧,能认识一下吗?",
```

`nudges`（46-50 行）追加 5 条：

```go
	"忙完了叫我一声呀~",
	"咦,人呢?我等你回来聊~",
	"你一忙起来就不理我啦?",
	"想听听你的想法,回来告诉我呀。",
	"没事,你先忙,我在这儿等你~",
```

- [ ] **Step 2: 构建 + 包内测试**

Run: `go build ./... && go test ./internal/robot/`
Expected: 编译通过、PASS

- [ ] **Step 3: Commit**

```bash
git add server/internal/robot/engage.go
git commit -m "fix(robot): expand outreach openers and nudge line pools"
```

---

## 上线后观察项（非本计划任务）

- `robot_reply_cache` 条目的 `responses_json` 变体数应逐步涨到 5（后台「回复缓存」页可见）
- 同一瓶子的多条机器人回复不再一字不差
- LLM 调用量短期小幅上升（穿透生成变体），变体池满后回落
