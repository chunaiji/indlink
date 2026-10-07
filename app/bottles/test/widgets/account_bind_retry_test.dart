import 'package:bottles/core/network/api_exception.dart';
import 'package:bottles/core/platform/oauth.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/features/me/account_security_page.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// A6b：绑定撞上「已被占用」，用户选「换一个 Google 账号」后必须真的再绑一次。
class _FlakyAuth extends MockAuthRepository {
  _FlakyAuth(super.db);

  int bindCalls = 0;

  @override
  Future<void> bindOAuth(String provider, String idToken) async {
    bindCalls++;
    if (bindCalls == 1) {
      throw ApiException(ErrCode.oauthAlreadyBound, 'x@y.z 已绑在另一个账号');
    }
  }
}

class _FakeOAuth implements OAuthClient {
  @override
  Future<String> googleIdToken() async => 'token';

  @override
  Future<String> appleIdToken() async => 'token';
}

void main() {
  testWidgets('choosing another account after "taken" binds again', (t) async {
    late _FlakyAuth auth;
    await pumpAt(
      t,
      layoutMatrix[2],
      const AccountSecurityPage(),
      skipDefaults: {authRepoProvider},
      overrides: [
        authRepoProvider.overrideWith((ref) {
          auth = _FlakyAuth(ref.watch(mockBackendProvider));
          return auth;
        }),
        oauthClientProvider.overrideWith((ref) => _FakeOAuth()),
      ],
    );

    await t.tap(find.text('Link').first);
    await t.pumpAndSettle();
    expect(auth.bindCalls, 1);
    expect(find.text('Use another Google account'), findsOneWidget);

    await t.tap(find.text('Use another Google account'));
    await t.pumpAndSettle();
    expect(auth.bindCalls, 2, reason: 'retry must not be swallowed by the busy guard');
  });
}
