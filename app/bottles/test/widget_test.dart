import 'package:bottles/app/app.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/core/storage/prefs.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('App boots into the login screen when no token is stored',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await Prefs.load();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [prefsProvider.overrideWithValue(prefs)],
        child: const BottlesApp(),
      ),
    );

    // A1 启动页先出现，随后 redirect 守卫把没有登录态的用户带到登录页。
    await tester.pump();
    expect(find.text('DRIFT'), findsWidgets);
  });
}
