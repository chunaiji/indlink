package pay

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"driftbottle/internal/model"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	msg := "wxapp\n1700000000\nnonce123\nprepay_id=abc\n"
	sig, err := signSHA256(priv, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(&priv.PublicKey, msg, sig); err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	if err := verifySHA256(&priv.PublicKey, msg+"x", sig); err == nil {
		t.Fatal("篡改消息后验签应失败")
	}
}

func TestDecryptAEADRoundtrip(t *testing.T) {
	key := "01234567890123456789012345678901" // 32 bytes
	plain := `{"out_trade_no":"X"}`
	nonce := "abcdef123456" // 12 bytes
	ad := "transaction"
	ct := sealAEAD(t, key, plain, nonce, ad)
	got, err := decryptAEAD(key, ct, nonce, ad)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != plain {
		t.Fatalf("解密结果不符: %s", got)
	}
}

// TestWxVerifyCallback 走完整的"验签 + 解密"链路。
func TestWxVerifyCallback(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	apiv3 := "01234567890123456789012345678901"

	d := NewWxDriver(WxCreds{
		AppID: "wxapp", MchID: "123", APIv3Key: apiv3,
		SerialNo: "MCHSER", PrivateKeyPEM: "dummy",
	})
	// 绕过文件加载,直接注入密钥
	d.once.Do(func() {})
	d.privKey = priv
	d.platformPub = &priv.PublicKey

	resource := `{"out_trade_no":"ORDER123","transaction_id":"TXN9","trade_state":"SUCCESS","amount":{"total":3000}}`
	nonce := "abcdef123456"
	ad := "transaction"
	ct := sealAEAD(t, apiv3, resource, nonce, ad)

	env := map[string]interface{}{
		"id": "evt", "event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]string{
			"algorithm": "AEAD_AES_256_GCM", "ciphertext": ct, "nonce": nonce, "associated_data": ad,
		},
	}
	body, _ := json.Marshal(env)

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	wnonce := "callbacknonce"
	signStr := ts + "\n" + wnonce + "\n" + string(body) + "\n"
	sig, _ := signSHA256(priv, signStr)

	req := httptest.NewRequest("POST", "/api/pay/callback/wx", bytes.NewReader(body))
	req.Header.Set("Wechatpay-Timestamp", ts)
	req.Header.Set("Wechatpay-Nonce", wnonce)
	req.Header.Set("Wechatpay-Signature", sig)
	req.Header.Set("Wechatpay-Serial", "PLATSER")

	res, err := d.VerifyCallback(req)
	if err != nil {
		t.Fatalf("回调校验应通过: %v", err)
	}
	if res.OrderNo != "ORDER123" || res.TxnID != "TXN9" || !res.Paid || res.AmountFen != 3000 {
		t.Fatalf("回调解析结果异常: %+v", res)
	}

	// 篡改签名应失败
	req2 := httptest.NewRequest("POST", "/api/pay/callback/wx", bytes.NewReader(body))
	req2.Header.Set("Wechatpay-Timestamp", ts)
	req2.Header.Set("Wechatpay-Nonce", wnonce)
	req2.Header.Set("Wechatpay-Signature", "AAAA")
	if _, err := d.VerifyCallback(req2); err == nil {
		t.Fatal("伪造签名应被拒绝")
	}
}

func sealAEAD(t *testing.T, key, plain, nonce, ad string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte(nonce), []byte(plain), []byte(ad))
	return base64.StdEncoding.EncodeToString(ct)
}

func TestWxAppPrepayParams(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if r.Header.Get("Authorization") == "" {
			t.Error("缺 Authorization 头")
		}
		_, _ = w.Write([]byte(`{"prepay_id":"wx123"}`))
	}))
	defer srv.Close()

	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP", NotifyURL: "https://x/cb"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	if d.Name() != "wx_app" {
		t.Fatalf("Name = %s", d.Name())
	}

	p, err := d.Prepay(&model.PayOrder{OrderNo: "N1", PriceMinor: 100}, "")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v3/pay/transactions/app" {
		t.Fatalf("path = %s", gotPath)
	}
	if _, has := gotBody["payer"]; has {
		t.Fatal("APP 下单不该带 payer")
	}
	if p["appid"] != "wxopen" || p["partnerid"] != "1900" || p["prepayid"] != "wx123" || p["package"] != "Sign=WXPay" {
		t.Fatalf("参数不对: %+v", p)
	}
	// APP 的签名串第四行是裸 prepay_id
	msg := "wxopen\n" + p["timestamp"].(string) + "\n" + p["noncestr"].(string) + "\nwx123\n"
	if err := verifySHA256(&priv.PublicKey, msg, p["sign"].(string)); err != nil {
		t.Fatalf("APP 拉起签名应可验: %v", err)
	}
}

func TestWxQueryByOutTradeNo(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	var gotURL, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		gotMethod = r.Method
		_, _ = w.Write([]byte(`{"out_trade_no":"N1","transaction_id":"TX1","trade_state":"SUCCESS","amount":{"total":100,"payer_total":100}}`))
	}))
	defer srv.Close()
	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1", PriceMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotURL != "/v3/pay/transactions/out-trade-no/N1?mchid=1900" {
		t.Fatalf("%s %s", gotMethod, gotURL)
	}
	if !res.Paid || res.TxnID != "TX1" || res.AmountFen != 100 {
		t.Fatalf("got %+v", res)
	}
}

// 微信对不存在的单返回 404 ORDERNOTEXIST:那是「没付」,不是错误。
func TestWxQueryNotExistIsUnpaid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"code":"ORDERNOTEXIST","message":"订单不存在"}`))
	}))
	defer srv.Close()
	d := NewWxDriver(WxCreds{AppID: "wxopen", MchID: "1900", APIv3Key: "k", SerialNo: "S", PrivateKeyPEM: "dummy", TradeType: "APP"})
	d.once.Do(func() {})
	d.privKey = priv
	d.apiBase = srv.URL
	res, err := d.Query(&model.PayOrder{OrderNo: "N1"})
	if err != nil || res.Paid {
		t.Fatalf("got %+v %v", res, err)
	}
}
