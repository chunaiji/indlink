# 漂流瓶社交小程序

变现型陌生人轻社交小程序:扔瓶 / 捞瓶建立连接,金币充值 + 站内消费(开聊 / 解锁回信 / 道具礼物)变现。一套 uni-app 代码同时发布微信、支付宝;后端 Go 模块化单体。

设计文档见 `docs/superpowers/specs/2026-06-20-driftbottle-design.md`,页面设计稿见 `漂流瓶/`(参考)与 scratchpad 中的 HTML 高保真稿。

> **编译 & 联调命令** 见 [`docs/DEV_RUNBOOK.md`](docs/DEV_RUNBOOK.md)(后端运行 / 接口测试 / 前端编译 / 联调配置 / 排查 / **生产部署**)。

## 生产环境

| 用途 | 地址 |
|---|---|
| **API Base URL** | `https://ambertu.com/message` |
| **前端 BASE_URL** | `https://ambertu.com/message/api` |
| **前端 WS_URL** | `wss://ambertu.com/message/ws` |
| **管理后台** | `https://ambertu.com/message-admin/` |
| **支付回调** | `https://ambertu.com/message/api/pay/callback/wx` |
| **服务器** | `43.136.54.189`（systemd: `driftbottle`） |

## 目录结构

```
server/    Go 后端(Gin + GORM + Redis + WebSocket)
client/    uni-app 前端(Vue3 + Pinia,微信/支付宝双端)
admin/     运营管理后台(Vue3 + Vite,独立工程)
docs/      设计文档
deploy.js  一键部署脚本(SSH 上传 + systemd + nginx)
```

## 后端运行(server/)

依赖:Go 1.22+、MySQL 8.0、Redis。

```bash
cd server
cp .env.example .env          # 按需填写 MySQL/Redis/微信/支付宝
go mod tidy                    # 拉取依赖
go run ./cmd/api               # 启动,默认 :8980,自动建表+初始化档位/道具/配置
```

- 健康检查:`GET /health`
- API 前缀:`/api`
- WebSocket:`/ws?token=<JWT>`
- 管理后台 API 前缀:`/admin/api`

测试与静态检查:

```bash
go vet ./...
go test ./...
go build ./...
```

> 注:不带平台凭证时(开发态),登录/支付走 mock(`wxdev_<code>` / mock 支付参数),便于本地联调;
> 填入正式 appid / 商户号后,在 `internal/user/oauth.go` 与 `internal/pay/driver_*.go` 的 TODO(real) 处接官方 SDK。

## 前端运行(client/)

依赖:HBuilderX 或 `@dcloudio` CLI。

```bash
cd client
npm install
npm run dev:mp-weixin     # 微信开发者工具打开 dist/dev/mp-weixin
npm run dev:mp-alipay     # 支付宝小程序 IDE 打开 dist/dev/mp-alipay
```

在 `manifest.json` 填入微信/支付宝 appid;在 `utils/config.js` 填入后端域名。

## 管理后台运行(admin/)

```bash
cd admin
npm install
npm run dev    # http://localhost:5174（代理到 :8980）
npm run build  # 产物 admin/dist，nginx 托管到 /message-admin/
```

## 关键设计

- **支付与消费分离**:充值(真钱→金币)只在平台异步回调入账且幂等;消费(开聊/解锁/送礼)走 `wallet.Debit` 行锁事务,余额不足拦截。
- **跨平台抽象**:平台差异(登录/支付/系统信息)只锁在 `client/utils/platform.js`,业务代码不出现 `wx.`/`my.`。
- **iOS 合规**:微信 iOS 端按 `config.ios_recharge_off` 开关隐藏充值入口,无需发版。
- **价格/开关配置化**:开聊几币、解锁几币、是否强制认证等都在 `config` 表,运营热调。
- **自定义 TabBar**:5 个 Tab 的显示/隐藏由管理后台动态配置,客户端实时拉取。
- **推送模块**:微信订阅消息(`subscribeMessage.send`),支持回信/聊天场景模板推送。
