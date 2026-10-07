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

  /// 业务动作（扔瓶 / 捞瓶 / 解锁 / 送礼 / 支付）
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
