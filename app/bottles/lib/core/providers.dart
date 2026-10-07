import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_cache_manager/flutter_cache_manager.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/mock/mock_backend.dart';
import '../data/mock/mock_repositories.dart';
import '../data/remote/remote_repositories.dart';
import '../data/repositories.dart';
import '../domain/models/spark.dart';
import '../domain/models/bottle.dart';
import '../domain/models/user.dart';
import 'config/app_config.dart';
import 'config/remote_config.dart';
import 'logging/log_config.dart';
import 'logging/log_record.dart';
import 'logging/logger.dart';
import 'network/api_client.dart';
import 'network/chat_socket.dart';
import 'storage/prefs.dart';
import 'storage/credential_store.dart';
import 'storage/token_store.dart';

/// 在 `main()` 里用 `overrideWithValue` 注入，避免全应用等一个异步初始化。
final prefsProvider = Provider<Prefs>((ref) {
  throw UnimplementedError('prefsProvider must be overridden in main()');
});

final tokenStoreProvider = Provider<TokenStore>((ref) => TokenStore());

/// 「记住密码」的安全存储（Keychain / Keystore）。
final credentialStoreProvider = Provider<CredentialStore>(
  (ref) => CredentialStore(),
);

final mockBackendProvider = Provider<MockBackend>((ref) {
  final db = MockBackend();
  ref.onDispose(db.dispose);
  return db;
});

final apiClientProvider = Provider<ApiClient>((ref) {
  final client = ApiClient(
    ref.watch(tokenStoreProvider),
    onUnauthorized: () async {
      await ref.read(tokenStoreProvider).clear();
      ref.read(authProvider.notifier).signedOut();
    },
  );
  final locale = ref.watch(localeProvider);
  client.languageTag = locale?.toLanguageTag() ?? 'zh-CN';
  return client;
});

final chatSocketProvider = Provider<ChatSocket?>((ref) {
  if (AppConfig.useMock) return null;
  final me = ref.watch(authProvider).profile;
  if (me == null) return null;
  final socket = ChatSocket(myId: me.id);
  final token = ref.read(tokenStoreProvider).cached;
  if (token != null) socket.connect(token);
  ref.onDispose(socket.dispose);
  return socket;
});

// ------------------------------------------------------------------ 仓储

final authRepoProvider = Provider<AuthRepository>((ref) {
  if (AppConfig.useMock) {
    return MockAuthRepository(ref.watch(mockBackendProvider));
  }
  final prefs = ref.watch(prefsProvider);
  return RemoteAuthRepository(
    ref.watch(apiClientProvider),
    // 服务端下发的日志配置：立即生效，并缓存供下次冷启动用——
    // 那时还没有任何请求成功，只能靠缓存。
    onLogConfig: (cfg) {
      Log.updateConfig(
        LogConfig.merge(
          base: const LogConfig.defaults(),
          server: cfg,
          devForce: prefs.logDevForce,
        ),
      );
      prefs.setLogConfig(cfg);
    },
  );
});

final bottleRepoProvider = Provider<BottleRepository>((ref) {
  return AppConfig.useMock
      ? MockBottleRepository(ref.watch(mockBackendProvider))
      : RemoteBottleRepository(ref.watch(apiClientProvider));
});

/// 逆地理。Mock 返回一组固定的假地名，让地图选点在没网/没 Key 时也能走通交互。
final geoRepoProvider = Provider<GeoRepository>((ref) {
  return AppConfig.useMock
      ? MockGeoRepository()
      : RemoteGeoRepository(ref.watch(apiClientProvider));
});

final discoverRepoProvider = Provider<DiscoverRepository>((ref) {
  return AppConfig.useMock
      ? MockDiscoverRepository(ref.watch(mockBackendProvider))
      : RemoteDiscoverRepository(ref.watch(apiClientProvider));
});

final chatRepoProvider = Provider<ChatRepository>((ref) {
  if (AppConfig.useMock) {
    return MockChatRepository(ref.watch(mockBackendProvider));
  }
  final socket = ref.watch(chatSocketProvider);
  return RemoteChatRepository(
    ref.watch(apiClientProvider),
    myId: ref.watch(authProvider).profile?.id ?? '',
    socket: socket?.messages ?? const Stream.empty(),
  );
});

final momentRepoProvider = Provider<MomentRepository>((ref) {
  return AppConfig.useMock
      ? MockMomentRepository(ref.watch(mockBackendProvider))
      : RemoteMomentRepository(ref.watch(apiClientProvider));
});

final walletRepoProvider = Provider<WalletRepository>((ref) {
  return AppConfig.useMock
      ? MockWalletRepository(ref.watch(mockBackendProvider))
      : RemoteWalletRepository(ref.watch(apiClientProvider));
});

final notifyRepoProvider = Provider<NotifyRepository>((ref) {
  return AppConfig.useMock
      ? MockNotifyRepository(ref.watch(mockBackendProvider))
      : RemoteNotifyRepository(ref.watch(apiClientProvider));
});

// -------------------------------------------------------------- 登录态

enum AuthStatus {
  /// 启动时读 secure storage + 静默校验 JWT，期间停在 A1 启动页。
  checking,
  unauthenticated,

  /// 新注册用户先走「完善资料」两步引导，未完成不进主 Tab。
  onboarding,
  authenticated,
}

class AuthState {
  const AuthState({required this.status, this.profile});

  final AuthStatus status;
  final UserProfile? profile;

  bool get isLoggedIn =>
      status == AuthStatus.authenticated || status == AuthStatus.onboarding;
}

final authProvider = NotifierProvider<AuthNotifier, AuthState>(
  AuthNotifier.new,
);

class AuthNotifier extends Notifier<AuthState> {
  @override
  AuthState build() => const AuthState(status: AuthStatus.checking);

  /// 启动鉴权（A1）：有 JWT 就静默校验，401 清 token 回登录页。
  Future<void> bootstrap() async {
    final store = ref.read(tokenStoreProvider);

    // Keychain / Keystore 读取失败（如设备异常、测试环境无平台通道）
    // 等价于「没有登录态」，不能让启动卡死。
    String? token;
    try {
      token = await store.read();
    } catch (_) {
      token = null;
    }

    if (token == null || token.isEmpty) {
      state = const AuthState(status: AuthStatus.unauthenticated);
      return;
    }

    try {
      final profile = await ref.read(authRepoProvider).fetchProfile();
      state = AuthState(status: AuthStatus.authenticated, profile: profile);
    } catch (_) {
      await store.clear();
      state = const AuthState(status: AuthStatus.unauthenticated);
    }
  }

  Future<void> completeLogin(AuthResult result) async {
    await ref.read(tokenStoreProvider).write(result.token);
    state = AuthState(
      status: result.isNew ? AuthStatus.onboarding : AuthStatus.authenticated,
      profile: result.profile,
    );
  }

  void updateProfile(UserProfile profile) {
    state = AuthState(status: state.status, profile: profile);
  }

  /// 完成「完善资料」后进主 Tab。
  void finishOnboarding(UserProfile profile) {
    state = AuthState(status: AuthStatus.authenticated, profile: profile);
  }

  void signedOut() {
    state = const AuthState(status: AuthStatus.unauthenticated);
  }

  Future<void> signOut() async {
    await ref.read(tokenStoreProvider).clear();
    signedOut();
  }

  Future<void> deleteAccount() async {
    await ref.read(authRepoProvider).deleteAccount();
    await signOut();
  }
}

// ---------------------------------------------------------- 全局共享数据

/// 余额在顶部栏、礼物面板、充值页、扣费拦截四处出现，收在一个 provider 里。
final walletProvider = AsyncNotifierProvider<WalletNotifier, Wallet>(
  WalletNotifier.new,
);

class WalletNotifier extends AsyncNotifier<Wallet> {
  @override
  Future<Wallet> build() {
    // 跟着当前用户走：注册 / 换号后要重拉，否则看到的是上一个人的（或登录前的 0）余额。
    ref.watch(authProvider.select((s) => s.profile?.id));
    return ref.watch(walletRepoProvider).wallet();
  }

  Future<void> refresh() async {
    state = AsyncData(await ref.read(walletRepoProvider).wallet());
  }

  void setLocal(Wallet w) => state = AsyncData(w);
}

/// 每日免费次数。首页两个按钮、次数用尽弹窗都读它。
final quotaProvider = AsyncNotifierProvider<QuotaNotifier, QuotaInfo>(
  QuotaNotifier.new,
);

class QuotaNotifier extends AsyncNotifier<QuotaInfo> {
  @override
  Future<QuotaInfo> build() {
    ref.watch(authProvider.select((s) => s.profile?.id));
    return ref.watch(bottleRepoProvider).quota();
  }

  Future<void> refresh() async {
    state = AsyncData(await ref.read(bottleRepoProvider).quota());
  }

  void setLocal(QuotaInfo q) => state = AsyncData(q);
}

/// 消息 Tab 的未读红点。WS 推来新消息时本地累加，不重拉列表。
final unreadProvider = NotifierProvider<UnreadNotifier, int>(
  UnreadNotifier.new,
);

class UnreadNotifier extends Notifier<int> {
  @override
  int build() => 0;

  void set(int v) => state = v;
  void increment() => state = state + 1;

  /// 进入该 Tab 后延迟 400ms 再清零——立刻消失会让人怀疑自己看错了。
  Future<void> clearDelayed() async {
    await Future<void>.delayed(const Duration(milliseconds: 400));
    state = 0;
  }
}

// ------------------------------------------------------------ 偏好设置

/// null = 跟随系统。
final localeProvider = NotifierProvider<LocaleNotifier, Locale?>(
  LocaleNotifier.new,
);

class LocaleNotifier extends Notifier<Locale?> {
  @override
  Locale? build() {
    final code = ref.watch(prefsProvider).localeCode;
    return code == null ? null : Locale(code);
  }

  Future<void> set(Locale? locale) async {
    await ref.read(prefsProvider).setLocaleCode(locale?.languageCode);
    state = locale;
  }
}

final themeModeProvider = NotifierProvider<ThemeModeNotifier, ThemeMode>(
  ThemeModeNotifier.new,
);

class ThemeModeNotifier extends Notifier<ThemeMode> {
  @override
  ThemeMode build() => switch (ref.watch(prefsProvider).themeMode) {
    'light' => ThemeMode.light,
    'dark' => ThemeMode.dark,
    _ => ThemeMode.system,
  };

  Future<void> set(ThemeMode mode) async {
    await ref.read(prefsProvider).setThemeMode(mode.name);
    state = mode;
  }
}

// ------------------------------------------------------- 运营远程配置

/// `GET /api/app-config` —— 运营在管理后台配的文案、「我的」入口显隐、客服。
///
/// **同步给值，不阻塞启动**：首帧直接用 Prefs 缓存（首次安装就用内置值），
/// 拉取在后台进行，回来后 Riverpod 自然重建。
///
/// 做成同步 [Notifier] 而不是 [AsyncNotifier]，是因为调用点全在渲染路径上
/// （标题、弹框文案、入口显隐）。给它们一个 AsyncValue 等于让每处都写一遍
/// loading 分支，而「还没拉到」的正确表现本来就是「用内置值」，不是转圈。
final appConfigProvider = NotifierProvider<AppConfigNotifier, AppRemoteConfig>(
  AppConfigNotifier.new,
);

class AppConfigNotifier extends Notifier<AppRemoteConfig> {
  @override
  AppRemoteConfig build() {
    final prefs = ref.watch(prefsProvider);
    // 语种是这份配置的一部分：中英文在服务端是两批键，切语言必须重拉。
    // 重拉回来之前会短暂沿用上一语种的缓存文案，一帧的事，不值得挡住界面。
    final lang = ref.watch(localeProvider)?.toLanguageTag() ?? 'zh-CN';

    var alive = true;
    ref.onDispose(() => alive = false);
    unawaited(_fetch(lang, () => alive));

    final cached = prefs.appConfig;
    return cached == null
        ? const AppRemoteConfig.builtin()
        : AppRemoteConfig.fromJson(cached);
  }

  Future<void> _fetch(String lang, bool Function() alive) async {
    try {
      final cfg = await ref.read(authRepoProvider).appConfig(lang: lang);
      if (!alive()) return;
      state = cfg;
      await ref.read(prefsProvider).setAppConfig(cfg.toJson());
      unawaited(prefetchSplashImages(cfg.splashImages));
    } catch (e) {
      // 拉不到就维持现状（缓存或内置），不打扰用户——配置失败不是用户的事。
      // 但要留痕：「后台改了文案 App 没变」时，这条是唯一线索。
      Log.w(
        LogTag.sys,
        'app-config 拉取失败，沿用本地值',
        fields: {'lang': lang, 'err': e.toString()},
      );
    }
  }
}

/// 把后台配的启动页图静默下载进本地缓存（`DefaultCacheManager`，与网络图片共用一套）。
///
/// 失败静默：启动页是锦上添花，下不下来都不该打扰用户；下次拉配置会再试。
Future<void> prefetchSplashImages(List<String> urls) async {
  for (final url in urls) {
    try {
      await DefaultCacheManager().downloadFile(url);
    } catch (e) {
      Log.w(
        LogTag.sys,
        '启动页图片预下载失败',
        fields: {'url': url, 'err': e.toString()},
      );
    }
  }
}

/// 匿名发件人显示名。后台配了用配的，没配用 ARB 内置（[builtin] 传 `l.commonAnonymous`）。
///
/// 详情页作者行、回复行、举报目标、捞瓶信纸四处都要它，
/// 收在这里省得各写各的空值判断然后慢慢写出四种不一样的结果。
String anonSenderName(WidgetRef ref, String builtin) =>
    AppRemoteConfig.or(ref.watch(appConfigProvider).anonSender, builtin);

/// 夜场：21:00–01:00 的**产品状态**，与系统暗色是两根独立的轴。
/// 四种组合都要成立，所以它不读 Brightness，只看钟点。
final nightModeProvider = Provider<bool>((ref) {
  final h = DateTime.now().hour;
  return h >= 21 || h < 1;
});

/// 海面皮肤：用户在海洋页顶部手动切的白天 / 夜晚，持久化在 Prefs；auto = 跟随钟点。
///
/// ⚠️ 这只是**画法**。「夜场」（[nightModeProvider]）是产品状态——夜瓶只在夜场可捞、
/// 夜场扔瓶不限次——不随皮肤变。把海面切成夜晚不会提前开夜场。
enum OceanSkin { auto, day, night }

final oceanSkinProvider = NotifierProvider<OceanSkinNotifier, OceanSkin>(
  OceanSkinNotifier.new,
);

class OceanSkinNotifier extends Notifier<OceanSkin> {
  @override
  OceanSkin build() => switch (ref.watch(prefsProvider).oceanSkin) {
    'day' => OceanSkin.day,
    'night' => OceanSkin.night,
    _ => OceanSkin.auto,
  };

  Future<void> set(OceanSkin skin) async {
    await ref.read(prefsProvider).setOceanSkin(skin.name);
    state = skin;
  }
}

/// 实际画哪一张海：手动皮肤优先；auto 跟着夜场走。
enum SeaLook { day, night }

final seaLookProvider = Provider<SeaLook>((ref) {
  switch (ref.watch(oceanSkinProvider)) {
    case OceanSkin.day:
      return SeaLook.day;
    case OceanSkin.night:
      return SeaLook.night;
    case OceanSkin.auto:
      return ref.watch(nightModeProvider) ? SeaLook.night : SeaLook.day;
  }
});

final sparkRepoProvider = Provider<SparkRepository>((ref) {
  if (AppConfig.useMock) return MockSparkRepository();
  return RemoteSparkRepository(ref.watch(apiClientProvider));
});

/// 火花事件流。语种在这里才定，跟随当前 locale。
final sparkStreamProvider = StreamProvider<SparkEvent>((ref) {
  final socket = ref.watch(chatSocketProvider);
  if (socket == null) return const Stream<SparkEvent>.empty();
  final english = ref.watch(localeProvider)?.languageCode == 'en';
  return socket.sparkFrames.map((m) => SparkEvent.fromJson(m, english: english));
});
