# 「我的」功能落地 + 聊天本地删会话

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-22 |
| 状态 | COMPLETED |
| 触发 | 按 `docs/MINE_FEATURES_PLAN.md` 批次A/B + feature-roadmap #9 落地 |
| 关联 | L1 `l1/pay-wallet.md` · `l1/chat-ws.md` · `l1/match.md` |

---

## 本次落地范围

### 1. 道具商城页（批次A）
- 路由：`pages/items/items.vue`（已注册于 `pages.json`）
- API：`GET /item/list` + `POST /item/buy`（后端早已就绪）
- 功能：余额展示 + 3栏 grid 道具卡片 + 购买确认弹框 + 金币不足引导充值

### 2. 黑名单列表+解除（批次B）
- 路由：`pages/blocklist/blocklist.vue`（已注册）
- API：`GET /block/list` + `POST /block/remove`（后端早已就绪）
- 功能：已拉黑用户列表 + "解除" → 确认弹框 → 本地移除

### 3. 浏览记录（批次B）
**后端改动：**
- `internal/relation/service.go`：新增 `ViewedCard` struct + `ViewedByMe(userID, page, size)` — 两步查询（先按 `last_interaction_at desc` 取 ID 和时间，再批量 JOIN User 表）
- `internal/relation/handler.go`：注册 `GET /relation/i-viewed` → `h.iViewed`
- `client/src/api/index.js`：`relationApi.iViewedList(params)`

**前端改动：**
- 新建 `pages/viewed/viewed.vue`（时间格式化：刚刚/N分钟前/N小时前/日期）
- `pages.json` 注册 `pages/viewed/viewed`
- `pages/mine/mine.vue`：`soon('浏览记录')` → `goViewed()` 跳转

### 4. 聊天本地删会话（#9）
- `pages/message/message.vue`：
  - `hiddenChats:Set<string>` 存 `uni.storage('hiddenChats')`
  - `visibleChats` computed 属性过滤
  - 聊天行加 `@longpress="showChatMenu(c)"` → ActionSheet → "删除会话" → 写 storage
  - WS 收新消息时从 hidden 集合移除对应 chat_id（自动恢复）

---

## 未改动的既有接口

- `GET /block/list`、`POST /block/remove`、`GET /item/list`、`POST /item/buy` — 后端零改动
- `pages.json` 中 `items/items`、`blocklist/blocklist` — 早已注册

---

## 验证要点

1. 道具商城：点击道具 → 弹框 → 购买 → 余额刷新 → 成功 Toast
2. 道具商城：余额不足 → 跳转充值弹框
3. 黑名单：列表展示 → 解除 → 本地从列表移除
4. 浏览记录：显示"我看过谁"，按时间降序，时间格式正确
5. 聊天删会话：长按 → "删除会话" → 从列表消失 → 收新消息 → 会话重新出现

---

## 待续（批次C）

- **我的收藏**：需新建 `collection` 表 + 接口 + 前端（较重，独立里程碑）
- **我的动态**：需新建 `moment` 表 + 完整模块（见 feature-roadmap #5）
