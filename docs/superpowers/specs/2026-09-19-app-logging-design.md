# App 日志系统设计

| 字段 | 值 |
|---|---|
| 日期 | 2026-09-19 |
| 状态 | DESIGN — 已确认，待出实施计划 |
| 来源 | 产品侧提出「规划 app 的日志系统，包括所有的请求、错误日志等，方便排查问题，暂时存 app 文件系统」 |
| 关联 | `docs/prototype/v1-screens.html`（设置页入口）· `server/pkg/apilog`（服务端外呼日志，形态参考） |
| 分支 | `feat/app-v1` |

---

## 一、目标与现状

### 1.1 要解决什么

线上出问题时，现在**什么都拿不到**。用户说「发瓶子失败了」，只能靠猜。

### 1.2 现状的两个缺口（本设计一并修复）

**缺口 1：完全没有全局错误捕获。**
`lib/main.dart` 里没有 `FlutterError.onError`、没有 `PlatformDispatcher.onError`、没有任何 zone 守卫。widget 构建异常与未捕获的异步异常**全部落地无声**——用户看到红屏或界面卡住，开发侧一无所知。

**缺口 2：网络错误的原始信息被丢弃。**

```dart
// lib/core/network/api_client.dart 现状
} on DioException catch (e) {
  throw ApiException.network(e.message ?? 'network error');
}
```

`e.response`（状态码、响应体）、`e.type`（超时 / 连接失败 / 取消）、堆栈全部丢失。而这些恰恰是排查「加载不出来」时唯一有用的东西。

### 1.3 不在本设计范围

- **崩溃上报服务**（Sentry / Crashlytics）——只留接缝，不实现，理由见 §六
- **行为分析埋点**——UI 点击、滚动等不记录，那是分析需求不是日志需求
- **客户端与服务端日志关联**——服务端**没有请求 ID 机制**（`model.go:379` 的 `TraceID` 是微信内容安全回调专用，与 HTTP 请求无关），`pkg/apilog` 只记服务端**对外**调用。加 `X-Request-Id` 需要服务端改动，见 §七遗留

---

## 二、决策记录

| # | 决策点 | 选定 | 理由 / 代价 |
|---|---|---|---|
| 1 | 使用场景 | **三个都要**：用户导出、App 内查看、为上报预留 | 三者共用核心，只有出口不同 |
| 2 | 记什么 | **元数据常开，body 由开关控制（默认关）** | 默认状态下手机上不存在敏感数据，而「用户导出发客服」这条路径不可控 |
| 3 | 绝不记录（不可配置） | `Authorization` 头、密码、验证码 | 无排查价值，风险最高。即使 body 开关打开也不记 |
| 4 | 存储位置 | **Application Support + 排除云备份** | Cache 目录会被系统在空间紧张时清掉——而那往往正是用户要报障的时刻 |
| 5 | 容量 / 保留 | **100 MB / 31 天**，均由后台可配 | — |
| 6 | 写入策略 | **内存缓冲 + 批量刷盘** | 每条同步写会在低端机上掉帧；独立 isolate 是过度设计且崩溃时同样丢数据 |
| 7 | 存储格式 | **JSONL** | 滚动删除只需按行截断；机器可读便于过滤与将来上传 |
| 8 | 会话 ID | 每次冷启动生成 8 位十六进制 | 导出的日志能分清是哪一次启动 |
| 9 | 记录范围 | 网络 / 错误 / 登录态 / 导航 / 业务动作；**不记 UI 交互** | 见 §四 |
| 10 | 全局错误捕获 | `FlutterError.onError` + `PlatformDispatcher.onError`，**不用 `runZonedGuarded`** | 两个钩子已覆盖完整；多一层 zone 会引入 `Zone mismatch` 与初始化顺序问题 |
| 11 | `Log` 门面 | **静态单例**，不走 Riverpod | 调用点包括 `main()` 里的错误处理器，那时 `ProviderScope` 尚不存在 |
| 12 | 配置粒度 | **按租户**，不支持按用户 | sysconfig 本身是租户级；按用户需要另一套下发机制 |
| 13 | 本地开发者开关 | **加** | 弥补决策 12：测试与自查可在本机强制开启，不影响线上 |
| 14 | 上报 | **只留 `LogSink` 接缝，不实现** | 上传队列 / 重试 / 采样是另一个等量子系统，且尚未选定服务商 |

---

## 三、核心与数据结构

### 3.1 文件划分

`lib/core/logging/`，每个文件一件事：

| 文件 | 职责 |
|---|---|
| `log_record.dart` | `LogRecord` / `LogLevel` / `LogTag` + JSONL 序列化 |
| `logger.dart` | `Log` 门面、环形缓冲、刷盘调度 |
| `log_sink.dart` | `LogSink` 抽象 + `ConsoleSink`（仅 debug 构建） |
| `log_file_store.dart` | 文件写入、轮转、清理 |
| `log_redactor.dart` | 脱敏 |
| `log_config.dart` | 运行期配置（三层来源，见 §五） |

### 3.2 `LogRecord`

```dart
enum LogLevel { debug, info, warn, error }   // 顺序即严重度
enum LogTag { net, auth, nav, biz, sys }

class LogRecord {
  final DateTime time;
  final LogLevel level;
  final LogTag tag;
  final String message;
  final Map<String, Object?>? fields;  // 结构化附加：code / http / ms / path
  final Object? error;
  final StackTrace? stack;
}
```

JSONL 用短键，文件是机器读的（人读走查看页与导出）：

```
{"t":"2026-09-19T17:23:41.882+08:00","lv":"error","tag":"net","msg":"POST /bottle/create","http":200,"code":3003,"ms":412,"sid":"a3f27b19"}
```

短键的收益不在省空间，在**每行更窄让查看页的正则过滤更快**——那是排查时的高频操作。

### 3.3 `Log` 门面

```dart
Log.i(LogTag.net, 'POST /bottle/create', fields: {'code': 0, 'ms': 340});
Log.e(LogTag.sys, '未捕获异常', error: e, stack: s);
```

**这是本项目里唯一的全局单例**，其余一律走 Riverpod。理由：调用点包括 `main()` 中的全局错误处理器，那里 `ProviderScope` 还不存在；把 logger 穿过每个构造函数传下去比一个全局更糟。

可测性靠内部持有可替换实例。但真正需要测的是三个**纯函数**——JSONL 序列化、脱敏规则、清理判定——它们都不依赖 `Log` 本身。

会话 ID 在 `Log.init()` 生成，由 `Logger` 统一附加，调用点不用管。

---

## 四、采集点

### 4.1 网络：`LoggingInterceptor`

加到 `ApiClient._dio.interceptors`，位于现有 auth 拦截器之后。

它在 `_request` 的 try/catch **下面**，因此能看到完整的 `DioException`——**顺带修复 §1.2 的缺口 2，且不改 `ApiClient` 的对外行为**。

**要点：业务错误码不是 HTTP 错误。** `{"code":3003}` 走的是 HTTP 200，`onError` 不触发。拦截器必须在 `onResponse` 里解一层 `response.data['code']`，一条日志同时带两者：

- 只看 HTTP → 全是 200，看不出业务失败
- 只看业务码 → 看不出超时与连接失败

级别规则：`code == 0` 记 `info`；`code != 0` 记 `warn`；`DioException` 记 `error`。

### 4.2 错误：两个钩子

`main.dart` 中：

```dart
FlutterError.onError = …                   // 构建 / 布局 / 渲染
PlatformDispatcher.instance.onError = …    // 未捕获的异步异常
```

**不使用 `runZonedGuarded`**（决策 10）。两个钩子已覆盖完整，而多一层 zone 要求 `runApp` 在同一 zone 内调用，否则 `Zone mismatch`；与 `ensureInitialized()` 的顺序也很讲究，配错的表现是启动即崩。

两处都要**同步刷盘再放行**——崩溃前那几条最有价值。

### 4.3 导航：记路由模式，不记实参

`go_router` 的 `observers:` 挂 `NavigatorObserver`。直接记 `route.settings.name` 会写下 `/ocean/bottle/1938274650283`。

用一个**纯函数**把数字段替换为占位：

```dart
/// '/ocean/bottle/1938274650283' → '/ocean/bottle/:id'
String routePattern(String path);
```

保留「用户当时在哪一页」，不把业务 ID 串写进日志。纯函数，可测。

### 4.4 登录态

`lib/app/router.dart` 已有 `ref.listen<AuthState>` 在桥接登录态给 go_router，在那里加一行即可，不新起监听。「用户说突然被登出了」靠这条查。

### 4.5 业务动作：显式调用，6 处

扔瓶、捞瓶、解锁回信、送礼、发起支付、支付结果。这些与钱和配额相关，出纠纷要能对账。

**不做自动埋点**——自动埋点要么记太多，要么恰好记不到关键分支。

### 4.6 不记录

UI 点击、滚动等交互。量极大、信噪比极低，且属行为分析而非故障排查。

---

## 五、存储、轮转与配置

### 5.1 目录与命名

```
<Application Support>/logs/
  2026-09-19.log        ← 当天，正在写
  2026-09-19.1.log      ← 当天超过 5 MB 后滚出
  2026-09-18.log
```

按天分文件 + 单文件 5 MB 上限。不让单文件无限增长的理由很实际：100 MB 的单文件既无法在查看页翻阅，也无法导出。

### 5.2 排除云备份（有平台代价）

Application Support 在 iOS 与 Android 上**默认都进云备份**。不排除的话日志会随用户上 iCloud / Google Drive，换机还会带过去。

- **iOS**：在 `AppDelegate.swift` 启动时给目录设 `NSURLIsExcludedFromBackupKey`（`path_provider` 不暴露此项，约 5 行 Swift）
- **Android**：`AndroidManifest` 挂 `android:dataExtractionRules`，XML 排除 `files/logs`

> ⚠️ **iOS 这一段无法在当前开发机验证**（Windows 上 `flutter build ipa` 子命令都不存在）。需 macOS + Xcode 确认。

### 5.3 清理：纯函数

```dart
/// 返回应删除的文件名。三条规则按顺序应用：
///   1. 超过 retainDays 的整个删除
///   2. 删完仍超 maxTotalBytes,从最旧的继续删,直到降至上限内
///   3. **正在写入的文件永不删除**
List<String> planCleanup(
  List<LogFileInfo> files, {
  required int maxTotalBytes,
  required int retainDays,
  required DateTime now,
  required String activeFile,
});
```

第 3 条只在「当天日志已超总量上限」这一边界触发，实际极少遇到，但一旦遇到就是数据损坏——必须有测试覆盖。

清理在**每次刷盘后**检查，而非定时器：定时器在 App 被杀后不跑，刷盘则一定发生。

### 5.4 缓冲的两条规则

环形缓冲容量 500 条（有界，防止错误风暴吃光内存）。缓冲满需丢弃时：

**只丢 debug / info，永不丢 warn / error。** 错误风暴恰恰是缓冲会满的场景，而那时最该保住的就是错误本身。

刷盘触发：**200 条** / **2 秒** / **App 进入后台** / **出现 error 级日志** / 显式调用。

### 5.5 配置：五个 sysconfig key

挂进现有的 `clientConfig()`（`internal/user/handler.go:306`），登录与 `/user/profile` 两处都会下发，**改配置不需要重新登录**。

| key | 类型 | 默认 | 作用 |
|---|---|---|---|
| `app_log_enabled` | bool | `1` | 总开关 |
| `app_log_level` | text | `info` | 最低记录级别 |
| `app_log_body` | bool | **`0`** | 请求 / 响应体开关 |
| `app_log_max_mb` | int | `100` | 总量上限 |
| `app_log_retain_days` | int | `31` | 保留天数 |

五个都必须**同步写 `sysconfig.go` 的 defaults**（硬约束 1：空串会导致开关逻辑反转）并**登记 `admin/meta.go` 白名单**（硬约束 2），分组新开「App 日志」。

### 5.6 引导问题：配置来自服务端，日志却要更早工作

**启动崩溃与登录失败恰恰最需要日志，而那时一个请求都还没成功。**

配置三层，后者覆盖前者：

1. **编译期默认值**（上表）——冷启动第一毫秒可用
2. **上次收到的服务端配置**，缓存于 `Prefs`——启动时立即读取
3. **本次下发的服务端配置**——登录或拉取资料后覆盖

这样「运营昨天关闭了日志」在今天冷启动时即生效，无需等第一个请求成功。

### 5.7 配置粒度的限制

`sysconfig` 是**租户级**，因此**无法只为某一个出问题的用户打开 body 记录**——打开即全租户生效。

配合 100 MB 上限这不至于出事（开十分钟再关，量可控），但与理想用法有落差：理想是「给张三开详细日志」，实际是「全开十分钟等张三复现」。

**补偿措施：本地开发者开关。** 设置页连点版本号 7 次打开隐藏面板，可在**本机**强制开启 body 记录与 debug 级别，不影响线上配置。测试与自查走这条。

### 5.8 脱敏

`log_redactor.dart`，纯函数。

**不可配置的硬规则**（决策 3）：

- `Authorization` 头永不记录，只记「是否携带 token」
- 字段名命中 `password` / `code`（验证码场景）/ `id_token` / `token` / `secret` 时，值替换为 `***`

body 开关打开时**仍然过这套脱敏**。

---

## 六、出口

三个出口共用同一份存储，**入口位置不同**。

### 6.1 导出（用户报障）——可见入口

设置页「导出日志」，普通用户可见。

1. 取**最近 3 天**或**最近 5 MB**（取小者）。100 MB 发给客服没人看得了，微信也发不动
2. JSONL 转为可读文本：`17:23:41 [ERROR][net] POST /bottle/create → 3003 (412ms)`
3. 走 `share_plus`（**已是依赖，无需新增**）弹系统分享面板

**导出前弹确认，列出内容**（时间范围、条数、是否含请求体）。这是日志离开设备的唯一路径，用户有权知道自己在发什么；万一 body 开关开着，这是最后一道提醒。

### 6.2 查看页（开发与测试）——藏于开发者面板

放在 §5.7 的隐藏面板内。功能：按**级别**与**标签**过滤、关键词搜索、「跟随最新」开关。

**性能约束**：5 MB JSONL 约两万行，全量读入再渲染会卡死。做法是**只读最新文件的最后 500 行**，翻到底再加载更多；过滤时重新扫描而非全量驻留内存。

### 6.3 上报——只留接缝

不实现。理由：上传队列、失败重试、网络类型判断、采样率、后端接收端点与存储，构成一个与日志系统等量的子系统，且尚未选定崩溃上报服务商。

已铺好的路（成本为零）：

- **JSONL + 会话 ID + 结构化字段**本身即上传格式，无需转换
- **`LogSink` 抽象**即接缝——将来的 `RemoteSink` 实现同一接口，与 `FileSink` 并列注册，采集侧一行不改

---

## 七、风险与遗留

| 风险 | 影响 | 处置 |
|---|---|---|
| **iOS 备份排除无法在当前机器验证** | 日志可能随 iCloud 上传，与设计意图相反 | 需 macOS 确认；实施计划中标为未验证项 |
| **缓冲丢失窗口** | 崩溃可能丢掉最后 2 秒的 info 日志 | error 级强制刷盘 + 错误钩子内同步刷盘，真正可能丢的只有「一切正常时的最后几条 info」 |
| **body 开关误留开启** | 手机上长期存在敏感数据 | 默认关；导出前确认屏会显示「含请求体」；建议运营侧约定开启后必须关闭 |
| **查看页性能** | 全量加载会卡死 | 尾部 500 行 + 增量加载，见 §6.2 |
| **日志本身出错** | 日志系统抛异常反而拖垮 App | 所有写盘与序列化包 try/catch 后静默吞掉——**日志永远不能成为故障源** |

### 遗留，不在本期

1. **崩溃上报服务接入**——决策 14，需先选型
2. **按用户的配置粒度**——§5.7，需要用户级下发机制
3. **`X-Request-Id` 客户端与服务端日志关联**——服务端现无请求 ID 机制（§1.3），加它属服务端改动
4. **服务端入站请求日志**——`pkg/apilog` 只记对外调用，不记进来的请求
