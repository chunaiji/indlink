# 漂流瓶 Flutter App（海外版）技术设计

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-14 |
| 状态 | DESIGN — **部分被 V1 范围清单推翻，待决策** |
| 范围 | MVP 五屏 Flutter App + 后端增量改造 |
| 关联 | `2026-09-15-app-v1-scope.md`（V1 模块范围）· `docs/PROJECT_OVERVIEW.md`（现状全景）· `.kfo/l1/tenant-saas.md`（多租户）· `.kfo/l1/user-auth.md`（登录）· `.kfo/l1/chat-ws.md`（WebSocket） |

> ⚠️ **2026-09-15 更新**：产品侧给出的 App V1 模块清单（17 模块）与本文 §二 的决策 3/4/5、§三 范围、§七 第 7 项、§九 路线图**存在直接冲突**（登录方式、变现、目标市场收窄至印度）。冲突登记见 `2026-09-15-app-v1-scope.md` §三。
>
> 本文的以下部分**不受影响，仍然有效**：§四 Flutter 分层与依赖选型、§5.1 数据模型约束、§5.2 租户识别、§5.4 账号删除语义、§5.5 国际化方案。

---

## 一、目标与背景

把现有漂流瓶小程序转为 Flutter 原生 App，上架 **Google Play + App Store**，**以海外市场为主**。

采用**增量迭代**：先用 MVP 五屏打通完整体验闭环并拿到上架资格，后续按路线图逐步补齐功能。

现有小程序（工具形态/社交形态双形态）**并行维护**，与 App **共用同一套 Go 后端**。所有后端改动必须是**加法**，不得影响小程序现有行为。

## 二、已确认决策

| # | 决策点 | 结论 |
|---|---|---|
| 1 | 客户端技术栈 | Flutter |
| 2 | 目标市场 | 海外为主 |
| 3 | MVP 范围 | 核心链路 5 屏 |
| 4 | 登录方式 | 邮箱 + 密码，**不发验证邮件**；后续迭代加 Google（届时 iOS 必须补 Apple 登录） |
| 5 | 充值变现 | **第一版不做**。无充值、无广告、无 IAP |
| 6 | 小程序 | 并行维护，共用后端 |
| 7 | 后端部署 | **MVP 阶段沿用国内腾讯云节点**，海外延迟列为已知风险 |
| 8 | 文案国际化 | **后端按 `Accept-Language` 下发** |

## 三、范围

### 3.1 MVP 五屏（做）

| 屏 | 对应小程序页 | 核心能力 |
|---|---|---|
| 登录 / 注册 | 新增（小程序无此形态） | 邮箱注册、邮箱登录、登录态持久化 |
| 首页 | `pages/ocean` | 扔瓶、捞瓶（Redis feed）、每日次数显示 |
| 收件详情 | `pages/detail` | 查看瓶子、回信、解锁回信、发起聊天 |
| 聊天 | `pages/chat` | 会话消息列表、发消息、WebSocket 实时收消息 |
| 我的 | `pages/mine` | 资料展示、编辑资料、金币余额、**账号删除**、退出登录 |

### 3.2 明确不做（MVP）

- 充值、道具商城、礼物、订单、金币流水页
- 流量主广告（16 个广告位全部不迁移）
- 同城、扩列墙、动态广场、动态详情、魅力周榜
- 收藏、浏览记录、水印相机
- **黑名单管理页**（注意：拉黑「动作」在 MVP 内且为上架硬性要求，见 §5.6；此处不做的是管理已拉黑列表的独立页面）
- 微信订阅消息推送（App 端不适用，见路线图迭代 2 的 FCM）
- 微信/支付宝登录、微信支付
- 邮箱验证邮件、找回密码

### 3.3 金币在 MVP 的处理

不做充值，但**保留金币消费逻辑**——注册赠币（`reg_reward_coins`，默认 50）足以覆盖 MVP 体验：开聊 5 币、解锁回信 2 币。余额耗尽即无法继续开聊，这是**可接受的 MVP 行为**，不做额外处理。

> 注意：签到（`checkin_enabled`）默认关闭。若希望 MVP 用户能持续获得金币，应在后台打开签到开关，但**签到 UI 不在 MVP 五屏内**，需评估是否放入迭代 1。

---

## 四、架构

### 4.1 整体拓扑

```
[Flutter App]  Android / iOS
      │  HTTPS + WSS，JWT 鉴权，Accept-Language: en
      ▼
[Nginx + TLS]  https://ambertu.com
      └── /message/  →  [Go 单体 :8980]
                          ├── /api/*        ← 小程序与 App 共用
                          ├── /api/auth/email/*   ← App 新增
                          ├── /ws           ← 共用
                          └── /admin/api/*  ← 管理后台
[微信小程序] ──┘（并行，走 /api/auth/login 原路径，零改动）
```

### 4.2 Flutter 分层

```
app/
  lib/
    main.dart                     入口
    app.dart                      MaterialApp.router + 主题 + localizationsDelegates
    core/
      config.dart                 API_BASE / WS_BASE（--dart-define 可覆盖）
      http/dio_client.dart        拦截器：JWT 注入 / Accept-Language / 解包 / 401 跳登录
      http/api_exception.dart     后端 code → 异常映射
      storage/token_store.dart    flutter_secure_storage 存 JWT
      router/app_router.dart      go_router + 登录态 redirect 守卫
    l10n/
      app_en.arb                  默认语言
      app_zh.arb
    features/
      auth/       data/ · domain/ · application/ · presentation/
      bottle/     data/ · domain/ · application/ · presentation/
      chat/       data/（含 chat_socket.dart）· domain/ · application/ · presentation/
      profile/    data/ · domain/ · application/ · presentation/
```

每个 feature 内部四层：`data`（API 调用）/ `domain`（模型）/ `application`（Riverpod controller）/ `presentation`（页面与组件）。单个文件超过 ~300 行即拆分。

### 4.3 依赖选型

| 用途 | 包 | 理由 |
|---|---|---|
| 路由 | `go_router` | 官方推荐，声明式，`redirect` 天然适合登录态守卫 |
| 状态管理 | `flutter_riverpod` | 编译期安全、不依赖 BuildContext、易测试 |
| 网络 | `dio` | 拦截器统一处理 JWT / 语言头 / 错误码 |
| WebSocket | `web_socket_channel` | 直连现有 `/ws?token=`，后端零改动 |
| 安全存储 | `flutter_secure_storage` | JWT 存 Keychain / Keystore |
| 普通存储 | `shared_preferences` | 非敏感的本地偏好 |
| 图片缓存 | `cached_network_image` | 头像与瓶子图片 |
| 国际化 | `flutter_localizations` + `intl` | 官方方案，ARB |

---

## 五、后端改动

全部为加法，小程序路径不受影响。

### 5.1 数据模型变更（GORM AutoMigrate 自动加列）

`model.User` 新增：

```go
Email     string     `gorm:"size:128;index" json:"email"`
PwdHash   string     `gorm:"size:100" json:"-"`          // bcrypt，绝不下发
DeletedAt *time.Time `json:"-"`                          // 账号删除时间；仅记录
```

> **关键约束**：`DeletedAt` 使用 `*time.Time` 而**非** `gorm.DeletedAt`。后者会开启 GORM 全局软删除，静默改变现有所有查询的行为，属于高危变更。账号是否有效**一律以 `Status` 判断**。

`User.Status` 取值扩展：`active` / `frozen` / `banned` / **`deleted`**（新增）。

`model.Notification` 新增（用于 i18n，见 §5.5）：

```go
TmplKey    string `gorm:"size:48" json:"-"`   // 文案模板键；空=用已存的 Title/Body
ParamsJSON string `gorm:"size:512" json:"-"`  // 渲染参数
```

### 5.2 租户识别 — 复用现有多租户机制

App 没有小程序那样的 appid。**不引入新概念**，而是复用 `app_credentials` 表：

- 新增一行：`platform = "app"`，`appid = "<打包时写死的 app key>"`，绑定目标 `tenant_id`
- `user.resolveLogin` 的 platform switch 增加 `"app"` 分支，直接返回 `r.TenantID`（App 无需 code2session，不取 secret）
- App 登录请求带 `appid`，与小程序完全同构

好处：租户隔离、凭证管理、后台「租户凭证」页全部自动复用，零新增机制。

### 5.3 新增 API

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/email/register` | 否 | `{appid, email, password, nickname?}` |
| POST | `/api/auth/email/login` | 否 | `{appid, email, password}` |
| DELETE | `/api/user/account` | 是 | 账号删除（Apple 强制要求） |

注册/登录响应结构与现有 `/api/auth/login` **完全对齐**：`{token, user, is_new}` + `clientConfig(tenantID)`，复用 `jwtutil.Generate`，下游所有鉴权中间件零改动。

错误码复用现有定义（`internal/common/errs`）：

| 场景 | 码 |
|---|---|
| 邮箱已注册 / 参数非法 | `1001` CodeBadRequest |
| 邮箱或密码错误 | `2002` CodeLoginFailed |
| 账号被封禁或已删除 | `1004` CodeForbidden |

唯一性按 `(tenant_id, email)` 在**应用层查重**，不加 DB 复合唯一索引——与现有 openid 的处理方式一致，规避空值唯一冲突（见 `.kfo/l1/tenant-saas.md` 记录的坑）。

### 5.4 账号删除语义

Apple 要求 App 内提供真实删除而非停用。实现为**匿名化 + 释放标识**，在单事务内完成：

1. `Status = "deleted"`，`DeletedAt = now`
2. 清空 PII：`Email` / `Nickname` / `Avatar` / `Bio` / `WxOpenID` / `AlipayUID` / `UnionID` 置空
3. 邮箱标识释放，可被重新注册
4. 该用户的瓶子 `Status = "deleted"`，动态删除
5. **保留**：钱包流水、支付订单（财务对账与审计需要，且已与自然人解绑）

登录与鉴权中间件需拒绝 `Status = "deleted"` 的用户。

### 5.5 国际化（后端按 Accept-Language 下发）

三个需要国际化的表面，分别用最小改动方案：

**① 错误与提示消息**

- 新增中间件解析 `Accept-Language`，归一化为 `en` / `zh`，写入 gin Context
- 新增 `internal/i18n` 包：`T(lang, key string, args ...any) string`，catalog 为 Go map，**不落库**
- `response.Fail` 增加可传 key 的变体；现有调用点保持不变，按 MVP 涉及的接口逐个迁移

**② sysconfig 文案（`ui_text_*` 等 20+ 项）**

利用现有 key 为字符串的特性，**加语言后缀**，不改表结构：

```
ui_text_nav_title       → 中文（现有，不动）
ui_text_nav_title.en    → 英文（新增）
```

`sysconfig` 新增 `GetStringLang(tenantID, lang, key)`：先查 `key + "." + lang`，未命中回退 `key`，再接现有的「租户 → 全局 → 代码默认」三级回退。

> 硬约束提醒：新增的 `.en` 键同样要考虑是否写入 `defaults` 与 `admin/meta.go` 白名单。建议 `.en` 变体**只登记 MVP 实际使用的那几个**，避免后台配置页膨胀一倍。

**③ 通知文案**

现状：`notify.Create(...)` 在写入时就把中文 title/body 落库，语言在写时定死。

改为**写键、读时渲染**：

- 写入时填 `TmplKey`（如 `notif.moment_comment`）+ `ParamsJSON`
- 读取时按请求方 `Accept-Language` 渲染
- `TmplKey` 为空时回退到已存的 `Title`/`Body` —— **小程序的历史数据与现有写入路径完全不受影响**

### 5.6 内容安全（App 端）

微信 `msgSecCheck` / `mediaCheckAsync` 是小程序专用接口，**在 App 端完全失效**。

MVP 最低限度（满足 App Store 审核指南 1.2 的形式要求）：

- 保留现有本地敏感词库与防引流（`moderation` 模块已有）
- 保留举报与拉黑（已有，需在 MVP UI 中提供入口）
- 「我的」页提供开发者联系方式

接入 OpenAI Moderation（可复用已配置的 LLM 凭证）放入**迭代 2**。

> App Store 对匿名陌生人社交按 17+ 分级审核，UGC 机制缺失会直接被拒。举报与拉黑入口**必须**出现在 MVP 的聊天与详情页，不可后置。

---

## 六、关键流程

### 6.1 注册 / 登录

```
App 启动
  └─ token_store 读 JWT
       ├─ 有 → GET /api/user/profile 校验
       │        ├─ 200 → 进首页
       │        └─ 401 → 清 token，进登录页
       └─ 无 → 进登录页

注册：POST /api/auth/email/register {appid, email, password}
  → 后端 resolveLogin("app", appid) 定租户
  → (tenant_id, email) 查重
  → bcrypt 哈希 → 建 User（复用 idgen + 昵称生成）
  → 发注册奖励 reg_reward_coins
  → 签发 JWT，返回 {token, user, is_new: true}
  → App 存 token，直接进首页（注册即登录）
```

### 6.2 捞瓶

复用现有 `bottle` 模块与 Redis feed，**后端零改动**。App 侧仅需正确传递 JWT 与处理 `3003 CodeQuotaExceeded`（次数用完）。

### 6.3 聊天 WebSocket

连接 `wss://ambertu.com/message/ws?token=<JWT>`，**后端零改动**。

App 侧需处理：前后台切换时断开/重连、指数退避重连、重连后拉取增量消息补齐断线期间的消息。

> 已知风险：后端在国内节点，海外 WebSocket 长连接稳定性与延迟均未经验证。见 §八 R1。

---

## 七、上架合规清单

| # | 要求 | 来源 | MVP 是否覆盖 |
|---|---|---|---|
| 1 | App 内账号删除入口 | App Store 5.1.1(v) | ✅ §5.4 |
| 2 | UGC：内容过滤 + 举报 + 拉黑 + 联系方式 | App Store 1.2 | ✅ §5.6 |
| 3 | 隐私政策 URL | 双平台 | ⚠️ **待产出** |
| 4 | 年龄分级 17+ | App Store | ⚠️ 提审时填写 |
| 5 | 数据安全表单 | Google Play Data Safety | ⚠️ **待填写** |
| 6 | GDPR / CCPA（若覆盖欧盟、加州） | 法规 | ⚠️ **待评估** |
| 7 | 无第三方登录时不强制 Apple 登录 | App Store 4.8 | ✅ MVP 仅邮箱 |

> 第 3、5、6 项不是代码工作，但**缺失同样会导致拒审**，需在提审前完成。

---

## 八、风险登记

| ID | 风险 | 影响 | 缓解 |
|---|---|---|---|
| **R1** | 后端在腾讯云国内节点，海外用户延迟 200–350ms，WebSocket 长连接稳定性未验证 | 聊天体验差，可能不可用 | MVP 阶段接受（已决策）。上线前用真实海外网络实测；预留迁移方案：海外 VPS + 数据库同步 |
| **R2** | 匿名陌生人社交是 App Store 高风险类目 | 首次提审被拒概率高 | 严格落实 §七 清单；准备审核说明文档；预留 2–3 轮提审时间 |
| **R3** | 后端 i18n 改造触及 `response.Fail` 等公共路径 | 可能影响小程序 | 所有改动走「新增变体 + 旧调用保持不变」，逐接口迁移；`go vet` + 小程序回归验证 |
| **R4** | MVP 无充值，用户金币耗尽即无法开聊 | 体验断档 | 已接受。必要时后台调高 `reg_reward_coins` 或开启签到 |
| **R5** | 无推送，用户收不到新消息提醒 | 留存差 | 已知，迭代 2 用 FCM 解决 |
| **R6** | `.en` 配置键可能使后台配置页项数翻倍 | 运营体验下降 | 只登记 MVP 实际用到的键；后台按语言分组展示 |

---

## 九、迭代路线图

| 迭代 | 内容 | 说明 |
|---|---|---|
| **0（本 spec）** | MVP 五屏 + 邮箱登录 + 账号删除 + i18n 基建 + 租户复用 | 目标：跑通闭环并具备提审资格 |
| 1 | Google 登录 + **Sign in with Apple**（4.8 强制配套）+ 找回密码 + 邮箱验证 | 登录体系完善 |
| 2 | FCM 推送 + OpenAI Moderation 内容安全 | 留存与合规补强 |
| 3 | 社交扩展：同城、扩列墙、动态广场、魅力周榜 | 内容丰富度 |
| 4 | 变现：Android 三方支付 + iOS IAP（含票据校验） | 需重新评估 Apple 30% 抽成 |
| 待定 | 后端海外节点迁移 | 由 R1 实测结果触发 |

---

## 十、验收标准（迭代 0）

**功能**

1. 邮箱注册 → 自动登录 → 首页，钱包显示注册赠币
2. 杀进程重启 App → 仍为登录态，直接进首页
3. 首页扔瓶成功；捞瓶能捞到瓶子；次数用完时提示正确
4. 详情页回信、解锁回信（扣币正确）、发起聊天（扣币正确）
5. 聊天双向收发，WebSocket 实时到达；断网重连后消息不丢
6. 聊天与详情页可举报、可拉黑
7. 「我的」可编辑资料、可退出登录
8. 账号删除后，原邮箱可重新注册；旧 JWT 立即失效
9. `Accept-Language: en` 时错误提示与后端文案为英文

**回归（不得破坏）**

10. 微信小程序完整回归：登录、扔瓶、捞瓶、聊天、我的页配置下发均与改动前一致
11. 管理后台配置页可正常读写，新增 `.en` 键不影响既有配置

**工程**

12. `cd server && go build ./... && go vet ./...` 通过
13. `cd app && flutter analyze` 无 error
14. 后端新增逻辑（密码哈希校验、Accept-Language 解析、i18n 渲染、账号删除状态判定）有不依赖 DB 的单元测试
15. `cd client && npm run build:mp-weixin` 通过（确认小程序未被影响）

---

## 十一、待决问题

| # | 问题 | 需要谁决策 |
|---|---|---|
| 1 | 隐私政策与数据安全表单由谁产出、托管在哪个域名 | 产品 / 法务 |
| 2 | App key（`app_credentials` 中 `platform="app"` 那行的 appid）取值与保密级别 | 技术 |
| 3 | MVP 是否放入签到入口以缓解 R4 | 产品 |
| 4 | 海外测试设备与网络环境如何准备（用于 R1 实测） | 技术 |
| 5 | App 与小程序是否共用同一 `tenant_id`（影响数据是否互通、用户是否跨端可见） | 产品 |

> **第 5 项优先级最高**：若共用租户，App 用户能捞到小程序用户的瓶子、互相聊天，冷启动内容更丰富；若隔离，则 App 是全新的空池子，需要机器人填充。这直接影响 MVP 的内容策略。
