# 国内版 App(`app/bottles_zh`):微信 / 支付宝 登录与支付

> 2026-10-06 · L2 · 关联 [l1/user-auth](../l1/user-auth.md) [l1/pay-wallet](../l1/pay-wallet.md) [l1/tenant-saas](../l1/tenant-saas.md)
> 原始文档:`docs/superpowers/specs/2026-10-06-app-zh-wechat-alipay-design.md` · `docs/superpowers/plans/2026-10-06-app-zh-wechat-alipay.md` · `docs/Android-WeChat-SSO-Integration.md`
> 实施 commit:`eb922d6`
> 状态:**COMPLETED**(代码通;**未提审**,待正式凭证)

## 1. 形态

`app/bottles` 的**国内分支**,不是开关:包名 / Bundle ID `com.ambertu.bottles.cn`,`APP_ID` 默认
`drift_app_cn`(独立租户),显示名「漂流瓶」。**海外版 `app/bottles` 一个字不改。**

| 平台 | 登录 | 充值 |
|---|---|---|
| Android | 微信 / 支付宝 | 微信支付 / 支付宝 |
| iOS | 微信 / 支付宝 / **Apple**(Google 登录则必须有 Apple,见合规项) | **IAP**(苹果规则,数字商品只能走内购) |

依赖:`fluwx ^6.0.4`(微信)、`tobias ^5.3.4`(支付宝)。

## 2. 服务端:全部是加法

- `app_credentials` 加平台 `wx_app` / `alipay_app`(**此后被服务商化取代**,见 [2026-10-07-sso-provider-configs](2026-10-07-sso-provider-configs.md))。
- `pay.Driver` 多两种实现(微信 APP 交易类型、支付宝 App 支付),并新增可选 `Querier` 做主动查单。
- 微信 / 支付宝登录并入既有 `loginOrCreateOAuth` 链路,不另开一条。

## 3. 客户端:只换两处

`oauth.dart` 与两个 `PayChannelAdapter`。**支付六屏状态机与登录状态机一行不动** —— 这是
2026-09-21 支付链路设计时留下的扩展点第一次兑现。

## 4. 签名与金额的硬细节(错一处就验签失败)

| 项 | 规则 |
|---|---|
| 微信 APP 拉起参数 | 签名串**第四行是裸 `prepay_id`** |
| 支付宝签名内容 | 值**不** URL 编码;输出 `orderStr` 时值**要** URL 编码 |
| 支付宝通知验签 | 剔除 `sign` 与 `sign_type` |
| 金额单位 | 支付宝是**元字符串两位小数**(1 分 → `"0.01"`),服务端订单金额是**分** |

⚠️ `total_amount` 带小数 / 千分位 / 空串时**必须拒收**,不能解析失败当 0 通过金额校验。

## 5. 其它约束

- 客户端轮询与渠道异步回调可能同时到达:`HandleCallback` 的 `WHERE status='pending'` 已幂等,`SyncIfStale` **不得绕过它另开入账口**。
- 用户在微信里取消后又点「已完成支付」:页面只是再查一次服务端,**渠道的取消不是终态**。
- 小程序租户误传 `channel` 必须 400。
- 凭证后台改了没重启:驱动缓存按 credstore `Version()` 失效。
