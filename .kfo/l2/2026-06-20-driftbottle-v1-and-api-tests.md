# 漂流瓶 V1 实现 + 接口测试骨架

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-20 |
| 状态 | COMPLETED(dev 环境对远程库全链路验证通过) |
| 触发 | 参考资料(产品文档 + 10 张截图)→ 设计 → 全栈实现 → 接口测试 |
| 数据来源 | 远程开发库 `<远程开发库>`(MySQL `ai_message` + Redis db2) |
| 关联 | L0 `l0/architecture.md` · L1 `l1/pay-wallet.md` · L3 `l3/2026-06-20-int64-id-js-precision.md` · L4 `l4/cross-platform-api-patterns.md` |
| 实施 | 工作区直改(项目暂未 git 化) |

---

## 一、范围(V1 = 核心 + 同城/扩列)

登录 / 扔瓶·捞瓶 / 回信→聊天(WS) / 消息 / 我的 / 金币充值+扣费 / 认证开关 / 同城列表+开聊扣币 / 扩列墙。**不做**:VIP 订阅、情绪画像/AI 推荐、动态广场、双平台账号合并。

## 二、后端(Go 模块化单体)

- 模块:user / bottle / match / chat / relation / pay / wallet / item / moderation + common/sysconfig。
- 基础设施:Gin + GORM + go-redis + gorilla-websocket;雪花 ID;JWT 鉴权;Redis 限流。
- `go build / vet / test` 全绿;路由注册测试(40+ 路由装配无冲突)。

## 三、前端(uni-app,微信+支付宝双端)

- 9 页面:海洋(首页)/同城/扩列/消息/我的 + 扔瓶/详情/聊天/充值。
- 跨端封装 `utils/platform.js`:登录(`wx.login`/`my.getAuthCode`)、支付(`wx.requestPayment`/`my.tradePay`)、系统信息;业务零感知。
- 白天沙滩 + 夜间星空双主题(CSS 变量);Pinia store;条件编译 `#ifdef MP-WEIXIN/MP-ALIPAY`。

## 四、微信支付 APIv3(真实实现)

`driver_wx.go` + `wxcrypto.go`(纯标准库):下单签名 + paySign + 回调验签(平台公钥)+ AES-GCM 解密 + 防重放。单测覆盖签名/验签往返、AES-GCM 往返、完整回调链路 + 伪造签名拒绝。详见 `l1/pay-wallet.md`。

## 五、接口测试(api-testing skill 产出)

`server/api-tests/`:Postman collection(40 请求 / 128 断言,6 文件夹,每接口 6 类用例矩阵)+ Newman 跑测 + 响应留底 + dev/test/pre 环境。

- 适配 Go:`/auth/login` 取 JWT 代替 OAuth;成功判定 `code===0`;鉴权失败 HTTP401+2001。
- dev mock 登录:`code=apiX_{{runId}}` 每次跑全新用户,余额断言确定。
- **跑测结果:40/40 请求、128 功能断言通过**;覆盖登录→扔/捞瓶→回信/解锁→开聊→充值(含回调幂等)→风控全链路 + 三层断言。
- **已知 flaky**:"响应时间<2s" 断言在**远程跨网开发库**首次写入(建用户+发奖励)偶发 >2s 失败(功能断言始终过)。已登记 `api-tests/known-flaky.md`;切本地/同机房库即消失,非代码缺陷。

## 六、本轮发现并修复的真实 bug

**int64 雪花 ID 在 JS 端精度丢失** —— smoke 测试时 `bottle_id`/`chat_id`/`user_id` 末位被改 → 拿改过的 ID 再查"不存在"。根因与修法见 L3,可复用规律见 L4。修复:后端全 ID `json:",string"`、请求体接受字符串 ID;前端 `api/index.js` 统一 `String()`、`chat.vue` 不再 `Number()` 截断。

## 七、开发环境

`.env` 配远程库;新增 `REDIS_URL` 连接串支持;`cmd/dbinit` 非破坏性连通性校验 + 建库。后端跑 `:8980`,`/health` 正常。

**编译 & 联调命令手册:`docs/DEV_RUNBOOK.md`**(后端运行 / 接口测试 / 前端 HBuilderX·CLI 编译 / 联调地址配置 / 调试排查)。

## 八、本地图片上传(联调补充,2026-06-20 追加)

为编码联调发图,补上传(对象存储为后续方案,此为临时本地服务):

- 后端 `internal/upload`:`POST /api/upload`(鉴权,multipart `file`),类型白名单(jpg/png/gif/webp)+ 10MB 限制,按 `日期/雪花.ext` 落盘 `UploadDir`(默认 `./uploads`)。
- 静态服务:`r.Static("/static", UploadDir)` → `GET /static/<day>/<file>`。
- 返回绝对 URL:配 `PUBLIC_BASE_URL` 用之,留空则按请求 Host 推导(联调友好)。
- 前端:`utils/upload.js`(`uni.uploadFile` 带 token);`throw.vue` 选图→上传→`media_url`→发图瓶(`content_type=image`)。
- **已验证**:上传返回 URL、`GET /static` 回 200 image/png、非图类型拒(1001)、无 token 401。
- 切对象存储时只需替换 `upload` 的落盘 + URL 拼装,接口契约不变。

## 九、待办(后续里程碑)

- 支付宝真实下单 + RSA2 验签(目前骨架)
- 接口测试扩到全部模块 + 接入 CI
- 前端真机双端编译(HBuilderX / CLI)
- 图片由本地 `/static` 切对象存储(OSS/COS)+ CDN
- 上线合规:微信支付/支付宝商户号 + 社交类目资质(企业主体已就绪 ✅)
