import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
      'three package tiles fit 360 wide at text scale 1.2 without overflow',
      (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(360, 780);
    t.platformDispatcher.textScaleFactorTestValue = 1.2;
    addTearDown(() {
      t.view.reset();
      t.platformDispatcher.clearTextScaleFactorTestValue();
    });
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(
        body: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              for (final p in [
                (60, '¥6', null, null),
                (600, '¥59.9', '¥79', '最划算'),
                (3000, '¥299', null, '送 300'),
              ]) ...[
                Expanded(
                  child: PackageTile(
                    coins: p.$1,
                    price: p.$2,
                    original: p.$3,
                    badge: p.$4,
                    selected: p.$1 == 600,
                    onTap: () {},
                  ),
                ),
                const SizedBox(width: 8),
              ],
            ],
          ),
        ),
      ),
    ));
    await t.pump();
    expect(t.takeException(), isNull);
    expect(find.text('¥59.9'), findsOneWidget);
  });
}
