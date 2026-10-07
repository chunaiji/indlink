# 每日扔/捞次数限制 + 道具购买次数 规划;聊天/我的/弹框 UI 修整

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-23 |
| 状态 | 部分 COMPLETED(UI #1–4 已落地)+ PROPOSED(#5 每日次数限制待实现)|
| 触发 | 用户 5 项:聊天头像左右、我的功能后台可配、首页头像、弹框按钮样式、扔/捞每日限额规划 |
| 关联 | L1 `l1/bottle-feed.md`(捞瓶)· `l1/pay-wallet.md`(钱包/道具)· `l1/admin-platform.md`(配置)|

---

## 已完成(UI)

1. **聊天头像左右**:`mine(m)` 用 `String(sender_id)===String(myId)` 比较(修 int64/字符串类型不一致导致全判为对方),自己消息头像在右、对方在左。
2. **我的-其他功能后台可配**:新增 10 个 `fn_show_*` 配置(sysconfig)+ 公开接口 `GET /sys/mine-functions` + 管理台「我的功能」组开关;`mine.vue` 按返回值 `v-if` 显隐各项。
3. **首页 nav-r 头像**:`:size` 68→88,`margin-top:16rpx`(更大、略下移)。
4. **捞瓶弹框按钮**:`.m-actions` 设宽 600rpx,`换一个` flex:1 / `查看详情` flex:1.7,按钮拉满信纸宽度(对齐参考图)。

---

## #5 每日扔/捞次数限制 + 道具续次(PROPOSED)

### 目标
- 每个用户每天 **扔瓶 / 捞瓶** 各有免费次数上限,管理台可配。
- 次数用完:① 提示明日恢复;② 或**消耗道具/金币**继续(买"次数包"道具)。

### 配置项(sysconfig,管理台「每日限额」组)
| key | 含义 | 默认 |
|---|---|---|
| `quota_throw_daily` | 每日免费扔瓶次数 | 10 |
| `quota_scoop_daily` | 每日免费捞瓶次数 | 20 |
| `quota_throw_item_cost` | 超额扔一次消耗金币(0=禁止超额) | 5 |
| `quota_scoop_item_cost` | 超额捞一次消耗金币 | 2 |

### 计数存储(Redis,按天过期)
- key:`quota:{tenantID}:{userID}:{action}:{yyyymmdd}`,`action ∈ {throw, scoop}`。
- 每次成功扔/捞 `INCR` + 首次 `EXPIRE` 到当天 24:00(或 +24h)。
- 读当前已用:`GET`;剩余 = `max(0, limit - used)`。
- 选 Redis 而非建表:天然按天滚动、零清理、高频读写;与 feed 缓存同一基础设施。

### 后端流程
1. **扔瓶 `Create`**:进函数先查 `used = quota.Get(throw)`;
   - `used < limit` → 放行,成功后 `INCR`。
   - `used >= limit` 且 `quota_throw_item_cost > 0` → 走 `wallet.Debit(cost, scene="quota_throw")` 扣币放行(扣成功才创建,同一事务);余额不足返回 `5001`。
   - `cost == 0` → 返回新错误码 `CodeQuotaExceeded`(如 3003),前端提示明日再来。
2. **捞瓶 `Scoop`** 同理(scene="quota_scoop")。
3. 计数只在**真正成功**后自增(扔瓶落库成功 / 捞瓶返回非空)。

### 道具方案(二选一,建议 B)
- **A. 直接扣金币**:超额按 `*_item_cost` 扣币,最简单,无需新道具。
- **B. 次数包道具**(更符合"买道具"):`item` 加类型 `quota_throw`/`quota_scoop`,购买后进 `item_order`/用户道具库存;超额时优先消耗库存道具,无道具再走扣币或拦截。需在 `item` 模块加"使用道具"接口。
- V1 先做 A(扣币续次),B 作为增强(道具商城已有,加两个道具 + 消耗逻辑即可)。

### 前端
- 海洋页扔/捞前不预判,直接请求;命中 `CodeQuotaExceeded` → 弹窗「今日次数已用完,看广告/买次数包/明日恢复」;命中 `5001`(超额扣币余额不足)→ 引导充值。
- 可选:首页显示「今日剩余 扔 N / 捞 M」(加一个 `GET /bottle/quota` 返回剩余)。

### 关联改动
- `errs` 加 `CodeQuotaExceeded`。
- `internal/quota` 新模块(Redis 计数封装)或并入 `bottle`。
- `bottle.Create/Scoop` 注入 quota 检查 + wallet。
- admin meta + sysconfig key。

### 开放决策(需用户拍板)
1. 超额续次走 **扣金币(A)** 还是 **买次数包道具(B)**?(建议先 A)
2. 免费次数默认值(扔 10 / 捞 20 合适?)
3. 是否在首页显示剩余次数?
