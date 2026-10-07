# 服务商配置独立化(支付 / 地图 / 内容安全)

> 2026-10-06 · L2 · 关联 [l1/provider-configs](../l1/provider-configs.md) [l1/pay-wallet](../l1/pay-wallet.md) [l1/tenant-saas](../l1/tenant-saas.md) [l1/admin-platform](../l1/admin-platform.md)
> 原始文档:`docs/superpowers/specs/2026-10-06-provider-configs-design.md` · `docs/superpowers/plans/2026-10-06-provider-configs.md`
> 实施 commit:`9b2c0ff`
> 状态:**COMPLETED**

## 1. 问题

第三方服务商的凭据过去分两处:`app_credentials` 表(支付)与散落的 sysconfig 键(地图、审核)。
**加一家服务商要同时动模型、凭证存储、配置白名单和 Vue 页面四处**,且同一件事有两个真相源。

## 2. 做法

新包 `internal/provider`:**声明式 schema**(`schema.go` 的 `Definition`)+ 加密存储 + 带索引的缓存
+ 启动时一次性迁移。数据落 `provider_configs` 表,**一行 = 租户 × 域 × 服务商**,字段整体作为
AES 加密的 JSON blob 存。

> **加一家服务商 = 加一个 `Definition` + 对应域的一个适配器**,其余不动。

四个域(`KindPay` / `KindMap` / `KindModeration`,SSO 见 [2026-10-07-sso-provider-configs](2026-10-07-sso-provider-configs.md)):

| 域 | 服务商 | 选择方式 |
|---|---|---|
| 支付 | 微信支付 / 支付宝 / Apple IAP / Google Play | 并存,客户端选渠道 |
| 地图 | 腾讯 / Google / **高德(新)** / **百度(新)** | 单选(`SingleActive`) |
| 内容安全 | 微信 / **支付宝(新)** | 单选 |

新增 `POST /pay/play/verify`(Google Play 服务端校验)。支付宝内容安全走
`alipay.security.risk.content.sync.detect`,请求形状取自线上可用实现而非记忆。

## 3. 兼容线上数据(全部是加法)

- **渠道名统一**为 `wechat` / `alipay` / `apple` / `google_play`;历史订单里的 `wx` / `wx_app` / `alipay_app` / `ios` 经 `canonicalChannel` 折回,**旧回调 URL 仍可达**。
- `app_credentials` 的支付列**只读不删**,凭证页去掉支付字段(保证单一真相源)。
- 启动时一次性迁移搬值,完成后写全局键 `provider_migrated=1`;**12 个被取代的 sysconfig 键从代码里删除**。
- 迁移幂等:目标行已存在不覆盖。

## 4. 评审时盯住的坑

1. **直辖市**:百度/高德返回的 `city` 可能是**空数组**而非字符串,解析不能炸,退回 `province`。
2. **微信回调 `Wechatpay-Serial` 同时命中两个租户**(同商户号服务两租户):按 serial 命中后还要用解密出的 `mch_id` 核对,不能取第一个。
3. **单选域并发写**:事务内先把同域其它行 `active=false` 再存本行,`Active()` 任何时刻最多返回一行。
4. **Google Play `purchaseState=1/2`**(已退款/待处理)不得入账,要返回明确错误。
5. **迁移时 `wx` 与 `wx_app` 两行都有商户号**:同租户只能留一行 `pay/wechat`,按租户类型取,另一行丢弃并打日志。

## 5. 后台

侧边栏新增「服务商」区,三个卡片页共用一个通用组件 + `/providers/:kind` 接口。
机密字段列表接口只回 `"set"` / `""`,**写空串 = 保持不变**(与 `/config` 同规则)。
租户页顺带支持新建凭证行、设置 App 租户的 `APP_ID`。

`app_pay_mock_enabled` 仍留在 sysconfig「支付联调」分组不动(⚠️ 上线必须为 0)。
