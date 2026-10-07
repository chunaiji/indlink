# 同城 Tab 图片开关(cover)设计

日期:2026-07-03
状态:已确认,待实现

## 背景与目标

同城 Tab 目前是「搜索栏 + 性别/排序筛选 + 陌生人用户列表 + 广告 + tabBar」。
需要一个后台可控的开关:

- 开关**打开**:同城页只显示一张图片(隐藏搜索/筛选/用户列表),图片按屏宽铺满、高度自适应、可上下滚动。
- 开关**关闭**:隐藏图片,显示原有页面结构。

图片地址与开关均按租户在后台配置,通过一个独立只读接口下发,进入同城页时拉取,做到「改后台 → 重进 Tab / 下拉刷新即生效」。

## 需求确认(问答结论)

| 维度 | 结论 |
|------|------|
| 图片来源 | 后台按租户配置(sysconfig 下发) |
| 生效时机 | 单独配置接口,进同城页拉取 |
| 图片布局 | `mode="widthFix"` 宽度铺满、高度自适应、可滚动 |
| 点击交互 | 纯展示,不可点击 |
| 配置键命名 | `city_cover_on` / `city_cover_image` |
| 覆盖态 tabBar | 保留(仍是 Tab 页,可切走) |
| 接口路径 | `GET /city-config`,前端归入 `sysApi` |

## 数据流

```
管理台配置(按租户) city_cover_on / city_cover_image
   ▼
sysconfig(三级回退:租户值 → 全局 tenant0 → 代码默认)
   ▼
GET /city-config(sysconfig.Handler 只读接口)
   ▼
city.vue onShow 拉取 → cover = { on, image }
   ▼
cover.on && cover.image ? 显示图片(隐藏原页面) : 显示原页面(隐藏图片)
```

## 后端改动(Go)

### 1. `internal/sysconfig/sysconfig.go` — 新增配置键

```go
// 同城页图片覆盖
KeyCityCoverOn    = "city_cover_on"    // 覆盖开关 (0/1)
KeyCityCoverImage = "city_cover_image" // 覆盖图 URL(空=不覆盖)
```

### 2. `internal/sysconfig/sysconfig.go` — `defaults` 补默认值

> 关键:新增 key 必须加 defaults,否则空串会导致开关逻辑异常(项目既有教训)。

```go
KeyCityCoverOn:    "0",  // 默认关:显示原内容
KeyCityCoverImage: "",
```

### 3. `internal/sysconfig/handler.go` — 新增只读接口

照 `getTabs` 写法:

```go
// Register 内
api.GET("/city-config", h.getCityConfig)

// getCityConfig 返回同城页覆盖图开关与图片地址。
func (h *Handler) getCityConfig(c *gin.Context) {
    tid := h.tenantOf(c)
    response.OK(c, gin.H{
        "cover_on":    GetString(tid, KeyCityCoverOn) == "1",
        "cover_image": GetString(tid, KeyCityCoverImage),
    })
}
```

### 4. 管理台配置项注册

在 `internal/admin/meta.go` 按现有配置项格式注册 `city_cover_on` / `city_cover_image`,
让运营在管理台可视化编辑(具体字段格式实现时对齐现有项)。

## 前端改动(uni-app)

### 1. `client/src/api/index.js` — `sysApi` 新增方法

```js
cityConfig: () => get('/city-config'),
```

### 2. `client/src/pages/city/city.vue`

- `data` 新增:
  ```js
  cover: { on: false, image: '' }
  ```
- `onShow` 逻辑:
  ```js
  onShow() {
    this.loadCover()
    this.paidUsers = uni.getStorageSync('paid_city_users') || {}
    if (!this.cover.on) this.reload()   // 覆盖态无需拉用户列表
  }
  ```
  > 注:`loadCover` 为异步,需保证覆盖态确定后再决定是否 `reload`;
  > 实现时用 `await` 或在 `loadCover` 回调内触发 `reload`,避免竞态。
  > 兜底:`loadCover` 失败时 `cover.on = false`,照常 `reload()`。
- `template` 用 `v-if` 整体二选一:
  ```html
  <!-- 覆盖态:仅图片,可滚动 -->
  <scroll-view v-if="cover.on && cover.image" scroll-y class="cover-scroll">
    <image :src="cover.image" mode="widthFix" class="cover-img" />
  </scroll-view>
  <!-- 常规态:原搜索栏 + 筛选 + 列表 -->
  <block v-else>
    …原有结构…
  </block>
  ```
- `<tab-bar :current="1" />` 始终保留。
- 样式:`.cover-img { width: 100%; }`,`.cover-scroll` 高度占满内容区。

## 默认与边界(安全兜底)

| 情况 | 表现 |
|------|------|
| 未配置 / 接口拉取失败 | `cover.on=false` → 显示原内容(绝不白屏) |
| `cover_on=1` 但图片 URL 为空 | 视为不覆盖 → 回退原内容 |
| `cover_on=1` 且有图 | 只显示图片,隐藏搜索/筛选/列表 |

设计原则:**默认关、任何异常回退到原页面**。

## 测试

- 后端:`getCityConfig` 单测,验证三级回退 + 默认值(参照 `internal/sysconfig/resolve_test.go`)。
- 前端:手动切换开关,验证两态切换、图片滚动、失败兜底。

## 影响范围

- 后端:`sysconfig.go`、`handler.go`、`admin/meta.go`(+ 对应测试)。
- 前端:`api/index.js`、`pages/city/city.vue`。
- 不改动:同城列表接口 `/city/users`、其他页面。
