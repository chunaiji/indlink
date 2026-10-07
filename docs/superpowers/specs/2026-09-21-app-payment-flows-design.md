# App 支付全链路（§11⁺ 六屏）设计

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-21 |
| 状态 | DESIGN — 已确认，待出实施计划 |
| 来源 | 原型 §11⁺ 已画完但客户端未实现，是 53 屏原型里唯一成片的缺口 |
| 范围 | H7a / H7b / H8 / H9 / H10 / H11 六屏 + 支撑它们的客户端支付状态机 + 服务端两处加法 |
| 关联 | `docs/prototype/v1-screens.html`（L3195–3700）· `docs/APP_V1_PROGRESS.md` §二 · `docs/superpowers/plans/2026-09-19-play-billing.md`（Task 2–5 未做） |
| 分支 | `feat/app-v1` |

---

## 一、目标与现状

### 1.1 要解决什么

原型对这几屏的判断是：「一个新业务逻辑都没有，全是状态的可见化」。但支付客诉集中的两处
——**「我付了钱没到账」**与**「我不小心付了两次」**——恰恰只能靠这几屏解决。
前者需要 H9 + H10 让用户自己查到订单状态，后者需要 H7a 在跳转期间明确告诉用户「交易还在进行」。

### 1.2 现状：这条链路现在是断的

充值页（H3）已经存在，但它下面什么都没有接上。

**断点 1：客户端把下单结果整个丢掉。**

```dart
// lib/data/remote/remote_repositories.dart:653
Future<Wallet> recharge(RechargePackage pkg, PayChannel channel) async {
  final path = channel == PayChannel.iap ? '/pay/iap/verify' : '/pay/order';
  await _api.post<dynamic>(path, body: {'package_id': ...});
  return wallet();      // ← order_no 与 pay_params 全部丢弃
}
```

服务端 `POST /pay/order` 返回的是 `{order_no, pay_params}`（`pay/handler.go:65`）。
客户端不接住 `order_no`，整条「跳渠道 → 回来 → 查状态」就无从谈起——
它现在的行为是**下完单立刻当作已支付**，直接重拉钱包。

**断点 2：iOS 那一支必定 400。**

同一个函数给 `/pay/iap/verify` 传的是 `package_id`，而服务端要的是
`signed_transaction`（`pay/handler.go:81`，`binding:"required"`）。这条路径从来没通过。

**断点 3：App 平台根本没有渠道。**

`buildDriver`（`pay/service.go:74`）只认 `wx` / `alipay`，其余一律
「不支持的支付平台」。而 `createOrder` 用**登录平台**决定渠道（`pay/handler.go:55`），
App 端登录 `platform=app`，所以下单必被拒。Play Billing 的服务端（play-billing 计划 Task 2–5）尚未实现。

**断点 4：没有按单号查状态的接口。**

只有列表 `GET /pay/orders`（`pay/handler.go:68`）。H7a 的轮询、H9 的「主动查单」、
H10 里点处理中的订单，都需要单笔查询。

**断点 5（客户端数据层）：`TxnScene.parse` 不认 `refund`。**

服务端有 `SceneRefund = "refund"`（`wallet/service.go:29`），
客户端 `lib/domain/models/wallet.dart` 的 `parse` 没有这一支，会掉进兜底的
`TxnScene.reward`——H11 的退款流水会显示成「奖励」，**金额还是负的**，直接误导用户。

### 1.3 不在本设计范围

- **H3i（IAP 商品列表）**——依赖 App Store Connect 商品 ID 与 StoreKit 拉取的本地化价格，
  拿不到凭证就无法验证，与本轮解耦，留到 StoreKit 接入时一起做
- **Play Billing / StoreKit 的真实接入**——本设计只定义适配器接口，不实现具体渠道
- **App Store Server Notifications 的接入**——`/pay/iap/notify` 已存在，退款扣回逻辑已在
  `pay/iap.go`；Play 侧的 Voided Purchases 轮询属于 play-billing 计划 Task 4
- **负余额时冻结消费**——**服务端已天然满足**：`wallet.Debit` 在 `bal < coins` 时返回
  `ErrInsufficient`（`wallet/service.go:115`），余额为负时任何消费都过不去。
  H11 只需把这件事**说清楚**，不需要新代码

---

## 二、决策记录

| # | 决策点 | 选定 | 理由 / 代价 |
|---|---|---|---|
| 1 | 没有真渠道时 H7a/H7b 接在什么上 | **抽 `PayChannelAdapter`，本轮只实现 mock 适配器** | 六屏与状态机立刻可跑可测；Play Billing / StoreKit 以后各加一份适配器，屏幕代码不动 |
| 2 | 订单怎么变成「已支付」 | **服务端加一个 sysconfig 控制的 mock 渠道**，默认关 | 成功 / 失败 / 一直 pending 三条分支在真机上都能走通，也是以后回归支付链路的固定手段 |
| 3 | mock 的入账走哪条路 | **复用现有 `HandleCallback`** | 它是唯一入账口且幂等、带金额校验（`pay/service.go:162`）。另开一条入账路径等于把幂等重写一遍 |
| 4 | H7a/H7b/H8/H9 的路由 | **一条路由 `/me/recharge/pay/:orderNo` 承载四个阶段** | 它们是同一笔交易的四个阶段。拆成四条路由后「付完按返回」会退回「支付中」，这是支付页最常见的返回栈事故 |
| 5 | 状态获取方式 | **客户端轮询**，退避 1→2→4→8s 封顶 | 复用 chat 的 WebSocket 会把支付耦合到聊天 Hub 上，而那个 Hub 目前是单实例内存态（`APP_V1_PROGRESS.md` §三 C） |
| 6 | 轮询何时放弃 | **累计 30s 未到终态 → H9 掉单** | 原型明示「服务端校验失败时不能一直挂着」。30s 之后继续转圈只会让用户重复支付 |
| 7 | 回到前台时的行为 | **`AppLifecycleState.resumed` 立即强制查一次并重置退避** | 用户从渠道 App 切回来的那一刻是状态最可能已变的时刻，等退避周期到期是白等 |
| 8 | H8 之后回哪 | **`from` 参数一路透传，主按钮回跳原场景** | 充值大多是被 H5「次数用尽」/ Z4「余额不足」打断后进来的。回到「我的」等于让用户自己找回去 |
| 9 | H11 的入口 | **钱包页检测 `coins < 0` 时的顶部横幅** | 不做推送。负余额是低频事件，为它加一个推送场景不划算 |
| 10 | 「恢复购买」在 mock 阶段做什么 | **拉 `/pay/orders` 里所有 pending 单，逐个查单** | 语义与真实渠道一致（查渠道未完成订单）；接入 StoreKit / Play 后在同一个按钮里补渠道侧查询 |
| 11 | 查单接口的越权防护 | **`WHERE order_no = ? AND user_id = ?`，查不到一律 `CodeOrderNotFound`** | 只按 order_no 查就是一个能遍历他人订单的接口。订单号可枚举与否不该是安全前提 |
| 12 | §1.2 的三个客户端 bug | **本轮一并修** | 都在这条链路上，不修则六屏建在沙上 |

---

## 三、服务端设计

两处加法，都不触碰小程序路径。

### 3.1 `GET /api/pay/order/:orderNo` — 单笔查单

挂在 `pay/handler.go` 现有的 `api` 组上（带 `auth`）。

```
GET /api/pay/order/:orderNo
→ 200 {"order_no","status","coins","price_fen","currency","platform","created_at","paid_at"}
→ 5002 CodeOrderNotFound   订单不存在 **或不属于当前用户**（两种情况同一个响应）
```

Service 侧新增：

```go
// Order 按单号查单笔订单。
//
// ⚠️ user_id 必须进 WHERE：只按 order_no 查，这就成了一个能遍历他人订单的接口。
// 订单号能不能被猜到，不该成为安全前提。
func (s *Service) Order(userID int64, orderNo string) (*model.PayOrder, error)
```

字段名沿用 `model.PayOrder` 的 JSON tag，`price_fen` **保持不变**——
`model.go:318` 已写明，改它会让线上小程序订单页立刻显示 `¥NaN`，而小程序发版要过微信审核。

### 3.2 mock 渠道

**sysconfig 新增 1 个 key**（写 `defaults` + 登记 `admin/meta.go` 白名单）：

| key | 默认 | 说明 |
|---|---|---|
| `app_pay_mock_enabled` | `"0"` | App 模拟支付渠道（**仅联调，上线前必须关**） |

meta 登记：

```go
{Key: sysconfig.KeyAppPayMockEnabled, LabelZh: "模拟支付渠道(仅联调,上线前关闭)",
 LabelEn: "Mock payment channel (testing only, disable before launch)",
 Group: GroupAppPay, Type: "bool"},
```

**Driver 实现** `pay/driver_mock.go`：

```go
// mockDriver 联调用的假渠道。Prepay 不调任何外部服务，只回一个标记；
// 真正的「支付结果」由客户端显式调 /pay/mock/settle 触发。
//
// ⚠️ 只在 app_pay_mock_enabled=1 时才会被 buildDriver 返回，默认关。
type mockDriver struct{}

func (mockDriver) Name() string { return "mock" }
func (mockDriver) Prepay(order *model.PayOrder, _ string) (map[string]interface{}, error) {
    return map[string]interface{}{"mock": true, "order_no": order.OrderNo}, nil
}
```

`VerifyCallback` / `SuccessResponse` 返回「不支持」——mock 不走平台异步回调那条路。

`buildDriver` 只加一个前置分支：`platform == "app"` 且开关打开 → `mockDriver`，
否则维持原逻辑（`wx` / `alipay`）。

> ⚠️ **`driverFor` 在 `platform == "app"` 时必须跳过缓存。**
> 缓存键是 `tenantID:platform`，mockDriver 一旦进了缓存，后台把开关关掉之后
> `createOrder` 仍然会走 mock——一个「已关闭」的假支付渠道继续发币。
> mockDriver 无状态、构造成本为零，每次现取即可。

**结算端点**（路由常驻，开关在 handler 内部按当前租户查）：

```
POST /api/pay/mock/settle   {"order_no": "...", "result": "success"|"fail"}
```

- `success` → 构造 `CallbackResult{OrderNo, TxnID: "mock-"+orderNo, Paid: true, AmountFen: order.PriceMinor}`
  交给**现有的** `HandleCallback("mock", res)`：幂等、金额校验、入账、首充打标全部白拿
- `fail` → 只把 `status` 置 `failed`（`WHERE status = 'pending'`，幂等），不动钱包
- 两种结果都必须先校验订单属于当前用户，理由同 §3.1
- **开关关闭时返回 `CodeNotFound`**：路由是常驻的（启动时注册一次），开关是租户级且可热更，
  所以判断只能在 handler 里做。返回 404 而不是 403——一个关掉的联调后门不该自曝存在

> **为什么结算由客户端显式触发，而不是服务端定时器自动置 paid：**
> 自动置 paid 只能测出「成功」这一条分支。而这六屏里真正值钱的是 H9 掉单和 H10 的失败行——
> 那两条只有让联调的人自己选结果才测得到。

---

## 四、客户端设计

### 4.1 渠道适配器 `lib/core/pay/pay_channel.dart`

```dart
/// 下单结果。`payParams` 原样透传给适配器，服务端给什么渠道就由谁认。
class PendingOrder {
  final String orderNo;
  final Map<String, dynamic> payParams;
}

/// 渠道拉起的结果。注意它**不代表支付成功**——
/// 成功与否一律以服务端订单状态为准，渠道说什么都不算。
enum ChannelOutcome { launched, cancelled, failed }

abstract class PayChannelAdapter {
  /// 拉起渠道。返回后进入轮询，由服务端订单状态定生死。
  Future<ChannelOutcome> launch(PendingOrder order);
}
```

本轮实现 `MockChannelAdapter`：识别 `payParams["mock"] == true`，
弹一个三选一（模拟成功 / 模拟失败 / 不返回结果）。前两个调 `/pay/mock/settle`，
第三个什么都不做——那正是掉单（H9）的现场。

以后 `PlayBillingAdapter`、`StoreKitAdapter` 各实现一份，**屏幕与状态机代码不改**。

### 4.2 状态机 `lib/features/me/payment_controller.dart`

```
                          ┌─ Android ─► awaitingChannel (H7a) ─┐
creating ──► launching ──►┤                                    ├──► succeeded (H8)
                 │        └─ iOS ─────► verifying (H7b) ───────┘
                 │                          │
            cancelled                  30s 超时 / 渠道明确失败
                 │                          ▼
                 └───────────────────► dropped (H9) / failed
```

- `awaitingChannel` 与 `verifying` 是**同一档的两个平台变体，不是先后两步**：
  Android 走「跳出去付」（H7a），iOS 走「票据交服务端核实」（H7b）。
  两者共用同一个轮询器与同一套终态判定，只换文案与插图
- 轮询 `GET /pay/order/:orderNo`，间隔 1→2→4→8s 封顶，**累计 30s** 仍非终态 → `dropped`
- `status == "paid"` → `succeeded`；`"failed"` → `failed`；`"refunded"` → 直接进 H11
- `AppLifecycleState.resumed` → 立即强制查一次并重置退避
- **防重复支付**：进入支付页后禁止再次下单；`awaitingChannel` 期间按返回键要二次确认
  （「交易可能仍在进行，确定离开？」），确认后**轮询不停**，转为后台完成，结果以 toast 告知

### 4.3 路由（`lib/app/routes.dart` 新增）

| 路由 | 屏 | 说明 |
|---|---|---|
| `/me/recharge/pay/:orderNo` | H7a / H7b / H8 / H9 | 四个阶段共用，`from` 作 query 参数透传 |
| `/me/recharge/records` | H10 | 充值记录 + 恢复购买 |
| `/me/wallet/voided` | H11 | 退款已撤销说明 |

`RechargePage` 改为：下单成功 → `context.push(Routes.payFlow(orderNo, from: ...))`，
**不再**自己 `pop` + toast。

### 4.4 六屏规格

**H7a 支付处理中（Android）**——大号 loading + 「交易还在进行，请不要重复支付」+ 已耗时秒数。
底部两个弱按钮：「已完成支付」（强制查一次）/「遇到问题」（→ H9）。

**H7b 票据校验中（iOS）**——同构，文案换成「正在向 App Store 核实票据」。
**不给取消按钮**：票据已在苹果侧生成，取消只会变成掉单。

**H8 支付成功**——金币数 + 新余额。主按钮按 `from` 回跳原场景（无 `from` 则回钱包），
次按钮「查看充值记录」→ H10。

**H9 已扣款未到账**——三个出口：「主动查单」（再查一次，成功则转 H8）、
「查看充值记录」、「联系客服」。文案必须说清**钱不会丢**，超过 24 小时未到账走人工补单。

**H10 充值记录**——`GET /pay/orders` 分页列表，每行：金币数 / 渠道 · 时间 / 状态标签 / 金额。
状态四色：已到账（绿）· 处理中（橙，**可点，点了就查单**）· 支付失败（灰）· 已退款（红）。
右上角「恢复购买」= 拉所有 pending 单逐个查一遍，结束后 toast 报「N 笔已更新」。

**H11 退款已撤销**——顶部负余额（红）+ 累计充值 / 累计消费。一段说明：
渠道于某日退还了多少钱、对应多少金币已扣回、部分已消费故余额为负。
下面是相关流水（refund 与对应的 recharge 两条）。
底部「去充值」（主）/「这不是我操作的」（→ 客服）。

### 4.5 文案

全部进 `lib/l10n/app_zh.arb` 与 `app_en.arb`。两份 key 必须齐——
缺 key 时 `flutter gen-l10n` 会静默回落英文，在中文界面里冒出一句英文。

---

## 五、要改的既有代码

| 文件 | 改动 |
|---|---|
| `lib/data/repositories.dart` | `recharge()` 改为返回 `PendingOrder`（`orderNo` + `payParams`），不再返回 `Wallet`；新增 `orderStatus(orderNo)`、`settleMock(orderNo, result)` |
| `lib/data/remote/remote_repositories.dart` | 接住 `order_no` / `pay_params`；**删掉** iOS 那支传错参数的 `/pay/iap/verify` 调用（StoreKit 接入时再按真实签名写） |
| `lib/data/mock/mock_backend.dart` | 补订单状态机的假数据，让 widget test 能走完四个阶段 |
| `lib/domain/models/wallet.dart` | `TxnScene` 加 `refund`，`parse` 补这一支 |
| `lib/features/me/recharge_page.dart` | 下单后跳支付页，不再假装已完成 |
| `lib/features/me/wallet_page.dart` | `coins < 0` 时顶部横幅 → H11 |

---

## 六、测试策略

**服务端：只有纯函数测试。**

仓库里 27 个 `_test.go` 中 `gorm.Open` 出现 **0 次**——服务端没有任何数据库测试基建，
现有的 `pay` 测试（`driver_wx_test.go` / `play_test.go`）全是纯函数。
本设计**不引入** sqlite 或 sqlmock：为两个端点引一棵依赖树，与 play-billing 计划
「本期一个 REST 端点不值得一棵依赖树」是同一个判断。

因此把该守的约束**挤进纯函数**里守：

- `pay/order_test.go`：`orderScope(userID, orderNo)` 返回的条件里必须同时含
  `order_no` 与 `user_id`。这个测试看着很轻，但它守的正是「有人图省事把 `user_id`
  从 WHERE 里删掉」——那一改动会把查单变成遍历他人订单的接口
- `pay/driver_mock_test.go`：`mockCallbackResult(order)` 的 `AmountFen` 必须等于
  `order.PriceMinor`（金额取错则 `HandleCallback` 的金额校验形同虚设）、
  `TxnID` 带 `mock-` 前缀（便于事后把假订单从真账里摘出来）；
  `mockSettleResult("success"|"fail"|其它)` 的解析与拒绝

> **明说局限**：入账幂等由 `HandleCallback` 的
> `WHERE status = 'pending'` + `RowsAffected == 0` 保证（`pay/service.go:186`），
> 该分支**本设计不新增测试也无法在当前基建下测**。它是既有代码、已在小程序线上跑了很久，
> 本设计只是复用它而没有改动——这是刻意的取舍，不是遗漏。

**客户端**

- `payment_controller` 纯逻辑测试：退避序列、30s 转 `dropped`、`resumed` 强制查询并重置退避、
  `refunded` 直接进 H11
- 六屏 widget test 走 `mock_backend`，覆盖成功 / 失败 / 掉单三条路径
- `TxnScene.parse('refund')` 的回归测试——这正是本轮修掉的 bug

**验证命令**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
cd app/bottles && flutter analyze && flutter test
```

> `/internal/robot` 有既有 flaky 测试（回复池随机选取），与本改动无关，见 `APP_V1_PROGRESS.md` §五。

---

## 七、遗留

| # | 事项 | 说明 |
|---|---|---|
| 1 | H3i + StoreKit | 需 App Store Connect 商品 ID；价格必须用商店返回的本地化字符串，不能自己拼 ₹ |
| 2 | Play Billing 服务端 | play-billing 计划 Task 2–5。做完后加 `PlayBillingAdapter`，本设计的屏幕不动 |
| 3 | 客服入口 | H9 / H11 的「联系客服」目前只能指向设置页现有入口，没有工单系统 |
| 4 | 24 小时人工补单 | 后台没有「手动补单」操作页，H9 的承诺目前靠人工改库兑现 |
