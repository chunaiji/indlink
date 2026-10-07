# L4 Knowledge Entry — 跨端小程序 + JS 客户端 API 规律

- **Source L3s:**
  - `l3/2026-06-20-int64-id-js-precision.md`(int64 ID JS 精度丢失)
  - `l3/2026-06-21-mp-tap-not-binding.md`(mp 端裸事件不触发)
- **关联 L1/L2:** `l1/pay-wallet.md` · `l1/ui-design-system.md` · `l2/2026-06-20-driftbottle-v1-and-api-tests.md` · `l2/2026-06-21-ocean-experience-redo.md`
- **Pattern Type:** RULE + ANTI-PATTERN
- **L1 Impact: YES → `l1/pay-wallet.md`(int64 已回灌 `json:",string"`)· `l1/ui-design-system.md`(MP 模板事件/emoji 规则已回灌)**

---

## 规则 1:面向 JS 客户端的 API,int64 ID 必须字符串序列化

**描述:** 雪花/数据库自增等 int64 ID 常 >2^53,JS(IEEE-754 double)解析为 number 时静默丢精度。小程序前端、Postman、浏览器都是 JS。
**规则:** 后端响应中所有 int64 ID **以字符串序列化**(Go `json:",string"`);接受 ID 的请求字段同样接受字符串;数组形式的 ID 列表返回 `[]string`。
**例外:** 小整数(枚举/小于 2^53 的自增小表如 package_id/item_id)可保留 number,但不一致性会埋坑,统一字符串更稳。
**验证:** 漂流瓶 2026-06-20 改为 ID-as-string 后,核心链路 40 请求 / 128 断言全过,前后端 ID 不再错乱。

## 反模式 1:用 JSON.parse 在测试/客户端解析含大整数 ID 的响应

**描述:** `JSON.parse('{"id": 326549707776069632}')` 得到 `326549707776069600`(末位被改),**不报错**。用改过的 ID 再请求 → "资源不存在",极易误判为后端 bug。
**根因:** number 类型精度上限 2^53;舍入静默发生。
**规则:** ① 后端从源头 ID-as-string(治本);② 测试脚本若必须从原始报文取大整数,用正则/字符串提取,勿 `JSON.parse` 后取值。

## 规则 2:平台异步回调入账必须幂等 + 验签 + 金额校验

**描述:** 支付平台会重试回调;加币逻辑若不幂等 → 重复入账(资损)。
**规则:** ① 加币**只在平台异步回调内**发生,前端"支付成功"仅 UI 提示;② 以商户单号(`order_no`)做幂等(已 paid 直接成功 / status 乐观更新 RowsAffected 判定);③ 验签(平台公钥)+ 回传金额与订单二次校验;④ 入账与 order 状态置位放同一 DB 事务。
**验证:** 重复回调测试余额不变(110→110)。

## 规则 3:跨端差异锁在单一适配层,业务零感知

**描述:** uni-app 双端(微信/支付宝)的登录、支付、系统信息 API 不同(`wx.*` vs `my.*`)。
**规则:** 用条件编译(`#ifdef MP-WEIXIN/MP-ALIPAY`)把差异封装在 `utils/platform.js` 等单一文件,业务代码只调封装函数,**永不出现 `wx.`/`my.`**。平台合规开关(如 iOS 充值)走后台 config,不发版。

## 规则 4:性能类断言对环境敏感,远程库延迟登记为 flaky

**描述:** 接口测试的"响应时间 < Ns"断言,在 dev 连**远程跨网数据库**时,写操作(建用户/下单)首次冷写偶发超阈值 → 间歇红,但功能断言(code/字段)始终过。
**规则:** ① 性能类断言与功能断言分离看待;② 远程库导致的超时登记 `known-flaky.md`(文件夹级 + 原因 + 复查日期),不掩盖、不删断言;③ 性能基线应在本地/同机房库上跑才有意义。
**例证:** 漂流瓶 2026-06-20,登录建用户对远程库偶发 2.3s>2s;切本地库 <50ms 不复现。

## 反模式 2:钱的余额以缓存为真相源

**描述:** 把金币余额放 Redis 当权威值 → 缓存与库不一致时资损。
**规则:** 余额真相源 = MySQL,扣减走 `SELECT ... FOR UPDATE` 行锁事务;Redis 只缓存只读展示。

## 模式 MP-1(RULE):uni-app vue3 + mp 端,模板事件一律用调用表达式,禁止裸方法名

**描述:** 当前 uni-app vue3 alpha 在 mp-weixin 端,`@tap="contact"`(裸方法名)编译出的绑定运行时**不触发**;`@tap="contact($event)"` / `@tap="soon('x')"`(调用表达式)正常。表现为"按钮点了没反应",而带参数的处理器却好用 —— 极易误判为遮挡/缓存/框架损坏(实际排查见 L3)。
**规则:** ① 所有 `@tap/@click/@change/@confirm/@longpress/@input` **必须写成调用表达式**,无参也要 `fn($event)`;② 需要事件对象的(如 `@change`)用 `$event` 传入;③ 新增/审阅模板时把"裸事件"当 lint 红线。
**反向教训:** 不要凭直觉认为"inline 表达式不稳、改成裸方法引用更安全" —— 此 alpha 恰恰相反。
**例证:** 2026-06-21,全站 36 处裸事件批量加 `($event)` 后所有按钮恢复;`soon('浏览记录')` 当时是唯一能触发的,正是它指向了真因。
**复发(2026-06-25):** 新建 `privacy.vue` 又写裸 `@tap="pickAdd"`,同症复发——批量替换只治存量,**新代码会再犯**,须 lint/review 卡点。排查应直接看 `dist/.../xxx.js` 的 `o(...)` 绑定有无 `()`,别盯源码猜。

## 模式 MP-2(RULE):mp 端只用老码点 emoji + 静态资源 ASCII 文件名

**描述:** ① Emoji 12+(2019 后,如 🪙 U+1FA99、🪣 U+1FAA3)在微信旧 WebView 渲染成**豆腐块/方框**;② 中文文件名静态图(`左.png`)在 mp 端图片路径易失效。
**规则:** ① UI 里只用 Emoji ≤6.0 的老码点(🌊💬❤️🔒🎁🍾🎣🚀💖📎💌 等)做装饰,涉及"币/价格"等用纯文本或 CSS 图标,避免新码点;② 拷入 `static/` 的素材一律改 ASCII 名(`bottle-l.png`)。
**例证:** 2026-06-21 清理 `🪙×6`(→"金币/币")、`🪣×1`(→🍶);`左/右.png` → `bottle-l/r.png`。

## 模式 OPS-1(RULE):改 Go 路由/代码后必须重启服务(`go run` 不热重载)

**描述:** 新增路由后接口仍 404,因联调跑的是**旧二进制**;`go run`/已编译 binary 都不会热加载源码改动。
**规则:** 后端改动 → 杀旧进程 → 重新 `go run ./cmd/api`;404(非连接拒绝)优先怀疑"服务没重启"而非路由写错。验证用无 token 请求看是否返回 401(路由在)而非 404(路由缺)。
**例证:** 2026-06-21 `/api/block/list` 404 → 重启后返回 401(已注册),功能恢复。

## 模式 OPS-2(RULE):避开 SQL 保留字做列名,或用 gorm `column:` 改名

**描述:** GORM 模型字段 `Read bool` → 列名 `read`,而 `read` 是 MySQL 保留字。原始 `db.Where("read = ?", false)` 不会自动加反引号 → 静默报错,`Count` 吞错返回 0(红点永远 0,极难察觉)。
**规则:** ① 布尔/状态字段避开保留字(`read/order/status/key/desc/group` 等),命名 `is_read/is_xxx`;② 必须用保留字时,模型加 `gorm:"column:is_read"`,且**所有原始 SQL 串里同步用新列名**;③ `Count`/`Find` 的 error 不要默默忽略,联调期至少 log。
**例证:** 2026-06-21 通知未读数恒为 0 → 根因 `read` 保留字 → 改列 `is_read` 后正常(1→0)。

## 模式 OPS-3(RULE):mp 端调试,改源码必须保证 vite watch 在跑;工具跑的是 dist 不是源码

**描述:** 微信开发者工具加载的是 uni-app 编译产物 `dist/dev/mp-weixin`,工具的"普通编译"只重打包 dist,**不跑 vite**。若 `npm run dev:mp-weixin` 监听进程没开/已停,改 `.vue` 源码永远进不了 dist,表现为"改什么都没反应",极易与代码 bug 混淆。`manifest.json`/`pages.json` 等构建期配置改动还需**重启** watch 才生效(增量监听不覆盖)。
**规则:** ① 调试 mp 前先确认 `npm run dev:mp-weixin` 常驻运行(输出 `Watching for changes...`);② "改了没反应"先核对 `dist/.../xxx.wxml|js` 是否含新改动,再怀疑代码;③ 改 manifest/pages.json 后重启 watch;④ 与 OPS-1(Go 重启)同源——"产物不是最新"是 mp+Go 联调最常见的伪 bug。
**例证:** 2026-06-25 相机页"改 view、加括号都没反应",根因之一是 watch 停了,dist 仍是旧 `<button>`/裸 `@tap`;重启 watch 后产物才更新。
