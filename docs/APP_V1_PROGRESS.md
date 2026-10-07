# App V1 开发进度

| 字段 | 值 |
|---|---|
| 更新日期 | 2026-10-07 |
| 范围 | Flutter App（`app/bottles` 海外版 + `app/bottles_zh` 国内版）+ 后端增量（`server/`） |
| 验证状态 | `go build ./...` ✅　`go vet ./...` ✅　`flutter analyze` ✅　`flutter test` ✅ |
| 关联 | `docs/prototype/v1-screens.html`（53 屏原型）· `docs/superpowers/specs/2026-09-15-app-v1-scope.md`（17 模块范围）· `docs/superpowers/specs/2026-09-21-app-payment-flows-design.md`（支付全链路） |

---

## 零、本轮已确认的决策

这几条之前是待决项，现在已定并已落到代码里：

| # | 决策 | 落点 |
|---|---|---|
| T1 | **App 用独立 tenant_id**，与小程序隔离 | `user.resolveAppTenant` 三级解析：`app_credentials` 里 `platform="app"` 的行 → `APP_DEFAULT_TENANT_ID` → 单租户默认值。<br>App 只有一个包、租户是部署期常量，所以后加的中间那档让它**不必维护凭证行**（2026-09-17，线上取 `358804313465688064`）。<br>⚠️ 连带后果：App 冷启动是**空池子**，捞不到小程序用户的瓶子，内容需要机器人填充 |
| T3 | 地理服务用 **Google Maps** | `sysconfig.app_maps_provider=google` 时 `geo.regeo` 走 Google Geocoding；空值保持腾讯（小程序现状不变） |
| T2 | chat Hub **暂不改** Redis Pub/Sub | 保持单实例内存态。presence 因此做了两级降级（见下） |
| — | Analytics、图片审核 **本轮不做** | 无代码改动 |

---

## 一、后端：§18 的 10 个新增接口

**全部已实现。** 所有改动都是加法，小程序路径未被触碰。

| # | 接口 | 状态 | 实现位置 | 说明 |
|---|---|---|---|---|
| 1 | `POST /api/auth/otp/send` | ✅ | `user/otp.go` | Redis 存码带 TTL；**三维频控**：同标识重发冷却 / 单标识日限 / 单 IP 时限。<br>2026-09-17 起 body 里 `phone` 与 `email` **二选一**，两条链路共用 `issueOTP` 的闸门与键空间——换渠道绕不开频控 |
| 2 | `POST /api/auth/otp/verify` | ✅ | `user/otp.go` | 连错 5 次作废验证码；返回结构与 `/auth/login` 完全一致。同样支持 `email` |
| 3 | `POST /api/auth/google` | ✅ | `user/appauth.go` | JWKS 验签 + 核对 iss/aud/exp |
| 4 | `POST /api/auth/apple` | ✅ | `user/appauth.go` | 同上，公钥缓存 6h 且支持轮换 |
| 5 | `DELETE /api/user/account` | ✅ | `user/account.go` | 单事务匿名化 + 释放标识；旧 JWT 立即失效 |
| 6 | `GET /api/discover/users` | ✅ | `discover/service.go` | bounding box 预筛 + Haversine 精算 + 内存打分 |
| 7 | `POST /api/push/device` | ✅ | `push/device.go` | 设备令牌登记，换账号自动转移绑定 |
| 8 | `POST /api/pay/iap/verify` | ✅ | `pay/iap.go` | 按 `transaction_id` 幂等入账 |
| 9 | `POST /api/pay/iap/notify` | ✅ | `pay/iap.go` | 退款/撤销扣回金币，可记负账并冻结账号 |
| 10 | `GET /api/chat/presence` | ✅ | `chat/presence.go` | hub 在线 → 否则按 `LastActiveAt` 5 分钟窗口降级 |

顺带补的（原型里有但 §18 没列）：

- `GET /api/discover/count` —— 筛选页底部的「N 人符合」
- `POST /api/discover/skip` —— 左滑落库。**没有复用 MatchLog**：它的维度是 BottleID，拿来存「被滑过的人」语义不对；改用 `Relation` 加 `type=skip`
- `DELETE /api/push/device` —— 退出登录时解绑

### 数据模型变更（GORM AutoMigrate 自动加列）

`model.User` 新增：`Phone` / `Email` / `GoogleSub` / `AppleSub` / `Language` / `Interests` / `Lat` / `Lng` / `DeletedAt`

> `Email` 统一小写归一后存取——不归一会让 `Meera@…` 与 `meera@…` 注册出两个账号。
> 与 `Phone` 一样在注销时置空以释放标识。

> ⚠️ `DeletedAt` 用 `*time.Time` 而**非** `gorm.DeletedAt`——后者会开启 GORM 全局软删除，
> 静默改变现有所有查询的行为。账号是否有效一律以 `Status` 判断。

新表：`DeviceToken`（推送设备）、`IAPTransaction`（内购交易，主键即 Apple transaction_id）
`CoinPackage` 加 `IOSProductID`（两端定价必须分开，30% 抽成要在定价里吃掉）

### 新增配置（已写 defaults + 已登记后台白名单）

后台配置在 2026-09-21/22 重构成**分区 → 分组 → 键**三层，并按 `Tenant.Type` 过滤：
App 租户看到 133 项，小程序租户 174 项（详见 `.kfo/l1/admin-platform.md` §三）。

App 相关分组：`App 登录` / `App 完善资料` / `App 协议` / `App 推送` / `App 日志` / `App 发现`（在「App」分区），
`iOS 内购` / `支付联调`（「支付」分区），`App 文案` / `App 我的入口`（「界面与文案」分区），`App 客服`（「客服与跳转」分区）。
`地理服务` 与 `邮件发送 (SMTP)` 两端共用，不再是 App 专属分组。

**除 `app_ui_*` / `app_fn_*` / `app_contact_*` 外全部默认关或空**，不影响线上小程序。
那三批的默认值按「与 App 当前行为一致」定（文案空串 = 用内置 ARB，开关 `1`），所以上线是零行为变更。

> ⚠️ 这批键**刻意不复用**小程序的 `ui_text_*` / `fn_show_*` / `contact_*`：defaults 是全局的、没法按端分叉，
> 而小程序那边 `fn_show_recharge` 等默认 `"0"`、`ui_text_nav_title` 默认「漂流瓶」、`contact_text` 写着客服微信。
> 复用等于 App 上线当天丢三个入口、标题被改、客服指向一个打不开的微信号。

---

## 二、客户端：原型 29 屏

| 模块 | 屏 | 状态 |
|---|---|---|
| 1 Auth | A1 启动鉴权 / A2 手机号登录 / **A2e 邮箱登录** / A3 验证码 / A4 完善资料 | ✅ |
| 2 Bottle | B1 海洋 / B1n 夜场 / B1a 捞取 / B1b 开瓶 / B2 写瓶子 / B3 详情 / B4 我的瓶子 / B4s 漂流海图 | ✅ |
| 3 Match | C1 滑卡 / C2 筛选 / C3 用户主页 | ✅ |
| 4 Chat | D1 会话列表 / D2 聊天窗 | ✅ |
| 5–6 Relation·Moment | E1 关系 / F1 动态广场 / F3 动态详情 / 发动态 | ✅ |
| 7–9 Media·Push·Safety | G1 媒体上传 / G2 通知中心 / G3 举报拉黑 | ✅ |
| 10–14 变现 | H1 我的 / H2 钱包 / H3 充值 / H4 礼物 / H5 次数用尽 / H6 签到 | ✅ |
| 11⁺ 支付全链路 | H7a·H7b 支付中 / H8 成功 / H9 掉单 / H10 充值记录 / H11 退款已撤销 | ✅（2026-09-21，走 mock 渠道） |
| 11⁺ 支付全链路 | H3i IAP 列表 | ⬜ 仍缺，依赖 App Store Connect 商品 ID + StoreKit |
| 20 状态兜底 | Z1–Z5 空态/加载/失败/余额不足 | ✅ |

客户端还做了原型 §21 要求的动效：抛瓶抛物线、捞瓶 2.5s 仪式、送礼飞行（连送合并）、滑卡双通道判定。

---

## 二·五、真实接口对接（2026-09-17）

`AppConfig.useMock` 默认已翻成 **false**，App 直连后端。Mock 保留作离线改 UI 与 widget test 用。

### 响应形状：按 platform 分派

后端模型是小程序时代长出来的（主键 `user_id`/`bottle_id`、tags 逗号串、media 单列、作者平铺），
App 的 UI 按 V1 原型做，要的是 `id` + 数组 + 嵌套 author。两边都没错，只是长在不同年代。

解法是 `internal/common/appdto`：**同一批路由**，按 JWT 里的 `platform=app` 分派响应结构，
小程序走原分支零改动。没有另开 `/app/*` 路由——鉴权、限流、租户解析都已经挂在这批路由上，
复制一份只会多一处会漂移的地方。

### 已对接并验证的主链路

| 链路 | 改了什么 |
|---|---|
| 登录 | `/auth/otp/*` 返回的 user 转 `appdto.User`（`user_id`→`id`） |
| 资料 | `/user/profile` 客户端改取 `user` 键（原来取错层，字段全空）；`/user/update` App 端返回更新后的资料，且 gender 转 int8、languages 转逗号串 |
| 捞瓶 | **`scoop` 原来调错了路由**：`/bottle/scoop` 是不计次数的一批预览，`/bottle/scoop-one` 才是捞一个。两者对调；`tag`→`tags` |
| 抛瓶 | `images`→`media_url`、`city_only`→`scope`、补 `content_type`/`is_anonymous`/`night` |
| 配额 | `/bottle/quota` App 端补 `throw_total`/`scoop_total`（「次数用尽」弹窗要用） |

### 已知缺口（**别当它能用**）

| # | 缺口 | 现状 |
|---|---|---|
| 1 | 「我捞过的」瓶子 | 后端只有「我扔的」和「我收藏的」。要从 `MatchLog(action=view)` 反查，**接口不存在**。客户端该 tab 先返回空列表走空态，没拿「我扔的」冒充 |
| 2 | 解锁回信 | 后端 `POST /bottle/reply/:rid/unlock` 是**单条**维度返回 `{content}`，客户端签名是**按瓶子**批量返回列表。维度不同，改路径弥合不了。走到会 404 |
| 3 | `POST /ad/reward` | 路由表里**根本没有**这条 |
| 4 | `liked` / `collected` | 除 `/bottle/mine` 外恒为 false，详情页收藏按钮会一直显示未收藏 |
| 5 | `distance_km`、`bottle_count`、`moment_count`、`relation_stage` | 后端 profile 不提供，降级为 null/0，对应 UI 不显示 |
| 6 | 聊天 / 发现 / 动态三组 | **尚未逐字段核对**，本轮只保证主链路。方法内标 ⚠️ 的是已知不匹配 |

## 二·六、密码登录改造（2026-09-17）

从「验证码即登录」改成 **日常登录用密码，验证码只用于注册与找回密码**。手机号额外保留一条免密登录。

### 安全设计：验证码必须绑用途

`/auth/otp/send` 加 `purpose=register|reset|login`，Redis 码键变成 `otp:code:<purpose>:<标识>`。

> **不绑的后果**：攻击者只要知道受害者邮箱，就能以「注册」名义要一个码，再拿它走重置流程改掉密码。
> 这不是理论风险，是加密码登录时最容易漏的一环。

**码键带 purpose、频控键不带**，这个不对称是有意的：码按用途隔离，限额按标识合并——换用途或换渠道都绕不开三维频控。

### 新增接口

| 接口 | 说明 |
|---|---|
| `POST /auth/register` | 验证码 + 密码一次建号。标识已存在直接拒 |
| `POST /auth/login/password` | 日常登录。**账号不存在与密码错误返回同一句**，不给用户名枚举接口 |
| `POST /auth/password/reset` | 找回密码；也是老账号补设密码的唯一入口，成功后直接签发令牌 |
| `POST /auth/otp/verify`（保留） | 手机号免密登录 |

### 其他决策

- **密码只卡 8 位长度，不卡复杂度**。印度用户大量在手机小键盘输入，强制大小写+符号会显著掉注册转化；bcrypt(cost 10) + 登录频控已能挡住在线撞库
- **`guardPurpose` 会暴露「某标识是否已注册」**。这是权衡后的取舍——不然注册页只能在提交时才失败。防撞库靠频控，不靠藏这个信息
- 注册与找回密码形状完全相同（标识 → 验证码 → 设密码），客户端用同一个 `CredentialFlowPage`，靠 `flow` 参数分叉
- **邮箱不给「验证码登录」**：会和注册流程撞车。该入口只在手机号 Tab 出现

### ⚠️ 连带后果

**验证码时代注册的老账号没有密码**。手机号用户还能免密登录，**邮箱用户只能走「忘记密码」设一次密码**。登录接口对这类账号返回明确提示，不让人反复试密码。

线上 `358804313465688064` 目前是空池子，所以现在改代价最小。

---

## 二·七、远程配置（2026-09-22）

在此之前 App 只调三个配置接口（`/profile-options`、`/legal/:doc`、`/hook-count`），**没有配置文档的存放层**——
`core/config/app_config.dart` 是编译期常量，不联网。运营想调的东西（匿名发件人名、海洋页标题、付费墙文案、客服渠道）全部冻在包里，改一个字要发版。

### 后端

`GET /api/app-config` 一次下发三块：

```json
{ "support": {"text","image"},
  "mine":    {"wallet","recharge","wallet_log","items","blocklist"},
  "copy":    {"anon_sender","ocean_title","quota_title","quota_body"},
  "auth":    {"google_client_id"} }
```

- **免鉴权**：冷启动还没登录就要用。有 Bearer 取其租户；**没有则回落 App 租户**（`APP_DEFAULT_TENANT_ID`），不是小程序的 `DEFAULT_TENANT_ID`
- **语种走 `?lang=`，在 handler 里读，不挂语言中间件**——那条中间件只挂 `/admin/api`，`response/isolation_test.go` 守着。`/legal` 当初也是这么绕的
- 英文键为空回退中文键，中文也空才给空串。少配一门语言不该变成开天窗
- `/hook-count` 没并进来：它按当前时刻算数，缓存了就错了
- 没有合成一个「四合一」去复用 `/features` + `/mine-functions`：那两个的形状是小程序的，且冷启动串行四个请求首屏要么干等要么跳变

> ⚠️ **回落租户这条是踩过的坑（2026-09-22 线上）。** 首版用的是 `tenantOf`，它回落 `DEFAULT_TENANT_ID`——小程序租户。
> 文案与开关那批键没暴露问题，因为默认值全是「空串 / true」，两个租户都没配过、取回来一样；
> `app_google_client_id` 是第一个两边**真的不同**的值，于是这个接口下发了空串，客户端的 Google 登录修复在生产上等于没修。
>
> 修法是单开一个 `appTenantOf`，**没有并进 `tenantOf`**——`/notice` `/tabs` `/features` `/ads` 都在用那个，
> 一起改等于把小程序的免登配置整个换成 App 的。`sysconfig/app_config_test.go` 里两条测试守着。

### 客户端

`core/config/remote_config.dart` + `appConfigProvider`，三层合并（与 `LogConfig` 同一套路数）：
**内置 ARB → Prefs 缓存 → 本次拉取**。

做成同步 `Notifier` 而非 `AsyncNotifier`：调用点全在渲染路径上（标题、弹框文案、入口显隐），
给它们 `AsyncValue` 等于每处都写一遍 loading 分支，而「还没拉到」的正确表现本来就是用内置值，不是转圈。
首帧用缓存渲染，拉回来 Riverpod 自然重建；切语言会重拉（中英在服务端是两批键）。

落点：设置页客服行（配了弹文案+二维码，没配还是 `mailto:`）、「我的」5 个入口显隐、4 处文案，
以及 `oauthClientProvider` 的 `serverClientId`（Google 登录，见 §三A）。

> `google_client_id` **必须进 `toJson`**：冷启动第一帧走的是 Prefs 那份，漏了它就是「第一次点 Google 登录必失败，重启一次才好」。

### 顺带改的 ARB

`commonAnonymousMeta` 从 `"匿名 · {gender} {age}"` 改成 `"{name} · {gender} {age}"`。
不改的话运营配了匿名称呼，详情页作者行还是写死的「匿名」，同一页两个叫法。新增 `commonClose`。

### 明确没做

| 事项 | 原因 |
|---|---|
| **导航 tab 远程下发** | App 是 5 个固定 tab，`routes.dart:5` 明确不走 `/api/tabs`；小程序那 6 个是另一套页面 |
| **页面覆盖** | 小程序过审用的遮罩，Apple/Google 审核不吃这套 |
| **快捷回复** | App 的聊天与回信输入框都没有这个 UI |
| **关联小程序跳转** | 要接微信 OpenSDK 并做主体绑定，不是配置能解决的 |
| **广告配置** | App 没有任何广告 SDK，见 §三A |
| **「设置」「客服」开关** | 设置页装着退出登录/切语言/协议；客服是 App Store 1.2 对 UGC 应用的硬要求。配没了能直接导致下架 |

> ⚠️ **「看视频领金币」按钮目前是死的。** `ad/service.go:34` 要求 `ad_enabled` + `ad_reward_coin_on` + `ad_reward_unit 非空`
> 三者同时成立才发币，App 租户配不了广告分组，所以点了不发币也不报错。
> 挡住它的恰恰是这条——真放开广告配置而 AdMob 没接，就变成点一下白送币。**顺序必须是先接 SDK 再放配置。**

## 二·八、UI 对齐原型（2026-10-01 ～ 2026-10-03）

依据 `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md`（规范）与
`docs/superpowers/plans/2026-10-01-app-prototype-alignment.md`（30 个任务）。核心模型：
**逻辑像素 + 375pt 设计基准 + 约束布局吸收尺寸差异，绝不整页等比缩放。**

### 根与主题（一处改动全 App 见效）

| 项 | 落点 |
|---|---|
| 文字缩放钳 0.9～1.2、锁竖屏、edge-to-edge | `app/app.dart`、`main.dart` |
| Material 密度 compact、44pt 按钮下限、每档文字显式行高 | `core/design/theme.dart` |
| 断点只在一处：宽 600 / 矮屏 700 / 内容限宽 480 | `core/design/tokens.dart` `Breakpoints` |
| 阴影统一按原型 ×1.488 | `tokens.dart` `Shadows` |
| Tab 栏 27pt 图标 / 12pt 标签；宽 ≥ 600 内容限宽居中 | `app/shell.dart` |

### 组件库（`lib/ui/widgets/`）

新增：`DesignCanvas` / `CanvasPositioned`（场景画布）、`ConversationRow`、`ChatInputBar`、
`MessageBubble` / `SystemPill` / `GiftPill` / `ImageMessage`、`PhotoGrid` / `PhotoTileOverlay`、
`PaymentStatusView`、`PackageTile`、`CheckInStrip`、`QuadGrid`、`HeaderStats`、`SelectField`、
`RadioRow`、`StatusTag`、`OrderIdChip`、`ActionCircleButton`、`HiButton`、`BtnKind.oauth`。
对齐：`AppButton` / `MiniButton`、`NavBar`（44）、`BottomActionBar`（自己读安全区，调用方不包 SafeArea）、
`GradientHeader`、`ListRowItem`（44 / roomy 52）、`PillChip`（36）、`SegmentedRow`（44，选中铺满）、
`CoinChip`（30）、`AvatarRing`（在线点 15）、`SheetShell`（封顶 85%）、`ModalCard`（30 圆 ✕、金色头部）、
`EmptyState`（57 图标，矮屏 40）、`NoticeBanner`（线性图标）。

### 页面

全部页面头部有规范 §6 的登记注释（类别 / 固定区 / 弹性区 / 可滚动区 / 键盘）。
场景页：海面（画布 375×490）、启动页、登录、定位说明、滑卡、支付状态。
表单页多余高度规则：登录按 1:2 分到头部上方与底部；写瓶 / 发动态的文本框是弹性区。
硬编码中文迁入 ARB：第三方登录状态、账号与安全、定位说明、地点 / 地图、定位被拒。

### 验证

- `test/layout/all_pages_test.dart`：六档机型（360×780、375×667、393×851、430×932、720×860、360×780 @1.2）
  × 全部页面，聊天室 / 写瓶 / 发动态加键盘变体，海面加三键导航变体；失败时打印溢出控件的 file:line。
- `test/widgets/*`：组件尺寸断言；海面夜场 × 暗色四组合 golden。
- 比对工具 `app/bottles/tool/proto_render.py` + `compare.py`：原型按真机逻辑尺寸渲染后与截图并排。
- 这一轮矩阵抓出并修掉 30 余处窄屏 / 大字体溢出，以及两个老 bug（聊天室 dispose 里用 ref、
  `GridView(shrinkWrap)` 放进 `SliverFillRemaining` 抛错）。

### 真机返工（2026-10-04，小米 9 第二批截图）

七屏并排比对后修了十处，细目在计划末尾「返工清单」。两条升级为规范 §6 的铁律：
**Tab 根之下全是全屏页**（子路由挂 `parentNavigatorKey`，`test/app/router_test.dart` 守）；
**组件按内容自收缩**（`Container(alignment:)` 在 `Wrap` 里会撑满整行，横向滚动条在 `Column` 里会被居中）。
顺带修了 `tool/proto_render.py` 在本机挂死的三个坑（相对路径拼成坏 file URL、复用 profile 目录撞锁、stdin 未接 DEVNULL）。

### 待办

- 打包 MiSans（规范 D6）：等官方 TTF 放到 `app/bottles/tool/fonts_src/`，跑 `tool/subset_fonts.py` 后接 pubspec / theme。
- 真机核对（规范 §8.3）：小米 9 标准 / 大字体各一遍，截图放 `效果图/`，用 `compare.py` 并排。
- 其余页面的像素级 golden。

## 二·九、聊天扣费与送礼口径（2026-10-04）

真机第四轮反馈集中在「标价与实扣不一致」，本质是客户端自己算了价。一条规则收口：
**客户端不许自己算价，一律问服务端。**

| 键 | 含义 | 默认 |
|---|---|---|
| `price_chat` | 开聊扣 M | 5 |
| `price_msg` | 每条扣 N | 0（不扣） |
| `chat_free_msgs` | 每个会话**发送方**前 L 条免费 | 0 |

三键并入后台「价格」分组；真扣在 `chat/service.go`（`StartChat` / `SendMessage` + `withinFreeMsgs`）。
`/app-config` 新增 `pricing` 段，打招呼按钮标价与聊天页顶部提示都从这里取（**之前写死 5**）。
发现页的 rewind / skip 价格也进了 `pricing`，**不登录就能拿到**，首次撤回不再显示 0 金币。

> ⚠️ 低余额系统提示**只在 N>0 时发**——按条不收费却弹「余额不足」，用户点了没扣钱，提示是假的。
> ⚠️ App 与小程序是两个租户，改规则要改 App 那个租户。

**送礼**：`chat.SendGift` 与 `moment.SendGift` 统一成「背包先抵扣、差额扣币」，同一事务内完成，
每单位记一条 `ItemOrder(target)`，所以聊天里送的礼也进礼物墙；客户端只对差额标价。
**点赞**：首次点赞给被赞者加魅力，只增不减，取消赞不扣回。
**瓶子回信**：整瓶解锁改为**按条解锁**（`/bottle/reply/:rid/unlock`）。

其它同批：`Config.Value` 改 longtext（中英文协议正文此前被 255 截断）、`/legal` 未登录时解析 App 租户
（此前读小程序租户，登录页永远显示「准备中」）、`User.Birthday` + 服务端算年龄、性别只能设一次、
self DTO 补 `bottle_count` / `moment_count`（「我的」页此前所有人都是 0 个瓶子）。

详见 `.kfo/l2/2026-10-04-chat-pricing-and-gift-bag.md`。

## 二·十、海面玩法与运营手段（2026-10-05）

- **海面**：真实瓶子美术（六槽六倾角，全部落在地平线以下、避开灯塔），**点一下漂浮的瓶子就捞**
  （删掉预览气泡）；昼 / 黄昏（17:00–21:00）/ 夜三态，夜态优先；登录页与启动页复用同一个 sprite。
- **漂流轨迹**：服务端把 `MatchLog` 聚合成按城市的节点（thrown / seen / replied），内嵌进瓶子详情、
  我的瓶子、轨迹三个响应；**捞过这只瓶子的人才看得到**。App 详情页渲染「这只瓶子去过哪」时间线。
- **运营**：后台手动调币（`wallet.AdminAdjust`，只动余额，流水带备注、场景 `admin`）、
  在线人数曲线（`online_base` + 24h 曲线 + 按分钟确定性抖动 `online_jitter`）、
  远程启动图 `app_splash_image_1..5`（经 `/app-config` 的 `splash.images`）、道具账本 `GET /item/orders`。
- **机器人按语言生成**：档案（名字 / 城市 / 兴趣 / 简介）成套按 `robot_language` 生成，
  LLM 回复与搭讪开场白都跟随该机器人的语言——**这是出海前置**，此前英文用户看到的是一池中文昵称。

详见 `.kfo/l2/2026-10-05-sea-artwork-drift-trace-ops.md`。

## 二·十一、服务商配置独立化（2026-10-06 / 10-07）

第三方服务商的凭据过去分两处（`app_credentials` 表 + 散落的 sysconfig 键），加一家要动四个地方。
现在统一落 `provider_configs` 表，**一行 = 租户 × 域 × 服务商**，字段整体 AES 加密成 JSON blob，
由 `internal/provider/schema.go` 的声明式 `Definition` 描述。
**加一家服务商 = 加一个 `Definition` + 一个适配器。**

| 域 | 服务商 | 选择 |
|---|---|---|
| `pay` | 微信支付 / 支付宝 / Apple IAP / Google Play | 并存 |
| `map` | 腾讯 / Google / **高德**(新) / **百度**(新) | 单选 |
| `moderation` | 微信 / **支付宝**(新) | 单选 |
| `sso` | 微信 / 支付宝 / Google / Apple | 并存，全部 App 专属 |

后台侧边栏新增「服务商」四页。渠道名统一为 `wechat` / `alipay` / `apple` / `google_play`，
历史订单值经 `canonicalChannel` 折回，旧回调路径仍可达。新增 `POST /pay/play/verify`。

**SSO 这一域解决的是「能不能用」此前有三套答案**：微信查开关+凭证行、支付宝查了错的凭证行
（它的密钥其实在支付卡片上）、Google 与 Apple 根本没查过——按钮无条件渲染，租户想关都关不掉。
现在 `/app-config` 的 `auth` 段统一按**「启用 且 必填齐全」**下发四个布尔，两端登录页据此显示按钮；
支付宝登录卡片用 `DependsOn` 声明依赖 `pay/alipay`。

> ⚠️ 迁移标记两个且必须独立：`provider_migrated`（支付/地图/审核）与 **`sso_migrated`**。
> 线上前者早已是 `1`，SSO 复用它等于迁移永不执行。
> ⚠️ 已删键：`app_login_wechat_enabled` / `app_login_alipay_enabled` / `app_google_client_id` /
> `app_apple_bundle_id` / `app_wechat_universal_link`；`app_credentials` 的 `wx_app` / `alipay_app` 不再被读。

详见 `.kfo/l2/2026-10-06-provider-configs.md`、`.kfo/l2/2026-10-07-sso-provider-configs.md`、
`.kfo/l1/provider-configs.md`。

## 二·十二、国内版 App 与主动匹配（2026-10-06 / 10-07）

**国内版 `app/bottles_zh`**：`app/bottles` 的分支而非开关，包名 `com.ambertu.bottles.cn`，
独立租户 `drift_app_cn`。Android 用微信 / 支付宝登录与支付，iOS 用微信 / 支付宝 / Apple 登录 + IAP。
服务端全部是加法；客户端**只换 `oauth.dart` 与两个 `PayChannelAdapter`，支付六屏状态机一行不动**
——2026-09-21 留的扩展点第一次兑现。海外版 `app/bottles` 一个字没改。
详见 `.kfo/l2/2026-10-06-app-zh-china-build.md`。

**主动匹配（火花）**：在线真人每隔随机 N–M 分钟被系统配一个人（`spark_real_ratio` 概率配真人、
其余配机器人），弹窗后点进去**免开聊费**。调度是每分钟一次 tick + 每人一个 Redis TTL 键，
不是每个在线用户一个 timer。机器人配对**预建会话 + 发开场白再弹窗**；真人配对双向弹窗但不预建，
点击走 `POST /spark/accept`。

> ⚠️ **`/spark/accept` 必须校验 Redis 里的配对记录**，否则任何人都能和任意人免费开聊、绕过 `price_chat`。
> ⚠️ `spark_interval_min > spark_interval_max`（运营填反）会让 `rand.Intn` panic，**把常驻调度器静默打死**——
> `nextInterval` 自带交换与全 0 守卫。

10 个配置键全部默认关或空，总开关 `spark_enabled` 默认关；与机器人搭讪共用每日计数键
`outreach:<租户>:<uid>:<日期>`，当天互斥。详见 `.kfo/l1/spark.md`。

## 三、还没做完的

### A. 需要外部凭证才能继续（代码框架已就位）

| 事项 | 卡在哪 | 代码现状 |
|---|---|---|
| **短信真实发送** | 印度需先在 **TRAI DLT 平台**注册实体与模板（开户前置，非代码工作）；还要选服务商 | `user/otp.go: sendSMS()` 留了分支点，provider 为空时只打日志 |
| **邮件真实发送** | 选服务商（SendGrid / SES / 阿里云邮推）+ 给发信域名配 **SPF / DKIM / DMARC**，不配验证码直接进垃圾箱 | `user/otp.go: sendEmail()` 与 `sendSMS` 同构，provider 为空时只打日志。**其余链路已完整**：频控、归一、登录建号都通 |
| **Google / Apple 登录** | 只差**后台填值**：后台「服务商 → 登录」页的 Google / Apple 两张卡片，填在 **App 租户**下（Apple 还需 Apple Developer 开 Sign in with Apple）。<br>⚠️ 2026-10-07 起旧键 `app_google_client_id` / `app_apple_bundle_id` **已删除**，填在那里不再有任何效果 | **代码两端都已打通**（2026-09-22 修完）：客户端 `core/platform/oauth.dart` 接了 `google_sign_in` + `sign_in_with_apple`，取到真 idToken 后交 `loginWithProvider`；服务端 JWKS 验签完整。<br>两张卡片的 audience 都**接受逗号分隔的多个 aud**——Android 签出来的 aud 是 Web client ID，iOS 是 iOS 的，只填一个必然拒掉一端。约定**第一个是 Web client ID**，`/app-config` 下发的就是它（Android 拿它当 `serverClientId`，拿不到则 `idToken` 恒为 null）|
| **Google 地图（App 选地点页空白）** | Maps key 所在的 Google Cloud 项目**没开通计费**：Static Maps / Geocoding 用同一把 key 直接返回 `You must enable Billing`，Maps SDK for Android 则表现为米色空白 + Google 水印（2026-10-04 真机现象）。处理：给该项目绑卡开通 Billing → 启用 **Maps SDK for Android** → 把 key 限制到 Android 应用（包名 `com.ambertu.bottles` + 调试 / 正式签名 SHA-1，正式签名还没配见 `build.gradle.kts` TODO）。另：大陆网络不翻墙也看不到瓦片，测试机需能访问 Google | key 从 `android/local.properties` 的 `MAPS_API_KEY` 注入 manifest，代码链路完整；选点功能不依赖瓦片（坐标 + 逆地理走自家 `/geo/regeo`），所以空白地图下「用这个地点」仍可用 |
| **IAP 真实入账** | 需要 App Store Connect 商品 ID、Issuer ID、.p8 私钥 | 服务端校验与幂等已完整；**客户端未接 StoreKit**。<br>客户端渠道适配器已就位（2026-09-21）：接 StoreKit 时实现一份 `StoreKitAdapter` 即可，**六个屏幕与状态机的代码不用动** |
| **Android 支付（海外）** | 需要印度收款主体资质（RBI 规定持牌 PA）+ 渠道商户号 | `pay.Driver` 接口现成，加一份 Driver 即可。<br>**国内版已经接完**（2026-10-06）：微信支付 / 支付宝两份 Driver + 两个 `PayChannelAdapter`，见 §二·十二 |
| **FCM / APNs 下发** | 需要 Firebase 项目与 APNs 证书 | 设备登记已完成；**下发逻辑未写**（`push/device.go` 只到 `ActiveDevices`） |
| **AdMob 激励视频** | 需要广告位 ID；且「是否要做广告」本身还没定（原 C5 冲突） | 入账接口 `POST /api/ad/reward` 现成，客户端是占位——`purchase_flows.dart:87` 直接调入账、全程不播广告。<br>**现在这按钮一个币都发不出来**（服务端要求 `ad_reward_unit` 非空，App 租户配不了广告分组），所以暂时无害。<br>⚠️ 顺序必须是**先接 SDK 再放配置**：反过来做，运营一开就是点一下白送币 |

### B. 纯工程，可以直接排期

| 事项 | 说明 |
|---|---|
| ~~**支付全链路 6 屏落地**~~ | ✅ **已完成**（2026-09-21，见 `docs/superpowers/specs/2026-09-21-app-payment-flows-design.md`）。<br>H7a/H7b/H8/H9 四态共用一条路由 `/me/recharge/pay/:orderNo`，H10 `/me/recharge/records`，H11 `/me/wallet/voided`。<br>客户端抽了 `PayChannelAdapter`（`lib/core/pay/pay_channel.dart`），本轮只有 `MockChannelAdapter`；状态机在 `payment_controller.dart`，轮询 `GET /pay/order/:orderNo`，退避 1→2→4→8s，累计 30s 转掉单。<br>服务端新增单笔查单接口与一个**默认关闭**的 mock 渠道（`pay/driver_mock.go`，结算复用幂等的 `HandleCallback`）。<br>⚠️ **上线前必须确认后台「App 支付」分组的 `app_pay_mock_enabled` 为关闭状态**——开着等于任何登录用户都能凭空发币。<br>顺带修掉三个既有 bug：客户端丢弃 `order_no`、iOS 给 `/pay/iap/verify` 传 `package_id`（必 400）、`TxnScene` 不认 `refund`（退款流水显示成「奖励」） |
| **对象存储 + CDN** | `upload` 仍落本地磁盘、`/static` 裸服务。客户端压缩已做（1080 长边 + JPEG 82），服务端缩略图与 CDN 没有。**对印度用户这是体验硬伤** |
| **视频上传** | V1 清单里 Media 模块含 video，转码链路完全没开始 |
| **海报保存/分享的权限兜底** | `Gal` / `SharePlus` 已接，但用户拒绝相册权限时的降级路径没测 |
| **i18n 后端下发** | **管理后台那一半已做**（2026-09-21，见 `docs/superpowers/specs/2026-09-21-admin-i18n-design.md`）：`internal/common/i18n` 按 `Accept-Language` 解析语言，`response.Fail`/`Abort` 在出口查表翻译，admin 的配置项标签全部双语内联。<br>**C 端仍是中文**：语言中间件只挂 `/admin/api`，客户端路由拿不到语言上下文（`response/isolation_test.go` 守着这条线）。<br>App 要接入只需两步：把 `i18n.Middleware()` 挂到 C 端路由组，再把 C 端消息的译文补进 `i18n/catalog.go`。通知模板仍未做。<br>**这里说的是错误消息**；运营文案那条线已经通了（2026-09-22，`/app-config` 按 `?lang=` 下发，见 §二·七），两者互不依赖 |

### C. 已知取舍（清楚代价，本轮故意不做）

| 事项 | 代价 |
|---|---|
| **chat Hub 单实例** | 后端只能单机部署。多实例会丢消息，也会让 presence 只看到本机连接 |
| **presence 降级** | 没有真 presence 时按「5 分钟内活跃」判定，会把刚离开的人显示成在线 |
| **discover 打分权重写死** | 语言/兴趣/距离/活跃度的权重是常量，建议后续随捞瓶 feed 一起进 sysconfig |
| **IAP JWS 不本地验签** | 改为带 API 私钥主动向苹果核实。没配 .p8 时只接受沙盒票据，生产必须配齐 |
| **反作弊（模块 15）** | 除 OTP 三维频控外没有任何统一反作弊。邀请奖励、金币异常、支付欺诈都没防 |
| **Analytics（模块 17）** | 按你的要求本轮不做，无埋点体系 |
| **图片审核** | 按你的要求本轮不做。微信 `mediaCheckAsync` 在 App 端**完全失效**，当前 App 侧图片零审核——提审前必须补 |

---

## 四、提审前必须闭环的合规项

| # | 要求 | 状态 |
|---|---|---|
| 1 | App 内账号删除入口 | ✅ 设置页 + `DELETE /api/user/account` |
| 2 | UGC 四件套：过滤 / 举报 / 拉黑 / 联系方式 | ⚠️ 举报拉黑✅、联系方式✅、文本过滤✅（本地词库），**图片审核 ❌** |
| 3 | 隐私政策 URL | ❌ 客户端已留入口，指向占位地址，**正式文本待产出** |
| 4 | 年龄分级 17+ | ⬜ 提审时填写 |
| 5 | Google Play 数据安全表单 | ❌ 待填写 |
| 6 | 有 Google 登录则必须有 Apple 登录 | ✅ 两端代码都已打通（SDK 已接、验签完整）；Apple 按钮仅 iOS 显示（`showAppleSignIn`）。<br>⚠️ 剩下的是**配置**：后台「服务商 → 登录」的 Apple 卡片要在 App 租户下填上 bundle ID，不填则按钮根本不显示（`/app-config` 按「启用且必填齐全」下发） |
| 7 | iOS 数字商品走 IAP | ✅ 充值页已按平台分叉，iOS 只显示 App Store |

---

## 五、环境问题（与代码无关，但挡住了验证）

**Android 构建**：已修复，排查过程见 `docs/APP_DEBUG_GUIDE.md` 第 1 节（空代理环境变量 / Proxifier 虚拟 DNS / 跨盘符增量编译三个独立的坑）。

**模拟器**：2026-09-17 修好，AVD `drift_api35`（Android 35 · Google APIs · x86_64）可用，
`flutter emulators --launch drift_api35` 直接起。

病根同「坑 2」：`dl.google.com` 被 Proxifier 虚拟 DNS 映射成回环地址，
而**下载器是 Java 进程，不在 Proxifier 规则里**，于是静默连不上，`system-images` 一直是空的。
绕法不是关 Proxifier，而是用**被规则匹配的进程**（PowerShell）直接下 zip 再解压进 SDK——
完整命令见 `docs/APP_DEBUG_GUIDE.md` 第 3 节。

连带处理：`ANDROID_AVD_HOME=D:\Android\avd`。C 盘只剩 8.5 GB，而镜像解压后就占 3.5 GB。

**依赖锁定，不要随手升级**（升级会让 `flutter test`/`build` 在任何平台直接失败）：

- `flutter_secure_storage` 锁 9.x
- `path_provider_foundation` override 到 2.4.1

原因：两者在新版本会引入 `objective_c` 的 native-assets hook，而该 hook 引用了 Flutter 3.47 已移除的 `Architecture.arm64e`。

**已知的既有 flaky 测试**（改动前就存在，已用干净 HEAD 副本验证）：
`internal/robot` 的 `TestSanitizeOutgoingReply_*` / `TestBuildIdentityResponse_*` 因回复池随机选取而随机失败。

---

## 六、建议的下一步顺序

1. **线上补 `APP_DEFAULT_TENANT_ID=358804313465688064`** —— 这是 App 端一切接口的前提，不配就登录不了。
   ⚠️ `.env` 在 gitignore 里、`deploy.js` 不会带上去，要手动改服务器上那份并重启 `driftbottle`
2. 补图片审核（提审硬门槛）+ 隐私政策文本
3. ~~支付全链路 6 屏落地~~ ✅ 已完成（2026-09-21）。下一步是接真渠道：Play Billing 服务端（`play-billing` 计划 Task 2–5）+ 各自的 Adapter
4. 对象存储 + CDN（印度用户体验硬伤）
5. 短信服务商 + DLT 注册、邮件服务商 + 域名 SPF/DKIM/DMARC（周期长，建议并行启动）
6. Google/Apple 登录 SDK + IAP StoreKit（需要账号与证书）
