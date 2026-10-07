import 'package:bottles/core/design/theme.dart';
import 'package:bottles/domain/models/moment.dart';
import 'package:bottles/domain/models/user.dart';
import 'package:bottles/features/moment/moment_feed_page.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Moment _moment(int pics) => Moment(
  id: 'm',
  author: const UserBrief(id: 'u', nickname: 'Aanya'),
  text: 'hi',
  createdAt: DateTime.utc(2026, 10, 1),
  images: List.filled(pics, ''),
);

Widget _wrap(Widget w) => MaterialApp(
  theme: AppTheme.light(),
  localizationsDelegates: L.localizationsDelegates,
  supportedLocales: L.supportedLocales,
  home: Scaffold(body: SizedBox(width: 393, child: w)),
);

Rect _tile(WidgetTester t, int i) =>
    t.getRect(find.byKey(ValueKey('moment-pic-$i')));

/// 原型 .mcd .pics 是一行 flex：三张图并排各占三分之一，不是两列大图。
void main() {
  testWidgets('three pictures share one row', (t) async {
    await t.pumpWidget(_wrap(MomentCard(moment: _moment(3))));
    expect(_tile(t, 0).top, _tile(t, 2).top);
    expect(_tile(t, 0).width, closeTo(_tile(t, 2).width, .5));
    expect(_tile(t, 0).width, lessThan(130));
  });

  testWidgets('two pictures share one row at half width', (t) async {
    await t.pumpWidget(_wrap(MomentCard(moment: _moment(2))));
    expect(_tile(t, 0).top, _tile(t, 1).top);
    expect(_tile(t, 0).width, closeTo(_tile(t, 1).width, .5));
  });

  testWidgets('four pictures wrap into a 3-column grid', (t) async {
    await t.pumpWidget(_wrap(MomentCard(moment: _moment(4))));
    expect(_tile(t, 3).top, greaterThan(_tile(t, 0).top));
    expect(_tile(t, 3).left, _tile(t, 0).left);
  });
}
