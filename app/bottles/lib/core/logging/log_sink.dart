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
