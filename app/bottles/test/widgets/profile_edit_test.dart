import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/me/profile_edit_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

class _SpyAuth extends MockAuthRepository {
  _SpyAuth(super.db);

  String? nickname;
  String? birthday;
  bool sentGender = false;

  @override
  Future<UserProfile> updateProfile({
    String? nickname,
    String? avatar,
    Gender? gender,
    int? age,
    String? birthday,
    String? bio,
    List<String>? languages,
    List<String>? interests,
    double? lat,
    double? lng,
  }) {
    this.nickname = nickname;
    this.birthday = birthday;
    sentGender = gender != null;
    return super.updateProfile(
      nickname: nickname,
      avatar: avatar,
      birthday: birthday,
    );
  }
}

/// 编辑资料：昵称 + 出生日期能改并提交；性别不在这一页，也不会被顺手发出去。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets(
      'profile edit fits: ${cfg.name}',
      (t) => pumpAt(t, cfg, const ProfileEditPage()),
    );
  }

  testWidgets('saves nickname and the picked birthday', (t) async {
    late _SpyAuth auth;
    await pumpAt(
      t,
      layoutMatrix[2],
      const ProfileEditPage(),
      skipDefaults: {authRepoProvider},
      overrides: [
        authRepoProvider.overrideWith((ref) {
          auth = _SpyAuth(ref.watch(mockBackendProvider));
          return auth;
        }),
      ],
    );

    await t.enterText(find.byKey(const ValueKey('profile-nickname')), 'Nova');
    await t.tap(find.byType(ValueField));
    await t.pumpAndSettle();
    expect(find.byType(DatePickerDialog), findsOneWidget);
    await t.tap(find.text('OK'));
    await t.pumpAndSettle();
    expect(find.text('2000-01-01'), findsOneWidget);

    await t.tap(find.text('Save'));
    await t.pump(const Duration(milliseconds: 600));
    expect(auth.nickname, 'Nova');
    expect(auth.birthday, '2000-01-01');
    expect(auth.sentGender, isFalse);
  });
}
