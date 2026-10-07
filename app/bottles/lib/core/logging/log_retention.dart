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
