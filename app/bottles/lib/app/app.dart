import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/design/theme.dart';
import '../core/design/tokens.dart';
import '../core/providers.dart';
import '../l10n/app_localizations.dart';
import '../features/spark/spark_overlay.dart';
import 'router.dart';

class BottlesApp extends ConsumerStatefulWidget {
  const BottlesApp({super.key});

  @override
  ConsumerState<BottlesApp> createState() => _BottlesAppState();
}

class _BottlesAppState extends ConsumerState<BottlesApp> {
  late final AppLifecycleListener _lifecycle;

  @override
  void initState() {
    super.initState();
    // 前后台切换时断开/重连长连接：省电，也避免服务端挂着僵尸连接。
    _lifecycle = AppLifecycleListener(
      onPause: () => ref.read(chatSocketProvider)?.pause(),
      onResume: () => ref.read(chatSocketProvider)?.resume(),
    );
  }

  @override
  void dispose() {
    _lifecycle.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // 火花弹窗:任意页面都要能弹,所以挂在 App 根上用 root navigator 的 context。
    ref.listen(sparkStreamProvider, (_, next) {
      final e = next.value;
      final ctx = rootNavigatorKey.currentContext;
      if (e != null && ctx != null) showSparkDialog(ctx, e);
    });

    return MaterialApp.router(
      title: 'Drift',
      debugShowCheckedModeBanner: false,
      routerConfig: ref.watch(routerProvider),
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      themeMode: ref.watch(themeModeProvider),

      // 文字缩放只在这里钳一次（规范 §4.5）：MIUI / EMUI 用户开「大字体」很常见，
      // 不钳全 App 放大 1.3 倍、按钮换行；钳死成 1.0 又是无障碍投诉来源。
      // 页面里禁止再包一层 MediaQuery 覆盖它。
      builder: (context, child) {
        final mq = MediaQuery.of(context);
        return MediaQuery(
          data: mq.copyWith(
            textScaler: mq.textScaler.clamp(
              minScaleFactor: Breakpoints.textScaleMin,
              maxScaleFactor: Breakpoints.textScaleMax,
            ),
          ),
          child: child ?? const SizedBox.shrink(),
        );
      },

      // null = 跟随系统；系统语言不在支持列表时回退到中文（模板语言）。
      locale: ref.watch(localeProvider),
      localizationsDelegates: L.localizationsDelegates,
      supportedLocales: L.supportedLocales,
      localeResolutionCallback: (device, supported) {
        if (device == null) return supported.first;
        for (final s in supported) {
          if (s.languageCode == device.languageCode) return s;
        }
        return supported.first;
      },
    );
  }
}
