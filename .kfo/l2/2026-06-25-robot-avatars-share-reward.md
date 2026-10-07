# 机器人头像按性别 · 分享奖励告知 · 流水文案统一

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-25 |
| 状态 | COMPLETED（后端已部署+迁移已执行；小程序待上传）|
| 触发 | 机器人男女头像混用；分享有奖励但用户看不到告知；流水项目名中英混杂 |
| 关联 | L1 `l1/robot.md` · L1 `l1/pay-wallet.md` · 同日 `l2/2026-06-25-camera-page-and-ui-polish.md` |

---

## 一、机器人头像按性别分池

**问题**：头像池单一（`uploads/robot/`，全是 beauty 女性图），男机器人也分到女头像。

**方案**：
- 新增男性头像目录 `server/header_image_man`（430 张），部署到服务器 `uploads/robot_man/`，URL `…/static/robot_man/`。
- `robot/avatars.go`：拆双池 `avatarPoolFemale`(robot/) / `avatarPoolMale`(robot_man/)，`randomAvatarByGender(gender)`（1=男 2=女，对应池空时回退另一池）。
- `robot/service.go`：新建机器人 `Avatar: randomAvatarByGender(gender)`；空头像回填也按性别。
- `deploy.js`：第 5 步加传 `header_image_man → uploads/robot_man`（`uploadDirIfNeeded` 按数量幂等跳过）。

**存量迁移（幂等）**：`ReassignAvatarsByGender(tenantID)` —— 头像 URL 已含对应性别目录片段（男 `/static/robot_man/`、女 `/static/robot/`）则跳过，否则按性别重刷。在 `scheduler.go` 启动 10s 后对每租户跑一次，因幂等故每次重启安全（只修历史混用）。

**部署验证**：男头像 HTTP 200；DB 分布 `man_pool gender=1 → 112`、`woman_pool gender=2 → 98`，零错配。

---

## 二、分享奖励缺少告知

**根因**：`claimShareReward()` 在 `onShareAppMessage` 触发瞬间弹 `uni.showToast`（1.5s, icon none），而此刻微信分享面板正好弹出盖住 toast，用户回到页面时早已消失 → "看不出来"。微信转发无可靠的"分享完成"回调。

**修复**（`ocean.vue` + `detail.vue`）：
- 领奖结果暂存 `pendingShareReward`，不在领奖瞬间弹。
- `onShow`（回到页面）时 `flushShareReward()` 弹**模态框**（"分享成功 🎉 / 恭喜获得 N 金币奖励"，需点「收下」）。
- 部分机型分享后不触发 onShow，加 `setTimeout(…, 1500)` 兜底。
- `flushShareReward` 先清零再弹，onShow 与延迟兜底互斥，只弹一次。

---

## 三、流水/订单项目名统一中文

**问题**：钱包流水按 `scene` 英文码映射中文，`wallet-log.vue` / `orders.vue` 的 `LABELS` 只覆盖 5 种（recharge/reward/chat/unlock/gift），漏了 `share/checkin/msg` → 这几种直接显示英文码（如 "share"）。

**修复**：两页 `LABELS`/`ICONS` 补全全部 8 种 wallet scene（+`share:分享奖励`、`checkin:签到奖励`、`msg:消息消费`），`sceneLabel` 兜底由 `|| s` 改为 `|| '其他'`，杜绝漏出英文。

---

## 四、零散 UI 调整

| 页面 | 调整 |
|---|---|
| `city.vue` | 金币 `.coin-city` 26→52rpx；`.cost` 字号 19→36rpx + `line-height:1`，数字 `.cost-num` `translateY(-6rpx)` 微上移；开聊按钮 `.chatbtn` `min-width:170rpx`（先加 200 再按需缩 15%）|
| `mine.vue` | 真人认证图标 `yirenzheng.png → yirenzheng_1.png` |
