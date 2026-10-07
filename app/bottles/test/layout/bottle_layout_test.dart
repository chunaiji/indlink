import 'package:bottles/core/design/theme.dart';
import 'package:bottles/features/bottle/bottle_detail_page.dart';
import 'package:bottles/features/bottle/drift_map_page.dart';
import 'package:bottles/features/bottle/my_bottles_page.dart';
import 'package:bottles/features/bottle/ocean_page.dart';
import 'package:bottles/features/bottle/write_bottle_page.dart';
import 'package:bottles/ui/widgets/sea_scene.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('ocean fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const OceanPage()));
    testWidgets(
      'ocean fits with 3-button nav: ${cfg.name}',
      (t) => pumpAt(
        t,
        cfg.withInsets(viewPadding: threeButtonNav, suffix: ' +nav'),
        const OceanPage(),
      ),
    );
    testWidgets('write bottle fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const WriteBottlePage()));
    testWidgets(
      'write bottle fits with keyboard: ${cfg.name}',
      (t) => pumpAt(
        t,
        cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'),
        const WriteBottlePage(),
      ),
    );
    testWidgets('bottle detail fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const BottleDetailPage(bottleId: 'b1')));
    testWidgets('my bottles fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const MyBottlesPage()));
    testWidgets('drift map fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const DriftMapPage(bottleId: 'b1')));
  }

  // 夜场 × 系统暗色是两根独立的轴，四种组合都要成立（Review Focus 3）。
  for (final night in [false, true]) {
    for (final dark in [false, true]) {
      testWidgets('sea scene night=$night dark=$dark', (t) async {
        t.view.devicePixelRatio = 1;
        t.view.physicalSize = const Size(393, 600);
        addTearDown(t.view.reset);
        await t.pumpWidget(MaterialApp(
          theme: dark ? AppTheme.dark() : AppTheme.light(),
          home: Scaffold(body: SeaScene(night: night)),
        ));
        // 海面是整图素材，解码是异步的：先把图预载进缓存，金图才不会在
        // 「图还没到」和「图到了」之间随机抖。
        await t.runAsync(() async {
          final ctx = t.element(find.byType(SeaScene));
          for (final a in [SeaScene.dayAsset, SeaScene.nightAsset]) {
            await precacheImage(AssetImage(a), ctx);
          }
        });
        await t.pump(const Duration(milliseconds: 100));
        await expectLater(
          find.byType(SeaScene),
          matchesGoldenFile(
            'goldens/sea_${night ? 'night' : 'day'}_${dark ? 'dark' : 'light'}.png',
          ),
        );
      });
    }
  }
}
