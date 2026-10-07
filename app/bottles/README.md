# Drift · 漂流瓶 App（Flutter）

按 `docs/prototype/v1-screens.html`（17 模块 / 22 屏）实现的 Flutter 客户端。
与小程序共用同一套 Go 后端，所有后端改动必须是加法。

设计与范围依据：
- `docs/prototype/v1-screens.html` —— 视觉与交互原型（本工程的唯一 UI 依据）
- `docs/superpowers/specs/2026-09-15-app-v1-scope.md` —— V1 模块范围与冲突登记
- `docs/superpowers/specs/2026-09-14-flutter-app-overseas-design.md` —— 分层、依赖选型、i18n 方案
- `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md` —— 页面自适应规范（多机型 / 安全区 / 字体缩放 / 设计画布），所有 UI 开发与评审的基准

---

## 跑起来

```bash
cd app/bottles
flutter run                      # 默认 Mock 数据源，不需要后端
```

接真实后端（App 端接口就绪后）：

```bash
flutter run \
  --dart-define=USE_MOCK=false \
  --dart-define=API_BASE=https://ambertu.com/message/api \
  --dart-define=WS_BASE=wss://ambertu.com/message/ws \
  --dart-define=APP_ID=drift_app_dev
```

> 租户解析顺序：`app_credentials` 里 `platform="app"` 那行 → 服务端的 `APP_DEFAULT_TENANT_ID` →
> 单租户默认值。配了第二项就不用管 `APP_ID` 传什么。

验证：

```bash
flutter analyze     # 应为 No issues found
flutter test        # 全量；布局矩阵单独跑：flutter test test/layout/all_pages_test.dart
python tool/proto_render.py --screen B1 --size 393x851   # 原型按真机尺寸渲染，比对用（见 tool/README.md）
flutter gen-l10n    # 改过 ARB 后重新生成
```

> **为什么默认 Mock**：原型里标虚线的接口（OTP 登录、Google/Apple 登录、
> `GET /api/discover/users`、`POST /api/push/device`）后端尚未实现。
> Mock 后端是**有状态**的——扣币、配额、会话、轨迹都会真实变化，
> 因此「解锁回信扣 2 币 → 余额不足 → 跳充值」这类链路可以完整走通。

---

## 目录

```
lib/
  main.dart                 入口：先读本地偏好，再起 ProviderScope
  app/
    app.dart                MaterialApp.router + 主题 + 多语言 + 前后台生命周期
    router.dart             go_router：登录态守卫 + 5 Tab StatefulShellRoute
    routes.dart             路由常量（页面之间只认这里的路径）
    shell.dart              底部 5 Tab 外壳
  core/
    design/tokens.dart      色板 / 字号 / 圆角 / 间距 / 动效参数（对齐原型 CSS 变量）
    design/theme.dart       明暗两套 ThemeData
    config/app_config.dart  dart-define 配置
    network/                Dio 客户端、错误码映射、WebSocket（指数退避重连）
    storage/                JWT（Keychain/Keystore）、偏好（SharedPreferences）
    utils/                  时间口径、JSON 解析
    catalog.dart            语言 / 兴趣 / 标签的展示名
    providers.dart          Riverpod 根：仓储切换、登录态、钱包、配额、未读
  domain/models/            共享领域模型（跨 feature，避免循环依赖）
  data/
    repositories.dart       仓储接口（注释里标了每个方法对应的真实后端路由）
    mock/                   内存假后端 + Mock 仓储
    remote/                 Dio 实现（后端就绪后 USE_MOCK=false 直接切）
  ui/
    widgets/                共享组件（按钮四级语义、chip、卡片、骨架、弹层、海面场景）
    flows/                  跨模块业务流程（举报拉黑、礼物面板、次数用尽/余额不足）
  features/                 auth · bottle · discover · chat · moment · me
  l10n/                     app_zh.arb（模板）· app_en.arb · 生成产物
```

**与 spec §4.2 的一处偏离**：共享领域模型放在 `lib/domain/models/` 而不是各 feature 内部。
`Bottle` / `Conversation` / `Moment` 都要引用 `UserBrief`，放在 feature 内会形成循环依赖。

---

## 屏幕对照

| 原型 | 屏幕 | 实现 |
|---|---|---|
| A1 | 启动鉴权 | `features/auth/splash_page.dart` |
| A2 | 手机号登录 | `features/auth/login_page.dart` |
| A3 | 验证码 | `features/auth/otp_page.dart` |
| A4 | 完善资料（两步） | `features/auth/onboarding_page.dart` |
| B1 / B1n | 海洋首页 / 夜场 | `features/bottle/ocean_page.dart` |
| B1a / B1b | 捞取中 / 开瓶 | `features/bottle/scoop_flow.dart` |
| B2 | 写瓶子 | `features/bottle/write_bottle_page.dart` |
| B3 | 瓶子详情与解锁 | `features/bottle/bottle_detail_page.dart` |
| B4 | 我的瓶子 · 轨迹 | `features/bottle/my_bottles_page.dart` |
| B4s | 漂流海图（可分享） | `features/bottle/drift_map_page.dart` |
| C1 | 滑卡发现 | `features/discover/discover_page.dart` + `swipe_deck.dart` |
| C2 | 筛选 | `features/discover/filter_page.dart` |
| C3 | 用户主页 | `features/discover/user_profile_page.dart` |
| D1 | 会话列表 | `features/chat/chat_list_page.dart` |
| D2 | 聊天窗 | `features/chat/chat_room_page.dart` |
| E1 | 关系 | `features/me/relations_page.dart` |
| F1 | 动态广场 | `features/moment/moment_feed_page.dart` |
| F3 | 动态详情 | `features/moment/moment_detail_page.dart` |
| G1 | 媒体上传 | `features/moment/post_moment_page.dart`（并入发布页，见下） |
| G2 | 通知中心 | `features/me/notifications_page.dart` |
| G3 | 举报与拉黑 | `ui/flows/safety_flows.dart`（聊天/瓶子/动态/主页四处复用） |
| H1 | 我的 | `features/me/me_page.dart` |
| H2 | 钱包流水 | `features/me/wallet_page.dart` |
| H3 | 充值（平台分叉） | `features/me/recharge_page.dart` |
| H4 | 礼物面板 | `ui/flows/gift_flows.dart` |
| H5 | 次数用尽 | `ui/flows/purchase_flows.dart` |
| H6 | 签到与奖励 | `features/me/rewards_page.dart` |
| Z1–Z5 | 空态 / 加载 / 失败 / 余额不足 | `ui/widgets/states.dart` + 各页 |

两处对原型的有意调整：

1. **A4 拆成两步**：第 1 步身份（头像/名字/年龄/性别），第 2 步匹配维度（语言/兴趣/定位）。
   原型缩略图把 6 组字段挤在一屏，真机上要滚很长，且「第 1 步，共 2 步」的标注需要有实际的第 2 步。
2. **G1 并入发布页**：选图交给系统选择器（`image_picker`），独立的「选择图片」屏会与系统 UI 重复；
   原型 G1 真正承载的信息（已选 N/9、压缩进度、上传进度、失败重试）保留在发布页内。

---

## 设计系统要点（都在 `core/design/tokens.dart`）

- **按钮颜色是语义，不是审美**：海蓝实心 = 主线免费动作；粉实心 = 要花金币或真钱；
  灰描边 = 次要；警示实心 = 破坏性。花币的次要动作用**粉描边**，保证「粉 = 付费」在任何层级成立。
  直接结果：**海洋首页整片不出现粉色**。
- **渐变只用于品牌面**（头图 / Tab 选中块 / 进度条），按钮一律实色。
- **夜场与系统暗色是两根独立的轴**：夜场（21:00–01:00）是产品状态，只改头图渐变与海面配色；
  `--brand` 粉色不参与昼夜叙事。四种组合都要成立。
- 字号只用 8 档、圆角按层级 6 档、间距 4pt 栅格、可点区域下限 44pt。
- 动效参数（滑卡阈值、捞瓶仪式时长、弹层进出曲线）见 `Motion`。

---

## 已知限制与待办

**后端未实现（Mock 已按预期形态先行）**
- `POST /api/auth/otp/send|verify`、`POST /api/auth/google|apple`
- `GET /api/discover/users`、`GET /api/discover/count`（需先给 `User` 补 language / interests / 经纬度）
- `POST /api/push/device`（FCM + APNs 设备 token 表）
- `GET /api/bottle/scoop?peek=1`（海面预览，只读不消耗次数；数据本身是现成的）

**客户端待补**
- 对象存储 + 海外 CDN、服务端缩略图——客户端压缩已就位（见下），但 `upload` 仍落本地磁盘裸服务
- Google / Apple 登录按钮要换成官方 SDK 组件（品牌规范强制，自绘会被打回），需要 clientId / Apple 开发者配置
- 激励视频接 AdMob（现有 `ad` 模块是微信流量主，App 端作废），需要广告位 ID
- 视频上传与转码（V1 清单里的 Media 模块含 video，属全新工作）

**已经接上的（不必再排期）**
- **图片压缩**：`core/media/image_pipeline.dart`，1080 长边 + JPEG 82，选图四处（头像 / 瓶子 / 动态 / 聊天）统一走它；
  发动态页显示真实的「原图 → 压缩后」体积对比。压不动或失败时回退原图，不挡发布链路。
- **海报保存与分享**：`Gal.putImageBytes` 落相册、`SharePlus` 拉系统分享面板，海报仍是客户端本地合成。
- **抛瓶 / 送礼两组仪式动画**：按原型 §21 的参数实现，送礼连送合并成一次特效。
- **收藏、关注**：接入真实仓储，收藏联动「我的瓶子 · 已收藏」分页。
- **在线状态降级**：后端没有 presence 能力时按 `LastActiveAt` 显示「最近活跃」，而不是一律离线。
- **聊天健壮性**：乐观发送（先上屏再等 ACK）、失败消息留在列表里可点重发、
  WebSocket 重连后拉增量补齐断线消息且保留本地未发出的内容、断线期间顶部提示。

**合规（提审前必须闭环）**
- 账号删除入口 ✅ 设置页；举报/拉黑 ✅ 四处；开发者联系方式 ✅ 设置页
- 隐私政策 URL、Google Play 数据安全表单、17+ 年龄分级：**待产出**
- iOS 数字商品必须走 IAP（3.1.1），第三方渠道会被拒审；充值页已按平台分叉，IAP 票据校验待接

**依赖锁定（不要随手升级）**
- `flutter_secure_storage` 锁在 9.x，`path_provider_foundation` override 到 2.4.1：
  10+ / 2.6.0 会引入 `objective_c` 的 native-assets hook，它引用了 Flutter 3.47 已移除的
  `Architecture.arm64e`，导致 `flutter test` / `flutter build` 在任何平台直接失败。
- 已移除 `windows/` 平台目录：Windows 桌面构建需要系统开启开发者模式（symlink 支持），
  在未开启的机器上会让 `flutter test` 也一起失败。需要时开启后执行
  `flutter create --platforms=windows .` 重新生成。

**本机 Android 构建环境（与本工程代码无关）**

`flutter build apk` 目前跑不通，卡在 Android SDK 工具链而不是项目配置：

```
Android license status unknown.
Process 'command '...\cmdline-tools\latest\bin\sdkmanager.bat'' finished with non-zero exit value 1
```

`sdkmanager` 自身启动即抛 `java.net.URL` 异常（常见于 `cmdline-tools` 版本与代理/镜像设置冲突）。
处理顺序：先让 `sdkmanager --version` 能正常执行，再 `flutter doctor --android-licenses` 接受许可。
在此之前，本工程的验证手段是 `flutter analyze` + `flutter test`，两者均通过。

原生侧该配的已经配好：`AndroidManifest.xml` 加了网络权限、相册写入（maxSdkVersion 29）、
url_launcher 的 `<queries>`；`Info.plist` 加了相册读写用途说明与 `LSApplicationQueriesSchemes`。
