import 'dart:io';

import 'package:flutter/foundation.dart';

import 'package:path_provider/path_provider.dart';

import 'log_config.dart';
import 'log_retention.dart';
import 'log_sink.dart';

/// JSONL 落盘 + 按天与大小轮转 + 清理。
///
/// 目录用 Application Support 而不是 Cache：Cache 会被系统在空间紧张时清掉，
/// 而那往往正是用户要报障的时刻（spec 决策 4）。
/// 云备份排除是**平台侧**的事，见 Android 的 backup rules 与 iOS AppDelegate。
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
  ///
  /// 排除 `export.txt`——它是导出产物而非日志，扩展名判断已经挡掉了，
  /// 这里再点一句是因为那个文件就躺在同一个目录里。
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
    } catch (e) {
      // 日志永远不能成为故障源：写不进去就算了，绝不向上抛。
      // 但 debug 构建里要吼一声——一个静默失败的日志系统没法排查它自己。
      if (kDebugMode) debugPrint('[log] 写盘失败: $e');
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
      if (name.length < 10) continue;
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
