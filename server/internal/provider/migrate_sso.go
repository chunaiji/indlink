package provider

import (
	"log"
	"sort"
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
