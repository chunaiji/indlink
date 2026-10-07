import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/app.dart';
import 'core/logging/log_config.dart';
import 'core/logging/log_record.dart';
import 'core/logging/logger.dart';
import 'core/providers.dart';
import 'core/storage/prefs.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 手机锁竖屏（规范 D4）：少一整个维度的测试面。
  // 规范 §4.8 给图片查看器留了横屏例外，尚未实现——实现时由它自己临时放开、退出时恢复。
  await SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);

  // Android 15 起系统默认 edge-to-edge，这里统一开启并把系统栏画成透明，
  // 三地区机型行为一致；页面靠 SafeArea 避让，不依赖系统自动加 padding（规范 §4.4）。
  await SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      systemNavigationBarColor: Colors.transparent,
      systemNavigationBarDividerColor: Colors.transparent,
      systemNavigationBarContrastEnforced: false,
    ),
  );

  // 偏好设置（语言 / 外观 / 筛选条件）在首帧前读好，
  // 避免整个应用为了一个异步初始化多套一层 loading。
  final prefs = await Prefs.load();

  // 日志要在任何业务代码之前就绪：**启动崩溃与登录失败恰恰最需要它**。
  // 配置先用编译期默认值 + 上次缓存的服务端值，本次下发的值稍后覆盖。
  await Log.init(
    config: LogConfig.merge(
      base: const LogConfig.defaults(),
      server: prefs.logConfig,
      devForce: prefs.logDevForce,
    ),
  );

  // 两个钩子覆盖完整，**不用 runZonedGuarded**：多一层 zone 要求 runApp
  // 在同一 zone 内调用，否则 Zone mismatch；与 ensureInitialized 的顺序
  // 也很讲究，配错的表现是启动即崩（spec 决策 10）。
  final prevOnError = FlutterError.onError;
  FlutterError.onError = (details) {
    Log.e(LogTag.sys, 'flutter error',
        error: details.exception, stack: details.stack);
    prevOnError?.call(details);
  };

  PlatformDispatcher.instance.onError = (error, stack) {
    Log.e(LogTag.sys, 'uncaught async', error: error, stack: stack);
    return false; // 不吞掉，继续走默认处理
  };

  runApp(
    ProviderScope(
      overrides: [prefsProvider.overrideWithValue(prefs)],
      child: const BottlesApp(),
    ),
  );
}
