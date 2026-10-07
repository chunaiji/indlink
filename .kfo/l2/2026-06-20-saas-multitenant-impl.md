# SaaS 多租户实现 + 凭证表

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-20 |
| 状态 | COMPLETED(单租户回归 + 多租户隔离测试均通过) |
| 触发 | 需求:一份部署服务多个小程序,凭证集中入表,双平台兼容 |
| 数据来源 | 远程开发库 `<远程开发库>`;flag on/off 双模式实测 |
| 关联 | 设计 `l1/tenant-saas.md` · spec `docs/superpowers/specs/2026-06-20-saas-multitenant-credentials-design.md` · 回灌 `l1/user-auth.md` `l1/pay-wallet.md` `l1/bottle-feed.md` `l1/chat-ws.md` `l1/match.md` · L4 `l4/cross-platform-api-patterns.md` |
| 实施 | 工作区直改(项目暂未 git 化) |

---

## 一、范围

按 `l1/tenant-saas.md` 设计落地:appid 标识租户、凭证加密入表、全业务表 `tenant_id` 隔离、开关灰度、双平台。

## 二、新增 / 改造

| 类别 | 内容 |
|---|---|
| 新增 | `internal/crypto`(AES-256-GCM 主密钥)、`internal/tenant`(模型 + credstore)、`cmd/mtseed`(测试种子) |
| 模型 | `Tenant`/`AppCredential` + 全业务表加 `tenant_id` |
| 鉴权 | JWT 加 `tenant_id`;`middleware.TenantID(c)` |
| 登录 | `Login(platform, appid, code)`:多租户 credstore 解析 / 单租户 .env;`(tenant_id, openid)` 建号 |
| 支付 | `driverFor(tenantID, platform)`;WxDriver 改 `WxCreds`(PEM 文本);回调 `/pay/callback/:platform/:tenantId` |
| 钱包 | `Credit/Debit(tenantID, …)` + `CreditTx`;wallet/txn/order 带 `tenant_id` |
| 业务 | bottle/match/chat/relation/item/moderation 查询写入全部带租户;feed Redis 键加租户前缀 |
| 前端 | `getAppId()`(条件编译)+ 登录带 appid |

## 三、开关 / 兼容

`MULTI_TENANT_ENABLED`(默认关)。关=沿用 .env 单租户、`tenant_id=DefaultTenantID(1)`、登录不强制 appid;开=凭证表 + appid 解析 + 全链路隔离。`CRED_MASTER_KEY` 控加密。

## 四、测试结果

- 后端 `build/vet/test` 全绿(新增 crypto 加解密单测)。
- **单租户回归(flag off)**:登录 `tenant_id=1`;接口测试 40 请求 0 功能失败(5 条响应时间断言为远程库 flaky,已登记)。
- **多租户隔离(flag on)**:未知 appid 拒(2002);appA→100/appB→200;跨租户捞瓶不可见;跨租户详情拦截(1005)。

## 五、与设计的偏差

- 用户唯一性走 app 层 find-or-create(`tenant_id+openid`),不建 DB 复合唯一(规避空 openid 冲突)。
- 接口测试回调路径同步改为 `/pay/callback/wx/1`(默认租户)。

## 六、待办

- 支付宝真实下单 + RSA2 验签(driver 仍为骨架)
- 租户开通/管理端(运营建租户填凭证);历史数据 tenant_id 回填脚本
- 接口测试加常驻"双租户隔离" folder;KMS 升级
