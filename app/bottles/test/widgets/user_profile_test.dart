import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/discover/user_profile_page.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

class _BareProfile extends MockDiscoverRepository {
  _BareProfile(super.db);

  @override
  Future<UserProfile> card(String userId) async => const UserProfile(
    brief: UserBrief(id: 'x', nickname: 'Nobody', age: 20),
    bio: '',
  );
}

/// C3：资料为空时名字和统计之间不能留一大块空白（真机截到过 60pt 的空档：
/// 空串 bio 占了一行）；底部左键是「送礼」。
void main() {
  testWidgets('empty profile keeps name and stats close, footer says gift', (
    t,
  ) async {
    await pumpAt(
      t,
      layoutMatrix[2],
      const UserProfilePage(userId: 'x'),
      skipDefaults: {discoverRepoProvider},
      overrides: [
        discoverRepoProvider.overrideWith(
          (ref) => _BareProfile(ref.watch(mockBackendProvider)),
        ),
      ],
    );
    final name = t.getRect(find.text('Nobody, 20'));
    final stats = t.getRect(find.byType(StatsRow));
    expect(stats.top - name.bottom, lessThan(24));
    expect(find.text('Gift'), findsOneWidget);
    expect(find.text('Charm'), findsNothing);

    // 还没和 TA 聊过：送礼不能假装成功，要提示先打招呼。
    await t.tap(find.text('Gift'));
    await t.pump(const Duration(milliseconds: 600));
    expect(find.text('Say hi first — gifts are sent in chat'), findsOneWidget);
  });
}
