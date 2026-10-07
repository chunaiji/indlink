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
