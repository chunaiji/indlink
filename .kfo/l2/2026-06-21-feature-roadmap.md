# 待做功能规划(机器人/扔捞匹配/支付/推送/动态/同城/分享/聊天/管理台)

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层(规划) |
| 日期 | 2026-06-21 |
| 状态 | PROPOSED(逐项落地后拆分为独立 L2 COMPLETED 并回标本文件) |
| 触发 | 用户一次性提出 11 项待做功能,要求先出规划文档 |
| 关联 | L1 `l1/bottle-feed.md` · `l1/match.md` · `l1/pay-wallet.md` · `l1/user-auth.md` · `l1/chat-ws.md` · L4 `l4/cross-platform-api-patterns.md` |

> 说明:第 11 项「骗审功能」不在本规划内 —— 见末尾「不予实现项」。第 8 项的"自己不能捡自己"已在 `feed.go`(`user_id <> ?`)实现,本文件只规划其匹配算法部分。

---

## 优先级与依赖

```
P0 管理平台(#10) ──┬─> 机器人(#1)         # 机器人靠管理台配置/投放
                    └─> 配置项(分享/匹配权重/推送开关 等)
P0 同城字段+搜索(#6)                        # 注册采集 + 列表展示,改动小、收益高
P1 精准匹配算法(#8) ─> 复用到 捞瓶 / 扩列(#2)
P1 微信支付(#3)                             # APIv3 已有 driver,补真实下单+回调闭环
P1 回复推送(#4)                             # 订阅消息 + 站内信
P2 海洋分享/筛选(#7)
P2 我的动态(#5)                             # 新模块,较大
P2 聊天记录+本地删会话(#9)                   # 记录已落库,补本地删除
```

---

## #10 管理平台(P0,基座)—— ✅ 已落地,详见 `l2/2026-06-21-admin-platform.md`

> 基座已完成:Go `internal/admin`(隔离鉴权 + sysconfig 白名单配置中心 + 概览)+ Vue3 后台(登录/概览/配置/改密),端到端冒烟通过。
> ✅ **内容池维护页 + 租户凭证页已落地(2026-06-22):** 见 `l2/2026-06-22-p2-p3-features.md`。

**目标:** 一个独立 Web 后台,运营可配置机器人数量、匹配权重、分享文案、推送开关、内容池、租户凭证等。

- **形态:** Go 服务新增 `internal/admin` 模块,暴露 `/admin/*` API(独立 `admin_token` 鉴权,与 C 端 JWT 隔离);前端用轻量 Vue3 SPA(单独 `admin/` 目录,Vite 构建,部署到 `/admin`)。
- **数据模型:** 新表 `sys_config(tenant_id, key, value_json, updated_at)` —— KV 配置中心;管理员表 `admin_user(username, pwd_hash, role)`。
- **首批配置项:** 机器人开关/数量/投放频率、匹配权重(见 #8)、分享标题/图、推送模板 ID、敏感词表、各开关灰度。
- **读取:** C 端服务启动加载 + 定时刷新(或 Redis 缓存 `config:<tenant>:<key>`),改配置不发版。
- **开放决策:** 管理台是否多租户共用一套(按 `tenant_id` 切)—— 建议是,与现有 SaaS 架构一致。

## #1 机器人(P0)—— ✅ 基础已落地,AI 人格层 PROPOSED

> 已完成(2026-06-21):`internal/robot`(service + scheduler)+ `model.RobotContent` 内容池 + `User.IsRobot`。调度器按 `robot_*` 配置定时投放瓶子/概率回信,速率累加器精确还原"每小时 N 个",`feed.go` 已对机器人降权。冒烟:开启后单 tick 投放 10 瓶 + 回信 5 条。
> ✅ **管理台内容池维护页已落地(2026-06-22):** 见 `l2/2026-06-22-p2-p3-features.md`。
> 📋 **AI 人格化对话引擎(PROPOSED 2026-06-23):** 叠加 AI 人格层 + 两级回复缓存（关键字规则 + LLM 自积累）+ 聊天续接自动回复，目标覆盖 60%+ 高频问题无需调 LLM，见 `l2/2026-06-23-ai-persona-robot-design.md` 和 `l1/robot.md`。**待确认:** LLM 供应商 / 机器人聊天扣币豁免 / 首批关键字规则内容。

**目标:** 注入 N 个机器人账号,让海洋/同城/扩列"有人气":自动扔瓶、回信、被捞到时像真人。

- **数据模型:** `User` 加 `is_robot bool`(或 `account_type`);新表 `robot_content(tenant_id, type[bottle/reply], text, tags, weight)` 内容池;`robot_profile` 头像昵称城市性别池。
- **后端:** `internal/robot` 模块 + 定时任务(cron):按管理台配置的频率,随机机器人从内容池扔瓶/对热门瓶回信;机器人资料走资料池。
- **匹配融入:** 机器人瓶子正常进 feed,但**控制占比**(配置项,如真人不足时补位),避免全是机器人。
- **合规:** 机器人不参与真实支付/不诱导付费;仅做内容氛围。**需在隐私说明体现"含运营账号"**。
- **开放决策:** 机器人回信是否触发对真人的推送(建议触发,但标注来源,提升留存)。

## #6 同城字段采集 + 展示 + 搜索(P0)—— ✅ 展示+搜索已落地

> 已完成(2026-06-21):`match.CityUsers` 返回体补 `created_at/follow_count/fans_count`(`fillCounts` 批量查 relation 避免 N+1),新增 `keyword`(昵称/城市模糊)+ `sort`(active/new)参数;`city.vue` 搜索框接上 keyword、加活跃/新人排序、卡片展示性别/年龄/城市/关注/粉丝/注册时间。端到端冒烟通过。
> **注册资料完善流程已补**:新页 `pages/profile-edit`(头像/昵称/性别/年龄/城市),新用户 `is_new` → 首进海洋自动引导,我的页头像区可点编辑 + 未完善横幅;后端 `/user/update` 早已支持。**待续:** 真实经纬度距离。

**目标:** 注册/完善资料时采集 地址、注册时间、性别、关注/被关注数,并在同城列表展示;实现同城搜索。

- **现状:** `User` 已有 `gender/age/city/created_at/last_active_at`;`relation.StatsOf` 已能出 `i_like(关注)/like_me(被关注)/viewed_me`。**缺:** 注册引导采集 + 列表返回这些字段 + 搜索条件。
- **后端:**
  - 资料完善接口补 `city/gender/age`(地址可用微信 `getLocation`→逆地理 或手选城市;"时间"用 `created_at`)。
  - `match.CityUsers` 返回体补 `gender/age/city/created_at/follow_count/fans_count/online_hint`(批量查 relation 统计,避免 N+1)。
  - 搜索参数:`keyword`(昵称模糊)、`gender`、`city`、`sort`(活跃/新人/距离)。
- **前端:** `city.vue` 搜索框已存在但 `reload` 未带 keyword → 接上;卡片展示性别/年龄/城市/关注数/被关注数/在线提示。
- **开放决策:** 是否引入经纬度做真实"距离"(需 `User` 加 `lat/lng` + 定位授权);V1 可先按 `city` 字符串同城,距离用"附近"模糊词。

## #8 精准匹配算法(P1)—— ✅ 已落地

> 已完成(2026-06-21):`feed.go` rebuild 升级为加权打分 `wTag·标签重合 + wCity·同城 + wGender·异性 + wFresh·新鲜度 + wHeat·热度 − wRobot·机器人 + wRandom·随机`,权重全走 sysconfig(管理台可调)。候选池补拉黑过滤(`blocks`),作者性别/is_robot 批量查(`authorMeta`),浏览者兴趣 = 入参标签 + 自己发过的瓶子标签。新增 `User.IsRobot` 字段。捞瓶冒烟通过。
> ✅ **同城/扩列墙拉黑过滤已落地(2026-06-22):** `match.CityUsers` 和 `match.ExpandWall` 均补 `blockedIDs` 双向过滤，见 `l2/2026-06-22-p2-p3-features.md`。**待续:** 扩列墙复用打分算法;机器人惩罚待真实注入机器人后生效。

**目标:** 捞瓶/扩列不再纯随机,按用户画像精准匹配;且永不捞到自己(✅ 已实现)。

- **候选集(已具备):** 本租户 · active · 未过期 · `user_id <> 我` · 未看过(`match_log` 去重)· 排除已拉黑(待接 `moderation`)。
- **打分(扩展 `feed.go` 现有 heat 排序):**
  ```
  score = w1·标签重合度 + w2·同城(city 命中) + w3·异性优先(按用户取向)
        + w4·新鲜度(created_at 衰减) + w5·热度(heat_score 归一)
        - w6·机器人惩罚(降低机器人占比) + w7·随机扰动(防固化)
  ```
  权重 `w1..w7` 走 #10 管理台配置;`match_log` 保证不重复捞同一只。
- **落点:** 抽到 `internal/bottle/feed.go` 的打分函数(已有 `Limit(size*3)` 多取后打分截断的钩子),新增 `scoreCandidate(viewer, bottle)`。
- **开放决策:** 取向字段(找同性/异性/不限)需在资料里加 `prefer_gender`;无则默认不限。

## #3 微信支付(P1)

**目标:** 把 APIv3 真实下单+回调闭环跑通(driver 已在 `l1/pay-wallet.md`)。

- **后端:** 确认 `pay` 模块 JSAPI 下单(`out_trade_no`/`openid`/金额)→ 返回前端 `payParams`(已封装 `requestPay`);**回调**验签(平台公钥)+ 金额校验 + 幂等(`order_no`,已规划)+ 入账与订单状态同事务(L4 规则 2)。
- **配置:** 商户号/APIv3 Key/证书序列号/平台证书 → 走 **凭证加密表**(SaaS 已有 `credstore`),管理台维护。
- **前端:** 充值页已就绪(轮询余额到账)。
- **合规:** iOS + 微信 内购开关已在 `recharge.vue` 处理(`iosRechargeOff`)。
- **验证:** 沙箱/小额真实支付 → 回调入账 → 重复回调余额不变。

## #4 别人回复瓶子 → 推送(P1)—— ✅ 站内信+WS 已落地,详见 `l1/notify.md`

> 已完成(2026-06-21):`internal/notify`(站内信落库 + 在线 WS 实时推 + 红点未读)+ `model.Notification`;`bottle.Reply` 经 `OnReplied` 回调解耦触发;前端「互动通知」页 + 消息页红点(WS 实时 +1)。端到端冒烟通过。**待续:** 微信订阅消息真实下发(已留 `SendWxSubscribe` 钩子 + `notify_wx_template_id` 配置,需模板 ID + access_token 基建)。

**目标:** 我的瓶子被回信时,推送提醒(微信订阅消息 + 站内信)。

- **微信订阅消息:** 申请模板(如"收到新回信");用户在关键动作(扔瓶/查看)处 `requestSubscribeMessage` 拿一次性授权;后端存授权次数表 `subscribe_grant(user_id, tmpl_id, remain)`;`bottle.Reply` 成功后,若瓶主有授权额度 → 调微信 `subscribeMessage.send`。
- **站内信:** 新增 `notification(user_id, type, ref_id, text, read)` 表 + 红点;消息页加"互动通知"入口。WS 在线时实时推。
- **数据触发点:** `bottle.Reply` 事务后发事件(异步队列/直接调),避免阻塞回信。
- **开放决策:** 订阅消息一次性授权用完即止 —— 是否在多处补授权入口;站内信是兜底。

## #2 扩列功能规划(P1)

**目标:** 把现有 `expand.vue`(活跃/新人/附近 + 喜欢)做成完整"交友扩列墙"。

- **现状:** `match.ExpandWall(type)` + `relation.Like`;`附近` 已接 `getLocationOnce`。
- **补全:**
  - Tab:活跃/新人/附近/同城,卡片展示资料(复用 #6 字段)。
  - 互动:喜欢→进入"互相喜欢"才可开聊(或付费开聊,复用 chat 扣币);拉黑/举报(接 `moderation`)。
  - 机器人补位(#1)。
  - 算法复用 #8 打分。
- **开放决策:** "互相喜欢才聊" vs "付费直接聊"——建议两者并存(免费需匹配,付费可直聊)。

## #7 海洋分享 + 筛选(P2)—— ✅ 已落地

> 已完成(2026-06-21):
> - **分享**:海洋页 `onShareAppMessage`(转发好友)+ `onShareTimeline`(朋友圈);标题走 `sysconfig.share_title`(管理台可调,经 login 下发到 `userStore.shareTitle`);信纸弹框含 `open-type="share"` 按钮,分享带 `?bottle=<id>` 深链,`onLoad` 收到 `bottle` 参数自动打开该瓶(`openShared`)。
> - **筛选**:`openFilter` 改为底部弹层(漂向 不限/同城/全国 · 性别 不限/女/男);`bottle.Scoop` 扩展 `Filter{Scope,Gender}` → `feed.Next/rebuild`(同城硬过滤 city、性别 `user_id IN (gender=?)`);**筛选签名进 feed 缓存键** 避免与默认 feed 串味。
> - 端到端冒烟:login 返回 share_title、管理台改后 login 即变;scoop `?gender=2`/`?scope=local`/无筛选均 code 0 返回。
>
> **原始规划:**
- **分享:** 页面 `onShareAppMessage` + `onShareTimeline`;分享卡标题走 #10 配置;带 `?bottle_id` 落地。
- **筛选:** 弹层按漂向/性别筛选,参数传 `bottleApi.scoop`。
- **后端:** `Scoop` 扩展 `scope/gender`。

## #5 我的动态(P2,新模块)

**目标:** 类朋友圈的个人动态流。

- **数据模型:** `moment(moment_id, user_id, content, media_urls, visible[public/follow], like_count, comment_count, created_at)` + `moment_like` + `moment_comment`。
- **后端:** `internal/moment` 模块:发布/删除/列表(我的 + 关注的人)/点赞/评论;审核走敏感词。
- **前端:** "我的"页加"我的动态"入口(现为占位 `soon`);动态详情页;发布页(复用图片上传)。
- **关系:** 依赖 `relation` 的关注关系做"关注的人动态"。
- **开放决策:** V1 是否只做"我的动态"(自己可见+主页展示),关注流放 V2。

## #9 聊天记录 + 本地删除会话(P2)—— ✅ 已落地

> 已完成(2026-06-22):`message.vue` 长按会话行弹出 ActionSheet → "删除会话" → `hiddenChats:Set<string>` 写入 `uni.storage`；`visibleChats` computed 属性过滤隐藏项；收到新 WS 消息时自动从隐藏集合移除对应会话。历史记录分页(`GET /chat/:id/messages`)已具备，无需额外改动。

- **现状:** 消息走 HTTP 落库一次(`l1/chat-ws.md`),记录已持久。
- **补:**
  - **历史记录:** 进会话拉历史分页(`GET /chat/:id/messages?before=`);已基本具备,确认分页。
  - **本地删除会话列表项:** 用户在消息列表删除某会话 —— **仅本地隐藏**(mp `storage` 存 `hiddenChats:Set<chat_id>`),不删服务端记录;对方/重新收到消息时可重新出现(从 hidden 移除)。
  - 消息列表渲染时过滤 `hiddenChats`。
- **开放决策:** 是否提供"清空聊天记录"(服务端软删,双方各自视角)——建议 V2,V1 先做本地隐藏会话。

---

## 不予实现项

### #11 骗审功能 —— 不实现

「骗审」指向平台(微信/App Store)审核员展示一份阉割/合规的假版本,审核通过后再用服务端开关切回含支付/社交/不合规内容的真实版本,本质是**用欺骗手段绕过平台的内容与支付合规审核**。这属于刻意规避平台安全/合规控制的欺骗性做法,违反平台政策,**不予设计或实现**。

**正当替代方案(可做):**
- **诚实的功能开关/灰度**:未过审的能力对所有人(含真人)真实关闭,过审后再开;不针对审核员做差异化。
- **合规模式**:按真实法规做地区/年龄限制、iOS 内购合规(已在 `recharge` 处理)。
- **真正满足审核要求**:补齐资质、内容审核、隐私说明,从根上过审。

如需,我可以把上面的"诚实功能开关"纳入 #10 管理平台一并规划。
