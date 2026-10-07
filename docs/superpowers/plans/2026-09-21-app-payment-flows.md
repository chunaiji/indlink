# App 支付全链路（§11⁺ 六屏）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把原型 §11⁺ 的 H7a / H7b / H8 / H9 / H10 / H11 六屏落地，让 App 的一笔充值从下单、跳渠道、查状态到成功、掉单、退款全程对用户可见。

**Architecture:** 客户端抽一层 `PayChannelAdapter`，本轮只实现 mock 适配器；支付状态机统一轮询服务端订单状态，四个阶段共用一条路由。服务端两处加法：单笔查单接口，以及一个默认关闭的 mock 渠道（结算复用现有且幂等的 `HandleCallback`，不新开入账路径）。

**Tech Stack:** Go 1.22 + Gin + GORM v2（服务端）· Flutter + Riverpod + go_router（客户端）· 不新增任何依赖

**Spec:** `docs/superpowers/specs/2026-09-21-app-payment-flows-design.md`

## Global Constraints

- **不新增任何依赖**：服务端不引 sqlite / sqlmock，客户端不加 pub 包。`pubspec.yaml` 的 `flutter_secure_storage ^9.2.4` 与 `path_provider_foundation 2.4.1` override **不许动**（升级会让 `flutter test` / `build` 在任何平台直接失败）。
- **服务端测试一律纯函数**：仓库 27 个 `_test.go` 里 `gorm.Open` 出现 0 次，没有数据库测试基建。要守的约束挤进纯函数里守。
- **sysconfig 新 key 必须同步写 `internal/sysconfig/sysconfig.go` 的 `defaults`**（空串会导致开关逻辑反转），并登记 `internal/admin/meta.go` 白名单：**中英文标签各一条**（`LabelZh` / `LabelEn`）+ 分组用 `Group<Xxx>` 常量。漏填英文标签 `meta_test.go` 会点名报错。本计划新增 1 个 key。
- **`PayOrder` 的 JSON 名 `price_fen` 不许改**（`model.go:318`）：线上小程序订单页直接读 `o.price_fen`，改了立刻显示 `¥NaN`，而小程序发版要过微信审核。
- **ID 一律字符串下发**（json tag 加 `,string`）。`order_no` 本身是字符串，不受影响。
- **多租户**：所有查询按租户 / 用户过滤。
- **l10n 两份 arb 的 key 必须齐**：`lib/l10n/app_zh.arb` 与 `app_en.arb`。缺 key 时 `gen-l10n` 静默回落英文，会在中文界面里冒出一句英文。
- **提交前验证**：
  - `cd server && go build ./... && go vet ./...`
  - `cd server && go test $(go list ./... | grep -v /internal/robot)`（`/internal/robot` 有既有 flaky 测试，与本改动无关）
  - `cd app/bottles && flutter analyze && flutter test`
- 回复用中文；代码 / 标识符 / commit message 英文（conventional commits）。

---

## File Structure

| 文件 | 职责 | 动作 |
|---|---|---|
| `server/internal/pay/order_query.go` | 单笔查单：`orderScope` 纯函数 + `Service.Order` | 创建 |
| `server/internal/pay/order_query_test.go` | `orderScope` 必须带 `user_id` 的守卫测试 | 创建 |
| `server/internal/pay/driver_mock.go` | mock 渠道 Driver + 结算的纯函数 | 创建 |
| `server/internal/pay/driver_mock_test.go` | 金额来源、TxnID 前缀、result 解析 | 创建 |
| `server/internal/pay/handler.go` | 两条新路由 + 两个 handler | 修改 |
| `server/internal/pay/service.go` | `buildDriver` 加 mock 分支、`driverFor` 对 app 跳过缓存、`SettleMock` | 修改 |
| `server/internal/sysconfig/sysconfig.go` | `KeyAppPayMockEnabled` + defaults | 修改 |
| `server/internal/admin/meta.go` | 白名单一行（中英标签） | 修改 |
| `app/bottles/lib/domain/models/wallet.dart` | `TxnScene.refund`、`PendingOrder`、`OrderStatus` | 修改 |
| `app/bottles/lib/data/repositories.dart` | `WalletRepository` 三个方法签名 | 修改 |
| `app/bottles/lib/data/remote/remote_repositories.dart` | 接住 `order_no`，删掉传错参数的 IAP 调用 | 修改 |
| `app/bottles/lib/data/mock/mock_backend.dart` | 订单状态机假数据 | 修改 |
| `app/bottles/lib/data/mock/mock_repositories.dart` | 跟随接口改动 | 修改 |
| `app/bottles/lib/core/pay/pay_channel.dart` | `PayChannelAdapter` + `MockChannelAdapter` | 创建 |
| `app/bottles/lib/features/me/payment_controller.dart` | 支付状态机 + 轮询 | 创建 |
| `app/bottles/lib/features/me/payment_flow_page.dart` | H7a / H7b / H8 / H9 四态一页 | 创建 |
| `app/bottles/lib/features/me/pay_records_page.dart` | H10 充值记录 | 创建 |
| `app/bottles/lib/features/me/voided_page.dart` | H11 退款已撤销 | 创建 |
| `app/bottles/lib/features/me/recharge_page.dart` | 下单后跳支付页 | 修改 |
| `app/bottles/lib/features/me/wallet_page.dart` | 负余额横幅 | 修改 |
| `app/bottles/lib/app/routes.dart` / `router.dart` | 三条新路由 | 修改 |
| `app/bottles/lib/l10n/app_zh.arb` / `app_en.arb` | 六屏文案 | 修改 |
| `app/bottles/test/payment_controller_test.dart` | 状态机纯逻辑 | 创建 |
| `app/bottles/test/payment_flow_test.dart` | 六屏 widget test | 创建 |

`order_query.go` 与 `driver_mock.go` 分开：前者是「查」，后者是「联调用的假渠道」。放一个文件里，以后删 mock 时会连查单一起带走。

---

## Task 1: 服务端单笔查单接口

**Files:**
- Create: `server/internal/pay/order_query.go`
- Create: `server/internal/pay/order_query_test.go`
- Modify: `server/internal/pay/handler.go:21-32`（`Register`）

**Interfaces:**
- Consumes: `model.PayOrder`（`server/internal/model/model.go:305`）、`errs.CodeOrderNotFound = 5002`
- Produces:
  - `func orderScope(userID int64, orderNo string) (string, []interface{})`
  - `func (s *Service) Order(userID int64, orderNo string) (*model.PayOrder, error)`
  - 路由 `GET /api/pay/order/:orderNo`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/pay/order_query_test.go`：

```go
package pay

import (
	"strings"
	"testing"
)

// orderScope 是为了能在没有数据库的情况下守住一条约束:user_id 必须在 WHERE 里。
//
// 这个测试看着很轻,但它守的正是「有人图省事把 user_id 从条件里删掉」——
// 那一改动会把查单接口变成一个能遍历他人订单的接口,而且不会有任何报错。
func TestOrderScopeAlwaysFiltersByUser(t *testing.T) {
	where, args := orderScope(42, "NO123")

	if !strings.Contains(where, "user_id") {
		t.Fatalf("查单条件必须含 user_id,否则能查到别人的订单: %q", where)
	}
	if !strings.Contains(where, "order_no") {
		t.Fatalf("查单条件必须含 order_no: %q", where)
	}
	if len(args) != 2 {
		t.Fatalf("应有两个绑定参数(order_no, user_id),实得 %d 个", len(args))
	}
	if args[0] != "NO123" || args[1] != int64(42) {
		t.Fatalf("绑定参数顺序或内容不符: %#v", args)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/pay/ -run TestOrderScope -v`
Expected: 编译失败，`undefined: orderScope`

- [ ] **Step 3: 实现 `orderScope` 与 `Service.Order`**

创建 `server/internal/pay/order_query.go`：

```go
package pay

import (
	"errors"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"

	"gorm.io/gorm"
)

// orderScope 单笔查单的查询条件。
//
// 抽成纯函数是为了能在没有数据库基建的情况下断言 user_id 一定在 WHERE 里
// (仓库里没有任何 DB 测试,见计划的 Global Constraints)。
func orderScope(userID int64, orderNo string) (string, []interface{}) {
	return "order_no = ? AND user_id = ?", []interface{}{orderNo, userID}
}

// Order 按单号查单笔订单。
//
// ⚠️ user_id 必须进 WHERE:只按 order_no 查,这就成了一个能遍历他人订单的接口。
// 订单号能不能被猜到,不该成为安全前提。
//
// 「不存在」与「不属于你」返回同一个错误:区分开来等于提供一个订单号存在性探测接口。
func (s *Service) Order(userID int64, orderNo string) (*model.PayOrder, error) {
	where, args := orderScope(userID, orderNo)
	var order model.PayOrder
	if err := s.db.Where(where, args...).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeOrderNotFound, "订单不存在")
		}
		return nil, err
	}
	return &order, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/pay/ -run TestOrderScope -v`
Expected: PASS

- [ ] **Step 5: 挂路由**

在 `server/internal/pay/handler.go` 的 `Register` 里，`api.GET("/pay/orders", auth, h.orders)` 之后加一行：

```go
	// 单笔查单:H7a 轮询、H9 主动查单、H10 点击处理中的订单都用它
	api.GET("/pay/order/:orderNo", auth, h.order)
```

在同文件 `orders` 函数之后加 handler：

```go
// order 单笔查单。返回订单当前状态,客户端据此判断支付是否到账。
func (h *Handler) order(c *gin.Context) {
	o, err := h.svc.Order(middleware.UserID(c), c.Param("orderNo"))
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "查询失败")
		return
	}
	response.OK(c, o)
}
```

> 直接下发 `model.PayOrder`:它的 JSON tag 已经是对外形状,`price_fen` 这个名字
> **不许改**(`model.go:318`,线上小程序订单页直接读它)。

- [ ] **Step 6: 验证**

Run: `cd server && go build ./... && go vet ./... && go test ./internal/pay/ -v`
Expected: 全部 PASS

- [ ] **Step 7: 提交**

```bash
git add server/internal/pay/order_query.go server/internal/pay/order_query_test.go server/internal/pay/handler.go
git commit -m "feat(pay): query a single order by number, scoped to its owner"
```

---

## Task 2: 服务端 mock 渠道

**Files:**
- Create: `server/internal/pay/driver_mock.go`
- Create: `server/internal/pay/driver_mock_test.go`
- Modify: `server/internal/pay/service.go:55-93`（`driverFor` / `buildDriver`）
- Modify: `server/internal/pay/handler.go`（一条新路由 + handler）
- Modify: `server/internal/sysconfig/sysconfig.go`（key + defaults）
- Modify: `server/internal/admin/meta.go`（白名单一行）

**Interfaces:**
- Consumes: Task 1 的 `Service.Order`；现有 `Service.HandleCallback(platform string, res *CallbackResult) error`（`service.go:162`）；`Driver` 接口（`driver.go`）
- Produces:
  - `sysconfig.KeyAppPayMockEnabled = "app_pay_mock_enabled"`
  - `func mockSettleResult(raw string) (bool, error)`
  - `func mockCallbackResult(order *model.PayOrder) *CallbackResult`
  - `func (s *Service) SettleMock(tenantID, userID int64, orderNo, result string) error`
  - 路由 `POST /api/pay/mock/settle`

- [ ] **Step 1: 写失败的测试**

创建 `server/internal/pay/driver_mock_test.go`：

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/pay/ -run TestMock -v`
Expected: 编译失败，`undefined: mockSettleResult` / `undefined: mockCallbackResult`

- [ ] **Step 3: 加 sysconfig key**

`server/internal/sysconfig/sysconfig.go`，在 `KeyAppIAPKeyEnc` 那组常量之后加：

```go
	// App 模拟支付渠道(仅联调)
	KeyAppPayMockEnabled = "app_pay_mock_enabled" // 模拟支付渠道 (0/1),上线前必须关
```

在 `defaults` 里 `KeyAppIAPKeyEnc: ""` 之后加：

```go
	KeyAppPayMockEnabled: "0", // 联调时后台打开;上线前务必关掉,否则任何人都能凭空发币
```

- [ ] **Step 4: 登记后台白名单**

`server/internal/admin/meta.go`，在 `KeyAppIAPKeyEnc` 那一行之后加：

```go
	{Key: sysconfig.KeyAppPayMockEnabled, LabelZh: "模拟支付渠道(仅联调,上线前关闭)", LabelEn: "Mock payment channel (testing only, disable before launch)", Group: GroupAppPay, Type: "bool"},
```

- [ ] **Step 5: 实现 mock driver 与两个纯函数**

创建 `server/internal/pay/driver_mock.go`：

```go
package pay

import (
	"net/http"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/model"
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
```

- [ ] **Step 6: 跑测试确认通过**

Run: `cd server && go test ./internal/pay/ -run TestMock -v`
Expected: PASS

- [ ] **Step 7: 接进 `driverFor` / `buildDriver`**

`server/internal/pay/service.go`。`driverFor` 开头加跳过缓存的分支：

```go
func (s *Service) driverFor(tenantID int64, platform string) (Driver, error) {
	// App 平台不走缓存:mock 开关是租户级且可热更,一旦把 mockDriver 缓存下来,
	// 后台关掉开关后 createOrder 仍会走 mock——一个「已关闭」的假渠道继续发币。
	// mockDriver 无状态、构造成本为零,每次现取。
	if platform == "app" {
		return s.buildDriver(tenantID, platform)
	}

	key := fmt.Sprintf("%d:%s", tenantID, platform)
	// ...以下维持原样
```

`buildDriver` 开头加 mock 分支（在 `if s.cfg.MultiTenant {` **之前**）：

```go
func (s *Service) buildDriver(tenantID int64, platform string) (Driver, error) {
	// 联调用的假渠道。默认关;关着时 App 平台仍然走原逻辑,即「不支持的支付平台」。
	if platform == "app" && sysconfig.GetBool(tenantID, sysconfig.KeyAppPayMockEnabled) {
		return mockDriver{}, nil
	}

	if s.cfg.MultiTenant {
	// ...以下维持原样
```

记得给 `service.go` 补 import `"driftbottle/internal/sysconfig"`。

- [ ] **Step 8: 实现 `SettleMock`**

在 `server/internal/pay/driver_mock.go` 末尾加：

```go
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
```

import 补 `"driftbottle/internal/sysconfig"`。

- [ ] **Step 9: 挂结算路由**

`server/internal/pay/handler.go` 的 `Register` 里，紧跟 Task 1 那行之后：

```go
	// 联调用的模拟结算。路由常驻,开关在 handler 内部按租户查
	// (sysconfig 是租户级且可热更,启动时判断会读成租户 0 的值且永不更新)。
	api.POST("/pay/mock/settle", auth, h.mockSettle)
```

handler：

```go
type mockSettleReq struct {
	OrderNo string `json:"order_no" binding:"required"`
	Result  string `json:"result" binding:"required"` // success / fail
}

// mockSettle 联调用:模拟渠道回调。开关关闭时当作接口不存在。
func (h *Handler) mockSettle(c *gin.Context) {
	var req mockSettleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	err := h.svc.SettleMock(middleware.TenantID(c), middleware.UserID(c), req.OrderNo, req.Result)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "结算失败")
		return
	}
	response.OK(c, gin.H{"order_no": req.OrderNo, "result": req.Result})
}
```

- [ ] **Step 10: 验证**

Run: `cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)`
Expected: 全部 PASS（`admin` 包的 `meta_test.go` 会校验新 key 的中英标签齐全）

- [ ] **Step 11: 提交**

```bash
git add server/internal/pay/driver_mock.go server/internal/pay/driver_mock_test.go server/internal/pay/service.go server/internal/pay/handler.go server/internal/sysconfig/sysconfig.go server/internal/admin/meta.go
git commit -m "feat(pay): mock payment channel behind a default-off switch"
```

---

## Task 3: 客户端数据层 + 三个既有 bug

**Files:**
- Modify: `app/bottles/lib/domain/models/wallet.dart`
- Modify: `app/bottles/lib/data/repositories.dart:299-310`
- Modify: `app/bottles/lib/data/remote/remote_repositories.dart:653-660`
- Modify: `app/bottles/lib/data/mock/mock_repositories.dart:381-426`
- Modify: `app/bottles/lib/data/mock/mock_backend.dart:1012`
- Create: `app/bottles/test/wallet_model_test.dart`

**Interfaces:**
- Consumes: Task 1/2 的 `GET /api/pay/order/:orderNo`、`POST /api/pay/mock/settle`
- Produces:
  - `class PendingOrder { final String orderNo; final Map<String, dynamic> payParams; bool get isMock; }`
  - `enum OrderStatus { pending, paid, failed, refunded }` + `OrderStatus.parse(Object?)` + `bool get isTerminal`
  - `class PayOrderInfo { orderNo, status, coins, priceMinor, currency, platform, createdAt, paidAt }`
  - `TxnScene.refund`
  - `WalletRepository.recharge(pkg, channel) -> Future<PendingOrder>`
  - `WalletRepository.orderStatus(String orderNo) -> Future<PayOrderInfo>`
  - `WalletRepository.settleMock(String orderNo, bool success) -> Future<void>`
  - `WalletRepository.orders() -> Future<List<PayOrderInfo>>`

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/wallet_model_test.dart`：

```dart
import 'package:bottles/domain/models/wallet.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // 服务端有 SceneRefund = "refund"(wallet/service.go:29),客户端此前没有这一支,
  // 退款流水会掉进兜底的 reward——H11 里会把「退款 −300」显示成「奖励」。
  test('refund txns are not mistaken for rewards', () {
    expect(TxnScene.parse('refund'), TxnScene.refund);
    expect(TxnScene.refund.wire, 'refund');
  });

  test('order status parses the four server states', () {
    expect(OrderStatus.parse('pending'), OrderStatus.pending);
    expect(OrderStatus.parse('paid'), OrderStatus.paid);
    expect(OrderStatus.parse('failed'), OrderStatus.failed);
    expect(OrderStatus.parse('refunded'), OrderStatus.refunded);
    // 未知状态当作 pending:继续轮询好过谎报成功。
    expect(OrderStatus.parse('weird'), OrderStatus.pending);
    expect(OrderStatus.parse(null), OrderStatus.pending);
  });

  test('only paid/failed/refunded end the polling loop', () {
    expect(OrderStatus.pending.isTerminal, isFalse);
    expect(OrderStatus.paid.isTerminal, isTrue);
    expect(OrderStatus.failed.isTerminal, isTrue);
    expect(OrderStatus.refunded.isTerminal, isTrue);
  });

  test('a pending order knows whether the mock channel produced it', () {
    expect(const PendingOrder(orderNo: 'A', payParams: {'mock': true}).isMock, isTrue);
    expect(const PendingOrder(orderNo: 'A', payParams: {}).isMock, isFalse);
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/wallet_model_test.dart`
Expected: 编译失败，`OrderStatus` / `PendingOrder` / `TxnScene.refund` 未定义

- [ ] **Step 3: 补模型**

`app/bottles/lib/domain/models/wallet.dart`。`TxnScene` 枚举里 `adReward` 之后加一支：

```dart
  /// 充值被渠道退款后扣回。服务端 `wallet.SceneRefund`。
  refund,
```

`parse` 的 switch 里补一行（放在 `'recharge'` 之后）：

```dart
        'refund' => TxnScene.refund,
```

文件末尾加三个类型：

```dart
/// 下单结果。`payParams` 原样透传给渠道适配器,服务端给哪个渠道就由谁认。
class PendingOrder {
  const PendingOrder({required this.orderNo, required this.payParams});

  final String orderNo;
  final Map<String, dynamic> payParams;

  /// 服务端的 mock 渠道会在 pay_params 里打这个标记(pay/driver_mock.go)。
  bool get isMock => payParams['mock'] == true;
}

/// 服务端订单状态。**它是支付成功与否的唯一判据**——渠道说什么都不算。
enum OrderStatus {
  pending,
  paid,
  failed,
  refunded;

  /// 未知取值一律当 pending:继续轮询好过谎报成功。
  static OrderStatus parse(Object? v) => switch (v) {
        'paid' => OrderStatus.paid,
        'failed' => OrderStatus.failed,
        'refunded' => OrderStatus.refunded,
        _ => OrderStatus.pending,
      };

  /// 终态才停轮询。
  bool get isTerminal => this != OrderStatus.pending;
}

class PayOrderInfo {
  const PayOrderInfo({
    required this.orderNo,
    required this.status,
    required this.coins,
    required this.priceMinor,
    required this.currency,
    required this.platform,
    required this.createdAt,
    this.paidAt,
  });

  final String orderNo;
  final OrderStatus status;
  final int coins;

  /// 最小货币单位(paise / cent)。服务端 JSON 名是 `price_fen`——
  /// 那个名字不许改,线上小程序订单页直接读它(model.go:318)。
  final int priceMinor;
  final String currency;
  final String platform;
  final DateTime createdAt;
  final DateTime? paidAt;

  factory PayOrderInfo.fromJson(Map<String, dynamic> j) => PayOrderInfo(
        orderNo: (j['order_no'] ?? '').toString(),
        status: OrderStatus.parse(j['status']),
        coins: intOf(j['coins']),
        priceMinor: intOf(j['price_fen']),
        currency: (j['currency'] ?? '').toString(),
        platform: (j['platform'] ?? '').toString(),
        createdAt: utcOrEpoch(j['created_at']),
        paidAt: j['paid_at'] == null ? null : utcOrEpoch(j['paid_at']),
      );
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/wallet_model_test.dart`
Expected: PASS

- [ ] **Step 5: 改 `WalletRepository` 接口**

`app/bottles/lib/data/repositories.dart`，把 `recharge` 那一条替换为四条：

```dart
  /// `POST /api/pay/order` —— **只下单,不代表已支付**。
  ///
  /// 返回 order_no 之后由支付状态机接管:拉渠道 → 轮询 [orderStatus]。
  /// 旧实现在这里直接 `return wallet()`,等于下完单就当作已到账。
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel);

  /// `GET /api/pay/order/:orderNo` —— 支付成功与否的唯一判据。
  Future<PayOrderInfo> orderStatus(String orderNo);

  /// `GET /api/pay/orders` —— H10 充值记录。
  Future<List<PayOrderInfo>> orders();

  /// `POST /api/pay/mock/settle` —— 仅联调渠道可用,开关关闭时服务端返回 404。
  Future<void> settleMock(String orderNo, bool success);
```

- [ ] **Step 6: 改远端实现**

`app/bottles/lib/data/remote/remote_repositories.dart`，把 `recharge` 整个替换：

```dart
  @override
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel) async {
    // ⚠️ iOS 的 IAP 不在这条路上:/pay/iap/verify 要的是 StoreKit 的
    // signed_transaction(pay/handler.go:81),不是 package_id。旧代码给它传
    // package_id,那条路径从来没通过(必 400)。StoreKit 接入时另走 IAP 适配器。
    final data = _obj(await _api.post<dynamic>('/pay/order', body: {
      // package_id 后端是 int64 且没有 `,string`,传字符串会 400。
      'package_id': int.tryParse(pkg.id) ?? 0,
    }));
    return PendingOrder(
      orderNo: (data['order_no'] ?? '').toString(),
      payParams: (data['pay_params'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
  }

  @override
  Future<PayOrderInfo> orderStatus(String orderNo) async =>
      PayOrderInfo.fromJson(_obj(await _api.get<dynamic>('/pay/order/$orderNo')));

  @override
  Future<List<PayOrderInfo>> orders() async {
    final data = await _api.get<dynamic>('/pay/orders');
    return [for (final o in _rows(data, 'list')) PayOrderInfo.fromJson(o)];
  }

  @override
  Future<void> settleMock(String orderNo, bool success) => _api.post<dynamic>(
        '/pay/mock/settle',
        body: {'order_no': orderNo, 'result': success ? 'success' : 'fail'},
      );
```

> `_obj` / `_rows` 是该文件里已有的私有辅助。若 `orders()` 的实际响应是裸数组，
> `_rows(data, 'list')` 已经兼容两种形状——先按现有辅助的行为走，不要另写解析。

- [ ] **Step 7: 改 mock 实现**

`app/bottles/lib/data/mock/mock_backend.dart`，把 `recharge` 替换为一个带状态的订单表：

```dart
  /// 联调用的订单表。key 是 order_no。
  final Map<String, Map<String, dynamic>> orders = {};
  int _orderSeq = 0;

  /// 下单:只落一笔 pending,**不加币**。加币发生在结算时,与真实链路一致。
  Map<String, dynamic> createOrder(RechargePackage pkg) {
    final no = 'MOCK${(++_orderSeq).toString().padLeft(6, '0')}';
    orders[no] = {
      'order_no': no,
      'status': 'pending',
      'coins': pkg.coins,
      'price_fen': pkg.priceMinor,
      'currency': pkg.currency,
      'platform': 'app',
      'created_at': DateTime.now().toUtc().toIso8601String(),
      'paid_at': null,
    };
    return {'order_no': no, 'pay_params': {'mock': true, 'order_no': no}};
  }

  Map<String, dynamic> orderStatus(String orderNo) =>
      orders[orderNo] ?? {'order_no': orderNo, 'status': 'pending', 'coins': 0};

  List<Map<String, dynamic>> orderList() => orders.values.toList().reversed.toList();

  /// 结算。成功时才加币,且对同一单幂等——真实链路的 HandleCallback 也是这么做的。
  void settleMock(String orderNo, bool success) {
    final o = orders[orderNo];
    if (o == null || o['status'] != 'pending') return;
    if (!success) {
      o['status'] = 'failed';
      return;
    }
    o['status'] = 'paid';
    o['paid_at'] = DateTime.now().toUtc().toIso8601String();
    credit(o['coins'] as int, TxnScene.recharge, note: '充值 · $orderNo');
  }
```

> `pkg.priceMinor` / `pkg.currency`:若 `RechargePackage` 现有字段名不同（如 `priceLabel`），
> 按该类**实际字段**取，不要新增字段。价格只用于 H10 的展示。

`app/bottles/lib/data/mock/mock_repositories.dart` 的 `MockWalletRepository` 跟随：

```dart
  @override
  Future<PendingOrder> recharge(RechargePackage pkg, PayChannel channel) async {
    final d = _db.createOrder(pkg);
    return _delay(
      PendingOrder(
        orderNo: d['order_no'] as String,
        payParams: (d['pay_params'] as Map).cast<String, dynamic>(),
      ),
      600,
    );
  }

  @override
  Future<PayOrderInfo> orderStatus(String orderNo) =>
      _delay(PayOrderInfo.fromJson(_db.orderStatus(orderNo)), 200);

  @override
  Future<List<PayOrderInfo>> orders() =>
      _delay([for (final o in _db.orderList()) PayOrderInfo.fromJson(o)], 300);

  @override
  Future<void> settleMock(String orderNo, bool success) async {
    _db.settleMock(orderNo, success);
    return _delay(null, 300);
  }
```

- [ ] **Step 8: 修好调用点，让它先编译过**

`recharge_page.dart` 的 `_pay` 此刻会因为返回类型变化而报错。**本步只做最小改动**
（Task 6 再接支付页）：把 `await ref.read(walletRepoProvider).recharge(pkg, channel);`
的结果接住并暂时 `debugPrint` 掉不用，或直接留一个 `final order = await ...;`
配合 `// TODO(Task 6)` —— **不要**在这里写跳转逻辑。

> 允许这一处临时 TODO 的理由:Task 3 的交付物是数据层,支付页属于 Task 6。
> 让编译先过,两个任务才能各自独立提交与回滚。

- [ ] **Step 9: 验证**

Run: `cd app/bottles && flutter analyze && flutter test`
Expected: analyze 无 error（info 级提示可接受），测试全过

- [ ] **Step 10: 提交**

```bash
git add app/bottles/lib/domain/models/wallet.dart app/bottles/lib/data app/bottles/test/wallet_model_test.dart app/bottles/lib/features/me/recharge_page.dart
git commit -m "feat(app): order model and repository for the real payment lifecycle"
```

---

## Task 4: 渠道适配器

**Files:**
- Create: `app/bottles/lib/core/pay/pay_channel.dart`
- Create: `app/bottles/test/pay_channel_test.dart`

**Interfaces:**
- Consumes: Task 3 的 `PendingOrder`、`WalletRepository.settleMock`
- Produces:
  - `enum ChannelOutcome { launched, cancelled, failed }`
  - `abstract class PayChannelAdapter { Future<ChannelOutcome> launch(PendingOrder order); }`
  - `class MockChannelAdapter implements PayChannelAdapter`（构造参数：`Future<void> Function(String orderNo, bool success) settle` 与 `Future<MockChoice?> Function() ask`）
  - `enum MockChoice { success, fail, noResponse }`

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/pay_channel_test.dart`：

```dart
import 'package:bottles/core/pay/pay_channel.dart';
import 'package:bottles/domain/models/wallet.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const order = PendingOrder(orderNo: 'NO1', payParams: {'mock': true});

  test('choosing success settles the order as paid', () async {
    String? settled;
    bool? asPaid;
    final adapter = MockChannelAdapter(
      settle: (no, ok) async {
        settled = no;
        asPaid = ok;
      },
      ask: () async => MockChoice.success,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(settled, 'NO1');
    expect(asPaid, isTrue);
  });

  test('choosing failure settles the order as failed', () async {
    bool? asPaid;
    final adapter = MockChannelAdapter(
      settle: (_, ok) async => asPaid = ok,
      ask: () async => MockChoice.fail,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(asPaid, isFalse);
  });

  // 「不返回结果」就是掉单现场:什么都不结算,让轮询自己超时到 H9。
  test('choosing no response settles nothing, so the poller can time out', () async {
    var called = false;
    final adapter = MockChannelAdapter(
      settle: (_, __) async => called = true,
      ask: () async => MockChoice.noResponse,
    );

    expect(await adapter.launch(order), ChannelOutcome.launched);
    expect(called, isFalse);
  });

  test('dismissing the sheet counts as cancelled', () async {
    final adapter = MockChannelAdapter(
      settle: (_, __) async {},
      ask: () async => null,
    );

    expect(await adapter.launch(order), ChannelOutcome.cancelled);
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/pay_channel_test.dart`
Expected: 编译失败，`package:bottles/core/pay/pay_channel.dart` 不存在

- [ ] **Step 3: 实现适配器**

创建 `app/bottles/lib/core/pay/pay_channel.dart`：

```dart
import '../../domain/models/wallet.dart';

/// 渠道拉起的结果。
///
/// ⚠️ 它**不代表支付成功**——成功与否一律以服务端订单状态为准,渠道说什么都不算。
/// 渠道说成功而服务端没入账,给用户发币就是白送;渠道说失败而服务端已入账,
/// 报失败会引来一通「我明明付了」的客诉。两边不一致时,服务端说了算。
enum ChannelOutcome { launched, cancelled, failed }

abstract class PayChannelAdapter {
  /// 拉起渠道。返回后进入轮询,由服务端订单状态定生死。
  Future<ChannelOutcome> launch(PendingOrder order);
}

/// 联调时让人自己选结果。
enum MockChoice { success, fail, noResponse }

/// 假渠道适配器。
///
/// 三个选项对应三条必须测到的路径:成功(H8)、失败、以及**什么都不回**——
/// 最后一个正是掉单(H9)的现场,也是真实渠道里最难复现的一种。
class MockChannelAdapter implements PayChannelAdapter {
  const MockChannelAdapter({required this.settle, required this.ask});

  /// 调 `POST /pay/mock/settle`。
  final Future<void> Function(String orderNo, bool success) settle;

  /// 弹出三选一。返回 null 表示用户把面板关掉了。
  final Future<MockChoice?> Function() ask;

  @override
  Future<ChannelOutcome> launch(PendingOrder order) async {
    final choice = await ask();
    if (choice == null) return ChannelOutcome.cancelled;
    switch (choice) {
      case MockChoice.success:
        await settle(order.orderNo, true);
      case MockChoice.fail:
        await settle(order.orderNo, false);
      case MockChoice.noResponse:
        break; // 故意什么都不做
    }
    return ChannelOutcome.launched;
  }
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/pay_channel_test.dart`
Expected: 4 个测试全过

- [ ] **Step 5: 提交**

```bash
git add app/bottles/lib/core/pay/pay_channel.dart app/bottles/test/pay_channel_test.dart
git commit -m "feat(app): payment channel adapter with a mock implementation"
```

---

## Task 5: 支付状态机

**Files:**
- Create: `app/bottles/lib/features/me/payment_controller.dart`
- Create: `app/bottles/test/payment_controller_test.dart`

**Interfaces:**
- Consumes: Task 3 的 `PayOrderInfo` / `OrderStatus`、Task 4 的 `PayChannelAdapter`
- Produces:
  - `enum PaymentPhase { launching, awaitingChannel, verifying, succeeded, dropped, failed, refunded }`
  - `class PaymentState { phase, order, elapsed, attempts }`
  - `Duration pollDelay(int attempt)`（纯函数，供测试与控制器共用）
  - `PaymentPhase phaseFor(OrderStatus status, {required bool ios})`
  - `class PaymentController extends AutoDisposeNotifier<PaymentState>`，方法 `start()`、`pollNow()`、`onResumed()`、`dispose` 时停表

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/payment_controller_test.dart`：

```dart
import 'package:bottles/domain/models/wallet.dart';
import 'package:bottles/features/me/payment_controller.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('poll backoff', () {
    // 1 → 2 → 4 → 8 秒封顶。开头密是因为多数支付在几秒内就有结果,
    // 封顶是因为再稀就不如让用户自己点「已完成支付」。
    test('doubles up to eight seconds and then stays there', () {
      expect(pollDelay(0), const Duration(seconds: 1));
      expect(pollDelay(1), const Duration(seconds: 2));
      expect(pollDelay(2), const Duration(seconds: 4));
      expect(pollDelay(3), const Duration(seconds: 8));
      expect(pollDelay(4), const Duration(seconds: 8));
      expect(pollDelay(99), const Duration(seconds: 8));
    });
  });

  group('phase mapping', () {
    test('pending shows the platform-specific waiting screen', () {
      expect(phaseFor(OrderStatus.pending, ios: false), PaymentPhase.awaitingChannel);
      expect(phaseFor(OrderStatus.pending, ios: true), PaymentPhase.verifying);
    });

    test('terminal statuses map to their own screens', () {
      expect(phaseFor(OrderStatus.paid, ios: false), PaymentPhase.succeeded);
      expect(phaseFor(OrderStatus.failed, ios: false), PaymentPhase.failed);
      // 退款不是支付失败,它有自己的一屏(H11),且要解释负余额。
      expect(phaseFor(OrderStatus.refunded, ios: false), PaymentPhase.refunded);
    });
  });

  group('timeout', () {
    // 原型明示「不能一直挂着」。30 秒之后继续转圈只会让用户重复支付。
    test('gives up after thirty seconds', () {
      expect(hasTimedOut(const Duration(seconds: 29)), isFalse);
      expect(hasTimedOut(const Duration(seconds: 30)), isTrue);
      expect(hasTimedOut(const Duration(seconds: 31)), isTrue);
    });
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/payment_controller_test.dart`
Expected: 编译失败，`payment_controller.dart` 不存在

- [ ] **Step 3: 写纯函数与状态类型**

创建 `app/bottles/lib/features/me/payment_controller.dart`，先写纯函数部分：

```dart
import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/logging/log_record.dart';
import '../../core/logging/logger.dart';
import '../../core/pay/pay_channel.dart';
import '../../core/providers.dart';
import '../../domain/models/wallet.dart';

/// 一笔支付走过的阶段。
///
/// ⚠️ [awaitingChannel] 与 [verifying] 是**同一档的两个平台变体,不是先后两步**:
/// Android 跳出去付(H7a),iOS 把票据交服务端核实(H7b)。两者共用同一个轮询器。
enum PaymentPhase {
  launching,
  awaitingChannel, // H7a
  verifying, // H7b
  succeeded, // H8
  dropped, // H9
  failed,
  refunded, // → H11
}

/// 轮询退避:1 → 2 → 4 → 8 秒封顶。
Duration pollDelay(int attempt) {
  const steps = [1, 2, 4, 8];
  return Duration(seconds: attempt < steps.length ? steps[attempt] : 8);
}

/// 累计多久还没到终态就判掉单。
const kPaymentTimeout = Duration(seconds: 30);

bool hasTimedOut(Duration elapsed) => elapsed >= kPaymentTimeout;

/// 服务端订单状态 → 屏幕阶段。
PaymentPhase phaseFor(OrderStatus status, {required bool ios}) => switch (status) {
      OrderStatus.paid => PaymentPhase.succeeded,
      OrderStatus.failed => PaymentPhase.failed,
      OrderStatus.refunded => PaymentPhase.refunded,
      OrderStatus.pending =>
        ios ? PaymentPhase.verifying : PaymentPhase.awaitingChannel,
    };

class PaymentState {
  const PaymentState({
    required this.phase,
    this.order,
    this.elapsed = Duration.zero,
    this.attempts = 0,
  });

  final PaymentPhase phase;
  final PayOrderInfo? order;
  final Duration elapsed;
  final int attempts;

  PaymentState copyWith({
    PaymentPhase? phase,
    PayOrderInfo? order,
    Duration? elapsed,
    int? attempts,
  }) =>
      PaymentState(
        phase: phase ?? this.phase,
        order: order ?? this.order,
        elapsed: elapsed ?? this.elapsed,
        attempts: attempts ?? this.attempts,
      );
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/payment_controller_test.dart`
Expected: 全部 PASS

- [ ] **Step 5: 写控制器**

在同一文件追加：

```dart
/// 支付状态机。
///
/// 只有一条真理:**服务端订单状态**。渠道回调、用户点「我已支付」、从后台切回来——
/// 这些统统只是「该查一次了」的信号,不改变状态本身。
class PaymentController extends AutoDisposeFamilyNotifier<PaymentState, String> {
  Timer? _timer;
  final _watch = Stopwatch();
  bool _stopped = false;

  bool get _ios => !kIsWeb && Platform.isIOS;

  @override
  PaymentState build(String orderNo) {
    ref.onDispose(_stop);
    return const PaymentState(phase: PaymentPhase.launching);
  }

  /// 拉起渠道后开始轮询。[adapter] 由页面按平台注入。
  Future<void> start(PayChannelAdapter adapter, PendingOrder pending) async {
    _watch.start();
    state = state.copyWith(
      phase: _ios ? PaymentPhase.verifying : PaymentPhase.awaitingChannel,
    );
    Log.i(LogTag.biz, 'pay_launch', fields: {'order': pending.orderNo});

    final outcome = await adapter.launch(pending);
    if (outcome == ChannelOutcome.failed) {
      // 渠道明确失败也不直接下结论:服务端可能已经入账(回调先到)。查一次再说。
      Log.w(LogTag.biz, 'pay_channel_failed', fields: {'order': pending.orderNo});
    }
    await pollNow();
    _schedule();
  }

  /// 立刻查一次。用户点「已完成支付」、从后台切回来、H9 点「主动查单」都走这里。
  Future<void> pollNow() async {
    if (_stopped) return;
    try {
      final info = await ref.read(walletRepoProvider).orderStatus(arg);
      final phase = phaseFor(info.status, ios: _ios);
      state = state.copyWith(
        phase: phase,
        order: info,
        elapsed: _watch.elapsed,
        attempts: state.attempts + 1,
      );
      if (info.status.isTerminal) {
        _stop();
        if (info.status == OrderStatus.paid) {
          // 到账了才刷钱包,顺带让流水页下次进入是新的。
          await ref.read(walletProvider.notifier).refresh();
          ref.invalidate(walletTxnsProvider);
        }
        Log.i(LogTag.biz, 'pay_settled',
            fields: {'order': arg, 'status': info.status.name});
      }
    } catch (e) {
      // 查单失败不改变阶段:网络抖一下不该让用户以为钱丢了,继续退避重试。
      state = state.copyWith(
        elapsed: _watch.elapsed,
        attempts: state.attempts + 1,
      );
      Log.w(LogTag.biz, 'pay_poll_failed', fields: {'order': arg});
    }
    _checkTimeout();
  }

  /// 从后台切回来:立刻查一次并把退避重置。
  ///
  /// 用户从渠道 App 切回来的那一刻,是状态最可能已经变了的时刻。
  /// 等退避周期到期是白等,而那几秒正是他盯着转圈的几秒。
  void onResumed() {
    if (_stopped) return;
    state = state.copyWith(attempts: 0);
    _timer?.cancel();
    pollNow().then((_) => _schedule());
  }

  void _schedule() {
    if (_stopped) return;
    _timer?.cancel();
    _timer = Timer(pollDelay(state.attempts), () => pollNow().then((_) => _schedule()));
  }

  void _checkTimeout() {
    if (_stopped || state.phase == PaymentPhase.succeeded) return;
    if (hasTimedOut(_watch.elapsed)) {
      _stop();
      state = state.copyWith(phase: PaymentPhase.dropped);
      Log.w(LogTag.biz, 'pay_dropped', fields: {'order': arg});
    }
  }

  void _stop() {
    _stopped = true;
    _timer?.cancel();
    _timer = null;
    if (_watch.isRunning) _watch.stop();
  }
}

final paymentControllerProvider = AutoDisposeNotifierProvider.family<
    PaymentController, PaymentState, String>(PaymentController.new);
```

> **注意 Riverpod 版本**:`pubspec.yaml` 是 `flutter_riverpod ^3.4.3`。
> 若 `AutoDisposeFamilyNotifier` / `AutoDisposeNotifierProvider.family` 在该版本下
> 名称不同，**照仓库里已有的 family provider 写法改**（`lib/core/providers.dart` 里有现成例子），
> 不要为此升级依赖。

- [ ] **Step 6: 验证**

Run: `cd app/bottles && flutter analyze && flutter test`
Expected: analyze 无 error，测试全过

- [ ] **Step 7: 提交**

```bash
git add app/bottles/lib/features/me/payment_controller.dart app/bottles/test/payment_controller_test.dart
git commit -m "feat(app): payment state machine polling the server order status"
```

---

## Task 6: H7a / H7b / H8 / H9 四态一页

**Files:**
- Create: `app/bottles/lib/features/me/payment_flow_page.dart`
- Create: `app/bottles/test/payment_flow_test.dart`
- Modify: `app/bottles/lib/app/routes.dart`
- Modify: `app/bottles/lib/app/router.dart`
- Modify: `app/bottles/lib/features/me/recharge_page.dart`
- Modify: `app/bottles/lib/l10n/app_zh.arb` / `app_en.arb`

**Interfaces:**
- Consumes: Task 4 的 `MockChannelAdapter`、Task 5 的 `paymentControllerProvider`
- Produces:
  - `Routes.payFlow(String orderNo, {String? from})` → `/me/recharge/pay/:orderNo?from=...`
  - `class PaymentFlowPage extends ConsumerStatefulWidget`（构造参数 `orderNo`、`from`）

- [ ] **Step 1: 加路由常量**

`app/bottles/lib/app/routes.dart`，在 `recharge` 之后：

```dart
  /// H7a / H7b / H8 / H9 —— **四个阶段共用一条路由**。
  ///
  /// 拆成四条会让「付完按返回」退回「支付中」,这是支付页最常见的返回栈事故。
  /// [from] 是被打断的原场景,H8 的主按钮照它回跳(多数充值是被 H5 次数用尽
  /// 或 Z4 余额不足打断后进来的,回到「我的」等于让用户自己找回去)。
  static String payFlow(String orderNo, {String? from}) =>
      '/me/recharge/pay/$orderNo${from == null ? '' : '?from=${Uri.encodeComponent(from)}'}';

  /// H10 充值记录。
  static const payRecords = '/me/recharge/records';

  /// H11 退款已撤销。
  static const voided = '/me/wallet/voided';
```

- [ ] **Step 2: 写文案**

`app/bottles/lib/l10n/app_zh.arb` 加（`app_en.arb` 加同样的 key，英文文案）：

```json
  "payWaitingTitle": "支付处理中",
  "payWaitingBody": "交易还在进行，请不要重复支付",
  "payWaitingElapsed": "已等待 {sec} 秒",
  "payVerifyingTitle": "正在核实票据",
  "payVerifyingBody": "正在向 App Store 核实这笔交易",
  "payDoneButton": "我已完成支付",
  "payTroubleButton": "遇到问题",
  "paySuccessTitle": "充值成功",
  "paySuccessCoins": "{coins} 金币已到账",
  "payBackToScene": "回到刚才的页面",
  "payViewRecords": "查看充值记录",
  "payDroppedTitle": "已扣款未到账",
  "payDroppedBody": "钱不会丢。若 24 小时内仍未到账，请联系客服，我们会人工补单",
  "payRecheckButton": "主动查单",
  "payContactSupport": "联系客服",
  "payFailedTitle": "支付未完成",
  "payFailedBody": "这笔交易没有成功，你没有被扣款",
  "payLeaveConfirm": "交易可能仍在进行，确定离开？",
  "payMockTitle": "模拟支付（联调）",
  "payMockSuccess": "模拟支付成功",
  "payMockFail": "模拟支付失败",
  "payMockNoResponse": "不返回结果（模拟掉单）"
```

带占位符的 key 需要在 arb 里配 `@key` 的 `placeholders`——**照该文件里已有的带参 key 抄格式**
（例如 `quotaUsedUpTitle`）。

- [ ] **Step 3: 写页面**

创建 `app/bottles/lib/features/me/payment_flow_page.dart`。骨架：

```dart
/// H7a / H7b / H8 / H9 四态一页。
///
/// 为什么是一页:它们是同一笔交易的四个阶段。做成四条路由后,H8 按返回会回到
/// 「支付中」——用户会以为又要付一次。
class PaymentFlowPage extends ConsumerStatefulWidget {
  const PaymentFlowPage({
    super.key,
    required this.orderNo,
    required this.pending,
    this.from,
  });

  final String orderNo;

  /// 下单结果。页面负责按它选适配器,再交给状态机。
  final PendingOrder pending;

  /// 被打断的原场景。H8 主按钮照它回跳。
  final String? from;

  @override
  ConsumerState<PaymentFlowPage> createState() => _PaymentFlowPageState();
}

class _PaymentFlowPageState extends ConsumerState<PaymentFlowPage>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _start());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState s) {
    // 从渠道 App 切回来的那一刻,状态最可能已经变了。
    if (s == AppLifecycleState.resumed) {
      ref.read(paymentControllerProvider(widget.orderNo).notifier).onResumed();
    }
  }

  Future<void> _start() async {
    final repo = ref.read(walletRepoProvider);
    final adapter = MockChannelAdapter(
      settle: repo.settleMock,
      ask: () => showMockChoiceSheet(context),
    );
    await ref
        .read(paymentControllerProvider(widget.orderNo).notifier)
        .start(adapter, widget.pending);
  }
  // build:按 state.phase switch 出四个子视图
}
```

四个子视图按 spec §4.4：

- **H7a**（`awaitingChannel`）：大号 loading + `payWaitingTitle` / `payWaitingBody` + `payWaitingElapsed`；底部两个弱按钮 `payDoneButton`（调 `pollNow()`）、`payTroubleButton`（直接把 phase 推到 dropped）
- **H7b**（`verifying`）：同构，换 `payVerifyingTitle` / `payVerifyingBody`，**不给取消按钮**（票据已在苹果侧生成，取消只会变成掉单）
- **H8**（`succeeded`）：金币数 + 新余额；主按钮 `payBackToScene`（有 `from` 就 `context.go(from)`，否则回钱包）、次按钮 `payViewRecords`
- **H9**（`dropped`）：`payDroppedTitle` / `payDroppedBody` + 三个出口：`payRecheckButton`（`pollNow()`，成功自然转 H8）、`payViewRecords`、`payContactSupport`
- `failed`：`payFailedTitle` / `payFailedBody` + 回充值页
- `refunded`：直接 `context.go(Routes.voided)`

返回键拦截（`PopScope`）：`awaitingChannel` / `verifying` 阶段弹 `payLeaveConfirm` 二次确认。
**确认离开后不要 dispose 掉轮询**——`AutoDispose` 会在页面销毁时停表，
所以这里改为：确认后仍 `pop`，并 toast 告知「交易仍在进行，可在充值记录里查看」。

> 样式与组件复用 `ui/widgets/` 下现成的 `GradientHeader` / `PrimaryButton` / `CoinChip` /
> `AsyncView` / `SkeletonRows`，配色用 `core/design/tokens.dart` 的 `Dim`。
> 不要自造一套按钮或间距。

- [ ] **Step 4: 挂进 router**

`app/bottles/lib/app/router.dart` 加三条 GoRoute（H10 / H11 的页面在 Task 7/8 创建，
本步只加支付页这一条，其余两条等对应任务）：

```dart
      GoRoute(
        path: '/me/recharge/pay/:orderNo',
        builder: (c, s) {
          final extra = s.extra as PendingOrder?;
          return PaymentFlowPage(
            orderNo: s.pathParameters['orderNo']!,
            // 直接深链进来(比如从记录页)时没有 extra,按 mock 之外的空参数走,
            // 状态机照样能靠轮询判断这笔单的死活。
            pending: extra ??
                PendingOrder(
                  orderNo: s.pathParameters['orderNo']!,
                  payParams: const {},
                ),
            from: s.uri.queryParameters['from'],
          );
        },
      ),
```

- [ ] **Step 5: 接上充值页**

`app/bottles/lib/features/me/recharge_page.dart` 的 `_pay`，把 Task 3 留下的 TODO 换成：

```dart
      final order = await ref.read(walletRepoProvider).recharge(pkg, channel);
      Log.i(LogTag.biz, 'pay_order_created', fields: {'order': order.orderNo});
      if (!mounted) return;
      context.push(
        Routes.payFlow(order.orderNo, from: widget.from),
        extra: order,
      );
```

`RechargePage` 需要接一个可空的 `from`（从 H5 / Z4 跳过来时带上）。
若现有调用点没有传，默认 `null` 即可——**不要**为此改 H5 的弹窗逻辑，那是另一件事。

- [ ] **Step 6: 写 widget test**

创建 `app/bottles/test/payment_flow_test.dart`，用 `mock_backend` 走三条路径：

```dart
// 三条路径:成功 → H8、失败 → 失败态、不返回结果 → 轮询超时 → H9。
//
// 超时那条用 tester.pump(Duration(seconds: 31)) 推进虚拟时钟,不要真等 30 秒。
```

三个 `testWidgets`：
1. `settleMock(no, true)` 后 `pump` 若干次 → 断言出现 `paySuccessTitle` 的文案
2. `settleMock(no, false)` → 断言出现 `payFailedTitle`
3. 什么都不结算，`pump` 过 31 秒 → 断言出现 `payDroppedTitle`

`ProviderScope` 的 override 照 `test/widget_test.dart` 现有写法（`prefsProvider.overrideWithValue`），
仓库的 `AppConfig.useMock` 在测试里应为 true —— 若不是，用 `walletRepoProvider.overrideWithValue(MockWalletRepository(MockBackend()))`。

- [ ] **Step 7: 验证**

Run: `cd app/bottles && flutter analyze && flutter test`
Expected: analyze 无 error，全部测试通过

- [ ] **Step 8: 提交**

```bash
git add app/bottles/lib/features/me/payment_flow_page.dart app/bottles/lib/app app/bottles/lib/features/me/recharge_page.dart app/bottles/lib/l10n app/bottles/test/payment_flow_test.dart
git commit -m "feat(app): payment progress, success and dropped-order screens"
```

---

## Task 7: H10 充值记录

**Files:**
- Create: `app/bottles/lib/features/me/pay_records_page.dart`
- Modify: `app/bottles/lib/app/router.dart`
- Modify: `app/bottles/lib/features/me/wallet_page.dart`（入口）
- Modify: `app/bottles/lib/l10n/app_zh.arb` / `app_en.arb`

**Interfaces:**
- Consumes: Task 3 的 `WalletRepository.orders()` / `orderStatus()`、`Routes.payRecords`
- Produces: `class PayRecordsPage extends ConsumerStatefulWidget`

- [ ] **Step 1: 写文案**

两份 arb 各加：

```json
  "payRecordsTitle": "充值记录",
  "payRestore": "恢复购买",
  "payRestoreDone": "{n} 笔订单已更新",
  "payStatusPaid": "已到账",
  "payStatusPending": "处理中",
  "payStatusFailed": "支付失败",
  "payStatusRefunded": "已退款",
  "payRecordsEmpty": "还没有充值记录"
```

- [ ] **Step 2: 写页面**

创建 `app/bottles/lib/features/me/pay_records_page.dart`：

- 列表数据来自 `walletRepoProvider.orders()`，用 `AsyncView` 渲染三态（骨架 / 空态 / 失败）
- 每行：金币数（主）/ 渠道 · 时间（次）/ 状态标签 / 金额
- 状态四色，取 `core/design/tokens.dart` 里现成的语义色，不要写死十六进制：
  已到账（绿）· 处理中（橙）· 支付失败（灰）· 已退款（红）
- **处理中的行可点**：点了调 `orderStatus(orderNo)` 刷这一行；若变成 `paid` 就刷新钱包
- 右上角「恢复购买」：

```dart
  /// 「恢复购买」。
  ///
  /// 金币是消耗型商品,苹果不强制提供这个入口,但它是掉单时用户能自救的唯一按钮。
  /// mock 阶段的语义 = 把所有 pending 单重查一遍;接入 StoreKit / Play 之后,
  /// 在同一个按钮里补上「拉渠道未完成交易」那一步,**按钮与文案都不用改**。
  Future<void> _restore() async {
    final repo = ref.read(walletRepoProvider);
    final list = await repo.orders();
    var updated = 0;
    for (final o in list.where((o) => o.status == OrderStatus.pending)) {
      final fresh = await repo.orderStatus(o.orderNo);
      if (fresh.status != OrderStatus.pending) updated++;
    }
    if (updated > 0) await ref.read(walletProvider.notifier).refresh();
    // toast: payRestoreDone(updated)
  }
```

- [ ] **Step 3: 挂路由 + 钱包页入口**

`router.dart` 加 `GoRoute(path: Routes.payRecords, builder: (c, s) => const PayRecordsPage())`。
`wallet_page.dart` 顶部操作区加一个「充值记录」入口指向它。

- [ ] **Step 4: 写 widget test**

在 `app/bottles/test/payment_flow_test.dart` 追加一个 group：
mock 里造三笔不同状态的订单 → 断言四种状态标签文案各自出现、空列表时出现 `payRecordsEmpty`。

- [ ] **Step 5: 验证**

Run: `cd app/bottles && flutter analyze && flutter test`
Expected: 全过

- [ ] **Step 6: 提交**

```bash
git add app/bottles/lib/features/me/pay_records_page.dart app/bottles/lib/app/router.dart app/bottles/lib/features/me/wallet_page.dart app/bottles/lib/l10n app/bottles/test/payment_flow_test.dart
git commit -m "feat(app): recharge history with per-order recheck and restore"
```

---

## Task 8: H11 退款已撤销 + 负余额横幅 + 文档收尾

**Files:**
- Create: `app/bottles/lib/features/me/voided_page.dart`
- Modify: `app/bottles/lib/features/me/wallet_page.dart`
- Modify: `app/bottles/lib/app/router.dart`
- Modify: `app/bottles/lib/l10n/app_zh.arb` / `app_en.arb`
- Modify: `docs/APP_V1_PROGRESS.md`

**Interfaces:**
- Consumes: Task 3 的 `TxnScene.refund`、`Routes.voided`；现有 `walletProvider`（`Wallet.coins` / `totalRecharged` / `totalSpent`）、`walletTxnsProvider`
- Produces: `class VoidedPage extends ConsumerWidget`

- [ ] **Step 1: 写文案**

两份 arb 各加：

```json
  "voidedTitle": "有一笔充值被退款了",
  "voidedBody": "渠道已退还这笔充值，对应的金币已从账户扣回。由于其中一部分已经消费，余额出现负数",
  "voidedFrozen": "余额为负时，扔瓶 / 解锁 / 送礼暂停，充值仍然可用。补足后自动恢复",
  "voidedRelated": "相关流水",
  "voidedGoRecharge": "去充值",
  "voidedNotMe": "这不是我操作的",
  "walletNegativeBanner": "余额为负，点此了解原因"
```

- [ ] **Step 2: 写页面**

创建 `app/bottles/lib/features/me/voided_page.dart`：

- 顶部：负余额（红，大号）+ 累计充值 / 累计消费（`wallet.totalRecharged` / `totalSpent`）
- 说明区：`voidedTitle` + `voidedBody` + `voidedFrozen`
- 相关流水：从 `walletTxnsProvider` 里筛 `TxnScene.refund` 与紧邻的 `TxnScene.recharge`，各展示
- 底部：`voidedGoRecharge`（主，→ `Routes.recharge`）/ `voidedNotMe`（次，→ 客服入口）

页面顶部加一段注释，把「为什么必须有这屏」写清楚：

```dart
/// H11 退款已撤销。
///
/// 用户直接向渠道申请退款时,App 端收不到任何同步信号——服务端主动查是感知退款的
/// 唯一途径(pay/iap.go:295 对 Apple 侧写过同一句话)。服务端的 revokeIAP 明确
/// 「允许扣成负数」:币可能已经花掉,装作没发生只会让账永远对不平。
///
/// 那么界面就必须有个说法。否则用户打开钱包看到负数余额,只会认为是系统 bug 并直接投诉。
///
/// 注意「消费暂停」不需要任何客户端代码:wallet.Debit 在 bal < coins 时返回
/// ErrInsufficient(wallet/service.go:115),余额为负时任何消费都过不去。这屏只负责说清楚。
```

- [ ] **Step 3: 钱包页横幅**

`wallet_page.dart`：`wallet.coins < 0` 时在余额卡下方显示一条红色横幅
（`walletNegativeBanner`），点击 `context.push(Routes.voided)`。

- [ ] **Step 4: 挂路由**

`router.dart` 加 `GoRoute(path: Routes.voided, builder: (c, s) => const VoidedPage())`。

- [ ] **Step 5: 写 widget test**

追加两个 `testWidgets`：
1. 余额为负时钱包页出现横幅；余额非负时不出现
2. H11 页面展示负余额数值与 refund 流水（**并断言它显示为「退款」而不是「奖励」**——
   这正是 Task 3 修掉的 bug 的端到端回归）

- [ ] **Step 6: 更新进度文档**

`docs/APP_V1_PROGRESS.md`：
- §二 的表里把「11⁺ 支付全链路」那一行从 `⬜ 仅原型，未实现` 改为
  `⚠️ H7a/H7b/H8/H9/H10/H11 已实现（mock 渠道）；H3i 仍缺（依赖 StoreKit 商品 ID）`
- §三 B 的「支付全链路 6 屏落地」一行标记完成，并注明**上线前必须确认
  `app_pay_mock_enabled` 为关闭状态**
- §三 A 的「IAP 真实入账」补一句：客户端渠道适配器已就位，接 StoreKit 时实现
  `StoreKitAdapter` 即可，屏幕代码不动

- [ ] **Step 7: 全量验证**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
cd app/bottles && flutter analyze && flutter test
```
Expected: 全部通过

- [ ] **Step 8: 提交**

```bash
git add app/bottles/lib/features/me/voided_page.dart app/bottles/lib/features/me/wallet_page.dart app/bottles/lib/app/router.dart app/bottles/lib/l10n app/bottles/test docs/APP_V1_PROGRESS.md
git commit -m "feat(app): explain a voided refund and its negative balance"
```

---

## 自查

**Spec 覆盖对照**

| Spec 章节 | 落在哪个 Task |
|---|---|
| §1.2 断点 1（丢 order_no） | Task 3 Step 6 |
| §1.2 断点 2（IAP 传错参数） | Task 3 Step 6（删掉该调用并注明原因） |
| §1.2 断点 3（App 无渠道） | Task 2 Step 7 |
| §1.2 断点 4（无单笔查单） | Task 1 |
| §1.2 断点 5（`refund` 未解析） | Task 3 Step 1/3，Task 8 Step 5 端到端回归 |
| §3.1 查单接口 + 越权 | Task 1 全部 |
| §3.2 mock 渠道 + sysconfig + 缓存陷阱 | Task 2 全部 |
| §4.1 渠道适配器 | Task 4 |
| §4.2 状态机（退避 / 超时 / resumed / 防重复） | Task 5 + Task 6 Step 3 |
| §4.3 三条路由 | Task 6 Step 1/4、Task 7 Step 3、Task 8 Step 4 |
| §4.4 H7a/H7b/H8/H9 | Task 6 Step 3 |
| §4.4 H10 | Task 7 |
| §4.4 H11 | Task 8 |
| §4.5 l10n 双份 | Task 6 Step 2、Task 7 Step 1、Task 8 Step 1 |
| §五 要改的既有代码 | Task 3（数据层）、Task 6 Step 5（充值页）、Task 8 Step 3（钱包页） |
| §六 测试策略 | Task 1 Step 1、Task 2 Step 1、Task 4 Step 1、Task 5 Step 1、Task 6 Step 6、Task 7 Step 4、Task 8 Step 5 |

**命名一致性**：`PendingOrder` / `PayOrderInfo` / `OrderStatus` / `ChannelOutcome` /
`PaymentPhase` / `pollDelay` / `phaseFor` / `hasTimedOut` / `orderScope` /
`mockSettleResult` / `mockCallbackResult` / `SettleMock` / `Routes.payFlow` /
`Routes.payRecords` / `Routes.voided` —— 在定义处与引用处逐个核对过，一致。

**两处需要执行时按仓库实际情况对齐的地方**（已在对应步骤里写明，不是占位符）：
1. Task 5 的 Riverpod family notifier 写法，照 `lib/core/providers.dart` 现有 family provider 抄
2. Task 3 的 `RechargePackage` 价格字段名，照该类实际字段取

## 前置条件（代码之外）

- 联调时需在后台「App 支付」分组把 `app_pay_mock_enabled` 打开；**上线前必须关掉**
- `APP_DEFAULT_TENANT_ID` 需在服务器 `.env` 里配好（`APP_V1_PROGRESS.md` §六第 1 条），
  否则 App 端所有接口都拿不到租户，支付链路同样走不通
