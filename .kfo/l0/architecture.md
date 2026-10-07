# 漂流瓶社交小程序 — 系统架构

| 字段 | 值 |
|---|---|
| KFO 层级 | L0 — 系统架构层(必读背景) |
| 最后更新 | 2026-06-22 |
| 关联原始文档 | `docs/superpowers/specs/2026-06-20-driftbottle-design.md` |

---

## 一、产品定位

变现型陌生人轻社交小程序:扔瓶/捞瓶建立连接,**金币充值 + 站内消费**(开聊、解锁回信、道具礼物)变现。一套 uni-app 代码同时发布微信 + 支付宝。

## 二、整体拓扑

```
[微信小程序]   [支付宝小程序]   (uni-app 一套代码,条件编译双端)
       \           /
     [Nginx + TLS]  https://ambertu.com/message
          |  |
          |  └─── /message-admin/   管理后台静态文件(Vue3 产物)
          |
   [Go 单体服务 (Gin) :8980]     模块化单体,非微服务
     /api/*   /admin/api/*   /ws   /health
       /    |    \
   MySQL  Redis  图片存储(联调:本地 /static;生产:COS/OSS)
```

部署目标 2C4G:单二进制 + Nginx + MySQL + Redis,V1 不引 MQ(异步用 Redis List + 定时 worker)。图片联调期走后端本地 `/static`,生产切对象存储。

**生产地址:** `https://ambertu.com/message`(API: `/message/api`，WS: `/message/ws`，管理台: `/message-admin/`)

## 三、后端模块(包边界即模块边界)

```
server/internal/
  user/        登录(微信/支付宝 driver)、资料、真人认证(开关)
  bottle/      扔瓶/捞瓶(Redis feed)、回信/解锁、热度
  match/       同城列表、扩列墙(活跃/新人/附近);双向拉黑过滤
  chat/        会话、消息、WebSocket Hub;ListChatsWithPartner 携带对方信息
  relation/    喜欢、看过我、关系强度
  pay/         支付适配层:CreateOrder / HandleCallback(微信 APIv3 已实现,支付宝骨架)
  wallet/      金币钱包:Credit / Debit / Balance(行锁事务 + 流水)
  item/        道具/礼物
  collection/  收藏(bottle,可扩展 target_type)
  moderation/  敏感词、防引流、举报、拉黑
  upload/      本地图片上传 + /static 静态服务(联调临时,后续切 OSS)
  push/        微信订阅消息推送(subscribeMessage.send);access_token 缓存
  admin/       运营后台:登录/统计/配置/机器人内容池/租户凭证(scope=admin JWT)
  common/      中间件 / JWT / 错误码 / 响应 / 限流
  sysconfig/   价格/开关热配置(config 表);含 tab 显示、公告、推送模板配置
  tenant/      多租户凭证加密存取(COMPLETED)
  robot/       NPC 机器人(内容池调度)
  notify/      站内通知 + WS 实时推送
```

**前端 `client/` 关键层:**
- `components/tab-bar/` — 自定义 TabBar(5 个 Tab 可按后台配置显示/隐藏)
- `utils/config.js` — `BASE_URL` / `WS_URL` 按平台条件编译
- `utils/platform.js` — 跨平台差异隔离(登录/支付/系统信息)

## 四、数据流骨架

- **捞瓶**:Redis 预生成 `user_feed:{uid}` 列表(标签+同城+热度+随机打分),不实时查库,10 分钟过期。
- **充值(真钱→金币)**:`pay.CreateOrder` 下单 → 前端拉起支付 → 平台异步回调 `pay.HandleCallback`(验签→幂等→入账)→ `wallet.Credit`。
- **消费(站内扣金币)**:`wallet.Debit` 在单事务内 `SELECT ... FOR UPDATE` 扣余额 + 写流水 + 执行业务,余额不足拒绝。
- **聊天**:`gorilla/websocket` Hub(单实例内存版,在线连接 map),消息只落库一次 + WS 推送对方。

## 五、关键 NFR / 约束

| 项 | 约束 |
|---|---|
| 钱的一致性 | 余额真相源 = MySQL `wallet.balance`,绝不以 Redis 为准;扣减走行锁 |
| 支付安全 | 加币只在平台异步回调内发生,且对 `order_no` 幂等 |
| iOS 合规 | 微信 iOS 端虚拟币充值受限 → `config.ios_recharge_off` 开关隐藏入口,不发版 |
| 跨端 | 平台差异(登录/支付/系统信息)只锁在前端 `utils/platform.js`,业务零感知 |
| **JS 客户端 int64** | 所有 ID 字符串序列化(见 `l3/2026-06-20-int64-id-js-precision.md` / `l4/cross-platform-api-patterns.md`) |

## 五.5、SaaS 多租户(COMPLETED)

**已实现**:appid 当租户标识、凭证集中加密入表(`tenant_credentials`)、全业务表加 `tenant_id` 隔离、管理台可查看/修改租户凭证并热刷新内存缓存。详见 `l1/tenant-saas.md`、`l2/2026-06-20-saas-multitenant-impl.md`。

## 六、技术栈

Go 1.22+(Gin / GORM / gorilla-websocket / go-redis)· MySQL 8.0 · Redis · uni-app(Vue3 + Pinia)· 微信支付 APIv3(标准库 RSA/AES-GCM 自实现)。

## 七、关联

- L1 设计:`l1/pay-wallet.md`
- L2 里程碑:`l2/2026-06-20-driftbottle-v1-and-api-tests.md` · `l2/2026-06-22-ui-ux-push-tabbar.md`
- L3 复盘:`l3/2026-06-20-int64-id-js-precision.md`
- L4 沉淀:`l4/cross-platform-api-patterns.md`
