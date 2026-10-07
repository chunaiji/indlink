# 国内版 App 微信 / 支付宝 登录与支付 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `app/bottles_zh` 在 Android 上能用微信 / 支付宝登录与充值，iOS 上用微信 / 支付宝 / Apple 登录 + IAP 充值；后台能录入凭证，账号到手即通。

**Architecture:** 服务端全部加法：`app_credentials` 新平台 `wx_app` / `alipay_app`；`pay.Driver` 多两种实现（微信 APP 交易类型、支付宝 App 支付）并新增可选 `Querier` 主动查单；微信 / 支付宝登录并入既有 `loginOrCreateOAuth` 链路。App 侧只换 `oauth.dart` 与两个 `PayChannelAdapter`，六屏状态机与登录状态机不动。

**Tech Stack:** Go 1.2x（Gin + GORM）、标准库 RSA/SHA256、Flutter 3.47 + Riverpod 3、`fluwx ^6.0.4`、`tobias ^5.3.4`、Vue3 管理后台。

**Spec:** `docs/superpowers/specs/2026-10-06-app-zh-wechat-alipay-design.md`

## Global Constraints

- 项目约定：**不主动跑测试 / 脚本，每次运行需用户授权；git 操作仅在明确指示时执行**——本计划的「运行」步骤先取得用户一次性授权，「提交」步骤一律跳过（由用户自己提交）。
- 服务端改动必须是加法；海外版 `app/bottles` 一个字不改。
- sysconfig 新 key 必须同步写 `defaults`，并在 `internal/admin/meta.go` 登记中英文标签 + `Group<Xxx>` 常量。
- `/admin/api` 新增的中文错误消息必须补进 `internal/common/i18n/catalog.go`（AST 测试点名）。
- ID 一律字符串下发（`,string`）。
- App 侧新增 sysconfig 开关默认 `1`、文案默认空串（`sysconfig/app_config_test.go` 守）。
- 国内版包名 / Bundle ID：`com.ambertu.bottles.cn`；`APP_ID` 默认 `drift_app_cn`；App 显示名「漂流瓶」。
- 微信 APP 拉起参数签名串第四行是**裸 prepay_id**；支付宝签名内容**值不 URL 编码**，输出 orderStr 时**值 URL 编码**；通知验签剔除 `sign` 与 `sign_type`。
- 支付宝金额单位是元字符串两位小数（`1` 分 → `"0.01"`）；服务端订单金额单位是分。
- `app_pay_mock_enabled` 默认 `0`，mock 开着时 App 下单忽略 `channel`。
- Flutter 工程：事件 / 路由 / 布局规范见 `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md`；Tab 根之下全是全屏页。

## Review Focus

1. **支付宝通知里 `total_amount` 带小数 / 带千分位 / 空串** → 金额解析失败必须拒收而不是当 0 通过金额校验。（Task 2 `YuanToFen` 表驱动测试）
2. **同一订单在客户端轮询与渠道异步回调同时到达** → `HandleCallback` 的 `WHERE status='pending'` 已幂等；`SyncIfStale` 不得绕过它另开入账口。（仓库没有 DB 测试基建，靠 Task 5 Step 4 的代码：`SyncIfStale` 里唯一的写操作就是 `HandleCallback`，评审时盯这一点）
3. **用户在微信里取消后又在 App 里点「已完成支付」** → 页面只是再查一次服务端，不得把渠道的取消当成终态。（Task 14 `wechatPayOutcome(-2)` = cancelled 且状态机后续 `pollNow` 仍可到 paid）
4. **小程序租户误传 `channel`** → 必须 400，不能让小程序走到 App 渠道。（Task 5 `resolveChannel` 表）
5. **凭证后台改了但服务没重启** → 驱动缓存按 credstore 版本失效。（Task 1 `Version()` 测试 + Task 5 缓存失效测试）

---

### Task 1: credstore 索引按平台区分 serial + 版本号；`AppCredential.Platform` 加宽

**Files:**
- Modify: `server/internal/model/model.go:31`
- Modify: `server/internal/tenant/credstore.go`
- Create: `server/internal/tenant/credstore_test.go`

**Interfaces:**
- Produces: `func (s *Store) ByPlatformSerial(platform, serial string) (*Resolved, bool)`（**签名变更**，加 platform 参数）；`func (s *Store) Version() uint64`；包内 `func buildIndex(rows []*Resolved) (byAppID, byTenant, bySerial map[string]*Resolved)`。

- [ ] **Step 1: 写失败测试**

```go
// server/internal/tenant/credstore_test.go
package tenant

import "testing"

// 同一商户号同时服务小程序(wx)与 App(wx_app)时平台证书序列号相同,
// 单键索引会互相覆盖——回调反查租户就会查到另一个平台的驱动。
func TestBuildIndexKeepsSameSerialAcrossPlatforms(t *testing.T) {
	rows := []*Resolved{
		{TenantID: 1, Platform: "wx", AppID: "wxmini", PayPlatformSerial: "SER1"},
		{TenantID: 2, Platform: "wx_app", AppID: "wxopen", PayPlatformSerial: "SER1"},
	}
	_, _, ser := buildIndex(rows)
	if got := ser[keySerial("wx", "SER1")]; got == nil || got.TenantID != 1 {
		t.Fatalf("wx:SER1 应指向租户 1, got %+v", got)
	}
	if got := ser[keySerial("wx_app", "SER1")]; got == nil || got.TenantID != 2 {
		t.Fatalf("wx_app:SER1 应指向租户 2, got %+v", got)
	}
}

func TestBuildIndexSkipsEmptySerial(t *testing.T) {
	_, _, ser := buildIndex([]*Resolved{{TenantID: 1, Platform: "alipay_app", AppID: "2021x"}})
	if len(ser) != 0 {
		t.Fatalf("空 serial 不该进索引, got %d", len(ser))
	}
}

func TestVersionBumpsOnSwap(t *testing.T) {
	s := New(nil)
	v0 := s.Version()
	s.swap(buildIndex(nil))
	if s.Version() != v0+1 {
		t.Fatalf("swap 后版本应 +1, got %d → %d", v0, s.Version())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/tenant/ -run 'TestBuildIndex|TestVersion' -v`
Expected: FAIL（`buildIndex` / `keySerial` / `swap` / `Version` 未定义）

- [ ] **Step 3: 实现**

`server/internal/model/model.go:31` 把 `Platform string \`gorm:"size:8;uniqueIndex:uk_plat_appid"` 改为 `size:16`，注释补一句：`// wx/alipay/app/wx_app/alipay_app;从 size:8 加宽,alipay_app 10 字符会被静默截断`。

`server/internal/tenant/credstore.go`：

```go
import "sync/atomic"

type Store struct {
	db       *gorm.DB
	mu       sync.RWMutex
	byAppID  map[string]*Resolved
	byTenant map[string]*Resolved
	bySerial map[string]*Resolved // key = platform:PayPlatformSerial,供回调自动识别租户
	ver      atomic.Uint64
}

func keySerial(platform, serial string) string { return platform + ":" + serial }

// Reload 全量加载 active 凭证并解密缓存。
func (s *Store) Reload() error {
	var rows []model.AppCredential
	if err := s.db.Where("status = ?", "active").Find(&rows).Error; err != nil {
		return err
	}
	resolved := make([]*Resolved, 0, len(rows))
	for i := range rows {
		r, err := decrypt(&rows[i])
		if err != nil {
			return fmt.Errorf("解密凭证失败(appid=%s): %w", rows[i].AppID, err)
		}
		resolved = append(resolved, r)
	}
	s.swap(buildIndex(resolved))
	return nil
}

// buildIndex 纯函数:把解密后的行建成三张索引。拆出来是为了能在没有 DB 的测试里断言索引键。
func buildIndex(rows []*Resolved) (byAppID, byTenant, bySerial map[string]*Resolved) {
	byAppID = make(map[string]*Resolved, len(rows))
	byTenant = make(map[string]*Resolved, len(rows))
	bySerial = make(map[string]*Resolved, len(rows))
	for _, r := range rows {
		byAppID[keyAppID(r.Platform, r.AppID)] = r
		byTenant[keyTenant(r.TenantID, r.Platform)] = r
		if r.PayPlatformSerial != "" {
			bySerial[keySerial(r.Platform, r.PayPlatformSerial)] = r
		}
	}
	return
}

// swap 原子替换三张索引并把版本号 +1。读到不同版本的调用方(pay 的驱动缓存)据此整体失效。
func (s *Store) swap(a, t, ser map[string]*Resolved) {
	s.mu.Lock()
	s.byAppID, s.byTenant, s.bySerial = a, t, ser
	s.mu.Unlock()
	s.ver.Add(1)
}

// Version 每次 Reload 自增。
func (s *Store) Version() uint64 { return s.ver.Load() }

// ByPlatformSerial 按 平台 + 微信平台公钥 ID 反查租户(回调免 tenantId 用)。
func (s *Store) ByPlatformSerial(platform, serial string) (*Resolved, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.bySerial[keySerial(platform, serial)]
	return r, ok
}
```

`server/internal/pay/service.go` 的 `DriverBySerial` 里 `s.creds.ByPlatformSerial(serial)` 改为 `s.creds.ByPlatformSerial(platform, serial)`（Task 5 会再整体改这段，这里先让编译通过）。

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go build ./... && go test ./internal/tenant/ -v`
Expected: PASS ×3

---

### Task 2: 支付宝签名 / 网关公共包 `internal/common/alipay`

**Files:**
- Create: `server/internal/common/alipay/alipay.go`
- Create: `server/internal/common/alipay/alipay_test.go`

**Interfaces:**
- Produces:
  - `func New(appID, privateKey, alipayPublicKey string) (*Client, error)`（公钥可空=只签不验）
  - `func (c *Client) Sign(params map[string]string) (string, error)`
  - `func (c *Client) Verify(params map[string]string) error`（通知验签：剔 `sign`+`sign_type`）
  - `func (c *Client) BuildRequest(method string, bizContent any, extra map[string]string) (map[string]string, error)`（含 `sign`）
  - `func (c *Client) Execute(method string, bizContent any, extra map[string]string) (json.RawMessage, error)`
  - `func (c *Client) ParseResponse(method string, body []byte) (json.RawMessage, error)`
  - `func SignContent(params map[string]string, exclude ...string) string`、`func Encode(params map[string]string) string`
  - `func GatewayFor(appID string) string`、`func FenToYuan(fen int64) string`、`func YuanToFen(s string) (int64, error)`
  - `var HTTPClient *http.Client`、字段 `Client.Gateway`（测试可指向 httptest）

- [ ] **Step 1: 写失败测试**

```go
// server/internal/common/alipay/alipay_test.go
package alipay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testKeys(t *testing.T) (privPEM, pubPEM string, priv *rsa.PrivateKey) {
	t.Helper()
	priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}))
	return
}

func TestSignContentSortsSkipsEmptyAndExcludes(t *testing.T) {
	got := SignContent(map[string]string{
		"b": "2", "a": "1", "empty": "", "sign": "x", "sign_type": "RSA2",
	}, "sign_type")
	if got != "a=1&b=2" {
		t.Fatalf("got %q", got)
	}
	// 请求签名时 sign_type 要参与
	if got := SignContent(map[string]string{"a": "1", "sign_type": "RSA2", "sign": "x"}); got != "a=1&sign_type=RSA2" {
		t.Fatalf("got %q", got)
	}
}

func TestSignVerifyRoundtripAndTamper(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, err := New("2021000000000001", privPEM, pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	p := map[string]string{"out_trade_no": "N1", "total_amount": "0.01", "subject": "金币充值", "sign_type": "RSA2"}
	sig, err := c.Sign(p)
	if err != nil {
		t.Fatal(err)
	}
	p["sign"] = sig
	if err := c.Verify(p); err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	p["total_amount"] = "0.02"
	if err := c.Verify(p); err == nil {
		t.Fatal("篡改金额后验签应失败")
	}
}

// 后台贴进来的常常是「裸 base64 没有 PEM 头」(支付宝密钥工具的默认输出)。
func TestNewAcceptsBareBase64Keys(t *testing.T) {
	_, _, priv := testKeys(t)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if _, err := New("2021x", base64.StdEncoding.EncodeToString(pkcs8), base64.StdEncoding.EncodeToString(pkix)); err != nil {
		t.Fatal(err)
	}
	pkcs1 := x509.MarshalPKCS1PrivateKey(priv)
	if _, err := New("2021x", base64.StdEncoding.EncodeToString(pkcs1), ""); err != nil {
		t.Fatalf("PKCS1 也要收: %v", err)
	}
}

func TestEncodeURLEscapesValues(t *testing.T) {
	got := Encode(map[string]string{"biz_content": `{"a":"b c"}`, "app_id": "1"})
	if got != "app_id=1&biz_content="+url.QueryEscape(`{"a":"b c"}`) {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRequestIncludesPublicParamsAndSign(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	p, err := c.BuildRequest("alipay.trade.app.pay", map[string]string{"out_trade_no": "N1"}, map[string]string{"notify_url": "https://x/cb"})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"app_id", "method", "format", "charset", "sign_type", "timestamp", "version", "notify_url", "biz_content", "sign"} {
		if p[k] == "" {
			t.Errorf("缺少 %s", k)
		}
	}
	if p["method"] != "alipay.trade.app.pay" || p["sign_type"] != "RSA2" || p["charset"] != "utf-8" {
		t.Fatalf("公共参数不对: %+v", p)
	}
	if err := c.verify(SignContent(p), p["sign"]); err != nil {
		t.Fatalf("请求签名应能用同一把公钥验: %v", err)
	}
	// 不带 biz_content 的方法(oauth.token)
	p2, _ := c.BuildRequest("alipay.system.oauth.token", nil, map[string]string{"grant_type": "authorization_code", "code": "c1"})
	if _, has := p2["biz_content"]; has {
		t.Fatal("bizContent 为 nil 时不该有 biz_content")
	}
}

// 网关响应:验签对象是 <method>_response 节点的**原始 JSON 子串**。
func TestParseResponseVerifiesNodeAndChecksCode(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	node := `{"code":"10000","msg":"Success","trade_no":"T1","trade_status":"TRADE_SUCCESS"}`
	sig, _ := c.signContent(node)
	body := `{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`
	got, err := c.ParseResponse("alipay.trade.query", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	var r struct{ TradeNo string `json:"trade_no"` }
	_ = json.Unmarshal(got, &r)
	if r.TradeNo != "T1" {
		t.Fatalf("got %s", got)
	}
	// 篡改节点
	bad := strings.Replace(body, "T1", "T2", 1)
	if _, err := c.ParseResponse("alipay.trade.query", []byte(bad)); err == nil {
		t.Fatal("篡改后应验签失败")
	}
	// 业务失败
	node2 := `{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"交易不存在"}`
	sig2, _ := c.signContent(node2)
	if _, err := c.ParseResponse("alipay.trade.query", []byte(`{"alipay_trade_query_response":`+node2+`,"sign":"`+sig2+`"}`)); err == nil || !strings.Contains(err.Error(), "ACQ.TRADE_NOT_EXIST") {
		t.Fatalf("应带 sub_code 报错, got %v", err)
	}
	// oauth.token 成功响应没有 code 字段,不能当失败
	node3 := `{"access_token":"a","user_id":"2088x"}`
	sig3, _ := c.signContent(node3)
	if _, err := c.ParseResponse("alipay.system.oauth.token", []byte(`{"alipay_system_oauth_token_response":`+node3+`,"sign":"`+sig3+`"}`)); err != nil {
		t.Fatalf("无 code 的成功响应应通过: %v", err)
	}
}

func TestExecutePostsFormToGateway(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	var gotForm url.Values
	node := `{"code":"10000","user_id":"2088x"}`
	sig, _ := c.signContent(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`{"alipay_system_oauth_token_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	c.Gateway = srv.URL
	if _, err := c.Execute("alipay.system.oauth.token", nil, map[string]string{"grant_type": "authorization_code", "code": "c1"}); err != nil {
		t.Fatal(err)
	}
	if gotForm.Get("method") != "alipay.system.oauth.token" || gotForm.Get("code") != "c1" || gotForm.Get("sign") == "" {
		t.Fatalf("表单不对: %v", gotForm)
	}
}

func TestMoneyAndGateway(t *testing.T) {
	cases := map[int64]string{1: "0.01", 100: "1.00", 1200: "12.00", 123456: "1234.56"}
	for fen, want := range cases {
		if got := FenToYuan(fen); got != want {
			t.Errorf("FenToYuan(%d) = %s, want %s", fen, got, want)
		}
	}
	for s, want := range map[string]int64{"0.01": 1, "12.00": 1200, "12": 1200, "1234.56": 123456, "0.1": 10} {
		got, err := YuanToFen(s)
		if err != nil || got != want {
			t.Errorf("YuanToFen(%q) = %d,%v want %d", s, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "1,234.00", "-1"} {
		if _, err := YuanToFen(bad); err == nil {
			t.Errorf("YuanToFen(%q) 应报错", bad)
		}
	}
	if GatewayFor("9021000000000001") != GatewaySandbox || GatewayFor("2021000000000001") != GatewayProd {
		t.Fatal("网关选择错误")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/common/alipay/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**

```go
// server/internal/common/alipay/alipay.go
// Package alipay 支付宝开放平台最小客户端:RSA2 签名 / 验签、网关调用、金额换算。
//
// 纯标准库,不引 SDK:我们只用 4 个接口(trade.app.pay / trade.create / trade.query /
// system.oauth.token),官方 SDK 几 MB 的依赖换不来什么。pay 与 user 两个包共用。
package alipay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	GatewayProd    = "https://openapi.alipay.com/gateway.do"
	GatewaySandbox = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
)

// HTTPClient 可被测试替换。
var HTTPClient = &http.Client{Timeout: 10 * time.Second}

var cst = time.FixedZone("CST", 8*3600)

type Client struct {
	AppID   string
	Gateway string
	priv    *rsa.PrivateKey
	pub     *rsa.PublicKey // 支付宝公钥;空 = 只签不验(开发态)
}

// New 构造客户端。alipayPublicKey 可空。
func New(appID, privateKey, alipayPublicKey string) (*Client, error) {
	priv, err := ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("解析支付宝应用私钥失败: %w", err)
	}
	c := &Client{AppID: appID, Gateway: GatewayFor(appID), priv: priv}
	if strings.TrimSpace(alipayPublicKey) != "" {
		pub, err := ParsePublicKey(alipayPublicKey)
		if err != nil {
			return nil, fmt.Errorf("解析支付宝公钥失败: %w", err)
		}
		c.pub = pub
	}
	return c, nil
}

// GatewayFor 沙箱应用的 appid 以 9021 开头,走沙箱网关。
func GatewayFor(appID string) string {
	if strings.HasPrefix(appID, "9021") {
		return GatewaySandbox
	}
	return GatewayProd
}

var wsRe = regexp.MustCompile(`\s+`)

// normalizeKey 后台贴的密钥常是裸 base64(支付宝密钥工具默认输出);补上 PEM 头再解析。
func normalizeKey(s, header string) []byte {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "-----BEGIN") {
		return []byte(s)
	}
	s = wsRe.ReplaceAllString(s, "")
	var b strings.Builder
	b.WriteString("-----BEGIN " + header + "-----\n")
	for len(s) > 64 {
		b.WriteString(s[:64] + "\n")
		s = s[64:]
	}
	b.WriteString(s + "\n-----END " + header + "-----\n")
	return []byte(b.String())
}

// ParsePrivateKey 支持 PKCS#8 / PKCS#1,带或不带 PEM 头。
func ParsePrivateKey(s string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(normalizeKey(s, "PRIVATE KEY"))
	if block == nil {
		return nil, errors.New("无效的私钥")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("私钥不是 RSA 类型")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ParsePublicKey PKIX 公钥,带或不带 PEM 头。
func ParsePublicKey(s string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(normalizeKey(s, "PUBLIC KEY"))
	if block == nil {
		return nil, errors.New("无效的公钥")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥不是 RSA 类型")
	}
	return rk, nil
}

// SignContent 待签名串:按 key 升序 `k=v&k=v`,值**不**URL 编码,空值跳过,`sign` 永远剔除,
// exclude 里的键一并剔除(通知验签要剔 sign_type;请求签名不剔)。
func SignContent(params map[string]string, exclude ...string) string {
	skip := map[string]bool{"sign": true}
	for _, k := range exclude {
		skip[k] = true
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if skip[k] || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// Encode 把参数编成请求体 / orderStr:按 key 升序,值 URL 编码。
func Encode(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(params[k]))
	}
	return b.String()
}

func (c *Client) signContent(content string) (string, error) {
	h := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.priv, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Sign 对参数做 RSA2 签名(请求用:sign_type 参与)。
func (c *Client) Sign(params map[string]string) (string, error) {
	return c.signContent(SignContent(params))
}

func (c *Client) verify(content, signB64 string) error {
	if c.pub == nil {
		return errors.New("未配置支付宝公钥,无法验签")
	}
	sig, err := base64.StdEncoding.DecodeString(signB64)
	if err != nil {
		return fmt.Errorf("签名不是合法 base64: %w", err)
	}
	h := sha256.Sum256([]byte(content))
	return rsa.VerifyPKCS1v15(c.pub, crypto.SHA256, h[:], sig)
}

// Verify 异步通知验签:剔 sign 与 sign_type。
func (c *Client) Verify(params map[string]string) error {
	return c.verify(SignContent(params, "sign_type"), params["sign"])
}

// BuildRequest 组装 method 的完整签名参数(含 sign)。bizContent 为 nil 时不带 biz_content
// (system.oauth.token 的业务参数在公共层,而不是 biz_content 里)。
func (c *Client) BuildRequest(method string, bizContent any, extra map[string]string) (map[string]string, error) {
	p := map[string]string{
		"app_id": c.AppID, "method": method, "format": "JSON", "charset": "utf-8",
		"sign_type": "RSA2", "timestamp": time.Now().In(cst).Format("2006-01-02 15:04:05"), "version": "1.0",
	}
	for k, v := range extra {
		if v != "" {
			p[k] = v
		}
	}
	if bizContent != nil {
		b, err := json.Marshal(bizContent)
		if err != nil {
			return nil, err
		}
		p["biz_content"] = string(b)
	}
	sign, err := c.Sign(p)
	if err != nil {
		return nil, err
	}
	p["sign"] = sign
	return p, nil
}

// Execute 调网关,返回 <method>_response 节点(已验签、已判业务码)。
func (c *Client) Execute(method string, bizContent any, extra map[string]string) (json.RawMessage, error) {
	params, err := c.BuildRequest(method, bizContent, extra)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Post(c.Gateway, "application/x-www-form-urlencoded;charset=utf-8", strings.NewReader(Encode(params)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return c.ParseResponse(method, body)
}

// ParseResponse 解析网关 JSON:验签对象是响应节点的**原始子串**(json.RawMessage 原样保留)。
//
// 成功响应不一定有 code(system.oauth.token 就没有),所以「有 code 且不是 10000」才算失败。
func (c *Client) ParseResponse(method string, body []byte) (json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("支付宝响应不是 JSON: %w", err)
	}
	key := strings.ReplaceAll(method, ".", "_") + "_response"
	node, ok := top[key]
	if !ok {
		node, ok = top["error_response"]
		if !ok {
			return nil, fmt.Errorf("支付宝响应缺少 %s: %s", key, string(body))
		}
	}
	var sign string
	_ = json.Unmarshal(top["sign"], &sign)
	if c.pub != nil && sign != "" {
		if err := c.verify(string(node), sign); err != nil {
			return nil, fmt.Errorf("支付宝响应验签失败: %w", err)
		}
	} else if c.pub != nil {
		return nil, errors.New("支付宝响应缺少 sign")
	}
	var st struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	_ = json.Unmarshal(node, &st)
	if st.Code != "" && st.Code != "10000" {
		return nil, fmt.Errorf("支付宝 %s 失败: %s %s %s %s", method, st.Code, st.Msg, st.SubCode, st.SubMsg)
	}
	return node, nil
}

// SignRaw / VerifyRaw 对任意字符串签名 / 验签(网关响应节点、其它包的测试)。
func (c *Client) SignRaw(content string) (string, error) { return c.signContent(content) }
func (c *Client) VerifyRaw(content, signB64 string) error  { return c.verify(content, signB64) }

// FenToYuan 分 → "12.00"。
func FenToYuan(fen int64) string { return fmt.Sprintf("%d.%02d", fen/100, fen%100) }

var yuanRe = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// YuanToFen "12.00" / "12" / "0.1" → 分。格式不对一律报错——支付宝通知里的金额要拿去和订单比对,
// 解析失败当 0 会让金额校验形同虚设。
func YuanToFen(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !yuanRe.MatchString(s) {
		return 0, fmt.Errorf("金额格式不正确: %q", s)
	}
	parts := strings.SplitN(s, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	var frac int64
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		frac, _ = strconv.ParseInt(f, 10, 64)
	}
	return whole*100 + frac, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go test ./internal/common/alipay/ -v`
Expected: PASS 全部

---

### Task 3: 支付宝驱动真正实现（APP 支付 / 小程序 create / 通知验签 / 查单）

**Files:**
- Modify: `server/internal/pay/driver.go`（加 `Querier`）
- Rewrite: `server/internal/pay/driver_alipay.go`
- Create: `server/internal/pay/driver_alipay_test.go`

**Interfaces:**
- Consumes: Task 2 的 `alipay.Client`。
- Produces: `type Querier interface{ Query(order *model.PayOrder) (*CallbackResult, error) }`；`AliCreds{AppID, PrivateKeyPEM, PublicKey, NotifyURL, PID, TradeType string}`（`TradeType` 取 `"APP"` / `""`=小程序）；`AlipayDriver` 实现 `Driver` + `Querier`；`Name()` 在 APP 模式返回 `alipay_app`。Prepay(APP) 返回 `{"order_str": string}`；Prepay(小程序) 返回 `{"tradeNO": string}`。

- [ ] **Step 1: 写失败测试**

```go
// server/internal/pay/driver_alipay_test.go
package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/model"
)

func aliTestCreds(t *testing.T) (AliCreds, *alipay.Client) {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}))
	// 测试里「支付宝公钥」就用同一对的公钥:我们既当商户签,又当支付宝验。
	c := AliCreds{AppID: "2021000000000001", PrivateKeyPEM: privPEM, PublicKey: pubPEM, NotifyURL: "https://x/api/pay/callback/alipay_app", PID: "2088000000000001", TradeType: "APP"}
	cli, err := alipay.New(c.AppID, privPEM, pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	return c, cli
}

func TestAlipayAppPrepayBuildsSignedOrderStr(t *testing.T) {
	c, cli := aliTestCreds(t)
	d := NewAlipayDriver(c)
	if d.Name() != "alipay_app" {
		t.Fatalf("Name = %s", d.Name())
	}
	params, err := d.Prepay(&model.PayOrder{OrderNo: "N1", PriceMinor: 1200}, "")
	if err != nil {
		t.Fatal(err)
	}
	orderStr, _ := params["order_str"].(string)
	if orderStr == "" {
		t.Fatalf("缺少 order_str: %+v", params)
	}
	vals, err := url.ParseQuery(orderStr)
	if err != nil {
		t.Fatal(err)
	}
	flat := map[string]string{}
	for k := range vals {
		flat[k] = vals.Get(k)
	}
	if flat["method"] != "alipay.trade.app.pay" || flat["notify_url"] != c.NotifyURL || flat["app_id"] != c.AppID {
		t.Fatalf("公共参数不对: %v", flat)
	}
	biz := flat["biz_content"]
	for _, want := range []string{`"out_trade_no":"N1"`, `"total_amount":"12.00"`, `"product_code":"QUICK_MSECURITY_PAY"`, `"timeout_express":"30m"`} {
		if !strings.Contains(biz, want) {
			t.Errorf("biz_content 缺 %s: %s", want, biz)
		}
	}
	// 还原后的参数要能用支付宝公钥(这里是同一对)验过请求签名
	if err := cli.Verify(map[string]string{}); err == nil {
		t.Fatal("空参数不该验过")
	}
	content := alipay.SignContent(flat)
	if err := verifyWith(cli, content, flat["sign"]); err != nil {
		t.Fatalf("orderStr 签名应可验: %v", err)
	}
}

func TestAlipayVerifyCallback(t *testing.T) {
	c, cli := aliTestCreds(t)
	d := NewAlipayDriver(c)
	form := map[string]string{
		"app_id": c.AppID, "out_trade_no": "N1", "trade_no": "T1", "trade_status": "TRADE_SUCCESS",
		"total_amount": "12.00", "sign_type": "RSA2", "charset": "utf-8",
	}
	sign, _ := cli.Sign(stripSignType(form))
	form["sign"] = sign
	req := func(f map[string]string) *http.Request {
		v := url.Values{}
		for k, val := range f {
			v.Set(k, val)
		}
		r := httptest.NewRequest(http.MethodPost, "/cb", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return r
	}
	res, err := d.VerifyCallback(req(form))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Paid || res.OrderNo != "N1" || res.TxnID != "T1" || res.AmountFen != 1200 {
		t.Fatalf("结果不对: %+v", res)
	}
	// 篡改金额
	bad := map[string]string{}
	for k, v := range form {
		bad[k] = v
	}
	bad["total_amount"] = "0.01"
	if _, err := d.VerifyCallback(req(bad)); err == nil {
		t.Fatal("篡改后应失败")
	}
	// 别人的 app_id(签名对但不是我的应用)
	other := map[string]string{}
	for k, v := range form {
		other[k] = v
	}
	other["app_id"] = "2021999999999999"
	delete(other, "sign")
	other["sign"], _ = cli.Sign(stripSignType(other))
	if _, err := d.VerifyCallback(req(other)); err == nil {
		t.Fatal("app_id 不匹配应失败")
	}
	// 关单通知:验签通过但 Paid=false
	closed := map[string]string{}
	for k, v := range form {
		closed[k] = v
	}
	closed["trade_status"] = "TRADE_CLOSED"
	delete(closed, "sign")
	closed["sign"], _ = cli.Sign(stripSignType(closed))
	res, err = d.VerifyCallback(req(closed))
	if err != nil || res.Paid {
		t.Fatalf("关单应 Paid=false: %+v %v", res, err)
	}
}

func TestAlipayQuery(t *testing.T) {
	c, cli := aliTestCreds(t)
	node := `{"code":"10000","trade_no":"T9","trade_status":"TRADE_SUCCESS","total_amount":"12.00","out_trade_no":"N1"}`
	sig, _ := signNode(cli, node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	d := NewAlipayDriver(c)
	d.gateway = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1", PriceMinor: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Paid || res.TxnID != "T9" || res.AmountFen != 1200 {
		t.Fatalf("got %+v", res)
	}
}

// 查不到订单(ACQ.TRADE_NOT_EXIST)不是错误:用户根本没付,返回未支付。
func TestAlipayQueryNotExistIsUnpaid(t *testing.T) {
	c, cli := aliTestCreds(t)
	node := `{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"交易不存在"}`
	sig, _ := signNode(cli, node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	d := NewAlipayDriver(c)
	d.gateway = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1"})
	if err != nil || res.Paid {
		t.Fatalf("got %+v %v", res, err)
	}
}

// --- helpers ---
func stripSignType(m map[string]string) map[string]string {
	// 通知的 sign 是在剔除 sign_type 后算的;Sign() 不剔,所以这里手动去掉再签。
	out := map[string]string{}
	for k, v := range m {
		if k != "sign_type" && k != "sign" {
			out[k] = v
		}
	}
	return out
}
func signNode(cli *alipay.Client, node string) (string, error) {
	// 节点签名 = 对原始 JSON 子串签名;借 Sign 的内部实现:把子串当唯一参数值会多出 "k=",所以走 BuildRequest 不行。
	// 用导出的 SignRaw。
	return cli.SignRaw(node)
}
func verifyWith(cli *alipay.Client, content, sign string) error { return cli.VerifyRaw(content, sign) }
```

（`SignRaw` / `VerifyRaw` 已在 Task 2 的实现里。）

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/pay/ -run Alipay -v`
Expected: FAIL（`d.gateway` / `Query` / `PID` / `TradeType` 不存在）

- [ ] **Step 3: 实现**

`server/internal/pay/driver.go` 末尾追加：

```go
// Querier 可选能力:主动向渠道查单。回调丢了时由查单接口补偿入账(order_query.go SyncIfStale)。
// mock 渠道不实现——它没有「渠道那边」可问。
type Querier interface {
	// Query 订单不存在 / 未支付返回 Paid=false 且 err=nil;网络或验签错误才返回 err。
	Query(order *model.PayOrder) (*CallbackResult, error)
}
```

`server/internal/pay/driver_alipay.go` 整体替换：

```go
package pay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/model"
	"driftbottle/pkg/apilog"
)

// AliCreds 支付宝凭证材料。
type AliCreds struct {
	AppID         string
	PrivateKeyPEM string // 应用私钥(PKCS8/PKCS1,可裸 base64)
	PublicKey     string // 支付宝公钥(不是应用公钥)
	NotifyURL     string
	PID           string // 商户 PID(2088 开头),授权登录要;支付不用
	TradeType     string // "APP" = App 支付(alipay.trade.app.pay);"" = 小程序(alipay.trade.create)
	TenantID      int64  // 仅用于接口日志归属
}

// AlipayDriver 支付宝:App 支付 / 小程序支付两种交易类型共用一份。
type AlipayDriver struct {
	c       AliCreds
	gateway string // 测试可覆盖;空 = 按 appid 选正式/沙箱

	once    sync.Once
	cli     *alipay.Client
	loadErr error
}

func NewAlipayDriver(c AliCreds) *AlipayDriver { return &AlipayDriver{c: c} }

func (d *AlipayDriver) Name() string {
	if d.c.TradeType == "APP" {
		return "alipay_app"
	}
	return "alipay"
}

func (d *AlipayDriver) hasCreds() bool {
	return d.c.AppID != "" && d.c.PrivateKeyPEM != "" && d.c.PublicKey != ""
}

func (d *AlipayDriver) client() (*alipay.Client, error) {
	d.once.Do(func() {
		cli, err := alipay.New(d.c.AppID, d.c.PrivateKeyPEM, d.c.PublicKey)
		if err != nil {
			d.loadErr = err
			return
		}
		if d.gateway != "" {
			cli.Gateway = d.gateway
		}
		d.cli = cli
	})
	return d.cli, d.loadErr
}

// Prepay App 支付:不调网关,服务端签好 orderStr 交给客户端 SDK;小程序:alipay.trade.create 取 trade_no。
func (d *AlipayDriver) Prepay(order *model.PayOrder, payerID string) (map[string]interface{}, error) {
	if !d.hasCreds() {
		return map[string]interface{}{"mock": true, "orderNo": order.OrderNo, "tradeNO": "mock-" + order.OrderNo}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	if d.c.TradeType == "APP" {
		biz := map[string]string{
			"out_trade_no":    order.OrderNo,
			"total_amount":    alipay.FenToYuan(order.PriceMinor),
			"subject":         "金币充值",
			"product_code":    "QUICK_MSECURITY_PAY",
			"timeout_express": "30m",
		}
		params, err := cli.BuildRequest("alipay.trade.app.pay", biz, map[string]string{"notify_url": d.c.NotifyURL})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"order_str": alipay.Encode(params)}, nil
	}
	if payerID == "" {
		return nil, errors.New("支付宝小程序下单缺少 buyer_id")
	}
	biz := map[string]string{
		"out_trade_no": order.OrderNo,
		"total_amount": alipay.FenToYuan(order.PriceMinor),
		"subject":      "金币充值",
		"buyer_id":     payerID,
	}
	node, err := cli.Execute("alipay.trade.create", biz, map[string]string{"notify_url": d.c.NotifyURL})
	apilog.Record(d.c.TenantID, "alipay_trade_create", "out_trade_no="+order.OrderNo, 0, errString(err, node), err == nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		TradeNo string `json:"trade_no"`
	}
	_ = json.Unmarshal(node, &r)
	if r.TradeNo == "" {
		return nil, errors.New("支付宝下单未返回 trade_no")
	}
	return map[string]interface{}{"tradeNO": r.TradeNo}, nil
}

// VerifyCallback 异步通知:表单 → RSA2 验签 → app_id 核对 → 状态 / 金额。
//
// 读 r.Form 而不是自己 ParseForm 后丢弃:handler 为了按 app_id 反查租户已经 ParseForm 过一次,
// Body 已被消费,再 Parse 一次拿到的是空表单。
func (d *AlipayDriver) VerifyCallback(r *http.Request) (*CallbackResult, error) {
	if len(r.Form) == 0 {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}
	params := make(map[string]string, len(r.Form))
	for k := range r.Form {
		params[k] = r.Form.Get(k)
	}
	outTradeNo := params["out_trade_no"]
	if outTradeNo == "" {
		return nil, errors.New("支付宝回调缺少 out_trade_no")
	}
	status := params["trade_status"]
	// 开发态:无凭证时直接信任明文(本地自测)
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: outTradeNo, TxnID: params["trade_no"],
			Paid: status == "TRADE_SUCCESS" || status == "TRADE_FINISHED" || status == ""}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	if err := cli.Verify(params); err != nil {
		return nil, fmt.Errorf("支付宝回调验签失败: %w", err)
	}
	if params["app_id"] != d.c.AppID {
		return nil, fmt.Errorf("支付宝回调 app_id 不匹配: %s", params["app_id"])
	}
	fen, err := alipay.YuanToFen(params["total_amount"])
	if err != nil {
		return nil, fmt.Errorf("支付宝回调金额非法: %w", err)
	}
	return &CallbackResult{
		OrderNo: outTradeNo, TxnID: params["trade_no"],
		Paid:      status == "TRADE_SUCCESS" || status == "TRADE_FINISHED",
		AmountFen: fen,
	}, nil
}

// Query alipay.trade.query。交易不存在 = 用户没付,不算错。
func (d *AlipayDriver) Query(order *model.PayOrder) (*CallbackResult, error) {
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	node, err := cli.Execute("alipay.trade.query", map[string]string{"out_trade_no": order.OrderNo}, nil)
	apilog.Record(d.c.TenantID, "alipay_trade_query", "out_trade_no="+order.OrderNo, 0, errString(err, node), err == nil)
	if err != nil {
		if strings.Contains(err.Error(), "ACQ.TRADE_NOT_EXIST") {
			return &CallbackResult{OrderNo: order.OrderNo}, nil
		}
		return nil, err
	}
	var r struct {
		TradeNo     string `json:"trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	}
	_ = json.Unmarshal(node, &r)
	fen, _ := alipay.YuanToFen(r.TotalAmount)
	return &CallbackResult{
		OrderNo: order.OrderNo, TxnID: r.TradeNo,
		Paid:      r.TradeStatus == "TRADE_SUCCESS" || r.TradeStatus == "TRADE_FINISHED",
		AmountFen: fen,
	}, nil
}

func (d *AlipayDriver) SuccessResponse() (string, []byte) {
	return "text/plain", []byte("success")
}

func errString(err error, node json.RawMessage) string {
	if err != nil {
		return err.Error()
	}
	return string(node)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go build ./... && go test ./internal/pay/ ./internal/common/alipay/ -v`
Expected: PASS（旧的 mock / wx 测试也仍通过）

---

### Task 4: 微信驱动加 APP 交易类型 + 主动查单

**Files:**
- Modify: `server/internal/pay/driver_wx.go`
- Modify: `server/internal/pay/driver_wx_test.go`（追加用例）

**Interfaces:**
- Produces: `WxCreds.TradeType string`（`"APP"` / `""`=JSAPI）、`WxCreds.TenantID int64`；`WxDriver.apiBase string`（测试覆盖）；`WxDriver` 实现 `Querier`；`Name()` APP 模式返回 `wx_app`；APP 模式 `Prepay` 返回 `{"appid","partnerid","prepayid","package":"Sign=WXPay","noncestr","timestamp","sign"}`。

- [ ] **Step 1: 写失败测试**（追加到 `driver_wx_test.go`，import 补 `"net/http"`、`"driftbottle/internal/model"`）

```go
func TestWxAppPrepayParams(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if r.Header.Get("Authorization") == "" {
			t.Error("缺 Authorization 头")
		}
		_, _ = w.Write([]byte(`{"prepay_id":"wx123"}`))
	}))
	defer srv.Close()

	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP", NotifyURL: "https://x/cb"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	if d.Name() != "wx_app" {
		t.Fatalf("Name = %s", d.Name())
	}

	p, err := d.Prepay(&model.PayOrder{OrderNo: "N1", PriceMinor: 100}, "")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v3/pay/transactions/app" {
		t.Fatalf("path = %s", gotPath)
	}
	if _, has := gotBody["payer"]; has {
		t.Fatal("APP 下单不该带 payer")
	}
	if p["appid"] != "wxopen" || p["partnerid"] != "1900" || p["prepayid"] != "wx123" || p["package"] != "Sign=WXPay" {
		t.Fatalf("参数不对: %+v", p)
	}
	// APP 的签名串第四行是裸 prepay_id
	msg := "wxopen\n" + p["timestamp"].(string) + "\n" + p["noncestr"].(string) + "\nwx123\n"
	if err := verifySHA256(&priv.PublicKey, msg, p["sign"].(string)); err != nil {
		t.Fatalf("APP 拉起签名应可验: %v", err)
	}
}

func TestWxQueryByOutTradeNo(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var gotURL, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		gotMethod = r.Method
		_, _ = w.Write([]byte(`{"out_trade_no":"N1","transaction_id":"TX1","trade_state":"SUCCESS","amount":{"total":100,"payer_total":100}}`))
	}))
	defer srv.Close()
	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1", PriceMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotURL != "/v3/pay/transactions/out-trade-no/N1?mchid=1900" {
		t.Fatalf("%s %s", gotMethod, gotURL)
	}
	if !res.Paid || res.TxnID != "TX1" || res.AmountFen != 100 {
		t.Fatalf("got %+v", res)
	}
}

// 微信对不存在的单返回 404 ORDERNOTEXIST:那是「没付」,不是错误。
func TestWxQueryNotExistIsUnpaid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"code":"ORDERNOTEXIST","message":"订单不存在"}`))
	}))
	defer srv.Close()
	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1"})
	if err != nil || res.Paid {
		t.Fatalf("got %+v %v", res, err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/pay/ -run 'TestWxApp|TestWxQuery' -v`
Expected: FAIL（`TradeType` / `apiBase` / `Query` 不存在）

- [ ] **Step 3: 实现**

`driver_wx.go` 结构体与命名：

```go
type WxCreds struct {
	// ...原字段不动...
	TradeType string // "APP" = App 支付(/v3/pay/transactions/app);"" = 小程序 JSAPI
	TenantID  int64  // 接口日志归属
}

type WxDriver struct {
	c       WxCreds
	apiBase string // 测试覆盖;空 = wxAPIBase

	once        sync.Once
	loadErr     error
	privKey     *rsa.PrivateKey
	platformPub *rsa.PublicKey
}

func (d *WxDriver) Name() string {
	if d.c.TradeType == "APP" {
		return "wx_app"
	}
	return "wx"
}

func (d *WxDriver) base() string {
	if d.apiBase != "" {
		return d.apiBase
	}
	return wxAPIBase
}
```

`Prepay` 按交易类型分叉（body 用 map，省得两套 struct；删掉旧 `jsapiReq` 与 `doSignedPost`）：

```go
func (d *WxDriver) Prepay(order *model.PayOrder, payerID string) (map[string]interface{}, error) {
	if !d.hasCreds() {
		return map[string]interface{}{
			"mock": true, "orderNo": order.OrderNo, "timeStamp": "0",
			"nonceStr": order.OrderNo, "package": "prepay_id=mock", "signType": "RSA", "paySign": "mock",
		}, nil
	}
	if err := d.loadKeys(); err != nil {
		return nil, err
	}
	isApp := d.c.TradeType == "APP"
	body := map[string]interface{}{
		"appid": d.c.AppID, "mchid": d.c.MchID, "description": "金币充值",
		"out_trade_no": order.OrderNo, "notify_url": d.c.NotifyURL,
		"amount": map[string]interface{}{"total": order.PriceMinor, "currency": "CNY"},
	}
	path := "/v3/pay/transactions/jsapi"
	if isApp {
		path = "/v3/pay/transactions/app"
	} else {
		if payerID == "" {
			return nil, errors.New("微信下单缺少 payer openid")
		}
		body["payer"] = map[string]string{"openid": payerID}
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	respBody, status, err := d.doSigned(http.MethodPost, path, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, wxAPIError("微信下单失败", status, respBody)
	}
	var r struct {
		PrepayID string `json:"prepay_id"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	if r.PrepayID == "" {
		return nil, errors.New("微信下单未返回 prepay_id")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randNonce()
	if isApp {
		// APP 调起签名串第四行是**裸 prepay_id**(与 JSAPI 的 "prepay_id=xxx" 不同,官方文档如此)。
		sign, err := signSHA256(d.privKey, d.c.AppID+"\n"+ts+"\n"+nonce+"\n"+r.PrepayID+"\n")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"appid": d.c.AppID, "partnerid": d.c.MchID, "prepayid": r.PrepayID,
			"package": "Sign=WXPay", "noncestr": nonce, "timestamp": ts, "sign": sign,
		}, nil
	}
	pkg := "prepay_id=" + r.PrepayID
	paySign, err := signSHA256(d.privKey, d.c.AppID+"\n"+ts+"\n"+nonce+"\n"+pkg+"\n")
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"timeStamp": ts, "nonceStr": nonce, "package": pkg, "signType": "RSA", "paySign": paySign,
	}, nil
}

// doSigned 带 APIv3 签名的请求。path 含 query(查单用);GET 时 body 为空但签名串仍要留空行。
func (d *WxDriver) doSigned(method, path string, bodyBytes []byte) ([]byte, int, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randNonce()
	signStr := method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + string(bodyBytes) + "\n"
	signature, err := signSHA256(d.privKey, signStr)
	if err != nil {
		return nil, 0, err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		d.c.MchID, nonce, signature, timestamp, d.c.SerialNo)
	req, err := http.NewRequest(method, d.base()+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "driftbottle-wxpay/1.0")
	resp, err := wxHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	apilog.Record(d.c.TenantID, "wxpay_"+strings.ToLower(method), path, resp.StatusCode, string(respBody), resp.StatusCode < 300)
	return respBody, resp.StatusCode, nil
}

func wxAPIError(prefix string, status int, body []byte) error {
	var e struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &e)
	return fmt.Errorf("%s(%d): %s %s", prefix, status, e.Code, e.Message)
}

// Query GET /v3/pay/transactions/out-trade-no/{no}?mchid=。404 ORDERNOTEXIST = 未支付。
func (d *WxDriver) Query(order *model.PayOrder) (*CallbackResult, error) {
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	if err := d.loadKeys(); err != nil {
		return nil, err
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(order.OrderNo) + "?mchid=" + url.QueryEscape(d.c.MchID)
	respBody, status, err := d.doSigned(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	if status != http.StatusOK {
		return nil, wxAPIError("微信查单失败", status, respBody)
	}
	var res wxDecryptedResource
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, err
	}
	return &CallbackResult{
		OrderNo: order.OrderNo, TxnID: res.TransactionID,
		Paid: res.TradeState == "SUCCESS", AmountFen: res.Amount.Total,
	}, nil
}
```

import 补 `"net/url"`、`"strings"`、`"driftbottle/pkg/apilog"`。

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go test ./internal/pay/ -v`
Expected: PASS（含原有 `TestWxVerifyCallback`）

---

### Task 5: `pay.Service` 渠道选择 / 新平台驱动 / 主动查单补偿 / 回调按平台反查租户

**Files:**
- Modify: `server/internal/pay/service.go`
- Modify: `server/internal/pay/order_query.go`
- Modify: `server/internal/pay/handler.go`
- Create: `server/internal/pay/channel_test.go`
- Modify: `server/cmd/api/main.go:96`（`pay.New` 多一个参数）

**Interfaces:**
- Consumes: Task 1 `creds.Version()` / `ByPlatformSerial(platform, serial)`；Task 3/4 驱动；Task 6 的 `userSvc.GetAlipayUID`（本任务先在 main.go 传 `nil` 占位，Task 6 再换）。
- Produces:
  - `func resolveChannel(platform, channel string, mockOn bool) (string, error)`（纯函数）
  - `func New(db, w, getOpenID, getAlipayUID OpenIDFunc, cfg, creds) *Service`（**签名变更**）
  - `func (s *Service) CreateOrder(tenantID, userID, packageID int64, platform, channel string) (string, map[string]interface{}, error)`（**签名变更**）
  - `func (s *Service) SyncIfStale(order *model.PayOrder) *model.PayOrder`
  - `func (s *Service) DriverByAppID(platform, appid string) (Driver, bool)`
  - 请求体 `POST /pay/order {package_id, channel?}`；回调路由 `/pay/callback/:platform` 支持 `wx_app` / `alipay_app`。

- [ ] **Step 1: 写失败测试**

```go
// server/internal/pay/channel_test.go
package pay

import (
	"testing"
	"time"

	"driftbottle/internal/model"
)

func TestResolveChannel(t *testing.T) {
	cases := []struct {
		platform, channel string
		mock              bool
		want              string
		wantErr           bool
	}{
		{"app", "wx_app", false, "wx_app", false},
		{"app", "alipay_app", false, "alipay_app", false},
		{"app", "", false, "", true},          // App 必须选渠道
		{"app", "upi", false, "", true},       // 未知渠道
		{"app", "wx_app", true, "app", false}, // mock 开着忽略 channel
		{"app", "", true, "app", false},
		{"wx", "", false, "wx", false},
		{"wx", "wx_app", false, "", true}, // 小程序不许传 channel
		{"alipay", "", false, "alipay", false},
	}
	for _, c := range cases {
		got, err := resolveChannel(c.platform, c.channel, c.mock)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("resolveChannel(%q,%q,%v) = %q,%v want %q,err=%v", c.platform, c.channel, c.mock, got, err, c.want, c.wantErr)
		}
	}
}

func TestNeedsSync(t *testing.T) {
	now := time.Now()
	fresh := &model.PayOrder{Status: "pending", CreatedAt: now.Add(-2 * time.Second)}
	stale := &model.PayOrder{Status: "pending", CreatedAt: now.Add(-6 * time.Second)}
	paid := &model.PayOrder{Status: "paid", CreatedAt: now.Add(-60 * time.Second)}
	if needsSync(fresh, now) {
		t.Error("5 秒内不查")
	}
	if !needsSync(stale, now) {
		t.Error("超过 5 秒的 pending 该查")
	}
	if needsSync(paid, now) {
		t.Error("终态不查")
	}
}

// 同一订单 10 秒内只向渠道问一次:客户端 1/2/4/8 秒轮询,不能每次都打渠道。
func TestSyncThrottle(t *testing.T) {
	s := &Service{}
	if !s.takeSyncSlot("N1", time.Unix(100, 0)) {
		t.Fatal("第一次应放行")
	}
	if s.takeSyncSlot("N1", time.Unix(105, 0)) {
		t.Fatal("10 秒内应拦下")
	}
	if !s.takeSyncSlot("N1", time.Unix(111, 0)) {
		t.Fatal("10 秒后应放行")
	}
	if !s.takeSyncSlot("N2", time.Unix(105, 0)) {
		t.Fatal("别的订单不受影响")
	}
}

// 凭证版本变了,驱动缓存整体失效。
func TestDriverCacheInvalidatesOnCredVersion(t *testing.T) {
	s := &Service{driverCache: map[string]Driver{"1:wx_app": mockDriver{}}, cacheVer: 1}
	s.ensureCacheVersion(1)
	if len(s.driverCache) != 1 {
		t.Fatal("版本未变不该清")
	}
	s.ensureCacheVersion(2)
	if len(s.driverCache) != 0 || s.cacheVer != 2 {
		t.Fatal("版本变了应清空并记新版本")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/pay/ -run 'TestResolveChannel|TestNeedsSync|TestSyncThrottle|TestDriverCache' -v`
Expected: FAIL（函数不存在）

- [ ] **Step 3: 实现 service.go**

```go
type Service struct {
	db           *gorm.DB
	wallet       *wallet.Service
	getOpenID    OpenIDFunc // 微信小程序 openid(JSAPI 下单要)
	getAlipayUID OpenIDFunc // 支付宝小程序 buyer_id(trade.create 要)
	cfg          *config.Config
	creds        *tenant.Store

	mu          sync.RWMutex
	driverCache map[string]Driver // key "tenantID:platform"
	cacheVer    uint64            // 对应 creds.Version();不一致时整体清缓存

	syncMu   sync.Mutex
	lastSync map[string]time.Time // orderNo → 上次主动查单时间
}

func New(db *gorm.DB, w *wallet.Service, getOpenID, getAlipayUID OpenIDFunc, cfg *config.Config, creds *tenant.Store) *Service {
	return &Service{
		db: db, wallet: w, getOpenID: getOpenID, getAlipayUID: getAlipayUID, cfg: cfg, creds: creds,
		driverCache: map[string]Driver{},
	}
}

// appChannels App 端可选的支付渠道。/app-config 的可用性另按凭证 + 开关算。
var appChannels = map[string]bool{"wx_app": true, "alipay_app": true}

// resolveChannel 把「登录平台 + 客户端选的渠道」折成驱动平台。
//
// App 平台必须选渠道(mock 开着时例外:联调走 mock,渠道参数忽略);
// 小程序平台**不许**传渠道——登录平台就是支付平台,传了说明客户端在乱来。
func resolveChannel(platform, channel string, mockOn bool) (string, error) {
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
	return platform, nil
}

// ensureCacheVersion 凭证重载过就清空驱动缓存(后台改凭证不用重启)。调用方持有 s.mu。
func (s *Service) ensureCacheVersion(v uint64) {
	if s.cacheVer != v {
		s.driverCache = map[string]Driver{}
		s.cacheVer = v
	}
}

func (s *Service) driverFor(tenantID int64, platform string) (Driver, error) {
	// App 平台不走缓存:mock 开关是租户级且可热更(理由见旧注释)。
	if platform == "app" {
		return s.buildDriver(tenantID, platform)
	}
	key := fmt.Sprintf("%d:%s", tenantID, platform)
	var ver uint64
	if s.creds != nil {
		ver = s.creds.Version()
	}
	s.mu.Lock()
	s.ensureCacheVersion(ver)
	if d, ok := s.driverCache[key]; ok {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()

	d, err := s.buildDriver(tenantID, platform)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ensureCacheVersion(ver)
	s.driverCache[key] = d
	s.mu.Unlock()
	return d, nil
}
```

`buildDriver` 多租户分支：

```go
	if s.cfg.MultiTenant {
		r, ok := s.creds.ByTenantPlatform(tenantID, platform)
		if !ok {
			return nil, errs.New(errs.CodeBadRequest, "该租户未配置该平台支付凭证")
		}
		switch platform {
		case "wx", "wx_app":
			tt := ""
			if platform == "wx_app" {
				tt = "APP"
			}
			return NewWxDriver(WxCreds{
				AppID: r.AppID, MchID: r.MchID, APIv3Key: r.PayAPIv3Key, SerialNo: r.PaySerialNo,
				PrivateKeyPEM: r.PayPrivateKeyPEM, PlatformPubPEM: r.PayPlatformKeyPEM,
				PlatformSerial: r.PayPlatformSerial, NotifyURL: r.NotifyURL, TradeType: tt, TenantID: tenantID,
			}), nil
		case "alipay", "alipay_app":
			tt := ""
			if platform == "alipay_app" {
				tt = "APP"
			}
			return NewAlipayDriver(AliCreds{
				AppID: r.AppID, PrivateKeyPEM: r.AlipayPrivateKeyPEM, PublicKey: r.AlipayPublicKey,
				NotifyURL: r.NotifyURL, PID: r.MchID, TradeType: tt, TenantID: tenantID,
			}), nil
		}
		return nil, errs.New(errs.CodeBadRequest, "不支持的支付平台")
	}
```

单租户分支不动（`wx_app` / `alipay_app` 落到「不支持的支付平台」）。

`CreateOrder`：

```go
func (s *Service) CreateOrder(tenantID, userID, packageID int64, platform, channel string) (orderNo string, payParams map[string]interface{}, err error) {
	mockOn := platform == "app" && sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled)
	driverPlatform, err := resolveChannel(platform, channel, mockOn)
	if err != nil {
		return "", nil, err
	}
	d, err := s.driverFor(tenantID, driverPlatform)
	if err != nil {
		return "", nil, err
	}
	// ...档位查询不变...
	order := model.PayOrder{
		// ...其余字段不变...
		Platform: driverPlatform,
	}
	if err := s.db.Create(&order).Error; err != nil {
		return "", nil, err
	}
	var payerID string
	switch {
	case driverPlatform == "wx" && s.getOpenID != nil:
		payerID, _ = s.getOpenID(userID)
	case driverPlatform == "alipay" && s.getAlipayUID != nil:
		payerID, _ = s.getAlipayUID(userID)
	}
	params, err := d.Prepay(&order, payerID)
	if err != nil {
		return "", nil, err
	}
	return order.OrderNo, params, nil
}
```

回调反查：

```go
// DriverBySerial 微信系:按 平台 + 平台公钥 ID 反查租户。
func (s *Service) DriverBySerial(platform, serial string) (Driver, bool) {
	if s.cfg.MultiTenant && serial != "" {
		if r, ok := s.creds.ByPlatformSerial(platform, serial); ok {
			d, err := s.driverFor(r.TenantID, platform)
			return d, err == nil
		}
	}
	d, err := s.driverFor(s.cfg.DefaultTenantID, platform)
	return d, err == nil
}

// DriverByAppID 支付宝系:通知表单里带 app_id,据此反查租户。
func (s *Service) DriverByAppID(platform, appid string) (Driver, bool) {
	if s.cfg.MultiTenant && appid != "" {
		if r, ok := s.creds.ByAppID(platform, appid); ok {
			d, err := s.driverFor(r.TenantID, platform)
			return d, err == nil
		}
	}
	d, err := s.driverFor(s.cfg.DefaultTenantID, platform)
	return d, err == nil
}
```

`cmd/api/main.go:96` 改为 `paySvc := pay.New(db, walletSvc, userSvc.GetWxOpenID, nil, cfg, credStore)`（Task 6 把 `nil` 换成 `userSvc.GetAlipayUID`）。

- [ ] **Step 4: 实现 order_query.go 的主动查单**

```go
import (
	"log"
	"time"
)

// needsSync pending 且下单超过 5 秒才向渠道问:5 秒内用户多半还在收银台里,问了也是「未支付」。
func needsSync(o *model.PayOrder, now time.Time) bool {
	return o.Status == "pending" && now.Sub(o.CreatedAt) > 5*time.Second
}

// takeSyncSlot 同一订单 10 秒内只放行一次。
func (s *Service) takeSyncSlot(orderNo string, now time.Time) bool {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if s.lastSync == nil {
		s.lastSync = map[string]time.Time{}
	}
	if last, ok := s.lastSync[orderNo]; ok && now.Sub(last) < 10*time.Second {
		return false
	}
	s.lastSync[orderNo] = now
	if len(s.lastSync) > 10000 { // 顺手清老条目,别让 map 一直长
		for k, t := range s.lastSync {
			if now.Sub(t) > time.Minute {
				delete(s.lastSync, k)
			}
		}
	}
	return true
}

// SyncIfStale 回调丢了的补偿:向渠道查一次,支付了就走**唯一的入账口** HandleCallback,再把最新订单读回来。
// 任何错误只打日志,不影响查单响应——客户端会继续轮询。
func (s *Service) SyncIfStale(order *model.PayOrder) *model.PayOrder {
	now := time.Now()
	if order == nil || !needsSync(order, now) || !s.takeSyncSlot(order.OrderNo, now) {
		return order
	}
	d, err := s.driverFor(order.TenantID, order.Platform)
	if err != nil {
		return order
	}
	q, ok := d.(Querier)
	if !ok {
		return order
	}
	res, err := q.Query(order)
	if err != nil {
		log.Printf("[pay] 主动查单失败 order=%s: %v", order.OrderNo, err)
		return order
	}
	if res == nil || !res.Paid {
		return order
	}
	if err := s.HandleCallback(order.Platform, res); err != nil {
		log.Printf("[pay] 主动查单入账失败 order=%s: %v", order.OrderNo, err)
		return order
	}
	fresh, err := s.Order(order.UserID, order.OrderNo)
	if err != nil {
		return order
	}
	return fresh
}
```

- [ ] **Step 5: 实现 handler.go**

```go
type createOrderReq struct {
	PackageID int64  `json:"package_id" binding:"required"`
	Channel   string `json:"channel"` // App 端:wx_app / alipay_app;小程序留空
}
// createOrder:
orderNo, params, err := h.svc.CreateOrder(tid, uid, req.PackageID, platform, req.Channel)

// order:错误处理不变,最后一行改为
response.OK(c, h.svc.SyncIfStale(o))

// callbackAuto 通用回调:微信系按 Wechatpay-Serial 头、支付宝系按表单 app_id 反查租户。
func (h *Handler) callbackAuto(c *gin.Context) {
	platform := c.Param("platform")
	var d Driver
	var ok bool
	switch platform {
	case "wx", "wx_app":
		d, ok = h.svc.DriverBySerial(platform, c.Request.Header.Get("Wechatpay-Serial"))
	case "alipay", "alipay_app":
		_ = c.Request.ParseForm()
		d, ok = h.svc.DriverByAppID(platform, c.Request.Form.Get("app_id"))
	}
	if !ok {
		c.String(400, "unknown platform/tenant")
		return
	}
	// ...验签 + HandleCallback + SuccessResponse 与原来相同...
}
```

- [ ] **Step 6: 全量编译与测试**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/pay/ ./cmd/... -v`
Expected: PASS

---

### Task 6: 用户身份扩到微信 / 支付宝（appIdentity、绑定 / 解绑、DTO、换 token 调用）

**Files:**
- Modify: `server/internal/user/otp.go:403-470`（`appIdentity`、`loginOrCreateApp`）
- Modify: `server/internal/user/oauthlink.go`
- Modify: `server/internal/user/oauth.go`（加 `wxAppCode2Token`）
- Create: `server/internal/user/alipay_auth.go`
- Modify: `server/internal/user/service.go`（加 `GetAlipayUID`）
- Modify: `server/internal/common/appdto/appdto.go`
- Modify: `server/internal/user/oauthlink_test.go`（追加）
- Create: `server/internal/user/alipay_auth_test.go`
- Modify: `server/cmd/api/main.go:96`（`nil` → `userSvc.GetAlipayUID`）

**Interfaces:**
- Produces:
  - `appIdentity{Phone, Email, GoogleSub, AppleSub, WxOpenID, AlipayUID, UnionID, Nickname, Avatar string}`
  - `loginMethods{Phone, Password, Google, Apple, Wechat, Alipay bool}`
  - `func subColumn(provider string) string`：`wechat→wx_openid`、`alipay→alipay_uid`
  - `func isOAuthProvider(p string) bool`（google/apple/wechat/alipay）
  - `func wxAppCode2Token(tenantID int64, appid, secret, code string) (*oauthResult, error)`，`oauthResult` 加 `Nickname`、`Avatar`
  - `func alipayAuthInfo(cli *alipay.Client, pid string) (string, error)`、`func alipayCode2UID(tenantID int64, cli *alipay.Client, code string) (*oauthResult, error)`、`func parseAuthInfoParams(authInfo string) map[string]string`（测试用）
  - `func (s *Service) GetAlipayUID(userID int64) (string, error)`
  - `appdto.User` 加 `WechatBound bool json:"wechat_bound"`、`AlipayBound bool json:"alipay_bound"`

- [ ] **Step 1: 写失败测试**

追加到 `oauthlink_test.go`：

```go
func TestRemainingAfterUnbindCountsWechatAlipay(t *testing.T) {
	m := loginMethods{Wechat: true, Alipay: true}
	if got := m.remainingAfterUnbind("wechat"); got != 1 {
		t.Fatalf("解绑微信后应剩 1(支付宝), got %d", got)
	}
	m = loginMethods{Wechat: true}
	if got := m.remainingAfterUnbind("wechat"); got != 0 {
		t.Fatalf("唯一方式不可解绑, got %d", got)
	}
}

func TestSubColumnAndProviderSet(t *testing.T) {
	for p, col := range map[string]string{"google": "google_sub", "apple": "apple_sub", "wechat": "wx_openid", "alipay": "alipay_uid"} {
		if subColumn(p) != col || !isOAuthProvider(p) {
			t.Errorf("%s → %s", p, subColumn(p))
		}
	}
	if isOAuthProvider("facebook") {
		t.Fatal("未知 provider 不该放行")
	}
}

// 第三方标识优先级:任何 sub 都排在 phone / email 之前。
func TestIdentityWherePrefersThirdPartySub(t *testing.T) {
	cond, val := appIdentity{WxOpenID: "o1", Email: "a@b.c"}.where()
	if cond != "wx_openid = ?" || val != "o1" {
		t.Fatalf("%s %s", cond, val)
	}
	cond, val = appIdentity{AlipayUID: "2088", Phone: "+8613800000000"}.where()
	if cond != "alipay_uid = ?" || val != "2088" {
		t.Fatalf("%s %s", cond, val)
	}
}
```

新建 `alipay_auth_test.go`：

```go
package user

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"driftbottle/internal/common/alipay"
)

func aliClient(t *testing.T) *alipay.Client {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	c, err := alipay.New("2021000000000001",
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// authInfo 的固定字段(支付宝「App 支付宝登录」文档),签名要能用同一把公钥验回来。
func TestAlipayAuthInfoShape(t *testing.T) {
	cli := aliClient(t)
	s, err := alipayAuthInfo(cli, "2088000000000001")
	if err != nil {
		t.Fatal(err)
	}
	p := parseAuthInfoParams(s)
	want := map[string]string{
		"apiname": "com.alipay.account.auth", "app_id": "2021000000000001", "app_name": "mc",
		"auth_type": "AUTHACCOUNT", "biz_type": "openservice", "method": "alipay.open.auth.sdk.code.get",
		"pid": "2088000000000001", "product_id": "APP_FAST_LOGIN", "scope": "kuaijie", "sign_type": "RSA2",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("%s = %q want %q", k, p[k], v)
		}
	}
	if p["target_id"] == "" || p["sign"] == "" {
		t.Fatal("缺 target_id / sign")
	}
	if err := cli.VerifyRaw(alipay.SignContent(p), p["sign"]); err != nil {
		t.Fatalf("authInfo 签名应可验: %v", err)
	}
	if _, err := alipayAuthInfo(cli, ""); err == nil {
		t.Fatal("没配 PID 应报错")
	}
}

func TestAlipayCode2UID(t *testing.T) {
	cli := aliClient(t)
	node := `{"access_token":"t","user_id":"2088111","open_id":"o1"}`
	sig, _ := cli.SignRaw(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("method") != "alipay.system.oauth.token" || r.PostForm.Get("code") != "c1" || r.PostForm.Get("grant_type") != "authorization_code" {
			t.Errorf("表单不对: %v", r.PostForm)
		}
		_, _ = w.Write([]byte(`{"alipay_system_oauth_token_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	cli.Gateway = srv.URL
	r, err := alipayCode2UID(1, cli, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if r.OpenID != "2088111" {
		t.Fatalf("got %+v", r)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/user/ -run 'TestRemainingAfterUnbindCounts|TestSubColumn|TestIdentityWherePrefers|TestAlipay' -v`
Expected: FAIL

- [ ] **Step 3: 实现 otp.go（appIdentity / loginOrCreateApp）**

```go
type appIdentity struct {
	Phone     string
	Email     string
	GoogleSub string
	AppleSub  string
	WxOpenID  string // 微信开放平台 openid(App 登录)
	AlipayUID string // 支付宝 user_id
	UnionID   string // 微信 unionid,建号时顺带存,不参与查找
	// Nickname / Avatar 第三方给的资料,仅建号时用;空则用默认昵称。
	Nickname string
	Avatar   string
}

func (id appIdentity) where() (string, string) {
	switch {
	// ⚠️ 所有第三方 sub 都排在 phone / email 之前(理由见旧注释:不按邮箱自动合并账号)。
	case id.GoogleSub != "":
		return "google_sub = ?", id.GoogleSub
	case id.AppleSub != "":
		return "apple_sub = ?", id.AppleSub
	case id.WxOpenID != "":
		return "wx_openid = ?", id.WxOpenID
	case id.AlipayUID != "":
		return "alipay_uid = ?", id.AlipayUID
	case id.Phone != "":
		return "phone = ?", id.Phone
	default:
		return "email = ?", id.Email
	}
}
```

`loginOrCreateApp` 建号处：

```go
	nickname := fmt.Sprintf("用户%06d", idgen.Next()%1000000)
	if strings.TrimSpace(id.Nickname) != "" {
		nickname = truncateRunes(strings.TrimSpace(id.Nickname), 32)
	}
	u = model.User{
		UserID:         idgen.Next(),
		TenantID:       tenantID,
		Nickname:       nickname,
		Avatar:         id.Avatar,
		AnonymousLevel: 1,
		Status:         "active",
		Phone:          id.Phone,
		Email:          id.Email,
		GoogleSub:      nilIfEmpty(id.GoogleSub),
		AppleSub:       nilIfEmpty(id.AppleSub),
		WxOpenID:       id.WxOpenID,
		AlipayUID:      id.AlipayUID,
		UnionID:        id.UnionID,
		CreatedAt:      time.Now(),
		LastActiveAt:   time.Now(),
		LastLoginAt:    time.Now(),
	}
```

包内加：

```go
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
```

- [ ] **Step 4: 实现 oauthlink.go（loginMethods / subColumn / Bind / Unbind）**

```go
type loginMethods struct {
	Phone    bool
	Password bool
	Google   bool
	Apple    bool
	Wechat   bool
	Alipay   bool
}

func (m loginMethods) remainingAfterUnbind(provider string) int {
	switch provider {
	case "google":
		m.Google = false
	case "apple":
		m.Apple = false
	case "wechat":
		m.Wechat = false
	case "alipay":
		m.Alipay = false
	}
	n := 0
	for _, has := range []bool{m.Phone, m.Password, m.Google, m.Apple, m.Wechat, m.Alipay} {
		if has {
			n++
		}
	}
	return n
}

var oauthColumns = map[string]string{
	"google": "google_sub", "apple": "apple_sub", "wechat": "wx_openid", "alipay": "alipay_uid",
}

func isOAuthProvider(p string) bool { _, ok := oauthColumns[p]; return ok }

// subColumn provider 对应的列名。调用方保证 provider 已校验。
func subColumn(provider string) string { return oauthColumns[provider] }

// nullableSubColumn google_sub / apple_sub 是 *string 带唯一索引(空存 NULL);
// wx_openid / alipay_uid 是历史 string 列(空存 '')。写值与清空要按列类型分叉。
func nullableSubColumn(provider string) bool { return provider == "google" || provider == "apple" }

func bindValue(provider, sub string) interface{} {
	if nullableSubColumn(provider) {
		return nilIfEmpty(sub)
	}
	return sub
}

func unbindValue(provider string) interface{} {
	if nullableSubColumn(provider) {
		return nil
	}
	return ""
}
```

`BindOAuth`：开头校验改为 `if !isOAuthProvider(provider) { return errs.New(errs.CodeBadRequest, "不支持的绑定类型") }`；查重时 string 列要排除空串：`First(&other, col+" = ? AND "+col+" <> ''", sub)`；最后 `Update(col, bindValue(provider, sub))`。

`UnbindOAuth`：校验同上；`loginMethods{..., Wechat: u.WxOpenID != "", Alipay: u.AlipayUID != ""}`；最后 `Update(subColumn(provider), unbindValue(provider))`。

- [ ] **Step 5: 实现 oauth.go 的微信 App 换 token**

```go
type oauthResult struct {
	OpenID   string
	UnionID  string
	Nickname string
	Avatar   string
}

// wxAppCode2Token 微信开放平台(移动应用)登录:code → access_token/openid/unionid,再顺手取昵称头像。
// userinfo 失败不阻断登录——资料只是建号时的初值。
func wxAppCode2Token(tenantID int64, appid, secret, code string) (*oauthResult, error) {
	if appid == "" || secret == "" {
		return &oauthResult{OpenID: "wxappdev_" + code}, nil
	}
	u := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		url.QueryEscape(appid), url.QueryEscape(secret), url.QueryEscape(code))
	var tok struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	body, err := wxGetJSON(u, &tok)
	apilog.Record(tenantID, "wx_oauth2_access_token", "code 换 token", tok.ErrCode, body, err == nil && tok.ErrCode == 0)
	if err != nil {
		return nil, err
	}
	if tok.ErrCode != 0 || tok.OpenID == "" {
		return nil, fmt.Errorf("微信登录失败:%s", tok.ErrMsg)
	}
	res := &oauthResult{OpenID: tok.OpenID, UnionID: tok.UnionID}
	var info struct {
		Nickname   string `json:"nickname"`
		HeadImgURL string `json:"headimgurl"`
		ErrCode    int    `json:"errcode"`
	}
	infoURL := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN",
		url.QueryEscape(tok.AccessToken), url.QueryEscape(tok.OpenID))
	if body, err := wxGetJSON(infoURL, &info); err == nil && info.ErrCode == 0 {
		res.Nickname, res.Avatar = info.Nickname, info.HeadImgURL
		apilog.Record(tenantID, "wx_sns_userinfo", "取昵称头像", 0, body, true)
	}
	return res, nil
}

// wxGetJSON GET 并解析 JSON,返回原文供日志。
func wxGetJSON(u string, out interface{}) (string, error) {
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	// 日志里不能出现 secret / access_token:记响应体即可,URL 不记。
	return string(b), json.Unmarshal(b, out)
}
```

import 补 `"io"`、`"driftbottle/pkg/apilog"`。原 `alipayCode2UID(appid, code)` 开发态函数**删除**（新实现在 `alipay_auth.go`），`Login()` 里小程序支付宝分支改为调用 `s.alipayMiniLogin(tenantID, aliAppID, code)`：

```go
// alipayMiniLogin 小程序支付宝登录:有凭证走网关,无凭证开发态派生。
func (s *Service) alipayMiniLogin(tenantID int64, appid, code string) (*oauthResult, error) {
	cli, err := s.alipayClient(tenantID, "alipay")
	if err != nil || cli == nil {
		return &oauthResult{OpenID: "alidev_" + code}, nil
	}
	return alipayCode2UID(tenantID, cli, code)
}
```

- [ ] **Step 6: 新建 alipay_auth.go**

```go
package user

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/common/errs"
	"driftbottle/pkg/apilog"
)

// alipayClient 按租户 + 平台(alipay / alipay_app)构造支付宝客户端。无凭证返回 (nil, nil) 让调用方走开发态。
func (s *Service) alipayClient(tenantID int64, platform string) (*alipay.Client, error) {
	if s.creds == nil {
		return nil, nil
	}
	r, ok := s.creds.ByTenantPlatform(tenantID, platform)
	if !ok || r.AlipayPrivateKeyPEM == "" {
		return nil, nil
	}
	return alipay.New(r.AppID, r.AlipayPrivateKeyPEM, r.AlipayPublicKey)
}

// alipayAuthInfo 服务端签发「App 支付宝登录」的授权串(alipay.open.auth.sdk.code.get),客户端原样交给 SDK。
// 应用私钥不出服务端,所以必须在这里签。
func alipayAuthInfo(cli *alipay.Client, pid string) (string, error) {
	if pid == "" {
		return "", errs.New(errs.CodeLoginFailed, "未配置支付宝 PID")
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	p := map[string]string{
		"apiname":    "com.alipay.account.auth",
		"app_id":     cli.AppID,
		"app_name":   "mc",
		"auth_type":  "AUTHACCOUNT",
		"biz_type":   "openservice",
		"method":     "alipay.open.auth.sdk.code.get",
		"pid":        pid,
		"product_id": "APP_FAST_LOGIN",
		"scope":      "kuaijie",
		"sign_type":  "RSA2",
		"target_id":  hex.EncodeToString(buf),
	}
	sign, err := cli.Sign(p)
	if err != nil {
		return "", err
	}
	p["sign"] = sign
	return alipay.Encode(p), nil
}

// parseAuthInfoParams 把 authInfo 还原成参数表(测试与排错用)。
func parseAuthInfoParams(authInfo string) map[string]string {
	out := map[string]string{}
	vals, err := url.ParseQuery(authInfo)
	if err != nil {
		return out
	}
	for k := range vals {
		out[k] = vals.Get(k)
	}
	return out
}

// alipayCode2UID auth_code → user_id(alipay.system.oauth.token)。业务参数在公共层,不走 biz_content。
func alipayCode2UID(tenantID int64, cli *alipay.Client, code string) (*oauthResult, error) {
	node, err := cli.Execute("alipay.system.oauth.token", nil, map[string]string{
		"grant_type": "authorization_code", "code": code,
	})
	detail := "换 user_id"
	if err != nil {
		apilog.Record(tenantID, "alipay_oauth_token", detail, 0, err.Error(), false)
		return nil, fmt.Errorf("支付宝登录失败:%w", err)
	}
	var r struct {
		UserID string `json:"user_id"`
		OpenID string `json:"open_id"`
	}
	_ = json.Unmarshal(node, &r)
	apilog.Record(tenantID, "alipay_oauth_token", detail, 0, redactToken(string(node)), true)
	uid := r.UserID
	if uid == "" {
		uid = r.OpenID
	}
	if uid == "" {
		return nil, errors.New("支付宝未返回 user_id")
	}
	return &oauthResult{OpenID: uid}, nil
}

// redactToken 日志里抹掉 access_token / refresh_token。
func redactToken(s string) string {
	for _, k := range []string{"access_token", "refresh_token"} {
		if i := strings.Index(s, `"`+k+`":"`); i >= 0 {
			j := strings.Index(s[i+len(k)+4:], `"`)
			if j > 0 {
				s = s[:i+len(k)+4] + "***" + s[i+len(k)+4+j:]
			}
		}
	}
	return s
}
```

- [ ] **Step 7: service.go / appdto / main.go**

`service.go` 在 `GetWxOpenID` 旁加：

```go
// GetAlipayUID 支付宝小程序 trade.create 的 buyer_id。
func (s *Service) GetAlipayUID(userID int64) (string, error) {
	var u model.User
	if err := s.db.Select("alipay_uid").First(&u, "user_id = ?", userID).Error; err != nil {
		return "", err
	}
	return u.AlipayUID, nil
}
```

`appdto.go`：`User` 结构加

```go
	WechatBound bool `json:"wechat_bound"`
	AlipayBound bool `json:"alipay_bound"`
```

`FromUser` 加 `WechatBound: u.WxOpenID != "", AlipayBound: u.AlipayUID != "",`。

`main.go:96`：`pay.New(db, walletSvc, userSvc.GetWxOpenID, userSvc.GetAlipayUID, cfg, credStore)`。

- [ ] **Step 8: 运行确认通过**

Run: `cd server && go build ./... && go test ./internal/user/ -v`
Expected: PASS

---

### Task 7: 登录 / 绑定接口 `/auth/wechat` `/auth/alipay` `/auth/alipay/auth-info`

**Files:**
- Modify: `server/internal/user/appauth.go`（`verifyOAuthSub` 加分支；新增 `LoginWithWechat` / `LoginWithAlipay` / `AlipayAuthInfo`）
- Modify: `server/internal/user/handler.go`
- Modify: `server/cmd/api/routes_test.go`（若枚举路由）

**Interfaces:**
- Consumes: Task 6 全部。
- Produces:
  - `POST /auth/wechat {appid, code}`、`POST /auth/alipay {appid, auth_code}` → 与 `/auth/google` 同形应答
  - `GET /auth/alipay/auth-info?appid=` → `{auth_info}`
  - `POST /auth/wechat/bind (auth) {appid, code}`、`POST /auth/alipay/bind (auth) {appid, auth_code}`
  - `POST /auth/unbind {provider}` 接受 `wechat` / `alipay`
  - `func (s *Service) verifyOAuthSub(appid, provider, credential string) (sub, email string, emailVerified bool, tenantID int64, err error)`（第三参语义改为「凭据」：google/apple 是 id_token，wechat 是 code，alipay 是 auth_code）

- [ ] **Step 1: 写失败测试**

在 `server/cmd/api/routes_test.go` 查看现有写法；若它有期望路由表，追加：

```go
	{"POST", "/api/auth/wechat"},
	{"POST", "/api/auth/alipay"},
	{"GET", "/api/auth/alipay/auth-info"},
	{"POST", "/api/auth/wechat/bind"},
	{"POST", "/api/auth/alipay/bind"},
```

若没有期望表，新建 `server/internal/user/routes_test.go`：

```go
package user

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOAuthRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	NewHandler(&Service{}).Register(r.Group("/api"), noop)
	want := map[string]bool{
		"POST /api/auth/wechat": false, "POST /api/auth/alipay": false, "GET /api/auth/alipay/auth-info": false,
		"POST /api/auth/wechat/bind": false, "POST /api/auth/alipay/bind": false,
	}
	for _, ri := range r.Routes() {
		if _, ok := want[ri.Method+" "+ri.Path]; ok {
			want[ri.Method+" "+ri.Path] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("路由未注册: %s", k)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/user/ -run TestOAuthRoutes -v`
Expected: FAIL

- [ ] **Step 3: 实现 appauth.go**

```go
// appCreds 取 App 第三方登录凭证(wx_app / alipay_app)。
func (s *Service) appCreds(tenantID int64, platform, human string) (*tenant.Resolved, error) {
	if s.creds != nil {
		if r, ok := s.creds.ByTenantPlatform(tenantID, platform); ok {
			return r, nil
		}
	}
	return nil, errs.New(errs.CodeLoginFailed, "未配置"+human+"登录")
}

// wechatIdentity code → 身份。
func (s *Service) wechatIdentity(tenantID int64, code string) (*oauthResult, error) {
	r, err := s.appCreds(tenantID, "wx_app", "微信")
	if err != nil {
		return nil, err
	}
	res, err := wxAppCode2Token(tenantID, r.AppID, r.Secret, code)
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, err.Error())
	}
	return res, nil
}

// alipayIdentity auth_code → 身份。
func (s *Service) alipayIdentity(tenantID int64, authCode string) (*oauthResult, error) {
	if _, err := s.appCreds(tenantID, "alipay_app", "支付宝"); err != nil {
		return nil, err
	}
	cli, err := s.alipayClient(tenantID, "alipay_app")
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, "支付宝凭证无效:"+err.Error())
	}
	if cli == nil {
		return nil, errs.New(errs.CodeLoginFailed, "未配置支付宝登录")
	}
	res, err := alipayCode2UID(tenantID, cli, authCode)
	if err != nil {
		return nil, errs.New(errs.CodeLoginFailed, err.Error())
	}
	return res, nil
}

// LoginWithWechat 微信开放平台登录 / 建号。
func (s *Service) LoginWithWechat(appid, code string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	res, err := s.wechatIdentity(tenantID, code)
	if err != nil {
		return nil, false, err
	}
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		WxOpenID: res.OpenID, UnionID: res.UnionID, Nickname: res.Nickname, Avatar: res.Avatar,
	}, false)
}

// LoginWithAlipay 支付宝登录 / 建号。
func (s *Service) LoginWithAlipay(appid, authCode string) (*model.User, bool, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return nil, false, err
	}
	res, err := s.alipayIdentity(tenantID, authCode)
	if err != nil {
		return nil, false, err
	}
	return s.loginOrCreateOAuth(tenantID, appIdentity{AlipayUID: res.OpenID}, false)
}

// AlipayAuthInfo 给客户端 SDK 的授权串。
func (s *Service) AlipayAuthInfo(appid string) (string, error) {
	tenantID, err := s.resolveAppTenant(appid)
	if err != nil {
		return "", err
	}
	r, err := s.appCreds(tenantID, "alipay_app", "支付宝")
	if err != nil {
		return "", err
	}
	cli, err := s.alipayClient(tenantID, "alipay_app")
	if err != nil || cli == nil {
		return "", errs.New(errs.CodeLoginFailed, "支付宝凭证无效")
	}
	return alipayAuthInfo(cli, r.MchID) // 支付宝行的 mch_id 列存 PID
}
```

`verifyOAuthSub(appid, provider, credential string)` 的 switch 加：

```go
	case "wechat":
		res, e := s.wechatIdentity(tenantID, credential)
		if e != nil {
			return "", "", false, 0, e
		}
		return res.OpenID, "", false, tenantID, nil
	case "alipay":
		res, e := s.alipayIdentity(tenantID, credential)
		if e != nil {
			return "", "", false, 0, e
		}
		return res.OpenID, "", false, tenantID, nil
```

import 补 `"driftbottle/internal/tenant"`。

- [ ] **Step 4: 实现 handler.go**

```go
type codeLoginReq struct {
	AppID string `json:"appid"`
	Code  string `json:"code" binding:"required"`
}
type alipayLoginReq struct {
	AppID    string `json:"appid"`
	AuthCode string `json:"auth_code" binding:"required"`
}

func (h *Handler) loginWechat(c *gin.Context) {
	var req codeLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithWechat(req.AppID, req.Code)
	h.appLoginResp(c, u, isNew, err)
}

func (h *Handler) loginAlipay(c *gin.Context) {
	var req alipayLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	u, isNew, err := h.svc.LoginWithAlipay(req.AppID, req.AuthCode)
	h.appLoginResp(c, u, isNew, err)
}

func (h *Handler) alipayAuthInfo(c *gin.Context) {
	info, err := h.svc.AlipayAuthInfo(c.Query("appid"))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "获取授权信息失败")
		return
	}
	response.OK(c, gin.H{"auth_info": info})
}

func (h *Handler) bindWechat(c *gin.Context) { h.bindOAuthWith(c, "wechat") }
func (h *Handler) bindAlipay(c *gin.Context) { h.bindOAuthWith(c, "alipay") }

// bindOAuthWith 微信 / 支付宝绑定:请求体字段与登录一致(code / auth_code)。
func (h *Handler) bindOAuthWith(c *gin.Context, provider string) {
	var credential string
	if provider == "wechat" {
		var req codeLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, errs.CodeBadRequest, "参数错误")
			return
		}
		credential = req.Code
	} else {
		var req alipayLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, errs.CodeBadRequest, "参数错误")
			return
		}
		credential = req.AuthCode
	}
	appid := c.Query("appid")
	h.finishBind(c, provider, appid, credential)
}
```

把现有 `bindOAuth` 的「验签 → BindOAuth → 应答」部分抽成 `finishBind(c, provider, appid, credential)`，`bindOAuth(c, provider)`（google/apple）解析 `oauthLoginReq` 后也调它。微信 / 支付宝的 appid 从请求体取：`codeLoginReq.AppID` / `alipayLoginReq.AppID`（上面 `c.Query("appid")` 改为对应 `req.AppID`）。

`Register` 加路由：

```go
	api.POST("/auth/wechat", h.loginWechat)
	api.POST("/auth/alipay", h.loginAlipay)
	api.GET("/auth/alipay/auth-info", h.alipayAuthInfo)
	api.POST("/auth/wechat/bind", auth, h.bindWechat)
	api.POST("/auth/alipay/bind", auth, h.bindAlipay)
```

- [ ] **Step 5: 运行确认通过**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/user/ ./cmd/... -v`
Expected: PASS

---

### Task 8: sysconfig 开关 + `/app-config` 下发渠道可用性

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`（key + defaults）
- Modify: `server/internal/sysconfig/handler.go`
- Modify: `server/internal/sysconfig/app_config_test.go`（追加）
- Modify: `server/internal/admin/meta.go`
- Modify: `server/cmd/api/main.go`（注入凭证查询）

**Interfaces:**
- Produces:
  - 键：`KeyAppLoginWechatEnabled="app_login_wechat_enabled"`、`KeyAppLoginAlipayEnabled="app_login_alipay_enabled"`、`KeyAppWechatUniversalLink="app_wechat_universal_link"`、`KeyAppPayWechatEnabled="app_pay_wechat_enabled"`、`KeyAppPayAlipayEnabled="app_pay_alipay_enabled"`
  - `type CredLookup func(tenantID int64, platform string) (appID string, ok bool)`；`func (h *Handler) WithCredLookup(fn CredLookup) *Handler`
  - `/app-config` 响应：`auth.wechat` `auth.wechat_app_id` `auth.wechat_universal_link` `auth.alipay`；新段 `pay: {wechat, alipay}`

- [ ] **Step 1: 写失败测试**（追加到 `app_config_test.go`）

```go
// 没配凭证时,就算开关默认开,渠道也不可用——按钮不该露出来让人点了报错。
func TestAppConfigChannelsOffWithoutCreds(t *testing.T) {
	setCache(map[int64]map[string]string{})
	d := getAppConfig(t, 100, "")
	auth := d["auth"].(map[string]any)
	pay := d["pay"].(map[string]any)
	if auth["wechat"] != false || auth["alipay"] != false || auth["wechat_app_id"] != "" {
		t.Fatalf("auth = %+v", auth)
	}
	if pay["wechat"] != false || pay["alipay"] != false {
		t.Fatalf("pay = %+v", pay)
	}
}

func getAppConfigWith(t *testing.T, tenant int64, lookup CredLookup) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(tenant, tenant).WithCredLookup(lookup).Register(r.Group("/api"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app-config", nil))
	var body struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Data
}

func TestAppConfigChannelsOnWithCredsAndOffBySwitch(t *testing.T) {
	lookup := func(tid int64, platform string) (string, bool) {
		switch platform {
		case "wx_app":
			return "wxopen123", true
		case "alipay_app":
			return "2021x", true
		}
		return "", false
	}
	setCache(map[int64]map[string]string{100: {KeyAppWechatUniversalLink: "https://ambertu.com/ul/"}})
	d := getAppConfigWith(t, 100, lookup)
	auth := d["auth"].(map[string]any)
	pay := d["pay"].(map[string]any)
	if auth["wechat"] != true || auth["alipay"] != true || auth["wechat_app_id"] != "wxopen123" || auth["wechat_universal_link"] != "https://ambertu.com/ul/" {
		t.Fatalf("auth = %+v", auth)
	}
	if pay["wechat"] != true || pay["alipay"] != true {
		t.Fatalf("pay = %+v", pay)
	}
	// 开关关掉:凭证在也不露出
	setCache(map[int64]map[string]string{100: {KeyAppLoginWechatEnabled: "0", KeyAppPayAlipayEnabled: "0"}})
	d = getAppConfigWith(t, 100, lookup)
	auth = d["auth"].(map[string]any)
	pay = d["pay"].(map[string]any)
	if auth["wechat"] != false || auth["alipay"] != true || pay["wechat"] != true || pay["alipay"] != false {
		t.Fatalf("auth=%+v pay=%+v", auth, pay)
	}
}

func TestChannelSwitchDefaultsOn(t *testing.T) {
	for _, k := range []string{KeyAppLoginWechatEnabled, KeyAppLoginAlipayEnabled, KeyAppPayWechatEnabled, KeyAppPayAlipayEnabled} {
		if defaults[k] != "1" {
			t.Errorf("%s 默认应为 1, got %q", k, defaults[k])
		}
	}
	if defaults[KeyAppWechatUniversalLink] != "" {
		t.Error("universal link 默认应为空")
	}
}
```

（`defaults` 是 `sysconfig.go` 里默认值 map 的变量名；若实际名字不同按实际改。）

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/sysconfig/ -run 'TestAppConfigChannels|TestChannelSwitch' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

`sysconfig.go` 常量块（放在 `KeyAppLoginEmailEnabled` 之后）：

```go
	// App 第三方登录渠道开关(国内版:微信 / 支付宝)。默认开;/app-config 还要求该租户配了对应凭证才下发 true。
	KeyAppLoginWechatEnabled  = "app_login_wechat_enabled"
	KeyAppLoginAlipayEnabled  = "app_login_alipay_enabled"
	KeyAppWechatUniversalLink = "app_wechat_universal_link" // iOS 微信 SDK 必填的 Universal Link;Android 忽略
```

`KeyAppPayMockEnabled` 之后：

```go
	// App 充值渠道开关(Android 国内版)。同样「开关 && 凭证」才下发 true。
	KeyAppPayWechatEnabled = "app_pay_wechat_enabled"
	KeyAppPayAlipayEnabled = "app_pay_alipay_enabled"
```

defaults：

```go
	KeyAppLoginWechatEnabled:  "1",
	KeyAppLoginAlipayEnabled:  "1",
	KeyAppWechatUniversalLink: "",
	KeyAppPayWechatEnabled:    "1",
	KeyAppPayAlipayEnabled:    "1",
```

`handler.go`：

```go
// CredLookup 查某租户某平台的凭证 appid(来自 tenant.Store);nil = 单租户部署,一律视为未配置。
type CredLookup func(tenantID int64, platform string) (appID string, ok bool)

type Handler struct {
	defaultTenant int64
	appTenant     int64
	credLookup    CredLookup
}

func (h *Handler) WithCredLookup(fn CredLookup) *Handler { h.credLookup = fn; return h }

func (h *Handler) credAppID(tid int64, platform string) (string, bool) {
	if h.credLookup == nil {
		return "", false
	}
	return h.credLookup(tid, platform)
}
```

`getAppConfig` 里：

```go
	wxAppID, hasWx := h.credAppID(tid, "wx_app")
	_, hasAli := h.credAppID(tid, "alipay_app")
	// ...
		"auth": gin.H{
			"google_client_id": firstCSV(GetString(tid, KeyAppGoogleClientID)),
			"phone":            show(KeyAppLoginPhoneEnabled),
			"email":            show(KeyAppLoginEmailEnabled),
			// 国内版渠道:开关 && 凭证。没凭证就不露按钮,免得「点了报未配置」。
			"wechat":                show(KeyAppLoginWechatEnabled) && hasWx,
			"wechat_app_id":         wxAppID, // 客户端注册微信 SDK 用;没凭证为空串
			"wechat_universal_link": GetString(tid, KeyAppWechatUniversalLink),
			"alipay":                show(KeyAppLoginAlipayEnabled) && hasAli,
		},
		// 充值渠道(Android)。与 pricing 分开:pricing 是扣费规则,这里是「能用什么付」。
		"pay": gin.H{
			"wechat": show(KeyAppPayWechatEnabled) && hasWx,
			"alipay": show(KeyAppPayAlipayEnabled) && hasAli,
		},
```

（`wxAppID` 在 `!hasWx` 时本就是空串。）

`meta.go`：

```go
	{Key: sysconfig.KeyAppLoginWechatEnabled, LabelZh: "开放微信登录(国内版)", LabelEn: "Allow WeChat sign-in (CN build)", Group: GroupAppAuth, Type: "bool"},
	{Key: sysconfig.KeyAppLoginAlipayEnabled, LabelZh: "开放支付宝登录(国内版)", LabelEn: "Allow Alipay sign-in (CN build)", Group: GroupAppAuth, Type: "bool"},
	{Key: sysconfig.KeyAppWechatUniversalLink, LabelZh: "微信 Universal Link(iOS)", LabelEn: "WeChat universal link (iOS)", Group: GroupAppAuth, Type: "text"},
	{Key: sysconfig.KeyAppPayWechatEnabled, LabelZh: "开放微信支付(Android 国内版)", LabelEn: "Allow WeChat Pay (Android CN build)", Group: GroupAppPay, Type: "bool"},
	{Key: sysconfig.KeyAppPayAlipayEnabled, LabelZh: "开放支付宝支付(Android 国内版)", LabelEn: "Allow Alipay (Android CN build)", Group: GroupAppPay, Type: "bool"},
```

`main.go`：找到 `sysconfig.NewHandler(` 那行（`grep -n "sysconfig.NewHandler" cmd/api/main.go`），改为

```go
	sysconfig.NewHandler(cfg.DefaultTenantID, cfg.AppDefaultTenantID).
		WithCredLookup(func(tid int64, platform string) (string, bool) {
			r, ok := credStore.ByTenantPlatform(tid, platform)
			if !ok {
				return "", false
			}
			return r.AppID, true
		}).Register(api)
```

- [ ] **Step 4: 运行确认通过**

Run: `cd server && go build ./... && go test ./internal/sysconfig/ ./internal/admin/ -v`
Expected: PASS（`meta_test.go` 对新 key 的中英文标签检查通过）

---

### Task 9: 后台「租户凭证」支持新增行与新平台

**Files:**
- Modify: `server/internal/admin/service.go`（`CredRow` 加租户、`CreateCredential`）
- Modify: `server/internal/admin/handler.go`（`POST /credentials`）
- Modify: `server/internal/common/i18n/catalog.go`
- Create: `server/internal/admin/credential_test.go`
- Modify: `admin/src/api.js`
- Modify: `admin/src/views/Credentials.vue`
- Modify: `admin/src/locales/zh-CN.json`、`admin/src/locales/en.json`

**Interfaces:**
- Produces: `POST /admin/api/credentials {tenant_id: "123", platform, appid}`；`CredRow` 加 `TenantID int64 json:"tenant_id,string"`、`TenantName string json:"tenant_name"`；`func validCredPlatform(p string) bool`。

- [ ] **Step 1: 写失败测试**

```go
// server/internal/admin/credential_test.go
package admin

import "testing"

func TestValidCredPlatform(t *testing.T) {
	for _, p := range []string{"wx", "alipay", "app", "wx_app", "alipay_app"} {
		if !validCredPlatform(p) {
			t.Errorf("%s 应合法", p)
		}
	}
	for _, p := range []string{"", "weixin", "ios", "gplay"} {
		if validCredPlatform(p) {
			t.Errorf("%s 不该合法", p)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd server && go test ./internal/admin/ -run TestValidCredPlatform -v`
Expected: FAIL

- [ ] **Step 3: 服务端实现**

`service.go`：

```go
var credPlatforms = map[string]bool{"wx": true, "alipay": true, "app": true, "wx_app": true, "alipay_app": true}

func validCredPlatform(p string) bool { return credPlatforms[p] }

type CredRow struct {
	ID         int64  `json:"id,string"`
	TenantID   int64  `json:"tenant_id,string"`
	TenantName string `json:"tenant_name"`
	// ...原字段不变...
}

func (s *Service) ListCredentials() ([]CredRow, error) {
	var rows []model.AppCredential
	if err := s.db.Order("tenant_id asc, platform asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	var tenants []model.Tenant
	s.db.Select("tenant_id, name").Find(&tenants)
	names := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		names[t.TenantID] = t.Name
	}
	out := make([]CredRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CredRow{
			ID: r.ID, TenantID: r.TenantID, TenantName: names[r.TenantID],
			// ...其余字段同原来...
		})
	}
	return out, nil
}

type CreateCredReq struct {
	TenantID int64  `json:"tenant_id,string" binding:"required"`
	Platform string `json:"platform" binding:"required"`
	AppID    string `json:"appid" binding:"required"`
}

// CreateCredential 给租户新建一行凭证(密钥之后用 UpdateCredential 填)。
func (s *Service) CreateCredential(req CreateCredReq) error {
	if !validCredPlatform(req.Platform) {
		return errors.New("不支持的平台")
	}
	var n int64
	s.db.Model(&model.Tenant{}).Where("tenant_id = ?", req.TenantID).Count(&n)
	if n == 0 {
		return errors.New("租户不存在")
	}
	s.db.Model(&model.AppCredential{}).Where("platform = ? AND app_id = ?", req.Platform, req.AppID).Count(&n)
	if n > 0 {
		return errors.New("该 AppID 已被占用")
	}
	now := time.Now()
	if err := s.db.Create(&model.AppCredential{
		ID: idgen.Next(), TenantID: req.TenantID, Platform: req.Platform, AppID: strings.TrimSpace(req.AppID),
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		return err
	}
	if s.credStore != nil {
		return s.credStore.Reload()
	}
	return nil
}
```

`handler.go`：

```go
	auth.POST("/credentials", h.createCredential)

func (h *Handler) createCredential(c *gin.Context) {
	var req CreateCredReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "参数错误")
		return
	}
	if err := h.svc.CreateCredential(req); err != nil {
		response.FailErr(c, 1002, "创建失败", err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}
```

`catalog.go` `En` 表加：

```go
		// ── 租户凭证 (service.go CreateCredential) ──
		"不支持的平台":      "Unsupported platform",
		"租户不存在":       "Tenant not found",
		"该 AppID 已被占用": "This AppID is already in use",
```

- [ ] **Step 4: 前端实现**

`admin/src/api.js` 在 `updateCredential` 旁加：`createCredential: (data) => req('POST', '/credentials', data),`。租户列表用已有的 `api.listTenants()`（`api.js:96`）。

`Credentials.vue`：

1. 平台标签函数替换模板里 `row.platform === 'wx' ? ... : ...` 三元：
```js
const platformLabels = { wx: 'platformWx', alipay: 'platformAlipay', app: 'platformApp', wx_app: 'platformWxApp', alipay_app: 'platformAlipayApp' }
function platLabel(p) { return t('credentials.' + (platformLabels[p] || 'platformWx')) }
```
模板用 `{{ platLabel(row.platform) }}`，卡片头部加 `<span class="tenant">{{ row.tenant_name || row.tenant_id }}</span>`。
2. 条件显示：把 `row.platform === 'wx'` 改为 `isWx(row.platform)`，`row.platform === 'alipay'` 改为 `isAli(row.platform)`：
```js
const isWx = (p) => p === 'wx' || p === 'wx_app'
const isAli = (p) => p === 'alipay' || p === 'alipay_app'
```
编辑弹窗同理。支付宝行的 `mch_id` 标签：`isAli(editing.platform) ? t('credentials.pid') : t('credentials.mchId')`。
3. 顶部加「新增凭证」按钮与弹窗：
```html
<button class="btn" @click="openCreate()">{{ t('credentials.addBtn') }}</button>
<div v-if="createModal" class="overlay" @click.self="createModal = false">
  <div class="dlg">
    <div class="dlg-hd">{{ t('credentials.addTitle') }}</div>
    <div class="field"><label>{{ t('credentials.tenant') }}</label>
      <select v-model="createForm.tenant_id" class="ipt">
        <option v-for="tn in tenants" :key="tn.tenant_id" :value="tn.tenant_id">{{ tn.name }} ({{ tn.tenant_id }})</option>
      </select></div>
    <div class="field"><label>{{ t('credentials.platform') }}</label>
      <select v-model="createForm.platform" class="ipt">
        <option v-for="p in ['wx','alipay','app','wx_app','alipay_app']" :key="p" :value="p">{{ platLabel(p) }}</option>
      </select></div>
    <div class="field"><label>AppID</label><input v-model="createForm.appid" class="ipt" /></div>
    <div class="dlg-ft">
      <button class="btn ghost" @click="createModal = false">{{ t('common.cancel') }}</button>
      <button class="btn" @click="create">{{ t('common.save') }}</button>
    </div>
    <div v-if="createMsg" class="err">{{ createMsg }}</div>
  </div>
</div>
```
```js
const tenants = ref([])
const createModal = ref(false)
const createForm = ref({ tenant_id: '', platform: 'wx_app', appid: '' })
const createMsg = ref('')
async function openCreate() {
  createMsg.value = ''
  try { tenants.value = await api.listTenants() || [] } catch (e) {}
  createModal.value = true
}
async function create() {
  try {
    await api.createCredential(createForm.value)
    createModal.value = false
    toast.value = t('credentials.created')
    setTimeout(() => { toast.value = '' }, 3000)
    load()
  } catch (e) { createMsg.value = e.message }
}
```
（循环变量叫 `tn` / `p`，别用 `t`——会遮蔽翻译函数。）

locales 两个文件 `credentials` 段各加：

| key | zh-CN | en |
|---|---|---|
| platformApp | App 租户映射 | App tenant mapping |
| platformWxApp | 微信开放平台(App) | WeChat Open Platform (App) |
| platformAlipayApp | 支付宝(App) | Alipay (App) |
| pid | 商户 PID | Partner ID (PID) |
| addBtn | + 新增凭证 | + Add credential |
| addTitle | 新增凭证 | Add credential |
| tenant | 租户 | Tenant |
| platform | 平台 | Platform |
| created | 已创建，请继续填写密钥 | Created. Fill in the secrets next. |

- [ ] **Step 5: 验证**

Run: `cd server && go build ./... && go test ./internal/admin/ -v && cd ../admin && npm run build`
Expected: Go PASS（含 i18n AST 覆盖测试）；admin 构建通过（locale key 一致性前置校验通过）

---

## 国内版 App（`app/bottles_zh`）

> 以下所有路径相对 `app/bottles_zh/`。每个任务结束跑 `flutter analyze`（应 No issues found）。

### Task 10: 工程标识改为国内版

**Files:**
- Modify: `android/app/build.gradle.kts:24,34`
- Move: `android/app/src/main/kotlin/com/ambertu/bottles/MainActivity.kt` → `android/app/src/main/kotlin/com/ambertu/bottles/cn/MainActivity.kt`
- Modify: `android/app/src/main/AndroidManifest.xml`（`android:label`）
- Modify: `ios/Runner.xcodeproj/project.pbxproj`（6 处 `PRODUCT_BUNDLE_IDENTIFIER`）
- Modify: `ios/Runner/Info.plist`（`CFBundleDisplayName`）
- Modify: `lib/core/config/app_config.dart:28-31`
- Modify: `lib/features/auth/auth_controller.dart`（`dialCode` 默认 `'+86'`）

- [ ] **Step 1: 写失败测试**

```dart
// test/app_identity_test.dart
import 'package:bottles/core/config/app_config.dart';
import 'package:bottles/features/auth/auth_controller.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('CN build points at the CN app tenant', () {
    expect(AppConfig.appId, 'drift_app_cn');
  });
  test('CN build defaults the dial code to +86', () {
    expect(const OtpState().dialCode, '+86');
  });
}
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/app_identity_test.dart`
Expected: FAIL（`drift_app_dev` / `+91`）

- [ ] **Step 3: 实现**

- `build.gradle.kts`：`namespace = "com.ambertu.bottles.cn"`、`applicationId = "com.ambertu.bottles.cn"`。
- `git mv`（或手工移动）`MainActivity.kt` 到 `kotlin/com/ambertu/bottles/cn/`，首行改 `package com.ambertu.bottles.cn`。
- `AndroidManifest.xml`：`android:label="漂流瓶"`。
- `project.pbxproj`：`com.ambertu.bottles;` → `com.ambertu.bottles.cn;`（3 处），`com.ambertu.bottles.RunnerTests;` → `com.ambertu.bottles.cn.RunnerTests;`（3 处）。
- `Info.plist`：`CFBundleDisplayName` → `漂流瓶`。
- `app_config.dart`：`defaultValue: 'drift_app_cn'`，注释改为「国内租户的 `platform=app` 行 appid」。
- `auth_controller.dart:53`：`this.dialCode = '+86'`；`lib/core/storage/prefs.dart:13,139` 两处默认 `'+91'` 改 `'+86'`；`login_page.dart:51` 与 `credential_flow_page.dart:42` 的 `_dialCodes` 改为 `['+86', '+852', '+853', '+886', '+1']`。`grep -rn "'+91'" lib test` 应只剩测试里的断言，一并改成 `+86`。

- [ ] **Step 4: 运行确认通过**

Run: `flutter test test/app_identity_test.dart test/widgets/login_channels_test.dart && flutter analyze`
Expected: PASS；No issues found

---

### Task 11: 依赖与远程配置字段

**Files:**
- Modify: `pubspec.yaml`
- Modify: `lib/core/config/remote_config.dart`
- Modify: `test/remote_config_test.dart`（追加）

**Interfaces:**
- Produces: `AppRemoteConfig` 新字段 `wechatLogin`、`alipayLogin`（bool，内置 true）、`wechatAppId`、`wechatUniversalLink`（String，内置 ''）、`payWechat`、`payAlipay`（bool，内置 true）；解析自 `auth.wechat` `auth.alipay` `auth.wechat_app_id` `auth.wechat_universal_link` `pay.wechat` `pay.alipay`；`toJson` 回写同形。

- [ ] **Step 1: 写失败测试**（追加到 `test/remote_config_test.dart`）

```dart
  group('CN channels', () {
    test('builtin: channels on, ids empty', () {
      const b = AppRemoteConfig.builtin();
      expect(b.wechatLogin, isTrue);
      expect(b.alipayLogin, isTrue);
      expect(b.payWechat, isTrue);
      expect(b.payAlipay, isTrue);
      expect(b.wechatAppId, '');
      expect(b.wechatUniversalLink, '');
    });

    test('parses auth/pay sections and round-trips through toJson', () {
      final c = AppRemoteConfig.fromJson({
        'auth': {
          'wechat': false,
          'alipay': true,
          'wechat_app_id': 'wx1',
          'wechat_universal_link': 'https://ambertu.com/ul/',
        },
        'pay': {'wechat': true, 'alipay': false},
      });
      expect(c.wechatLogin, isFalse);
      expect(c.alipayLogin, isTrue);
      expect(c.wechatAppId, 'wx1');
      expect(c.wechatUniversalLink, 'https://ambertu.com/ul/');
      expect(c.payWechat, isTrue);
      expect(c.payAlipay, isFalse);
      final again = AppRemoteConfig.fromJson(c.toJson());
      expect(again.wechatLogin, isFalse);
      expect(again.payAlipay, isFalse);
      expect(again.wechatAppId, 'wx1');
    });

    test('missing sections fall back to builtin', () {
      final c = AppRemoteConfig.fromJson({});
      expect(c.wechatLogin, isTrue);
      expect(c.payAlipay, isTrue);
    });
  });
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/remote_config_test.dart`
Expected: FAIL（字段不存在）

- [ ] **Step 3: 实现**

`pubspec.yaml`：删除 `google_sign_in: ^6.2.2`；在 `sign_in_with_apple` 后加

```yaml
  # 国内版第三方登录 / 支付 SDK。版本以 pub get 解析为准。
  fluwx: ^6.0.4
  tobias: ^5.3.4
```

文件末尾（`flutter:` 段之外、顶层）加 tobias 的 iOS 回跳 scheme：

```yaml
tobias:
  url_scheme: ambertubottlescn
```

然后 `flutter pub get`。

`remote_config.dart`：构造函数 / `builtin` / 字段 / `fromJson` / `toJson` 各加六项：

```dart
  // 构造参数
  required this.wechatLogin,
  required this.alipayLogin,
  required this.wechatAppId,
  required this.wechatUniversalLink,
  required this.payWechat,
  required this.payAlipay,

  // builtin
  wechatLogin = true,
  alipayLogin = true,
  wechatAppId = '',
  wechatUniversalLink = '',
  payWechat = true,
  payAlipay = true,

  /// 国内版第三方登录渠道。服务端下发的是「开关 && 配了凭证」,所以 true 时按钮一定能用。
  /// 内置 true:没拉到配置之前先按都可用画,点下去没 AppID 会得到明确提示而不是静默。
  final bool wechatLogin;
  final bool alipayLogin;

  /// 微信开放平台 AppID(注册 SDK 用)与 iOS Universal Link。空 = 后台没配。
  final String wechatAppId;
  final String wechatUniversalLink;

  /// Android 充值渠道。iOS 走 IAP,不看这两个。
  final bool payWechat;
  final bool payAlipay;

  // fromJson(先取 final pay = _obj(json['pay']);)
  wechatLogin: _bool(auth['wechat']) ?? b.wechatLogin,
  alipayLogin: _bool(auth['alipay']) ?? b.alipayLogin,
  wechatAppId: _str(auth['wechat_app_id']),
  wechatUniversalLink: _str(auth['wechat_universal_link']),
  payWechat: _bool(pay['wechat']) ?? b.payWechat,
  payAlipay: _bool(pay['alipay']) ?? b.payAlipay,

  // toJson 的 'auth' 段追加
  'wechat': wechatLogin,
  'alipay': alipayLogin,
  'wechat_app_id': wechatAppId,
  'wechat_universal_link': wechatUniversalLink,
  // 新段
  'pay': {'wechat': payWechat, 'alipay': payAlipay},
```

- [ ] **Step 4: 运行确认通过**

Run: `flutter test test/remote_config_test.dart && flutter analyze`
Expected: PASS；analyze 会报 `google_sign_in` import 失效（`lib/core/platform/oauth.dart`）——Task 12 处理，此处允许暂红。

---

### Task 12: 第三方登录客户端换成微信 / 支付宝 / Apple

**Files:**
- Rewrite: `lib/core/platform/oauth.dart`
- Create: `lib/core/platform/oauth_results.dart`（纯函数，可测）
- Create: `test/oauth_results_test.dart`
- Modify: `lib/data/repositories.dart:25-85`
- Modify: `lib/data/remote/remote_repositories.dart:173-200`
- Modify: `lib/data/mock/mock_repositories.dart:75-85`
- Modify: `lib/features/auth/auth_controller.dart:330-345`
- Modify: `lib/features/auth/oauth_pending_overlay.dart`、`lib/features/auth/oauth_conflict_page.dart`（provider → 品牌名映射）
- Modify: `test/widgets/account_bind_retry_test.dart`（`_FakeOAuth`）

**Interfaces:**
- Produces:
  - `abstract class OAuthClient { Future<String> wechatCode(); Future<String> alipayAuthCode(String authInfo); Future<String> appleIdToken(); Future<bool> get wechatInstalled; Future<bool> get alipayInstalled; }`
  - `oauth_results.dart`：`String? wechatAuthCode(int errCode, String? code, {required String expectState, required String? gotState})`（返回 null=取消，抛 `OAuthCancelled` / `StateError`）——具体见实现；`String alipayAuthCodeFrom(Map<Object?, Object?> result)`；`Map<String, String> parseAlipayResult(String s)`；`String brandName(String provider)`
  - `AuthRepository.loginWithProvider(String provider, {required String credential})`、`AuthRepository.bindOAuth(String provider, String credential)`、`AuthRepository.alipayAuthInfo()`
  - `remote_repositories.dart` 顶层 `Map<String, Object?> oauthBody(String provider, String credential)`
  - provider 取值 `'wechat'` / `'alipay'` / `'apple'`

- [ ] **Step 1: 写失败测试**

```dart
// test/oauth_results_test.dart
import 'package:bottles/core/platform/oauth.dart';
import 'package:bottles/core/platform/oauth_results.dart';
import 'package:bottles/data/remote/remote_repositories.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('wechat auth result', () {
    test('ok with matching state yields the code', () {
      expect(
        wechatAuthCode(0, 'c1', expectState: 's1', gotState: 's1'),
        'c1',
      );
    });
    test('user cancel (-2) throws OAuthCancelled', () {
      expect(
        () => wechatAuthCode(-2, null, expectState: 's1', gotState: 's1'),
        throwsA(isA<OAuthCancelled>()),
      );
    });
    test('denied (-4) is an error, not a cancel', () {
      expect(
        () => wechatAuthCode(-4, null, expectState: 's1', gotState: 's1'),
        throwsA(isNot(isA<OAuthCancelled>())),
      );
    });
    test('state mismatch is rejected even when errCode is ok', () {
      expect(
        () => wechatAuthCode(0, 'c1', expectState: 's1', gotState: 'evil'),
        throwsA(isA<StateError>()),
      );
    });
  });

  group('alipay auth result', () {
    test('parses auth_code out of the result string', () {
      final m = {
        'resultStatus': '9000',
        'result':
            'success=true&result_code=200&app_id=2021x&auth_code=AC1&scope=kuaijie&user_id=2088',
        'memo': '',
      };
      expect(alipayAuthCodeFrom(m), 'AC1');
    });
    test('6001 is a cancel', () {
      expect(
        () => alipayAuthCodeFrom({'resultStatus': '6001', 'result': ''}),
        throwsA(isA<OAuthCancelled>()),
      );
    });
    test('9000 without auth_code is an error', () {
      expect(
        () => alipayAuthCodeFrom({'resultStatus': '9000', 'result': 'success=false'}),
        throwsA(isNot(isA<OAuthCancelled>())),
      );
    });
  });

  test('oauthBody puts the credential under the key each provider expects', () {
    expect(oauthBody('wechat', 'c')['code'], 'c');
    expect(oauthBody('alipay', 'a')['auth_code'], 'a');
    expect(oauthBody('apple', 't')['id_token'], 't');
    expect(oauthBody('wechat', 'c').containsKey('appid'), isTrue);
  });

  test('brandName', () {
    expect(brandName('wechat'), '微信');
    expect(brandName('alipay'), '支付宝');
    expect(brandName('apple'), 'Apple');
  });
}
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/oauth_results_test.dart`
Expected: FAIL（文件不存在）

- [ ] **Step 3: 实现 oauth_results.dart**

```dart
// lib/core/platform/oauth_results.dart
import 'oauth.dart' show OAuthCancelled;

/// 微信授权回调 → code。
///
/// errCode: 0 成功、-2 用户取消、-4 拒绝授权、其它 SDK 错误。
/// state 必须与本次发起时一致——不校验等于任何人都能把别人的 code 塞进来。
String wechatAuthCode(
  int errCode,
  String? code, {
  required String expectState,
  required String? gotState,
}) {
  if (errCode == -2) throw const OAuthCancelled();
  if (errCode != 0) throw Exception('微信授权失败($errCode)');
  if (gotState != expectState) throw StateError('微信授权 state 不匹配');
  if (code == null || code.isEmpty) throw Exception('微信未返回 code');
  return code;
}

/// 支付宝授权结果 → auth_code。resultStatus 9000 成功、6001 取消。
String alipayAuthCodeFrom(Map<Object?, Object?> result) {
  final status = '${result['resultStatus'] ?? ''}';
  if (status == '6001') throw const OAuthCancelled();
  if (status != '9000') throw Exception('支付宝授权失败($status)');
  final code = parseAlipayResult('${result['result'] ?? ''}')['auth_code'];
  if (code == null || code.isEmpty) throw Exception('支付宝未返回 auth_code');
  return code;
}

/// 支付宝 `result` 字段是 `k=v&k=v`,值**没有** URL 编码,不能用 Uri.splitQueryString。
Map<String, String> parseAlipayResult(String s) {
  final out = <String, String>{};
  for (final part in s.split('&')) {
    final i = part.indexOf('=');
    if (i <= 0) continue;
    out[part.substring(0, i)] = part.substring(i + 1);
  }
  return out;
}

/// 品牌名,A2g 等待层 / A2k 冲突页 / 账号安全页共用。
String brandName(String provider) => switch (provider) {
      'wechat' => '微信',
      'alipay' => '支付宝',
      'apple' => 'Apple',
      _ => provider,
    };
```

- [ ] **Step 4: 重写 oauth.dart**

```dart
// lib/core/platform/oauth.dart
import 'dart:async';
import 'dart:io' show Platform;
import 'dart:math';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:fluwx/fluwx.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';
import 'package:tobias/tobias.dart';

import '../providers.dart';
import 'oauth_results.dart';

/// Apple 登录按钮是否显示(仅 iOS;App Store 4.8:有第三方登录就必须有它)。
bool get showAppleSignIn => !kIsWeb && Platform.isIOS;

class OAuthCancelled implements Exception {
  const OAuthCancelled();
}

/// 把三个第三方 SDK 关在这一个文件里。对外只暴露「给我一个凭据」:
/// 微信给 code、支付宝给 auth_code、Apple 给 identityToken,身份一律由服务端换。
abstract class OAuthClient {
  Future<String> wechatCode();
  Future<String> alipayAuthCode(String authInfo);
  Future<String> appleIdToken();
  Future<bool> get wechatInstalled;
  Future<bool> get alipayInstalled;
}

class RealOAuthClient implements OAuthClient {
  RealOAuthClient({required this.wechatAppId, required this.universalLink});

  /// 微信开放平台 AppID,由 `/api/app-config` 下发;空 = 后台没配。
  final String wechatAppId;
  final String universalLink;

  final _fluwx = Fluwx();
  final _tobias = Tobias();
  bool _registered = false;

  /// 注册幂等;没 AppID 直接报「未配置」,别让用户点一下没反应。
  Future<void> _ensureWechat() async {
    if (wechatAppId.isEmpty) throw Exception('未配置微信登录');
    if (_registered) return;
    await _fluwx.registerApi(
      appId: wechatAppId,
      universalLink: universalLink.isEmpty ? null : universalLink,
    );
    _registered = true;
  }

  @override
  Future<bool> get wechatInstalled async {
    if (wechatAppId.isEmpty) return false;
    await _ensureWechat();
    return _fluwx.isWeChatInstalled;
  }

  @override
  Future<bool> get alipayInstalled => _tobias.isAliPayInstalled;

  @override
  Future<String> wechatCode() async {
    await _ensureWechat();
    final state = 'login_${Random.secure().nextInt(1 << 30)}';
    final done = Completer<String>();
    void onResp(WeChatResponse resp) {
      if (resp is! WeChatAuthResponse || done.isCompleted) return;
      try {
        done.complete(wechatAuthCode(
          resp.errCode ?? -1,
          resp.code,
          expectState: state,
          gotState: resp.state,
        ));
      } catch (e) {
        done.completeError(e);
      }
    }

    _fluwx.addSubscriber(onResp);
    try {
      final sent = await _fluwx.sendWeChatAuth(scope: 'snsapi_userinfo', state: state);
      if (!sent) throw Exception('无法拉起微信');
      // 微信没有「超时」回调:用户切去微信又直接回桌面,这里会永远等。60 秒当取消。
      return await done.future.timeout(
        const Duration(seconds: 60),
        onTimeout: () => throw const OAuthCancelled(),
      );
    } finally {
      _fluwx.removeSubscriber(onResp);
    }
  }

  @override
  Future<String> alipayAuthCode(String authInfo) async {
    final result = await _tobias.auth(authInfo);
    return alipayAuthCodeFrom(result.cast<Object?, Object?>());
  }

  @override
  Future<String> appleIdToken() async {
    try {
      final cred = await SignInWithApple.getAppleIDCredential(
        scopes: [AppleIDAuthorizationScopes.email, AppleIDAuthorizationScopes.fullName],
      );
      final token = cred.identityToken;
      if (token == null || token.isEmpty) throw Exception('Apple 未返回 identityToken');
      return token;
    } on SignInWithAppleAuthorizationException catch (e) {
      if (e.code == AuthorizationErrorCode.canceled) throw const OAuthCancelled();
      rethrow;
    }
  }
}

final oauthClientProvider = Provider<OAuthClient>((ref) {
  final cfg = ref.watch(appConfigProvider);
  return RealOAuthClient(
    wechatAppId: cfg.wechatAppId,
    universalLink: cfg.wechatUniversalLink,
  );
});
```

> ⚠️ `pub get` 后打开 `.pub-cache` 里 fluwx 的 `lib/src/fluwx.dart` 与 `response/wechat_response.dart`，核对：`registerApi` 参数名、`sendWeChatAuth` 签名、`addSubscriber`/`removeSubscriber` 名称、`WeChatAuthResponse.errCode/code/state` 字段、`isWeChatInstalled` 是 getter 还是方法。按实际改，**逻辑不变**。tobias 同理核对 `auth` / `pay` / `isAliPayInstalled`。

- [ ] **Step 5: 仓库与控制器**

`repositories.dart`：

```dart
  /// 第三方登录。credential:微信 code / 支付宝 auth_code / Apple identityToken。
  Future<AuthResult> loginWithProvider(String provider, {required String credential});
  Future<void> bindOAuth(String provider, String credential);
  /// `GET /auth/alipay/auth-info` —— 支付宝 SDK 要的服务端签名授权串。
  Future<String> alipayAuthInfo();
```

`remote_repositories.dart`：

```dart
/// 各家凭据在请求体里的键不一样:服务端按 provider 分路由,键名也随路由。
Map<String, Object?> oauthBody(String provider, String credential) => {
      'appid': AppConfig.appId,
      switch (provider) {
        'wechat' => 'code',
        'alipay' => 'auth_code',
        _ => 'id_token',
      }: credential,
    };

  @override
  Future<AuthResult> loginWithProvider(String provider, {required String credential}) async {
    final data = await _api.post<dynamic>('/auth/$provider', body: oauthBody(provider, credential));
    return _authResult(data);
  }

  @override
  Future<void> bindOAuth(String provider, String credential) =>
      _api.post<dynamic>('/auth/$provider/bind', body: oauthBody(provider, credential));

  @override
  Future<String> alipayAuthInfo() async {
    final data = _obj(await _api.get<dynamic>('/auth/alipay/auth-info', query: {'appid': AppConfig.appId}));
    return '${data['auth_info'] ?? ''}';
  }
```

（`_api.get` 的 query 参数名以 `core/network` 里的实际签名为准。）

`mock_repositories.dart`：`loginWithProvider(String provider, {required String credential})` 签名对齐；`bindOAuth(String provider, String credential)`；加 `Future<String> alipayAuthInfo() async => 'mock-auth-info';`。

`auth_controller.dart` `loginWithProvider` 里取凭据的三行换成：

```dart
      final client = ref.read(oauthClientProvider);
      final credential = switch (provider) {
        'wechat' => await client.wechatCode(),
        'alipay' => await client.alipayAuthCode(
            await ref.read(authRepoProvider).alipayAuthInfo(),
          ),
        _ => await client.appleIdToken(),
      };
      if (state.pendingProvider != provider) return false;
      final result = await ref
          .read(authRepoProvider)
          .loginWithProvider(provider, credential: credential);
```

注释里的「DEVELOPER_ERROR / SHA-1」段落改成微信的对应项：「最常见失败是 `-6`/未拉起:开放平台登记的包名 + 签名 MD5 与安装包不符,Debug 包签名与 Release 不同」。

`oauth_pending_overlay.dart` / `oauth_conflict_page.dart`：凡是 `provider == 'apple' ? 'Apple' : 'Google'` 之类改用 `brandName(provider)`。

`test/widgets/account_bind_retry_test.dart` 的 `_FakeOAuth`：

```dart
class _FakeOAuth implements OAuthClient {
  @override
  Future<String> wechatCode() async => 'code';
  @override
  Future<String> alipayAuthCode(String authInfo) async => 'auth';
  @override
  Future<String> appleIdToken() async => 'token';
  @override
  Future<bool> get wechatInstalled async => true;
  @override
  Future<bool> get alipayInstalled async => true;
}
```

- [ ] **Step 6: 运行确认通过**

Run: `flutter test test/oauth_results_test.dart && flutter analyze`
Expected: PASS；analyze 剩余报错应只在 `login_page.dart` / `account_security_page.dart`（`googleIdToken` 调用，Task 13 / 15 处理）

---

### Task 13: 登录页第三方按钮：微信 / 支付宝 / Apple

**Files:**
- Modify: `lib/ui/widgets/brand_marks.dart`（加 `WechatMark`、`AlipayMark`、`BrandCircleButton`）
- Modify: `lib/features/auth/login_page.dart:430-450`
- Modify: `lib/l10n/app_zh.arb`、`lib/l10n/app_en.arb`
- Modify: `test/widgets/login_channels_test.dart`（追加）

**Interfaces:**
- Consumes: Task 11 `appConfig.wechatLogin / alipayLogin`；Task 12 `showAppleSignIn`、`_oauth(provider)`。
- Produces: `WechatMark({double size = 28})`、`AlipayMark({double size = 28})`、`BrandCircleButton({required Widget mark, required Color color, required String label, VoidCallback? onTap})`；l10n 键 `loginWechat` `loginAlipay` `loginApple` `loginOpeningWechat` `loginOpeningAlipay` `loginWechatNotInstalled` `loginAlipayNotInstalled` `channelNotConfigured`。

- [ ] **Step 1: 写失败测试**（追加到 `login_channels_test.dart`）

```dart
class _NoThirdParty extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({
    'auth': {'wechat': false, 'alipay': false},
  });
}

class _AlipayOnly extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({
    'auth': {'wechat': false, 'alipay': true},
  });
}

  // ---- 追加到 main() ----
  testWidgets('default: WeChat + Alipay circles, no Google', (t) async {
    await pumpAt(t, layoutMatrix[2], const LoginPage());
    expect(find.byType(WechatMark), findsOneWidget);
    expect(find.byType(AlipayMark), findsOneWidget);
    expect(find.text('Continue with Google'), findsNothing);
    expect(find.textContaining('Google'), findsNothing);
  });

  testWidgets('alipay only: one circle', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const LoginPage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_AlipayOnly.new)],
    );
    expect(find.byType(WechatMark), findsNothing);
    expect(find.byType(AlipayMark), findsOneWidget);
  });

  testWidgets('no third-party (Android): divider and row disappear', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const LoginPage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_NoThirdParty.new)],
    );
    expect(find.byType(BrandCircleButton), findsNothing);
    expect(find.text('或'), findsNothing);
    expect(find.text('or'), findsNothing);
  });
```

import 补 `package:bottles/ui/widgets/brand_marks.dart`。

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/widgets/login_channels_test.dart`
Expected: FAIL（`WechatMark` 未定义）

- [ ] **Step 3: 实现 brand_marks.dart**（追加）

```dart
/// 微信「两个气泡」。自绘,白色气泡落在品牌绿底上(由 BrandCircleButton 提供底色)。
class WechatMark extends StatelessWidget {
  const WechatMark({super.key, this.size = 28, this.color = Colors.white});
  final double size;
  final Color color;
  @override
  Widget build(BuildContext context) =>
      CustomPaint(size: Size.square(size), painter: _WechatPainter(color));
}

class _WechatPainter extends CustomPainter {
  const _WechatPainter(this.color);
  final Color color;
  @override
  void paint(Canvas canvas, Size s) {
    final w = s.width;
    final paint = Paint()..color = color;
    // 大气泡左上,小气泡右下,略重叠;眼睛用底色挖出。
    final big = Rect.fromLTWH(0, w * .08, w * .66, w * .54);
    final small = Rect.fromLTWH(w * .40, w * .36, w * .56, w * .46);
    canvas.drawOval(big, paint);
    canvas.drawOval(small, paint);
    final eye = Paint()..blendMode = BlendMode.clear;
    canvas.saveLayer(Offset.zero & s, Paint());
    canvas.drawOval(big, paint);
    canvas.drawOval(small, paint);
    for (final c in [Offset(w * .22, w * .30), Offset(w * .44, w * .30)]) {
      canvas.drawCircle(c, w * .045, eye);
    }
    for (final c in [Offset(w * .58, w * .56), Offset(w * .76, w * .56)]) {
      canvas.drawCircle(c, w * .04, eye);
    }
    canvas.restore();
  }

  @override
  bool shouldRepaint(covariant _WechatPainter old) => old.color != color;
}

/// 支付宝「支」字标。品牌蓝底由 BrandCircleButton 提供。
class AlipayMark extends StatelessWidget {
  const AlipayMark({super.key, this.size = 28, this.color = Colors.white});
  final double size;
  final Color color;
  @override
  Widget build(BuildContext context) => SizedBox.square(
        dimension: size,
        child: Center(
          child: Text(
            '支',
            style: TextStyle(
              color: color,
              fontSize: size * .72,
              fontWeight: FontWeight.w800,
              height: 1,
            ),
          ),
        ),
      );
}

/// 第三方登录圆形按钮:48pt 品牌色圆 + 下方 12pt 标签。原型 A2 的「其他方式」区。
class BrandCircleButton extends StatelessWidget {
  const BrandCircleButton({
    super.key,
    required this.mark,
    required this.color,
    required this.label,
    this.onTap,
  });
  final Widget mark;
  final Color color;
  final String label;
  final VoidCallback? onTap;

  static const wechatGreen = Color(0xFF07C160);
  static const alipayBlue = Color(0xFF1677FF);

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null;
    return Semantics(
      button: true,
      label: label,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Opacity(
          opacity: enabled ? 1 : .4,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
                alignment: Alignment.center,
                child: mark,
              ),
              const SizedBox(height: 6),
              Text(label, style: const TextStyle(fontSize: 12)),
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 4: 登录页**

`login_page.dart` 把 `_OrDivider` 到 Apple 按钮那段替换为：

```dart
                          if (socials.isNotEmpty) ...[
                            const SizedBox(height: Dim.s4),
                            _OrDivider(label: l.commonOr),
                            const SizedBox(height: Dim.s4),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                for (var i = 0; i < socials.length; i++) ...[
                                  if (i > 0) const SizedBox(width: 28),
                                  socials[i],
                                ],
                              ],
                            ),
                          ],
```

在 `build` 里（拿到 `l` 与 `cfg = ref.watch(appConfigProvider)` 之后）组装：

```dart
    final socials = <Widget>[
      if (cfg.wechatLogin)
        BrandCircleButton(
          mark: const WechatMark(),
          color: BrandCircleButton.wechatGreen,
          label: l.loginWechat,
          onTap: () => _oauth('wechat'),
        ),
      if (cfg.alipayLogin)
        BrandCircleButton(
          mark: const AlipayMark(),
          color: BrandCircleButton.alipayBlue,
          label: l.loginAlipay,
          onTap: () => _oauth('alipay'),
        ),
      if (showAppleSignIn)
        BrandCircleButton(
          mark: const Icon(Icons.apple_rounded, color: Colors.white, size: 28),
          color: Colors.black,
          label: l.loginApple,
          onTap: () => _oauth('apple'),
        ),
    ];
```

删掉 `GoogleMark` 的 import 与用法。`_oauth` 前面加「未安装」检查：

```dart
  Future<void> _oauth(String provider) async {
    if (!await _ensureAgreed()) return;
    final client = ref.read(oauthClientProvider);
    final l = L.of(context);
    try {
      if (provider == 'wechat' && !await client.wechatInstalled) {
        if (mounted) showToast(context, l.loginWechatNotInstalled, error: true);
        return;
      }
      if (provider == 'alipay' && !await client.alipayInstalled) {
        if (mounted) showToast(context, l.loginAlipayNotInstalled, error: true);
        return;
      }
    } catch (_) {
      // 查询安装状态失败(没 AppID 等)照常往下走,由登录流程给出明确错误。
    }
    if (!mounted) return;
    // ...原有逻辑不变...
  }
```

A2g 等待层文案：`oauth_pending_overlay.dart` 里用 `provider == 'wechat' ? l.loginOpeningWechat : provider == 'alipay' ? l.loginOpeningAlipay : <原 Apple 文案>`。

ARB（zh / en）：

```json
  "loginWechat": "微信",            "loginWechat": "WeChat",
  "loginAlipay": "支付宝",          "loginAlipay": "Alipay",
  "loginApple": "Apple",            "loginApple": "Apple",
  "loginOpeningWechat": "正在打开微信…",      "loginOpeningWechat": "Opening WeChat…",
  "loginOpeningAlipay": "正在打开支付宝…",    "loginOpeningAlipay": "Opening Alipay…",
  "loginWechatNotInstalled": "未安装微信",    "loginWechatNotInstalled": "WeChat is not installed",
  "loginAlipayNotInstalled": "未安装支付宝",  "loginAlipayNotInstalled": "Alipay is not installed",
  "channelNotConfigured": "该方式暂未开放",    "channelNotConfigured": "Not available yet"
```

然后 `flutter gen-l10n`。

- [ ] **Step 5: 运行确认通过**

Run: `flutter test test/widgets/login_channels_test.dart test/layout/auth_layout_test.dart && flutter analyze`
Expected: PASS（`auth_layout_test` 若断言了 Google 按钮文案，改为断言 `BrandCircleButton`）

---

### Task 14: 支付适配器 + 充值页渠道 + 支付流程页选适配器

**Files:**
- Modify: `lib/domain/models/wallet.dart:83-137`
- Create: `lib/core/pay/wechat_pay_adapter.dart`、`lib/core/pay/alipay_adapter.dart`、`lib/core/pay/sdk_gateways.dart`、`lib/core/pay/channel_select.dart`
- Modify: `lib/features/me/payment_flow_page.dart:78-90`
- Modify: `lib/features/me/recharge_page.dart`
- Modify: `lib/data/remote/remote_repositories.dart:745-765`
- Modify: `lib/l10n/*.arb`
- Create: `test/pay_adapters_test.dart`、`test/widgets/recharge_channels_test.dart`
- Modify: `test/pay_channel_test.dart`（不变，确认仍过）

**Interfaces:**
- Produces:
  - `enum PayChannel { wechat, alipay, iap }` + `extension PayChannelWire on PayChannel { String? get wire }`（`wx_app` / `alipay_app` / null）
  - `PendingOrder.isAlipay`（有 `order_str`）、`PendingOrder.isWechat`（有 `prepayid`）
  - `enum ChannelKind { mock, wechat, alipay, unknown }`、`ChannelKind channelKindOf(PendingOrder o)`
  - `class WechatPayAdapter implements PayChannelAdapter { WechatPayAdapter({required Future<int?> Function(Map<String, dynamic>) pay}); }`、`ChannelOutcome wechatPayOutcome(int? errCode)`
  - `class AlipayAdapter implements PayChannelAdapter { AlipayAdapter({required Future<String?> Function(String orderStr) pay}); }`、`ChannelOutcome alipayOutcome(String? resultStatus)`
  - `sdk_gateways.dart`：`Future<int?> wechatPayViaSdk(Map<String, dynamic> p, {required String appId, required String universalLink})`、`Future<String?> alipayViaSdk(String orderStr)`
  - `WalletRepository.recharge(pkg, channel)` 请求体带 `channel: channel.wire`（null 不带）

- [ ] **Step 1: 写失败测试**

```dart
// test/pay_adapters_test.dart
import 'package:bottles/core/pay/alipay_adapter.dart';
import 'package:bottles/core/pay/channel_select.dart';
import 'package:bottles/core/pay/pay_channel.dart';
import 'package:bottles/core/pay/wechat_pay_adapter.dart';
import 'package:bottles/domain/models/wallet.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('wechat', () {
    test('errCode → outcome', () {
      expect(wechatPayOutcome(0), ChannelOutcome.launched);
      expect(wechatPayOutcome(null), ChannelOutcome.launched); // 没回调:让轮询定
      expect(wechatPayOutcome(-2), ChannelOutcome.cancelled);
      expect(wechatPayOutcome(-1), ChannelOutcome.failed);
    });
    test('adapter hands pay_params to the SDK closure', () async {
      Map<String, dynamic>? got;
      final a = WechatPayAdapter(pay: (p) async {
        got = p;
        return 0;
      });
      const o = PendingOrder(orderNo: 'N1', payParams: {'prepayid': 'wx1', 'sign': 's'});
      expect(await a.launch(o), ChannelOutcome.launched);
      expect(got?['prepayid'], 'wx1');
    });
  });

  group('alipay', () {
    test('resultStatus → outcome', () {
      expect(alipayOutcome('9000'), ChannelOutcome.launched);
      expect(alipayOutcome('8000'), ChannelOutcome.launched);
      expect(alipayOutcome('6004'), ChannelOutcome.launched);
      expect(alipayOutcome(null), ChannelOutcome.launched);
      expect(alipayOutcome('6001'), ChannelOutcome.cancelled);
      expect(alipayOutcome('4000'), ChannelOutcome.failed);
    });
    test('adapter passes order_str', () async {
      String? got;
      final a = AlipayAdapter(pay: (s) async {
        got = s;
        return '9000';
      });
      const o = PendingOrder(orderNo: 'N1', payParams: {'order_str': 'app_id=1&sign=x'});
      expect(await a.launch(o), ChannelOutcome.launched);
      expect(got, 'app_id=1&sign=x');
    });
    test('missing order_str fails without calling the SDK', () async {
      var called = false;
      final a = AlipayAdapter(pay: (_) async {
        called = true;
        return '9000';
      });
      expect(await a.launch(const PendingOrder(orderNo: 'N1', payParams: {})), ChannelOutcome.failed);
      expect(called, isFalse);
    });
  });

  test('channelKindOf picks by pay_params shape', () {
    expect(channelKindOf(const PendingOrder(orderNo: 'a', payParams: {'mock': true})), ChannelKind.mock);
    expect(channelKindOf(const PendingOrder(orderNo: 'a', payParams: {'order_str': 'x'})), ChannelKind.alipay);
    expect(channelKindOf(const PendingOrder(orderNo: 'a', payParams: {'prepayid': 'x'})), ChannelKind.wechat);
    expect(channelKindOf(const PendingOrder(orderNo: 'a', payParams: {})), ChannelKind.unknown);
  });

  test('PayChannel wire names', () {
    expect(PayChannel.wechat.wire, 'wx_app');
    expect(PayChannel.alipay.wire, 'alipay_app');
    expect(PayChannel.iap.wire, isNull);
  });
}
```

```dart
// test/widgets/recharge_channels_test.dart
import 'package:bottles/core/config/remote_config.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/features/me/recharge_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

class _AlipayOff extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({'pay': {'wechat': true, 'alipay': false}});
}

class _NoChannel extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({'pay': {'wechat': false, 'alipay': false}});
}

/// H3(Android):渠道行按 /app-config 显隐;两个都关时按钮禁用并提示。
/// 测试跑在非 iOS 平台,所以走的是第三方渠道分支。
void main() {
  testWidgets('both channels: two radio rows, WeChat preselected', (t) async {
    await pumpAt(t, layoutMatrix[2], const RechargePage());
    expect(find.byType(RadioRow), findsNWidgets(2));
    expect(find.text('微信支付'), findsOneWidget);
    expect(find.text('支付宝'), findsOneWidget);
  });

  testWidgets('alipay off: only WeChat row', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const RechargePage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_AlipayOff.new)],
    );
    expect(find.byType(RadioRow), findsOneWidget);
    expect(find.text('支付宝'), findsNothing);
  });

  testWidgets('no channel: notice shown, no radio rows', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const RechargePage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_NoChannel.new)],
    );
    expect(find.byType(RadioRow), findsNothing);
    expect(find.text('暂未开放充值'), findsOneWidget);
  });
}
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/pay_adapters_test.dart test/widgets/recharge_channels_test.dart`
Expected: FAIL

- [ ] **Step 3: 模型与适配器**

`wallet.dart`：

```dart
/// 充值渠道。Android 二选一;iOS 恒 iap(3.1.1)。
enum PayChannel { wechat, alipay, iap }

extension PayChannelWire on PayChannel {
  /// 服务端 `POST /pay/order` 的 channel 取值;iap 不走这条接口。
  String? get wire => switch (this) {
        PayChannel.wechat => 'wx_app',
        PayChannel.alipay => 'alipay_app',
        PayChannel.iap => null,
      };
}

// PendingOrder 追加
  /// 支付宝 App 支付:服务端签好的 orderStr(`pay/driver_alipay.go`)。
  bool get isAlipay => (payParams['order_str'] as String?)?.isNotEmpty == true;

  /// 微信 App 支付:prepayid 等拉起参数(`pay/driver_wx.go` APP 分支)。
  bool get isWechat => (payParams['prepayid'] as String?)?.isNotEmpty == true;
```

`lib/core/pay/channel_select.dart`：

```dart
import '../../domain/models/wallet.dart';

enum ChannelKind { mock, wechat, alipay, unknown }

/// 按 pay_params 的形状选适配器:服务端没有单独的「渠道」字段,形状就是契约。
ChannelKind channelKindOf(PendingOrder o) {
  if (o.isMock) return ChannelKind.mock;
  if (o.isAlipay) return ChannelKind.alipay;
  if (o.isWechat) return ChannelKind.wechat;
  return ChannelKind.unknown;
}
```

`lib/core/pay/wechat_pay_adapter.dart`：

```dart
import '../../domain/models/wallet.dart';
import 'pay_channel.dart';

/// 微信 SDK 的 errCode → 渠道结果。0 成功、-2 取消、null = 没等到回调(用户切去微信又回桌面)。
/// 成功 / 无回调都只是「拉起了」:到没到账由服务端订单状态说了算。
ChannelOutcome wechatPayOutcome(int? errCode) => switch (errCode) {
      0 || null => ChannelOutcome.launched,
      -2 => ChannelOutcome.cancelled,
      _ => ChannelOutcome.failed,
    };

class WechatPayAdapter implements PayChannelAdapter {
  const WechatPayAdapter({required this.pay});

  /// 把 pay_params 交给 SDK,返回 errCode;超时无回调返回 null。真实实现见 sdk_gateways.dart。
  final Future<int?> Function(Map<String, dynamic> params) pay;

  @override
  Future<ChannelOutcome> launch(PendingOrder order) async {
    if (!order.isWechat) return ChannelOutcome.failed;
    return wechatPayOutcome(await pay(order.payParams));
  }
}
```

`lib/core/pay/alipay_adapter.dart`：

```dart
import '../../domain/models/wallet.dart';
import 'pay_channel.dart';

/// 支付宝 resultStatus → 渠道结果。9000 成功、8000/6004 处理中、6001 取消、其它失败。
ChannelOutcome alipayOutcome(String? resultStatus) => switch (resultStatus) {
      '9000' || '8000' || '6004' || null => ChannelOutcome.launched,
      '6001' => ChannelOutcome.cancelled,
      _ => ChannelOutcome.failed,
    };

class AlipayAdapter implements PayChannelAdapter {
  const AlipayAdapter({required this.pay});

  /// 把 orderStr 交给 SDK,返回 resultStatus。
  final Future<String?> Function(String orderStr) pay;

  @override
  Future<ChannelOutcome> launch(PendingOrder order) async {
    if (!order.isAlipay) return ChannelOutcome.failed;
    return alipayOutcome(await pay(order.payParams['order_str'] as String));
  }
}
```

`lib/core/pay/sdk_gateways.dart`（唯一碰 SDK 的地方；字段 / 方法名按 pub get 后的源码核对）：

```dart
import 'dart:async';

import 'package:fluwx/fluwx.dart';
import 'package:tobias/tobias.dart';

/// 微信 App 支付:注册 → pay → 等 WeChatPaymentResponse。60 秒无回调返回 null。
Future<int?> wechatPayViaSdk(
  Map<String, dynamic> p, {
  required String appId,
  required String universalLink,
}) async {
  final fluwx = Fluwx();
  if (appId.isEmpty) return -1;
  await fluwx.registerApi(appId: appId, universalLink: universalLink.isEmpty ? null : universalLink);
  final done = Completer<int?>();
  void onResp(WeChatResponse r) {
    if (r is WeChatPaymentResponse && !done.isCompleted) done.complete(r.errCode);
  }

  fluwx.addSubscriber(onResp);
  try {
    final ok = await fluwx.pay(
      which: Payment(
        appId: '${p['appid']}',
        partnerId: '${p['partnerid']}',
        prepayId: '${p['prepayid']}',
        packageValue: '${p['package']}',
        nonceStr: '${p['noncestr']}',
        timestamp: int.tryParse('${p['timestamp']}') ?? 0,
        sign: '${p['sign']}',
      ),
    );
    if (!ok) return -1;
    return await done.future.timeout(const Duration(seconds: 60), onTimeout: () => null);
  } finally {
    fluwx.removeSubscriber(onResp);
  }
}

/// 支付宝 App 支付。
Future<String?> alipayViaSdk(String orderStr) async {
  final r = await Tobias().pay(orderStr);
  final v = r['resultStatus'];
  return v == null ? null : '$v';
}
```

- [ ] **Step 4: 支付流程页与仓库**

`payment_flow_page.dart` `_start` 末尾替换为：

```dart
    final repo = ref.read(walletRepoProvider);
    await ctrl.start(_adapterFor(pending, repo), pending);
  }

  /// 按下单结果的形状选渠道适配器;本页其余代码不认识任何 SDK。
  PayChannelAdapter _adapterFor(PendingOrder pending, WalletRepository repo) {
    switch (channelKindOf(pending)) {
      case ChannelKind.alipay:
        return const AlipayAdapter(pay: alipayViaSdk);
      case ChannelKind.wechat:
        final cfg = ref.read(appConfigProvider);
        return WechatPayAdapter(
          pay: (p) => wechatPayViaSdk(p, appId: cfg.wechatAppId, universalLink: cfg.wechatUniversalLink),
        );
      case ChannelKind.mock:
      case ChannelKind.unknown:
        // unknown 也给 mock 面板:那只会在 mock 开关开着时出现(服务端 resolveChannel 保证),
        // 生产上不会走到。真走到了,面板的 settle 会被服务端 404 掉,页面照常超时到 H9。
        return MockChannelAdapter(settle: repo.settleMock, ask: () => _askMockChoice());
    }
  }
```

import `../../core/pay/alipay_adapter.dart`、`wechat_pay_adapter.dart`、`sdk_gateways.dart`、`channel_select.dart`、`../../data/repositories.dart`。

`remote_repositories.dart` `recharge`：

```dart
        body: {
          'package_id': int.tryParse(pkg.id) ?? 0,
          // Android 国内版:wx_app / alipay_app;iOS IAP 不走这里。
          if (channel.wire != null) 'channel': channel.wire,
        },
```

同时把那段「⚠️ 支付链路未完成」的旧注释删掉（它已经完成了）。

- [ ] **Step 5: 充值页**

`recharge_page.dart`：

```dart
  PayChannel? _channel; // null = 还没选 / 没有可用渠道

  List<PayChannel> _available(AppRemoteConfig cfg) => [
        if (cfg.payWechat) PayChannel.wechat,
        if (cfg.payAlipay) PayChannel.alipay,
      ];
```

`_pay` 里 `final channel = _iosIap ? PayChannel.iap : _channel;` 后加 `if (channel == null) return;`，`recharge(pkg, channel)` 用非空值。

`build` 里取 `final cfg = ref.watch(appConfigProvider); final avail = _available(cfg); final chosen = _channel != null && avail.contains(_channel) ? _channel : (avail.isEmpty ? null : avail.first);`。

渠道区的 `else` 分支替换为：

```dart
                          ] else if (avail.isEmpty) ...[
                            NoticeBanner(icon: Icons.info_outline_rounded, text: l.rechargeNoChannel),
                          ] else ...[
                            for (var i = 0; i < avail.length; i++) ...[
                              if (i > 0) const SizedBox(height: Dim.s2),
                              RadioRow(
                                icon: avail[i] == PayChannel.wechat
                                    ? Icons.chat_bubble_rounded
                                    : Icons.account_balance_wallet_rounded,
                                label: avail[i] == PayChannel.wechat ? l.rechargeWechat : l.rechargeAlipay,
                                selected: chosen == avail[i],
                                onTap: () => setState(() => _channel = avail[i]),
                              ),
                            ],
                          ],
```

底部按钮 `onTap` 条件加 `|| (!_iosIap && chosen == null)` 时为 null；`_pay` 用 `chosen`（把它存到 state：`_channel ??= chosen` 后调用）。

删除 `PayChannel.upi` / `card` 的所有引用（`grep -rn "PayChannel\.\(upi\|card\)" lib test`），ARB 删 `rechargeUpiSub` / `rechargeCardSub`（两个文件都删），加：

```json
  "rechargeWechat": "微信支付",     "rechargeWechat": "WeChat Pay",
  "rechargeAlipay": "支付宝",       "rechargeAlipay": "Alipay",
  "rechargeNoChannel": "暂未开放充值", "rechargeNoChannel": "Top-up is not available yet"
```

`flutter gen-l10n`。

- [ ] **Step 6: 运行确认通过**

Run: `flutter test test/pay_adapters_test.dart test/pay_channel_test.dart test/payment_flow_test.dart test/payment_controller_test.dart test/widgets/recharge_channels_test.dart test/layout/me_layout_test.dart && flutter analyze`
Expected: PASS；No issues found

---

### Task 15: 账号与安全页绑定项

**Files:**
- Modify: `lib/features/me/account_security_page.dart:55-70,190-245`
- Modify: `lib/domain/models/user.dart:160-265`
- Modify: `lib/l10n/*.arb`
- Modify: `test/widgets/account_bind_retry_test.dart`（断言文案若写死 Google 要改）

**Interfaces:**
- Consumes: Task 12 `OAuthClient`、`brandName`、`authRepo.bindOAuth(provider, credential)`。
- Produces: `UserProfile.wechatBound` / `alipayBound`（解析 `wechat_bound` / `alipay_bound`）；l10n `accountWechat`、`accountAlipay`。

- [ ] **Step 1: 写失败测试**

```dart
// test/widgets/account_rows_test.dart
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/me/account_security_page.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

void main() {
  test('UserProfile parses CN bound flags', () {
    final u = UserProfile.fromJson({'id': '1', 'nickname': 'n', 'wechat_bound': true, 'alipay_bound': false});
    expect(u.wechatBound, isTrue);
    expect(u.alipayBound, isFalse);
  });

  testWidgets('A6: rows are 微信 / 支付宝, no Google', (t) async {
    await pumpAt(t, layoutMatrix[2], const AccountSecurityPage());
    await t.pumpAndSettle();
    expect(find.text('微信'), findsOneWidget);
    expect(find.text('支付宝'), findsOneWidget);
    expect(find.text('Google'), findsNothing);
  });
}
```

- [ ] **Step 2: 运行确认失败**

Run: `flutter test test/widgets/account_rows_test.dart`
Expected: FAIL

- [ ] **Step 3: 实现**

`user.dart`：加 `this.wechatBound = false, this.alipayBound = false,`、`final bool wechatBound; final bool alipayBound;`、`fromJson` 解析 `j['wechat_bound'] == true` / `j['alipay_bound'] == true`、`copyWith` 透传。

`account_security_page.dart`：

```dart
  String _brand(String provider) => brandName(provider);

  Future<String> _credential(String provider) async {
    final client = ref.read(oauthClientProvider);
    return switch (provider) {
      'wechat' => client.wechatCode(),
      'alipay' => client.alipayAuthCode(await ref.read(authRepoProvider).alipayAuthInfo()),
      _ => client.appleIdToken(),
    };
  }
  // _bind 里:
      final credential = await _credential(provider);
      await ref.read(authRepoProvider).bindOAuth(provider, credential);
```

`ways` 列表换成 `[phone非空, email非空, me?.wechatBound == true, me?.alipayBound == true, me?.appleBound == true]`。

`ListGroup` 的 Google 行换成两行：

```dart
              _OAuthRow(
                title: l.accountWechat,
                bound: me?.wechatBound == true,
                busy: _busy || me == null,
                canUnbind: canUnbind,
                onBind: () => _bind('wechat'),
                onUnbind: () => _unbind('wechat'),
              ),
              _OAuthRow(
                title: l.accountAlipay,
                bound: me?.alipayBound == true,
                busy: _busy || me == null,
                canUnbind: canUnbind,
                onBind: () => _bind('alipay'),
                onUnbind: () => _unbind('alipay'),
              ),
```

Apple 行保持（iOS 显示 `_OAuthRow`，Android 显示「仅 iOS」）。import `../../core/platform/oauth_results.dart`。

ARB：`"accountWechat": "微信"` / `"WeChat"`，`"accountAlipay": "支付宝"` / `"Alipay"`。`flutter gen-l10n`。

`account_bind_retry_test.dart` 若断言 `'Google'` 相关文案改为对应微信行；其「换一个账号」逻辑不变。

- [ ] **Step 4: 运行确认通过**

Run: `flutter test test/widgets/account_rows_test.dart test/widgets/account_bind_retry_test.dart && flutter analyze`
Expected: PASS；No issues found

---

### Task 16: Android / iOS 平台配置

**Files:**
- Modify: `android/app/src/main/AndroidManifest.xml`
- Modify: `android/app/build.gradle.kts`（Release 签名从 `key.properties` 读）
- Modify: `android/.gitignore`（确认含 `key.properties` / `*.jks`）
- Modify: `ios/Runner/Info.plist`
- Create: `ios/Runner/Runner.entitlements`
- Modify: `ios/Runner.xcodeproj/project.pbxproj`（`CODE_SIGN_ENTITLEMENTS`）
- Modify: `ios/Flutter/Debug.xcconfig`、`Release.xcconfig`（`WECHAT_APP_ID`）

- [ ] **Step 1: 核对 fluwx 的 Android 入口 Activity**

Run: `grep -rn "WXEntryActivity\|WXPayEntryActivity\|activity" "$(flutter pub cache dir 2>/dev/null || echo "$LOCALAPPDATA/Pub/Cache")/hosted/pub.dev/fluwx-"*/android/src/main/AndroidManifest.xml`
按输出决定下一步：
- 若插件 manifest 已声明 `com.jarvan.fluwx.wxapi.FluwxWXEntryActivity` / `FluwxWXPayEntryActivity` → Step 2 用 `activity-alias`。
- 若插件 README / 源码要求在应用包的 `wxapi` 子包自己建 `WXEntryActivity`（继承插件提供的基类）→ 在 `kotlin/com/ambertu/bottles/cn/wxapi/` 建两个空子类并在 manifest 注册 `.wxapi.WXEntryActivity` / `.wxapi.WXPayEntryActivity`（`exported=true`、`launchMode=singleTop`、`theme=@android:style/Theme.Translucent.NoTitleBar`）。

- [ ] **Step 2: AndroidManifest**

`<queries>` 里追加：

```xml
        <!-- 微信 / 支付宝 SDK 的包可见性(Android 11+):不声明 isWeChatInstalled / isAliPayInstalled 恒 false -->
        <package android:name="com.tencent.mm"/>
        <package android:name="com.eg.android.AlipayGphone"/>
```

`<application>` 里追加（按 Step 1 结论二选一；这是 alias 版）：

```xml
        <!-- 微信 SDK 回调入口。微信按「包名.wxapi.WXEntryActivity」反射查找,所以别名必须叫这个名字。 -->
        <activity-alias
            android:name="${applicationId}.wxapi.WXEntryActivity"
            android:exported="true"
            android:launchMode="singleTop"
            android:theme="@android:style/Theme.Translucent.NoTitleBar"
            android:targetActivity="com.jarvan.fluwx.wxapi.FluwxWXEntryActivity"/>
        <activity-alias
            android:name="${applicationId}.wxapi.WXPayEntryActivity"
            android:exported="true"
            android:launchMode="singleTop"
            android:theme="@android:style/Theme.Translucent.NoTitleBar"
            android:targetActivity="com.jarvan.fluwx.wxapi.FluwxWXPayEntryActivity"/>
```

- [ ] **Step 3: Release 签名**

`build.gradle.kts` 顶部 `mapsApiKey` 旁加：

```kotlin
// 正式签名从 android/key.properties 读(gitignore)。微信开放平台按「包名 + 签名 MD5」绑定,
// 用 debug 签名出的包微信直接拒绝拉起,所以 Release 必须走这里。
val keyProps = Properties().apply {
    val f = rootProject.file("key.properties")
    if (f.exists()) f.inputStream().use { load(it) }
}
```

`android {}` 里：

```kotlin
    signingConfigs {
        create("release") {
            if (keyProps.getProperty("storeFile") != null) {
                storeFile = file(keyProps.getProperty("storeFile"))
                storePassword = keyProps.getProperty("storePassword")
                keyAlias = keyProps.getProperty("keyAlias")
                keyPassword = keyProps.getProperty("keyPassword")
            }
        }
    }
    buildTypes {
        release {
            signingConfig = if (keyProps.getProperty("storeFile") != null)
                signingConfigs.getByName("release") else signingConfigs.getByName("debug")
        }
    }
```

`android/.gitignore` 确认有 `key.properties` 与 `*.jks`；没有就加。另在 `android/key.properties.example` 写四个键的样例。

- [ ] **Step 4: iOS**

`Info.plist` 的 `LSApplicationQueriesSchemes` 数组追加 `weixin`、`weixinULAPI`、`weixinURLParamsAPI`、`alipay`、`alipays`；新增：

```xml
	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleTypeRole</key><string>Editor</string>
			<key>CFBundleURLName</key><string>weixin</string>
			<key>CFBundleURLSchemes</key><array><string>$(WECHAT_APP_ID)</string></array>
		</dict>
		<dict>
			<key>CFBundleTypeRole</key><string>Editor</string>
			<key>CFBundleURLName</key><string>alipay</string>
			<key>CFBundleURLSchemes</key><array><string>ambertubottlescn</string></array>
		</dict>
	</array>
```

`ios/Flutter/Debug.xcconfig` 与 `Release.xcconfig` 各加一行 `WECHAT_APP_ID=wxPLACEHOLDER`（账号到手后改；留注释「微信开放平台 AppID,与 /app-config 下发的一致」）。

`ios/Runner/Runner.entitlements`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.developer.associated-domains</key>
	<array>
		<string>applinks:ambertu.com</string>
	</array>
</dict>
</plist>
```

`project.pbxproj` 的三个 Runner 配置块各加 `CODE_SIGN_ENTITLEMENTS = Runner/Runner.entitlements;`（与 `PRODUCT_BUNDLE_IDENTIFIER = com.ambertu.bottles.cn;` 同块），并把文件加入 `Runner` group 的 `PBXFileReference`（在 Xcode 里拖入最稳；无 Mac 时手工按 `Info.plist` 的条目照抄一份 fileRef）。

- [ ] **Step 5: 验证**

Run: `flutter build apk --debug`（本机有 Android SDK 时）；`flutter analyze`
Expected: APK 构建成功、manifest merge 无冲突；analyze No issues found。iOS 部分无 Mac 不构建，留给真机阶段。

---

### Task 17: 全量验证与收尾

**Files:**
- Modify: `app/bottles_zh/README.md`（顶部加「国内版」说明与 `--dart-define` 示例）
- Modify: `docs/superpowers/specs/2026-10-06-app-zh-wechat-alipay-design.md` §七（若实施中发现字段对应有变，同步）

- [ ] **Step 1: 服务端**

Run: `cd server && go build ./... && go vet ./... && go test ./...`
Expected: 全绿

- [ ] **Step 2: 管理后台**

Run: `cd admin && npm run build`
Expected: 通过（locale key 一致性校验通过）

- [ ] **Step 3: App**

Run: `cd app/bottles_zh && flutter analyze && flutter test`
Expected: No issues found；全部测试通过（含 `test/layout/all_pages_test.dart` 布局矩阵）

- [ ] **Step 4: 核对海外版未被波及**

Run: `cd /j/code/net_workspace/ai-message && git status --short app/bottles | head`
Expected: 无输出（`app/bottles` 没有改动）

- [ ] **Step 5: README**

在 `app/bottles_zh/README.md` 标题下加：

```markdown
> **国内版**（包名 `com.ambertu.bottles.cn`）。登录:微信 / 支付宝 / 手机号(+ iOS Apple);充值:Android 微信支付 / 支付宝,iOS IAP。
> 与海外版 `app/bottles` 同源分叉于 2026-10-06,设计见 `docs/superpowers/specs/2026-10-06-app-zh-wechat-alipay-design.md`。
> 运行:`flutter run --dart-define=APP_ID=drift_app_cn`(其余 define 同海外版)。
> 账号到位后要填的东西见设计文档 §七。
```

- [ ] **Step 6: 交付说明**

向用户列出：改动文件清单、测试结果原文、§七「账号对号入座」表、以及两条待真机验证项（微信 Release 签名 MD5、iOS Universal Link 的 AASA 文件）。**不提交 git**（等用户指示）。
