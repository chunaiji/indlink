# 漂流瓶 编译 & 联调命令手册

> 面向开发联调。后端 Go + 远程 MySQL/Redis;前端 uni-app(微信/支付宝双端)。
> 关联:`.kfo/l0/architecture.md` · `.kfo/l2/2026-06-20-driftbottle-v1-and-api-tests.md`

目录:[一、后端](#一后端serverapi) · [二、接口测试](#二接口测试serverapi-tests) · [三、前端编译](#三前端编译client) · [四、联调配置](#四联调必配) · [五、调试排查](#五调试--排查) · [六、一键速查](#六一键速查)

---

## 一、后端(server/)

```bash
cd server

# 1) 依赖(首次/改 go.mod 后)
go mod tidy

# 2) 配置:复制并按需填 .env(已配远程库,可直接用)
cp .env.example .env        # 已有 .env 则跳过

# 3) 连通性校验 + 建库(非破坏性,确保 ai_message 存在)
go run ./cmd/dbinit

# 4) 开发运行(读取 .env,监听 :8980,自动建表+初始化档位/道具/配置)
go run ./cmd/api
#   或编译成二进制再跑:
go build -o ./bin/api.exe ./cmd/api      # Windows;Linux 用 ./bin/api
./bin/api.exe

# 5) 健康检查
curl http://127.0.0.1:8980/health         # → {"status":"ok"}

# 6) 质量门(提交前)
go build ./...
go vet ./...
go test ./...
```

**后台常驻运行(Git Bash):**
```bash
./bin/api.exe > ./bin/server.log 2>&1 &   # 日志写 bin/server.log
# 停:
taskkill //IM api.exe //F                  # Windows
```

---

## 二、接口测试(server/api-tests/)

```bash
# 一次性装 newman
npm i -g newman newman-reporter-htmlextra

cd server/api-tests
# 跑全部 / 指定 collection / 指定文件夹
./run.sh -e dev                                   # *nix / Git Bash
./run.sh -e dev -m core
./run.sh -e dev -m core -f "05 钱包 / 充值(钱·幂等核心)"
# Windows PowerShell:
./run.ps1 -e dev -m core
# 跳过已知 flaky(known-flaky.md):
./run.sh -e dev -m core --allow-flaky
```

报告:`reports/core.html`(人看)· `reports/core.responses.md`(逐请求响应)·
失败排查:`jq -c 'select(.assertionsFailed>0)' reports/core.responses.jsonl`

> 前置:后端需先跑在 `:8980`。当前 40 用例 / 128 断言全过(响应时间断言对远程库偶发 flaky,已登记)。

---

## 三、前端编译(client/)

> 两条路径任选。**联调推荐 HBuilderX(最省事)**;CLI 适合脚本化/CI。

### 路径 A — HBuilderX(推荐)
1. HBuilderX 打开 `client/` 目录。
2. `manifest.json` 填微信、支付宝 appid;`utils/config.js` 填后端地址(见第四节)。
3. 顶部「运行 → 运行到小程序模拟器 → 微信开发者工具 / 支付宝小程序」。
4. 自动拉起对应 IDE 加载产物;改代码即时热更。

### 路径 B — CLI(Vite)— 已验证可用

**工程结构(CLI 模式):源码在 `client/src/`**(HBuilderX 用根目录;CLI/Vite 用 `src/`):
```
client/
├── index.html          (引用 /src/main.js)
├── package.json        (依赖已锁定:uni-app vue3 alpha + Vite 7 + sass)
├── vite.config.js
└── src/  main.js · App.vue · manifest.json · pages.json · uni.scss · pages/ api/ store/ utils/
```

```bash
cd client
npm install                  # 已锁定可解析的版本组合

# 开发(产物在 dist/dev/<platform>,用对应 IDE 打开)
npm run dev:mp-weixin        # → dist/dev/mp-weixin   微信开发者工具打开
npm run dev:mp-alipay        # → dist/dev/mp-alipay   支付宝小程序 IDE 打开

# 生产构建(dist/build/<platform>,已验证两端均可编译)
npm run build:mp-weixin      # → dist/build/mp-weixin
npm run build:mp-alipay      # → dist/build/mp-alipay
```

> **依赖要点(已踩坑修复)**:
> - 所有 `@dcloudio/*` 锁定 vue3 版本 `3.0.0-alpha-1000920260615733`(纯 `^3.0.0` 不存在 → ETARGET)。
> - 该 vue3 alpha 需 **Vite 7**(peer `vite@7.3.3`)+ Vue 3.5,不是 Vite 5。
> - `<style lang="scss">` 需 `sass` 依赖(已加入 devDependencies)。
> - sass 仅有 legacy-js-api 弃用警告,不影响构建。

### 用 IDE 打开产物
- 微信:微信开发者工具 → 导入项目 → 选 `dist/dev/mp-weixin`(或 HBuilderX 自动拉起)→ 填 appid。
- 支付宝:支付宝小程序开发者工具 → 打开 `dist/dev/mp-alipay`。

---

## 四、联调必配

| 配置 | 文件 | 说明 |
|---|---|---|
| 后端地址 | `client/utils/config.js` | 改 `MP-WEIXIN`/`MP-ALIPAY` 分支的 `BASE_URL`/`WS_URL` 为 IDE/真机可达地址 |
| 小程序 appid | `client/manifest.json` | `mp-weixin.appid` / `mp-alipay.appid` |
| 图片前缀 | `server/.env` `PUBLIC_BASE_URL` | 留空=按请求 Host 推导;后端公网时填公网前缀,保证图片在小程序可加载 |

**`BASE_URL` 怎么填:**
- 后端跑在你本机、用**微信开发者工具模拟器**:`http://127.0.0.1:8980/api`(模拟器可访问本机)。
- 真机预览 / 后端在另一台机:用**局域网 IP** `http://192.168.x.x:8980/api`,WS 用 `ws://192.168.x.x:8980/ws`。
- 后端部署到服务器(真机/提审):用**公网 HTTPS** `https://域名/api`、`wss://域名/ws`。

**域名校验:**
- 微信开发者工具:右上「详情 → 本地设置 → 勾"不校验合法域名…"」(联调期)。
- 真机/提审:微信小程序后台「开发管理 → 开发设置 → 服务器域名」配置 request / socket / uploadFile / downloadFile 合法域名(必须 HTTPS/WSS)。
- 支付宝:小程序后台配置对应域名白名单。

---

## 五、调试 & 排查

```bash
# 后端日志(后台运行时)
tail -f server/bin/server.log

# 快速验证某接口(dev mock 登录,任意 code 可登录)
curl -XPOST http://127.0.0.1:8980/api/auth/login \
  -H 'Content-Type: application/json' -d '{"platform":"wx","code":"dbg1"}'

# 带 token 调用
curl http://127.0.0.1:8980/api/wallet/balance -H "Authorization: Bearer <token>"

# 验证图片上传 + 静态访问
curl -XPOST http://127.0.0.1:8980/api/upload -H "Authorization: Bearer <token>" -F "file=@x.png"
curl -I <返回的 url>          # 应 200 image/*

# 模拟微信支付回调(dev,无凭证时走明文 JSON)
curl -XPOST http://127.0.0.1:8980/api/pay/callback/wx -H 'Content-Type: application/json' \
  -d '{"out_trade_no":"<order_no>","trade_state":"SUCCESS","amount":{"total":600}}'
```

**常见问题:**
| 现象 | 排查 |
|---|---|
| 小程序请求失败/被拦 | `BASE_URL` 是否 IDE 可达;开发者工具是否勾"不校验合法域名" |
| 真机连不上后端 | 用局域网 IP 不是 127.0.0.1;手机与后端同网段;防火墙放行 8980 |
| 图片加载不出 | `PUBLIC_BASE_URL` / 推导出的 Host 真机不可达;改成可达前缀 |
| WS 不通 | `WS_URL` 协议(ws/wss)与域名;真机需 wss + 合法域名 |
| 大整数 ID 错乱 | 已修(ID 全字符串);勿在前端对 ID `Number()` |
| 登录响应慢偶发超时 | 远程库延迟,见 `api-tests/known-flaky.md`;切本地库即解 |

---

## 六、生产环境

### 地址清单

| 用途 | 地址 |
|---|---|
| **API Base URL** | `https://ambertu.com/message` |
| **管理后台** | `https://ambertu.com/message-admin/` |
| **支付回调** | `https://ambertu.com/message/api/pay/callback/wx` |
| **服务器** | `43.136.54.189` |
| **管理员账号** | `admin / admin123` |

### 服务器目录

```
/usr/jack/deploy/go_workspace/
├── driftbottle          # Go 二进制
├── .env                 # 生产配置（DB/Redis 用 127.0.0.1）
├── cert/                # 支付证书（apiclient_key.pem 等）
├── uploads/             # 上传文件
└── admin-dist/          # 管理后台静态文件
```

### 服务管理

```bash
# 查看服务状态
systemctl status driftbottle

# 重启服务
systemctl restart driftbottle

# 实时日志
journalctl -u driftbottle -f

# 查看最近 100 行日志
journalctl -u driftbottle -n 100 --no-pager
```

### 重新部署

```bash
# 1. 本机交叉编译
cd server
GOOS=linux GOARCH=amd64 go build -o driftbottle-linux ./cmd/api/

# 2. 运行部署脚本（根目录）
cd ..
node deploy.js

# 3. 仅更新管理后台
cd admin && npm run build && cd ..
node deploy_admin.js
```

---

## 七、一键速查

```bash
# 后端起飞
cd server && go run ./cmd/dbinit && go run ./cmd/api

# 接口回归(另开终端)
cd server/api-tests && ./run.sh -e dev -m core

# 前端(HBuilderX 打开 client/ 运行到微信/支付宝;或 CLI:)
cd client && npm install && npm run dev:mp-weixin
```
