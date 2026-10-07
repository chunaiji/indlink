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

// 旧读法是「不等于 google 就是腾讯」。迁移必须照搬这条,否则 app_maps_provider 存了
// "Tencent"/"qq"/带空格的值时,两行都 active=false,地址水印从此静默失效。
func TestPlanMigrationMapProviderFallsBackToTencent(t *testing.T) {
	for _, raw := range []string{"", "tencent", "Tencent", "qq", " tencent "} {
		cfg := func(tid int64, key string) string {
			if key == oldMapsProvider {
				return raw
			}
			if key == oldQQKey {
				return "QQ"
			}
			if key == oldGoogleMapKey {
				return "GG"
			}
			return ""
		}
		got := byKeyOf(planMigration(nil, func(int64) string { return "app" }, cfg, []int64{1}))
		tc := got[keyOf(1, KindMap, "tencent")]
		if !tc.Active {
			t.Errorf("app_maps_provider=%q 时腾讯应 active", raw)
		}
		if got[keyOf(1, KindMap, "google")].Active {
			t.Errorf("app_maps_provider=%q 时 google 不该 active", raw)
		}
	}
}

// 多租户 SaaS:白标租户的 notify_url 指向它自己的域名,迁移不能改写成产品默认域。
func TestPlanMigrationKeepsExistingNotifyURL(t *testing.T) {
	creds := []legacyRow{
		{TenantID: 1, Platform: "wx", AppID: "wx1", MchID: "100", APIv3Key: "k", SerialNo: "S",
			PrivateKeyPEM: "P", PlatformKeyPEM: "PK", PlatformSerial: "PS", NotifyURL: "https://white.label/api/pay/callback/wx"},
		{TenantID: 2, Platform: "wx", AppID: "wx2", MchID: "200", APIv3Key: "k", SerialNo: "S",
			PrivateKeyPEM: "P", PlatformKeyPEM: "PK", PlatformSerial: "PS2"},
	}
	cfg := func(int64, string) string { return "" }
	got := byKeyOf(planMigration(creds, func(int64) string { return "miniprogram" }, cfg, []int64{1, 2}))
	if u := got[keyOf(1, KindPay, "wechat")].Fields["notify_url"]; u != "https://white.label/api/pay/callback/wx" {
		t.Fatalf("已有 notify_url 应保留, got %s", u)
	}
	if u := got[keyOf(2, KindPay, "wechat")].Fields["notify_url"]; u != notifyBase+"wechat" {
		t.Fatalf("空 notify_url 才用默认, got %s", u)
	}
}
