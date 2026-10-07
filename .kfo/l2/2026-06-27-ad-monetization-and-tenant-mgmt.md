# 流量主广告落地 · 租户管理 · 多租户配置作用域

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-27 |
| 状态 | COMPLETED + DEPLOYED（后端/admin 已部署；小程序广告需上传发版）|
| 触发 | 接入微信流量主变现；多租户上线后缺租户管理入口、config 误改全局、雪花租户 ID 前端丢精度 |
| 关联 | `docs/superpowers/plans/2026-06-25-multitenant-config-and-ads.md`（计划）· L3 `l3/2026-06-20-int64-id-js-precision.md`（精度复发）· L1 `l1/admin-platform.md` |

---

## 一、流量主广告 Phase B（计划 B1–B5）

多租户配置改造 A1–A3 已由前序提交完成（Config 复合主键 + sysconfig 三级回退 + 空 appid 兜底）。本次实现广告：

- **B1 配置键**：`ad_enabled` / `ad_inter_gap_sec` / `ad_reward_coin_coins` / `ad_reward_coin_daily` + 各广告位 `*_on` 开关；admin `meta.go`「流量主广告」分组。
- **B2 激励发币**：`internal/ad` 包，`POST /api/ad/reward`。Redis `Incr` 计每日上限、`bizNo` 让 `wallet.Credit` 幂等、看完(`isEnded`)才发；`wallet.SceneAdReward="ad_reward"`。
- **B3 下发**：`GET /api/ads` 按租户返回 `{enabled, inter_gap_sec, slots:{key:{on,unit}}}`。
- **B4 客户端基建**：`adApi`、`store/ads`（`canShow`）、`<ad-slot>`（easycom 自动注册）、`utils/ad`（`showInterstitial` 节流 / `showRewarded` 看完回调领奖）。
- **B5 接入**：App 启动预拉；11 个信息流位 + 捞瓶/详情插屏 + 我的页激励视频。

## 二、广告改为「按类型配 ID」（B1 重构）

原设计每个广告位各配一个 unit id（15 个），运营繁琐。改为：

- 按**类型**各配一个 unit：`ad_banner_unit` / `ad_inter_unit` / `ad_reward_unit` / `ad_native_unit`。
- 15 个广告位只留 `*_on` 开关，标签带类型前缀（`[原生·信息流] 首页`…）。
- `getAds` 按位输出 `{on, unit}`，unit 由所属类型解析 → **客户端零改动**。

## 三、Banner → 原生模板（微信平台变更）

微信下线 Banner、自动升级为原生模板广告。改动：
- 客户端 `ad-slot` 渲染 `<ad-custom :unit-id>`（原 `<ad ad-type="banner">` 作废）。
- admin 文案改「原生模板广告位ID(信息流/列表位,原Banner)」「[原生·信息流] xxx」。
- 配置键 `ad_banner_unit` 沿用（仅改标签),已填值不丢。

## 四、相机页补广告位

相机页(`privacy.vue`,默认启动页,流量最高)原先漏接。新增 slot `banner_privacy`（共用原生信息流 unit）：sysconfig 键 + getAds + meta 开关「[原生·信息流] 相机」+ 页面 `<ad-slot>`。

## 五、租户管理（admin 新增入口）

多租户上线后无创建租户入口。补：
- 后端 `admin.CreateTenant(name, appid, secret)`：事务建 `Tenant` + wx `AppCredential`(secret 加密)，appid 查重，建完 `credStore.Reload()` 即时生效；`POST /tenants`。
- 前端 `Tenants.vue`（列表 + 新增弹框）+ 路由 + Layout 导航「🏢 租户管理」。

## 六、Config 多租户作用域加固

config 端本就按 `tenantFromCtx` 隔离(三级回退)，但「全部租户」(0)下编辑写的是**全局默认**，易误操作。加：
- Config 页顶部作用域横幅：全局=橙色警示 / 具体租户=蓝色提示。
- 「全部租户」下首次保存二次确认(本次会话确认一次)。

## 七、雪花租户 ID 前端丢精度（int64 精度复发）

**现象**：新建租户(雪花 `tenant_id`,18 位 >2^53)在 admin config 页选中后**下拉框空白**,且 `X-Tenant-ID` 发错 → 读错租户。旧租户 1/100/200 小数字不复现。
**根因**：admin 前端用 `parseInt`/`Number()` 处理 `tenant_id` → 超 2^53 丢精度 → 与 `<option>` 值不匹配。即 L3 `int64-id-js-precision` 在 admin 侧复发。
**修复**：`tenant.js`/`Layout.vue`/`Config.vue` 全程按**字符串**处理 tenant_id（后端 `json:"tenant_id,string"` 本就是字符串,前端别再转 Number）。

## 八、运维

- **批量造数据**：`seed_tenant329.js`（SSH+MySQL 一次性）给指定租户建 10 机器人(按性别取 robot/ robot_man 头像)+ 50 漂流瓶(active,30天过期)。
- **credStore 直写库不生效**：app_credentials 直接写库后,运行中的内存 credStore 不会自动刷新 → 登录报 `2002 未知 appid`。需 `credStore.Reload()`(admin CreateTenant/UpdateCredential 会触发)或**重启服务**(启动时 Reload)。
- ⚠️ **重启地雷**：`main.go` 里 `credStore.Reload()` 失败是 `log.Fatal` → 服务起不来。若任一 `secret_enc` 是**明文**(明文微信 secret 32 hex;加密后 ~80 base64),解密失败会拖垮整个服务。重启前先核对所有 `secret_enc` 均为加密/空。
