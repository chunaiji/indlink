# App 日志系统 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** App 记录所有网络请求与错误到本机文件，用户可导出发给客服，开发者可在 App 内查看，且行为由后台可配。

**Architecture:** 采集侧（Dio 拦截器 / 全局错误钩子 / 路由观察者）把 `LogRecord` 投给静态门面 `Log`；`Log` 攒在有界环形缓冲里，按条数/时间/生命周期/错误级批量刷给 `LogSink`；`FileSink` 负责 JSONL 落盘、按天与大小轮转、按天数与总量清理。所有判断逻辑（脱敏、清理计划、路由模式、序列化）抽成纯函数先测——本仓库 20 个 Go 测试与 1 个 widget 测试都不碰真实 I/O，这条惯例继续。

**Tech Stack:** Flutter 3.47.5 / Dart 3.13.4 · Riverpod · Dio 5.11.1 · go_router 18.0.1 · path_provider 2.1.6 · share_plus 12.0.2 — **全部已是现有依赖，本计划不新增任何包**

**Spec:** `docs/superpowers/specs/2026-09-19-app-logging-design.md`

## Global Constraints

- **不新增依赖。** `path_provider` / `share_plus` / `go_router` / `dio` 均已在 `pubspec.yaml`。不要升 `flutter_secure_storage`（锁 `^9.2.4`）与 `path_provider_foundation`（`dependency_overrides: 2.4.1`）——会撞 Flutter 3.47 已移除的 `Architecture.arm64e`。
- **绝不记录（不可配置，spec 决策 3）**：`Authorization` 头、`password`、验证码 `code`、`id_token`、`token`、`secret`。body 开关打开时**仍然过脱敏**。
- **日志永远不能成为故障源**：所有写盘与序列化包 `try/catch` 后静默吞掉。
- **新增 sysconfig key 必须同步写 `internal/sysconfig/sysconfig.go` 的 defaults**（空串会导致开关逻辑反转），并登记 `internal/admin/meta.go` 白名单。本计划新增 5 个。
- **不使用 `runZonedGuarded`**（spec 决策 10）。只用 `FlutterError.onError` + `PlatformDispatcher.instance.onError`。
- **`share_plus` 12.x 的 API 是 `SharePlus.instance.share(ShareParams(...))`**，不是旧版的 `Share.share(...)`。现有用法见 `lib/features/bottle/drift_map_page.dart:88`。
- **提交前验证**：`cd app/bottles && flutter analyze && flutter test`；后端 `cd server && go build ./... && go vet ./...`。
- **跑 Go 测试绕开已知 flaky 包**：`go test $(go list ./... | grep -v /internal/robot)`。
- 回复用中文；代码 / 标识符 / commit message 英文（conventional commits）。

---

## File Structure

**客户端（`app/bottles/`）**

| 文件 | 职责 | 动作 |
|---|---|---|
| `lib/core/logging/log_record.dart` | `LogRecord` / `LogLevel` / `LogTag` + JSONL 序列化 | 创建 |
| `lib/core/logging/log_redactor.dart` | 脱敏（纯函数） | 创建 |
| `lib/core/logging/log_retention.dart` | 清理计划（纯函数） | 创建 |
| `lib/core/logging/log_config.dart` | 三层配置合并 | 创建 |
| `lib/core/logging/log_sink.dart` | `LogSink` 抽象 + `ConsoleSink` | 创建 |
| `lib/core/logging/log_file_sink.dart` | JSONL 落盘、轮转、调用清理 | 创建 |
| `lib/core/logging/logger.dart` | `Log` 门面 + 环形缓冲 + 刷盘调度 | 创建 |
| `lib/core/logging/log_export.dart` | JSONL → 可读文本、导出范围裁剪 | 创建 |
| `lib/core/network/logging_interceptor.dart` | Dio 拦截器 | 创建 |
| `lib/core/network/api_client.dart` | 挂拦截器 | 修改 |
| `lib/app/nav_logger.dart` | `NavigatorObserver` + 路由模式提取 | 创建 |
| `lib/app/router.dart` | 挂 observer、登录态日志 | 修改 |
| `lib/main.dart` | `Log.init()` + 两个错误钩子 | 修改 |
| `lib/core/storage/prefs.dart` | 缓存服务端配置 + 本地开发者开关 | 修改 |
| `lib/features/me/settings_page.dart` | 导出入口 + 连点版本号 | 修改 |
| `lib/features/me/log_viewer_page.dart` | 查看页 | 创建 |
| `test/logging/*_test.dart` | 四个纯函数的测试 | 创建 |

拆这么细的理由：`logger.dart` 只管「攒和刷」，`log_file_sink.dart` 只管「写和转」，`log_retention.dart` 只管「该删谁」。三者混在一个文件里，那个「正在写入的文件永不删除」的边界条件就会变得很难测——它需要同时知道「当前活跃文件」和「清理规则」，分开之后前者是参数、后者是纯函数。

**服务端（`server/`）**

| 文件 | 职责 | 动作 |
|---|---|---|
| `internal/sysconfig/sysconfig.go` | 5 个新 key + defaults | 修改 |
| `internal/admin/meta.go` | 后台白名单，新分组「App 日志」 | 修改 |
| `internal/user/handler.go:306` | `clientConfig()` 加 5 个字段 | 修改 |

---

## Task 1: 日志记录与序列化

**Files:**
- Create: `app/bottles/lib/core/logging/log_record.dart`
- Test: `app/bottles/test/logging/log_record_test.dart`

**Interfaces:**
- Consumes: 无
- Produces:
  - `enum LogLevel { debug, info, warn, error }`（顺序即严重度，`index` 可比较）
  - `enum LogTag { net, auth, nav, biz, sys }`
  - `class LogRecord({required DateTime time, required LogLevel level, required LogTag tag, required String message, Map<String, Object?>? fields, Object? error, StackTrace? stack})`
  - `String LogRecord.toJsonLine(String sessionId)`

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/logging/log_record_test.dart`：

```dart
import 'dart:convert';

import 'package:bottles/core/logging/log_record.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final t = DateTime.utc(2026, 9, 19, 9, 23, 41, 882);

  test('序列化成一行 JSON，且确实只有一行', () {
    final r = LogRecord(
      time: t,
      level: LogLevel.error,
      tag: LogTag.net,
      message: 'POST /bottle/create',
      fields: {'http': 200, 'code': 3003, 'ms': 412},
    );
    final line = r.toJsonLine('a3f27b19');

    // 一行一条是整个存储格式的前提：滚动删除按行截断、查看页按行解析。
    // 消息里带换行也不能破坏这一点。
    expect(line.contains('\n'), isFalse);

    final m = jsonDecode(line) as Map<String, dynamic>;
    expect(m['lv'], 'error');
    expect(m['tag'], 'net');
    expect(m['msg'], 'POST /bottle/create');
    expect(m['sid'], 'a3f27b19');
    expect(m['http'], 200);
    expect(m['code'], 3003);
    expect(m['ms'], 412);
  });

  test('消息含换行时不会拆成两行', () {
    final r = LogRecord(
      time: t,
      level: LogLevel.warn,
      tag: LogTag.sys,
      message: '第一行\n第二行',
    );
    expect(r.toJsonLine('s').contains('\n'), isFalse);
  });

  test('fields 为空时不写空对象，异常与堆栈按需写入', () {
    final plain = LogRecord(
      time: t, level: LogLevel.info, tag: LogTag.biz, message: 'ok',
    ).toJsonLine('s');
    final m = jsonDecode(plain) as Map<String, dynamic>;
    expect(m.containsKey('err'), isFalse);
    expect(m.containsKey('st'), isFalse);

    final withErr = LogRecord(
      time: t, level: LogLevel.error, tag: LogTag.sys, message: 'boom',
      error: StateError('bad'), stack: StackTrace.fromString('#0 a\n#1 b'),
    ).toJsonLine('s');
    final m2 = jsonDecode(withErr) as Map<String, dynamic>;
    expect(m2['err'].toString(), contains('bad'));
    expect(m2['st'].toString(), contains('#0 a'));
  });

  test('级别顺序即严重度，用于按最低级别过滤', () {
    expect(LogLevel.debug.index < LogLevel.info.index, isTrue);
    expect(LogLevel.info.index < LogLevel.warn.index, isTrue);
    expect(LogLevel.warn.index < LogLevel.error.index, isTrue);
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/logging/log_record_test.dart`
Expected: 编译失败，`Error: Couldn't resolve the package 'bottles' ... log_record.dart` 或 `Undefined name 'LogRecord'`

- [ ] **Step 3: 实现**

创建 `app/bottles/lib/core/logging/log_record.dart`：

```dart
import 'dart:convert';

/// 日志级别。声明顺序即严重度——按最低级别过滤时直接比 `index`。
enum LogLevel { debug, info, warn, error }

/// 日志标签。查看页按它和级别两个维度过滤。
///
/// 只有这五类，不要随手加：标签一多，过滤就失去意义，
/// 而「这条到底该打哪个标签」的争论会比它带来的价值更贵。
enum LogTag {
  /// 网络请求
  net,

  /// 登录态变化
  auth,

  /// 页面跳转
  nav,

  /// 业务动作（扔瓶/捞瓶/解锁/送礼/支付）
  biz,

  /// 系统与未捕获异常
  sys,
}

class LogRecord {
  const LogRecord({
    required this.time,
    required this.level,
    required this.tag,
    required this.message,
    this.fields,
    this.error,
    this.stack,
  });

  final DateTime time;
  final LogLevel level;
  final LogTag tag;
  final String message;

  /// 结构化附加字段，平铺进 JSON（`code` / `http` / `ms` / `path`…）。
  final Map<String, Object?>? fields;

  final Object? error;
  final StackTrace? stack;

  /// 序列化成一行 JSON。
  ///
  /// **保证不含换行**——这是整个存储格式的前提：清理时按行截断、
  /// 查看页按行解析。`jsonEncode` 会把字符串里的换行转义成 `\n` 两个字符，
  /// 所以只要整体走 `jsonEncode` 就天然满足；手工拼字符串则不然。
  ///
  /// 短键不是为了省空间（31 天量级省不了多少），是为了让每行更窄，
  /// 查看页的正则过滤更快——那是排查时的高频操作。
  String toJsonLine(String sessionId) {
    final m = <String, Object?>{
      't': time.toIso8601String(),
      'lv': level.name,
      'tag': tag.name,
      'msg': message,
      'sid': sessionId,
    };
    if (fields != null) m.addAll(fields!);
    if (error != null) m['err'] = error.toString();
    if (stack != null) m['st'] = stack.toString();
    return jsonEncode(m);
  }
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/logging/log_record_test.dart`
Expected: PASS（4 个用例）

- [ ] **Step 5: 提交**

```bash
cd app/bottles && flutter analyze
git add app/bottles/lib/core/logging/log_record.dart app/bottles/test/logging/log_record_test.dart
git commit -m "feat(app): log record type and JSONL serialisation"
```

---

## Task 2: 脱敏

**Files:**
- Create: `app/bottles/lib/core/logging/log_redactor.dart`
- Test: `app/bottles/test/logging/log_redactor_test.dart`

**Interfaces:**
- Consumes: 无
- Produces:
  - `const redactedMark = '***'`
  - `Map<String, Object?> redactMap(Map<String, Object?> input)`
  - `Map<String, String> redactHeaders(Map<String, dynamic> headers)`

> 这是整个系统风险最高的一块。请求日志会碰到 token、密码、验证码、ID Token，
> 而这些会写进手机、并在用户导出时经过一条不可控的路径。

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/logging/log_redactor_test.dart`：

```dart
import 'package:bottles/core/logging/log_redactor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('redactMap', () {
    test('敏感字段一律替换，不区分大小写', () {
      final out = redactMap({
        'password': 'hunter2',
        'Password': 'hunter2',
        'code': '482913',
        'id_token': 'eyJhbGciOi...',
        'idToken': 'eyJhbGciOi...',
        'token': 'abc',
        'secret': 'shh',
      });
      for (final v in out.values) {
        expect(v, redactedMark);
      }
    });

    test('非敏感字段原样保留', () {
      final out = redactMap({'content': '今晚的海', 'coins': 60, 'scope': 'local'});
      expect(out['content'], '今晚的海');
      expect(out['coins'], 60);
      expect(out['scope'], 'local');
    });

    test('嵌套结构里的敏感字段同样被替换', () {
      // 登录请求体就是嵌套的,只查一层会漏。
      final out = redactMap({
        'user': {'phone': '+919876543210', 'password': 'hunter2'},
        'list': [
          {'token': 'a'},
          {'ok': 1},
        ],
      });
      final user = out['user'] as Map<String, Object?>;
      expect(user['password'], redactedMark);
      final list = out['list'] as List<Object?>;
      expect((list[0] as Map<String, Object?>)['token'], redactedMark);
      expect((list[1] as Map<String, Object?>)['ok'], 1);
    });

    test('字段名只是包含敏感词也要替换', () {
      // refresh_token / access_token / verifyCode 这类都要覆盖到。
      final out = redactMap({
        'refresh_token': 'x',
        'accessToken': 'y',
        'verifyCode': 'z',
      });
      for (final v in out.values) {
        expect(v, redactedMark);
      }
    });
  });

  group('redactHeaders', () {
    test('Authorization 永不记录值，只记是否携带', () {
      final out = redactHeaders({'Authorization': 'Bearer eyJhbGciOi...'});
      // 值本身绝不出现——哪怕截断后的前几位也不行。
      expect(out['Authorization'], isNot(contains('eyJ')));
      expect(out['Authorization'], 'present');
    });

    test('没带 Authorization 时标记 absent', () {
      expect(redactHeaders({})['Authorization'], 'absent');
    });

    test('其余头原样保留', () {
      final out = redactHeaders({'Accept-Language': 'zh-CN'});
      expect(out['Accept-Language'], 'zh-CN');
    });
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/logging/log_redactor_test.dart`
Expected: 编译失败，`Undefined name 'redactMap'`

- [ ] **Step 3: 实现**

创建 `app/bottles/lib/core/logging/log_redactor.dart`：

```dart
/// 脱敏后的占位值。
const redactedMark = '***';

/// 字段名命中其中任意一个子串（不区分大小写）即脱敏。
///
/// 用**子串**而不是精确匹配，是为了覆盖 `refresh_token` / `accessToken` /
/// `verifyCode` 这类变体——精确匹配的名单永远追不上新接口。
/// 代价是可能误伤（比如某个业务字段叫 `barcode`，含 `code`），
/// 但**误伤的后果是少记一条信息，漏网的后果是泄露凭证**，这个取舍不用犹豫。
const _sensitiveParts = <String>[
  'password',
  'passwd',
  'token',
  'secret',
  'code',
];

bool _isSensitive(String key) {
  final k = key.toLowerCase();
  return _sensitiveParts.any(k.contains);
}

/// 递归脱敏。嵌套结构里的敏感字段同样替换——登录请求体就是嵌套的。
Map<String, Object?> redactMap(Map<String, Object?> input) {
  final out = <String, Object?>{};
  input.forEach((k, v) {
    if (_isSensitive(k)) {
      out[k] = redactedMark;
      return;
    }
    out[k] = _redactValue(v);
  });
  return out;
}

Object? _redactValue(Object? v) {
  if (v is Map) {
    return redactMap(v.map((k, val) => MapEntry(k.toString(), val)));
  }
  if (v is List) return v.map(_redactValue).toList();
  return v;
}

/// 请求头脱敏。
///
/// `Authorization` 的值**永不记录**，连截断后的前几位也不记——
/// JWT 的头部是固定的 `eyJ`，记前几位没有任何诊断价值，
/// 却会让人误以为「记一点没关系」。只记有没有带。
Map<String, String> redactHeaders(Map<String, dynamic> headers) {
  final out = <String, String>{};
  headers.forEach((k, v) {
    if (k.toLowerCase() == 'authorization') return;
    out[k] = v.toString();
  });
  final has = headers.keys.any((k) => k.toLowerCase() == 'authorization');
  out['Authorization'] = has ? 'present' : 'absent';
  return out;
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/logging/log_redactor_test.dart`
Expected: PASS（7 个用例）

- [ ] **Step 5: 提交**

```bash
cd app/bottles && flutter analyze
git add app/bottles/lib/core/logging/log_redactor.dart app/bottles/test/logging/log_redactor_test.dart
git commit -m "feat(app): redact credentials before they reach the log"
```

---

## Task 3: 清理计划与路由模式

**Files:**
- Create: `app/bottles/lib/core/logging/log_retention.dart`
- Create: `app/bottles/lib/app/nav_logger.dart`（本任务只写纯函数部分）
- Test: `app/bottles/test/logging/log_retention_test.dart` · `app/bottles/test/logging/route_pattern_test.dart`

**Interfaces:**
- Consumes: 无
- Produces:
  - `class LogFileInfo({required String name, required int bytes, required DateTime day})`
  - `List<String> planCleanup(List<LogFileInfo> files, {required int maxTotalBytes, required int retainDays, required DateTime now, required String activeFile})`
  - `String routePattern(String path)`

- [ ] **Step 1: 写清理计划的失败测试**

创建 `app/bottles/test/logging/log_retention_test.dart`：

```dart
import 'package:bottles/core/logging/log_retention.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final now = DateTime(2026, 9, 19);
  const mb = 1024 * 1024;

  LogFileInfo f(String name, int bytes, int daysAgo) => LogFileInfo(
        name: name,
        bytes: bytes,
        day: now.subtract(Duration(days: daysAgo)),
      );

  test('超过保留天数的整个删掉', () {
    final del = planCleanup(
      [f('2026-09-19.log', mb, 0), f('2026-08-01.log', mb, 49)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, ['2026-08-01.log']);
  });

  test('天数都没超但总量超了，从最旧的开始删到降下来为止', () {
    final del = planCleanup(
      [
        f('2026-09-19.log', 40 * mb, 0),
        f('2026-09-18.log', 40 * mb, 1),
        f('2026-09-17.log', 40 * mb, 2),
      ],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    // 120MB 超了 100MB,删最旧的一个降到 80MB 即可,不该多删。
    expect(del, ['2026-09-17.log']);
  });

  test('正在写入的文件永远不删，哪怕它自己就超了总量', () {
    // 这是最危险的边界:边写边删会损坏文件。
    // 实际很难遇到(需要当天日志就超过总量上限),但遇到就是数据损坏。
    final del = planCleanup(
      [f('2026-09-19.log', 200 * mb, 0)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, isEmpty);
  });

  test('活跃文件超龄也不删', () {
    final del = planCleanup(
      [f('2026-08-01.log', mb, 49)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-08-01.log',
    );
    expect(del, isEmpty);
  });

  test('什么都没超时不删任何东西', () {
    final del = planCleanup(
      [f('2026-09-19.log', mb, 0), f('2026-09-18.log', mb, 1)],
      maxTotalBytes: 100 * mb,
      retainDays: 31,
      now: now,
      activeFile: '2026-09-19.log',
    );
    expect(del, isEmpty);
  });

  test('空列表不炸', () {
    expect(
      planCleanup([], maxTotalBytes: 100 * mb, retainDays: 31, now: now, activeFile: 'x'),
      isEmpty,
    );
  });
}
```

- [ ] **Step 2: 写路由模式的失败测试**

创建 `app/bottles/test/logging/route_pattern_test.dart`：

```dart
import 'package:bottles/app/nav_logger.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('数字段替换成占位，不把业务 ID 写进日志', () {
    expect(routePattern('/ocean/bottle/1938274650283'), '/ocean/bottle/:id');
    expect(routePattern('/discover/user/77'), '/discover/user/:id');
    expect(routePattern('/chats/1938274650283'), '/chats/:id');
  });

  test('没有 ID 的路径原样返回', () {
    expect(routePattern('/ocean'), '/ocean');
    expect(routePattern('/me/settings'), '/me/settings');
  });

  test('多个 ID 段都替换', () {
    expect(routePattern('/a/123/b/456'), '/a/:id/b/:id');
  });

  test('含数字的单词不误伤', () {
    // 'v1' 'sha256' 这类是路径的一部分,不是 ID。
    expect(routePattern('/api/v1/ping'), '/api/v1/ping');
  });

  test('空串与根路径', () {
    expect(routePattern(''), '');
    expect(routePattern('/'), '/');
  });
}
```

- [ ] **Step 3: 跑两个测试确认失败**

Run: `cd app/bottles && flutter test test/logging/log_retention_test.dart test/logging/route_pattern_test.dart`
Expected: 编译失败，`Undefined name 'planCleanup'` / `Undefined name 'routePattern'`

- [ ] **Step 4: 实现清理计划**

创建 `app/bottles/lib/core/logging/log_retention.dart`：

```dart
/// 一个日志文件的元信息。不含内容——清理只需要名字、大小、日期。
class LogFileInfo {
  const LogFileInfo({
    required this.name,
    required this.bytes,
    required this.day,
  });

  final String name;
  final int bytes;
  final DateTime day;
}

/// 计算应删除哪些日志文件。纯函数，不碰文件系统。
///
/// 三条规则按顺序应用：
///   1. 超过 [retainDays] 的整个删除
///   2. 删完仍超 [maxTotalBytes]，从最旧的继续删，直到降至上限内
///   3. **[activeFile] 永远不删** —— 边写边删会损坏文件
///
/// 第 3 条只在「当天日志已超总量上限」这一边界触发，实际极少遇到，
/// 但一旦遇到就是数据损坏，所以它是一条独立的守卫而不是顺带的判断。
List<String> planCleanup(
  List<LogFileInfo> files, {
  required int maxTotalBytes,
  required int retainDays,
  required DateTime now,
  required String activeFile,
}) {
  final doomed = <String>{};

  // 规则 1：超龄
  final cutoff = now.subtract(Duration(days: retainDays));
  for (final f in files) {
    if (f.name == activeFile) continue;
    if (f.day.isBefore(cutoff)) doomed.add(f.name);
  }

  // 规则 2：总量。按日期升序（最旧在前）逐个删，直到降到上限内。
  final remaining = files.where((f) => !doomed.contains(f.name)).toList()
    ..sort((a, b) => a.day.compareTo(b.day));
  var total = remaining.fold<int>(0, (s, f) => s + f.bytes);
  for (final f in remaining) {
    if (total <= maxTotalBytes) break;
    if (f.name == activeFile) continue; // 规则 3
    doomed.add(f.name);
    total -= f.bytes;
  }

  return doomed.toList();
}
```

- [ ] **Step 5: 实现路由模式**

创建 `app/bottles/lib/app/nav_logger.dart`（先只放纯函数，观察者在 Task 7 补）：

```dart
/// 纯数字的路径段视为 ID。
///
/// 要求整段都是数字：`v1` / `sha256` 这类是路径的一部分而不是 ID，
/// 用 `\d+` 不加锚点会把它们也吃掉。
final _idSegment = RegExp(r'^\d+$');

/// 把路径里的 ID 段换成占位。
///
/// `/ocean/bottle/1938274650283` → `/ocean/bottle/:id`
///
/// 记「用户当时在哪一页」对还原故障现场很有价值，但没必要把一串串业务 ID
/// 写进会被导出的日志里。
String routePattern(String path) {
  if (path.isEmpty) return path;
  return path
      .split('/')
      .map((s) => _idSegment.hasMatch(s) ? ':id' : s)
      .join('/');
}
```

- [ ] **Step 6: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/logging/log_retention_test.dart test/logging/route_pattern_test.dart`
Expected: PASS（6 + 5 = 11 个用例）

- [ ] **Step 7: 提交**

```bash
cd app/bottles && flutter analyze
git add app/bottles/lib/core/logging/log_retention.dart app/bottles/lib/app/nav_logger.dart app/bottles/test/logging/
git commit -m "feat(app): log retention planning and route pattern extraction"
```

---

## Task 4: 配置三层合并

**Files:**
- Create: `app/bottles/lib/core/logging/log_config.dart`
- Test: `app/bottles/test/logging/log_config_test.dart`

**Interfaces:**
- Consumes: Task 1 的 `LogLevel`
- Produces:
  - `class LogConfig({bool enabled, LogLevel minLevel, bool logBody, int maxMB, int retainDays})`
  - `const LogConfig.defaults()`
  - `LogConfig LogConfig.merge({required LogConfig base, Map<String, Object?>? server, bool? devForce})`

> 引导问题（spec §5.6）：**启动崩溃与登录失败最需要日志，而那时一个请求都没成功。**
> 所以配置必须有编译期默认值，服务端配置只是覆盖。

- [ ] **Step 1: 写失败的测试**

创建 `app/bottles/test/logging/log_config_test.dart`：

```dart
import 'package:bottles/core/logging/log_config.dart';
import 'package:bottles/core/logging/log_record.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('默认值：开启、info、不记 body、100MB、31 天', () {
    const c = LogConfig.defaults();
    expect(c.enabled, isTrue);
    expect(c.minLevel, LogLevel.info);
    expect(c.logBody, isFalse); // 默认不记 body 是安全前提,不能改
    expect(c.maxMB, 100);
    expect(c.retainDays, 31);
  });

  test('服务端配置覆盖默认值', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {
        'app_log_enabled': false,
        'app_log_level': 'warn',
        'app_log_body': true,
        'app_log_max_mb': 20,
        'app_log_retain_days': 7,
      },
    );
    expect(c.enabled, isFalse);
    expect(c.minLevel, LogLevel.warn);
    expect(c.logBody, isTrue);
    expect(c.maxMB, 20);
    expect(c.retainDays, 7);
  });

  test('服务端只给一部分时，其余保持原值', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_level': 'debug'},
    );
    expect(c.minLevel, LogLevel.debug);
    expect(c.maxMB, 100); // 没给就不动
  });

  test('服务端给了无法识别的级别时回退默认，不崩', () {
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_level': 'verbose'},
    );
    expect(c.minLevel, LogLevel.info);
  });

  test('本地开发者开关覆盖一切：强制 debug + 记 body', () {
    // 弥补「配置只能按租户」的限制:本机排查不该动线上配置。
    final c = LogConfig.merge(
      base: const LogConfig.defaults(),
      server: {'app_log_enabled': false, 'app_log_level': 'error'},
      devForce: true,
    );
    expect(c.enabled, isTrue);
    expect(c.minLevel, LogLevel.debug);
    expect(c.logBody, isTrue);
  });

  test('server 为 null 时等于不覆盖', () {
    final c = LogConfig.merge(base: const LogConfig.defaults(), server: null);
    expect(c.minLevel, LogLevel.info);
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/logging/log_config_test.dart`
Expected: 编译失败，`Undefined name 'LogConfig'`

- [ ] **Step 3: 实现**

创建 `app/bottles/lib/core/logging/log_config.dart`：

```dart
import 'log_record.dart';

/// 日志运行期配置。
///
/// 三层来源，后者覆盖前者：
///   1. [LogConfig.defaults] —— 编译期常量，冷启动第一毫秒可用
///   2. 上次收到并缓存在 Prefs 里的服务端配置 —— 启动时立即读取
///   3. 本次服务端下发 —— 登录或拉取资料后覆盖
///
/// 为什么必须有第 1 层：**启动崩溃与登录失败恰恰最需要日志，
/// 而那时一个请求都还没成功**。等配置回来再决定记不记，就什么都记不到。
class LogConfig {
  const LogConfig({
    required this.enabled,
    required this.minLevel,
    required this.logBody,
    required this.maxMB,
    required this.retainDays,
  });

  const LogConfig.defaults()
      : enabled = true,
        minLevel = LogLevel.info,
        // 默认不记 body 是整个系统的安全前提(spec 决策 2)。
        // 改这个默认值等于让每台手机都存着凭证,不要改。
        logBody = false,
        maxMB = 100,
        retainDays = 31;

  final bool enabled;
  final LogLevel minLevel;
  final bool logBody;
  final int maxMB;
  final int retainDays;

  int get maxTotalBytes => maxMB * 1024 * 1024;

  /// 合并服务端配置与本地开发者开关。
  ///
  /// [devForce] 为 true 时**覆盖一切**——包括服务端把日志关掉的情况。
  /// 它弥补的是「配置只能按租户」这个限制：本机排查不该动线上配置。
  static LogConfig merge({
    required LogConfig base,
    Map<String, Object?>? server,
    bool? devForce,
  }) {
    var c = base;
    if (server != null) {
      c = LogConfig(
        enabled: _bool(server['app_log_enabled']) ?? c.enabled,
        minLevel: _level(server['app_log_level']) ?? c.minLevel,
        logBody: _bool(server['app_log_body']) ?? c.logBody,
        maxMB: _int(server['app_log_max_mb']) ?? c.maxMB,
        retainDays: _int(server['app_log_retain_days']) ?? c.retainDays,
      );
    }
    if (devForce == true) {
      c = LogConfig(
        enabled: true,
        minLevel: LogLevel.debug,
        logBody: true,
        maxMB: c.maxMB,
        retainDays: c.retainDays,
      );
    }
    return c;
  }

  /// 服务端可能下发 bool、也可能下发 0/1 或 "0"/"1"——sysconfig 存的是字符串。
  static bool? _bool(Object? v) {
    if (v == null) return null;
    if (v is bool) return v;
    if (v is num) return v != 0;
    final s = v.toString();
    if (s.isEmpty) return null;
    return s != '0' && s.toLowerCase() != 'false';
  }

  static int? _int(Object? v) {
    if (v == null) return null;
    if (v is num) return v.toInt();
    return int.tryParse(v.toString());
  }

  /// 无法识别的级别回退 null（由调用方保持原值），**不抛异常**——
  /// 后台填错一个字不该让 App 起不来。
  static LogLevel? _level(Object? v) {
    if (v == null) return null;
    final s = v.toString().toLowerCase();
    for (final l in LogLevel.values) {
      if (l.name == s) return l;
    }
    return null;
  }
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/logging/log_config_test.dart`
Expected: PASS（6 个用例）

- [ ] **Step 5: 提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/lib/core/logging/log_config.dart app/bottles/test/logging/log_config_test.dart
git commit -m "feat(app): three-layer log configuration with compiled-in defaults"
```

---

## Task 5: Sink、文件落盘与 `Log` 门面

**Files:**
- Create: `app/bottles/lib/core/logging/log_sink.dart`
- Create: `app/bottles/lib/core/logging/log_file_sink.dart`
- Create: `app/bottles/lib/core/logging/logger.dart`

**Interfaces:**
- Consumes: Task 1 `LogRecord`/`LogLevel`/`LogTag`；Task 3 `planCleanup`/`LogFileInfo`；Task 4 `LogConfig`
- Produces:
  - `abstract class LogSink { Future<void> write(List<String> lines); Future<void> dispose(); }`
  - `class ConsoleSink implements LogSink`
  - `class FileLogSink implements LogSink`，含 `static Future<Directory> logDir()` 与 `Future<List<File>> files()`
  - `class Log`：`static Future<void> init({required LogConfig config})`、`static void d/i/w/e(...)`、`static Future<void> flush()`、`static void updateConfig(LogConfig)`、`static String get sessionId`

> 本任务**没有单元测试**——它全是文件 I/O 与定时器，本仓库不做这类测试
> （20 个 Go 测试与 1 个 widget 测试都不碰真实 I/O）。正确性靠 Task 1–4 的
> 纯函数 + Task 9 的手工验证。这一点在计划末尾的「已知薄弱处」里也记了。

- [ ] **Step 1: 写 `LogSink` 与 `ConsoleSink`**

创建 `app/bottles/lib/core/logging/log_sink.dart`：

```dart
import 'package:flutter/foundation.dart';

/// 日志出口。
///
/// 这个抽象就是将来接崩溃上报的接缝：`RemoteSink` 实现同一接口，
/// 与 `FileLogSink` 并列注册，采集侧一行都不用改（spec §6.3）。
abstract class LogSink {
  /// [lines] 已经是序列化好的 JSONL，每个元素一行、不含换行。
  Future<void> write(List<String> lines);

  Future<void> dispose();
}

/// 只在 debug 构建输出到控制台。
///
/// release 构建里什么都不做——控制台日志在用户手机上没人看，
/// 而 `debugPrint` 在高频调用下本身有开销。
class ConsoleSink implements LogSink {
  @override
  Future<void> write(List<String> lines) async {
    if (!kDebugMode) return;
    for (final l in lines) {
      debugPrint(l);
    }
  }

  @override
  Future<void> dispose() async {}
}
```

- [ ] **Step 2: 写 `FileLogSink`**

创建 `app/bottles/lib/core/logging/log_file_sink.dart`：

```dart
import 'dart:io';

import 'package:path_provider/path_provider.dart';

import 'log_config.dart';
import 'log_retention.dart';
import 'log_sink.dart';

/// JSONL 落盘 + 按天与大小轮转 + 清理。
///
/// 目录用 Application Support 而不是 Cache：Cache 会被系统在空间紧张时清掉，
/// 而那往往正是用户要报障的时刻（spec 决策 4）。
/// 云备份排除是**平台侧**的事，见 Task 6。
class FileLogSink implements LogSink {
  FileLogSink(this._config);

  LogConfig _config;
  Directory? _dir;
  File? _active;
  int _activeBytes = 0;

  /// 单文件上限。不让单文件无限增长：100MB 的单文件既无法在查看页翻阅，
  /// 也无法导出（spec §5.1）。
  static const _maxFileBytes = 5 * 1024 * 1024;

  void updateConfig(LogConfig c) => _config = c;

  static Future<Directory> logDir() async {
    final base = await getApplicationSupportDirectory();
    final d = Directory('${base.path}/logs');
    if (!await d.exists()) await d.create(recursive: true);
    return d;
  }

  /// 按文件名日期倒序（最新在前）返回全部日志文件。查看页与导出都用它。
  static Future<List<File>> listFiles() async {
    final d = await logDir();
    final fs = await d
        .list()
        .where((e) => e is File && e.path.endsWith('.log'))
        .cast<File>()
        .toList();
    fs.sort((a, b) => b.path.compareTo(a.path));
    return fs;
  }

  @override
  Future<void> write(List<String> lines) async {
    if (lines.isEmpty) return;
    try {
      final f = await _fileFor(DateTime.now());
      final payload = '${lines.join('\n')}\n';
      await f.writeAsString(payload, mode: FileMode.append, flush: false);
      _activeBytes += payload.length;
      await _cleanup();
    } catch (_) {
      // 日志永远不能成为故障源(Global Constraints)。写不进去就算了。
    }
  }

  /// 取当天的活跃文件，必要时轮转。
  Future<File> _fileFor(DateTime now) async {
    final d = _dir ??= await logDir();
    final day = '${now.year.toString().padLeft(4, '0')}-'
        '${now.month.toString().padLeft(2, '0')}-'
        '${now.day.toString().padLeft(2, '0')}';

    // 跨天或超过单文件上限就换一个。
    final needRoll = _active == null ||
        !_active!.path.contains(day) ||
        _activeBytes >= _maxFileBytes;
    if (!needRoll) return _active!;

    var i = 0;
    File candidate;
    while (true) {
      final name = i == 0 ? '$day.log' : '$day.$i.log';
      candidate = File('${d.path}/$name');
      final len = await candidate.exists() ? await candidate.length() : 0;
      if (len < _maxFileBytes) {
        _activeBytes = len;
        break;
      }
      i++;
    }
    _active = candidate;
    return candidate;
  }

  /// 清理在**每次刷盘后**做，而不是定时器：
  /// 定时器在 App 被杀后不跑，刷盘则一定发生（spec §5.3）。
  Future<void> _cleanup() async {
    final d = _dir;
    if (d == null) return;
    final infos = <LogFileInfo>[];
    for (final f in await listFiles()) {
      final name = f.uri.pathSegments.last;
      final day = DateTime.tryParse(name.substring(0, 10));
      if (day == null) continue;
      infos.add(LogFileInfo(name: name, bytes: await f.length(), day: day));
    }
    final activeName = _active?.uri.pathSegments.last ?? '';
    final doomed = planCleanup(
      infos,
      maxTotalBytes: _config.maxTotalBytes,
      retainDays: _config.retainDays,
      now: DateTime.now(),
      activeFile: activeName,
    );
    for (final name in doomed) {
      try {
        await File('${d.path}/$name').delete();
      } catch (_) {
        // 删不掉就下次再说。
      }
    }
  }

  @override
  Future<void> dispose() async {}
}
```

- [ ] **Step 3: 写 `Log` 门面**

创建 `app/bottles/lib/core/logging/logger.dart`：

```dart
import 'dart:async';
import 'dart:math';

import 'log_config.dart';
import 'log_file_sink.dart';
import 'log_record.dart';
import 'log_sink.dart';

/// 日志门面。
///
/// **这是本项目里唯一的全局单例**，其余一律走 Riverpod。理由：调用点包括
/// `main()` 里的全局错误处理器，那时 `ProviderScope` 还不存在；把 logger
/// 穿过每个构造函数传下去比一个全局更糟（spec 决策 11）。
class Log {
  Log._();

  static LogConfig _config = const LogConfig.defaults();
  static final List<LogSink> _sinks = [];
  static final List<LogRecord> _buffer = [];
  static Timer? _timer;
  static String _sessionId = '';
  static FileLogSink? _fileSink;

  /// 缓冲上限。有界是为了防止错误风暴把内存吃掉。
  static const _bufferCap = 500;

  /// 攒够这么多条就刷。
  static const _flushAt = 200;

  static const _flushEvery = Duration(seconds: 2);

  static String get sessionId => _sessionId;
  static LogConfig get config => _config;

  /// 每次冷启动生成一个短 ID，所有日志带上。
  /// 导出的日志里能一眼分清这是哪一次启动，而不是三天的记录混在一起。
  static Future<void> init({required LogConfig config}) async {
    _config = config;
    _sessionId = (Random().nextInt(0xFFFFFFF) + 0x1000000)
        .toRadixString(16)
        .padLeft(8, '0');
    _fileSink = FileLogSink(config);
    _sinks
      ..clear()
      ..addAll([ConsoleSink(), _fileSink!]);
    _timer?.cancel();
    _timer = Timer.periodic(_flushEvery, (_) => flush());
    i(LogTag.sys, 'session start', fields: {'sid': _sessionId});
  }

  static void updateConfig(LogConfig c) {
    _config = c;
    _fileSink?.updateConfig(c);
  }

  static void d(LogTag tag, String msg, {Map<String, Object?>? fields}) =>
      _add(LogLevel.debug, tag, msg, fields: fields);

  static void i(LogTag tag, String msg, {Map<String, Object?>? fields}) =>
      _add(LogLevel.info, tag, msg, fields: fields);

  static void w(LogTag tag, String msg, {Map<String, Object?>? fields}) =>
      _add(LogLevel.warn, tag, msg, fields: fields);

  static void e(
    LogTag tag,
    String msg, {
    Object? error,
    StackTrace? stack,
    Map<String, Object?>? fields,
  }) =>
      _add(LogLevel.error, tag, msg,
          fields: fields, error: error, stack: stack);

  static void _add(
    LogLevel level,
    LogTag tag,
    String msg, {
    Map<String, Object?>? fields,
    Object? error,
    StackTrace? stack,
  }) {
    if (!_config.enabled) return;
    if (level.index < _config.minLevel.index) return;

    _buffer.add(LogRecord(
      time: DateTime.now(),
      level: level,
      tag: tag,
      message: msg,
      fields: fields,
      error: error,
      stack: stack,
    ));

    // 缓冲满时**只丢 debug/info，永不丢 warn/error**。
    // 错误风暴恰恰是缓冲会满的场景,而那时最该保住的就是错误本身(spec §5.4)。
    while (_buffer.length > _bufferCap) {
      final victim = _buffer.indexWhere(
        (r) => r.level.index < LogLevel.warn.index,
      );
      if (victim == -1) break; // 全是 warn/error,宁可暂时超一点也不丢
      _buffer.removeAt(victim);
    }

    // error 立刻刷：崩溃前那几条最有价值。
    if (level == LogLevel.error || _buffer.length >= _flushAt) {
      unawaited(flush());
    }
  }

  /// 把缓冲刷给所有 sink。失败静默——日志不能成为故障源。
  static Future<void> flush() async {
    if (_buffer.isEmpty) return;
    final batch = List<LogRecord>.from(_buffer);
    _buffer.clear();
    final lines = <String>[];
    for (final r in batch) {
      try {
        lines.add(r.toJsonLine(_sessionId));
      } catch (_) {
        // 单条序列化失败不影响整批。
      }
    }
    for (final s in _sinks) {
      try {
        await s.write(lines);
      } catch (_) {}
    }
  }
}
```

- [ ] **Step 4: 编译验证**

Run: `cd app/bottles && flutter analyze && flutter test`
Expected: `No issues found!`，已有测试全过

- [ ] **Step 5: 提交**

```bash
git add app/bottles/lib/core/logging/
git commit -m "feat(app): buffered file sink and the Log facade"
```

---

## Task 6: 平台侧——排除云备份

**Files:**
- Modify: `app/bottles/ios/Runner/AppDelegate.swift`
- Create: `app/bottles/android/app/src/main/res/xml/data_extraction_rules.xml`
- Create: `app/bottles/android/app/src/main/res/xml/backup_rules.xml`
- Modify: `app/bottles/android/app/src/main/AndroidManifest.xml`

**Interfaces:**
- Consumes: Task 5 的目录约定（`<Application Support>/logs`）
- Produces: 无代码接口，纯平台配置

> ⚠️ **iOS 这一段无法在当前开发机验证**（Windows 上 `flutter build ipa` 子命令
> 都不存在）。按标准模式写，需 macOS + Xcode 确认。

- [ ] **Step 1: Android 备份规则**

创建 `app/bottles/android/app/src/main/res/xml/data_extraction_rules.xml`（Android 12+）：

```xml
<?xml version="1.0" encoding="utf-8"?>
<!-- 日志不进云备份：它只服务于本机排查，跟着用户上 Google Drive
     或在换机时被带走，既无意义又扩大了暴露面。 -->
<data-extraction-rules>
    <cloud-backup>
        <exclude domain="file" path="logs/" />
    </cloud-backup>
    <device-transfer>
        <exclude domain="file" path="logs/" />
    </device-transfer>
</data-extraction-rules>
```

创建 `app/bottles/android/app/src/main/res/xml/backup_rules.xml`（Android 11 及以下）：

```xml
<?xml version="1.0" encoding="utf-8"?>
<full-backup-content>
    <exclude domain="file" path="logs/" />
</full-backup-content>
```

- [ ] **Step 2: Manifest 引用**

`app/bottles/android/app/src/main/AndroidManifest.xml` 的 `<application>` 标签加两个属性：

```xml
        android:dataExtractionRules="@xml/data_extraction_rules"
        android:fullBackupContent="@xml/backup_rules"
```

- [ ] **Step 3: iOS 排除备份标记**

`app/bottles/ios/Runner/AppDelegate.swift`，在 `didFinishLaunchingWithOptions` 里、`return super...` 之前加：

```swift
    // 日志目录排除 iCloud 备份。
    //
    // Application Support 默认会进 iCloud，日志跟着上云既无意义又扩大暴露面，
    // 而且换机恢复时会被带过去。path_provider 不暴露这个属性，只能在这里打标记。
    //
    // 目录可能还不存在（首次启动时 Dart 侧尚未创建），所以先建再标记；
    // 失败只打日志不中断启动——排不掉备份不该让 App 起不来。
    if let support = FileManager.default.urls(
      for: .applicationSupportDirectory, in: .userDomainMask).first {
      var logs = support.appendingPathComponent("logs", isDirectory: true)
      do {
        try FileManager.default.createDirectory(
          at: logs, withIntermediateDirectories: true)
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try logs.setResourceValues(values)
      } catch {
        NSLog("[log] 排除备份失败: \(error)")
      }
    }
```

- [ ] **Step 4: 构建验证（仅 Android）**

Run: `cd app/bottles && flutter build apk --debug`
Expected: 构建成功。Android 的备份规则若 XML 有误会在资源编译阶段直接报错，所以构建通过即说明格式正确。

- [ ] **Step 5: 提交**

```bash
git add app/bottles/android app/bottles/ios
git commit -m "feat(app): keep logs out of cloud backup on both platforms"
```

---

## Task 7: 采集点接入

**Files:**
- Create: `app/bottles/lib/core/network/logging_interceptor.dart`
- Modify: `app/bottles/lib/core/network/api_client.dart`
- Modify: `app/bottles/lib/app/nav_logger.dart`（补观察者）
- Modify: `app/bottles/lib/app/router.dart`
- Modify: `app/bottles/lib/main.dart`

**Interfaces:**
- Consumes: Task 1 `LogTag`；Task 2 `redactMap`/`redactHeaders`；Task 3 `routePattern`；Task 5 `Log`
- Produces:
  - `class LoggingInterceptor extends Interceptor`
  - `class NavLogger extends NavigatorObserver`

- [ ] **Step 1: Dio 拦截器**

创建 `app/bottles/lib/core/network/logging_interceptor.dart`：

```dart
import 'package:dio/dio.dart';

import '../logging/log_record.dart';
import '../logging/log_redactor.dart';
import '../logging/logger.dart';

/// 记录每一次网络请求。
///
/// **要点：业务错误码不是 HTTP 错误。** `{"code":3003}` 走的是 HTTP 200，
/// `onError` 根本不触发，所以必须在 `onResponse` 里解一层 `data['code']`。
/// 一条日志同时带两者——只看 HTTP 全是 200 看不出业务失败，
/// 只看业务码又看不出超时与连接失败（spec §4.1）。
///
/// 它挂在 `ApiClient._request` 的 try/catch **下面**，因此能看到完整的
/// `DioException`——那些信息在上层会被压成一句 message 丢掉。
class LoggingInterceptor extends Interceptor {
  static const _startKey = 'log_start_ms';

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    options.extra[_startKey] = DateTime.now().millisecondsSinceEpoch;
    handler.next(options);
  }

  int _elapsed(RequestOptions o) {
    final start = o.extra[_startKey];
    if (start is! int) return -1;
    return DateTime.now().millisecondsSinceEpoch - start;
  }

  Map<String, Object?> _base(RequestOptions o) => {
        'm': o.method,
        'path': o.path,
        'ms': _elapsed(o),
      };

  @override
  void onResponse(Response<dynamic> res, ResponseInterceptorHandler handler) {
    final f = _base(res.requestOptions)..['http'] = res.statusCode;

    final body = res.data;
    final code = body is Map ? (body['code'] as num?)?.toInt() : null;
    if (code != null) f['code'] = code;

    if (Log.config.logBody) {
      f['req'] = _safeBody(res.requestOptions.data);
      f['res'] = _safeBody(body);
      f['hdr'] = redactHeaders(res.requestOptions.headers);
    }

    final msg = '${res.requestOptions.method} ${res.requestOptions.path}';
    // 业务码非 0 是 warn 而不是 info:它是失败,只是 HTTP 层没体现。
    if (code != null && code != 0) {
      Log.w(LogTag.net, msg, fields: f);
    } else {
      Log.i(LogTag.net, msg, fields: f);
    }
    handler.next(res);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final f = _base(err.requestOptions)
      ..['http'] = err.response?.statusCode
      ..['type'] = err.type.name;
    if (Log.config.logBody) {
      f['res'] = _safeBody(err.response?.data);
    }
    Log.e(
      LogTag.net,
      '${err.requestOptions.method} ${err.requestOptions.path}',
      error: err.message,
      fields: f,
    );
    handler.next(err);
  }

  /// body 一律过脱敏，并截断——单条日志不该因为一个大响应撑到几百 KB。
  Object? _safeBody(Object? body) {
    if (body == null) return null;
    if (body is Map) {
      return redactMap(body.map((k, v) => MapEntry(k.toString(), v)));
    }
    final s = body.toString();
    return s.length > 2000 ? '${s.substring(0, 2000)}…' : s;
  }
}
```

- [ ] **Step 2: 挂到 `ApiClient`**

`app/bottles/lib/core/network/api_client.dart`，在现有 `InterceptorsWrapper` 之后加一行（顺序重要：auth 拦截器先跑，日志拦截器才能看到带上 token 后的头）：

```dart
    _dio.interceptors.add(LoggingInterceptor());
```

并加 import `import 'logging_interceptor.dart';`。

- [ ] **Step 3: 导航观察者**

`app/bottles/lib/app/nav_logger.dart` 追加（`routePattern` 已在 Task 3 写好）：

```dart
import 'package:flutter/widgets.dart';

import '../core/logging/log_record.dart';
import '../core/logging/logger.dart';

/// 记录页面跳转，为孤立的错误提供「用户当时在干什么」的上下文。
///
/// 只记路由模式不记实参——见 [routePattern]。
class NavLogger extends NavigatorObserver {
  void _log(String action, Route<dynamic>? route) {
    final name = route?.settings.name;
    if (name == null || name.isEmpty) return;
    Log.i(LogTag.nav, action, fields: {'route': routePattern(name)});
  }

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) =>
      _log('push', route);

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) =>
      _log('pop', route);

  @override
  void didReplace({Route<dynamic>? newRoute, Route<dynamic>? oldRoute}) =>
      _log('replace', newRoute);
}
```

- [ ] **Step 4: 挂 observer 与登录态日志**

`app/bottles/lib/app/router.dart`：

① `GoRouter(` 的参数里加：

```dart
    observers: [NavLogger()],
```

② 已有的 `ref.listen<AuthState>` 在桥接登录态给 go_router，加一行日志即可，**不要新起一个监听**。它现在是表达式体的 lambda，得先改成块体：

```dart
  ref.listen<AuthState>(authProvider, (_, next) {
    refresh.value = next.status;
    Log.i(LogTag.auth, 'auth status', fields: {'status': next.status.name});
  });
```

③ 加 import：`import '../core/logging/log_record.dart';`、`import '../core/logging/logger.dart';`、`import 'nav_logger.dart';`

- [ ] **Step 5: 全局错误钩子**

`app/bottles/lib/main.dart` 改为：

```dart
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/app.dart';
import 'core/logging/log_config.dart';
import 'core/logging/log_record.dart';
import 'core/logging/logger.dart';
import 'core/providers.dart';
import 'core/storage/prefs.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 偏好设置（语言 / 外观 / 筛选条件）在首帧前读好，
  // 避免整个应用为了一个异步初始化多套一层 loading。
  final prefs = await Prefs.load();

  // 日志要在任何业务代码之前就绪：**启动崩溃与登录失败恰恰最需要它**。
  // 配置先用编译期默认值 + 上次缓存的服务端值，本次下发的值稍后覆盖。
  await Log.init(
    config: LogConfig.merge(
      base: const LogConfig.defaults(),
      server: prefs.logConfig,
      devForce: prefs.logDevForce,
    ),
  );

  // 两个钩子覆盖完整，**不用 runZonedGuarded**：多一层 zone 要求 runApp
  // 在同一 zone 内调用，否则 Zone mismatch；与 ensureInitialized 的顺序
  // 也很讲究，配错的表现是启动即崩（spec 决策 10）。
  final prevOnError = FlutterError.onError;
  FlutterError.onError = (details) {
    Log.e(LogTag.sys, 'flutter error',
        error: details.exception, stack: details.stack);
    // 同步刷盘再放行——崩溃前那几条最有价值。
    Log.flush();
    prevOnError?.call(details);
  };

  PlatformDispatcher.instance.onError = (error, stack) {
    Log.e(LogTag.sys, 'uncaught async', error: error, stack: stack);
    Log.flush();
    return false; // 不吞掉,继续走默认处理
  };

  runApp(
    ProviderScope(
      overrides: [prefsProvider.overrideWithValue(prefs)],
      child: const BottlesApp(),
    ),
  );
}
```

- [ ] **Step 6: `Prefs` 加两项**

`app/bottles/lib/core/storage/prefs.dart`：

```dart
  static const _kLogConfig = 'drift.logConfig';
  static const _kLogDevForce = 'drift.logDevForce';
```

```dart
  /// 上次收到的服务端日志配置（JSON）。
  ///
  /// 缓存它是为了让「运营昨天关掉了日志」在今天冷启动时就生效，
  /// 而不必等第一个请求成功——那时可能已经崩了。
  Map<String, Object?>? get logConfig {
    final s = _sp.getString(_kLogConfig);
    if (s == null || s.isEmpty) return null;
    try {
      return jsonDecode(s) as Map<String, Object?>;
    } catch (_) {
      return null;
    }
  }

  Future<void> setLogConfig(Map<String, Object?> v) =>
      _sp.setString(_kLogConfig, jsonEncode(v));

  /// 本地开发者开关。覆盖服务端配置，只影响本机。
  /// 弥补「配置只能按租户」的限制：本机排查不该动线上配置。
  bool get logDevForce => _sp.getBool(_kLogDevForce) ?? false;
  Future<void> setLogDevForce(bool v) => _sp.setBool(_kLogDevForce, v);
```

加 import `dart:convert`。

- [ ] **Step 7: 业务动作 6 处**

在下列位置各加一行 `Log.i(LogTag.biz, …)`：

| 文件 | 位置 | 记什么 |
|---|---|---|
| `lib/features/bottle/write_bottle_page.dart` | `_cast()` 成功后 | `{'act':'throw','hasPlace': _place != null}` |
| `lib/features/bottle/ocean_controller.dart` | 捞瓶成功后 | `{'act':'scoop'}` |
| `lib/features/bottle/bottle_detail_page.dart` | 解锁回信成功后 | `{'act':'unlock'}` |
| `lib/features/chat/chat_controller.dart` | 送礼成功后 | `{'act':'gift'}` |
| `lib/features/me/recharge_page.dart` | `_pay()` 发起时 | `{'act':'pay_start','pkg': pkg.id}` |
| `lib/features/me/recharge_page.dart` | 支付结果回来后 | `{'act':'pay_result','ok': ok}` |

**不记金额与商品名之外的任何用户数据。** 这几处是为了出纠纷时能对账，不是行为分析。

- [ ] **Step 8: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/lib
git commit -m "feat(app): wire logging into network, errors, navigation and key actions"
```

---

## Task 8: 服务端配置下发

**Files:**
- Modify: `server/internal/sysconfig/sysconfig.go`
- Modify: `server/internal/admin/meta.go`
- Modify: `server/internal/user/handler.go:306`

**Interfaces:**
- Consumes: 无
- Produces: `clientConfig()` 响应新增 5 个字段

- [ ] **Step 1: 加 key 与 defaults**

`server/internal/sysconfig/sysconfig.go`，在 App 相关那一组之后加：

```go
	// App 日志（客户端本地日志系统，见 docs/superpowers/specs/2026-09-19-app-logging-design.md）
	KeyAppLogEnabled     = "app_log_enabled"      // 总开关 (0/1)
	KeyAppLogLevel       = "app_log_level"        // debug/info/warn/error
	KeyAppLogBody        = "app_log_body"         // 是否记录请求/响应体 (0/1)
	KeyAppLogMaxMB       = "app_log_max_mb"       // 本地日志总量上限(MB)
	KeyAppLogRetainDays  = "app_log_retain_days"  // 本地日志保留天数
```

defaults 里加（**五个都要写**，空串会导致开关逻辑反转）：

```go
	KeyAppLogEnabled:    "1",
	KeyAppLogLevel:      "info",
	// 默认不记 body:请求体里有密码、验证码、ID Token,
	// 而日志会被用户导出发给客服——那条路径不可控。排查时临时打开,用完关掉。
	KeyAppLogBody:       "0",
	KeyAppLogMaxMB:      "100",
	KeyAppLogRetainDays: "31",
```

- [ ] **Step 2: 登记后台白名单**

`server/internal/admin/meta.go` 加一组：

```go
	{Key: sysconfig.KeyAppLogEnabled, Label: "App 日志总开关", Group: "App 日志", Type: "bool"},
	{Key: sysconfig.KeyAppLogLevel, Label: "最低记录级别(debug/info/warn/error)", Group: "App 日志", Type: "text"},
	{Key: sysconfig.KeyAppLogBody, Label: "记录请求/响应体(排查时临时开,用完关)", Group: "App 日志", Type: "bool"},
	{Key: sysconfig.KeyAppLogMaxMB, Label: "本地日志总量上限(MB)", Group: "App 日志", Type: "int"},
	{Key: sysconfig.KeyAppLogRetainDays, Label: "本地日志保留天数", Group: "App 日志", Type: "int"},
```

- [ ] **Step 3: 加进 `clientConfig`**

`server/internal/user/handler.go` 的 `clientConfig()`（第 306 行）返回的 map 里加：

```go
		"app_log_enabled":     sysconfig.GetBool(tenantID, sysconfig.KeyAppLogEnabled),
		"app_log_level":       sysconfig.GetString(tenantID, sysconfig.KeyAppLogLevel),
		"app_log_body":        sysconfig.GetBool(tenantID, sysconfig.KeyAppLogBody),
		"app_log_max_mb":      sysconfig.GetInt(tenantID, sysconfig.KeyAppLogMaxMB),
		"app_log_retain_days": sysconfig.GetInt(tenantID, sysconfig.KeyAppLogRetainDays),
```

`clientConfig` 在登录与 `/user/profile` 两处都会下发，所以**改配置不需要用户重新登录**。

- [ ] **Step 4: 客户端消费**

`app/bottles/lib/data/remote/remote_repositories.dart` 的 `fetchProfile()` 与登录响应解析处，把这 5 个字段存进 `Prefs.setLogConfig(...)` 并调 `Log.updateConfig(...)`。

具体做法：在 `RemoteAuthRepository` 里新增一个私有方法，登录与拉资料两处各调一次：

```dart
  /// clientConfig 里的日志配置：缓存下来供下次冷启动用，并立即生效。
  void _applyLogConfig(Map<String, dynamic> m) {
    const keys = [
      'app_log_enabled', 'app_log_level', 'app_log_body',
      'app_log_max_mb', 'app_log_retain_days',
    ];
    final cfg = <String, Object?>{};
    for (final k in keys) {
      if (m.containsKey(k)) cfg[k] = m[k];
    }
    if (cfg.isEmpty) return;
    onLogConfig?.call(cfg);
  }
```

`onLogConfig` 是构造时注入的回调，由 `providers.dart` 接到 `Prefs.setLogConfig` + `Log.updateConfig` —— 这样 `data/` 层不直接依赖 `Prefs` 与 `Log`，保持原有分层。

- [ ] **Step 5: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test $(go list ./... | grep -v /internal/robot)
cd ../app/bottles && flutter analyze && flutter test
git add server/internal app/bottles/lib
git commit -m "feat(log): deliver app log settings through clientConfig"
```

---

## Task 9: 出口——导出与查看页

**Files:**
- Create: `app/bottles/lib/core/logging/log_export.dart`
- Create: `app/bottles/lib/features/me/log_viewer_page.dart`
- Modify: `app/bottles/lib/features/me/settings_page.dart`
- Modify: `app/bottles/lib/app/routes.dart` · `router.dart`
- Test: `app/bottles/test/logging/log_export_test.dart`

**Interfaces:**
- Consumes: Task 1 `LogRecord`；Task 5 `FileLogSink.listFiles()`
- Produces:
  - `String formatLogLine(Map<String, Object?> json)`
  - `Future<File> buildExportFile({int maxDays = 3, int maxBytes = 5 * 1024 * 1024})`
  - `Routes.logViewer`

- [ ] **Step 1: 写格式化的失败测试**

创建 `app/bottles/test/logging/log_export_test.dart`：

```dart
import 'package:bottles/core/logging/log_export.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('JSONL 转成人能读的一行', () {
    final s = formatLogLine({
      't': '2026-09-19T17:23:41.882+08:00',
      'lv': 'error',
      'tag': 'net',
      'msg': 'POST /bottle/create',
      'http': 200,
      'code': 3003,
      'ms': 412,
      'sid': 'a3f27b19',
    });
    expect(s, contains('17:23:41'));
    expect(s, contains('[ERROR]'));
    expect(s, contains('[net]'));
    expect(s, contains('POST /bottle/create'));
    expect(s, contains('code=3003'));
    expect(s, contains('412ms'));
    // sid 不重复出现在每一行——导出文件头部写一次就够。
    expect(s, isNot(contains('a3f27b19')));
  });

  test('缺字段时不崩，能出多少出多少', () {
    final s = formatLogLine({'msg': 'bare'});
    expect(s, contains('bare'));
  });

  test('坏行原样返回，不吞掉', () {
    // 文件可能被写坏(断电/杀进程),这时宁可显示原始内容也不要静默丢弃。
    final s = formatLogLine({'raw': '{不是合法 JSON'});
    expect(s, contains('不是合法 JSON'));
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/bottles && flutter test test/logging/log_export_test.dart`
Expected: 编译失败，`Undefined name 'formatLogLine'`

- [ ] **Step 3: 实现格式化与导出**

创建 `app/bottles/lib/core/logging/log_export.dart`：

```dart
import 'dart:convert';
import 'dart:io';

import 'log_file_sink.dart';

/// JSONL 一行 → 人能读的一行。
///
/// 存储用 JSONL（机器读、便于过滤与将来上传），导出转成文本——
/// 发给客服的东西不能是一堆 JSON。存储与呈现分开，两边各自最优。
String formatLogLine(Map<String, Object?> json) {
  if (json.containsKey('raw')) return json['raw'].toString();

  final t = json['t']?.toString() ?? '';
  // 只取时:分:秒。日期在文件名和导出头部都有，每行再带一次是噪声。
  final hhmmss = t.length >= 19 ? t.substring(11, 19) : t;
  final lv = (json['lv']?.toString() ?? 'info').toUpperCase();
  final tag = json['tag']?.toString() ?? '-';
  final msg = json['msg']?.toString() ?? '';

  final extra = <String>[];
  for (final k in ['http', 'code', 'type', 'route', 'act']) {
    if (json[k] != null) extra.add('$k=${json[k]}');
  }
  if (json['ms'] != null) extra.add('${json['ms']}ms');
  if (json['err'] != null) extra.add('err=${json['err']}');

  final tail = extra.isEmpty ? '' : '  ${extra.join(' ')}';
  return '$hhmmss [$lv][$tag] $msg$tail';
}

/// 生成导出文件。
///
/// 只取最近 [maxDays] 天或 [maxBytes]（取小者）：100 MB 发给客服没人看得了，
/// 微信也发不动（spec §6.1）。
Future<File> buildExportFile({
  int maxDays = 3,
  int maxBytes = 5 * 1024 * 1024,
}) async {
  final files = await FileLogSink.listFiles(); // 已按日期倒序
  final cutoff = DateTime.now().subtract(Duration(days: maxDays));
  final buf = StringBuffer()
    ..writeln('Drift 日志导出')
    ..writeln('生成时间: ${DateTime.now()}')
    ..writeln('范围: 最近 $maxDays 天 / 最多 ${maxBytes ~/ 1024 ~/ 1024} MB')
    ..writeln('---');

  var used = 0;
  for (final f in files) {
    final name = f.uri.pathSegments.last;
    final day = DateTime.tryParse(name.substring(0, 10));
    if (day != null && day.isBefore(cutoff)) break;
    for (final line in await f.readAsLines()) {
      if (used >= maxBytes) break;
      Map<String, Object?> m;
      try {
        m = jsonDecode(line) as Map<String, Object?>;
      } catch (_) {
        m = {'raw': line};
      }
      final s = formatLogLine(m);
      buf.writeln(s);
      used += s.length;
    }
    if (used >= maxBytes) break;
  }

  final dir = await FileLogSink.logDir();
  final out = File('${dir.path}/export.txt');
  await out.writeAsString(buf.toString());
  return out;
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/bottles && flutter test test/logging/log_export_test.dart`
Expected: PASS（3 个用例）

- [ ] **Step 5: 设置页加导出入口与开发者开关**

`app/bottles/lib/features/me/settings_page.dart`：

⓪ **先把 `SettingsPage` 从 `ConsumerWidget` 改成 `ConsumerStatefulWidget`** —— 连点计数要存状态，现在这个页面是无状态的。`build(context, ref)` 的 `ref` 变成 `this.ref`，`_confirm` 留在 State 里，签名不动。

① 在「安全」那组之后加一个可见入口：

```dart
              ListRowItem(
                title: '导出日志',
                leadingEmoji: '🧾',
                subtitle: '遇到问题时发给客服，帮助定位',
                onTap: () => _exportLogs(context),
              ),
```

② `_exportLogs` 先弹确认，**列出里面有什么**——这是日志离开设备的唯一路径，用户有权知道自己在发什么；万一 body 开关开着，这是最后一道提醒。

复用页面里已有的 `_confirm`（签名 `(context, {title, body, confirmLabel, danger})`，返回 `Future<bool>` 而不是 `bool?`）：

```dart
  Future<void> _exportLogs(BuildContext context) async {
    final withBody = Log.config.logBody;
    final ok = await _confirm(
      context,
      title: '导出日志',
      body: '将导出最近 3 天的运行记录（最多 5 MB），用于排查问题。\n\n'
          '内容包含：访问过的接口、错误信息、页面跳转。\n'
          '${withBody ? '⚠️ 当前设置下还包含请求内容。\n' : ''}'
          '不包含：密码、验证码、登录凭证。',
      confirmLabel: '导出并分享',
    );
    if (!ok) return;
    final file = await buildExportFile();
    await SharePlus.instance.share(
      ShareParams(files: [XFile(file.path, mimeType: 'text/plain')]),
    );
  }
```

③ `meAbout` 那一行现在是 `onTap: () {}`，改成连点 7 次开启开发者面板：

```dart
              ListRowItem(
                title: l.meAbout,
                leadingEmoji: 'ℹ️',
                trailingText: 'v1.0.0',
                showChevron: false,
                // 连点 7 次开启开发者面板。藏起来是因为里面的开关
                // （强制 debug 级别 + 记录请求体）不该被普通用户误触。
                onTap: () {
                  if (++_aboutTaps < 7) return;
                  _aboutTaps = 0;
                  context.push(Routes.logViewer);
                },
              ),
```

在 ⓪ 新建的 `_SettingsPageState` 里加字段 `int _aboutTaps = 0;`。

- [ ] **Step 6: 查看页**

创建 `app/bottles/lib/features/me/log_viewer_page.dart`：

- 顶部：级别下拉 + 标签下拉 + 关键词输入
- 列表：**只读最新文件的最后 500 行**，翻到底「加载更多」
- 底部：一个「强制详细日志（仅本机）」开关，写 `Prefs.setLogDevForce` 并调 `Log.updateConfig`

**性能约束必须遵守**：5 MB JSONL 约两万行，全量读入再渲染会卡死。用 `readAsLines()` 后取尾部 500 行，过滤时重新扫描而不是把全部驻留内存。

- [ ] **Step 7: 路由注册**

`routes.dart` 加 `static const logViewer = '/me/log-viewer';`，`router.dart` 在 `me` 分支下注册。

- [ ] **Step 8: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test && flutter build apk --debug
git add app/bottles/lib app/bottles/test
git commit -m "feat(app): export logs for support and view them in-app"
```

- [ ] **Step 9: 端到端手工验证**

在 `Pixel_7_API_36` 模拟器上：

1. 冷启动 → 进「我的 → 设置 → 关于」连点 7 次 → 打开查看页 → 应看到 `session start` 与若干 `nav` / `net` 记录
2. 断网后下拉刷新 → 查看页应出现 `error` 级 `net` 记录，`type=connectionError`
3. 故意触发一次配额用尽（连捞到次数耗尽）→ 应出现 `warn` 级、`code=3003`
4. **搜 `Bearer`、搜自己的密码 → 必须一条都搜不到**（脱敏验证，这一步不能跳）
5. 打开「强制详细日志」→ 再发一次请求 → 应看到 `req`/`res` 字段，且里面的 `password` 是 `***`
6. 设置页「导出日志」→ 确认弹窗应提示「还包含请求内容」→ 分享到自己的微信 → 打开确认是可读文本
7. 杀掉 App 再启动 → 上次的日志仍在（验证落盘与 Application Support 位置）

---

## 自查

**Spec 覆盖**：

| Spec 要求 | 落点 |
|---|---|
| §3.2 `LogRecord` + JSONL | Task 1 |
| §5.8 脱敏（含 Authorization 硬规则） | Task 2 |
| §5.3 清理三规则 | Task 3 |
| §4.3 路由模式 | Task 3 |
| §5.5/§5.6 配置三层 | Task 4 + Task 8 |
| §5.4 缓冲两规则 | Task 5 |
| §5.1 目录与轮转 | Task 5 |
| §5.2 排除云备份 | Task 6 |
| §4.1 网络拦截器（含业务码） | Task 7 |
| §4.2 两个错误钩子、不用 zone | Task 7 |
| §4.4 登录态 | Task 7 |
| §4.5 业务动作 6 处 | Task 7 |
| §6.1 导出 + 确认屏 | Task 9 |
| §6.2 查看页 + 尾部 500 行 | Task 9 |
| §5.7 本地开发者开关 | Task 9 |
| §6.3 上报接缝（`LogSink`） | Task 5（抽象已就位，不实现远端） |

**不在本计划**（spec §七遗留）：崩溃上报服务接入；按用户的配置粒度；`X-Request-Id` 客户端与服务端关联；服务端入站请求日志。

**已知薄弱处**：

1. **Task 5 全无单元测试** —— 它是文件 I/O 与定时器，本仓库不做这类测试。正确性靠 Task 1–4 的纯函数与 Task 9 Step 9 的手工验证。**那 7 步不要跳，尤其第 4 步（搜 Bearer 与密码）。**
2. **Task 6 的 iOS 部分无法在当前开发机验证** —— Windows 上出不了 iOS 包。Android 侧靠构建通过验证（XML 有误会在资源编译阶段直接报错）。
3. **查看页的性能只在模拟器上验证** —— 真机低端机上两万行文件的读取耗时可能更长，若卡顿需改成分块读取。
