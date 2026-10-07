# 管理后台多语言设计

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-21 |
| 范围 | `admin/`（Vue3 + Vite 后台）+ `server/internal/admin` 与 `server/internal/common` 的增量 |
| 语言 | 中文 `zh-CN`（默认）· 英文 `en` |
| 状态 | 设计已确认，待实施 |

## 一、目标

管理后台可在中文与英文之间切换，**界面文案、系统配置项标签、接口错误消息**三层全部跟随切换。小程序端与 App 端行为零变化。

### 关于「印度语」

需求最初提的是「中文 / 英文 / 印度语」。确认后印度语指**印度英语**，即与英文共用一套文案，因此实际语言列表是**两项**。框架按多语言设计（不是硬编码的中英二元开关），将来真要独立印地语，加一个 locale 文件与一份消息表即可，无需返工。

## 二、现状盘点

后台现在**零 i18n 基建**——`package.json` 里只有 `vue`、`vue-router`、`echarts`，全部文案硬编码在源码里。

三层中文，实测规模：

| 层 | 规模 | 位置 |
|---|---|---|
| 前端静态文案 | 25 个视图，约 600 条 | `admin/src/views/*.vue`、`router.js`、`api.js` |
| 配置项元信息 | **209 个标签 + 30 个分组名** | `server/internal/admin/meta.go` |
| 接口错误消息 | 82 处 `response.Fail` + 15 处 `errors.New`/`fmt.Errorf` | `server/internal/admin/*.go` |

**边界是干净的**：核过 `overview.go` 与 `service.go`，除配置页与错误消息外，admin 接口不下发任何中文展示文案。返回的昵称、城市、标签值是用户数据，不属于翻译范围。

后台只与 `/admin/api/*` 通信（`api.js` 的 `BASE` 写死了这个前缀），因此第三层的改造边界就是 `internal/admin` 这一个包，不涉及面向 C 端的 handler。

## 三、关键决策

### D1 · 前端用 vue-i18n@9，不手写

仓库风格偏极简（`api.js` 自称「极简 fetch 封装」），手写一个 `t()` 确实能省 30KB 依赖。但插值、回退、响应式切换这几件事写完就是个半成品 vue-i18n，而 600 条文案的规模已经过了那个临界点。用 `legacy: false` 的 Composition API 模式。

### D2 · 错误消息以中文原文为 key，不引入错误常量

97 条中文消息分两类，**两类的源码都一行不改**：

- 82 处直接写在 `response.Fail(c, code, "参数错误")` 里的字面量
- 15 处写在 `errors.New` / `fmt.Errorf` 里，经 `handler.go` 的 11 处 `err.Error()` 流进 `response.Fail`

改的只有 `response.Fail` 与 `response.Abort` 内部——出口处按请求语言查一次消息表。两类消息都在出口汇合，所以一个拦截点就够。

理由：另一条路是把每条消息抽成常量（`msg.BadParams`），那要动 97 处源码，并且要为 97 条消息取名，而取名正是这类改造里最容易陷入反复的环节；调用点也从「一眼看见消息内容」退化成「要跳转才知道说了什么」。

代价是**中文原文被改动时译文会静默失效**（回退成中文）。这个代价用 D6 的覆盖测试兜住。

回退策略：查不到就返回中文原文。**永远不会出现空白或 `MISSING_KEY` 上线**。

### D3 · 语言中间件只挂 admin 路由组

中间件挂在 `handler.go:53` 的 `r.Group("/admin/api")` 上。C 端路由拿不到语言上下文，`i18n.T` 直接返回原文，**小程序与 App 行为零变化**。

这不是保守，是必要的：`wx.request` 在部分机型上会带 `Accept-Language: en`，若全局挂载，中文用户可能突然收到英文报错。

挂在 `g` 而不是 `auth` 上——登录页也有语言切换器，登录失败的提示应当跟随所选语言。

### D4 · 沿用 `Accept-Language`，不新造 `X-Lang`

`app/bottles/lib/core/network/api_client.dart:45` 的注释写着「后端按 `Accept-Language` 下发文案」，App 早已在发这个头，只是后端没接。中间件写成通用实现，将来 App 做 i18n 时挂到 C 端路由上即可生效，不用重写。

`Accept-Language` 是 CORS 安全清单内的请求头，不触发预检，**无需改 `middleware.CORS()`**。

解析做简化版：取首个语言标签，前缀匹配 `en` 则为 `en`，否则 `zh-CN`。不实现 RFC 7231 的 q 权重排序——后台与 App 都只发单个干净标签。这是有意的取舍，写在代码注释里。

### D5 · 配置分组名改稳定 code

见第五节的坑 1。`ConfigField` 拆成两个字段：`group`（稳定 code，供前端逻辑判断）与 `group_label`（已本地化，供界面显示）。

### D6 · 用测试兜住漂移，前端不引入测试框架

三道检查：

| 检查 | 位置 | 挡住什么 |
|---|---|---|
| 消息表覆盖 | `internal/admin/i18n_coverage_test.go` | D2 的中文原文漂移 |
| 配置项双语完整 | `internal/admin/meta_test.go` | 209 个字段漏填英文标签 |
| locale key 对齐 | `admin/scripts/check-locales.mjs` | 两个 locale 文件 key 集合不一致 |

前端目前没有测试框架，为这件事引入 vitest 不值——一个 20 行的 key 对齐脚本能挡住这类改造最常见的失败模式（漏 key）。脚本挂在 `npm run build` 前置，缺 key 即构建失败。

### D7 · 默认中文，不按浏览器语言自动判定

首次访问固定中文，切换后存 localStorage。自动判定会让任何英文浏览器的使用者一进来就看到英文后台，是个不必要的意外；而持久化之后，印度同事也只需切换一次。

### D8 · 时间戳保持 ISO，数字跟随 locale

时间统一 `YYYY-MM-DD HH:mm:ss`，两种语言都不变——后台现在就是这么显示的（`Tenants.vue:133`），而 `9/21` 与 `21/9` 的歧义在运营查订单时代价太高。

数字用 `toLocaleString(当前 locale)` 替换现有的硬编码 `'zh-CN'`（`Dashboard.vue:71-72`）。`zh-CN` 与 `en` 的千分位规则一致，实际无可见差异，但避免了硬编码。

> ⚠️ 不要用 `en-IN`——它是 lakh 分组（`12,34,567`），中国团队看财务数字会当成 bug。

金额符号 `¥` 是数据属性不是语言属性，保持不变。

## 四、设计

### 4.1 前端

新增：

```
admin/src/i18n/index.js          装配 + localStorage 持久化 + 当前 locale 读取
admin/src/locales/zh-CN.json     按页面命名空间组织
admin/src/locales/en.json
admin/src/components/LangToggle.vue
admin/scripts/check-locales.mjs
```

**Key 命名**：`页面.区块.条目`，如 `users.table.nickname`、`config.launch.toReview`。跨页复用的进 `common.*`（保存 / 取消 / 删除 / 确认 / 搜索 / 重置这类）。命名空间与视图文件一一对应，`Users.vue` 的 key 全部落在 `users.*` 下，找文案不用全局搜。

**localStorage key** 用 `driftbottle.admin.lang`，与 `ThemeToggle.vue:27` 的 `driftbottle.admin.theme` 保持一致的命名空间风格。

**`LangToggle.vue`** 照 `ThemeToggle.vue` 的结构写（两个按钮的分段控件），放在 `Layout.vue` 顶栏 `.acts` 里 `<ThemeToggle />` 旁边。登录页顶部也放一个——登录前就该能切。

**`api.js` 从 `i18n/index.js` 取实时 locale**（`currentLocale()`）而不是自己读 localStorage。`i18n/index.js` 只依赖 vue-i18n 与两个 JSON，不引 `api.js`，**不构成循环依赖**；取实时状态也比读存储更准——存储只在切换时写入，程序化改 locale 时两者会漂移。

**切换语言不刷新页面**。vue-i18n 响应式生效，600 条静态文案瞬时切换。唯一依赖服务端文案的是系统配置页，让它 `watch` locale 变化后自己重拉一次数据即可。

> 对照：租户切换用的是 `location.reload()`（`Layout.vue:136`），那是因为**所有**页面都要按新租户重取数据。语言不是这种情况，不要照抄。

**`router.js` 的 `meta.title`** 改存 i18n key（`meta: { titleKey: 'nav.dashboard' }`），`Layout.vue` 的 `title` computed 里过一次 `t()`。侧栏 `GROUPS` 常量同理，`label` 改成 key，渲染时翻译——注意 `GROUPS` 现在是模块级常量，要改成 `computed` 才能响应语言切换。

### 4.2 后端

新增包 `server/internal/common/i18n/`：

```
lang.go      Lang 类型、Middleware()、FromContext()
catalog.go   消息表
```

**为什么单独开包而不是塞进 `middleware`**：`response` 需要读语言，若语言读取函数在 `middleware` 里，就是 `response → middleware`；而 `middleware` 里的中断逻辑将来可能要用 `response`，构成环。`i18n` 只依赖 gin，谁都能引，不会成环。

消息表按语言分层，加语种就是加一个 map：

```go
var catalog = map[Lang]map[string]string{
    En: {
        "参数错误":     "Invalid parameter",
        "用户名或密码错误": "Incorrect username or password",
        // ...
    },
}

// T 查不到一律返回中文原文。
func T(lang Lang, zh string) string { ... }
```

`response.Fail` 与 `response.Abort` 各加一行：

```go
func Fail(c *gin.Context, code int, msg string) {
    c.JSON(http.StatusOK, Body{Code: code, Msg: i18n.T(i18n.FromContext(c), msg)})
}
```

**`meta.go` 改造**。内部元信息结构与下发 DTO 拆开：

```go
// 内部：双语内联
type configFieldMeta struct {
    Key     string
    LabelZh string
    LabelEn string
    Group   string // 稳定 code
    Type    string
}

// 下发：已按请求语言解析
type ConfigField struct {
    Key        string `json:"key"`
    Label      string `json:"label"`
    Group      string `json:"group"`       // 稳定 code，前端逻辑用
    GroupLabel string `json:"group_label"` // 已本地化，界面显示用
    Type       string `json:"type"`
    Value      string `json:"value"`
}
```

30 个分组的 code 映射（**保持 `meta.go` 中的声明顺序**——前端的分组显示顺序依赖它）：

| 分组 | code | 项数 | | 分组 | code | 项数 |
|---|---|---|---|---|---|---|
| 通用 | `common` | 4 | | UI 文案 | `ui_text` | 7 |
| 价格 | `price` | 5 | | 导航 | `nav` | 6 |
| 机器人 | `robot` | 14 | | 页面覆盖 | `page_override` | 2 |
| 匹配权重 | `match` | 8 | | 首页计数 | `home_count` | 3 |
| 分享 | `share` | 2 | | 每日限额 | `quota` | 4 |
| 公告 | `notice` | 2 | | 聊天 | `chat` | 2 |
| AI 引擎 | `ai` | 10 | | 快捷回复 | `quick_reply` | 2 |
| 客服与跳转 | `support` | 20 | | 内容安全 | `moderation` | 2 |
| 流量主广告 | `ads` | 24 | | 水印相机 | `camera` | 4 |
| 我的功能 | `mine` | 15 | | 推送 | `push` | 8 |
| 增长运营 | `growth` | 15 | | 留存功能 | `retention` | 8 |
| App 登录 | `app_auth` | 18 | | App 支付 | `app_pay` | 5 |
| App 地理 | `app_geo` | 3 | | App 推送 | `app_push` | 1 |
| App 日志 | `app_log` | 5 | | App 协议 | `app_legal` | 4 |
| App 完善资料 | `app_profile` | 5 | | App 发现 | `app_discover` | 1 |

合计 209 项，与现状一致。

### 4.3 数据流

```
用户点 LangToggle
  → localStorage['driftbottle.admin.lang'] = 'en'
  → vue-i18n locale 变更（600 条静态文案瞬时切换）
  → 系统配置页 watch 到变更，重拉 GET /config
       → api.js 带上 Accept-Language: en
       → i18n.Middleware 解析写入 context
       → handler 按语言解析 209 个 Label 与 30 个 GroupLabel
  → 后续任何请求出错
       → response.Fail 查消息表，下发英文
```

## 五、两个中文当逻辑键的坑

这不是顺手优化，是不处理就会翻车的。

### 坑 1 · `Config.vue:70` 拿中文分组名做判断

```js
v-if="group === 'AI 引擎' && expanded[group]"
```

分组名一翻译，LLM 连接测试按钮直接消失。D5 的 code 化就是为了修它，改完是 `group === 'ai'`。

连带：`grouped` computed 目前产出 `{ 中文分组名: 字段[] }` 的对象，模板 `v-for="(fields, group) in grouped"` 直接把 key 当标题显示。要改成数组结构 `[{ code, label, fields }]`，显示 `label`、判断 `code`。

### 坑 2 · `Users.vue:74` 的 `t === '付费用户'`

```js
:class="t === '付费用户' ? 'gold' : 'gray'"
```

这里的 `t` 是后端来的**用户标签数据值**，不是界面文案。**绝对不能进翻译表**——翻了之后与数据库里的值对不上，付费用户的金色徽章会失效。保持原样。

## 六、翻译边界判据

实施时对每一条中文按此判断，避免误伤：

| 翻 | 不翻 |
|---|---|
| 界面标签、按钮、表头、占位符、提示 | 用户数据（昵称、城市、瓶子内容、标签值） |
| 错误消息（`response.Fail` 的 msg） | 枚举的**值**（`'human'`/`'robot'`/`'banned'`）——只翻它的显示名 |
| 配置项标签与分组显示名 | 配置项的 **key**（`sysconfig.KeyXxx`） |
| 路由标题、侧栏菜单名 | 源码注释 |
| | 与数据库值比较的字符串字面量（坑 2） |

源码注释保持中文，不翻——它们是给维护者看的，而维护者读中文。

## 七、验证

| 层 | 命令 |
|---|---|
| 后端 | `cd server && go build ./... && go vet ./...` |
| 后端测试 | `go test ./internal/admin/... ./internal/common/...` |
| 前端 | `cd admin && npm run build`（前置跑 key 对齐脚本） |

**已知的既有 flaky 测试**：`internal/robot` 的 `TestSanitizeOutgoingReply_*` / `TestBuildIdentityResponse_*` 因回复池随机选取而随机失败，与本改动无关，跑测试时绕开该包。

人工验收：两种语言各走一遍登录 → 概览 → 系统配置（展开若干分组，确认 LLM 测试按钮仍在）→ 用户管理（确认付费用户金色徽章仍在）→ 触发一次错误（如改密码填错旧密码）。

## 八、不做的事

| 事项 | 理由 |
|---|---|
| C 端（小程序 / App）错误消息 i18n | 超出「管理后台多语言」范围。中间件已写成通用实现，将来挂上去即可 |
| 独立印地语 | 确认为印度英语。框架支持后续追加 |
| 前端测试框架（vitest） | 一个 key 对齐脚本足以挡住主要失败模式 |
| `fmt.Errorf` 带格式化占位符的 2 条消息（`tags.go:26,33`） | 精确匹配的消息表处理不了格式串。数量极少且是标签长度校验的边缘提示，保持中文，在覆盖测试里显式豁免 |
| 后台账号级语言偏好（存服务端） | localStorage 够用。后台使用者固定在自己的浏览器上工作 |

## 九、风险

| 风险 | 缓解 |
|---|---|
| 漏翻某个页面的零散文案 | key 对齐脚本只能查 key 集合不一致，查不出「压根没提取」。实施时逐文件过，以「该文件再无中文字面量（注释除外）」为完成判据 |
| 中文原文漂移导致译文静默失效 | D6 的 AST 覆盖测试，漂移变成构建失败 |
| 英文文案过长撑破密集表格 | 后台是紧密度布局（`Layout.vue:7` 的 `data-density="tight"`）。表头类文案优先选短词；验收时在 1440px 与 1280px 各看一遍 |
| 209 个配置标签的英文翻译质量 | 机器翻译，运营术语（金币 / 档位 / 魅力值 / 破冰 / 捞瓶）可能不地道。已在此说明，后续可校对回填，不阻塞上线 |
