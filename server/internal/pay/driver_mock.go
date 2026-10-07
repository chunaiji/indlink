package pay

import (
	"net/http"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
	"driftbottle/internal/sysconfig"
)

// mockDriver 联调用的假渠道。
//
// Prepay 不调任何外部服务,只回一个标记;真正的「支付结果」由客户端显式调
// /pay/mock/settle 触发。让联调的人自己选结果,是因为这套屏里真正值钱的是
// H9 掉单与 H10 的失败行——自动置 paid 只能测出「成功」那一条分支。
//
// ⚠️ 只在 app_pay_mock_enabled=1 时才会被 buildDriver 返回,默认关。
type mockDriver struct{}

func (mockDriver) Name() string { return "mock" }

func (mockDriver) Prepay(order *model.PayOrder, _ string) (map[string]interface{}, error) {
	return map[string]interface{}{"mock": true, "order_no": order.OrderNo}, nil
}

// VerifyCallback mock 不走平台异步回调那条路,结算由客户端显式调端点触发。
func (mockDriver) VerifyCallback(*http.Request) (*CallbackResult, error) {
	return nil, errs.New(errs.CodeBadRequest, "模拟渠道不支持异步回调")
}

func (mockDriver) SuccessResponse() (string, []byte) {
	return "text/plain", []byte("mock")
}

// mockSettleResult 把请求里的 result 解析成「是否支付成功」。
//
// 未知取值一律报错:默默当成成功,会让一个拼错的参数变成凭空发币。
func mockSettleResult(raw string) (bool, error) {
	switch raw {
	case "success":
		return true, nil
	case "fail":
		return false, nil
	default:
		return false, errs.New(errs.CodeBadRequest, "result 只能是 success 或 fail")
	}
}

// mockCallbackResult 用订单本身拼出回调结果。
//
// ⚠️ 金额必须取自订单:HandleCallback 会拿它与订单金额比对(service.go:179),
// 取错来源那道校验就白设了。
func mockCallbackResult(order *model.PayOrder) *CallbackResult {
	return &CallbackResult{
		OrderNo:   order.OrderNo,
		TxnID:     "mock-" + order.OrderNo,
		Paid:      true,
		AmountFen: order.PriceMinor,
	}
}

// SettleMock 联调用:把一笔 mock 订单结算成功或失败。
//
// 成功一路交给现有的 HandleCallback——它是唯一的入账口,幂等、带金额校验、
// 顺带首充打标(service.go:162)。另开一条入账路径等于把幂等重写一遍。
func (s *Service) SettleMock(tenantID, userID int64, orderNo, result string) error {
	if !sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled) {
		// 404 而不是 403:一个关掉的联调后门不该自曝存在。
		return errs.New(errs.CodeNotFound, "接口不存在")
	}
	// 必须先确认订单属于当前用户,理由同 Service.Order。
	order, err := s.Order(userID, orderNo)
	if err != nil {
		return err
	}
	paid, err := mockSettleResult(result)
	if err != nil {
		return err
	}
	if !paid {
		// 只置失败,不动钱包。WHERE status = 'pending' 保证幂等。
		return s.db.Model(&model.PayOrder{}).
			Where("order_no = ? AND status = ?", order.OrderNo, "pending").
			Update("status", "failed").Error
	}
	return s.HandleCallback("mock", mockCallbackResult(order))
}
