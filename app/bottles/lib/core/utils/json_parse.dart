/// 后端一律 UTC 下发，本地渲染时才 `toLocal()`。
///
/// 印度是 IST（UTC+5:30，带半小时偏移），后端 KPI 又是东八区口径，
/// 三套时区混用必出「跨天」错位——所以模型层只认 UTC。
DateTime? utcOrNull(Object? v) {
  if (v == null) return null;
  if (v is int) {
    return DateTime.fromMillisecondsSinceEpoch(v * 1000, isUtc: true);
  }
  return DateTime.tryParse('$v')?.toUtc();
}

DateTime utcOrEpoch(Object? v) =>
    utcOrNull(v) ?? DateTime.fromMillisecondsSinceEpoch(0, isUtc: true);

List<String> stringList(Object? v) {
  if (v is List) return v.map((e) => '$e').toList(growable: false);
  if (v is String && v.isNotEmpty) return v.split(',');
  return const [];
}

/// 硬约束：ID 一律字符串。后端即使误下发数字也在这里收口。
String idOf(Object? v) => v == null ? '' : '$v';

int intOf(Object? v, [int fallback = 0]) => (v as num?)?.toInt() ?? fallback;
