# 第三方登录服务商化(sso 域)

> 2026-10-06 ～ 2026-10-07 · L2 · 关联 [l1/provider-configs](../l1/provider-configs.md) [l1/user-auth](../l1/user-auth.md)
> 原始文档:`docs/superpowers/specs/2026-10-06-sso-provider-configs-design.md` · `docs/superpowers/plans/2026-10-06-sso-provider-configs.md`
> 实施 commit:`f3bf784` `066dfe2` `3232b47` `8f5de6c` `969b073` `c1e4b7d` `12ae5fe` `cab3ae9`
> 状态:**COMPLETED**

## 1. 问题

四家登录(微信 / 支付宝 / Google / Apple)的「能不能用」有**三套不同答案**:
微信查一个开关 + 一行凭证;支付宝查一个开关 + **一行查错了的凭证**(它的密钥其实在支付卡片上);
Google 与 Apple **根本没查过**,按钮无条件渲染,租户想关都关不掉。

## 2. 做法

`provider_configs` 加第四个域 `KindSSO`,四张卡片,全部 `PlatformApp`,**并存不单选**(和支付一样由客户端选渠道)。
`/app-config` 的 `auth` 段改为统一回答一个问题:**「启用 且 必填齐全」** → 下发四个布尔,
两个 Flutter 工程的登录页据此显示按钮。老客户端忽略新增的 `google` / `apple` 两个布尔,行为不变。

## 3. `DependsOn`:支付宝登录的密钥不在登录卡片上

支付宝登录与支付**同一个应用、同一把私钥**。给 `provider.Definition` 加了可选的 `DependsOn`,
登录卡片声明依赖 `pay/alipay` —— 支付那边缺密钥会**连带判定登录不可用**,登录页立刻不显示支付宝
按钮,而不是显示一个点了报签名错误的按钮。

⚠️ `DependsOn` 指向不存在的卡片(schema 写错)时 `Missing` / `Usable` **不能 panic**,否则整个后台
服务商页和 `/app-config` 一起挂。

## 4. 迁移标记必须独立

迁移标记是 **`sso_migrated`,不是 `provider_migrated`** —— 后者线上早已是 `"1"`,复用等于迁移
永不执行,每个租户都变成「没有任何登录配置」。
迁移跑第二遍不得覆盖运营在新页面手工改过的值;**没有来源的渠道不建卡片**,否则后台永远挂一个红色「缺 1 项」。

## 5. 删掉的东西(单一真相源)

| 删除项 | 说明 |
|---|---|
| `app_login_wechat_enabled` / `app_login_alipay_enabled` | 旧开关 |
| `app_google_client_id` / `app_apple_bundle_id` / `app_wechat_universal_link` | 旧凭证键 |
| `app_credentials` 的 `wx_app` / `alipay_app` 两行 | 不再被读,凭证页也不再接受这两个平台值 |

读取侧同步改:微信登录从登录卡片取 appid/secret,Google / Apple 取各自的 audience,
**支付宝 partner ID 不再回落旧凭证行**(留着回落 = 新页面改的值静默输给旧行的陈值);
支付宝私钥仍来自支付卡片。

> `google_client_id` 是**逗号分隔多值**,第一个按约定是 Web client ID;下发给客户端的只取第一个(`firstCSV`)。

## 6. 两端的内置默认

四家全未配置时,登录页只剩手机号 / 邮箱,**不能出现一个按钮都没有的死页面,也不能崩**;
「或」分割线跟着一起收起,免得空分割线悬在上面。

- `app/bottles`(海外):内置默认 Google 开、Apple 开 —— 与它此前发布的行为一致。
- `app/bottles_zh`(国内):内置默认 Google 关、Apple 开。
