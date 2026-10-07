import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// 「记住密码」：密码只进 Keychain / Keystore，绝不落 SharedPreferences。
///
/// 只记**最近一次成功登录**的那一组；账号在 [Prefs.loginMemory] 里，
/// 这里只有密码本身。换账号登录成功就覆盖，关掉「记住密码」就清掉。
class CredentialStore {
  CredentialStore([FlutterSecureStorage? storage])
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(encryptedSharedPreferences: true),
            iOptions: IOSOptions(
              accessibility: KeychainAccessibility.first_unlock,
            ),
          );

  static const _kPassword = 'drift.login.password';

  final FlutterSecureStorage _storage;

  Future<String?> readPassword() async {
    try {
      return await _storage.read(key: _kPassword);
    } catch (_) {
      // Keystore 偶发不可用（换机恢复、系统升级后首启）——当作没记住，不影响登录。
      return null;
    }
  }

  Future<void> writePassword(String password) async {
    try {
      await _storage.write(key: _kPassword, value: password);
    } catch (_) {}
  }

  Future<void> clearPassword() async {
    try {
      await _storage.delete(key: _kPassword);
    } catch (_) {}
  }
}
