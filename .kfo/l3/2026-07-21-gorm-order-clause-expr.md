# GORM Order 静默忽略 clause.Expr (gorm-order-clause-expr)

> 2026-07-21 · 后台/GORM · L3

## 现象

用户列表多级排序里"付费用户优先"和"在线优先"两级不生效——付费用户没排到最前,列表只按 `is_robot ASC + balance DESC` 排。编译通过、无报错、无 panic,排序悄悄少了两级。

## 根因

排序用了 `db.Order(clause.Expr{SQL: "(...) DESC", Vars: [...]})` 做参数化排序。但 **GORM v2 的 `Order(value interface{})` 只处理 `string` 和 `clause.OrderByColumn` 两种类型**,`clause.Expr` 落到 default 分支被直接丢弃——不进 ORDER BY,也不报错。

## 解决

改用字符串形式的 Order(本场景值均安全,无注入):

```go
// 在线: user_id 是纯数字, fmt.Sprintf 拼进 IN
if len(onlineIDs) > 0 {
    q = q.Order("(users.user_id IN (" + strings.Join(idStrs, ",") + ")) DESC")
}
// 付费: 标签是常量
q = q.Order("(users.tags LIKE '%" + model.TagPaidUser + "%') DESC")
```

## 预防

- 需要动态/参数化 ORDER BY 时,用 `string` 拼接(确认无注入:纯数字/常量安全)或 `clause.OrderByColumn{Column: clause.Column{Name:..., Raw:true}}`,**不要传 `clause.Expr` 给 `.Order()`**。
- **排序/过滤类改动必须实调接口验证结果**,`go build` 通过不代表排序生效。本轮靠签临时 admin JWT 实调线上接口才发现。

## 复用

同类"静默失效"坑: 见 [l3/reply-cache-poisoning](2026-06-24-reply-cache-poisoning.md)(污染入库无报错)。教训一致——**无报错 ≠ 行为正确,关键逻辑要端到端验证**。
