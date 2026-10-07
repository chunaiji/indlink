import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/states.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('EmptyState icon is 57 on a regular screen and 40 on a short one',
      (t) async {
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    for (final (h, expected) in [(851.0, 57.0), (667.0, 40.0)]) {
      t.view.physicalSize = Size(393, h);
      await t.pumpWidget(MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(
          body: EmptyState(icon: Icons.inbox, title: 't', body: 'b'),
        ),
      ));
      expect(t.widget<Icon>(find.byType(Icon)).size, expected,
          reason: 'height $h');
    }
  });

  testWidgets('EmptyState title is 20pt', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const Scaffold(
        body: EmptyState(icon: Icons.inbox, title: 'title', body: 'b'),
      ),
    ));
    expect(t.widget<Text>(find.text('title')).style!.fontSize, 20);
  });

  testWidgets('FailureState offline uses the wifi-off icon', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const Scaffold(
        body: FailureState(
          title: 't',
          body: 'b',
          ctaLabel: 'retry',
          offline: true,
        ),
      ),
    ));
    expect(find.byIcon(Icons.wifi_off_rounded), findsOneWidget);
  });
}
