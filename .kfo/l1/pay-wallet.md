# L1 设计 — 支付 / 钱包模块(pay + wallet)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-20 |
| 覆盖模块 | `internal/pay`、`internal/wallet`、`internal/item` |
| 关联 | L0 `l0/architecture.md` · L2 `l2/2026-06-20-driftbottle-v1-and-api-tests.md` · L4 `l4/cross-platform-api-patterns.md` |

---

## 一、核心思想:充值与消费严格分离

| | 充值(真钱→金币) | 消费(站内扣金币) |
|---|---|---|
| 入口 | `pay.CreateOrder` + 平台异步回调 `pay.HandleCallback` | `wallet.Debit` |
| 加/扣 | `wallet.Credit`(只在回调内) | 单事务内 `FOR UPDATE` 扣减 |
| 幂等 | `pay_order.order_no` 唯一 + status 乐观更新 | biz_no 流水 |
| 真相源 | MySQL `wallet.balance`(绝不用 Redis) | 同左 |

## 二、wallet 接口(只暴露这三个)

```go
Credit(userID, coins, scene, bizNo) error          // 充值入账 / 注册奖励
Debit(userID, coins, scene, bizNo, biz func(tx)) error  // 开聊/解锁/送礼,业务回调与扣费同事务
Balance(userID) (int64, error)
```

- `Debit` 把"扣余额 + 写流水 + 执行业务(biz 回调)"放进**同一个 DB 事务**,任一失败整体回滚 → 不会"扣了钱业务没成"。
- 扣减前 `SELECT ... FOR UPDATE` 行锁,余额不足返回 `errs.ErrInsufficient`(code 5001)。
- scene 枚举:`recharge / chat / unlock / gift / reward`,全部进 `wallet_txn` 流水可对账。

## 三、pay 适配层(driver 模式)

```go
type Driver interface {
    Name() string
    Prepay(order, payerID) (payParams, error)        // 平台下单
    VerifyCallback(r) (*CallbackResult, error)        // 验签 + 解密 → 归一化
    SuccessResponse() (contentType, body)             // 平台要求的应答报文
}
```

- 微信、支付宝各一个 driver;`pay.Service` 持有 map,按登录平台路由。
- **微信 APIv3 已真实实现**(`driver_wx.go` + `wxcrypto.go`):
  - 下单:JSAPI `/v3/pay/transactions/jsapi`,商户私钥 RSA-SHA256 签 `Authorization`,拿 prepay_id 后生成小程序 paySign。
  - 回调:平台公钥验签 `时间戳\n随机串\nbody\n` + APIv3 密钥 AES-256-GCM 解密 resource;防重放(时间戳±5min)。
- **支付宝 driver 目前为骨架**(`alipay.trade.create` + RSA2 验签待补)。
- 无凭证(dev)时 driver 回退 mock,本地可全链路联调。

## 四、回调入账幂等(钱的安全核心)

```
HandleCallback:
  验签 → 查 order_no
    已 paid? → 直接返回成功(幂等,不重复加币)
    金额校验(平台回传 total == 订单 price_fen)
    单事务{ order.status pending→paid (乐观更新, RowsAffected==0 则已被处理)
            + wallet.Credit(coins) + wallet_txn(credit, recharge, bizNo=order_no) }
```

**红线:加币只在平台异步回调内发生;前端"支付成功"仅作 UI 提示,绝不据此加币(可伪造)。**

## 五、配置化价格(运营热调)

开聊几币 / 解锁几币 / 注册奖励 / 是否强制认证 / iOS 是否隐藏充值,全在 `config` 表(`sysconfig` 包带默认值 + 缓存),改值不发版。

## 六、已验证(dev,远程库)

充值闭环:余额 50 → 下单 pkg1(60币)→ 回调 SUCCESS → 110;**重复回调仍 110(幂等)**;流水 `credit 60 recharge | credit 50 reward`。消费:解锁 −2、开聊 −5,余额 43 精确。

## 六·5、付费标签与金币口径（2026-07-21）

- **付费用户自动标签**：`HandleCallback` 入账事务内一条幂等 SQL 给 `users.tags` 追加"付费用户"(`NOT LIKE` 去重)，与订单 pending→paid 幂等同步。
- **`total_recharged` 口径**：`CreditTx` 对**所有** credit(充值/签到/奖励)都累加，故它是"历史累计获得"而非纯充值。后台"总金币"用它、"剩余金币"用 `balance`；要纯充值额需另按流水 `scene=recharge` 汇总。

## 七、多租户改造(已实现)

- **driver-per-tenant**:`pay.driverFor(tenantID, platform)` 按租户构造/缓存 driver;凭证来源——多租户从 `credstore`(解密 PEM),单租户从 `.env`(读密钥文件)。WxDriver 改为接收 `WxCreds`(PEM 文本)而非文件路径。
- **回调带租户**:路由 `/pay/callback/:platform/:tenantId`;下单时按租户 `notify_url` 写入,回调直接拿 tenantId 选验签凭证。
- **钱包/订单带 tenant_id**:`wallet.Credit/Debit(tenantID, userID, …)` 入账与流水均写 `tenant_id`;`pay_order.tenant_id` 用于回调入账与隔离。
- 入账复用 `wallet.CreditTx(tx, tenantID, …)`(回调事务内,免嵌套事务)。详见 `l1/tenant-saas.md`。
