package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/model"
)

func aliTestCreds(t *testing.T) (AliCreds, *alipay.Client) {
	t.Helper()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	pkix, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}))
	// 测试里「支付宝公钥」就用同一对的公钥:我们既当商户签,又当支付宝验。
	c := AliCreds{AppID: "2021000000000001", PrivateKeyPEM: privPEM, PublicKey: pubPEM, NotifyURL: "https://x/api/pay/callback/alipay_app", PID: "2088000000000001", TradeType: "APP"}
	cli, err := alipay.New(c.AppID, privPEM, pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	return c, cli
}

// notifySign 模拟支付宝给通知签名:剔 sign / sign_type。
func notifySign(cli *alipay.Client, form map[string]string) string {
	s, _ := cli.SignRaw(alipay.SignContent(form, "sign_type"))
	return s
}

func formReq(f map[string]string) *http.Request {
	v := url.Values{}
	for k, val := range f {
		v.Set(k, val)
	}
	r := httptest.NewRequest(http.MethodPost, "/cb", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestAlipayAppPrepayBuildsSignedOrderStr(t *testing.T) {
	c, cli := aliTestCreds(t)
	d := NewAlipayDriver(c)
	if d.Name() != "alipay_app" {
		t.Fatalf("Name = %s", d.Name())
	}
	params, err := d.Prepay(&model.PayOrder{OrderNo: "N1", PriceMinor: 1200}, "")
	if err != nil {
		t.Fatal(err)
	}
	orderStr, _ := params["order_str"].(string)
	if orderStr == "" {
		t.Fatalf("缺少 order_str: %+v", params)
	}
	vals, err := url.ParseQuery(orderStr)
	if err != nil {
		t.Fatal(err)
	}
	flat := map[string]string{}
	for k := range vals {
		flat[k] = vals.Get(k)
	}
	if flat["method"] != "alipay.trade.app.pay" || flat["notify_url"] != c.NotifyURL || flat["app_id"] != c.AppID {
		t.Fatalf("公共参数不对: %v", flat)
	}
	biz := flat["biz_content"]
	for _, want := range []string{`"out_trade_no":"N1"`, `"total_amount":"12.00"`, `"product_code":"QUICK_MSECURITY_PAY"`, `"timeout_express":"30m"`} {
		if !strings.Contains(biz, want) {
			t.Errorf("biz_content 缺 %s: %s", want, biz)
		}
	}
	// 还原后的参数要能用支付宝公钥(这里是同一对)验过请求签名
	if err := cli.VerifyRaw(alipay.SignContent(flat), flat["sign"]); err != nil {
		t.Fatalf("orderStr 签名应可验: %v", err)
	}
}

func TestAlipayVerifyCallback(t *testing.T) {
	c, cli := aliTestCreds(t)
	d := NewAlipayDriver(c)
	form := map[string]string{
		"app_id": c.AppID, "out_trade_no": "N1", "trade_no": "T1", "trade_status": "TRADE_SUCCESS",
		"total_amount": "12.00", "sign_type": "RSA2", "charset": "utf-8",
	}
	form["sign"] = notifySign(cli, form)
	res, err := d.VerifyCallback(formReq(form))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Paid || res.OrderNo != "N1" || res.TxnID != "T1" || res.AmountFen != 1200 {
		t.Fatalf("结果不对: %+v", res)
	}
	// 篡改金额
	bad := map[string]string{}
	for k, v := range form {
		bad[k] = v
	}
	bad["total_amount"] = "0.01"
	if _, err := d.VerifyCallback(formReq(bad)); err == nil {
		t.Fatal("篡改后应失败")
	}
	// 别人的 app_id(签名对但不是我的应用)
	other := map[string]string{}
	for k, v := range form {
		other[k] = v
	}
	other["app_id"] = "2021999999999999"
	delete(other, "sign")
	other["sign"] = notifySign(cli, other)
	if _, err := d.VerifyCallback(formReq(other)); err == nil {
		t.Fatal("app_id 不匹配应失败")
	}
	// 关单通知:验签通过但 Paid=false
	closed := map[string]string{}
	for k, v := range form {
		closed[k] = v
	}
	closed["trade_status"] = "TRADE_CLOSED"
	delete(closed, "sign")
	closed["sign"] = notifySign(cli, closed)
	res, err = d.VerifyCallback(formReq(closed))
	if err != nil || res.Paid {
		t.Fatalf("关单应 Paid=false: %+v %v", res, err)
	}
}

func TestAlipayQuery(t *testing.T) {
	c, cli := aliTestCreds(t)
	node := `{"code":"10000","trade_no":"T9","trade_status":"TRADE_SUCCESS","total_amount":"12.00","out_trade_no":"N1"}`
	sig, _ := cli.SignRaw(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	d := NewAlipayDriver(c)
	d.gateway = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1", PriceMinor: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Paid || res.TxnID != "T9" || res.AmountFen != 1200 {
		t.Fatalf("got %+v", res)
	}
}

// 查不到订单(ACQ.TRADE_NOT_EXIST)不是错误:用户根本没付,返回未支付。
func TestAlipayQueryNotExistIsUnpaid(t *testing.T) {
	c, cli := aliTestCreds(t)
	node := `{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"交易不存在"}`
	sig, _ := cli.SignRaw(node)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"alipay_trade_query_response":` + node + `,"sign":"` + sig + `"}`))
	}))
	defer srv.Close()
	d := NewAlipayDriver(c)
	d.gateway = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1"})
	if err != nil || res.Paid {
		t.Fatalf("got %+v %v", res, err)
	}
}
