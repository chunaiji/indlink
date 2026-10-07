package user

import (
	"encoding/json"
	"testing"
)

// idTokenClaims 必须能吃到 Google 的 email_verified。
// 缺了它，「该邮箱已注册」的冲突判断就只能拿未经验证的邮箱去匹配已有账号——那是账号接管。
func TestIDTokenClaimsEmailVerified(t *testing.T) {
	var c idTokenClaims
	raw := `{"sub":"110","email":"meera@gmail.com","email_verified":true,"name":"Meera"}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("解析 claims 失败: %v", err)
	}
	if c.Email != "meera@gmail.com" {
		t.Fatalf("email 应为 meera@gmail.com, 实际 %q", c.Email)
	}
	if !c.EmailVerified {
		t.Fatal("email_verified=true 时应解析为 true")
	}
}

// 字段缺失时必须是 false，不能是「未知」。Apple 的 token 里就没有这个字段。
func TestIDTokenClaimsEmailVerifiedAbsent(t *testing.T) {
	var c idTokenClaims
	if err := json.Unmarshal([]byte(`{"sub":"110"}`), &c); err != nil {
		t.Fatalf("解析 claims 失败: %v", err)
	}
	if c.EmailVerified {
		t.Fatal("字段缺失时应为 false")
	}
}

// nilIfEmpty 是可空唯一索引列的唯一正确写法：
// 空串在 MySQL 唯一索引里会互相冲突，NULL 不会。
func TestNilIfEmpty(t *testing.T) {
	if nilIfEmpty("") != nil {
		t.Fatal("空串必须转成 nil，否则多行 '' 会撞唯一索引")
	}
	p := nilIfEmpty("abc")
	if p == nil {
		t.Fatal("非空串不应转成 nil")
	}
	if *p != "abc" {
		t.Fatalf("值应为 abc, 实际 %q", *p)
	}
}

// decideOAuthLogin 是「不自动合并」这条产品决策的唯一落点。
// 五种组合必须全部覆盖——漏掉任何一种，要么用户凭空多出第二个账号，
// 要么攻击者能用同名邮箱的 Google 账号接管已有账号。
func TestDecideOAuthLogin(t *testing.T) {
	cases := []struct {
		name          string
		subHit        bool
		emailVerified bool
		emailHit      bool
		want          oauthDecision
	}{
		{"sub 命中即登录，不看邮箱", true, true, true, oauthLogin},
		{"sub 命中，邮箱未占用也照样登录", true, false, false, oauthLogin},
		{"全新用户，建号", false, true, false, oauthCreate},
		{"邮箱已占用但未验证 → 仍建号，未验证的邮箱不参与冲突判断", false, false, true, oauthCreate},
		{"邮箱已验证且已占用 → 冲突，提示去绑定", false, true, true, oauthEmailTaken},
	}
	for _, c := range cases {
		got := decideOAuthLogin(c.subHit, c.emailVerified, c.emailHit)
		if got != c.want {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.want, got)
		}
	}
}

// appIdentity.where() 的分支优先级：sub 必须排在 email 之前。
//
// 这条如果被改错，不会有任何报错——只会让第三方登录悄悄变成「按邮箱查账号」，
// 也就是产品明确否决的自动合并。所以必须有测试钉住。
func TestAppIdentityWherePrefersSub(t *testing.T) {
	cases := []struct {
		name     string
		id       appIdentity
		wantCond string
		wantVal  string
	}{
		{
			"同时有 GoogleSub 和 Email 时按 sub 查，不按 email",
			appIdentity{GoogleSub: "g-123", Email: "meera@gmail.com"},
			"google_sub = ?", "g-123",
		},
		{
			"同时有 AppleSub 和 Email 时按 sub 查",
			appIdentity{AppleSub: "a-456", Email: "meera@gmail.com"},
			"apple_sub = ?", "a-456",
		},
		{"只有 Phone", appIdentity{Phone: "+919876543210"}, "phone = ?", "+919876543210"},
		{"只有 Email", appIdentity{Email: "meera@gmail.com"}, "email = ?", "meera@gmail.com"},
	}
	for _, c := range cases {
		cond, val := c.id.where()
		if cond != c.wantCond || val != c.wantVal {
			t.Errorf("%s: 期望 (%q, %q), 实际 (%q, %q)", c.name, c.wantCond, c.wantVal, cond, val)
		}
	}
}

// 解绑后必须至少还剩一种可登录方式，否则用户把自己锁在门外，
// 而且再也没有任何入口能进来改回去。
func TestRemainingAfterUnbind(t *testing.T) {
	cases := []struct {
		name     string
		m        loginMethods
		provider string
		want     int
	}{
		{"只有 Google，解绑后归零", loginMethods{Google: true}, "google", 0},
		{"手机号 + Google，解绑 Google 还剩手机号", loginMethods{Phone: true, Google: true}, "google", 1},
		{"Google + Apple，解绑 Google 还剩 Apple", loginMethods{Google: true, Apple: true}, "google", 1},
		{"只有 Apple，解绑 Apple 归零", loginMethods{Apple: true}, "apple", 0},
		{"解绑一个本来就没绑的，数量不变", loginMethods{Phone: true, Password: true}, "google", 2},
		{"四种全有，解绑 Apple 还剩三种", loginMethods{Phone: true, Password: true, Google: true, Apple: true}, "apple", 3},
	}
	for _, c := range cases {
		if got := c.m.remainingAfterUnbind(c.provider); got != c.want {
			t.Errorf("%s: 期望 %d, 实际 %d", c.name, c.want, got)
		}
	}
}

func TestRemainingAfterUnbindCountsWechatAlipay(t *testing.T) {
	m := loginMethods{Wechat: true, Alipay: true}
	if got := m.remainingAfterUnbind("wechat"); got != 1 {
		t.Fatalf("解绑微信后应剩 1(支付宝), got %d", got)
	}
	m = loginMethods{Wechat: true}
	if got := m.remainingAfterUnbind("wechat"); got != 0 {
		t.Fatalf("唯一方式不可解绑, got %d", got)
	}
}

func TestSubColumnAndProviderSet(t *testing.T) {
	for p, col := range map[string]string{"google": "google_sub", "apple": "apple_sub", "wechat": "wx_openid", "alipay": "alipay_uid"} {
		if subColumn(p) != col || !isOAuthProvider(p) {
			t.Errorf("%s → %s", p, subColumn(p))
		}
	}
	if isOAuthProvider("facebook") {
		t.Fatal("未知 provider 不该放行")
	}
}

// 第三方标识优先级:任何 sub 都排在 phone / email 之前。
func TestIdentityWherePrefersThirdPartySub(t *testing.T) {
	cond, val := appIdentity{WxOpenID: "o1", Email: "a@b.c"}.where()
	if cond != "wx_openid = ?" || val != "o1" {
		t.Fatalf("%s %s", cond, val)
	}
	cond, val = appIdentity{AlipayUID: "2088", Phone: "+8613800000000"}.where()
	if cond != "alipay_uid = ?" || val != "2088" {
		t.Fatalf("%s %s", cond, val)
	}
}
