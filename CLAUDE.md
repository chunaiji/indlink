# CLAUDE.md

漂流瓶匿名社交小程序（多租户 SaaS）。Go 后端 + uni-app 双端小程序 + Vue3 管理后台。

## 产品形态（重要）

双形态小程序，由 sysconfig 远程开关控制：
- **工具形态**（审核/默认）：「小纸条水印相机」，启动页 `pages/privacy`
- **社交形态**：漂流瓶（扔瓶/捞瓶/私聊/动态广场/送礼/签到），tab 显隐、页面覆盖、功能开关全部走后台配置

**一切新增用户可见功能必须挂 sysconfig 开关且默认关闭**，工具形态下绝不露出。

## 目录

```
server/   Go 后端(Gin + GORM v2 + MySQL + Redis),入口 cmd/api,模块在 internal/<域>/
client/   uni-app Vue3(alpha) + Pinia,微信/支付宝双端
admin/    Vue3 + Vite 管理后台(线上 /message-admin/)
docs/     DEV_RUNBOOK.md(本地起服务/联调)、superpowers/plans/(功能规划)
.kfo/     知识库(L1 域文档/L2 迭代复盘/L3 问题档案)
deploy.js / deploy_admin.js   部署脚本(根目录)
```

## 常用命令

```bash
# 后端本地跑
cd server && go run ./cmd/api          # 需本地 MySQL/Redis,配置见 .env
go build ./... && go vet ./...         # 提交前验证

# 小程序(微信开发者工具导入 client/dist/dev/mp-weixin)
cd client && npm run dev:mp-weixin     # watch 模式;⚠️ 新增页面(改 pages.json)后必须重启 watch
cd client && npm run build:mp-weixin   # 发行构建(验证语法用)

# 管理后台
cd admin && npm run build

# 部署(编译 → 上传 → systemd 重启,幂等)
cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api
node deploy.js          # 后端 → 175.178.182.166
node deploy_admin.js    # 管理后台静态文件
```

## 部署环境

- **生产服务器 175.178.182.166**（2026-07-31 起），SSH 密钥 `~/.ssh/huawei_app_ed25519`；旧机 43.136.54.189 已废弃勿部署
- nginx 站点配置在 `/etc/nginx/conf.d/pet.conf`（不是 nginx.conf），`/message/`→8980、`/message-admin/`→静态
- 服务 `systemctl status driftbottle`，日志 `journalctl -u driftbottle -f`
- 线上：API `https://ambertu.com/message/api`，后台 `https://ambertu.com/message-admin/`
- 密钥在 `deploy.local.json`（gitignore），表结构靠 GORM AutoMigrate（启动自动加表/列）

## 硬约束（历史踩坑,违反必出线上事故）

1. **sysconfig 新 key 必须同步写 defaults**（`internal/sysconfig/sysconfig.go`），空串会导致开关逻辑反转
2. sysconfig key 要进管理后台必须登记 `internal/admin/meta.go` 白名单：**中英文标签各一条**（`LabelZh`/`LabelEn`）+ 分组用 `Group<Xxx>` 常量（不是中文字符串）+ 类型（`image` 支持后台直接上传图片）。漏填英文标签 `meta_test.go` 会点名报错
3. **ID 一律字符串下发**（json tag 加 `,string`），JS 端 int64 精度丢失；前端传 ID 同样 String()
4. **统计时间口径按东八区**：勿用 `time.Now().Truncate(24h)`（按 UTC 截断偏 8 小时），参照 `internal/admin/overview.go` 的 periodStarts
5. GORM v2 `.Order()` 只认 string，`clause.Expr` 被静默忽略——动态排序用字符串拼接
6. 多租户：所有表带 tenant_id，所有查询按租户过滤（tenantID=0 仅限后台全量汇总）
7. 微信 scroll-view 会拦截 tap——页面滚动布局用 view + min-height
8. **`app_pay_mock_enabled` 上线前必须关**（默认 `0`）：开着等于任何人都能调 `POST /pay/mock/settle` 凭空发币。它只服务联调，`buildDriver` 仅在该开关为 1 时才返回 mock 渠道

## uni-app 编译三坑（本项目 alpha 版特有,写模板必读）

1. **事件必须写调用表达式**：`@tap="fn()"` / `@tap="fn($event)"`；裸方法名 `@tap="fn"` 编译后不调用。**自定义组件事件同理**：`@send="onSend($event)"`
2. **弹层禁用 `@tap.stop`**（不阻止冒泡）：遮罩关闭用**背景层分离**模式——mask 容器不绑事件，内部 absolute 铺满的 bg 层绑关闭，panel 作 bg 的兄弟节点
3. **tab 页内弹层 z-index ≥ 1000**：自定义 tabBar z-index=999，低于它底部按钮被挡住点不到

## 前端构建两坑（排查"改了代码不生效"先看这里）

1. **多个 dev watch 进程互相覆盖产物**：历史会话残留的 `dev:mp-weixin` 进程会用旧源码状态盖掉新编译输出。改动不生效时先杀干净再全量重编：
   `Get-CimInstance Win32_Process | ? { $_.CommandLine -match 'mp-weixin' } | % { Stop-Process -Id $_.ProcessId -Force }`，删 `dist/dev/mp-weixin` 后重启 watch
2. **两个产物目录**：`dist/dev`（watch,带日志）与 `dist/build`（发行,build:mp-weixin 手动构建才更新）。开发者工具导入哪个要确认——排查时先核对工具项目路径,再让用户点「编译」重载

## 关键机制速查

- **功能开关下发**：`GET /api/features`（留存功能/动态广场/关联小程序），前端 `store/features.js`；tab/页面覆盖走 `/tabs` `/pages-config`
- **捞瓶 feed**：Redis 预生成队列（`bottle/feed.go`），打分权重后台可调；深夜瓶（night 标签）只在夜场时段可捞，缓存 key 带昼夜维度
- **捞瓶行为记录在 MatchLog**（action=view/reply/like/skip）——漂流轨迹、去重都靠它
- **送礼三处**（聊天/动态）统一链路：扣币事务 + ItemOrder(target) + User.Charm 累加 + 通知；礼物墙/魅力周榜从 ItemOrder 聚合，送礼后调 `rank.InvalidateWeekCache`
- **机器人**：LLM 人格回复 + 身份防泄露三层防护（`robot/identity_guard.go`），出站消息统一 sanitize
- **聊天扣费规则**（2026-10-04）：后台「价格」分组三个键——`price_chat` 开聊扣 M、`price_msg` 每条扣 N（默认 0=不扣）、`chat_free_msgs` 每个会话发送方前 L 条免费（默认 0）。真扣在 `chat/service.go`（`StartChat` / `SendMessage` + `withinFreeMsgs`），App 只从 `/app-config` 的 `pricing` 段取数显示（打招呼按钮标价、聊天页顶部提示）。低余额系统提示只在 N>0 时发。App 与小程序是两个租户，改规则要改 App 那个租户
- **推送**：微信订阅消息 5 场景模板（sysconfig 配模板 ID），定时任务在 `push/scheduler.go`（签到 09:00、深夜场开场前 5 分钟）
- **主动匹配（火花）**（2026-10-07）：在线真人每隔随机 N–M 分钟被系统配一个人（`spark_real_ratio` 概率配真人、其余配机器人），App 弹窗后点进去**免开聊费**。调度在 `internal/spark`（每分钟扫 WS 在线用户，每人一个 Redis TTL 键）。机器人配对预建会话 + 发开场白；真人配对双向弹窗但不预建，点击走 `POST /spark/accept`——**那个接口必须校验 Redis 配对记录**，否则任何人都能和任意人免费开聊。与 `robot/outreach.go` 共用每日计数键 `outreach:<租户>:<uid>:<日期>`，当天互斥。总开关 `spark_enabled` 默认关。
- **微信凭证同源**：access_token 的 appid/secret **优先取租户凭证表**（`app_credentials`,与登录同源,后台「租户凭证」页维护），sysconfig 推送分组仅后备——新租户配一次凭证,登录/推送/内容安全全通（`push.getAccessToken`）
- **内容安全**（微信过审要求,`moderation/wxcheck.go`）：文本 `CheckUGC`（本地词库→msgSecCheck v2,7 个发布场景全覆盖,场景值 1资料/2评论/4社交）；图片 C 端上传统一入口 + 水印相机选图静默上传,提交 `mediaCheckAsync` 留档；服务商与开关在后台「服务商 → 内容安全」页(微信 / 支付宝单选,默认都没启用)；API 失败/无 openid 放行不阻断。结果回调依赖微信"消息推送"(会影响客服消息)暂未启用,违规图靠人工巡查
- **服务商配置**（2026-10-06）：支付 / 地图 / 内容安全的服务商凭据统一在 `provider_configs` 表（后台侧边栏「服务商」三页，按租户），schema 在 `internal/provider/schema.go`；加一家服务商 = 加一个 `Definition` + 对应域的适配器。`app_credentials` 只管登录；旧 sysconfig 的支付 / 地图 / 审核键已删，启动时 `provider.Migrate` 一次性搬值（标记 `provider_migrated`）。支付渠道名统一为 `wechat` / `alipay` / `apple` / `google_play`，历史值经 `canonicalChannel` 折回，旧回调路径仍可达
- **第三方登录服务商化**（2026-10-07）：微信 / 支付宝 / Google / Apple 四家的凭证与开关统一在 `provider_configs` 的 `sso` 域（后台「服务商 → 登录」页），`/app-config` 的 `auth` 段按「启用且必填齐全」下发四个布尔，两个 App 的登录页据此显示按钮。**支付宝登录的密钥不在登录卡片上**——它与支付同一个应用、同一把私钥，登录卡片用 `DependsOn` 声明依赖 `pay/alipay`，支付那边缺密钥会连带判定登录不可用。迁移标记是 `sso_migrated`，**不是** `provider_migrated`（后者线上早已是 1，复用等于永不执行）。旧键 `app_login_wechat_enabled` / `app_login_alipay_enabled` / `app_google_client_id` / `app_apple_bundle_id` / `app_wechat_universal_link` 已删；`app_credentials` 的 `wx_app` / `alipay_app` 两行不再被读，凭证页也不再接受这两个平台值。
- **接口日志**：所有外呼(内容安全/订阅推送/逆地理)经 `pkg/apilog` 异步落库,后台「📡 接口日志」页可查,保留 7 天——排查"有没有调、返回什么"先看这里
- **App 支付链路**（2026-09-21，原型 §11⁺ 六屏）：下单接住 `order_no` → 跳渠道 → 轮询 `GET /pay/order/:orderNo`（限本人）判结果，状态机在 `features/me/payment_controller.dart`。渠道走适配器 `core/pay/pay_channel.dart`，现有 mock 一份；**接 StoreKit / Play Billing 只需再加一份适配器，六个屏与状态机不动**。⚠️ mock 渠道见硬约束 8
- **管理后台多语言**（中/英，2026-09-21）：前端 vue-i18n，文案在 `admin/src/locales/*.json`（两个文件 key 必须一致，`npm run build` 前置校验会卡）；后端 `internal/common/i18n` 按 `Accept-Language` 翻译，消息表**以中文原文为 key**，加新错误消息要同步补 `catalog.go`（`internal/admin` 的 AST 测试会点名）。<br>⚠️ **语言中间件只挂 `/admin/api`**，C 端恒定中文——`response/isolation_test.go` 守着这条线，别挂到全局。<br>⚠️ 写模板注意：`v-for="t in ..."` 会遮蔽翻译函数 `t`，循环变量要另起名；模块级常量不能存翻译后的字符串（不响应语言切换），存 key 或改 computed
- **地址水印**：水印相机定位(静默,拒绝授权降级)→ `GET /geo/regeo` 服务端代理当前生效的地图服务商(腾讯/Google/高德/百度,在后台「服务商 → 地图」页单选;没配降级经纬度)
- **App 远程配置**（2026-09-22）：`GET /api/app-config` 一次下发文案/「我的」入口显隐/客服/Google client ID，免鉴权，语种走 `?lang=`（**在 handler 里读,不挂语言中间件**——那条线由 `response/isolation_test.go` 守着）。App 侧 `core/config/remote_config.dart` + `appConfigProvider`，三层合并：内置 ARB → Prefs 缓存 → 本次拉取，**同步给值不阻塞启动**。<br>⚠️ **免鉴权时它回落 App 租户（`appTenantOf` / `APP_DEFAULT_TENANT_ID`），不是 `tenantOf` 的小程序租户**——App 与小程序是两个租户。首版用错回落，把 Google client ID 下发成空串，客户端登录修复在生产上等于没修；文案/开关那批没暴露是因为默认值两个租户都一样。**别图省事并进 `tenantOf`**，`/notice` `/tabs` `/features` `/ads` 还在用它。<br>⚠️ **这批键刻意不复用小程序的 `ui_text_*` / `fn_show_*` / `contact_*`**：defaults 是全局的没法按端分叉，而 `fn_show_recharge` 等默认 `0`、`ui_text_nav_title` 默认「漂流瓶」，复用等于 App 上线当天丢入口、改标题。App 用 `app_ui_*` / `app_fn_*` / `app_contact_*`，**文案默认空串(空=用内置 ARB)、开关默认 1**，配置上线是零行为变更；两端各有一条测试守着（`sysconfig/app_config_test.go` 与 `test/remote_config_test.dart`）。<br>⚠️ 没有「设置」和「客服」开关：客服是 App Store 1.2 对 UGC 应用的硬要求,配没了能直接导致下架

## 协作约定

- 回复用中文；代码/标识符/commit message 英文（conventional commits）
- 不主动跑测试/脚本，每次运行需明确授权；git 操作仅在明确指示时执行
- 提交前后端跑 `go build ./... && go vet ./...`，前端跑 `npm run build:mp-weixin` 验证
