# 聊天扣费规则可配 + 背包优先送礼 + App 登录开关

> 2026-10-04 · L2 · 关联 [l1/chat-ws](../l1/chat-ws.md) [l1/pay-wallet](../l1/pay-wallet.md) [l1/admin-platform](../l1/admin-platform.md)
> 触发:小米 9 第四轮真机反馈(见 [l2/2026-10-03-app-prototype-alignment](2026-10-03-app-prototype-alignment.md) §4)暴露的三处「标价与实扣不一致」
> 状态:**COMPLETED**

一条主线:**客户端不许自己算价,一律问服务端**。三处写死的价钱全部改成后台可配 + `/app-config` 下发。

## 1. 聊天扣费三键(用户拍板的规则)

| 键 | 含义 | 默认 |
|---|---|---|
| `price_chat` | 开聊扣 M | 5 |
| `price_msg` | 每条扣 N | 0(不扣) |
| `chat_free_msgs` | 每个会话**发送方**前 L 条免费 | 0 |

- 真扣在 `chat/service.go`:`StartChat` / `SendMessage` + `withinFreeMsgs`(按会话内该发送方的非系统消息条数判定)。
- 三键并入后台「价格」分组;`/app-config` 新增 `pricing` 段,App 的打招呼按钮标价与聊天页顶部提示都从这里取(**之前写死 5**)。
- ⚠️ 低余额系统提示**只在 N>0 时发** —— 之前按条不收费也弹「余额不足」,用户点了没扣钱,提示是假的。
- ⚠️ App 与小程序是两个租户,改规则要改 App 那个租户。

## 2. 送礼:背包先抵扣,差额扣币

`chat.SendGift` 与 `moment.SendGift` 统一成一条规则(`7696c7f`):同一事务内先消耗自己持有的
`ItemOrder` 存量,不够的部分再扣币,**每单位记一条 `ItemOrder(target)`** —— 所以聊天里送的礼也
会进礼物墙。客户端只对差额标价,首帧先定选中项再算抵扣。

此前的 bug:手里有 ×1 仍按全价标、按全价校验余额。

## 3. 其它按配置化

- **发现页**:撤回(rewind)挂价格角标,✕ 只有后台开了 `app_discover_skip_charge_enabled`(默认 0)才带价。两项价格进 `/app-config` 的 `pricing`,**不登录也能拿到**,首次撤回不再显示 0 金币。
- **App 登录方式**:`app_login_phone_enabled` / `app_login_email_enabled`(默认均 1)进「App 登录」分组,`/app-config` 下发 `auth.phone` / `auth.email`。**两个都关按两个都开处理**,避免把所有人锁在门外。
- **瓶子回信按条解锁**:`/bottle/reply/:rid/unlock`,每条锁着的回信挂「解锁 · 价」小标(原来是整瓶解锁)。

## 4. 资料与法务文本

- `Config.Value` 改 `longtext` —— 中英文协议正文塞不进 255,之前被静默截断。
- `/legal` 未登录时解析 **App 租户**(与 `/app-config` 一致)。此前读的是小程序租户,登录页永远显示「准备中」。
- `User.Birthday`(YYYY-MM-DD),年龄服务端算,只在本人资料里返回;**性别只能设一次**,再改返回业务错误并透传到前端。
- `/user/profile`、`/user/update`、App 登录响应改为共用同一个 self DTO,补上 `bottle_count` / `moment_count`(「我的」页此前所有人都显示 0 个瓶子)。
- 首次点赞给被赞者加魅力(`da0f47e`):**魅力只增不减**,取消赞不扣回;自增与 relation 行同事务,并顺手失效周榜缓存。
