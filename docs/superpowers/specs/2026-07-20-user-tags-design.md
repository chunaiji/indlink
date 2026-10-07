# 用户标签系统 + 后台用户列表金币列 设计

日期：2026-07-20
状态：已批准（待实现）

## 需求

1. 管理员可在后台给用户打自定义标签（自由输入）
2. 用户充值成功后，自动打上「付费用户」标签
3. 后台用户列表（/message-admin/#/users）新增「标签」列和「金币」列
4. 支持按标签筛选用户

约束：标签仅后台管理用，C 端不展示、不下发。

## 1. 数据模型

`model.User` 新增字段（AutoMigrate 自动加列）：

```go
Tags string `gorm:"size:255" json:"tags"` // 逗号分隔,后台管理用
```

- 与 `Bottle.Tags` 既有模式一致（逗号分隔字符串，V1 简化）。
- 「付费用户」为系统自动标签，常量 `model.TagPaidUser = "付费用户"`，与手动标签同存此字段，不做类型区分。
- 多租户：标签存于 user 行，天然隔离。

## 2. 充值自动打标签

挂在 `pay/service.go` `HandleCallback` 的入账事务里，`wallet.CreditTx` 成功后执行：

```sql
UPDATE users
SET tags = IF(tags = '' OR tags IS NULL, '付费用户', CONCAT(tags, ',付费用户'))
WHERE user_id = ? AND (tags IS NULL OR tags NOT LIKE '%付费用户%')
```

- 单条 SQL 原子且幂等（已含则不追加）；外层已有订单 pending→paid 幂等保护，不会重复打。
- 行为说明：管理员手动删除「付费用户」后，用户下次充值会自动重新打上（预期行为）。

## 3. 后端接口

### 新增 `PUT /admin/users/:id/tags`

- Body：`{"tags": "付费用户,羊毛党"}`（前端把标签数组 join 成串）
- Handler/Service 照 `banUser`/`muteUser` 现有风格。
- 服务端校验：总长 ≤255，超长报参数错误。

### 修改 `GET /admin/users`

- 新增 `tag` 查询参数：`WHERE tags LIKE '%<tag>%'`（接受子串宽松匹配）。
- `UserRow` 新增 `Balance int64 json:"balance"`（金币余额）：对当前页 user_id 批量查一次 `wallets`（`WHERE user_id IN (...)`）后组装，不 join 全表。

## 4. 管理后台前端（admin/src/views/Users.vue + api.js）

- 表格新增两列：
  - **标签**：每个标签一个胶囊 badge；「付费用户」用金色系区分；无标签显示 —
  - **金币**：余额数字
- 操作列新增「标签」按钮 → 复用现有 modal 模式：
  - 现有标签可点 × 删除
  - 输入框回车添加：单个标签 ≤16 字、禁逗号
  - 保存调 `PUT /users/:id/tags`
  - 机器人行也允许打标签
- 工具栏新增标签筛选输入框（回车/搜索触发）
- `api.js`：新增 `updateUserTags(id, tags)`；`listUsers` params 增加 `tag`

## 5. 测试与验证

- Go 单测（参考 `overview_test.go` 先例）：
  - pay 打标签 SQL 幂等性（两次回调只打一次）
  - `ListUsers` tag 筛选 + 余额组装
- 手动验证：后台打/删标签、按标签筛选、充值后自动出现「付费用户」

## 不做的事（YAGNI）

- 不做标签库管理页
- 不做标签颜色配置
- 不做 C 端展示/下发
- 不做标签统计报表

## 备注

`Wallet.TotalRecharged > 0` 本身可判定付费用户，但标签体系更通用（运营还可打其他标签），两者不冲突。
