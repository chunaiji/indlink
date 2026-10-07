package pay

import (
	"net/http"

	"driftbottle/internal/model"
)

// CallbackResult 回调验签后的归一化结果。
type CallbackResult struct {
	OrderNo   string // 商户订单号(我们生成的 order_no)
	TxnID     string // 平台交易号
	Paid      bool   // 是否支付成功
	AmountFen int64  // 平台回传的实付金额(分),用于二次校验
}

// Driver 支付渠道适配接口。微信 / 支付宝各实现一份。
// 真实环境需接入官方 SDK 完成下单与验签;此处保留稳定接口,业务层与渠道解耦。
type Driver interface {
	Name() string
	// Prepay 调平台预下单,返回给前端 uni.requestPayment 使用的支付参数。
	// payerID:微信为 openid;支付宝可为空。
	Prepay(order *model.PayOrder, payerID string) (map[string]interface{}, error)
	// VerifyCallback 解析并验签平台异步回调。
	VerifyCallback(r *http.Request) (*CallbackResult, error)
	// SuccessResponse 返回应答给平台的报文(微信/支付宝要求不同)。
	SuccessResponse() (contentType string, body []byte)
}

// Querier 可选能力:主动向渠道查单。回调丢了时由查单接口补偿入账(order_query.go SyncIfStale)。
// mock 渠道不实现——它没有「渠道那边」可问。
type Querier interface {
	// Query 订单不存在 / 未支付返回 Paid=false 且 err=nil;网络或验签错误才返回 err。
	Query(order *model.PayOrder) (*CallbackResult, error)
}
