import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';

/// 一次定位结果。
///
/// 只有经纬度——地址由服务端 `GET /geo/regeo` 逆地理得到。
/// 客户端不持有 Geocoding 的 Key，那把 Key 只在服务端（见后台「App 地理」分组）。
class LocationFix {
  const LocationFix(this.lat, this.lng);
  final double lat;
  final double lng;
}

/// 定位权限的三种状态。
///
/// 把「被永久拒绝」单列出来，是因为它决定了 UI 该说什么：
/// 还能再问 → 显示 A7 说明屏；已被永久拒绝 → 只能引导去系统设置。
enum LocationPermissionState { granted, denied, deniedForever }

/// 把定位插件关在这一个文件里，理由同 `haptics.dart`：
/// 插件 API 变动频繁，隔离之后升级只改这一处。
abstract class LocationClient {
  Future<LocationPermissionState> permission();

  /// 弹系统权限框。
  ///
  /// ⚠️ iOS 上这个框**一辈子只弹一次**，拒绝后再也唤不起来，
  /// 所以调用前必须先过 A7 说明屏，让用户知道自己在同意什么。
  Future<LocationPermissionState> request();

  /// 当前位置。拿不到时返回 null——**不抛异常**。
  ///
  /// 「没有位置」是正常状态而不是错误：用户可以拒绝授权、可以关掉系统定位服务、
  /// 可以在室内拿不到卫星。整个产品对此的态度是降级而非阻断。
  Future<LocationFix?> current();

  /// 永久拒绝后系统框唤不起来，只能把人送去系统设置（Z6 的「去设置开启」）。
  Future<void> openSettings();
}

class RealLocationClient implements LocationClient {
  @override
  Future<void> openSettings() async {
    await Geolocator.openAppSettings();
  }

  @override
  Future<LocationPermissionState> permission() async {
    return _map(await Geolocator.checkPermission());
  }

  @override
  Future<LocationPermissionState> request() async {
    return _map(await Geolocator.requestPermission());
  }

  @override
  Future<LocationFix?> current() async {
    try {
      if (!await Geolocator.isLocationServiceEnabled()) return null;
      if (await permission() != LocationPermissionState.granted) return null;
      final pos = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.medium,
          // 定位是锦上添花，不值得让用户干等。拿不到就先不带位置，
          // 下一次进 App 再试。
          timeLimit: Duration(seconds: 8),
        ),
      );
      return LocationFix(pos.latitude, pos.longitude);
    } catch (_) {
      // 超时、硬件故障、模拟器没设位置——一律当作「这次拿不到」。
      return null;
    }
  }

  LocationPermissionState _map(LocationPermission p) => switch (p) {
    LocationPermission.always ||
    LocationPermission.whileInUse => LocationPermissionState.granted,
    LocationPermission.deniedForever => LocationPermissionState.deniedForever,
    _ => LocationPermissionState.denied,
  };
}

final locationClientProvider = Provider<LocationClient>(
  (ref) => RealLocationClient(),
);
