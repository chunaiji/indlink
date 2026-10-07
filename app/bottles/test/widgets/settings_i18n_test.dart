import 'package:bottles/features/me/settings_page.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// 设置页在英文下不能漏中文（真机：账号与安全那一行是写死的中文）。
void main() {
  testWidgets('settings rows are localised', (t) async {
    await pumpAt(t, layoutMatrix[2], const SettingsPage());
    expect(find.text('Account & security'), findsOneWidget);
    expect(find.text('账号与安全'), findsNothing);
  });
}
