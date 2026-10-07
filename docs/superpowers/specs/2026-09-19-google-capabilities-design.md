# Google 三项能力设计（SSO / Maps / Play Billing）

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-19 |
| 状态 | DESIGN — 已确认，待出实施计划 |
| 来源 | 产品侧提出「Google SSO 登录 / Google Map / Google Pay」三项，要求先出中文原型 |
| 关联 | `2026-09-15-app-v1-scope.md`（V1 模块范围）· `2026-09-14-flutter-app-overseas-design.md`（技术设计）· `docs/prototype/v1-screens.html`（原型，已按本设计更新） |
| 分支 | `feat/app-v1` |

---

## 一、范围

### 1.1 本期做三项

| # | 子项目 | 后端现状 | 体量 |
|---|---|---|---|
| 1 | **Google SSO 端上打通**（含 Apple 登录） | 已实现，差五个缺口 | 小 |
| 2 | **定位与地图**（进 App 取经纬度 + 发布时手选地点） | `geo/regeo` 已实现 | 中 |
| 3 | **Google Play Billing** | 无，但 Apple IAP 可对称照搬 | 中 |

### 1.2 明确不做，且不是简单砍掉

产品侧最初提的第三项是「Google Pay」，确认需求后是 **A 线下活动/门票 + B 实物周边 + C 订阅含实物权益**三者都有。

**结论：Google Pay 本身接入工作量很小，但它背后必须配套的东西构成一个完整的电商/履约子系统，体量比本期三项加起来还大。**

| 需求 | 实际拖进来的 |
|---|---|
| A 线下活动 / 门票 | 活动与场次管理、票种与库存、核销（二维码 + 核销端）、退票规则、实名与年龄限制 |
| B 实物周边 | 商品目录与 SKU、库存扣减、收货地址簿、运费模板、订单状态机、物流单号、售后退换 |
| C 订阅 + 实物权益 | 订阅计划、续订/宽限期/退订、权益发放，实物部分又落回 B 的整条履约链 |
| 三者共用 | 统一订单中心、退款与争议、发票与税、客服工单、对账 |

现状核对：`internal/item` 是虚拟道具与次数包，`model.PayOrder` 只有金币充值一种场景，`model.User` 无地址字段，原型 37 屏无任何商品/购物车/订单/物流/核销屏。**这是全新业务，不是在已有流程上加一个支付方式。**

**合规约束（不可绕过）**：Google Play 要求应用内销售的数字商品（金币、次数包、礼物）**必须走 Google Play Billing**，第三方渠道仅允许用于实物与线下服务。C 的订阅本身属数字商品，**加了实物权益不改变判定** —— 实物权益只能作为独立订单走 Google Pay。

**因此：电商/履约子系统（A+B+C）与其上的 Google Pay driver 单独立项，各自走一轮 brainstorm → spec → plan。本期不做。**

### 1.3 顺带修正的既有问题

原型 H3「充值 · 平台分叉」原本把 Android 侧画成 **UPI（GPay / PhonePe / Paytm）+ Card / Netbanking**。按此实现会被拒审、已上架会被下架。本期改为 Play Billing。

> 用户侧无损失：Play 结算内部照样支持 UPI、卡、话费代扣，印度用户熟悉的付款方式一个不少，换的只是收银台。

---

## 二、决策记录

| # | 决策点 | 选定 | 理由 / 代价 |
|---|---|---|---|
| 1 | Flutter 版本 | **3.47.5** | 硬约束是 `pubspec.yaml` 的 `sdk: ^3.13.0` → Flutter ≥ 3.47.0。取 3.47 线最新 patch，两台开发机版本不同但都满足硬约束 |
| 2 | Google 登录上哪些端 | **Android + iOS**，Web 不做 | Web 无发布计划，且要另配一套 Client ID |
| 3 | iOS 的 Apple 登录 | **本期一起做** | App Store 4.8：iOS 上有 Google 登录却无 Apple 登录 = 直接拒审。非可选项 |
| 4 | release keystore | **本期定下来** | Google 登录 Android Client ID 绑签名 SHA-1，不定则上架后登录必失效 |
| 5 | 已有账号与 Google 账号打通 | **不自动合并，提示用户去绑定** | 自动按邮箱合并等于把「Google 说这邮箱是他的」当所有权证明，任何疏漏即账号接管 |
| 6 | 地图形态 | **A + B 都要** | A = 进 App 即取经纬度（底座）；B = 发瓶/发动态时地图手选地点（叠加层） |
| 7 | 地址搜索框 | **本期不做** | Places Autocomplete 是独立计费 API（按会话计费），拖图选点已够用 |
| 8 | 位置精度与下发 | **存精确值，只下发城市 + 距离** | 写死在 DTO 层，参照 `discover/service.go` 的 `json:"-"` 做法 |
| 9 | 定位权限时机 | **先自解释屏（A7），再弹系统框；且在进入主界面后** | iOS 权限框一辈子只弹一次；注册流程里弹会明显拉低完成率 |
| 10 | iOS 权限文案 | **写 `Info.plist`，不进后台配置** | 随包审核的静态文案，后台改了不生效 |
| 11 | 价格真相来源 | **后台 `CoinPackage` 为准，Play Console 手工对齐** | 配套服务端校验：实付金额与后台档位价不一致则**拒绝入账并告警**，不照发金币 |
| 12 | 市场范围 | **印度先上，留多市场结构** | 价格按 ISO 国家码拆行，`*` 兜底 |
| 13 | 多市场地区维度 | **ISO 国家码** | — |
| 14 | 退款感知 | **只轮询 Voided Purchases，不接 RTDN**（见 §六） | 本期不做订阅，接 Pub/Sub 是为未做的功能付成本 |
| 15 | 订阅 | **本期不做** | 续订/宽限期/退订是另一套状态机 |
| 16 | 服务账号 JSON 存储 | **`textarea` + AES-GCM 加密入库** | 复用 `internal/crypto`，与 `.p8` 同样处理 |
| 17 | 原型新增屏放哪 | **插入各自模块，不新开 §18** | 原型按 17 个模块组织，硬拆一节会让同模块的屏散在两处 |
| 18 | 英文原型 | **中文先行，英文版插占位块** | 4 个受影响章节各插 `⏳ CN-first section` |
| 19 | `PriceFen` 字段名 | **改名 `PriceMinor`** | 语义不再是「分」；跨币种统计出错是静默的、极难发现。代价：改 `overview.go` / `admin/handler.go` 及后台前端字段名 |
| 20 | `google_sub` 唯一性 | **`*string` + `uniqueIndex`，存量 `''` 刷 `NULL`** | 唯一一个把约束放在数据库里的方案；账号唯一性是应用层保证不住的东西 |
| 21 | feed 缓存位置粒度 | **geohash 4 位（约 40km）** | 跨城才失效，市内移动不重建 |

---

## 三、数据模型改动

全部靠 GORM AutoMigrate 加列；两处改既有字段、两张新表、一次性数据迁移。

### 3.1 `User` — 改 2 列、加 1 列

```go
// 🔧 改：index → uniqueIndex，且改为可空
//    现状 model.go:76-77 是 gorm:"size:64;index"，数据库层面不阻止两个账号绑同一个
//    Google 身份。A6b「该账号已被占用」若只做在应用层，绑定按钮连点两次即可击穿。
//    不能直接加唯一索引：手机号用户的 google_sub 是 ''，MySQL 唯一索引不允许重复 ''
//    （NULL 才允许）。故改为可空，未绑定存 NULL。
GoogleSub *string `gorm:"size:64;uniqueIndex:uk_tenant_google,priority:2" json:"-"`
AppleSub  *string `gorm:"size:64;uniqueIndex:uk_tenant_apple,priority:2" json:"-"`
// 两个唯一索引的 priority:1 都是 tenant_id

// 🆕
EmailVerified bool `json:"-"` // Google ID Token 的 email_verified 落库
```

**一次性迁移**：`UPDATE users SET google_sub = NULL WHERE google_sub = ''`（apple_sub 同）。**必须在加唯一索引之前执行**，否则建索引失败。

> **既有问题，本期不修**：`Phone`（`model.go:70`）与 `Email`（`:71`）同为 `index` 而非 `uniqueIndex`，存在并发注册出两个同手机号账号的可能。本次查出，修它属另一件事。

### 3.2 `Bottle` — 加 3 列

```go
Lat       float64 `gorm:"index:idx_bottle_geo,priority:1" json:"-"`
Lng       float64 `gorm:"index:idx_bottle_geo,priority:2" json:"-"`
PlaceName string  `gorm:"size:64" json:"place_name"`
```

**存量数据是这里唯一的风险**：加列后老瓶子经纬度为 0（落在几内亚湾）。所有距离计算必须先判 `Lat != 0 || Lng != 0`，否则老瓶子距离排序全乱。回退口径是现有的 `City` 字段。

### 3.3 `Moment` — 加 4 列（目前一个位置字段都没有）

```go
City      string  `gorm:"size:32" json:"city"`
Lat       float64 `gorm:"index:idx_moment_geo,priority:1" json:"-"`
Lng       float64 `gorm:"index:idx_moment_geo,priority:2" json:"-"`
PlaceName string  `gorm:"size:64" json:"place_name"`
```

### 3.4 支付 — 2 处改既有 + 2 张新表

```go
// PayOrder：两处必须改
Platform  string `gorm:"size:16"` // 🔧 从 size:8 加宽。塞 google_play（11 字符）会被静默截断
Currency  string `gorm:"size:8"`  // 🆕 INR/USD…，不加则 overview.go 跨币种统计全是假数据
PriceMinor int64                  // 🔧 原 PriceFen。语义改为「最小货币单位」，不再隐含人民币

// CoinPackage 加一列，与已有的 ios_product_id 对称
PlayProductID string `gorm:"size:64;index"`
```

> **设计修正**：初稿曾把 `PlayProductID` / `IAPProductID` 放在价格表上。查 `iap.go` 后发现 `CoinPackage` **已有 `ios_product_id`**，且 Play 商品 ID 是**全局唯一、多国共用一个**的 —— 随国家变的只是价格。**商品 ID 属于档位，价格才属于地区。**

```go
// 🆕 多市场定价
// iap.go 原注释：「iOS 档位单独配（30% 抽成要在定价里吃掉，两端价格本就不同）」
// → 故必须有 platform 维度，不只是 region
type CoinPackagePrice struct {
    ID        int64
    PackageID int64  `gorm:"uniqueIndex:uk_pkg,priority:1"`
    Platform  string `gorm:"size:8;uniqueIndex:uk_pkg,priority:2"` // gplay / ios
    Region    string `gorm:"size:8;uniqueIndex:uk_pkg,priority:3"` // ISO 国家码，* 兜底
    Currency  string `gorm:"size:8"`
    Amount    int64  // 最小货币单位
}

// 🆕 Play 购买记录，对称 model.IAPTransaction
type PlayPurchase struct {
    ID            int64
    TenantID      int64  `gorm:"index"`
    UserID        int64  `gorm:"index"`
    OrderID       string `gorm:"size:64;uniqueIndex"` // GPA.xxxx，幂等键
    PurchaseToken string `gorm:"size:512"`
    ProductID     string `gorm:"size:64"`
    OrderNo       string `gorm:"size:32;index"`       // 关联 PayOrder
    State         string `gorm:"size:16"`             // pending/purchased/refunded
    Coins         int64
    Currency      string `gorm:"size:8"`
    AckedAt       *time.Time
    RefundedAt    *time.Time
    CreatedAt     time.Time
}
```

`CoinPackage` 原有的单一价格字段**保留但降级为兜底**，等 `CoinPackagePrice` 有 `*` 行后可废弃 —— 不直接删是给存量数据和后台页面一个过渡期。

---

## 四、Google SSO

### 4.1 已实现，零改动

`POST /api/auth/google` 与 `/api/auth/apple`（`user/handler.go:30`）：`appauth.go` 已做 JWKS 验签、issuer 校验（Google 两种写法都收）、aud 校验、banned/deleted 状态拦截，`jwksCache` 带缓存。

### 4.2 五个缺口

**缺口 1 — claims 未取 `email_verified`**

`idTokenClaims`（`appauth.go:129`）只声明了 `Email` 与 `Name`：

```go
type idTokenClaims struct {
    jwt.RegisteredClaims
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"` // 🆕
    Name          string `json:"name"`
}
```

**缺口 2 — `LoginWithGoogle` 把 email 丢了**

现为 `loginOrCreateApp(tenantID, appIdentity{GoogleSub: claims.Subject})`，`Email` 空着。后果：**Google 注册的账号没有邮箱**，既做不了冲突判断，用户日后也找不回账号。改为同时带 `Email`；`email_verified == false` 时按无邮箱处理。

**缺口 3 — 无冲突分支**

`loginOrCreateApp`（`user/otp.go`）是「查不到就建」两段式。需在中间插入：`google_sub` 未命中 → 若 `email_verified` 且该邮箱已有账号 → **不建号**，返回专用错误码交由原型 A2k 呈现。复用现成的 `identityExists` helper。

**缺口 4 — 无绑定/解绑接口**

```go
api.POST("/auth/google/bind", auth, h.bindGoogle)
api.POST("/auth/apple/bind",  auth, h.bindApple)
api.POST("/auth/unbind",      auth, h.unbind)   // body: {provider: "google"|"apple"}
```

绑定与登录流程**完全一样**（拿 ID Token → 验签），只有最后一步不同：登录是「按 sub 查用户」，绑定是「把 sub 写到当前登录用户」。抽出共用：

```go
func (s *Service) verifyGoogleIdentity(appid, idToken string) (sub, email string, verified bool, err error)
```

**解绑的前置校验**：解绑后必须仍有可登录方式，否则用户会把自己锁在门外。

**缺口 5 — 唯一索引**：见 §3.1。

### 4.3 客户端（Flutter 侧零基础）

```yaml
google_sign_in: ^7.x      # 7.x 在 Android 上已迁到 Credential Manager
sign_in_with_apple: ^7.x  # 仅 iOS 走原生；Android 会退化成网页流程，不启用
```

- `AuthRepository` 增加 `loginWithGoogle()` / `loginWithApple()` / `bindGoogle()` / `unbind()`
- Apple 按钮**仅 iOS 显示**，用 `Platform.isIOS` 运行期判断，不要用编译期常量（Web 端会挂）

### 4.4 凭证形态（配置前必读）

**OAuth Client ID ≠ API Key。** 登录要的是 Client ID，形如 `123456789-abcdef.apps.googleusercontent.com`；`AIzaSy…` 开头的是 Maps 系列的 API Key，**不能用于 SSO**。

```
Web client ID     ← 服务端校验 ID Token 的 aud，填后台 app_google_client_id
Android client ID ← 绑 包名 com.ambertu.bottles + 签名 SHA-1，不填后台
iOS client ID     ← 绑 Bundle ID，写进 iOS 工程，不填后台
```

**最常配错的一处**：Android / iOS 的 Client ID 只让设备端能发起授权；服务端校验用的 `aud` 始终是 **Web client ID**。

**release 签名的坑**：`build.gradle.kts` 中 release 仍指向 debug 签名（TODO）。Google 登录绑签名证书 SHA-1，**debug 包能登录不代表 release 包能登录**；若启用 Play App Signing，Google 会用自己的证书重签，SHA-1 再变一次 —— release 的 SHA-1 必须从 **Play Console「应用完整性」页**取，不是从本地 keystore 取。

---

## 五、定位与地图

### 5.1 已实现

`geo/regeo`（`geo/geo.go`）按 `app_maps_provider` 分派，Google 分支已解析 `address_components` 取城市，并为印度做了兜底：

```go
if t == "locality" { city = comp.LongName }
else if city == "" && t == "administrative_area_level_2" { city = comp.LongName }
```

返回 `{"address", "city"}`。`User.Lat/Lng` 已存在（`idx_user_geo` 复合索引），`POST /api/user/update` 已能上报（`user/service.go:188`，要求成对提交）。

### 5.2 五个缺口

**缺口 1 — 缺短地名**：返回的是 `formatted_address`（完整地址串）。M1 底部地址卡要短标签「Bandra West」，需再取一层 `sublocality` / `neighborhood` 作为 `PlaceName`。

**缺口 2 — 两分支返回结构不一致**：腾讯分支只返回 `{"address"}`，Google 分支返回 `{"address","city"}`。统一为 `{address, city, place}` 三字段，腾讯分支缺的补空串。

**缺口 3 — Flutter 端从零开始**

```yaml
geolocator: ^13.x
google_maps_flutter: ^2.x
```

**Key 的澄清**：`/geo/regeo` 那把 Key 在服务端、不下发；但**地图渲染的两把 Key 必须打进 App 包**（Android 进 manifest、iOS 进 AppDelegate），Maps SDK 的硬性要求。它们的防护不是保密，而是**包名 + SHA-1 / Bundle ID 限制**。故必须拆三把且各自加限制：

```
Android Key  限制 Android 应用（包名 + SHA-1），启用 Maps SDK for Android
iOS Key      限制 iOS 应用（Bundle ID），启用 Maps SDK for iOS
服务端 Key    限制 IP 白名单，启用 Geocoding API  ← 这把填后台 app_google_map_key
```

Maps Key 裸奔会被刷爆账单，是常见事故。

**缺口 4 — feed 缓存没有位置维度**

```go
func (ft Filter) sig() string { return fmt.Sprintf("%s-%d", ft.Scope, ft.Gender) }
// key = user_feed:{tenant}:{user}:{sig}（夜场再拼 "-n"）
```

缓存键按用户隔离，但 `sig` 无位置维度：用户从孟买飞到德里，捞到的仍是按孟买位置打分的那批瓶子，且该队列不会自行过期。

> **但这个问题比看起来轻**：队列存的是瓶子 ID，「2.4 km」是响应时用当前用户位置与瓶子经纬度**现算**的。陈旧的只是**池子构成**，不是显示的距离。故粒度可以取粗。

**方案**：`sig` 拼入 **geohash 4 位**（约 40km × 20km）。跨城市才失效，市内移动不重建，缓存命中率几乎不受影响。

**缺口 5 — 打分缺距离维度**

现有打分（`bottle/feed.go:146`）：`wTag·标签 + wCity·同城 + wGender·异性 + wFresh·新鲜 + wHeat·热度 + wRobot + wRandom`，权重全部后台可调。

新增 `wDist · 距离衰减` 与 sysconfig key `match_w_dist`（**必须同步写 defaults**）。两个边界：

- **瓶子无经纬度**（存量数据）→ 距离项**不计分也不惩罚**，退回 `wCity` 维度，其余权重归一化重算
- **用户无经纬度**（拒绝定位）→ 整个距离维度置 0，与 Z6 已有处理一致

### 5.3 下发口径（写死在 DTO 层）

```go
Lat float64 `json:"-"`
Lng float64 `json:"-"`
// 响应只出现：
DistanceKM *float64 `json:"distance_km,omitempty"` // 先例：discover/service.go:84
PlaceName  string   `json:"place_name"`
City       string   `json:"city"`
```

**距离做区间化**（<1km / 2.4km / >50km）。精确到小数点后一位的连续距离，配合多次采样可三角定位。

**地图上永远只显示用户自己选的那一个点，绝不标注其他用户。** 即使做模糊化，多点采样仍可反推真实住址 —— 这是 Tinder / Grindr 都真实出过事的攻击面。

---

## 六、Google Play Billing

### 6.1 `VerifyIAP` 的七步照搬六步

| 步骤 | Apple 侧（`pay/iap.go`） | Play 侧 |
|---|---|---|
| ① 总开关 | `app_iap_enabled` | 🆕 `app_play_enabled` |
| ② 配置校验 | Bundle ID | 🆕 服务账号 JSON |
| ③ 解凭证 | 解 JWS payload | **结构性不同**，见下 |
| ④ 向平台核实 | `verifyWithApple` | 🆕 `purchases.products.get` |
| ⑤ 归属 / 环境校验 | bundleID、sandbox、revocation | packageName、`purchaseType`、`purchaseState` |
| ⑥ 商品 → 档位 | `ios_product_id` | `play_product_id` |
| ⑦ 事务幂等入账 | `transaction_id` 唯一 | `orderId` 唯一 |

**③ 是唯一结构性不同的一步。** Apple 的 JWS 自带交易信息（可先解出再核实），Play 的 `purchaseToken` 是不透明串，**不查 Google 就什么都不知道**。因此 Play 侧**没有**「拿不到 API 凭证就退而信客户端」的兜底分支 —— 未配服务账号只能直接拒绝，不存在沙盒放行的中间态。

**测试单识别**：Play 的测试购买 `orderId` 与正式单**长得一模一样**，区分靠 `purchases.products.get` 返回的 `purchaseType`（`0` = 测试）。与 Apple 靠 `Environment=Sandbox` 区分不同，容易漏。

### 6.2 新增路由

```go
api.POST("/pay/play/verify", auth, h.playVerify) // 客户端提交 purchaseToken
```

### 6.3 `consume` 顺序（最贵的一处）

客户端必须 `verify` 成功后才 `consumeAsync`。提前确认 = Google 认为已交付，钱扣了、币没到，且该交易**再也查不回来**。与 `pay/iap.go:175` 所写「必须等服务端返回成功才 `finish`」同类。

**Play 规定购买后 3 天内未 acknowledge 会自动退款。** 故 `playVerify` 成功入账后须立刻 acknowledge，失败必须有重试，不能挂着。

### 6.4 `PENDING` 是合法中间态，不是掉单

印度的 UPI 延迟付款、话费代扣、现金支付会返回 `PENDING`，可能等数小时至数天。

- 文案必须与真掉单分开：PENDING 说「等待付款确认，完成后自动到账」，**不要说「正在核对」**
- **`PENDING` 期间绝不入账金币、不 acknowledge**，等状态转 `PURCHASED` 才发币

### 6.5 退款感知：轮询 Voided Purchases

**RTDN 主要推订阅事件，一次性商品（金币包）的退款不在标准 RTDN 里。**

**方案**：定时轮询 `purchases.voidedpurchases.list`（支持按时间增量拉），每小时一次，挂在 `push/scheduler.go` 已有的定时任务骨架上。**本期不接 Pub/Sub / RTDN** —— 本期不做订阅，接 RTDN 是为未做的功能付成本。将来上订阅时再补。

> 初稿曾把这条写成「RTDN」，是想当然。真正解决一次性商品退款的是 Voided Purchases API。

**退款处置**（沿用 `revokeIAP` 已确立的原则）：允许扣成负数 —— 币可能已花掉，装作没发生只会让账永远对不平。对应原型 H11：

- 余额为负时**冻结消费类功能**（扔瓶 / 解锁 / 送礼），**充值仍可用**，补足后自动恢复
- **不自动封号**：多数退款是正常行为，封号会误伤。反复退款交由 §15 反作弊
- 「这不是我操作的」入口**转人工，不自动恢复金币** —— 自动恢复等于「退款还留着货」

### 6.6 价格校验

价格以**后台 `CoinPackage` / `CoinPackagePrice` 为准**，Play Console 手工对齐。服务端入账时比对 Play 回传的实付金额与后台档位价，**不一致则拒绝入账并告警**，不照发金币。

---

## 七、配置项

全部新 key **必须同步写 `internal/sysconfig/sysconfig.go` 的 defaults**（硬约束 1：空串会导致开关逻辑反转），并登记 `internal/admin/meta.go` 白名单（硬约束 2）。

### 7.1 已存在，无需新增

| key | 分组 | 说明 |
|---|---|---|
| `app_google_client_id` | App 登录 | 填 **Web** client ID |
| `app_apple_bundle_id` | App 登录 | — |
| `app_maps_provider` | App 地理 | 填 `google` |
| `app_google_map_key` | App 地理 | **服务端那把**（IP 白名单 + Geocoding） |
| `app_discover_max_km` | App 发现 | — |

### 7.2 新增

| key | 分组 | 类型 | 默认 | 说明 |
|---|---|---|---|---|
| `app_play_enabled` | App 支付 | bool | `0` | Play Billing 总开关 |
| `app_play_package_name` | App 支付 | text | 空 | `com.ambertu.bottles` |
| `app_play_sa_json` | App 支付 | textarea | 空 | 服务账号 JSON 全文，AES-GCM 加密 |
| `app_play_allow_test` | App 支付 | bool | `0` | 允许测试单入账，对应 `app_iap_sandbox` |
| `match_w_dist` | 匹配权重 | int | `15` | 捞瓶距离衰减权重，见下 |

**`match_w_dist` 默认值的取法**：现有权重是 `tag:40 / city:25 / gender:15 / fresh:10 / heat:10 / robot:5 / random:15`。距离与「同城」在语义上重叠（同城≈距离近），直接加一个高权重会让地理维度在总分里翻倍。故：

- `match_w_dist` 默认 **15**，与 `gender` 同级，低于 `city`
- **`match_w_city` 不动**（保持 25）。同城是离散的行政区划信号，距离是连续的物理信号，两者不等价 —— 跨省相邻市可能只有 30km，同市两端可能 60km
- 上线后按实际 CTR 调；两个 key 都在后台「匹配权重」分组可调，无需发版

### 7.3 服务账号配置（两步容易漏）

整个 JSON 贴进后台，不是只填 `client_email`。另外：

1. **Play Console → 用户和权限**里把该服务账号加进去，授予「查看财务数据」与「管理订单」权限 —— 光有 JSON 不授权，API 返回 401
2. 本期不接 RTDN，故**不需要**建 Pub/Sub 主题

### 7.4 后台页面改动

- 「充值档位」页：新增 `play_product_id` 输入框；价格从单值改为**按 platform × region 多行**
- 新增「账号与安全」相关：无后台配置项，纯客户端

---

## 八、原型

已按本设计更新 `docs/prototype/v1-screens.html`（45 → 55 屏），英文版 `v1-screens-en.html` 4 个受影响章节插入 `⏳ CN-first section` 占位块。

### 8.1 新增 10 屏

| ID | 屏 | 要点 |
|---|---|---|
| A2g | Google 登录中 | `aud` 用 Web client ID |
| A2k | 邮箱已注册 · 冲突 | 决策 5 的落点 |
| A2p | Apple 登录中 · 仅 iOS | 昵称邮箱只首次回传；中继邮箱不是真实邮箱 |
| A7 | 定位权限说明 | iOS 权限框只弹一次 |
| A6 | 账号与安全 | 绑定/解绑；至少保留一种登录方式 |
| A6b | 绑定结果 · 被占用 | `google_sub` 唯一约束的用户可见面 |
| M1 | 地图选点 | 只显示自己；大头针固定屏幕中心 |
| M2 | 地点已选 / 不显示 | 「不显示地点」是一等选项 |
| F2 | 发动态 | **原型既有缺口**，非本次需求引入 |
| H11 | 退款已撤销 | 负余额的用户可见态 |

### 8.2 修改 7 屏

`A2`（补第三方登录区）· `A4`（定位时机）· `Z6`（补手选地址降级路径 + 无经纬度时不显示距离）· `B2`（地点行 + 存量经纬度回退）· **`H3`（UPI → Play Billing，本期最重要的一处修正）** · `H7a`（删「我已完成支付」；补 consume 顺序与 3 天自动退款）· `H9`（补 `PENDING` 第四态）

### 8.3 图标体系

原型 §0 原已声明「Tab 图标不用 emoji」，但只落实到 Tab 栏。本次**将该政策扩展到手机框内所有图标**，并补充一条原文未提的理由：**emoji 在暗色模式下无法反相**（本原型有完整暗色配色）。

sprite 由 5 个 symbol 扩到 22 个，新增 16 个沿用同一套 24 栅格 + `stroke-width:1.7`。本期新增/修改的屏内 emoji 已清零（仅保留 `✕` `♡` `✓` 等**排版符号**，非 emoji）。

**遗留**：早期屏仍有约 200 处 emoji（头像占位、礼物、相机等）未收敛，已在 §0 标注 `⏳ 尚未收敛`，并写明**真机实现一律以 sprite 为准，不要照抄 emoji**。全量收敛涉及 45 屏，建议单独排期。

---

## 九、风险与遗留

| 风险 | 影响 | 处置 |
|---|---|---|
| **存量瓶子经纬度为 0** | 距离排序全乱（老瓶子被算作在几内亚湾） | 所有距离计算先判空，回退 `City` 维度。**实施时必须有针对该分支的测试** |
| **`google_sub` 唯一索引迁移** | 迁移前有重复 `''` 会导致建索引失败 | 先跑 `UPDATE … SET google_sub = NULL WHERE google_sub = ''` 再建索引；上线前在生产快照上演练一次 |
| **`PriceFen` → `PriceMinor` 改名** | 后台前端字段名同步变更，漏改则统计页空白 | 前后端同版本发布；`grep -rn PriceFen` 清点全部引用点 |
| **release SHA-1** | 不定则上架后 Google 登录必然失效 | 从 Play Console「应用完整性」页取，不是本地 keystore |
| **iOS 无法在本机验证** | 本机（Windows）`flutter build ipa` 子命令都不存在 | iOS 部分只能写代码与原型，需 macOS + Xcode 才能真跑。见 `docs/APP_ENV_SETUP.md §5` |
| **Maps Key 未加限制** | 泄露后账单被刷爆 | 三把 Key 各自加限制后才可上线 |
| **多市场定价人工对齐** | 后台价与 Play 实扣不一致 | 服务端校验不一致即**拒绝入账并告警**（决策 11 的配套） |

### 遗留，不在本期

1. **电商 / 履约子系统（A+B+C）+ Google Pay driver** —— 见 §1.2，另立项目
2. **`Phone` / `Email` 唯一索引** —— 既有问题，本次查出未修
3. **原型早期屏 emoji 全量收敛** —— 涉及 45 屏，单独排期
4. **Places Autocomplete 地址搜索** —— 决策 7，下一期
5. **订阅** —— 决策 15；上订阅时需补 RTDN / Pub/Sub
