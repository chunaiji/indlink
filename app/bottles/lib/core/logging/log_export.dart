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
    if (name.length < 10) continue;
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
