import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/sea_scene.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(
  theme: AppTheme.light(),
  home: Scaffold(body: SizedBox(width: 393, height: 770, child: w)),
);

String? _skinAsset(WidgetTester t) {
  for (final i in t.widgetList<Image>(find.byType(Image))) {
    final p = i.image;
    if (p is AssetImage) return p.assetName;
  }
  return null;
}

/// 白天 / 夜晚各一张整图；瓶子落在海平线与沙滩线之间的水带里，不压 dock。
void main() {
  testWidgets('day paints the daytime sea', (t) async {
    await t.pumpWidget(_wrap(const SeaScene(night: false)));
    await t.pump();
    expect(_skinAsset(t), SeaScene.dayAsset);
  });

  testWidgets('night paints the night sea', (t) async {
    await t.pumpWidget(_wrap(const SeaScene(night: true)));
    await t.pump();
    expect(_skinAsset(t), SeaScene.nightAsset);
  });

  test('skin fit keeps the water band between horizon and dock', () {
    final fit = SkinFit.solve(SeaScene.daySkin, 393, 770);
    // 铺满：图至少和画布一样高，顶边不低于 0。
    expect(fit.height, greaterThanOrEqualTo(770));
    expect(fit.top, lessThanOrEqualTo(0));
    // 海平线在画布内、靠上半区附近；水带下沿不压进 dock。
    expect(fit.horizon, inInclusiveRange(300, 520));
    expect(fit.waterBottom, lessThanOrEqualTo(770 - SeaScene.dockReserve));
    expect(fit.bandHeight, greaterThan(60));
    for (final s in bottleSlots) {
      final top = fit.slotTop(s.dy);
      expect(top, greaterThanOrEqualTo(fit.horizon));
      expect(top + bottleSize.height, lessThanOrEqualTo(fit.waterBottom + 1));
    }
  });

  test('short canvas still covers and clamps the band', () {
    final fit = SkinFit.solve(SeaScene.daySkin, 360, 540);
    expect(fit.height, greaterThanOrEqualTo(540));
    expect(fit.bandHeight, greaterThanOrEqualTo(0));
  });
}
