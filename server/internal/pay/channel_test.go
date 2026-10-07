package pay

import (
	"testing"
	"time"

	"driftbottle/internal/model"
	"driftbottle/internal/provider"
)

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
		{"app", "wx_app", false, "wechat", false}, // 已发布的国内版 App 还在传旧名
		{"app", "alipay_app", false, "alipay", false},
		{"app", "", false, "", true},
		{"app", "apple", false, "", true},     // IAP 不走 /pay/order
		{"app", "wechat", true, "app", false}, // mock 开着忽略 channel
		{"wx", "", false, "wechat", false},    // 小程序:登录平台即渠道
		{"alipay", "", false, "alipay", false},
		{"wx", "wechat", false, "", true}, // 小程序不许传 channel
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

// 微信回调的租户解析:serial 命中就用那家;命中不到或歧义时**不能猜默认租户**,
// 要把所有候选交给 handler 逐个验签——验签过的那家才是真的。
func TestWxCallbackCandidatesOrder(t *testing.T) {
	rows := []*provider.Resolved{
		{TenantID: 1, Kind: provider.KindPay, Provider: "wechat", Enabled: true, Fields: map[string]string{"mch_id": "100", "platform_serial": "SER1"}},
		{TenantID: 2, Kind: provider.KindPay, Provider: "wechat", Enabled: true, Fields: map[string]string{"mch_id": "200", "platform_serial": "SER1"}},
		{TenantID: 3, Kind: provider.KindPay, Provider: "wechat", Enabled: true, Fields: map[string]string{"mch_id": "300", "platform_serial": "SER9"}},
		{TenantID: 4, Kind: provider.KindMap, Provider: "google", Enabled: true, Fields: map[string]string{"key": "k"}},
	}
	// 唯一命中:只给那一家
	got := wxCallbackTenants(rows, "SER9")
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("唯一 serial 应只给租户 3, got %v", got)
	}
	// 歧义:两家都给,按租户 ID 升序(确定性)
	got = wxCallbackTenants(rows, "SER1")
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("同 serial 两租户应全给且有序, got %v", got)
	}
	// 未知 serial(微信轮换平台公钥后):退回全部微信租户,逐个试
	got = wxCallbackTenants(rows, "UNKNOWN")
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("未知 serial 应给全部微信租户, got %v", got)
	}
	// 没有任何微信配置:空
	if got := wxCallbackTenants(rows[3:], "SER1"); len(got) != 0 {
		t.Fatalf("无微信行应为空, got %v", got)
	}
}
