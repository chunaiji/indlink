import 'package:bottles/features/moment/moment_detail_page.dart';
import 'package:bottles/features/moment/moment_feed_page.dart';
import 'package:bottles/features/moment/post_moment_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

/// 动态系页面在六档机型上都不允许 overflow（规范 §8.2）。
void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('moment feed fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const MomentFeedPage()));
    testWidgets('post moment fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const PostMomentPage()));
    testWidgets(
      'post moment fits with keyboard: ${cfg.name}',
      (t) => pumpAt(
        t,
        cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'),
        const PostMomentPage(),
      ),
    );
    testWidgets('moment detail fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const MomentDetailPage(momentId: 'm1')));
  }
}
