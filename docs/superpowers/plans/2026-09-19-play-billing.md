# Google Play Billing 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Android 端的金币充值走 Google Play Billing，服务端用服务账号核实票据后入账，并能发现用户事后向 Google 申请的退款。

**Architecture:** `internal/pay/iap.go` 已经把「应用内计费」这条链路完整走过一遍（验票 → 商品映射 → 事务幂等入账 → 退款扣回 → 负余额冻结），Play 侧对称照搬，只有「拿什么去核实」这一步结构不同。定价改为 `(package, platform, region)` 三维，以后台为准、Play Console 手工对齐，服务端校验实付金额不一致即拒收。退款靠定时轮询 Voided Purchases，本期不接 RTDN。

**Tech Stack:** Go 1.22 + Gin + GORM v2 + MySQL · `golang-jwt/jwt/v5`（已有）· Google Play Developer API v3（REST，手写调用）

**Spec:** `docs/superpowers/specs/2026-09-19-google-capabilities-design.md`（§三、§六）

## Global Constraints

- **新增 sysconfig key 必须同步写 `internal/sysconfig/sysconfig.go` 的 defaults**（空串会导致开关逻辑反转），并登记 `internal/admin/meta.go` 白名单。本计划新增 4 个。
- **多租户**：所有表带 `tenant_id`，所有查询按租户过滤。
- **ID 一律字符串下发**（`json` tag 加 `,string`）。
- **金额一律最小货币单位**（分 / paise / cent），不要用浮点。
- **服务端校验成功前不得让客户端 consume**：提前确认 = Google 认为已交付，钱扣了币没到，且该交易再也查不回来。同 `pay/iap.go:175` 对 Apple 的告诫。
- **提交前验证**：`cd server && go build ./... && go vet ./...`
- **跑测试绕开已知 flaky 包**：`go test $(go list ./... | grep -v /internal/robot)`。
- 回复用中文；代码 / 标识符 / commit message 英文（conventional commits）。

## 两个实施前就定下的技术决定

**一、不引入 `golang.org/x/oauth2/google`，手写服务账号取 token。**

`go.mod` 里目前没有任何 Google/oauth2 依赖。引入 `oauth2/google` 会拉进一串传递依赖，而我们需要的只有两件事：用服务账号私钥签一个 RS256 JWT、拿它换 access token。`iap.go` 给 Apple 签 ES256 JWT 就是手写的（`appStoreToken`），照同样的路子写 Play 大约 60 行，只用已有的 `golang-jwt/jwt/v5` 和标准库。

> 若后续要接更多 Google 服务（比如上传构建、Play 商家报表），再引入官方客户端不迟。**本期一个 REST 端点不值得一棵依赖树。**

**二、`PriceFen` 改名 `PriceMinor`（spec 决策 19）放在本计划里做。**

它是支付域的字段，改名要连带改 `overview.go`、`admin/handler.go`、后台前端和 `bootstrap.Seed`。放在这里一次改完，比散在别处安全。

---

## File Structure

| 文件 | 职责 | 动作 |
|---|---|---|
| `server/internal/model/model.go` | `PayOrder` 改字段、`CoinPackage` 加列、两张新表 | 修改 |
| `server/internal/pay/playauth.go` | **新增**：服务账号取 token + 调 Play API | 创建 |
| `server/internal/pay/playauth_test.go` | **新增**：token 缓存与过期的纯函数测试 | 创建 |
| `server/internal/pay/play.go` | **新增**：验票入账、退款扣回 | 创建 |
| `server/internal/pay/play_test.go` | **新增**：价格选取与状态判定的纯函数测试 | 创建 |
| `server/internal/pay/playcron.go` | **新增**：Voided Purchases 轮询 | 创建 |
| `server/internal/pay/handler.go` | 新路由 | 修改 |
| `server/internal/pay/iap.go` | 跟随 `PriceFen` 改名 | 修改 |
| `server/internal/sysconfig/sysconfig.go` | 4 个新 key + defaults | 修改 |
| `server/internal/admin/meta.go` | 后台白名单 | 修改 |
| `server/internal/admin/handler.go` | 档位管理带 `play_product_id` 与多地区价 | 修改 |
| `server/internal/admin/overview.go` | 跟随 `PriceFen` 改名 | 修改 |
| `server/internal/bootstrap/migrate.go` | Seed 跟随改名 | 修改 |
| `server/cmd/api/main.go` | 挂 Voided Purchases 定时任务 | 修改 |

`playauth.go` 与 `play.go` 分开：前者是「怎么跟 Google 说话」（凭证、token、HTTP），后者是「说完之后怎么记账」。`iap.go` 把这两件事放在一个文件里已经 367 行，不必复制这个问题。

---

## Task 1: 数据模型与 `PriceFen` 改名

**Files:**
- Modify: `server/internal/model/model.go`
- Modify: `server/internal/pay/iap.go` · `server/internal/admin/overview.go` · `server/internal/admin/handler.go` · `server/internal/bootstrap/migrate.go`
- Test: `server/internal/pay/play_test.go`（创建）

**Interfaces:**
- Consumes: 无
- Produces:
  - `model.PayOrder.PriceMinor int64` · `model.PayOrder.Currency string`
  - `model.CoinPackage.PlayProductID string`
  - `model.CoinPackagePrice` · `model.PlayPurchase`
  - `pay.pickPrice(prices []model.CoinPackagePrice, platform, region string) *model.CoinPackagePrice`

- [x] **Step 1: 写失败的测试**

创建 `server/internal/pay/play_test.go`：

```go
package pay

import (
	"testing"

	"driftbottle/internal/model"
)

// pickPrice 从多地区价目里选出该平台该地区的价格。
//
// 「后台为准 + 留多市场结构」这条决策的落点。三条规则：
//   - 精确命中 (platform, region) 优先
//   - 命中不到就落该平台的 "*" 兜底行
//   - 平台不匹配的行**绝不能选**——iOS 与 Android 定价本就不同
//     (两家抽成都要在定价里吃掉),选错等于按错误的价格校验实付金额
func TestPickPrice(t *testing.T) {
	prices := []model.CoinPackagePrice{
		{PackageID: 3, Platform: "gplay", Region: "IN", Currency: "INR", Amount: 19900},
		{PackageID: 3, Platform: "gplay", Region: "*", Currency: "USD", Amount: 299},
		{PackageID: 3, Platform: "ios", Region: "IN", Currency: "INR", Amount: 24900},
	}

	t.Run("精确命中地区", func(t *testing.T) {
		got := pickPrice(prices, "gplay", "IN")
		if got == nil || got.Amount != 19900 || got.Currency != "INR" {
			t.Fatalf("应命中 gplay/IN, 实际 %+v", got)
		}
	})

	t.Run("未命中地区时落 * 兜底", func(t *testing.T) {
		got := pickPrice(prices, "gplay", "US")
		if got == nil || got.Amount != 299 || got.Currency != "USD" {
			t.Fatalf("应落 gplay/*, 实际 %+v", got)
		}
	})

	t.Run("不同平台同地区必须各取各的", func(t *testing.T) {
		g := pickPrice(prices, "gplay", "IN")
		i := pickPrice(prices, "ios", "IN")
		if g == nil || i == nil {
			t.Fatal("两个平台都应命中")
		}
		if g.Amount == i.Amount {
			t.Errorf("两端定价本就不同,不应取到同一个值: %d", g.Amount)
		}
	})

	t.Run("平台没有任何价目时返回 nil，不要回退到别的平台", func(t *testing.T) {
		if got := pickPrice(prices, "wx", "IN"); got != nil {
			t.Errorf("wx 没有价目应返回 nil, 实际 %+v", got)
		}
	})

	t.Run("空价目表返回 nil", func(t *testing.T) {
		if got := pickPrice(nil, "gplay", "IN"); got != nil {
			t.Errorf("应返回 nil, 实际 %+v", got)
		}
	})
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/pay/ -run TestPickPrice -v`
Expected: 编译失败，`undefined: pickPrice`

- [x] **Step 3: 改 `PayOrder`**

`server/internal/model/model.go` 的 `PayOrder`：

```go
	// Platform 从 size:8 加宽:塞 google_play(11 字符) 会被**静默截断**。
	// 实际用短码 gplay,加宽是为了以后再加渠道时不用再改一次。
	Platform string `gorm:"size:16" json:"platform"` // wx/alipay/ios/gplay
	// PriceMinor 原名 PriceFen。改名是因为语义不再是「分」——
	// Play/App Store 结算的是各国本地货币,单位是该货币的最小单位
	// (INR 的 paise、USD 的 cent)。留着 Fen 这个名字,
	// 下一个人会理所当然地按人民币去算,而且**错得很安静**。
	PriceMinor int64  `gorm:"column:price_fen" json:"price_minor"`
	Currency   string `gorm:"size:8" json:"currency"` // INR/USD/CNY…空=CNY(历史数据)
```

> `column:price_fen` 保留原列名：改列名要写数据迁移，而列名本身不会误导人——
> 误导人的是 Go 侧的字段名和 JSON 字段名。**这是刻意的取舍，别顺手把列名也改了。**

- [x] **Step 4: 全仓跟随改名**

```bash
cd server && grep -rn "PriceFen" --include=*.go .
```

逐个改成 `PriceMinor`：`internal/pay/iap.go`、`internal/admin/overview.go`、`internal/admin/handler.go`、`internal/bootstrap/migrate.go`（`Seed` 里 6 个档位）。

`overview.go` 里注释写「已支付订单 price_fen/100」的地方，改成说明**该口径只对 CNY 成立**，跨币种统计要按 `currency` 分组——本期不实现分组，但要把这个已知缺陷写在注释里，不要让它无声存在。

> 后台前端也引用了 `price_fen` 这个 JSON 字段名。`admin/` 下搜一遍：
> `grep -rn "price_fen" admin/src/`，同步改成 `price_minor`。**漏改会让充值档位页的价格显示空白。**

- [x] **Step 5: `CoinPackage` 加列**

```go
	// PlayProductID Google Play Console 里配置的商品 ID。与 IOSProductID 对称。
	// 商品 ID 是**全局唯一、多国共用一个**的,随国家变的只是价格——
	// 所以它属于档位,价格才属于 CoinPackagePrice。
	PlayProductID string `gorm:"size:64;index" json:"play_product_id"`
```

- [x] **Step 6: 两张新表**

在 `CoinPackage` 之后加：

```go
// CoinPackagePrice 档位的多平台多地区定价。
//
// 为什么要 platform 维度:iap.go 的注释写过「两端定价本就不同(苹果 30% 抽成
// 要在定价里吃掉)」。为什么要 region 维度:印度先上,但结构要能容纳多市场。
// Region 用 ISO 国家码,"*" 为兜底行。
type CoinPackagePrice struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id,string"`
	PackageID int64  `gorm:"uniqueIndex:uk_pkg_plat_region,priority:1" json:"package_id"`
	Platform  string `gorm:"size:8;uniqueIndex:uk_pkg_plat_region,priority:2" json:"platform"` // gplay/ios
	Region    string `gorm:"size:8;uniqueIndex:uk_pkg_plat_region,priority:3" json:"region"`   // IN/US/*
	Currency  string `gorm:"size:8" json:"currency"`
	Amount    int64  `json:"amount"` // 最小货币单位
}

// PlayPurchase Google Play 购买记录,对称 IAPTransaction。
type PlayPurchase struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id,string"`
	TenantID      int64      `gorm:"index" json:"tenant_id,string"`
	UserID        int64      `gorm:"index" json:"user_id,string"`
	OrderID       string     `gorm:"size:64;uniqueIndex" json:"order_id"` // GPA.xxxx,幂等键
	PurchaseToken string     `gorm:"size:512;index" json:"-"`             // 不下发:凭它可查询购买详情
	ProductID     string     `gorm:"size:64" json:"product_id"`
	OrderNo       string     `gorm:"size:32;index" json:"order_no"` // 关联 PayOrder
	Coins         int64      `json:"coins"`
	State         string     `gorm:"size:16" json:"state"` // pending/purchased/refunded
	Currency      string     `gorm:"size:8" json:"currency"`
	AmountMinor   int64      `json:"amount_minor"` // Play 回传的实付,留档对账用
	AckedAt       *time.Time `json:"acked_at"`
	RefundedAt    *time.Time `json:"refunded_at"`
	CreatedAt     time.Time  `json:"created_at"`
}
```

两张表都要加进 `model.AllModels()`。

- [x] **Step 7: 实现 `pickPrice`**

创建 `server/internal/pay/play.go`，先只放这个函数：

```go
package pay

import "driftbottle/internal/model"

// pickPrice 选出该平台该地区的价格：精确命中优先，落不到则取该平台的 "*" 兜底。
//
// **绝不跨平台回退**：iOS 与 Android 的定价本就不同(两家的抽成都要在定价里吃掉)，
// 拿 iOS 的价去校验 Play 的实付金额，结果必然是「金额不一致」而拒收所有正常购买。
func pickPrice(prices []model.CoinPackagePrice, platform, region string) *model.CoinPackagePrice {
	var fallback *model.CoinPackagePrice
	for i := range prices {
		p := &prices[i]
		if p.Platform != platform {
			continue
		}
		if p.Region == region {
			return p
		}
		if p.Region == "*" {
			fallback = p
		}
	}
	return fallback
}
```

- [x] **Step 8: 跑测试确认通过**

Run: `cd server && go test ./internal/pay/ -run TestPickPrice -v`
Expected: PASS（5 个子测试）

- [x] **Step 9: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
grep -rn "PriceFen" --include=*.go . && echo "还有残留!" || echo "改名干净"
git add server/internal/model/ server/internal/pay/ server/internal/admin/ server/internal/bootstrap/ admin/src/
git commit -m "feat(pay): add play billing tables, rename PriceFen to PriceMinor"
```

---

## Task 2: 服务账号取 token 与调 Play API

**Files:**
- Create: `server/internal/pay/playauth.go` · `server/internal/pay/playauth_test.go`
- Modify: `server/internal/sysconfig/sysconfig.go` · `server/internal/admin/meta.go`

**Interfaces:**
- Consumes: Task 1 的模型
- Produces:
  - `sysconfig.KeyAppPlayEnabled` / `KeyAppPlayPackageName` / `KeyAppPlaySAJSONEnc` / `KeyAppPlayAllowTest`
  - `pay.playToken` 缓存结构与 `func (c *playTokenCache) valid(now time.Time) bool`
  - `func (s *Service) playAccessToken(tenantID int64) (string, error)`
  - `func (s *Service) fetchPlayPurchase(tenantID int64, productID, token string) (*playPurchase, error)`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/pay/playauth_test.go`：

```go
package pay

import (
	"testing"
	"time"
)

// access token 要缓存：每次验票都去换一次 token，等于把一次购买变成两次跨境往返，
// 而 token 有效期是一小时。缓存的唯一风险是「用了已过期的」，所以留足安全边际。
func TestPlayTokenCacheValidity(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		cache playTokenCache
		want  bool
	}{
		{"空 token 无效", playTokenCache{token: "", expiry: now.Add(time.Hour)}, false},
		{"已过期无效", playTokenCache{token: "x", expiry: now.Add(-time.Second)}, false},
		{"刚好到期无效", playTokenCache{token: "x", expiry: now}, false},
		{"还剩 30 秒:视为无效(安全边际内)", playTokenCache{token: "x", expiry: now.Add(30 * time.Second)}, false},
		{"还剩 10 分钟有效", playTokenCache{token: "x", expiry: now.Add(10 * time.Minute)}, true},
	}
	for _, c := range cases {
		if got := c.cache.valid(now); got != c.want {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.want, got)
		}
	}
}

// purchaseState / purchaseType 的判定。
//
// 这两个字段决定「能不能发币」，判错的后果是白送或者漏发：
//   - purchaseState 0=已购买 1=已取消 2=待处理(PENDING)
//   - purchaseType 存在且为 0 表示**测试单**(正式单该字段缺省)
func TestPlayPurchaseStateChecks(t *testing.T) {
	zero := 0
	cases := []struct {
		name          string
		p             playPurchase
		wantPurchased bool
		wantPending   bool
		wantTest      bool
	}{
		{"正式已购买", playPurchase{PurchaseState: 0}, true, false, false},
		{"已取消", playPurchase{PurchaseState: 1}, false, false, false},
		{"待处理(印度 UPI 延迟付款)", playPurchase{PurchaseState: 2}, false, true, false},
		{"测试单", playPurchase{PurchaseState: 0, PurchaseType: &zero}, true, false, true},
	}
	for _, c := range cases {
		if got := c.p.purchased(); got != c.wantPurchased {
			t.Errorf("%s: purchased 期望 %v 实际 %v", c.name, c.wantPurchased, got)
		}
		if got := c.p.pending(); got != c.wantPending {
			t.Errorf("%s: pending 期望 %v 实际 %v", c.name, c.wantPending, got)
		}
		if got := c.p.isTest(); got != c.wantTest {
			t.Errorf("%s: isTest 期望 %v 实际 %v", c.name, c.wantTest, got)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/pay/ -run "TestPlayToken|TestPlayPurchaseState" -v`
Expected: 编译失败，`undefined: playTokenCache` / `undefined: playPurchase`

- [ ] **Step 3: 实现**

创建 `server/internal/pay/playauth.go`：

```go
package pay

// Google Play Developer API 的凭证与调用。
//
// 与 iap.go 的 Apple 侧最大的结构不同：Apple 的 JWS 自带交易信息，可以先解出来
// 再向苹果核实；Play 给的 purchaseToken 是个不透明串，**不查 Google 就什么都不知道**。
// 所以这里没有「拿不到 API 凭证就退而信客户端」的兜底分支——未配服务账号只能直接拒绝。
//
// 为什么手写而不用 golang.org/x/oauth2/google：需要的只有「用服务账号私钥签 JWT
// 换 access token」这一件事，而引入官方客户端会拉进一串传递依赖。
// iap.go 给 Apple 签 ES256 JWT 也是手写的，保持一致。

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"driftbottle/internal/crypto"
	"driftbottle/internal/sysconfig"

	"github.com/golang-jwt/jwt/v5"
)

const (
	playTokenURL = "https://oauth2.googleapis.com/token"
	playAPIBase  = "https://androidpublisher.googleapis.com/androidpublisher/v3"
	playScope    = "https://www.googleapis.com/auth/androidpublisher"
)

// playHTTPClient 超时给足：跨境请求。与 iapHTTPClient 同理。
var playHTTPClient = &http.Client{Timeout: 20 * time.Second}

// serviceAccount 后台粘贴的服务账号 JSON 里我们用得到的字段。
type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// playTokenCache access token 缓存。
//
// token 有效期一小时。不缓存的话每次验票都要多一次跨境往返，
// 把一次购买变成两次网络调用。
type playTokenCache struct {
	token  string
	expiry time.Time
}

// tokenSafetyMargin 提前多久认为 token 失效。
// 取 60 秒：够覆盖一次 API 调用的往返，避免「查询发出时还有效、到达时已过期」。
const tokenSafetyMargin = 60 * time.Second

func (c playTokenCache) valid(now time.Time) bool {
	return c.token != "" && c.expiry.After(now.Add(tokenSafetyMargin))
}

var (
	playTokenMu    sync.Mutex
	playTokenByTen = map[int64]playTokenCache{}
)

// playAccessToken 取（或刷新）该租户的 Play API access token。
func (s *Service) playAccessToken(tenantID int64) (string, error) {
	playTokenMu.Lock()
	defer playTokenMu.Unlock()
	if c, ok := playTokenByTen[tenantID]; ok && c.valid(time.Now()) {
		return c.token, nil
	}

	raw := sysconfig.GetString(tenantID, sysconfig.KeyAppPlaySAJSONEnc)
	if raw == "" {
		return "", errors.New("未配置 Play 服务账号")
	}
	plain, err := crypto.Decrypt(raw)
	if err != nil {
		return "", fmt.Errorf("解密服务账号 JSON 失败: %w", err)
	}
	var sa serviceAccount
	if err := json.Unmarshal([]byte(plain), &sa); err != nil {
		return "", fmt.Errorf("服务账号 JSON 格式不正确: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return "", errors.New("服务账号 JSON 缺少 client_email 或 private_key")
	}

	key, err := parseRSAPrivateKey(sa.PrivateKey)
	if err != nil {
		return "", err
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = playTokenURL
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   sa.ClientEmail,
		"scope": playScope,
		"aud":   tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", fmt.Errorf("签发服务账号 JWT 失败: %w", err)
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signed},
	}
	resp, err := playHTTPClient.PostForm(tokenURI, form)
	if err != nil {
		return "", fmt.Errorf("换取 Play access token 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// 401 往往不是密钥错,而是**服务账号没在 Play Console 里授权**——
		// 这一步很容易漏,错误信息里点明,省得排查时只看到一个裸的 401。
		return "", fmt.Errorf("换取 Play access token 返回 %d: %s"+
			"(若为 401,先确认该服务账号已在 Play Console→用户和权限里授予「查看财务数据」与「管理订单」)",
			resp.StatusCode, string(body))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", errors.New("Play access token 响应格式不正确")
	}
	playTokenByTen[tenantID] = playTokenCache{
		token:  tr.AccessToken,
		expiry: now.Add(time.Duration(tr.ExpiresIn) * time.Second),
	}
	return tr.AccessToken, nil
}

// parseRSAPrivateKey 解析服务账号 JSON 里的 PEM 私钥。
// JSON 里的换行是 \n 转义，json.Unmarshal 已经还原过，这里直接 PEM 解码即可。
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if block == nil {
		return nil, errors.New("服务账号私钥 PEM 格式不正确")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析服务账号私钥失败: %w", err)
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("服务账号私钥不是 RSA")
	}
	return rk, nil
}

// playPurchase purchases.products.get 的响应（只取用得到的字段）。
type playPurchase struct {
	OrderID          string `json:"orderId"`
	PurchaseState    int    `json:"purchaseState"`    // 0已购买 1已取消 2待处理
	ConsumptionState int    `json:"consumptionState"` // 0未消耗 1已消耗
	AckState         int    `json:"acknowledgementState"`
	PurchaseTimeMs   string `json:"purchaseTimeMillis"`
	RegionCode       string `json:"regionCode"`
	// PurchaseType 存在且为 0 表示测试单；正式购买**该字段缺省**，
	// 所以必须用指针区分「没有这个字段」与「值是 0」。
	PurchaseType *int `json:"purchaseType"`
}

func (p playPurchase) purchased() bool { return p.PurchaseState == 0 }
func (p playPurchase) pending() bool   { return p.PurchaseState == 2 }

// isTest 是否测试购买。
//
// ⚠️ Play 的测试单 orderId 与正式单**长得一模一样**，只能靠这个字段区分。
// 与 Apple 靠 Environment=Sandbox 不同，容易漏判，漏了就是白送币。
func (p playPurchase) isTest() bool { return p.PurchaseType != nil && *p.PurchaseType == 0 }

// fetchPlayPurchase 向 Google 核实一笔购买。
func (s *Service) fetchPlayPurchase(tenantID int64, productID, purchaseToken string) (*playPurchase, error) {
	pkgName := sysconfig.GetString(tenantID, sysconfig.KeyAppPlayPackageName)
	if pkgName == "" {
		return nil, errors.New("未配置 Play 包名")
	}
	tok, err := s.playAccessToken(tenantID)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/applications/%s/purchases/products/%s/tokens/%s",
		playAPIBase, url.PathEscape(pkgName), url.PathEscape(productID), url.PathEscape(purchaseToken))
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := playHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 Play 购买失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询 Play 购买返回 %d: %s", resp.StatusCode, string(body))
	}
	var p playPurchase
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("Play 购买响应格式不正确: %w", err)
	}
	return &p, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/pay/ -run "TestPlayToken|TestPlayPurchaseState" -v`
Expected: PASS（两个测试函数，共 9 个断言）

- [ ] **Step 5: 加 4 个 sysconfig key 与 defaults**

`server/internal/sysconfig/sysconfig.go`，在 IAP 那一组之后加：

```go
	// Google Play Billing
	KeyAppPlayEnabled     = "app_play_enabled"      // 总开关 (0/1)
	KeyAppPlayPackageName = "app_play_package_name" // com.ambertu.bottles
	KeyAppPlaySAJSONEnc   = "app_play_sa_json"      // 服务账号 JSON 全文(AES-GCM 密文)
	KeyAppPlayAllowTest   = "app_play_allow_test"   // 允许测试单入账 (0/1),对应 app_iap_sandbox
```

defaults 里加（**四个都要写，空串会让开关逻辑反转**）：

```go
	KeyAppPlayEnabled:     "0",
	KeyAppPlayPackageName: "",
	KeyAppPlaySAJSONEnc:   "",
	KeyAppPlayAllowTest:   "0",
```

- [ ] **Step 6: 登记后台白名单**

`server/internal/admin/meta.go`，在「App 支付」组里加：

```go
	{Key: sysconfig.KeyAppPlayEnabled, Label: "Google Play 内购总开关", Group: "App 支付", Type: "bool"},
	{Key: sysconfig.KeyAppPlayPackageName, Label: "Play 应用包名", Group: "App 支付", Type: "text"},
	{Key: sysconfig.KeyAppPlaySAJSONEnc, Label: "Play 服务账号 JSON(加密存储,粘贴全文)", Group: "App 支付", Type: "textarea"},
	{Key: sysconfig.KeyAppPlayAllowTest, Label: "允许测试单入账(上线前关闭)", Group: "App 支付", Type: "bool"},
```

- [ ] **Step 7: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/internal/pay/playauth.go server/internal/pay/playauth_test.go \
        server/internal/sysconfig/ server/internal/admin/meta.go
git commit -m "feat(pay): authenticate to Play Developer API with a service account"
```

---

## Task 3: 验票入账

**Files:**
- Modify: `server/internal/pay/play.go`
- Modify: `server/internal/pay/handler.go`
- Test: `server/internal/pay/play_test.go`

**Interfaces:**
- Consumes: Task 1 的 `pickPrice` 与模型；Task 2 的 `fetchPlayPurchase`
- Produces:
  - `func (s *Service) VerifyPlay(tenantID, userID int64, productID, purchaseToken string) (*model.PlayPurchase, error)`
  - `POST /api/pay/play/verify`

- [ ] **Step 1: 写失败的测试**

追加到 `server/internal/pay/play_test.go`：

```go
// 实付金额与后台档位价的比对。
//
// 「后台为准 + Play Console 手工对齐」这条决策的安全阀。手工对齐必然会有对不上的时候
// (改价漏同步、汇率调整),这时**必须拒收并告警,不能照发金币**——
// 照发等于把定价权交给了谁手最快。
func TestPriceMatches(t *testing.T) {
	cases := []struct {
		name                   string
		wantMinor, gotMinor    int64
		wantCur, gotCur        string
		ok                     bool
	}{
		{"完全一致", 19900, 19900, "INR", "INR", true},
		{"金额不一致", 19900, 9900, "INR", "INR", false},
		{"币种不一致", 19900, 19900, "INR", "USD", false},
		{"Play 未回传金额(0)时不拦截", 19900, 0, "INR", "", true},
		{"币种大小写不敏感", 19900, 19900, "inr", "INR", true},
	}
	for _, c := range cases {
		if got := priceMatches(c.wantMinor, c.wantCur, c.gotMinor, c.gotCur); got != c.ok {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.ok, got)
		}
	}
}
```

> 「Play 未回传金额时不拦截」是刻意的：`purchases.products.get` 在部分场景下不返回
> 价格字段。没有金额就没法比对，此时应当**放行并记录**，而不是把所有正常购买挡在门外。
> 这是可用性与严格性之间的取舍，写在测试里以免以后被人「顺手改严」。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/pay/ -run TestPriceMatches -v`
Expected: 编译失败，`undefined: priceMatches`

- [ ] **Step 3: 实现比对函数**

追加到 `play.go`：

```go
// priceMatches 后台档位价与 Play 回传实付的比对。
//
// gotMinor 为 0 表示 Play 没回传金额(部分场景确实不返回)——此时无从比对，放行。
// 币种比较忽略大小写。
func priceMatches(wantMinor int64, wantCur string, gotMinor int64, gotCur string) bool {
	if gotMinor == 0 {
		return true
	}
	if wantMinor != gotMinor {
		return false
	}
	if gotCur != "" && !strings.EqualFold(wantCur, gotCur) {
		return false
	}
	return true
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/pay/ -run TestPriceMatches -v`
Expected: PASS（5 个用例）

- [ ] **Step 5: 实现 `VerifyPlay`**

追加到 `play.go`。结构照搬 `VerifyIAP`（`iap.go:177`）的七步，逐步对照：

```go
// VerifyPlay 校验 Play 购买票据并入账。
//
// ⚠️ 客户端必须**等本接口返回成功后才 consume**。提前 consume = Google 认为已交付，
// 用户钱扣了币没到，且这笔交易再也查不回来。与 iap.go:175 对 Apple 的告诫同理。
//
// ⚠️ Play 规定购买后 3 天内未 acknowledge 会**自动退款**，所以入账成功后
// 立刻 acknowledge(由客户端 consume 隐式完成)，失败要有重试。
func (s *Service) VerifyPlay(tenantID, userID int64, productID, purchaseToken string) (*model.PlayPurchase, error) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyAppPlayEnabled) {
		return nil, errs.New(errs.CodeForbidden, "Google Play 内购未开启")
	}
	if productID == "" || purchaseToken == "" {
		return nil, errs.New(errs.CodeBadRequest, "缺少商品或票据")
	}

	// 没有「信客户端」的兜底分支:purchaseToken 是不透明串,不查 Google 什么都不知道。
	p, err := s.fetchPlayPurchase(tenantID, productID, purchaseToken)
	if err != nil {
		log.Printf("[play] 核实购买失败 tenant=%d product=%s: %v", tenantID, productID, err)
		return nil, errs.New(errs.CodePaySignError, "交易校验失败")
	}
	if p.pending() {
		// 印度的 UPI 延迟付款/话费代扣会停在这里,可能几小时到几天。
		// **合法中间态,不是掉单**,更不能发币。
		return nil, errs.New(errs.CodeOrderNotFound, "支付确认中，完成后会自动到账")
	}
	if !p.purchased() {
		return nil, errs.New(errs.CodeForbidden, "该交易未完成支付")
	}
	if p.isTest() && !sysconfig.GetBool(tenantID, sysconfig.KeyAppPlayAllowTest) {
		return nil, errs.New(errs.CodeForbidden, "测试交易不予入账")
	}
	if p.OrderID == "" {
		return nil, errs.New(errs.CodePaySignError, "交易缺少 orderId")
	}

	var pkg model.CoinPackage
	if err := s.db.First(&pkg, "play_product_id = ? AND status = ?", productID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "未知的内购商品:"+productID)
	}

	// 价格校验:后台为准,不一致拒收并告警(决策 11 的配套)
	var prices []model.CoinPackagePrice
	s.db.Find(&prices, "package_id = ?", pkg.PackageID)
	want := pickPrice(prices, "gplay", p.RegionCode)
	if want == nil {
		log.Printf("[play] 档位无 gplay 价目 package=%d region=%s", pkg.PackageID, p.RegionCode)
		return nil, errs.New(errs.CodeServerError, "该商品暂未配置价格")
	}
	if !priceMatches(want.Amount, want.Currency, 0, "") {
		// purchases.products.get 不回传金额,这里恒为 true;
		// 留着这个调用是为了将来换用会回传金额的接口时不必重写校验。
		log.Printf("[play] 价格校验未通过 package=%d", pkg.PackageID)
		return nil, errs.New(errs.CodeForbidden, "价格校验未通过，请联系客服")
	}

	coins := pkg.Coins + pkg.BonusCoins

	var saved model.PlayPurchase
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 幂等：同一 orderId 只入账一次。
		// App 冷启动时 queryPurchasesAsync 会把未 consume 的交易重新推上来,
		// 不幂等就会一笔钱发两次币——与 Apple 侧 transactionId 的处理一致。
		var exist model.PlayPurchase
		e := tx.First(&exist, "order_id = ?", p.OrderID).Error
		if e == nil {
			saved = exist
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}

		order := model.PayOrder{
			OrderNo:     genOrderNo(),
			TenantID:    tenantID,
			UserID:      userID,
			PackageID:   pkg.PackageID,
			Platform:    "gplay",
			PriceMinor:  want.Amount,
			Currency:    want.Currency,
			Coins:       coins,
			Status:      "paid",
			PlatformTxn: p.OrderID,
			CreatedAt:   time.Now(),
		}
		paidAt := playPurchasedAt(p.PurchaseTimeMs)
		order.PaidAt = &paidAt
		if e := tx.Create(&order).Error; e != nil {
			return e
		}

		saved = model.PlayPurchase{
			TenantID: tenantID, UserID: userID,
			OrderID: p.OrderID, PurchaseToken: purchaseToken, ProductID: productID,
			OrderNo: order.OrderNo, Coins: coins, State: "purchased",
			Currency: want.Currency, AmountMinor: want.Amount,
			CreatedAt: time.Now(),
		}
		if e := tx.Create(&saved).Error; e != nil {
			return e
		}

		if e := wallet.CreditTx(tx, tenantID, userID, coins, wallet.SceneRecharge, order.OrderNo); e != nil {
			return e
		}
		// 首充打「付费用户」标签,与微信/支付宝/IAP 链路保持一致
		return tx.Exec(
			"UPDATE users SET tags = IF(tags = '' OR tags IS NULL, ?, CONCAT(tags, ',', ?)) "+
				"WHERE user_id = ? AND (tags IS NULL OR tags NOT LIKE ?)",
			model.TagPaidUser, model.TagPaidUser, userID, "%"+model.TagPaidUser+"%",
		).Error
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// playPurchasedAt 解析 Play 的毫秒时间戳字符串。解析不出就用当前时间。
func playPurchasedAt(ms string) time.Time {
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil || n <= 0 {
		return time.Now()
	}
	return time.UnixMilli(n)
}
```

- [ ] **Step 6: 加路由**

`server/internal/pay/handler.go`，在 `iapVerify` 那行之后：

```go
	api.POST("/pay/play/verify", auth, h.playVerify)
```

handler：

```go
type playVerifyReq struct {
	ProductID     string `json:"product_id" binding:"required"`
	PurchaseToken string `json:"purchase_token" binding:"required"`
}

func (h *Handler) playVerify(c *gin.Context) {
	var req playVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	rec, err := h.svc.VerifyPlay(middleware.TenantID(c), middleware.UserID(c), req.ProductID, req.PurchaseToken)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "入账失败")
		return
	}
	response.OK(c, gin.H{"order_id": rec.OrderID, "coins": rec.Coins, "order_no": rec.OrderNo})
}
```

- [ ] **Step 7: 路由鉴权测试**

`server/cmd/api/routes_test.go` 的 `TestOAuthBindRoutesRequireAuth` 是同类检查的先例。新增一个：

```go
// TestPlayVerifyRequiresAuth 验票接口必须要求登录态。
//
// 漏挂 auth 的后果不是报错，而是 middleware.UserID(c) 取到 0，
// 金币入账到「用户 0」头上——钱扣了、币进了黑洞，且不会有任何告警。
func TestPlayVerifyRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	sentinel := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	pay.NewHandler(nil).Register(api, sentinel)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/pay/play/verify", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("验票接口必须要求登录态,期望 401,实际 %d", w.Code)
	}
}
```

跑一次并**确认它能失败**：临时去掉路由上的 `auth` 再跑，应报 `期望 401,实际 200`，然后还原。

- [ ] **Step 8: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/internal/pay/ server/cmd/api/routes_test.go
git commit -m "feat(pay): verify Play purchases and credit coins idempotently"
```

---

## Task 4: 退款发现（Voided Purchases 轮询）

**Files:**
- Create: `server/internal/pay/playcron.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Consumes: Task 2 的 `playAccessToken`；Task 3 的 `model.PlayPurchase`
- Produces:
  - `func StartVoidedPurchasesCron(svc *Service, tenantID int64)`
  - `func (s *Service) revokePlay(orderID string) error`

- [ ] **Step 1: 实现退款扣回**

创建 `server/internal/pay/playcron.go`。`revokePlay` 照搬 `revokeIAP`（`iap.go:324`）的全部决策，包括那条最重要的：

```go
// revokePlay 退款/撤销：扣回已发放的金币。
//
// 允许扣成负数——币可能已经花掉了。装作没发生只会让账永远对不平；
// 记负账 + 冻结账号，交人工跟进，这是唯一诚实的处理。(与 revokeIAP 同一原则)
func (s *Service) revokePlay(orderID string) error {
	if orderID == "" {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var rec model.PlayPurchase
		if err := tx.First(&rec, "order_id = ?", orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 没入过账,无需扣回
			}
			return err
		}
		if rec.State != "purchased" {
			return nil // 幂等
		}
		now := time.Now()
		if err := tx.Model(&model.PlayPurchase{}).
			Where("order_id = ? AND state = ?", orderID, "purchased").
			Updates(map[string]interface{}{"state": "refunded", "refunded_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PayOrder{}).
			Where("order_no = ?", rec.OrderNo).
			Update("status", "refunded").Error; err != nil {
			return err
		}
		if err := wallet.CreditTx(tx, rec.TenantID, rec.UserID, -rec.Coins,
			wallet.SceneRefund, rec.OrderNo); err != nil {
			return err
		}
		var w model.Wallet
		if err := tx.Select("balance").First(&w, "user_id = ?", rec.UserID).Error; err == nil && w.Balance < 0 {
			log.Printf("[play] 退款后余额为负 uid=%d balance=%d，已冻结账号", rec.UserID, w.Balance)
			if err := tx.Model(&model.User{}).Where("user_id = ?", rec.UserID).
				Update("status", "frozen").Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

- [ ] **Step 2: 实现轮询**

```go
// StartVoidedPurchasesCron 每小时拉一次 Voided Purchases，扣回已退款的金币。
//
// ⚠️ 为什么不是 RTDN：RTDN(Pub/Sub 实时通知)主要推**订阅**事件，
// **一次性商品的退款不在标准 RTDN 里**。金币包是一次性商品，只接 RTDN
// 等于完全感知不到退款。正确通道是轮询 purchases.voidedpurchases.list。
//
// 用户直接向 Google 申请退款时 App 端收不到任何同步信号，
// 这个轮询是唯一的发现途径(iap.go:295 对 Apple 侧写过同一句话)。
func StartVoidedPurchasesCron(svc *Service, tenantID int64) {
	go func() {
		for {
			// 启动后先等一轮再跑:避免服务重启风暴时同时打 Google。
			time.Sleep(time.Hour)
			svc.sweepVoidedPurchases(tenantID)
		}
	}()
}

// sweepVoidedPurchases 拉取最近的退款并逐笔扣回。
//
// startTime 取「上次扫到哪」的持久化位点会更严谨，但那需要再加一张表。
// 本期取**最近 7 天**的固定窗口：revokePlay 是幂等的，重复扫到已处理的记录无副作用，
// 而 7 天足以覆盖任何合理的服务中断。
func (s *Service) sweepVoidedPurchases(tenantID int64) {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyAppPlayEnabled) {
		return
	}
	pkgName := sysconfig.GetString(tenantID, sysconfig.KeyAppPlayPackageName)
	if pkgName == "" {
		return
	}
	tok, err := s.playAccessToken(tenantID)
	if err != nil {
		log.Printf("[play/cron] 取 access token 失败: %v", err)
		return
	}
	since := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	u := fmt.Sprintf("%s/applications/%s/purchases/voidedpurchases?startTime=%d",
		playAPIBase, url.PathEscape(pkgName), since)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := playHTTPClient.Do(req)
	if err != nil {
		log.Printf("[play/cron] 拉取退款列表失败: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[play/cron] 拉取退款列表返回 %d: %s", resp.StatusCode, string(body))
		return
	}
	var r struct {
		VoidedPurchases []struct {
			OrderID string `json:"orderId"`
		} `json:"voidedPurchases"`
	}
	if json.Unmarshal(body, &r) != nil {
		log.Printf("[play/cron] 退款列表格式不正确")
		return
	}
	n := 0
	for _, v := range r.VoidedPurchases {
		if err := s.revokePlay(v.OrderID); err != nil {
			log.Printf("[play/cron] 扣回失败 order=%s: %v", v.OrderID, err)
			continue
		}
		n++
	}
	log.Printf("[play/cron] 退款扫描完成 拉取=%d 处理=%d", len(r.VoidedPurchases), n)
}
```

- [ ] **Step 3: 挂到 main**

`server/cmd/api/main.go`，在 `push.StartNightCron(pushSvc)` 附近加：

```go
	pay.StartVoidedPurchasesCron(paySvc, cfg.DefaultTenantID)
```

> 与 `robot.StartOutreachCron(robotSvc, chatSvc, cfg.DefaultTenantID)` 同样只跑默认租户。
> 多租户下要遍历租户，那是另一件事，本期不做——但**要在这行加注释说明**，
> 否则新增租户后退款会无声地扫不到。

- [ ] **Step 4: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/internal/pay/playcron.go server/cmd/api/main.go
git commit -m "feat(pay): sweep voided Play purchases hourly and claw back coins"
```

> ⚠️ **本任务无法在本机验证**：需要真实的 Play Console 服务账号与一笔真实退款。
> 上线前必须在沙盒环境走一遍：测试单购买 → 入账 → 在 Play Console 发起退款 →
> 等一轮轮询 → 确认金币被扣回、订单状态变 `refunded`、余额为负时账号被冻结。

---

## Task 5: 后台档位管理

**Files:**
- Modify: `server/internal/admin/handler.go`
- Modify: `admin/src/`（充值档位页）

**Interfaces:**
- Consumes: Task 1 的 `CoinPackagePrice` / `PlayProductID`
- Produces: 档位增删改接口支持 `play_product_id` 与多地区价目

- [ ] **Step 1: 接口带上新字段**

`admin/handler.go` 的档位增删改（`:846` / `:860` 附近）已经直接绑 `model.CoinPackage`，加了 `PlayProductID` 列后自动支持，**无需改代码**——先确认这一点再动手。

价目是新表，需要新增两个接口：

```go
	g.GET("/coin-packages/:id/prices", h.packagePrices)
	g.POST("/coin-packages/:id/prices", h.savePackagePrices) // 全量覆盖该档位的价目
```

全量覆盖而非逐条增删：价目表每档最多十几行，整体提交比维护逐行状态简单得多，也不会出现「删了一半」的中间态。

- [ ] **Step 2: 前端页面**

「充值档位」页每行增加 `play_product_id` 输入框，并加一个「价格」按钮打开抽屉，抽屉里按 `platform × region` 维护价目，`*` 行置顶且不可删（兜底行必须存在）。

- [ ] **Step 3: 验证并提交**

```bash
cd server && go build ./... && go vet ./...
cd ../admin && npm run build
git add server/internal/admin/ admin/src/
git commit -m "feat(admin): manage play product ids and per-region prices"
```

---

## 实施记录（2026-09-19，Task 1 已完成）

Task 1 已实施并提交（`01e6ccb`）。**Task 2–5 未做**，等 Play Console 凭证就绪。

### ⚠️ 计划的一处错误，已在实施中纠正

计划 Task 1 Step 4 写的是「后台前端也引用了 `price_fen`，`admin/` 下搜一遍」。
**搜漏了 `client/`** —— 线上运行的微信小程序 `client/src/pages/orders/orders.vue:23`
直接读 `o.price_fen` 渲染订单金额。

按计划把 JSON 字段改成 `price_minor`，**线上小程序订单列表会立刻显示 `¥NaN`**，
而小程序发版要过微信审核，无法与后端同步上线。

**实际做法**：只改 Go 侧字段名（那才是真正误导人的地方），
**列名和 JSON 名都保留 `price_fen`**。`PriceMinor int64 \`gorm:"column:price_fen" json:"price_fen"\``。

要真正改掉 JSON 名，需要一个能与小程序发版协同的窗口，或先并行下发
`price_fen` + `price_minor` 两个字段做过渡。**这件事没做，别以为做了。**

### 另一处刻意不改

`admin/service.go` 的 `OrderRow.PriceFen` 保持原名。它由**裸 SQL `Scan`** 填充，
GORM 按列名约定映射字段，改成 `PriceMinor` 会去找 `price_minor` 列、找不到就
**静默填 0** —— 后台订单列表金额全变 0 且不报错。收益很小、风险是静默的，不值得。
两处都加了注释说明原因。

### 已完成的部分

- `PayOrder`：`Platform` 加宽到 16（`google_play` 11 字符原会被静默截断）、
  新增 `Currency`、`PriceFen` → `PriceMinor`（Go 侧）
- `CoinPackage`：新增 `PlayProductID`
- 新表 `CoinPackagePrice`（`package × platform × region` 三元唯一）与 `PlayPurchase`，
  已加进 `model.AllModels()`
- `pickPrice` 及其 5 个测试用例

`CoinPackage.PriceFen`（档位价）**未改名**，它确实仍是人民币分，且微信/支付宝链路在用。

---

## 自查

**Spec 覆盖**（对照 spec §三 3.4 与 §六）：

| Spec 要求 | 落点 |
|---|---|
| `PayOrder.Platform` 加宽 | Task 1 Step 3 |
| `PriceFen` → `PriceMinor` + `Currency` | Task 1 Step 3/4 |
| `CoinPackage.PlayProductID` | Task 1 Step 5 |
| `CoinPackagePrice`（platform × region） | Task 1 Step 6，`pickPrice` 测试 5 例 |
| `PlayPurchase` 表 | Task 1 Step 6 |
| 七步照搬六步 | Task 3 Step 5 |
| ③ 结构性不同（无信客户端兜底） | Task 3 Step 5，注释写明 |
| 测试单靠 `purchaseType` 识别 | Task 2（`isTest`，指针区分缺省与 0） |
| `PENDING` 不发币 | Task 3 Step 5 |
| consume 顺序 / 3 天自动退款 | Task 3 Step 5 注释 |
| 幂等以 `orderId` | Task 3 Step 5 |
| 价格校验不一致拒收 | Task 3（`priceMatches`，5 例） |
| 4 个 sysconfig key + defaults + meta | Task 2 Step 5/6 |
| 退款靠 Voided Purchases 轮询，不接 RTDN | Task 4 |
| 退款处置（负余额、冻结、不自动恢复） | Task 4 Step 1 |
| 后台档位页改造 | Task 5 |

**不在本计划**（spec 的遗留项）：订阅（决策 15）；RTDN / Pub/Sub（决策 14，上订阅时再补）；原型 H3/H7a/H9/H11 的 Flutter 实现（属客户端，需 Play Console 就绪后另行排期）。

**已知薄弱处**：

1. **Task 4 完全无法在本机验证**，需要真实 Play Console 与真实退款。步骤末尾列了沙盒验证清单。
2. **`priceMatches` 目前恒为 true**——`purchases.products.get` 不回传金额。校验函数与测试都写了，但实际拦不住任何东西。这是刻意保留的骨架：将来若改用会回传金额的接口（或加 Play 商家报表对账），不必重写。**不要因为「现在没用」就删掉它。**
3. **轮询只跑默认租户**，与 `robot.StartOutreachCron` 一致。多租户下新增租户的退款会扫不到，已要求在 main 里加注释。

---

## 前置条件（代码之外，必须先就绪）

1. **Play Console 建应用并配置商品**：每个金币档位建一个一次性商品（consumable），记下 product ID。
2. **服务账号授权**：Google Cloud 建服务账号 → 下载 JSON → **在 Play Console → 用户和权限里把该账号加进去**，授予「查看财务数据」与「管理订单」。光有 JSON 不授权，API 返回 401。
3. **后台填 4 个配置**：包名、服务账号 JSON 全文、总开关、测试单开关。
4. **License Testers**：Play Console 里把测试账号加进去，才能用测试单走通链路而不真扣款。
5. **应用需已上传到某个测试轨道**：Play Developer API 对从未发布过的应用会返回 404。
