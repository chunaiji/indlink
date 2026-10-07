import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/overlays.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('SheetShell caps its height at 85% of the screen and scrolls',
      (t) async {
    t.view.physicalSize = const Size(393, 851);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: Builder(builder: (ctx) {
        return TextButton(
          onPressed: () => showAppSheet<void>(
            ctx,
            builder: (_) => const SizedBox(height: 2000),
          ),
          child: const Text('open'),
        );
      })),
    ));
    await t.tap(find.text('open'));
    await t.pumpAndSettle();
    expect(t.getSize(find.byType(SheetShell)).height,
        lessThanOrEqualTo(851 * .85 + 1));
    expect(t.takeException(), isNull);
  });

  testWidgets('ModalCard close button is a 30pt circle', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(
        body: ModalCard(
          title: 't',
          body: const Text('b'),
          actions: const [],
          onClose: () {},
        ),
      ),
    ));
    expect(t.getSize(find.byKey(const ValueKey('modal-close'))),
        const Size(30, 30));
  });

  testWidgets('showAppModal scales in and centers the card', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: Builder(builder: (ctx) {
        return TextButton(
          onPressed: () => showAppModal<void>(
            ctx,
            builder: (_) =>
                const ModalCard(title: 'hello', body: Text('b'), actions: []),
          ),
          child: const Text('open'),
        );
      })),
    ));
    await t.tap(find.text('open'));
    await t.pump();
    expect(find.byType(ScaleTransition), findsWidgets);
    await t.pumpAndSettle();
    expect(find.text('hello'), findsOneWidget);
  });
}
