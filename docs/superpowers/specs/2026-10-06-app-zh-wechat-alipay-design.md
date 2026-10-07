# 国内版 App（`app/bottles_zh`）微信 / 支付宝 登录与支付 设计

> 日期：2026-10-06
> 范围：Go 后端（加法）、国内版 Flutter 工程 `app/bottles_zh`、管理后台「租户凭证」页
> 前置文档：`2026-09-21-app-payment-flows-design.md`（六屏支付状态机与 `PayChannelAdapter`）、
> `2026-06-20-saas-multitenant-credentials-design.md`（`app_credentials` 与 credstore）

---

## 一、目标与现状

### 1.1 要解决什么

`app/bottles_zh` 要上线中国境内应用市场。海外版的 Google / Apple 登录与 Play / IAP 充值在国内不可用，
需要换成：

| 能力 | Android | iOS |
|---|---|---|
| 登录 | 微信、支付宝、手机号 | 微信、支付宝、手机号、**Apple**（App Store 4.8：有第三方登录就必须有 Apple 登录） |
| 充值 | **微信支付 App 支付**、**支付宝 App 支付** | **IAP**（3.1.1：虚拟币不允许第三方支付；现有 StoreKit 链路不动） |

账号（微信开放平台移动应用、微信支付商户号、支付宝开放平台应用）由用户自行申请。
本设计的目标是：**账号一到手，后台填进去就能跑**，开发不等账号。

### 1.2 现状

- `app/bottles_zh` 是 2026-10-06 从 `app/bottles` 整目录复制出来的，逐字节相同（418 个文件），尚未 git 跟踪。
- 服务端支付：`pay.Driver` 接口（Prepay / VerifyCallback / SuccessResponse）；微信 JSAPI 驱动完整（APIv3 签名、回调验签、AEAD 解密）；
  支付宝驱动只有壳（下单与验签都是 `TODO(real)`）；mock 渠道与幂等入账 `HandleCallback` 可复用。
- 服务端登录：App 端 Google / Apple 走 `loginOrCreateOAuth(appIdentity)`；小程序微信走 `wxCode2Session`；
  支付宝 `alipayCode2UID` 是 TODO。`User` 表已有 `wx_openid` / `alipay_uid` / `union_id` 列。
- 凭证：`app_credentials` 按 `(tenant, platform)` / `(platform, appid)` / `pay_platform_serial` 三索引缓存，字段 AES-GCM 加密；
  平台取值只有 `wx` / `alipay` / `app`（后者只做 appid→租户映射）。后台「租户凭证」页**只能改不能新增**。
- App 端：六屏支付状态机 + `PayChannelAdapter` 抽象已就位，页面只注入了 `MockChannelAdapter`；
  `core/platform/oauth.dart` 把 Google / Apple SDK 关在一个文件里。
- 生产多租户（`MULTI_TENANT_ENABLED=1`），海外 App 租户 `358804313465688064`。

### 1.3 不在本设计范围

- Google Maps → 腾讯地图（国内版地图另开任务）。
- 应用市场合规（隐私弹窗文案、SDK 清单、备案号展示）。
- 微信 / 支付宝的**退款**与**对账单下载**。退款仍由后台手工在商户平台操作，`refunded` 状态保持人工回填。
- 小程序端（`client/`）的支付宝正式下单：驱动实现后小程序自然受益，但小程序侧联调不在本次。

---

## 二、决策记录

| # | 决策 | 理由 |
|---|---|---|
| D1 | 国内版用**新租户**（后台新建，`app_credentials` 加一行 `platform=app, appid=drift_app_cn`） | 国内用户与海外用户两个池子；CNY 档位、凭证、文案各配各的，互不牵连 |
| D2 | iOS 国内版保留 Apple 登录与 IAP，微信 / 支付宝支付只在 Android 出现 | App Store 4.8 / 3.1.1 硬规则，违反直接拒审 |
| D3 | Android 包名 / iOS Bundle ID 改为 `com.ambertu.bottles.cn` | 微信开放平台按「包名 + 签名」绑定，先定下来；与海外版可共存一机 |
| D4 | 凭证放 `app_credentials`，新增平台 `wx_app`、`alipay_app`，不放 sysconfig | 与「微信凭证同源」惯例一致；credstore 已有加密、按 serial / appid 反查租户、热重载；sysconfig 没有这些 |
| D5 | 支付宝 App 支付的 `orderStr` 由**服务端签好**下发，客户端只负责交给 SDK | 应用私钥不出服务端；支付宝 App 支付本身就是这个模型（无需预下单 HTTP 调用） |
| D6 | 支付宝授权登录的 `authInfo` 同样服务端签发（`GET /auth/alipay/auth-info`） | 同 D5；PID 复用 `app_credentials.mch_id` 列（支付宝行语义为「商户 PID」） |
| D7 | 下单请求加 `channel` 字段，仅 `platform=app` 的登录态可传；`PayOrder.Platform` 存 `wx_app` / `alipay_app` | 一个 App 租户下用户可选两种渠道，不能再「以登录平台决定支付渠道」；订单页 / 后台订单筛选天然按渠道区分 |
| D8 | 单笔查单对 pending 超过 5 秒的订单**主动向渠道查一次**（微信 `out-trade-no` 查询 / 支付宝 `alipay.trade.query`），命中则走 `HandleCallback` 入账 | 异步通知会丢；客户端轮询本来就在打这个接口，顺手补偿比另起对账任务便宜。入账仍只有 `HandleCallback` 一个口 |
| D9 | 微信登录 provider 名 `wechat`、支付宝 `alipay`，与 Google / Apple 同走 `loginOrCreateOAuth`；身份落 `wx_openid` / `alipay_uid`，`union_id` 顺带存 | 复用冲突判断、建号、绑定 / 解绑、「最后一种登录方式不可解绑」全部既有逻辑 |
| D10 | 微信建号时用 `sns/userinfo` 的昵称 / 头像初始化资料 | 国内用户习惯；失败不阻断登录，回落「用户 xxxxxx」 |
| D11 | 新增 4 个 sysconfig 开关 `app_login_wechat_enabled` / `app_login_alipay_enabled` / `app_pay_wechat_enabled` / `app_pay_alipay_enabled`，默认 `1`；`/app-config` 下发的可用性 = 开关开 **且** 该租户配了对应凭证 | App 侧开关惯例默认 1（见 CLAUDE.md）；没配凭证时按钮不露出，不会出现「点了报错」。工具形态那条「默认关」规则约束的是小程序 |
| D12 | credstore 的 `bySerial` 索引键改为 `platform:serial` | 同一商户号同时服务小程序（`wx`）与 App（`wx_app`）时平台证书序列号相同，单键会互相覆盖 |
| D13 | 客户端 SDK：`fluwx ^6.0.4`（微信）、`tobias ^5.3.4`（支付宝） | 两者是 Flutter 社区事实标准，均覆盖登录 + 支付；版本以 `flutter pub get` 解析为准 |
| D14 | 国内版 `pubspec` 去掉 `google_sign_in`，保留 `sign_in_with_apple`；`google_maps_flutter` 本次不动 | D2；地图见 1.3 |

---

## 三、服务端设计

### 3.1 凭证

**`model.AppCredential.Platform`** 从 `size:8` 加宽到 `size:16`（AutoMigrate 自动改列，与 `PayOrder.Platform` 当年同款）。新增取值：

| platform | 用途 | 用到的列 |
|---|---|---|
| `wx_app` | 微信开放平台**移动应用**（登录 + App 支付） | `app_id`（开放平台 AppID，`wx` 开头）、`secret_enc`（AppSecret，换 token 用）、`mch_id`、`pay_apiv3_key_enc`、`pay_serial_no`、`pay_private_key_enc`、`pay_platform_key`、`pay_platform_serial`、`notify_url`（填 `https://ambertu.com/message/api/pay/callback/wx_app`） |
| `alipay_app` | 支付宝开放平台**移动应用**（登录 + App 支付） | `app_id`（2021 开头）、`mch_id`（**复用为 PID**，2088 开头，授权登录要）、`alipay_private_key_enc`（应用私钥 PKCS8 / PKCS1 都收）、`alipay_public_key`（**支付宝公钥**，不是应用公钥）、`notify_url`（填 `.../api/pay/callback/alipay_app`） |

credstore：
- `Resolved` 不加字段（PID 走 `MchID`）。
- 新增 `Version() uint64`，每次 `Reload` 自增；`pay.Service` 的驱动缓存按它失效，后台改完凭证不用重启服务。
- `bySerial` 键改为 `keySerial(platform, serial)`（D12）；`ByPlatformSerial(platform, serial)` 签名相应加参数，现有唯一调用点 `pay.Service.DriverBySerial` 同步改。

后台（`internal/admin`）：
- 新增 `POST /admin/api/credentials`：`{tenant_id, platform, appid}` 建行（`platform` 白名单 `wx` / `alipay` / `app` / `wx_app` / `alipay_app`；`(platform, appid)` 唯一冲突给友好提示），建完 `credStore.Reload()`。
- `ListCredentials` 加 `tenant_id`（已有）与 `tenant_name`，前端按租户分组显示。
- `Credentials.vue`：平台标签补 4 种；`wx_app` 的编辑字段与 `wx` 相同；`alipay_app` / `alipay` 的 `mch_id` 标签显示为「商户 PID」；新增「新增凭证」弹窗（租户下拉 + 平台下拉 + AppID）。中英文 locale 各补 key（`npm run build` 前置校验）。

### 3.2 支付驱动

#### 接口

```go
// driver.go 新增（可选能力，mock 不实现）
type Querier interface {
    // Query 主动向渠道查单。订单不存在 / 未支付返回 Paid=false, err=nil；网络或验签错误返回 err。
    Query(order *model.PayOrder) (*CallbackResult, error)
}
```

`Prepay` 签名不变；`payerID` 对 App 渠道恒为空。

#### 微信（`driver_wx.go`）

- `WxCreds` 加 `TradeType string`（`"JSAPI"` 默认 / `"APP"`）。`NewWxDriver` 不变，`buildDriver` 对 `wx_app` 传 `TradeType: "APP"`，`Name()` 返回 `wx_app`。
- `Prepay`：
  - APP：`POST /v3/pay/transactions/app`，请求体无 `payer`；拉起参数：
    `{"appid","partnerid"(mchid),"prepayid","package":"Sign=WXPay","noncestr","timestamp","sign"}`，
    签名串 `appid\n timestamp\n nonce\n prepay_id\n`（**APP 模式第四行是裸 prepay_id，不是 `prepay_id=xxx`**）。
  - JSAPI：原逻辑不动。
- `Query`：`GET /v3/pay/transactions/out-trade-no/{out_trade_no}?mchid=`，签名串 `GET\n path(含 query)\n ts\n nonce\n\n`（空 body 也要留空行）。
  `trade_state == SUCCESS` → `Paid=true`，带 `transaction_id` 与 `amount.total`。
- `doSignedPost` 泛化成 `doSigned(method, path, body)`，供 Query 复用。
- 回调验签 / 解密不变。

#### 支付宝（`driver_alipay.go` 重写 + 新文件 `alipay_sign.go`）

`alipay_sign.go`（包内共享，驱动与登录都用）：
- `alipaySign(priv *rsa.PrivateKey, params map[string]string) (string, error)`：剔除 `sign`，按 key 升序拼 `k=v&...`（**原始值不 URL 编码**），SHA256withRSA，base64。
- `alipayVerify(pub *rsa.PublicKey, params map[string]string) error`：剔除 `sign` / `sign_type`，同样拼串验签。
- `alipayGatewayCall(creds, method string, bizContent any, extra map[string]string) (json.RawMessage, error)`：
  向 `https://openapi.alipay.com/gateway.do` 发 `application/x-www-form-urlencoded`，公共参数 `app_id / method / format=JSON / charset=utf-8 / sign_type=RSA2 / timestamp(东八区 `2006-01-02 15:04:05`) / version=1.0`，
  解析 `<method 下划线>_response`，`code != "10000"` 返回 `sub_msg`；**响应验签**：对 `xxx_response` 的原始 JSON 子串 + 响应 `sign` 用支付宝公钥验。
- 支付宝公钥与应用私钥都可能是「裸 base64 无 PEM 头」——解析前补 `-----BEGIN PUBLIC KEY-----` / `PRIVATE KEY` 头，PKCS8 与 PKCS1 都尝试。

`AlipayDriver`：
- `AliCreds` 加 `TradeType`（`"APP"` / `"MINI"`）与 `PID`。
- `Prepay`（APP）：不调网关，拼 `alipay.trade.app.pay` 的公共参数 + `biz_content`：
  `{"out_trade_no","total_amount":"12.00"(分→元两位小数),"subject":"金币充值","product_code":"QUICK_MSECURITY_PAY","timeout_express":"30m"}`，
  含 `notify_url`；签名后输出 `orderStr`：所有参数（含 sign）**值 URL 编码** 后 `k=v&` 拼接。返回 `{"order_str": ...}`。
- `Prepay`（MINI，小程序）：`alipay.trade.create` 带 `buyer_id=payerID`，返回 `{"tradeNO": trade_no}`——小程序侧已按这个字段名读。
- `VerifyCallback`：`r.ParseForm()` → 全部表单项进 map → `alipayVerify`；再校验 `app_id == creds.AppID`；
  `trade_status ∈ {TRADE_SUCCESS, TRADE_FINISHED}` → Paid；`AmountFen = round(total_amount*100)`。无凭证时保留现有明文开发态。
- `Query`：`alipay.trade.query` `{"out_trade_no"}` → `trade_status` / `trade_no` / `total_amount`。
- `SuccessResponse` 不变（`success`）。

#### `pay.Service`

- `CreateOrder(tenantID, userID, packageID, platform, channel string)`：
  - `platform == "app"`：`channel` 必须 ∈ {`wx_app`, `alipay_app`}，否则 400「请选择支付方式」；mock 开关开时**忽略 channel** 走 mock（联调不变）。
    有效时 `driverPlatform = channel`，订单 `Platform = channel`。
  - 其它平台：`channel` 必须为空（防小程序乱传），行为不变。
  - `buildDriver` 新增 `wx_app` / `alipay_app` 分支，凭证取 `creds.ByTenantPlatform(tid, platform)`；单租户 `.env` 模式不支持这两个平台（返回「不支持的支付平台」）。
  - App 渠道走缓存（它们不是 mock，构造要解析密钥）；`platform == "app"` 本身仍不缓存。
- `Order(userID, orderNo)` 后新增 `SyncIfStale(order)`（D8）：`status == pending && now - created_at > 5s && 渠道实现了 Querier` → `Query` → `Paid` 则 `HandleCallback(order.Platform, res)`，再重读返回。
  Query 出错只打日志不影响查单响应；同一订单进程内 `sync.Map` 记上次同步时间，限 10 秒一次，防客户端轮询把渠道打爆（单实例部署，不上 Redis）。
- `HandleCallback` 不变（它已按 `order_no` 定位订单，不关心 platform）。
- 回调：
  - `callbackAuto(:platform)`：`wx` / `wx_app` 用 `Wechatpay-Serial` + platform 反查；`alipay` / `alipay_app` 先 `ParseForm` 取 `app_id` 用 `creds.ByAppID(platform, app_id)` 反查（`ParseForm` 后 `r.Form` 仍可被驱动复用，驱动改为读 `r.Form` 而不是重新 `ParseForm`）。

- 所有外呼（微信下单 / 查单、支付宝网关、微信 `sns/*`）经 `pkg/apilog` 异步落库（项目惯例，后台「接口日志」页可查）。

### 3.3 登录

#### 路由（`internal/user/handler.go`）

```
POST /auth/wechat               {appid, code}                → 登录/建号，应答同 /auth/google
POST /auth/alipay               {appid, auth_code}           → 同上
GET  /auth/alipay/auth-info?appid=                           → {auth_info}  服务端签好的授权串
POST /auth/wechat/bind   (auth) {appid, code}
POST /auth/alipay/bind   (auth) {appid, auth_code}
POST /auth/unbind        (auth) {provider}   provider 扩到 wechat / alipay
```

#### 服务

- `wxAppCode2Token(appid, secret, code)`：`GET https://api.weixin.qq.com/sns/oauth2/access_token` → `{access_token, openid, unionid}`；
  再 `GET /sns/userinfo?access_token&openid&lang=zh_CN` → `{nickname, headimgurl}`（失败忽略，D10）。放 `oauth.go`，与 `wxCode2Session` 并列。
- `alipayCode2UID(creds, code)` 真正实现：`alipay.system.oauth.token` `grant_type=authorization_code&code=` → `user_id`（优先）/ `open_id`。
  可选再调 `alipay.user.info.share` 取昵称头像（需「获取会员信息」权限，拿不到就跳过）。
- `alipayAuthInfo(creds)`：`apiname=com.alipay.account.auth&app_id=&app_name=mc&auth_type=AUTHACCOUNT&biz_type=openservice&method=alipay.open.auth.sdk.code.get&pid=&product_id=APP_FAST_LOGIN&scope=kuaijie&sign_type=RSA2&target_id=<随机>`，
  `alipaySign` 后 `&sign=<urlencode>`。无 PID 时返回错误「未配置支付宝 PID」。
- `appIdentity` 加 `WxOpenID`、`AlipayUID`；`where()` 在 `GoogleSub` / `AppleSub` 之后、`Phone` 之前加两个 case（同属「签发方保证唯一」的标识）。
- `loginOrCreateApp` 建号时写 `WxOpenID` / `UnionID` / `AlipayUID`；`appIdentity` 再加 `Nickname` / `Avatar`，非空时覆盖默认昵称 / 头像。
- `verifyOAuthSub` 加 `wechat` / `alipay` 分支（code 换身份），`sub` 即 openid / user_id；`emailVerified=false`。
- `subColumn`：`wechat → wx_openid`，`alipay → alipay_uid`。这两列是 `string` 非指针，`BindOAuth` / `UnbindOAuth` 写值 / 置空串而不是 nil（按列类型分支）。
- `loginMethods` 加 `Wechat` / `Alipay`，`remainingAfterUnbind` 同步。
- `appdto` 加 `wechat_bound` / `alipay_bound`（与 `google_bound` 同款）。
- 凭证解析：`resolveAppTenant(appid)` 不变（`platform=app` 行定租户）；微信 / 支付宝凭证用 `creds.ByTenantPlatform(tenantID, "wx_app" / "alipay_app")`，没有则「未配置微信登录」/「未配置支付宝登录」（`CodeLoginFailed`）。

### 3.4 `/app-config`

`auth` 段新增：

```json
"wechat": true,                 // app_login_wechat_enabled && 有 wx_app 凭证
"wechat_app_id": "wx...",       // 客户端注册 SDK 用；没凭证为空串
"wechat_universal_link": "...", // 新 sysconfig app_wechat_universal_link（iOS 必填项，Android 忽略）
"alipay": true                  // app_login_alipay_enabled && 有 alipay_app 凭证
```

新增 `pay` 段（与 `pricing` 分开——`pricing` 是扣费规则，这里是充值渠道）：

```json
"pay": {"wechat": true, "alipay": true}   // 各自 = 开关 && 凭证
```

免鉴权请求仍回落 `appTenantOf`（国内租户的 `APP_ID` 是 `drift_app_cn`，有 Bearer 时按 JWT 租户）。

### 3.5 sysconfig 新 key（`sysconfig.go` defaults + `meta.go` 白名单，中英文标签）

| key | 默认 | 分组 | 类型 |
|---|---|---|---|
| `app_login_wechat_enabled` | `1` | `GroupAppAuth` | bool |
| `app_login_alipay_enabled` | `1` | `GroupAppAuth` | bool |
| `app_wechat_universal_link` | `""` | `GroupAppAuth` | text |
| `app_pay_wechat_enabled` | `1` | `GroupAppPay` | bool |
| `app_pay_alipay_enabled` | `1` | `GroupAppPay` | bool |

`sysconfig/app_config_test.go` 补这 5 个键的默认值断言（App 键「开关默认 1 / 文案默认空」铁律）。

---

## 四、客户端设计（`app/bottles_zh`）

### 4.1 工程标识

- Android：`applicationId` / `namespace` → `com.ambertu.bottles.cn`，Kotlin 包目录同步迁移（`MainActivity.kt`），`android:label` → 「漂流瓶」。
- iOS：`PRODUCT_BUNDLE_IDENTIFIER` → `com.ambertu.bottles.cn`（三个 configuration），`CFBundleDisplayName` → 「漂流瓶」。
- `AppConfig.appId` 默认值 → `drift_app_cn`；`pubspec.name` 保持 `bottles`（改了要动所有 import，无收益）。

### 4.2 依赖（`pubspec.yaml`）

```yaml
fluwx: ^6.0.4
tobias: ^5.3.4
# 删除 google_sign_in；保留 sign_in_with_apple（iOS）
tobias:
  url_scheme: ambertubottlescn   # iOS 回跳 scheme，不能含下划线
```

`fluwx` 的 `pubspec` 配置块只填 `app_id` 做占位，真正注册在运行时拿到 `/app-config` 的 `wechat_app_id` 后进行。

### 4.3 第三方登录 `core/platform/oauth.dart`

```dart
abstract class OAuthClient {
  Future<String> wechatCode();                 // 拉起微信授权，返回 code；取消抛 OAuthCancelled
  Future<String> alipayAuthCode(String authInfo); // 交给支付宝 SDK，返回 auth_code
  Future<String> appleIdToken();               // 不变，仅 iOS
}
```

- `RealOAuthClient(wechatAppId, universalLink)`：首次调用 `Fluwx().registerApi(...)`（幂等，`appId` 为空直接抛「未配置微信登录」）；
  `sendWeChatAuth(scope: 'snsapi_userinfo', state: <随机>)`，用 `addSubscriber(onWeChatAuthResponse)` 等结果，**校验 state**，
  `errCode == -2` → `OAuthCancelled`，`-4` → 拒绝授权（普通错误）；60 秒无回调按取消处理。
- 支付宝：`Tobias().auth(authInfo)` → `resultStatus == '9000'` 且 `result` 里解析 `auth_code=`；`6001` → `OAuthCancelled`。
- `oauthClientProvider` 读 `appConfigProvider` 的 `wechatAppId` / `wechatUniversalLink`。
- `wechatInstalled` / `alipayInstalled` 两个 Future getter（`isWeChatInstalled` / `isAliPayInstalled`），登录页未安装时按钮灰显并提示。

### 4.4 `AuthController` / 仓库

- `loginWithProvider(provider)`：`'wechat'` → `wechatCode()`；`'alipay'` → 先 `authRepo.alipayAuthInfo()` 再 `alipayAuthCode()`；`'apple'` 不变。
- `AuthRepository.loginWithProvider(provider, {idToken, code})`：请求体按 provider 组装
  （`wechat`: `{appid, code}`；`alipay`: `{appid, auth_code}`；`apple`: `{appid, id_token}`）。`bindOAuth` 同款。
- 新增 `AuthRepository.alipayAuthInfo()` → `GET /auth/alipay/auth-info`。
- Mock 仓库补对应分支（widget test 不依赖 SDK）。

### 4.5 登录页（A2）

「其他方式」区按 `/app-config` 显隐：微信（品牌绿 `#07C160`，圆形图标按钮）、支付宝（品牌蓝 `#1677FF`）、Apple（仅 iOS，黑）。
三个都关时整段「或」分隔线不显示。A2g 等待层文案按 provider 显示「正在打开微信…」/「正在打开支付宝…」。
`OtpState.dialCode` 默认值从 `+91` 改为 `+86`。

### 4.6 支付

`domain/models/wallet.dart`：

```dart
enum PayChannel { wechat, alipay, iap }   // 原 upi / card 删除
```

`core/pay/`：
- `wechat_pay_adapter.dart`：`WechatPayAdapter(fluwx)`：`pay_params` 取 `appid/partnerid/prepayid/package/noncestr/timestamp/sign` → `Fluwx().pay(which: Payment(...))`；
  用 `onWeChatPaymentResponse` 等结果：`errCode 0 → launched`（**仍以服务端为准**）、`-2 → cancelled`、其它 → `failed`。
- `alipay_adapter.dart`：`AlipayAdapter(tobias)`：`pay_params['order_str']` → `Tobias().pay(orderStr)`：`9000/8000/6004 → launched`、`6001 → cancelled`、其它 → `failed`。
- `PaymentFlowPage._start` 按 `pending.payParams` 选适配器：`isMock → Mock`，`有 order_str → Alipay`，`有 prepayid → Wechat`。
  **状态机与六屏不改**（正是 2026-09-21 设计留的扩展点）。
- `recharge_page.dart`：Android 渠道单选换成「微信支付」「支付宝」两行（图标用品牌色圆标），按 `appConfig.payWechat / payAlipay` 显隐，
  默认选第一个可用项；两个都不可用时按钮禁用并提示「暂未开放充值」。`recharge()` 请求体加 `channel: 'wx_app' | 'alipay_app'`。
  iOS 仍走 `_iosIap`，不变。
- `AppRemoteConfig` 加 `wechatLogin / alipayLogin / wechatAppId / wechatUniversalLink / payWechat / payAlipay`，内置值：开关 `true`、字符串空串；`test/remote_config_test.dart` 补断言。

### 4.7 账号与安全页

绑定列表改为：微信、支付宝、Apple（iOS）。`UserProfile` 加 `wechatBound / alipayBound`。
`_bind('wechat')` 流程同登录（code → `/auth/wechat/bind`）。

### 4.8 平台配置

Android `AndroidManifest.xml`：
- `<queries>` 加 `<package android:name="com.tencent.mm"/>`、`<package android:name="com.eg.android.AlipayGphone"/>`。
- 微信回调入口：按 `flutter pub get` 后 `fluwx` 插件自带 manifest 决定——若插件已声明 `FluwxWXEntryActivity` / `FluwxWXPayEntryActivity`，
  则加两条 `activity-alias`（`${applicationId}.wxapi.WXEntryActivity` / `WXPayEntryActivity` → 插件 Activity，`exported=true`）；
  这一点在实施时读插件源码确认，不凭记忆写。
- 签名：Release 必须用正式 keystore（微信按签名 MD5 绑定）；`build.gradle.kts` 从 `key.properties` 读签名配置（文件 gitignore）。

iOS `Info.plist`：
- `LSApplicationQueriesSchemes` 加 `weixin`、`weixinULAPI`、`weixinURLParamsAPI`、`alipay`、`alipays`。
- `CFBundleURLTypes` 加两项：微信（scheme = 微信 AppID，构建期从 `--dart-define` 无法注入 plist，**用 xcconfig 变量 `WECHAT_APP_ID`**，默认空）、支付宝（`ambertubottlescn`）。
- `Runner.entitlements` 加 Associated Domains `applinks:ambertu.com`；服务器 nginx 需在 `https://ambertu.com/.well-known/apple-app-site-association` 提供 AASA（部署项，写进 §七）。
- `AppDelegate.swift`：fluwx / tobias 均通过插件处理 `openURL` / `continueUserActivity`，无需手写。

### 4.9 文案

`app_zh.arb` / `app_en.arb` 新增：`loginWechat`、`loginAlipay`、`loginOpeningWechat`、`loginOpeningAlipay`、`loginWechatNotInstalled`、`loginAlipayNotInstalled`、
`rechargeWechat`、`rechargeAlipay`、`rechargeNoChannel`、`bindWechat`、`bindAlipay`、`channelNotConfigured`。`flutter gen-l10n` 重新生成。

---

## 五、要改的既有代码（清单）

**server**
- `internal/model/model.go`：`AppCredential.Platform size:16`
- `internal/tenant/credstore.go`：serial 索引键带平台
- `internal/pay/driver.go` `driver_wx.go` `driver_alipay.go` `alipay_sign.go`(新) `service.go` `handler.go` `order_query.go`
- `internal/user/oauth.go` `appauth.go` `oauthlink.go` `otp.go`(appIdentity) `handler.go` `alipay_auth.go`(新)
- `internal/common/appdto/appdto.go`
- `internal/sysconfig/sysconfig.go` `handler.go` `app_config_test.go`
- `internal/admin/meta.go` `service.go` `handler.go` `common/i18n/catalog.go`（新错误文案）

**admin**
- `src/views/Credentials.vue` `src/locales/zh-CN.json` `src/locales/en.json` `src/api/*`

**app/bottles_zh**
- `pubspec.yaml`、`android/`、`ios/`、`lib/core/config/app_config.dart` `remote_config.dart`、`lib/core/platform/oauth.dart`、`lib/core/pay/*`、
  `lib/data/repositories.dart` `remote/remote_repositories.dart` `mock/mock_repositories.dart`、`lib/domain/models/wallet.dart` `user.dart`、
  `lib/features/auth/auth_controller.dart` `login_page.dart`、`lib/features/me/recharge_page.dart` `payment_flow_page.dart` `account_security_page.dart`、`lib/l10n/*.arb`

---

## 六、测试策略

**server（无 DB 基建，纯函数 + httptest）**
- `alipay_sign_test.go`：用测试内生成的 RSA 密钥对，`alipaySign` → `alipayVerify` 往返；拼串排除 `sign`/`sign_type`；中文值不编码。
- `driver_alipay_test.go`：`Prepay(APP)` 输出的 `order_str` 可被 `url.ParseQuery` 还原并通过 `alipayVerify`；`total_amount` 分→元格式（`1` → `0.01`、`1200` → `12.00`）；
  `VerifyCallback` 对篡改金额 / 错 app_id 拒绝；`TRADE_CLOSED` → `Paid=false`。
- `driver_wx_test.go`：APP 拉起参数签名串第四行为裸 prepay_id；`doSigned(GET)` 签名串含空 body 行；`httptest` 假网关返回 `prepay_id`。
- `service_test.go`：`CreateOrder` 的 channel 校验表（app+空 → 400；app+wx_app → ok；wx+wx_app → 400；mock 开 → 忽略 channel）。
- `credstore_test.go`：同 serial 两平台互不覆盖。
- `oauthlink_test.go`：`remainingAfterUnbind` 加 wechat / alipay；`appIdentity.where()` 优先级。
- `routes_test.go`：新路由注册。
- `meta_test.go` / `catalog` AST 测试自动覆盖新 key 与新文案。

**app/bottles_zh**
- `test/pay_channel_test.dart`：两个适配器用假 SDK 接口（抽 `WechatPayGateway` / `AlipayGateway` 接口注入）映射 `ChannelOutcome` 全表。
- `test/payment_flow_test.dart`：按 `pay_params` 选适配器的三分支。
- `test/remote_config_test.dart`：新字段默认值与解析。
- `test/widgets/login_channels_test.dart`：微信 / 支付宝 / Apple 按钮显隐矩阵（含「三个都关不显示分隔线」、「未安装灰显」）。
- `test/widgets/recharge_channels_test.dart`（新）：渠道行显隐与默认选中、两个都关禁用按钮。
- `flutter analyze` 零告警；`test/layout/all_pages_test.dart` 布局矩阵继续通过。

**人工联调（账号到位后）**
- 微信：真机 Release 包（签名 MD5 已登记）→ 授权 → 建号 → 充值 1 分钱档位 → 回调入账 → 关掉回调（nginx 临时 403）再付一笔验证 D8 主动查单补偿。
- 支付宝：沙箱环境先过（`alipay_app` 行 appid 填沙箱 appid、网关不变——沙箱网关是 `openapi-sandbox.dl.alipaydev.com`，驱动加 `sandbox` 判断：appid 以 `9021` 开头走沙箱网关）。

---

## 七、部署与账号待办（账号申请到手后对号入座）

| 来源 | 拿到什么 | 填到哪 |
|---|---|---|
| 后台「租户」 | 新建国内租户 | 记下 tenant_id |
| 后台「租户凭证」 | 新增 `app` 行 `appid=drift_app_cn` | 国内版 `APP_ID` |
| 微信开放平台 → 移动应用 | AppID / AppSecret；登记包名 `com.ambertu.bottles.cn` + Release 签名 MD5；iOS Bundle ID + Universal Link | `wx_app` 行 `app_id` / `secret`；`app_wechat_universal_link` |
| 微信支付商户平台 | 商户号、APIv3 密钥、商户 API 证书（序列号 + 私钥）、平台证书（序列号 + 公钥）；**绑定上面的开放平台 AppID** | `wx_app` 行对应列；`notify_url` 填 `/api/pay/callback/wx_app` |
| 支付宝开放平台 → 移动应用 | AppID、应用私钥（自己生成）、支付宝公钥、PID；签约「App 支付」「App 支付宝登录」；登记包名 + 签名、iOS Bundle ID + scheme | `alipay_app` 行 `app_id` / `ali_private_key` / `ali_pub_key` / `mch_id`(PID)；`notify_url` 填 `/api/pay/callback/alipay_app` |
| 服务器 nginx | `/.well-known/apple-app-site-association`（`Content-Type: application/json`，不带扩展名） | `pet.conf` 加 location |
| 后台「系统配置」 | 四个开关默认开，无需动；iOS 继续配 IAP 分组 | — |

---

## 八、遗留

- 腾讯地图替换 Google Maps（国内版地图功能当前会渲染空白）。
- 微信 / 支付宝退款接口与对账单。
- 支付宝 `alipay.user.info.share` 昵称头像需额外权限，首版可能拿不到，回落默认昵称。
- 小程序端支付宝正式支付的联调（驱动已可用）。
- 国内版应用市场合规清单（隐私政策 SDK 列表要加微信 / 支付宝 SDK）。
