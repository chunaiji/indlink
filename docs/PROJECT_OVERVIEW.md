# 项目全景 — 模块 / 特点 / 变现

| 字段 | 值 |
|---|---|
| 最后更新 | 2026-09-14 |
| 口径 | **基于代码实际状态**梳理（非设计文档口径），与 `.kfo/l0/architecture.md` 的差异见「八、现状与缺口」 |
| 适用 | 新人快速建立全局认知 / 对外介绍项目 / 盘点变现能力 |

---

## 一、产品形态

**双形态小程序，由 sysconfig 远程开关控制，切形态不发版：**

| 形态 | 定位 | 启动页 | 说明 |
|---|---|---|---|
| 工具形态（审核态/默认） | 「小纸条水印相机」 | `pages/privacy` | 拍照 + 地址水印 + 品牌平铺水印 |
| 社交形态 | 漂流瓶匿名社交 | `pages/ocean` | 扔瓶/捞瓶/私聊/动态广场/送礼/签到 |

**硬约束：一切新增用户可见功能必须挂 sysconfig 开关且默认关闭**，工具形态下绝不露出。

形态切换依赖四条独立下发链路：

| 接口 | 控制内容 |
|---|---|
| `GET /api/tabs` | 6 个 Tab 的显隐（首页/同城/扩列/消息/我的/权限） |
| `GET /api/mine-functions` | 「我的」页 13 个功能项显隐 + 客服文案/图片 + 完善资料跑马灯 |
| `GET /api/pages-config` | 全站页面图片覆盖（`pages_cover_on` + 覆盖图） |
| `GET /api/features` | 留存功能开关（漂流轨迹/深夜瓶/资料卡/魅力周榜/动态广场）+ 关联小程序 + 水印样式 |

---

## 二、工程结构与技术栈

```
server/   Go 1.22+ 单体（Gin + GORM v2 + MySQL 8 + Redis），模块化单体，包边界即模块边界
          入口 cmd/api/main.go，业务在 internal/<域>/，共 29 个包、34 张表
client/   uni-app Vue3(alpha) + Pinia，微信/支付宝双端条件编译，21 个页面 + 8 个组件
admin/    Vue3 + Vite 管理后台，22 个视图（20 功能页 + Layout + Login），线上 /message-admin/
docs/     DEV_RUNBOOK + 11 份设计 spec + 7 份实施 plan
.kfo/     知识库：L0 架构 / L1 域文档 ×11 / L2 迭代复盘 ×17 / L3 问题档案 ×5 / L4 沉淀
```

拓扑：

```
[微信小程序]  [支付宝小程序]        （一套 uni-app 代码）
        \        /
      [Nginx + TLS]  https://ambertu.com
       |            └── /message-admin/  管理后台静态文件
       └── /message/  →  [Go 单体 :8980]   /api/*  /admin/api/*  /ws  /health
                             /      |      \
                          MySQL   Redis   本地 /static 图片
```

生产：175.178.182.166，nginx 配置在 `/etc/nginx/conf.d/pet.conf`，服务 `systemctl status driftbottle`。

---

## 三、后端模块清单（`server/internal/`）

### 3.1 基础设施层

| 模块 | 职责 |
|---|---|
| `config` | .env 环境配置加载 |
| `bootstrap` | GORM AutoMigrate 建表 + 默认数据 Seed（启动自动加表/加列） |
| `crypto` | AES-256-GCM 凭证加解密，主密钥来自 env `CRED_MASTER_KEY` |
| `tenant` | 租户凭证 credstore：按 appid 缓存解密后的凭证，支持热刷新 |
| `sysconfig` | 两百多个热配置键，三级回退：租户值 → 全局 tenant0 → 代码默认；启动时 seedDefaults 回写 DB。C 端按端各有只读下发接口（`/features` `/tabs` `/ads` `/app-config` …） |
| `model` | 全部 34 张表的结构体定义 + `AllModels()` |
| `common/` | `jwtutil` / `middleware`（鉴权+CORS+租户注入）/ `errs` / `response` / `ratelimit` |
| `upload` | 本地图片上传 + `/static` 静态服务；上传回调挂微信图片异步检测 |
| `geo` | 逆地理编码服务端代理（腾讯位置服务，key 后台配；无 key 降级经纬度） |
| `pkg/apilog` | 所有外呼（内容安全/订阅推送/逆地理/access_token）异步落库，保留 7 天 |

### 3.2 核心业务层

| 模块 | 职责 | 关键实现 |
|---|---|---|
| `user` | 微信/支付宝登录、资料、真人认证、活跃打点 | 登录带 appid 解析租户；资料文本过内容安全 |
| `bottle` | 扔瓶 / 捞瓶 / 回信 / 解锁 / 热度 / 过期 | Redis 预生成 feed（`feed.go`），限流 10 次/分 |
| `match` | 同城列表、扩列墙、双向拉黑过滤、机器人注入 | 打分权重 7 项后台可调 |
| `chat` | 会话、消息、WebSocket Hub | 单实例内存版 Hub；消息落库一次 + WS 推对方 |
| `relation` | 喜欢 / 看过我 / 关系强度（stage + interaction_count） | — |
| `moment` | 动态广场：发布（≤9 图）、评论、点赞、动态内送礼 | 评论/送礼触发通知回调 |
| `collection` | 收藏（当前仅 bottle，`target_type` 可扩展） | — |
| `notify` | 站内通知 + 在线 WS 实时推送 | 回信/捞瓶/评论/收礼四类回调 |
| `moderation` | 敏感词、防引流、举报、拉黑 + 微信内容安全 | `CheckUGC` 7 个发布场景；图片 `mediaCheckAsync` |
| `push` | 微信订阅消息 5 个模板场景 + 定时任务 | 回信/聊天/活动/作品推荐/签到；签到 09:00、深夜场开场前 5 分钟 |
| `rank` | 用户资料卡（含礼物墙）+ 魅力周榜 Top20 | 从 ItemOrder 聚合，Redis 缓存 2 分钟，送礼后主动失效 |
| `checkin` | 每日签到 7 天阶梯发币 + 激励视频补签 | 唯一索引防重 + bizNo 幂等 |
| `admin` | 运营后台 API，**70+ 路由**，独立 scope=admin JWT | 统计/配置/用户/内容/机器人/租户/订单 |

### 3.3 变现层

| 模块 | 职责 |
|---|---|
| `wallet` | 金币钱包：`Credit` / `Debit` / `Balance`，行锁事务 + 全量流水 |
| `pay` | 充值适配层：微信支付 APIv3（完整）/ 支付宝（骨架），driver-per-tenant |
| `item` | 道具/礼物目录 + 购买/赠送 + 次数包充值 |
| `quota` | 每日扔/捞次数限制：Redis 按天免费额度 + 购买的次数包余额 |
| `ad` | 流量主激励视频发币（Redis 原子计数控每日上限 + bizNo 幂等） |
| `share` | 分享领金币（按天上限兜量） |

### 3.4 差异化层

| 模块 | 规模 | 职责 |
|---|---|---|
| `robot` | **20 个文件**（全项目最重） | AI 人格化对话引擎：人格配置 + 四层回复 + 关系记忆 + 主动触达 + 内容池调度 |

---

## 四、前端与后台

### 客户端页面（21 个）

| 分组 | 页面 |
|---|---|
| Tab 页 | `ocean`(首页/海洋) `city`(同城) `expand`(扩列墙/动态广场) `message`(消息) `mine`(我的) |
| 工具形态 | `privacy`(水印相机，非 tabBar 页，reLaunch 跳转) |
| 社交主链路 | `detail`(收件详情) `chat`(聊天) `mybottles`(我的瓶子) `notifications`(互动通知) `viewed`(浏览记录) `collection`(我的收藏) |
| 动态 | `moments`(我的动态) `moment-detail`(动态详情) `rank`(魅力周榜) |
| 变现 | `recharge`(充值金币) `items`(道具商城) `orders`(我的订单) `wallet-log`(金币流水) |
| 其他 | `profile-edit`(完善资料) `blocklist`(黑名单) |

关键组件：`tab-bar`(自定义 TabBar) · `ad-slot`(广告位) · `page-cover`(页面覆盖) · `gift-picker`(送礼) · `user-card`(资料卡) · `user-avatar` · `sk-list`(骨架屏) · `empty-state`。

### 管理后台（20 个功能页）

| 分组 | 页面 |
|---|---|
| 经营 | `Dashboard`(KPI+趋势) `Orders`(充值订单) `WalletTxns`(钱包流水) `Packages`(充值档位) `Items`(道具) |
| 内容审核 | `Users` `Messages` `Bottles` `Moments` |
| 机器人 | `Persona`(人格库) `RobotProfile`(机器人档案) `RobotContent`(内容池) `KeywordRules`(关键词规则) `ReplyCache`(回复缓存) `RobotChats`(机器人会话/人工接管) |
| 平台 | `Config`(两百多项配置，三十余分组) `Tenants`(租户) `Credentials`(租户凭证) `ApiLogs`(接口日志) `Account` |

配置页按**分区 → 分组 → 键**三层组织，8 个分区：App / AI 与机器人 / 界面与文案 / 运营与变现 / 支付 / 广告 / 客服与跳转 / 系统与安全。
分区 code 进路由（`/config/:section`，链接可分享），另有一个跨分区搜索（名称 / key / 分组名任一命中）。

三条与内容无关但很要紧的机制：**按 `Tenant.Type` 过滤**（小程序租户与 App 租户看到的不是同一批项，一项不可见的分区不出 Tab）、
**机密项脱敏**（9 个键只下发 `is_set` 不下发值，清空要走显式「清除」）、**中英双语**（标签在 `meta.go` 内联双语）。
细节见 `.kfo/l1/admin-platform.md` §三、§八。

---

## 五、特点模块（差异化 / 技术壁垒）

### ⭐ 1. 双形态远程开关架构

一套代码两个产品。审核态是完全可用的水印相机工具，过审后逐项开启社交能力。配置改动即时生效（sysconfig 热刷新），无需发版、无需重新提审。这是整个项目最核心的产品设计，也是后台那两百多项配置存在的理由。

### ⭐ 2. `robot` — AI 人格化对话引擎

**四层回复漏斗，越往下越贵，逐层拦截以压 LLM 成本：**

| 层 | 机制 | 成本 | 目的 |
|---|---|---|---|
| Tier 0 | 身份硬拦截 `identity_guard.go` | 0（代码硬编码） | "你是机器人吗" → 7 条否认话术随机返回，不依赖 LLM 遵循性 |
| Tier 1 | 关键词规则 `keymatcher.go` | 0（查 DB） | 打招呼/问年龄/情感安慰等高频场景，后台维护 |
| Tier 2 | LLM 自积累缓存 `replycache.go` | 0（命中即返回） | SHA256(归一化文本\|人格角色)[:8]，同角色共享，≤5 条变体 |
| Tier 3 | LLM 生成 `llmclient.go` | 付费 | OpenAI 兼容接口，生成后异步回写缓存 |

配套机制：

- **人格系统**：关系角色(friend/partner/companion/mentor) × 情感风格(warm_soft/calm/energetic/playful/dominant) × 语音风格，可批量创建
- **关系记忆**：用户×机器人独立记录，`familiarity` 每次对话递增，驱动语气亲密度；会话摘要压缩
- **身份防泄露三层防护**：系统 prompt 以真实人类自居 + 末尾硬性禁止块；history 必须以 user/assistant 独立消息传入（嵌进 system prompt 会导致 LLM 覆盖身份禁令）；AI 自白短语在缓存读写双向过滤，防污染
- **主动触达**：新用户破冰、沉默用户唤回、会话沉默追问，按时段窗口 + 抽样比例 + 每日上限调度
- **素人化伪装**：部分机器人用「用户XXXXXX」昵称 + 空头像 + 未认证，仿真实用户
- **防复读**：变体池 <5 时按 40% 概率穿透 LLM 积累变体；聊天查重扩到近 5 条；同瓶回信带 exclude

### ⭐ 3. SaaS 多租户

**appid 即租户标识**（公开非密，前端运行时自取），凭证 AES-GCM 加密入 `app_credentials` 表，一份部署服务 N 个小程序。

- 全业务表带 `tenant_id`，所有查询强制租户过滤（tenantID=0 仅限后台全量汇总）
- Redis 键带租户前缀（`user_feed:{tid}:{uid}`）
- 支付 driver-per-tenant，回调路径 `/pay/callback/:platform/:tenantId`
- **微信凭证同源**：access_token 优先取租户凭证表 —— 新租户配一次凭证，登录/推送/内容安全全通
- 同一微信号在不同小程序 = 不同账号 + 不同钱包

### ⭐ 4. 捞瓶 feed 打分引擎

Redis 预生成队列（不实时查库，10 分钟过期），7 个权重后台可调：标签重合 40 / 同城 25 / 性别 15 / 新鲜度 10 / 热度 10 / 机器人惩罚 −5 / 随机扰动 15。深夜瓶按昼夜维度分缓存 key，night 标签只在夜场时段可捞。所有捞瓶行为落 `MatchLog`（view/reply/like/skip），支撑漂流轨迹与去重。

### ⭐ 5. 内容安全与可观测（过审刚需的工程化）

- 文本：本地词库 → 微信 `msgSecCheck v2`，覆盖 7 个发布场景（场景值 1资料/2评论/4社交）
- 图片：C 端上传统一入口 + 水印相机选图静默上传，提交 `mediaCheckAsync` 留档
- API 失败 / 支付宝用户放行不阻断；开关默认关
- 所有外呼经 `pkg/apilog` 异步落库，后台「📡 接口日志」页可查 —— 排查"到底调没调、返回什么"的唯一可信来源

### ⭐ 6. 资金一致性设计

- 余额真相源 = MySQL `wallet.balance`，绝不以 Redis 为准
- 扣减走 `SELECT ... FOR UPDATE` 行锁事务：扣余额 + 写流水 + 执行业务在同一事务，余额不足直接拒绝
- 加币只在平台异步回调内发生，且对 `order_no` 幂等
- 所有发币场景（签到/分享/广告/注册）均带 `bizNo`，钱包侧天然幂等；Redis 计数失败自动回滚

---

## 六、变现模块（盈利）

### 6.1 直接付费 —— 金币经济（主变现）

**入口：真钱 → 金币**

| 环节 | 实现 | 状态 |
|---|---|---|
| 充值档位 | `CoinPackage`（币数 + 赠送 + 价格分），后台 `Packages` 页可配 | ✅ |
| 微信支付 | APIv3，标准库自实现 RSA/AES-GCM 验签（`pay/driver_wx.go` + `wxcrypto.go`） | ✅ 完整 |
| 支付宝支付 | `driver_alipay.go` — `Prepay`/`VerifyCallback` 均为 TODO，无凭证时返回 mock | ⚠️ **未接入** |
| 回调入账 | 验签 → 按 `order_no` 幂等 → `wallet.Credit`；成功后自动打「付费用户」标签 | ✅ |

**出口：金币 → 站内消费（9 个流水场景，5 个扣费点）**

| 场景 | 流水 scene | 配置项 | 默认值 | 代码位置 |
|---|---|---|---|---|
| 开聊 | `chat` | `price_chat` | 5 金币 | `chat/service.go:82` |
| 解锁回信 | `unlock` | `price_unlock` | 2 金币 | `bottle/service.go:457` |
| 每条消息扣费 | `msg` | `price_msg` | **0 = 关闭** | `chat/service.go:215` |
| 道具/礼物 | `gift` | `Item.PriceCoin` | 后台配 | `item/service.go:61` `moment/service.go:357` |
| 次数包 | `gift` | `quota_throw_pack` / `quota_scoop_pack` | 10 / 20 次 | `item/service.go:70` |

发币场景：`recharge`(充值) `reward`(注册送 50) `checkin`(签到) `share`(分享) `ad_reward`(激励视频)。

**送礼闭环（最有变现潜力的链路）：**

```
扣币事务 → ItemOrder(带 target) → 收礼方 User.Charm 累加 → 通知
                                        ↓
                            礼物墙 / 魅力周榜（从 ItemOrder 聚合）
                                        ↓
                              排名刺激 → 更多送礼
```
送礼三处入口（聊天页 / 动态列表 / 动态详情）走统一链路，送礼后调 `rank.InvalidateWeekCache` 让榜单即时反映。

### 6.2 流量主广告（第二变现）

**16 个广告位**，按类型配一个 unit id 共用，各位独立开关：

| 类型 | 数量 | 位置 |
|---|---|---|
| Banner | 12 | 首页/同城/扩列/消息/我的/详情/聊天/收藏/订单/流水/浏览记录/权限页 |
| 插屏 | 2 | 捞瓶 / 详情（最小间隔 180 秒） |
| 激励视频 | 1 | 看完发金币（默认 5 币，每日上限 5 次）；同时用于签到补签 |
| 原生模板 | 1 | 「我的」页宫格 |

### 6.3 增长 / 留存（撑 ARPU 和 DAU，不直接收钱）

| 手段 | 配置 | 默认 |
|---|---|---|
| 注册奖励 | `reg_reward_coins` | 50 币 |
| 每日签到 | `checkin_ladder` 7 天阶梯 | `5,5,10,10,15,15,30`，断签重头；每月 2 次激励视频补签 |
| 分享奖励 | `share_reward_coins` / `_daily_limit` | 5 币 × 3 次/天（默认关） |
| 机器人主动触达 | `outreach_*` | 19–22 点窗口，活跃用户抽 10%，每人每日 1 次（默认关） |
| 每日免费次数 | `quota_throw_daily` / `quota_scoop_daily` | 扔 10 / 捞 20，用完引导买次数包 |
| 余额不足挽回 | `low_balance_threshold` | 余额 <10 币自动推系统消息 |
| 首页热度钩子 | `hook_base` + 每分钟净增 | 「今日已有 N 条回应」，base 56000（伪造计数） |
| 关联小程序导流 | `link_mp_*` | 首页最多 4 个入口，可配图标/路径 |

### 6.4 SaaS 复制

多租户能力本身即商业模式：同一套后端 + 同一个后台，新增租户只需建 Tenant + 填一次凭证，即可再开一个小程序主体。后台租户列表可查看 appid、内联编辑、按租户查看全部统计。

### 6.5 经营看板（`admin/overview.go`）

| 指标 | 口径 |
|---|---|
| 真人新增用户 | `is_robot=0` |
| 真人活跃用户 | `last_active_at` 在期内 |
| 充值额（元） | 已支付订单 `price_fen/100` |
| 已支付订单数 / 付费用户数（去重） | `status='paid'` |
| **ARPU** | 充值额 / 付费人数 |
| 消费金币 | 流水 `direction='debit'` 求和 |

每项均分「今日 / 本周 / 本月 / 本年」四段 + 近 30 天日趋势，**时间口径统一按东八区**（历史上用 `time.Now().Truncate(24h)` 按 UTC 截断偏 8 小时，已修正）。

---

## 七、数据表（34 张）

| 域 | 表 |
|---|---|
| 租户 | `tenants` `app_credentials` |
| 用户 | `users` `relations` `blocks` |
| 漂流瓶 | `bottles` `bottle_replies` `reply_unlocks` `match_logs` |
| 聊天 | `chats` `messages` |
| 资金 | `wallets` `wallet_txns` `coin_packages` `pay_orders` `items` `item_orders` |
| 动态 | `moments` `moment_likes` `moment_comments` |
| 机器人 | `persona_configs` `robot_memories` `robot_keyword_rules` `robot_reply_caches` `robot_contents` |
| 运营 | `configs` `admin_users` `notifications` `collections` `checkin_logs` `push_subscriptions` |
| 风控 | `reports` `wx_media_checks` `api_call_logs` |

表结构靠 GORM AutoMigrate（启动自动加表/加列），无手写迁移脚本。

---

## 八、现状与缺口

### 8.1 当前默认配置下变现几乎全关

线上默认跑的是**审核 / 工具形态**，变现要靠后台逐项打开：

| 开关 | 默认 | 影响 |
|---|---|---|
| `fn_show_recharge` | 0 | 充值入口隐藏 |
| `fn_show_items` | 0 | 道具商城隐藏 |
| `fn_show_orders` / `fn_show_walletlog` | 0 | 订单、流水隐藏 |
| `ios_recharge_off` | 1 | iOS 端隐藏充值（微信 iOS 虚拟币合规） |
| `ad_enabled` + 全部 16 个广告位 | 0 | 广告全关 |
| `checkin_enabled` / `share_reward_enabled` | 0 | 签到、分享奖励全关 |
| `price_msg` | 0 | 消息扣费关闭 |
| `robot_enabled` / `ai_bot_enabled` / `ai_chat_enabled` | 0 | 机器人全关 |
| `square_enabled` / `charm_rank_enabled` / `user_card_enabled` / `night_bottle_enabled` / `bottle_trace_enabled` | 0 | 留存五功能全关 |

### 8.2 已知缺口

| 缺口 | 说明 | 影响 |
|---|---|---|
| 支付宝支付未接入 | `driver_alipay.go` 下单与验签均为 TODO | 双端里只有微信能收钱 |
| 内容安全结果回调未启用 | 依赖微信「消息推送」配置，会影响客服消息 | 违规图靠人工巡查 |
| chat Hub 单实例内存版 | 在线连接存在进程内存 map | 无法水平扩容，多实例会丢在线态 |
| 图片存本地 `/static` | 未切对象存储（COS/OSS） | 单机磁盘绑定，迁移/扩容受限 |
| 机器人投放时间未拟人化 | 当前均匀速率 | 深夜投放量与真人作息不符 |

### 8.3 与 `.kfo/l0/architecture.md` 的差异

L0 架构文档最后更新 2026-06-22，其模块清单**缺少后续新增的 9 个包**：`checkin` `rank` `share` `ad` `geo` `moment` `quota` `config` `bootstrap`。以本文档为准。

---

## 九、关联文档

| 主题 | 位置 |
|---|---|
| 本地起服务 / 联调 | `docs/DEV_RUNBOOK.md` |
| 微信支付配置 | `docs/wechat-pay-setup.md` |
| 系统架构（含数据流骨架） | `.kfo/l0/architecture.md` |
| 域级设计（11 篇） | `.kfo/l1/` — robot / pay-wallet / bottle-feed / chat-ws / match / tenant-saas / user-auth / notify / admin-platform / mp-permissions / ui-design-system |
| 迭代复盘（17 篇） | `.kfo/l2/` |
| 踩坑档案（5 篇） | `.kfo/l3/` — int64 精度 / @tap 绑定 / LLM 历史位置 / 缓存污染 / GORM Order |
| 开发硬约束 | `CLAUDE.md`（sysconfig defaults、ID 字符串化、东八区口径、uni-app 编译三坑等） |
