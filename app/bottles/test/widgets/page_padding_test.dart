import 'package:bottles/features/me/notifications_page.dart';
import 'package:bottles/features/me/settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// 没有底部按钮的二级页，列表最后一项和手势条之间要留 gutter + 安全区，
/// 否则最后一行贴着屏底（真机：写瓶 / 充值 / 筛选 / 设置都反馈过）。
void main() {
  final cfg = layoutMatrix[2].withInsets(viewPadding: threeButtonNav);

  testWidgets('settings list pads the bottom inset', (t) async {
    await pumpAt(t, cfg, const SettingsPage());
    final list = t.widget<ListView>(find.byType(ListView));
    expect(list.padding!.resolve(TextDirection.ltr).bottom, 16 + 48);
  });

  testWidgets('notifications list pads the bottom inset', (t) async {
    await pumpAt(t, cfg, const NotificationsPage());
    final list = t.widget<ListView>(find.byType(ListView).first);
    expect(list.padding!.resolve(TextDirection.ltr).bottom, 16 + 48);
  });
}
