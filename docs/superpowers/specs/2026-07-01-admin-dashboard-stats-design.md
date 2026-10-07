# Admin 概览页数据增强 — 设计文档

- 日期：2026-07-01
- 分支：main
- 目标：把 admin 后台「概览/dashboard」从仅有的全量 COUNT 卡片，升级为带「今日/本周/本月」当期 KPI + 近 30 天趋势折线图的运营概览，覆盖新增用户、充值金额、消费流水、付费用户/ARPU。

## 1. 背景与现状

- 前端 `admin/`：Vue3 + Vite，刻意零依赖（无 UI 库、无图表库）。
  - `admin/src/views/Dashboard.vue`（41 行）：7 个数字卡片，调 `GET /admin/api/stats`。
  - `admin/src/api.js`：`stats()` 等封装；Bearer Token 鉴权。
- 后端 `server/`：Gin + GORM + MySQL/MariaDB，多租户（所有业务表带 `tenant_id`）。
  - `internal/admin/service.go` `Stats(tenantID)`：全量 COUNT。
  - `internal/admin/handler.go`：`tenantFromCtx(c)` 已支持 `X-Tenant-ID` header 或 `?tenant_id=`，`0` = 全租户。
  - `GET /admin/api/tenants`（`listTenants`）已存在 → 租户筛选器数据源。
- 数据底子（均可支撑统计）：
  - `users.created_at` + `is_robot` + `tenant_id`：按时间统计新增（默认排除机器人）。
  - `pay_orders`：`price_fen`(分)、`status`(paid)、`paid_at`、`tenant_id`：充值金额。
  - `wallet_txns`：`direction`(credit/debit)、`coins`、`scene`、`created_at`、`tenant_id`：消费流水。

## 2. 需求（已确认）

1. 呈现形式：**数字卡片 + 趋势折线图**（引入图表库 ECharts）。
2. KPI 指标（每张卡片同时显示 今日/本周/本月 三个值），现有总量卡片保留：
   - 新增用户（真人，`is_robot=false`）
   - 充值金额（元，`pay_orders.price_fen/100`，`status=paid`，按 `paid_at` 归期）
   - 消费流水（猛币，`wallet_txns` `direction=debit` 的 `coins` 求和）
   - 付费用户 / ARPU（当期充值去重用户数；ARPU = 充值额 / 付费人数）
3. 租户范围：**顶部加租户筛选器**，默认全租户汇总（`tenant_id=0`），可切单租户。
4. 趋势：**近 30 天固定、日粒度**，3 条折线（新增用户 / 充值额 / 消费额）。
5. 时间口径：今日 = 当天 00:00 起；本周 = 自然周（周一 00:00 起）；本月 = 当月 1 号 00:00 起。**全部按 `Asia/Shanghai` 计算**（修正现有 `time.Now().Truncate(24h)` 按 UTC 截断、东八区偏 8h 的问题）。

## 3. 后端设计

### 3.1 新增接口

`GET /admin/api/stats/overview`，复用 `tenantFromCtx(c)`（`0` = 全租户），鉴权同其他 admin 接口（`authMiddleware`）。一次返回当期 KPI + 近 30 天趋势。现有 `/stats` 不动。

响应（`response.OK` 包裹）：

```jsonc
{
  "kpi": {
    "new_users":     { "today": 12, "week": 80,  "month": 320 },
    "recharge_yuan": { "today": 99.00, "week": 880.50, "month": 3200.00 },
    "consume_coins": { "today": 540, "week": 3900, "month": 15800 },
    "paying_users":  { "today": 3,  "week": 22,  "month": 88 },
    "arpu_yuan":     { "today": 33.00, "week": 40.02, "month": 36.36 }
  },
  "trend": {
    "dates":         ["06-02", "...", "07-01"],
    "new_users":     [/* 30 个 int */],
    "recharge_yuan": [/* 30 个 float, 元 */],
    "consume_coins": [/* 30 个 int, 猛币 */]
  }
}
```

### 3.2 实现（`internal/admin/service.go` + `handler.go`）

- 新增 `Overview(tenantID int64) OverviewResp`，定义对应 struct（`PeriodInt{Today,Week,Month int64}`、`PeriodYuan{...float64}`、`Trend{...}`）。
- 时区：包级 `var cst = loadCST()`，`loadCST()` 取 `time.LoadLocation("Asia/Shanghai")`，失败回退 `time.FixedZone("CST", 8*3600)`。
- 边界 helper：`func dayStart(t)`, `func weekStart(t)`(周一), `func monthStart(t)`，均在 `cst` 下计算。
- 当期 KPI：每指标三段（today/week/month）用带 `WHERE` 的聚合：
  - 新增用户：`users` `WHERE is_robot = 0 AND created_at >= ? [AND tenant_id=?]` → `Count`。
  - 充值额：`pay_orders` `WHERE status='paid' AND paid_at >= ? [AND tenant_id=?]` → `SUM(price_fen)`，Go 里 `/100.0`。
  - 消费额：`wallet_txns` `WHERE direction='debit' AND created_at >= ? [AND tenant_id=?]` → `SUM(coins)`。
  - 付费用户：同充值条件 → `COUNT(DISTINCT user_id)`。
  - ARPU：充值额 / 付费人数（人数为 0 时记 0，避免除零）。
- 趋势（近 30 天）：每指标一条 `GROUP BY` 查询，再在 Go 补齐 0：
  - `SELECT DATE(CONVERT_TZ(created_at,'+00:00','+08:00')) d, <agg> v ... WHERE <时间下界=30天前的cst零点> GROUP BY d`。
  - 若 DB 未装时区表导致 `CONVERT_TZ` 返回 NULL，则退化为应用层按 `created_at` 落桶（在 Go 里用 `cst` 归日）。**实现时先验证 `CONVERT_TZ` 可用性**；不可用则统一走「查 30 天明细的轻量字段（仅时间+值）再 Go 分桶」的方案。
  - Go 端构造 `dates[30]` 与对应值数组，缺失日填 0。
- 租户过滤：`tenantID == 0` 不加 `tenant_id` 条件；否则加。封装一个 `scope(db, tenantID)` helper 复用。
- handler：`func (h *Handler) statsOverview(c)` → `response.OK(c, h.svc.Overview(tenantFromCtx(c)))`；在 `Register` 中 `auth.GET("/stats/overview", h.statsOverview)`。

## 4. 前端设计

### 4.1 依赖

- `admin/package.json` 增加 `echarts`。按需引入（`echarts/core` + `LineChart` + `GridComponent`/`TooltipComponent`/`LegendComponent` + `CanvasRenderer`）以控制打包体积。

### 4.2 组件

- 新增 `admin/src/components/Chart.vue`：薄封装。props：`option`（ECharts option）；`onMounted` init、`watch(option)` setOption、`onBeforeUnmount` dispose、`resize` 监听。容器默认高度（如 280px）。
- 重写 `admin/src/views/Dashboard.vue`：
  - 顶部：租户下拉（`全部租户` + `tenants()` 列表），`v-model` 绑定 `tenantId`，change 时重新拉数。
  - 区块 1「总量」：保留现有 `stats()` 的卡片。
  - 区块 2「当期 KPI」：4 组卡片，每组标题 + 今日/本周/本月三个数（充值额/ARPU 显示「¥」，消费显示「猛币」）。
  - 区块 3「近 30 天趋势」：3 个 `Chart.vue` 折线（或一个多 series，先做 3 个独立图，清晰）。
  - 加载态/空态处理；金额格式化（千分位、两位小数）。

### 4.3 API 层

- `admin/src/api.js` 增加：
  - `statsOverview: (tenantId) => req('GET', '/stats/overview' + (tenantId ? `?tenant_id=${tenantId}` : ''))`
  - `tenants: () => req('GET', '/tenants')`（若尚未封装）

### 4.4 数据流

```
Dashboard onMounted
  → tenants()                  // 填充下拉
  → stats(tenantId)            // 总量卡片
  → statsOverview(tenantId)    // KPI + 趋势
切换租户 → 重新 stats()/statsOverview()
```

## 5. 涉及文件

后端：
- `server/internal/admin/service.go`（新增 `Overview` + helper + struct）
- `server/internal/admin/handler.go`（新增 handler + 注册路由）

前端：
- `admin/package.json`（加 echarts）
- `admin/src/api.js`（加 statsOverview / tenants）
- `admin/src/components/Chart.vue`（新增）
- `admin/src/views/Dashboard.vue`（重写）

## 6. 测试与验证

- 后端：`cd server && go build ./... && go vet ./internal/admin/...`；手工 `curl` 校验 `/admin/api/stats/overview`（全租户与单租户各一次），核对金额单位（元）、趋势数组长度=30、缺失日为 0。
- 前端：`cd admin && npm install && npm run build` 通过；本地 `npm run dev` 目视检查卡片与折线渲染、租户切换刷新。
- 验收口径：今日/本周/本月边界落在 `Asia/Shanghai`；趋势 30 天连续无断点。

## 7. 非目标（YAGNI）

- 自定义日期范围 / 周月粒度切换（本期固定近 30 天日粒度）。
- 消费场景分布图、留存/漏斗等高级分析（后续按需）。
- 导出 CSV、定时报表。
