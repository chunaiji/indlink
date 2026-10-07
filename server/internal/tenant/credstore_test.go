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
