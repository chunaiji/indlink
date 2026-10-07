# 留存增长四功能规划：深夜瓶 / 阶梯签到 / 礼物墙+周榜 / 漂流轨迹

日期：2026-08-10
状态：规划（未实施）
目标：给扔瓶人/送礼人/日活用户各补一个回访钩子，全部复用现有基建（sysconfig 远程配置、push activity 模板、ItemOrder、tags、notify、激励视频）。

## 通用约束（全部功能适用）

1. **双形态合规**：所有新增用户可见入口必须挂 sysconfig 远程开关，默认关闭；工具形态（水印相机）下绝不露出。
2. **sysconfig 新 key 必须同步加 defaults**（历史 bug：空串会导致开关逻辑反转，见 feedback_sysconfig_defaults）。
3. ID 一律字符串下发（JS int64 精度）。
4. 多租户：所有新表带 tenant_id，所有统计按租户隔离。

---

## F1 深夜瓶（每晚 22:00 固定栏目）

**玩法**
- 每晚 22:00 – 次日 02:00 为「深夜场」（时段可配）。
- 窗口内扔瓶可选「深夜瓶」类型；深夜瓶只在夜场时段能被捞到（稀缺感），普通时段不进捞瓶池。
- 捞瓶在夜场时段优先捞深夜瓶池（当晚 + 近 3 晚未过期的）。
- 每晚 21:55 向订阅用户发订阅消息「深夜瓶已开启」。

**数据**：零迁移。复用 Bottle 现有 tags 体系，深夜瓶打 `night` 标签。

**后端改动**
- `bottle/service.go` Create：请求带 `night=true` 且当前在夜场时段 → tags 追加 night。
- Scoop/ScoopOne：夜场时段优先查 night 标签池，非夜场排除 night 瓶。
- 定时推送：参照签到定时任务先例，新增每日 21:55 cron，走 push 模块 activity 模板群发（`SendByScene("activity")` 已有群发逻辑）。
- sysconfig 新 key（含 defaults）：`night_bottle_enabled`（默认 0）、`night_start`（默认 22）、`night_end`（默认 2）、`night_push_title` 文案。

**前端改动**
- ocean 页：夜场时段顶部浮现「🌙 深夜场」入口徽标（主题已有 night 模式，22 点自动切换已可用）；扔瓶写纸条面板加「深夜瓶」勾选（仅夜场显示）。
- 捞到深夜瓶时瓶子气泡加深夜样式标识。
- 推送订阅：复用现有 pushApi.subscribe，场景 activity。

**工作量**：后端 1 天（含 cron 与池逻辑）+ 前端 0.5 天。

---

## F2 签到 7 天阶梯

**玩法**
- 连续签到 7 天一轮，奖励递增，第 7 天大奖，第 8 天开始新一轮。
- 默认数值 `5,5,10,10,15,15,30`（sysconfig 可配）。
- 断签 1 天内可通过「看激励视频」补签（每月上限可配）；断签超 1 天从 Day1 重来。

**数据**：零迁移。CheckinLog 已有 user+date 唯一索引，连续天数用 date 倒推查询（最多回查 7 条）；补签 = 插入缺失日期的记录。

**后端改动**
- `checkin/service.go`：
  - `streak(userID)`：从昨天起逐日回查 CheckinLog 计算连续天数。
  - Status 返回增加：`streak`、`ladder`（七档数组）、`today_index`、`can_makeup`（昨天缺签且本月补签次数未超限）。
  - Sign：按 `(streak % 7)` 档位发币（沿用现有钱包入账逻辑）。
  - 新增 `POST /checkin/makeup`：校验激励视频奖励回调（复用 ad 模块 reward 校验思路，scene 用 `makeup_checkin`），插入昨日记录（币按该档位）。
- sysconfig 新 key（含 defaults）：`checkin_ladder`（默认 "5,5,10,10,15,15,30"）、`checkin_makeup_limit`（默认 2/月）。

**前端改动**
- mine 页签到卡扩为 7 格进度条（已签格 ✓、今天高亮、Day7 礼盒图标），下方「看视频补签」入口（can_makeup 时显示）。
- 复用现有激励视频组件（ads store）。

**工作量**：后端 0.5 天 + 前端 0.5 天。四个功能里性价比最高，建议最先做。

---

## F3 礼物墙 + 魅力周榜（顺带产出轻量资料卡）

**玩法**
- **用户资料半屏弹层（user-card 组件）**：头像/昵称/性别年龄城市/Bio/魅力值/礼物墙（收到的礼物按数量聚合，最多展示 8 种）+ 喜欢按钮 + 开聊按钮。同城/扩列/聊天页点头像打开——同时补上「先看人再付费开聊」的漏斗断层。
- **魅力周榜**：本周（周一零点起）收礼金币数 Top 20，扩列墙顶部横幅入口进入。上榜展示周魅力值与头像，头名给「👑 本周魅力之星」。

**数据**：零迁移。礼物墙与周榜均从 ItemOrder 聚合（target_id 维度）；User.Charm 是累计值仅做展示，周榜必须按周聚合 ItemOrder。

**后端改动**
- 新增 `GET /user/:id/card`：公开资料（昵称/性别年龄/城市/Bio/魅力/粉丝）+ 礼物墙聚合（ItemOrder where target_id group by item）。注意脱敏：不下发 openid/手机等。
- 新增 `GET /rank/charm?period=week`：`SELECT target_id, SUM(coins) FROM item_orders WHERE created_at >= 本周一 GROUP BY target_id ORDER BY sum DESC LIMIT 20`，Redis 缓存 5 分钟（redis 已接）。
- 机器人参与：榜单默认包含机器人（冷启动占位、也是送礼诱导）；后台开关可切「仅真人」。
- sysconfig 新 key（含 defaults）：`user_card_enabled`（默认 0）、`charm_rank_enabled`（默认 0）。

**前端改动**
- 新组件 `components/user-card`（半屏弹层）；同城/扩列/聊天页头像 @tap 接入。
- 新页面 `pages/rank/rank`（周榜）；扩列墙顶部入口横幅（开关控制）。
- 礼物墙空态引导文案：「还没有礼物，做 TA 的第一个送礼人」→ 点击直接拉起送礼面板（刺激复购的关键一跳）。

**工作量**：后端 1 天 + 前端 1.5 天（组件 + 榜单页 + 三处接入）。

---

## F4 瓶子漂流轨迹

**玩法**
- mybottles 页每只扔出的瓶子显示「漂过 N 座城 · 被捞 N 次」摘要，点开看时间线：`漂出 → 被广州的人捞起 → 收到 1 条回应 → 被杭州的人捞起…`。
- 捞瓶人只显示城市 + 匿名称呼（复用 uiText.anonFriend），不暴露身份。
- 当天瓶子首次被捞时，给扔瓶人发一条**站内互动通知**（notify 已有，当天聚合一条防骚扰），不发订阅消息。

**数据**：新表（当前捞瓶不落库，这是轨迹的前置）：

```go
// BottleScoopLog 捞瓶记录(漂流轨迹数据源)
type BottleScoopLog struct {
    ID        int64     `gorm:"primaryKey;autoIncrement"`
    TenantID  int64     `gorm:"index"`
    BottleID  int64     `gorm:"index:idx_bottle_time"`
    UserID    int64     // 捞瓶人(轨迹展示时脱敏)
    City      string    `gorm:"size:32"` // 捞瓶人城市快照
    CreatedAt time.Time `gorm:"index:idx_bottle_time"`
}
```

**后端改动**
- Scoop/ScoopOne 成功后异步插 BottleScoopLog（goroutine，不拖慢捞瓶主链路）+ 当天首捞发 notify。
- 新增 `GET /bottle/:id/trace`（仅瓶主可查）：scoop_logs UNION 回复（replies 已有表）UNION 点赞，按时间排序；头部聚合 distinct(city)、count(scoop)。
- `bottle/mine` 列表返回增加 `scoop_count`、`city_count` 两个聚合字段。
- 表注册进 bootstrap AutoMigrate。
- sysconfig 新 key（含 defaults）：`bottle_trace_enabled`（默认 0）。

**前端改动**
- mybottles 列表项加轨迹摘要行 + 「查看轨迹」入口。
- 轨迹时间线半屏弹层（纵向时间轴，节点：捞起/回应/点赞，城市高亮）。

**工作量**：后端 1 天 + 前端 1 天。注意：上线后轨迹从零开始积累（历史捞瓶无记录），属预期。

---

## F5 朋友圈(动态广场)——文字+图片动态,评论互动

状态:已规划未实施(2026-08-10 追加)

**现状基础**:moment 模块已有——发文字动态(visible: public/private)、我的动态、删除、点赞(MomentLike 唯一索引防重)。缺:图片、公开流、评论、通知、后台管理。

**玩法**
- 发布:文字(500 字) + 最多 9 张图(九宫格);走现有 /upload 通道。
- 广场:公开动态流,时间倒序分页;卡片 = 头像/昵称 + 文字 + 图片九宫格 + 点赞 + 评论数 + 最新 2 条评论预览。点头像弹 user-card 资料卡(F3 组件复用,联动开聊转化)。
- 评论:动态下评论,支持回复某人(单层,不做多级嵌套);作者与评论者可删自己的。
- 通知:被评论 → notify 站内信;被点赞按天聚合一条,防骚扰。

**数据**(AutoMigrate,低风险)
- Moment 加列:`Images string`(JSON 数组存图 URL,空=纯文字)、`CommentCount int`。
- 新表 MomentComment:comment_id/tenant_id/moment_id(索引)/user_id/reply_to_user_id(回复某人,0=直接评论)/content/created_at。

**接口**
- `POST /moment` 扩展 images 参数(≤9,校验来自本站上传域名)。
- `GET /moment/feed` 公开流:visible=public、排除拉黑双向、分页;批量带作者信息/我是否已赞/评论预览(前 2 条),避免 N+1。
- `GET /moment/:id/comments` 评论分页;`POST /moment/:id/comment`(moderation.CheckText + 限流,复用回信限流器);`POST /moment/comment/:id/remove`(评论者本人或动态作者可删)。
- 通知:评论成功回调 notify.Create(type=moment_comment)。

**前端**
- 扩列墙页顶部 segtabs 加「动态」维度(扩列/动态二选一切换,零 tabBar 改动,复用页面骨架)。
- 发布入口:广场右下角浮动 ➕;moments(我的动态)页同步支持选图(uni.chooseImage → 复用 utils/upload 批量传)。
- 评论区:卡片点"评论"展开半屏(参照轨迹弹层交互)。
- 图片预览 uni.previewImage。

**合规(重点)**
- 文字:发布与评论都走 moderation.CheckText。
- 图片:上传通道现状仅存储无机审——上线初期靠后台人工巡查+举报兜底;后续接微信 imgSecCheck(media check 异步回调)再全量开放。**规划为两阶段:一期图片功能仅对 is_verified 真人认证用户开放,降低风险面**。
- admin 后台:新增「动态管理」页(参照瓶子管理 Bottles.vue):动态列表/删动态/删评论,默认筛真人。
- 开关:features 加 `square_enabled`(sysconfig `square_enabled`,默认 0,含 defaults);工具形态不露出。

**排期**:后端 1.5 天(表+feed+评论+通知+admin) + 前端 2 天(广场 tab+九宫格发布+评论区) ≈ 3.5 人天。依赖:user-card(F3,已上线)。

### F5.1 动态送礼(打赏)——2026-08-10 追加规划

**目标**:给动态作者送礼物,打通「内容 → 打赏 → 魅力值/礼物墙/周榜」消费闭环;送礼记录公开展示带动跟风。

**现状可复用**:gift 类道具目录(Item type=gift)、扣费+ItemOrder(自动计入 F3 礼物墙与魅力周榜)、Charm 累加与通知参照聊天 SendGift(chat/service.go:104);注意 item.Buy 赠送**不加 Charm 不通知**,不能直接复用,需新接口。

**玩法**
- 入口两处:动态卡操作行 ❤️ 💬 旁加 **🎁 送礼**;评论弹层输入框旁 🎁。
- 点击弹「礼物选择半屏面板」:gift 道具横向网格(图标/名称/价格)+ 当前余额 + 赠送按钮;余额不足引导充值。
- 送出后:自动在该动态评论区插入一条**礼物评论**(type=gift,如"送出了一个「玫瑰」🌹",样式高亮),公开可见制造跟风;作者收到站内通知「收到礼物 🎁」。
- 礼物自动计入作者魅力值、资料卡礼物墙、魅力周榜(零额外开发,数据同源 ItemOrder)。

**数据**:MomentComment 加 `Type string`(text/gift,默认 text,AutoMigrate 加列);礼物流水复用 ItemOrder,无新表。

**后端**
- `POST /moment/:id/gift {item_id}`:校验 item type=gift → wlt.Debit 事务(ItemOrder + gift 评论 + comment_count+1) → User.Charm += 价格 → OnGifted 回调通知作者。bizNo 幂等,参照 SendGift。
- 开关:跟随 square_enabled,不单独加 key。

**前端**
- 新组件 gift-picker(半屏):itemApi.list 筛 type=gift;确认赠送 → 调 /moment/:id/gift → 刷新余额 + 评论区插入礼物评论。
- 动态卡/评论弹层加 🎁 入口;礼物评论特殊样式(金色底,🎁 前缀)。
- 聊天页现有送礼面板如有可复用的样式一并对齐。

**工作量**:后端 0.5 天 + 前端 1 天。依赖 F5(已上线)。

## F7 互动通知跳转闭环——2026-08-10 追加规划

**问题**:通知 4 类只有 reply 可点(跳收件详情);scoop/moment_comment/moment_gift 点击无反应,通知的回访价值断在最后一步。

**核心缺口**:无单条动态详情页——动态只存在于广场 feed 流,通知/未来的分享都无法定位到具体一条。

**方案**

1. **新增动态详情页** `pages/moment-detail/moment-detail?id=`(通知跳转的落点,也是未来分享卡片的落点):
   - 布局复用广场卡片:作者头像(点开资料卡)/文字/九宫格图/点赞/送礼
   - 评论区页面内嵌完整列表(非弹层)+ 底部常驻评论输入框 + 🎁
   - 动态已删除/不可见 → 空态「动态已经不在了」
   - 后端配套 `GET /moment/:id`:单条 FeedItem(作者信息+我是否已赞),校验 visible=public 或本人

2. **通知路由分发**(notifications.vue open):

   | type | 跳转 |
   |------|------|
   | reply 收到回信 | /pages/detail/detail?id=bottle_id(已有) |
   | scoop 瓶子被捞 | /pages/mybottles/mybottles?trace_id=bottle_id,进入后自动弹出该瓶的漂流轨迹 |
   | moment_comment 被评论 | /pages/moment-detail/moment-detail?id=moment_id |
   | moment_gift 收到礼物 | 同上 |

3. **通知行视觉升级**:图标按类型区分(💌回信/🌊被捞/💬评论/🎁礼物,礼物金色底);可跳转的行尾加「›」;点击行有按压反馈(hover-class)。

**注意**:mybottles 需支持 onLoad 接 trace_id 参数 → 列表加载完自动打开对应瓶子的轨迹弹层;事件绑定全部用调用表达式(编译坑③)。

**工作量**:后端 0.5 天(单条接口) + 前端 1 天(详情页+路由分发+通知行升级)。依赖 F5/F5.1(已上线)。

## 排期与依赖

```
第 1 批  F2 阶梯签到     （1 天）   独立,最快见效
第 2 批  F4 漂流轨迹     （2 天）   独立;scoop 落库越早上线数据越早积累
第 3 批  F1 深夜瓶       （1.5 天） 独立;需要一个 cron
第 4 批  F3 礼物墙+周榜  （2.5 天） user-card 组件是后续"完整资料卡/心动匹配"的地基
```

总计约 7 人天。四者相互独立可并行/重排；建议按上表顺序（先小后大、数据积累类优先）。

## 验证清单（每批上线前）

- [ ] 新 sysconfig key 已加 defaults，且开关关闭时入口完全不可见（工具形态回归）
- [ ] 微信/支付宝双端各过一遍新入口（tap 必须写成调用表达式 `fn()`，uni-app alpha 版裸方法名不触发）
- [ ] 多租户：切非默认租户验证统计/榜单隔离
- [ ] admin 后台 Config 页能改到新 key；涉及数值的（签到阶梯）改完即时生效
