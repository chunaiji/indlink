# L1 设计 — 互动通知(notify,#4)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-21 |
| 覆盖 | `server/internal/notify/*` · `model.Notification` · `bottle.Service.OnReplied` · 前端 `pages/notifications` + 消息页 |
| 关联 | 规划 `l2/2026-06-21-feature-roadmap.md`(#4)· `l1/chat-ws.md`(WS Hub)· `l1/admin-platform.md`(模板配置)|

---

## 一、定位

别人回信你的瓶子(及后续点赞等)→ **站内信落库 + 在线 WS 实时推 + 红点未读**;微信「订阅消息」(离线触达)留下发钩子。

## 二、数据 / 接口

- `Notification(user_id 接收者, type[reply/like/system], ref_id 关联瓶子, title, body, is_read, created_at)`。
  - ⚠️ 列名 `is_read` 而非 `read` —— `read` 是 MySQL 保留字,原始 `WHERE read=?` 会静默报错(踩过,见 L4)。
- `GET /notify/unread`(红点数)· `GET /notify/list`(拉取即标记全部已读)· `POST /notify/read`(id 为空=全部已读)。

## 三、触发与解耦

- `bottle.Service.OnReplied func(tenantID, ownerID, bottleID, replierID)` 可选回调字段,`main` 注入 `notify.OnBottleReplied`;`Reply` 成功后调用 → 通知瓶主(回信者自己不通知)。**bottle 不依赖 notify 包**(回调解耦)。
- `notify` 通过 `Pusher` 接口(`IsOnline/PushTo`,由 `chat.Hub` 实现)在线实时推,**不反向依赖 chat**。

## 四、实时推送

用户在线 → `hub.PushTo(userID, {type:"notify", notify_type, ref_id, title, body})`。前端 `onWSMessage` 收到 `type==="notify"` → 消息页红点 +1;进「互动通知」页清零并标记已读。

## 五、微信订阅消息(离线触达)——钩子,待接

`SendWxSubscribe` 当前为 stub:配置了 `sysconfig.notify_wx_template_id` 才尝试,且仅日志记录。真实下发待补:① 申请模板 ID;② 客户端关键动作 `requestSubscribeMessage` 拿一次性授权 + 存额度表;③ 取 `access_token` → `POST cgi-bin/message/subscribe/send`。access_token 基建尚未建,故留 TODO。

## 六、验证(2026-06-21)

A 扔瓶 → B 回信 → A `unread=1`、列表含「收到新回信 💌」、拉列表后 `unread=0`、自己回自己不产生通知。✅
