# 漂流瓶社交小程序 · 设计文档(V1)

> 版本:V1 草案 · 日期:2026-06-20
> 方向:**变现型**(以参考截图为准),跨平台(微信 + 支付宝),后端 Go,含支付。

---

## 0. 一句话定位

一个基于漂流瓶机制的陌生人轻社交小程序:用户扔瓶 / 捞瓶建立连接,通过**金币充值 + 站内消费(开聊、解锁回信、道具礼物)**变现。一套 uni-app 代码同时发布微信、支付宝;后端为 Go 模块化单体。

---

## 1. 核心决策(已与产品确认)

| 维度 | 结论 |
|---|---|
| 产品方向 | 变现型,以截图为准。视觉:**白天沙滩 + 夜间星空**双主题 |
| 变现机制 | 金币充值+消费、增值道具/礼物、真人认证(**开关控制**,非必需);**V1 不做 VIP 订阅** |
| 前端框架 | **uni-app**(Vue 语法),条件编译适配微信/支付宝 |
| 后端 | **Go(Gin)+ 模块化单体**(非微服务),2C4G 可跑 |
| 数据 | MySQL 8.0(主)+ Redis(缓存/在线)+ 对象存储(COS/OSS) |
| 实时 | gorilla/websocket |
| V1 范围 | 登录 / 扔瓶·捞瓶 / 回信→聊天 / 消息 / 我的 / 金币充值+扣费 / 认证开关 / 同城列表+开聊扣币 / 扩列墙(活跃·新人·附近) |

### 1.1 V1 明确不做(YAGNI)
- VIP 会员订阅体系
- 情绪画像 `emotion_profile`、用户标签画像 `user_tag`、行为训练日志 `interaction_log`(文档 V2/V4 的高级推荐能力)
- AI 种子瓶生成、AI 回信
- 兴趣房间、动态广场(扩列墙先顶替"发现"需求)
- 双平台账号合并(V1 两端各算独立账号,见 6.1)

---

## 2. 合规约束(影响支付设计,务必先读)

1. **微信小程序**:虚拟币(金币)充值属"虚拟支付",**iOS 端被微信禁止**走微信支付充值;安卓可走微信支付。处理:iOS 端按开关隐藏充值入口。
2. **支付宝小程序**:陌生人社交类目审核较严,虚拟币相对宽松,需匹配对应资质类目。
3. 两端支付都必须走**各自平台官方支付**(微信支付 / 支付宝支付),不可用第三方聚合绕过。
4. **充值 = 真实支付**(经平台异步回调入账),**消费 = 站内金币扣减**,两者严格分离。

**设计应对:** 支付做成"网关适配层(driver) + 统一订单/钱包/流水";前端按运行平台 + `config` 开关决定充值入口是否展示。合规风险隔离在一层内,**改开关不发版**。

---

## 3. 前端设计(uni-app)

### 3.1 底部 5 Tab
| Tab | 页面 | 核心内容 |
|---|---|---|
| 🌊 海洋 | 漂流瓶主场景 | 双主题;捞一个 / 扔一个 / 我的瓶子;海面漂浮动画 |
| 🔍 同城 | 同城用户列表 | 城市/性别筛选;用户卡片 +「开聊 −5金币」 |
| ➕ 扩列(中间凸起) | 扩列墙 | 活跃 / 新人 / 附近 三子 Tab;卡片 + 喜欢 |
| 💬 消息 | 消息中心 | 未读回信、聊天列表、系统消息(注册奖励等) |
| 🐱 我的 | 个人中心 | 钱包(余额+充值)、我喜欢/喜欢我/看过我、认证中心、相册、设置 |

### 3.2 关键二级页面
- **扔瓶子页**:输入(文字/语音/图片)、标签、匿名开关、同城/全国、可见时长(24h/7d/永久)→「扔进海里」
- **收件详情页**:瓶内容 + 评论(「仅作者可看」付费解锁)+「打招呼/回信」
- **聊天页**:WebSocket 实时;快捷回信按钮(我懂你 / 我也经历过 / 说说细节? / 继续聊)
- **充值页**:金币套餐档位、微信/支付宝支付(**iOS 条件隐藏**)
- **认证中心**:实名 + 真人头像(受后台开关控制)

### 3.3 工程结构
```
client/
├─ pages/        # ocean, city, expand, message, mine + 二级页
├─ components/   # BottleCard, UserCard, ChatBubble, CoinPanel...
├─ store/        # pinia: user, wallet, chat, theme
├─ api/          # 按模块封装的请求
├─ utils/        # request, ws, platform/{login,pay}, theme
└─ static/       # 双主题素材
pages.json       # 路由
manifest.json    # 微信 appid + 支付宝 appid
```

### 3.4 跨平台抽象(差异只锁一层)
| 差异点 | 微信 | 支付宝 | 封装 |
|---|---|---|---|
| 登录 | `wx.login` | `my.getAuthCode` | `utils/platform/login.js` → `getLoginCode()` |
| 支付 | `wx.requestPayment` | `my.tradePay` | `utils/platform/pay.js` → `requestPay(params)` |
| 系统信息 | `uni.getSystemInfo`(已抹平) | 同左 | 直接用 uni |
| 录音/上传/文件 | uni 内置已抹平 | 同左 | 直接用 uni |

- 业务代码**永不出现** `wx.` / `my.`,只调封装函数。
- 双主题用 CSS 变量 + `theme` store,按本地时间或用户设置切换。
- 构建:`dev:mp-weixin` / `dev:mp-alipay` 双产物。

---

## 4. 后端设计(Go 模块化单体)

### 4.1 目录结构
```
server/
├─ cmd/api/main.go            # 入口,装配所有模块
├─ internal/
│  ├─ user/        # 登录(微信/支付宝 driver)、资料、认证、匿名等级
│  ├─ bottle/      # 扔瓶、捞瓶、我的瓶子、热度、过期
│  ├─ match/       # 捞瓶打分、同城列表、扩列墙(活跃/新人/附近)
│  ├─ chat/        # 会话、消息、WebSocket Hub
│  ├─ relation/    # 喜欢、看过我、关系强度
│  ├─ pay/         # 支付适配层:CreateOrder / HandleCallback(微信+支付宝 driver)
│  ├─ wallet/      # 金币钱包:Debit / Credit / Balance / 流水
│  ├─ item/        # 增值道具/礼物:目录、购买、消费
│  ├─ moderation/  # 敏感词、举报、拉黑、图片审核钩子
│  └─ common/      # 中间件、错误、分页、鉴权、配置开关
├─ pkg/            # db, redis, ws, logger, snowflake-id
└─ migrations/     # SQL 迁移
```

### 4.2 模块边界(包接口,而非进程拆分)
- `pay`:`CreateOrder(ctx, userID, packageID, platform) -> (orderNo, payParams)`、`HandleCallback(ctx, platform, raw) -> error`
- `wallet`:`Credit(ctx, userID, coins, scene, bizNo)`、`Debit(ctx, userID, coins, scene, bizNo) -> error`、`Balance(ctx, userID)`
- 业务侧(开聊/解锁/买道具)只调 `wallet.Debit`,不关心钱的来源。后期要拆微服务,沿包边界即可。

### 4.3 高并发要点(对齐参考文档原则)
- 捞瓶 feed 走 **Redis 预生成列表**,不实时查 MySQL(定时/触发刷新)。
- 浏览/点赞等行为先入 Redis,异步刷库(V1 用 Redis List + 定时 worker,不引 MQ)。
- 聊天 `gorilla/websocket`,在线用户存 Redis,消息只落库一次,最近消息缓存 Redis。
- **钱包扣减用 MySQL 行锁/乐观锁保证强一致**(钱不靠缓存)。

---

## 5. 数据模型

### 5.1 核心业务表
| 表 | 关键字段 | 说明 |
|---|---|---|
| `user` | user_id, wx_openid, alipay_uid, unionid, nickname, avatar, gender, age, city, is_verified, anonymous_level, status, last_active_at | 双平台身份 |
| `bottle` | bottle_id, user_id, content, content_type(text/audio/image), tags, is_anonymous, scope(local/national), status, heat_score, reply_count, created_at, expire_at | 漂流瓶 |
| `bottle_tag` / `tag` | bottle_id↔tag_id;tag_name, tag_type | 标签 |
| `match_log` | viewer_id, bottle_id, action(view/like/reply/skip), score, match_type, created_at | 捞瓶行为+打分 |
| `chat` / `message` | chat_id, user_a, user_b, source_bottle_id, relation_stage;message: type, read_status, created_at | 聊天 |
| `relation` | user_a, user_b, strength_score, stage, interaction_count, last_interaction_at | 关系 |
| `report` / `block` | 举报、拉黑 | 风控 |

### 5.2 支付与变现表
| 表 | 关键字段 | 说明 |
|---|---|---|
| `coin_package` | package_id, name, coins, bonus_coins, price_fen, status | 充值档位 |
| `pay_order` | order_no(唯一), user_id, package_id, platform(wx/alipay), price_fen, coins, status(pending/paid/failed/refunded), platform_txn_id, created_at, paid_at | 充值订单;回调幂等靠 order_no+status |
| `wallet` | user_id, balance, total_recharged, total_spent, updated_at | 一人一钱包,扣减走行锁 |
| `wallet_txn` | txn_id, user_id, direction(credit/debit), coins, scene(recharge/chat/unlock/gift/reward), biz_no, balance_after, created_at | 全流水,可对账 |
| `item` / `item_order` | 道具目录(超级曝光/置顶/礼物)、价格(金币);购买记录 | 道具/礼物 |
| `config` | key, value | 后台开关:verify_required、各场景扣币价、ios_recharge_off 等 |

### 5.3 定调说明
- **价格不写死在代码**,放 `coin_package` / `config`,运营可热调。
- **金币余额 = `wallet.balance`,绝不以 Redis 为真相源**;Redis 只缓存只读展示。
- `pay_order.order_no` 用雪花 ID,回调先查它做幂等再 `wallet.Credit`。

### 5.4 核心索引
- `pay_order(order_no)` 唯一、`pay_order(user_id, status)`
- `wallet_txn(user_id, created_at)`
- `bottle(status, expire_at)`、`bottle(heat_score)`
- `match_log(viewer_id, created_at)`
- `user(wx_openid)` 唯一、`user(alipay_uid)` 唯一

---

## 6. 支付与金币流程

### 6.1 登录(双平台 → user_id)
```
uni.login → code
  #ifdef MP-WEIXIN → 后端 user.WxLogin → 微信换 openid/session
  #ifdef MP-ALIPAY → 后端 user.AlipayLogin → 支付宝换 user_id
后端按 openid/alipay_uid 找或建 user → 签发 JWT(user_id, platform)
```
> V1:两端各算独立账号,钱包随 user_id。后期可用 unionid / 手机号合并。

### 6.2 充值(真钱 → 金币,钱的安全核心)
```
1. 选档位(¥6=60金币)
2. POST /pay/order {package_id}   (价格后端从 config 读,前端价格不可信)
3. pay.CreateOrder: 建 pay_order(status=pending) → 调平台下单 → 返回支付参数
4. 前端 uni.requestPayment → 用户付款
5. 平台异步回调 → pay.HandleCallback:
     验签 → 查 order_no(已 paid 直接返回 success=幂等) → 校验金额
     → 单事务{ order.status=paid + wallet.Credit + wallet_txn(credit,recharge) }
6. 前端支付成功仅作 UI 提示;余额以回调入账为准
```
**红线:加币只在平台异步回调里发生,且幂等;绝不在前端"支付成功"时加币。**

### 6.3 消费(站内扣金币,以开聊为例)
```
POST /chat/start {target_user_id}
单事务{
   SELECT balance ... FOR UPDATE        // 行锁
   balance < price(config) → 回滚,返回"余额不足→引导充值"
   wallet.Debit(scene=chat, biz_no=chat_id)
   chat.Create()
}
→ 返回 chat_id → 进聊天页(WebSocket)
```
解锁回信 / 买道具 / 送礼同理,统一走 `wallet.Debit(scene=...)`。

### 6.4 iOS 合规
```
充值页:platform=wx && system=ios && config.ios_recharge_off → 隐藏入口/提示
```

### 6.5 注册奖励 / 运营送币
```
注册 → wallet.Credit(reward_coins, scene=reward, biz_no=register)
→ 对应"系统消息:注册奖励"
```

---

## 7. 部署架构(2C4G)

```
        [微信小程序]   [支付宝小程序]
              \           /
           [Nginx + TLS 反向代理]
            /  HTTP/api    \  WS/ws
         [Go 单体服务 (systemd/docker)]
            /     |      \
        MySQL   Redis   对象存储(COS/OSS)
```
- 单二进制部署;Nginx 转发 HTTP + WebSocket。
- **支付回调 URL** 配 `/pay/callback/{platform}`,必须公网 HTTPS。
- 密钥/证书走环境变量(不进 git);价格/开关走 `config` 表(热调)。
- V1 不引 MQ;异步用 Redis List + 定时 worker。
- 每日导出 `pay_order` + `wallet_txn` 对账,防丢单/重复入账。

---

## 8. 安全与风控(V1 最小集)
- 接口限流(防刷瓶)、JWT 鉴权、支付回调**验签 + 幂等**。
- 敏感词过滤(发瓶/聊天)、图片审核钩子、举报、拉黑。
- 防引流:屏蔽微信号/手机号等外部联系方式。
- 真人认证:`config.verify_required` 开关;开启时未认证不能发瓶/开聊。

---

## 9. V1 接口清单(草案)

**用户 / 认证**
- `POST /auth/login`        平台登录,返回 JWT
- `GET  /user/profile`      我的资料
- `POST /user/update`       更新资料
- `POST /user/verify`       提交真人/实名认证

**漂流瓶**
- `POST /bottle/create`     扔瓶
- `GET  /bottle/scoop`      捞一个(feed,走 Redis)
- `GET  /bottle/mine`       我的瓶子
- `GET  /bottle/:id`        收件详情
- `POST /bottle/:id/reply`  回信(可能扣币解锁)
- `POST /bottle/:id/like`   喜欢
- `POST /bottle/:id/skip`   跳过

**同城 / 扩列**
- `GET  /city/users`        同城列表(筛选:城市/性别)
- `GET  /expand/wall`       扩列墙(type=active/new/nearby)

**聊天**
- `POST /chat/start`        开聊(扣币)→ chat_id
- `GET  /chat/list`         会话列表
- `GET  /chat/:id/messages` 历史消息(分页)
- `WS   /ws`                实时消息通道

**消息 / 关系**
- `GET  /message/notices`   系统消息
- `GET  /relation/likes`    我喜欢 / 喜欢我 / 看过我

**支付 / 钱包 / 道具**
- `GET  /pay/packages`      充值档位
- `POST /pay/order`         下单 → 支付参数
- `POST /pay/callback/wx`   微信回调(平台调用)
- `POST /pay/callback/alipay` 支付宝回调(平台调用)
- `GET  /wallet/balance`    余额
- `GET  /wallet/txns`       流水
- `GET  /item/list`         道具/礼物目录
- `POST /item/buy`          购买/赠送(扣币)

**风控**
- `POST /report`            举报
- `POST /block`             拉黑

---

## 10. V1 里程碑拆分(建议顺序)

1. **地基**:Go 工程骨架 + MySQL 迁移 + Redis + JWT 中间件 + uni-app 骨架 + 双平台登录跑通。
2. **漂流瓶闭环**:扔瓶 / 捞瓶(Redis feed)/ 我的瓶子 / 收件详情。
3. **聊天**:WebSocket Hub + 会话 + 消息 + 快捷回信。
4. **钱包 + 支付**:wallet/wallet_txn + pay 适配层 + 微信/支付宝下单 & 回调(幂等)+ 充值页。
5. **消费接入**:开聊扣币 / 解锁回信 / 道具礼物。
6. **同城 + 扩列墙**:列表 + 筛选 + 喜欢。
7. **风控 + 认证开关 + iOS 充值开关 + 对账**。
8. **联调 / 双端发版 / 平台审核**。

---

## 11. 待确认 / 风险
- 支付资质:需企业主体 + 微信支付/支付宝商户号 + 对应小程序类目(社交/交友)。**这是上线硬门槛**,需尽早申请。
- 图片/语音审核:V1 留"钩子",接入腾讯云/阿里云内容安全或人工审核,需预算。
- 双平台账号合并策略(unionid/手机号)留待 V2。
```
