import 'package:bottles/app/shell.dart';
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

// AppShell 依赖 go_router 的 StatefulNavigationShell，这里只测抽出来的 AppTabBar。
Widget _wrap(Widget bar) => ProviderScope(
      child: MaterialApp(
        theme: AppTheme.light(),
        localizationsDelegates: L.localizationsDelegates,
        supportedLocales: L.supportedLocales,
        home: Scaffold(bottomNavigationBar: bar),
      ),
    );

void main() {
  testWidgets('tab bar icons are 27pt and labels 12pt', (t) async {
    await t.pumpWidget(_wrap(AppTabBar(index: 0, onTap: (_) {})));
    final icon = t.widget<Icon>(find.byType(Icon).first);
    expect(icon.size, 27);
    final label = t.widget<Text>(find.byType(Text).first);
    expect(label.style!.fontSize, 12);
  });

  testWidgets('tab bar is about 68pt tall without a bottom inset', (t) async {
    await t.pumpWidget(_wrap(AppTabBar(index: 0, onTap: (_) {})));
    expect(t.getSize(find.byType(AppTabBar)).height, closeTo(68, 3));
  });
}
