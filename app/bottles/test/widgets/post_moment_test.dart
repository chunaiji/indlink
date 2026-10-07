import 'package:bottles/core/network/api_exception.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/domain/models/moment.dart';
import 'package:bottles/domain/models/place.dart';
import 'package:bottles/features/moment/post_moment_page.dart';
import 'package:bottles/ui/widgets/headers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// 记下发布参数后故意失败：成功会 context.pop()，而测试里没有 GoRouter。
class _SpyMoments extends MockMomentRepository {
  _SpyMoments(super.db);

  String? visible;

  @override
  Future<Moment> post({
    required String text,
    List<String> images = const [],
    Place? place,
    String visible = 'public',
  }) async {
    this.visible = visible;
    throw ApiException(ErrCode.serverError, 'nope');
  }
}

/// F2：「发布」在导航栏右侧（原型 .nv .sp2），没有底部按钮；
/// 「谁可以看」选了「仅自己」要真的随请求下去。
void main() {
  testWidgets('post lives in the nav bar and sends the chosen visibility', (
    t,
  ) async {
    late _SpyMoments repo;
    await pumpAt(
      t,
      layoutMatrix[2],
      const PostMomentPage(),
      skipDefaults: {momentRepoProvider},
      overrides: [
        momentRepoProvider.overrideWith((ref) {
          repo = _SpyMoments(ref.watch(mockBackendProvider));
          return repo;
        }),
      ],
    );

    expect(find.byType(BottomActionBar), findsNothing);
    expect(
      find.descendant(of: find.byType(NavBar), matching: find.text('Post')),
      findsOneWidget,
    );

    await t.enterText(find.byType(TextField), 'hello');
    await t.tap(find.text('Only me'));
    await t.pump();
    await t.tap(find.text('Post'));
    await t.pump();
    expect(repo.visible, 'self');
  });
}
