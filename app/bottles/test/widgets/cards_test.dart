import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(theme: AppTheme.light(), home: Scaffold(body: w));

void main() {
  testWidgets('ListRowItem is at least 44 tall, 52 when roomy', (t) async {
    await t.pumpWidget(_wrap(const ListGroup(children: [
      ListRowItem(title: 'a'),
      ListRowItem(title: 'b', roomy: true),
    ])));
    final rows = find.byType(ListRowItem);
    expect(t.getSize(rows.at(0)).height, greaterThanOrEqualTo(44));
    expect(t.getSize(rows.at(1)).height, greaterThanOrEqualTo(52));
  });

  testWidgets('QuadGrid lays 4 tiles of 48pt, 62 when roomy', (t) async {
    await t.pumpWidget(_wrap(QuadGrid(items: [
      for (var i = 0; i < 4; i++)
        QuadItem(icon: Icons.star, label: '$i', onTap: () {}),
    ])));
    expect(find.byType(QuadTile), findsNWidgets(4));
    expect(t.getSize(find.byKey(const ValueKey('quad-icon-0'))).height, 48);

    await t.pumpWidget(_wrap(QuadGrid(roomy: true, items: [
      for (var i = 0; i < 4; i++)
        QuadItem(icon: Icons.star, label: '$i', onTap: () {}),
    ])));
    expect(t.getSize(find.byKey(const ValueKey('quad-icon-0'))).height, 62);
  });

  testWidgets('NoticeBanner uses a vector icon, never emoji', (t) async {
    await t.pumpWidget(_wrap(const NoticeBanner(text: 'x')));
    expect(find.byType(Icon), findsOneWidget);
    expect(find.text('⚠️'), findsNothing);
  });

  testWidgets('TimelineList rows are at least 39 tall', (t) async {
    await t.pumpWidget(_wrap(const TimelineList(items: [
      ('孟买', '3 天前'),
      ('班加罗尔', '昨天'),
    ])));
    expect(t.getSize(find.byKey(const ValueKey('timeline-row-0'))).height,
        greaterThanOrEqualTo(39));
  });
}
