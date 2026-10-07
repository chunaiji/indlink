import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// 震动。
///
/// ⚠️ 不要直接用 `HapticFeedback.*`：Android 上它走 `View.performHapticFeedback`，
/// 被系统「触感反馈 / 触摸震动」这个开关一票否决——关掉就**完全静默**，
/// 而且 `selectionClick`(CLOCK_TICK)、`mediumImpact`(KEYBOARD_TAP) 这两档
/// 在多数真机上弱到根本感觉不到。捞瓶仪式的「拉扯感」全靠震动，不能听天由命。
///
/// 所以 Android 走原生 `Vibrator`（`drift/haptics` 通道 + VIBRATE 权限），
/// 时长与强度都可控；iOS 与其他平台仍用系统 haptic（那边表现一直是对的）。
class Haptics {
  const Haptics._();

  static const _channel = MethodChannel('drift/haptics');

  /// 设备有没有振动器。查一次缓存住——通道调用不贵，但它在动画里每 350ms 被问一次。
  static bool? _hasVibrator;

  static bool get _useNative => !kIsWeb && Platform.isAndroid;

  static Future<bool> _available() async {
    if (!_useNative) return false;
    if (_hasVibrator != null) return _hasVibrator!;
    try {
      _hasVibrator = await _channel.invokeMethod<bool>('hasVibrator') ?? false;
    } on PlatformException {
      _hasVibrator = false;
    } on MissingPluginException {
      // 旧版本安装包里没有这个通道（热重载 / 增量安装常见），降级别崩。
      _hasVibrator = false;
    }
    return _hasVibrator!;
  }

  /// [ms] 时长；[amplitude] 1~255，设备不支持强度控制时忽略。
  static Future<void> _buzz(int ms, int amplitude) async {
    if (!await _available()) return;
    try {
      await _channel.invokeMethod<void>('vibrate', {
        'ms': ms,
        'amplitude': amplitude,
      });
    } on PlatformException {
      // 震动失败不值得打断任何流程。
    }
  }

  /// 轻：选中、松手落定。
  static Future<void> light() =>
      _useNative ? _buzz(12, 90) : HapticFeedback.lightImpact();

  /// 中：起手、状态切换。
  static Future<void> medium() =>
      _useNative ? _buzz(22, 160) : HapticFeedback.mediumImpact();

  /// 重：咬钩、成功落袋这种情绪顶点。
  static Future<void> heavy() =>
      _useNative ? _buzz(45, 255) : HapticFeedback.heavyImpact();

  /// 拉扯：仪式期间每一下的「咯噔」。比 light 稍长，连成一串才有绷紧感。
  static Future<void> tick() =>
      _useNative ? _buzz(18, 130) : HapticFeedback.selectionClick();
}
