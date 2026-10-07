# KFO 知识库索引 — 漂流瓶社交小程序

> 本目录是漂流瓶项目(`ai-message`)的 Knowledge Flow OS 知识库,按 L0–L4 五层归档。
> 原始设计/计划文档保留在 `docs/`,`.kfo/` 是其结构化提炼与跨层索引。

---

## L0 — 系统架构层(必读背景)

| 文件 | 内容 |
|---|---|
| `l0/architecture.md` | 整体架构:uni-app 双端 + Go 模块化单体 + 支付/钱包分离 + Redis feed + WS 聊天 + 关键 NFR |

**原始文档映射:** `docs/superpowers/specs/2026-06-20-driftbottle-design.md`

---

## L1 — 设计层(模块级参考)

| 文件 | 覆盖模块 | 核心 |
|---|---|---|
| `l1/pay-wallet.md` | `pay` / `wallet` / `item` | 充值消费分离 · 回调幂等 · 行锁扣减 · 微信 APIv3 driver |
| `l1/bottle-feed.md` | `bottle.feed` | 捞瓶 Redis 预生成列表 · 标签/同城/热度/随机打分 · match_log 去重 |
| `l1/chat-ws.md` | `chat`(hub/service/handler) | WebSocket Hub 推送 · 消息走 HTTP 落库一次 · 开聊扣币 · 多实例 Pub/Sub 扩展点 |
| `l1/user-auth.md` | `user`(oauth/service/handler) | 双平台登录(dev mock)· JWT · 注册奖励 · 认证开关门槛 · 账号合并待办 |
| `l1/match.md` | `match` | 同城列表 · 扩列墙(活跃/新人/附近)· 不扣费/不打分 · 拉黑过滤待办 |
| `l1/tenant-saas.md` | `tenant`/`credstore`/全表 | **IMPLEMENTED** SaaS 多租户:appid 标识 · 凭证加密入表 · tenant_id 隔离 · 双平台 · 开关灰度 · **`Tenant.Type` 小程序/App 二选一(驱动后台按端过滤)** |
| `l1/ui-design-system.md` | `uni.scss` / `components/*` / 全页样式 | 企业级 UI 设计系统:令牌 · 头像/空状态/骨架组件 · 图片规范 · **MP 端事件/emoji 硬约束 + 海洋信纸/投海交互组件** |
| `l1/admin-platform.md` | `internal/admin` / `sysconfig` / `admin/`(Vue) | 管理后台:账号/token/路由全隔离 · sysconfig 白名单配置中心(**分区/分组/键三层 · 按租户类型过滤 · 机密脱敏 · 中英双语**)· /admin/api 与 SPA 部署边界 |
| `l1/robot.md` | `internal/robot` / `RobotContent` / `PersonaConfig` / `RobotMemory` / `RobotKeywordRule` / `RobotReplyCache` | 运营机器人:调度器投放/回信(已实现) · **AI 人格层 + 两级缓存 + 聊天续接(PROPOSED)** · 见 `l2/2026-06-23-ai-persona-robot-design.md` |
| `l1/notify.md` | `internal/notify` / `Notification` / `bottle.OnReplied` | 互动通知:回信→站内信+WS 实时推+红点未读 · 回调解耦 · 微信订阅消息钩子(待接)|
| `l2/2026-06-23-daily-quota-and-config-ui.md` | 每日扔/捞限额规划 + 我的功能后台可配/聊天头像/弹框按钮 UI | 部分完成(UI 已落地;限额 PROPOSED)|
| `l1/bottle-feed.md`(更新) | `internal/bottle/feed.go` | 捞瓶 feed:候选+拉黑过滤 · **#8 加权精准匹配**(标签/同城/异性/新鲜/热度/机器人惩罚/随机,权重 sysconfig) |

**L1 覆盖:** pay-wallet · bottle-feed · chat-ws · user-auth · match(核心模块已全覆盖;`relation`/`moderation` 较薄,暂并入 L0/各 L1 描述)

---

## L2 — 开发执行层(按时间归档)

| 文件 | 里程碑 | 状态 |
|---|---|---|
| `l2/2026-06-20-driftbottle-v1-and-api-tests.md` | V1 全栈实现(后端+前端+微信支付 APIv3)+ 本地图片上传 + Postman/Newman 接口测试(40 用例) | COMPLETED(dev 验证) |
| `l2/2026-06-20-saas-multitenant-impl.md` | SaaS 多租户实现:凭证表加密 + appid 解析 + tenant_id 全链路隔离 + driver-per-tenant | COMPLETED(单租户回归+多租户隔离测试通过) |
| `l2/2026-06-20-enterprise-ui-refactor.md` | 企业级 UI 改造:令牌系统 + 全 9 页统一圆角/阴影/字重 + 头像/空状态/骨架组件 | COMPLETED(微信端构建通过) |
| `l2/2026-06-21-ocean-experience-redo.md` | 海洋页 1:1 复刻 + 捞瓶信纸弹框(限高滚动)+ 写纸条→卷起→塞瓶→投海动画 + 我的瓶子页(已发瓶+回应)+ 删 throw 统一入口 + emoji 豆腐块清理 + 全站事件根因修复 | COMPLETED(dev 构建通过,待真机视觉确认) |
| `l2/2026-06-21-feature-roadmap.md` | 待做功能规划:管理平台/机器人/同城字段+搜索/精准匹配算法/微信支付/回复推送/扩列/分享筛选/我的动态/聊天本地删会话(共 10 项;#11 骗审不予实现) | PROPOSED(#10 已落地) |
| `l2/2026-06-21-admin-platform.md` | 管理平台 #10:Go `internal/admin`(隔离鉴权+白名单配置中心+概览)+ Vue3 后台(登录/概览/配置/改密),端到端冒烟通过 | COMPLETED |
| `l2/2026-06-22-mine-features-chat-hide.md` | 「我的」道具商城/黑名单/浏览记录 + 聊天本地删会话(#9):后端补 `/relation/i-viewed`;前端3页新建+消息页本地隐藏逻辑 | COMPLETED |
| `l2/2026-06-22-p2-p3-features.md` | P2/P3 批次:拉黑过滤(同城/扩列) + 我的收藏(collection 模块) + 管理台机器人内容池 CRUD + 管理台租户凭证查看/修改 | COMPLETED |
| `l2/2026-06-23-ai-persona-robot-design.md` | AI 人格化对话引擎:人格绑定 + 两级回复缓存(关键字规则 + LLM 自积累)+ 聊天续接，目标减少 60%+ LLM 调用 | COMPLETED（待接真实 LLM 配置验证） |
| `l2/2026-06-24-mp-permissions-and-ui-polish.md` | 微信隐私框架(chooseAvatar)+ 圆形头像 + 回信键盘适配 + 开聊心跳动画 + price_chat 下发 + 空头像补全 | COMPLETED + DEPLOYED |
| `l2/2026-06-25-camera-page-and-ui-polish.md` | 相机/水印页落地(@tap 复发/隐私/点击区)+「权限→相机」改名设默认页 + 我的动态后台开关 + 导航默认隐藏 + 推送测试 + UI 图标统一 | COMPLETED + DEPLOYED(小程序待上传) |
| `l2/2026-06-25-robot-avatars-share-reward.md` | 机器人头像按性别分池(robot_man + 幂等迁移)+ 分享奖励回页面后模态告知 + 流水/订单项目名统一中文 + 零散 UI | COMPLETED + DEPLOYED(小程序待上传) |
| `l2/2026-06-27-ad-monetization-and-tenant-mgmt.md` | 流量主广告 B1–B5(按类型配 ID/banner→原生模板/相机页位)+ 租户管理入口 + Config 作用域加固 + 雪花租户 ID 前端精度修复 + 造数据/credStore 重启运维 | COMPLETED + DEPLOYED(小程序待上传) |

---

## L3 — 复盘分析层(按时间归档)

| 文件 | 复盘主题 |
|---|---|
| `l3/2026-06-20-int64-id-js-precision.md` | int64 雪花 ID 在 JS 客户端精度丢失:三次误判→真因→ID-as-string 修复 |
| `l3/2026-06-21-mp-tap-not-binding.md` | mp 端 `@tap="方法名"` 裸引用不触发:埋点+产物比对→真因(须用调用表达式)→全站 36 处批量修(**2026-06-25 相机页复发记录见文末第六节**) |

---

## L4 — 知识沉淀层

| 文件 | 模式类型 | 核心结论 |
|---|---|---|
| `l4/cross-platform-api-patterns.md` | RULE + ANTI-PATTERN | int64 ID 必须字符串序列化 / 支付回调幂等+验签 / 跨端差异锁单层 / 余额不以缓存为真相源 / **MP-1 模板事件须调用表达式 / MP-2 老码点 emoji+ASCII 素材名 / OPS-1 改 Go 须重启** |

---

## 待办

- 支付宝真实下单+验签归档(实现后补 L2/L1)
- 接口测试扩全模块 + CI 接入
- 写纸条弹框补图片瓶能力(旧 throw 页已下线)
- 海洋灯塔/瓶子接透明底 PNG 素材提升还原度
