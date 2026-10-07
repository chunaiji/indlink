# 管理平台(#10)落地:Go `internal/admin` + Vue3 后台

| 字段 | 值 |
|---|---|
| KFO 层级 | L2 — 开发执行层 |
| 日期 | 2026-06-21 |
| 状态 | COMPLETED(后端 build 通过 + API 端到端冒烟通过;前端 Vite build 通过) |
| 触发 | 规划 #10 落地,用 Vue 实现管理后台 |
| 关联 | 规划 `l2/2026-06-21-feature-roadmap.md`(#10/#1/#8 配置项)· L1 `l1/admin-platform.md` · `internal/sysconfig` |

---

## 一、做了什么

运营管理后台(与 C 端完全隔离),可登录、看概览、热调系统配置(含机器人/匹配权重/分享文案)、改密码。

**后端 `internal/admin`(Go/Gin):**
- 新模型 `AdminUser`(bcrypt 密码,role=admin/super);启动 `admin.Init` 无管理员则按 env 播种默认账号 `ADMIN_DEFAULT_USER`/`ADMIN_DEFAULT_PASSWORD`(缺省 `admin/admin123`,日志提示改密)。
- 独立 token:`scope=admin` 的 JWT(复用 `cfg.JWTSecret` 但 scope 隔离,与 C 端 `jwtutil` 不互通),`authMiddleware` 校验。
- 路由挂 `/admin/api/*`(**刻意不与 C 端 `/api` 同前缀、也不与 SPA 静态 `/admin` 冲突**,规避 gin catch-all 冲突):`POST /login`、`GET /me`、`POST /password`、`GET /stats`、`GET /config`、`PUT /config`。
- 配置读写复用现有 `sysconfig`(KV+缓存),`PUT` 仅允许 `meta.go` 白名单键,写后热刷新。`sysconfig` 新增机器人/匹配权重/分享键 + `GetString`。

**前端 `admin/`(Vue3 + Vite + vue-router):**
- 独立工程;hash 路由 + 相对 base,产物可部署任意子路径。
- 页面:登录、概览(7 项统计卡)、系统配置(按组渲染:通用/价格/机器人/匹配权重/分享;bool 用开关,int/text 失焦即存)、账号设置(改密)。
- `src/api.js` fetch 封装:带 Bearer、401 跳登录、解包 `{code,msg,data}`。
- 开发期 `vite.config.js` 把 `/admin/api` 代理到 `:8980`,免跨域。

## 二、验证(端到端冒烟)

| 用例 | 结果 |
|---|---|
| 正确账号登录 | 返回 token ✅ |
| 错误密码 | `code 2002` ✅ |
| 无 token 访问 `/config` | HTTP 401 ✅ |
| 取 config | 18 项 ✅ |
| 改 `robot_count=33` | 保存 ok,`stats.robots=33`(热刷新生效)✅ |
| 写非白名单键 `evil_key` | 拒绝 `非法配置项` ✅ |

## 三、运行方式

- 后端:照常 `go run ./cmd/api`(已自动建 `admin_user` 表 + 播种)。
- 后台前端(开发):`cd admin && npm install && npm run dev` → `http://localhost:5174`(代理到 8980)。
- 后台前端(生产):`npm run build` → `admin/dist` 由 nginx/静态服务托管(**不从 Go 服务 `/admin` 暴露**,避免与 `/admin/api` 的 gin 路由冲突)。

## 四、衔接规划

- 机器人(#1)/匹配权重(#8)/分享文案(#7)的配置项已就位,后续这些功能的执行逻辑直接读 `sysconfig` 对应键即可。
- 待办:配置项的"批量保存/重置默认";机器人内容池、租户凭证维护页(后续随 #1/#3 补)。
