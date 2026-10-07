package alipay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testKeys(t *testing.T) (privPEM, pubPEM string, priv *rsa.PrivateKey) {
	t.Helper()
	priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}))
	return
}

func TestSignContentSortsSkipsEmptyAndExcludes(t *testing.T) {
	got := SignContent(map[string]string{
		"b": "2", "a": "1", "empty": "", "sign": "x", "sign_type": "RSA2",
	}, "sign_type")
	if got != "a=1&b=2" {
		t.Fatalf("got %q", got)
	}
	// 请求签名时 sign_type 要参与
	if got := SignContent(map[string]string{"a": "1", "sign_type": "RSA2", "sign": "x"}); got != "a=1&sign_type=RSA2" {
		t.Fatalf("got %q", got)
	}
}

func TestSignVerifyRoundtripAndTamper(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, err := New("2021000000000001", privPEM, pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	p := map[string]string{"out_trade_no": "N1", "total_amount": "0.01", "subject": "金币充值", "sign_type": "RSA2"}
	// 模拟支付宝发通知:它的签名**不含 sign_type**,与请求签名(含 sign_type)是两套规则。
	sig, err := c.SignRaw(SignContent(p, "sign_type"))
	if err != nil {
		t.Fatal(err)
	}
	p["sign"] = sig
	if err := c.Verify(p); err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	p["total_amount"] = "0.02"
	if err := c.Verify(p); err == nil {
		t.Fatal("篡改金额后验签应失败")
	}
}

// 后台贴进来的常常是「裸 base64 没有 PEM 头」(支付宝密钥工具的默认输出)。
func TestNewAcceptsBareBase64Keys(t *testing.T) {
	_, _, priv := testKeys(t)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if _, err := New("2021x", base64.StdEncoding.EncodeToString(pkcs8), base64.StdEncoding.EncodeToString(pkix)); err != nil {
		t.Fatal(err)
	}
	pkcs1 := x509.MarshalPKCS1PrivateKey(priv)
	if _, err := New("2021x", base64.StdEncoding.EncodeToString(pkcs1), ""); err != nil {
		t.Fatalf("PKCS1 也要收: %v", err)
	}
}

func TestEncodeURLEscapesValues(t *testing.T) {
	got := Encode(map[string]string{"biz_content": `{"a":"b c"}`, "app_id": "1"})
	if got != "app_id=1&biz_content="+url.QueryEscape(`{"a":"b c"}`) {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRequestIncludesPublicParamsAndSign(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	p, err := c.BuildRequest("alipay.trade.app.pay", map[string]string{"out_trade_no": "N1"}, map[string]string{"notify_url": "https://x/cb"})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"app_id", "method", "format", "charset", "sign_type", "timestamp", "version", "notify_url", "biz_content", "sign"} {
		if p[k] == "" {
			t.Errorf("缺少 %s", k)
		}
	}
	if p["method"] != "alipay.trade.app.pay" || p["sign_type"] != "RSA2" || p["charset"] != "utf-8" {
		t.Fatalf("公共参数不对: %+v", p)
	}
	if err := c.VerifyRaw(SignContent(p), p["sign"]); err != nil {
		t.Fatalf("请求签名应能用同一把公钥验: %v", err)
	}
	// 不带 biz_content 的方法(oauth.token)
	p2, _ := c.BuildRequest("alipay.system.oauth.token", nil, map[string]string{"grant_type": "authorization_code", "code": "c1"})
	if _, has := p2["biz_content"]; has {
		t.Fatal("bizContent 为 nil 时不该有 biz_content")
	}
}

// 网关响应:验签对象是 <method>_response 节点的**原始 JSON 子串**。
func TestParseResponseVerifiesNodeAndChecksCode(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	node := `{"code":"10000","msg":"Success","trade_no":"T1","trade_status":"TRADE_SUCCESS"}`
	sig, _ := c.SignRaw(node)
	body := `{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`
	got, err := c.ParseResponse("alipay.trade.query", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		TradeNo string `json:"trade_no"`
	}
	_ = json.Unmarshal(got, &r)
	if r.TradeNo != "T1" {
		t.Fatalf("got %s", got)
	}
	// 篡改节点
	bad := strings.Replace(body, "T1", "T2", 1)
	if _, err := c.ParseResponse("alipay.trade.query", []byte(bad)); err == nil {
		t.Fatal("篡改后应验签失败")
	}
	// 业务失败
	node2 := `{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"交易不存在"}`
	sig2, _ := c.SignRaw(node2)
	if _, err := c.ParseResponse("alipay.trade.query", []byte(`{"alipay_trade_query_response":`+node2+`,"sign":"`+sig2+`"}`)); err == nil || !strings.Contains(err.Error(), "ACQ.TRADE_NOT_EXIST") {
		t.Fatalf("应带 sub_code 报错, got %v", err)
	}
	// oauth.token 成功响应没有 code 字段,不能当失败
	node3 := `{"access_token":"a","user_id":"2088x"}`
	sig3, _ := c.SignRaw(node3)
	if _, err := c.ParseResponse("alipay.system.oauth.token", []byte(`{"alipay_system_oauth_token_response":`+node3+`,"sign":"`+sig3+`"}`)); err != nil {
		t.Fatalf("无 code 的成功响应应通过: %v", err)
	}
}

func TestExecutePostsFormToGateway(t *testing.T) {
	privPEM, pubPEM, _ := testKeys(t)
	c, _ := New("2021x", privPEM, pubPEM)
	var gotForm url.Values
	node := `{"code":"10000","user_id":"2088x"}`
	sig, _ := c.SignRaw(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`{"alipay_system_oauth_token_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	c.Gateway = srv.URL
	if _, err := c.Execute("alipay.system.oauth.token", nil, map[string]string{"grant_type": "authorization_code", "code": "c1"}); err != nil {
		t.Fatal(err)
	}
	if gotForm.Get("method") != "alipay.system.oauth.token" || gotForm.Get("code") != "c1" || gotForm.Get("sign") == "" {
		t.Fatalf("表单不对: %v", gotForm)
	}
}

func TestMoneyAndGateway(t *testing.T) {
	cases := map[int64]string{1: "0.01", 100: "1.00", 1200: "12.00", 123456: "1234.56"}
	for fen, want := range cases {
		if got := FenToYuan(fen); got != want {
			t.Errorf("FenToYuan(%d) = %s, want %s", fen, got, want)
		}
	}
	for s, want := range map[string]int64{"0.01": 1, "12.00": 1200, "12": 1200, "1234.56": 123456, "0.1": 10} {
		got, err := YuanToFen(s)
		if err != nil || got != want {
			t.Errorf("YuanToFen(%q) = %d,%v want %d", s, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "1,234.00", "-1"} {
		if _, err := YuanToFen(bad); err == nil {
			t.Errorf("YuanToFen(%q) 应报错", bad)
		}
	}
	if GatewayFor("9021000000000001") != GatewaySandbox || GatewayFor("2021000000000001") != GatewayProd {
		t.Fatal("网关选择错误")
	}
}
