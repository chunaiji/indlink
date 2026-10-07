import 'package:bottles/core/providers.dart';
import 'package:bottles/features/bottle/bottle_detail_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// B3：回信按条解锁。每条锁着的回信挂一枚解锁小标，点一条付一条的钱；
/// 整瓶「解锁 N 条回信」的大按钮已去掉。
void main() {
  testWidgets('tapping one locked reply unlocks only that reply', (t) async {
    await pumpAt(t, layoutMatrix[2], const BottleDetailPage(bottleId: 'mb1'));
    final pill = find.byKey(const ValueKey('reply-unlock-pill'));
    expect(pill, findsNWidgets(3));
    expect(find.textContaining('Unlock 3'), findsNothing);

    final container = ProviderScope.containerOf(
      t.element(find.byType(BottleDetailPage)),
    );
    final before = container.read(mockBackendProvider).wallet.coins;

    await t.tap(pill.first);
    await t.pumpAndSettle();
    expect(find.text('Unlock for 2 coins'), findsOneWidget);
    await t.tap(find.text('Unlock for 2 coins'));
    await t.pumpAndSettle();

    expect(find.byKey(const ValueKey('reply-unlock-pill')), findsNWidgets(2));
    expect(container.read(mockBackendProvider).wallet.coins, before - 2);
  });
}
