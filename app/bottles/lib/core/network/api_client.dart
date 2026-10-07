import 'package:dio/dio.dart';

import '../config/app_config.dart';
import '../storage/token_store.dart';
import 'api_exception.dart';
import 'logging_interceptor.dart';

/// 统一网络出口。
///
/// 后端响应恒为 `{code, msg, data}`，这里解包成 `data`，非 0 码抛 [ApiException]，
/// 页面层只处理业务数据与错误码，不碰 HTTP 细节。
class ApiClient {
  ApiClient(this._tokenStore, {required this.onUnauthorized, Dio? dio})
      : _dio = dio ?? Dio() {
    _dio.options
      ..baseUrl = AppConfig.apiBase
      ..connectTimeout = AppConfig.connectTimeout
      ..receiveTimeout = AppConfig.receiveTimeout
      ..responseType = ResponseType.json;

    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _tokenStore.read();
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          options.headers['Accept-Language'] = languageTag;
          handler.next(options);
        },
      ),
    );

    // 顺序要紧：auth 拦截器先跑，日志拦截器才看得到带上 token 后的头
    // （它只记「有没有带」，不记值）。
    _dio.interceptors.add(LoggingInterceptor());
  }

  final Dio _dio;
  final TokenStore _tokenStore;

  /// 401 / 403 时回调宿主，清登录态并跳登录页。
  final Future<void> Function() onUnauthorized;

  /// 后端按 `Accept-Language` 下发文案（错误消息 / sysconfig `.en` 变体 / 通知模板）。
  String languageTag = 'zh-CN';

  Future<T> get<T>(String path, {Map<String, dynamic>? query}) =>
      _request<T>(() => _dio.get(path, queryParameters: query));

  Future<T> post<T>(String path, {Object? body, Map<String, dynamic>? query}) =>
      _request<T>(() => _dio.post(path, data: body, queryParameters: query));

  Future<T> delete<T>(String path, {Object? body}) =>
      _request<T>(() => _dio.delete(path, data: body));

  Future<T> upload<T>(String path, FormData form) =>
      _request<T>(() => _dio.post(path, data: form));

  Future<T> _request<T>(Future<Response<dynamic>> Function() send) async {
    late final Response<dynamic> res;
    try {
      res = await send();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        await onUnauthorized();
        throw ApiException(ErrCode.unauthorized, e.message ?? 'unauthorized');
      }
      throw ApiException.network(e.message ?? 'network error');
    }

    final body = res.data;
    if (body is! Map) {
      // 非包裹格式（例如静态资源代理），原样返回。
      return body as T;
    }

    final code = (body['code'] as num?)?.toInt() ?? 0;
    if (code != 0) {
      // 只有登录态失效才踢回登录页。forbidden(1004) 还覆盖封禁、功能开关关闭等，
      // 一律当掉线会把用户莫名其妙地登出。
      if (code == ErrCode.unauthorized) {
        await onUnauthorized();
      }
      throw ApiException(
        code,
        (body['msg'] ?? body['message'] ?? '') as String,
        data: body['data'],
      );
    }
    return body['data'] as T;
  }
}
