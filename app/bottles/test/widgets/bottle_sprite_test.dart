import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/sea_scene.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(
  theme: AppTheme.light(),
  home: Scaffold(body: Center(child: w)),
);

/// 漂流瓶素材统一走 BottleSprite：方框见方、按角度旋转；海面瓶与它共用。
void main() {
  testWidgets('BottleSprite keeps a square box of the requested width', (
    t,
  ) async {
    await t.pumpWidget(_wrap(const BottleSprite(width: 54, tilt: -62)));
    expect(t.getSize(find.byType(BottleSprite)), const Size(54, 54));
    final img = t.widget<Image>(find.byType(Image));
    expect((img.image as AssetImage).assetName, BottleSprite.asset);
  });

  testWidgets('FloatingBottle renders the sprite at its slot tilt', (t) async {
    await t.pumpWidget(_wrap(const FloatingBottle(tilt: 40)));
    final sprite = t.widget<BottleSprite>(find.byType(BottleSprite));
    expect(sprite.tilt, 40);
  });

  testWidgets('sea artwork swaps the sprite for the matching sea image', (
    t,
  ) async {
    await t.pumpWidget(
      _wrap(const FloatingBottle(art: BottleArt.seaRight, night: true)),
    );
    expect(find.byType(BottleSprite), findsNothing);
    // 夜晚用抠掉水面的 -night 变体。
    final img = t.widget<Image>(find.byType(Image));
    expect((img.image as AssetImage).assetName, SeaBottleImage.rightNightAsset);
    // 占位仍是方框，瓶位 / 气泡坐标不受素材切换影响。
    expect(t.getSize(find.byType(SeaBottleImage)), bottleSize);
  });

  testWidgets('scoop and cast keep the transparent sprite by default', (
    t,
  ) async {
    await t.pumpWidget(_wrap(const FloatingBottle(tilt: -75)));
    expect(find.byType(BottleSprite), findsOneWidget);
    expect(find.byType(SeaBottleImage), findsNothing);
  });

  test('every slot carries a distinct tilt', () {
    expect(bottleTilts.length, bottleSlots.length);
    expect(bottleTilts.toSet().length, bottleTilts.length);
  });
}
