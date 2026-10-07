import 'package:flutter/widgets.dart';

import '../../l10n/app_localizations.dart';

/// 时间的两种口径，**不能混用**：
///
/// * 瓶子说「漂了 3 天」——时长，[driftedFor]
/// * 回信说「12 分钟前」——时刻，[timeAgo]
///
/// 异步是这个产品的本质，时间是它唯一能被看见的形式。
/// 一律 UTC 存、按用户本地时区渲染：印度是 IST（UTC+5:30，带半小时偏移），
/// 后端 KPI 又是东八区口径，三套时区混用必出跨天错位。
String timeAgo(BuildContext context, DateTime utc) {
  final l = L.of(context);
  final diff = DateTime.now().difference(utc.toLocal());

  if (diff.inMinutes < 1) return l.commonJustNow;
  if (diff.inMinutes < 60) return l.commonMinutesAgo(diff.inMinutes);
  if (diff.inHours < 24) return l.commonHoursAgo(diff.inHours);
  if (diff.inDays == 1) return l.commonYesterday;
  if (diff.inDays < 7) return l.commonDaysAgo(diff.inDays);
  if (diff.inDays < 14) return l.commonLastWeek;
  return _date(utc.toLocal());
}

String driftedFor(BuildContext context, DateTime utc) =>
    L.of(context).commonDriftedDays(
          DateTime.now().difference(utc.toLocal()).inDays.clamp(0, 9999),
        );

/// 会话列表用的紧凑相对时间。
String compactAgo(BuildContext context, DateTime utc) {
  final l = L.of(context);
  final diff = DateTime.now().difference(utc.toLocal());
  if (diff.inMinutes < 1) return l.commonJustNow;
  if (diff.inMinutes < 60) return l.commonMinutesAgo(diff.inMinutes);
  if (diff.inHours < 24) return l.commonHoursAgo(diff.inHours);
  if (diff.inDays == 1) return l.commonYesterday;
  if (diff.inDays < 7) return l.commonDaysAgo(diff.inDays);
  return l.commonLastWeek;
}

/// 流水行的 `MM-DD HH:mm`。
String stampShort(DateTime utc) {
  final d = utc.toLocal();
  return '${pad2(d.month)}-${pad2(d.day)} ${pad2(d.hour)}:${pad2(d.minute)}';
}

String _date(DateTime local) =>
    '${local.year}-${pad2(local.month)}-${pad2(local.day)}';

String pad2(int v) => v.toString().padLeft(2, '0');
