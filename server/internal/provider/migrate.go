package provider

import (
	"log"
	"sort"
	"strings"

	"driftbottle/internal/crypto"
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
	oldIAPKey       = "app_iap_key" // 密文,解密后再存进加密 JSON
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
	TenantID                                                          int64
	Platform, AppID, Secret, MchID, APIv3Key, SerialNo, PrivateKeyPEM string
	PlatformKeyPEM, PlatformSerial, AlipayPrivateKeyPEM               string
	AlipayPublicKey, NotifyURL                                        string
}

type planned struct {
	TenantID int64
	Kind     string
	Provider string
	Enabled  bool
	Active   bool
	Fields   map[string]string
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
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
		preferWx := "wx"
		if isApp {
			preferWx = "wx_app"
		}
		for i := range byTenant[tid] {
			c := &byTenant[tid][i]
			if c.MchID == "" || (c.Platform != "wx" && c.Platform != "wx_app") {
				continue
			}
			if wx == nil {
				wx = c
				continue
			}
			if c.Platform == preferWx && wx.Platform != preferWx {
				log.Printf("[provider-migrate] tenant=%d 丢弃 %s 行的商户配置,保留 %s", tid, wx.Platform, preferWx)
				wx = c
			}
		}
		if wx != nil {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "wechat", Enabled: on(tid, oldPayWechatOn, "1"), Fields: map[string]string{
				"app_id": wx.AppID, "mch_id": wx.MchID, "apiv3_key": wx.APIv3Key, "serial_no": wx.SerialNo,
				"private_key": wx.PrivateKeyPEM, "platform_key": wx.PlatformKeyPEM, "platform_serial": wx.PlatformSerial,
				"notify_url": notifyOr(wx.NotifyURL, "wechat"),
			}})
		}
		// —— 支付宝 ——
		var ali *legacyRow
		preferAli := "alipay"
		if isApp {
			preferAli = "alipay_app"
		}
		for i := range byTenant[tid] {
			c := &byTenant[tid][i]
			if c.AlipayPrivateKeyPEM == "" || (c.Platform != "alipay" && c.Platform != "alipay_app") {
				continue
			}
			if ali == nil || (c.Platform == preferAli && ali.Platform != preferAli) {
				ali = c
			}
		}
		if ali != nil {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "alipay", Enabled: on(tid, oldPayAlipayOn, "1"), Fields: map[string]string{
				"app_id": ali.AppID, "private_key": ali.AlipayPrivateKeyPEM, "alipay_public_key": ali.AlipayPublicKey,
				"pid": ali.MchID, "notify_url": notifyOr(ali.NotifyURL, "alipay"), "sandbox": "0",
			}})
		}
		// —— IAP ——
		if cfg(tid, oldIAPIssuer) != "" || cfg(tid, oldIAPKey) != "" || cfg(tid, oldIAPEnabled) == "1" {
			out = append(out, planned{TenantID: tid, Kind: KindPay, Provider: "apple", Enabled: on(tid, oldIAPEnabled, "0"), Fields: map[string]string{
				"bundle_id": cfg(tid, oldAppleBundle), "issuer_id": cfg(tid, oldIAPIssuer), "key_id": cfg(tid, oldIAPKeyID),
				"p8_key": cfg(tid, oldIAPKey), "sandbox": boolStr(on(tid, oldIAPSandbox, "0")),
			}})
		}
		// —— 地图 ——
		// 旧读法是「不等于 google 就是腾讯」(geo.go 的原逻辑)。照搬这条:
		// 只认空串会让 "Tencent" / "qq" / 带空格的值两行都 active=false,地址水印从此静默失效。
		mapsProvider := strings.ToLower(strings.TrimSpace(cfg(tid, oldMapsProvider)))
		if mapsProvider != "google" {
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
				"text_on":  boolStr(cfg(tid, oldSecTextOn) == "1"),
				"image_on": boolStr(cfg(tid, oldSecImageOn) == "1"),
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

// notifyOr 保留租户原有的回调地址;只有空的时候才填产品默认域。
// 白标租户的 notify_url 指向它自己的域名,改写等于把它的订单指到别人的服务器。
func notifyOr(existing, channel string) string {
	if strings.TrimSpace(existing) != "" {
		return strings.TrimSpace(existing)
	}
	return notifyBase + channel
}

func decryptOrKeep(v string) (string, error) {
	if !crypto.Enabled() {
		return v, nil
	}
	return crypto.Decrypt(v)
}
