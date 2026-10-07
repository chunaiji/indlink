# L1 设计 — 管理平台(admin)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-09-22 |
| 覆盖 | `server/internal/admin/*` · `server/internal/sysconfig` · `admin/`(Vue 工程) |
| 关联 | L2 `l2/2026-06-21-admin-platform.md` · L2 `l2/2026-06-23-ws-reconnect-online-status.md` · L2 `l2/2026-06-24-admin-tenant-refactor.md` · spec `docs/superpowers/specs/2026-09-21-admin-i18n-design.md` |

---

## 一、定位

运营/管理用后台,**与 C 端用户体系完全隔离**:独立账号表、独立 token scope、独立路由前缀、独立前端工程。

## 二、鉴权隔离

- 账号:`AdminUser`(bcrypt),与 `User` 无关。
- Token:`scope=admin` 的 JWT,复用 `cfg.JWTSecret` 签名但 scope 校验保证与 C 端 token 不可互用;`authMiddleware` 注入 `admin_id/name/role`。
- 默认账号:`admin.Init` 在无管理员时按 env 播种(生产务必改密/改 env)。

## 三、配置中心

- 复用 `internal/sysconfig`(`Config` KV 表 + 内存缓存 + `Set` 热刷新)。取值三级回退:**租户值 → 全局(tenant 0) → 代码 defaults**;租户存空串视为未设,继续回退。
- 后台只允许改 `admin/meta.go` 的**白名单键**(防止写入任意键污染);前端按 `group/type` 渲染表单。

### 三层元信息(2026-09-21/22 重构)

配置项两百多条,挤在一页找不着东西,现在是**分区 → 分组 → 键**三层,分区与分组的 code 都是稳定 ASCII,只有标签翻译:

| 层 | 定义处 | 说明 |
|---|---|---|
| 分区(section) | `sectionOrder` / `sectionLabels` | 8 个:App / AI 与机器人 / 界面与文案 / 运营与变现 / 支付 / 广告 / 客服与跳转 / 系统与安全。前端路由 `/config/:section`,链接可分享 |
| 分组(group) | `groupMeta`(`groupInfo{LabelZh,LabelEn,Section,Platform}`) | 三十余组,每组挂在一个分区下并声明适用端 |
| 键(field) | `configMeta`(`configFieldMeta`) | 单键可用 `Platform` 覆盖所属分组的端 |

**按端过滤。** `Tenant.Type`(`miniprogram`/`app`)决定看见哪些:`effectivePlatform(field, group)` → `visibleFor(platform, tenantType)`。实测小程序租户 174 项、App 租户 133 项。`tenantID=0`(全部租户视图)不过滤。一项都不可见的分区不出 Tab。

**机密脱敏。** 9 个键标 `Secret: true`(`app_otp_dev_code` / 短信与 SMTP 凭证 / `app_iap_key_enc` / `app_google_map_key` / `push_wx_secret` / `llm_api_key` / `geo_qq_key`):服务端下发 `is_set` 而**永不下发值**,前端渲染密码框 + 显式「清除」按钮,提交空串是 no-op 而非清空。
> ⚠️ 这是**传输层脱敏,不是加密**。sysconfig 仍以明文落库,键名里的 `_enc` 是命名不是加密(消费方一律 try-decrypt-else-plaintext,见 `mailer.go:79`)。静态加密未做。

**跨分区搜索。** 名称、key、分组名三者任一命中即出;搜索时 Tab 选中态熄灭并显示命中横幅,避免误以为结果受当前 Tab 约束。

**双语标签。** 每个键必须同时有 `LabelZh` + `LabelEn`,分组用 `Group<Xxx>` 常量而非中文字面量。`meta_test.go` 的 AST 测试会点名漏填的键。

### 端专属配置

小程序与 App 的页面结构不同,**不共用同一批键**:

| | 小程序 | App |
|---|---|---|
| 导航 | `tab_*` 6 个可配 tab | 5 个固定 tab(`routes.dart:5` 明确不走 `/api/tabs`) |
| 我的入口 | `fn_show_*` 15 项 | `app_fn_*` 5 项 |
| UI 文案 | `ui_text_*` 7 项 | `app_ui_*` 4 项 × 中英 |
| 客服 | `contact_*` + 关联小程序 18 项 | `app_contact_*` 3 项 |
| 广告 | `ad_*` 24 项(微信流量主) | 无(App 未接广告 SDK) |
| 共用 | 首页计数 `hook_*`、价格、限额、机器人、LLM、匹配权重、SMTP、地理 | |

> ⚠️ **为什么不共用**:defaults 是全局的,没法按端分叉。`fn_show_recharge` 等默认 `"0"`(小程序过审要求默认隐藏)、`ui_text_nav_title` 默认「漂流瓶」、`contact_text` 默认写着客服微信——而 App 这些入口常显、标题是「今晚的海」、客服是邮箱。复用等于 App 上线当天丢入口、改标题、客服失效。
>
> App 那批键的默认值按「与 App 当前行为一致」定:**文案默认空串(空=用内置 ARB)、开关默认 `1`**,所以这批配置上线是零行为变更。两端各有一条测试守着(`sysconfig/app_config_test.go` 的 `TestAppConfigDefaultsAreInert`、`app/bottles/test/remote_config_test.dart`)。

### C 端下发

配置不经后台直接给 C 端,而是各有一个只读接口,全部免鉴权(见 `sysconfig/handler.go`):
`/notice` `/tabs` `/mine-functions` `/hook-count` `/ads` `/pages-config` `/features` `/legal/:doc` `/profile-options` `/app-config`。

`GET /api/app-config`(2026-09-22)是 App 的聚合入口,一次拿文案 + 入口显隐 + 客服 + Google client ID;语种走 `?lang=`,**在 handler 里读,不挂语言中间件**(见 §八)。

### ⚠️ 免鉴权时回落哪个租户,两套

| 解析器 | 用于 | 无 Bearer 时回落 |
|---|---|---|
| `tenantOf` | `/notice` `/tabs` `/features` `/ads` `/mine-functions` `/hook-count` `/pages-config` `/legal` `/profile-options` | `DEFAULT_TENANT_ID`(小程序租户) |
| `appTenantOf` | **只有** `/app-config` | `APP_DEFAULT_TENANT_ID`;为 0(单租户部署)时再回落 `DEFAULT_TENANT_ID` |

App 与小程序是**两个租户**(见 `l1/tenant-saas.md`),而 `/app-config` 冷启动时还没登录。
2026-09-22 线上就栽在这:`/app-config` 当时用的是 `tenantOf`,回落到小程序租户,
于是把 Google client ID 下发成空串 —— 客户端那侧的登录修复因此在生产上等于没修。
之前那批文案/开关键没暴露问题,是因为它们默认值全是「空串 / true」,两个租户都没配过,取回来长得一样;
`app_google_client_id` 是第一个两边**真的不同**的值。

> 不要把这条合并进 `tenantOf` 图省事 —— 那个还服务着上表第一行那些小程序接口,
> 一起改等于把小程序的免登配置整个换成 App 的。
> `TestAppConfigFallsBackToTheAppTenant` 与 `TestAppConfigFallsBackToDefaultWhenAppTenantUnset` 守着这两条。

## 四、路由与部署边界(关键约束)

- 后台 API 一律 `/admin/api/*`,**不可**与 SPA 静态挂载 `/admin/*filepath` 同时由 gin 注册(catch-all 冲突)。
- SPA 走独立 Vite 工程:开发用 dev server + 代理;生产用 nginx 托管 `admin/dist`。Go 服务不直接吐 SPA。

## 五、WS 在线状态（RobotChats 页）

`admin.Service` 持有 `chatSvc *chat.Service`，通过 `SetChatService()` 在 `main.go` 初始化时注入（`adminSvc.SetChatService(chatSvc)`）。

- `GET /admin/api/online`：调 `chatSvc.Hub().OnlineUserIDs()`，返回当前在线用户 ID 列表
- `admin/src/views/RobotChats.vue`：每 8s 轮询，`onlineSet`(Set\<string\>) 驱动对话列表和面板头部的绿/灰状态点

> `chatSvc` 为 nil 时（理论上不应发生）返回空数组，不 panic。

## 六、租户选择器（2026-06-24 实装）

Admin 前端全局维护"当前租户"上下文，所有 API 调用自动附带：

- **`admin/src/tenant.js`**：`tenantStore`（reactive），持久化到 localStorage
- **`admin/src/api.js`**：`req()` 自动注入 `X-Tenant-ID: {id}` header
- **`admin/src/views/Layout.vue`**：顶栏租户下拉选择器，切换时整页刷新
- **`server/internal/admin/handler.go`**：`tenantFromCtx(c)` 读 header/query，替代 `defaultTenantID()`
- **Service 层 LIST 方法**：`tenantID=0` 不加 WHERE（全部租户），`tenantID>0` 按租户过滤

写操作在 tenantID=0 时仍 fallback `defaultTenantID()`，防止全部租户视图下误建数据。

## 八、多语言(2026-09-21)

后台中/英双语,三层各自解决:

| 层 | 方案 |
|---|---|
| 前端文案 | vue-i18n,`admin/src/locales/{zh-CN,en}.json`。**两个文件 key 必须一致**,`npm run build` 的前置校验 `scripts/check-locales.mjs` 会卡住不一致的构建 |
| 配置项标签 | `meta.go` 里 `LabelZh`/`LabelEn` 内联,按 `i18n.Lang` 出参时选 |
| 错误消息 | `internal/common/i18n` 按 `Accept-Language` 解析;`response.Fail`/`Abort` 在**出口**查表翻译,97 个调用点零改动。消息表**以中文原文为 key**(`catalog.go`) |

> ⚠️ **语言中间件只挂 `/admin/api`,C 端恒定中文。** `wx.request` 在部分机型会带 `Accept-Language: en`,挂到全局会让小程序中文用户突然收到英文报错。`response/isolation_test.go` 守着这条线:同一个 `Accept-Language: en`,不挂中间件必须出中文、挂了必须出英文。
>
> 需要按语种下发的 C 端接口自己在 handler 里读 `?lang=`(`/legal/:doc`、`/app-config` 都是这么做的),不要去动中间件。

> ⚠️ 拼接出来的错误消息(`"更新失败:"+err.Error()`)永远匹配不上以字面量为 key 的消息表,而 AST 扫描也看不见它(是 BinaryExpr 不是 BasicLit)。统一走 `response.FailErr(c, code, zhPrefix, err)`;`TestNoConcatenatedMessages` 会拦住新写的拼接。

## 九、扩展点

- 机器人(#1):内容池表 + cron 管理页;读 `robot_*` 配置。
- 匹配(#8):权重 `match_w_*` 已就位,`feed.go` 打分直接读。
- ~~多租户:`sysconfig` 当前全局~~ —— **已实现**:`Config` 表带 `tenant_id`,取值租户优先、回退全局与 defaults(见 §三)。
- 机密静态加密:目前只做传输层脱敏,落库仍是明文(见 §三)。
- 内容审核供应商框架 + App 借用小程序审核能力:**已论证但未做**。凭证(access_token)是租户级、可借;但 `msgSecCheck` 要的 `openid` 是「用户 × 小程序」的配对,App 用户没有,借不了。
