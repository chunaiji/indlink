import 'package:bottles/features/auth/onboarding_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// A4：第一步点「确认」先弹性别锁定提示，再想想就留在当前步。
void main() {
  testWidgets('confirming step one warns that gender is final', (t) async {
    await pumpAt(t, layoutMatrix[2], const OnboardingPage());
    await t.enterText(find.byType(TextField).first, 'Nova');
    // 年龄不再手填：点出生日期 → 选择器默认停在「最小年龄」那天，直接 OK。
    await t.tap(find.byType(ValueField));
    await t.pumpAndSettle();
    await t.tap(find.text('OK'));
    await t.pumpAndSettle();

    await t.tap(find.text('Confirm'));
    await t.pumpAndSettle();
    expect(find.text("Gender can't be changed later"), findsOneWidget);

    await t.tap(find.text('Let me think'));
    await t.pumpAndSettle();
    expect(find.text("Gender can't be changed later"), findsNothing);
    // 还在第一步：昵称框还在。
    expect(find.text('Nova'), findsOneWidget);
  });
}
