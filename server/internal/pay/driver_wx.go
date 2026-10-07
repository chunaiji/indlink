package pay

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"driftbottle/internal/model"
	"driftbottle/pkg/apilog"
)

// WxCreds 微信支付所需凭证材料(密钥为 PEM 文本,来源:单租户 .env 文件 / 多租户 credstore)。
type WxCreds struct {
	AppID          string
	MchID          string
	APIv3Key       string
	SerialNo       string
	PrivateKeyPEM  string // 商户 API 私钥
	PlatformPubPEM string // 平台公钥(回调验签)
	PlatformSerial string
	NotifyURL      string
	TradeType      string // "APP" = App 支付(/v3/pay/transactions/app);"" = 小程序 JSAPI
	TenantID       int64  // 接口日志归属
}

// WxDriver 微信支付(小程序 JSAPI / App 支付,均走 APIv3)。
type WxDriver struct {
	c       WxCreds
	apiBase string // 测试覆盖;空 = wxAPIBase

	once        sync.Once
	loadErr     error
	privKey     *rsa.PrivateKey
	platformPub *rsa.PublicKey
}

func NewWxDriver(c WxCreds) *WxDriver { return &WxDriver{c: c} }

func (d *WxDriver) Name() string {
	if d.c.TradeType == "APP" {
		return "wx_app"
	}
	return "wx"
}

const wxAPIBase = "https://api.mch.weixin.qq.com"

var wxHTTPClient = &http.Client{Timeout: 10 * time.Second}

func (d *WxDriver) base() string {
	if d.apiBase != "" {
		return d.apiBase
	}
	return wxAPIBase
}

func (d *WxDriver) hasCreds() bool {
	return d.c.MchID != "" && d.c.APIv3Key != "" && d.c.SerialNo != "" && d.c.PrivateKeyPEM != "" && d.c.AppID != ""
}

func (d *WxDriver) loadKeys() error {
	d.once.Do(func() {
		priv, err := parsePrivateKeyPEM([]byte(d.c.PrivateKeyPEM))
		if err != nil {
			d.loadErr = fmt.Errorf("解析商户私钥失败: %w", err)
			return
		}
		d.privKey = priv
		if d.c.PlatformPubPEM != "" {
			pub, err := parsePublicKeyPEM([]byte(d.c.PlatformPubPEM))
			if err != nil {
				d.loadErr = fmt.Errorf("解析平台公钥失败: %w", err)
				return
			}
			d.platformPub = pub
		}
	})
	return d.loadErr
}

// ---------------- 下单 ----------------

// Prepay 预下单。JSAPI 返回 uni.requestPayment 参数;APP 返回 SDK 拉起参数。
func (d *WxDriver) Prepay(order *model.PayOrder, payerID string) (map[string]interface{}, error) {
	if !d.hasCreds() {
		return map[string]interface{}{
			"mock": true, "orderNo": order.OrderNo, "timeStamp": "0",
			"nonceStr": order.OrderNo, "package": "prepay_id=mock", "signType": "RSA", "paySign": "mock",
		}, nil
	}
	if err := d.loadKeys(); err != nil {
		return nil, err
	}
	isApp := d.c.TradeType == "APP"
	body := map[string]interface{}{
		"appid": d.c.AppID, "mchid": d.c.MchID, "description": "金币充值",
		"out_trade_no": order.OrderNo, "notify_url": d.c.NotifyURL,
		"amount": map[string]interface{}{"total": order.PriceMinor, "currency": "CNY"},
	}
	path := "/v3/pay/transactions/jsapi"
	if isApp {
		path = "/v3/pay/transactions/app"
	} else {
		if payerID == "" {
			return nil, errors.New("微信下单缺少 payer openid")
		}
		body["payer"] = map[string]string{"openid": payerID}
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	respBody, status, err := d.doSigned(http.MethodPost, path, bodyBytes)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, wxAPIError("微信下单失败", status, respBody)
	}
	var r struct {
		PrepayID string `json:"prepay_id"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	if r.PrepayID == "" {
		return nil, errors.New("微信下单未返回 prepay_id")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randNonce()
	if isApp {
		// APP 调起签名串第四行是**裸 prepay_id**(与 JSAPI 的 "prepay_id=xxx" 不同,官方文档如此)。
		sign, err := signSHA256(d.privKey, d.c.AppID+"\n"+ts+"\n"+nonce+"\n"+r.PrepayID+"\n")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"appid": d.c.AppID, "partnerid": d.c.MchID, "prepayid": r.PrepayID,
			"package": "Sign=WXPay", "noncestr": nonce, "timestamp": ts, "sign": sign,
		}, nil
	}
	pkg := "prepay_id=" + r.PrepayID
	paySign, err := signSHA256(d.privKey, d.c.AppID+"\n"+ts+"\n"+nonce+"\n"+pkg+"\n")
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"timeStamp": ts, "nonceStr": nonce, "package": pkg, "signType": "RSA", "paySign": paySign,
	}, nil
}

// doSigned 带 APIv3 签名的请求。path 含 query(查单用);GET 时 body 为空但签名串仍要留空行。
func (d *WxDriver) doSigned(method, path string, bodyBytes []byte) ([]byte, int, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randNonce()
	signStr := method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + string(bodyBytes) + "\n"
	signature, err := signSHA256(d.privKey, signStr)
	if err != nil {
		return nil, 0, err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		d.c.MchID, nonce, signature, timestamp, d.c.SerialNo)
	req, err := http.NewRequest(method, d.base()+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "driftbottle-wxpay/1.0")
	resp, err := wxHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	apilog.Record(d.c.TenantID, "wxpay_"+strings.ToLower(method), path, resp.StatusCode, string(respBody), resp.StatusCode < 300)
	return respBody, resp.StatusCode, nil
}

func wxAPIError(prefix string, status int, body []byte) error {
	var e struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &e)
	return fmt.Errorf("%s(%d): %s %s", prefix, status, e.Code, e.Message)
}

// Query GET /v3/pay/transactions/out-trade-no/{no}?mchid=。404 ORDERNOTEXIST = 未支付。
func (d *WxDriver) Query(order *model.PayOrder) (*CallbackResult, error) {
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	if err := d.loadKeys(); err != nil {
		return nil, err
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(order.OrderNo) + "?mchid=" + url.QueryEscape(d.c.MchID)
	respBody, status, err := d.doSigned(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	if status != http.StatusOK {
		return nil, wxAPIError("微信查单失败", status, respBody)
	}
	var res wxDecryptedResource
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, err
	}
	return &CallbackResult{
		OrderNo: order.OrderNo, TxnID: res.TransactionID,
		Paid: res.TradeState == "SUCCESS", AmountFen: res.Amount.Total,
	}, nil
}

// ---------------- 回调验签 + 解密 ----------------

type wxCallbackEnvelope struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Resource  struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		Nonce          string `json:"nonce"`
		AssociatedData string `json:"associated_data"`
	} `json:"resource"`
}

type wxDecryptedResource struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Amount        struct {
		Total      int64 `json:"total"`
		PayerTotal int64 `json:"payer_total"`
	} `json:"amount"`
}

func (d *WxDriver) VerifyCallback(r *http.Request) (*CallbackResult, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// 开发态:无凭证时直接解析明文 JSON(本地自测)
	if !d.hasCreds() {
		var b struct {
			OutTradeNo    string `json:"out_trade_no"`
			TransactionID string `json:"transaction_id"`
			TradeState    string `json:"trade_state"`
			Amount        struct {
				Total int64 `json:"total"`
			} `json:"amount"`
		}
		if err := json.Unmarshal(body, &b); err != nil {
			return nil, err
		}
		if b.OutTradeNo == "" {
			return nil, errors.New("回调缺少 out_trade_no")
		}
		return &CallbackResult{OrderNo: b.OutTradeNo, TxnID: b.TransactionID,
			Paid: b.TradeState == "SUCCESS" || b.TradeState == "", AmountFen: b.Amount.Total}, nil
	}

	if err := d.loadKeys(); err != nil {
		return nil, err
	}
	if d.platformPub == nil {
		return nil, errors.New("未配置微信平台公钥,无法验签回调")
	}

	timestamp := r.Header.Get("Wechatpay-Timestamp")
	nonce := r.Header.Get("Wechatpay-Nonce")
	signature := r.Header.Get("Wechatpay-Signature")
	serial := r.Header.Get("Wechatpay-Serial")
	if timestamp == "" || nonce == "" || signature == "" {
		return nil, errors.New("回调缺少验签头")
	}
	if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		if diff := time.Now().Unix() - ts; diff > 300 || diff < -300 {
			return nil, errors.New("回调时间戳超出允许范围")
		}
	}
	if d.c.PlatformSerial != "" && serial != "" && serial != d.c.PlatformSerial {
		return nil, fmt.Errorf("回调平台序列号不匹配: %s", serial)
	}

	verifyStr := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	if err := verifySHA256(d.platformPub, verifyStr, signature); err != nil {
		return nil, fmt.Errorf("回调验签失败: %w", err)
	}

	var env wxCallbackEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	plain, err := decryptAEAD(d.c.APIv3Key, env.Resource.Ciphertext, env.Resource.Nonce, env.Resource.AssociatedData)
	if err != nil {
		return nil, fmt.Errorf("回调资源解密失败: %w", err)
	}
	var res wxDecryptedResource
	if err := json.Unmarshal(plain, &res); err != nil {
		return nil, err
	}
	if res.OutTradeNo == "" {
		return nil, errors.New("解密结果缺少 out_trade_no")
	}
	return &CallbackResult{
		OrderNo: res.OutTradeNo, TxnID: res.TransactionID,
		Paid: res.TradeState == "SUCCESS", AmountFen: res.Amount.Total,
	}, nil
}

func (d *WxDriver) SuccessResponse() (string, []byte) {
	return "application/json", []byte(`{"code":"SUCCESS","message":"成功"}`)
}
