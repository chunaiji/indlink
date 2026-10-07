import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// JWT 存 Keychain / Keystore，不落 SharedPreferences。
class TokenStore {
  TokenStore([FlutterSecureStorage? storage])
      : _storage = storage ??
            const FlutterSecureStorage(
              // Android 走 EncryptedSharedPreferences（Keystore 托管密钥），
              // iOS 用 first_unlock 以便后台任务也能读到 JWT。
              aOptions: AndroidOptions(encryptedSharedPreferences: true),
              iOptions: IOSOptions(
                accessibility: KeychainAccessibility.first_unlock,
              ),
            );

  static const _kToken = 'drift.jwt';

  final FlutterSecureStorage _storage;

  /// 内存缓存，避免每次请求都读 Keychain。
  String? _cached;

  String? get cached => _cached;

  Future<String?> read() async {
    _cached ??= await _storage.read(key: _kToken);
    return _cached;
  }

  Future<void> write(String token) async {
    _cached = token;
    await _storage.write(key: _kToken, value: token);
  }

  Future<void> clear() async {
    _cached = null;
    await _storage.delete(key: _kToken);
  }
}
