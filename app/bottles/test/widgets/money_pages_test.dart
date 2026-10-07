import 'package:bottles/features/me/recharge_page.dart';
import 'package:bottles/features/me/wallet_page.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// H2 余额卡通栏；H3 档位卡铺满两列格宽（真机上两者都缩成了中间一小块）。
void main() {
  testWidgets('wallet balance card spans the content width', (t) async {
    await pumpAt(t, layoutMatrix[2], const WalletPage());
    final card = t.getSize(find.byKey(const ValueKey('wallet-balance-card')));
    expect(card.width, 393 - 32);
  });

  testWidgets('package tiles fill their grid cells', (t) async {
    await pumpAt(t, layoutMatrix[2], const RechargePage());
    final tile = t.getSize(find.byType(PackageTile).first);
    expect(tile.width, closeTo((393 - 32 - 8) / 2, 1));
  });
}
