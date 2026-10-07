package pay

import (
	"strings"
	"testing"

	"driftbottle/internal/model"
)

func TestMockSettleResultParsing(t *testing.T) {
	paid, err := mockSettleResult("success")
	if err != nil || !paid {
		t.Fatalf(`"success" 应解析为已支付, got paid=%v err=%v`, paid, err)
	}
	paid, err = mockSettleResult("fail")
	if err != nil || paid {
		t.Fatalf(`"fail" 应解析为未支付, got paid=%v err=%v`, paid, err)
	}
	if _, err := mockSettleResult("paid"); err == nil {
		t.Fatal("未知取值必须报错,不能默默当成成功")
	}
	if _, err := mockSettleResult(""); err == nil {
		t.Fatal("空取值必须报错")
	}
}

// 金额必须取自订单本身。
//
// HandleCallback 会拿 AmountFen 与订单金额比对(service.go:179),
// 这里若取错来源,那道校验就形同虚设——而它是防「改价下单」的唯一一道闸。
func TestMockCallbackResultTakesAmountFromOrder(t *testing.T) {
	order := &model.PayOrder{OrderNo: "NO9", PriceMinor: 19900, Coins: 300}

	res := mockCallbackResult(order)

	if res.AmountFen != order.PriceMinor {
		t.Fatalf("回调金额必须取自订单: want %d, got %d", order.PriceMinor, res.AmountFen)
	}
	if res.OrderNo != "NO9" {
		t.Fatalf("单号不符: %s", res.OrderNo)
	}
	if !res.Paid {
		t.Fatal("成功结算的回调 Paid 必须为 true")
	}
	// 前缀是为了事后能把联调产生的假订单从真账里摘出来。
	if !strings.HasPrefix(res.TxnID, "mock-") {
		t.Fatalf("交易号应带 mock- 前缀,便于事后对账剔除: %s", res.TxnID)
	}
}
