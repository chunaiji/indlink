# 主动匹配（火花）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在线真人每隔随机 N–M 分钟被系统配一个人（按概率真人或机器人），App 弹窗「XX 与你碰撞出了火花」，点进去免开聊费直接聊。

**Architecture:** 新包 `internal/spark`：常驻调度器每分钟扫 WS 在线用户，每人在 Redis 有独立的「下次匹配时刻」键。配到机器人就预建会话 + 发开场白再弹窗；配到真人就只给双方弹窗，谁点击谁调 `/spark/accept`，服务端校验 Redis 配对记录后免费建会话。纯函数（掷骰、间隔、候选过滤）与副作用分离，前者全部可测。

**Tech Stack:** Go 1.2x（Gin + GORM + go-redis）、Flutter 3.47 + Riverpod 3（两个工程）。

**Spec:** `docs/superpowers/specs/2026-10-07-spark-matching-design.md`

## Global Constraints

- 项目约定：测试 / 构建命令沿用既有授权（`go build/vet/test`、`flutter analyze/test`、`npm run build`）；**git 提交只在用户明示时**，计划里不含提交步骤。
- 服务端改动一律加法；`internal/robot/outreach.go` 的行为不变（只把 `claimOutreach` 导出）。
- sysconfig 新键必须同步写 `defaults`，并在 `internal/admin/meta.go` 登记中英文标签 + `Group<Xxx>` 常量。
- App 文案键惯例：**文案默认空串（空 = 用内置 ARB），开关默认按本功能要求全关**。
- 本功能仅 App：`GroupSpark` 的 `Platform` 为 `PlatformApp`。
- 两个 Flutter 工程 `app/bottles_zh` 与 `app/bottles` 各落一遍，内容一致。
- 只免开聊费：免费建会话走 `chat.EnsureFreeChat`（不碰钱包）；`price_msg` 照常。
- Redis 键全部带租户与日期前缀，当日级 TTL；与 `outreach` 共用计数键 `outreach:<租户>:<uid>:<日期>`。

## Review Focus

1. **`spark_interval_min > spark_interval_max`（运营填反了）** → 不能让 `rand.Intn(负数)` panic 把整个调度器打死。（Task 1 `nextInterval` 表驱动测试含交换与全 0）
2. **`/spark/accept` 传一个从未匹配过的 peer_id** → 必须 403，否则任何人都能和任意人免费开聊，绕过 `price_chat`。（Task 4 测试）
3. **真人候选在 `eligiblePeers` 通过、随后 `ClaimDaily` 失败（被另一个 tick 抢走额度）** → 要换下一个候选，而不是整轮放弃。（Task 3 测试）
4. **同一对真人在同一分钟被两个方向各扫到一次** → 只能产生一次配对，不能互相弹两遍。（Task 3 `pairKey` 顺序无关 + 去重测试）
5. **WS 同时推来两条火花（两个租户事件 / 重连后补推）** → App 只弹一个，不排队叠弹窗。（Task 7 widget 测试）

---

### Task 1: `internal/spark` 纯函数（掷骰、间隔、时段、配对键）

**Files:**
- Create: `server/internal/spark/rules.go`
- Create: `server/internal/spark/rules_test.go`

**Interfaces:**
- Produces:
  - `type Kind int`，常量 `KindRobot Kind = iota`、`KindReal`
  - `func pickKind(realRatio int, roll func(n int) int) Kind`
  - `func nextInterval(minMin, maxMin int, roll func(n int) int) time.Duration`
  - `func inWindow(hour, start, end int) bool`
  - `func pairKey(tenantID, a, b int64) string`
  - `const defaultInterval = 30 * time.Minute`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/spark/rules_test.go
package spark

import (
	"testing"
	"time"
)

// roll 注入确定性「随机数」:返回固定值,便于断言分支。
func fixedRoll(v int) func(int) int { return func(int) int { return v } }

func TestPickKind(t *testing.T) {
	// ratio=30: roll 返回 0..29 配真人,30..99 配机器人
	if got := pickKind(30, fixedRoll(0)); got != KindReal {
		t.Errorf("roll=0 ratio=30 应配真人, got %v", got)
	}
	if got := pickKind(30, fixedRoll(29)); got != KindReal {
		t.Errorf("roll=29 ratio=30 应配真人, got %v", got)
	}
	if got := pickKind(30, fixedRoll(30)); got != KindRobot {
		t.Errorf("roll=30 ratio=30 应配机器人, got %v", got)
	}
	// 边界:0 永远机器人,100 永远真人
	if got := pickKind(0, fixedRoll(0)); got != KindRobot {
		t.Errorf("ratio=0 应全机器人, got %v", got)
	}
	if got := pickKind(100, fixedRoll(99)); got != KindReal {
		t.Errorf("ratio=100 应全真人, got %v", got)
	}
	// 越界配置夹回 [0,100],不能让负数把 roll 的上界算成负的
	if got := pickKind(-5, fixedRoll(0)); got != KindRobot {
		t.Errorf("ratio=-5 应当成 0, got %v", got)
	}
	if got := pickKind(300, fixedRoll(99)); got != KindReal {
		t.Errorf("ratio=300 应当成 100, got %v", got)
	}
}

// 运营把 min/max 填反、填 0、填负数,都不能让 rand.Intn 收到非正数而 panic——
// 那会把整个调度 goroutine 打死,功能静默消失。
func TestNextInterval(t *testing.T) {
	cases := []struct {
		min, max, roll int
		want           time.Duration
	}{
		{20, 60, 0, 20 * time.Minute},  // 取下界
		{20, 60, 40, 60 * time.Minute}, // roll 上界(span=41,roll=40)
		{20, 20, 0, 20 * time.Minute},  // 相等
		{60, 20, 0, 20 * time.Minute},  // 填反了:交换后取下界
		{0, 0, 0, defaultInterval},     // 都没配
		{-5, -1, 0, defaultInterval},   // 负数
		{0, 60, 0, defaultInterval},    // 下界非正:整体回落,不要 0 间隔狂刷
	}
	for _, c := range cases {
		if got := nextInterval(c.min, c.max, fixedRoll(c.roll)); got != c.want {
			t.Errorf("nextInterval(%d,%d,roll=%d) = %v want %v", c.min, c.max, c.roll, got, c.want)
		}
	}
}

func TestInWindow(t *testing.T) {
	if !inWindow(12, 10, 23) || !inWindow(10, 10, 23) {
		t.Error("区间内与左端点应在窗内")
	}
	if inWindow(23, 10, 23) || inWindow(9, 10, 23) {
		t.Error("右端点不含,左端点之前不在窗内")
	}
	if inWindow(12, 23, 10) {
		t.Error("start >= end 视为未配置,一律不在窗内(与 outreach 同语义)")
	}
}

// 去重键与两个人的先后顺序无关:否则 A→B 与 B→A 会被当成两对,同一分钟互弹两次。
func TestPairKeyIsOrderIndependent(t *testing.T) {
	if pairKey(7, 100, 200) != pairKey(7, 200, 100) {
		t.Fatal("pairKey 必须与顺序无关")
	}
	if pairKey(7, 100, 200) == pairKey(8, 100, 200) {
		t.Fatal("不同租户必须是不同的键")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/spark/ 2>&1 | tail -3`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**

```go
// Package spark 主动匹配(火花):在线真人每隔随机 N–M 分钟被系统配一个人,
// App 弹窗提示,点进去免开聊费直接聊。
//
// 与 robot/outreach.go 的区别:那个是「挑近期活跃真人,让机器人静默发消息」;
// 这个要求对方**此刻在线**、会配两个真人、要弹窗、且开聊免费。
package spark

import (
	"fmt"
	"time"
)

type Kind int

const (
	KindRobot Kind = iota
	KindReal
)

// defaultInterval 间隔没配 / 配得不可用时的回落值。
// 不能回落到 0:那会让每一轮 tick 都给同一个人弹窗。
const defaultInterval = 30 * time.Minute

// pickKind 掷骰子决定配真人还是机器人。realRatio 是真人概率(%),越界夹到 [0,100]。
// roll 注入是为了测试能断言分支,生产传 rand.Intn。
func pickKind(realRatio int, roll func(n int) int) Kind {
	if realRatio < 0 {
		realRatio = 0
	}
	if realRatio > 100 {
		realRatio = 100
	}
	if realRatio == 0 {
		return KindRobot
	}
	if roll(100) < realRatio {
		return KindReal
	}
	return KindRobot
}

// nextInterval 下次匹配的间隔。填反了就交换;下界非正或两者都没配就回落 defaultInterval。
//
// ⚠️ 这里每一条兜底都是在挡 rand.Intn 收到非正数时的 panic——调度器是个常驻 goroutine,
// 一次 panic 就让整个功能静默消失,而后台配置页允许运营填任何数。
func nextInterval(minMin, maxMin int, roll func(n int) int) time.Duration {
	if minMin > maxMin {
		minMin, maxMin = maxMin, minMin
	}
	if minMin <= 0 || maxMin <= 0 {
		return defaultInterval
	}
	span := maxMin - minMin + 1
	return time.Duration(minMin+roll(span)) * time.Minute
}

// inWindow 当前小时是否在时段内(右端点不含)。start >= end 视为未配置,一律不在窗内。
// 与 robot/outreach.go 的 inWindow 同语义,刻意不跨包复用:那是 robot 的私有函数,
// 导出它只为这一个用途会把两个包绑在一起。
func inWindow(hour, start, end int) bool {
	if start >= end {
		return false
	}
	return hour >= start && hour < end
}

// pairKey 当日配对去重键,与两人先后顺序无关。
func pairKey(tenantID, a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("spark:pair:%d:%d:%d", tenantID, a, b)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && gofmt -l internal/spark; go vet ./internal/spark/ && go test ./internal/spark/ -v 2>&1 | grep -E "^(--- |ok|FAIL)"`
Expected: 4 条全 PASS

---

### Task 2: 免费会话与共享的每日配额

**Files:**
- Modify: `server/internal/chat/service.go:499-512`
- Modify: `server/internal/robot/outreach.go:116-131`
- Create: `server/internal/chat/free_chat_test.go`

**Interfaces:**
- Produces:
  - `func (s *chat.Service) EnsureFreeChat(tenantID, u1, u2 int64) (int64, error)` —— 建会话且**不扣费**；已存在则返回现有 `chat_id`
  - `func (s *chat.Service) EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error)` —— 保留为薄包装，`outreach.go` 不改调用
  - `func robot.ClaimDailyReach(tenantID, userID int64, dailyCap int) bool` —— 原 `claimOutreach` 导出，键名不变

- [ ] **Step 1: 写失败测试**

```go
// server/internal/chat/free_chat_test.go
package chat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// 免费会话是火花与机器人触达共用的唯一入口。这条用 AST 盯住它的函数体里
// 不出现任何钱包调用——哪天有人顺手加一行 Debit,两个功能会同时开始扣币,
// 而那种错误在没有 DB 测试基建的仓库里跑不出来。
func TestEnsureFreeChatNeverTouchesWallet(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "service.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "EnsureFreeChat" {
			return true
		}
		found = true
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			sel, ok := m.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "wlt" {
				t.Errorf("EnsureFreeChat 里出现了钱包调用 wlt.%s —— 免费会话不能扣费", sel.Sel.Name)
			}
			if strings.Contains(sel.Sel.Name, "Debit") {
				t.Errorf("EnsureFreeChat 里出现了 %s —— 免费会话不能扣费", sel.Sel.Name)
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatal("没找到 EnsureFreeChat")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/chat/ -run TestEnsureFreeChatNeverTouchesWallet 2>&1 | tail -3`
Expected: FAIL，"没找到 EnsureFreeChat"

- [ ] **Step 3: 实现**

`server/internal/chat/service.go` 把现有 `EnsureRobotChat` 改为：

```go
// EnsureFreeChat 建会话且**不扣费**;已存在则返回现有会话。
//
// 两个调用方:机器人主动触达(robot/outreach.go)与主动匹配(internal/spark)。
// 普通开聊走 StartChat,那条要扣 price_chat;这条刻意绕开钱包——
// 系统主动给人牵的线,不该让被动的一方付钱。
func (s *Service) EnsureFreeChat(tenantID, u1, u2 int64) (int64, error) {
	if existing, err := s.findChat(tenantID, u1, u2); err == nil && existing != nil {
		return existing.ChatID, nil
	}
	a, b := order(u1, u2)
	chat := &model.Chat{
		ChatID: idgen.Next(), TenantID: tenantID, UserA: a, UserB: b,
		RelationStage: "stranger", UpdatedAt: time.Now(), CreatedAt: time.Now(),
	}
	if err := s.db.Create(chat).Error; err != nil {
		return 0, err
	}
	return chat.ChatID, nil
}

// EnsureRobotChat 机器人会话。保留这个名字是因为 robot/outreach.go 在用它,
// 语义与 EnsureFreeChat 完全一致。
func (s *Service) EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error) {
	return s.EnsureFreeChat(tenantID, botUserID, userID)
}
```

`server/internal/robot/outreach.go` 把 `claimOutreach` 导出（函数体一字不改，只改名与注释）：

```go
// ClaimDailyReach 占用一次用户当日被主动触达的额度;超过 dailyCap 返回 false。
//
// 键名 outreach:<租户>:<uid>:<日期> **由 robot 与 spark 共用**:两者都是「系统主动找上门」,
// 共用一个计数器才能做到当天互斥——用户不会先被机器人私信、过一会儿又被弹一次火花。
// 各自的 dailyCap 不同,但消耗的是同一个额度。
func ClaimDailyReach(tenantID, userID int64, dailyCap int) bool {
	ctx := context.Background()
	key := fmt.Sprintf("outreach:%d:%d:%s", tenantID, userID, time.Now().Format("20060102"))
	n, err := cache.RDB.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	if int(n) > dailyCap {
		cache.RDB.Decr(ctx, key)
		return false
	}
	return true
}
```

同文件内 `outreachTick` 里的调用点 `claimOutreach(tenantID, u.UserID, dailyCap)` 改为 `ClaimDailyReach(...)`。

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go build ./... && go vet ./internal/chat/ ./internal/robot/ && go test ./internal/chat/ ./internal/robot/ 2>&1 | tail -3`
Expected: chat PASS；robot 仍是既有的 `TestBuildIdentityResponse_*` 失败（与本任务无关，不要去动）

---

### Task 3: 配对候选过滤与调度循环

**Files:**
- Create: `server/internal/spark/service.go`
- Create: `server/internal/spark/service_test.go`

**Interfaces:**
- Consumes: Task 1 的 `pickKind` / `nextInterval` / `inWindow` / `pairKey`；Task 2 的 `chat.EnsureFreeChat`、`robot.ClaimDailyReach`。
- Produces:
  - `type Candidate struct{ UserID int64; Nickname, Avatar string }`
  - `type Deps struct { DB *gorm.DB; Online func() []int64; Push func(userID int64, payload any); EnsureChat func(tenantID, u1, u2 int64) (int64, error); SendAs func(tenantID, fromUserID, chatID int64, text, kind string) (*model.Message, error); Opening func(tenantID, botUserID int64) string; ClaimDaily func(tenantID, userID int64, cap int) bool; IsBlocked func(a, b int64) bool }`
  - `type Service struct{ d Deps }` + `func New(d Deps) *Service`
  - `func eligiblePeers(self int64, online []Candidate, pairedToday func(peer int64) bool, blocked func(peer int64) bool, capLeft func(peer int64) bool) []Candidate`
  - `func (s *Service) matchReal(tenantID, self int64, peers []Candidate, cap int, roll func(int) int) (Candidate, bool)`
  - `func Start(d Deps, tenantID int64)`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/spark/service_test.go
package spark

import "testing"

func cands(ids ...int64) []Candidate {
	out := make([]Candidate, 0, len(ids))
	for _, id := range ids {
		out = append(out, Candidate{UserID: id, Nickname: "u"})
	}
	return out
}

func idsOf(cs []Candidate) []int64 {
	out := make([]int64, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.UserID)
	}
	return out
}

// 候选过滤的每一条规则各来一例:自己、已拉黑、当日已配过、配额已满。
func TestEligiblePeers(t *testing.T) {
	online := cands(1, 2, 3, 4, 5)
	got := eligiblePeers(
		1,
		online,
		func(peer int64) bool { return peer == 2 }, // 2 当日已配过
		func(peer int64) bool { return peer == 3 }, // 3 已拉黑
		func(peer int64) bool { return peer != 4 }, // 4 配额已满(capLeft=false)
	)
	if len(got) != 1 || got[0].UserID != 5 {
		t.Fatalf("只应剩 5, got %v", idsOf(got))
	}
	// 一个都不剩时返回空而不是 nil 解引用
	if got := eligiblePeers(1, cands(1), func(int64) bool { return false },
		func(int64) bool { return false }, func(int64) bool { return true }); len(got) != 0 {
		t.Fatalf("只有自己时应为空, got %v", idsOf(got))
	}
}

// 选中的候选 ClaimDaily 失败(额度被另一个 tick 抢走)时要换下一个,不能整轮放弃——
// 否则在线人少的时候,一个额度满了的人会把别人的匹配机会一起堵掉。
func TestMatchRealFallsThroughWhenClaimFails(t *testing.T) {
	var claimed []int64
	s := New(Deps{ClaimDaily: func(_ int64, userID int64, _ int) bool {
		claimed = append(claimed, userID)
		return userID == 9 // 只有 9 能占到额度
	}})
	// roll 恒 0 = 总是先挑列表里的第一个
	got, ok := s.matchReal(1, 100, cands(7, 8, 9), 3, func(int) int { return 0 })
	if !ok || got.UserID != 9 {
		t.Fatalf("应最终选中 9, got %+v ok=%v", got, ok)
	}
	if len(claimed) != 3 {
		t.Fatalf("应依次尝试 7、8、9, got %v", claimed)
	}
}

func TestMatchRealReturnsFalseWhenNobodyClaims(t *testing.T) {
	s := New(Deps{ClaimDaily: func(int64, int64, int) bool { return false }})
	if _, ok := s.matchReal(1, 100, cands(7, 8), 3, func(int) int { return 0 }); ok {
		t.Fatal("全都占不到额度时应返回 false,由调用方退回机器人")
	}
}

// 同一分钟里 A 扫到 B、B 也扫到 A:第二次必须被 pairedToday 挡掉,否则两人互弹两遍。
// 这条盯的是 eligiblePeers 真的把 pairedToday 用在了过滤上(而不是只过滤了别的)。
func TestEligiblePeersHonoursPairedToday(t *testing.T) {
	// 模拟:A(1) 刚和 B(2) 配过,记在同一个与顺序无关的键上
	done := map[string]bool{pairKey(7, 1, 2): true}
	pairedFor := func(self int64) func(int64) bool {
		return func(peer int64) bool { return done[pairKey(7, self, peer)] }
	}
	noBlock := func(int64) bool { return false }
	always := func(int64) bool { return true }

	if got := eligiblePeers(1, cands(1, 2), pairedFor(1), noBlock, always); len(got) != 0 {
		t.Fatalf("A 看 B 应已被排除, got %v", idsOf(got))
	}
	if got := eligiblePeers(2, cands(1, 2), pairedFor(2), noBlock, always); len(got) != 0 {
		t.Fatalf("反方向 B 看 A 同样要被排除(pairKey 与顺序无关), got %v", idsOf(got))
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/spark/ -run 'TestEligiblePeers|TestMatchReal' 2>&1 | tail -3`
Expected: FAIL（`Candidate` / `eligiblePeers` / `New` 未定义）

- [ ] **Step 3: 实现**

```go
// server/internal/spark/service.go
package spark

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"

	"gorm.io/gorm"
)

// Candidate 一个可被配对的人(真人或机器人)。
type Candidate struct {
	UserID   int64
	Nickname string
	Avatar   string
}

// Deps 外部依赖全部以函数注入:调度器要能在没有 DB / Redis / WS 的情况下被测。
type Deps struct {
	DB         *gorm.DB
	Online     func() []int64                                     // chat.Hub.OnlineUserIDs
	Push       func(userID int64, payload any)                    // chat.Hub.PushTo
	EnsureChat func(tenantID, u1, u2 int64) (int64, error)        // chat.Service.EnsureFreeChat
	SendAs     func(tenantID, fromUserID, chatID int64, text, kind string) (*model.Message, error)
	Opening    func(tenantID, botUserID int64) string             // 机器人开场白
	ClaimDaily func(tenantID, userID int64, cap int) bool         // robot.ClaimDailyReach
	IsBlocked  func(a, b int64) bool                              // moderation.Service.IsBlocked
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// eligiblePeers 真人候选过滤:排除自己、当日已配过的、互相拉黑的、配额已满的。
//
// capLeft 是**只读**检查,真正占额度在选中之后的 ClaimDaily;两者之间的窗口里
// 对方可能被另一个 tick 占满——所以 matchReal 选中后 ClaimDaily 失败要换下一个。
func eligiblePeers(self int64, online []Candidate, pairedToday func(peer int64) bool,
	blocked func(peer int64) bool, capLeft func(peer int64) bool) []Candidate {
	out := make([]Candidate, 0, len(online))
	for _, c := range online {
		if c.UserID == self || pairedToday(c.UserID) || blocked(c.UserID) || !capLeft(c.UserID) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// matchReal 从候选里挑一个能真正占到额度的人。挑中后 ClaimDaily 失败就换下一个,
// 全都占不到才返回 false(调用方据此退回机器人)。
func (s *Service) matchReal(tenantID, self int64, peers []Candidate, cap int, roll func(int) int) (Candidate, bool) {
	pool := make([]Candidate, len(peers))
	copy(pool, peers)
	for len(pool) > 0 {
		i := roll(len(pool))
		pick := pool[i]
		pool = append(pool[:i], pool[i+1:]...)
		if s.d.ClaimDaily(tenantID, pick.UserID, cap) {
			return pick, true
		}
	}
	return Candidate{}, false
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && gofmt -l internal/spark; go vet ./internal/spark/ && go test ./internal/spark/ 2>&1 | tail -2`
Expected: PASS

- [ ] **Step 5: 补调度循环(同文件)**

```go
// 调度器状态键。
func nextKey(tenantID, userID int64) string { return fmt.Sprintf("spark:next:%d:%d", tenantID, userID) }
func seenKey(tenantID, userID int64) string { return fmt.Sprintf("spark:seen:%d:%d", tenantID, userID) }

// endOfDay 今天剩余时间,配对去重键的 TTL。
func endOfDay(now time.Time) time.Duration {
	y, m, d := now.Date()
	return time.Until(time.Date(y, m, d, 23, 59, 59, 0, now.Location()))
}

// Start 启动调度器:固定每分钟扫一次在线用户,到点的人各自匹配一次。
//
// 为什么是「固定 tick + 每人一个 Redis TTL 键」而不是给每人起一个定时器:
// 在线用户可能成千上万,每人一个 goroutine 定时器的代价远高于每分钟扫一遍。
func Start(d Deps, tenantID int64) {
	go func() {
		for {
			time.Sleep(time.Minute)
			func() {
				defer func() {
					// 调度器是常驻 goroutine:一次 panic 就让功能静默消失,
					// 这里兜住并打日志,下一分钟继续。
					if r := recover(); r != nil {
						log.Printf("[spark] tick panic: %v", r)
					}
				}()
				New(d).tick(tenantID, time.Now())
			}()
		}
	}()
	log.Printf("[spark] 调度器已启动(每 1 分钟一次;总开关 spark_enabled)")
}

func (s *Service) tick(tenantID int64, now time.Time) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeySparkEnabled) {
		return
	}
	if !inWindow(now.Hour(), sysconfig.GetInt(tenantID, sysconfig.KeySparkWindowStart),
		sysconfig.GetInt(tenantID, sysconfig.KeySparkWindowEnd)) {
		return
	}
	online := s.d.Online()
	if len(online) == 0 {
		return
	}
	// 在线 ID → 同租户的真人资料
	var users []model.User
	s.d.DB.Select("user_id, nickname, avatar, is_robot, status").
		Where("tenant_id = ? AND user_id IN ? AND is_robot = ? AND status = ?",
			tenantID, online, false, "active").Find(&users)
	if len(users) == 0 {
		return
	}
	pool := make([]Candidate, 0, len(users))
	for _, u := range users {
		pool = append(pool, Candidate{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar})
	}

	ctx := context.Background()
	dailyCap := sysconfig.GetInt(tenantID, sysconfig.KeySparkUserDailyCap)
	if dailyCap <= 0 {
		dailyCap = 1
	}
	ratio := sysconfig.GetInt(tenantID, sysconfig.KeySparkRealRatio)
	minM := sysconfig.GetInt(tenantID, sysconfig.KeySparkIntervalMin)
	maxM := sysconfig.GetInt(tenantID, sysconfig.KeySparkIntervalMax)

	for _, self := range pool {
		if cache.RDB.Exists(ctx, nextKey(tenantID, self.UserID)).Val() > 0 {
			continue // 还没到点
		}
		// 先设下次时刻:匹配失败也不要在同一分钟原地重试
		cache.RDB.Set(ctx, nextKey(tenantID, self.UserID), 1, nextInterval(minM, maxM, rand.Intn))
		// 首次被扫到只设键不弹窗,否则用户一打开 App 就被糊脸
		if cache.RDB.SetNX(ctx, seenKey(tenantID, self.UserID), 1, 24*time.Hour).Val() {
			continue
		}
		if !s.d.ClaimDaily(tenantID, self.UserID, dailyCap) {
			continue
		}
		s.matchOne(ctx, tenantID, self, pool, dailyCap, ratio, now)
	}
}

// matchOne 给一个人配一次。真人候选为空或都占不到额度时退回机器人,不浪费这次机会。
func (s *Service) matchOne(ctx context.Context, tenantID int64, self Candidate,
	pool []Candidate, dailyCap, ratio int, now time.Time) {
	paired := func(peer int64) bool {
		return cache.RDB.Exists(ctx, pairKey(tenantID, self.UserID, peer)).Val() > 0
	}
	if pickKind(ratio, rand.Intn) == KindReal {
		peers := eligiblePeers(self.UserID, pool, paired,
			func(peer int64) bool { return s.d.IsBlocked(self.UserID, peer) },
			func(peer int64) bool { return true }, // 只读额度检查交给 matchReal 的 ClaimDaily
		)
		if peer, ok := s.matchReal(tenantID, self.UserID, peers, dailyCap, rand.Intn); ok {
			s.deliverReal(ctx, tenantID, self, peer, now)
			return
		}
	}
	s.deliverRobot(ctx, tenantID, self, paired, now)
}
```

> `eligiblePeers` 的 `capLeft` 在生产里恒 true，真正的额度判定由 `matchReal` 的 `ClaimDaily` 承担（它会消耗额度，只读检查会和它重复计数）。参数保留是为了让过滤规则在测试里能被单独断言。

- [ ] **Step 6: 运行确认仍通过**

Run: `cd server && go build ./... && go test ./internal/spark/ 2>&1 | tail -2`
Expected: PASS（`deliverReal` / `deliverRobot` 在 Task 4 实现前先留空壳：本步骤只加 `tick` / `matchOne`，两个 deliver 方法在 Task 4 补齐。为让本步骤能编译，先各写一个空实现并在 Task 4 替换。）

```go
// 占位:Task 4 补齐
func (s *Service) deliverReal(ctx context.Context, tenantID int64, a, b Candidate, now time.Time) {}
func (s *Service) deliverRobot(ctx context.Context, tenantID int64, self Candidate, paired func(int64) bool, now time.Time) {}
```

---

### Task 4: 送达（弹窗载荷、机器人开场白、`/spark/accept`）

**Files:**
- Create: `server/internal/spark/deliver.go`
- Create: `server/internal/spark/handler.go`
- Create: `server/internal/spark/deliver_test.go`
- Modify: `server/internal/spark/service.go`（删掉 Task 3 的两个占位方法）

**Interfaces:**
- Consumes: Task 3 的 `Service` / `Candidate` / `Deps` / `pairKey`。
- Produces:
  - `type Payload struct { Type string; ChatID string; PeerID string; Peer PeerInfo; Title, Text, TitleEn, TextEn string }`
  - `type PeerInfo struct { ID, Nickname, Avatar string }`
  - `func buildPayload(chatID int64, peer Candidate, title, text, titleEn, textEn string) Payload`
  - `func renderCopy(tpl, fallback, nickname string) string`
  - `func (s *Service) Accept(tenantID, userID, peerID int64) (int64, error)`
  - `func NewHandler(svc *Service) *Handler` + `func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc)` → `POST /spark/accept`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/spark/deliver_test.go
package spark

import (
	"encoding/json"
	"strings"
	"testing"
)

// 文案:后台没配(空串)时用内置兜底;{nickname} 要被替换掉,不能把占位符推给用户看。
func TestRenderCopy(t *testing.T) {
	if got := renderCopy("", "{nickname} 与你碰撞出了火花", "小鱼"); got != "小鱼 与你碰撞出了火花" {
		t.Errorf("空配置应用兜底文案, got %q", got)
	}
	if got := renderCopy("你和 {nickname} 对上眼了", "兜底", "小鱼"); got != "你和 小鱼 对上眼了" {
		t.Errorf("配置了就用配置, got %q", got)
	}
	if got := renderCopy("没有占位符", "兜底", "小鱼"); got != "没有占位符" {
		t.Errorf("不写占位符也合法, got %q", got)
	}
	if strings.Contains(renderCopy("{nickname}", "兜底", ""), "{nickname}") {
		t.Error("昵称为空时也要把占位符去掉")
	}
}

// 真人配对的载荷**不能**带 chat_id:会话要等用户点了 /spark/accept 才建,
// 载荷里给了 chat_id 等于告诉客户端「已经有会话了」,它会直接跳进一个不存在的房间。
func TestBuildPayloadOmitsChatIDForRealMatch(t *testing.T) {
	p := buildPayload(0, Candidate{UserID: 42, Nickname: "小鱼", Avatar: "a.png"}, "标题", "正文", "T", "B")
	if p.ChatID != "" {
		t.Fatalf("真人配对不该有 chat_id, got %q", p.ChatID)
	}
	if p.PeerID != "42" || p.Peer.Nickname != "小鱼" || p.Type != "spark" {
		t.Fatalf("%+v", p)
	}
	// ID 一律字符串下发(JS 端 int64 精度会丢)
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), `"peer_id":42`) {
		t.Fatal("ID 必须以字符串下发")
	}

	p2 := buildPayload(777, Candidate{UserID: 42, Nickname: "小鱼"}, "标题", "正文", "T", "B")
	if p2.ChatID != "777" {
		t.Fatalf("机器人配对要带 chat_id, got %q", p2.ChatID)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/spark/ -run 'TestRenderCopy|TestBuildPayload' 2>&1 | tail -3`
Expected: FAIL（`renderCopy` / `buildPayload` 未定义）

- [ ] **Step 3: 实现 deliver.go**

```go
package spark

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

type PeerInfo struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// Payload WS 推给客户端的火花事件。
//
// 中英文各推一份:WS 连接上没有 Accept-Language,服务端不知道这个客户端是什么语种,
// 让客户端按自己的 locale 挑。空串表示后台没配,客户端用内置 ARB 文案。
type Payload struct {
	Type    string   `json:"type"`
	ChatID  string   `json:"chat_id,omitempty"` // 机器人配对才有;真人配对等 /spark/accept
	PeerID  string   `json:"peer_id"`
	Peer    PeerInfo `json:"peer"`
	Title   string   `json:"title"`
	Text    string   `json:"text"`
	TitleEn string   `json:"title_en"`
	TextEn  string   `json:"text_en"`
}

// renderCopy 后台文案为空时用兜底,并把 {nickname} 换成对方昵称。
func renderCopy(tpl, fallback, nickname string) string {
	if strings.TrimSpace(tpl) == "" {
		tpl = fallback
	}
	return strings.ReplaceAll(tpl, "{nickname}", nickname)
}

func buildPayload(chatID int64, peer Candidate, title, text, titleEn, textEn string) Payload {
	p := Payload{
		Type:   "spark",
		PeerID: strconv.FormatInt(peer.UserID, 10),
		Peer: PeerInfo{
			ID: strconv.FormatInt(peer.UserID, 10), Nickname: peer.Nickname, Avatar: peer.Avatar,
		},
		Title: title, Text: text, TitleEn: titleEn, TextEn: textEn,
	}
	if chatID != 0 {
		p.ChatID = strconv.FormatInt(chatID, 10)
	}
	return p
}

const (
	fallbackTitleZh = "有人和你对上眼了"
	fallbackTextZh  = "{nickname} 与你碰撞出了火花"
	fallbackTitleEn = "Someone caught your eye"
	fallbackTextEn  = "You and {nickname} just sparked"
)

// copyFor 组一对(中/英)标题与正文。
func copyFor(tenantID int64, nickname string) (title, text, titleEn, textEn string) {
	title = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTitle), fallbackTitleZh, nickname)
	text = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkText), fallbackTextZh, nickname)
	titleEn = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTitleEN), fallbackTitleEn, nickname)
	textEn = renderCopy(sysconfig.GetString(tenantID, sysconfig.KeySparkTextEN), fallbackTextEn, nickname)
	return
}

// markPaired 记下这一对今天配过了,TTL 到当天结束。
// 它同时是 /spark/accept 的授权依据:没有这条记录就不能免费建会话。
func (s *Service) markPaired(ctx context.Context, tenantID, a, b int64, now time.Time) {
	cache.RDB.Set(ctx, pairKey(tenantID, a, b), 1, endOfDay(now))
}

// deliverReal 真人配对:**不建会话**,只给双方推弹窗。
// 预建会话会让两人的消息列表各多出一个空会话——没人说话的那种。
func (s *Service) deliverReal(ctx context.Context, tenantID int64, a, b Candidate, now time.Time) {
	s.markPaired(ctx, tenantID, a.UserID, b.UserID, now)
	ta, xa, tae, xae := copyFor(tenantID, b.Nickname)
	s.d.Push(a.UserID, buildPayload(0, b, ta, xa, tae, xae))
	tb, xb, tbe, xbe := copyFor(tenantID, a.Nickname)
	s.d.Push(b.UserID, buildPayload(0, a, tb, xb, tbe, xbe))
	log.Printf("[spark] tenant=%d 真人配对 %d <-> %d", tenantID, a.UserID, b.UserID)
}

// deliverRobot 机器人配对:先建会话、让机器人说第一句,再弹窗。
// 顺序不能反:用户点进去要看到内容,空会话会让人直接退出去。
func (s *Service) deliverRobot(ctx context.Context, tenantID int64, self Candidate,
	paired func(int64) bool, now time.Time) {
	var bots []model.User
	s.d.DB.Select("user_id, nickname, avatar").
		Where("tenant_id = ? AND is_robot = ? AND status = ?", tenantID, true, "active").
		Limit(500).Find(&bots)
	var bot *model.User
	for i := range bots {
		if !paired(bots[i].UserID) {
			bot = &bots[i]
			break
		}
	}
	if bot == nil {
		return
	}
	chatID, err := s.d.EnsureChat(tenantID, bot.UserID, self.UserID)
	if err != nil {
		log.Printf("[spark] 建会话失败 tenant=%d bot=%d user=%d: %v", tenantID, bot.UserID, self.UserID, err)
		return
	}
	if text := s.d.Opening(tenantID, bot.UserID); text != "" {
		if _, err := s.d.SendAs(tenantID, bot.UserID, chatID, text, "text"); err != nil {
			log.Printf("[spark] 开场白发送失败 chat=%d: %v", chatID, err)
		}
	}
	s.markPaired(ctx, tenantID, bot.UserID, self.UserID, now)
	peer := Candidate{UserID: bot.UserID, Nickname: bot.Nickname, Avatar: bot.Avatar}
	t, x, te, xe := copyFor(tenantID, bot.Nickname)
	s.d.Push(self.UserID, buildPayload(chatID, peer, t, x, te, xe))
	log.Printf("[spark] tenant=%d 机器人配对 bot=%d -> user=%d chat=%d", tenantID, bot.UserID, self.UserID, chatID)
}

// Accept 真人配对点击后换一个免费会话。
//
// ⚠️ **必须校验配对记录存在**:这是唯一拦住「任何人调这个接口就能和任意人免费开聊」的闸门。
// 没有它,price_chat 形同虚设。
func (s *Service) Accept(tenantID, userID, peerID int64) (int64, error) {
	if userID == peerID {
		return 0, errs.New(errs.CodeBadRequest, "不能和自己开聊")
	}
	ctx := context.Background()
	if cache.RDB.Exists(ctx, pairKey(tenantID, userID, peerID)).Val() == 0 {
		return 0, errs.New(errs.CodeForbidden, "匹配已过期")
	}
	var peer model.User
	if err := s.d.DB.Select("user_id, status").
		First(&peer, "tenant_id = ? AND user_id = ?", tenantID, peerID).Error; err != nil {
		return 0, errs.New(errs.CodeNotFound, "对方不存在")
	}
	if peer.Status != "active" {
		return 0, errs.New(errs.CodeForbidden, "对方账号不可用")
	}
	return s.d.EnsureChat(tenantID, userID, peerID)
}
```

删掉 `service.go` 里 Task 3 留的两个占位方法。

- [ ] **Step 4: 实现 handler.go**

```go
package spark

import (
	"strconv"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.POST("/spark/accept", auth, h.accept)
}

type acceptReq struct {
	PeerID string `json:"peer_id" binding:"required"`
}

// accept 真人火花点击「去聊聊」:校验配对记录后免费建会话。
func (h *Handler) accept(c *gin.Context) {
	var req acceptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	peerID, err := strconv.ParseInt(req.PeerID, 10, 64)
	if err != nil || peerID <= 0 {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	chatID, err := h.svc.Accept(middleware.TenantID(c), middleware.UserID(c), peerID)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "打开会话失败")
		return
	}
	response.OK(c, gin.H{"chat_id": strconv.FormatInt(chatID, 10)})
}
```

- [ ] **Step 5: 补 Accept 的闸门测试**

```go
// 追加到 server/internal/spark/deliver_test.go
// 防白嫖闸门:没被匹配过的 peer 必须拒绝。这条如果失效,任何登录用户
// 都能拿 /spark/accept 和任意人免费开聊,price_chat 就白设了。
func TestAcceptRejectsSelf(t *testing.T) {
	s := New(Deps{})
	if _, err := s.Accept(1, 100, 100); err == nil {
		t.Fatal("和自己开聊应被拒")
	}
}
```

> 配对记录校验那条需要 Redis，仓库没有 DB/Redis 测试基建（27 个 `_test.go` 里 `gorm.Open` 出现 0 次）。本步骤只覆盖不依赖外部的那半；Redis 那半由 §六的人工联调一条覆盖，并在 `Accept` 的注释里标明它是闸门。

- [ ] **Step 6: 运行确认通过**

Run: `cd server && gofmt -l internal/spark; go build ./... && go vet ./internal/spark/ && go test ./internal/spark/ -v 2>&1 | grep -E "^(--- |ok|FAIL)"`
Expected: 全 PASS

---

### Task 5: 配置键与装配

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces：10 个键常量 + defaults；`GroupSpark = "spark"`；`spark.Start(...)` 与路由装配。

- [ ] **Step 1: 写失败测试**

```go
// 追加到 server/internal/sysconfig/app_config_test.go
// 火花是主动打扰用户的功能:默认必须全关,文案默认空串(空=App 用内置文案)。
// 默认值一旦写错,所有 App 租户会在发版当天集体开始弹窗。
func TestSparkDefaultsAreOff(t *testing.T) {
	if defaults[KeySparkEnabled] != "0" {
		t.Errorf("spark_enabled 默认必须关, got %q", defaults[KeySparkEnabled])
	}
	for _, k := range []string{KeySparkTitle, KeySparkText, KeySparkTitleEN, KeySparkTextEN} {
		if defaults[k] != "" {
			t.Errorf("%s 默认应为空串(用 App 内置文案), got %q", k, defaults[k])
		}
	}
	nums := map[string]string{
		KeySparkIntervalMin: "20", KeySparkIntervalMax: "60", KeySparkRealRatio: "30",
		KeySparkWindowStart: "10", KeySparkWindowEnd: "23", KeySparkUserDailyCap: "3",
	}
	for k, want := range nums {
		if defaults[k] != want {
			t.Errorf("%s 默认 = %q, want %q", k, defaults[k], want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/sysconfig/ -run TestSparkDefaultsAreOff 2>&1 | tail -3`
Expected: FAIL（常量未定义）

- [ ] **Step 3: 实现 sysconfig.go**

在 `KeyOutreachUserDailyCap` 之后加常量：

```go
	// 主动匹配(火花):在线真人每隔随机 N–M 分钟被系统配一个人,App 弹窗。
	// 与 outreach 共用每日计数键,当天互斥。
	KeySparkEnabled      = "spark_enabled"        // 总开关 (0/1),默认关
	KeySparkIntervalMin  = "spark_interval_min"   // 每人下次匹配的最小间隔(分钟)
	KeySparkIntervalMax  = "spark_interval_max"   // 最大间隔(分钟)
	KeySparkRealRatio    = "spark_real_ratio"     // 配到真人的概率 A(%),其余配机器人
	KeySparkWindowStart  = "spark_window_start"   // 时段开始小时 0-23(含)
	KeySparkWindowEnd    = "spark_window_end"     // 时段结束小时 0-23(不含)
	KeySparkUserDailyCap = "spark_user_daily_cap" // 每人每日上限(与 outreach 共用计数)
	KeySparkTitle        = "spark_title"          // 弹窗标题,空=App 内置文案
	KeySparkTitleEN      = "spark_title_en"
	KeySparkText         = "spark_text" // 弹窗正文,支持 {nickname}
	KeySparkTextEN       = "spark_text_en"
```

defaults 里加：

```go
	KeySparkEnabled:      "0",
	KeySparkIntervalMin:  "20",
	KeySparkIntervalMax:  "60",
	KeySparkRealRatio:    "30",
	KeySparkWindowStart:  "10",
	KeySparkWindowEnd:    "23",
	KeySparkUserDailyCap: "3",
	KeySparkTitle:        "",
	KeySparkTitleEN:      "",
	KeySparkText:         "",
	KeySparkTextEN:       "",
```

- [ ] **Step 4: 实现 meta.go**

`GroupAppDiscover` 之后加分组常量 `GroupSpark = "spark"`，`groupMeta` 里加：

```go
	GroupSpark: {"主动匹配", "Proactive matching", SectionGrowth, PlatformApp},
```

`configMeta` 里加 11 条（紧挨 App 发现页那组）：

```go
	{Key: sysconfig.KeySparkEnabled, LabelZh: "主动匹配总开关(默认关)", LabelEn: "Proactive matching master switch (off by default)", Group: GroupSpark, Type: "bool"},
	{Key: sysconfig.KeySparkIntervalMin, LabelZh: "每人匹配最小间隔(分钟)", LabelEn: "Minimum interval per user (minutes)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkIntervalMax, LabelZh: "每人匹配最大间隔(分钟)", LabelEn: "Maximum interval per user (minutes)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkRealRatio, LabelZh: "配到真人的概率(%,其余为机器人)", LabelEn: "Chance of matching a real user (%); rest are bots", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkWindowStart, LabelZh: "匹配时段开始小时", LabelEn: "Matching window start hour", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkWindowEnd, LabelZh: "匹配时段结束小时(不含)", LabelEn: "Matching window end hour (exclusive)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkUserDailyCap, LabelZh: "每人每日匹配上限(与主动触达共用)", LabelEn: "Daily matches per user (shared with bot outreach)", Group: GroupSpark, Type: "int"},
	{Key: sysconfig.KeySparkTitle, LabelZh: "弹窗标题", LabelEn: "Popup title", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkTitleEN, LabelZh: "弹窗标题(英文)", LabelEn: "Popup title (English)", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkText, LabelZh: "弹窗正文(支持 {nickname})", LabelEn: "Popup body (supports {nickname})", Group: GroupSpark, Type: "text"},
	{Key: sysconfig.KeySparkTextEN, LabelZh: "弹窗正文(英文)", LabelEn: "Popup body (English)", Group: GroupSpark, Type: "text"},
```

- [ ] **Step 5: 装配 main.go**

在 `chatSvc` 与 `robot.StartOutreachCron` 之后、路由注册附近加：

```go
	// 主动匹配(火花):在线真人定时被配一个人,App 弹窗;总开关 spark_enabled 默认关
	sparkDeps := spark.Deps{
		DB:         db,
		Online:     hub.OnlineUserIDs,
		Push:       func(userID int64, payload any) { hub.PushTo(userID, payload) },
		EnsureChat: chatSvc.EnsureFreeChat,
		SendAs:     chatSvc.SendMessage,
		Opening:    robotSvc.Opening,
		ClaimDaily: robot.ClaimDailyReach,
		IsBlocked:  modSvc.IsBlocked,
	}
	sparkSvc := spark.New(sparkDeps)
	spark.NewHandler(sparkSvc).Register(api, auth)
	spark.Start(sparkDeps, cfg.DefaultTenantID)
```

`robotSvc.Opening` 需要在 `internal/robot` 导出一个包装（`genOpening` 现在是私有的）：

```go
// 追加到 server/internal/robot/outreach.go
// Opening 供外部(spark)复用机器人开场白生成:LLM 失败时回退静态内容池。
func (s *Service) Opening(tenantID, botUserID int64) string {
	return genOpening(s, tenantID, botUserID)
}
```

import 加 `"driftbottle/internal/spark"`。

- [ ] **Step 6: 运行确认通过**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/sysconfig/ ./internal/admin/ ./internal/spark/ ./cmd/... 2>&1 | tail -5`
Expected: PASS（`meta_test.go` 的分组/分区一致性测试自动覆盖 `GroupSpark`）

---

### Task 6: 客户端数据层（两个工程各一遍）

**Files（`app/bottles_zh` 与 `app/bottles` 路径相同，各做一次）:**
- Create: `lib/domain/models/spark.dart`
- Modify: `lib/data/repositories.dart`
- Modify: `lib/data/remote/remote_repositories.dart`
- Modify: `lib/data/mock/mock_repositories.dart`
- Modify: `lib/core/providers.dart`
- Create: `test/spark_model_test.dart`

**Interfaces:**
- Produces:
  - `class SparkEvent { final String? chatId; final String peerId; final String nickname; final String avatar; final String title; final String body; factory SparkEvent.fromJson(Map<String, Object?> j, {required bool english}); bool get hasChat; }`
  - `abstract class SparkRepository { Future<String> accept(String peerId); }`
  - `final sparkRepoProvider = Provider<SparkRepository>(...)`

- [ ] **Step 1: 写失败测试**

```dart
// test/spark_model_test.dart
import 'package:bottles/domain/models/spark.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const payload = {
    'type': 'spark',
    'peer_id': '42',
    'peer': {'id': '42', 'nickname': '小鱼', 'avatar': 'a.png'},
    'title': '有人和你对上眼了',
    'text': '小鱼 与你碰撞出了火花',
    'title_en': 'Someone caught your eye',
    'text_en': 'You and 小鱼 just sparked',
  };

  test('picks the copy matching the app locale', () {
    final zh = SparkEvent.fromJson(payload, english: false);
    expect(zh.title, '有人和你对上眼了');
    expect(zh.body, '小鱼 与你碰撞出了火花');
    final en = SparkEvent.fromJson(payload, english: true);
    expect(en.title, 'Someone caught your eye');
  });

  // 服务端那半没配文案时推空串,客户端必须回落到内置文案,不能显示空白弹窗
  test('falls back to built-in copy when the server sends empty strings', () {
    final e = SparkEvent.fromJson({
      ...payload,
      'title': '',
      'text': '',
      'title_en': '',
      'text_en': '',
    }, english: false);
    expect(e.title.isNotEmpty, isTrue);
    expect(e.body.contains('小鱼'), isTrue);
  });

  // 真人配对没有 chat_id:客户端据此决定是直接跳转还是先调 /spark/accept
  test('hasChat distinguishes bot match from real match', () {
    expect(SparkEvent.fromJson(payload, english: false).hasChat, isFalse);
    expect(
      SparkEvent.fromJson({...payload, 'chat_id': '777'}, english: false).hasChat,
      isTrue,
    );
  });
}
```

- [ ] **Step 2: 运行确认失败**

Run（PowerShell，`cd app/bottles_zh`）: `flutter test test/spark_model_test.dart`
Expected: FAIL（`spark.dart` 不存在）

- [ ] **Step 3: 实现 `lib/domain/models/spark.dart`**

```dart
/// 一次「碰撞出火花」事件（WS `type: "spark"`）。
///
/// 服务端中英文各推一份：WS 连接上没有 Accept-Language，它不知道这个客户端
/// 是什么语种，所以两份都给，由这里按 App 当前 locale 挑。
class SparkEvent {
  const SparkEvent({
    required this.chatId,
    required this.peerId,
    required this.nickname,
    required this.avatar,
    required this.title,
    required this.body,
  });

  /// 机器人配对才有：会话已建好、开场白也发了，点击直接跳。
  /// 真人配对为 null，要先调 `/spark/accept` 换一个 chat_id。
  final String? chatId;
  final String peerId;
  final String nickname;
  final String avatar;
  final String title;
  final String body;

  bool get hasChat => (chatId ?? '').isNotEmpty;

  static const _fallbackTitleZh = '有人和你对上眼了';
  static const _fallbackBodyZh = '{nickname} 与你碰撞出了火花';
  static const _fallbackTitleEn = 'Someone caught your eye';
  static const _fallbackBodyEn = 'You and {nickname} just sparked';

  factory SparkEvent.fromJson(Map<String, Object?> j, {required bool english}) {
    final peer = (j['peer'] as Map?)?.cast<String, Object?>() ?? const {};
    final nickname = '${peer['nickname'] ?? ''}';
    String pick(String key, String enKey, String fallback) {
      final v = '${j[english ? enKey : key] ?? ''}';
      final tpl = v.trim().isEmpty ? fallback : v;
      return tpl.replaceAll('{nickname}', nickname);
    }

    final chat = '${j['chat_id'] ?? ''}';
    return SparkEvent(
      chatId: chat.isEmpty ? null : chat,
      peerId: '${j['peer_id'] ?? ''}',
      nickname: nickname,
      avatar: '${peer['avatar'] ?? ''}',
      title: pick('title', 'title_en', english ? _fallbackTitleEn : _fallbackTitleZh),
      body: pick('text', 'text_en', english ? _fallbackBodyEn : _fallbackBodyZh),
    );
  }
}
```

- [ ] **Step 4: 仓库三件套**

`lib/data/repositories.dart` 加：

```dart
abstract class SparkRepository {
  /// `POST /api/spark/accept` —— 真人火花点「去聊聊」时换一个免费会话。
  /// 服务端会校验这对确实被匹配过；过期返回错误，调用方提示即可。
  Future<String> accept(String peerId);
}
```

`lib/data/remote/remote_repositories.dart` 加：

```dart
class RemoteSparkRepository implements SparkRepository {
  RemoteSparkRepository(this._api);

  final ApiClient _api;

  @override
  Future<String> accept(String peerId) async {
    final data = _obj(
      await _api.post<dynamic>('/spark/accept', body: {'peer_id': peerId}),
    );
    return '${data['chat_id'] ?? ''}';
  }
}
```

`lib/data/mock/mock_repositories.dart` 加：

```dart
class MockSparkRepository implements SparkRepository {
  @override
  Future<String> accept(String peerId) async => 'mock-chat-$peerId';
}
```

`lib/core/providers.dart` 加（与其它仓库 provider 同样式）：

```dart
final sparkRepoProvider = Provider<SparkRepository>((ref) {
  if (AppConfig.useMock) return MockSparkRepository();
  return RemoteSparkRepository(ref.watch(apiClientProvider));
});
```

> `_obj` 与 `apiClientProvider` 在各自文件里已有，照现有写法用即可。

- [ ] **Step 5: 运行确认通过**

Run（两个工程各跑一次）: `flutter test test/spark_model_test.dart && flutter analyze`
Expected: PASS；No issues found

---

### Task 7: 客户端弹窗与 WS 接入（两个工程各一遍）

**Files（两个工程路径相同）:**
- Create: `lib/features/spark/spark_overlay.dart`
- Modify: `lib/core/network/chat_socket.dart`
- Modify: `lib/app/app.dart`（挂全局监听）
- Modify: `lib/l10n/app_zh.arb`、`lib/l10n/app_en.arb`
- Create: `test/widgets/spark_overlay_test.dart`

**Interfaces:**
- Consumes: Task 6 的 `SparkEvent`、`sparkRepoProvider`。
- Produces:
  - `final sparkStreamProvider = StreamProvider<SparkEvent>(...)`（从 socket 分流出来）
  - `class SparkDialog extends ConsumerWidget`（弹窗本体）
  - `Future<void> showSparkDialog(BuildContext context, SparkEvent e)` —— 已有弹窗时直接返回，不排队
  - l10n 键：`sparkGoChat`、`sparkLater`、`sparkExpired`

- [ ] **Step 1: 写失败测试**

```dart
// test/widgets/spark_overlay_test.dart
import 'package:bottles/domain/models/spark.dart';
import 'package:bottles/features/spark/spark_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

SparkEvent _event({String? chatId}) => SparkEvent(
  chatId: chatId,
  peerId: '42',
  nickname: '小鱼',
  avatar: '',
  title: '有人和你对上眼了',
  body: '小鱼 与你碰撞出了火花',
);

void main() {
  testWidgets('shows the peer nickname and both buttons', (t) async {
    await pumpAt(t, layoutMatrix[2], SparkDialog(event: _event()));
    expect(find.textContaining('小鱼'), findsWidgets);
    expect(find.text('有人和你对上眼了'), findsOneWidget);
  });

  // 两条火花同时到(重连补推 / 两个事件挨着)时只弹一个,不叠弹窗也不排队——
  // 叠起来用户要连点两次才能回到原来的页面。
  testWidgets('a second spark while one is open is dropped', (t) async {
    await pumpAt(t, layoutMatrix[2], const _Host());
    final ctx = t.element(find.byType(_Host));
    await showSparkDialog(ctx, _event());
    await t.pump();
    await showSparkDialog(ctx, _event());
    await t.pump();
    expect(find.byType(SparkDialog), findsOneWidget);
  });
}

class _Host extends StatelessWidget {
  const _Host();
  @override
  Widget build(BuildContext context) => const Scaffold(body: SizedBox());
}
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/widgets/spark_overlay_test.dart`
Expected: FAIL（`spark_overlay.dart` 不存在）

- [ ] **Step 3: 实现 `lib/features/spark/spark_overlay.dart`**

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/routes.dart';
import '../../core/design/tokens.dart';
import '../../core/providers.dart';
import '../../domain/models/spark.dart';
import '../../l10n/app_localizations.dart';
import '../../ui/widgets/buttons.dart';
import '../../ui/widgets/overlays.dart';

/// 同一时刻只允许一个火花弹窗。
///
/// 第二条直接丢弃而不是排队：排队意味着用户关掉一个又冒出一个，
/// 要连点两次才能回到他原本在看的页面。
bool _sparkOpen = false;

Future<void> showSparkDialog(BuildContext context, SparkEvent e) async {
  if (_sparkOpen) return;
  _sparkOpen = true;
  try {
    await showDialog<void>(
      context: context,
      barrierDismissible: true,
      builder: (_) => SparkDialog(event: e),
    );
  } finally {
    _sparkOpen = false;
  }
}

class SparkDialog extends ConsumerStatefulWidget {
  const SparkDialog({super.key, required this.event});

  final SparkEvent event;

  @override
  ConsumerState<SparkDialog> createState() => _SparkDialogState();
}

class _SparkDialogState extends ConsumerState<SparkDialog> {
  bool _busy = false;

  Future<void> _go() async {
    final e = widget.event;
    // 机器人配对：会话已建好、开场白也发了，直接跳。
    if (e.hasChat) {
      Navigator.of(context).pop();
      context.push(Routes.chat(e.chatId!));
      return;
    }
    // 真人配对：先换一个免费会话。服务端会校验这对确实被匹配过。
    setState(() => _busy = true);
    try {
      final chatId = await ref.read(sparkRepoProvider).accept(e.peerId);
      if (!mounted) return;
      Navigator.of(context).pop();
      context.push(Routes.chat(chatId));
    } catch (_) {
      if (!mounted) return;
      setState(() => _busy = false);
      showToast(context, L.of(context).sparkExpired, error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final e = widget.event;
    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(Dim.r4)),
      child: Padding(
        padding: const EdgeInsets.all(Dim.s5),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CircleAvatar(
              radius: 36,
              backgroundImage: e.avatar.isEmpty ? null : NetworkImage(e.avatar),
              child: e.avatar.isEmpty ? Text(e.nickname.characters.take(1).toString()) : null,
            ),
            const SizedBox(height: Dim.s4),
            Text(
              e.title,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: Dim.t4, fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: Dim.s2),
            Text(e.body, textAlign: TextAlign.center),
            const SizedBox(height: Dim.s5),
            AppButton(label: l.sparkGoChat, loading: _busy, onTap: _busy ? null : _go),
            const SizedBox(height: Dim.s2),
            AppButton(
              label: l.sparkLater,
              kind: BtnKind.ghost,
              onTap: _busy ? null : () => Navigator.of(context).pop(),
            ),
          ],
        ),
      ),
    );
  }
}
```

（`characters` 需要 `import 'package:characters/characters.dart';`，Flutter SDK 自带。）

- [ ] **Step 4: WS 分流与全局挂载**

`lib/core/network/chat_socket.dart`：现有解析里按 `type` 分发，加一条

```dart
      case 'spark':
        _sparkController.add(SparkEvent.fromJson(m, english: english));
        break;
```

并暴露 `Stream<SparkEvent> get sparkStream => _sparkController.stream;`（`english` 由构造时传入的 locale 决定，与现有 socket 取配置的方式一致）。

`lib/core/providers.dart` 加：

```dart
final sparkStreamProvider = StreamProvider<SparkEvent>(
  (ref) => ref.watch(chatSocketProvider).sparkStream,
);
```

`lib/app/app.dart` 在 `MaterialApp.router` 的 `builder` 里监听（用 `navigatorKey.currentContext` 保证任意页面都能弹）：

```dart
    ref.listen(sparkStreamProvider, (_, next) {
      final e = next.valueOrNull;
      final ctx = rootNavigatorKey.currentContext;
      if (e != null && ctx != null) showSparkDialog(ctx, e);
    });
```

`rootNavigatorKey` 若 `router.dart` 未导出则补一个 `export`。

- [ ] **Step 5: 文案**

`lib/l10n/app_zh.arb` / `app_en.arb` 各加三条：

```json
  "sparkGoChat": "去聊聊",     "sparkGoChat": "Say hi",
  "sparkLater": "以后再说",    "sparkLater": "Maybe later",
  "sparkExpired": "这次匹配已经过期了",  "sparkExpired": "This match has expired"
```

然后 `flutter gen-l10n`。

- [ ] **Step 6: 运行确认通过（两个工程各一次）**

Run: `flutter gen-l10n && flutter test test/widgets/spark_overlay_test.dart test/spark_model_test.dart && flutter analyze`
Expected: PASS；No issues found

---

### Task 8: 全量验证与文档

- [ ] **Step 1: 服务端全量**

Run: `cd server && go build ./... && go vet ./... && go test ./...`
Expected: 全绿，除 `internal/robot` 的 `TestBuildIdentityResponse_*`（既有失败，与本计划无关，`internal/robot` 本计划只导出两个函数，不动它们的逻辑）

- [ ] **Step 2: 两个 Flutter 工程**

Run（PowerShell）: `cd app/bottles_zh && flutter analyze && flutter test`，再对 `app/bottles` 跑同样两条
Expected: No issues found；全部测试通过

- [ ] **Step 3: 后台**

Run: `cd admin && npm run build`
Expected: 通过（`/config` 会多出「主动匹配」分组，无需改前端代码）

- [ ] **Step 4: 文档**

`CLAUDE.md`「关键机制速查」加一条：

> - **主动匹配（火花）**（2026-10-07）：在线真人每隔随机 N–M 分钟被系统配一个人（`spark_real_ratio` 概率配真人、其余配机器人），App 弹窗后点进去**免开聊费**。调度在 `internal/spark`（每分钟扫 WS 在线用户，每人一个 Redis TTL 键）。机器人配对预建会话 + 发开场白；真人配对双向弹窗但不预建，点击走 `POST /spark/accept`——**那个接口必须校验 Redis 配对记录**，否则任何人都能和任意人免费开聊。与 `robot/outreach.go` 共用每日计数键 `outreach:<租户>:<uid>:<日期>`，当天互斥。总开关 `spark_enabled` 默认关。

- [ ] **Step 5: 人工联调清单（上线前按顺序走）**

1. 后台「主动匹配」开总开关，`spark_interval_min/max` 临时改成 `1`/`2` 便于观察。
2. 两台设备登录两个真人账号，都停在 App 前台（WS 连着）。
3. 把 `spark_real_ratio` 调到 `100`，确认两边同时弹窗、文案里的昵称是对方。
4. 一边点「去聊聊」，确认进了会话且**钱包流水没有开聊扣费记录**；另一边点进去是同一个会话。
5. 把 `spark_real_ratio` 调到 `0`，确认配到机器人、点进去已有一条开场白。
6. 用 Postman 直接调 `POST /spark/accept`，`peer_id` 填一个从没匹配过的用户 → **必须返回「匹配已过期」**。这条是防白嫖闸门，务必实测。
7. 把 `spark_user_daily_cap` 调成 `1`，确认同一天不会弹第二次；再开 `outreach_enabled`，确认两者共用额度（被机器人私信过就不再弹火花）。
8. 验完把间隔、比例、上限改回生产值，总开关按运营计划决定开关。
