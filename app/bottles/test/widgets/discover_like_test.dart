import 'package:bottles/features/discover/discover_controller.dart';
import 'package:bottles/features/discover/discover_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// C1：点爱心只是「喜欢」，不切人；只有左右滑才换下一张。
void main() {
  testWidgets('tapping the heart likes without advancing the deck', (t) async {
    await pumpAt(t, layoutMatrix[2], const DiscoverPage());
    final container = ProviderScope.containerOf(
      t.element(find.byType(DiscoverPage)),
    );
    final top = container.read(deckProvider).value!.first.id;

    // 牌堆叠着三张卡，命中测试的那一套预检会被后面的卡搅糊涂；事件本身落在顶卡的按钮上
    // （下面断言 likedIds 已包含顶卡就是证据），所以关掉这条预警。
    await t.tap(
      find.byIcon(Icons.favorite_border_rounded).first,
      warnIfMissed: false,
    );
    await t.pump(const Duration(milliseconds: 500));

    expect(container.read(deckProvider).value!.first.id, top);
    expect(container.read(likedIdsProvider), contains(top));
    // 撤回旁边不再有 ✕：跳过只靠左滑。
    expect(find.byIcon(Icons.close_rounded), findsNothing);
    expect(find.byIcon(Icons.favorite_rounded), findsWidgets);
    await t.pump(const Duration(seconds: 1));
  });
}
