import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/platform/location.dart';
import '../../core/providers.dart';

/// 定位状态。
///
/// [fix] 为 null 表示「这次没拿到位置」——可能是没授权、系统定位关着、
/// 或者室内拿不到卫星。三种情况对产品的意义一样：**降级，不阻断**。
class LocationState {
  const LocationState({this.fix, this.permission, this.busy = false});

  final LocationFix? fix;
  final LocationPermissionState? permission;
  final bool busy;

  bool get hasFix => fix != null;

  /// 是否还值得弹 A7 说明屏。
  ///
  /// 已授权就不用问；被永久拒绝也不用问——那时系统框已经唤不起来了，
  /// 再显示「开启定位」按钮是骗用户点一个不会有反应的东西。
  bool get canAsk => permission == LocationPermissionState.denied;

  LocationState copyWith({
    LocationFix? fix,
    LocationPermissionState? permission,
    bool? busy,
  }) => LocationState(
    fix: fix ?? this.fix,
    permission: permission ?? this.permission,
    busy: busy ?? this.busy,
  );
}

class LocationController extends Notifier<LocationState> {
  @override
  LocationState build() => const LocationState();

  LocationClient get _client => ref.read(locationClientProvider);

  /// 进入主界面后调一次：已授权就静默取位置并上报，没授权就什么都不做。
  ///
  /// **不在这里弹系统权限框**——那是 A7 说明屏点了「开启定位」之后的事。
  /// 启动即弹框是拒绝率最高的做法，而 iOS 只给一次机会。
  Future<void> refreshIfGranted() async {
    // 设置页把定位关了：不取、不报，权限状态也不碰。
    if (!ref.read(prefsProvider).locationEnabled) return;
    final perm = await _client.permission();
    state = state.copyWith(permission: perm);
    if (perm != LocationPermissionState.granted) return;
    await _locateAndReport();
  }

  /// 用户在 A7 上点了「开启定位」。
  ///
  /// 返回是否拿到了授权，调用方据此决定要不要提示「可以去系统设置里打开」。
  Future<bool> requestAndLocate() async {
    state = state.copyWith(busy: true);
    final perm = await _client.request();
    state = state.copyWith(permission: perm, busy: false);
    if (perm != LocationPermissionState.granted) return false;
    await _locateAndReport();
    return true;
  }

  /// Z6：永久拒绝后把人送去系统设置，回来时重新读一次权限。
  Future<void> openSettings() async {
    await _client.openSettings();
    state = state.copyWith(permission: await _client.permission());
  }

  /// 取位置并上报给服务端。
  ///
  /// 上报失败**静默吞掉**：位置只影响推荐排序和距离显示，
  /// 为它弹一个错误提示是本末倒置。下次进 App 会再试一次。
  Future<void> _locateAndReport() async {
    final fix = await _client.current();
    if (fix == null) return;
    state = state.copyWith(fix: fix);
    try {
      await ref
          .read(authRepoProvider)
          .updateProfile(lat: fix.lat, lng: fix.lng);
    } catch (_) {
      // 见上：位置上报失败不打扰用户。
    }
  }
}

final locationControllerProvider =
    NotifierProvider<LocationController, LocationState>(LocationController.new);
