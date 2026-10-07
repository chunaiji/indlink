import 'package:bottles/core/design/tokens.dart';
import 'package:bottles/data/mock/mock_backend.dart';
import 'package:bottles/features/discover/discover_page.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:bottles/ui/widgets/headers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../layout/matrix.dart';

/// C1：花金币的是「把划走的人找回来」（撤回），不是跳过——价格角标要挂在撤回键上。
/// 胶囊行在头图下方留 s2，不再上提压住头图。
void main() {
  testWidgets('rewind carries the coin badge and chips clear the header', (
    t,
  ) async {
    await pumpAt(t, layoutMatrix[2], const DiscoverPage());

    // 牌堆预渲染三张卡，每张都有自己的动作行；看最上面那张。
    final rewind = find.byKey(const ValueKey('rewind-action')).first;
    expect(rewind, findsOneWidget);
    expect(
      find.descendant(
        of: rewind,
        matching: find.text('${MockBackend.priceRewind}'),
      ),
      findsOneWidget,
    );

    final header = t.getRect(find.byType(GradientHeader));
    final chip = t.getRect(find.byType(PillChip).first);
    expect(chip.top - header.bottom, greaterThanOrEqualTo(Dim.s2));
  });
}
