import 'package:bottles/features/auth/auth_controller.dart';
import 'package:bottles/features/auth/credential_flow_page.dart';
import 'package:bottles/features/auth/login_page.dart';
import 'package:bottles/features/auth/oauth_conflict_page.dart';
import 'package:bottles/features/auth/onboarding_page.dart';
import 'package:bottles/features/auth/otp_page.dart';
import 'package:bottles/features/auth/splash_page.dart';
import 'package:bottles/features/location/location_intro_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

/// 登录 / 注册 / 验证码页在六档机型上都不允许 overflow（规范 §8.2）。
/// 这里不比对像素（golden 留给后续），只守一条线：**布局不能爆**。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('login fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const LoginPage()));
    testWidgets(
      'login fits with keyboard: ${cfg.name}',
      (t) => pumpAt(
        t,
        cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'),
        const LoginPage(),
      ),
    );
    testWidgets(
      'register fits: ${cfg.name}',
      (t) => pumpAt(t, cfg, const CredentialFlowPage(flow: AuthFlow.register)),
    );
    testWidgets('otp fits: ${cfg.name}', (t) => pumpAt(t, cfg, const OtpPage()));
    testWidgets('splash fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const SplashPage()));
    testWidgets(
      'oauth conflict fits: ${cfg.name}',
      (t) => pumpAt(t, cfg, const OAuthConflictPage(message: 'x@y.z 已注册')),
    );
    testWidgets('onboarding fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const OnboardingPage()));
    testWidgets('location intro fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const LocationIntroPage()));
  }
}
