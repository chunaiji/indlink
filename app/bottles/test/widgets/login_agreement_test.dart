import 'package:bottles/features/auth/login_page.dart';
import 'package:bottles/ui/widgets/overlays.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// A2：没勾「继续即表示同意…」就点登录，先弹协议确认；取消则什么都不发生。
void main() {
  testWidgets('signing in without the agreement opens the consent modal', (
    t,
  ) async {
    await pumpAt(t, layoutMatrix[2], const LoginPage());
    expect(find.byKey(const ValueKey('auth-agree-check')), findsOneWidget);

    await t.enterText(find.byType(TextField).at(0), '9999999999');
    await t.enterText(find.byType(TextField).at(1), 'secret123');
    await t.pump();
    await t.tap(find.text('Sign in'));
    await t.pumpAndSettle();

    expect(find.byType(ModalCard), findsOneWidget);
    expect(find.text('Agree and continue'), findsOneWidget);

    await t.tap(find.text('Cancel'));
    await t.pumpAndSettle();
    expect(find.byType(ModalCard), findsNothing);
  });

  testWidgets('tapping the dot toggles the agreement', (t) async {
    await pumpAt(t, layoutMatrix[2], const LoginPage());
    final dot = find.byKey(const ValueKey('auth-agree-check'));
    await t.tap(dot);
    await t.pump();
    expect(
      find.descendant(of: dot, matching: find.byIcon(Icons.check_rounded)),
      findsOneWidget,
    );
  });
}
