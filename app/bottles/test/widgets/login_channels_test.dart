import 'package:bottles/core/config/remote_config.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/features/auth/login_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

class _EmailOff extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({
    'auth': {'phone': true, 'email': false},
  });
}

class _PhoneOff extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({
    'auth': {'phone': false, 'email': true},
  });
}


/// pumpLoginWith 用一份 auth 段起登录页。
class _AuthOnly extends AppConfigNotifier {
  _AuthOnly(this.auth);
  final Map<String, Object?> auth;
  @override
  AppRemoteConfig build() => AppRemoteConfig.fromJson({'auth': auth});
}

Future<void> pumpLoginWith(WidgetTester t, Map<String, Object?> auth) {
  return pumpAt(
    t,
    layoutMatrix[2],
    const LoginPage(),
    skipDefaults: {appConfigProvider},
    overrides: [appConfigProvider.overrideWith(() => _AuthOnly(auth))],
  );
}

/// A2：后台关掉一种登录标识后，切换条消失，表单直接是开着的那一种；
/// 默认（都开）保持原样。
void main() {
  testWidgets('both on: the phone/email switch is shown', (t) async {
    await pumpAt(t, layoutMatrix[2], const LoginPage());
    expect(find.byType(SegmentedRow), findsOneWidget);
    expect(find.byType(PhoneField), findsOneWidget);
  });

  testWidgets('email off: no switch, phone form only', (t) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const LoginPage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_EmailOff.new)],
    );
    expect(find.byType(SegmentedRow), findsNothing);
    expect(find.byType(PhoneField), findsOneWidget);
    expect(find.byType(EmailField), findsNothing);
  });

  testWidgets('phone off: email form even though state defaults to phone', (
    t,
  ) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const LoginPage(),
      skipDefaults: {appConfigProvider},
      overrides: [appConfigProvider.overrideWith(_PhoneOff.new)],
    );
    expect(find.byType(SegmentedRow), findsNothing);
    expect(find.byType(EmailField), findsOneWidget);
    expect(find.byType(PhoneField), findsNothing);
  });

  // 这条今天完全没有守护:国际版的 Google 按钮一直是无条件渲染的。
  testWidgets('google button disappears when the server turns it off', (
    t,
  ) async {
    await pumpLoginWith(t, const {'google': false, 'apple': false});
    expect(find.text('Continue with Google'), findsNothing);
  });

  testWidgets('google button shows when enabled', (t) async {
    await pumpLoginWith(t, const {'google': true, 'apple': false});
    expect(find.text('Continue with Google'), findsOneWidget);
  });
}
