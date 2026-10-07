package sysconfig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// getAppConfig 打一次 /app-config,返回 data 段。
func getAppConfig(t *testing.T, tenant int64, query string) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(tenant, tenant).Register(r.Group("/api"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app-config"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d", w.Code)
	}
	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	if body.Code != 0 {
		t.Fatalf("code = %d, want 0", body.Code)
	}
	return body.Data
}

func setCache(rows map[int64]map[string]string) {
	mu.Lock()
	cached = rows
	mu.Unlock()
}

// 没配过任何东西时:文案全空(App 用内置 ARB),入口全开(与 App 现状一致)。
//
// 这条是整批键的安全网。默认值一旦被改成非空文案或 "0",
// 所有 App 租户会在发版当天集体改样 —— 那正是不复用小程序 ui_text_*
// 和 fn_show_* 的原因。
func TestAppConfigDefaultsAreInert(t *testing.T) {
	setCache(map[int64]map[string]string{})
	d := getAppConfig(t, 100, "")

	copyBlock := d["copy"].(map[string]any)
	for k, v := range copyBlock {
		if v != "" {
			t.Errorf("copy.%s 默认应为空串(用 App 内置文案), got %q", k, v)
		}
	}
	if s := d["support"].(map[string]any); s["text"] != "" || s["image"] != "" {
		t.Errorf("support 默认应为空, got %+v", s)
	}
	for k, v := range d["mine"].(map[string]any) {
		if v != true {
			t.Errorf("mine.%s 默认应为 true(App 今天就是全显), got %v", k, v)
		}
	}
	if a := d["auth"].(map[string]any); a["phone"] != true || a["email"] != true {
		t.Errorf("auth.phone/email 默认应都开, got %+v", a)
	}
}

// 后台关掉一种登录标识,/app-config 要如实下发;另一种不受影响。
func TestAppConfigLoginChannelSwitches(t *testing.T) {
	setCache(map[int64]map[string]string{100: {KeyAppLoginEmailEnabled: "0"}})
	a := getAppConfig(t, 100, "")["auth"].(map[string]any)
	if a["phone"] != true || a["email"] != false {
		t.Errorf("auth = %+v, want phone on / email off", a)
	}
}

// 语种回退:英文请求优先 _en,_en 空则回退中文键,中文键也空才给空串。
func TestAppConfigLangFallback(t *testing.T) {
	setCache(map[int64]map[string]string{
		100: {
			KeyAppUIOceanTitle:   "今晚的海",
			KeyAppUIOceanTitleEN: "Tonight's Sea",
			KeyAppUIQuotaTitle:   "今天的 {n} 次捞完了", // 只配了中文
			KeyAppContactText:    "客服 QQ 12345",
		},
	})

	zh := getAppConfig(t, 100, "")["copy"].(map[string]any)
	if zh["ocean_title"] != "今晚的海" {
		t.Errorf("中文请求 ocean_title = %q", zh["ocean_title"])
	}

	en := getAppConfig(t, 100, "?lang=en-US")["copy"].(map[string]any)
	if en["ocean_title"] != "Tonight's Sea" {
		t.Errorf("英文请求应取 _en, got %q", en["ocean_title"])
	}
	if en["quota_title"] != "今天的 {n} 次捞完了" {
		t.Errorf("_en 为空时应回退中文键, got %q", en["quota_title"])
	}
	if en["anon_sender"] != "" {
		t.Errorf("两个键都没配应给空串, got %q", en["anon_sender"])
	}

	// 客服文案走同一条回退链
	if s := getAppConfig(t, 100, "?lang=en")["support"].(map[string]any); s["text"] != "客服 QQ 12345" {
		t.Errorf("support.text 英文缺失应回退中文, got %q", s["text"])
	}
}

// 入口开关:显式写 "0" 才隐藏,其余一律显示 —— 与 getMineFunctions 同一口径。
func TestAppConfigMineSwitches(t *testing.T) {
	setCache(map[int64]map[string]string{
		100: {KeyAppFnRecharge: "0", KeyAppFnItems: "1"},
	})
	mine := getAppConfig(t, 100, "")["mine"].(map[string]any)
	if mine["recharge"] != false {
		t.Errorf(`recharge 配了 "0" 应隐藏, got %v`, mine["recharge"])
	}
	if mine["items"] != true {
		t.Errorf("items 配了 \"1\" 应显示, got %v", mine["items"])
	}
	if mine["blocklist"] != true {
		t.Errorf("blocklist 未配应走默认显示, got %v", mine["blocklist"])
	}
}

// Android 拿不到 ID token 就发不出登录请求,而它需要 Web client ID 当
// serverClientId。这条接口是唯一的下发通道,所以它必须在 /app-config 里。
//
// 与服务端验签同源(都读 app_google_client_id):两边分开配,迟早配歪一个,
// 而症状是「登录点了没反应」——最难查的那一类。
func TestAppConfigCarriesGoogleClientID(t *testing.T) {
	// 多个值时第一个是 Web client ID,下发的就是它。
	// 来源已从 sysconfig 改成「服务商 → 登录 → Google」卡片。
	auth := authSectionWith(t,
		func(_ int64, p string) bool { return p == "google" },
		func(_ int64, p, k string) string {
			if p == "google" && k == "client_id" {
				return "web.apps.googleusercontent.com,ios.apps.googleusercontent.com"
			}
			return ""
		})
	if auth["google_client_id"] != "web.apps.googleusercontent.com" {
		t.Errorf("应下发第一个(Web)client ID, got %q", auth["google_client_id"])
	}
}

// 没配过时给空串,而不是缺字段 —— 客户端据此走「未配置」的降级,
// 不是崩在解析上。
func TestAppConfigGoogleClientIDEmptyWhenUnset(t *testing.T) {
	setCache(map[int64]map[string]string{})

	auth := getAppConfig(t, 100, "")["auth"].(map[string]any)
	if auth["google_client_id"] != "" {
		t.Errorf("未配置时应为空串, got %q", auth["google_client_id"])
	}
}

// /app-config 是 App 专用的,免鉴权(冷启动还没登录)。这时回落到哪个租户,
// 决定了 App 拿到的是谁的配置。
//
// ⚠️ 必须是 App 租户(APP_DEFAULT_TENANT_ID),不是小程序的 DEFAULT_TENANT_ID。
// 2026-09-22 线上就栽在这:Google client ID 配在 App 租户,而这个接口回落到
// 小程序租户,于是下发了个空串 —— 修了客户端也照样登录不了。
//
// 同样要紧的是**别把这条改到 tenantOf 里**:/notice /tabs /features /ads
// 都在用它,那些是小程序的接口,一起改等于把小程序的免登配置换成 App 的。
func TestAppConfigFallsBackToTheAppTenant(t *testing.T) {
	// defaultTenant=100(小程序), appTenant=999(App)。断言服务商查询收到的是哪个租户。
	perTenant := map[int64]string{
		100: "miniprogram.apps.googleusercontent.com",
		999: "app.apps.googleusercontent.com",
	}
	h := NewHandler(100, 999).
		WithSSOUsable(func(int64, string) bool { return true }).
		WithSSOField(func(tid int64, p, k string) string {
			if p == "google" && k == "client_id" {
				return perTenant[tid]
			}
			return ""
		})
	got := getAppConfigFromHandler(t, h)["auth"].(map[string]any)["google_client_id"]
	if got != "app.apps.googleusercontent.com" {
		t.Fatalf("应取 App 租户的值, got %q", got)
	}
}

// appTenant 没设(单租户部署)时回落 defaultTenant —— 不能因为没配这个环境变量
// 就让 App 拿到全空配置。
func TestAppConfigFallsBackToDefaultWhenAppTenantUnset(t *testing.T) {
	h := NewHandler(100, 0).
		WithSSOUsable(func(int64, string) bool { return true }).
		WithSSOField(func(tid int64, p, k string) string {
			if tid == 100 && p == "google" && k == "client_id" {
				return "only.apps.googleusercontent.com"
			}
			return ""
		})
	got := getAppConfigFromHandler(t, h)["auth"].(map[string]any)["google_client_id"]
	if got != "only.apps.googleusercontent.com" {
		t.Fatalf("appTenant 为 0 时应回落 defaultTenant, got %q", got)
	}
}

// 聊天扣费规则随 /app-config 下发,App 据此显示价格;数字必须和后台配的一致。
func TestAppConfigCarriesChatPricing(t *testing.T) {
	setCache(map[int64]map[string]string{
		100: {KeyPriceChat: "3", KeyPriceMsg: "1", KeyChatFreeMsgs: "5"},
	})
	p := getAppConfig(t, 100, "")["pricing"].(map[string]any)
	if p["chat_start"] != float64(3) || p["chat_msg"] != float64(1) || p["chat_free_msgs"] != float64(5) {
		t.Errorf("pricing = %+v, want 3/1/5", p)
	}

	// 没配过:开聊沿用默认 5,按条与免费条数都是 0。
	setCache(map[int64]map[string]string{})
	p = getAppConfig(t, 100, "")["pricing"].(map[string]any)
	if p["chat_start"] != float64(5) || p["chat_msg"] != float64(0) || p["chat_free_msgs"] != float64(0) {
		t.Errorf("default pricing = %+v, want 5/0/0", p)
	}
}

// /legal 免鉴权时按 App 租户取正文,不是小程序的默认租户。
func TestLegalUsesAppTenant(t *testing.T) {
	setCache(map[int64]map[string]string{200: {KeyAppLegalTerms: "APP 条款正文"}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(100, 200).Register(r.Group("/api"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/legal/terms", nil))
	if !strings.Contains(w.Body.String(), "APP 条款正文") {
		t.Errorf("legal should come from the app tenant, got %s", w.Body.String())
	}
}

// 发现页定价随 /app-config 下发;没配过按默认 10 / 关 / 1。
func TestAppConfigCarriesDiscoverPricing(t *testing.T) {
	setCache(map[int64]map[string]string{})
	p := getAppConfig(t, 100, "")["pricing"].(map[string]any)
	if p["rewind"] != float64(10) || p["skip_charge_enabled"] != false || p["skip_price"] != float64(1) {
		t.Errorf("discover pricing = %+v, want 10/false/1", p)
	}
	setCache(map[int64]map[string]string{100: {KeyAppRewindPrice: "7", KeyAppDiscoverSkipChargeEnabled: "1"}})
	p = getAppConfig(t, 100, "")["pricing"].(map[string]any)
	if p["rewind"] != float64(7) || p["skip_charge_enabled"] != true {
		t.Errorf("configured discover pricing = %+v", p)
	}
}

// 没配凭证时,就算开关默认开,渠道也不可用——按钮不该露出来让人点了报错。
func TestAppConfigChannelsOffWithoutCreds(t *testing.T) {
	setCache(map[int64]map[string]string{})
	d := getAppConfig(t, 100, "")
	auth := d["auth"].(map[string]any)
	pay := d["pay"].(map[string]any)
	if auth["wechat"] != false || auth["alipay"] != false || auth["wechat_app_id"] != "" {
		t.Fatalf("auth = %+v", auth)
	}
	if pay["wechat"] != false || pay["alipay"] != false || pay["apple"] != false || pay["google_play"] != false {
		t.Fatalf("pay = %+v", pay)
	}
}

func getAppConfigWith(t *testing.T, tenant int64, lookup CredLookup, payUsable func(int64, string) bool) map[string]any {
	t.Helper()
	return getAppConfigFromHandler(t, NewHandler(tenant, tenant).WithCredLookup(lookup).WithPayUsable(payUsable))
}

// getAppConfigFromHandler 跑一次 /app-config 取 data。所有 helper 共用这一份 gin 样板。
func getAppConfigFromHandler(t *testing.T, h *Handler) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/app-config", nil))
	var body struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Data
}

// 支付与登录现在是两个独立的服务商域:开了支付不等于开了登录。
// 这条守的就是两者不再互相影响。
func TestAppConfigPayFromProviderStore(t *testing.T) {
	setCache(map[int64]map[string]string{})
	h := NewHandler(100, 100).
		WithPayUsable(func(_ int64, p string) bool { return p == "wechat" || p == "apple" }).
		WithSSOUsable(func(_ int64, p string) bool { return p == "alipay" }).
		WithSSOField(func(_ int64, p, k string) string { return "" })
	d := getAppConfigFromHandler(t, h)
	pay := d["pay"].(map[string]any)
	if pay["wechat"] != true || pay["alipay"] != false || pay["apple"] != true || pay["google_play"] != false {
		t.Fatalf("pay = %+v", pay)
	}
	auth := d["auth"].(map[string]any)
	// 支付开了微信却没开登录 → 登录按钮不露
	if auth["wechat"] != false {
		t.Fatalf("支付开了不等于登录开了: %+v", auth)
	}
	// 反过来:登录开了支付没开 → 登录按钮照露
	if auth["alipay"] != true {
		t.Fatalf("登录开了就该露: %+v", auth)
	}
}

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

// authSectionWith 用注入的 SSO 查询跑一次 /app-config,取出 auth 段。
func authSectionWith(t *testing.T, usable func(int64, string) bool,
	field func(int64, string, string) string) map[string]any {
	t.Helper()
	h := NewHandler(1, 1).WithSSOUsable(usable).WithSSOField(field)
	body := getAppConfigFromHandler(t, h) // 既有 helper getAppConfigWith 的同款做法
	auth, _ := body["auth"].(map[string]any)
	if auth == nil {
		t.Fatal("响应里没有 auth 段")
	}
	return auth
}
