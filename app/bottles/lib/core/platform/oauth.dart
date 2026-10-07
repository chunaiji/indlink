import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import '../providers.dart';

/// Apple 登录按钮是否显示。
///
/// 先判 `kIsWeb` 再判 `Platform.isIOS`：Web 端访问 `dart:io` 的 Platform 会直接抛异常，
/// 顺序反了 Web 构建会在运行期崩。
///
/// App Store 4.8 只约束 iOS。Android 上不显示是对的——`sign_in_with_apple`
/// 在 Android 会退化成需要另配 Service ID 的网页流程，显示了也点不通。
bool get showAppleSignIn => !kIsWeb && Platform.isIOS;

/// 用户主动取消。
///
/// 单独一个类型而不是返回 null：调用方要区分「用户改主意了」（静默返回，
/// 不该弹任何错误提示）和「拿 token 失败了」（该提示）。
class OAuthCancelled implements Exception {
  const OAuthCancelled();
}

/// 把两个第三方登录 SDK 关在这一个文件里。
///
/// 对外只暴露「给我一个 idToken」，因为服务端要的就只有这个——昵称、头像、邮箱
/// 一律以服务端验签后的 claims 为准，不信客户端传上来的任何身份信息。
///
/// 单独成文件是因为这两个 SDK 的 API 变动频繁（google_sign_in 6.x → 7.x 换过一次），
/// 隔离之后升级只改这一处，不波及 auth_controller 与登录页。
abstract class OAuthClient {
  /// 取 Google ID Token。用户取消时抛 [OAuthCancelled]。
  Future<String> googleIdToken();

  /// 取 Apple identityToken。仅 iOS 可用，用户取消时抛 [OAuthCancelled]。
  Future<String> appleIdToken();
}

class RealOAuthClient implements OAuthClient {
  const RealOAuthClient({required this.serverClientId});

  /// Google 的 **Web** client ID，由 `/api/app-config` 下发。
  ///
  /// ⚠️ 这个参数不是可选项。`google_sign_in` 在 Android 上只有拿到它
  /// （或能从 `google-services.json` 生成的 `default_web_client_id` 读到）
  /// 才会返回 `idToken`；本项目两样都没有的那段时间，`auth.idToken` 恒为 null，
  /// 登录在客户端就抛掉了——**服务端日志里一条请求都没有**，查起来毫无线索。
  ///
  /// iOS 另走 Info.plist 的 reversed client ID，签出来的 aud 是 iOS client ID，
  /// 所以服务端那边要能接受多个 aud（`user.splitClientIDs`）。
  final String serverClientId;

  @override
  Future<String> googleIdToken() async {
    if (serverClientId.isEmpty) {
      // 明确报「没配」，而不是让用户点一下没反应。
      throw Exception('未配置 Google Client ID');
    }
    final account = await GoogleSignIn(serverClientId: serverClientId).signIn();
    if (account == null) throw const OAuthCancelled();
    final auth = await account.authentication;
    final token = auth.idToken;
    if (token == null || token.isEmpty) {
      throw Exception('Google 未返回 ID Token');
    }
    return token;
  }

  @override
  Future<String> appleIdToken() async {
    try {
      final cred = await SignInWithApple.getAppleIDCredential(
        scopes: [
          AppleIDAuthorizationScopes.email,
          AppleIDAuthorizationScopes.fullName,
        ],
      );
      final token = cred.identityToken;
      if (token == null || token.isEmpty) {
        throw Exception('Apple 未返回 identityToken');
      }
      return token;
    } on SignInWithAppleAuthorizationException catch (e) {
      if (e.code == AuthorizationErrorCode.canceled) {
        throw const OAuthCancelled();
      }
      rethrow;
    }
  }
}

final oauthClientProvider = Provider<OAuthClient>(
  (ref) => RealOAuthClient(
    serverClientId: ref.watch(appConfigProvider).googleClientId,
  ),
);
