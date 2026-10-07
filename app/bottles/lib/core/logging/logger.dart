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
    // 固定 8 位十六进制。拼两段 16 位而不是 `nextInt(1 << 32)`：
    // 后者超出 `nextInt` 在 web 上的取值范围，这里不冒那个险。
    final r = Random();
    _sessionId = r.nextInt(1 << 16).toRadixString(16).padLeft(4, '0') +
        r.nextInt(1 << 16).toRadixString(16).padLeft(4, '0');
    _fileSink = FileLogSink(config);
    _sinks
      ..clear()
      ..addAll([ConsoleSink(), _fileSink!]);
    _timer?.cancel();
    _timer = Timer.periodic(_flushEvery, (_) => flush());
    i(LogTag.sys, 'session start');
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
    // 错误风暴恰恰是缓冲会满的场景，而那时最该保住的就是错误本身（spec §5.4）。
    while (_buffer.length > _bufferCap) {
      final victim = _buffer.indexWhere(
        (r) => r.level.index < LogLevel.warn.index,
      );
      if (victim == -1) break; // 全是 warn/error，宁可暂时超一点也不丢
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
