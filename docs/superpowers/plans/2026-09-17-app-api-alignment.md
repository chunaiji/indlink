# App 端接口对接实施文档

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-17 |
| 状态 | **第 1–5 项已实施**（2026-09-17），验证：`go build`/`go vet`/`flutter analyze`/`flutter test` 全过。剩 C1、通知组未做 |
| 前置 | 主链路（登录/资料/抛捞瓶/配额）已对接，见 `docs/APP_V1_PROGRESS.md` 二·五节 |
| 范围 | 剩余四组：聊天 / 动态 / 钱包·道具 / 通知 |

## 评审决议

| # | 议题 | 决议 |
|---|---|---|
| D1 | 解锁回信的维度 | **改客户端**：`unlockReplies(bottleId)` → `unlockReply(replyId)`，连带改详情页。按单条解锁是更合理的计费粒度 |
| D2 | 动态点赞 toggle vs set | **改客户端**：接住后端返回的 `liked`，不再本地取反 |
| C2 | `POST /ad/reward` | **暂不做**。广告要不要上还没定，先把客户端那条改成明确失败，别留一个会 404 的调用 |
| C3 | moment feed 分 tab | **暂不做**。tab 的产品语义（关注/最新/热门）未定，后端继续忽略该参数 |

---

## 一、问题的性质

不是「客户端写错了路径」，是**两套模型长在不同年代**：后端出自小程序时代（主键 `message_id`、对方信息平铺成 `partner_*`、图片是 JSON 字符串），App 照 V1 原型画（`id`、嵌套 `peer`、数组）。两边对各自的消费者都是对的。

**做法**：`internal/common/appdto` + 按 JWT 的 `platform=app` 分派，在出口转一次。小程序走原分支零改动。

```go
func isApp(c *gin.Context) bool { return middleware.Platform(c) == "app" }

if isApp(c) {
    response.OK(c, appdto.FromXxx(v))
    return
}
response.OK(c, v)   // 小程序原样
```

**不另开 `/app/*` 路由**：鉴权、限流、租户解析、内容安全都已挂在这批路由上，复制一份只会多一处会漂移的地方。

**客户端 `domain/models/*.dart` 原则上不动**——它按 UI 需要设计，让它迁就库表是本末倒置。

---

## 二、字段映射表（实施时照这个改）

### 2.1 聊天消息 `appdto.Message` ← `model.Message`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `message_id` | 改名 |
| `from_id` | `sender_id` | 改名 |
| `content` | `content` | 原样 |
| `type` | `type` | 原样（text/image/gift/system） |
| `created_at` | `created_at` | 原样 |
| `read_status` | `read_status` | 原样（bool，客户端两种都认） |
| `image` | **无** | `type=="image"` 时把 `content` 同时写进 `image`——后端图片 URL 就存在 content 里 |
| `gift`（对象） | **无** | `type=="gift"` 时 content 是 JSON `{name,icon,coins}`，解出来放进 `gift` |
| `qty` | **无** | 见 B2，随 qty 落库一起加 |

### 2.2 会话 `appdto.Conversation` ← `chat.ChatItem`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `chat_id`（内嵌 model.Chat） | 改名 |
| `peer`（对象） | `partner_id` / `partner_nickname` / `partner_avatar` / `partner_verified` | **聚合成嵌套对象**，键名用 `id`/`nickname`/`avatar` |
| `last_message` | `last_message` | 原样 |
| `last_at` | **无** | 用 `updated_at`（会话最后活动时间） |
| `last_type` | **无** | 库里没存。**建议**：`model.Chat` 加一列 `last_type`，发消息时一起写；不加就恒为 text，图片/礼物会话的列表预览会显示成文字 |
| `unread` | `unread_count` | 改名 |
| `from_bottle` | `source_bottle_id` | `!= 0` → true |

### 2.3 动态 `appdto.Moment` ← `model.Moment`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `moment_id` | 改名 |
| `content` | `content` | 原样 |
| `author`（对象） | **无** | 按 `user_id` 批量查用户，聚合成对象 |
| `images`（数组） | `images` | ⚠️ **库里是 JSON 数组字符串**。直接下发的话客户端 `stringList` 会按逗号切，得到 `["[\"a\"", "\"b\"]"]` 这种垃圾。**必须 `json.Unmarshal` 成真数组** |
| `like_count` / `comment_count` | 同名 | 原样 |
| `created_at` | `created_at` | 原样 |
| `liked` | **无** | 批量查 `MomentLike`，一条 `IN` |
| `following` | **无** | 批量查 `Relation(type=like)` |
| `city` | **无** | 关联 `users.city`，或先给空 |

### 2.4 动态评论 `appdto.MomentComment` ← `model.MomentComment`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `comment_id` | 改名 |
| `content` | `content` | 原样 |
| `author`（对象） | **无** | 同上，按 `user_id` 聚合 |
| `created_at` | `created_at` | 原样 |
| `gift`（对象） | **无** | `type=="gift"` 时 content 是 JSON `{name,icon,coins}`，解出来 |

### 2.5 礼物 `appdto.Gift` ← `model.Item`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `item_id` | 改名（注意后端这个字段**没有** `,string`，是纯数字；客户端 `idOf` 能收） |
| `name` / `icon` | 同名 | 原样 |
| `coins` | `price_coin` | 改名（客户端也认 `price`） |
| `charm` | **无** | 库里没有这一列。要么 `model.Item` 加列，要么按 `price_coin` 等值折算——**推荐加列**，魅力值和售价解耦才好运营 |

### 2.6 钱包流水 `appdto.WalletTxn` ← `model.WalletTxn`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `txn_id` | 改名 |
| `amount`（**带符号**） | `direction`(credit/debit) + `coins`（恒正） | `direction=="debit" ? -coins : coins`。客户端靠正负判断收入/支出 |
| `scene` | `scene` | 原样，枚举值已对齐 |
| `balance_after` | `balance_after` | 原样 |
| `created_at` | `created_at` | 原样 |
| `note` | **无**（只有 `biz_no`） | `biz_no` 是幂等键不是给人看的。按 `scene` 生成一句人话，或先给空 |

### 2.7 充值档位 `appdto.RechargePackage` ← `model.CoinPackage`

| 客户端要 | 后端有 | 转换 |
|---|---|---|
| `id` | `package_id` | 改名 |
| `coins` | `coins` + `bonus_coins` | 客户端要**到账总数**，下发 `coins + bonus_coins` |
| `price_label` | `price_fen`（分） | 后端格式化成带货币符号的串。⚠️ 印度要卢比不是人民币，格式化口径按租户定 |
| `bonus_percent` | 无 | `bonus_coins * 100 / coins` |
| `ios_product_id` | `ios_product_id` | 原样，iOS 走 IAP 时用 |

### 2.8 通知 / 举报 —— 基本已对齐

`model.Notification` 的 `id`/`type`/`title`/`body`/`read`/`created_at`/`ref_id` 与客户端一致，估计不用改。
`model.Report` 的 `reason`（porn/spam/harassment/minor）与 `target_type` 也对得上。
⚠️ 客户端 `engagement.dart` 的 `NotificationItem`/`RelationItem`/`CheckinStatus` **尚未逐字段核对**，做到这一步时再对。

---

## 三、后端要补的能力（不只是改名）

| # | 事项 | 说明 |
|---|---|---|
| **B1** | `liked` / `collected` | 瓶子详情与列表都缺。按 `bottleIDs` 批量查 `Collection` + `MatchLog(action=like)`，一条 `IN` 查询。**不要在循环里查**——`bottle.Mine` 已有现成的批量聚合写法可抄 |
| **B2** | **chat 送礼不认 `qty`** | `giftReq` 只有 `ItemID`。现状是连送 ×10 **只送 1 个、只扣 1 个的币**。加 `Qty int`，扣币与 Charm 累加乘以数量，并做上限校验 |
| **B3** | profile 的 `bottle_count`/`moment_count`/`relation_stage` | 前两个 count 查一下；`relation_stage` 取 `Relation.Stage`，只在看别人主页时有意义 |
| **C1** | 「我捞过的瓶子」 | App 该 tab 现在返回空列表。数据在 `MatchLog(action=view)`，缺查询入口。**给 `/bottle/mine` 加 `type=thrown\|scooped`**，比新开路由省事 |
| **可选** | `model.Chat` 加 `last_type` | 见 2.2。不加的话会话列表里图片/礼物消息会预览成文字 |
| **可选** | `model.Item` 加 `charm` | 见 2.5 |

---

## 四、App 端改动清单

| 文件 | 改什么 |
|---|---|
| `data/remote/remote_repositories.dart` | 主战场。chat/moment/wallet/notify 四组的 path 与字段。**`sendImage` 现在传 `{'image': url}`，后端只认 `content`** |
| `data/repositories.dart` | D1：`unlockReplies(String bottleId)` → `unlockReply(String replyId)`，返回 `BottleReply` |
| `data/mock/mock_repositories.dart` | 跟着接口签名改，保持 mock 可用（离线改 UI 靠它） |
| `features/bottle/bottle_detail_page.dart` | D1 连带：解锁按钮改成按单条回信调用 |
| `features/moment/moment_controller.dart` | D2：点赞接住返回的 `liked`，不再本地取反 |
| `features/me/wallet_page.dart` | 确认流水正负号显示（后端下发带符号 amount 后） |
| 广告入口 | C2：改成明确提示「暂未开放」，不再发那个 404 请求 |

---

## 五、实施顺序

按「用户能不能用」排，不按工作量排：

1. **B2 送礼 qty** —— 唯一一条资损方向的 bug，改动也最小，先修
2. **聊天组** —— 2.1 + 2.2 + 客户端适配。App 的核心留存功能，没有聊天其余都是空转
3. **B1 liked/collected** —— 主链路的尾巴，影响捞瓶后的收藏与点赞
4. **动态组** —— 2.3 + 2.4 + D2 + 客户端
5. **钱包·道具** —— 2.5 + 2.6 + 2.7
6. **C1 我捞过的** —— 独立 tab，缺了不阻断别的
7. **通知** —— 2.8。下发链路本来就没做（缺 FCM 凭证），优先级最低

---

## 五·五、实施记录（2026-09-17）

第 1–5 项已完成。实施中改掉的、规划时没预料到的：

| 发现 | 处理 |
|---|---|
| **送礼是库存制不是扣币**。规划里写的「只扣 1 个的币」不准确——实际是消耗 `ItemOrder` 里一件自购库存 | 按 qty 批量删库存，`RowsAffected < qty` 整单回滚（先删一部分再报错会吞库存）；charm 累加 ×qty；上限 99 |
| **`item_id` / `package_id` 在后端是 `int64` 且没有 `,string` tag** | 与 ID 一律字符串的约定**相反**。客户端这三处必须发数字，否则 JSON 解析直接 400。加了 `_itemId()` 收口 |
| **`model.Item` 不需要加 charm 列** | `chat.SendGift` 累加魅力值用的就是 `PriceCoin`——charm 本来就等于售价。加列反而会和发放逻辑对不上 |
| **`GET /pay/packages` 不挂鉴权**，拿不到 platform | 这一组分派不了，改由客户端 `RechargePackage.fromJson` 兼容两种形状。已删掉写了一半的 `FromPackages`，不留死代码 |
| ~~`/item/buy` 没有「次数包」品类~~ **这条我判断错了** | `model.Item.Type` 的注释写着 boost/top/superlike/gift，但那是过时注释——`migrate.go` 的 seed 里就有 `quota_throw`/`quota_scoop`，`item.Buy` 也会调 `quota.AddPack` 发次数。**`buyScoopPack` 已修好**：先从 `/item/list` 找 `type=quota_scoop` 的商品再买，不硬编码 seed 的 ID 7。顺手把那句过时注释也改了 |
| **`/relation/like` 没有取关** | 单向 like，传 `liked:false` 会被当成再 like 一次。已标注，未擅自加接口 |
| **`model.Chat` 加了 `last_type` 列** | 规划里标「可选」。不加的话图片消息在会话列表显示成一串原始 URL，决定加。AutoMigrate 自动加列，小程序不读，零影响 |

仍然坏着、已在代码里标 ⚠️ 的：`unlockReplies`（D1 待改客户端）、`buyScoopPack`、`setFollow` 的取关、`recharge`（只下单没唤起支付）、moment feed 的 tab。

## 六、风险与固定成本

- **这份清单是静态核对的产物**。时间格式、空值、枚举取值这类问题只有真连上才会暴露，实施时一定还会冒出几条
- **每轮改动都要部署才能验证**。App 连的是线上 `ambertu.com`，改完后端不 `node deploy.js` 就等于没改。这是选线上联调的固定成本
- **线上联调会往生产库写测试数据**，全部落在 `tenant_id=358804313465688064` 下，事后要清
- **线上还缺 `APP_DEFAULT_TENANT_ID=358804313465688064`**，不配的话登录直接返回「未配置 App 凭证」，一切免谈
- `discover` 组是本轮为 App 新写的，抽查下来路径、逗号分隔、gender 取值、`{list}` 包装全都对得上，估计不用改——但没跑过，不打包票
