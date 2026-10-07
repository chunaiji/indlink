# SSO 登录服务商化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 App 第三方登录（微信 / 支付宝 / Google / Apple）的凭证与开关并入 `provider_configs`，成为第四个域 `sso`；`/app-config` 按「启用且必填齐全」下发，两个 Flutter 工程的登录页按下发结果显示按钮。

**Architecture:** 给 `provider.Definition` 加一个可选的 `DependsOn`，让「支付宝登录的密钥来自支付卡片」这件事能被 `Usable` 判定；新增 `sso` 域的四张卡片；用独立标记 `sso_migrated` 做一次性迁移；`/app-config` 的 `auth` 段改为注入式查服务商，和已有的 `pay` 段同一个写法。

**Tech Stack:** Go 1.2x + Gin + GORM v2（`server/`）、Vue 3 + Vite（`admin/`）、Flutter 3.47 + Riverpod 3（`app/bottles`、`app/bottles_zh`）

**Spec:** `docs/superpowers/specs/2026-10-06-sso-provider-configs-design.md`

## Global Constraints

- 新 sysconfig 键必须同步写 `defaults`（`internal/sysconfig/sysconfig.go`），空串会让开关逻辑反转。**本计划只删键不加键**，删除时 `defaults` 与 `internal/admin/meta.go` 的 `configMeta` 条目必须同时删，`meta_test.go` 的 `TestConfigMetaIsComplete` 两个方向都查。
- 服务商域常量用 `provider.KindSSO = "sso"`；四家都是 `provider.PlatformApp`。
- `SingleActive` 不含 `sso`——四家并存，和支付一样由客户端选渠道。
- 机密字段 `Secret: true` 后台只回 `"set"`，写空串表示「不改」（`store.go` 的 `mergeFields`）。
- 后台两个语言文件 `admin/src/locales/zh-CN.json` 与 `en.json` 的 key 必须完全一致，`npm run build` 的前置 `scripts/check-locales.mjs` 会卡住不一致。
- `admin` 包里新增的中文错误串必须同步进 `internal/common/i18n/catalog.go`，`i18n_coverage_test.go` 的 AST 扫描会点名。
- ID 一律字符串下发（json tag 加 `,string`）。
- 多租户：所有查询按 `tenant_id` 过滤。
- `/app-config` 免鉴权时回落 **App 租户**（`appTenantOf`），不是 `tenantOf` 的小程序租户。
- `google_client_id` 是**逗号分隔多值**，第一个按约定是 Web client ID；下发给客户端的仍只有第一个（`firstCSV`）。
- 迁移标记用 `sso_migrated`，**不能**复用 `provider_migrated`——线上后者已是 `"1"`，复用等于迁移永不执行。
- 回复用中文；代码 / 标识符 / commit message 用英文（conventional commits）。

## Review Focus

1. **某租户 `pay/alipay` 的私钥被删，但 `sso/alipay` 仍启用** → 登录页必须立刻不显示支付宝按钮，而不是显示一个点了报签名错误的按钮。（Task 1 的 `MissingWith` + Task 2 的依赖声明）
2. **`DependsOn` 指向一个不存在的卡片（schema 写错）** → `Missing` / `Usable` 不能 panic，否则整个后台服务商页和 `/app-config` 一起挂。（Task 1 测试）
3. **迁移在已经 `provider_migrated=1` 的线上库执行** → `sso_migrated` 必须独立判定并真的执行。（Task 3 测试）
4. **迁移跑第二遍** → 不能覆盖运营在新页面手工改过的值。（Task 3 测试）
5. **四家全未配置时打开 App** → 登录页只剩手机号 / 邮箱，不能出现一个按钮都没有的死页面，也不能崩。（Task 7、Task 8 的 widget 测试）

---

### Task 1: `DependsOn` —— 卡片之间的凭证依赖

**Files:**
- Modify: `server/internal/provider/schema.go`
- Modify: `server/internal/provider/store.go:197-204`
- Test: `server/internal/provider/schema_test.go`

**Interfaces:**
- Produces:
  - `type Ref struct{ Kind, Provider string }`
  - `Definition.DependsOn *Ref`（新字段）
  - `func MissingWith(d Definition, fields, depFields map[string]string) []string`
  - `Store.Usable` 行为变更：声明了 `DependsOn` 的卡片，被依赖卡片缺必填时也算不可用

- [ ] **Step 1: 写失败测试**

```go
// 追加到 server/internal/provider/schema_test.go

// 依赖判定:被依赖卡片缺必填时,本卡片也算没配齐。
// 没有这条,运营在支付页删了支付宝私钥,登录页还会露出支付宝按钮,
// 用户点进去拿到的是「签名错误」——而后台两张卡片都显示绿色。
func TestMissingWithReportsDependencyGaps(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "alipay",
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"},
		Fields:    []Field{{Key: "pid", Required: true}},
	}
	// 自己齐了,被依赖的一个字段都没有
	miss := MissingWith(d, map[string]string{"pid": "2088x"}, nil)
	if len(miss) == 0 {
		t.Fatal("被依赖卡片没配时必须报缺项")
	}
	for _, m := range miss {
		if !strings.HasPrefix(m, "pay/alipay.") {
			t.Errorf("被依赖卡片的缺项要带来源前缀,便于后台提示去哪配: %q", m)
		}
	}
	// 两边都齐
	dep := map[string]string{"app_id": "2021", "private_key": "P", "alipay_public_key": "K"}
	if got := MissingWith(d, map[string]string{"pid": "2088x"}, dep); len(got) != 0 {
		t.Fatalf("两边都齐时不该缺: %v", got)
	}
	// 自己缺,报自己的,不带前缀
	got := MissingWith(d, nil, dep)
	if len(got) != 1 || got[0] != "pid" {
		t.Fatalf("自己的缺项不带前缀: %v", got)
	}
}

// schema 写错把 DependsOn 指到不存在的卡片上,不能 panic——
// 那会让后台服务商页和 /app-config 一起挂,而不只是这一张卡片不可用。
func TestMissingWithSurvivesUnknownDependency(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "x",
		DependsOn: &Ref{Kind: "nope", Provider: "nope"},
		Fields:    []Field{{Key: "k", Required: true}},
	}
	got := MissingWith(d, map[string]string{"k": "v"}, nil)
	if len(got) != 0 {
		t.Fatalf("依赖的卡片定义不存在时按「无依赖」处理,got %v", got)
	}
}

// 老的 Missing 不变:只看自己的字段。现有调用方(后台卡片、Usable)都还在用它。
func TestMissingStillIgnoresDependencies(t *testing.T) {
	d := Definition{
		Kind: "sso", Provider: "alipay",
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"},
		Fields:    []Field{{Key: "pid", Required: true}},
	}
	if got := Missing(d, map[string]string{"pid": "2088x"}); len(got) != 0 {
		t.Fatalf("Missing 只管自己的字段: %v", got)
	}
}
```

测试文件顶部的 import 需要 `"strings"`，若尚未引入请加上。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/provider/ -run 'TestMissingWith|TestMissingStill' 2>&1 | tail -5`
Expected: FAIL（`Ref` / `DependsOn` / `MissingWith` 未定义）

- [ ] **Step 3: 实现 schema.go**

在 `Definition` 结构体里加字段（紧跟 `DocURL`）：

```go
	DocURL   string
	// DependsOn 本卡片的凭证来自另一张卡片(同租户)。
	//
	// 用于「同一个第三方应用同时服务两个域」的情形:支付宝登录与支付是同一个
	// 应用、同一把私钥(internal/user/alipay_auth.go 一直这么取)。让登录卡片
	// 自己再存一份私钥,运营就要填两遍,迟早填歪一个——而症状是「支付好的,
	// 登录报签名错误」。
	//
	// 只支持一跳:被依赖的卡片自己不能再声明 DependsOn(TestDefinitionsAreConsistent 守着)。
	DependsOn *Ref
```

在 `Definition` 之前加类型：

```go
// Ref 指向另一张卡片。
type Ref struct{ Kind, Provider string }
```

在 `Missing` 之后加：

```go
// MissingWith 自己的缺项,外加被依赖卡片的缺项(后者带 "<kind>/<provider>." 前缀,
// 让后台能提示运营该去哪一页补)。depFields 传 nil 表示被依赖的卡片从没配过。
//
// 依赖的卡片定义不存在时按「没有依赖」处理:schema 写错只该让这张卡片失去保护,
// 不该把整个服务商页和 /app-config 一起打挂。
func MissingWith(d Definition, fields, depFields map[string]string) []string {
	miss := Missing(d, fields)
	if d.DependsOn == nil {
		return miss
	}
	dep, ok := Find(d.DependsOn.Kind, d.DependsOn.Provider)
	if !ok {
		return miss
	}
	prefix := d.DependsOn.Kind + "/" + d.DependsOn.Provider + "."
	for _, k := range Missing(dep, depFields) {
		miss = append(miss, prefix+k)
	}
	return miss
}
```

- [ ] **Step 4: 让 `Usable` 用上它**

`server/internal/provider/store.go` 的 `Usable` 改成：

```go
// Usable enabled && 必填齐全(含被依赖卡片的必填)。/app-config 与下单前置判断都用它。
func (s *Store) Usable(tid int64, kind, provider string) bool {
	r, ok := s.Get(tid, kind, provider)
	if !ok || !r.Enabled {
		return false
	}
	d, ok := Find(kind, provider)
	if !ok {
		return false
	}
	var depFields map[string]string
	if d.DependsOn != nil {
		if dr, ok := s.Get(tid, d.DependsOn.Kind, d.DependsOn.Provider); ok {
			depFields = dr.Fields
		}
	}
	return len(MissingWith(d, r.Fields, depFields)) == 0
}
```

- [ ] **Step 5: 运行确认通过**

Run: `cd server && gofmt -l internal/provider; go vet ./internal/provider/ && go test ./internal/provider/ -count=1 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add server/internal/provider/schema.go server/internal/provider/store.go server/internal/provider/schema_test.go
git commit -m "feat(provider): let a card declare that its credentials live on another card"
```

---

### Task 2: `sso` 域的四张卡片

**Files:**
- Modify: `server/internal/provider/schema.go`
- Test: `server/internal/provider/schema_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Ref` / `DependsOn` / `MissingWith`
- Produces: `provider.KindSSO = "sso"`；四个 `Definition`：`wechat` / `alipay` / `google` / `apple`

- [ ] **Step 1: 写失败测试**

```go
// 追加到 server/internal/provider/schema_test.go

// 四家 SSO 卡片的形状。这条测的是「配置的人打开页面能不能照着填完」,
// 所以盯的是必填集合和依赖关系,不是文案。
func TestSSODefinitions(t *testing.T) {
	want := map[string][]string{
		"wechat": {"app_id", "app_secret"},
		"alipay": {"pid"},
		"google": {"client_id"},
		"apple":  {"bundle_id"},
	}
	defs := Definitions(KindSSO)
	if len(defs) != 4 {
		t.Fatalf("应有 4 家, got %d", len(defs))
	}
	for _, d := range defs {
		if d.Platform != PlatformApp {
			t.Errorf("%s 只对 App 租户显示,小程序不走这些", d.Provider)
		}
		req := []string{}
		for _, f := range d.Fields {
			if f.Required {
				req = append(req, f.Key)
			}
		}
		if got := want[d.Provider]; got == nil {
			t.Errorf("多出一家 %s", d.Provider)
		} else if strings.Join(req, ",") != strings.Join(got, ",") {
			t.Errorf("%s 必填项 = %v, want %v", d.Provider, req, got)
		}
	}
	// SSO 四家并存,不是单选
	if SingleActive(KindSSO) {
		t.Error("SSO 不能是单选:一个 App 同时提供多个登录渠道")
	}
}

// 支付宝登录的密钥来自支付卡片,自己只存 PID。
func TestSSOAlipayDependsOnPay(t *testing.T) {
	d, ok := Find(KindSSO, "alipay")
	if !ok {
		t.Fatal("没有 sso/alipay")
	}
	if d.DependsOn == nil || d.DependsOn.Kind != KindPay || d.DependsOn.Provider != "alipay" {
		t.Fatalf("sso/alipay 必须依赖 pay/alipay, got %+v", d.DependsOn)
	}
	// 自己不能再存一份私钥,否则就是两份会漂移的配置
	for _, f := range d.Fields {
		if f.Key == "private_key" || f.Key == "alipay_public_key" {
			t.Errorf("sso/alipay 不该自带 %s,它来自支付卡片", f.Key)
		}
	}
	// 登录卡片上要讲清密钥在哪配,否则运营会以为登录坏了
	if !strings.Contains(d.LabelZh+d.Fields[0].HelpZh, "支付") {
		t.Error("要在卡片上说明密钥来自支付页")
	}
}

// 微信不走依赖:开放平台移动应用登录要 appid+secret,而支付要 appid+商户号+三把密钥,
// secret 支付根本不用。两者 appid 常相同但不保证同一个应用。
func TestSSOWechatIsStandalone(t *testing.T) {
	d, _ := Find(KindSSO, "wechat")
	if d.DependsOn != nil {
		t.Error("微信登录不依赖微信支付")
	}
	var secretIsSecret bool
	for _, f := range d.Fields {
		if f.Key == "app_secret" {
			secretIsSecret = f.Secret
		}
	}
	if !secretIsSecret {
		t.Error("app_secret 必须标 Secret,否则后台会把它明文回显")
	}
}

// 只支持一跳:被依赖的卡片自己不能再声明依赖。
func TestDependenciesAreAtMostOneHop(t *testing.T) {
	for _, d := range definitions {
		if d.DependsOn == nil {
			continue
		}
		dep, ok := Find(d.DependsOn.Kind, d.DependsOn.Provider)
		if !ok {
			t.Errorf("%s/%s 依赖了不存在的 %s/%s", d.Kind, d.Provider, d.DependsOn.Kind, d.DependsOn.Provider)
			continue
		}
		if dep.DependsOn != nil {
			t.Errorf("%s/%s 的依赖又有依赖,只支持一跳", d.Kind, d.Provider)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/provider/ -run 'TestSSO|TestDependenciesAre' 2>&1 | tail -5`
Expected: FAIL（`KindSSO` 未定义）

- [ ] **Step 3: 实现**

`server/internal/provider/schema.go` 的 Kind 常量块加一行：

```go
	KindSSO        = "sso"
```

在 `definitions` 切片里，支付四家之后、地图之前插入：

```go
	// ---------------- 第三方登录(App 专属) ----------------
	{
		Kind: KindSSO, Provider: "wechat", LabelZh: "微信登录", LabelEn: "WeChat sign-in", Platform: PlatformApp,
		DocURL: "https://open.weixin.qq.com",
		Fields: []Field{
			{Key: "app_id", LabelZh: "开放平台移动应用 AppID", LabelEn: "Open Platform app AppID", Type: "text", Required: true,
				HelpZh: "微信开放平台「移动应用」的 AppID,不是小程序的;要通过应用审核才能用",
				HelpEn: "AppID of the WeChat Open Platform mobile app, not the mini-program; the app must pass review first"},
			{Key: "app_secret", LabelZh: "AppSecret", LabelEn: "AppSecret", Type: "text", Secret: true, Required: true},
			{Key: "universal_link", LabelZh: "Universal Link(iOS 必填)", LabelEn: "Universal Link (required on iOS)", Type: "text",
				HelpZh: "iOS 微信 SDK 必填,安卓忽略。不填只影响 iOS,安卓登录照常",
				HelpEn: "Required by the iOS WeChat SDK and ignored on Android; leaving it empty only breaks iOS"},
		},
	},
	{
		Kind: KindSSO, Provider: "alipay", LabelZh: "支付宝登录(密钥来自支付页)", LabelEn: "Alipay sign-in (keys come from the payment page)", Platform: PlatformApp,
		DependsOn: &Ref{Kind: KindPay, Provider: "alipay"}, DocURL: "https://open.alipay.com",
		Fields: []Field{
			{Key: "pid", LabelZh: "商户 PID(2088 开头)", LabelEn: "Partner ID (2088…)", Type: "text", Required: true,
				HelpZh: "App 授权登录签名要用。应用 AppID 与私钥与「支付 → 支付宝」共用同一个应用,在那边配",
				HelpEn: "Needed to sign the app sign-in request. The app ID and private key are shared with Providers → Payment → Alipay; set them there"},
		},
	},
	{
		Kind: KindSSO, Provider: "google", LabelZh: "Google 登录", LabelEn: "Google sign-in", Platform: PlatformApp,
		DocURL: "https://console.cloud.google.com/apis/credentials",
		Fields: []Field{
			{Key: "client_id", LabelZh: "Client ID(多个用逗号分隔)", LabelEn: "Client ID (comma-separated for several)", Type: "text", Required: true,
				HelpZh: "校验 Google ID Token 的 aud。安卓还要用第一个当 serverClientId,所以第一个必须是 Web client ID",
				HelpEn: "Audiences accepted when verifying the Google ID token. Android also uses the first one as serverClientId, so put the Web client ID first"},
		},
	},
	{
		Kind: KindSSO, Provider: "apple", LabelZh: "Apple 登录", LabelEn: "Apple sign-in", Platform: PlatformApp,
		DocURL: "https://developer.apple.com/account/resources/identifiers",
		Fields: []Field{
			{Key: "bundle_id", LabelZh: "Bundle ID(多个用逗号分隔)", LabelEn: "Bundle ID (comma-separated for several)", Type: "text", Required: true,
				HelpZh: "⚠️ 关掉这一家在 iOS 上有下架风险:App Store 审核指南 4.8 要求,只要提供了任何第三方登录,就必须同时提供 Apple 登录",
				HelpEn: "⚠️ Turning this off risks iOS rejection: App Store guideline 4.8 requires Sign in with Apple whenever any other third-party sign-in is offered"},
		},
	},
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && gofmt -l internal/provider; go vet ./internal/provider/ && go test ./internal/provider/ -count=1 2>&1 | tail -3`
Expected: PASS（`TestDefinitionsAreConsistent` 会自动覆盖新定义）

- [ ] **Step 5: 提交**

```bash
git add server/internal/provider/schema.go server/internal/provider/schema_test.go
git commit -m "feat(provider): add the sso kind with wechat, alipay, google and apple"
```

---

### Task 3: 一次性迁移 `MigrateSSO`

**Files:**
- Create: `server/internal/provider/migrate_sso.go`
- Create: `server/internal/provider/migrate_sso_test.go`

**Interfaces:**
- Consumes: Task 2 的 `KindSSO` 与四张卡片；既有的 `legacyRow`（`migrate.go:36-41`）、`planned`、`Store.Upsert`
- Produces:
  - `func planSSOMigration(creds []legacyRow, cfg func(tid int64, key string) string, tenantIDs []int64) []planned`
  - `func MigrateSSO(db *gorm.DB, creds []*tenant.Resolved, store *Store) error`
  - 完成标记 `ssoMigratedFlag = "sso_migrated"`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/provider/migrate_sso_test.go
package provider

import "testing"

func ssoPlanOf(t *testing.T, got []planned, tenant int64, prov string) planned {
	t.Helper()
	for _, p := range got {
		if p.TenantID == tenant && p.Kind == KindSSO && p.Provider == prov {
			return p
		}
	}
	t.Fatalf("没搬出 tenant=%d sso/%s, got %+v", tenant, prov, got)
	return planned{}
}

// 微信:凭证行的 appid/secret + sysconfig 的 universal link 与开关,一次搬齐。
// 漏任何一项都等于线上微信登录当场坏掉。
func TestPlanSSOMigrationWechat(t *testing.T) {
	creds := []legacyRow{{TenantID: 7, Platform: "wx_app", AppID: "wxabc", Secret: "S3CR3T"}}
	cfg := func(_ int64, k string) string {
		switch k {
		case "app_wechat_universal_link":
			return "https://ambertu.com/app/"
		case "app_login_wechat_enabled":
			return "1"
		}
		return ""
	}
	p := ssoPlanOf(t, planSSOMigration(creds, cfg, []int64{7}), 7, "wechat")
	if p.Fields["app_id"] != "wxabc" || p.Fields["app_secret"] != "S3CR3T" {
		t.Errorf("凭证没搬全: %+v", p.Fields)
	}
	if p.Fields["universal_link"] != "https://ambertu.com/app/" {
		t.Errorf("universal link 没搬: %+v", p.Fields)
	}
	if !p.Enabled {
		t.Error("原开关是开的,搬完也要是开的")
	}
}

// 开关原本是关的,搬完必须还是关的——否则迁移当天所有租户的微信按钮集体冒出来。
func TestPlanSSOMigrationKeepsSwitchOff(t *testing.T) {
	creds := []legacyRow{{TenantID: 7, Platform: "wx_app", AppID: "wxabc", Secret: "S"}}
	cfg := func(_ int64, k string) string {
		if k == "app_login_wechat_enabled" {
			return "0"
		}
		return ""
	}
	if p := ssoPlanOf(t, planSSOMigration(creds, cfg, []int64{7}), 7, "wechat"); p.Enabled {
		t.Error("原开关是关的,不能搬成开")
	}
}

// Google / Apple 历史上没有开关,按「配了值就算开」推断;没配值就根本不建卡片
// (建一张空卡片会让后台出现一个永远红的「缺 1 项」badge)。
func TestPlanSSOMigrationInfersGoogleAndApple(t *testing.T) {
	cfg := func(_ int64, k string) string {
		if k == "app_google_client_id" {
			return "web.apps.googleusercontent.com,ios.apps.googleusercontent.com"
		}
		return "" // apple bundle id 没配
	}
	got := planSSOMigration(nil, cfg, []int64{7})
	g := ssoPlanOf(t, got, 7, "google")
	if g.Fields["client_id"] != "web.apps.googleusercontent.com,ios.apps.googleusercontent.com" {
		t.Errorf("CSV 多值要原样搬: %q", g.Fields["client_id"])
	}
	if !g.Enabled {
		t.Error("配了 client id 就该推断为开")
	}
	for _, p := range got {
		if p.Provider == "apple" {
			t.Error("apple 没配 bundle id,不该建卡片")
		}
	}
}

// 支付宝:PID 来自凭证行的 MchID。密钥不搬,它本来就在支付卡片上。
func TestPlanSSOMigrationAlipayTakesPIDOnly(t *testing.T) {
	creds := []legacyRow{{TenantID: 7, Platform: "alipay_app", AppID: "2021x", MchID: "2088pid"}}
	cfg := func(_ int64, k string) string {
		if k == "app_login_alipay_enabled" {
			return "1"
		}
		return ""
	}
	p := ssoPlanOf(t, planSSOMigration(creds, cfg, []int64{7}), 7, "alipay")
	if p.Fields["pid"] != "2088pid" {
		t.Errorf("PID 没搬: %+v", p.Fields)
	}
	if _, has := p.Fields["private_key"]; has {
		t.Error("私钥不该搬到登录卡片,它在支付卡片上")
	}
}

// 没有任何来源的租户不产生任何计划。
func TestPlanSSOMigrationEmptyTenantProducesNothing(t *testing.T) {
	if got := planSSOMigration(nil, func(int64, string) string { return "" }, []int64{7}); len(got) != 0 {
		t.Fatalf("什么都没配的租户不该产生计划: %+v", got)
	}
}

// 只看 App 专用的两行,小程序的 wx / alipay 行不碰。
func TestPlanSSOMigrationIgnoresMiniProgramRows(t *testing.T) {
	creds := []legacyRow{{TenantID: 7, Platform: "wx", AppID: "wxmini", Secret: "S"}}
	if got := planSSOMigration(creds, func(int64, string) string { return "" }, []int64{7}); len(got) != 0 {
		t.Fatalf("小程序凭证行不该被搬: %+v", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/provider/ -run TestPlanSSOMigration 2>&1 | tail -5`
Expected: FAIL（`planSSOMigration` 未定义）

- [ ] **Step 3: 实现 `migrate_sso.go`**

```go
package provider

import (
	"log"
	"strings"

	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
	"driftbottle/internal/tenant"

	"gorm.io/gorm"
)

// 旧位置的键名。和 migrate.go 一样,迁移脚本是它们最后的读者。
const (
	legacyWechatLoginOn = "app_login_wechat_enabled"
	legacyAlipayLoginOn = "app_login_alipay_enabled"
	legacyUniversalLink = "app_wechat_universal_link"
	legacyGoogleClient  = "app_google_client_id"
	legacyAppleBundle   = "app_apple_bundle_id"

	// ssoMigratedFlag 独立于 provider_migrated:线上后者早已是 "1",
	// 复用它等于这次迁移永远不执行,而且不报错——所有租户的登录配置集体为空。
	ssoMigratedFlag = "sso_migrated"
)

// onOrDefault 旧开关缺省视为开(原 defaults 就是 "1");显式 "0" 才是关。
func onOrDefault(v string) bool { return strings.TrimSpace(v) != "0" }

// planSSOMigration 纯函数:旧凭证行 + 旧 sysconfig → 要写的 sso 卡片。
// 没有来源的渠道不建卡片——空卡片会在后台挂一个永远红的「缺 1 项」。
func planSSOMigration(creds []legacyRow, cfg func(tid int64, key string) string, tenantIDs []int64) []planned {
	byTenant := map[int64]map[string]legacyRow{}
	for _, c := range creds {
		if c.Platform != "wx_app" && c.Platform != "alipay_app" {
			continue // 小程序的 wx / alipay 行原地不动
		}
		if byTenant[c.TenantID] == nil {
			byTenant[c.TenantID] = map[string]legacyRow{}
		}
		byTenant[c.TenantID][c.Platform] = c
	}

	var out []planned
	for _, tid := range tenantIDs {
		rows := byTenant[tid]

		if wx, ok := rows["wx_app"]; ok && wx.AppID != "" {
			out = append(out, planned{
				TenantID: tid, Kind: KindSSO, Provider: "wechat",
				Enabled: onOrDefault(cfg(tid, legacyWechatLoginOn)),
				Fields: map[string]string{
					"app_id":         wx.AppID,
					"app_secret":     wx.Secret,
					"universal_link": cfg(tid, legacyUniversalLink),
				},
			})
		}
		if ali, ok := rows["alipay_app"]; ok && ali.MchID != "" {
			out = append(out, planned{
				TenantID: tid, Kind: KindSSO, Provider: "alipay",
				Enabled: onOrDefault(cfg(tid, legacyAlipayLoginOn)),
				Fields:  map[string]string{"pid": ali.MchID},
			})
		}
		// Google / Apple 历史上没有开关,配了值就视为开。
		if v := strings.TrimSpace(cfg(tid, legacyGoogleClient)); v != "" {
			out = append(out, planned{
				TenantID: tid, Kind: KindSSO, Provider: "google",
				Enabled: true, Fields: map[string]string{"client_id": v},
			})
		}
		if v := strings.TrimSpace(cfg(tid, legacyAppleBundle)); v != "" {
			out = append(out, planned{
				TenantID: tid, Kind: KindSSO, Provider: "apple",
				Enabled: true, Fields: map[string]string{"bundle_id": v},
			})
		}
	}
	return out
}

// MigrateSSO 一次性把第三方登录配置搬进 sso 域。已搬过直接返回。
// 结构与 Migrate 一一对应,只是标记和计划函数不同。
func MigrateSSO(db *gorm.DB, creds []*tenant.Resolved, store *Store) error {
	if sysconfig.GetString(0, ssoMigratedFlag) == "1" {
		return nil
	}
	var tenants []model.Tenant
	if err := db.Select("tenant_id").Find(&tenants).Error; err != nil {
		return err
	}
	ids := make([]int64, 0, len(tenants))
	for _, t := range tenants {
		ids = append(ids, t.TenantID)
	}
	rows := make([]legacyRow, 0, len(creds))
	for _, c := range creds {
		rows = append(rows, legacyRow{
			TenantID: c.TenantID, Platform: c.Platform,
			AppID: c.AppID, Secret: c.Secret, MchID: c.MchID,
		})
	}
	for _, p := range planSSOMigration(rows, sysconfig.GetString, ids) {
		// 已经有行就不覆盖:运营可能已经在新页面手工配过。
		if _, exists := store.Get(p.TenantID, p.Kind, p.Provider); exists {
			continue
		}
		en := p.Enabled
		if err := store.Upsert(p.TenantID, p.Kind, p.Provider,
			UpsertInput{Enabled: &en, Fields: p.Fields}); err != nil {
			return err
		}
		keys := make([]string, 0, len(p.Fields))
		for k := range p.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys) // 日志要能逐行对照,顺序不能随 map 乱跳
		log.Printf("[provider-migrate/sso] tenant=%d %s/%s 已迁移(enabled=%v 字段=%v)",
			p.TenantID, p.Kind, p.Provider, en, keys)
	}
	return sysconfig.Set(0, ssoMigratedFlag, "1")
}
```

> `planSSOMigration` 的 `cfg` 直接收 `sysconfig.GetString`——签名天然吻合
> （`func(int64, string) string`），和 `Migrate` 里那个带解密的闭包不同，
> 这里没有要解密的字段。import 别忘了 `"sort"`。

> `legacyRow`（`migrate.go:36-41`）现在没有 `Secret` 字段，加上：
> 把 `Platform, AppID, MchID, ...` 那行改成 `Platform, AppID, Secret, MchID, ...`。
> 它是给迁移用的投影结构，加字段不影响既有用例。

- [ ] **Step 4: 运行确认通过**

Run: `cd server && gofmt -l internal/provider; go vet ./internal/provider/ && go test ./internal/provider/ -count=1 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add server/internal/provider/migrate_sso.go server/internal/provider/migrate_sso_test.go server/internal/provider/migrate.go
git commit -m "feat(provider): one-shot migration of sso config behind its own flag"
```

---

### Task 4: `/app-config` 的 `auth` 段改读服务商

**Files:**
- Modify: `server/internal/sysconfig/handler.go:20-57`（注入点）与 `:146-182`（`auth` 段）
- Test: `server/internal/sysconfig/app_config_test.go`

**Interfaces:**
- Consumes: Task 2 的 `KindSSO`
- Produces:
  - `func (h *Handler) WithSSOUsable(fn func(tenantID int64, provider string) bool) *Handler`
  - `func (h *Handler) WithSSOField(fn func(tenantID int64, provider, key string) string) *Handler`
  - `/app-config` 的 `auth` 新增 `google`、`apple` 两个布尔

- [ ] **Step 1: 写失败测试**

```go
// 追加到 server/internal/sysconfig/app_config_test.go

// 四家都没配 → 四个布尔全 false。登录页据此只剩手机号 / 邮箱。
// 这条是「没配就不露按钮」的底线:露一个点了就报错的按钮比没有按钮更糟。
func TestAppConfigSSOAllOffWhenNothingConfigured(t *testing.T) {
	auth := authSectionWith(t, func(int64, string) bool { return false }, func(int64, string, string) string { return "" })
	for _, k := range []string{"wechat", "alipay", "google", "apple"} {
		if auth[k] != false {
			t.Errorf("%s 应为 false, got %v", k, auth[k])
		}
	}
}

// 启用且齐全的渠道才下发 true,四家一个判法。
func TestAppConfigSSOFromProviderStore(t *testing.T) {
	usable := func(_ int64, p string) bool { return p == "wechat" || p == "google" }
	field := func(_ int64, p, k string) string {
		switch p + "." + k {
		case "wechat.app_id":
			return "wxabc"
		case "wechat.universal_link":
			return "https://ambertu.com/app/"
		case "google.client_id":
			return "web.googleusercontent.com,ios.googleusercontent.com"
		}
		return ""
	}
	auth := authSectionWith(t, usable, field)
	if auth["wechat"] != true || auth["google"] != true {
		t.Errorf("启用且齐全的要 true: %+v", auth)
	}
	if auth["alipay"] != false || auth["apple"] != false {
		t.Errorf("没启用的要 false: %+v", auth)
	}
	if auth["wechat_app_id"] != "wxabc" {
		t.Errorf("wechat_app_id = %v", auth["wechat_app_id"])
	}
	if auth["wechat_universal_link"] != "https://ambertu.com/app/" {
		t.Errorf("universal link = %v", auth["wechat_universal_link"])
	}
	// 多值只下发第一个(安卓拿它当 serverClientId),行为与迁移前一致
	if auth["google_client_id"] != "web.googleusercontent.com" {
		t.Errorf("google_client_id 应只下发第一个, got %v", auth["google_client_id"])
	}
}
```

同时在该测试文件里加一个 helper（放在文件末尾）：

```go
// authSectionWith 用注入的 SSO 查询跑一次 /app-config,取出 auth 段。
func authSectionWith(t *testing.T, usable func(int64, string) bool,
	field func(int64, string, string) string) map[string]interface{} {
	t.Helper()
	h := NewHandler(1, 1).WithSSOUsable(usable).WithSSOField(field)
	body := getAppConfigFromHandler(t, h) // 既有 helper getAppConfigWith 的同款做法
	auth, _ := body["auth"].(map[string]interface{})
	if auth == nil {
		t.Fatal("响应里没有 auth 段")
	}
	return auth
}
```

> 仓库里已有 `getAppConfigWith`（`app_config_test.go:280-293`）在构造 handler 并取 body。
> 实现本任务时把它抽成 `getAppConfigFromHandler(t, h)`，两个 helper 共用，避免复制一遍 gin 的样板。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/sysconfig/ -run TestAppConfigSSO 2>&1 | tail -5`
Expected: FAIL（`WithSSOUsable` 未定义）

- [ ] **Step 3: 实现注入点**

`handler.go` 的 `Handler` 结构体里，`mapProvider` 之后加：

```go
	// ssoUsable / ssoField provider.Store 的 sso 域投影:登录渠道「启用且必填齐全」
	// 才对客户端露出。nil = 单租户部署或没接,一律视为不可用。
	//
	// sysconfig 包不 import provider——注入进来,和 payUsable 同一个路子。
	ssoUsable func(tenantID int64, provider string) bool
	ssoField  func(tenantID int64, provider, key string) string
```

`WithMapProvider` 旁边加：

```go
// WithSSOUsable 注入第三方登录渠道可用性(main.go 传 providerStore.Usable)。
func (h *Handler) WithSSOUsable(fn func(int64, string) bool) *Handler {
	h.ssoUsable = fn
	return h
}

// WithSSOField 注入第三方登录卡片的字段取值(main.go 传 providerStore.Get + Resolved.Get)。
func (h *Handler) WithSSOField(fn func(int64, string, string) string) *Handler {
	h.ssoField = fn
	return h
}

func (h *Handler) sso(tid int64, p string) bool { return h.ssoUsable != nil && h.ssoUsable(tid, p) }

func (h *Handler) ssoVal(tid int64, p, k string) string {
	if h.ssoField == nil {
		return ""
	}
	return h.ssoField(tid, p, k)
}
```

- [ ] **Step 4: 改 `auth` 段**

删掉 `handler.go:146-147` 的两行 `credAppID` 调用，把 `auth` 段换成：

```go
		// 第三方登录渠道。来源是服务商配置(后台「服务商 → 登录」页):启用且必填齐全。
		// 四家一个判法——迁移前这里是三种写法:微信判开关+凭证、支付宝判开关+**错的**凭证
		// (密钥其实在支付卡片上)、Google 和 Apple 压根不判。
		"auth": gin.H{
			// 手机号 / 邮箱登录注册开关,默认都开;App 侧两个都关按都开处理。不属于服务商。
			"phone": show(KeyAppLoginPhoneEnabled),
			"email": show(KeyAppLoginEmailEnabled),

			"wechat":                h.sso(tid, "wechat"),
			"wechat_app_id":         h.ssoVal(tid, "wechat", "app_id"), // 客户端注册微信 SDK 用
			"wechat_universal_link": h.ssoVal(tid, "wechat", "universal_link"),
			"alipay":                h.sso(tid, "alipay"),
			"apple":                 h.sso(tid, "apple"),
			"google":                h.sso(tid, "google"),
			// 多值时取第一个,约定第一个是 Web client ID(见 user.primaryClientID)。
			// 安卓的 Google 登录必须拿它当 serverClientId,否则 idToken 为 null,
			// 症状是「点了没反应」——服务端日志里连一条请求都不会有。
			"google_client_id": firstCSV(h.ssoVal(tid, "google", "client_id")),
		},
```

- [ ] **Step 5: 修既有测试**

`app_config_test.go` 里这几条依赖旧语义，按新语义改：
`TestAppConfigChannelsOffWithoutCreds`、`TestAppConfigPayFromProviderStore`、
`TestAppConfigCarriesGoogleClientID`、`TestAppConfigGoogleClientIDEmptyWhenUnset`、
`TestAppConfigFallsBackToTheAppTenant`、`TestAppConfigLoginChannelSwitches`。

改法统一：原先通过 `setCache(KeyAppGoogleClientID, …)` 和 `WithCredLookup(…)` 构造的输入，
改成通过 `WithSSOUsable` / `WithSSOField` 注入。`TestAppConfigFallsBackToTheAppTenant`
断言的是**租户回落**，把它的输入也换成注入式，断言注入函数收到的是 App 租户 ID。

- [ ] **Step 6: 运行确认通过**

Run: `cd server && gofmt -l internal/sysconfig; go vet ./internal/sysconfig/ && go test ./internal/sysconfig/ -count=1 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add server/internal/sysconfig/handler.go server/internal/sysconfig/app_config_test.go
git commit -m "feat(sysconfig): serve login channels from the sso provider cards"
```

---

### Task 5: 登录链路改读 `sso` 卡片

**Files:**
- Modify: `server/internal/user/appauth.go:173-193`（audience）、`:347-368`（微信凭证）
- Modify: `server/internal/user/alipay_auth.go:49-54`（PID）
- Test: `server/internal/user/sso_source_test.go`（新建）

**Interfaces:**
- Consumes: Task 2 的 `KindSSO` 四张卡片；既有的 `s.providers *provider.Store`
- Produces:
  - `func (s *Service) ssoField(tenantID int64, prov, key string) string`
  - `wechatIdentity` 改为从 `sso/wechat` 取 appid/secret
  - `splitClientIDs` 的输入改为 `sso/google` 的 `client_id`、`sso/apple` 的 `bundle_id`
  - `alipayPID` 改为读 `sso/alipay` 的 `pid`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/user/sso_source_test.go
package user

import (
	"testing"

	"driftbottle/internal/provider"
)

// 取值统一走 ssoField:卡片不存在、字段没填、store 为 nil,都返回空串而不是 panic。
// 这三种情况在「租户还没配登录」时每天都会发生。
func TestSSOFieldIsSafeWhenUnconfigured(t *testing.T) {
	var s Service // providers 为 nil
	if got := s.ssoField(7, "wechat", "app_id"); got != "" {
		t.Errorf("没有 store 时应返回空串, got %q", got)
	}
}

// Google 的多值 audience 契约不能因为换了来源而变:第一个仍是 Web client ID。
func TestSplitClientIDsUnchangedBySource(t *testing.T) {
	got := splitClientIDs("web.googleusercontent.com, ios.googleusercontent.com ,")
	if len(got) != 2 || got[0] != "web.googleusercontent.com" {
		t.Fatalf("CSV 解析变了: %v", got)
	}
	if primaryClientID("web.googleusercontent.com,ios.googleusercontent.com") != "web.googleusercontent.com" {
		t.Error("第一个必须是 Web client ID")
	}
}

// 支付宝 PID 改从登录卡片取,不再回落凭证行的 MchID。
func TestAlipayPIDComesFromSSOCard(t *testing.T) {
	row := &provider.Resolved{Fields: map[string]string{"pid": "2088sso"}}
	if got := alipayPID(row, "2088old"); got != "2088sso" {
		t.Fatalf("应取登录卡片的 PID, got %q", got)
	}
	if got := alipayPID(nil, "2088old"); got != "" {
		t.Fatalf("登录卡片没配就是没配,不再回落旧凭证行, got %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/user/ -run 'TestSSOField|TestAlipayPIDComes' 2>&1 | tail -5`
Expected: FAIL（`ssoField` 未定义；`alipayPID` 仍回落）

- [ ] **Step 3: 加取值辅助**

`server/internal/user/appauth.go` 里加：

```go
// ssoField 取「服务商 → 登录」卡片的某个字段。没接 store、没建卡片、没填值
// 都返回空串——调用方据此报「未配置」,不要在这里 panic。
func (s *Service) ssoField(tenantID int64, prov, key string) string {
	if s.providers == nil {
		return ""
	}
	row, ok := s.providers.Get(tenantID, provider.KindSSO, prov)
	if !ok {
		return ""
	}
	return row.Get(key)
}
```

- [ ] **Step 4: 改三处取值**

微信（`appauth.go:358-368` 的 `wechatIdentity`）：

```go
// wechatIdentity code → 身份。凭证来自「服务商 → 登录 → 微信登录」卡片。
func (s *Service) wechatIdentity(tenantID int64, code string) (*oauthResult, error) {
	appID := s.ssoField(tenantID, "wechat", "app_id")
	secret := s.ssoField(tenantID, "wechat", "app_secret")
	if appID == "" || secret == "" {
		return nil, errs.New(errs.CodeLoginFailed, "未配置微信登录")
	}
	res, err := wxAppCode2Token(tenantID, appID, secret, code)
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, err.Error())
	}
	return res, nil
}
```

Google / Apple 的 audience：把 `appauth.go:249`、`:297` 的
`sysconfig.GetString(tenantID, sysconfig.KeyAppGoogleClientID)` 换成
`s.ssoField(tenantID, "google", "client_id")`；`:262`、`:325` 的
`sysconfig.GetString(tenantID, sysconfig.KeyAppAppleBundleID)` 换成
`s.ssoField(tenantID, "apple", "bundle_id")`。`splitClientIDs` / `primaryClientID` 不动。

支付宝 PID（`alipay_auth.go:49-54`）：

```go
// alipayPID 授权登录要的商户 PID,来自「服务商 → 登录 → 支付宝登录」卡片。
// 不再回落旧凭证行:迁移已经把值搬过来了,留回落只会让「新页面改了不生效」。
func alipayPID(row *provider.Resolved, _ string) string {
	if row == nil {
		return ""
	}
	return row.Get("pid")
}
```

`appauth.go:415-435` 的 `AlipayAuthInfo` 里，把传给 `alipayPID` 的第一个参数
从 `providers.Get(tenantID, provider.KindPay, "alipay")` 改成
`providers.Get(tenantID, provider.KindSSO, "alipay")`；第二个参数（旧凭证行的
MchID）传空串即可，相关的 `creds.ByTenantPlatform(tenantID, "alipay_app")` 查询删掉。

`alipay_auth.go:20-41` 的 `alipayClient` **不动**：它取的是密钥，密钥按设计仍在支付卡片上。

- [ ] **Step 5: 运行确认通过**

Run: `cd server && gofmt -l internal/user; go build ./... && go vet ./internal/user/ && go test ./internal/user/ -count=1 2>&1 | tail -3`
Expected: PASS（`alipay_auth_test.go:82` 的 `TestAlipayPIDPrefersProviderRow` 语义已变，按新行为改名改断言）

- [ ] **Step 6: 提交**

```bash
git add server/internal/user/
git commit -m "feat(user): read sso credentials from the provider cards"
```

---

### Task 6: 装配、后台入口、删旧键

**Files:**
- Modify: `server/cmd/api/main.go`（迁移调用、注入、probe 分派）
- Modify: `server/internal/admin/service.go:629`（`credPlatforms`）
- Modify: `server/internal/admin/meta.go`（删五条 `configMeta`）
- Modify: `server/internal/sysconfig/sysconfig.go`（删五个常量与 defaults）
- Modify: `admin/src/views/Layout.vue:124-131`
- Modify: `admin/src/locales/zh-CN.json`、`admin/src/locales/en.json`
- Test: `server/internal/admin/credential_test.go`

**Interfaces:**
- Consumes: Task 3 的 `MigrateSSO`、Task 4 的 `WithSSOUsable` / `WithSSOField`

- [ ] **Step 1: 写失败测试**

```go
// 追加到 server/internal/admin/credential_test.go

// 迁移后 App 登录凭证只在「服务商 → 登录」页维护。凭证页继续收 wx_app / alipay_app
// 就会出现两个页面都能配微信,正是这次要消掉的分裂。
func TestCredPlatformsDropsAppSSORows(t *testing.T) {
	for _, p := range []string{"wx_app", "alipay_app"} {
		if validCredPlatform(p) {
			t.Errorf("%s 应已退出凭证页", p)
		}
	}
	for _, p := range []string{"wx", "alipay", "app"} {
		if !validCredPlatform(p) {
			t.Errorf("%s 仍应可用", p)
		}
	}
}
```

> 校验函数是 `validCredPlatform(p string) bool`（`admin/service.go:631`），
> `credential_test.go:5` 的 `TestValidCredPlatform` 已经在调它。

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/admin/ -run TestCredPlatformsDrops 2>&1 | tail -3`
Expected: FAIL

- [ ] **Step 3: 收紧 `credPlatforms`**

`server/internal/admin/service.go:629`：

```go
// credPlatforms 凭证行允许的平台:小程序 wx / alipay、App 租户映射 app。
// App 的第三方登录(原 wx_app / alipay_app)已迁至「服务商 → 登录」页,
// 不再从这里维护;已有的行保留但不可编辑,确认稳定后由人工清理。
var credPlatforms = map[string]bool{"wx": true, "alipay": true, "app": true}
```

- [ ] **Step 4: 删旧 sysconfig 键**

`server/internal/sysconfig/sysconfig.go` 删掉这五个常量及其 `defaults` 条目：
`KeyAppGoogleClientID`、`KeyAppAppleBundleID`、`KeyAppLoginWechatEnabled`、
`KeyAppLoginAlipayEnabled`、`KeyAppWechatUniversalLink`。
`KeyAppLoginPhoneEnabled` / `KeyAppLoginEmailEnabled` **保留**。

`server/internal/admin/meta.go` 删掉这五个键对应的 `configMeta` 条目。
`meta_secret_test.go:92-93` 的 `notSecret` 白名单里那两条一并删掉。

`server/internal/sysconfig/app_config_test.go:315-327` 的 `TestChannelSwitchDefaultsOn`
整条删除——它断言的三个 defaults 已不存在。

- [ ] **Step 5: 装配 main.go**

迁移调用，紧跟现有的 `provider.Migrate(...)` 之后：

```go
	// 第三方登录配置迁移。独立标记 sso_migrated:provider_migrated 线上早已是 "1",
	// 复用它这段永远不会跑。
	if err := provider.MigrateSSO(db, credStore.All(), providerStore); err != nil {
		log.Printf("[provider/sso] 迁移失败(不阻断启动): %v", err)
	}
```

`/app-config` 注入，在 `WithMapProvider(...)` 之后、`Register(api)` 之前：

```go
		WithSSOUsable(func(tid int64, p string) bool { return providerStore.Usable(tid, provider.KindSSO, p) }).
		WithSSOField(func(tid int64, p, k string) string {
			row, ok := providerStore.Get(tid, provider.KindSSO, p)
			if !ok {
				return ""
			}
			return row.Get(k)
		}).
```

probe 分派（`adminSvc.SetProbe` 的 switch）加一条：

```go
		case provider.KindSSO:
			// SSO 没有「调一个接口就能验」的探活:微信要用户授权码、
			// Google / Apple 验的是客户端传来的 token。只回报配置是否齐全。
			if providerStore.Usable(tid, provider.KindSSO, prov) {
				return "配置齐全;实际可用性要用 App 真实登录一次验证", nil
			}
			return "", errs.New(errs.CodeBadRequest, "配置不齐全")
```

> 这条新增的中文串要同步进 `internal/common/i18n/catalog.go`，否则
> `internal/admin` 的 AST 测试会点名。

- [ ] **Step 6: 后台侧边栏与文案**

`admin/src/views/Layout.vue` 的 `nav.groupProviders` 组里加第四条：

```js
          { to: '/providers/sso', labelKey: 'nav.providersSso', icon: '🔑' },
```

`admin/src/locales/zh-CN.json`：`nav.providersSso` = `"登录"`，
`providers.hint.sso` = `"App 第三方登录渠道。启用并填齐的渠道才会出现在 App 登录页。支付宝登录的密钥与「支付」页共用同一个应用，在那边配。"`

`admin/src/locales/en.json`：`nav.providersSso` = `"Sign-in"`，
`providers.hint.sso` = `"Third-party sign-in channels for the app. A channel appears on the login screen only when it is enabled and fully configured. Alipay sign-in shares its keys with the Payment page."`

两个文件 key 必须同增，否则 `npm run build` 的前置校验直接失败。

- [ ] **Step 7: 运行确认通过**

Run: `cd server && go build ./... && go vet ./... && go test ./... 2>&1 | grep -vE '^ok|no test files' | head -10`
Expected: 只剩 `internal/robot` 既有的随机化失败（与本计划无关）

Run: `cd admin && npm run build 2>&1 | tail -3`
Expected: 构建通过

- [ ] **Step 8: 提交**

```bash
git add server/cmd/api/main.go server/internal/admin/ server/internal/sysconfig/ server/internal/common/i18n/catalog.go admin/src/
git commit -m "feat(admin,api): wire the sso provider page and retire the old login keys"
```

---

### Task 7: 国内版 App 登录页按配置显示

**Files:**
- Modify: `app/bottles_zh/lib/core/config/remote_config.dart`
- Modify: `app/bottles_zh/lib/features/auth/login_page.dart:272-294`
- Test: `app/bottles_zh/test/remote_config_cn_test.dart`、`app/bottles_zh/test/widgets/login_channels_test.dart`

**Interfaces:**
- Consumes: Task 4 的 `auth` 段新字段 `google` / `apple`
- Produces: `AppRemoteConfig.googleLogin`、`AppRemoteConfig.appleLogin`

- [ ] **Step 1: 写失败测试**

```dart
// 追加到 app/bottles_zh/test/remote_config_cn_test.dart

  test('parses the four sso switches', () {
    final cfg = AppRemoteConfig.fromJson({
      'auth': {
        'wechat': true,
        'alipay': false,
        'google': true,
        'apple': false,
      },
    }, const AppRemoteConfig.builtin());
    expect(cfg.wechatLogin, isTrue);
    expect(cfg.alipayLogin, isFalse);
    expect(cfg.googleLogin, isTrue);
    expect(cfg.appleLogin, isFalse);
  });

  // 老服务端不下发 google / apple 时按内置值走,不能把按钮全吞掉。
  test('falls back to builtin when the server omits the new switches', () {
    const b = AppRemoteConfig.builtin();
    final cfg = AppRemoteConfig.fromJson({'auth': {'wechat': true}}, b);
    expect(cfg.googleLogin, b.googleLogin);
    expect(cfg.appleLogin, b.appleLogin);
  });
```

```dart
// 追加到 app/bottles_zh/test/widgets/login_channels_test.dart

  // 四家全关 → 一个第三方按钮都没有,但页面不能空:手机号/邮箱表单还在。
  testWidgets('no social buttons when every channel is off', (t) async {
    await pumpLoginWith(t, const {
      'wechat': false, 'alipay': false, 'google': false, 'apple': false,
    });
    expect(find.byType(BrandCircleButton), findsNothing);
    expect(find.byType(TextField), findsWidgets);
  });

  // Apple 即使配置开着,非 iOS 平台也不显示——那是平台能力,不是配置能开出来的。
  testWidgets('apple stays hidden off iOS even when enabled', (t) async {
    await pumpLoginWith(t, const {
      'wechat': false, 'alipay': false, 'google': false, 'apple': true,
    });
    expect(find.byType(BrandCircleButton), findsNothing);
  });
```

> `pumpLoginWith(tester, Map<String, Object?> auth)` 是本任务要在该测试文件里补的
> helper：按既有 `_NoThirdParty` / `_AlipayOnly` 的做法覆盖 `appConfigProvider`，
> 把传入的 map 当作 `auth` 段。补完后把既有的四个 `_Xxx` 类改用它，少四份样板。
> widget 测试默认不在 iOS 上跑，所以第二条断言的是「平台门仍然生效」。

- [ ] **Step 2: 运行确认失败**

Run（PowerShell，`cd app/bottles_zh`）: `flutter test test/remote_config_cn_test.dart test/widgets/login_channels_test.dart`
Expected: FAIL（`googleLogin` 未定义）

- [ ] **Step 3: 实现 remote_config.dart**

构造函数参数里，`alipayLogin` 之后加：

```dart
    required this.googleLogin,
    required this.appleLogin,
```

`builtin()` 初始化列表里，`alipayLogin = true` 之后加：

```dart
      googleLogin = false,
      appleLogin = true,
```

> 国内版内置默认：Google 关（境内用不上，配了才开），Apple 开（iOS 必须有）。

字段声明，`alipayLogin` 之后加：

```dart
  /// Google / Apple 登录是否可用。来源是后台「服务商 → 登录」页：启用且填齐。
  /// Apple 还要再过一道平台门（见 showAppleSignIn）——配置能关掉它，但开不出来。
  final bool googleLogin;
  final bool appleLogin;
```

`fromJson` 里，`alipayLogin:` 那行之后加：

```dart
      googleLogin: _bool(auth['google']) ?? b.googleLogin,
      appleLogin: _bool(auth['apple']) ?? b.appleLogin,
```

`toJson` 的 `'auth'` 段里加 `'google': googleLogin, 'apple': appleLogin,`（缓存要round-trip）。

- [ ] **Step 4: 实现登录页**

`login_page.dart:272-294` 的 `socials` 列表，在支付宝之后、Apple 之前插入 Google，
并把 Apple 的条件改成「平台 && 配置」：

```dart
      if (cfg.googleLogin)
        BrandCircleButton(
          mark: const GoogleMark(size: 24),
          color: Colors.white,
          label: l.loginGoogle,
          onTap: () => _oauth('google'),
        ),
      if (showAppleSignIn && cfg.appleLogin)
        BrandCircleButton(
          mark: const Icon(Icons.apple_rounded, color: Colors.white, size: 28),
          color: Colors.black,
          label: l.loginApple,
          onTap: () => _oauth('apple'),
        ),
```

国内版 ARB 里有 `loginWechat`「微信」/ `loginAlipay`「支付宝」/ `loginApple`「Apple」
这组圆钮短标签，但没有 `loginGoogle`。在 `app_zh.arb` 与 `app_en.arb` 各加一条
`"loginGoogle": "Google"`（两种语言同值，品牌名不翻译），然后 `flutter gen-l10n`。
不要改用 `authContinueWithGoogle`——那是长按钮文案「用 Google 继续」，放进圆钮会溢出。
`GoogleMark` 在 `app/bottles_zh/lib/ui/widgets/brand_marks.dart` 里已经有，直接用。

- [ ] **Step 5: 运行确认通过**

Run（`cd app/bottles_zh`）: `flutter gen-l10n && flutter test && flutter analyze`
Expected: 全部通过；No issues found（`test/widget_test.dart` 的
"App boots into the login screen when no token is stored" 是改动前就存在的失败，
用 `git stash` 确认过再放行）

- [ ] **Step 6: 提交**

```bash
git add app/bottles_zh/
git commit -m "feat(app-zh): gate every sso button on the server config"
```

---

### Task 8: 国际版 App 登录页按配置显示

**Files:**
- Modify: `app/bottles/lib/core/config/remote_config.dart`
- Modify: `app/bottles/lib/features/auth/login_page.dart:433-447`
- Test: `app/bottles/test/remote_config_test.dart`、`app/bottles/test/widgets/login_channels_test.dart`

**Interfaces:**
- Consumes: Task 4 的 `auth` 段
- Produces: 与国内版同名的 `wechatLogin` / `alipayLogin` / `googleLogin` / `appleLogin`

- [ ] **Step 1: 写失败测试**

```dart
// 追加到 app/bottles/test/remote_config_test.dart

  test('parses the four sso switches', () {
    final cfg = AppRemoteConfig.fromJson({
      'auth': {
        'wechat': false,
        'alipay': false,
        'google': true,
        'apple': true,
      },
    }, const AppRemoteConfig.builtin());
    expect(cfg.googleLogin, isTrue);
    expect(cfg.appleLogin, isTrue);
    expect(cfg.wechatLogin, isFalse);
  });

  // 老服务端不下发这四个键时按内置值走:国际版内置 Google 与 Apple 都开,
  // 行为与改动前(无条件显示)一致,升级服务端之前不会丢按钮。
  test('keeps the old behaviour when the server omits them', () {
    const b = AppRemoteConfig.builtin();
    final cfg = AppRemoteConfig.fromJson(const {'auth': {}}, b);
    expect(cfg.googleLogin, isTrue);
    expect(cfg.appleLogin, isTrue);
  });
```

```dart
// 追加到 app/bottles/test/widgets/login_channels_test.dart

  // 这条今天完全没有守护:国际版的 Google 按钮一直是无条件渲染的。
  testWidgets('google button disappears when the server turns it off', (t) async {
    await pumpLoginWith(t, const {'google': false, 'apple': false});
    expect(find.text('Continue with Google'), findsNothing);
  });

  testWidgets('google button shows when enabled', (t) async {
    await pumpLoginWith(t, const {'google': true, 'apple': false});
    expect(find.text('Continue with Google'), findsOneWidget);
  });
```

> `pumpLoginWith` 同 Task 7，在本工程的测试文件里按同样做法补一份。
> `authContinueWithGoogle` 在 `app/bottles/lib/l10n/app_en.arb:65` 是
> `"Continue with Google"`，上面的断言字面量已按它写好。

- [ ] **Step 2: 运行确认失败**

Run（`cd app/bottles`）: `flutter test test/remote_config_test.dart test/widgets/login_channels_test.dart`
Expected: FAIL（`googleLogin` 未定义）

- [ ] **Step 3: 实现 remote_config.dart**

国际版现在**没有**这四个字段，四个一起加。构造函数参数：

```dart
    required this.wechatLogin,
    required this.alipayLogin,
    required this.googleLogin,
    required this.appleLogin,
```

`builtin()` 初始化列表：

```dart
      wechatLogin = false,
      alipayLogin = false,
      googleLogin = true,
      appleLogin = true,
```

> 国际版内置默认：微信 / 支付宝关（海外用不上），Google / Apple 开——
> 与改动前「Google 无条件显示、Apple 看平台」的行为一致，升级服务端之前不丢按钮。

字段声明：

```dart
  /// 四个第三方登录渠道是否可用。来源是后台「服务商 → 登录」页：启用且填齐。
  /// Apple 还要再过一道平台门（见 showAppleSignIn）——配置能关掉它，但开不出来。
  final bool wechatLogin;
  final bool alipayLogin;
  final bool googleLogin;
  final bool appleLogin;
```

`fromJson` 的 auth 段：

```dart
      wechatLogin: _bool(auth['wechat']) ?? b.wechatLogin,
      alipayLogin: _bool(auth['alipay']) ?? b.alipayLogin,
      googleLogin: _bool(auth['google']) ?? b.googleLogin,
      appleLogin: _bool(auth['apple']) ?? b.appleLogin,
```

`toJson` 的 `'auth'` 段加这四个键。

- [ ] **Step 4: 实现登录页**

`login_page.dart:433-447`，Google 按钮加条件、Apple 条件加配置：

```dart
                          if (cfg.googleLogin)
                            AppButton(
                              label: l.authContinueWithGoogle,
                              kind: BtnKind.oauth,
                              leading: const GoogleMark(size: 20),
                              onTap: () => _oauth('google'),
                            ),
                          if (showAppleSignIn && cfg.appleLogin) ...[
                            const SizedBox(height: Dim.s2),
                            AppButton(
                              label: l.authContinueWithApple,
                              kind: BtnKind.oauth,
                              icon: Icons.apple_rounded,
                              onTap: () => _oauth('apple'),
                            ),
                          ],
```

`cfg` 若该 build 方法里还没取，加 `final cfg = ref.watch(appConfigProvider);`。

分隔线 `_OrDivider` 原本无条件显示在第三方登录之上，现在四家可能全关——
把它也改成 `if (cfg.googleLogin || (showAppleSignIn && cfg.appleLogin)) ...[` 包住，
否则会出现一条下面什么都没有的「或」分隔线。

- [ ] **Step 5: 运行确认通过**

Run（`cd app/bottles`）: `flutter test && flutter analyze`
Expected: 全部通过；No issues found（`test/widget_test.dart` 那条是既有失败）

- [ ] **Step 6: 提交**

```bash
git add app/bottles/
git commit -m "feat(app): gate every sso button on the server config"
```

---

### Task 9: 全量验证与文档

- [ ] **Step 1: 服务端全量**

Run: `cd server && go build ./... && go vet ./... && go test ./...`
Expected: 全绿，除 `internal/robot` 的 `TestBuildIdentityResponse_*` / `TestSanitizeOutgoingReply_*`
（既有的随机化断言失败，与本计划无关；用 `git stash` 确认过）

- [ ] **Step 2: 两个 Flutter 工程**

Run（各一次）: `flutter analyze && flutter test`
Expected: No issues found；除 `test/widget_test.dart` 那条既有失败外全部通过

- [ ] **Step 3: 后台**

Run: `cd admin && npm run build`
Expected: 通过（两个语言文件 key 一致性校验是前置步骤）

- [ ] **Step 4: 文档**

`CLAUDE.md`「关键机制速查」的「服务商配置」那条之后加一条：

> - **第三方登录服务商化**（2026-10-06）：微信 / 支付宝 / Google / Apple 四家的凭证与开关统一在 `provider_configs` 的 `sso` 域（后台「服务商 → 登录」页），`/app-config` 的 `auth` 段按「启用且必填齐全」下发四个布尔，两个 App 的登录页据此显示按钮。**支付宝登录的密钥不在登录卡片上**——它与支付同一个应用、同一把私钥，登录卡片用 `DependsOn` 声明依赖 `pay/alipay`，支付那边缺密钥会连带判定登录不可用。迁移标记是 `sso_migrated`，**不是** `provider_migrated`（后者线上早已是 1，复用等于永不执行）。旧键 `app_login_wechat_enabled` / `app_login_alipay_enabled` / `app_google_client_id` / `app_apple_bundle_id` / `app_wechat_universal_link` 已删；`app_credentials` 的 `wx_app` / `alipay_app` 两行不再被读，凭证页也不再接受这两个平台值。

- [ ] **Step 5: 提交**

```bash
git add CLAUDE.md
git commit -m "docs: record sso provider configs in the quick reference"
```

- [ ] **Step 6: 人工联调清单（上线前按顺序走）**

1. 部署前在预发库跑一遍，查 `provider_configs` 里 `kind='sso'` 的行数与租户数对得上。
2. 后台「服务商 → 登录」页，确认四张卡片的值与迁移前一致（微信 appid、Google client ID 逐字对）。
3. 国内版 App 冷启动，确认微信 / 支付宝按钮都在，各登录一次成功。
4. 把「支付 → 支付宝」的应用私钥清空保存，刷新 App 配置，**确认支付宝登录按钮消失**。这条验的是 `DependsOn`，务必实测。把私钥填回去，确认按钮回来。
5. 关掉「登录 → Google」，确认国际版 App 的 Google 按钮消失、Apple 还在。
6. 四家全关，确认登录页只剩手机号 / 邮箱，页面不空不崩。
7. 确认「租户凭证」页已经不能再新增 `wx_app` / `alipay_app`。
8. 全部验完把配置改回生产值。

---
