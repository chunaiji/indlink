# 次数用完文案可配 + 机器人防复读 + 情感基调 — 设计文档

- 日期：2026-07-05
- 分支：main
- 范围：3 项
  1. #1 首页「今日次数用完」弹框文案改后台可配
  2. #2 机器人对相同输入回复完全相同（连续复读，暴露机器人）——修复
  3. #3 机器人人格基调偏「情感/暧昧」——**暧昧不露骨**（明确禁止露骨性内容）

## 0. 现状（已核实）

- **#1**：`client/src/pages/ocean/ocean.vue quotaExceeded()` 用 `uni.showModal` 写死标题/内容。
- **#2**：`server/internal/robot/bot_worker.go handle()`：Tier1 关键字、Tier2 缓存(`cacheKey(用户消息, 人格角色)`)命中即发；同会话重复发同一句时命中同一条 → 复读，且缓存回复可能带旧上下文、二次不连贯。
- **#3**：`server/internal/robot/promptbuilder.go buildPrompt()` 已有身份/风格/硬性禁止，无情感基调导向。

## 1. #1 次数用完文案可配

- 新配置键（`sysconfig` + admin meta，并入 `user.clientConfig`）：
  - `KeyUITextQuotaTitle = "ui_text_quota_title"`，默认 `"次数已用完"`
  - `KeyUITextQuotaMsg = "ui_text_quota_msg"`，默认 `"可前往道具商城购买次数包继续,或明天免费次数恢复。"`
- 客户端 `store/user.js applyClientConfig`：`uiText.quotaTitle` / `uiText.quotaMsg`（默认同上）。
- `ocean.vue quotaExceeded(action)`：标题/内容读 `useUserStore().uiText.quotaTitle/quotaMsg`；保留「去购买」跳道具页。
  - 注：现文案含"扔瓶/捞瓶"动态词;改为通用文案（不区分动作），简化。

## 2. #2 机器人防复读

- 目标：**同一会话不连续发送与上一条机器人消息相同的内容**。
- `bot_worker.go`：
  - 新增 `func (p *BotWorkerPool) lastBotMessage(chatID, botUserID int64) string`：查该会话最近一条由该机器人发送的消息内容（`messages` 表 `chat_id=? AND sender_id=?` order by `created_at desc` limit 1）。
  - `handle()` 开头取 `lastBot := p.lastBotMessage(job.ChatID, job.BotUserID)`。
  - Tier1 关键字命中：仅当 `reply != lastBot` 才发；否则**穿透到下一层**。
  - Tier2 缓存命中：仅当 `cached != lastBot`（且非 AI 自白）才发；否则穿透到 Tier3。
  - Tier3 LLM：始终重新生成（温度带来差异），作为兜底天然避免复读。
- 效果：第二次发"继续聊"时，缓存候选==上一条 → 穿透到 LLM 生成不同回复。仅防"连续重复"，非全时段去重（YAGNI）。

## 3. #3 情感基调（暧昧不露骨）

- `promptbuilder.go buildPrompt()`：在"请根据以上人格…"之后、多轮钩子之前，加入情感陪伴基调：
  ```
  你是情感陪伴角色:多给情绪价值,主动关心对方感受,可以温柔、走心、适度暧昧和撒娇,让人感到被在乎。
  ```
- 在末尾【硬性禁止】追加一条**内容红线**（既是产品合规也是模型护栏）：
  ```
  5. 不涉及露骨性行为描写、色情、低俗或违法内容;暧昧可以,但保持得体不越界。
  ```
- 说明：只调基调 + 加红线，不引入配置开关（YAGNI）；`familiarityHint` 高亲密度仍"克制不过度"，与红线一致。

## 4. 涉及文件

- `server/internal/sysconfig/sysconfig.go`（#1 两键 + defaults）
- `server/internal/admin/meta.go`（#1 两 ConfigField）
- `server/internal/user/handler.go`（#1 clientConfig 加两字段）
- `client/src/store/user.js`（#1 applyClientConfig + uiText 默认）
- `client/src/pages/ocean/ocean.vue`（#1 quotaExceeded 读配置）
- `server/internal/robot/bot_worker.go`（#2 lastBotMessage + handle 防复读）
- `server/internal/robot/promptbuilder.go`（#3 情感基调 + 红线）

## 5. 测试与验证

- 后端：`go build/vet/test ./...`；`clientConfig` 单测扩展（含 quota 键）；`lastBotMessage`/防复读逻辑若可纯函数化则单测（穿透判断），否则 build + 生产验证。
- 客户端（uni-app 无自动化）：审查 + 人工——次数用完弹框显示后台文案;机器人连发相同输入不再复读;回复更走心暧昧但不露骨。
- 部署：后端 deploy.js；客户端(ocean.vue/user.js)由 HBuilderX 人工上传。

## 6. 非目标（YAGNI）

- 不做机器人回复的全时段去重（只防连续重复）。
- 不做暧昧程度的配置开关。
- 露骨性内容：**明确不实现**（微信合规 + 伦理红线）。
