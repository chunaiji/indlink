import 'package:bottles/core/config/remote_config.dart';
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/core/storage/prefs.dart';
import 'package:bottles/data/mock/mock_repositories.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 规范 §8.2 的机型矩阵。Ahem 测试字体每个字都是正方形、比真实字体宽，
/// 这里能过的真机上一定能过。
class LayoutCase {
  const LayoutCase(
    this.name,
    this.size, {
    this.textScale = 1.0,
    this.viewPadding = EdgeInsets.zero,
    this.viewInsets = EdgeInsets.zero,
  });

  final String name;
  final Size size;
  final double textScale;
  final EdgeInsets viewPadding;
  final EdgeInsets viewInsets;

  /// 派生三键导航 / 键盘变体。
  LayoutCase withInsets({
    EdgeInsets? viewPadding,
    EdgeInsets? viewInsets,
    String suffix = '',
  }) => LayoutCase(
    '$name$suffix',
    size,
    textScale: textScale,
    viewPadding: viewPadding ?? this.viewPadding,
    viewInsets: viewInsets ?? this.viewInsets,
  );
}

const layoutMatrix = <LayoutCase>[
  LayoutCase('narrow 360x780 (Redmi / Samsung)', Size(360, 780)),
  LayoutCase('short 375x667 (iPhone SE)', Size(375, 667)),
  LayoutCase('base 393x851 (Mi 9 / iPhone 15)', Size(393, 851)),
  LayoutCase('tall 430x932 (iPhone Pro Max)', Size(430, 932)),
  LayoutCase('wide 720x860 (fold opened)', Size(720, 860)),
  LayoutCase('large-text 360x780 @1.2', Size(360, 780), textScale: 1.2),
];

/// 三键导航：底部多吃 48dp。
const threeButtonNav = EdgeInsets.only(top: 34, bottom: 48);

/// 印度低端机键盘能占到屏高 45%。
const keyboard300 = EdgeInsets.only(bottom: 300);

/// 远程配置在 build 里会发起网络拉取，测试里换成只给内置值的版本，
/// 否则 dio 的超时 Timer 会在测试结束后仍挂着。
class StaticAppConfig extends AppConfigNotifier {
  @override
  AppRemoteConfig build() => const AppRemoteConfig.builtin();
}

/// 把 [page] 放进带主题与本地化的 MaterialApp，按 [cfg] 设窗口、字体缩放、
/// 安全区与键盘，pump 一帧后断言没有 overflow 之类的异常。
Future<void> pumpAt(
  WidgetTester tester,
  LayoutCase cfg,
  Widget page, {
  List<Override> overrides = const [],

  /// 测试自己要覆盖的仓库 provider 放这里，默认的 Mock 覆盖会跳过它（同一容器不能覆盖两次）。
  Set<Object> skipDefaults = const {},
  bool dark = false,
}) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await Prefs.load();

  // overflow 的「error-causing widget」只在 FlutterErrorDetails 里，takeException
  // 拿不到；在这里截一份 details 的全文，失败时一并打印。
  final original = FlutterError.onError;
  final log = <String>[];
  FlutterError.onError = (details) {
    log.add(details.toString());
    original?.call(details);
  };
  addTearDown(() => FlutterError.onError = original);

  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = cfg.size;
  tester.view.viewPadding = _fake(cfg.viewPadding);
  tester.view.padding = _fake(cfg.viewPadding);
  tester.view.viewInsets = _fake(cfg.viewInsets);
  tester.platformDispatcher.textScaleFactorTestValue = cfg.textScale;
  addTearDown(() {
    tester.view.reset();
    tester.platformDispatcher.clearTextScaleFactorTestValue();
  });

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        ...defaultOverrides(prefs, skipDefaults: skipDefaults),
        ...overrides,
      ],
      child: MaterialApp(
        theme: AppTheme.light(),
        darkTheme: AppTheme.dark(),
        themeMode: dark ? ThemeMode.dark : ThemeMode.light,
        localizationsDelegates: L.localizationsDelegates,
        supportedLocales: L.supportedLocales,
        home: page,
      ),
    ),
  );
  await tester.pump();
  _expectNoException(tester, '${cfg.name} (first frame)', log);
  // Mock 仓库带 160～300ms 的模拟延迟，而且会串联（资料到了才去拉动态）。
  // 推三轮 400ms 让链式拉取全部落地，再看加载完成态：骨架屏不爆不代表真实内容不爆。
  for (var i = 0; i < 3; i++) {
    await tester.pump(const Duration(milliseconds: 400));
  }
  _expectNoException(tester, '${cfg.name} (loaded)', log);
}

/// 测试里的默认覆盖：prefs、只给内置值的远程配置、全部仓库换 Mock、不连 WebSocket。
/// 路由级测试（MaterialApp.router）也用它，别再手抄一份。
List<Override> defaultOverrides(
  Prefs prefs, {
  Set<Object> skipDefaults = const {},
}) => [
  prefsProvider.overrideWithValue(prefs),
  if (!skipDefaults.contains(appConfigProvider))
    appConfigProvider.overrideWith(StaticAppConfig.new),
  // 全部仓库换成 Mock：页面 build 时的拉取不能碰网络，
  // 否则 dio 的超时 Timer 会让每个布局测试都以 pending timer 收场。
  if (!skipDefaults.contains(authRepoProvider))
    authRepoProvider.overrideWith(
      (ref) => MockAuthRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(bottleRepoProvider))
    bottleRepoProvider.overrideWith(
      (ref) => MockBottleRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(geoRepoProvider))
    geoRepoProvider.overrideWith((ref) => MockGeoRepository()),
  if (!skipDefaults.contains(discoverRepoProvider))
    discoverRepoProvider.overrideWith(
      (ref) => MockDiscoverRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(chatRepoProvider))
    chatRepoProvider.overrideWith(
      (ref) => MockChatRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(momentRepoProvider))
    momentRepoProvider.overrideWith(
      (ref) => MockMomentRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(walletRepoProvider))
    walletRepoProvider.overrideWith(
      (ref) => MockWalletRepository(ref.watch(mockBackendProvider)),
    ),
  if (!skipDefaults.contains(notifyRepoProvider))
    notifyRepoProvider.overrideWith(
      (ref) => MockNotifyRepository(ref.watch(mockBackendProvider)),
    ),
  chatSocketProvider.overrideWith((ref) => null),
];

/// 失败时把截到的 FlutterErrorDetails 全文（含「error-causing widget」的 file:line）
/// 一并打印，否则只能看到「overflowed by 7.3 pixels」却不知道是谁。
void _expectNoException(WidgetTester tester, String stage, List<String> log) {
  final e = tester.takeException();
  if (e == null) return;
  final where = log
      .expand((d) => d.split('\n'))
      .where((line) => line.contains('file:///') && line.contains('/lib/'))
      .toSet()
      .join('\n');
  fail('$stage\n$e\n$where');
}

FakeViewPadding _fake(EdgeInsets e) =>
    FakeViewPadding(left: e.left, top: e.top, right: e.right, bottom: e.bottom);
