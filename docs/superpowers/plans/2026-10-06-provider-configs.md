# 服务商配置（支付 / 地图 / 内容安全）独立化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把支付 / 地图 / 内容安全的服务商配置收进一张 `provider_configs` 表与三个后台页面，补齐高德、百度逆地理、支付宝内容安全与 Google Play 服务端校验，`/config` 不再出现这些项。

**Architecture:** 新包 `internal/provider`：声明式服务商 schema + 加密存储 + 带索引的缓存 + 启动时一次性迁移。支付 / 地图 / 内容安全三个域各自改为从 `provider.Store` 取配置并按服务商分派；后台新增 `/providers/:kind` 三个接口与一个通用卡片页。旧 sysconfig 键在迁移脚本搬完值后从代码里删除。

**Tech Stack:** Go 1.2x（Gin + GORM）、标准库 RSA/ECDSA JWT（`golang-jwt/v5` 已有）、Vue3 + vue-i18n 管理后台。

**Spec:** `docs/superpowers/specs/2026-10-06-provider-configs-design.md`

## Global Constraints

- 项目约定：测试 / 构建命令沿用用户授权（`go build/vet/test`、`npm run build`）；**git 提交只在用户明示时**，计划里的「提交」步骤一律跳过。
- 服务端改动必须兼容线上数据：`app_credentials` 的支付列**只读不删**；旧回调路径 `wx` / `wx_app` / `alipay_app` 继续可达；订单表 `platform` 的历史值 `wx` / `wx_app` / `alipay_app` / `ios` 通过别名表折回。
- sysconfig 新键必须进 `defaults`；`/admin/api` 新错误文案进 `internal/common/i18n/catalog.go`；admin 两个 locale 文件 key 必须一致。
- 迁移幂等：目标行已存在不覆盖；完成标记写 sysconfig 全局键 `provider_migrated=1`。
- 机密字段：列表接口只回 `"set"` / `""`，写空视为「保持不变」（与 `/config` 同规则）。
- 单选领域（地图、内容安全）同租户同领域至多一行 `active=true`，写入事务内互斥。
- `app_pay_mock_enabled` 留在 sysconfig「支付联调」分组不动。

## Review Focus

1. **百度 / 高德在直辖市返回的 `city` 是空数组而不是字符串** → 解析不能炸，退回 `province`。（Task 7 `parseAmap` / `parseBaidu` 用例带直辖市样本）
2. **微信回调的 `Wechatpay-Serial` 同时命中两个租户（同一商户号服务两个租户）** → 必须按 `(serial)` 命中后再用解密出的 `mch_id` 核对，而不是取第一个。（Task 3 `ByLookup` 测试：同 B 不同 A 两行都能各自命中，且空 A 查询返回「歧义」）
3. **Upsert 写入 `active=true` 的同时另一行也是 `active=true`（并发或旧数据）** → 事务内先 UPDATE 同领域其它行 active=false 再保存本行，`Active()` 任何时刻最多返回一行。（无 DB 测试基建，靠 Task 1 Step 5 `Upsert` 事务代码评审）
4. **Google Play 购买已退款 / 待处理（`purchaseState=1/2`）** → 不入账且返回明确错误，不能当成功。（Task 5 `playDecision` 表驱动）
5. **迁移时 `app_credentials` 里 `wx` 行与 `wx_app` 行都有商户号** → 同租户只能有一行 `pay/wechat`，取 `wx_app`（App 租户）或 `wx`（小程序租户），另一行丢弃并打日志。（Task 2 `planMigration` 用例）

---

### Task 1: `internal/provider`：模型、schema、存储与索引

**Files:**
- Modify: `server/internal/model/model.go`（`ProviderConfig` + `AllModels`）
- Create: `server/internal/provider/schema.go`
- Create: `server/internal/provider/store.go`
- Create: `server/internal/provider/schema_test.go`
- Create: `server/internal/provider/store_test.go`

**Interfaces:**
- Produces:
  - 常量 `KindPay="pay"`、`KindMap="map"`、`KindModeration="moderation"`；`PlatformBoth/PlatformMP/PlatformApp`
  - `type Field struct{ Key, LabelZh, LabelEn, Type string; Secret, Required bool; Options []string; Default string; HelpZh, HelpEn string }`
  - `type Definition struct{ Kind, Provider, LabelZh, LabelEn, Platform string; Fields []Field; LookupA, LookupB, DocURL string }`
  - `func Definitions(kind string) []Definition`、`func Find(kind, provider string) (Definition, bool)`、`func SingleActive(kind string) bool`、`func Missing(d Definition, fields map[string]string) []string`
  - `type Resolved struct{ TenantID int64; Kind, Provider string; Enabled, Active bool; Fields map[string]string }` + `func (r *Resolved) Get(k string) string`、`func (r *Resolved) Bool(k string) bool`
  - `type Store`：`New(db) *Store`、`Reload() error`、`Get(tid, kind, provider) (*Resolved, bool)`、`Active(tid, kind) (*Resolved, bool)`、`Usable(tid, kind, provider) bool`、`ByLookup(kind, provider, a, b string) (*Resolved, bool)`、`TenantType(tid) string`、`Version() uint64`、`Upsert(tid int64, kind, provider string, in UpsertInput) error`、`All() []*Resolved`
  - `type UpsertInput struct{ Enabled *bool; Active *bool; Fields map[string]string }`
  - 纯函数：`mergeFields(d Definition, old, in map[string]string) map[string]string`、`lookupValues(d Definition, fields map[string]string) (a, b string)`、`buildIndex(rows []*Resolved) (byKey, byLookup map[string]*Resolved)`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/provider/schema_test.go
package provider

import "testing"

// 每个 (kind, provider) 只能有一份定义;Lookup 字段必须真的存在;必填字段要有双语标签。
func TestDefinitionsAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range definitions {
		k := d.Kind + "/" + d.Provider
		if seen[k] {
			t.Errorf("重复定义 %s", k)
		}
		seen[k] = true
		if d.LabelZh == "" || d.LabelEn == "" {
			t.Errorf("%s 缺双语名", k)
		}
		keys := map[string]bool{}
		for _, f := range d.Fields {
			if f.Key == "" || f.LabelZh == "" || f.LabelEn == "" {
				t.Errorf("%s 字段 %q 缺标签", k, f.Key)
			}
			keys[f.Key] = true
		}
		for _, lk := range []string{d.LookupA, d.LookupB} {
			if lk != "" && !keys[lk] {
				t.Errorf("%s 的 Lookup 字段 %q 不存在", k, lk)
			}
		}
	}
	for _, kind := range []string{KindPay, KindMap, KindModeration} {
		if len(Definitions(kind)) == 0 {
			t.Errorf("领域 %s 没有任何服务商", kind)
		}
	}
	if !SingleActive(KindMap) || !SingleActive(KindModeration) || SingleActive(KindPay) {
		t.Fatal("地图 / 内容安全单选,支付多选")
	}
}

func TestMissingListsRequiredFieldsOnly(t *testing.T) {
	d, _ := Find(KindPay, "alipay")
	miss := Missing(d, map[string]string{"app_id": "2021x"})
	if len(miss) != 2 || miss[0] != "private_key" || miss[1] != "alipay_public_key" {
		t.Fatalf("got %v", miss)
	}
	if got := Missing(d, map[string]string{"app_id": "a", "private_key": "p", "alipay_public_key": "k"}); len(got) != 0 {
		t.Fatalf("齐全时不该缺: %v", got)
	}
}
```

```go
// server/internal/provider/store_test.go
package provider

import "testing"

func TestMergeFieldsKeepsSecretOnEmptyAndDropsUnknown(t *testing.T) {
	d, _ := Find(KindPay, "wechat")
	old := map[string]string{"app_id": "wx1", "apiv3_key": "OLDKEY", "mch_id": "100"}
	got := mergeFields(d, old, map[string]string{
		"app_id":    "wx2",
		"apiv3_key": "",       // 机密写空 = 保持
		"mch_id":    "",       // 非机密写空 = 清空
		"bogus":     "whatever", // 未知键丢弃
	})
	if got["app_id"] != "wx2" || got["apiv3_key"] != "OLDKEY" || got["mch_id"] != "" {
		t.Fatalf("got %+v", got)
	}
	if _, has := got["bogus"]; has {
		t.Fatal("未知键不该写入")
	}
}

func TestLookupValuesProjectSchemaKeys(t *testing.T) {
	d, _ := Find(KindPay, "wechat")
	a, b := lookupValues(d, map[string]string{"mch_id": "190", "platform_serial": "SER"})
	if a != "190" || b != "SER" {
		t.Fatalf("%s %s", a, b)
	}
	d2, _ := Find(KindMap, "amap")
	if a, b := lookupValues(d2, map[string]string{"key": "k"}); a != "" || b != "" {
		t.Fatal("地图服务商没有 lookup")
	}
}

// 同一平台证书序列号可能服务两个租户(同一商户号):按 (a,b) / (a,) / (,b) 三种键都能查,
// 但 (,b) 命中多行时必须报歧义而不是随便挑一个。
func TestBuildIndexLookupKeys(t *testing.T) {
	rows := []*Resolved{
		{TenantID: 1, Kind: KindPay, Provider: "wechat", Enabled: true, Fields: map[string]string{"mch_id": "100", "platform_serial": "SER"}},
		{TenantID: 2, Kind: KindPay, Provider: "wechat", Enabled: true, Fields: map[string]string{"mch_id": "200", "platform_serial": "SER"}},
		{TenantID: 3, Kind: KindPay, Provider: "alipay", Enabled: true, Fields: map[string]string{"app_id": "2021x"}},
	}
	byKey, byLookup := buildIndex(rows)
	if byKey[keyOf(1, KindPay, "wechat")] == nil || byKey[keyOf(3, KindPay, "alipay")] == nil {
		t.Fatal("主键索引缺失")
	}
	if r := byLookup[lookupKey(KindPay, "wechat", "100", "SER")]; r == nil || r.TenantID != 1 {
		t.Fatal("(a,b) 应命中租户 1")
	}
	if r := byLookup[lookupKey(KindPay, "wechat", "200", "")]; r == nil || r.TenantID != 2 {
		t.Fatal("(a,) 应命中租户 2")
	}
	if r, ok := byLookup[lookupKey(KindPay, "wechat", "", "SER")]; !ok || r != nil {
		t.Fatal("(,b) 两租户同 serial 应登记为歧义(nil 占位)")
	}
	if r := byLookup[lookupKey(KindPay, "alipay", "2021x", "")]; r == nil || r.TenantID != 3 {
		t.Fatal("支付宝按 app_id 命中")
	}
}

func TestResolvedBool(t *testing.T) {
	r := &Resolved{Fields: map[string]string{"a": "1", "b": "true", "c": "0", "d": ""}}
	if !r.Bool("a") || !r.Bool("b") || r.Bool("c") || r.Bool("d") || r.Bool("nope") {
		t.Fatal("bool 解析")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/provider/ 2>&1 | tail -3`
Expected: FAIL（包不存在）

- [ ] **Step 3: 模型**

`server/internal/model/model.go` 在 `AppCredential` 之后加：

```go
// ProviderConfig 一租户 × 一领域(支付/地图/内容安全) × 一服务商 一行。
//
// 字段整包加密存 JSON:机密与非机密混在一个 JSON 里,省得两列对账;
// LookupA/B 是 schema 指定的两个字段的明文投影,给回调反查租户用(微信 mch_id+platform_serial、支付宝 app_id)。
type ProviderConfig struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"uniqueIndex:uk_tenant_kind_provider,priority:1;index" json:"tenant_id,string"`
	Kind      string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:2" json:"kind"`
	Provider  string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:3" json:"provider"`
	Enabled   bool      `json:"enabled"`
	Active    bool      `gorm:"index" json:"active"` // 单选领域「当前用谁」;支付领域恒 false
	FieldsEnc string    `gorm:"type:text" json:"-"`
	LookupA   string    `gorm:"size:128;index" json:"-"`
	LookupB   string    `gorm:"size:128;index" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

`AllModels()` 的 `&Tenant{}, &AppCredential{},` 后加 `&ProviderConfig{},`。

- [ ] **Step 4: schema.go**

```go
// Package provider 第三方服务商(支付 / 地图 / 内容安全)的声明式配置:
// 服务商长什么样写在这里,后台页面按它渲染,业务包按它取值。加一家服务商 = 加一个 Definition。
package provider

const (
	KindPay        = "pay"
	KindMap        = "map"
	KindModeration = "moderation"

	PlatformBoth = "both"
	PlatformMP   = "miniprogram"
	PlatformApp  = "app"
)

type Field struct {
	Key      string
	LabelZh  string
	LabelEn  string
	Type     string // text / textarea / bool / select
	Secret   bool
	Required bool
	Options  []string
	Default  string
	HelpZh   string
	HelpEn   string
}

type Definition struct {
	Kind     string
	Provider string
	LabelZh  string
	LabelEn  string
	Platform string // 对哪种租户显示
	Fields   []Field
	LookupA  string // 投影到 ProviderConfig.LookupA 的字段键(回调反查)
	LookupB  string
	DocURL   string
}

const notifyBase = "https://ambertu.com/message/api/pay/callback/"

var definitions = []Definition{
	// ---------------- 支付 ----------------
	{
		Kind: KindPay, Provider: "wechat", LabelZh: "微信支付", LabelEn: "WeChat Pay", Platform: PlatformBoth,
		LookupA: "mch_id", LookupB: "platform_serial", DocURL: "https://pay.weixin.qq.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "AppID(小程序 / 开放平台移动应用)", LabelEn: "AppID (mini-program / open-platform app)", Type: "text", Required: true,
				HelpZh: "小程序租户填小程序 AppID,App 租户填开放平台移动应用 AppID;必须与商户号绑定", HelpEn: "Mini-program AppID for mini-program tenants, Open Platform app AppID for App tenants; must be bound to the merchant"},
			{Key: "mch_id", LabelZh: "商户号", LabelEn: "Merchant ID", Type: "text", Required: true},
			{Key: "apiv3_key", LabelZh: "APIv3 密钥", LabelEn: "APIv3 key", Type: "text", Secret: true, Required: true},
			{Key: "serial_no", LabelZh: "商户 API 证书序列号", LabelEn: "Merchant certificate serial", Type: "text", Required: true},
			{Key: "private_key", LabelZh: "商户 API 私钥 PEM", LabelEn: "Merchant private key PEM", Type: "textarea", Secret: true, Required: true},
			{Key: "platform_key", LabelZh: "微信支付平台公钥 PEM", LabelEn: "WeChat Pay platform public key PEM", Type: "textarea", Required: true},
			{Key: "platform_serial", LabelZh: "平台公钥 ID(PUB_KEY_ID_…)", LabelEn: "Platform key ID (PUB_KEY_ID_…)", Type: "text", Required: true},
			{Key: "notify_url", LabelZh: "回调地址", LabelEn: "Notify URL", Type: "text", Default: notifyBase + "wechat"},
		},
	},
	{
		Kind: KindPay, Provider: "alipay", LabelZh: "支付宝", LabelEn: "Alipay", Platform: PlatformBoth,
		LookupA: "app_id", DocURL: "https://open.alipay.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "应用 AppID", LabelEn: "App ID", Type: "text", Required: true},
			{Key: "private_key", LabelZh: "应用私钥", LabelEn: "App private key", Type: "textarea", Secret: true, Required: true},
			{Key: "alipay_public_key", LabelZh: "支付宝公钥(不是应用公钥)", LabelEn: "Alipay public key (not the app public key)", Type: "textarea", Required: true},
			{Key: "pid", LabelZh: "商户 PID(2088 开头,App 授权登录要)", LabelEn: "Partner ID (2088…, needed for app sign-in)", Type: "text"},
			{Key: "notify_url", LabelZh: "回调地址", LabelEn: "Notify URL", Type: "text", Default: notifyBase + "alipay"},
			{Key: "sandbox", LabelZh: "沙箱环境", LabelEn: "Sandbox", Type: "bool", Default: "0"},
		},
	},
	{
		Kind: KindPay, Provider: "apple", LabelZh: "iOS 内购(App Store)", LabelEn: "iOS in-app purchase (App Store)", Platform: PlatformApp,
		LookupA: "bundle_id", DocURL: "https://appstoreconnect.apple.com",
		Fields: []Field{
			{Key: "bundle_id", LabelZh: "Bundle ID", LabelEn: "Bundle ID", Type: "text", Required: true},
			{Key: "issuer_id", LabelZh: "App Store Connect Issuer ID", LabelEn: "App Store Connect Issuer ID", Type: "text", Required: true},
			{Key: "key_id", LabelZh: "内购私钥 Key ID", LabelEn: "IAP key ID", Type: "text", Required: true},
			{Key: "p8_key", LabelZh: ".p8 私钥", LabelEn: ".p8 private key", Type: "textarea", Secret: true, Required: true},
			{Key: "sandbox", LabelZh: "允许沙盒票据入账(上线前关闭)", LabelEn: "Accept sandbox receipts (turn off before launch)", Type: "bool", Default: "0"},
		},
	},
	{
		Kind: KindPay, Provider: "google_play", LabelZh: "Google Play 结算", LabelEn: "Google Play Billing", Platform: PlatformApp,
		LookupA: "package_name", DocURL: "https://play.google.com/console",
		Fields: []Field{
			{Key: "package_name", LabelZh: "应用包名", LabelEn: "Package name", Type: "text", Required: true},
			{Key: "service_account_json", LabelZh: "服务账号 JSON", LabelEn: "Service account JSON", Type: "textarea", Secret: true, Required: true,
				HelpZh: "Google Cloud 服务账号密钥文件全文,需在 Play Console 授予「查看财务数据」与「管理订单」", HelpEn: "Full service-account key file; grant it order/financial permissions in Play Console"},
		},
	},
	// ---------------- 地图(单选) ----------------
	{Kind: KindMap, Provider: "tencent", LabelZh: "腾讯位置服务", LabelEn: "Tencent Location", Platform: PlatformBoth, DocURL: "https://lbs.qq.com",
		Fields: []Field{{Key: "key", LabelZh: "Key", LabelEn: "Key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "google", LabelZh: "Google Maps", LabelEn: "Google Maps", Platform: PlatformBoth, DocURL: "https://console.cloud.google.com/google/maps-apis",
		Fields: []Field{{Key: "key", LabelZh: "Geocoding API Key", LabelEn: "Geocoding API key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "amap", LabelZh: "高德地图", LabelEn: "AMap", Platform: PlatformBoth, DocURL: "https://console.amap.com",
		Fields: []Field{{Key: "key", LabelZh: "Web 服务 Key", LabelEn: "Web service key", Type: "text", Secret: true, Required: true}}},
	{Kind: KindMap, Provider: "baidu", LabelZh: "百度地图", LabelEn: "Baidu Maps", Platform: PlatformBoth, DocURL: "https://lbsyun.baidu.com",
		Fields: []Field{
			{Key: "ak", LabelZh: "AK", LabelEn: "AK", Type: "text", Secret: true, Required: true},
			{Key: "sk", LabelZh: "SK(开了 SN 校验才填)", LabelEn: "SK (only if SN check is on)", Type: "text", Secret: true},
		}},
	// ---------------- 内容安全(单选) ----------------
	{Kind: KindModeration, Provider: "wechat", LabelZh: "微信内容安全", LabelEn: "WeChat content security", Platform: PlatformMP, DocURL: "https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/sec-center/sec-check/msgSecCheck.html",
		Fields: []Field{
			{Key: "text_on", LabelZh: "文本检测(msgSecCheck)", LabelEn: "Text check (msgSecCheck)", Type: "bool", Default: "1"},
			{Key: "image_on", LabelZh: "图片检测(mediaCheckAsync)", LabelEn: "Image check (mediaCheckAsync)", Type: "bool", Default: "1"},
		}},
	{Kind: KindModeration, Provider: "alipay", LabelZh: "支付宝内容安全", LabelEn: "Alipay content security", Platform: PlatformBoth, DocURL: "https://opendocs.alipay.com/open/02fnk6",
		Fields: []Field{
			{Key: "text_on", LabelZh: "文本检测", LabelEn: "Text check", Type: "bool", Default: "1"},
			{Key: "image_on", LabelZh: "图片检测", LabelEn: "Image check", Type: "bool", Default: "1"},
		}},
}

// Definitions 某领域的全部服务商,按声明顺序。
func Definitions(kind string) []Definition {
	var out []Definition
	for _, d := range definitions {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

func Find(kind, provider string) (Definition, bool) {
	for _, d := range definitions {
		if d.Kind == kind && d.Provider == provider {
			return d, true
		}
	}
	return Definition{}, false
}

// SingleActive 该领域是否「同一时刻只能用一家」。支付由客户端选渠道,可多家并存。
func SingleActive(kind string) bool { return kind == KindMap || kind == KindModeration }

// Missing 缺哪些必填字段(按声明顺序)。
func Missing(d Definition, fields map[string]string) []string {
	var miss []string
	for _, f := range d.Fields {
		if f.Required && strings.TrimSpace(fields[f.Key]) == "" {
			miss = append(miss, f.Key)
		}
	}
	return miss
}
```

（import `"strings"`。）

- [ ] **Step 5: store.go**

```go
package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"driftbottle/internal/crypto"
	"driftbottle/internal/model"
	"driftbottle/pkg/idgen"

	"gorm.io/gorm"
)

// Resolved 解密后的一行。
type Resolved struct {
	TenantID int64
	Kind     string
	Provider string
	Enabled  bool
	Active   bool
	Fields   map[string]string
}

func (r *Resolved) Get(k string) string { return r.Fields[k] }

// Bool "1" / "true"(忽略大小写)为真。
func (r *Resolved) Bool(k string) bool {
	v := strings.ToLower(strings.TrimSpace(r.Fields[k]))
	return v == "1" || v == "true"
}

type Store struct {
	db          *gorm.DB
	mu          sync.RWMutex
	byKey       map[string]*Resolved
	byLookup    map[string]*Resolved // nil 值 = 歧义占位
	tenantTypes map[int64]string
	ver         atomic.Uint64
}

func New(db *gorm.DB) *Store {
	return &Store{db: db, byKey: map[string]*Resolved{}, byLookup: map[string]*Resolved{}, tenantTypes: map[int64]string{}}
}

func keyOf(tid int64, kind, provider string) string {
	return fmt.Sprintf("%d:%s:%s", tid, kind, provider)
}

func lookupKey(kind, provider, a, b string) string { return kind + ":" + provider + ":" + a + "|" + b }

// encodeFields / decodeFields 整包加密;未配主密钥(本地开发)时明文 JSON。
func encodeFields(fields map[string]string) (string, error) {
	b, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	if !crypto.Enabled() {
		return string(b), nil
	}
	return crypto.Encrypt(string(b))
}

func decodeFields(enc string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(enc) == "" {
		return out, nil
	}
	raw := enc
	if !strings.HasPrefix(strings.TrimSpace(enc), "{") {
		plain, err := crypto.Decrypt(enc)
		if err != nil {
			return nil, err
		}
		raw = plain
	}
	return out, json.Unmarshal([]byte(raw), &out)
}

// lookupValues 把 schema 指定的两个字段投影成索引值。
func lookupValues(d Definition, fields map[string]string) (a, b string) {
	if d.LookupA != "" {
		a = strings.TrimSpace(fields[d.LookupA])
	}
	if d.LookupB != "" {
		b = strings.TrimSpace(fields[d.LookupB])
	}
	return
}

// buildIndex 建主键索引与反查索引。反查登记 (a,b)、(a,)、(,b) 三种键;
// 同一个键被两行命中 → 置 nil 当歧义占位(调用方拿到 ok=true && nil 就知道要再核对)。
func buildIndex(rows []*Resolved) (byKey, byLookup map[string]*Resolved) {
	byKey = make(map[string]*Resolved, len(rows))
	byLookup = map[string]*Resolved{}
	put := func(k string, r *Resolved) {
		if prev, seen := byLookup[k]; seen && prev != r {
			byLookup[k] = nil
			return
		}
		byLookup[k] = r
	}
	for _, r := range rows {
		byKey[keyOf(r.TenantID, r.Kind, r.Provider)] = r
		d, ok := Find(r.Kind, r.Provider)
		if !ok {
			continue
		}
		a, b := lookupValues(d, r.Fields)
		if a != "" && b != "" {
			put(lookupKey(r.Kind, r.Provider, a, b), r)
		}
		if a != "" {
			put(lookupKey(r.Kind, r.Provider, a, ""), r)
		}
		if b != "" {
			put(lookupKey(r.Kind, r.Provider, "", b), r)
		}
	}
	return
}

// Reload 全量加载并解密;顺带把租户类型表装进来(支付按租户类型定交易类型)。
func (s *Store) Reload() error {
	var rows []model.ProviderConfig
	if err := s.db.Find(&rows).Error; err != nil {
		return err
	}
	resolved := make([]*Resolved, 0, len(rows))
	for i := range rows {
		f, err := decodeFields(rows[i].FieldsEnc)
		if err != nil {
			return fmt.Errorf("解密服务商配置失败(tenant=%d %s/%s): %w", rows[i].TenantID, rows[i].Kind, rows[i].Provider, err)
		}
		resolved = append(resolved, &Resolved{TenantID: rows[i].TenantID, Kind: rows[i].Kind, Provider: rows[i].Provider,
			Enabled: rows[i].Enabled, Active: rows[i].Active, Fields: f})
	}
	var tenants []model.Tenant
	s.db.Select("tenant_id, type").Find(&tenants)
	types := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		types[t.TenantID] = t.Type
	}
	byKey, byLookup := buildIndex(resolved)
	s.mu.Lock()
	s.byKey, s.byLookup, s.tenantTypes = byKey, byLookup, types
	s.mu.Unlock()
	s.ver.Add(1)
	return nil
}

func (s *Store) Version() uint64 { return s.ver.Load() }

func (s *Store) Get(tid int64, kind, provider string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byKey[keyOf(tid, kind, provider)]
	return r, ok
}

// Active 单选领域当前生效的那家:enabled && active。
func (s *Store) Active(tid int64, kind string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.byKey {
		if r.TenantID == tid && r.Kind == kind && r.Enabled && r.Active {
			return r, true
		}
	}
	return nil, false
}

// Usable enabled && 必填齐全。/app-config 与下单前置判断都用它。
func (s *Store) Usable(tid int64, kind, provider string) bool {
	r, ok := s.Get(tid, kind, provider)
	if !ok || !r.Enabled {
		return false
	}
	d, ok := Find(kind, provider)
	return ok && len(Missing(d, r.Fields)) == 0
}

// ByLookup 回调反查。a / b 可只给一个;命中歧义或未命中都返回 false。
func (s *Store) ByLookup(kind, provider, a, b string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byLookup[lookupKey(kind, provider, strings.TrimSpace(a), strings.TrimSpace(b))]
	return r, ok && r != nil
}

func (s *Store) TenantType(tid int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tenantTypes[tid]
}

func (s *Store) All() []*Resolved {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Resolved, 0, len(s.byKey))
	for _, r := range s.byKey {
		out = append(out, r)
	}
	return out
}

type UpsertInput struct {
	Enabled *bool
	Active  *bool
	Fields  map[string]string
}

// mergeFields 旧值 + 本次写入 → 新值。机密写空保持,非机密写空清空,未知键丢弃。
func mergeFields(d Definition, old, in map[string]string) map[string]string {
	out := make(map[string]string, len(d.Fields))
	for _, f := range d.Fields {
		v, given := in[f.Key]
		switch {
		case !given:
			out[f.Key] = old[f.Key]
		case f.Secret && strings.TrimSpace(v) == "":
			out[f.Key] = old[f.Key]
		default:
			out[f.Key] = strings.TrimSpace(v)
		}
	}
	return out
}

var ErrUnknownProvider = errors.New("未知的服务商")

// Upsert 合并写一行;单选领域 active=true 时事务内把同领域其它行置 false;写完 Reload。
func (s *Store) Upsert(tid int64, kind, provider string, in UpsertInput) error {
	d, ok := Find(kind, provider)
	if !ok {
		return ErrUnknownProvider
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var row model.ProviderConfig
		err := tx.Where("tenant_id = ? AND kind = ? AND provider = ?", tid, kind, provider).First(&row).Error
		isNew := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !isNew {
			return err
		}
		old := map[string]string{}
		if !isNew {
			if old, err = decodeFields(row.FieldsEnc); err != nil {
				return err
			}
		} else {
			for _, f := range d.Fields {
				if f.Default != "" {
					old[f.Key] = f.Default
				}
			}
		}
		fields := mergeFields(d, old, in.Fields)
		enc, err := encodeFields(fields)
		if err != nil {
			return err
		}
		a, b := lookupValues(d, fields)
		now := time.Now()
		if isNew {
			row = model.ProviderConfig{ID: idgen.Next(), TenantID: tid, Kind: kind, Provider: provider, CreatedAt: now}
		}
		if in.Enabled != nil {
			row.Enabled = *in.Enabled
		}
		if in.Active != nil && SingleActive(kind) {
			row.Active = *in.Active
		}
		row.FieldsEnc, row.LookupA, row.LookupB, row.UpdatedAt = enc, a, b, now
		if row.Active && SingleActive(kind) {
			if err := tx.Model(&model.ProviderConfig{}).
				Where("tenant_id = ? AND kind = ? AND provider <> ?", tid, kind, provider).
				Update("active", false).Error; err != nil {
				return err
			}
		}
		if isNew {
			return tx.Create(&row).Error
		}
		return tx.Save(&row).Error
	}) // 事务结束后由调用方(或这里)Reload
}
```

`Upsert` 末尾（事务成功后）追加 `return s.Reload()`：把 `return s.db.Transaction(...)` 改成

```go
	if err := s.db.Transaction(func(tx *gorm.DB) error { ... }); err != nil {
		return err
	}
	return s.Reload()
```

- [ ] **Step 6: 运行确认通过**

Run: `cd server && gofmt -l internal/provider; go vet ./internal/provider/ && go test ./internal/provider/ -v 2>&1 | grep -E "^(--- |ok|FAIL)"`
Expected: 全部 PASS

---

### Task 2: 迁移：旧凭证 / 旧 sysconfig → `provider_configs`

**Files:**
- Modify: `server/internal/tenant/credstore.go`（加 `All()`）
- Create: `server/internal/provider/migrate.go`
- Create: `server/internal/provider/migrate_test.go`

**Interfaces:**
- Consumes: `tenant.Resolved`（已解密的凭证行）、`sysconfig.GetString`。
- Produces:
  - `func (s *tenant.Store) All() []*Resolved`
  - `type legacyRow struct{ TenantID int64; Platform, AppID, MchID, APIv3Key, SerialNo, PrivateKeyPEM, PlatformKeyPEM, PlatformSerial, AlipayPrivateKeyPEM, AlipayPublicKey, NotifyURL string }`
  - `type planned struct{ TenantID int64; Kind, Provider string; Enabled, Active bool; Fields map[string]string }`
  - `func planMigration(creds []legacyRow, tenantType func(int64) string, cfg func(tid int64, key string) string, tenantIDs []int64) []planned`
  - `func Migrate(db *gorm.DB, creds []*tenant.Resolved, store *Store) error`（幂等;写 `provider_migrated=1`）
  - 旧键名常量（字符串字面量，不再引用 sysconfig 常量）：`app_iap_enabled app_iap_sandbox app_iap_issuer_id app_iap_key_id app_iap_key app_apple_bundle_id app_pay_wechat_enabled app_pay_alipay_enabled app_maps_provider app_google_map_key geo_qq_key sec_check_text_on sec_check_image_on`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/provider/migrate_test.go
package provider

import "testing"

func byKeyOf(ps []planned) map[string]planned {
	m := map[string]planned{}
	for _, p := range ps {
		m[keyOf(p.TenantID, p.Kind, p.Provider)] = p
	}
	return m
}

// 同租户 wx 与 wx_app 都带商户号:App 租户取 wx_app,小程序租户取 wx;另一行丢弃。
func TestPlanMigrationPicksWechatRowByTenantType(t *testing.T) {
	creds := []legacyRow{
		{TenantID: 1, Platform: "wx", AppID: "wxmini", MchID: "100", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "P", PlatformKeyPEM: "PK", PlatformSerial: "PS", NotifyURL: "https://x/wx"},
		{TenantID: 1, Platform: "wx_app", AppID: "wxopen", MchID: "100", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "P", PlatformKeyPEM: "PK", PlatformSerial: "PS"},
		{TenantID: 2, Platform: "wx", AppID: "wxmini2"}, // 没商户号 → 不迁
	}
	types := func(id int64) string {
		if id == 1 {
			return "app"
		}
		return "miniprogram"
	}
	cfg := func(int64, string) string { return "" }
	got := byKeyOf(planMigration(creds, types, cfg, []int64{1, 2}))
	w, ok := got[keyOf(1, KindPay, "wechat")]
	if !ok || w.Fields["app_id"] != "wxopen" || w.Fields["mch_id"] != "100" || !w.Enabled {
		t.Fatalf("租户 1 应取 wx_app 行: %+v", w)
	}
	if w.Fields["notify_url"] != notifyBase+"wechat" {
		t.Fatalf("notify_url 应统一成新回调地址, got %s", w.Fields["notify_url"])
	}
	if _, ok := got[keyOf(2, KindPay, "wechat")]; ok {
		t.Fatal("没商户号的行不该迁")
	}
}

func TestPlanMigrationAlipayAndSwitches(t *testing.T) {
	creds := []legacyRow{
		{TenantID: 1, Platform: "alipay_app", AppID: "2021x", MchID: "2088pid", AlipayPrivateKeyPEM: "PRIV", AlipayPublicKey: "PUB"},
	}
	cfg := func(tid int64, key string) string {
		switch key {
		case "app_pay_alipay_enabled":
			return "0"
		case "app_pay_wechat_enabled":
			return "1"
		}
		return ""
	}
	got := byKeyOf(planMigration(creds, func(int64) string { return "app" }, cfg, []int64{1}))
	a := got[keyOf(1, KindPay, "alipay")]
	if a.Fields["app_id"] != "2021x" || a.Fields["pid"] != "2088pid" || a.Fields["private_key"] != "PRIV" || a.Fields["alipay_public_key"] != "PUB" {
		t.Fatalf("支付宝字段: %+v", a.Fields)
	}
	if a.Enabled {
		t.Fatal("app_pay_alipay_enabled=0 应迁成 enabled=false")
	}
}

func TestPlanMigrationIAPMapModeration(t *testing.T) {
	cfg := func(tid int64, key string) string {
		return map[string]string{
			"app_iap_enabled": "1", "app_iap_sandbox": "1", "app_iap_issuer_id": "ISS", "app_iap_key_id": "KID", "app_iap_key": "P8", "app_apple_bundle_id": "com.x.y",
			"geo_qq_key": "QQ", "app_google_map_key": "GG", "app_maps_provider": "google",
			"sec_check_text_on": "1", "sec_check_image_on": "0",
		}[key]
	}
	got := byKeyOf(planMigration(nil, func(int64) string { return "app" }, cfg, []int64{7}))
	ap := got[keyOf(7, KindPay, "apple")]
	if !ap.Enabled || ap.Fields["bundle_id"] != "com.x.y" || ap.Fields["p8_key"] != "P8" || ap.Fields["sandbox"] != "1" {
		t.Fatalf("apple: %+v", ap)
	}
	tc, gg := got[keyOf(7, KindMap, "tencent")], got[keyOf(7, KindMap, "google")]
	if tc.Fields["key"] != "QQ" || gg.Fields["key"] != "GG" {
		t.Fatal("地图 key")
	}
	if tc.Active || !gg.Active || !gg.Enabled {
		t.Fatalf("app_maps_provider=google 时 google 应 active: tencent=%v google=%v", tc.Active, gg.Active)
	}
	md := got[keyOf(7, KindModeration, "wechat")]
	if !md.Enabled || !md.Active || md.Fields["text_on"] != "1" || md.Fields["image_on"] != "0" {
		t.Fatalf("moderation: %+v", md)
	}
}

// 什么都没配的租户:一行都不迁。
func TestPlanMigrationEmptyTenantProducesNothing(t *testing.T) {
	got := planMigration(nil, func(int64) string { return "miniprogram" }, func(int64, string) string { return "" }, []int64{9})
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/provider/ -run TestPlanMigration 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: credstore.All()**

`server/internal/tenant/credstore.go` 追加：

```go
// All 全部解密行(迁移用)。
func (s *Store) All() []*Resolved {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Resolved, 0, len(s.byAppID))
	for _, r := range s.byAppID {
		out = append(out, r)
	}
	return out
}
```

- [ ] **Step 4: migrate.go**

```go
package provider

import (
	"log"
	"sort"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"

	"gorm.io/gorm"
)

// 旧 sysconfig 键(常量已删,这里用字面量;迁移脚本是它们最后的读者)。
const (
	oldIAPEnabled   = "app_iap_enabled"
	oldIAPSandbox   = "app_iap_sandbox"
	oldIAPIssuer    = "app_iap_issuer_id"
	oldIAPKeyID     = "app_iap_key_id"
	oldIAPKey       = "app_iap_key" // 密文,原样搬(解密后再存进加密 JSON)
	oldAppleBundle  = "app_apple_bundle_id"
	oldPayWechatOn  = "app_pay_wechat_enabled"
	oldPayAlipayOn  = "app_pay_alipay_enabled"
	oldMapsProvider = "app_maps_provider"
	oldGoogleMapKey = "app_google_map_key"
	oldQQKey        = "geo_qq_key"
	oldSecTextOn    = "sec_check_text_on"
	oldSecImageOn   = "sec_check_image_on"

	migratedFlag = "provider_migrated"
)

// legacyRow app_credentials 的解密投影(与 tenant.Resolved 同形,解耦是为了测试不依赖 tenant 包)。
type legacyRow struct {
	TenantID                                                     int64
	Platform, AppID, MchID, APIv3Key, SerialNo, PrivateKeyPEM    string
	PlatformKeyPEM, PlatformSerial, AlipayPrivateKeyPEM          string
	AlipayPublicKey, NotifyURL                                   string
}

type planned struct {
	TenantID int64
	Kind     string
	Provider string
	Enabled  bool
	Active   bool
	Fields   map[string]string
}

// planMigration 纯函数:旧数据 → 目标行。
func planMigration(creds []legacyRow, tenantType func(int64) string, cfg func(int64, string) string, tenantIDs []int64) []planned {
	var out []planned
	byTenant := map[int64][]legacyRow{}
	for _, c := range creds {
		byTenant[c.TenantID] = append(byTenant[c.TenantID], c)
	}
	on := func(tid int64, key, def string) bool {
		v := cfg(tid, key)
		if v == "" {
			v = def
		}
		return v == "1"
	}
	sort.Slice(tenantIDs, func(i, j int) bool { return tenantIDs[i] < tenantIDs[j] })
	for _, tid := range tenantIDs {
		isApp := tenantType(tid) == "app"
		// —— 微信支付:同租户可能 wx 与 wx_app 都有,按租户类型取一行 ——
		var wx *legacyRow
		for i := range byTenant[tid] {
			c := &byTenant[tid][i]
			if c.MchID == "" || (c.Platform != "wx" && c.Platform != "wx_app") {
				continue
			}
			prefer := "wx"
			if isApp {
				prefer = "wx_app"
			}
			if wx == nil || c.Platform == prefer {
				if wx != nil && c.Platform == prefer {
					log.Printf("[provider-migrate] tenant=%d 丢弃 %s 行的商户配置,保留 %s", tid, wx.Platform, prefer)
				}
				wx = c
			}
		}
		if wx != nil {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "wechat", Enabled: on(tid, oldPayWechatOn, "1"), Fields: map[string]string{
				"app_id": wx.AppID, "mch_id": wx.MchID, "apiv3_key": wx.APIv3Key, "serial_no": wx.SerialNo,
				"private_key": wx.PrivateKeyPEM, "platform_key": wx.PlatformKeyPEM, "platform_serial": wx.PlatformSerial,
				"notify_url": notifyBase + "wechat",
			}})
		}
		// —— 支付宝 ——
		var ali *legacyRow
		for i := range byTenant[tid] {
			c := &byTenant[tid][i]
			if c.AlipayPrivateKeyPEM == "" || (c.Platform != "alipay" && c.Platform != "alipay_app") {
				continue
			}
			prefer := "alipay"
			if isApp {
				prefer = "alipay_app"
			}
			if ali == nil || c.Platform == prefer {
				ali = c
			}
		}
		if ali != nil {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "alipay", Enabled: on(tid, oldPayAlipayOn, "1"), Fields: map[string]string{
				"app_id": ali.AppID, "private_key": ali.AlipayPrivateKeyPEM, "alipay_public_key": ali.AlipayPublicKey,
				"pid": ali.MchID, "notify_url": notifyBase + "alipay", "sandbox": "0",
			}})
		}
		// —— IAP ——
		if cfg(tid, oldIAPIssuer) != "" || cfg(tid, oldIAPKey) != "" || cfg(tid, oldIAPEnabled) == "1" {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "apple", Enabled: on(tid, oldIAPEnabled, "0"), Fields: map[string]string{
				"bundle_id": cfg(tid, oldAppleBundle), "issuer_id": cfg(tid, oldIAPIssuer), "key_id": cfg(tid, oldIAPKeyID),
				"p8_key": cfg(tid, oldIAPKey), "sandbox": map[bool]string{true: "1", false: "0"}[on(tid, oldIAPSandbox, "0")],
			}})
		}
		// —— 地图 ——
		mapsProvider := cfg(tid, oldMapsProvider)
		if mapsProvider == "" {
			mapsProvider = "tencent"
		}
		if k := cfg(tid, oldQQKey); k != "" {
			out = append(out, planned{TenantID: tid, Kind: KindMap, Provider: "tencent", Enabled: true, Active: mapsProvider == "tencent", Fields: map[string]string{"key": k}})
		}
		if k := cfg(tid, oldGoogleMapKey); k != "" {
			out = append(out, planned{TenantID: tid, Kind: KindMap, Provider: "google", Enabled: true, Active: mapsProvider == "google", Fields: map[string]string{"key": k}})
		}
		// —— 内容安全(微信) ——
		if cfg(tid, oldSecTextOn) == "1" || cfg(tid, oldSecImageOn) == "1" {
			out = append(out, planned{TenantID: tid, Kind: KindModeration, Provider: "wechat", Enabled: true, Active: true, Fields: map[string]string{
				"text_on": map[bool]string{true: "1", false: "0"}[cfg(tid, oldSecTextOn) == "1"],
				"image_on": map[bool]string{true: "1", false: "0"}[cfg(tid, oldSecImageOn) == "1"],
			}})
		}
	}
	return out
}

// Migrate 启动时执行一次:已有目标行不覆盖;完成后写 provider_migrated=1。
func Migrate(db *gorm.DB, creds []*tenant.Resolved, store *Store) error {
	if sysconfig.GetString(0, migratedFlag) == "1" {
		return nil
	}
	var tenants []model.Tenant
	if err := db.Select("tenant_id, type").Find(&tenants).Error; err != nil {
		return err
	}
	ids := make([]int64, 0, len(tenants))
	types := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		ids = append(ids, t.TenantID)
		types[t.TenantID] = t.Type
	}
	rows := make([]legacyRow, 0, len(creds))
	for _, c := range creds {
		rows = append(rows, legacyRow{TenantID: c.TenantID, Platform: c.Platform, AppID: c.AppID, MchID: c.MchID, APIv3Key: c.PayAPIv3Key,
			SerialNo: c.PaySerialNo, PrivateKeyPEM: c.PayPrivateKeyPEM, PlatformKeyPEM: c.PayPlatformKeyPEM, PlatformSerial: c.PayPlatformSerial,
			AlipayPrivateKeyPEM: c.AlipayPrivateKeyPEM, AlipayPublicKey: c.AlipayPublicKey, NotifyURL: c.NotifyURL})
	}
	// 旧 .p8 存的是 AES 密文:解出来再交给 Upsert(它会整包重新加密)
	cfg := func(tid int64, key string) string {
		v := sysconfig.GetString(tid, key)
		if key == oldIAPKey && v != "" {
			if plain, err := decryptOrKeep(v); err == nil {
				return plain
			}
		}
		return v
	}
	for _, p := range planMigration(rows, func(id int64) string { return types[id] }, cfg, ids) {
		if _, exists := store.Get(p.TenantID, p.Kind, p.Provider); exists {
			continue
		}
		en, ac := p.Enabled, p.Active
		if err := store.Upsert(p.TenantID, p.Kind, p.Provider, UpsertInput{Enabled: &en, Active: &ac, Fields: p.Fields}); err != nil {
			return err
		}
		log.Printf("[provider-migrate] tenant=%d %s/%s 已迁移(enabled=%v active=%v)", p.TenantID, p.Kind, p.Provider, en, ac)
	}
	return sysconfig.Set(0, migratedFlag, "1")
}

func decryptOrKeep(v string) (string, error) {
	if !crypto.Enabled() {
		return v, nil
	}
	return crypto.Decrypt(v)
}
```

（import 补 `"driftbottle/internal/crypto"`。`sysconfig.Set` 已存在；`migratedFlag` 不进 `defaults` 也不进 meta 白名单——它只是个一次性标记。）

- [ ] **Step 5: 运行确认通过**

Run: `cd server && go vet ./internal/provider/ ./internal/tenant/ && go test ./internal/provider/ ./internal/tenant/ 2>&1 | tail -3`
Expected: PASS

---

### Task 3: 支付驱动改读 `provider.Store`（渠道名统一、回调反查、沙箱网关）

**Files:**
- Modify: `server/internal/pay/service.go`
- Modify: `server/internal/pay/handler.go`
- Modify: `server/internal/pay/order_query.go`
- Modify: `server/internal/pay/driver_alipay.go`（`AliCreds.Sandbox`）
- Modify: `server/internal/pay/channel_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/api/routes_test.go`（`pay.NewHandler(nil)` 不变，无需改）

**Interfaces:**
- Consumes: Task 1 `provider.Store`（`Get/Usable/ByLookup/TenantType/Version`）。
- Produces:
  - `func New(db, w, getOpenID, getAlipayUID OpenIDFunc, cfg, creds *tenant.Store, providers *provider.Store) *Service`（**签名变更**）
  - `func canonicalChannel(p string) string`：`wx`/`wx_app`→`wechat`，`alipay_app`→`alipay`，`ios`→`apple`，`gplay`→`google_play`，其它原样
  - `func resolveChannel(platform, channel string, mockOn bool) (string, error)` 返回规范渠道名（`wechat` / `alipay` / `app`）
  - `PayOrder.Platform` 新值：`wechat` / `alipay` / `apple` / `google_play` / `app`
  - `func (s *Service) DriverByWxSerial(serial string) (Driver, bool)`、`func (s *Service) DriverByAlipayAppID(appid string) (Driver, bool)`（替代 `DriverBySerial` / `DriverByAppID`）
  - `AliCreds.Sandbox bool`

- [ ] **Step 1: 改测试**（`channel_test.go` 的 `TestResolveChannel` 替换为）

```go
func TestCanonicalChannel(t *testing.T) {
	for in, want := range map[string]string{"wx": "wechat", "wx_app": "wechat", "wechat": "wechat", "alipay_app": "alipay", "alipay": "alipay", "ios": "apple", "gplay": "google_play", "app": "app", "": ""} {
		if got := canonicalChannel(in); got != want {
			t.Errorf("canonicalChannel(%q) = %q want %q", in, got, want)
		}
	}
}

func TestResolveChannel(t *testing.T) {
	cases := []struct {
		platform, channel string
		mock              bool
		want              string
		wantErr           bool
	}{
		{"app", "wechat", false, "wechat", false},
		{"app", "wx_app", false, "wechat", false},     // 已发布的国内版 App 还在传旧名
		{"app", "alipay_app", false, "alipay", false},
		{"app", "", false, "", true},
		{"app", "apple", false, "", true},             // IAP 不走 /pay/order
		{"app", "wechat", true, "app", false},         // mock 开着忽略 channel
		{"wx", "", false, "wechat", false},            // 小程序:登录平台即渠道
		{"alipay", "", false, "alipay", false},
		{"wx", "wechat", false, "", true},             // 小程序不许传 channel
	}
	for _, c := range cases {
		got, err := resolveChannel(c.platform, c.channel, c.mock)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("resolveChannel(%q,%q,%v) = %q,%v want %q,err=%v", c.platform, c.channel, c.mock, got, err, c.want, c.wantErr)
		}
	}
}
```

`TestSyncThrottle` / `TestNeedsSync` / `TestDriverCacheInvalidatesOnCredVersion` 不动。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/pay/ -run 'TestCanonicalChannel|TestResolveChannel' 2>&1 | tail -3`
Expected: FAIL（`canonicalChannel` 未定义）

- [ ] **Step 3: service.go**

```go
// 字段
type Service struct {
	// ...原字段...
	creds     *tenant.Store
	providers *provider.Store
	// ...
}

func New(db *gorm.DB, w *wallet.Service, getOpenID, getAlipayUID OpenIDFunc, cfg *config.Config, creds *tenant.Store, providers *provider.Store) *Service {
	return &Service{db: db, wallet: w, getOpenID: getOpenID, getAlipayUID: getAlipayUID, cfg: cfg, creds: creds, providers: providers, driverCache: map[string]Driver{}}
}

// channelAliases 历史渠道名 → 规范名。订单表与已发布客户端里还有旧值,一律先折算。
var channelAliases = map[string]string{"wx": "wechat", "wx_app": "wechat", "alipay_app": "alipay", "ios": "apple", "gplay": "google_play"}

func canonicalChannel(p string) string {
	if c, ok := channelAliases[p]; ok {
		return c
	}
	return p
}

// appChannels App 端可在 /pay/order 选的渠道(IAP / Play 走各自的 verify 接口,不在这里)。
var appChannels = map[string]bool{"wechat": true, "alipay": true}

// resolveChannel 「登录平台 + 客户端选的渠道」→ 规范渠道名。
func resolveChannel(platform, channel string, mockOn bool) (string, error) {
	channel = canonicalChannel(channel)
	if platform == "app" {
		if mockOn {
			return "app", nil
		}
		if !appChannels[channel] {
			return "", errs.New(errs.CodeBadRequest, "请选择支付方式")
		}
		return channel, nil
	}
	if channel != "" {
		return "", errs.New(errs.CodeBadRequest, "不支持的支付方式")
	}
	return canonicalChannel(platform), nil
}
```

`driverFor(tenantID, channel)`：`ver` 取 `s.providers.Version()`（`providers` 为 nil 时 0），其余逻辑不变（`channel == "app"` 不缓存）。

`buildDriver(tenantID, channel)`：

```go
func (s *Service) buildDriver(tenantID int64, channel string) (Driver, error) {
	if channel == "app" && sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled) {
		return mockDriver{}, nil
	}
	if s.providers != nil {
		if row, ok := s.providers.Get(tenantID, provider.KindPay, channel); ok && s.providers.Usable(tenantID, provider.KindPay, channel) {
			isApp := s.providers.TenantType(tenantID) == "app"
			switch channel {
			case "wechat":
				tt := ""
				if isApp {
					tt = "APP"
				}
				return NewWxDriver(WxCreds{
					AppID: row.Get("app_id"), MchID: row.Get("mch_id"), APIv3Key: row.Get("apiv3_key"), SerialNo: row.Get("serial_no"),
					PrivateKeyPEM: row.Get("private_key"), PlatformPubPEM: row.Get("platform_key"), PlatformSerial: row.Get("platform_serial"),
					NotifyURL: row.Get("notify_url"), TradeType: tt, TenantID: tenantID,
				}), nil
			case "alipay":
				tt := ""
				if isApp {
					tt = "APP"
				}
				return NewAlipayDriver(AliCreds{
					AppID: row.Get("app_id"), PrivateKeyPEM: row.Get("private_key"), PublicKey: row.Get("alipay_public_key"),
					NotifyURL: row.Get("notify_url"), PID: row.Get("pid"), TradeType: tt, Sandbox: row.Bool("sandbox"), TenantID: tenantID,
				}), nil
			}
		}
		if s.cfg.MultiTenant {
			return nil, errs.New(errs.CodeBadRequest, "该租户未配置该支付渠道")
		}
	}
	// 单租户 .env 回落(本地开发)
	switch channel {
	case "wechat":
		// ...原 case "wx" 的 .env 分支原样...
	case "alipay":
		return NewAlipayDriver(AliCreds{AppID: s.cfg.Alipay.AppID, NotifyURL: s.cfg.Alipay.NotifyURL}), nil
	}
	return nil, errs.New(errs.CodeBadRequest, "不支持的支付平台")
}
```

删除多租户分支里旧的 `s.creds.ByTenantPlatform(...)` 读法（`creds` 字段保留给别处，不再在支付里用）。`CreateOrder` 里 `payerID` 分支改为 `driverPlatform == "wechat" && 登录平台是小程序`：

```go
	var payerID string
	switch {
	case driverPlatform == "wechat" && platform == "wx" && s.getOpenID != nil:
		payerID, _ = s.getOpenID(userID)
	case driverPlatform == "alipay" && platform == "alipay" && s.getAlipayUID != nil:
		payerID, _ = s.getAlipayUID(userID)
	}
```

回调反查替换 `DriverBySerial` / `DriverByAppID`：

```go
// DriverByWxSerial 微信回调:平台公钥 ID → 租户。同 serial 两租户时 ByLookup 返回 false,回落默认租户。
func (s *Service) DriverByWxSerial(serial string) (Driver, bool) {
	if s.providers != nil && serial != "" {
		if r, ok := s.providers.ByLookup(provider.KindPay, "wechat", "", serial); ok {
			d, err := s.driverFor(r.TenantID, "wechat")
			return d, err == nil
		}
	}
	d, err := s.driverFor(s.cfg.DefaultTenantID, "wechat")
	return d, err == nil
}

// DriverByAlipayAppID 支付宝回调:表单 app_id → 租户。
func (s *Service) DriverByAlipayAppID(appid string) (Driver, bool) {
	if s.providers != nil && appid != "" {
		if r, ok := s.providers.ByLookup(provider.KindPay, "alipay", appid, ""); ok {
			d, err := s.driverFor(r.TenantID, "alipay")
			return d, err == nil
		}
	}
	d, err := s.driverFor(s.cfg.DefaultTenantID, "alipay")
	return d, err == nil
}
```

`DriverForTenant(tenantID, platform)` 内部 `canonicalChannel(platform)`。

- [ ] **Step 4: handler.go / order_query.go / driver_alipay.go / main.go**

`handler.go` `callbackAuto`：

```go
	platform := canonicalChannel(c.Param("platform")) // wx / wx_app / alipay_app 旧路径仍可达
	var d Driver
	var ok bool
	switch platform {
	case "wechat":
		d, ok = h.svc.DriverByWxSerial(c.Request.Header.Get("Wechatpay-Serial"))
	case "alipay":
		_ = c.Request.ParseForm()
		d, ok = h.svc.DriverByAlipayAppID(c.Request.Form.Get("app_id"))
	}
```

`callback`（带 tenantId 的旧路由）里 `h.svc.DriverForTenant(tenantID, platform)` 不变（内部已折算）。`HandleCallback(platform, res)` 的 `platform` 传规范名。

`order_query.go` `SyncIfStale`：`s.driverFor(order.TenantID, canonicalChannel(order.Platform))`。

`driver_alipay.go`：`AliCreds` 加 `Sandbox bool`；`client()` 里 `if d.c.Sandbox { cli.Gateway = alipay.GatewaySandbox }`（测试覆盖的 `d.gateway` 优先级更高，放在其后）。

`cmd/api/main.go`：在 `credStore` 之后、`userSvc` 之前：

```go
	providerStore := provider.New(db)
	if err := providerStore.Reload(); err != nil {
		log.Fatalf("加载服务商配置失败: %v", err)
	}
	if err := provider.Migrate(db, credStore.All(), providerStore); err != nil {
		log.Fatalf("迁移服务商配置失败: %v", err)
	}
```

`pay.New(..., credStore, providerStore)`。

- [ ] **Step 5: 运行确认通过**

Run: `cd server && go build ./... && go vet ./internal/pay/ && go test ./internal/pay/ ./cmd/... 2>&1 | tail -4`
Expected: PASS

---

### Task 4: IAP 改读服务商配置；`/app-config` 支付可用性；删除旧 sysconfig 键与 `/config` 分组

**Files:**
- Modify: `server/internal/pay/iap.go`
- Modify: `server/internal/sysconfig/sysconfig.go`（删 12 个键 + defaults）
- Modify: `server/internal/sysconfig/handler.go`
- Modify: `server/internal/sysconfig/app_config_test.go`
- Modify: `server/internal/admin/meta.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Consumes: Task 1 `provider.Store`。
- Produces:
  - `func (h *sysconfig.Handler) WithPayUsable(fn func(tenantID int64, provider string) bool) *Handler`
  - `/app-config` 的 `pay` 段：`{wechat, alipay, apple, google_play}`
  - 删除的键：`KeyAppIAPEnabled KeyAppIAPSandbox KeyAppIAPIssuerID KeyAppIAPKeyID KeyAppIAPKeyEnc KeyAppPayWechatEnabled KeyAppPayAlipayEnabled KeyAppMapsProvider KeyAppGoogleMapKey KeyGeoQQKey KeySecCheckTextOn KeySecCheckImageOn`（`KeyAppAppleBundleID` 保留——Apple 登录验 aud 用）
  - 删除的分组常量：`GroupAppPay GroupGeo GroupPayMP`；`KeyIOSRechargeOff` 改入 `GroupPrice`

- [ ] **Step 1: 改测试**（`app_config_test.go`）

把 `TestAppConfigChannelsOffWithoutCreds` 的 pay 断言改为 `pay["apple"] != false || pay["google_play"] != false` 一并检查；`getAppConfigWith` 加第三个参数 `payUsable func(int64, string) bool` 并链式 `.WithPayUsable(payUsable)`；`TestAppConfigChannelsOnWithCredsAndOffBySwitch` 改为：

```go
func TestAppConfigPayFromProviderStore(t *testing.T) {
	lookup := func(tid int64, platform string) (string, bool) {
		if platform == "wx_app" {
			return "wxopen123", true
		}
		return "", false
	}
	usable := func(tid int64, p string) bool { return p == "wechat" || p == "apple" }
	setCache(map[int64]map[string]string{})
	d := getAppConfigWith(t, 100, lookup, usable)
	pay := d["pay"].(map[string]any)
	if pay["wechat"] != true || pay["alipay"] != false || pay["apple"] != true || pay["google_play"] != false {
		t.Fatalf("pay = %+v", pay)
	}
	auth := d["auth"].(map[string]any)
	if auth["wechat"] != true || auth["wechat_app_id"] != "wxopen123" {
		t.Fatalf("auth 仍由登录凭证决定: %+v", auth)
	}
}
```

`TestChannelSwitchDefaultsOn` 只保留登录两个开关与 universal link。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/sysconfig/ -run 'TestAppConfigPayFromProviderStore' 2>&1 | tail -3`
Expected: FAIL（`WithPayUsable` 未定义）

- [ ] **Step 3: sysconfig handler**

```go
type Handler struct {
	// ...
	credLookup CredLookup
	payUsable  func(tenantID int64, provider string) bool // provider.Store.Usable 的支付投影;nil = 全不可用
}

func (h *Handler) WithPayUsable(fn func(int64, string) bool) *Handler { h.payUsable = fn; return h }

func (h *Handler) pay(tid int64, p string) bool {
	return h.payUsable != nil && h.payUsable(tid, p)
}
// getAppConfig 里:
		"pay": gin.H{
			"wechat":      h.pay(tid, "wechat"),
			"alipay":      h.pay(tid, "alipay"),
			"apple":       h.pay(tid, "apple"),
			"google_play": h.pay(tid, "google_play"),
		},
```

`main.go`：`.WithPayUsable(func(tid int64, p string) bool { return providerStore.Usable(tid, provider.KindPay, p) })`。

- [ ] **Step 4: 删键**

`sysconfig.go`：删除 12 个常量及其 defaults 行（`KeyAppLoginWechatEnabled / KeyAppLoginAlipayEnabled / KeyAppWechatUniversalLink / KeyAppPayMockEnabled / KeyAppAppleBundleID / KeySecCallbackToken` 保留）。
`meta.go`：删除对应 12 条 `configFieldMeta`；`KeyIOSRechargeOff` 的 `Group` 改 `GroupPrice`；删除 `GroupAppPay`、`GroupGeo`、`GroupPayMP` 常量与 `groupMeta` 条目；`GroupModeration` 只剩回调 Token 等键，若该组因此为空则一并删除（`TestNoOrphanGroups` 会点名）。

- [ ] **Step 5: iap.go**

```go
// iapConfig 该租户的 App Store 配置(服务商页「iOS 内购」卡片)。
func (s *Service) iapConfig(tenantID int64) (*provider.Resolved, bool) {
	if s.providers == nil {
		return nil, false
	}
	r, ok := s.providers.Get(tenantID, provider.KindPay, "apple")
	return r, ok && r.Enabled
}

func (s *Service) appStoreToken(tenantID int64, bundleID string) (string, error) {
	row, ok := s.iapConfig(tenantID)
	if !ok || row.Get("issuer_id") == "" || row.Get("key_id") == "" || row.Get("p8_key") == "" {
		return "", errors.New("未配置 App Store Server API 凭证")
	}
	block, _ := pem.Decode([]byte(row.Get("p8_key")))
	// ...其余与原来相同(issuer := row.Get("issuer_id"), kid := row.Get("key_id"))...
}

// VerifyIAP 开头:
	row, ok := s.iapConfig(tenantID)
	if !ok {
		return nil, errs.New(errs.CodeForbidden, "内购未开启")
	}
	bundleID := row.Get("bundle_id")
	if bundleID == "" {
		return nil, errs.New(errs.CodeServerError, "未配置 Apple Bundle ID")
	}
	sandboxOK := row.Bool("sandbox")
	// 之后所有 sysconfig.GetBool(tenantID, sysconfig.KeyAppIAPSandbox) 换成 sandboxOK
```

`PayOrder.Platform` 由 `"ios"` 改为 `"apple"`（`canonicalChannel` 已把历史 `ios` 折回 `apple`）。去掉 `crypto` import（`p8_key` 已是明文）。

- [ ] **Step 6: 运行确认通过**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/sysconfig/ ./internal/admin/ ./internal/pay/ 2>&1 | tail -4`
Expected: PASS（`meta_test` 的 `TestNoOrphanGroups` / `TestNoEmptySections` 通过）

---

### Task 5: Google Play 服务端购买校验

**Files:**
- Create: `server/internal/pay/play_verify.go`
- Create: `server/internal/pay/play_verify_test.go`
- Modify: `server/internal/pay/handler.go`（`POST /pay/play/verify`）
- Modify: `server/internal/pay/service.go`（抽 `creditPaidOrder`）

**Interfaces:**
- Produces:
  - `func playToken(saJSON string, now time.Time, tokenURLOverride string) (string, error)`
  - `type playProduct struct{ OrderID string; PurchaseState, ConsumptionState, AcknowledgementState int; PurchaseTimeMillis string; RegionCode string }`
  - `func playDecision(p playProduct) error`
  - `func (s *Service) VerifyPlay(tenantID, userID int64, productID, purchaseToken string) (*model.PlayPurchase, error)`
  - `var playAPIBase = "https://androidpublisher.googleapis.com"`（测试覆盖）
  - `func creditPaidOrder(tx *gorm.DB, tenantID, userID, coins int64, orderNo string) error`（入账 + 首充打标，`HandleCallback` / `VerifyIAP` / `VerifyPlay` 共用）
  - `POST /pay/play/verify (auth) {product_id, purchase_token}` → `{order_id, coins, order_no}`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/pay/play_verify_test.go
package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeServiceAccount(t *testing.T, tokenURL string) string {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: must(x509.MarshalPKCS8PrivateKey(priv))}))
	b, _ := json.Marshal(map[string]string{"client_email": "sa@proj.iam.gserviceaccount.com", "private_key": pemStr, "token_uri": tokenURL})
	return string(b)
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func TestPlayTokenExchangesSignedJWT(t *testing.T) {
	var assertion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		assertion = r.PostForm.Get("assertion")
		if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type = %s", r.PostForm.Get("grant_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"ya29.x","expires_in":3600}`))
	}))
	defer srv.Close()
	tok, err := playToken(fakeServiceAccount(t, srv.URL), time.Unix(1700000000, 0), "")
	if err != nil || tok != "ya29.x" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	parts := strings.Split(assertion, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(payload, &claims)
	if claims["iss"] != "sa@proj.iam.gserviceaccount.com" || claims["scope"] != "https://www.googleapis.com/auth/androidpublisher" || claims["aud"] != srv.URL {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestPlayDecision(t *testing.T) {
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 0}); err != nil {
		t.Fatalf("已购买应通过: %v", err)
	}
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 1}); err == nil {
		t.Fatal("已取消/退款不该入账")
	}
	if err := playDecision(playProduct{OrderID: "GPA.1", PurchaseState: 2}); err == nil {
		t.Fatal("待处理不该入账")
	}
	if err := playDecision(playProduct{PurchaseState: 0}); err == nil {
		t.Fatal("没有 orderId 不该入账(没法幂等)")
	}
}

func TestPlayProductParse(t *testing.T) {
	raw := `{"purchaseTimeMillis":"1700000000000","purchaseState":0,"consumptionState":0,"orderId":"GPA.3333-1111","acknowledgementState":0,"regionCode":"IN","kind":"androidpublisher#productPurchase"}`
	var p playProduct
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if p.OrderID != "GPA.3333-1111" || p.RegionCode != "IN" || p.PurchaseState != 0 {
		t.Fatalf("%+v", p)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/pay/ -run 'TestPlay' 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: play_verify.go**

```go
package pay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/provider"
	"driftbottle/pkg/apilog"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Google Play 结算服务端校验。
//
// 客户端(in_app_purchase)拿到 purchaseToken 后调 /pay/play/verify;服务端用服务账号向
// androidpublisher 核实,入账后再 acknowledge——不 acknowledge 的购买 3 天后会被 Google 自动退款。
// 幂等键是 Play 的 orderId:App 冷启动会把未 consume 的购买重新推上来。

var playAPIBase = "https://androidpublisher.googleapis.com"
var playHTTPClient = &http.Client{Timeout: 10 * time.Second}

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// playToken 服务账号 JSON → access_token(JWT bearer 换取)。tokenURLOverride 仅测试用。
func playToken(saJSON string, now time.Time, tokenURLOverride string) (string, error) {
	var sa serviceAccount
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", fmt.Errorf("服务账号 JSON 不合法: %w", err)
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	if tokenURLOverride != "" {
		sa.TokenURI = tokenURLOverride
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("服务账号私钥不合法: %w", err)
	}
	claims := jwt.MapClaims{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/androidpublisher",
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	resp, err := playHTTPClient.PostForm(sa.TokenURI, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("Google 换 token 失败: %s", string(body))
	}
	return tok.AccessToken, nil
}

type playProduct struct {
	OrderID              string `json:"orderId"`
	PurchaseState        int    `json:"purchaseState"` // 0 已购买 1 已取消 2 待处理
	ConsumptionState     int    `json:"consumptionState"`
	AcknowledgementState int    `json:"acknowledgementState"`
	PurchaseTimeMillis   string `json:"purchaseTimeMillis"`
	RegionCode           string `json:"regionCode"`
}

func (p playProduct) purchasedAt() time.Time {
	ms, _ := strconv.ParseInt(p.PurchaseTimeMillis, 10, 64)
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

// playDecision 能不能入账。
func playDecision(p playProduct) error {
	if p.OrderID == "" {
		return errs.New(errs.CodePaySignError, "Google 未返回订单号")
	}
	switch p.PurchaseState {
	case 0:
		return nil
	case 1:
		return errs.New(errs.CodeForbidden, "该购买已取消或退款")
	default:
		return errs.New(errs.CodeForbidden, "该购买仍在处理中")
	}
}

// VerifyPlay 核实 purchaseToken 并入账(幂等)。
func (s *Service) VerifyPlay(tenantID, userID int64, productID, purchaseToken string) (*model.PlayPurchase, error) {
	if s.providers == nil || !s.providers.Usable(tenantID, provider.KindPay, "google_play") {
		return nil, errs.New(errs.CodeForbidden, "Google Play 结算未开启")
	}
	row, _ := s.providers.Get(tenantID, provider.KindPay, "google_play")
	pkgName := row.Get("package_name")
	token, err := playToken(row.Get("service_account_json"), time.Now(), "")
	if err != nil {
		apilog.Record(tenantID, "play_token", pkgName, 0, err.Error(), false)
		return nil, errs.New(errs.CodeServerError, "Google 凭证无效")
	}
	u := fmt.Sprintf("%s/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s",
		playAPIBase, url.PathEscape(pkgName), url.PathEscape(productID), url.PathEscape(purchaseToken))
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := playHTTPClient.Do(req)
	if err != nil {
		return nil, errs.New(errs.CodeServerError, "Google 查询失败")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	apilog.Record(tenantID, "play_verify", productID, resp.StatusCode, string(body), resp.StatusCode == 200)
	if resp.StatusCode != 200 {
		return nil, errs.New(errs.CodePaySignError, "购买凭证无效")
	}
	var p playProduct
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errs.New(errs.CodeServerError, "Google 响应不合法")
	}
	if err := playDecision(p); err != nil {
		return nil, err
	}
	var pkg model.CoinPackage
	if err := s.db.First(&pkg, "play_product_id = ? AND status = ?", productID, "active").Error; err != nil {
		return nil, errs.New(errs.CodeBadRequest, "未知的商品:"+productID)
	}
	var prices []model.CoinPackagePrice
	s.db.Where("package_id = ?", pkg.PackageID).Find(&prices)
	price := pickPrice(prices, "gplay", p.RegionCode)
	coins := pkg.Coins + pkg.BonusCoins

	var saved model.PlayPurchase
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var exist model.PlayPurchase
		if e := tx.First(&exist, "order_id = ?", p.OrderID).Error; e == nil {
			saved = exist
			return nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		order := model.PayOrder{OrderNo: genOrderNo(), TenantID: tenantID, UserID: userID, PackageID: pkg.PackageID,
			Platform: "google_play", Coins: coins, Status: "paid", PlatformTxn: p.OrderID, CreatedAt: time.Now()}
		if price != nil {
			order.PriceMinor, order.Currency = price.Amount, price.Currency
		}
		paidAt := p.purchasedAt()
		order.PaidAt = &paidAt
		if e := tx.Create(&order).Error; e != nil {
			return e
		}
		saved = model.PlayPurchase{TenantID: tenantID, UserID: userID, OrderID: p.OrderID, PurchaseToken: purchaseToken,
			ProductID: productID, OrderNo: order.OrderNo, Coins: coins, State: "purchased", Currency: order.Currency,
			AmountMinor: order.PriceMinor, CreatedAt: time.Now()}
		if e := tx.Create(&saved).Error; e != nil {
			return e
		}
		return creditPaidOrder(tx, tenantID, userID, coins, order.OrderNo)
	})
	if err != nil {
		return nil, err
	}
	// acknowledge:失败只记日志,下次冷启动客户端会再推一次,幂等保证不重复发币
	if p.AcknowledgementState == 0 {
		ackURL := u + ":acknowledge"
		ackReq, _ := http.NewRequest(http.MethodPost, ackURL, bytes.NewReader([]byte(`{}`)))
		ackReq.Header.Set("Authorization", "Bearer "+token)
		ackReq.Header.Set("Content-Type", "application/json")
		if r2, e := playHTTPClient.Do(ackReq); e == nil {
			r2.Body.Close()
			if r2.StatusCode < 300 {
				now := time.Now()
				s.db.Model(&model.PlayPurchase{}).Where("order_id = ?", p.OrderID).Update("acked_at", &now)
			}
			apilog.Record(tenantID, "play_ack", p.OrderID, r2.StatusCode, "", r2.StatusCode < 300)
		}
	}
	return &saved, nil
}
```

`service.go` 新增并在 `HandleCallback` / `VerifyIAP` 里替换原来的「CreditTx + 首充打标」两段：

```go
// creditPaidOrder 入账 + 首充打「付费用户」标签。所有渠道共用,这是唯一的发币口。
func creditPaidOrder(tx *gorm.DB, tenantID, userID, coins int64, orderNo string) error {
	if err := wallet.CreditTx(tx, tenantID, userID, coins, wallet.SceneRecharge, orderNo); err != nil {
		return err
	}
	return tx.Exec(
		"UPDATE users SET tags = IF(tags = '' OR tags IS NULL, ?, CONCAT(tags, ',', ?)) "+
			"WHERE user_id = ? AND (tags IS NULL OR tags NOT LIKE ?)",
		model.TagPaidUser, model.TagPaidUser, userID, "%"+model.TagPaidUser+"%",
	).Error
}
```

`handler.go`：

```go
	api.POST("/pay/play/verify", auth, h.playVerify)

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

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go vet ./internal/pay/ && go test ./internal/pay/ ./cmd/... 2>&1 | tail -3`
Expected: PASS

---

### Task 6: 支付宝登录改从支付配置取密钥

**Files:**
- Modify: `server/internal/user/alipay_auth.go`
- Modify: `server/internal/user/appauth.go`（`alipayIdentity` / `AlipayAuthInfo`）
- Modify: `server/internal/user/service.go`（`SetProviders`）
- Modify: `server/internal/user/alipay_auth_test.go`（追加）
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces: `func (s *Service) SetProviders(ps *provider.Store)`；`func alipayPID(row *provider.Resolved, fallback string) string`；`alipayClient(tenantID, platform)` 优先 `provider(pay/alipay)`。

- [ ] **Step 1: 写失败测试**（追加到 `alipay_auth_test.go`）

```go
func TestAlipayPIDPrefersProviderRow(t *testing.T) {
	row := &provider.Resolved{Fields: map[string]string{"pid": "2088new"}}
	if got := alipayPID(row, "2088old"); got != "2088new" {
		t.Fatalf("got %s", got)
	}
	if got := alipayPID(&provider.Resolved{Fields: map[string]string{}}, "2088old"); got != "2088old" {
		t.Fatalf("行里没 pid 应回落, got %s", got)
	}
	if got := alipayPID(nil, "2088old"); got != "2088old" {
		t.Fatal("nil 行回落")
	}
}
```

（import `"driftbottle/internal/provider"`。）

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/user/ -run TestAlipayPID 2>&1 | tail -2`
Expected: FAIL

- [ ] **Step 3: 实现**

`service.go`：`Service` 加 `providers *provider.Store`；`func (s *Service) SetProviders(ps *provider.Store) { s.providers = ps }`。

`alipay_auth.go`：

```go
// alipayClient 优先用服务商页「支付宝」卡片的密钥(登录与支付同一应用、同一把私钥);
// 没配过再回落 app_credentials 旧行(迁移期)。
func (s *Service) alipayClient(tenantID int64, platform string) (*alipay.Client, error) {
	if s.providers != nil {
		if row, ok := s.providers.Get(tenantID, provider.KindPay, "alipay"); ok && row.Get("private_key") != "" {
			cli, err := alipay.New(row.Get("app_id"), row.Get("private_key"), row.Get("alipay_public_key"))
			if err != nil {
				return nil, err
			}
			if row.Bool("sandbox") {
				cli.Gateway = alipay.GatewaySandbox
			}
			return cli, nil
		}
	}
	if s.creds == nil {
		return nil, nil
	}
	r, ok := s.creds.ByTenantPlatform(tenantID, platform)
	if !ok || r.AlipayPrivateKeyPEM == "" {
		return nil, nil
	}
	return alipay.New(r.AppID, r.AlipayPrivateKeyPEM, r.AlipayPublicKey)
}

func alipayPID(row *provider.Resolved, fallback string) string {
	if row != nil && row.Get("pid") != "" {
		return row.Get("pid")
	}
	return fallback
}
```

`appauth.go`：`alipayIdentity` 不再强制 `appCreds(tid,"alipay_app")`——改为 `cli, err := s.alipayClient(tenantID, "alipay_app")`，`cli == nil` 才报「未配置支付宝登录」。`AlipayAuthInfo`：

```go
	cli, err := s.alipayClient(tenantID, "alipay_app")
	if err != nil || cli == nil {
		return "", errs.New(errs.CodeLoginFailed, "未配置支付宝登录")
	}
	var row *provider.Resolved
	if s.providers != nil {
		row, _ = s.providers.Get(tenantID, provider.KindPay, "alipay")
	}
	fallback := ""
	if r, ok := s.creds.ByTenantPlatform(tenantID, "alipay_app"); ok {
		fallback = r.MchID
	}
	return alipayAuthInfo(cli, alipayPID(row, fallback))
```

`main.go`：`userSvc.SetProviders(providerStore)`（在 `providerStore` 创建之后）。

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go build ./... && go test ./internal/user/ 2>&1 | tail -2`
Expected: PASS

---

### Task 7: 地图逆地理：四家 Geocoder + 按服务商分派

**Files:**
- Rewrite: `server/internal/geo/geo.go`（只留 handler）
- Create: `server/internal/geo/geocoder.go`（接口 + 腾讯 + Google）
- Create: `server/internal/geo/amap.go`、`server/internal/geo/baidu.go`
- Create: `server/internal/geo/geocoder_test.go`
- Modify: `server/cmd/api/main.go`（`geo.NewHandler(providerStore)`）

**Interfaces:**
- Produces:
  - `type Result struct{ Address, City, Place string }`
  - `type Geocoder interface{ Regeo(lat, lng, lang string) (Result, error) }`
  - `func newTencent(key, base string) Geocoder`、`func newGoogle(key, base string) Geocoder`、`func newAmap(key, base string) Geocoder`、`func newBaidu(ak, sk, base string) Geocoder`（`base` 空 = 官方地址）
  - 纯解析：`parseTencent(raw []byte) (Result, error)`、`parseGoogle(raw []byte) (Result, error)`、`parseAmap(raw []byte) (Result, error)`、`parseBaidu(raw []byte) (Result, error)`、`baiduSN(path string, params url.Values, sk string) string`
  - `func GeocoderFor(row *provider.Resolved) (Geocoder, bool)`（供 Task 9 的「测试连通」复用）
  - `func NewHandler(ps *provider.Store) *Handler`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/geo/geocoder_test.go
package geo

import (
	"net/url"
	"testing"
)

func TestParseTencent(t *testing.T) {
	raw := `{"status":0,"result":{"address":"北京市东城区东长安街","formatted_addresses":{"recommend":"天安门"},"address_component":{"city":"北京市","district":"东城区"}}}`
	r, err := parseTencent([]byte(raw))
	if err != nil || r.Address != "天安门" || r.City != "北京市" || r.Place != "东城区" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := parseTencent([]byte(`{"status":110,"message":"key error"}`)); err == nil {
		t.Fatal("非 0 status 应报错")
	}
}

func TestParseGoogle(t *testing.T) {
	raw := `{"status":"OK","results":[{"formatted_address":"Bandra West, Mumbai","address_components":[{"long_name":"Bandra West","types":["sublocality_level_1","sublocality"]},{"long_name":"Mumbai","types":["locality"]}]}]}`
	r, err := parseGoogle([]byte(raw))
	if err != nil || r.City != "Mumbai" || r.Place != "Bandra West" {
		t.Fatalf("%+v %v", r, err)
	}
}

// 高德:直辖市的 city 是空数组 [] 而不是字符串,解析不能炸,退回 province。
func TestParseAmapMunicipality(t *testing.T) {
	raw := `{"status":"1","regeocode":{"formatted_address":"北京市东城区东华门街道天安门","addressComponent":{"province":"北京市","city":[],"district":"东城区"}}}`
	r, err := parseAmap([]byte(raw))
	if err != nil || r.City != "北京市" || r.Place != "东城区" || r.Address == "" {
		t.Fatalf("%+v %v", r, err)
	}
	raw2 := `{"status":"1","regeocode":{"formatted_address":"广东省深圳市南山区","addressComponent":{"province":"广东省","city":"深圳市","district":"南山区"}}}`
	r, _ = parseAmap([]byte(raw2))
	if r.City != "深圳市" {
		t.Fatalf("普通城市 city 应为字符串: %+v", r)
	}
	if _, err := parseAmap([]byte(`{"status":"0","info":"INVALID_USER_KEY"}`)); err == nil {
		t.Fatal("status 0 应报错")
	}
}

func TestParseBaidu(t *testing.T) {
	raw := `{"status":0,"result":{"formatted_address":"北京市东城区东长安街","addressComponent":{"city":"北京市","district":"东城区","province":"北京市"}}}`
	r, err := parseBaidu([]byte(raw))
	if err != nil || r.City != "北京市" || r.Place != "东城区" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := parseBaidu([]byte(`{"status":240,"message":"APP 服务被禁用"}`)); err == nil {
		t.Fatal("非 0 status 应报错")
	}
}

// 百度 SN:md5(urlencode(path?query + sk)),query 按传入顺序;文档例子可复算。
func TestBaiduSN(t *testing.T) {
	params := url.Values{}
	params.Set("address", "百度大厦")
	params.Set("output", "json")
	params.Set("ak", "yourak")
	got := baiduSN("/geocoder/v2/", params, "yoursk")
	if len(got) != 32 {
		t.Fatalf("sn 应为 32 位 md5, got %q", got)
	}
	if got2 := baiduSN("/geocoder/v2/", params, "othersk"); got2 == got {
		t.Fatal("sk 不同 sn 应不同")
	}
}

func TestGeocoderForUnknownProvider(t *testing.T) {
	if _, ok := GeocoderFor(nil); ok {
		t.Fatal("nil 行没有 geocoder")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/geo/ 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: geocoder.go（接口 + 腾讯 + Google，从旧 geo.go 搬逻辑）**

```go
package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"driftbottle/internal/provider"
)

type Result struct{ Address, City, Place string }

// Geocoder 经纬度 → 地址。lang 只有 Google 用(跟随 Accept-Language)。
type Geocoder interface {
	Regeo(lat, lng, lang string) (Result, error)
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

func getJSON(u string) ([]byte, error) {
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GeocoderFor 服务商行 → 实现。未知服务商 / 空行返回 false。
func GeocoderFor(row *provider.Resolved) (Geocoder, bool) {
	if row == nil {
		return nil, false
	}
	switch row.Provider {
	case "tencent":
		return newTencent(row.Get("key"), ""), true
	case "google":
		return newGoogle(row.Get("key"), ""), true
	case "amap":
		return newAmap(row.Get("key"), ""), true
	case "baidu":
		return newBaidu(row.Get("ak"), row.Get("sk"), ""), true
	}
	return nil, false
}

// ---------------- 腾讯 ----------------

type tencent struct{ key, base string }

func newTencent(key, base string) Geocoder {
	if base == "" {
		base = "https://apis.map.qq.com"
	}
	return &tencent{key: key, base: base}
}

func (g *tencent) Regeo(lat, lng, _ string) (Result, error) {
	raw, err := getJSON(fmt.Sprintf("%s/ws/geocoder/v1/?location=%s,%s&key=%s", g.base, url.QueryEscape(lat), url.QueryEscape(lng), url.QueryEscape(g.key)))
	if err != nil {
		return Result{}, err
	}
	return parseTencent(raw)
}

func parseTencent(raw []byte) (Result, error) {
	var r struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Result  struct {
			Address            string `json:"address"`
			FormattedAddresses struct {
				Recommend string `json:"recommend"`
			} `json:"formatted_addresses"`
			AddressComponent struct {
				City     string `json:"city"`
				District string `json:"district"`
			} `json:"address_component"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != 0 {
		return Result{}, fmt.Errorf("腾讯位置服务 %d: %s", r.Status, r.Message)
	}
	addr := r.Result.FormattedAddresses.Recommend
	if addr == "" {
		addr = r.Result.Address
	}
	place := r.Result.AddressComponent.District
	if place == "" {
		place = r.Result.AddressComponent.City
	}
	return Result{Address: addr, City: r.Result.AddressComponent.City, Place: place}, nil
}

// ---------------- Google ----------------

type google struct{ key, base string }

func newGoogle(key, base string) Geocoder {
	if base == "" {
		base = "https://maps.googleapis.com"
	}
	return &google{key: key, base: base}
}

func (g *google) Regeo(lat, lng, lang string) (Result, error) {
	if lang == "" {
		lang = "en"
	}
	raw, err := getJSON(fmt.Sprintf("%s/maps/api/geocode/json?latlng=%s,%s&key=%s&language=%s",
		g.base, url.QueryEscape(lat), url.QueryEscape(lng), url.QueryEscape(g.key), url.QueryEscape(lang)))
	if err != nil {
		return Result{}, err
	}
	return parseGoogle(raw)
}

func parseGoogle(raw []byte) (Result, error) {
	var r struct {
		Status  string `json:"status"`
		Results []struct {
			FormattedAddress  string `json:"formatted_address"`
			AddressComponents []struct {
				LongName string   `json:"long_name"`
				Types    []string `json:"types"`
			} `json:"address_components"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != "OK" || len(r.Results) == 0 {
		return Result{}, errors.New("Google Geocoding " + r.Status)
	}
	out := Result{Address: r.Results[0].FormattedAddress}
	for _, comp := range r.Results[0].AddressComponents {
		for _, t := range comp.Types {
			if t == "locality" {
				out.City = comp.LongName
			} else if out.City == "" && t == "administrative_area_level_2" {
				out.City = comp.LongName
			}
			if out.Place == "" && (t == "sublocality" || t == "sublocality_level_1" || t == "neighborhood") {
				out.Place = comp.LongName
			}
		}
	}
	if out.Place == "" {
		out.Place = out.City
	}
	return out, nil
}
```

- [ ] **Step 4: amap.go / baidu.go**

```go
// server/internal/geo/amap.go
package geo

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type amap struct{ key, base string }

func newAmap(key, base string) Geocoder {
	if base == "" {
		base = "https://restapi.amap.com"
	}
	return &amap{key: key, base: base}
}

// Regeo 高德要 location=经度,纬度(与腾讯 / 百度相反)。
func (g *amap) Regeo(lat, lng, _ string) (Result, error) {
	raw, err := getJSON(fmt.Sprintf("%s/v3/geocode/regeo?key=%s&location=%s,%s&extensions=base",
		g.base, url.QueryEscape(g.key), url.QueryEscape(lng), url.QueryEscape(lat)))
	if err != nil {
		return Result{}, err
	}
	return parseAmap(raw)
}

// flexString 高德在直辖市把 city 给成 [](空数组)而不是字符串。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexString(s)
		return nil
	}
	*f = "" // 数组 / null 一律当空
	return nil
}

func parseAmap(raw []byte) (Result, error) {
	var r struct {
		Status    string `json:"status"`
		Info      string `json:"info"`
		Regeocode struct {
			FormattedAddress flexString `json:"formatted_address"`
			AddressComponent struct {
				Province flexString `json:"province"`
				City     flexString `json:"city"`
				District flexString `json:"district"`
			} `json:"addressComponent"`
		} `json:"regeocode"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != "1" {
		return Result{}, fmt.Errorf("高德 %s", r.Info)
	}
	ac := r.Regeocode.AddressComponent
	city := string(ac.City)
	if city == "" {
		city = string(ac.Province)
	}
	place := string(ac.District)
	if place == "" {
		place = city
	}
	return Result{Address: string(r.Regeocode.FormattedAddress), City: city, Place: place}, nil
}
```

```go
// server/internal/geo/baidu.go
package geo

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type baidu struct{ ak, sk, base string }

func newBaidu(ak, sk, base string) Geocoder {
	if base == "" {
		base = "https://api.map.baidu.com"
	}
	return &baidu{ak: ak, sk: sk, base: base}
}

// baiduSN 百度 SN 校验:md5(urlencode(path + "?" + query + sk))。query 要按拼接顺序,所以用有序 encode。
func baiduSN(path string, params url.Values, sk string) string {
	query := params.Encode() // 按 key 排序;请求时也用同一串,顺序一致即可
	raw := url.QueryEscape(path + "?" + query + sk)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (g *baidu) Regeo(lat, lng, _ string) (Result, error) {
	const path = "/reverse_geocoding/v3/"
	params := url.Values{}
	params.Set("ak", g.ak)
	params.Set("output", "json")
	params.Set("coordtype", "wgs84ll")
	params.Set("location", lat+","+lng)
	query := params.Encode()
	if g.sk != "" {
		query += "&sn=" + baiduSN(path, params, g.sk)
	}
	raw, err := getJSON(g.base + path + "?" + query)
	if err != nil {
		return Result{}, err
	}
	return parseBaidu(raw)
}

func parseBaidu(raw []byte) (Result, error) {
	var r struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Result  struct {
			FormattedAddress string `json:"formatted_address"`
			AddressComponent struct {
				Province string `json:"province"`
				City     string `json:"city"`
				District string `json:"district"`
			} `json:"addressComponent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	if r.Status != 0 {
		return Result{}, fmt.Errorf("百度地图 %d: %s", r.Status, r.Message)
	}
	ac := r.Result.AddressComponent
	city := strings.TrimSpace(ac.City)
	if city == "" {
		city = ac.Province
	}
	place := ac.District
	if place == "" {
		place = city
	}
	return Result{Address: r.Result.FormattedAddress, City: city, Place: place}, nil
}
```

- [ ] **Step 5: geo.go 只留 handler**

```go
// Package geo 逆地理编码代理(水印相机地址水印 / 发现页):前端只传经纬度,服务端持 key 调当前生效的地图服务商。
package geo

import (
	"strings"

	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/internal/provider"
	"driftbottle/pkg/apilog"

	"github.com/gin-gonic/gin"
)

type Handler struct{ providers *provider.Store }

func NewHandler(ps *provider.Store) *Handler { return &Handler{providers: ps} }

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.GET("/geo/regeo", auth, h.regeo)
}

func empty(c *gin.Context) { response.OK(c, gin.H{"address": "", "city": "", "place": ""}) }

// regeo 经纬度 → 地址。没有生效的地图服务商 / 调用失败 → 空地址(前端降级显示经纬度)。
func (h *Handler) regeo(c *gin.Context) {
	lat, lng := c.Query("lat"), c.Query("lng")
	if lat == "" || lng == "" {
		empty(c)
		return
	}
	tenantID := middleware.TenantID(c)
	row, ok := h.providers.Active(tenantID, provider.KindMap)
	if !ok {
		empty(c)
		return
	}
	g, ok := GeocoderFor(row)
	if !ok {
		empty(c)
		return
	}
	lang := "en"
	if al := c.GetHeader("Accept-Language"); al != "" {
		if i := strings.IndexAny(al, ",;"); i > 0 {
			lang = strings.TrimSpace(al[:i])
		} else {
			lang = strings.TrimSpace(al)
		}
	}
	res, err := g.Regeo(lat, lng, lang)
	if err != nil {
		apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, -1, err.Error(), false)
		empty(c)
		return
	}
	apilog.Record(tenantID, "geo_"+row.Provider, lat+","+lng, 0, res.Address, true)
	response.OK(c, gin.H{"address": res.Address, "city": res.City, "place": res.Place})
}
```

`main.go`：`geo.NewHandler(providerStore).Register(api, auth)`。`/app-config` 加 `"map": gin.H{"provider": <Active(tid,"map") 的 Provider 或 ""}`（sysconfig handler 加 `WithMapProvider(fn func(int64) string)`，main 注入 `func(tid) string { if r, ok := providerStore.Active(tid, provider.KindMap); ok { return r.Provider }; return "" }`）。

- [ ] **Step 6: 运行确认通过**

Run: `cd server && go build ./... && go vet ./internal/geo/ && go test ./internal/geo/ ./internal/sysconfig/ 2>&1 | tail -3`
Expected: PASS

---

### Task 8: 内容安全：Checker 抽象 + 支付宝接入

**Files:**
- Modify: `server/internal/moderation/service.go`（`providers`、`SetProviders`、`CheckUGC` / `CheckImageAsync` 分派）
- Rename/Modify: `server/internal/moderation/wxcheck.go` → 保留文件，内部改成 `wechatChecker`
- Create: `server/internal/moderation/alipay_checker.go`
- Create: `server/internal/moderation/checker_test.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `type Checker interface{ CheckText(tenantID, userID int64, scene int, text string) error; CheckImageAsync(tenantID, userID int64, mediaURL string) }`
  - `func (s *Service) SetProviders(ps *provider.Store, alipayClient func(tenantID int64) (*alipay.Client, error))`
  - `func (s *Service) checkerFor(tenantID int64) (Checker, *provider.Resolved, bool)`
  - `func alipayVerdict(node []byte) (blocked bool, err error)`（纯函数）
  - `func (s *Service) ProbeText(tenantID int64) error`（供 Task 9「测试连通」）

- [ ] **Step 1: 写失败测试**

```go
// server/internal/moderation/checker_test.go
package moderation

import "testing"

// 支付宝内容检测响应:action=REJECTED 拦,PASSED/REVIEW 放(REVIEW 交人工,不阻断用户)。
func TestAlipayVerdict(t *testing.T) {
	blocked, err := alipayVerdict([]byte(`{"code":"10000","action":"REJECTED","unique_id":"u1"}`))
	if err != nil || !blocked {
		t.Fatalf("REJECTED 应拦: %v %v", blocked, err)
	}
	blocked, err = alipayVerdict([]byte(`{"code":"10000","action":"PASSED"}`))
	if err != nil || blocked {
		t.Fatalf("PASSED 应放: %v %v", blocked, err)
	}
	blocked, _ = alipayVerdict([]byte(`{"code":"10000","action":"REVIEW"}`))
	if blocked {
		t.Fatal("REVIEW 不阻断")
	}
	if _, err := alipayVerdict([]byte(`not json`)); err == nil {
		t.Fatal("坏响应应报错(调用方放行并记日志)")
	}
}

// 没有生效的服务商 → 不在线检测,只走本地词库。
func TestCheckerForNoneWhenNoActiveProvider(t *testing.T) {
	s := New(nil)
	if _, _, ok := s.checkerFor(1); ok {
		t.Fatal("未注入 providers 时不该有 checker")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/moderation/ -run 'TestAlipayVerdict|TestCheckerFor' 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: service.go 分派**

```go
type Service struct {
	db        *gorm.DB
	sensitive []string
	tokenFn    func(tenantID int64) (string, error)
	uploadDir  string
	publicBase string

	providers    *provider.Store
	alipayClient func(tenantID int64) (*alipay.Client, error)
}

// Checker 一家内容安全服务商。
type Checker interface {
	CheckText(tenantID, userID int64, scene int, text string) error
	CheckImageAsync(tenantID, userID int64, mediaURL string)
}

// SetProviders 注入服务商配置与支付宝客户端工厂(复用支付配置里的支付宝应用)。
func (s *Service) SetProviders(ps *provider.Store, alipayClient func(int64) (*alipay.Client, error)) {
	s.providers, s.alipayClient = ps, alipayClient
}

// checkerFor 当前生效的服务商(服务商页「内容安全」单选)。
func (s *Service) checkerFor(tenantID int64) (Checker, *provider.Resolved, bool) {
	if s.providers == nil {
		return nil, nil, false
	}
	row, ok := s.providers.Active(tenantID, provider.KindModeration)
	if !ok {
		return nil, nil, false
	}
	switch row.Provider {
	case "wechat":
		return &wechatChecker{s: s}, row, true
	case "alipay":
		return &alipayChecker{s: s}, row, true
	}
	return nil, nil, false
}

// CheckUGC 本地词库 → 当前服务商的文本检测(开关开着才调)。
func (s *Service) CheckUGC(tenantID, userID int64, scene int, text string) error {
	if err := s.CheckText(text); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	ck, row, ok := s.checkerFor(tenantID)
	if !ok || !row.Bool("text_on") {
		return nil
	}
	return ck.CheckText(tenantID, userID, scene, text)
}

// CheckImageAsync C 端上传成功后调;开关开着才交给服务商。
func (s *Service) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	if mediaURL == "" {
		return
	}
	ck, row, ok := s.checkerFor(tenantID)
	if !ok || !row.Bool("image_on") {
		return
	}
	ck.CheckImageAsync(tenantID, userID, mediaURL)
}

// ProbeText 后台「测试连通」:用一句固定文案走一遍当前服务商。
func (s *Service) ProbeText(tenantID int64) error {
	ck, _, ok := s.checkerFor(tenantID)
	if !ok {
		return errors.New("没有生效的内容安全服务商")
	}
	return ck.CheckText(tenantID, 0, SceneSocial, "今天天气不错")
}
```

`wxcheck.go`：把原 `CheckUGC` / `CheckImageAsync` 删掉，`wxMsgSecCheck` 与原 `CheckImageAsync` 的 goroutine 体改挂到

```go
type wechatChecker struct{ s *Service }

func (w *wechatChecker) CheckText(tenantID, userID int64, scene int, text string) error {
	return w.s.wxMsgSecCheck(tenantID, userID, scene, text)
}
func (w *wechatChecker) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	w.s.wxMediaCheckAsync(tenantID, userID, mediaURL) // 原 CheckImageAsync 去掉开关判断后的函数体
}
```

去掉文件里对 `sysconfig.KeySecCheck*` 的引用（`VerifyWxSignature` 的 `KeySecCallbackToken` 保留）。

- [ ] **Step 4: alipay_checker.go**

```go
package moderation

import (
	"encoding/json"
	"errors"
	"log"

	"driftbottle/internal/common/errs"
	"driftbottle/pkg/apilog"
)

// alipayChecker 支付宝内容安全(alipay.security.risk.content.detect)。
// 文本与图片同一接口,图片传 URL。凭据来自支付配置里的支付宝应用(SetProviders 注入的工厂)。
//
// ⚠️ 实施时用 WebFetch 核对 biz_content 字段名:文本 {"content": "..."},图片 {"content": url, "type": "IMAGE"} 为文档现状。
type alipayChecker struct{ s *Service }

// alipayVerdict 解析响应:action=REJECTED 拦;PASSED / REVIEW 放。坏响应报错(调用方放行)。
func alipayVerdict(node []byte) (bool, error) {
	var r struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(node, &r); err != nil {
		return false, err
	}
	if r.Action == "" {
		return false, errors.New("支付宝未返回 action")
	}
	return r.Action == "REJECTED", nil
}

func (a *alipayChecker) detect(tenantID int64, biz map[string]string, kind string) (bool, error) {
	if a.s.alipayClient == nil {
		return false, errors.New("未注入支付宝客户端")
	}
	cli, err := a.s.alipayClient(tenantID)
	if err != nil || cli == nil {
		return false, errors.New("未配置支付宝应用(到服务商 → 支付页配置)")
	}
	node, err := cli.Execute("alipay.security.risk.content.detect", biz, nil)
	if err != nil {
		apilog.Record(tenantID, "alipay_content_"+kind, biz["content"], 0, err.Error(), false)
		return false, err
	}
	blocked, err := alipayVerdict(node)
	apilog.Record(tenantID, "alipay_content_"+kind, biz["content"], 0, string(node), err == nil)
	return blocked, err
}

func (a *alipayChecker) CheckText(tenantID, userID int64, scene int, text string) error {
	blocked, err := a.detect(tenantID, map[string]string{"content": text}, "text")
	if err != nil {
		log.Printf("[seccheck] alipay text err tenant=%d: %v", tenantID, err)
		return nil // API 故障放行,本地词库已兜底(与微信同策略)
	}
	if blocked {
		return errs.ErrContentBlock
	}
	return nil
}

// CheckImageAsync 支付宝是同步接口;在 goroutine 里跑,违规直接删文件。
func (a *alipayChecker) CheckImageAsync(tenantID, userID int64, mediaURL string) {
	go func() {
		blocked, err := a.detect(tenantID, map[string]string{"content": mediaURL, "type": "IMAGE"}, "image")
		if err != nil {
			log.Printf("[seccheck] alipay image err tenant=%d: %v", tenantID, err)
			return
		}
		if blocked {
			a.s.removeUploadedFile(mediaURL)
			log.Printf("[seccheck] alipay image rejected tenant=%d user=%d url=%s", tenantID, userID, mediaURL)
		}
	}()
}
```

`main.go`：`modSvc.SetProviders(providerStore, func(tid int64) (*alipay.Client, error) { return userSvc.AlipayClient(tid) })`——`user.Service` 加一个导出包装 `func (s *Service) AlipayClient(tenantID int64) (*alipay.Client, error) { return s.alipayClient(tenantID, "alipay_app") }`。

- [ ] **Step 5: 运行确认通过**

Run: `cd server && go build ./... && go vet ./internal/moderation/ && go test ./internal/moderation/ 2>&1 | tail -3`
Expected: PASS

---

### Task 9: 后台接口 `/admin/api/providers/:kind`（列表 / 保存 / 测试连通）

**Files:**
- Create: `server/internal/admin/providers.go`
- Create: `server/internal/admin/providers_test.go`
- Modify: `server/internal/admin/handler.go`
- Modify: `server/internal/admin/service.go`（`SetProviderStore`、`SetProbe`）
- Modify: `server/internal/common/i18n/catalog.go`
- Modify: `server/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `type ProviderField struct{ Key, Label, Type string; Secret, Required bool; Options []string; Value string; Help string }`（机密 `Value` 为 `"set"` / `""`）
  - `type ProviderCard struct{ Provider, Label, Platform, DocURL string; Enabled, Active, Complete bool; Missing []string; Fields []ProviderField }`
  - `func (s *Service) ListProviders(tenantID int64, kind string, lang i18n.Lang) ([]ProviderCard, error)`
  - `type ProviderSaveReq struct{ Enabled *bool; Active *bool; Fields map[string]string }`
  - `func (s *Service) SaveProvider(tenantID int64, kind, prov string, req ProviderSaveReq) error`
  - `func (s *Service) SetProviderStore(ps *provider.Store)`、`func (s *Service) SetProbe(fn func(tenantID int64, kind, prov string) (string, error))`
  - `func cardFor(d provider.Definition, row *provider.Resolved, lang i18n.Lang) ProviderCard`（纯函数）
  - 路由：`GET /providers/:kind`、`PUT /providers/:kind/:provider`、`POST /providers/:kind/:provider/test`
  - 探活分派（main.go）：`pay`→`paySvc.Probe`、`map`→`geo.GeocoderFor(row).Regeo("39.9087","116.3975","zh-CN")`、`moderation`→`modSvc.ProbeText`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/admin/providers_test.go
package admin

import (
	"testing"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/provider"
)

func TestCardForMasksSecretsAndReportsMissing(t *testing.T) {
	d, _ := provider.Find(provider.KindPay, "alipay")
	row := &provider.Resolved{TenantID: 1, Kind: provider.KindPay, Provider: "alipay", Enabled: true,
		Fields: map[string]string{"app_id": "2021x", "private_key": "SECRET"}}
	c := cardFor(d, row, i18n.En)
	if c.Label != "Alipay" || !c.Enabled || c.Complete {
		t.Fatalf("card = %+v", c)
	}
	if len(c.Missing) != 1 || c.Missing[0] != "alipay_public_key" {
		t.Fatalf("missing = %v", c.Missing)
	}
	for _, f := range c.Fields {
		switch f.Key {
		case "private_key":
			if f.Value != "set" {
				t.Errorf("机密已设置应回 set, got %q", f.Value)
			}
		case "app_id":
			if f.Value != "2021x" {
				t.Errorf("非机密回明文")
			}
		case "sandbox":
			if f.Value != "0" {
				t.Errorf("未写过的字段回默认值, got %q", f.Value)
			}
		}
	}
}

func TestCardForNoRow(t *testing.T) {
	d, _ := provider.Find(provider.KindMap, "amap")
	c := cardFor(d, nil, i18n.ZhCN)
	if c.Enabled || c.Active || c.Complete || c.Label != "高德地图" {
		t.Fatalf("card = %+v", c)
	}
	if c.Fields[0].Value != "" {
		t.Fatal("没行时机密为空串")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/admin/ -run TestCardFor 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: providers.go**

```go
package admin

import (
	"errors"
	"fmt"

	"driftbottle/internal/common/i18n"
	"driftbottle/internal/provider"
)

type ProviderField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Secret   bool     `json:"secret"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Value    string   `json:"value"` // 机密:"set" / ""
	Help     string   `json:"help,omitempty"`
}

type ProviderCard struct {
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	Platform string          `json:"platform"`
	DocURL   string          `json:"doc_url"`
	Enabled  bool            `json:"enabled"`
	Active   bool            `json:"active"`
	Complete bool            `json:"complete"`
	Missing  []string        `json:"missing"`
	Fields   []ProviderField `json:"fields"`
}

func (s *Service) SetProviderStore(ps *provider.Store) { s.providers = ps }
func (s *Service) SetProbe(fn func(int64, string, string) (string, error)) { s.probe = fn }

// cardFor schema + 当前值 → 卡片。机密只告诉前端「设没设」。
func cardFor(d provider.Definition, row *provider.Resolved, lang i18n.Lang) ProviderCard {
	fields := map[string]string{}
	if row != nil {
		fields = row.Fields
	}
	c := ProviderCard{Provider: d.Provider, Label: pick(d.LabelZh, d.LabelEn, lang), Platform: d.Platform, DocURL: d.DocURL}
	if row != nil {
		c.Enabled, c.Active = row.Enabled, row.Active
	}
	c.Missing = provider.Missing(d, fields)
	if c.Missing == nil {
		c.Missing = []string{}
	}
	c.Complete = len(c.Missing) == 0
	for _, f := range d.Fields {
		v, has := fields[f.Key]
		if !has && row == nil {
			v = f.Default
		}
		if f.Secret {
			if v != "" {
				v = "set"
			}
		}
		c.Fields = append(c.Fields, ProviderField{Key: f.Key, Label: pick(f.LabelZh, f.LabelEn, lang), Type: f.Type, Secret: f.Secret,
			Required: f.Required, Options: f.Options, Value: v, Help: pick(f.HelpZh, f.HelpEn, lang)})
	}
	return c
}

var errNeedTenant = errors.New("请先选择租户")

func (s *Service) ListProviders(tenantID int64, kind string, lang i18n.Lang) ([]ProviderCard, error) {
	if tenantID == 0 {
		return nil, errNeedTenant
	}
	defs := provider.Definitions(kind)
	if len(defs) == 0 {
		return nil, fmt.Errorf("未知的服务商领域")
	}
	tenantType := s.tenantTypeOf(tenantID)
	out := make([]ProviderCard, 0, len(defs))
	for _, d := range defs {
		if !visibleFor(d.Platform, tenantType) {
			continue
		}
		var row *provider.Resolved
		if s.providers != nil {
			row, _ = s.providers.Get(tenantID, kind, d.Provider)
		}
		out = append(out, cardFor(d, row, lang))
	}
	return out, nil
}

type ProviderSaveReq struct {
	Enabled *bool             `json:"enabled"`
	Active  *bool             `json:"active"`
	Fields  map[string]string `json:"fields"`
}

func (s *Service) SaveProvider(tenantID int64, kind, prov string, req ProviderSaveReq) error {
	if tenantID == 0 {
		return errNeedTenant
	}
	if s.providers == nil {
		return fmt.Errorf("服务商存储未初始化")
	}
	if _, ok := provider.Find(kind, prov); !ok {
		return fmt.Errorf("未知的服务商")
	}
	// 支付宝内容安全依赖支付配置里的支付宝应用:没配就不让启用,免得开了个空壳
	if kind == provider.KindModeration && prov == "alipay" && req.Enabled != nil && *req.Enabled {
		if !s.providers.Usable(tenantID, provider.KindPay, "alipay") {
			return fmt.Errorf("请先在「支付」页配置并启用支付宝")
		}
	}
	return s.providers.Upsert(tenantID, kind, prov, provider.UpsertInput{Enabled: req.Enabled, Active: req.Active, Fields: req.Fields})
}

// ProbeProvider 测试连通。分派函数由 main 注入(各域包各自实现,admin 不 import 它们)。
func (s *Service) ProbeProvider(tenantID int64, kind, prov string) (string, error) {
	if tenantID == 0 {
		return "", errNeedTenant
	}
	if s.probe == nil {
		return "", fmt.Errorf("探活未配置")
	}
	return s.probe(tenantID, kind, prov)
}
```

`Service` 结构加 `providers *provider.Store` 与 `probe func(int64, string, string) (string, error)`。

`handler.go`：

```go
	auth.GET("/providers/:kind", h.listProviders)
	auth.PUT("/providers/:kind/:provider", h.saveProvider)
	auth.POST("/providers/:kind/:provider/test", h.probeProvider)

func (h *Handler) listProviders(c *gin.Context) {
	list, err := h.svc.ListProviders(tenantFromCtx(c), c.Param("kind"), i18n.FromContext(c))
	if err != nil {
		response.FailErr(c, 1001, "查询失败", err)
		return
	}
	response.OK(c, gin.H{"cards": list})
}

func (h *Handler) saveProvider(c *gin.Context) {
	var req ProviderSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.SaveProvider(tenantFromCtx(c), c.Param("kind"), c.Param("provider"), req); err != nil {
		response.FailErr(c, 1002, "保存失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) probeProvider(c *gin.Context) {
	msg, err := h.svc.ProbeProvider(tenantFromCtx(c), c.Param("kind"), c.Param("provider"))
	if err != nil {
		response.OK(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.OK(c, gin.H{"ok": true, "message": msg})
}
```

`catalog.go` `En` 加：`"请先选择租户": "Select a tenant first"`、`"未知的服务商领域": "Unknown provider category"`、`"未知的服务商": "Unknown provider"`、`"服务商存储未初始化": "Provider store not initialised"`、`"请先在「支付」页配置并启用支付宝": "Configure and enable Alipay on the Payments page first"`、`"探活未配置": "Connectivity test not configured"`。

- [ ] **Step 4: 探活实现与 main 分派**

`pay/probe.go`：

```go
// Probe 后台「测试连通」。微信 / 支付宝查一个不存在的单号:签名、证书、网关任一错都会在这里暴露;
// 查不到单(Paid=false, err=nil)就是通。Apple 只验私钥能签出 JWT;Play 验服务账号能换到 token。
func (s *Service) Probe(tenantID int64, prov string) (string, error) {
	switch prov {
	case "wechat", "alipay":
		d, err := s.buildDriver(tenantID, prov)
		if err != nil {
			return "", err
		}
		q, ok := d.(Querier)
		if !ok {
			return "", errors.New("该渠道不支持查单")
		}
		if _, err := q.Query(&model.PayOrder{OrderNo: "PROBE" + strconv.FormatInt(time.Now().Unix(), 10)}); err != nil {
			return "", err
		}
		return "签名与网关正常", nil
	case "apple":
		row, ok := s.iapConfig(tenantID)
		if !ok {
			return "", errors.New("未启用")
		}
		if _, err := s.appStoreToken(tenantID, row.Get("bundle_id")); err != nil {
			return "", err
		}
		return ".p8 私钥可用", nil
	case "google_play":
		row, ok := s.providers.Get(tenantID, provider.KindPay, "google_play")
		if !ok {
			return "", errors.New("未配置")
		}
		if _, err := playToken(row.Get("service_account_json"), time.Now(), ""); err != nil {
			return "", err
		}
		return "服务账号可用", nil
	}
	return "", errors.New("未知渠道")
}
```

`main.go`：

```go
	adminSvc.SetProviderStore(providerStore)
	adminSvc.SetProbe(func(tid int64, kind, prov string) (string, error) {
		switch kind {
		case provider.KindPay:
			return paySvc.Probe(tid, prov)
		case provider.KindMap:
			row, ok := providerStore.Get(tid, provider.KindMap, prov)
			if !ok {
				return "", fmt.Errorf("未配置")
			}
			g, ok := geo.GeocoderFor(row)
			if !ok {
				return "", fmt.Errorf("未知服务商")
			}
			res, err := g.Regeo("39.9087", "116.3975", "zh-CN")
			if err != nil {
				return "", err
			}
			return res.Address, nil
		case provider.KindModeration:
			return "检测通过", modSvc.ProbeText(tid)
		}
		return "", fmt.Errorf("未知领域")
	})
```

（内容安全探活要先 `Active`，所以测试按钮只对「当前生效」的服务商可用——前端照此禁用。）

- [ ] **Step 5: 运行确认通过**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/admin/ ./internal/common/i18n/ ./internal/pay/ 2>&1 | tail -4`
Expected: PASS（i18n 覆盖测试通过）

---

### Task 10: 后台页面：侧边栏「服务商」分组 + 通用卡片页

**Files:**
- Create: `admin/src/views/ProviderPage.vue`
- Modify: `admin/src/router.js`、`admin/src/views/Layout.vue`、`admin/src/api.js`
- Modify: `admin/src/locales/zh-CN.json`、`admin/src/locales/en.json`

- [ ] **Step 1: api.js**

```js
  getProviders: (kind) => req('GET', '/providers/' + kind),
  saveProvider: (kind, provider, data) => req('PUT', `/providers/${kind}/${provider}`, data),
  probeProvider: (kind, provider) => req('POST', `/providers/${kind}/${provider}/test`),
```

- [ ] **Step 2: router.js / Layout.vue**

router：`import ProviderPage from './views/ProviderPage.vue'`，路由 `{ path: 'providers/:kind', component: ProviderPage, meta: { titleKey: 'nav.providers' } }`。

Layout `GROUPS` 在「财务」分组后加：

```js
  {
    titleKey: 'nav.groupProviders',
    items: [
      { to: '/providers/pay', labelKey: 'nav.providersPay', icon: '💰' },
      { to: '/providers/map', labelKey: 'nav.providersMap', icon: '🗺️' },
      { to: '/providers/moderation', labelKey: 'nav.providersModeration', icon: '🛡️' },
    ],
  },
```

- [ ] **Step 3: ProviderPage.vue**

```vue
<template>
  <div>
    <p class="tip">{{ t('providers.hint.' + kind) }}</p>
    <div v-if="!tenantStore.currentTenantID" class="empty">{{ t('providers.needTenant') }}</div>
    <template v-else>
      <div v-for="card in cards" :key="card.provider" class="card pcard" :class="{ off: !card.enabled }">
        <div class="card-hd">
          <span class="pname">{{ card.label }}</span>
          <span class="badge" :class="card.complete ? 'green' : 'gray'">{{ card.complete ? t('providers.complete') : t('providers.missing', { n: card.missing.length }) }}</span>
          <span v-if="single && card.active" class="badge blue">{{ t('providers.current') }}</span>
          <a v-if="card.doc_url" class="doc" :href="card.doc_url" target="_blank" rel="noopener">{{ t('providers.apply') }}</a>
          <label class="switch">
            <input type="checkbox" :checked="card.enabled" @change="toggleEnabled(card, $event.target.checked)" />
            <span>{{ t('providers.enabled') }}</span>
          </label>
          <button v-if="single && !card.active" class="btn sm ghost" :disabled="!card.enabled" @click="setActive(card)">{{ t('providers.setCurrent') }}</button>
        </div>
        <div class="fields">
          <!-- 循环变量用 fd,不用 t —— 那是翻译函数 -->
          <div v-for="fd in card.fields" :key="fd.key" class="field" :class="{ wide: fd.type === 'textarea' }">
            <label>{{ fd.label }}<span v-if="fd.required" class="req">*</span></label>
            <template v-if="fd.type === 'bool'">
              <select v-model="draft[card.provider][fd.key]" class="ipt">
                <option value="1">{{ t('common.on') }}</option>
                <option value="0">{{ t('common.off') }}</option>
              </select>
            </template>
            <textarea v-else-if="fd.type === 'textarea'" v-model="draft[card.provider][fd.key]" class="ta" rows="4"
              :placeholder="fd.secret && fd.value === 'set' ? t('providers.secretSet') : ''"></textarea>
            <input v-else v-model="draft[card.provider][fd.key]" class="ipt" :type="fd.secret ? 'password' : 'text'"
              :placeholder="fd.secret && fd.value === 'set' ? t('providers.secretSet') : ''" />
            <small v-if="fd.help" class="help">{{ fd.help }}</small>
          </div>
        </div>
        <div class="card-ft">
          <button class="btn" :disabled="saving === card.provider" @click="save(card)">{{ t('common.save') }}</button>
          <button class="btn ghost" :disabled="probing === card.provider || !card.complete || (single && !card.active)" @click="probe(card)">{{ t('providers.test') }}</button>
          <span v-if="result[card.provider]" :class="result[card.provider].ok ? 'set' : 'unset'">{{ result[card.provider].message }}</span>
        </div>
      </div>
    </template>
    <div v-if="toast" class="gtoast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import { tenantStore } from '../tenant.js'

const { t } = useI18n()
const route = useRoute()
const kind = computed(() => route.params.kind)
const single = computed(() => kind.value === 'map' || kind.value === 'moderation')

const cards = ref([])
const draft = ref({})   // provider -> { key: value };机密字段初始为空串(= 不改)
const saving = ref('')
const probing = ref('')
const result = ref({})
const toast = ref('')

function flash(msg) { toast.value = msg; setTimeout(() => { toast.value = '' }, 3000) }

async function load() {
  if (!tenantStore.currentTenantID) { cards.value = []; return }
  try {
    const d = await api.getProviders(kind.value)
    cards.value = d.cards || []
    const next = {}
    for (const c of cards.value) {
      next[c.provider] = {}
      for (const fd of c.fields) next[c.provider][fd.key] = fd.secret ? '' : fd.value
    }
    draft.value = next
    result.value = {}
  } catch (e) { flash(e.message) }
}

onMounted(load)
watch([kind, () => tenantStore.currentTenantID], load)

async function save(card) {
  saving.value = card.provider
  try {
    await api.saveProvider(kind.value, card.provider, { fields: draft.value[card.provider] })
    flash(t('providers.saved'))
    await load()
  } catch (e) { flash(e.message) } finally { saving.value = '' }
}

async function toggleEnabled(card, enabled) {
  try {
    await api.saveProvider(kind.value, card.provider, { enabled, fields: {} })
    await load()
  } catch (e) { flash(e.message); await load() }
}

async function setActive(card) {
  try {
    await api.saveProvider(kind.value, card.provider, { active: true, fields: {} })
    await load()
  } catch (e) { flash(e.message) }
}

async function probe(card) {
  probing.value = card.provider
  try {
    const r = await api.probeProvider(kind.value, card.provider)
    result.value = { ...result.value, [card.provider]: r }
  } catch (e) {
    result.value = { ...result.value, [card.provider]: { ok: false, message: e.message } }
  } finally { probing.value = '' }
}
</script>

<style scoped>
.pcard { padding: var(--s-5); margin-bottom: var(--s-5); }
.pcard.off { opacity: .75; }
.card-hd { display: flex; align-items: center; gap: var(--s-3); flex-wrap: wrap; margin-bottom: var(--s-4); }
.pname { font-weight: 650; font-size: var(--t-3); }
.doc { font-size: var(--t-tag); color: var(--brand-text); }
.switch { margin-left: auto; display: flex; align-items: center; gap: var(--s-2); font-size: var(--t-tag); }
.fields { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: var(--s-3) var(--s-5); }
.field.wide { grid-column: 1 / -1; }
.field label { display: block; font-size: var(--t-tag); color: var(--ink-2); margin-bottom: 4px; }
.req { color: var(--danger); margin-left: 2px; }
.help { display: block; color: var(--ink-3); font-size: var(--t-tag); margin-top: 2px; }
.card-ft { display: flex; align-items: center; gap: var(--s-3); margin-top: var(--s-4); }
</style>
```

（`api.saveProvider` 的 `fields` 为 `{}` 时服务端 `mergeFields` 对未给出的键保持旧值。`common.on` / `common.off` 若 locale 里没有则一并补。）

- [ ] **Step 4: locales**（两文件各加）

| key | zh-CN | en |
|---|---|---|
| nav.groupProviders | 服务商 | Providers |
| nav.providers | 服务商配置 | Provider settings |
| nav.providersPay | 支付 | Payments |
| nav.providersMap | 地图 | Maps |
| nav.providersModeration | 内容安全 | Content safety |
| providers.hint.pay | 每个租户自行配置收款渠道。小程序租户用微信/支付宝，App 租户另有 iOS 内购与 Google Play；启用且必填齐全的渠道才会出现在客户端。 | Each tenant configures its own payment channels. Only channels that are enabled and complete show up in the client. |
| providers.hint.map | 逆地理编码（地址水印、附近）用哪家地图。同一时刻只能有一家生效。 | Which map provider serves reverse geocoding. Only one is active at a time. |
| providers.hint.moderation | 文本/图片内容安全用哪家。微信仅限小程序租户；支付宝需先在「支付」页配置支付宝应用。 | Which content-safety provider is used. WeChat is for mini-program tenants only; Alipay requires the Alipay app configured on the Payments page. |
| providers.needTenant | 请先在顶部选择一个租户 | Select a tenant at the top first |
| providers.complete | 已配置 | Configured |
| providers.missing | 缺 {n} 项 | {n} missing |
| providers.current | 当前生效 | Active |
| providers.apply | 申请入口 ↗ | Apply ↗ |
| providers.enabled | 启用 | Enabled |
| providers.setCurrent | 设为当前 | Set as active |
| providers.secretSet | 已设置，留空不修改 | Set; leave empty to keep |
| providers.test | 测试连通 | Test connection |
| providers.saved | 已保存 | Saved |
| common.on / common.off（若缺） | 开 / 关 | On / Off |

- [ ] **Step 5: 构建**

Run: `cd admin && npm run build`（PowerShell）
Expected: locale 校验通过、构建成功

---

### Task 11: 收尾：全量验证、文档、部署

- [ ] **Step 1: 服务端全量**

Run: `cd server && go build ./... && go vet ./... && go test ./...`
Expected: 全绿

- [ ] **Step 2: 旧键残留扫描**

Run: `grep -rn "KeyAppIAP\|KeyAppPayWechatEnabled\|KeyAppPayAlipayEnabled\|KeyAppMapsProvider\|KeyAppGoogleMapKey\|KeyGeoQQKey\|KeySecCheckTextOn\|KeySecCheckImageOn\|GroupAppPay\|GroupGeo\|GroupPayMP" server/ --include=*.go`
Expected: 无输出

- [ ] **Step 3: 后台**

Run: `cd admin && npm run build`
Expected: 通过

- [ ] **Step 4: 文档**

`CLAUDE.md`「关键机制速查」加一条：

> - **服务商配置**（2026-10-06）：支付 / 地图 / 内容安全的服务商凭据统一在 `provider_configs`（后台「服务商」三页，按租户），schema 在 `internal/provider/schema.go`；加一家服务商 = 加一个 `Definition` + 对应域的适配器。`app_credentials` 只管登录；旧 sysconfig 支付/地图/审核键已删，启动时 `provider.Migrate` 一次性搬值。

- [ ] **Step 5: 部署**

按 CLAUDE.md：`cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api && cd .. && node deploy.js && node deploy_admin.js`（PowerShell）。部署后：`journalctl -u driftbottle -n 50 | grep provider-migrate` 核对迁移日志；`curl https://ambertu.com/message/api/app-config` 看 `pay` 四键与 `map.provider`；后台三页能读写。
