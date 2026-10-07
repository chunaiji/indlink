import 'package:bottles/app/router.dart';
import 'package:bottles/app/routes.dart';
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/core/storage/prefs.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/auth/credential_flow_page.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../layout/matrix.dart';

/// 账号与安全 → 修改密码：登录态下 push /login/forgot 必须真的打开那一页。
/// 以前它在「已登录就踢回海面」的名单里，push 被改写成 /ocean，整个 Tab 壳被当成一页
/// 压在根导航器上——真机上是一块黑屏。
void main() {
  testWidgets('change password opens while signed in', (t) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await Prefs.load();
    final container = ProviderContainer(overrides: defaultOverrides(prefs));
    addTearDown(container.dispose);
    container
        .read(authProvider.notifier)
        .finishOnboarding(
          const UserProfile(
            brief: UserBrief(id: 'me', nickname: 'Me'),
          ),
        );
    final router = container.read(routerProvider);

    await t.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.light(),
          localizationsDelegates: L.localizationsDelegates,
          supportedLocales: L.supportedLocales,
        ),
      ),
    );
    for (var i = 0; i < 3; i++) {
      await t.pump(const Duration(milliseconds: 400));
    }

    router.push(Routes.forgotPassword);
    for (var i = 0; i < 3; i++) {
      await t.pump(const Duration(milliseconds: 400));
    }
    expect(find.byType(CredentialFlowPage), findsOneWidget);
    expect(t.takeException(), isNull);
  });
}
