import 'package:bottles/features/discover/discover_page.dart';
import 'package:bottles/features/discover/filter_page.dart';
import 'package:bottles/features/discover/swipe_deck.dart';
import 'package:bottles/features/discover/user_profile_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

/// 发现系页面在六档机型上都不允许 overflow（规范 §8.2）。
/// 宽屏（折叠展开）下滑卡要留在限宽 480 内、照片区不低于 3:4（Review Focus 2）。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('filter fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const FilterPage()));
    testWidgets('user profile fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const UserProfilePage(userId: 'u1')));
    testWidgets('discover fits: ${cfg.name}', (t) async {
      await pumpAt(t, cfg, const DiscoverPage());
      if (cfg.size.width >= 600) {
        final deck = find.byType(SwipeDeck);
        if (deck.evaluate().isNotEmpty) {
          final s = t.getSize(deck);
          expect(s.width, lessThanOrEqualTo(480), reason: 'deck width');
          expect(s.height / s.width, greaterThanOrEqualTo(4 / 3),
              reason: 'deck aspect');
        }
      }
    });
  }
}
