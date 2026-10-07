# AGENTS.md

漂流瓶匿名社交小程序（多租户 SaaS）。Go 后端 + uni-app 双端小程序 + Vue3 管理后台。

## 产品形态（重要）

双形态小程序，由 sysconfig 远程开关控制：
- **工具形态**（审核/默认）：「小纸条水印相机」，启动页 `pages/privacy`
- **社交形态**：漂流瓶（扔瓶/捞瓶/私聊/动态广场/送礼/签到），tab 显隐、页面覆盖、功能开关全部走后台配置

**一切新增用户可见功能必须挂 sysconfig 开关且默认关闭**，工具形态下绝不露出。

## 目录

```
server/   Go 后端(Gin + GORM v2 + MySQL + Redis),入口 cmd/api,模块在 internal/<域>/
client/   uni-app Vue3(alpha) + Pinia,微信/支付宝双端
admin/    Vue3 + Vite 管理后台(线上 /message-admin/)
docs/     DEV_RUNBOOK.md(本地起服务/联调)、superpowers/plans/(功能规划)
.kfo/     知识库(L1 域文档/L2 迭代复盘/L3 问题档案)
deploy.js / deploy_admin.js   部署脚本(根目录)
```

## 常用命令

```bash
# 后端本地跑
cd server && go run ./cmd/api          # 需本地 MySQL/Redis,配置见 .env
go build ./... && go vet ./...         # 提交前验证

# 小程序(微信开发者工具导入 client/dist/dev/mp-weixin)
cd client && npm run dev:mp-weixin     # watch 模式;⚠️ 新增页面(改 pages.json)后必须重启 watch
cd client && npm run build:mp-weixin   # 发行构建(验证语法用)

# 管理后台
cd admin && npm run build

# 部署(编译 → 上传 → systemd 重启,幂等)
cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api
node deploy.js          # 后端 → 175.178.182.166
node deploy_admin.js    # 管理后台静态文件
```

## 部署环境

- **生产服务器 175.178.182.166**（2026-07-31 起），SSH 密钥 `~/.ssh/huawei_app_ed25519`；旧机 43.136.54.189 已废弃勿部署
- nginx 站点配置在 `/etc/nginx/conf.d/pet.conf`（不是 nginx.conf），`/message/`→8980、`/message-admin/`→静态
- 服务 `systemctl status driftbottle`，日志 `journalctl -u driftbottle -f`
- 线上：API `https://ambertu.com/message/api`，后台 `https://ambertu.com/message-admin/`
- 密钥在 `deploy.local.json`（gitignore），表结构靠 GORM AutoMigrate（启动自动加表/列）

## 硬约束（历史踩坑,违反必出线上事故）

1. **sysconfig 新 key 必须同步写 defaults**（`internal/sysconfig/sysconfig.go`），空串会导致开关逻辑反转
2. sysconfig key 要进管理后台必须登记 `internal/admin/meta.go` 白名单（分组/类型；`image` 类型支持后台直接上传图片）
3. **ID 一律字符串下发**（json tag 加 `,string`），JS 端 int64 精度丢失；前端传 ID 同样 String()
4. **统计时间口径按东八区**：勿用 `time.Now().Truncate(24h)`（按 UTC 截断偏 8 小时），参照 `internal/admin/overview.go` 的 periodStarts
5. GORM v2 `.Order()` 只认 string，`clause.Expr` 被静默忽略——动态排序用字符串拼接
6. 多租户：所有表带 tenant_id，所有查询按租户过滤（tenantID=0 仅限后台全量汇总）
7. 微信 scroll-view 会拦截 tap——页面滚动布局用 view + min-height

## uni-app 编译三坑（本项目 alpha 版特有,写模板必读）

1. **事件必须写调用表达式**：`@tap="fn()"` / `@tap="fn($event)"`；裸方法名 `@tap="fn"` 编译后不调用。**自定义组件事件同理**：`@send="onSend($event)"`
2. **弹层禁用 `@tap.stop`**（不阻止冒泡）：遮罩关闭用**背景层分离**模式——mask 容器不绑事件，内部 absolute 铺满的 bg 层绑关闭，panel 作 bg 的兄弟节点
3. **tab 页内弹层 z-index ≥ 1000**：自定义 tabBar z-index=999，低于它底部按钮被挡住点不到

## 前端构建两坑（排查"改了代码不生效"先看这里）

1. **多个 dev watch 进程互相覆盖产物**：历史会话残留的 `dev:mp-weixin` 进程会用旧源码状态盖掉新编译输出。改动不生效时先杀干净再全量重编：
   `Get-CimInstance Win32_Process | ? { $_.CommandLine -match 'mp-weixin' } | % { Stop-Process -Id $_.ProcessId -Force }`，删 `dist/dev/mp-weixin` 后重启 watch
2. **两个产物目录**：`dist/dev`（watch,带日志）与 `dist/build`（发行,build:mp-weixin 手动构建才更新）。开发者工具导入哪个要确认——排查时先核对工具项目路径,再让用户点「编译」重载

## 关键机制速查

- **功能开关下发**：`GET /api/features`（留存功能/动态广场/关联小程序），前端 `store/features.js`；tab/页面覆盖走 `/tabs` `/pages-config`
- **捞瓶 feed**：Redis 预生成队列（`bottle/feed.go`），打分权重后台可调；深夜瓶（night 标签）只在夜场时段可捞，缓存 key 带昼夜维度
- **捞瓶行为记录在 MatchLog**（action=view/reply/like/skip）——漂流轨迹、去重都靠它
- **送礼三处**（聊天/动态）统一链路：扣币事务 + ItemOrder(target) + User.Charm 累加 + 通知；礼物墙/魅力周榜从 ItemOrder 聚合，送礼后调 `rank.InvalidateWeekCache`
- **机器人**：LLM 人格回复 + 身份防泄露三层防护（`robot/identity_guard.go`），出站消息统一 sanitize
- **推送**：微信订阅消息 5 场景模板（sysconfig 配模板 ID），定时任务在 `push/scheduler.go`（签到 09:00、深夜场开场前 5 分钟）
- **微信凭证同源**：access_token 的 appid/secret **优先取租户凭证表**（`app_credentials`,与登录同源,后台「租户凭证」页维护），sysconfig 推送分组仅后备——新租户配一次凭证,登录/推送/内容安全全通（`push.getAccessToken`）
- **内容安全**（微信过审要求,`moderation/wxcheck.go`）：文本 `CheckUGC`（本地词库→msgSecCheck v2,7 个发布场景全覆盖,场景值 1资料/2评论/4社交）；图片 C 端上传统一入口 + 水印相机选图静默上传,提交 `mediaCheckAsync` 留档；开关在后台「内容安全」分组,默认关；API 失败/支付宝用户放行不阻断。结果回调依赖微信"消息推送"(会影响客服消息)暂未启用,违规图靠人工巡查
- **接口日志**：所有外呼(内容安全/订阅推送/逆地理)经 `pkg/apilog` 异步落库,后台「📡 接口日志」页可查,保留 7 天——排查"有没有调、返回什么"先看这里
- **地址水印**：水印相机定位(静默,拒绝授权降级)→ `GET /geo/regeo` 服务端代理腾讯位置服务(key 在后台「水印相机」分组;无 key 降级经纬度)

## 协作约定

- 回复用中文；代码/标识符/commit message 英文（conventional commits）
- 不主动跑测试/脚本，每次运行需明确授权；git 操作仅在明确指示时执行
- 提交前后端跑 `go build ./... && go vet ./...`，前端跑 `npm run build:mp-weixin` 验证
