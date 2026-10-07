package admin

import (
	"strings"
	"testing"

	"driftbottle/internal/model"
)

func TestNormalizeTenantType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{model.TenantTypeMiniProgram, model.TenantTypeMiniProgram},
		{model.TenantTypeApp, model.TenantTypeApp},
		{"", model.TenantTypeMiniProgram},       // 不填按小程序,与存量行的回填值一致
		{"  app  ", model.TenantTypeApp},        // 两侧空白
		{"APP", model.TenantTypeApp},            // 大小写
		{"wechat", model.TenantTypeMiniProgram}, // 不认识的值不要落库,退回默认
	}
	for _, tc := range cases {
		if got := normalizeTenantType(tc.in); got != tc.want {
			t.Errorf("normalizeTenantType(%q) = %q, 期望 %q", tc.in, got, tc.want)
		}
	}
}

// 小程序租户必须给微信凭证 —— 没有 AppID 就登录不了,建了也是废租户。
func TestValidateTenantInput_MiniProgramNeedsCredential(t *testing.T) {
	if err := validateTenantInput(model.TenantTypeMiniProgram, "某某小程序", "wx123", "sec"); err != nil {
		t.Errorf("凭证齐全的小程序租户不该被拒: %v", err)
	}
	for _, missing := range []struct{ name, appid, secret string }{
		{"", "wx123", "sec"},
		{"某某小程序", "", "sec"},
		{"某某小程序", "wx123", ""},
	} {
		if err := validateTenantInput(model.TenantTypeMiniProgram, missing.name, missing.appid, missing.secret); err == nil {
			t.Errorf("缺字段的小程序租户该被拒: %+v", missing)
		}
	}
}

// App 租户没有小程序,不该被要求填微信凭证 —— 这正是现在建不出 app 租户的原因。
func TestValidateTenantInput_AppNeedsOnlyName(t *testing.T) {
	if err := validateTenantInput(model.TenantTypeApp, "Drift App", "", ""); err != nil {
		t.Errorf("只填名字的 app 租户应当通过: %v", err)
	}
	if err := validateTenantInput(model.TenantTypeApp, "   ", "", ""); err == nil {
		t.Error("名字为空的 app 租户仍该被拒")
	}
}

// 登录凭证行的平台随租户类型走:小程序 → wx,App → app(platform=app 行就是 App 的 appid→租户映射)。
func TestLoginCredentialPlatform(t *testing.T) {
	if got := loginCredentialPlatform(model.TenantTypeMiniProgram); got != "wx" {
		t.Errorf("小程序应落 wx 行, got %s", got)
	}
	if got := loginCredentialPlatform(model.TenantTypeApp); got != "app" {
		t.Errorf("App 应落 app 行, got %s", got)
	}
}

// 填了 appid 就建行,两种类型都是;没填就不建。
func TestWantsLoginCredential(t *testing.T) {
	if !wantsLoginCredential(model.TenantTypeMiniProgram, "wx123") {
		t.Error("小程序 + appid 应当建凭证行")
	}
	if !wantsLoginCredential(model.TenantTypeApp, "drift_app_cn") {
		t.Error("App + appid 也应当建凭证行(platform=app)")
	}
	if wantsLoginCredential(model.TenantTypeApp, "  ") {
		t.Error("没有 appid 时不该建凭证行")
	}
}

// App 租户的 appid 不需要 secret(platform=app 行只做租户映射)。
func TestValidateTenantInput_AppAppIDWithoutSecret(t *testing.T) {
	if err := validateTenantInput(model.TenantTypeApp, "Drift CN", "drift_app_cn", ""); err != nil {
		t.Errorf("App 租户只填 appid 不填 secret 应当通过: %v", err)
	}
}

func TestValidateTenantInputRejectsUnknownStatus(t *testing.T) {
	if err := validateTenantStatus("active"); err != nil {
		t.Errorf("active 应当通过: %v", err)
	}
	if err := validateTenantStatus("disabled"); err != nil {
		t.Errorf("disabled 应当通过: %v", err)
	}
	if err := validateTenantStatus("deleted"); err == nil {
		t.Error("未知状态应当被拒")
	}
}

// 错误消息要能进消息表(以中文原文为 key),所以不能带运行时拼接。
func TestTenantErrorsAreStaticStrings(t *testing.T) {
	err := validateTenantInput(model.TenantTypeMiniProgram, "n", "", "s")
	if err == nil {
		t.Fatal("期望报错")
	}
	if strings.ContainsAny(err.Error(), "0123456789") {
		t.Errorf("错误消息含运行时数值,消息表命中不了: %q", err.Error())
	}
}
