// server/internal/user/sso_source_test.go
package user

import (
	"testing"

	"driftbottle/internal/provider"
)

// 取值统一走 ssoField:卡片不存在、字段没填、store 为 nil,都返回空串而不是 panic。
// 这三种情况在「租户还没配登录」时每天都会发生。
func TestSSOFieldIsSafeWhenUnconfigured(t *testing.T) {
	var s Service // providers 为 nil
	if got := s.ssoField(7, "wechat", "app_id"); got != "" {
		t.Errorf("没有 store 时应返回空串, got %q", got)
	}
}

// Google 的多值 audience 契约不能因为换了来源而变:第一个仍是 Web client ID。
func TestSplitClientIDsUnchangedBySource(t *testing.T) {
	got := splitClientIDs("web.googleusercontent.com, ios.googleusercontent.com ,")
	if len(got) != 2 || got[0] != "web.googleusercontent.com" {
		t.Fatalf("CSV 解析变了: %v", got)
	}
	if primaryClientID("web.googleusercontent.com,ios.googleusercontent.com") != "web.googleusercontent.com" {
		t.Error("第一个必须是 Web client ID")
	}
}

// 支付宝 PID 改从登录卡片取,不再回落凭证行的 MchID。
func TestAlipayPIDComesFromSSOCard(t *testing.T) {
	row := &provider.Resolved{Fields: map[string]string{"pid": "2088sso"}}
	if got := alipayPID(row, "2088old"); got != "2088sso" {
		t.Fatalf("应取登录卡片的 PID, got %q", got)
	}
	if got := alipayPID(nil, "2088old"); got != "" {
		t.Fatalf("登录卡片没配就是没配,不再回落旧凭证行, got %q", got)
	}
}
