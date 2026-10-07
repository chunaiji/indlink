package user

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/provider"
)

func aliClient(t *testing.T) *alipay.Client {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	c, err := alipay.New("2021000000000001",
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// authInfo 的固定字段(支付宝「App 支付宝登录」文档),签名要能用同一把公钥验回来。
func TestAlipayAuthInfoShape(t *testing.T) {
	cli := aliClient(t)
	s, err := alipayAuthInfo(cli, "2088000000000001")
	if err != nil {
		t.Fatal(err)
	}
	p := parseAuthInfoParams(s)
	want := map[string]string{
		"apiname": "com.alipay.account.auth", "app_id": "2021000000000001", "app_name": "mc",
		"auth_type": "AUTHACCOUNT", "biz_type": "openservice", "method": "alipay.open.auth.sdk.code.get",
		"pid": "2088000000000001", "product_id": "APP_FAST_LOGIN", "scope": "kuaijie", "sign_type": "RSA2",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("%s = %q want %q", k, p[k], v)
		}
	}
	if p["target_id"] == "" || p["sign"] == "" {
		t.Fatal("缺 target_id / sign")
	}
	if err := cli.VerifyRaw(alipay.SignContent(p), p["sign"]); err != nil {
		t.Fatalf("authInfo 签名应可验: %v", err)
	}
	if _, err := alipayAuthInfo(cli, ""); err == nil {
		t.Fatal("没配 PID 应报错")
	}
}

func TestAlipayCode2UID(t *testing.T) {
	cli := aliClient(t)
	node := `{"access_token":"t","user_id":"2088111","open_id":"o1"}`
	sig, _ := cli.SignRaw(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("method") != "alipay.system.oauth.token" || r.PostForm.Get("code") != "c1" || r.PostForm.Get("grant_type") != "authorization_code" {
			t.Errorf("表单不对: %v", r.PostForm)
		}
		_, _ = w.Write([]byte(`{"alipay_system_oauth_token_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	cli.Gateway = srv.URL
	r, err := alipayCode2UID(1, cli, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if r.OpenID != "2088111" {
		t.Fatalf("got %+v", r)
	}
}

// PID 只认「服务商 → 登录 → 支付宝登录」卡片。
//
// 迁移已把旧凭证行的 mch_id 搬过来了,再留回落只会制造「在新页面改了不生效」
// —— 旧行还有值且优先,这种 bug 查起来要半天。
func TestAlipayPIDComesOnlyFromTheSSOCard(t *testing.T) {
	row := &provider.Resolved{Fields: map[string]string{"pid": "2088new"}}
	if got := alipayPID(row, "2088old"); got != "2088new" {
		t.Fatalf("got %s", got)
	}
	if got := alipayPID(&provider.Resolved{Fields: map[string]string{}}, "2088old"); got != "" {
		t.Fatalf("卡片里没 pid 就是没配,不再回落旧行, got %s", got)
	}
	if got := alipayPID(nil, "2088old"); got != "" {
		t.Fatalf("没卡片也不回落, got %s", got)
	}
}
