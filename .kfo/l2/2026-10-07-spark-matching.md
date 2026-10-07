# 主动匹配(火花)

> 2026-10-06 ～ 2026-10-07 · L2 · 关联 [l1/spark](../l1/spark.md) [l1/chat-ws](../l1/chat-ws.md) [l1/robot](../l1/robot.md)
> 原始文档:`docs/superpowers/specs/2026-10-07-spark-matching-design.md` · `docs/superpowers/plans/2026-10-07-spark-matching.md`
> 实施 commit:`4faa381` `6fb35f7` `a7055e6` `2eae8e9` `86b6543` `9ad9675` `9d6b5b9`
> 状态:**COMPLETED**(总开关 `spark_enabled` 默认关,启用是**每租户的刻意动作**)

## 1. 目标

在线真人每隔随机 N–M 分钟被系统配一个人(按 `spark_real_ratio` 概率配真人、其余配机器人),
App 弹窗「XX 与你碰撞出了火花」,点进去**免开聊费**直接聊。仅 App。

## 2. 架构

新包 `internal/spark`。**纯函数(掷骰、间隔、时段、配对键)与副作用分离**,前者全部可测,
`Service` 用注入的依赖,所以调度器不碰 DB / Redis / WebSocket 也能测。

- **调度器 = 每分钟一次 tick + 每人一个 Redis TTL 键**(「下次匹配时刻」),不是每个在线用户一个 timer。
- `eligiblePeers` 过滤掉:自己、被拉黑、已配过、已达上限的人。
- `matchReal` 在 `ClaimDaily` 输掉竞争时**换下一个候选**,而不是整轮放弃 —— 一个额度耗尽的人不该堵住整轮。

## 3. 真人配对与机器人配对走两条路

| | 机器人 | 真人 |
|---|---|---|
| 会话 | **先建会话 + 发开场白,再弹窗**(用户进去时房间不是空的) | **不预建**,弹窗 payload 里没有 `chat_id` |
| 开聊 | 直接进 | 点击后调 `POST /spark/accept` |

⚠️ **`/spark/accept` 必须校验 Redis 里那条配对记录** —— 这是唯一拦住「任何人都能和任意人免费开聊、
绕过 `price_chat`」的东西。没匹配过的 peer_id 必须 403。

## 4. 与机器人搭讪共用日配额

`EnsureRobotChat` 更名 `EnsureFreeChat`(旧名留作薄包装),让火花与机器人主动搭讪共用同一个
**不扣费建会话**入口;`claimOutreach` 导出为 `ClaimDailyReach`,两者消费**同一个每人每日计数键**
`outreach:<租户>:<uid>:<日期>`,因此当天互斥。

**只免开聊费**:`price_msg` 照常收。

## 5. 配置(10 个键,全部默认关或空)

`spark_enabled` · `spark_interval_min` / `_max`(分钟) · `spark_real_ratio`(真人概率 %) ·
`spark_window_start` / `_end`(时段,含/不含) · `spark_user_daily_cap` ·
`spark_title` / `_en` · `spark_text` / `_en`(支持 `{nickname}`,空 = 用 App 内置文案)。

Redis 键全部带租户与日期前缀,当日级 TTL。

## 6. 四个会打死功能的坑

1. **`interval_min > interval_max`**(运营填反):`rand.Intn` 拿到非正数会 panic,**把整个常驻调度器静默打死**。`nextInterval` 自带交换与全 0 的守卫,表驱动测试覆盖。
2. **同一对真人在同一分钟被两个方向各扫到一次**:`pairKey` 与顺序无关 + 去重,只产生一次配对。
3. **WS 同时推来两条火花**(两个租户事件 / 重连补推):App **只弹一个,第二条丢弃不排队** —— 否则用户要连点两次才能回到原来的页面。
4. **WebSocket 不带 `Accept-Language`**:`SparkEvent` **两种语言都带**,语言在 provider 里解析(socket 建于登录时,否则会钉死当时的语言);服务端文案留空时回落 App 内置字符串,弹窗永远不会是空的。
