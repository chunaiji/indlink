import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/avatar.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:bottles/ui/widgets/coin.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(
  theme: AppTheme.light(),
  home: Scaffold(body: Center(child: w)),
);

void main() {
  _alignmentTests();
  testWidgets('PillChip is 36 tall and at least 65 wide', (t) async {
    await t.pumpWidget(_wrap(const PillChip(label: '全部')));
    final s = t.getSize(find.byType(PillChip));
    expect(s.height, 36);
    expect(s.width, greaterThanOrEqualTo(65));
  });

  testWidgets('SegmentedRow is 44 tall', (t) async {
    await t.pumpWidget(
      _wrap(
        SegmentedRow(labels: const ['a', 'b'], index: 0, onChanged: (_) {}),
      ),
    );
    expect(t.getSize(find.byType(SegmentedRow)).height, 44);
  });

  testWidgets('CoinChip is 30 tall', (t) async {
    await t.pumpWidget(_wrap(const CoinChip(coins: 120)));
    expect(t.getSize(find.byType(CoinChip)).height, 30);
  });

  testWidgets('AvatarRing online dot is 15pt at size 44', (t) async {
    await t.pumpWidget(_wrap(const AvatarRing(online: true)));
    expect(
      t.getSize(find.byKey(const ValueKey('avatar-online-dot'))).width,
      15,
    );
  });
}

// ---- 2026-10-04 真机对齐（小米 9 第二批截图）----

Widget _page(Widget body) => MaterialApp(
  theme: AppTheme.light(),
  home: Scaffold(body: body),
);

void _alignmentTests() {
  testWidgets(
    'PillChip shrink-wraps inside a Wrap instead of filling the row',
    (t) async {
      await t.pumpWidget(_page(const Wrap(children: [PillChip(label: '全部')])));
      expect(t.getSize(find.byType(PillChip)).width, lessThan(150));
    },
  );

  testWidgets('ChipBar starts at the gutter even inside a centering Column', (
    t,
  ) async {
    await t.pumpWidget(
      _page(
        const Column(
          children: [
            ChipBar(
              children: [
                PillChip(label: 'a'),
                PillChip(label: 'b'),
              ],
            ),
          ],
        ),
      ),
    );
    expect(t.getRect(find.byType(PillChip).first).left, 16);
  });

  testWidgets('ValueField is 50 tall and shows its value', (t) async {
    await t.pumpWidget(
      _page(
        Center(
          child: ValueField(value: '18 – 30', onTap: () {}),
        ),
      ),
    );
    expect(t.getSize(find.byType(ValueField)).height, 50);
    expect(find.text('18 – 30'), findsOneWidget);
  });
}
