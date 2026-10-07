# 定位与地图 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** App 启动即获取经纬度；发漂流瓶与发动态可带地点（默认当前定位，可用地图手选，也可不带）；捞到的瓶子显示距离。

**Architecture:** 服务端 `geo/regeo` 已实现 Google/腾讯双供应商分派，`User.Lat/Lng` 已有并可上报——这两块零改动。新增的是：`Bottle`/`Moment` 的位置字段、捞瓶打分的距离维度、以及客户端从零接入定位与地图。判断逻辑（距离、分桶、降级）先抽成纯函数再测，与本仓库 20 个 `*_test.go` 一致：没有一个碰数据库。

**Tech Stack:** Go 1.x + Gin + GORM v2 + MySQL + Redis · Flutter 3.47.5 / Dart 3.13.4 + Riverpod · `geolocator` ^13.x · `google_maps_flutter` ^2.x

**Spec:** `docs/superpowers/specs/2026-09-19-google-capabilities-design.md`（§五）

## Global Constraints

- **Flutter ≥ 3.47.0**，Dart `^3.13.0`。不要升 `flutter_secure_storage`（锁 `^9.2.4`）与 `path_provider_foundation`（`dependency_overrides: 2.4.1`）——会撞 Flutter 3.47 已移除的 `Architecture.arm64e`。
- **新增 sysconfig key 必须同步写 `internal/sysconfig/sysconfig.go` 的 defaults**（空串会导致开关逻辑反转），并登记 `internal/admin/meta.go` 白名单。本计划新增 1 个：`match_w_dist`。
- **经纬度绝不下发给他人**：`Lat`/`Lng` 一律 `json:"-"`，对外只给 `distance_km` 与 `city`/`place_name`。先例见 `discover/service.go:84`。
- **地图上只显示用户自己选的那一个点**，绝不标注其他用户。
- **ID 一律字符串下发**（`json` tag 加 `,string`）；多租户所有查询按 `tenant_id` 过滤。
- **提交前验证**：`cd server && go build ./... && go vet ./...`；前端 `cd app/bottles && flutter analyze`。
- **跑测试绕开已知 flaky 包**：`go test $(go list ./... | grep -v /internal/robot)`——`internal/robot/identity_guard_test.go` 有 3 个既有的随机性不稳定测试，与本计划无关（已在 HEAD 验证同样失败）。
- 回复用中文；代码 / 标识符 / commit message 英文（conventional commits）。

## 对 spec 的三处修正（实施前已核实代码）

写 spec 时有三处判断不准，以此处为准：

1. **feed 缓存已有 10 分钟 TTL**（`feed.go` 的 `pipe.Expire(ctx, key, 10*time.Minute)`）。spec §五缺口 4 说「该队列不会自行过期」是错的。陈旧窗口被限制在 10 分钟内，所以位置分桶的价值是**把「最多陈旧 10 分钟」变成「一移动就立刻重建」**，而不是修一个无限期的 bug。仍然值得做——`Scope=local` 时用户跨城后会在这 10 分钟里看到旧城市的「同城」瓶子，那是肉眼可见的错误——但它不是紧急缺陷。
2. **腾讯分支也返回 `city`**（`r.Result.AddressComponent.City`）。spec §五缺口 2 说「两分支返回结构不一致」是错的，两边都是 `{address, city}`，只有**错误路径**缺 `city`。真正缺的只有短地名 `place`。
3. **`Next()` 与 `rebuild()` 各查了一次 viewer**，且 `rebuild` 的 `Select` 里没有 lat/lng。合并成一次加载即可拿到位置，**不增加查询数**。

---

## File Structure

**后端**

| 文件 | 职责 | 动作 |
|---|---|---|
| `server/pkg/geodist/geodist.go` | **新增**：距离与地理分桶纯函数 | 创建 |
| `server/pkg/geodist/geodist_test.go` | **新增**：上面的测试 | 创建 |
| `server/internal/model/model.go` | Bottle / Moment 位置字段 | 修改 |
| `server/internal/sysconfig/sysconfig.go` | `match_w_dist` key + default | 修改 |
| `server/internal/admin/meta.go` | 后台白名单登记 | 修改 |
| `server/internal/bottle/feed.go` | viewer 加载合并、sig 加位置、距离打分 | 修改 |
| `server/internal/bottle/service.go` | 建瓶写入位置 | 修改 |
| `server/internal/bottle/handler.go` | 建瓶入参 | 修改 |
| `server/internal/moment/*.go` | 发动态写入位置 | 修改 |
| `server/internal/geo/geo.go` | 返回短地名 `place` | 修改 |
| `server/internal/common/appdto/appdto.go` | 瓶子/动态 DTO 带 `distance_km` / `place_name` | 修改 |

`geodist` 放 `pkg/` 而不是塞进 `bottle/`：距离计算 `discover` 也要用（那里现在有一份私有的 `haversineKM`），提到公共包可以让两处共用同一份实现与同一份测试。

**前端**

| 文件 | 职责 | 动作 |
|---|---|---|
| `app/bottles/pubspec.yaml` | 依赖 | 修改 |
| `app/bottles/lib/core/platform/location.dart` | **新增**：包住定位插件 | 创建 |
| `app/bottles/lib/features/location/location_controller.dart` | **新增**：定位状态与上报 | 创建 |
| `app/bottles/lib/features/location/location_intro_page.dart` | **新增**：A7 权限说明屏 | 创建 |
| `app/bottles/lib/features/location/map_picker_page.dart` | **新增**：M1 地图选点 | 创建 |
| `app/bottles/lib/features/location/place_field.dart` | **新增**：M2 地点行，B2/F2 共用 | 创建 |
| `app/bottles/lib/features/bottle/write_bottle_page.dart` | B2 接入地点行 | 修改 |
| `app/bottles/lib/features/moment/compose_moment_page.dart` | **新增**：F2 发动态屏 | 创建 |
| `app/bottles/android/app/src/main/AndroidManifest.xml` | 定位权限 + Maps Key | 修改 |
| `app/bottles/ios/Runner/Info.plist` | 定位权限文案 | 修改 |
| `app/bottles/ios/Runner/AppDelegate.swift` | Maps Key | 修改 |

`place_field.dart` 单独成文件：B2 与 F2 的地点行是**同一个组件**，spec 明确要求两处同构；拆开写必然漂移。

---

## Task 1: 距离与分桶纯函数 + 位置字段

**Files:**
- Create: `server/pkg/geodist/geodist.go` · `server/pkg/geodist/geodist_test.go`
- Modify: `server/internal/model/model.go`（`Bottle` 与 `Moment`）

**Interfaces:**
- Consumes: 无
- Produces:
  - `geodist.KM(lat1, lng1, lat2, lng2 float64) float64`
  - `geodist.HasFix(lat, lng float64) bool`
  - `geodist.Bucket(lat, lng float64) string`
  - `model.Bottle.Lat/Lng/PlaceName` · `model.Moment.City/Lat/Lng/PlaceName`

- [x] **Step 1: 写失败的测试**

创建 `server/pkg/geodist/geodist_test.go`：

```go
package geodist

import (
	"math"
	"testing"
)

// KM 用 haversine 算球面距离。取两个已知城市对照，容差 2%。
func TestKM(t *testing.T) {
	cases := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		wantKM                 float64
	}{
		// 孟买 ↔ 德里，公认约 1150 km
		{"孟买-德里", 19.0760, 72.8777, 28.6139, 77.2090, 1150},
		// 孟买市内：Bandra ↔ Colaba，约 15 km
		{"孟买市内", 19.0596, 72.8295, 18.9067, 72.8147, 17},
		{"同一点为 0", 19.0760, 72.8777, 19.0760, 72.8777, 0},
	}
	for _, c := range cases {
		got := KM(c.lat1, c.lng1, c.lat2, c.lng2)
		if c.wantKM == 0 {
			if got != 0 {
				t.Errorf("%s: 同一点应为 0, 实际 %.3f", c.name, got)
			}
			continue
		}
		if math.Abs(got-c.wantKM)/c.wantKM > 0.02 {
			t.Errorf("%s: 期望约 %.0f km, 实际 %.1f km", c.name, c.wantKM, got)
		}
	}
}

// HasFix 判断有没有真实定位。
//
// 这是整个距离链路最关键的一个判断：Bottle 加了经纬度列之后，
// **存量瓶子全是 0**，而 (0,0) 是几内亚湾里一个真实存在的坐标。
// 不判空就会把所有老瓶子算成在西非，距离排序全乱。
func TestHasFix(t *testing.T) {
	cases := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"存量数据的零值", 0, 0, false},
		{"孟买", 19.0760, 72.8777, true},
		{"只有纬度也算有定位", 19.0760, 0, true},
		{"只有经度也算有定位", 0, 72.8777, true},
		{"南半球负值", -33.8688, 151.2093, true},
	}
	for _, c := range cases {
		if got := HasFix(c.lat, c.lng); got != c.want {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.want, got)
		}
	}
}

// Bucket 把经纬度粗化成缓存分桶键（约 40km 量级）。
// 市内移动不换桶、跨城换桶，这是它唯一要满足的性质。
func TestBucket(t *testing.T) {
	// 孟买市内两点（相距约 17km）应同桶
	a := Bucket(19.0596, 72.8295)
	b := Bucket(18.9067, 72.8147)
	if a != b {
		t.Errorf("市内两点应同桶: %q vs %q", a, b)
	}
	// 孟买 vs 德里（约 1150km）必须不同桶
	if c := Bucket(28.6139, 77.2090); c == a {
		t.Errorf("孟买与德里不应同桶, 都是 %q", c)
	}
	// 无定位时给一个稳定的固定桶，不能每次都不同——否则 feed 永远命中不了缓存
	if Bucket(0, 0) != Bucket(0, 0) {
		t.Error("无定位时分桶必须稳定")
	}
	if Bucket(0, 0) == a {
		t.Error("无定位的桶不能和有定位的桶相同")
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./pkg/geodist/ -v`
Expected: 编译失败，`undefined: KM` / `undefined: HasFix` / `undefined: Bucket`

- [x] **Step 3: 实现**

创建 `server/pkg/geodist/geodist.go`：

```go
// Package geodist 地理距离与分桶。
//
// 抽成公共包是因为 bottle(捞瓶打分) 与 discover(发现页) 都要算距离，
// 而 discover 里原本有一份私有的 haversineKM。两份实现意味着两处要各自维护
// 「零值怎么处理」这类边界，迟早漂移。
package geodist

import (
	"fmt"
	"math"
)

// KM 两点间球面距离（公里），haversine。
func KM(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// HasFix 是否有真实定位。
//
// ⚠️ 这是距离链路上最容易出事的一处。Bottle / Moment 加经纬度列之后，
// **存量数据全是 0**，而 (0.0, 0.0) 是几内亚湾里一个真实坐标点。
// 不判空的话，所有老瓶子会被算成在西非，距离排序整个失真。
// 凡是要用经纬度的地方，先过这一关。
func HasFix(lat, lng float64) bool {
	return lat != 0 || lng != 0
}

// noFixBucket 无定位时的固定桶名。
//
// 必须固定：如果每次返回不同的值，feed 缓存键每次都变，
// 没开定位的用户就永远命中不了缓存，每次捞瓶都触发一次全量重建。
const noFixBucket = "nofix"

// Bucket 经纬度 → 缓存分桶键，粒度约 40km × 40km。
//
// 用途是给捞瓶 feed 的缓存键加一个位置维度：用户跨城之后立刻换桶、
// 立刻重建队列，而市内移动不换桶、不浪费重建。
//
// 取 0.35° 而不是更细的粒度，是因为 feed 重建是一次带打分的全表扫描，
// 桶太细会让重建过于频繁。0.35° 纬度约 39km；经度方向随纬度收缩，
// 在印度（北纬 20° 上下）约 36km，量级合适。
func Bucket(lat, lng float64) string {
	if !HasFix(lat, lng) {
		return noFixBucket
	}
	const size = 0.35
	return fmt.Sprintf("%d_%d", int(math.Floor(lat/size)), int(math.Floor(lng/size)))
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./pkg/geodist/ -v`
Expected: PASS（3 个测试函数，共 12 个断言）

- [x] **Step 5: 加 Bottle 位置字段**

`server/internal/model/model.go` 的 `Bottle`，在 `City` 之后加：

```go
	// Lat/Lng 发瓶地点。⚠️ 存量瓶子为 0,用 geodist.HasFix 判空后再算距离——
	// (0,0) 是几内亚湾一个真实坐标,不判空老瓶子会被算成在西非。
	// 与 User.Lat/Lng 一样标 json:"-",对外只给 distance_km。
	Lat float64 `gorm:"index:idx_bottle_geo,priority:1" json:"-"`
	Lng float64 `gorm:"index:idx_bottle_geo,priority:2" json:"-"`
	// PlaceName 短地名("Bandra West"),展示用。空=发瓶时没带地点。
	PlaceName string `gorm:"size:64" json:"place_name,omitempty"`
```

- [x] **Step 6: 加 Moment 位置字段**

`server/internal/model/model.go` 的 `Moment`，在 `Visible` 之后加：

```go
	// 位置。Moment 原本一个位置字段都没有,四个一起加。
	// 口径与 Bottle 一致:存精确经纬度供算距离,对外只给 city / place_name。
	City      string  `gorm:"size:32" json:"city,omitempty"`
	Lat       float64 `gorm:"index:idx_moment_geo,priority:1" json:"-"`
	Lng       float64 `gorm:"index:idx_moment_geo,priority:2" json:"-"`
	PlaceName string  `gorm:"size:64" json:"place_name,omitempty"`
```

- [x] **Step 7: 让 discover 复用公共实现**

`server/internal/discover/service.go`：删掉私有的 `haversineKM` 函数，把调用处（`:182`）改为 `geodist.KM(...)`，并加 import `"driftbottle/pkg/geodist"`。若 `math` 因此不再被使用，一并删掉该 import。

- [x] **Step 8: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/pkg/geodist/ server/internal/model/model.go server/internal/discover/service.go
git commit -m "feat(geo): add shared distance helpers and location columns"
```

---

## Task 2: 捞瓶 feed 的位置维度

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`
- Modify: `server/internal/bottle/feed.go`
- Test: `server/internal/bottle/feed_geo_test.go`（创建）

**Interfaces:**
- Consumes: Task 1 的 `geodist.KM` / `HasFix` / `Bucket`
- Produces:
  - `sysconfig.KeyMatchWDist = "match_w_dist"`（默认 `"15"`）
  - `bottle.distanceScore(viewerLat, viewerLng, bLat, bLng, w float64) float64`

- [x] **Step 1: 写失败的测试**

创建 `server/internal/bottle/feed_geo_test.go`：

```go
package bottle

import (
	"math"
	"testing"
)

// distanceScore 距离衰减打分。
//
// 三条边界比公式本身重要得多：
//   - 瓶子没有经纬度(存量数据) → 不计分也**不惩罚**,退回 wCity 维度
//   - 浏览者没有经纬度(拒绝定位) → 同样不计分,与 Z6「降级不阻断」一致
//   - 有定位时越近分越高,且不为负
func TestDistanceScore(t *testing.T) {
	const w = 15
	mumbaiLat, mumbaiLng := 19.0760, 72.8777

	t.Run("瓶子无经纬度时不计分也不惩罚", func(t *testing.T) {
		if got := distanceScore(mumbaiLat, mumbaiLng, 0, 0, w); got != 0 {
			t.Errorf("存量瓶子应得 0 分, 实际 %.3f", got)
		}
	})

	t.Run("浏览者无经纬度时不计分", func(t *testing.T) {
		if got := distanceScore(0, 0, mumbaiLat, mumbaiLng, w); got != 0 {
			t.Errorf("未定位的浏览者应得 0 分, 实际 %.3f", got)
		}
	})

	t.Run("同一点拿满分", func(t *testing.T) {
		got := distanceScore(mumbaiLat, mumbaiLng, mumbaiLat, mumbaiLng, w)
		if math.Abs(got-w) > 0.001 {
			t.Errorf("零距离应拿满分 %.0f, 实际 %.3f", float64(w), got)
		}
	})

	t.Run("越近分越高", func(t *testing.T) {
		near := distanceScore(mumbaiLat, mumbaiLng, 19.0596, 72.8295, w) // 约 6km
		far := distanceScore(mumbaiLat, mumbaiLng, 28.6139, 77.2090, w)  // 约 1150km
		if near <= far {
			t.Errorf("近的应得分更高: near=%.3f far=%.3f", near, far)
		}
	})

	t.Run("再远也不给负分", func(t *testing.T) {
		// 对跖点附近
		if got := distanceScore(mumbaiLat, mumbaiLng, -19.0760, -107.1223, w); got < 0 {
			t.Errorf("距离项不应为负, 实际 %.3f", got)
		}
	})
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/bottle/ -run TestDistanceScore -v`
Expected: 编译失败，`undefined: distanceScore`

- [x] **Step 3: 实现打分函数**

在 `server/internal/bottle/feed.go` 的 `rebuild` 之前加：

```go
// distanceScore 距离衰减打分：越近分越高，上限 w，下限 0。
//
// 任一方没有定位就返回 0——**不计分，也不惩罚**。这一点很重要：
//   - 存量瓶子的经纬度是 0,若按「距离极远」处理,老瓶子会被永久压到池底
//   - 浏览者拒绝定位时,整个距离维度消失,其余维度照常工作(与 Z6 的降级原则一致)
//
// 衰减用 1/(1+d/halfKM)：在 halfKM 处得半分，无拐点、不会为负，
// 比线性截断更适合「同城很近」与「隔壁城市」之间的平滑过渡。
func distanceScore(viewerLat, viewerLng, bLat, bLng, w float64) float64 {
	if !geodist.HasFix(viewerLat, viewerLng) || !geodist.HasFix(bLat, bLng) {
		return 0
	}
	const halfKM = 25.0
	d := geodist.KM(viewerLat, viewerLng, bLat, bLng)
	return w / (1 + d/halfKM)
}
```

在 `feed.go` 的 import 里加 `"driftbottle/pkg/geodist"`。

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/bottle/ -run TestDistanceScore -v`
Expected: PASS（5 个子测试）

- [x] **Step 5: 加 sysconfig key 与 default**

`server/internal/sysconfig/sysconfig.go`，在 `KeyMatchWRandom` 之后加：

```go
	KeyMatchWDist   = "match_w_dist"   // 距离衰减(需要双方都有经纬度)
```

并在 defaults 里 `KeyMatchWRandom: "15",` 之后加：

```go
	KeyMatchWDist:   "15",
```

> 取 15 与 `gender` 同级、低于 `city` 的 25。距离与「同城」语义重叠（同城≈距离近），
> 给高权重会让地理维度在总分里翻倍。`match_w_city` **不动**——同城是离散的行政区划信号，
> 距离是连续的物理信号，两者不等价：跨省相邻市可能只有 30km，同市两端可能 60km。

- [x] **Step 6: 登记后台白名单**

`server/internal/admin/meta.go`，在其余 `KeyMatchW*` 那一组里加：

```go
	{Key: sysconfig.KeyMatchWDist, Label: "距离衰减(双方都有定位才生效)", Group: "匹配权重", Type: "int"},
```

- [x] **Step 7: 合并 viewer 加载，把位置带进 sig 与打分**

`feed.go` 现在在 `Next` 里算 key、在 `rebuild` 里查 viewer，而 sig 需要 viewer 的位置。
改为 **`Next` 查一次 viewer 并把它传给 `rebuild`**——查询数不变（`rebuild` 原本那次删掉）。

① `Next` 的开头改为：

```go
func (f *FeedService) Next(tenantID, userID int64, city string, tags []string, ft Filter) ([]model.Bottle, error) {
	ctx := context.Background()

	// 一次取齐浏览者画像:性别/城市供打分,经纬度供距离打分与缓存分桶。
	// 原本 rebuild 里也查一次,合并到这里,查询数不变。
	var viewer model.User
	f.db.Select("user_id, gender, city, lat, lng").First(&viewer, "user_id = ?", userID)
	if city == "" {
		city = viewer.City
	}

	sig := ft.sig()
	if InNightWindow(tenantID) {
		sig += "-n"
	}
	// 位置分桶进缓存键:跨城后立刻换桶、立刻重建,而不必等 10 分钟 TTL 自然过期。
	// 市内移动不换桶,不浪费一次全量重建。
	sig += "-" + geodist.Bucket(viewer.Lat, viewer.Lng)
	key := feedKey(tenantID, userID, sig)
```

② `Next` 里调用 `rebuild` 的那一处，改为传入 `viewer`：

```go
		if err := f.rebuild(ctx, tenantID, viewer, city, tags, ft); err != nil {
```

③ `rebuild` 的签名与开头改为：

```go
func (f *FeedService) rebuild(ctx context.Context, tenantID int64, viewer model.User, city string, tags []string, ft Filter) error {
	userID := viewer.UserID
	size := int(sysconfig.GetInt64(tenantID, sysconfig.KeyFeedSize))
	if size <= 0 {
		size = 50
	}
	tagSet := f.viewerTags(userID, tags)
```

（原来那三行 `var viewer model.User` / `f.db.Select(...)` / `if city == "" { city = viewer.City }` 删掉——已经在 `Next` 里做过。）

④ `rebuild` 末尾重算 key 的那一段，也要带上同一个分桶，否则写入的 key 和读取的 key 对不上：

```go
	sig := ft.sig()
	if night {
		sig += "-n"
	}
	sig += "-" + geodist.Bucket(viewer.Lat, viewer.Lng)
	key := feedKey(tenantID, userID, sig)
```

⑤ 打分循环里，在「同城」那一项之后加距离项：

```go
		// 同城
		if city != "" && b.City == city {
			s += wCity
		}
		// 距离衰减:双方都有经纬度才生效,否则不计分也不惩罚
		s += distanceScore(viewer.Lat, viewer.Lng, b.Lat, b.Lng, wDist)
```

并在权重那一段加：

```go
	wDist := float64(sysconfig.GetInt(tenantID, sysconfig.KeyMatchWDist))
```

- [x] **Step 8: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/internal/sysconfig/ server/internal/admin/meta.go server/internal/bottle/
git commit -m "feat(bottle): score feed by distance and bucket cache key by location"
```

> ⚠️ **本任务改动了捞瓶排序，但没有集成测试覆盖。** 上线前必须在有数据的环境手工验证：
> ① 未开定位的用户捞瓶正常（距离项为 0，不报错、不空池）；
> ② 存量瓶子（经纬度 0）不会被排到最前或最后；
> ③ 跨城后立刻拿到新池子而不是等 10 分钟。

---

## Task 3: 逆地理返回短地名

**Files:**
- Modify: `server/internal/geo/geo.go`

**Interfaces:**
- Consumes: 无
- Produces: `GET /geo/regeo` 响应增加 `place` 字段

- [x] **Step 1: Google 分支取 sublocality**

`server/internal/geo/geo.go` 的 `googleRegeo`，在解析 `city` 的循环里一并取短地名：

```go
	addr := r.Results[0].FormattedAddress
	city, place := "", ""
	for _, comp := range r.Results[0].AddressComponents {
		for _, t := range comp.Types {
			// locality 是「市」；印度部分地区只有 administrative_area_level_2，兜一层
			if t == "locality" {
				city = comp.LongName
			} else if city == "" && t == "administrative_area_level_2" {
				city = comp.LongName
			}
			// place 是给界面看的短地名（"Bandra West"）。
			// formatted_address 是完整地址串，太长，塞进地点行会撑破布局。
			if place == "" && (t == "sublocality" || t == "sublocality_level_1" || t == "neighborhood") {
				place = comp.LongName
			}
		}
	}
	// 取不到街区就退回城市名，保证这个字段永远有值可显示
	if place == "" {
		place = city
	}
	apilog.Record(tenantID, "geo_google", lat+","+lng, 0, addr, true)
	response.OK(c, gin.H{"address": addr, "city": city, "place": place})
```

- [x] **Step 2: 腾讯分支补同名字段**

腾讯分支的结构体加一层，并在成功返回处带上 `place`：

```go
			AddressComponent struct {
				City     string `json:"city"`
				District string `json:"district"`
			} `json:"address_component"`
```

```go
	place := r.Result.AddressComponent.District
	if place == "" {
		place = r.Result.AddressComponent.City
	}
	apilog.Record(middleware.TenantID(c), "geo_regeo", lat+","+lng, r.Status, addr, r.Status == 0)
	response.OK(c, gin.H{"address": addr, "city": r.Result.AddressComponent.City, "place": place})
```

- [x] **Step 3: 错误路径也要有这三个字段**

把三处 `response.OK(c, gin.H{"address": ""})` 统一改为：

```go
	response.OK(c, gin.H{"address": "", "city": "", "place": ""})
```

> 前端按固定形状解析。错误路径少字段，客户端要么崩、要么处处写判空。
> 这正是 spec 里我误以为「两分支结构不一致」的真实成因——**不一致的是成功路径与错误路径**。

- [x] **Step 4: 验证并提交**

```bash
cd server && go build ./... && go vet ./...
git add server/internal/geo/geo.go
git commit -m "feat(geo): return a short place name alongside address and city"
```

---

## Task 4: 发瓶 / 发动态写入位置

**Files:**
- Modify: `server/internal/bottle/handler.go` · `service.go`
- Modify: `server/internal/moment/handler.go` · `service.go`
- Modify: `server/internal/common/appdto/appdto.go`

**Interfaces:**
- Consumes: Task 1 的字段
- Produces: 建瓶 / 发动态请求体新增 `lat` / `lng` / `place_name` / `city`；瓶子 DTO 新增 `distance_km`

- [x] **Step 1: 建瓶入参**

`server/internal/bottle/handler.go` 的建瓶请求结构体加：

```go
	// 位置为可选:用户可以选择不带地点(原型 M2 的「不显示地点」)。
	// 用指针区分「没传」与「传了 0」——0 是几内亚湾的真实坐标。
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
	PlaceName string   `json:"place_name"`
```

`service.go` 的 `Throw`（构造 `model.Bottle` 那一处）加：

```go
	if in.Lat != nil && in.Lng != nil {
		b.Lat, b.Lng = *in.Lat, *in.Lng
	}
	b.PlaceName = in.PlaceName
```

> `City` 已有，沿用现有赋值逻辑，不动。

- [x] **Step 2: 发动态入参**

`server/internal/moment/handler.go` 的发布请求结构体加同样四个字段（`City` 也要，因为 `Moment` 原本没有）：

```go
	City      string   `json:"city"`
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
	PlaceName string   `json:"place_name"`
```

`service.go` 构造 `model.Moment` 处对应赋值，`Lat`/`Lng` 同样判 nil。

- [x] **Step 3: 瓶子 DTO 带上距离**

`server/internal/common/appdto/appdto.go`，瓶子结构体加：

```go
	// DistanceKM 捞到这个瓶子时的距离。双方都有定位才有值。
	// **只给距离,不给经纬度**——原始坐标配合多次采样可以三角定位。
	// 建议在转换处做区间化,不要给到小数点后多位。
	DistanceKM *float64 `json:"distance_km,omitempty"`
	PlaceName  string   `json:"place_name,omitempty"`
```

转换函数按 `geodist.HasFix` 判空后填充；两边任一无定位则留 `nil`（前端据此隐藏距离，显示城市名——见原型 Z6 的「不要显示成 0 km 或未知」）。

- [x] **Step 4: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
git add server/internal/bottle/ server/internal/moment/ server/internal/common/appdto/
git commit -m "feat(bottle,moment): accept and expose optional post location"
```

---

## Task 5: Flutter 定位底座（A7）

**Files:**
- Modify: `app/bottles/pubspec.yaml`
- Create: `app/bottles/lib/core/platform/location.dart`
- Create: `app/bottles/lib/features/location/location_controller.dart`
- Create: `app/bottles/lib/features/location/location_intro_page.dart`
- Modify: `android/app/src/main/AndroidManifest.xml` · `ios/Runner/Info.plist`

**Interfaces:**
- Consumes: 无
- Produces:
  - `abstract class LocationClient { Future<LocationFix?> current(); Future<bool> hasPermission(); Future<bool> request(); }`
  - `class LocationFix { final double lat, lng; }`
  - `locationControllerProvider`

- [ ] **Step 1: 加依赖**

`pubspec.yaml` 的 `dependencies` 加（不要碰被锁住的那两条）：

```yaml
  geolocator: ^13.0.0
```

Run: `cd app/bottles && flutter pub get`
Expected: `Got dependencies.`，**不出现 native-assets / `Architecture.arm64e` 报错**。若出现，`git checkout pubspec.yaml pubspec.lock` 后停下报告。

- [ ] **Step 2: 平台权限**

`android/app/src/main/AndroidManifest.xml` 的 `<manifest>` 下加：

```xml
    <uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION"/>
    <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
```

`ios/Runner/Info.plist` 加（**只申请 WhenInUse，不要 Always**——后者会触发额外审核问询）：

```xml
<key>NSLocationWhenInUseUsageDescription</key>
<string>用于显示瓶子漂了多远、优先推荐附近的人，以及发布时自动带上地点。随时可以关闭。</string>
```

> 这段文案**随包审核，不能后台配置**，后台改了不生效。

- [ ] **Step 3: 写定位封装**

创建 `app/bottles/lib/core/platform/location.dart`：

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';

/// 一次定位结果。只有经纬度——地址由服务端 /geo/regeo 逆地理得到，
/// 客户端不持有任何地图 Key 之外的地理能力。
class LocationFix {
  const LocationFix(this.lat, this.lng);
  final double lat;
  final double lng;
}

/// 把定位插件关在这一个文件里，理由同 oauth.dart：
/// 插件 API 变动频繁，隔离后升级只改这一处。
abstract class LocationClient {
  Future<bool> hasPermission();

  /// 弹系统权限框。返回是否拿到授权。
  /// ⚠️ iOS 上这个框**一辈子只弹一次**，拒绝后再也唤不起来，
  /// 所以调用前必须先过 A7 说明屏。
  Future<bool> request();

  /// 当前位置。无授权或定位服务关闭时返回 null——**不抛异常**，
  /// 因为「没有位置」是正常状态，不是错误（见原型 Z6：降级不阻断）。
  Future<LocationFix?> current();
}

class RealLocationClient implements LocationClient {
  @override
  Future<bool> hasPermission() async {
    final p = await Geolocator.checkPermission();
    return p == LocationPermission.always || p == LocationPermission.whileInUse;
  }

  @override
  Future<bool> request() async {
    final p = await Geolocator.requestPermission();
    return p == LocationPermission.always || p == LocationPermission.whileInUse;
  }

  @override
  Future<LocationFix?> current() async {
    if (!await Geolocator.isLocationServiceEnabled()) return null;
    if (!await hasPermission()) return null;
    try {
      final pos = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.medium),
      );
      return LocationFix(pos.latitude, pos.longitude);
    } catch (_) {
      // 超时、硬件故障、模拟器没设位置——一律当作「拿不到」。
      return null;
    }
  }
}

final locationClientProvider = Provider<LocationClient>((ref) => RealLocationClient());
```

> `geolocator` 13.x 的 `getCurrentPosition` 参数名以 `flutter pub get` 后的实际版本为准；
> 若签名不同，只改**本文件内部**，不要改 `LocationClient` 接口。

- [ ] **Step 4: 写 controller**

创建 `location_controller.dart`：进 App 后调 `current()`，拿到就 `POST /api/user/update` 上报 `lat`/`lng`（该接口已存在，要求成对提交），并缓存本次 fix 供发布页使用。**拿不到位置不报错、不阻断**。

- [ ] **Step 5: 写 A7 说明屏**

创建 `location_intro_page.dart`，按原型 A7：定位图标、标题「让瓶子知道你在哪儿」、三条用途列表（看到瓶子漂了多远 / 优先推荐附近的人 / 发瓶时自动带地点）、一条隐私说明（别人只能看到城市和大致距离）、「开启定位」与「暂不开启」两个按钮。

**出现时机是首次进入主界面之后，不是注册流程里**——注册时多一个权限框会明显拉低完成率。点「开启定位」才调 `request()`。

- [ ] **Step 6: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/
git commit -m "feat(app): location permission flow and reporting"
```

---

## Task 6: Flutter 地图选点（M1 / M2）

**Files:**
- Modify: `pubspec.yaml`
- Create: `lib/features/location/map_picker_page.dart` · `place_field.dart`
- Modify: `android/app/src/main/AndroidManifest.xml` · `ios/Runner/AppDelegate.swift`

**Interfaces:**
- Consumes: Task 5 的 `LocationClient`；Task 3 的 `place` 字段
- Produces:
  - `class PickedPlace { final double lat, lng; final String city, placeName; }`
  - `Future<PickedPlace?> showMapPicker(BuildContext, {LocationFix? initial})`
  - `class PlaceField extends ConsumerWidget`（B2 / F2 共用）

- [ ] **Step 1: 加依赖与 Key**

```yaml
  google_maps_flutter: ^2.9.0
```

Android，在 `AndroidManifest.xml` 的 `<application>` 内加：

```xml
        <meta-data android:name="com.google.android.geo.API_KEY"
                   android:value="REPLACE-WITH-ANDROID-MAPS-KEY"/>
```

iOS，`AppDelegate.swift` 里 `GMSServices.provideAPIKey("REPLACE-WITH-IOS-MAPS-KEY")`。

> **这两把 Key 必须打进包里**，Maps SDK 的硬性要求，躲不掉。防护不是保密，
> 而是**包名 + SHA-1 / Bundle ID 限制**——别人拿到也用不了。
> 它们与后台 `app_google_map_key`（服务端 Geocoding，IP 白名单）是**三把不同的 Key**，不要共用。

- [ ] **Step 2: 写地图选点页**

创建 `map_picker_page.dart`，按原型 M1：

- 地图铺满，**大头针固定在屏幕中心不动，拖动的是地图**（比拖大头针精度高，手指不挡目标点）
- 右下角一个「回到我的位置」按钮
- 地图停止移动 → 取中心经纬度 → 调 `GET /geo/regeo` → 底部地址卡显示 `place` / `city`
- 底部两个按钮：「用这个地点」「不显示地点」

**只显示用户自己的那一个点，不渲染任何其他用户。** 即使模糊化，多点采样仍可反推真实住址。

**降级**：地图 SDK 加载失败或无 Key 时，本页退化成一个只显示当前城市的地址列表，**不能白屏**。

- [ ] **Step 3: 写地点行组件**

创建 `place_field.dart`：一行，三态——

1. **未选**（有定位）：显示逆地理得到的当前地址 + 「当前定位，点击可改」
2. **已选**：显示所选地点 + 一个清除按钮
3. **不显示地点**：灰字「添加地点」+「这条不会带位置」

「不显示地点」必须是**一等选项**，不能靠「不去选」实现——默认带当前定位意味着用户不做任何操作就会暴露位置。

- [ ] **Step 4: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/
git commit -m "feat(app): map place picker shared by bottle and moment compose"
```

---

## Task 7: 接入发瓶与发动态（B2 / F2）

**Files:**
- Modify: `lib/features/bottle/write_bottle_page.dart`
- Create: `lib/features/moment/compose_moment_page.dart`
- Modify: `lib/data/repositories.dart` · `remote_repositories.dart` · `mock_repositories.dart`
- Modify: `lib/app/routes.dart` · `router.dart`

**Interfaces:**
- Consumes: Task 6 的 `PlaceField` / `PickedPlace`；Task 4 的新增请求字段
- Produces: `Routes.composeMoment = '/moment/compose'`

- [ ] **Step 1: Repository 带上位置字段**

发瓶与发动态的请求方法各加 `double? lat, double? lng, String? city, String? placeName` 四个可选参数，三处实现（抽象 / remote / mock）同步补齐。

Run: `cd app/bottles && flutter analyze`
Expected: 无 `missing_concrete_implementation`——Mock 漏补会在这里暴露。

- [ ] **Step 2: B2 接入地点行**

`write_bottle_page.dart` 在「投放范围」之后插入 `PlaceField`，提交时把选中的位置一并传给 repository。

- [ ] **Step 3: 建 F2 发动态屏**

创建 `compose_moment_page.dart`，按原型 F2：文本域 + 九宫格图片（≤9）+ 地点行 + 可见范围（公开/仅好友/仅自己）。

> **这一屏原本不存在**：原型从 F1 动态广场直接跳到 F3 动态详情，发动态屏是既有缺口，
> 不是本次需求引入的——但「发动态可以选地址」需要它作宿主。

在 `routes.dart` / `router.dart` 注册，并在动态广场（F1）加一个发布入口。

- [ ] **Step 4: 端到端手工验证**

Android 模拟器（`Pixel_7_API_36`，已建好）里：

1. 首次进入主界面 → 出现 A7 说明屏 → 点「开启定位」→ 系统框 → 授权
2. 发瓶页地点行自动显示当前地址
3. 点进去 → 地图选点 → 拖动地图 → 地址跟着变 → 「用这个地点」→ 回到发瓶页显示所选地点
4. 点清除 → 变成「添加地点」
5. **拒绝定位后重进**：A7 的「暂不开启」→ 发瓶页地点行显示「添加地点」，点进去仍可手选（Z6 的第二条降级路径）
6. 捞到带位置的瓶子显示「城市 · x km」；捞到存量瓶子（无经纬度）只显示城市，**不显示 0 km**

> 模拟器设位置：扩展控制面板 → Location，或 `adb emu geo fix <lng> <lat>`（注意是**经度在前**）。

- [ ] **Step 5: 提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/
git commit -m "feat(app): attach location when composing bottles and moments"
```

---

## 实施记录（2026-09-19，Task 1–4 已完成）

后端四个任务已实施并提交（`620d421` / `5242f77` / `2480fc2` / `c7bbbaa`）。
与计划不一致之处，接手 Task 5–7 前请先读：

1. **多做了一个 `geodist.CoarseKM`（距离粗化）并带测试。** 计划 Task 4 Step 3 只写了
   「建议在转换处做区间化」——那是个建议，不是可执行步骤。实际做成了纯函数：
   1km 内一律 0.5；10km 内取 0.5 的倍数；100km 内取整；再远取 10 的倍数。
   测试的重点不是分档对不对，而是**同一档里不同的真实距离必须粗化成同一个值**，
   那才是抵抗多点采样三角定位的那一下。
2. **新增 `appdto.DistanceFrom(viewerLat, viewerLng, targetLat, targetLng) *float64`。**
   `FromBottleWith` 不查库、拿不到浏览者位置，所以距离没法在转换函数内部算。
   做成独立函数，由调用方填到 `Bottle.DistanceKM` 上。
   **⚠️ 目前还没有任何调用方真正填它** —— 捞瓶链路要在 service 层把浏览者位置
   传到 DTO 转换处，这一步留给接手的人，否则 `distance_km` 永远是 nil。
3. **`moment.Create` 从位置参数改成了 `CreateInput` 结构体。** 加位置后会变成 8 个参数，
   调用处看不出哪个是哪个。形状对齐 `bottle.CreateInput`。只有一个调用点，改动安全。
4. **`geo.go` 的错误路径有 7 处**（计划说「三处」）。全部统一成 `{address, city, place}`
   三字段，共 9 个返回点形状一致。
5. **`internal/robot` 的 3 个既有 flaky 测试仍在**，与本计划无关。
   跑全量用 `go test $(go list ./... | grep -v /internal/robot)`。

**Task 2 末尾那三条手工验证仍未做**（本机无 MySQL/Redis）：
① 未开定位的用户捞瓶正常；② 存量瓶子不被排到两极；③ 跨城立刻换池。
**这三条在有数据的环境跑通之前，不要认为捞瓶排序改动是安全的。**

---

## 自查

**Spec 覆盖**（对照 spec §五）：

| Spec 要求 | 落点 |
|---|---|
| 缺口 1 缺短地名 | Task 3 |
| 缺口 2 两分支结构不一致 | Task 3 Step 3（**已修正为「成功路径与错误路径不一致」**） |
| 缺口 3 Flutter 端从零 | Task 5 / 6 |
| 缺口 4 feed 无位置维度 | Task 2 Step 7（**已修正：有 10 分钟 TTL，问题小于 spec 所述**） |
| 缺口 5 打分缺距离维度 | Task 2 |
| 三把 Key 各自加限制 | Task 6 Step 1 |
| 下发口径（只给 distance_km） | Task 1 Step 5/6 + Task 4 Step 3 |
| 存量瓶子经纬度为 0 的回退 | Task 1（`HasFix`）+ Task 2（`distanceScore`） |
| 地图只显示自己 | Task 6 Step 2 |
| 「不显示地点」是一等选项 | Task 6 Step 3 |
| A7 时机与 iOS 文案 | Task 5 Step 2/5 |
| 原型 B2 / F2 / M1 / M2 | Task 6 / 7 |

**不在本计划**（spec 的遗留项）：Places Autocomplete 地址搜索（决策 7，下一期）；`app_discover_max_km` 相关的发现页调整（已实现，无改动）。

**已知薄弱处**：Task 2 改动了捞瓶排序，而本仓库没有数据库测试，纯函数测试覆盖不到「排序后的池子长什么样」。Task 2 末尾列了三条必须手工验证的项，**不要跳过**。
