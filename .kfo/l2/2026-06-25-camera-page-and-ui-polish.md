# 相机/水印页落地 · 推送测试 · 导航默认隐藏 · UI 打磨

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-25 |
| 状态 | COMPLETED + DEPLOYED（后端+admin 已部署；小程序待上传发版）|
| 触发 | 相机页两个框点击全程无响应（多轮未解）；推送测试报错；导航项接口返回前闪现；UI 图标统一 |
| 关联 | L3 `l3/2026-06-21-mp-tap-not-binding.md`（同一根因复发）· L1 `l1/mp-permissions.md` · L1 `l1/admin-platform.md` · L4 `l4/cross-platform-api-patterns.md`（MP-1/OPS-1）|

---

## 一、相机/水印页（privacy.vue）从不可点到可用

### 1. 真因：`@tap` 裸方法名复发（与 L3 同根因）
新建的 `privacy.vue` 把 `@tap="pickAdd"` / `@tap="pickRemove"` 写成裸方法名，在本 uni-app alpha 的 mp 运行时**编译后不调用**（`($event)=>$options.pickAdd` 缺括号）。

- 系统化调试定位：在按钮上加 `@touchstart/@touchmove/@touchend/@tap` 全套埋点 + 纯 `<view @tap="dbg('x')">` 对照块。
- 决定性证据：`touchstart`/`touchend` 都触发，`pickAdd` 不触发；同页带参 `dbg('PLAIN-VIEW-TAP')` 正常 → 锁定"裸引用坏、调用表达式 OK"。
- 编译产物铁证：`privacy.js` 中 `g: o(($event)=>$options.pickAdd)`（不调用）vs `f: o(($event)=>$options.doSave(...))`（调用）。
- **修复**：`@tap="pickAdd()"` / `@tap="pickRemove()"`。
- ⚠️ 教训：之前误判为 `<button>` 元素问题、tab-bar 重渲染打断手势、跳转动画挂起 JS——全是猜测，绕了多轮。应**第一时间看编译产物 wxml/js**，而非盯源码。

### 2. 构建链路坑（"改什么都没反应"的另一半）
微信工具跑的是 `dist/dev/mp-weixin`，"普通编译"只重打包 dist，**不跑 vite**。必须 `npm run dev:mp-weixin` 监听进程常驻才会把源码编进 dist。该进程停了 → 源码改动永远进不去。`manifest.json` / `pages.json` 等构建期配置改动需**重启** watch 才生效。

### 3. chooseMedia 隐私拦截（errno 112）
`chooseMedia` 报 `api scope is not declared in the privacy agreement`。因 `manifest.json` `__usePrivacyCheck__: true` 开启强制校验，相册接口未在 MP 后台「用户隐私保护指引」声明。
- 临时方案（B）：`__usePrivacyCheck__: false` 绕过，先跑通拍照→水印→保存流程。
- ⚠️ **上线必须改回 `true`**，并在后台声明「相册（含外部相册）」，否则审核不过 / 线上 43101。

### 4. 点击区域 & canvas
- `<canvas>` 改 `v-if="left.state==='loading'"`，仅绘图时挂载，避免原生组件常驻拦截。
- 交互最终态：**上半部 banner 触发选图/拍照**，下半 foot 不触发，「保存」独立 `@tap.stop`（解决保存双触发——foot 带 @tap 时 `.stop` 在该版本挡不住冒泡）。

### 5. 「权限」Tab → 「相机」+ 设为默认页
- 显示名：`tab-bar.vue` `text: '相机'`；后台标签 `meta.go` `"相机 Tab"`。
- 默认启动页：`pages.json` 把 `pages/privacy/privacy` 移到 `pages[0]`（uni-app 启动页 = 数组第一项）。
- `key:'privacy'` 与路由不变（内部标识，改动会破坏既有配置/缓存）。

---

## 二、导航项「默认隐藏，接口返回后再显示」

避免接口返回前闪现不该显示的入口：
- `tab-bar.vue`：默认 `visibility` 全 `false`（原 privacy 默认 true）；删掉历史遗留的 `setTimeout(fetchVisibility, 800)`（那是 L3 误判"重渲染打断 tap"时加的，真因已确认无关），改为 `created` 内立即拉 `/tabs`。
- `mine.vue`：`FN_DEFAULT` 11 项全 `false`，等 `/mine-functions` 返回再 `{...FN_DEFAULT, ...f}`。接口未返回的 key 也默认隐藏（安全兜底）。

---

## 三、新增「我的动态」后台开关

`fn.moments` 之前只有 `FN_DEFAULT` 兜底 `true`，后台无对应开关、不可控。补全：
- `sysconfig.go`：`KeyFnMoments = "fn_show_moments"`，默认 `"1"`。
- `sysconfig/handler.go` `getMineFunctions`：加 `"moments": show(KeyFnMoments)`。
- `admin/meta.go`：加 `{KeyFnMoments, "我的动态", "我的功能", bool}`。
- 客户端 `mine.vue` 已读 `fn.moments`，无需改。

---

## 四、推送测试方法（答疑，无新代码）

管理后台用户管理「推送」走 `push.SendDirect`（用 `wx_openid` 直发，同步返回结果）。
- 微信一次性订阅消息规则：**用户每点一次「允许」= 可发 1 条**。
- 报错对照：`43101` 用户没订阅/额度用完；`47003` 模板字段不匹配（须 `thing1/2/3`，签到 `thing1/2/date_time1`）；`40003` openid 无效。
- 退出小程序也能收到——订阅消息进微信「服务通知」，不依赖小程序开着。
- 测试闭环：后台填模板 ID → 目标用户首页订阅框点「允许」→ 退出 → 后台推送 → 服务通知收到。
- 注：`push.js` 有 `push_subscribed_date` 每日节流，反复测需清该 storage。

---

## 五、UI 打磨

| 页面 | 文件 | 修改 |
|---|---|---|
| 详情页打招呼区 | `detail.vue` | 分享图标 `⤴` → `fenxiang.png`；去掉「回应一下」的 `liwu.png` 图标（删孤儿样式 `.gi`）|
| 同城列表 | `city.vue` | 金币图标 `.coin-city` 26→52rpx（翻倍）；开聊按钮 `.chatbtn` 加 `min-width:200rpx`+`justify-content:center`（约 +50% 宽）|
| 首页邀请胶囊 | `ocean.vue` | `🎁` emoji → `liwu.png`（`.gift` 由 `font-size` 改 `width/height`）|
| 捞瓶弹框 | `ocean.vue` | 收藏 `★/☆` → `shoucang.png`、转发 `⤴` → `fenxiang.png`（转发保留 `open-type="share"`，收藏选中态加浅黄底）|

---

## 六、部署

- 后端：交叉编译 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` → `node deploy.js`（systemd 重启，`active`）。
- admin：`npm run build` → `node deploy_admin.js`（`/message-admin/` → 200）。
- 小程序端改动不走服务器，需微信工具「上传」发版（注意发版前 `__usePrivacyCheck__` 改回 `true`）。
