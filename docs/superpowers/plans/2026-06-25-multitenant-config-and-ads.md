# 多租户配置改造 + 流量主广告 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把全局配置层改造成按租户隔离（租户值→全局→代码默认三级回退），并在其上接入微信流量主 4 类广告（按租户配置开关与广告ID）。

**Architecture:** Phase A 改 `config` 表为复合主键 `(tenant_id,key)`，`sysconfig` 全 API 加 `tenantID` 首参并按租户缓存+回退；71 处调用点与若干 ripple 签名同步改造（一个原子任务保证编译绿）。Phase B 新增 `ad` 配置键/下发端点/激励发币，客户端用 `store/ads` + `<ad-slot>` 组件 + `utils/ad.js` 渲染。

**Tech Stack:** Go 1.22、Gin、GORM(MySQL)、go-redis(`pkg/cache`)、uni-app(Vue3, mp-weixin)、雪花 ID。

## Global Constraints

- 配置解析三级回退：租户值(非空) → 全局 `tenant_id=0`(非空) → 代码 `defaults[key]`。
- `config` 表（GORM 表名 `configs`）复合主键 `(tenant_id, key)`；`tenant_id=0` = 全局默认；`key` 为 MySQL 保留字，原生 SQL 用反引号。
- `sysconfig` 公开 API 一律 `GetString/GetInt/GetInt64/GetBool(tenantID int64, key string)`、`Set(tenantID int64, key, value string)`。
- 资金发放走 `wallet` 包并产生 `WalletTxn`，`Scene` 字段 ≤24 字符。
- 新增 model 加入 `model.AllModels()`。
- 空 wx appid 登录回退默认 `wxe48b23d248677356`（env `DEFAULT_WX_APPID` 可覆盖）。
- 广告展示条件：`ad_enabled` ∧ `ad_<slot>_on` ∧ `ad_<slot>_unit` 非空（均按当前租户解析）。
- 每个任务结束 `cd server && go build ./... && go vet ./...` 必须通过。
- 提交信息结尾加：`Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

# Phase A：多租户配置改造

### Task A1: Config 复合主键 + 迁移

**Files:**
- Modify: `server/internal/model/model.go`（Config 结构体）
- Modify: `server/internal/bootstrap/migrate.go`

**Interfaces:**
- Produces: `Config{TenantID,Key,...}` 复合主键；`bootstrap.Migrate` 在 AutoMigrate 前执行 `migrateConfigTenant`。

- [ ] **Step 1: 改 Config 结构体**

`model.go` 的 `Config` 改为：
```go
// Config 后台配置(价格/开关,运营热调)。tenant_id=0 为全局默认,非0为租户覆盖。
type Config struct {
	TenantID  int64     `gorm:"primaryKey" json:"tenant_id,string"`
	Key       string    `gorm:"primaryKey;size:48" json:"key"`
	Value     string    `gorm:"size:255" json:"value"`
	Remark    string    `gorm:"size:128" json:"remark"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: 加幂等迁移函数并在 Migrate 中调用**

`migrate.go` 顶部 `Migrate` 改为先迁移配置表：
```go
func Migrate(db *gorm.DB) error {
	if err := dedupeChats(db); err != nil {
		return err
	}
	if err := migrateConfigTenant(db); err != nil {
		return err
	}
	return db.AutoMigrate(model.AllModels()...)
}

// migrateConfigTenant 把旧的 configs(主键 key) 升级为复合主键 (tenant_id, key)。
// 既有行 tenant_id 默认 0 → 自动成为全局默认。幂等：已有 tenant_id 列则跳过；全新库交给 AutoMigrate。
func migrateConfigTenant(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Config{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.Config{}, "tenant_id") {
		return nil
	}
	if err := db.Exec("ALTER TABLE configs ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 0").Error; err != nil {
		return err
	}
	return db.Exec("ALTER TABLE configs DROP PRIMARY KEY, ADD PRIMARY KEY (tenant_id, `key`)").Error
}
```

- [ ] **Step 3: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 通过（注意：此时 sysconfig 仍用旧 API，编译仍绿；下个任务才改 API）。

- [ ] **Step 4: Commit**

```bash
cd server && git add internal/model/model.go internal/bootstrap/migrate.go
git commit -m "feat(config): Config 复合主键(tenant_id,key)+迁移"
```

---

### Task A2: sysconfig 按租户 + 全部调用点改造（原子任务）

> 本任务把 `sysconfig` API 全部加 `tenantID` 首参，会一次性破坏所有调用点的编译，因此**必须在同一提交内**改完 sysconfig 核心 + 全部 71 处调用 + ripple 签名，最终编译通过。建议本任务在主会话内执行（inline），便于一气呵成。

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`（核心）
- Create: `server/internal/sysconfig/resolve_test.go`（单测）
- Modify: `server/internal/sysconfig/handler.go`（公开端点按租户）
- Modify: 全部调用点（下方枚举）+ ripple 签名文件

**Interfaces:**
- Produces:
  - `sysconfig.GetString(tenantID int64, key string) string`、`GetInt`、`GetInt64`、`GetBool`、`Set(tenantID, key, value)`
  - `checkin.Service.Status(tenantID, userID int64)`
  - `user.Service.EnsureVerifiedIfRequired(tenantID, userID int64) error` + `chat.Verifier` 接口同步
  - `(*robot.LLMClient).Chat(ctx, tenantID int64, systemPrompt string, history []llmMessage, userText string)`
  - `push.Service.GetTemplates(tenantID)`、`sendToScene(tenantID,...)`、`getAccessToken(tenantID)`
  - `admin.Service.Config(tenantID)`、`SetConfig(tenantID,key,value)`、`Stats(tenantID)`

- [ ] **Step 1: 写 sysconfig 解析单测（RED）**

Create `server/internal/sysconfig/resolve_test.go`：
```go
package sysconfig

import "testing"

func TestResolveFallback(t *testing.T) {
	// 直接构造按租户缓存:tenant 0 全局, tenant 100 覆盖
	mu.Lock()
	cached = map[int64]map[string]string{
		0:   {"k_global": "g", "k_both": "G"},
		100: {"k_both": "T", "k_empty": ""},
	}
	mu.Unlock()
	defaults["k_default"] = "D"

	cases := []struct{ tid int64; key, want string }{
		{100, "k_both", "T"},     // 租户值优先
		{100, "k_global", "g"},   // 回退全局
		{100, "k_default", "D"},  // 回退代码默认
		{100, "k_empty", "g_or"}, // 租户空串 → 回退(此处全局无该键→默认空)
		{200, "k_global", "g"},   // 无该租户 → 全局
	}
	// k_empty 期望:租户空串视为未设→回退全局(无)→默认(无)→""
	cases[3].want = ""
	for _, c := range cases {
		if got := GetString(c.tid, c.key); got != c.want {
			t.Errorf("GetString(%d,%q)=%q want %q", c.tid, c.key, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/sysconfig/ -run TestResolveFallback -v`
Expected: 编译失败（`GetString` 签名仍是单参 / 类型不符）。

- [ ] **Step 3: 重构 sysconfig 核心**

`sysconfig.go` 关键改动（保留 `defaults`、`Init`、`reload` 框架，改缓存与 API）：
```go
var (
	mu     sync.RWMutex
	cached map[int64]map[string]string // tenantID → key → value
	db     *gorm.DB
)

func reload() error {
	var rows []model.Config
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	m := make(map[int64]map[string]string)
	for _, r := range rows {
		if m[r.TenantID] == nil {
			m[r.TenantID] = map[string]string{}
		}
		m[r.TenantID][r.Key] = r.Value
	}
	mu.Lock()
	cached = m
	mu.Unlock()
	return nil
}

func Reload() error { return reload() }

// get 三级回退:租户值(非空) → 全局 tenant 0(非空) → 代码默认。
func get(tenantID int64, key string) string {
	mu.RLock()
	if tm, ok := cached[tenantID]; ok {
		if v, ok := tm[key]; ok && v != "" {
			mu.RUnlock()
			return v
		}
	}
	if gm, ok := cached[0]; ok {
		if v, ok := gm[key]; ok && v != "" {
			mu.RUnlock()
			return v
		}
	}
	mu.RUnlock()
	return defaults[key]
}

func GetString(tenantID int64, key string) string { return get(tenantID, key) }
func GetInt(tenantID int64, key string) int        { n, _ := strconv.Atoi(get(tenantID, key)); return n }
func GetInt64(tenantID int64, key string) int64    { n, _ := strconv.ParseInt(get(tenantID, key), 10, 64); return n }
func GetBool(tenantID int64, key string) bool      { return get(tenantID, key) == "1" }

// Set 写入指定租户的配置并更新缓存。
func Set(tenantID int64, key, value string) error {
	cfg := model.Config{TenantID: tenantID, Key: key, Value: value, UpdatedAt: time.Now()}
	if err := db.Save(&cfg).Error; err != nil {
		return err
	}
	mu.Lock()
	if cached == nil {
		cached = map[int64]map[string]string{}
	}
	if cached[tenantID] == nil {
		cached[tenantID] = map[string]string{}
	}
	cached[tenantID][key] = value
	mu.Unlock()
	return nil
}
```
`seedDefaults()` 改为写 **tenant 0**：把 `_ = Set(k, v)` 改为 `_ = Set(0, k, v)`；其 `loaded` 判断基于 `cached[0]`。

- [ ] **Step 4: sysconfig/handler.go 公开端点按租户**

这些端点（`/notice`、`/tabs`、`/mine-functions`、`/hook-count`）当前无鉴权。改为可选解析 JWT 取 tenant，回退默认租户。

在 `Handler` 加默认租户字段与可选 tenant 解析：
```go
type Handler struct{ defaultTenant int64 }

func NewHandler(defaultTenant int64) *Handler { return &Handler{defaultTenant: defaultTenant} }

// tenantOf 可选鉴权:有合法 Bearer 则取其 tenant,否则用默认租户。
func (h *Handler) tenantOf(c *gin.Context) int64 {
	a := c.GetHeader("Authorization")
	if strings.HasPrefix(a, "Bearer ") {
		if claims, err := jwtutil.Parse(strings.TrimPrefix(a, "Bearer ")); err == nil {
			return claims.TenantID
		}
	}
	return h.defaultTenant
}
```
把 handler 内所有 `GetString(KeyX)` / `GetInt(KeyX)` 改为 `GetString(tid, KeyX)`，其中 `tid := h.tenantOf(c)`（每个 handler 方法开头取一次）。`getMineFunctions` 的 `show := func(k string) bool { return GetString(tid, k) != "0" }`。
main.go 注册处 `sysconfig.NewHandler(cfg.DefaultTenantID).Register(api)`（import 已含 jwtutil/strings 需补）。

- [ ] **Step 5: 改全部业务调用点（按下表，加 tenantID 首参）**

规则：把 `sysconfig.GetX(KeyY)` → `sysconfig.GetX(<tenant>, KeyY)`，`<tenant>` 取该处在作用域内的 tenantID。逐文件：

| 文件:行 | 所在方法（tenant 来源） | `<tenant>` 表达式 |
|---|---|---|
| bottle/feed.go:77,134-140 | `rebuild(ctx, tenantID,…)` | `tenantID` |
| bottle/service.go:86,97,106,129,130 | Throw/Scoop/quota（`tenantID` 参数） | `tenantID` |
| bottle/service.go:260,261,338 | `UnlockReply(tenantID,…)` | `tenantID` |
| chat/service.go:70 | `StartChat(tenantID,…)` | `tenantID` |
| chat/service.go:140 | `SendMessage(tenantID,…)` | `tenantID` |
| item/service.go:71,73 | 方法持 `tenantID` | `tenantID` |
| notify/service.go:58 | `OnBottleReplied(tenantID,…)` | `tenantID` |
| robot/service.go:184,187,198,216,255,267 | `Tick(tenantID)` / throwBottles/maybeReply(持 tenantID) | `tenantID` |
| robot/bot_worker.go:99,169,170 | `handle(job)`/`deliver(job,…)` | `job.TenantID` |
| robot/bot_worker.go:61 | `InitWorkerPool`（启动期，无租户） | `defaultTenant`（见下） |
| robot/outreach.go:47,51,54,70,87 | `outreachTick(…, tenantID)` | `tenantID` |
| robot/outreach.go:135 | `genOpening(rsvc, tenantID,…)` | `tenantID` |
| robot/outreach.go:35 | `StartOutreachCron` 循环（cron 的租户） | `tenantID`（给 cron 加 tenant 形参，见下） |
| robot/llmclient.go:28,41 | `newLLMClient`/`RebuildSem`（启动期） | `defaultTenant` |
| share/service.go:31,32,34 | `Reward(tenantID,…)` | `tenantID` |
| user/service.go:115 | `Login`（已解析 tenantID） | `tenantID` |
| checkin/service.go:39,43 | `Sign(tenantID,…)` | `tenantID` |

robot 启动期 `defaultTenant`：`InitWorkerPool`、`newLLMClient`、`RebuildSem` 读取并发/LLM 参数用全局即可——给这些函数加 `defaultTenant int64` 形参，`main.go` 传 `cfg.DefaultTenantID`。`StartOutreachCron(rsvc, sender, defaultTenant)` 已有 `defaultTenant` 形参，循环内 `outreachTick(...)` 传它；`tickMin := sysconfig.GetInt(defaultTenant, KeyOutreachTickMin)`。

- [ ] **Step 6: ripple 签名改造（含调用方）**

**checkin.Status 加 tenantID：**
```go
func (s *Service) Status(tenantID, userID int64) (enabled bool, coins int64, signedToday bool) {
	enabled = sysconfig.GetBool(tenantID, sysconfig.KeyCheckinEnabled)
	coins = sysconfig.GetInt64(tenantID, sysconfig.KeyCheckinCoins)
	...
}
```
caller `checkin/handler.go` status：`enabled, coins, signed := h.svc.Status(middleware.TenantID(c), middleware.UserID(c))`。

**user.EnsureVerifiedIfRequired 加 tenantID + chat.Verifier 接口同步：**
```go
// user/service.go
func (s *Service) EnsureVerifiedIfRequired(tenantID, userID int64) error {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyVerifyRequired) { return nil }
	...
}
// chat/service.go 接口
type Verifier interface {
	EnsureVerifiedIfRequired(tenantID, userID int64) error
}
// chat StartChat 调用
if err := s.verf.EnsureVerifiedIfRequired(tenantID, userID); err != nil { return nil, err }
```

**llmclient.Chat 加 tenantID：**
```go
func (c *LLMClient) Chat(ctx context.Context, tenantID int64, systemPrompt string, history []llmMessage, userText string) (string, error) {
	endpoint := sysconfig.GetString(tenantID, sysconfig.KeyLLMAPIEndpoint)
	apiKeyEnc := sysconfig.GetString(tenantID, sysconfig.KeyLLMAPIKey)
	model := sysconfig.GetString(tenantID, sysconfig.KeyLLMModel)
	maxTokens := sysconfig.GetInt(tenantID, sysconfig.KeyLLMMaxTokens)
	temp := float64(sysconfig.GetInt(tenantID, sysconfig.KeyLLMTemperature)) / 10.0
	...
}
```
callers：
- `bot_worker.go` handle：`p.llm.Chat(ctx, job.TenantID, systemPrompt, historyMsgs, job.UserMsg)`
- `robot/service.go` genAIText：`s.llm.Chat(bgCtx, tenantID, s.buildPathABSystemPrompt(personaRole), nil, userPrompt)`（genAIText 已持 tenantID）
- `outreach.go` genOpening：`rsvc.llm.Chat(ctx, tenantID, sys, nil, "…")`
- `robot/bot_worker.go` `TestLLM`：改 `TestLLM(ctx, tenantID, prompt)` 并由 admin 调用方传 tenant（admin LLM 测试用 `tenantFromCtx(c)`，无则 defaultTenant）。

**push 按租户（templates/appid/secret/token）：**
- `GetTemplates(tenantID int64)`：`if id := sysconfig.GetString(tenantID, p.key); id != ""`。caller `push/handler.go` templates：`h.svc.GetTemplates(h.tenantOf(c))`（push handler 同样加可选 tenant 解析，或该端点改为需登录用 middleware.TenantID）。
- `sendToScene`（138 行所在方法）：加 `tenantID` 首参，`tmplID := sysconfig.GetString(tenantID, key)`；调用方传对应 tenant。
- `getAccessToken(tenantID int64)`：appid/secret 按租户；token 缓存改为按租户 `map[int64]tokenEntry`：
```go
type tokenEntry struct { token string; exp time.Time }
// Service 字段:tokens map[int64]tokenEntry (替换原单 token/tokenExp);mu 复用
func (s *Service) getAccessToken(tenantID int64) (string, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	if e, ok := s.tokens[tenantID]; ok && time.Now().Before(e.exp) {
		return e.token, nil
	}
	appid := sysconfig.GetString(tenantID, sysconfig.KeyPushWxAppID)
	secret := sysconfig.GetString(tenantID, sysconfig.KeyPushWxSecret)
	if appid == "" || secret == "" { return "", fmt.Errorf("push: appid/secret 未配置") }
	// …请求 token…成功后 s.tokens[tenantID] = tokenEntry{token, time.Now().Add(...)}
}
```
sendOne / 发送路径传 tenantID（`NotifyNewMessage(tenantID,…)` 已有；`sendCheckinAll` 用 `sub.TenantID`）。`push.New` 初始化 `tokens: map[int64]tokenEntry{}`。

**admin Config/SetConfig/Stats 加 tenantID：**
```go
func (s *Service) Config(tenantID int64) []ConfigField {
	out := make([]ConfigField, len(configMeta))
	copy(out, configMeta)
	for i := range out {
		out[i].Value = sysconfig.GetString(tenantID, out[i].Key)
	}
	return out
}
func (s *Service) SetConfig(tenantID int64, key, value string) error {
	if !allowedKey(key) { return errors.New("非法配置键") }
	if err := sysconfig.Set(tenantID, key, value); err != nil { return err }
	_ = sysconfig.Reload()
	return nil
}
func (s *Service) Stats(tenantID int64) Stats { /* st.Robots = sysconfig.GetInt64(tenantID, KeyRobotCount) */ }
```
callers `admin/handler.go`：`getConfig` → `h.svc.Config(tenantFromCtx(c))`；`putConfig` → `h.svc.SetConfig(tenantFromCtx(c), req.Key, req.Value)`；stats handler → `h.svc.Stats(tenantFromCtx(c))`。

**user/handler.go 登录与 profile 配置项：**
- 登录响应（行 57-67）：tenant 用 `u.TenantID`，如 `sysconfig.GetBool(u.TenantID, sysconfig.KeyIOSRechargeOff)`，其余同。
- profile（行 79）：`sysconfig.GetInt(middleware.TenantID(c), sysconfig.KeyPriceChat)`。

- [ ] **Step 7: 运行单测 + 全量构建**

Run: `cd server && go test ./internal/sysconfig/ -run TestResolveFallback -v && go build ./... && go vet ./...`
Expected: 单测 PASS；构建/vet 全绿。

- [ ] **Step 8: Commit**

```bash
cd server && git add internal/
git commit -m "feat(config): sysconfig 按租户(三级回退)+全部调用点改造"
```

---

### Task A3: 空 appid 兜底默认 + DefaultWxAppID

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/internal/user/service.go`（resolveLogin）

**Interfaces:**
- Produces: `config.Config.DefaultWxAppID`；`resolveLogin` 空 wx appid 回退默认。

- [ ] **Step 1: config 加 DefaultWxAppID**

`config.go` `Config` 结构体加字段：
```go
	DefaultWxAppID string // 多租户:wx appid 缺省时回退的默认小程序 appid
```
`Load()` 内 `MultiTenant` 附近加：
```go
		DefaultWxAppID: getEnv("DEFAULT_WX_APPID", "wxe48b23d248677356"),
```

- [ ] **Step 2: resolveLogin 兜底**

`user/service.go` `resolveLogin` 多租户分支开头：
```go
	if s.cfg.MultiTenant {
		if platform == "wx" && appid == "" {
			appid = s.cfg.DefaultWxAppID
		}
		r, ok := s.creds.ByAppID(platform, appid)
		if !ok {
			return 0, "", "", "", errs.New(errs.CodeLoginFailed, "未知的小程序 appid,请检查租户凭证配置")
		}
		...
```

- [ ] **Step 3: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
cd server && git add internal/config/config.go internal/user/service.go
git commit -m "feat(tenant): 空wx appid 登录回退默认 wxe48b23d248677356"
```

---

# Phase B：流量主广告

### Task B1: 广告配置键 + admin 白名单

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`

**Interfaces:**
- Produces: 常量 `KeyAdEnabled`、`KeyAdInterGapSec`、`KeyAdRewardCoins`、`KeyAdRewardDaily`，以及每个 slot 的 `KeyAd<Slot>On`/`KeyAd<Slot>Unit`（slot 见下 15 个）。

slot 列表：`BannerOcean BannerCity BannerExpand BannerMessage BannerMine BannerDetail BannerChat BannerCollection BannerOrders BannerWalletlog BannerViewed InterScoop InterDetail RewardCoin GridMine`，对应 key 串：`ad_banner_ocean_on/unit` … `ad_grid_mine_on/unit`。

- [ ] **Step 1: sysconfig 常量 + 默认值**

`sysconfig.go` 常量区追加（示例给全，按 slot 列表补齐 15 组）：
```go
	// 流量主广告
	KeyAdEnabled     = "ad_enabled"
	KeyAdInterGapSec = "ad_inter_gap_sec"
	KeyAdRewardCoins = "ad_reward_coin_coins"
	KeyAdRewardDaily = "ad_reward_coin_daily"

	KeyAdBannerOceanOn       = "ad_banner_ocean_on"
	KeyAdBannerOceanUnit     = "ad_banner_ocean_unit"
	KeyAdBannerCityOn        = "ad_banner_city_on"
	KeyAdBannerCityUnit      = "ad_banner_city_unit"
	KeyAdBannerExpandOn      = "ad_banner_expand_on"
	KeyAdBannerExpandUnit    = "ad_banner_expand_unit"
	KeyAdBannerMessageOn     = "ad_banner_message_on"
	KeyAdBannerMessageUnit   = "ad_banner_message_unit"
	KeyAdBannerMineOn        = "ad_banner_mine_on"
	KeyAdBannerMineUnit      = "ad_banner_mine_unit"
	KeyAdBannerDetailOn      = "ad_banner_detail_on"
	KeyAdBannerDetailUnit    = "ad_banner_detail_unit"
	KeyAdBannerChatOn        = "ad_banner_chat_on"
	KeyAdBannerChatUnit      = "ad_banner_chat_unit"
	KeyAdBannerCollectionOn  = "ad_banner_collection_on"
	KeyAdBannerCollectionUnit = "ad_banner_collection_unit"
	KeyAdBannerOrdersOn      = "ad_banner_orders_on"
	KeyAdBannerOrdersUnit    = "ad_banner_orders_unit"
	KeyAdBannerWalletlogOn   = "ad_banner_walletlog_on"
	KeyAdBannerWalletlogUnit = "ad_banner_walletlog_unit"
	KeyAdBannerViewedOn      = "ad_banner_viewed_on"
	KeyAdBannerViewedUnit    = "ad_banner_viewed_unit"
	KeyAdInterScoopOn        = "ad_inter_scoop_on"
	KeyAdInterScoopUnit      = "ad_inter_scoop_unit"
	KeyAdInterDetailOn       = "ad_inter_detail_on"
	KeyAdInterDetailUnit     = "ad_inter_detail_unit"
	KeyAdRewardCoinOn        = "ad_reward_coin_on"
	KeyAdRewardCoinUnit      = "ad_reward_coin_unit"
	KeyAdGridMineOn          = "ad_grid_mine_on"
	KeyAdGridMineUnit        = "ad_grid_mine_unit"
```
`defaults` 追加：`KeyAdEnabled:"0"`、`KeyAdInterGapSec:"180"`、`KeyAdRewardCoins:"5"`、`KeyAdRewardDaily:"5"`，全部 `*_on:"0"`、`*_unit:""`。

- [ ] **Step 2: admin meta 分组「流量主广告」**

`admin/meta.go` `configMeta` 末尾追加：总开关/间隔/激励参数（bool/int）+ 每个 slot 两项（`*_on` bool、`*_unit` text），Group 统一 `"流量主广告"`。例：
```go
	{Key: sysconfig.KeyAdEnabled, Label: "流量主总开关", Group: "流量主广告", Type: "bool"},
	{Key: sysconfig.KeyAdInterGapSec, Label: "插屏最小间隔(秒)", Group: "流量主广告", Type: "int"},
	{Key: sysconfig.KeyAdRewardCoins, Label: "激励视频每次金币", Group: "流量主广告", Type: "int"},
	{Key: sysconfig.KeyAdRewardDaily, Label: "激励视频每日上限", Group: "流量主广告", Type: "int"},
	{Key: sysconfig.KeyAdBannerOceanOn, Label: "首页Banner开关", Group: "流量主广告", Type: "bool"},
	{Key: sysconfig.KeyAdBannerOceanUnit, Label: "首页Banner广告ID", Group: "流量主广告", Type: "text"},
	// …其余 14 个 slot 同样两项…
```

- [ ] **Step 3: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
cd server && git add internal/sysconfig/sysconfig.go internal/admin/meta.go
git commit -m "feat(ad): 广告配置键(15位)+admin 白名单"
```

---

### Task B2: 激励发币（wallet 场景 + ad 包）

**Files:**
- Modify: `server/internal/wallet/service.go`
- Create: `server/internal/ad/service.go`、`server/internal/ad/handler.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Consumes: `cache.RDB`、`wallet.Service.Credit/Balance`、`sysconfig.Get*(tenantID,…)`。
- Produces: `wallet.SceneAdReward="ad_reward"`；`ad.New(db, *wallet.Service)`；`(*ad.Service).Reward(tenantID, userID int64)(rewarded bool, coins, balance int64, count, limit int, err error)`；`ad.NewHandler(svc).Register(api, auth)` → `POST /api/ad/reward`。

- [ ] **Step 1: wallet 场景常量**

`wallet/service.go` 常量区追加：`SceneAdReward = "ad_reward"`。

- [ ] **Step 2: 写 dayKey 单测（RED）**

Create `server/internal/ad/key_test.go`：
```go
package ad

import "testing"

func TestDayKey(t *testing.T) {
	if k := dayKey(1, 9, "20260625"); k != "adreward:1:9:20260625" {
		t.Fatalf("dayKey=%s", k)
	}
}
```
Run: `cd server && go test ./internal/ad/ -run TestDayKey -v` → 编译失败。

- [ ] **Step 3: 实现 ad/service.go**

```go
// Package ad 流量主激励视频发币(信任 isEnded + 每日上限)。
package ad

import (
	"context"
	"fmt"
	"log"
	"time"

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

func today() string                               { return time.Now().Format("20060102") }
func dayKey(tenantID, userID int64, d string) string {
	return fmt.Sprintf("adreward:%d:%d:%s", tenantID, userID, d)
}

func (s *Service) Reward(tenantID, userID int64) (rewarded bool, coins int64, balance int64, countToday, limit int, err error) {
	limit = sysconfig.GetInt(tenantID, sysconfig.KeyAdRewardDaily)
	coins = sysconfig.GetInt64(tenantID, sysconfig.KeyAdRewardCoins)
	on := sysconfig.GetBool(tenantID, sysconfig.KeyAdEnabled) &&
		sysconfig.GetBool(tenantID, sysconfig.KeyAdRewardCoinOn) &&
		sysconfig.GetString(tenantID, sysconfig.KeyAdRewardCoinUnit) != ""
	if !on || limit <= 0 || coins <= 0 {
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, 0, limit, nil
	}
	ctx := context.Background()
	key := dayKey(tenantID, userID, today())
	n, e := cache.RDB.Incr(ctx, key).Result()
	if e != nil {
		log.Printf("[ad] reward incr err uid=%d: %v", userID, e)
		return false, coins, 0, 0, limit, e
	}
	if n == 1 {
		cache.RDB.Expire(ctx, key, 48*time.Hour)
	}
	countToday = int(n)
	if int(n) > limit {
		cache.RDB.Decr(ctx, key)
		bal, _ := s.wlt.Balance(userID)
		return false, coins, bal, limit, limit, nil
	}
	bizNo := fmt.Sprintf("adreward:%d:%s:%d", userID, today(), n)
	if e := s.wlt.Credit(tenantID, userID, coins, wallet.SceneAdReward, bizNo); e != nil {
		log.Printf("[ad] reward credit err uid=%d: %v", userID, e)
		cache.RDB.Decr(ctx, key)
		return false, coins, 0, countToday - 1, limit, e
	}
	bal, _ := s.wlt.Balance(userID)
	return true, coins, bal, countToday, limit, nil
}
```

- [ ] **Step 4: 实现 ad/handler.go**

```go
package ad

import (
	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.Group("/ad", auth).POST("/reward", h.reward)
}

func (h *Handler) reward(c *gin.Context) {
	rewarded, coins, balance, count, limit, err := h.svc.Reward(middleware.TenantID(c), middleware.UserID(c))
	if err != nil {
		response.Fail(c, errs.CodeServerError, "领取失败")
		return
	}
	response.OK(c, gin.H{"rewarded": rewarded, "coins": coins, "balance": balance, "count_today": count, "limit": limit})
}
```

- [ ] **Step 5: main.go 注册**

`main.go` 路由区追加：`ad.NewHandler(ad.New(db, walletSvc)).Register(api, auth)`，import `"driftbottle/internal/ad"`。

- [ ] **Step 6: 构建 + 测试**

Run: `cd server && go test ./internal/ad/... && go build ./... && go vet ./...`
Expected: 全通过。

- [ ] **Step 7: Commit**

```bash
cd server && git add internal/wallet/service.go internal/ad/ cmd/api/main.go
git commit -m "feat(ad): 激励视频发币 POST /api/ad/reward"
```

---

### Task B3: GET /api/ads 下发

**Files:**
- Modify: `server/internal/sysconfig/handler.go`

**Interfaces:**
- Produces: `GET /api/ads` 返回 `{enabled, inter_gap_sec, slots:{<slot>:{on,unit[,coins,daily]}}}`，按当前租户解析。

- [ ] **Step 1: 加 getAds 端点**

`sysconfig/handler.go` `Register` 追加 `api.GET("/ads", h.getAds)`，并实现：
```go
func (h *Handler) getAds(c *gin.Context) {
	tid := h.tenantOf(c)
	slot := func(onKey, unitKey string) gin.H {
		return gin.H{"on": GetString(tid, onKey) == "1", "unit": GetString(tid, unitKey)}
	}
	reward := slot(KeyAdRewardCoinOn, KeyAdRewardCoinUnit)
	reward["coins"] = GetInt64(tid, KeyAdRewardCoins)
	reward["daily"] = GetInt(tid, KeyAdRewardDaily)
	response.OK(c, gin.H{
		"enabled":       GetString(tid, KeyAdEnabled) == "1",
		"inter_gap_sec": GetInt(tid, KeyAdInterGapSec),
		"slots": gin.H{
			"banner_ocean":      slot(KeyAdBannerOceanOn, KeyAdBannerOceanUnit),
			"banner_city":       slot(KeyAdBannerCityOn, KeyAdBannerCityUnit),
			"banner_expand":     slot(KeyAdBannerExpandOn, KeyAdBannerExpandUnit),
			"banner_message":    slot(KeyAdBannerMessageOn, KeyAdBannerMessageUnit),
			"banner_mine":       slot(KeyAdBannerMineOn, KeyAdBannerMineUnit),
			"banner_detail":     slot(KeyAdBannerDetailOn, KeyAdBannerDetailUnit),
			"banner_chat":       slot(KeyAdBannerChatOn, KeyAdBannerChatUnit),
			"banner_collection": slot(KeyAdBannerCollectionOn, KeyAdBannerCollectionUnit),
			"banner_orders":     slot(KeyAdBannerOrdersOn, KeyAdBannerOrdersUnit),
			"banner_walletlog":  slot(KeyAdBannerWalletlogOn, KeyAdBannerWalletlogUnit),
			"banner_viewed":     slot(KeyAdBannerViewedOn, KeyAdBannerViewedUnit),
			"inter_scoop":       slot(KeyAdInterScoopOn, KeyAdInterScoopUnit),
			"inter_detail":      slot(KeyAdInterDetailOn, KeyAdInterDetailUnit),
			"reward_coin":       reward,
			"grid_mine":         slot(KeyAdGridMineOn, KeyAdGridMineUnit),
		},
	})
}
```

- [ ] **Step 2: 构建验证**

Run: `cd server && go build ./... && go vet ./...`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
cd server && git add internal/sysconfig/handler.go
git commit -m "feat(ad): GET /api/ads 按租户下发广告配置"
```

---

### Task B4: 客户端广告基础设施

**Files:**
- Modify: `client/src/api/index.js`
- Create: `client/src/store/ads.js`
- Create: `client/src/components/ad-slot/ad-slot.vue`
- Create: `client/src/utils/ad.js`

**Interfaces:**
- Produces: `adApi.config()`/`adApi.reward()`；`useAdsStore()` 含 `fetchConfig()`、`canShow(slotKey)`、`slot(slotKey)`；`<ad-slot slot-key type>`；`utils/ad.js` `showInterstitial(slotKey)`、`showRewarded(slotKey)`。

- [ ] **Step 1: api.js**

`api/index.js` 追加：
```js
export const adApi = {
  config: () => get('/ads'),
  reward: () => post('/ad/reward', {}, { showError: false })
}
```

- [ ] **Step 2: store/ads.js**

```js
import { defineStore } from 'pinia'
import { adApi } from '../api/index'

export const useAdsStore = defineStore('ads', {
  state: () => ({ enabled: false, interGapSec: 180, slots: {}, loaded: false }),
  actions: {
    async fetchConfig() {
      try {
        const r = await adApi.config()
        if (r) {
          this.enabled = !!r.enabled
          this.interGapSec = r.inter_gap_sec || 180
          this.slots = r.slots || {}
          this.loaded = true
        }
      } catch (e) {}
    },
    slot(key) { return this.slots[key] || { on: false, unit: '' } },
    canShow(key) { const s = this.slot(key); return this.enabled && s.on && !!s.unit }
  }
})
```

- [ ] **Step 3: components/ad-slot/ad-slot.vue**

```html
<template>
  <ad v-if="visible" :unit-id="unit" :ad-type="type" ad-theme="white" @error="onErr" />
</template>
<script>
import { useAdsStore } from '../../store/ads'
export default {
  props: { slotKey: { type: String, required: true }, type: { type: String, default: 'banner' } },
  data() { return { err: false } },
  computed: {
    visible() { return !this.err && useAdsStore().canShow(this.slotKey) },
    unit() { return useAdsStore().slot(this.slotKey).unit }
  },
  methods: { onErr() { this.err = true } }
}
</script>
```

- [ ] **Step 4: utils/ad.js**

```js
import { useAdsStore } from '../store/ads'
import { adApi } from '../api/index'
import { useWalletStore } from '../store/wallet'

let _rewarded = null

export function showInterstitial(slotKey) {
  const ads = useAdsStore()
  if (!ads.canShow(slotKey)) return
  const last = uni.getStorageSync('ad_inter_last') || 0
  if (Date.now() - last < (ads.interGapSec || 180) * 1000) return
  try {
    const ad = wx.createInterstitialAd({ adUnitId: ads.slot(slotKey).unit })
    ad.onError(() => {})
    ad.show().catch(() => ad.load().then(() => ad.show()).catch(() => {}))
    uni.setStorageSync('ad_inter_last', Date.now())
  } catch (e) {}
}

export function showRewarded(slotKey) {
  const ads = useAdsStore()
  if (!ads.canShow(slotKey)) { uni.showToast({ title: '暂不可用', icon: 'none' }); return }
  try {
    if (!_rewarded) {
      _rewarded = wx.createRewardedVideoAd({ adUnitId: ads.slot(slotKey).unit })
      _rewarded.onError(() => {})
      _rewarded.onClose((res) => {
        if (res && res.isEnded) {
          adApi.reward().then((r) => {
            if (r && r.rewarded) { uni.showToast({ title: `+${r.coins} 金币`, icon: 'none' }); useWalletStore().fetchBalance() }
            else uni.showToast({ title: '今日已领完', icon: 'none' })
          }).catch(() => {})
        }
      })
    }
    _rewarded.show().catch(() => _rewarded.load().then(() => _rewarded.show()).catch(() => {}))
  } catch (e) {}
}
```

- [ ] **Step 5: 构建验证**

Run: `cd client && npm run build:mp-weixin`（PowerShell）
Expected: `Build complete`，无报错。

- [ ] **Step 6: Commit**

```bash
git add client/src/api/index.js client/src/store/ads.js client/src/components/ad-slot/ client/src/utils/ad.js
git commit -m "feat(client): 广告基础设施(store/ad-slot/utils)"
```

---

### Task B5: 客户端各页接入广告

**Files:**
- Modify: `client/src/pages/ocean/ocean.vue`、`city/city.vue`、`expand/expand.vue`、`message/message.vue`、`mine/mine.vue`、`detail/detail.vue`、`chat/chat.vue`、`collection/collection.vue`、`orders/orders.vue`、`wallet-log/wallet-log.vue`、`viewed/viewed.vue`
- 在需要的页 onShow/onLoad 调 `useAdsStore().fetchConfig()`（首次）

**Interfaces:**
- Consumes: `<ad-slot>`、`showInterstitial`、`showRewarded`、`useAdsStore`。

- [ ] **Step 1: 全局组件注册 + 配置预拉**

在 `App.vue` `onLaunch` 调 `useAdsStore().fetchConfig()` 预拉一次（失败静默）。`ad-slot` 用到的页面各自 `import AdSlot from '@/components/ad-slot/ad-slot.vue'` 并在 `components` 注册（uni-app 无 easycom 约定时需手动注册；若项目已配 easycom 可省）。

- [ ] **Step 2: Banner 接入（11 页）**

各页在指定位置插入（slotKey 对应）：
- ocean.vue：tabbar 上方 `<ad-slot slot-key="banner_ocean" type="banner" />`
- city/expand/message/collection/orders/wallet-log/viewed：列表底部对应 slotKey（`banner_city`/`banner_expand`/`banner_message`/`banner_collection`/`banner_orders`/`banner_walletlog`/`banner_viewed`）
- mine.vue：宫格下方 `banner_mine`
- detail.vue：回应列表上方 `banner_detail`
- chat.vue：顶部（scroll 区上方、footer 之外）`banner_chat`

- [ ] **Step 3: 插屏接入**

- ocean.vue 捞瓶结果关闭 `closePopup()` 末尾：`import { showInterstitial } from '@/utils/ad'; showInterstitial('inter_scoop')`
- detail.vue `onLoad` 末尾：`showInterstitial('inter_detail')`

- [ ] **Step 4: 激励视频接入（我的页）**

mine.vue：在签到卡片附近加按钮，`useAdsStore().canShow('reward_coin')` 为真才显示：
```html
<view v-if="adReward" class="checkin" @tap="onWatchAd">
  <text class="ci">🎬</text><text class="ct">看视频领金币</text><text class="cb">去观看 ›</text>
</view>
```
script：`import { showRewarded } from '@/utils/ad'`；`computed adReward(){ return useAdsStore().canShow('reward_coin') }`；`methods onWatchAd(){ showRewarded('reward_coin') }`；onShow 里 `useAdsStore().fetchConfig()`。

- [ ] **Step 5: 构建验证**

Run: `cd client && npm run build:mp-weixin`
Expected: `Build complete`。

- [ ] **Step 6: 联调验证（人工）**

后台某租户开 `ad_enabled` + 某 banner 开关 + 填测试广告 unit，进对应页确认展示；关掉则消失。激励视频看完确认发币与每日上限。

- [ ] **Step 7: Commit**

```bash
git add client/src/pages/ client/src/App.vue
git commit -m "feat(client): 各页接入流量主广告(banner/插屏/激励)"
```

---

## Self-Review

**Spec coverage:**
- Part1 配置表复合主键+迁移 → A1 ✓
- Part1 sysconfig 三级回退+71 调用点+ripple+admin 按租户+公开端点按租户 → A2 ✓
- Part1 空 appid 兜底 → A3 ✓
- Part2 广告配置键+白名单 → B1 ✓
- Part2 激励发币 → B2 ✓
- Part2 /api/ads 下发 → B3 ✓
- Part2 客户端基础设施 → B4 ✓
- Part2 15 广告位接入(含聊天/二级列表) → B5 ✓

**类型一致性:** `GetString/GetInt/GetInt64/GetBool(tenantID,key)`、`Set(tenantID,key,value)` 全文一致；ripple 签名（`Status`/`EnsureVerifiedIfRequired`/`Verifier`/`Chat`/push helpers/admin `Config`/`SetConfig`/`Stats`）在 A2 定义并列出调用方；广告 slot key 串在 B1/B3/B4/B5 一致（`banner_ocean`… `grid_mine`）；`SceneAdReward="ad_reward"` ≤24。

**已知风险/说明:**
- A2 是大原子任务（编译只在末尾绿），建议 inline 执行、谨慎分步；其余任务可子代理逐个。
- push 改为按租户 token 缓存（`map[int64]tokenEntry`），跨租户推送各自取 token。
- 公开配置端点（tabs/mine-functions/notice/hook-count/ads）用「可选 JWT → 默认租户」解析，登录后即按用户租户。
- DB 绑定逻辑靠 `go build/vet` + 关键纯逻辑单测（sysconfig 回退、ad dayKey）。
