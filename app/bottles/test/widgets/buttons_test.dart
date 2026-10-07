import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/brand_marks.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(
  theme: AppTheme.light(),
  home: Scaffold(body: Center(child: w)),
);

void main() {
  testWidgets('MiniButton is 44 tall and uses the 17pt label', (t) async {
    await t.pumpWidget(_wrap(const MiniButton(label: '充值')));
    expect(t.getSize(find.byType(MiniButton)).height, 44);
    final text = t.widget<Text>(find.text('充值'));
    expect(text.style!.fontSize, 17);
  });

  testWidgets('ActionCircleButton is a 50pt circle', (t) async {
    await t.pumpWidget(_wrap(const ActionCircleButton(icon: Icons.favorite)));
    expect(t.getSize(find.byType(ActionCircleButton)), const Size(50, 50));
  });

  testWidgets('RoundIconButton keeps the 44pt tap box used by input bars', (
    t,
  ) async {
    await t.pumpWidget(_wrap(const RoundIconButton(icon: Icons.image)));
    expect(t.getSize(find.byType(RoundIconButton)), const Size(44, 44));
  });

  testWidgets('AppButton keeps 50pt height at text scale 1.2', (t) async {
    t.platformDispatcher.textScaleFactorTestValue = 1.2;
    addTearDown(t.platformDispatcher.clearTextScaleFactorTestValue);
    await t.pumpWidget(_wrap(const AppButton(label: '登录')));
    expect(t.getSize(find.byType(AppButton)).height, 50);
  });

  testWidgets('HiButton renders the pay tone with the price inline', (t) async {
    await t.pumpWidget(_wrap(const HiButton(label: '打招呼', coins: 2)));
    expect(t.getSize(find.byType(HiButton)).height, 50);
    expect(find.text('2'), findsOneWidget);
  });

  testWidgets('IconCircleButton is a 36pt circle', (t) async {
    await t.pumpWidget(_wrap(const IconCircleButton(icon: Icons.my_location)));
    expect(t.getSize(find.byType(IconCircleButton)), const Size(36, 36));
  });

  testWidgets('MiniButton is a pill, not a circle, with a two-glyph label', (
    t,
  ) async {
    await t.pumpWidget(_wrap(const MiniButton(label: '充值', height: 48)));
    expect(t.getSize(find.byType(MiniButton)).width, greaterThanOrEqualTo(72));
  });

  testWidgets('GoogleMark paints at the requested size', (t) async {
    await t.pumpWidget(_wrap(const GoogleMark(size: 20)));
    expect(t.getSize(find.byType(GoogleMark)), const Size(20, 20));
  });

  testWidgets('AppButton renders a custom leading mark instead of an icon', (
    t,
  ) async {
    await t.pumpWidget(
      _wrap(
        const AppButton(
          label: 'Google',
          kind: BtnKind.oauth,
          leading: GoogleMark(size: 20),
        ),
      ),
    );
    expect(find.byType(GoogleMark), findsOneWidget);
  });
}
