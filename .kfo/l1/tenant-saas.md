# L1 设计 — SaaS 多租户 + 凭证表(IMPLEMENTED)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-09-22 |
| 状态 | **IMPLEMENTED — 已实现并通过隔离测试**(开关 `MULTI_TENANT_ENABLED` 默认关=单租户) |
| 覆盖 | `internal/tenant`(credstore)、`internal/crypto`;user/pay/wallet/中间件/全业务表均已改造 |
| 关联 | spec `docs/superpowers/specs/2026-06-20-saas-multitenant-credentials-design.md` · L2 `l2/2026-06-20-saas-multitenant-impl.md` · L0 `l0/architecture.md` · L1 `l1/user-auth.md` `l1/pay-wallet.md` · L4 `l4/cross-platform-api-patterns.md` |

---

## 目标

从"单 .env 一套 appid"演进为**一份部署服务多个小程序**(多租户),凭证集中入表,微信/支付宝双平台兼容。

## 核心决策

| 维度 | 结论 |
|---|---|
| 部署模型 | 一份部署多租户,数据按 `tenant_id` 隔离 |
| 租户标识 | **appid**(前端运行时自取,登录时带上;appid 公开非密) |
| 凭证存储 | `app_credential` 表;密钥字段 AES-256-GCM 加密(env 主密钥 `CRED_MASTER_KEY`) |
| 双平台 | `platform` 字段区分;driver 层不变,凭证来源 cfg→credstore |
| 迁移 | 开关 `MULTI_TENANT_ENABLED`;关=沿用 .env 单租户(不破坏当前联调) |

## 租户识别链路(crux)

登录换 openid 需 appid+secret → 后端必须先知租户。解法:**登录带 appid**。

```
前端 getAppId()(wx.getAccountInfoSync / my.getAppIdSync)
  → POST /auth/login { platform, appid, code }
后端:(platform,appid)→app_credential→secret(解密)→code2session→openid
     → tenant_id → 用户按 (tenant_id, openid) 复合唯一 → JWT{uid,tenant_id,platform}
后续:JWT 带 tenant_id;支付按 tenant_id 取凭证建 driver;回调走 /pay/callback/wx/:tenantId
```

## 表结构(摘)

- `tenant(tenant_id, name, status, type)`
- `app_credential(id, tenant_id, platform, appid UNIQUE(platform,appid), secret_enc, 微信支付字段…, 支付宝字段…, notify_url, status)`
- 同租户可有 wx + alipay 两行。

### `Tenant.Type`(2026-09-21)

`miniprogram` / `app` 二选一,**互斥**——一个租户只服务一个端。

```go
Type string `gorm:"size:16;not null;default:miniprogram" json:"type"`
```

`not null + default` 是有意的:AutoMigrate 加这一列时 MySQL 会把存量行一并回填成 `miniprogram`,不需要迁移脚本(线上 8 个租户已验证,零 NULL)。

用途:**后台配置按端过滤**(见 `l1/admin-platform.md` §三)。因为类型互斥,同一个配置键在小程序租户里存微信值、在 App 租户里存 Google/Apple 值也不冲突——冲突的只有那些 **defaults 本身带端偏向**的键,那批必须拆开(同上)。

连带:App 租户建号时**只校验名称**,微信 appid/secret 那对只在类型为小程序时才必填(`admin/tenant.go` 的纯函数校验器)。之前不分类型一律要求凭证,导致 App 租户根本建不出来。

## 数据隔离

- 全业务表加 `tenant_id`;用户唯一键 `(tenant_id, wx_openid)`/`(tenant_id, alipay_uid)`(同微信号跨小程序=不同账号/钱包)。
- 中间件注入 tenant_id,repository 强制 `WHERE tenant_id=?`(不带 = bug)。
- Redis 键加 tenant 前缀(`user_feed:{tid}:{uid}` 等)。

## 对现有 L1 的影响(回灌提示)

- `l1/user-auth.md`:登录新增 appid 解析租户;建号带 tenant_id;复合唯一键。**实现时回写该文档。**
- `l1/pay-wallet.md`:driver 由全局单例改 `driverFor(tenantId, platform)`;pay_order/wallet 带 tenant_id;回调路径带 tenantId。**实现时回写。**
- `l1/bottle-feed.md` / `l1/chat-ws.md` / `l1/match.md`:查询与 Redis 键加租户维度。

## 实现清单(均已完成 ✅)

1. ✅ 表 + 各业务表 tenant_id 列 + 默认租户种子(`bootstrap.Seed(db, DefaultTenantID)`)
2. ✅ `internal/crypto`(AES-GCM 主密钥)+ `internal/tenant`(credstore,按 appid/租户缓存解密)
3. ✅ user 登录 `Login(platform, appid, code)` + JWT `tenant_id` + `middleware.TenantID(c)`
4. ✅ `pay.driverFor(tenantID, platform)` driver-per-tenant + 回调路径 `/pay/callback/:platform/:tenantId`
5. ✅ 全业务查询/写入加租户过滤 + feed Redis 键 `user_feed:{tid}:{uid}`
6. ✅ 隔离测试通过:跨租户捞瓶不可见、详情拦截(1005)、未知 appid 拒(2002);开关关闭 40 用例功能回归不变

## 实现关键点(与设计的偏差/补充)

- 用户唯一性用 **app 层 find-or-create**(按 `tenant_id+openid`)而非 DB 复合唯一索引,规避空 openid(alipay 用户无 wx_openid)的唯一冲突。
- 单租户(flag off)余额/钱包查询按 `user_id`(雪花全局唯一)即可,租户过滤主要用于内容/列表类表。
- dev 凭证可留空 secret(`wxCode2Session` 回退 mock),空 `*_enc` 字段无需主密钥即可解密为空。

## 待确认(spec §十一)

- ~~租户开通/管理端(运营建租户填凭证)另议~~ —— **已实现**:后台「租户」页建号/改状态/选类型,「租户凭证」页维护 `app_credential` 行。<br>凭证同源很关键:access_token 的 appid/secret **优先取租户凭证表**,sysconfig 里的推送分组只是后备——新租户配一次凭证,登录/推送/内容安全全通(`push.getAccessToken`)。
- 历史单租户数据回填 tenant_id 脚本
- KMS 升级路径(换 encrypt/decrypt 实现)。注意 sysconfig 那侧的机密**目前完全没加密**(明文落库,只做了传输层脱敏),与本表的 AES-GCM 不是一回事。
