package provider

import "testing"

func TestMergeFieldsKeepsSecretOnEmptyAndDropsUnknown(t *testing.T) {
	d, _ := Find(KindPay, "wechat")
	old := map[string]string{"app_id": "wx1", "apiv3_key": "OLDKEY", "mch_id": "100"}
	got := mergeFields(d, old, map[string]string{
		"app_id":    "wx2",
		"apiv3_key": "",         // 机密写空 = 保持
		"mch_id":    "",         // 非机密写空 = 清空
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

// 脏数据(DBA 改库 / 部分备份恢复)让同领域两行都 active 时,Active() 必须每次返回同一行,
// 否则一半请求走腾讯一半走 Google——同一进程内结果漂移,最难查的那种。
func TestActiveIsDeterministicWhenTwoRowsActive(t *testing.T) {
	s := New(nil)
	rows := []*Resolved{
		{TenantID: 1, Kind: KindMap, Provider: "tencent", Enabled: true, Active: true, Fields: map[string]string{"key": "a"}},
		{TenantID: 1, Kind: KindMap, Provider: "google", Enabled: true, Active: true, Fields: map[string]string{"key": "b"}},
		{TenantID: 1, Kind: KindMap, Provider: "amap", Enabled: true, Active: false, Fields: map[string]string{"key": "c"}},
	}
	s.byKey, s.byLookup = buildIndex(rows)
	first, ok := s.Active(1, KindMap)
	if !ok {
		t.Fatal("应返回一行")
	}
	for i := 0; i < 50; i++ {
		got, ok := s.Active(1, KindMap)
		if !ok || got.Provider != first.Provider {
			t.Fatalf("第 %d 次返回 %v,与首次 %s 不一致", i, got, first.Provider)
		}
	}
}
