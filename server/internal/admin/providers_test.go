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
			if f.Value != "" {
				t.Errorf("已有行里没写过的字段原样为空, got %q", f.Value)
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
	d2, _ := provider.Find(provider.KindPay, "alipay")
	c2 := cardFor(d2, nil, i18n.ZhCN)
	for _, f := range c2.Fields {
		if f.Key == "sandbox" && f.Value != "0" {
			t.Fatalf("没行时非机密字段回 schema 默认值, got %q", f.Value)
		}
	}
}
