# Admin 全链路租户改造

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-24 |
| 状态 | COMPLETED |
| 触发 | 所有参数应跟着 tenant 走；后台管理要有租户选择器 |
| 关联 | L1 `l1/admin-platform.md` · `l1/tenant-saas.md` |

---

## 背景

`defaultTenantID()` 按主键顺序取第一个活跃租户（tenant 1），Admin 前端不传 tenant_id，导致所有写操作默认落到 tenant 1。小程序用户属于 tenant 100，造成机器人对话创建到错误租户、人格/关键字规则/回复缓存全部混入 tenant 1 等系统性问题。

---

## 后端改造

### 新增 `tenantFromCtx(c *gin.Context) int64`

```go
// server/internal/admin/handler.go
func tenantFromCtx(c *gin.Context) int64 {
    if h := c.GetHeader("X-Tenant-ID"); h != "" {
        if id, err := strconv.ParseInt(h, 10, 64); err == nil && id > 0 {
            return id
        }
    }
    if q := c.Query("tenant_id"); q != "" {
        if id, err := strconv.ParseInt(q, 10, 64); err == nil {
            return id
        }
    }
    return 0 // 0 = 全部租户（仅读操作有效）
}
```

- header 优先，query 兜底，0 = 全部租户
- 所有 admin handler 统一调用此函数，不再调 `defaultTenantID()`
- 写操作（POST/PUT）若 tenantID=0，再 fallback `defaultTenantID()`

### Service 层 LIST 方法改为条件过滤

```go
// tenantID=0 时不加 WHERE → 返回全部（全部租户视图）
// tenantID>0 时加 WHERE tenant_id=? → 按租户过滤
```

受影响方法：`ListRobotContent`、`ListPersonas`、`ListRobotProfiles`（raw SQL）、`ListKeywordRules`、`ListReplyCache`、`ListRobotChats`（raw SQL）。

### RobotChatMessages / SendAsRobot 签名精简

两个方法移除了 `tenantID` 参数（操作按 chatID 查找，tenant 从 chat 记录读取）：

```go
// before
func (s *Service) RobotChatMessages(tenantID, chatID int64, limit int) ...
func (s *Service) SendAsRobot(tenantID, chatID int64, content string) ...

// after
func (s *Service) RobotChatMessages(chatID int64, limit int) ...
func (s *Service) SendAsRobot(chatID int64, content string) ...
```

### Robot Chats SQL 条件过滤

```go
tenantClause := ""
filterArgs := []interface{}{}
if tenantID > 0 {
    tenantClause = " AND c.tenant_id = ?"
    filterArgs = append(filterArgs, tenantID)
}
// listSQL 和 countSQL 均用 filterArgs... 展开
```

---

## 前端改造（admin/）

### 新文件 `admin/src/tenant.js`

```js
import { reactive } from 'vue'

export const tenantStore = reactive({
  currentTenantID: parseInt(localStorage.getItem('admin_tenant_id') || '0', 10),
  tenants: [],
  setTenant(id) { this.currentTenantID = id; localStorage.setItem('admin_tenant_id', String(id)) },
  async load(listTenantsFunc) {
    const list = await listTenantsFunc()
    this.tenants = list || []
    if (!this.currentTenantID && this.tenants.length > 0) this.setTenant(this.tenants[0].tenant_id)
  }
})
```

### `api.js` 注入 `X-Tenant-ID`

```js
import { tenantStore } from './tenant.js'

async function req(method, path, body) {
  const headers = { ... }
  if (tenantStore.currentTenantID) headers['X-Tenant-ID'] = String(tenantStore.currentTenantID)
  ...
}
```

单处修改覆盖所有 API 调用，各 View 无需改动。

### `Layout.vue` 顶栏租户选择器

```html
<header class="top">
  <span>{{ title }}</span>
  <div class="tenant-wrap">
    <select :value="currentTenantID" @change="onTenantChange">
      <option value="0">全部租户</option>
      <option v-for="t in tenants" :key="t.tenant_id" :value="t.tenant_id">
        {{ t.name }} ({{ t.tenant_id }})
      </option>
    </select>
  </div>
</header>
```

切换租户时调 `tenantStore.setTenant(id)` + `router.go(0)` 整页刷新，当前租户持久化到 localStorage。

---

## 行为约定

| 操作类型 | tenantID=0 (全部) | tenantID>0 |
|---|---|---|
| 列表读取 | 返回所有租户数据 | 仅返回该租户数据 |
| 写入 (POST/PUT) | fallback defaultTenantID() | 写入指定租户 |

`defaultTenantID()` 保留但降级为只读 fallback，不在写路径主流程使用。

---

## 注意事项

- `listUsers` handler 直接读 `c.Query("tenant_id")`，不走 `tenantFromCtx`，与现有行为兼容
- 租户选择器默认选第一个返回的租户（非 tenant 1，因为 API 按 tenant_id ASC 返回）
- `startRobotChat` 用 `GetUserTenant(userID)` 而非选择器，确保聊天建在目标用户所属租户
