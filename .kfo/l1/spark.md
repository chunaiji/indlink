# L1 设计 — 主动匹配(spark / 火花)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-10-07 |
| 覆盖模块 | `internal/spark`(`rules.go` / `service.go` / `deliver.go` / `handler.go`) |
| 关联 | L1 `l1/chat-ws.md`(WS 推送与开聊扣费)· `l1/robot.md`(搭讪共用配额)· `l1/match.md`(另一条「找人」线)· L2 [2026-10-07-spark-matching](../l2/2026-10-07-spark-matching.md) |

---

## 一、职责

给**在线真人**每隔随机 N–M 分钟配一个人,App 弹窗「XX 与你碰撞出了火花」,点进去**免开聊费**。
仅 App;总开关 `spark_enabled` **默认关**。

与 `match`(用户主动翻列表)、`bottle.feed`(找内容)的区别:**系统主动推给你**。

## 二、调度模型

```
每分钟 tick → 扫 WS 在线用户 → 对每人查 Redis「下次匹配时刻」键 → 到点才配
```

**不是每个在线用户一个 timer** —— 常驻 goroutine + 每人一个 Redis TTL 键,实例重启不丢节奏,
在线用户数增长也不长出成千上万个 timer。

纯函数(掷骰 `kind`、间隔 `nextInterval`、时段 `inWindow`、配对键 `pairKey`)全部在 `rules.go`,
与副作用分离;`Service` 的依赖是注入的,所以不碰 DB / Redis / WS 也能测。

## 三、候选过滤与竞争

`eligiblePeers` 排除:自己、被拉黑、已配过、已达每日上限。
`matchReal` 在 `ClaimDaily` 输掉竞争(额度被另一个 tick 抢走)时**换下一个候选**,不是整轮放弃。

`pairKey` **与顺序无关** + 去重 —— 同一对真人在同一分钟被两个方向各扫到一次时,只产生一次配对。

## 四、两条投递路径

| | 机器人(概率 `100 - spark_real_ratio`) | 真人(概率 `spark_real_ratio`) |
|---|---|---|
| 会话 | **先建会话 + 发开场白,再弹窗** | **不预建**;payload 里没有 `chat_id` |
| 双方 | 单向 | 双向弹窗 |
| 进入 | 直接进 | 点击调 `POST /spark/accept` |

> ⚠️ **`/spark/accept` 必须校验 Redis 里的配对记录**(没匹配过 → 403)。这是唯一拦住
> 「任何人都能和任意人免费开聊、绕过 `price_chat`」的东西。

## 五、与机器人搭讪的关系

- 不扣费建会话的唯一入口是 `chat.EnsureFreeChat`(原 `EnsureRobotChat`,旧名留作薄包装)。
- 每人每日计数键 `outreach:<租户>:<uid>:<日期>` 由 `ClaimDailyReach`(原 `robot.claimOutreach`)
  统一消费 —— 火花与机器人搭讪**共用配额,当天互斥**,用户不会一天被两套系统轮番骚扰。
- **只免开聊费**,`price_msg` 照常收。

## 六、配置(10 键,全部默认关或空)

| 键 | 含义 |
|---|---|
| `spark_enabled` | 总开关 |
| `spark_interval_min` / `_max` | 每人下次匹配的间隔(分钟) |
| `spark_real_ratio` | 配到真人的概率(%),其余配机器人 |
| `spark_window_start` / `_end` | 时段小时,含 / 不含 |
| `spark_user_daily_cap` | 每人每日上限(与 outreach 共用计数) |
| `spark_title` / `_en` · `spark_text` / `_en` | 弹窗文案,支持 `{nickname}`;**空 = 用 App 内置文案** |

⚠️ `interval_min > interval_max`(运营填反)会让 `rand.Intn` 拿到非正数 panic,
**把整个常驻调度器静默打死** —— `nextInterval` 自带交换与全 0 守卫。

## 七、客户端

- **WebSocket 不带 `Accept-Language`** → `SparkEvent` 两种语言都带,语言在 provider 里解析
  (socket 建于登录时,在 socket 里取会钉死当时的语言)。
- **同时只弹一个**,第二条火花丢弃而不排队 —— 否则用户要连点两次才能回到原来的页面。
- 服务端文案为空时回落内置字符串,弹窗永远不会是空的。
