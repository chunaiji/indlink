# 机器人内容多样化 + 同城注入 + 主动引导 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复漂流瓶内容同质化、在同城列表注入机器人、让机器人主动引导真人聊天。

**Architecture:** 全部在 Go 后端。#1 把造瓶改成"成长型 robot_contents 池 + 随机主题 LLM"；#2 在 `match.CityUsers` 结果里注入机器人卡片；#3 复用 `BotWorkerPool` 的 `MessageSender`，在 `robot.Tick` 挂主动破冰/沉默追问子任务、在回信后引导加聊、并强化提示词。全部行为由 sysconfig 配置开关，主动引导默认关闭。

**Tech Stack:** Go 1.x, Gin, GORM, MySQL(loc=Local, CST), Redis(go-redis)。

## Global Constraints

- 数据库时间为东八区（DSN `loc=Local`，服务器 CST）；跨天/日聚合按此口径。
- 遵循现有 raw-SQL / GORM 链式写法与 `sysconfig.GetInt/GetBool/GetString` 配置读取。
- 新配置项必须同时加到 `sysconfig` 常量+defaults 和 `admin/meta.go` 的 `configMeta`（后台配置页据此渲染）。
- 主动引导总开关 `KeyRobotProactive` 默认 `"0"`（关）；#1/#2 默认生效。
- 每步结束保证 `cd server && go build ./...` 通过；纯逻辑函数必须先写失败测试（TDD）。
- 不破坏现有测试：`go test ./...` 全绿。
- 提交信息结尾加 `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`。

---

## File Structure

- `server/internal/sysconfig/sysconfig.go` — 新增 10 个配置键常量 + defaults。
- `server/internal/admin/meta.go` — 新增对应 ConfigField。
- `server/internal/robot/service.go` — `throwBottles` 重写、`genBottleText`、`bottleExists`、`SeedContent` 扩池、`maybeReply` 加引导加聊、`Tick` 挂子任务、`SetSender`、`sender` 字段。
- `server/internal/robot/bottlegen.go`（新增）— `bottleThemes`/`bottleMoods`/`buildBottlePrompt`。
- `server/internal/robot/bottlegen_test.go`（新增）— buildBottlePrompt 单测。
- `server/internal/robot/engage.go`（新增）— `trailingRobotMsgs`、`isOutreachCandidate`、`proactiveOutreach`、`nudgeStalledChats`、开场/追问模板。
- `server/internal/robot/engage_test.go`（新增）— trailingRobotMsgs / isOutreachCandidate 单测。
- `server/internal/robot/promptbuilder.go` — buildPrompt 追加提问钩子。
- `server/internal/robot/bot_worker.go` — （无需改，SetSender 在 main 调）。
- `server/internal/match/service.go` — `UserCard.IsRobot`、`toCards` 设值、`injectRobots`、`candidateRobots`、`CityUsers` 接线。
- `server/internal/match/inject_test.go`（新增）— injectRobots 单测。
- `server/cmd/api/main.go` — `robotSvc.SetSender(chatSvc)` 接线。
- 一次性 DB 脚本（ops，非代码）— 软删机器人旧瓶。

---

## Task 1: 新增配置键

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`

**Interfaces:**
- Produces: 常量 `KeyRobotBottlePoolTarget, KeyCityRobotTopM, KeyCityRobotTopN, KeyRobotProactive, KeyRobotBottle2ChatRatio, KeyRobotOutreachDailyCap, KeyRobotOutreachNewHours, KeyRobotOutreachSilentDays, KeyRobotNudgeSilentMin, KeyRobotNudgeMax`（均 `string` 值键）。

- [ ] **Step 1: 加常量**（`sysconfig.go` 的 `const (...)` 键区块末尾，紧跟现有机器人相关键）

```go
	// 机器人内容/同城/主动引导(#robot-quality)
	KeyRobotBottlePoolTarget   = "robot_bottle_pool_target"   // AI 瓶子池目标条数
	KeyCityRobotTopM           = "city_robot_top_m"           // 同城前 M 个位置
	KeyCityRobotTopN           = "city_robot_top_n"           // 其中机器人数(0=关)
	KeyRobotProactive          = "robot_proactive_enabled"    // 主动引导总开关(b/c/d)
	KeyRobotBottle2ChatRatio   = "robot_bottle2chat_ratio"    // 回信后引导加聊概率(%)
	KeyRobotOutreachDailyCap   = "robot_outreach_daily_cap"   // 破冰每租户每日上限
	KeyRobotOutreachNewHours   = "robot_outreach_new_hours"   // 新用户窗口(小时)
	KeyRobotOutreachSilentDays = "robot_outreach_silent_days" // 沉默用户阈值(天)
	KeyRobotNudgeSilentMin     = "robot_nudge_silent_min"     // 会话沉默追问阈值(分)
	KeyRobotNudgeMax           = "robot_nudge_max_per_chat"   // 每会话最多追问次数
```

- [ ] **Step 2: 加 defaults**（`defaults` map 末尾）

```go
	KeyRobotBottlePoolTarget:   "80",
	KeyCityRobotTopM:           "10",
	KeyCityRobotTopN:           "3",
	KeyRobotProactive:          "0",
	KeyRobotBottle2ChatRatio:   "20",
	KeyRobotOutreachDailyCap:   "20",
	KeyRobotOutreachNewHours:   "24",
	KeyRobotOutreachSilentDays: "7",
	KeyRobotNudgeSilentMin:     "30",
	KeyRobotNudgeMax:           "1",
```

- [ ] **Step 3: 加 admin ConfigField**（`meta.go` `configMeta` 末尾，机器人相关分组）

```go
	{Key: sysconfig.KeyRobotBottlePoolTarget, Label: "AI瓶子池目标条数", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyCityRobotTopM, Label: "同城前M个位置", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyCityRobotTopN, Label: "同城前M中机器人数(0关)", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotProactive, Label: "主动引导总开关", Group: "机器人", Type: "bool"},
	{Key: sysconfig.KeyRobotBottle2ChatRatio, Label: "回信后引导加聊概率%", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotOutreachDailyCap, Label: "破冰每租户每日上限", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotOutreachNewHours, Label: "破冰-新用户窗口(小时)", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotOutreachSilentDays, Label: "破冰-沉默阈值(天)", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotNudgeSilentMin, Label: "沉默追问阈值(分)", Group: "机器人", Type: "int"},
	{Key: sysconfig.KeyRobotNudgeMax, Label: "每会话最多追问次数", Group: "机器人", Type: "int"},
```

- [ ] **Step 4: 构建校验**

Run: `cd server && go build ./...`
Expected: exit 0

- [ ] **Step 5: Commit**

```bash
git add server/internal/sysconfig/sysconfig.go server/internal/admin/meta.go
git commit -m "feat(config): 新增机器人内容/同城/主动引导配置键"
```

---

## Task 2: #1 漂流瓶内容多样化

**Files:**
- Create: `server/internal/robot/bottlegen.go`
- Create: `server/internal/robot/bottlegen_test.go`
- Modify: `server/internal/robot/service.go`（`throwBottles`、加 `genBottleText`/`bottleExists`、`SeedContent` 扩池）

**Interfaces:**
- Produces: `buildBottlePrompt(theme, mood string) string`, `bottleThemes []string`, `bottleMoods []string`, `func (s *Service) genBottleText(tenantID int64, personaRole, theme, mood string) string`, `func bottleExists(pool []model.RobotContent, text string) bool`。

- [ ] **Step 1: 写失败测试** `server/internal/robot/bottlegen_test.go`

```go
package robot

import "testing"

func TestBuildBottlePrompt_ContainsThemeAndMood(t *testing.T) {
	p := buildBottlePrompt("失眠的夜", "温柔治愈")
	if !contains(p, "失眠的夜") || !contains(p, "温柔治愈") {
		t.Fatalf("prompt 缺主题或语气: %s", p)
	}
	if !contains(p, "50字") {
		t.Fatalf("prompt 缺字数约束: %s", p)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/robot/ -run TestBuildBottlePrompt`
Expected: FAIL（`undefined: buildBottlePrompt`）

- [ ] **Step 3: 实现** `server/internal/robot/bottlegen.go`

```go
package robot

// 造瓶随机主题/语气,保证 AI 生成多样,避免同质化。
var bottleThemes = []string{
	"加班晚归", "失眠的夜", "一个人旅行", "街角的小店", "想养只猫",
	"突然想家", "深夜emo", "刚搬到新城市", "考试季的压力", "下雨天",
	"看到晚霞", "地铁上的陌生人", "减肥又失败了", "久违的老朋友", "一杯热奶茶",
	"周末宅家", "分手后的日子", "第一次一个人吃火锅", "小时候的味道", "想被人理解",
}

var bottleMoods = []string{"温柔治愈", "有点丧但不消极", "轻松俏皮", "平静自省", "真诚走心"}

// buildBottlePrompt 拼装造瓶用户提示词。
func buildBottlePrompt(theme, mood string) string {
	return "以「" + theme + "」为主题，用" + mood + "的语气写一条真实、有温度的漂流瓶，50字以内，" +
		"自然地留一个引发共鸣或回应的钩子，不要打招呼，不要加引号。"
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/robot/ -run TestBuildBottlePrompt`
Expected: PASS

- [ ] **Step 5: 加 `genBottleText` 与 `bottleExists`**（`service.go`，放在 `genAIText` 附近；`gocontext`/`strings`/`log`/`time` 已导入）

```go
// genBottleText 直连 LLM 生成一条造瓶文案(绕过按角色的粗缓存,保证多样),失败返回 ""。
func (s *Service) genBottleText(tenantID int64, personaRole, theme, mood string) string {
	bgCtx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
	defer cancel()
	reply, err := s.llm.Chat(bgCtx, tenantID, s.buildPathABSystemPrompt(personaRole), nil, buildBottlePrompt(theme, mood))
	if err != nil {
		log.Printf("[robot] genBottleText err: %v", err)
		return ""
	}
	return strings.TrimSpace(reply)
}

func bottleExists(pool []model.RobotContent, text string) bool {
	for _, c := range pool {
		if c.Text == text {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: 重写 `throwBottles`**（替换 `service.go` 整个 `throwBottles` 函数体）

```go
func (s *Service) throwBottles(tenantID int64, robots []model.User, n int) {
	var pool []model.RobotContent
	s.db.Where("tenant_id = ? AND type = ?", tenantID, "bottle").Find(&pool)

	poolTarget := sysconfig.GetInt(tenantID, sysconfig.KeyRobotBottlePoolTarget)
	if poolTarget <= 0 {
		poolTarget = 80
	}
	aiEnabled := sysconfig.GetBool(tenantID, sysconfig.KeyAIBotEnabled)
	now := time.Now()

	for i := 0; i < n; i++ {
		r := robots[rand.Intn(len(robots))]
		var text, tags string

		// 池未满且 AI 开启:生成新文案(随机主题)入池;否则从池随机取。
		if aiEnabled && len(pool) < poolTarget {
			theme := bottleThemes[rand.Intn(len(bottleThemes))]
			mood := bottleMoods[rand.Intn(len(bottleMoods))]
			persona := s.registry.Get(r.UserID)
			gen := s.genBottleText(tenantID, persona.AffectiveStyle, theme, mood)
			if gen != "" && !bottleExists(pool, gen) {
				rc := model.RobotContent{TenantID: tenantID, Type: "bottle", Text: gen, Weight: 1, CreatedAt: now}
				if s.db.Create(&rc).Error == nil {
					pool = append(pool, rc)
				}
				text = gen
			}
		}
		if text == "" {
			if len(pool) == 0 {
				continue
			}
			c := pool[rand.Intn(len(pool))]
			text, tags = c.Text, c.Tags
		}

		b := model.Bottle{
			BottleID:    idgen.Next(),
			TenantID:    tenantID,
			UserID:      r.UserID,
			Content:     text,
			ContentType: "text",
			Tags:        tags,
			IsAnonymous: true,
			Scope:       "national",
			City:        r.City,
			Status:      "active",
			CreatedAt:   now,
			ExpireAt:    now.Add(7 * 24 * time.Hour),
		}
		s.db.Create(&b)
	}
	log.Printf("[robot] tenant=%d 投放 %d 个瓶子(池=%d/%d)", tenantID, n, len(pool), poolTarget)
}
```

- [ ] **Step 7: 扩充 `SeedContent` 静态兜底 bottle 池**（在 `SeedContent` 的 `bottles := []struct{...}{...}` 里，现有 10 条后追加以下条目）

```go
		{"地铁上看到一对老夫妻手牵手,突然很想有人陪。", "情感,交友"},
		{"减肥第一天就破功了,谁懂啊。", "吐槽,日常"},
		{"下雨天最适合躺着发呆,可惜还要上班。", "吐槽,日常"},
		{"今天被夸了一句,开心到现在。", "日常,治愈"},
		{"一个人吃火锅,店员问几位的时候有点尴尬。", "日常,树洞"},
		{"好久没联系的朋友突然发来消息,心里暖暖的。", "情感,交友"},
		{"深夜的便利店灯光,总让我觉得没那么孤单。", "治愈,树洞"},
		{"想去看海,谁陪我?不说话也行。", "交友,情感"},
		{"加班到最后一个走,城市的夜好安静。", "职场,树洞"},
		{"今天的云像棉花糖,拍了好多张照片。", "日常,治愈"},
		{"emo了,有没有人陪我说说话。", "情感,树洞"},
		{"突然好想吃小时候巷口那家的糖葫芦。", "日常,回忆"},
		{"新的一周,给自己打打气。", "日常,治愈"},
		{"失眠到三点,数羊都没用。", "失眠,树洞"},
		{"想找个人一起打游戏,菜也没关系。", "交友,日常"},
		{"今天对自己说了句辛苦了,眼眶有点热。", "情感,治愈"},
		{"一个人的城市,连生病都要自己扛。", "树洞,情感"},
		{"周末不想出门,只想躺平。", "吐槽,日常"},
		{"路过花店买了一束花给自己。", "治愈,日常"},
		{"想被人认真地问一句:你还好吗?", "情感,树洞"},
```

- [ ] **Step 8: 构建 + 全测**

Run: `cd server && go build ./... && go test ./internal/robot/`
Expected: build exit 0；测试 PASS

- [ ] **Step 9: Commit**

```bash
git add server/internal/robot/bottlegen.go server/internal/robot/bottlegen_test.go server/internal/robot/service.go
git commit -m "feat(robot): 造瓶改成长型内容池+随机主题, 修复同质化"
```

---

## Task 3: #2 同城机器人注入

**Files:**
- Modify: `server/internal/match/service.go`（`UserCard` 加字段、`toCards` 设值、`CityUsers` 接线、加 `injectRobots`/`candidateRobots`）
- Create: `server/internal/match/inject_test.go`

**Interfaces:**
- Consumes: `sysconfig.KeyCityRobotTopM/TopN`（Task 1）。
- Produces: `func injectRobots(cards, robots []UserCard, m, n int, pick func(int) int) []UserCard`（纯函数）。

- [ ] **Step 1: 写失败测试** `server/internal/match/inject_test.go`

```go
package match

import "testing"

func TestInjectRobots_FillsTopWindow(t *testing.T) {
	cards := make([]UserCard, 20) // 20 个真人
	for i := range cards {
		cards[i] = UserCard{UserID: int64(i + 1), IsRobot: false}
	}
	robots := []UserCard{
		{UserID: 101, IsRobot: true}, {UserID: 102, IsRobot: true},
		{UserID: 103, IsRobot: true}, {UserID: 104, IsRobot: true},
	}
	out := injectRobots(cards, robots, 10, 3, func(k int) int { return 0 }) // 每次插到最前

	cnt := 0
	for i := 0; i < 10; i++ {
		if out[i].IsRobot {
			cnt++
		}
	}
	if cnt != 3 {
		t.Fatalf("前10个机器人数=%d, want 3", cnt)
	}
	if len(out) != 23 {
		t.Fatalf("总长=%d, want 23", len(out))
	}
}

func TestInjectRobots_NoopWhenN0(t *testing.T) {
	cards := []UserCard{{UserID: 1}}
	out := injectRobots(cards, []UserCard{{UserID: 2, IsRobot: true}}, 10, 0, func(k int) int { return 0 })
	if len(out) != 1 {
		t.Fatalf("N=0 应不变, len=%d", len(out))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/match/ -run TestInjectRobots`
Expected: FAIL（`undefined: injectRobots` / `UserCard.IsRobot`）

- [ ] **Step 3: `UserCard` 加字段 + `toCards` 设值**

在 `UserCard` struct 末尾加：
```go
	IsRobot     bool      `json:"is_robot"`
```
在 `toCards` 的 `UserCard{...}` 字面量里加 `IsRobot: u.IsRobot,`：
```go
		out = append(out, UserCard{
			UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar,
			Gender: u.Gender, Age: u.Age, City: u.City, IsVerified: u.IsVerified,
			OnlineHint: hint, CreatedAt: u.CreatedAt, IsRobot: u.IsRobot,
		})
```

- [ ] **Step 4: 实现 `injectRobots`**（`service.go` 文件末尾）

```go
// injectRobots 在 cards 前 m 个位置随机插入机器人,使前 m 内机器人数尽量达到 n。
// pick(k) 返回 [0,k) 的随机位置(便于测试)。robots 为已去重的候选(标记 IsRobot=true)。
func injectRobots(cards, robots []UserCard, m, n int, pick func(int) int) []UserCard {
	if n <= 0 || len(robots) == 0 || m <= 0 {
		return cards
	}
	win := m
	if len(cards) < win {
		win = len(cards)
	}
	have := 0
	for i := 0; i < win; i++ {
		if cards[i].IsRobot {
			have++
		}
	}
	need := n - have
	if need > len(robots) {
		need = len(robots)
	}
	for k := 0; k < need; k++ {
		pos := pick(m)
		if pos > len(cards) {
			pos = len(cards)
		}
		r := robots[k]
		r.IsRobot = true
		cards = append(cards, UserCard{})
		copy(cards[pos+1:], cards[pos:])
		cards[pos] = r
	}
	return cards
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd server && go test ./internal/match/ -run TestInjectRobots`
Expected: PASS

- [ ] **Step 6: 加 `candidateRobots`（DB 查询）**（`service.go` 末尾）

```go
// candidateRobots 取本租户机器人卡片:同城优先,不足用其他城市补;跨城的城市显示为请求城市。
func (s *Service) candidateRobots(tenantID, selfID int64, city string, exclude map[int64]bool, blocked []int64, need int) []UserCard {
	fetch := func(sameCity bool) []model.User {
		q := s.db.Model(&model.User{}).
			Where("tenant_id = ? AND is_robot = 1 AND status = ? AND user_id <> ?", tenantID, "active", selfID)
		if len(blocked) > 0 {
			q = q.Where("user_id NOT IN ?", blocked)
		}
		if city != "" {
			if sameCity {
				q = q.Where("city = ?", city)
			} else {
				q = q.Where("city <> ?", city)
			}
		}
		var us []model.User
		q.Order("RAND()").Limit(need * 3).Find(&us)
		return us
	}
	picked := make([]model.User, 0, need)
	add := func(us []model.User, overrideCity bool) {
		for _, u := range us {
			if len(picked) >= need {
				return
			}
			if exclude[u.UserID] {
				continue
			}
			if overrideCity && city != "" {
				u.City = city // 跨城补进来的,显示为请求城市(营造同城人气)
			}
			exclude[u.UserID] = true
			picked = append(picked, u)
		}
	}
	add(fetch(true), false)
	if len(picked) < need {
		add(fetch(false), true)
	}
	cards := toCards(picked)
	for i := range cards {
		cards[i].IsRobot = true
	}
	s.fillCounts(cards)
	return cards
}
```

- [ ] **Step 7: `CityUsers` 接线**（在 `CityUsers` 的 `s.fillCounts(cards)` 之后、`return cards, nil` 之前插入）

```go
	// 同城机器人注入:前 M 个位置保 N 个机器人(随机位置)。
	if n := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopN); n > 0 {
		m := sysconfig.GetInt(tenantID, sysconfig.KeyCityRobotTopM)
		if m <= 0 {
			m = 10
		}
		win := m
		if len(cards) < win {
			win = len(cards)
		}
		have := 0
		for i := 0; i < win; i++ {
			if cards[i].IsRobot {
				have++
			}
		}
		if need := n - have; need > 0 {
			exclude := map[int64]bool{}
			for _, c := range cards {
				exclude[c.UserID] = true
			}
			robots := s.candidateRobots(tenantID, selfID, city, exclude, s.blockedIDs(selfID), need)
			cards = injectRobots(cards, robots, m, n, func(k int) int { return rand.Intn(k) })
		}
	}
```

- [ ] **Step 8: 加 imports**（`match/service.go` 顶部 import 块加 `"math/rand"` 与 `"driftbottle/internal/sysconfig"`）

```go
import (
	"math/rand"
	"strconv"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"

	"gorm.io/gorm"
)
```

- [ ] **Step 9: 构建 + 测试**

Run: `cd server && go build ./... && go test ./internal/match/`
Expected: build exit 0；PASS

- [ ] **Step 10: Commit**

```bash
git add server/internal/match/service.go server/internal/match/inject_test.go
git commit -m "feat(match): 同城列表前M个位置注入N个机器人(随机位置)"
```

---

## Task 4: #3a 提示词强化(多轮提问)

**Files:**
- Modify: `server/internal/robot/promptbuilder.go`（`buildPrompt` 返回串追加钩子）
- Modify: `server/internal/robot/service.go`（`genAIText` 的 reply 提示词加钩子）

**Interfaces:** 无新导出。

- [ ] **Step 1: `buildPrompt` 追加钩子**

在 `promptbuilder.go buildPrompt` 的 return 处,把最终 system prompt 末尾拼上一句(找到函数返回的字符串变量,追加)：
```go
	// 引导多轮:结尾自然带钩子
	sb.WriteString("\n回复自然地以一个问题或话题钩子结尾,引导对方继续聊,但不要生硬、不要每句都问。")
```
> 说明:`buildPrompt` 用 `strings.Builder`(变量 `sb`)拼装;若实际变量名不同,追加到返回字符串前即可。

- [ ] **Step 2: `genAIText` reply 提示词加钩子**（`service.go genAIText` 的 `switch contentType` 的 `default`(reply) 分支）

```go
	default:
		userPrompt = "请对以下漂流瓶内容写一条简短、温暖的回应，30字以内，" +
			"自然地带一个小问题引导对方继续聊：\n" + msgCtx
```

- [ ] **Step 3: 构建**

Run: `cd server && go build ./...`
Expected: exit 0

- [ ] **Step 4: Commit**

```bash
git add server/internal/robot/promptbuilder.go server/internal/robot/service.go
git commit -m "feat(robot): 提示词强化, 回复带提问钩子引导多轮"
```

---

## Task 5: #3 主动引导(破冰+沉默追问) 基础设施

**Files:**
- Create: `server/internal/robot/engage.go`
- Create: `server/internal/robot/engage_test.go`
- Modify: `server/internal/robot/service.go`（`Service` 加 `sender` 字段 + `SetSender`；`Tick` 挂子任务）
- Modify: `server/cmd/api/main.go`（`robotSvc.SetSender(chatSvc)`）

**Interfaces:**
- Consumes: `MessageSender`（`bot_worker.go` 已定义,含 `SendMessage/HistoryRecent/EnsureRobotChat`）；配置键(Task 1)。
- Produces: `func trailingRobotMsgs(msgs []model.Message, botUserID int64) int`、`func isOutreachCandidate(created, lastActive, now time.Time, newHours, silentDays int) bool`、`func (s *Service) SetSender(MessageSender)`、`func (s *Service) proactiveOutreach(tenantID int64, robots []model.User)`、`func (s *Service) nudgeStalledChats(tenantID int64, robots []model.User)`。

- [ ] **Step 1: 写失败测试** `server/internal/robot/engage_test.go`

```go
package robot

import (
	"testing"
	"time"

	"driftbottle/internal/model"
)

func TestTrailingRobotMsgs(t *testing.T) {
	bot := int64(9)
	msgs := []model.Message{ // 时间升序
		{SenderID: 1}, {SenderID: 9}, {SenderID: 1}, {SenderID: 9}, {SenderID: 9},
	}
	if got := trailingRobotMsgs(msgs, bot); got != 2 {
		t.Fatalf("trailing=%d, want 2", got)
	}
	// 末尾是真人 → 0
	msgs2 := []model.Message{{SenderID: 9}, {SenderID: 1}}
	if got := trailingRobotMsgs(msgs2, bot); got != 0 {
		t.Fatalf("trailing=%d, want 0", got)
	}
}

func TestIsOutreachCandidate(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	// 新用户:2小时前注册
	if !isOutreachCandidate(now.Add(-2*time.Hour), now, now, 24, 7) {
		t.Fatal("新用户应命中")
	}
	// 沉默用户:10天未活跃(注册很早)
	if !isOutreachCandidate(now.Add(-100*24*time.Hour), now.Add(-10*24*time.Hour), now, 24, 7) {
		t.Fatal("沉默用户应命中")
	}
	// 老用户且近期活跃 → 不命中
	if isOutreachCandidate(now.Add(-100*24*time.Hour), now.Add(-1*time.Hour), now, 24, 7) {
		t.Fatal("活跃老用户不应命中")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/robot/ -run 'TestTrailingRobotMsgs|TestIsOutreachCandidate'`
Expected: FAIL（undefined）

- [ ] **Step 3: 实现 `engage.go`**

```go
package robot

import (
	gocontext "context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

// trailingRobotMsgs 统计消息(升序)末尾连续由 botUserID 发送的条数。
func trailingRobotMsgs(msgs []model.Message, botUserID int64) int {
	cnt := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].SenderID == botUserID {
			cnt++
		} else {
			break
		}
	}
	return cnt
}

// isOutreachCandidate 判断真人是否为破冰目标:新注册 或 长期沉默。
func isOutreachCandidate(created, lastActive, now time.Time, newHours, silentDays int) bool {
	if now.Sub(created) <= time.Duration(newHours)*time.Hour {
		return true
	}
	if now.Sub(lastActive) >= time.Duration(silentDays)*24*time.Hour {
		return true
	}
	return false
}

var openers = []string{
	"嗨,刷到你,感觉你挺有意思的,想认识一下~",
	"你好呀,今天过得怎么样?",
	"突然想找人聊聊天,你最近在忙什么呀?",
	"看到你就想打个招呼,你平时喜欢做什么?",
}

var nudges = []string{
	"还在吗?突然想到还没听你说完呢~",
	"在忙吗?有空回我一句呀。",
	"是不是走开啦?我还想接着聊呢。",
}

func randLine(pool []string) string { return pool[rand.Intn(len(pool))] }

// proactiveOutreach 对新/沉默真人主动发起一次开场私聊(每租户每日上限)。
func (s *Service) proactiveOutreach(tenantID int64, robots []model.User) {
	if s.sender == nil {
		return
	}
	cap := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachDailyCap)
	if cap <= 0 {
		return
	}
	ctx := gocontext.Background()
	dayKey := fmt.Sprintf("robot_outreach:%d:%s", tenantID, time.Now().Format("20060102"))
	used, _ := cache.RDB.Get(ctx, dayKey).Int()
	if used >= cap {
		return
	}

	newHours := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachNewHours)
	silentDays := sysconfig.GetInt(tenantID, sysconfig.KeyRobotOutreachSilentDays)
	now := time.Now()
	newSince := now.Add(-time.Duration(newHours) * time.Hour)
	silentBefore := now.Add(-time.Duration(silentDays) * 24 * time.Hour)

	// 目标:真人、active、(新注册 或 沉默)、且无任何机器人会话。
	robotIDs := s.db.Model(&model.User{}).Select("user_id").Where("tenant_id = ? AND is_robot = 1", tenantID)
	hasBotChat := s.db.Model(&model.Chat{}).Select("user_a").
		Where("tenant_id = ? AND user_a IN (?)", tenantID, robotIDs)
	hasBotChatB := s.db.Model(&model.Chat{}).Select("user_b").
		Where("tenant_id = ? AND user_b IN (?)", tenantID, robotIDs)

	batch := cap - used
	if batch > 10 {
		batch = 10 // 每次 tick 最多 10 个,平滑投放
	}
	var targets []model.User
	s.db.Where("tenant_id = ? AND is_robot = 0 AND status = ?", tenantID, "active").
		Where("(created_at >= ? OR last_active_at <= ?)", newSince, silentBefore).
		Where("user_id NOT IN (?)", hasBotChat).
		Where("user_id NOT IN (?)", hasBotChatB).
		Order("created_at desc").Limit(batch).Find(&targets)

	sent := 0
	for _, u := range targets {
		r := robots[rand.Intn(len(robots))]
		chatID, err := s.sender.EnsureRobotChat(tenantID, r.UserID, u.UserID)
		if err != nil {
			continue
		}
		if _, err := s.sender.SendMessage(tenantID, r.UserID, chatID, randLine(openers), "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		cache.RDB.IncrBy(ctx, dayKey, int64(sent))
		cache.RDB.Expire(ctx, dayKey, 48*time.Hour)
		log.Printf("[robot] tenant=%d 主动破冰 %d 人", tenantID, sent)
	}
}

// nudgeStalledChats 对"机器人已发言、用户沉默"的会话追问一句(每会话限次)。
func (s *Service) nudgeStalledChats(tenantID int64, robots []model.User) {
	if s.sender == nil {
		return
	}
	silentMin := sysconfig.GetInt(tenantID, sysconfig.KeyRobotNudgeSilentMin)
	maxNudge := sysconfig.GetInt(tenantID, sysconfig.KeyRobotNudgeMax)
	if silentMin <= 0 || maxNudge <= 0 {
		return
	}
	now := time.Now()
	upper := now.Add(-time.Duration(silentMin) * time.Minute)
	lower := upper.Add(-60 * time.Minute) // 只抓刚沉默的一小时窗口,避免反复扫历史

	robotIDs := s.db.Model(&model.User{}).Select("user_id").Where("tenant_id = ? AND is_robot = 1", tenantID)
	var chats []model.Chat
	s.db.Where("tenant_id = ? AND updated_at > ? AND updated_at <= ?", tenantID, lower, upper).
		Where("user_a IN (?) OR user_b IN (?)", robotIDs, robotIDs).
		Limit(20).Find(&chats)

	sent := 0
	for _, ch := range chats {
		// 找出该会话里的机器人一方
		botID := ch.UserA
		var cntA int64
		s.db.Model(&model.User{}).Where("user_id = ? AND is_robot = 1", ch.UserA).Count(&cntA)
		if cntA == 0 {
			botID = ch.UserB
		}
		msgs, err := s.sender.HistoryRecent(ch.ChatID, 6)
		if err != nil {
			continue
		}
		tr := trailingRobotMsgs(msgs, botID)
		if tr < 1 || tr > maxNudge {
			continue // 0=用户最后发言(等正常回复);>max=已追够
		}
		if _, err := s.sender.SendMessage(tenantID, botID, ch.ChatID, randLine(nudges), "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Printf("[robot] tenant=%d 沉默追问 %d 条", tenantID, sent)
	}
}
```

- [ ] **Step 4: `Service` 加 `sender` 字段 + `SetSender`**（`service.go`）

在 `type Service struct { ... }` 末尾加字段：
```go
	sender MessageSender
```
在 `New(...)` 附近加方法：
```go
// SetSender 注入 chat.Service(通过 MessageSender),供主动引导发消息。
func (s *Service) SetSender(sender MessageSender) { s.sender = sender }
```

- [ ] **Step 5: `Tick` 挂子任务**（`service.go Tick` 末尾 `s.maybeReply(...)` 之后）

```go
	// 主动引导(总开关默认关)
	if sysconfig.GetBool(tenantID, sysconfig.KeyRobotProactive) {
		s.proactiveOutreach(tenantID, robots)
		s.nudgeStalledChats(tenantID, robots)
	}
```

- [ ] **Step 6: main.go 注入 sender**（`server/cmd/api/main.go`，在 `robot.InitWorkerPool(...)` 之后；`robotSvc` 与 `chatSvc` 为已存在变量,名称以实际为准）

```go
	robotSvc.SetSender(chatSvc)
```
> 若 `robot.InitWorkerPool` 的 sender 实参不是 `chatSvc` 变量名,用同一个传入 `InitWorkerPool` 的 sender 实参。

- [ ] **Step 7: 跑单测确认通过**

Run: `cd server && go test ./internal/robot/ -run 'TestTrailingRobotMsgs|TestIsOutreachCandidate'`
Expected: PASS

- [ ] **Step 8: 构建 + 全测**

Run: `cd server && go build ./... && go test ./...`
Expected: build exit 0；全绿

- [ ] **Step 9: Commit**

```bash
git add server/internal/robot/engage.go server/internal/robot/engage_test.go server/internal/robot/service.go server/cmd/api/main.go
git commit -m "feat(robot): 主动破冰新/沉默用户 + 会话沉默追问(总开关默认关)"
```

---

## Task 6: #3b 回信后引导加聊

**Files:**
- Modify: `server/internal/robot/service.go`（`maybeReply` 内,机器人回真人瓶后按概率引导加聊）

**Interfaces:** Consumes `s.sender`（Task 5）、`KeyRobotProactive`/`KeyRobotBottle2ChatRatio`。

- [ ] **Step 1: 在 `maybeReply` 成功创建 reply 之后加引导加聊**（`service.go maybeReply`,`cnt++` 之后）

```go
		// 回信后引导加聊(总开关 + 概率)
		if s.sender != nil && sysconfig.GetBool(tenantID, sysconfig.KeyRobotProactive) {
			if ratio := sysconfig.GetInt(tenantID, sysconfig.KeyRobotBottle2ChatRatio); ratio > 0 && rand.Intn(100) < ratio {
				if chatID, err := s.sender.EnsureRobotChat(tenantID, r.UserID, b.UserID); err == nil {
					opener := "刚看到你的瓶子,挺有共鸣的,想和你多聊两句~"
					s.sender.SendMessage(tenantID, r.UserID, chatID, opener, "text")
				}
			}
		}
```
> 注:`r`(机器人)、`b`(真人瓶)在 `maybeReply` 循环内已存在。

- [ ] **Step 2: 构建 + 测试**

Run: `cd server && go build ./... && go test ./internal/robot/`
Expected: build exit 0；PASS

- [ ] **Step 3: Commit**

```bash
git add server/internal/robot/service.go
git commit -m "feat(robot): 回信后按概率引导加聊(总开关+ratio)"
```

---

## Task 7: 存量清理 + 部署 + 验证

**Files:** 无代码改动(ops)。

- [ ] **Step 1: 交叉编译 + 全量校验**

```bash
cd server && go build ./... && go vet ./... && go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api
```
Expected: 全部 exit 0；生成 `driftbottle-linux`

- [ ] **Step 2: 部署后端**

```bash
node deploy.js
```
Expected: `active` + nginx 重载成功

- [ ] **Step 3: 一次性软删机器人旧瓶**（用 scratchpad 的 dbq.js，NODE_PATH 指仓库 node_modules）

```bash
NODE_PATH=J:/code/net_workspace/ai-message/node_modules node <scratchpad>/dbq.js "UPDATE bottles SET status='expired' WHERE user_id IN (SELECT user_id FROM (SELECT user_id FROM users WHERE is_robot=1) x); SELECT status, COUNT(*) FROM bottles GROUP BY status;"
```
Expected: 机器人瓶子转为 expired；active 数骤降

- [ ] **Step 4: 验证 #1/#2**

```bash
# #1: 触发若干 tick 后, 造瓶去重上升(直接看内容池增长)
NODE_PATH=... node dbq.js "SELECT COUNT(*) pool FROM robot_contents WHERE type='bottle'; SELECT COUNT(*) total, COUNT(DISTINCT content) distinct_content FROM bottles WHERE status='active';"
# #2: 同城前M含N(需真人 token 调 /api/city/users;或后台间接验证配置生效)
```
Expected: `robot_contents(bottle)` 随投放增长;新 active 瓶去重比显著提高

- [ ] **Step 5: 灰度开主动引导**（后台「系统配置」→ 机器人 → 打开「主动引导总开关」,先设小 `daily_cap`/`ratio`,观察日志)

- [ ] **Step 6: 推送**

```bash
git push origin main
```

---

## Self-Review

- **Spec 覆盖**:#1(Task2 造瓶+去重+扩池, Task7 清存量) / #2(Task3 注入) / #3a(Task4) / #3b(Task6) / #3c#3d(Task5) / 配置(Task1) / 测试与灰度(Task5/7) — 全覆盖。
- **占位符**:无 TBD/TODO;每步含实际代码或命令。
- **类型一致**:`injectRobots`、`trailingRobotMsgs`、`isOutreachCandidate`、`genBottleText`、`candidateRobots`、`SetSender` 签名跨任务一致;`UserCard.IsRobot` 在 Task3 定义后于同任务使用。
- **已知风险**:main.go 变量名(`robotSvc`/`chatSvc`)以实际为准;`buildPrompt` 内部变量名以实际为准(Task4 已注明兜底做法)。
