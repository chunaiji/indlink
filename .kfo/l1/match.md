# L1 设计 — 同城 / 扩列墙(match)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-20 |
| 覆盖模块 | `internal/match/service.go`、`handler.go` |
| 关联 | L0 `l0/architecture.md` · L1 `l1/chat-ws.md`(开聊入口)· L1 `l1/user-auth.md` |

---

> **多租户(已实现)**:`CityUsers/ExpandWall` 均加 `tenant_id` 过滤,只返回本租户用户。见 `l1/tenant-saas.md`。

## 一、职责

提供两类"找人"列表(区别于 `bottle.feed` 的"找内容"):

| 接口 | 用途 | 排序 |
|---|---|---|
| `GET /city/users` | 同城用户列表 | `last_active_at desc` |
| `GET /expand/wall` | 扩列墙(活跃/新人/附近) | 见下 |

均返回精简卡片 `UserCard`,前端点"开聊"走 `chat.StartChat`(扣金币,见 `l1/chat-ws.md` / `l1/pay-wallet.md`)。

## 二、UserCard(列表展示模型)

```
user_id(string) · nickname · avatar · gender · age · city · is_verified · online_hint
online_hint:最近活跃 < 1h → "刚刚在线";否则 → "注册 N 天"
```

> `user_id` 字符串序列化,前端拿去开聊不丢精度(见 `l4/cross-platform-api-patterns.md`)。

## 三、同城列表(CityUsers)

```sql
user_id <> :self AND status = 'active'
[AND city = :city] [AND gender = :gender]
ORDER BY last_active_at DESC
LIMIT :size OFFSET (:page-1)*:size      -- size 上限 50
```

筛选:城市、性别(0 全部 / 1 男 / 2 女)可选。

## 四、扩列墙(ExpandWall,三栏)

```
type=active(默认) → ORDER BY last_active_at DESC     -- 活跃
type=new          → ORDER BY created_at DESC          -- 新人
type=nearby       → [AND city=:city] ORDER BY last_active_at DESC  -- 附近(同城)
通用过滤:user_id<>self, status=active, [gender]
```

顶替了产品文档里的"发现/动态广场",作为 V1 的轻量发现入口。

## 五、与其他模块的边界

- **match 不扣费**:只产列表;扣费发生在用户从列表点开聊(`chat`)或解锁(`bottle`)。
- **match 不做推荐打分**:同城/扩列是"按维度排序的用户列表",与 `bottle.feed` 的瓶子打分是两条路径,别混。

## 六、已知缺口 / 待办

- **未过滤拉黑关系**:CityUsers/ExpandWall 目前只排除自己,未排除已拉黑/被拉黑的用户 → 待办:join `block` 表过滤(开聊时 `chat.StartChat` 仍有 `IsBlocked` 兜底拦截,但列表里仍会出现,体验待优化)。
- **无分页游标**:用 `OFFSET` 翻页,大数据量下深翻页性能衰减 → 后续可改 `last_active_at` 游标。
- **无地理距离**:`nearby` 用 city 字段近似,非真实经纬度距离 → V2 可接 LBS。
