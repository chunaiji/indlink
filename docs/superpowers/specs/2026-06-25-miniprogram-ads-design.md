# 多租户配置改造 + 小程序流量主广告接入设计

- 日期：2026-06-25
- 范围：server（配置层重构 + 广告下发 + 激励发币）、admin（配置按租户读写）、client（uni-app 广告渲染）
- 状态：待审核
- 说明：本 spec 合并两部分。**Part 1（多租户配置改造）是地基，必须先落地**；Part 2（流量主广告）建立在其上。

---

# Part 1：多租户配置改造

## 问题

- `model.Config` 主键仅 `key`，**无 tenant_id**，全局一份。
- 后台「系统配置」`getConfig`/`putConfig` 走 `sysconfig.Get/Set(key)`，**忽略租户**——所有租户共用同一份。
- admin 前端其实已全局带租户：`api.js` 每个请求自动注入 `X-Tenant-ID` header，后端 `tenantFromCtx(c)` 可读取，但 config 端点没用它。
- `sysconfig.Get*` 全项目 71 处调用，签名均不带租户。

## 目标

所有配置参数按租户隔离，且有「租户值 → 全局默认 → 代码默认」三级回退；后台按当前选中租户读写。

## 数据模型

`model.Config` 改为复合主键 `(tenant_id, key)`，`tenant_id=0` 行为全局默认：

```go
type Config struct {
	TenantID  int64     `gorm:"primaryKey" json:"tenant_id,string"` // 0 = 全局默认
	Key       string    `gorm:"primaryKey;size:48" json:"key"`
	Value     string    `gorm:"size:255" json:"value"`
	Remark    string    `gorm:"size:128" json:"remark"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

### 迁移安全（bootstrap，AutoMigrate 前执行）
GORM AutoMigrate 不会改既有表主键，需手工迁移，幂等：

```go
func migrateConfigTenant(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Config{}) {
		return nil // 全新库：AutoMigrate 按新结构建复合主键
	}
	if db.Migrator().HasColumn(&model.Config{}, "tenant_id") {
		return nil // 已迁移
	}
	if err := db.Exec("ALTER TABLE configs ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 0").Error; err != nil {
		return err
	}
	// 既有行 tenant_id 默认 0 → 自动成为全局默认；主键由 (key) 改为 (tenant_id, key)
	return db.Exec("ALTER TABLE configs DROP PRIMARY KEY, ADD PRIMARY KEY (tenant_id, `key`)").Error
}
```
在 `bootstrap.Migrate` 里 `migrateConfigTenant(db)` → 再 `AutoMigrate`。（表名 `configs`，列 `key` 为保留字需反引号。）

## sysconfig 重构

- 缓存改为 `map[int64]map[string]string`（tenantID → key → value）。
- `reload()` 全量加载所有租户行入缓存。
- 解析 `get(tenantID, key)`：
  1. 租户专属值（存在且非空）
  2. 全局值 `tenant_id=0`（存在且非空）
  3. 代码默认 `defaults[key]`
- 公开 API 全部加 tenantID 首参：
  - `GetString(tenantID int64, key string) string`
  - `GetInt(tenantID, key) int` / `GetInt64(...)` / `GetBool(...) bool`
  - `Set(tenantID int64, key, value string) error`（写对应租户行 + 刷新该租户缓存）
- `seedDefaults()` 把 `defaults` 写入 **tenant_id=0**（全局默认）尚未存在的键。

## 调用点改造（71 处）

逐处把 `sysconfig.GetX(key)` 改为 `sysconfig.GetX(tenantID, key)`。绝大多数调用都在已持有 tenantID 的 service 方法内（`StartChat(tenantID,…)`、`robot.Tick(tenantID,…)`、`share.Reward(tenantID,…)`、`user.Login` 解析出 tenantID 等）。

- 请求路径：用该方法已有的 tenantID。
- 后台任务：
  - 机器人 `Tick(tenantID,…)` 已带租户。
  - 推送 cron 等跨租户任务：按数据行所属 tenant 解析；单租户模式用 `cfg.DefaultTenantID`。
- 启动期读取（如 `InitWorkerPool` 读并发数）：用 `cfg.DefaultTenantID`（并发等运行参数可仅全局，回退链保证可用）。

## admin 配置按租户

- `getConfig(c)`：取 `tid := tenantFromCtx(c)`；每个白名单字段 `f.Value = sysconfig.GetString(tid, f.Key)`（已含回退）。额外返回 `inherited bool`（该值是否来自全局/默认，便于前端提示「继承全局」）。
- `putConfig(c)`：`SetConfig(tenantFromCtx(c), key, value)` 写当前租户行。
- `tid==0`（下拉选「全局/默认」或未选）：读写 tenant 0，即全局默认。
- admin 前端：已自动带 `X-Tenant-ID`，无需改请求；`Config.vue` 可选加「继承全局」灰标（小改，非必须）。

## 登录期租户解析：默认 appid 兜底（线上健壮性）

配置已按租户隔离，前提是每个请求能解析到正确租户。租户在**登录时**由 appid 解析并写入 JWT，后续请求都带 tenantID。问题：多租户模式下 `resolveLogin` 用 `creds.ByAppID(platform, appid)` 查租户，**appid 为空（客户端没传）会登录失败**，线上不可接受。

方案：appid 为空时回退到默认 appid `wxe48b23d248677356`（主小程序，对应凭证表里的某租户）。

- 新增配置：`config.Config.DefaultWxAppID`，env `DEFAULT_WX_APPID`，**默认值 `wxe48b23d248677356`**（可被 env 覆盖）。
- `resolveLogin(platform, appid)`：多租户分支开头，若 `platform=="wx" && appid==""` 则 `appid = cfg.DefaultWxAppID`，再走 `creds.ByAppID`。
- 仍未命中（默认 appid 也没配凭证）→ 维持原报错；空 appid 命中默认 → 正常解析该租户。
- 单租户模式不受影响（本就用 `DefaultTenantID`）。
- **前置数据**：凭证表 `app_credentials` 必须有 `wxe48b23d248677356` 对应的行（否则兜底也查不到）。部署前确认该凭证已配置。
- 仅登录一处改动；tenantID 入 JWT 后全链路一致。

## Part 1 验证
- 既有库迁移后既有配置行变为 tenant 0 全局默认，行为不变。
- 切换 admin 租户下拉，分别读写互不影响；未设租户值时回退全局默认。
- 空 appid 登录回退默认 `wxe48b23d248677356` 并解析到其租户；非空未知 appid 仍报错。
- `go build ./... && go vet ./...` 通过；71 处调用全部编译通过。

---

# Part 2：流量主广告接入

## 目标

接入微信流量主，4 种广告：Banner / 插屏 Interstitial / 激励视频 Rewarded / 格子 Grid。后台**按租户**配置每个广告位「开关 + 广告ID」；开启且 ID 非空才展示。激励视频看完发金币（数量/每日上限后台可配）。

> 所有广告配置都走 Part 1 的多租户配置层，天然按租户隔离。

## 广告位（页面 ↔ 类型 ↔ 位置）

每个 slot = 「开关 + 广告ID」两键。

| slot key | 页面 | 类型 | 位置 / 触发 |
|---|---|---|---|
| `banner_ocean` | 海洋首页 | Banner | tabbar 上方 |
| `banner_city` | 同城 | Banner | 列表底部 |
| `banner_expand` | 扩列 | Banner | 列表底部 |
| `banner_message` | 消息 | Banner | 列表底部 |
| `banner_mine` | 我的 | Banner | 宫格下方 |
| `banner_detail` | 瓶子详情 | Banner | 回应列表上方 |
| `banner_chat` | 聊天 | Banner | 顶部（输入区之外，默认关，运营按需开） |
| `banner_collection` | 我的收藏 | Banner | 列表底部 |
| `banner_orders` | 我的订单 | Banner | 列表底部 |
| `banner_walletlog` | 金币流水 | Banner | 列表底部 |
| `banner_viewed` | 浏览记录 | Banner | 列表底部 |
| `inter_scoop` | 海洋·捞瓶 | 插屏 | 捞瓶结果弹框关闭后（全局频控） |
| `inter_detail` | 瓶子详情 | 插屏 | 进入详情页（全局频控） |
| `reward_coin` | 我的 | 激励视频 | 「看视频领金币」按钮，看完发 N 币（每日上限） |
| `grid_mine` | 我的 | 格子 | 页面底部 |

> 聊天页广告默认关闭，且仅放在输入/消息区之外的顶部条，避免打扰；运营可按租户单独开。

## 配置项（sysconfig + configMeta，分组「流量主广告」）

### 全局（按租户）
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `ad_enabled` | bool | `0` | 流量主总开关，关闭则全站不展示 |
| `ad_inter_gap_sec` | int | `180` | 插屏全局最小间隔秒（客户端按本地时间戳频控） |

### 每个 slot 两键
`ad_<slot>_on`(bool, 默认`0`) + `ad_<slot>_unit`(text, 默认`""`)，slot 取上表 15 个 key。

### 激励视频奖励
| key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `ad_reward_coin_coins` | int | `5` | 看完一条发放金币 N |
| `ad_reward_coin_daily` | int | `5` | 每日最多领取次数 |

**展示规则**：`ad_enabled` ∧ `ad_<slot>_on` ∧ `ad_<slot>_unit` 非空（均按当前租户解析）。

## 配置下发：GET /api/ads（需登录，取 JWT 内 tenantID）

```json
{
  "enabled": true,
  "inter_gap_sec": 180,
  "slots": {
    "banner_ocean": { "on": true, "unit": "adunit-xxx" },
    "...":          { "on": false, "unit": "" },
    "reward_coin":  { "on": true, "unit": "adunit-yyy", "coins": 5, "daily": 5 }
  }
}
```
handler 用 `middleware.TenantID(c)` 解析每个 key。

## 激励发币：POST /api/ad/reward（需登录）

客户端激励视频 `onClose({isEnded:true})` 后调用：
- 校验 `ad_enabled` ∧ `ad_reward_coin_on` ∧ unit 非空。
- Redis 日限 `adreward:{tenant}:{user}:{date}`，超 `ad_reward_coin_daily` → `rewarded=false`（不报错）。
- 通过 → `wallet.Credit(tenant,user,coins,SceneAdReward,bizNo)`，`bizNo="adreward:{user}:{date}:{n}"`。
- 新增 `wallet.SceneAdReward = "ad_reward"`（≤24）。
- 返回 `{ rewarded, coins, balance, count_today, limit }`。

> 已知限制：小程序激励视频默认无服务端回调，本期「信任 isEnded + 每日上限」兜量（同 share）。后续可接流量主服务端回调二次校验，本期不做。

## 客户端实现（uni-app / mp-weixin）

- `api/index.js`：`adApi.config()` → `/api/ads`；`adApi.reward()` → `/api/ad/reward`。
- `store/ads.js`（新）：`fetchConfig()` 拉取缓存；`canShow(slotKey)` = `enabled && slot.on && slot.unit`；`slot(key)` 取配置。App 启动/各页 onShow 懒加载一次。
- `components/ad-slot/ad-slot.vue`（新，banner/grid 声明式）：
  ```html
  <ad v-if="show" :unit-id="unit" :ad-type="type" ad-theme="white" @error="onErr" />
  ```
  props `slotKey`、`type`(`banner`/`grid`)；读 `store/ads`；`@error` 本地隐藏。
- `utils/ad.js`（新，命令式）：
  - `showInterstitial(slotKey)`：读配置 + 本地时间戳频控（`< inter_gap_sec` 跳过）→ `wx.createInterstitialAd().show()`，失败静默。
  - `showRewarded(slotKey)`：`wx.createRewardedVideoAd()` 单例，`onClose(isEnded)` → `adApi.reward()` → toast + 刷新余额。
- 各页接入：
  - banner：`ocean/city/expand/message/mine/detail/chat/collection/orders/wallet-log/viewed` 放 `<ad-slot>`。
  - 插屏：`ocean` 捞瓶 `closePopup` 后 `showInterstitial('inter_scoop')`；`detail` onLoad `showInterstitial('inter_detail')`。
  - 激励：`mine` 放「看视频领金币」按钮（`canShow('reward_coin')` 才显示）。
- 降级：任一广告 SDK 失败静默；`ad_enabled` 关或 unit 空则不渲染/直接 return。

## 文件结构

**server**
- `internal/model/model.go`：`Config` 复合主键。
- `internal/bootstrap/migrate.go`：`migrateConfigTenant`。
- `internal/config/config.go`：新增 `DefaultWxAppID`（env `DEFAULT_WX_APPID`，默认 `wxe48b23d248677356`）。
- `internal/user/service.go`：`resolveLogin` 空 wx appid 兜底默认 appid。
- `internal/sysconfig/sysconfig.go`：缓存/解析/API 带 tenantID；新增全部 ad key + 默认值。
- `internal/sysconfig/handler.go`：`getAds`（`GET /api/ads`）；既有端点改按 tenant 解析。
- `internal/admin/handler.go` `service.go`：`getConfig`/`putConfig`/`Config()`/`SetConfig` 带 tenant。
- `internal/admin/meta.go`：新增「流量主广告」分组全部 key。
- `internal/wallet/service.go`：`SceneAdReward`。
- `internal/ad/service.go` `handler.go`（新）：激励发币。
- `cmd/api/main.go`：注册 ad 路由；调用点改造涉及的接线。
- 全项目 71 处 `sysconfig.Get*` 调用点改造。

**admin**
- 配置项进 `configMeta`，`Config.vue` 自动渲染；可选加「继承全局」灰标。

**client**
- `api/index.js`、`store/ads.js`、`components/ad-slot/ad-slot.vue`、`utils/ad.js`；
- 接入页：`ocean/city/expand/message/mine/detail/chat/collection/orders/wallet-log/viewed`。

## 数据流小结
```
配置:   admin(选租户) → X-Tenant-ID → put/get Config(tenant) → configs(tenant,key) 行
解析:   sysconfig.Get(tenant,key) = 租户值 → 全局(0) → 代码默认
广告下发: GET /api/ads (JWT tenant) → store/ads → canShow → <ad-slot>/utils.ad 渲染
激励:   看完 isEnded → POST /api/ad/reward → 日限校验 → Credit(ad_reward) → 刷新余额
```

## 不在本期范围（YAGNI）
- 激励视频服务端回调二次校验。
- 广告收益统计看板。
- `Config.vue` 的「继承全局」可视化标记可做可不做（非阻塞）。

## 待确认项
- 各页 banner 精确视觉位置，可实现时按布局微调。
- grid 格子广告空间较大，若「我的」底部不合适可改放或取消。
- 71 处调用点改造体量较大，实现计划将按包拆任务，逐包编译验证。
