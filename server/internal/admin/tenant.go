package admin

import (
	"fmt"
	"strings"

	"driftbottle/internal/model"
)

// 租户表单的纯校验逻辑。
//
// 抽成纯函数是为了能在没有数据库基建的情况下测(仓库里的 _test.go 没有一个碰 DB),
// 也因为「app 租户不该被索要微信凭证」这条规则值得单独立一个断言——
// 它正是现在后台建不出 app 租户的原因。

// normalizeTenantType 归一租户类型。空值与不认识的值一律退回小程序,
// 与 AutoMigrate 给存量行的回填值保持一致。
func normalizeTenantType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case model.TenantTypeApp:
		return model.TenantTypeApp
	default:
		return model.TenantTypeMiniProgram
	}
}

// validateTenantInput 校验新建/编辑租户的必填项。
//
// 小程序租户必须给微信凭证,否则建出来也登录不了;
// app 租户的 appid 可选(填了就建一行 platform=app 的租户映射),不需要 secret。
func validateTenantInput(tenantType, name, appid, secret string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("租户名不能为空")
	}
	if normalizeTenantType(tenantType) == model.TenantTypeApp {
		return nil
	}
	if strings.TrimSpace(appid) == "" || strings.TrimSpace(secret) == "" {
		return fmt.Errorf("小程序租户必须填写 AppID 与 Secret")
	}
	return nil
}

// validateTenantStatus 状态只认 active/disabled。
func validateTenantStatus(status string) error {
	if status != "active" && status != "disabled" {
		return fmt.Errorf("状态仅支持 active/disabled")
	}
	return nil
}

// loginCredentialPlatform 租户的「登录凭证行」落在哪个平台:
// 小程序是 wx(appid+secret 换 openid);App 是 app(appid→租户映射,`user.resolveAppTenant` 查它)。
func loginCredentialPlatform(tenantType string) string {
	if normalizeTenantType(tenantType) == model.TenantTypeApp {
		return "app"
	}
	return "wx"
}

// wantsLoginCredential 填了 appid 就写一行登录凭证,两种类型都是。
func wantsLoginCredential(tenantType, appid string) bool {
	return strings.TrimSpace(appid) != ""
}
