# L1 设计 — 用户 / 双平台登录 / 认证(user)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-10-07 |
| 覆盖模块 | `internal/user/oauth.go`、`service.go`、`handler.go` |
| 关联 | L0 `l0/architecture.md` · L1 `l1/pay-wallet.md`(注册奖励/openid)· L4 `l4/cross-platform-api-patterns.md` |

---

> **多租户已落地(2026-06-20)**:登录入参增加 `appid`,JWT 增加 `tenant_id`,建号按 `(tenant_id, openid)`。详见本文末「多租户改造」与 `l1/tenant-saas.md`。

## 一、登录:一套接口适配双平台

```
POST /auth/login { platform, appid, code }
  platform=wx     → wxCode2Session(code)    → openid(+unionid)
  platform=alipay → alipayCode2UID(code)     → alipay user_id
  按 wx_openid / alipay_uid 找用户:存在则更新 last_active_at;不存在则建号
  签 JWT(uid, platform, 168h) 返回 { token, user, is_new, ios_recharge_off }
```

- **dev 回退**:未配平台凭证时,`wxCode2Session` 返回 `wxdev_<code>`、`alipayCode2UID` 返回 `alidev_<code>` → 任意 code 可登录,本地/接口测试无需真实平台。
- 微信正式:调 `https://api.weixin.qq.com/sns/jscode2session`(appid+secret+js_code)。支付宝正式:`alipay.system.oauth.token`(目前骨架,标 TODO(real))。

## 二、建号 + 注册奖励

```
新用户:user_id=雪花, nickname="用户%06d", anonymous_level=1, status=active
       wx 存 wx_openid(+unionid);alipay 存 alipay_uid
注册奖励:wallet.Credit(uid, config.reg_reward_coins, scene=reward, biz="register:<uid>")
```

奖励币数走 `config`(默认 50),对应前端"系统消息:注册奖励"。biz 用 `register:<uid>` 保证幂等可追溯。

## 三、JWT 鉴权

- `Authorization: Bearer <token>`,中间件解析后把 `uid`/`platform` 注入 gin context。
- 缺/失效 token → **HTTP 401 + code 2001**(与业务错误 HTTP200+code≠0 区分,见 `l4`)。
- `platform` 进 claims:充值下单时据此选支付渠道(`pay` 模块复用)。

## 四、认证开关(可配置门槛)

```
EnsureVerifiedIfRequired(uid):
  config.verify_required == false → 直接放行
  == true 且 !user.is_verified   → 返回 ErrNeedVerify(code 2003)
```

- 被 `bottle.Create`(发瓶)与 `chat.StartChat`(开聊)调用 → 开关开启时未认证不能发瓶/开聊。
- `POST /user/verify`:V1 简化为直接置 `is_verified=true`;正式需接实名+真人头像审核(待办)。

## 五、资料与对外能力

| 接口/方法 | 用途 |
|---|---|
| `GET /user/profile` | 我的资料 |
| `POST /user/update` | 改昵称/头像/性别/年龄/城市(指针字段,选择性更新) |
| `POST /user/verify` | 提交真人认证 |
| `GetWxOpenID(uid)`(内部) | 供 `pay` 微信下单取 payer openid |

## 六、双平台账号策略(V1 取舍)

- **V1:两端各算独立账号**,钱包随 `user_id`。微信侧已捕获 `unionid` 为后续合并留口。
- 待办:用 `unionid` / 手机号绑定合并同一自然人(避免同人两端两个钱包)。

## 七、关键约束

- `wx_openid` / `alipay_uid` / `union_id` 显式列名(GORM 默认会把 `WxOpenID`→`wx_open_id`,曾导致 `Unknown column`,见 L3 间接相关教训)。
- `user_id` 等 ID 字符串序列化(`json:",string"`),前端不丢精度。
- `anonymous_level`(1~5)字段已留,V1 未深用(逐步解锁头像/资料为 V2 玩法)。

## 八、多租户改造(已实现)

- `Login(platform, appid, code)`:`resolveLogin` 决定租户与登录凭证——
  - **多租户开**:`credstore.ByAppID(platform, appid)` → 取该租户 `appid/secret`;未知 appid 返回 2002。
  - **关(默认)**:`tenant_id = DefaultTenantID`,用 `.env` 凭证(appid 可空)。
- `wxCode2Session(appid, secret, code)` 参数化(不再读全局 cfg);建号写 `tenant_id`;按 `(tenant_id, openid)` find-or-create。
- 签发 JWT 带 `tenant_id`;`GetWxOpenID` 供 pay 下单。详见 `l1/tenant-saas.md`。

## 九、App 登录与第三方登录服务商化(2026-10-07)

见 `l1/provider-configs.md` 与 L2 [2026-10-07-sso-provider-configs](../l2/2026-10-07-sso-provider-configs.md)。

- **四家登录(微信 / 支付宝 / Google / Apple)的凭证与开关搬进 `provider_configs` 的 `sso` 域**。
  `app_credentials` 从此**只管登录用的 appid+secret**,`wx_app` / `alipay_app` 两行不再被读。
- `/app-config` 的 `auth` 段按统一标准下发四个布尔:**启用 且 必填齐全**。
  此前三套答案(微信查开关+凭证行、支付宝查错了凭证行、Google/Apple 根本没查)已废。
- **支付宝登录的密钥不在登录卡片上**:它与支付同一个应用、同一把私钥,登录卡片用 `DependsOn`
  声明依赖 `pay/alipay`。支付那边缺密钥 → 登录页直接不显示支付宝按钮。
- `google_client_id` 是逗号分隔多值,第一个按约定是 Web client ID,下发只取第一个。
- 已删键:`app_login_wechat_enabled` / `app_login_alipay_enabled` / `app_google_client_id` /
  `app_apple_bundle_id` / `app_wechat_universal_link`。迁移标记 **`sso_migrated`**(不是 `provider_migrated`)。
- App 登录方式开关:`app_login_phone_enabled` / `app_login_email_enabled`(默认 1),
  经 `/app-config` 下发 `auth.phone` / `auth.email`;**两个都关按两个都开处理**,避免锁死所有人。

### 国内版 App(`app/bottles_zh`,2026-10-06)

独立租户 `drift_app_cn`。微信 / 支付宝登录并入既有 `loginOrCreateOAuth` 链路,不另开一条;
iOS 保留 Apple 登录(有 Google 登录就必须有 Apple)。详见 L2 [2026-10-06-app-zh-china-build](../l2/2026-10-06-app-zh-china-build.md)。

### 资料字段(2026-10-04)

`User.Birthday`(YYYY-MM-DD),**年龄服务端算**,只在本人资料里返回;最小年龄取 `app_profile_min_age`。
**性别只能设一次**,再改返回业务错误并透传到前端。
`/user/profile`、`/user/update`、App 登录响应共用同一个 self DTO(含 `bottle_count` / `moment_count`)。
