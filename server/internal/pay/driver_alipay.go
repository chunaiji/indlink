package pay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"driftbottle/internal/common/alipay"
	"driftbottle/internal/model"
	"driftbottle/pkg/apilog"
)

// AliCreds 支付宝凭证材料。
type AliCreds struct {
	AppID         string
	PrivateKeyPEM string // 应用私钥(PKCS8/PKCS1,可裸 base64)
	PublicKey     string // 支付宝公钥(不是应用公钥)
	NotifyURL     string
	PID           string // 商户 PID(2088 开头),授权登录要;支付不用
	TradeType     string // "APP" = App 支付(alipay.trade.app.pay);"" = 小程序(alipay.trade.create)
	Sandbox       bool   // 走沙箱网关(后台「支付宝」卡片的开关)
	TenantID      int64  // 仅用于接口日志归属
}

// AlipayDriver 支付宝:App 支付 / 小程序支付两种交易类型共用一份。
type AlipayDriver struct {
	c       AliCreds
	gateway string // 测试可覆盖;空 = 按 appid 选正式/沙箱

	once    sync.Once
	cli     *alipay.Client
	loadErr error
}

func NewAlipayDriver(c AliCreds) *AlipayDriver { return &AlipayDriver{c: c} }

func (d *AlipayDriver) Name() string {
	if d.c.TradeType == "APP" {
		return "alipay_app"
	}
	return "alipay"
}

func (d *AlipayDriver) hasCreds() bool {
	return d.c.AppID != "" && d.c.PrivateKeyPEM != "" && d.c.PublicKey != ""
}

func (d *AlipayDriver) client() (*alipay.Client, error) {
	d.once.Do(func() {
		cli, err := alipay.New(d.c.AppID, d.c.PrivateKeyPEM, d.c.PublicKey)
		if err != nil {
			d.loadErr = err
			return
		}
		if d.c.Sandbox {
			cli.Gateway = alipay.GatewaySandbox
		}
		if d.gateway != "" { // 测试覆盖优先
			cli.Gateway = d.gateway
		}
		d.cli = cli
	})
	return d.cli, d.loadErr
}

// Prepay App 支付:不调网关,服务端签好 orderStr 交给客户端 SDK;小程序:alipay.trade.create 取 trade_no。
func (d *AlipayDriver) Prepay(order *model.PayOrder, payerID string) (map[string]interface{}, error) {
	if !d.hasCreds() {
		return map[string]interface{}{"mock": true, "orderNo": order.OrderNo, "tradeNO": "mock-" + order.OrderNo}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	if d.c.TradeType == "APP" {
		biz := map[string]string{
			"out_trade_no":    order.OrderNo,
			"total_amount":    alipay.FenToYuan(order.PriceMinor),
			"subject":         "金币充值",
			"product_code":    "QUICK_MSECURITY_PAY",
			"timeout_express": "30m",
		}
		params, err := cli.BuildRequest("alipay.trade.app.pay", biz, map[string]string{"notify_url": d.c.NotifyURL})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"order_str": alipay.Encode(params)}, nil
	}
	if payerID == "" {
		return nil, errors.New("支付宝小程序下单缺少 buyer_id")
	}
	biz := map[string]string{
		"out_trade_no": order.OrderNo,
		"total_amount": alipay.FenToYuan(order.PriceMinor),
		"subject":      "金币充值",
		"buyer_id":     payerID,
	}
	node, err := cli.Execute("alipay.trade.create", biz, map[string]string{"notify_url": d.c.NotifyURL})
	apilog.Record(d.c.TenantID, "alipay_trade_create", "out_trade_no="+order.OrderNo, 0, errString(err, node), err == nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		TradeNo string `json:"trade_no"`
	}
	_ = json.Unmarshal(node, &r)
	if r.TradeNo == "" {
		return nil, errors.New("支付宝下单未返回 trade_no")
	}
	return map[string]interface{}{"tradeNO": r.TradeNo}, nil
}

// VerifyCallback 异步通知:表单 → RSA2 验签 → app_id 核对 → 状态 / 金额。
//
// 读 r.Form 而不是自己 ParseForm 后丢弃:handler 为了按 app_id 反查租户已经 ParseForm 过一次,
// Body 已被消费,再 Parse 一次拿到的是空表单。
func (d *AlipayDriver) VerifyCallback(r *http.Request) (*CallbackResult, error) {
	if len(r.Form) == 0 {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}
	params := make(map[string]string, len(r.Form))
	for k := range r.Form {
		params[k] = r.Form.Get(k)
	}
	outTradeNo := params["out_trade_no"]
	if outTradeNo == "" {
		return nil, errors.New("支付宝回调缺少 out_trade_no")
	}
	status := params["trade_status"]
	// 开发态:无凭证时直接信任明文(本地自测)
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: outTradeNo, TxnID: params["trade_no"],
			Paid: status == "TRADE_SUCCESS" || status == "TRADE_FINISHED" || status == ""}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	if err := cli.Verify(params); err != nil {
		return nil, fmt.Errorf("支付宝回调验签失败: %w", err)
	}
	if params["app_id"] != d.c.AppID {
		return nil, fmt.Errorf("支付宝回调 app_id 不匹配: %s", params["app_id"])
	}
	fen, err := alipay.YuanToFen(params["total_amount"])
	if err != nil {
		return nil, fmt.Errorf("支付宝回调金额非法: %w", err)
	}
	return &CallbackResult{
		OrderNo: outTradeNo, TxnID: params["trade_no"],
		Paid:      status == "TRADE_SUCCESS" || status == "TRADE_FINISHED",
		AmountFen: fen,
	}, nil
}

// Query alipay.trade.query。交易不存在 = 用户没付,不算错。
func (d *AlipayDriver) Query(order *model.PayOrder) (*CallbackResult, error) {
	if !d.hasCreds() {
		return &CallbackResult{OrderNo: order.OrderNo}, nil
	}
	cli, err := d.client()
	if err != nil {
		return nil, err
	}
	node, err := cli.Execute("alipay.trade.query", map[string]string{"out_trade_no": order.OrderNo}, nil)
	apilog.Record(d.c.TenantID, "alipay_trade_query", "out_trade_no="+order.OrderNo, 0, errString(err, node), err == nil)
	if err != nil {
		if strings.Contains(err.Error(), "ACQ.TRADE_NOT_EXIST") {
			return &CallbackResult{OrderNo: order.OrderNo}, nil
		}
		return nil, err
	}
	var r struct {
		TradeNo     string `json:"trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	}
	_ = json.Unmarshal(node, &r)
	fen, _ := alipay.YuanToFen(r.TotalAmount)
	return &CallbackResult{
		OrderNo: order.OrderNo, TxnID: r.TradeNo,
		Paid:      r.TradeStatus == "TRADE_SUCCESS" || r.TradeStatus == "TRADE_FINISHED",
		AmountFen: fen,
	}, nil
}

func (d *AlipayDriver) SuccessResponse() (string, []byte) {
	return "text/plain", []byte("success")
}

func errString(err error, node json.RawMessage) string {
	if err != nil {
		return err.Error()
	}
	return string(node)
}
