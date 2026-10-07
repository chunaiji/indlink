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
        // 默认不记 body 是整个系统的安全前提（spec 决策 2）。
        // 改这个默认值等于让每台手机都存着凭证，不要改。
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

  /// 服务端可能下发 bool，也可能下发 0/1 或 "0"/"1"——sysconfig 存的是字符串。
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
