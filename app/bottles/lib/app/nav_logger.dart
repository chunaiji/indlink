import 'package:flutter/widgets.dart';

import '../core/logging/log_record.dart';
import '../core/logging/logger.dart';

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
