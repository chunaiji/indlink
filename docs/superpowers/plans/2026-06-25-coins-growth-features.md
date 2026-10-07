# 金币增长功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在漂流瓶后端实现 5 项金币增长能力：每日签到、消息扣费、分享奖励、登录/活跃时间记录、机器人按概率主动触达活跃用户。

**Architecture:** 沿用模块化单体的包边界。配置统一进 `sysconfig` + `admin/meta.go` 白名单（前端自动渲染）。资金走 `wallet.Credit/Debit/CreditTx`（流水落 `WalletTxn`）。每日计数复用 `cache.RDB` 按天 key。定时触达仿 `push.StartCheckinCron` 的 goroutine 模式，机器人开场白复用 `robot` 包的 `buildPrompt` + `LLMClient.Chat`。

**Tech Stack:** Go 1.22、Gin、GORM(MySQL)、go-redis(`pkg/cache`)、雪花 ID(`pkg/idgen`)。

## Global Constraints

- 所有可配置开关/数值必须进 `sysconfig` 常量 + `defaults`，并加入 `admin/meta.go` 的 `configMeta` 白名单（否则后台无法保存）。新组统一 `Group: "增长运营"`。
- `WalletTxn.Scene` 字段 `size:24`，场景串不得超过 24 字符。
- 资金发放/扣减必须走 `wallet` 包并产生 `WalletTxn` 流水（可对账），禁止直接改 `Wallet.Balance`。
- 机器人账号（`is_robot=true`）发消息一律不扣费、不刷活跃、不触发微信订阅推送。
- 新增 model 必须加入 `model.AllModels()` 以参与 AutoMigrate。
- 每个任务结束执行 `cd server && go build ./... && go vet ./...` 必须通过。
- 提交信息结尾加：`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

### Task 1: 配置项地基（sysconfig 键 + admin 白名单）

集中加入全部 5 个功能用到的配置键，后续任务直接引用，避免散落。

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`

**Interfaces:**
- Produces: 常量 `KeyCheckinEnabled`、`KeyCheckinCoins`、`KeyPriceMsg`、`KeyShareRewardEnabled`、`KeyShareRewardCoins`、`KeyShareRewardDailyLimit`、`KeyOutreachEnabled`、`KeyOutreachWindowStart`、`KeyOutreachWindowEnd`、`KeyOutreachProbability`、`KeyOutreachLookbackMin`、`KeyOutreachTickMin`、`KeyOutreachUserDailyCap`，均可经 `sysconfig.GetBool/GetInt/GetInt64` 读取。

- [ ] **Step 1: 在 sysconfig.go 常量区追加键定义**

在 `const (...)` 块末尾（`KeyUITextNavTitle` 之后）追加：

```go
	// 每日签到(#增长1)
	KeyCheckinEnabled = "checkin_enabled" // 签到总开关 (0/1)
	KeyCheckinCoins   = "checkin_coins"   // 每日签到发放金币 N

	// 消息扣费(#增长2)
	KeyPriceMsg = "price_msg" // 每条消息消耗金币 A；0=关闭

	// 分享奖励(#增长3)
	KeyShareRewardEnabled    = "share_reward_enabled"     // 分享奖励总开关 (0/1)
	KeyShareRewardCoins      = "share_reward_coins"       // 单次分享奖励 M
	KeyShareRewardDailyLimit = "share_reward_daily_limit" // 每日最多领奖次数

	// 机器人主动触达(#增长5)
	KeyOutreachEnabled      = "outreach_enabled"        // 主动触达总开关 (0/1)
	KeyOutreachWindowStart  = "outreach_window_start"   // 触达时段开始小时 0-23(含)
	KeyOutreachWindowEnd    = "outreach_window_end"     // 触达时段结束小时 0-23(不含)
	KeyOutreachProbability  = "outreach_probability"    // 活跃用户抽样比例 N(%)
	KeyOutreachLookbackMin  = "outreach_lookback_min"   // 回溯活跃窗口 M(分钟)
	KeyOutreachTickMin      = "outreach_tick_min"       // 窗口内调度间隔(分钟)
	KeyOutreachUserDailyCap = "outreach_user_daily_cap" // 同一用户每日被触达上限
```

- [ ] **Step 2: 在 defaults map 追加默认值**

在 `var defaults = map[string]string{...}` 末尾（`KeyUITextNavTitle` 行之后）追加：

```go
	KeyCheckinEnabled: "0",
	KeyCheckinCoins:   "10",

	KeyPriceMsg: "0",

	KeyShareRewardEnabled:    "0",
	KeyShareRewardCoins:      "5",
	KeyShareRewardDailyLimit: "3",

	KeyOutreachEnabled:      "0",
	KeyOutreachWindowStart:  "19",
	KeyOutreachWindowEnd:    "22",
	KeyOutreachProbability:  "10",
	KeyOutreachLookbackMin:  "30",
	KeyOutreachTickMin:      "30",
	KeyOutreachUserDailyCap: "1",
```

- [ ] **Step 3: 在 admin/meta.go configMeta 追加白名单项**

在 `configMeta` 切片末尾（最后一项 `KeyReplyMaskLen` 之后）追加：

```go
	// 增长运营
	{Key: sysconfig.KeyCheckinEnabled, Label: "每日签到总开关", Group: "增长运营", Type: "bool"},
	{Key: sysconfig.KeyCheckinCoins, Label: "签到发放金币 N", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyPriceMsg, Label: "每条消息消耗金币(0=关闭)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyShareRewardEnabled, Label: "分享奖励总开关", Group: "增长运营", Type: "bool"},
	{Key: sysconfig.KeyShareRewardCoins, Label: "单次分享奖励 M", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyShareRewardDailyLimit, Label: "每日分享领奖次数上限", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachEnabled, Label: "机器人主动触达总开关", Group: "增长运营", Type: "bool"},
	{Key: sysconfig.KeyOutreachWindowStart, Label: "触达时段开始(小时0-23)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachWindowEnd, Label: "触达时段结束(小时0-23)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachProbability, Label: "活跃用户抽样比例(%)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachLookbackMin, Label: "回溯活跃窗口(分钟)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachTickMin, Label: "触达调度间隔(分钟)", Group: "增长运营", Type: "int"},
	{Key: sysconfig.KeyOutreachUserDailyCap, Label: "同用户每日触达上限", Group: "增长运营", Type: "int"},
```

- [ ] **Step 4: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 无报错。

- [ ] **Step 5: Commit**

```bash
cd server && git add internal/sysconfig/sysconfig.go internal/admin/meta.go
git commit -m "feat(config): 新增增长运营配置键(签到/消息扣费/分享/触达)"
```

---

### Task 2: 功能4 — 记录登录时间 + 节流刷新活跃时间

**Files:**
- Modify: `server/internal/model/model.go` (User 结构体)
- Modify: `server/internal/common/middleware/middleware.go` (Auth 钩子)
- Modify: `server/internal/user/service.go` (Login 写登录时间 + TouchActive)
- Modify: `server/cmd/api/main.go` (注册钩子)

**Interfaces:**
- Consumes: `cache.RDB`（go-redis 客户端，见 `pkg/cache`）。
- Produces: `(*user.Service).TouchActive(tenantID, userID int64)`；`middleware.OnAuthenticated func(tenantID, userID int64)` 包级变量。

- [ ] **Step 1: User 增加 LastLoginAt 字段**

在 `model.go` 的 `User` 结构体里，`LastActiveAt` 行下方追加：

```go
	LastLoginAt    time.Time `json:"last_login_at"`
```

- [ ] **Step 2: Login 写入登录时间**

在 `user/service.go` 的 `Login` 中，老用户分支把：

```go
		s.db.Model(&u).Update("last_active_at", time.Now())
```

改为：

```go
		now := time.Now()
		s.db.Model(&u).Updates(map[string]interface{}{"last_active_at": now, "last_login_at": now})
```

并在新建用户结构体里追加 `LastLoginAt: time.Now(),`（紧跟 `LastActiveAt: time.Now(),`）。

- [ ] **Step 3: 新增 TouchActive(节流刷新活跃时间)**

在 `user/service.go` 末尾追加（注意顶部 import 需要加入 `"context"` 和 `"driftbottle/pkg/cache"`）：

```go
// TouchActive 节流刷新最近活跃时间：每个用户最多每 60s 写一次 last_active_at(#增长4/5)。
func (s *Service) TouchActive(tenantID, userID int64) {
	if userID == 0 {
		return
	}
	ctx := context.Background()
	key := fmt.Sprintf("active:%d", userID)
	// SetNX 成功(此前不存在)才落库，TTL 60s 作为去抖窗口
	ok, err := cache.RDB.SetNX(ctx, key, "1", 60*time.Second).Result()
	if err != nil || !ok {
		return
	}
	s.db.Model(&model.User{}).Where("user_id = ?", userID).Update("last_active_at", time.Now())
}
```

- [ ] **Step 4: middleware 暴露认证后钩子**

在 `middleware.go` 顶部常量区下方追加包级变量：

```go
// OnAuthenticated 鉴权成功后回调(可选)，由 main 注入；用于刷新用户活跃时间。
var OnAuthenticated func(tenantID, userID int64)
```

在 `Auth()` 的 `c.Set(CtxPlatform, claims.Platform)` 之后、`c.Next()` 之前追加：

```go
		if OnAuthenticated != nil {
			go OnAuthenticated(claims.TenantID, claims.UserID)
		}
```

- [ ] **Step 5: main.go 注入钩子 + WS 连接刷新活跃**

在 `main.go` 装配 `userSvc` 之后（`auth := middleware.Auth()` 附近，任意在路由注册前）追加：

```go
	middleware.OnAuthenticated = func(tenantID, userID int64) { userSvc.TouchActive(tenantID, userID) }
```

- [ ] **Step 6: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 无报错。

- [ ] **Step 7: Commit**

```bash
cd server && git add internal/model/model.go internal/common/middleware/middleware.go internal/user/service.go cmd/api/main.go
git commit -m "feat(user): 记录登录时间 + Redis 去抖刷新最近活跃时间(#增长4)"
```

---

### Task 3: 功能1 — 每日签到领 N 金币

**Files:**
- Modify: `server/internal/model/model.go` (新增 CheckinLog + AllModels)
- Modify: `server/internal/wallet/service.go` (新增 SceneCheckin)
- Create: `server/internal/checkin/service.go`
- Create: `server/internal/checkin/handler.go`
- Modify: `server/cmd/api/main.go` (注册路由)

**Interfaces:**
- Consumes: `wallet.CreditTx(tx *gorm.DB, tenantID, userID, coins int64, scene, bizNo string) error`；`sysconfig.GetBool/GetInt64`。
- Produces: `checkin.New(db *gorm.DB) *Service`；`(*Service).Sign(tenantID, userID int64) (coins int64, balance int64, already bool, err error)`；`(*Service).Status(userID int64) (enabled bool, coins int64, signedToday bool)`；`checkin.NewHandler(svc).Register(api, auth)` 暴露 `POST /api/checkin`、`GET /api/checkin/status`。

- [ ] **Step 1: 新增 CheckinLog model + 注册 AllModels**

在 `model.go` 的 `Collection` 结构体后追加：

```go
// CheckinLog 每日签到记录(唯一索引保证一人一天一次，资金可对账)。
type CheckinLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  int64     `gorm:"index" json:"tenant_id,string"`
	UserID    int64     `gorm:"uniqueIndex:uk_user_date" json:"user_id,string"`
	Date      string    `gorm:"size:8;uniqueIndex:uk_user_date" json:"date"` // 20060102
	Coins     int64     `json:"coins"`
	CreatedAt time.Time `json:"created_at"`
}
```

在 `AllModels()` 返回切片里 `&Collection{},` 之后追加 `&CheckinLog{},`。

- [ ] **Step 2: wallet 新增签到场景常量**

在 `wallet/service.go` 常量区追加：

```go
	SceneCheckin = "checkin"
```

- [ ] **Step 3: 写签到服务的失败测试**

Create `server/internal/checkin/bizno_test.go`：

```go
package checkin

import (
	"strings"
	"testing"
)

func TestBizNo(t *testing.T) {
	got := bizNo(123, "20260625")
	if !strings.HasPrefix(got, "checkin:") {
		t.Fatalf("bizNo 前缀异常: %s", got)
	}
	if got != "checkin:123:20260625" {
		t.Fatalf("bizNo 格式异常: %s", got)
	}
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd server && go test ./internal/checkin/ -run TestBizNo -v`
Expected: 编译失败（包/函数未定义）。

- [ ] **Step 5: 实现 checkin/service.go**

Create `server/internal/checkin/service.go`：

```go
// Package checkin 每日签到领金币(#增长1)。
package checkin

import (
	"errors"
	"fmt"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"

	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

func today() string          { return time.Now().Format("20060102") }
func bizNo(userID int64, date string) string {
	return fmt.Sprintf("checkin:%d:%s", userID, date)
}

// Status 返回开关、发放数、今日是否已签。
func (s *Service) Status(userID int64) (enabled bool, coins int64, signedToday bool) {
	enabled = sysconfig.GetBool(sysconfig.KeyCheckinEnabled)
	coins = sysconfig.GetInt64(sysconfig.KeyCheckinCoins)
	var n int64
	s.db.Model(&model.CheckinLog{}).Where("user_id = ? AND date = ?", userID, today()).Count(&n)
	signedToday = n > 0
	return
}

// Sign 执行签到：插入唯一签到记录 + 入账金币(同事务)。重复签到返回 already=true 不重复发币。
func (s *Service) Sign(tenantID, userID int64) (coins int64, balance int64, already bool, err error) {
	if !sysconfig.GetBool(sysconfig.KeyCheckinEnabled) {
		return 0, 0, false, errs.New(errs.CodeBadRequest, "签到未开启")
	}
	date := today()
	reward := sysconfig.GetInt64(sysconfig.KeyCheckinCoins)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		rec := model.CheckinLog{TenantID: tenantID, UserID: userID, Date: date, Coins: reward, CreatedAt: time.Now()}
		if e := tx.Create(&rec).Error; e != nil {
			// 唯一键冲突 = 今日已签
			already = true
			return nil
		}
		if reward > 0 {
			if e := wallet.CreditTx(tx, tenantID, userID, reward, wallet.SceneCheckin, bizNo(userID, date)); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, false, err
	}

	var w model.Wallet
	s.db.Select("balance").First(&w, "user_id = ?", userID)
	if already {
		return 0, w.Balance, true, nil
	}
	return reward, w.Balance, false, nil
}

var _ = errors.Is // 占位避免未使用(若后续不需要可删)
```

> 注：`errors` 仅在需要更精细判断唯一键错误时使用。当前以「Create 失败即视为已签」简化；若需区分真实 DB 错误，可改为先 `SELECT` 再 `INSERT`。最后一行 `var _ = errors.Is` 仅为通过未使用 import 检查，若删掉 `errors` import 则一并删除此行。

- [ ] **Step 6: 实现 checkin/handler.go**

Create `server/internal/checkin/handler.go`：

```go
package checkin

import (
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/checkin", auth)
	g.GET("/status", h.status)
	g.POST("", h.sign)
}

func (h *Handler) status(c *gin.Context) {
	enabled, coins, signed := h.svc.Status(middleware.UserID(c))
	response.OK(c, gin.H{"enabled": enabled, "coins": coins, "signed_today": signed})
}

func (h *Handler) sign(c *gin.Context) {
	coins, balance, already, err := h.svc.Sign(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "签到失败")
		return
	}
	response.OK(c, gin.H{"coins": coins, "balance": balance, "already_signed": already})
}
```

- [ ] **Step 7: 运行 bizNo 测试确认通过**

Run: `cd server && go test ./internal/checkin/ -run TestBizNo -v`
Expected: PASS。

- [ ] **Step 8: main.go 注册签到路由**

在 `main.go` 路由区（`push.NewHandler(...).Register` 附近）追加：

```go
	checkin.NewHandler(checkin.New(db)).Register(api, auth)
```

并在 import 区加入 `"driftbottle/internal/checkin"`。

- [ ] **Step 9: 构建验证**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/checkin/...`
Expected: 全部通过。

- [ ] **Step 10: Commit**

```bash
cd server && git add internal/model/model.go internal/wallet/service.go internal/checkin/ cmd/api/main.go
git commit -m "feat(checkin): 每日签到领金币(#增长1)"
```

---

### Task 4: 功能2 — 聊天每条消息扣 A 金币

**Files:**
- Modify: `server/internal/wallet/service.go` (新增 SceneMsg)
- Modify: `server/internal/chat/service.go` (SendMessage 扣费)

**Interfaces:**
- Consumes: `wallet.Service.Debit(tenantID, userID, coins int64, scene, bizNo string, biz func(tx *gorm.DB) error) error`；`sysconfig.GetInt64(KeyPriceMsg)`；`errs.ErrInsufficient`。
- Produces: `SendMessage` 行为变化：真人 sender 且 `price_msg>0` 时每条消息扣费，余额不足返回 `errs.ErrInsufficient`。

- [ ] **Step 1: wallet 新增消息场景常量**

在 `wallet/service.go` 常量区追加：

```go
	SceneMsg = "msg"
```

- [ ] **Step 2: 改造 SendMessage 扣费逻辑**

在 `chat/service.go` 的 `SendMessage` 中，把当前「建消息 + 更新会话」的事务块：

```go
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chat{}).Where("chat_id = ?", chatID).
			Updates(map[string]interface{}{"last_message": truncate(content, 100), "updated_at": time.Now()}).Error
	})
	if err != nil {
		return nil, err
	}
```

替换为（抽出公共写库闭包 `persist`，按是否扣费选择路径）：

```go
	persist := func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chat{}).Where("chat_id = ?", chatID).
			Updates(map[string]interface{}{"last_message": truncate(content, 100), "updated_at": time.Now()}).Error
	}

	price := sysconfig.GetInt64(sysconfig.KeyPriceMsg)
	var err error
	if price > 0 && !sender.IsRobot {
		// 真人发消息：扣费与落库同事务，余额不足透传 ErrInsufficient
		err = s.wlt.Debit(tenantID, senderID, price, wallet.SceneMsg, "msg:"+strconv.FormatInt(msg.MessageID, 10), persist)
	} else {
		err = s.db.Transaction(persist)
	}
	if err != nil {
		return nil, err
	}
```

> 说明：`sender` 变量在前文已通过 `s.db.Select("is_muted,is_robot").First(&sender, senderID)` 取得；需要确保该查询同时取到 `is_robot`（当前 Select 已含）。`strconv` 已在文件 import 中。

- [ ] **Step 3: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 无报错。

- [ ] **Step 4: 手工核对扣费路径(代码审查)**

确认以下三点（无自动化测试，DB 绑定逻辑靠审查 + 构建）：
1. `sender.IsRobot` 为真时不进入扣费分支。
2. `price<=0` 时走原免费事务，行为与改造前一致。
3. WS 推送 / `onMsgPush` / `botEnqueue` 仍在 `if err != nil { return }` 之后执行。

- [ ] **Step 5: Commit**

```bash
cd server && git add internal/wallet/service.go internal/chat/service.go
git commit -m "feat(chat): 每条消息扣金币(机器人免扣)(#增长2)"
```

---

### Task 5: 功能3 — 分享领 M 金币(每日次数上限)

**Files:**
- Modify: `server/internal/wallet/service.go` (新增 SceneShare)
- Create: `server/internal/share/service.go`
- Create: `server/internal/share/handler.go`
- Modify: `server/cmd/api/main.go` (注册路由)

**Interfaces:**
- Consumes: `cache.RDB`；`wallet.Service.Credit(tenantID, userID, coins int64, scene, bizNo string) error`；`sysconfig.GetBool/GetInt/GetInt64`。
- Produces: `share.New(db *gorm.DB, w *wallet.Service) *Service`；`(*Service).Reward(tenantID, userID int64) (rewarded bool, coins int64, balance int64, countToday int, limit int, err error)`；`share.NewHandler(svc).Register(api, auth)` 暴露 `POST /api/share/reward`。

- [ ] **Step 1: wallet 新增分享场景常量**

在 `wallet/service.go` 常量区追加：

```go
	SceneShare = "share"
```

- [ ] **Step 2: 写每日计数 key 的失败测试**

Create `server/internal/share/key_test.go`：

```go
package share

import "testing"

func TestDayKey(t *testing.T) {
	k := dayKey(1, 99, "20260625")
	want := "share:1:99:20260625"
	if k != want {
		t.Fatalf("dayKey=%s want=%s", k, want)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd server && go test ./internal/share/ -run TestDayKey -v`
Expected: 编译失败（包/函数未定义）。

- [ ] **Step 4: 实现 share/service.go**

Create `server/internal/share/service.go`：

```go
// Package share 分享领金币(#增长3)：信任客户端触发 + Redis 按天上限兜量。
package share

import (
	"context"
	"fmt"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/wallet"
	"driftbottle/pkg/cache"

	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	wlt *wallet.Service
}

func New(db *gorm.DB, w *wallet.Service) *Service { return &Service{db: db, wlt: w} }

func today() string                            { return time.Now().Format("20060102") }
func dayKey(tenantID, userID int64, d string) string {
	return fmt.Sprintf("share:%d:%d:%s", tenantID, userID, d)
}

// Reward 占用一次每日分享额度并发放金币。达上限返回 rewarded=false(不报错)。
func (s *Service) Reward(tenantID, userID int64) (rewarded bool, coins int64, balance int64, countToday int, limit int, err error) {
	limit = sysconfig.GetInt(sysconfig.KeyShareRewardDailyLimit)
	coins = sysconfig.GetInt64(sysconfig.KeyShareRewardCoins)

	if !sysconfig.GetBool(sysconfig.KeyShareRewardEnabled) || limit <= 0 || coins <= 0 {
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, 0, limit, nil
	}

	ctx := context.Background()
	key := dayKey(tenantID, userID, today())
	n, e := cache.RDB.Incr(ctx, key).Result()
	if e != nil {
		return false, coins, 0, 0, limit, e
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	countToday = int(n)
	if int(n) > limit {
		// 超额回滚计数，不发币
		cache.RDB.Decr(ctx, key)
		countToday = limit
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, countToday, limit, nil
	}

	bizNo := fmt.Sprintf("share:%d:%s:%d", userID, today(), n)
	if e := s.wlt.Credit(tenantID, userID, coins, wallet.SceneShare, bizNo); e != nil {
		cache.RDB.Decr(ctx, key) // 发币失败回滚计数
		return false, coins, 0, countToday - 1, limit, e
	}
	var w model.Wallet
	s.db.Select("balance").First(&w, "user_id = ?", userID)
	return true, coins, w.Balance, countToday, limit, nil
}
```

- [ ] **Step 5: 实现 share/handler.go**

Create `server/internal/share/handler.go`：

```go
package share

import (
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.Group("/share", auth).POST("/reward", h.reward)
}

func (h *Handler) reward(c *gin.Context) {
	rewarded, coins, balance, count, limit, err := h.svc.Reward(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "领取失败")
		return
	}
	response.OK(c, gin.H{
		"rewarded": rewarded, "coins": coins, "balance": balance,
		"count_today": count, "limit": limit,
	})
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd server && go test ./internal/share/ -run TestDayKey -v`
Expected: PASS。

- [ ] **Step 7: main.go 注册分享路由**

在 `main.go` 路由区追加：

```go
	share.NewHandler(share.New(db, walletSvc)).Register(api, auth)
```

import 区加入 `"driftbottle/internal/share"`。

- [ ] **Step 8: 构建验证**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/share/...`
Expected: 全部通过。

- [ ] **Step 9: Commit**

```bash
cd server && git add internal/wallet/service.go internal/share/ cmd/api/main.go
git commit -m "feat(share): 分享领金币 + 每日次数上限(#增长3)"
```

---

### Task 6: 功能5 — 机器人按概率主动触达活跃用户

**Files:**
- Modify: `server/internal/robot/bot_worker.go` (MessageSender 接口扩展)
- Modify: `server/internal/chat/service.go` (实现 EnsureRobotChat)
- Create: `server/internal/robot/outreach.go`
- Create: `server/internal/robot/outreach_test.go`
- Modify: `server/cmd/api/main.go` (启动 cron)

**Interfaces:**
- Consumes: `BotRegistry.Get(botUserID)`、`LLMClient.Chat(ctx, system string, history []llmMessage, user string)`、`buildPrompt(persona, memory)`（同包 robot）、`cache.RDB`、`sysconfig.*`。
- Produces: `MessageSender` 接口新增方法 `EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error)`；纯函数 `inWindow(hour, start, end int) bool`、`sampleCount(total, pct int) int`；`StartOutreachCron(rsvc *Service, sender MessageSender, defaultTenant int64)`。

- [ ] **Step 1: MessageSender 接口新增 EnsureRobotChat**

在 `bot_worker.go` 的 `MessageSender` 接口里追加方法：

```go
type MessageSender interface {
	SendMessage(tenantID, senderID, chatID int64, content, msgType string) (*model.Message, error)
	HistoryRecent(chatID int64, limit int) ([]model.Message, error)
	EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error)
}
```

- [ ] **Step 2: chat.Service 实现 EnsureRobotChat**

在 `chat/service.go` 末尾追加（`order` 函数前任意位置）：

```go
// EnsureRobotChat 找到或创建 机器人↔用户 会话(不扣费，供机器人主动触达#增长5)。
func (s *Service) EnsureRobotChat(tenantID, botUserID, userID int64) (int64, error) {
	if existing, err := s.findChat(tenantID, botUserID, userID); err == nil && existing != nil {
		return existing.ChatID, nil
	}
	a, b := order(botUserID, userID)
	chat := &model.Chat{
		ChatID: idgen.Next(), TenantID: tenantID, UserA: a, UserB: b,
		RelationStage: "stranger", UpdatedAt: time.Now(), CreatedAt: time.Now(),
	}
	if err := s.db.Create(chat).Error; err != nil {
		return 0, err
	}
	return chat.ChatID, nil
}
```

- [ ] **Step 3: 写纯函数失败测试**

Create `server/internal/robot/outreach_test.go`：

```go
package robot

import "testing"

func TestInWindow(t *testing.T) {
	cases := []struct {
		hour, start, end int
		want             bool
	}{
		{20, 19, 22, true},
		{19, 19, 22, true},  // 含开始
		{22, 19, 22, false}, // 不含结束
		{18, 19, 22, false},
		{10, 19, 22, false},
		{5, 0, 0, false}, // start>=end 视为关闭
	}
	for _, c := range cases {
		if got := inWindow(c.hour, c.start, c.end); got != c.want {
			t.Errorf("inWindow(%d,%d,%d)=%v want %v", c.hour, c.start, c.end, got, c.want)
		}
	}
}

func TestSampleCount(t *testing.T) {
	if n := sampleCount(100, 10); n != 10 {
		t.Errorf("sampleCount(100,10)=%d want 10", n)
	}
	if n := sampleCount(5, 10); n != 0 { // 向下取整
		t.Errorf("sampleCount(5,10)=%d want 0", n)
	}
	if n := sampleCount(0, 50); n != 0 {
		t.Errorf("sampleCount(0,50)=%d want 0", n)
	}
	if n := sampleCount(100, 0); n != 0 {
		t.Errorf("sampleCount(100,0)=%d want 0", n)
	}
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd server && go test ./internal/robot/ -run 'TestInWindow|TestSampleCount' -v`
Expected: 编译失败（`inWindow` / `sampleCount` 未定义）。

- [ ] **Step 5: 实现 outreach.go**

Create `server/internal/robot/outreach.go`：

```go
package robot

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/pkg/cache"
)

// inWindow 判断当前小时是否在 [start, end) 内；start>=end 视为关闭(返回 false)。
func inWindow(hour, start, end int) bool {
	if start >= end {
		return false
	}
	return hour >= start && hour < end
}

// sampleCount 抽样数量 = total*pct/100，向下取整。
func sampleCount(total, pct int) int {
	if total <= 0 || pct <= 0 {
		return 0
	}
	return total * pct / 100
}

// StartOutreachCron 启动机器人主动触达定时器(#增长5)。
func StartOutreachCron(rsvc *Service, sender MessageSender, defaultTenant int64) {
	go func() {
		for {
			tickMin := sysconfig.GetInt(sysconfig.KeyOutreachTickMin)
			if tickMin <= 0 {
				tickMin = 30
			}
			time.Sleep(time.Duration(tickMin) * time.Minute)
			outreachTick(rsvc, sender, defaultTenant)
		}
	}()
	log.Printf("[robot/outreach] cron started")
}

func outreachTick(rsvc *Service, sender MessageSender, tenantID int64) {
	if !sysconfig.GetBool(sysconfig.KeyOutreachEnabled) {
		return
	}
	now := time.Now()
	if !inWindow(now.Hour(), sysconfig.GetInt(sysconfig.KeyOutreachWindowStart), sysconfig.GetInt(sysconfig.KeyOutreachWindowEnd)) {
		return
	}
	lookback := sysconfig.GetInt(sysconfig.KeyOutreachLookbackMin)
	if lookback <= 0 {
		lookback = 30
	}
	since := now.Add(-time.Duration(lookback) * time.Minute)

	// 近期活跃真人候选
	var candidates []model.User
	rsvc.db.Where("tenant_id = ? AND is_robot = ? AND status = ? AND is_muted = ? AND last_active_at >= ?",
		tenantID, false, "active", false, since).
		Limit(2000).Find(&candidates)
	if len(candidates) == 0 {
		return
	}

	// 抽样
	pick := sampleCount(len(candidates), sysconfig.GetInt(sysconfig.KeyOutreachProbability))
	if pick == 0 {
		return
	}
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	if pick > len(candidates) {
		pick = len(candidates)
	}
	selected := candidates[:pick]

	// 机器人池
	var robots []model.User
	rsvc.db.Where("tenant_id = ? AND is_robot = ? AND status = ?", tenantID, true, "active").Find(&robots)
	if len(robots) == 0 {
		return
	}

	cap := sysconfig.GetInt(sysconfig.KeyOutreachUserDailyCap)
	if cap <= 0 {
		cap = 1
	}
	sent := 0
	for _, u := range selected {
		if !claimOutreach(tenantID, u.UserID, cap) {
			continue
		}
		bot := robots[rand.Intn(len(robots))]
		chatID, err := sender.EnsureRobotChat(tenantID, bot.UserID, u.UserID)
		if err != nil {
			continue
		}
		text := genOpening(rsvc, tenantID, bot.UserID)
		if text == "" {
			continue
		}
		if _, err := sender.SendMessage(tenantID, bot.UserID, chatID, text, "text"); err != nil {
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Printf("[robot/outreach] tenant=%d 触达 %d 人 (候选=%d 抽样=%d)", tenantID, sent, len(candidates), pick)
	}
}

// claimOutreach 占用一次用户当日触达额度；超过 cap 返回 false。
func claimOutreach(tenantID, userID int64, cap int) bool {
	ctx := context.Background()
	key := fmt.Sprintf("outreach:%d:%d:%s", tenantID, userID, time.Now().Format("20060102"))
	n, err := cache.RDB.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	if int(n) > cap {
		cache.RDB.Decr(ctx, key)
		return false
	}
	return true
}

// genOpening 生成机器人主动开场白；LLM 失败回退静态内容池(reply)。
func genOpening(rsvc *Service, tenantID, botUserID int64) string {
	if sysconfig.GetBool(sysconfig.KeyAIChatEnabled) || sysconfig.GetBool(sysconfig.KeyAIBotEnabled) {
		persona := rsvc.registry.Get(botUserID)
		sys := buildPrompt(persona, model.RobotMemory{Familiarity: 0.3})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		reply, err := rsvc.llm.Chat(ctx, sys, nil, "请你主动发起聊天，发一句自然、简短、符合你人设的开场问候，不要透露AI身份，不超过30字。")
		if err == nil && reply != "" && !containsAIConfession(reply) {
			return reply
		}
	}
	// 回退静态池
	var pool []model.RobotContent
	rsvc.db.Where("tenant_id = ? AND type = ?", tenantID, "reply").Find(&pool)
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))].Text
}
```

> 注：`rsvc.db`、`rsvc.registry`、`rsvc.llm` 均为 `robot.Service` 的同包私有字段，`outreach.go` 在 package robot 内可直接访问。`buildPrompt` / `containsAIConfession` 同包可用。

- [ ] **Step 6: 运行纯函数测试确认通过**

Run: `cd server && go test ./internal/robot/ -run 'TestInWindow|TestSampleCount' -v`
Expected: PASS。

- [ ] **Step 7: main.go 启动 outreach cron**

在 `main.go` 中 `robot.InitWorkerPool(...)` 之后追加：

```go
	robot.StartOutreachCron(robotSvc, chatSvc, cfg.DefaultTenantID)
```

- [ ] **Step 8: 构建验证**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/robot/...`
Expected: 全部通过。

- [ ] **Step 9: Commit**

```bash
cd server && git add internal/robot/bot_worker.go internal/robot/outreach.go internal/robot/outreach_test.go internal/chat/service.go cmd/api/main.go
git commit -m "feat(robot): 按概率主动触达近期活跃用户(#增长5)"
```

---

### Task 7: 客户端 — 签到 + 分享领币 + 消息扣费提示

> uni-app(Vue3) 前端。本任务把三个 C 端触点接上。沿用 `client/src/utils` 既有请求封装与既有充值引导弹框。

**Files:**
- Modify: `client/src/pages/mine/mine.vue` (签到入口)
- Modify: 触发分享的页面 `onShareAppMessage`（如 `client/src/pages/ocean/ocean.vue` / `detail/detail.vue`）
- Modify: `client/src/pages/chat/chat.vue` (消息扣费余额不足提示)

**Interfaces:**
- Consumes 后端：`GET /api/checkin/status`、`POST /api/checkin`、`POST /api/share/reward`；发消息接口返回的 `ErrInsufficient` 错误码。

- [ ] **Step 1: 确认请求封装与错误码**

Run: 阅读 `client/src/utils/`（request 封装）与现有「余额不足」处理位置（搜索 `充值` / `余额`）。
Expected: 找到统一 request 方法与 `ErrInsufficient` 对应的 code 常量，复用之。

- [ ] **Step 2: 「我的」页接入签到**

在 `mine.vue` 加入签到按钮：`onShow` 调 `GET /api/checkin/status`，按 `signed_today`/`enabled` 渲染「签到领{coins}金币」或「今日已签」；点击调 `POST /api/checkin`，成功 `uni.showToast` 显示「+{coins}金币」，并刷新余额与按钮态。

- [ ] **Step 3: 分享回调接入领币**

在含 `onShareAppMessage` 的页面，分享回调里追加 `POST /api/share/reward`，按返回 `rewarded`：true 则 toast「分享成功 +{coins}金币」，false 则 toast「今日分享奖励已领完」。

- [ ] **Step 4: 消息扣费余额不足提示**

在 `chat.vue` 发送消息的失败处理中，识别 `ErrInsufficient` 错误码 → 复用现有充值引导弹框（与开聊一致），不再静默失败。

- [ ] **Step 5: 客户端联调验证**

Run: 启动后端 + 微信开发者工具，依次验证签到发币、分享发币(达上限提示)、`price_msg>0` 时发消息扣费与余额不足弹窗。
Expected: 三处行为符合预期，钱包流水可见对应 `checkin`/`share`/`msg` 记录。

- [ ] **Step 6: Commit**

```bash
git add client/src/pages/
git commit -m "feat(client): 签到/分享领币入口 + 消息扣费余额提示"
```

---

## Self-Review

**Spec coverage:**
- 功能1 签到 → Task 1(配置) + Task 3 + Task 7。✓
- 功能2 消息扣费 → Task 1(price_msg) + Task 4 + Task 7。✓
- 功能3 分享 → Task 1(配置) + Task 5 + Task 7。✓
- 功能4 登录/活跃时间 → Task 2。✓
- 功能5 机器人主动触达 → Task 1(配置) + Task 6。✓
- admin 白名单 → Task 1 Step 3。✓
- 主程序接线(迁移/路由/cron/中间件) → Task 2/3/5/6 各自 main.go 步骤。✓

**类型一致性:** `EnsureRobotChat(tenantID, botUserID, userID int64)(int64,error)` 在 Task 6 Step 1(接口) / Step 2(实现) 一致；`wallet.SceneCheckin/SceneMsg/SceneShare` 字符串值 `checkin`/`msg`/`share` 均 ≤24；`sysconfig` 键在 Task 1 定义、各任务引用名一致。

**已知简化(非占位符，刻意为之):**
- checkin 以「Create 失败即视为已签」简化唯一键判断（Task 3 Step 5 已注明）。
- 分享奖励信任客户端触发，靠每日上限兜量（spec 已声明）。
- DB 绑定逻辑(SendMessage 扣费、签到事务、触达查询)无自动化测试，靠 `go build`/`go vet` + 代码审查，纯逻辑(bizNo/dayKey/inWindow/sampleCount)有单测——与现有代码库测试风格一致。
