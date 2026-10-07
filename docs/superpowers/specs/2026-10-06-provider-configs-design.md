# 服务商配置（支付 / 地图 / 内容安全）独立化 设计

> 日期：2026-10-06
> 范围：Go 后端（新表 + 服务商抽象 + 3 个新接入）、管理后台（侧边栏新分组「服务商」三页）、`/config` 瘦身
> 前置：`2026-06-20-saas-multitenant-credentials-design.md`（`app_credentials`）、`2026-10-06-app-zh-wechat-alipay-design.md`（微信/支付宝 App 支付与登录）

---

## 一、目标与现状

### 1.1 要解决什么

运营方（不管租户是小程序还是 App）需要在后台**自行**配置用哪家服务商、填什么凭据：

| 领域 | 服务商 | 现状 |
|---|---|---|
| 支付 | 微信支付、支付宝、Google Play、iOS IAP | 微信/支付宝凭据在 `app_credentials` 行里（按平台拆成 `wx`/`wx_app`/`alipay`/`alipay_app` 四种行）；IAP 在 sysconfig「iOS 内购」分组；Play **只有价格表，没有校验链路**；开关散在「iOS 内购」「支付联调」「小程序支付」三个分组 |
| 地图（逆地理编码） | Google、高德、百度、腾讯 | `app_maps_provider` 二选一（google/tencent），腾讯 key 挂在「水印相机」分组、Google key 挂在「地理」分组；高德、百度**未接入** |
| 内容安全（文本 + 图片） | 微信、支付宝 | 只有微信 msgSecCheck / mediaCheckAsync，开关在「内容安全」分组，凭据借用微信登录 appid/secret；支付宝**未接入** |

问题：同一个服务商的字段散在两套机制（`app_credentials` 列 + sysconfig 键）、三四个分组里；新加一家服务商要改 model、credstore、meta、Vue 四处；运营找不到该填哪。

### 1.2 不在本设计范围

- App / 小程序**端上**的地图 SDK 切换（Flutter 国内版换高德/腾讯地图组件）。本设计只覆盖服务端逆地理编码。
- Google Play **客户端**接入（`in_app_purchase`）。本设计补齐服务端校验与配置，客户端另开。
- 微信内容安全的异步回调（依赖「消息推送」，现状未启用，不变）。
- 退款 / 对账。

---

## 二、决策记录

| # | 决策 | 理由 |
|---|---|---|
| D1 | 新表 `provider_configs`：`(tenant_id, kind, provider)` 唯一，`enabled` + 加密 JSON `fields`；支付 / 地图 / 内容安全全部走它 | 一家服务商一行、字段随服务商定义走，加服务商只加一个 Go 定义 + 零表结构变更 |
| D2 | `app_credentials` **只保留登录职责**（`wx`/`wx_app` 的 appid+secret、`alipay`/`alipay_app` 的 appid、`app` 的 appid→租户映射）；支付字段一次性迁入 `provider_configs`，迁完的列不再写、保留只读 | 登录与支付本就是两件事（小程序登录用 secret，支付用商户号）；回调反查租户改走 `provider_configs` 的索引 |
| D3 | 支付宝的应用私钥 / 支付宝公钥 / PID **只存一份**：在 `provider_configs(kind=pay, provider=alipay)`；支付宝登录从这里取密钥，`app_credentials` 的 alipay 行只剩 appid | 同一个支付宝应用的密钥登录和支付共用，存两份必然配歪一份 |
| D4 | 服务商定义是**服务端代码里的声明式 schema**（字段名、类型、是否机密、中英标签、适用租户类型），后台页面按 schema 渲染 | 与 `/config` 的 `meta.go` 同一思路；前端不硬编码字段 |
| D5 | 每个领域同时只能启用的服务商数量：支付**可多选**（微信 + 支付宝 + IAP + Play 并存）；地图**单选**（`active` 字段指定当前用哪家）；内容安全**单选** | 支付是客户端选渠道；地图和审核是服务端分派，必须唯一 |
| D6 | `/app-config` 的 `pay.wechat/alipay` 可用性 = `provider_configs` 该行 `enabled && 必填项齐全`；原 `app_pay_wechat_enabled` / `app_pay_alipay_enabled` 两个开关、IAP 的 5 个键（`app_login_wechat_enabled` 等登录开关与 `app_wechat_universal_link` 属登录范畴，不动）、`app_maps_provider`、`app_google_map_key`、`geo_qq_key`、`sec_check_text_on/image_on` 全部**迁移后删除**（含 meta 白名单与 defaults） | 两处开关必然打架；迁移脚本把旧值搬过去，删键不丢数据 |
| D7 | `app_pay_mock_enabled` 保留在 sysconfig「支付联调」分组不动 | 它不是服务商，是联调后门，放进支付页会被当成正式渠道 |
| D8 | 高德 / 百度逆地理用 Web 服务 REST 接口（`restapi.amap.com/v3/geocode/regeo`、`api.map.baidu.com/reverse_geocoding/v3`），百度支持可选 SK 签名 | 与现有腾讯 / Google 的实现同形：一个 GET、一个 key |
| D9 | 支付宝内容安全用 `alipay.security.risk.content.detect`（文本）与图片检测走同一网关，复用 `internal/common/alipay` 的签名客户端；凭据复用 `kind=pay, provider=alipay` 那行的密钥，审核行只存 `enabled` + `app_id` 引用 | 支付宝开放平台同一应用、同一把私钥；再要一份私钥等于 D3 的反面 |
| D10 | Google Play 服务端校验：Service Account JSON（机密）+ 包名；`POST /pay/play/verify {product_id, purchase_token}` 走 `androidpublisher/v3 purchases.products.get`，入账幂等键 `orderId`，表 `play_purchases` 已有 | 对称 IAP 的 `/pay/iap/verify`；客户端接 `in_app_purchase` 时只需调它 |
| D11 | 每张服务商卡片有「测试连通」：支付宝 → `alipay.trade.query` 查一个不存在的单号（返回 `ACQ.TRADE_NOT_EXIST` 即签名/网关通）；微信 → 查单同理（`ORDERNOTEXIST`）；地图 → 对天安门坐标逆地理；内容安全 → 检测固定一句话；IAP → 签一个 JWT 调 App Store Server API 的 `inApps/v1/lookup` 空查询；Play → 取 access token | 运营填完当场知道对不对，不用等用户付钱才发现证书贴错 |
| D12 | 侧边栏新分组「服务商」：支付 / 地图 / 内容安全 三页，跟随顶部租户选择器；「全部租户」时页面只读提示「请先选择租户」 | 服务商配置没有「全局默认」这回事，每个租户都是自己的账号 |

---

## 三、数据模型

```go
// ProviderConfig 一租户 × 一领域 × 一服务商 一行。
type ProviderConfig struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	TenantID  int64     `gorm:"uniqueIndex:uk_tenant_kind_provider,priority:1;index" json:"tenant_id,string"`
	Kind      string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:2" json:"kind"`     // pay / map / moderation
	Provider  string    `gorm:"size:16;uniqueIndex:uk_tenant_kind_provider,priority:3" json:"provider"` // wechat / alipay / apple / google_play / google / amap / baidu / tencent
	Enabled   bool      `json:"enabled"`
	// Active 单选领域(map / moderation)里「当前用谁」。同租户同 kind 至多一行为 true,写入时事务内互斥。支付领域恒 false。
	Active    bool      `gorm:"index" json:"active"`
	// FieldsEnc 字段 JSON 的 AES-GCM 密文(整包加密:机密字段与非机密字段混在一个 JSON 里,省得两列对账)。
	FieldsEnc string    `gorm:"type:text" json:"-"`
	// LookupA / LookupB 回调反查租户用的明文索引值,由 schema 指定哪两个字段投影过来:
	//   wechat: A=mch_id, B=platform_serial;  alipay: A=app_id;  apple: A=bundle_id;  google_play: A=package_name
	LookupA   string    `gorm:"size:128;index:idx_lookup_a"`
	LookupB   string    `gorm:"size:128;index:idx_lookup_b"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}
```

字段 JSON 解密后是 `map[string]string`。schema 决定哪些键合法；未知键写入时丢弃。

### 服务商 schema（`internal/provider/schema.go`）

```go
type Field struct {
	Key     string
	LabelZh string
	LabelEn string
	Type    string // text / textarea / bool / select
	Secret  bool   // 机密:列表接口只回 "已设置",写空跳过(与 /config 同规则)
	Required bool  // 「配置完整」判定用;enabled && complete 才算可用
	Options []string // select 用
	HelpZh, HelpEn string
}

type Definition struct {
	Kind       string // pay / map / moderation
	Provider   string
	LabelZh, LabelEn string
	Platform   string // both / miniprogram / app  —— 对哪种租户显示
	Fields     []Field
	LookupA, LookupB string // 投影到索引列的字段键
	DocURL     string // 申请入口
}
```

**支付**

| provider | Platform | 字段 |
|---|---|---|
| `wechat` | both | `app_id`(小程序或开放平台 AppID，必填)、`mch_id`(必填)、`apiv3_key`(机密,必填)、`serial_no`(必填)、`private_key`(机密,textarea,必填)、`platform_key`(textarea,必填)、`platform_serial`(必填)、`notify_url`(默认 `https://ambertu.com/message/api/pay/callback/wechat`)。**租户类型决定交易类型**：小程序 → JSAPI，App → APP；不再需要 `wx`/`wx_app` 两种行 |
| `alipay` | both | `app_id`(必填)、`private_key`(机密,必填)、`alipay_public_key`(textarea,必填)、`pid`(App 授权登录要)、`notify_url`、`sandbox`(bool，走沙箱网关；取代按 appid 前缀猜) |
| `apple` | app | `bundle_id`(必填)、`issuer_id`(必填)、`key_id`(必填)、`p8_key`(机密,必填)、`sandbox`(bool，允许沙盒票据) |
| `google_play` | app | `package_name`(必填)、`service_account_json`(机密,textarea,必填) |

**地图（单选）**

| provider | 字段 |
|---|---|
| `tencent` | `key` |
| `google` | `key` |
| `amap` | `key` |
| `baidu` | `ak`、`sk`(机密,可选；填了走 SN 签名) |

**内容安全（单选）**

| provider | Platform | 字段 |
|---|---|---|
| `wechat` | miniprogram | `text_on`(bool)、`image_on`(bool)；凭据复用 `app_credentials` 的 `wx` 行（access_token 已有同源逻辑）|
| `alipay` | both | `text_on`、`image_on`；密钥引用 `kind=pay,provider=alipay`（页面上显示「使用支付配置里的支付宝应用」，未配时禁用保存并给跳转）|

---

## 四、服务端设计

### 4.1 `internal/provider` 包

```go
type Store struct{ db *gorm.DB; mu; cache map[key]*Resolved; ver atomic.Uint64 }

type Resolved struct {
	TenantID int64; Kind, Provider string; Enabled, Active bool
	Fields map[string]string
}

func (s *Store) Reload() error                                   // 全量解密进缓存(与 tenant.Store 同款)
func (s *Store) Get(tenantID int64, kind, provider string) (*Resolved, bool)
func (s *Store) Active(tenantID int64, kind string) (*Resolved, bool)      // 单选领域当前生效的那家(enabled && active)
func (s *Store) Usable(tenantID int64, kind, provider string) bool         // enabled && 必填齐全
func (s *Store) ByLookup(kind, provider, a, b string) (*Resolved, bool)    // 回调反查:a 必配,b 可空
func (s *Store) Version() uint64
func (s *Store) Upsert(tenantID int64, kind, provider string, enabled, active bool, fields map[string]string) error // 合并写:机密空值保留旧值;active=true 时同 kind 其它行置 false
func (s *Store) List(tenantID int64, kind string, lang) []Card            // 后台用:schema + 当前值(机密掩码为 "set"/"")
func Complete(def Definition, fields map[string]string) bool
```

### 4.2 支付（`internal/pay`）改读 `provider.Store`

- `buildDriver(tenantID, platform)` 的 `platform` 语义改为**渠道**：`wechat` / `alipay` / `app`(mock)。交易类型由租户类型推：`tenant.Type == "app"` → APP，否则 JSAPI / 小程序 create。
  - `WxCreds` 从 `provider` 行取；`TradeType` 由租户类型定。
  - `AliCreds` 同上；`sandbox` 字段 → `alipay.Client.Gateway`。
- `resolveChannel`：App 平台 `channel` ∈ {`wechat`, `alipay`}；小程序平台按登录平台映射 `wx→wechat`、`alipay→alipay`。`PayOrder.Platform` 写 `wechat` / `alipay` / `apple` / `google_play` / `app`(mock)。**兼容**：历史订单的 `wx` / `wx_app` / `alipay_app` 值在 `SyncIfStale` 与回调里按别名表折回新渠道名。
- 回调路由 `/pay/callback/:platform`：`wechat` 用 `Wechatpay-Serial` + `ByLookup("pay","wechat","",serial)`（B 列）→ 查不到再用解密后的 `mchid`；`alipay` 用表单 `app_id` → `ByLookup("pay","alipay",appid,"")`。保留 `wx` / `wx_app` / `alipay_app` 三个旧路径作别名（商户平台里已填的 notify_url 不用改）。
- IAP：`appStoreToken` / `VerifyIAP` 改读 `provider(apple)` 的字段；`sysconfig.KeyAppIAP*` 删除。
- **Google Play 新增** `internal/pay/play_verify.go`：Service Account JSON → RS256 JWT → `oauth2.googleapis.com/token` → `GET androidpublisher/v3/applications/{pkg}/purchases/products/{productId}/tokens/{token}`；`purchaseState==0 && consumptionState==0` 入账，`orderId` 幂等（`PlayPurchase` 表），金额按 `CoinPackagePrice(platform=gplay, region)` 校验；`POST /pay/play/verify (auth) {product_id, purchase_token}`。入账仍走 `wallet.CreditTx` + 首充打标（抽成 `creditOrder(tx, ...)` 与 `HandleCallback` 共用）。
- `/app-config` 的 `pay` 段改为 `{wechat, alipay, apple, google_play}` = `Usable(...)`；`auth.wechat/alipay` 不变（登录仍看 `app_credentials`）。

### 4.3 支付宝登录改读支付配置（D3）

`user.alipayClient(tenantID, platform)`：先取 `provider.Get(tid,"pay","alipay")` 的 `app_id/private_key/alipay_public_key`，没有再回落 `app_credentials` 行（迁移期）；`AlipayAuthInfo` 的 PID 取 `fields["pid"]`。

### 4.4 地图（`internal/geo`）

```go
type Geocoder interface{ Regeo(lat, lng, lang string) (Result, error) }   // Result{Address, City, Place}
```
`tencent.go` / `google.go`（现有逻辑搬进来）/ `amap.go` / `baidu.go`。`regeo` handler：`provider.Active(tid,"map")` → 对应 Geocoder；没有生效行 → 空地址（现状降级）。腾讯 / Google 的 key 从 `provider` 行取；`app_maps_provider`、`app_google_map_key`、`geo_qq_key` 删除。`/app-config` 新增 `map.provider`（App 端将来切 SDK 用，这轮只下发）。

### 4.5 内容安全（`internal/moderation`）

```go
type Checker interface {
	CheckText(tenantID, userID int64, scene int, text string) error
	CheckImageAsync(tenantID, userID int64, mediaURL string)
}
```
`wechat_checker.go`（现 `wxcheck.go`）、`alipay_checker.go`（`alipay.security.risk.content.detect`：`biz_content={"content":...}`，结果 `action == "REJECTED"` 视为违规；图片同接口传 `type=image` 的 URL）。`CheckUGC` / `CheckImageAsync` 先 `provider.Active(tid,"moderation")`，再看该行 `text_on` / `image_on`。本地词库 `CheckText` 不变（它不是服务商）。`sec_check_text_on/image_on` 删除。

### 4.6 迁移（启动时一次，幂等）

`internal/provider/migrate.go`，在 `AutoMigrate` 后、`Store.Reload` 前执行，每条只在目标行不存在时写：

| 来源 | 目标 |
|---|---|
| `app_credentials` 的 `wx`/`wx_app` 行且 `mch_id != ""` | `pay/wechat`：app_id、mch_id、apiv3_key、serial_no、private_key、platform_key、platform_serial、notify_url；`enabled=true` |
| `alipay`/`alipay_app` 行且私钥非空 | `pay/alipay`：app_id、private_key、alipay_public_key、pid(=mch_id)、notify_url；`enabled=true` |
| sysconfig `app_iap_*` 非空 | `pay/apple`：bundle_id(=`app_apple_bundle_id`)、issuer_id、key_id、p8_key、sandbox；`enabled = app_iap_enabled` |
| sysconfig `app_pay_wechat_enabled/alipay_enabled` = "0" | 对应 pay 行 `enabled=false` |
| `geo_qq_key` / `app_google_map_key` 非空 | `map/tencent`、`map/google`，`active` 按 `app_maps_provider`（空 → tencent） |
| `sec_check_text_on/image_on` 任一为 "1" | `moderation/wechat` `enabled=active=true`，两个开关进字段 |

迁移完成写 sysconfig 全局键 `provider_migrated=1`（租户 0），不重复跑。旧 sysconfig 键从 `defaults` / `meta.go` 删除；`sysconfig.Get*` 对未知键返回空串，不会 panic。

### 4.7 后台接口（`/admin/api`）

```
GET  /providers/:kind                 → {cards:[{provider,label,platform,enabled,active,complete,doc_url,fields:[{key,label,type,secret,required,value|"set"}]}]}  (租户取 X-Tenant-ID,0 → 400 "请先选择租户")
PUT  /providers/:kind/:provider       {enabled, active, fields:{k:v}}   机密空值 = 保持
POST /providers/:kind/:provider/test  → {ok, message}                   D11
```
新错误文案进 `catalog.go`。

### 4.8 `/config` 瘦身

删除 meta 里 `GroupAppPay`（除 mock 外整组）、`GroupGeo`、`GroupModeration` 的开关两项、`GroupCamera` 的 `geo_qq_key`；`GroupPayMP` 剩 `ios_recharge_off` 一项，并入 `GroupPrice`；`SectionPay` 只剩「价格」「支付联调」。`meta_test.go` 的「每个键都有分组 / 中英标签」规则不变。

---

## 五、后台页面

侧边栏新增分组 `nav.groupProviders`「服务商」：`/providers/pay` 💰 支付、`/providers/map` 🗺️ 地图、`/providers/moderation` 🛡️ 内容安全。三页共用一个 `ProviderPage.vue`（按路由 `kind` 拉 `GET /providers/:kind`）：

- 顶部提示当前租户（名称 + 类型）；未选租户时整页提示。
- 每家服务商一张卡片：标题 + 「申请入口」链接 + 启用开关（单选领域是「设为当前」单选）+ 完整度徽章（已配置 / 缺 N 项）+ 字段表单（机密字段显示「已设置，留空不改」）+ 「保存」「测试连通」。
- 支付宝内容安全卡片：未配支付宝支付时显示「先到支付页配置支付宝应用」并禁用保存。
- 保存成功后刷新卡片；`/config` 页不再出现这些项。
- 文案进 `zh-CN.json` / `en.json`（两文件 key 一致）。

---

## 六、测试策略

- `provider/schema_test.go`：每个定义 Kind/Provider 唯一、LookupA/B 指向存在的字段、Required 字段都有标签。
- `provider/store_test.go`：`Complete`、`Upsert` 合并规则（机密空值保留、active 互斥）的纯函数部分；`ByLookup` 索引。
- `provider/migrate_test.go`：给定旧值表 → 期望行（纯函数 `planMigration(rows, kv) []ProviderConfig`）。
- `pay`：`resolveChannel` 新表；回调别名表；Play 校验的 JWT 签发与响应解析（httptest）；`creditOrder` 幂等结构不变。
- `geo`：四家 Geocoder 的响应解析（httptest 假服务）；`Active` 为空时降级。
- `moderation`：支付宝检测的请求体与 `REJECTED` 判定（httptest）；开关关闭不调用。
- `admin`：`GET /providers/pay` 机密掩码；`PUT` 写空机密不覆盖；`meta_test` 对删键后的白名单继续通过；i18n 覆盖测试。
- `sysconfig/app_config_test.go`：`pay` 段四键来自 `provider.Usable`。
- admin：`npm run build`（locale 校验）。
- 人工：迁移前后 `/app-config` 的 `pay`、后台三页读写、微信回调旧路径仍可达。

---

## 七、遗留

- App / 小程序端地图 SDK 随 `map.provider` 切换。
- Google Play 客户端接入（`in_app_purchase`）。
- 微信内容安全异步回调。
- `app_credentials` 中已迁出的支付列在下一个大版本删除。
