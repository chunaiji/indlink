# SSO 登录服务商化设计

> 把 App 第三方登录（微信 / 支付宝 / Google / Apple）的配置并入 `provider_configs`
> 体系，成为第四个域 `sso`；启用并填齐的渠道才在登录页出现。

**日期：** 2026-10-06
**状态：** 待审

---

## 一、为什么做

今天 App 的 SSO 配置散在三处，谁也说不清一个渠道「到底开没开」：

| 渠道 | 凭证在哪 | 开关在哪 | 登录页怎么决定显示 |
|---|---|---|---|
| 微信 | `app_credentials` 表 `platform='wx_app'` 行 | sysconfig `app_login_wechat_enabled` | 开关 && 有凭证行 |
| 支付宝 | **服务商页「支付 → 支付宝」卡片**，旧凭证行仅作回落 | sysconfig `app_login_alipay_enabled` | 开关 && 有凭证行 |
| Google | sysconfig `app_google_client_id` | **没有开关** | 国际版无条件显示；国内版没有 |
| Apple | sysconfig `app_apple_bundle_id` | **没有开关** | `Platform.isIOS` 硬判断 |

四个后果：

1. **配一个渠道要去两个页面。** 微信要在「租户凭证」填 appid/secret，再去 `/config`
   找开关。这正是服务商体系当初要消掉的分裂。
2. **Google 和 Apple 没法关。** 运营想临时下掉一个渠道只能发版。
3. **支付宝的开关和密钥不在一起。** 密钥早就读支付卡片（同一个应用、同一把私钥，
   `alipay_auth.go:20-41` 有注释说明），开关却还在 sysconfig。
4. **支付宝的判定是错的。** `/app-config` 判「有没有 `alipay_app` 凭证行」，
   而真正用的密钥在支付卡片。只要旧行还在，密钥删了按钮照样露出来，点了才报错。

另有一处已经分叉：`app_apple_bundle_id` 原本同时服务 SSO 和内购，内购那半
在服务商改造时搬进了 `pay/apple` 的 `bundle_id`，SSO 这半还读 sysconfig。
两个值现在可以不一致，而没有任何东西拦着。

## 二、目标与非目标

**目标**

- 新增服务商域 `sso`，四家：`wechat` / `alipay` / `google` / `apple`。
- `/app-config` 的 `auth` 段改为「启用 && 必填齐全」才下发 true，四家一个判法。
- 两个 Flutter 工程的登录页都按下发结果决定显示哪些按钮，四家一致。
- 把 `app_credentials` 的 `wx_app`、`alipay_app` 两行搬进 `sso` 域。

**非目标**

- 手机号 / 邮箱登录不是「服务商」，开关留在 sysconfig 不动。
- 小程序登录用的 `wx`、`alipay` 两行原地不动。微信 access_token 同源那条链路
  （登录 / 订阅推送 / 内容安全共用凭证，`push/service.go:218`）完全不碰。
- 不改任何一家的登录协议实现，只改「配置从哪读」。
- 不动 `resolveAppTenant`：它查 `platform='app'` 行，与本次搬的两行无关。

## 三、关键设计决定

### D1：支付宝登录不复制支付的密钥，而是声明依赖

支付宝的登录和支付用**同一个应用、同一把私钥**，代码里已经这么做了。
如果 `sso/alipay` 再开一份 `private_key`，运营填两遍、迟早填歪一个，
症状是「支付好的，登录报签名错误」。

给 `Definition` 加一个可选字段：

```go
// DependsOn 本卡片的凭证来自另一张卡片(同租户)。声明后:
//   - 本卡片不重复渲染被依赖卡片的字段
//   - Missing 连带报告被依赖卡片的缺项,于是 Usable 也跟着正确
DependsOn *Ref // Ref{Kind, Provider}
```

`sso/alipay` 声明 `DependsOn: &Ref{KindPay, "alipay"}`，自己只留登录专属的
`pid`（商户 PID，App 授权登录签名要用，收款不要——今天它错放在支付卡片上）。

仓库里已有一条同类规则：`admin/providers.go:113` 规定启用 `moderation/alipay`
必须先让 `pay/alipay` 可用。那是写死在保存逻辑里的一次性判断，管不住
`Usable`。本次做成 schema 字段，顺手让那条规则也能改用它。

微信不走这条：微信开放平台**移动应用**登录要 appid + secret，而微信支付要
appid + 商户号 + 三把密钥，secret 支付根本不用。两者 appid 常常相同但不保证
同一个应用，所以微信登录卡片自带 appid + secret。

### D2：就绪 = 启用 && 必填齐全，和支付同一个函数

`/app-config` 的 `pay` 段已经是 `providerStore.Usable(tid, provider)`。
`auth` 段照抄。今天三家三种写法（微信判开关+凭证、支付宝判开关+错的凭证、
Google 和 Apple 不判），改完一个写法。

### D3：Apple 的开关带下架警告，但仍然可关

苹果审核指南 4.8：App 只要提供了任何第三方登录，就必须同时提供 Apple 登录。
在 iOS 上关掉它有下架风险。但做成「不可关」也不对——安卓包、国内版用不上它。

折中：开关照给，字段说明里写明风险。这与 `app_pay_mock_enabled`（危险但保留，
靠说明和默认值兜）是同一种处理。

### D4：迁移要一个新标记，不能复用 `provider_migrated`

`provider.Migrate` 在 `provider_migrated == "1"` 时直接返回（`migrate.go:157`）。
线上已经是 `1` 了，把 SSO 迁移塞进同一个函数等于**永远不会执行**。

因此用独立标记 `sso_migrated`，独立的 `MigrateSSO` 入口，复用
`planMigration` 那套「纯函数 + 表驱动测试」的写法。

搬的内容：

| 目标 | 来源 | 没有来源时 |
|---|---|---|
| `sso/wechat` 的 `app_id` / `app_secret` | `wx_app` 行的 AppID / Secret | 不建卡片 |
| `sso/wechat` 的 `universal_link` | sysconfig `app_wechat_universal_link` | 空 |
| `sso/wechat` 的 `enabled` | sysconfig `app_login_wechat_enabled` | — |
| `sso/alipay` 的 `pid` | `alipay_app` 行的 `MchID`，回落 `pay/alipay` 的 `pid` | 空 |
| `sso/alipay` 的 `enabled` | sysconfig `app_login_alipay_enabled` | — |
| `sso/google` 的 `client_id` | sysconfig `app_google_client_id`（CSV 原样搬） | 不建卡片 |
| `sso/apple` 的 `bundle_id` | sysconfig `app_apple_bundle_id` | 不建卡片 |
| `sso/google` / `sso/apple` 的 `enabled` | 无开关可搬，按「配了值就算开」推断 | 关 |

迁移后**不保留回落读旧位置**。保留双读的代价是：运营在新页面改了值不生效，
因为旧位置还有值且优先——这种 bug 查起来要半天。迁移逐租户打印搬了什么，
和服务商那次一致；旧行与旧键的值都不删，回滚还能用。

### D5：`wx_app` / `alipay_app` 退出凭证页

迁移后 `credPlatforms`（`admin/service.go:629`）去掉这两个值，只留
`wx` / `alipay` / `app`。否则两个页面都能配微信 App 登录，又回到分裂。
已有的行不删，只是不再被读、不再可编辑。

### D6：Google 的 CSV 多值契约必须原样保留

`app_google_client_id` 是逗号分隔的多个 audience，第一个按约定是 Web client ID
（`appauth.go:173-193`，`appauth_audience_test.go` 守着）。搬进卡片后仍是一个
文本字段存 CSV，`splitClientIDs` / `primaryClientID` 不动，只换取值的来源。

### D7：国际版登录页这次一并改

国际版 `app/bottles` 今天把 Google 按钮无条件渲染，`remote_config.dart` 里
连 `wechatLogin` / `alipayLogin` 字段都没有。要做到「四家一致」，它必须补上
四个布尔的解析和按钮判断。不补就只有国内版受配置控制，等于没统一。

补完后国际版的默认行为不变：Google 和 Apple 配了就显示，微信支付宝没配就不显示。

## 四、数据与接口

### 4.1 `sso` 域的四张卡片

| 卡片 | Platform | 字段 | 依赖 |
|---|---|---|---|
| `wechat` | app | `app_id`✱、`app_secret`✱🔒、`universal_link` | — |
| `alipay` | app | `pid`✱ | `pay/alipay` |
| `google` | app | `client_id`✱ | — |
| `apple` | app | `bundle_id`✱ | — |

✱ = 必填，🔒 = 机密（整块 AES-GCM 存储，后台只回 `"set"`）

`universal_link` 选填：iOS 微信 SDK 要，安卓忽略。不填只影响 iOS。

四家都是 `PlatformApp`——小程序租户看到的是空页（`providers.none`）。
`SingleActive` 不含 `sso`：四家可并存，和支付一样由客户端选渠道。

`LookupA` / `LookupB` 都不设：SSO 没有「从回调反查租户」的场景，
租户是客户端带 appid header 决定的。

### 4.2 `/app-config` 的 `auth` 段

```json
"auth": {
  "phone": true,            // 不变,sysconfig
  "email": true,            // 不变,sysconfig
  "wechat": true,           // sso/wechat 启用且齐全
  "wechat_app_id": "wx…",   // 同卡片的 app_id
  "wechat_universal_link": "https://…",
  "alipay": true,           // sso/alipay 启用且齐全(含 pay/alipay 的必填)
  "google": true,           // 新增
  "google_client_id": "…",  // 同卡片 client_id 的第一个(firstCSV,行为不变)
  "apple": true             // 新增
}
```

字段名和位置全部不动，只新增 `google` / `apple` 两个布尔。

**向后兼容：** 老版本 App 不认新布尔，Google 按原样无条件显示、Apple 按
`Platform.isIOS` 显示，行为与今天一致。新版本认，按配置显示。两边都不会崩。

实现上照搬支付那套注入：`handler.WithPayUsable` 旁边加 `WithSSOUsable` 和
`WithSSOField`，`main.go` 传 `providerStore.Usable` 和一个取字段的闭包。
`sysconfig` 包仍然不 import `provider`。

### 4.3 登录页

两个工程都把四个布尔当「这个渠道能不能用」，与国内版现有 `wechatLogin` /
`alipayLogin` 的处理一致。Apple 改为 `平台支持 && 配置启用`，Google 改为
`配置启用`。四个都 false 时登录页只剩手机号 / 邮箱，不能出现空白页。

### 4.4 后台

只加一个侧边栏条目 `/providers/sso` 和两条 i18n（中英各一份，`nav.providersSso`
与 `providers.hint.sso`）。路由是动态的 `providers/:kind`，页面组件不用改。
探活分派（`main.go` 的 probe 表）加 `sso` 分支，否则「测试」按钮报「探活未配置」。

## 五、测试

| 用例 | 盯什么 |
|---|---|
| `sso/alipay` 的 `Missing` 把 `pay/alipay` 的缺项也算进去 | D1 的依赖判定生效，否则登录页会露出点了就报错的按钮 |
| `sso/wechat` 不声明依赖，缺 `app_secret` 即不可用 | 微信不能借支付的 appid 冒充已配置 |
| `DependsOn` 指向不存在的卡片时 `Missing` 不 panic | schema 写错不能打死整个后台页面 |
| 迁移：`wx_app` 行 + 开关 → `sso/wechat`，值逐项相等 | 迁移漏字段等于线上登录直接坏 |
| 迁移：Google 配了 client_id 但无开关 → `enabled=true`；没配 → 不建卡片 | D4 的推断规则 |
| 迁移：`sso_migrated` 与 `provider_migrated` 互不干扰 | D4 的核心，用错标记等于迁移永不执行 |
| 迁移：跑第二遍不覆盖人工改过的值 | 服务商那次的同款保护 |
| `/app-config` 四家都没配 → 四个布尔全 false | 「没配就不露按钮」 |
| `/app-config` 的 `auth` 仍回落 App 租户 | CLAUDE.md 记过的事故，`appTenantOf` |
| `google_client_id` 多值时仍只下发第一个 | D6，`firstCSV` 行为不变 |
| `credPlatforms` 不再接受 `wx_app` / `alipay_app` | D5，`credential_test.go` 已有这条 |
| 两个工程：四个布尔全 false 时登录页零个第三方按钮 | 不能出现死页面 |
| 国际版：`google: false` 时 Google 按钮消失 | D7，这条今天完全没有守护 |

受影响的既有测试（必须同步改，不能绕过）：
`sysconfig/app_config_test.go` 的七条、`admin/meta_test.go` 与
`meta_secret_test.go`、`admin/credential_test.go`、`provider/migrate_test.go`、
两个工程的 `remote_config*_test.dart` 与 `widgets/login_channels_test.dart`。
后台两个语言文件 key 必须同增，否则 `npm run build` 的前置校验直接卡住。

## 六、风险

1. **迁移漏搬 = 线上登录直接坏。** 缓解：迁移逐租户打印搬了什么；旧行旧键保留可回滚；
   上线后先用一个租户实测四个渠道再放开。
2. **Apple 开关被误关 → iOS 下架风险。** 缓解：字段说明写明，迁移按原状推断为开。
3. **`sso/alipay` 依赖 `pay/alipay`，运营在支付页删了密钥会连带关掉登录。**
   这是正确行为，但登录卡片上要显示「密钥来自支付页」，否则运营会以为登录页坏了。
4. **国际版改动没有存量测试兜底。** 缓解：按上表给它补齐与国内版对等的用例。
