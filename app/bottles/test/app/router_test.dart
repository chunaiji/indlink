import 'package:bottles/app/router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// 原型里只有 Tab 根页带底部 Tab 栏（B1/C1/D1/F1/H1）；写瓶、详情、筛选、
/// 资料、发动态、选地点……全部是盖住 Tab 栏的全屏页。
void main() {
  test('every page under a tab root is pushed on the root navigator', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final router = container.read(routerProvider);
    final shell = router.configuration.routes
        .whereType<StatefulShellRoute>()
        .single;

    final offenders = <String>[];
    void walk(List<RouteBase> routes) {
      for (final r in routes) {
        if (r is! GoRoute) continue;
        if (r.parentNavigatorKey == null) offenders.add(r.path);
        walk(r.routes);
      }
    }

    for (final branch in shell.branches) {
      for (final root in branch.routes) {
        if (root is GoRoute) walk(root.routes);
      }
    }
    expect(offenders, isEmpty, reason: 'still rendered inside the tab shell');
  });
}
