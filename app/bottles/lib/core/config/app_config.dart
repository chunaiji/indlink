/// 运行期配置 —— 全部可由 `--dart-define` 覆盖，不写死在代码里。
///
/// ```
/// flutter run --dart-define=USE_MOCK=false \
///             --dart-define=API_BASE=https://ambertu.com/message/api \
///             --dart-define=WS_BASE=wss://ambertu.com/message/ws
/// ```
class AppConfig {
  const AppConfig._();

  /// 默认走真实后端。Mock 数据源保留下来是为了**没有网络也能改 UI**
  /// （以及 widget test 不依赖服务端），用 `--dart-define=USE_MOCK=true` 切回去。
  static const bool useMock =
      bool.fromEnvironment('USE_MOCK', defaultValue: false);

  static const String apiBase = String.fromEnvironment(
    'API_BASE',
    defaultValue: 'https://ambertu.com/message/api',
  );

  static const String wsBase = String.fromEnvironment(
    'WS_BASE',
    defaultValue: 'wss://ambertu.com/message/ws',
  );

  /// 租户识别沿用小程序那套多租户机制：`app_credentials` 里 `platform="app"` 的那行 appid。
  /// 打包时注入，不提交到仓库。
  static const String appId = String.fromEnvironment(
    'APP_ID',
    defaultValue: 'drift_app_dev',
  );

  static const Duration connectTimeout = Duration(seconds: 10);
  static const Duration receiveTimeout = Duration(seconds: 15);
}
