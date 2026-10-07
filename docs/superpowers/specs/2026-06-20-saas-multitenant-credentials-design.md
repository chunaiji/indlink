# SaaS 多租户 + 凭证表设计

> 版本:草案 · 日期:2026-06-20 · 状态:IMPLEMENTED(已实现,见 `.kfo/l2/2026-06-20-saas-multitenant-impl.md`)
> 目标:从"单 .env 一套 appid"演进为"一份部署服务多个小程序",凭证集中入表,微信/支付宝双平台兼容。

---

## 一、决策摘要

| 维度 | 结论 |
|---|---|
| 部署模型 | **一份部署多租户**:一套后端服务多个小程序,数据按 `tenant_id` 隔离 |
| 租户标识 | **appid**(前端运行时自取并在登录时带上;appid 公开,非密) |
| 凭证存储 | 集中 `app_credential` 表;**密钥字段加密入库**(env 主密钥 AES-GCM) |
| 双平台 | `platform` 字段区分;driver 层不变,凭证来源由 cfg 改 tenant |
| 迁移 | 开关 `MULTI_TENANT_ENABLED`;关=沿用 .env 单租户(不影响当前联调) |
| 本轮范围 | **仅设计**;实现下轮 |

---

## 二、租户识别链路(crux)

多租户的核心难点:登录换 openid 需要 appid+secret,**后端必须先知道是哪个租户**才能选对凭证。解法:登录请求带 appid。

```
前端 getAppId()                          // 微信 wx.getAccountInfoSync().miniProgram.appId
                                         // 支付宝 my.getAppIdSync()
  → POST /auth/login { platform, appid, code }
后端:
  (platform, appid) → 查 app_credential → 取 secret(解密)
  → code2session(该 appid/secret) → openid
  → appid 所属 tenant_id → 找/建用户(tenant_id + openid 复合唯一)
  → 签 JWT { uid, tenant_id, platform }
后续请求:JWT 带 tenant_id(无需再传 appid);中间件注入 ctx
支付下单:按 JWT.tenant_id 取该租户支付凭证 → 构造对应 driver
```

- appid 仅登录必传;登录后靠 JWT.tenant_id。
- 可选:所有请求带 `X-App-Id` 头,中间件校验与 JWT.tenant_id 一致(防御性,非必须)。

## 三、凭证表结构

```
tenant
  tenant_id (PK) · name · status(active/frozen) · created_at

app_credential                                  -- 一行 = 一个小程序应用
  id (PK)
  tenant_id (FK, index)
  platform           -- wx / alipay
  appid              -- UNIQUE(platform, appid)
  secret_enc         -- 登录密钥(加密)
  -- 微信支付
  mch_id
  pay_apiv3_key_enc
  pay_serial_no
  pay_private_key_enc
  pay_platform_key   -- 平台公钥(公开,可不加密)
  pay_platform_serial
  -- 支付宝
  alipay_private_key_enc
  alipay_public_key  -- 公开
  notify_url
  status · created_at · updated_at
```

- `UNIQUE(platform, appid)`:一个 appid 全局唯一定位一条凭证。
- 同一 `tenant_id` 可有 wx + alipay 两行(同产品双端)。
- `*_enc` 字段密文存储(见第六节)。

## 四、数据隔离

- **所有业务表加 `tenant_id`**:user / bottle / chat / message / relation / wallet / wallet_txn / pay_order / match_log / report / block / item_order 等。
- **用户唯一键**:`wx_openid` → `UNIQUE(tenant_id, wx_openid)`、`(tenant_id, alipay_uid)`。同一微信号在不同小程序是不同账号、不同钱包。
- **查询强制带租户**:中间件把 `tenant_id` 注入 ctx;repository 层统一拼 `WHERE tenant_id = ?`。约定:**任何业务查询不带 tenant 过滤 = bug**。
- 索引同步加 `tenant_id` 前缀(如 `(tenant_id, status, expire_at)`)。
- feed key 改 `user_feed:{tenantId}:{uid}`;Redis 限流/在线键同样加 tenant 前缀。

## 五、凭证加载与 driver-per-tenant

- `config` 不再存 appid/secret/商户号;改由 `credstore` 包启动加载 + 缓存 `app_credential`(按 `(platform, appid)` 与 `tenant_id` 双索引;TTL + 手动 `Reload`)。
- `pay` / `user` 的 driver 从"全局单例"改为"**按租户构造/缓存**":`driverFor(tenantId, platform) → Driver`;凭证从 credstore 取(解密后)。
- 微信支付 APIv3 实现(`wxcrypto.go` 等)不变,只是私钥/密钥来源从 cfg 改 credstore。

## 六、密钥安全

- 敏感字段(`secret` / `pay_apiv3_key` / 各私钥)以 **AES-256-GCM 加密入库**;主密钥 `CRED_MASTER_KEY` 放环境变量(不进库、不进 git)。
- 公开字段(平台公钥、appid、mch_id、notify_url)可明文。
- 比原 .env 明文更安全:DB 泄露不直接暴露密钥;主密钥与数据分离。
- 后续可平滑升级为 KMS / Secrets Manager(只换 `encrypt/decrypt` 实现)。

## 七、双平台兼容

- 平台差异仍封装在 driver 层(已实现微信 APIv3;支付宝待补)。
- 前端 `utils/platform.js` 加 `getAppId()` 条件编译;登录参数加 `appid`。
- 凭证表 `platform` 区分两端;同租户两行各自独立配置。

## 八、迁移路径(平滑、不破坏联调)

1. 建 `tenant` / `app_credential` 表 + 各业务表 `tenant_id` 列(默认值给"默认租户")。
2. 种子:默认租户 + 用现有 .env 的 appid/secret 建一条 `app_credential`。
3. 开关 `MULTI_TENANT_ENABLED`:
   - **关(默认,当前联调)**:沿用 .env 单租户,`tenant_id` 恒为默认租户,登录不强制 appid。
   - **开**:走凭证表 + appid 解析 + 全链路 tenant 过滤。
4. 灰度:先开关闭联调,功能稳定后开开关接第二个小程序验证。

## 九、对现有代码的影响(实现时)

| 模块 | 改动 |
|---|---|
| model | 全业务表加 `tenant_id`;用户复合唯一键 |
| middleware | JWT claims 加 `tenant_id`;注入 ctx;repository helper 强制过滤 |
| user | 登录加 appid 解析租户;建用户带 tenant_id |
| pay/wallet | driver-per-tenant;pay_order/wallet 带 tenant_id |
| bottle/chat/match/... | 查询全部加 tenant 过滤;Redis key 加 tenant 前缀 |
| config | 拆出 credstore;加 `CRED_MASTER_KEY` / `MULTI_TENANT_ENABLED` |
| 新增 | `internal/tenant`(表+credstore)、`internal/crypto`(AES-GCM 主密钥) |

## 十、测试策略

- 单测:credstore 加解密往返;driverFor 缓存与隔离;租户过滤 helper。
- 接口测试扩展:两个租户(两个 appid)各登录 → 互不可见对方瓶子/会话/钱包(隔离断言);跨租户访问应拒。
- 回归:开关关闭时现有 40 用例不变。

## 十一、待确认 / 风险

- 租户开通流程(后台管理界面 / 运营建租户 + 填凭证)——本设计只覆盖运行时,管理端另议。
- 跨租户唯一性:nickname/瓶子可重名(隔离后天然 OK)。
- 历史单租户数据迁移到默认租户的脚本(开关切换时一次性回填 tenant_id)。
- 支付回调定位租户:**采用带租户的回调路径** `/pay/callback/wx/:tenantId`(`notify_url` 本就按 credential 配置,下单时写入该租户专属回调地址)→ 回调直接拿到 tenantId 选对验签凭证,无需反查。
