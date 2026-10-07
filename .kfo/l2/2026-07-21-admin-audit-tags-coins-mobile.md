# 后台审核/标签/金币/移动端 + 机器人去重

> 2026-07-21 · L2 · 关联 [l1/admin-platform](../l1/admin-platform.md) [l1/robot](../l1/robot.md) [l1/pay-wallet](../l1/pay-wallet.md)

本轮以后台运营能力 + 机器人真实感为主线,含一条 GORM 排序踩坑(见 [l3](../l3/2026-07-21-gorm-order-clause-expr.md))。

## 1. 用户标签系统 + 充值自动打标签

- `model.User` 加 `Tags string`(逗号分隔, `json:"-"` 不下发 C 端, 后台 `UserRow` 补出)。常量 `model.TagPaidUser="付费用户"`。
- `admin/tags.go`: `sanitizeTags`(trim/去重/单个≤16字/总≤255) + `PUT /admin/users/:id/tags`(错误信息透传前端)。
- **充值自动打标**: `pay.HandleCallback` 入账事务内一条幂等 SQL(`IF/CONCAT` + `NOT LIKE` 去重),跟随订单 pending→paid 幂等。
- 存量付费用户用同款 SQL 批量补标(生产库直接跑)。
- 前端 `Users.vue`: 标签列(金色"付费用户"胶囊)+ 编辑弹框(回车加/×删,保存时收编未回车残留)+ 按标签筛选。

## 2. 机器人重复回复修复

根因: 三层生成(关键字→缓存→LLM)的键只含(消息, personaRole)不含机器人个体; 且缓存命中即返回、只有 Tier3 后才 `AddVariant` → 变体池长期停留 1 条 → 同瓶/跨机器人/跨会话一字不差。

- `replycache.go`: 删旧 `Get`,改 `GetVariants`(过滤 AI 自白)+ `pickVariant(variants, exclude)`; 常量 `cacheRegenProb=40`。
- 变体池主动积累: 命中但变体<5 时 40% 概率穿透 LLM 生成新变体入池。
- 聊天侧(`bot_worker.go`): `lastBotMessage`→`lastBotMessages(chatID,botID,5)`,近 5 条防复读。
- 瓶子侧(`service.go maybeReply`): 同瓶已有回复查重, `genAIText` 加 `exclude` 参数,重复则弃。
- 硬编码/小池文案变体化: bottle→chat opener 5 条、openers 4→8、nudges 3→8。

## 3. 测试用户软删除

`wx_openid LIKE 'wxdev%'` 的 56 个开发期账号 `status='deleted'`。`ListUsers` 默认视图排除 deleted(状态下拉可选"已删除"查看)。软删只改状态,数据保留可恢复。

## 4. 机器人素人化(会聊小纸条租户)

200 个机器人中 150 个改造为仿真实新用户: 昵称 `用户XXXXXX`(仿 `user/service.go` 默认昵称格式, 随机 6 位数)、`is_verified=0`、`avatar=''`; 优先挑无聊天记录的。

- **配套代码改动**: `EnsureRobots` 补空头像仅对 `is_verified=1` 生效; `ReassignAvatarsByGender` 跳过空头像。否则每分钟调度会把清空的头像自动补回(定位见 [l3](../l3/2026-07-21-gorm-order-clause-expr.md) 同期排查思路)。

## 5. 后台多页面移动端适配

统一三招: 宽表格套 `.table-wrap{overflow-x:auto}` + 表格 `min-width`(窄屏横滑不压扁); 弹框 `width: min(px,92vw)`; 工具栏/分页 `flex-wrap`。

- RobotChats 双栏 → 窄屏上下堆叠(media query)。
- Dashboard 图表 grid `minmax(360px)` → `minmax(min(360px,100%),1fr)` 修溢出。
- Credentials 双列卡片 → 自适应单列 + 长密钥换行。Login 卡片 `min(360px,92vw)`。
- 覆盖 8+ 页; Layout 骨架早已有抽屉侧栏,无需改。

## 6. 用户列表在线列 + 多级排序

- `ListUsers` 改 LEFT JOIN wallets, 取内存 Hub 在线集合(`chatSvc.Hub().OnlineUserIDs()`)。
- 排序(最终版): **在线 > 真实用户(is_robot ASC) > 金币倒序 > 最后活跃**。中途曾含"付费用户"级,后按需求去掉。
- `UserRow.Online` 标记; 算出字段(OpenID/Tags/Online)标 `gorm:"-"` 避免 Find 列映射歧义。
- ⚠️ 排序踩坑见 L3(GORM Order 忽略 clause.Expr)。

## 7. 金币流水时间筛选

`ListWalletTxns` 加 `start/end` 参数, 按 `created_at` 过滤(起含当天 00:00:00、止含当天 23:59:59)。前端两个 `<input type="date">`。

## 8. 瓶子管理审核页(新)

- `admin/bottles.go`(service+handler 内聚一个文件): `ListBottles`(join users 带作者/机器人标记, 筛选 关键词/状态/类型) / `ListBottleReplies` / `DeleteBottle`(软删 status=deleted) / `DeleteBottleReply`(硬删 + reply_count-1)。
- 4 路由 `/admin/bottles*`。前端 `Bottles.vue`: 列表内展开(手风琴, 按需拉回应)+ 逐条删除。导航「用户」组。

## 9. 用户列表金币拆列

- `wallets.total_recharged` 实为**历史累计获得**(CreditTx 对所有 credit 累加, 不分 scene, 非仅充值)。
- `UserRow` 加 `TotalRecharged`; 前端"金币"列拆为 **总金币**(total_recharged) + **剩余金币**(balance), 排序按剩余倒序。

## 验证方式

后端改动一律**签临时 admin JWT(HS256+JWT_SECRET, 中间件只验签不查库)实调线上接口**验证结构与排序, 不只靠 `go build`。排序类改动尤其必须实调 —— 本轮 GORM clause.Expr 坑正是这样抓到的。

## 提交

分多个 commit 推 `origin/main`(标签/去重/移动端/在线列排序等)。纯数据操作(改名/清头像/软删/补标)直接生产库执行,不入代码库。
