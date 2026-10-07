import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:bottles/ui/widgets/headers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w, {EdgeInsets padding = EdgeInsets.zero}) => MaterialApp(
  theme: AppTheme.light(),
  home: MediaQuery(
    data: MediaQueryData(viewPadding: padding, padding: padding),
    child: Scaffold(body: Column(children: [const Spacer(), w])),
  ),
);

void main() {
  testWidgets('NavBar is 44 tall', (t) async {
    await t.pumpWidget(
      MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(appBar: NavBar(title: 'x')),
      ),
    );
    expect(t.getSize(find.byType(NavBar)).height, 44);
  });

  testWidgets('BottomActionBar keeps 16 below the button on gesture nav', (
    t,
  ) async {
    await t.pumpWidget(
      _wrap(const BottomActionBar(child: AppButton(label: 'go'))),
    );
    final bar = t.getRect(find.byType(BottomActionBar));
    final btn = t.getRect(find.byType(AppButton));
    expect(bar.bottom - btn.bottom, 16);
  });

  testWidgets('BottomActionBar keeps 16 above a 48pt three-button nav', (
    t,
  ) async {
    await t.pumpWidget(
      _wrap(
        const BottomActionBar(child: AppButton(label: 'go')),
        padding: const EdgeInsets.only(bottom: 48),
      ),
    );
    final bar = t.getRect(find.byType(BottomActionBar));
    final btn = t.getRect(find.byType(AppButton));
    expect(bar.bottom - btn.bottom, 64);
  });

  testWidgets('NavBar actions sit flush against the right edge', (t) async {
    await t.pumpWidget(
      MaterialApp(
        theme: AppTheme.light(),
        home: Scaffold(
          appBar: NavBar(
            title: 'x',
            actions: [TextButton(onPressed: () {}, child: const Text('go'))],
          ),
        ),
      ),
    );
    final width = t.getSize(find.byType(NavBar)).width;
    // 导航条右 padding s2 + TextButton 自身的水平内边距，文字离右沿不超过 32。
    expect(width - t.getRect(find.text('go')).right, lessThan(32));
  });

  testWidgets('NavBar subtitle sits flush against the right edge', (t) async {
    await t.pumpWidget(
      MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(
          appBar: NavBar(title: 'x', subtitle: 'sub'),
        ),
      ),
    );
    final width = t.getSize(find.byType(NavBar)).width;
    expect(width - t.getRect(find.text('sub')).right, 16);
  });

  testWidgets('BottomActionBar stacks children with an 8pt gap', (t) async {
    await t.pumpWidget(
      _wrap(
        const BottomActionBar(
          children: [
            AppButton(label: 'a'),
            AppButton(label: 'b', kind: BtnKind.ghost),
          ],
        ),
      ),
    );
    final a = t.getRect(find.byType(AppButton).at(0));
    final b = t.getRect(find.byType(AppButton).at(1));
    expect(b.top - a.bottom, 8);
  });

  testWidgets('HeaderStats shows three columns', (t) async {
    await t.pumpWidget(
      MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(
          body: HeaderStats(items: [('12', '关注'), ('34', '粉丝'), ('56', '魅力')]),
        ),
      ),
    );
    expect(find.text('12'), findsOneWidget);
    expect(find.text('魅力'), findsOneWidget);
  });

  testWidgets('slim GradientHeader is 54 tall without a status bar', (t) async {
    await t.pumpWidget(
      MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(
          body: Column(
            children: [
              GradientHeader(slim: true, topRow: SizedBox(height: 30)),
            ],
          ),
        ),
      ),
    );
    expect(t.getSize(find.byType(GradientHeader)).height, 54);
  });

  testWidgets('flat BottomActionBar draws no top border', (t) async {
    await t.pumpWidget(
      _wrap(const BottomActionBar(flat: true, child: AppButton(label: 'go'))),
    );
    final box = t.widget<Container>(
      find
          .descendant(
            of: find.byType(BottomActionBar),
            matching: find.byType(Container),
          )
          .first,
    );
    expect((box.decoration as BoxDecoration?)?.border, isNull);
  });
}
