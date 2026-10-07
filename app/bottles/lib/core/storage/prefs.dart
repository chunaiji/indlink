import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

/// 上次登录用的账号（不含密码——密码在 CredentialStore 的安全存储里）。
///
/// 记下渠道 / 方式 / 区号 / 号码 / 邮箱，下次打开登录页直接填好，
/// 用户只需要输密码（开了「记住密码」连密码也免了）。
class LoginMemory {
  const LoginMemory({
    this.channel = 'phone',
    this.method = 'password',
    this.dialCode = '+91',
    this.phone = '',
    this.email = '',
  });

  /// 'phone' | 'email'
  final String channel;

  /// 'password' | 'code'
  final String method;
  final String dialCode;
  final String phone;
  final String email;

  bool get isEmpty => phone.isEmpty && email.isEmpty;
}

/// 非敏感的本地偏好：语言、外观、发现页筛选条件、上次登录账号、海面皮肤、定位开关。
class Prefs {
  Prefs(this._sp);

  static const _kLocale = 'drift.locale';
  static const _kThemeMode = 'drift.themeMode';
  static const _kDiscoverFilter = 'drift.discoverFilter';
  static const _kPushAsked = 'drift.pushAsked';
  static const _kLocationAsked = 'drift.locationAsked';
  static const _kPrivacyAgreed = 'drift.privacyAgreed';
  static const _kLogConfig = 'drift.logConfig';
  static const _kLogDevForce = 'drift.logDevForce';
  static const _kAppConfig = 'drift.appConfig';
  static const _kOceanSkin = 'drift.oceanSkin';
  static const _kLocationEnabled = 'drift.locationEnabled';
  static const _kCastCityOnly = 'drift.castCityOnly';
  static const _kRememberPassword = 'drift.login.rememberPassword';
  static const _kLoginChannel = 'drift.login.channel';
  static const _kLoginMethod = 'drift.login.method';
  static const _kLoginDialCode = 'drift.login.dialCode';
  static const _kLoginPhone = 'drift.login.phone';
  static const _kLoginEmail = 'drift.login.email';

  final SharedPreferences _sp;

  static Future<Prefs> load() async =>
      Prefs(await SharedPreferences.getInstance());

  /// null = 跟随系统。
  String? get localeCode => _sp.getString(_kLocale);
  Future<void> setLocaleCode(String? code) async {
    if (code == null) {
      await _sp.remove(_kLocale);
    } else {
      await _sp.setString(_kLocale, code);
    }
  }

  /// 'system' | 'light' | 'dark'
  String get themeMode => _sp.getString(_kThemeMode) ?? 'system';
  Future<void> setThemeMode(String mode) => _sp.setString(_kThemeMode, mode);

  String? get discoverFilter => _sp.getString(_kDiscoverFilter);
  Future<void> setDiscoverFilter(String json) =>
      _sp.setString(_kDiscoverFilter, json);

  /// 推送权限弹窗只在「首次发出瓶子之后」问一次，授权率比启动时高得多。
  bool get pushAsked => _sp.getBool(_kPushAsked) ?? false;
  Future<void> setPushAsked() => _sp.setBool(_kPushAsked, true);

  /// 定位说明屏（A7）只在进入主界面后问一次。
  ///
  /// 与推送同理，且对定位更要紧：iOS 的定位系统框**一辈子只弹一次**，
  /// 拒绝之后 App 内再也唤不起来。所以必须先用自己的界面讲清用途，
  /// 而那一屏反复出现只会更快被划掉。
  bool get locationAsked => _sp.getBool(_kLocationAsked) ?? false;
  Future<void> setLocationAsked() => _sp.setBool(_kLocationAsked, true);

  /// 首次登录前隐私授权是否已同意（合规弹框，A1c）。同意后本地持久化，不再弹。
  bool get privacyAgreed => _sp.getBool(_kPrivacyAgreed) ?? false;
  Future<void> setPrivacyAgreed() => _sp.setBool(_kPrivacyAgreed, true);

  /// 上次收到的服务端日志配置（JSON）。
  ///
  /// 缓存它是为了让「运营昨天关掉了日志」在今天冷启动时就生效，
  /// 而不必等第一个请求成功——那时可能已经崩了。
  Map<String, Object?>? get logConfig => _json(_kLogConfig);

  Future<void> setLogConfig(Map<String, Object?> v) =>
      _sp.setString(_kLogConfig, jsonEncode(v));

  /// 本地开发者开关。覆盖服务端配置，只影响本机。
  /// 弥补「配置只能按租户」的限制：本机排查不该动线上配置。
  bool get logDevForce => _sp.getBool(_kLogDevForce) ?? false;
  Future<void> setLogDevForce(bool v) => _sp.setBool(_kLogDevForce, v);

  /// 上次拉到的 `/app-config`（原始 JSON）。
  ///
  /// 缓存它是为了让首帧就用上运营配的文案与入口显隐，而不是先渲染内置值、
  /// 请求回来再跳变一次。没缓存（首次安装）就用内置值，那是合法状态。
  Map<String, Object?>? get appConfig => _json(_kAppConfig);

  Future<void> setAppConfig(Map<String, Object?> v) =>
      _sp.setString(_kAppConfig, jsonEncode(v));

  // ---- 用户习惯：下次打开不用重设 ----

  /// 海面皮肤：'auto'（跟着夜场走）| 'day' | 'night'。
  /// 只改海面的画法，不改「夜场」这个产品状态（夜瓶、不限次扔瓶仍按时间）。
  String get oceanSkin => _sp.getString(_kOceanSkin) ?? 'auto';
  Future<void> setOceanSkin(String v) => _sp.setString(_kOceanSkin, v);

  /// 定位总开关（设置页）。关掉后不再取位置、不上报、不弹 A7 说明屏；
  /// 发现页与瓶子距离退化为按城市显示。默认开。
  bool get locationEnabled => _sp.getBool(_kLocationEnabled) ?? true;
  Future<void> setLocationEnabled(bool v) => _sp.setBool(_kLocationEnabled, v);

  /// 写瓶子页「精确位置 / 仅城市」上次的选择。默认精确位置。
  bool get castCityOnly => _sp.getBool(_kCastCityOnly) ?? false;
  Future<void> setCastCityOnly(bool v) => _sp.setBool(_kCastCityOnly, v);

  /// 「记住密码」开关。默认开——用户要的就是不用每次都输。
  bool get rememberPassword => _sp.getBool(_kRememberPassword) ?? true;
  Future<void> setRememberPassword(bool v) =>
      _sp.setBool(_kRememberPassword, v);

  LoginMemory get loginMemory => LoginMemory(
    channel: _sp.getString(_kLoginChannel) ?? 'phone',
    method: _sp.getString(_kLoginMethod) ?? 'password',
    dialCode: _sp.getString(_kLoginDialCode) ?? '+91',
    phone: _sp.getString(_kLoginPhone) ?? '',
    email: _sp.getString(_kLoginEmail) ?? '',
  );

  Future<void> setLoginMemory(LoginMemory m) async {
    await _sp.setString(_kLoginChannel, m.channel);
    await _sp.setString(_kLoginMethod, m.method);
    await _sp.setString(_kLoginDialCode, m.dialCode);
    await _sp.setString(_kLoginPhone, m.phone);
    await _sp.setString(_kLoginEmail, m.email);
  }

  Map<String, Object?>? _json(String key) {
    final s = _sp.getString(key);
    if (s == null || s.isEmpty) return null;
    try {
      return jsonDecode(s) as Map<String, Object?>;
    } catch (_) {
      return null;
    }
  }
}
