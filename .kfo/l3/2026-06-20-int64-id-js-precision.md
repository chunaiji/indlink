# int64 雪花 ID 在 JS 客户端精度丢失

| 字段 | 值 |
|---|---|
| KFO 层级 | L3 — 复盘分析层 |
| 日期 | 2026-06-20 |
| 触发 | 接口 smoke 测试时,用返回的 `bottle_id`/`chat_id` 再查接口,后端回 "不存在/未登录" |
| 数据来源 | curl smoke 链路 + 后端 GIN/GORM 日志 |
| 关联 | L1 `l1/pay-wallet.md` · L2 `l2/2026-06-20-driftbottle-v1-and-api-tests.md` · L4 `l4/cross-platform-api-patterns.md` |

---

## 一、症状

curl 串测核心链路时,登录成功、扔瓶成功(返回 `bottle_id`),但下一步 B 回信报 `{"code":1005,"msg":"瓶子不存在"}`,开聊后发消息报"会话不存在"。表面像后端 bug,但 chat 创建本身又成功了。

## 二、定位过程(三次误判 → 真因)

1. **误判①**:以为瓶子没落库 → 查库,瓶子在,ID 是 `...832801`。
2. **误判②**:以为路由/鉴权问题 → 但 chat start 成功、扣费正确,鉴权没问题。
3. **真因**:对比发现——chat 响应里 `user_a = ...069632`,而测试脚本解析出的 uid 是 `...069600`,**末位被改了**。

雪花 ID 是 19 位 int64(~3.2e17),**超过 JS `Number.MAX_SAFE_INTEGER`(2^53 ≈ 9.0e15,16 位)**。测试脚本用 `JSON.parse` 解析响应,大整数被静默舍入 → 用舍入后的 ID 再查,自然"不存在"。

## 三、根因

后端把 int64 ID 当 **JSON number** 序列化返回。任何 JS 运行时(Postman `pm.response.json()`、浏览器、**uni-app 小程序前端**)用 IEEE-754 double 解析时,>2^53 的整数精度丢失。这不是后端逻辑 bug,是**序列化契约对 JS 客户端不安全**。

> 关键认知:小程序前端本身就是 JS。这个问题在生产里会让前端拿到的所有 ID(瓶子、会话、用户)末位被改,导致点击详情/进会话/开聊全部错乱。是必修的真实 bug,不只是测试脚本问题。

## 四、修复

1. **后端响应**:所有雪花 int64 ID 字段加 `json:",string"`(`model.go` 全量 + 非 model 响应结构 `UserCard`/`ReplyView`;`relation.List` 由 `[]int64` 改 `[]string`)。
2. **后端请求**:接受字符串 ID 的请求体字段同样 `json:",string"`(`target_id` 等);可选 ID(`source_bottle_id`)用 `string` 字段 + 手动 `ParseInt`,避免传 0 时 `,string` 解析报错。
3. **前端**:`api/index.js` 发 ID 一律 `String(id)`、可选 ID 无值则省略;`chat.vue` 把 `Number(q.id)` 改为直接用字符串(WS 比较 `String(msg.chat_id) === this.chatId`)。
4. **路径参数**不受影响(本就是字符串,后端 `strconv.ParseInt`)。

## 五、验证

修后重跑 smoke + Newman:`user_id/bottle_id/chat_id` 全为字符串无截断,核心链路 40 请求 / 128 断言全过;余额数学精确(50−2−5=43,充值 +60=103)。

## 六、教训

- **测试脚本的精度丢失反而帮我们提前暴露了生产 bug** —— 若当时只在 Postman 里点一点(Postman 同样丢精度但人眼不易察觉),很可能漏到上线。
- 凡是"JS 消费 + int64 ID"的系统,序列化契约必须 ID-as-string,这是默认规则而非特例 → 见 `l4/cross-platform-api-patterns.md`。

## 七、复发记录(2026-06-27,admin 端)

多租户上线后新建租户的 `tenant_id` 是雪花 ID(18 位 >2^53)。admin 前端 `tenant.js`/`Layout.vue`/`Config.vue` 用 `parseInt`/`Number()` 处理它 → 丢精度 → 选中值与 `<option :value="t.tenant_id">`(字符串)不匹配 → **config 页租户下拉框显示空白**,且 `X-Tenant-ID` 发出错误值读错租户。旧租户 1/100/200 是小数字,所以一直没暴露。

后端早已 `json:"tenant_id,string"`(字符串契约正确),坑在前端又把它转回 Number。修复:admin 全程按字符串处理 tenant_id。**教训:ID-as-string 契约要贯穿到消费端的每一行,任何一次 `Number()`/`parseInt` 都会把契约破坏掉**;新写的多租户前端代码尤其要把"对 ID 调 Number"当 lint 红线。详见 `l2/2026-06-27-ad-monetization-and-tenant-mgmt.md`。
