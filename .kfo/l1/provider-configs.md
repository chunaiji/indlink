# L1 设计 — 服务商配置(provider)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-10-07 |
| 覆盖模块 | `internal/provider`(`schema.go` / `store.go` / `migrate.go` / `migrate_sso.go`)+ 支付 / 地图 / 内容安全 / 登录四域的适配器 + 后台「服务商」四页 |
| 关联 | L1 `l1/pay-wallet.md` · `l1/user-auth.md` · `l1/tenant-saas.md` · `l1/admin-platform.md` · L2 [2026-10-06-provider-configs](../l2/2026-10-06-provider-configs.md) [2026-10-07-sso-provider-configs](../l2/2026-10-07-sso-provider-configs.md) |

---

## 一、职责

所有**第三方服务商的凭据与开关**的唯一真相源。此前它们分散在 `app_credentials` 表(支付)
与散落的 sysconfig 键(地图、审核、登录),加一家服务商要动四个地方。

> **`app_credentials` 从此只管「登录小程序/App 用的 appid+secret」**(与登录同源),
> 支付列只读不删(历史回滚用),`wx_app` / `alipay_app` 两行已不再被读。

## 二、数据模型

`provider_configs` 表,**一行 = 租户 × 域(kind) × 服务商(provider)**:

| 列 | 说明 |
|---|---|
| `tenant_id` | 多租户隔离,所有查询必过滤 |
| `kind` | `pay` / `map` / `moderation` / `sso` |
| `provider` | `wechat` / `alipay` / `apple` / `google_play` / `google` / `tencent` / `amap` / `baidu` |
| `enabled` · `active` | 启用;`active` 仅对单选域有意义 |
| 字段值 | 整体作为 **AES 加密的 JSON blob** |

## 三、声明式 schema(`schema.go`)

每家服务商一个 `Definition`,描述它有哪些字段、哪些必填、哪些是机密、属于哪个平台。

```
Definition{ Kind, Provider, LabelZh, LabelEn, Platform, Fields[], DependsOn }
```

- `Platform`:`PlatformBoth` / `PlatformApp` —— 决定后台按租户类型过滤后看不看得到。
- `Secret: true` 的字段:列表接口只回 `"set"` / `""`,**写空串 = 保持不变**(与 `/config` 同规则,见 `store.go` 的 `mergeFields`)。
- `DependsOn`:声明「我的凭证住在另一张卡片上」。目前只有 `sso/alipay` → `pay/alipay`(登录与支付同一个应用、同一把私钥)。**指向不存在的卡片时 `Missing` / `Usable` 不能 panic**,否则后台服务商页与 `/app-config` 一起挂。
- `Usable()` 回答的是统一的一个问题:**启用 且 必填齐全**(含 `DependsOn` 那张卡片的必填)。

> **加一家服务商 = 加一个 `Definition` + 对应域的一个适配器。**

## 四、四个域

| 域 | 服务商 | 选择方式 | 消费方 |
|---|---|---|---|
| `pay` | 微信支付 / 支付宝 / Apple IAP / Google Play | 并存,客户端选渠道 | `pay.buildDriver` |
| `map` | 腾讯 / Google / 高德 / 百度 | **单选**(`SingleActive`) | `GET /geo/regeo` |
| `moderation` | 微信 / 支付宝 | **单选** | `moderation` |
| `sso` | 微信 / 支付宝 / Google / Apple | 并存,全部 `PlatformApp` | `/app-config` 的 `auth` 段 + `user` 登录链路 |

**单选域的互斥在事务内做**:写入 `active=true` 前先把同域其它行置 `false`,`Active()` 任何时刻最多返回一行。

## 五、渠道名归一

统一为 `wechat` / `alipay` / `apple` / `google_play`。历史订单里的 `wx` / `wx_app` / `alipay_app` / `ios`
经 `canonicalChannel` 折回,**旧回调路径仍可达**。

⚠️ 微信回调按 `Wechatpay-Serial` 查租户时,**同一商户号服务两个租户会同时命中两行** ——
命中后必须用解密出的 `mch_id` 再核对,不能取第一个。

## 六、迁移

启动时一次性把旧值搬进新表,幂等(目标行已存在不覆盖),完成后写标记:

| 标记 | 搬的东西 |
|---|---|
| `provider_migrated` | 支付 / 地图 / 内容安全(2026-10-06) |
| `sso_migrated` | 四家登录(2026-10-07) |

⚠️ **两个标记必须独立**。线上 `provider_migrated` 早已是 `"1"`,SSO 复用它等于迁移永不执行,
每个租户都会变成「没有任何登录配置」。
⚠️ 迁移跑第二遍不得覆盖运营在新页面手工改过的值;**没有来源的渠道不建卡片**,否则后台永远挂一个红色「缺 1 项」。

## 七、后台

侧边栏「服务商」区四页(支付 / 地图 / 内容安全 / 登录),共用一个通用卡片组件 + `/providers/:kind` 接口。
中英文标签在 `Definition` 里内联,不走 `meta.go` 白名单。
