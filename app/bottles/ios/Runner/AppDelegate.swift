import Flutter
import GoogleMaps
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    // Maps SDK 必须在任何 GMSMapView 创建之前初始化,否则地图直接崩。
    //
    // Key 从 Info.plist 读,而那一项的值由 ios/Flutter/Maps.xcconfig 注入
    // (该文件在 .gitignore 里)。没配时跳过初始化——地图会空白并在控制台报错,
    // 但 App 其余部分照常工作,不至于因为少一把 Key 就起不来。
    if let key = Bundle.main.object(forInfoDictionaryKey: "GMSApiKey") as? String,
       !key.isEmpty, !key.hasPrefix("$(") {
      GMSServices.provideAPIKey(key)
    }
    excludeLogsFromBackup()

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  /// 日志目录排除 iCloud 备份。
  ///
  /// Application Support 默认会进 iCloud,日志跟着上云既无意义又扩大暴露面,
  /// 换机恢复时还会被带过去。path_provider 不暴露这个属性,只能在这里打标记。
  ///
  /// 目录可能还不存在(首次启动时 Dart 侧尚未创建),所以先建再标记;
  /// 失败只打日志不中断启动——排不掉备份不该让 App 起不来。
  private func excludeLogsFromBackup() {
    guard let support = FileManager.default.urls(
      for: .applicationSupportDirectory, in: .userDomainMask).first else { return }
    var logs = support.appendingPathComponent("logs", isDirectory: true)
    do {
      try FileManager.default.createDirectory(
        at: logs, withIntermediateDirectories: true)
      var values = URLResourceValues()
      values.isExcludedFromBackup = true
      try logs.setResourceValues(values)
    } catch {
      NSLog("[log] 排除备份失败: \(error)")
    }
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
  }
}
