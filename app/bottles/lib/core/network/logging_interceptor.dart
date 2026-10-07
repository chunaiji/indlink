import 'package:dio/dio.dart';

import '../logging/log_record.dart';
import '../logging/log_redactor.dart';
import '../logging/logger.dart';

/// 记录每一次网络请求。
///
/// **要点：业务错误码不是 HTTP 错误。** `{"code":3003}` 走的是 HTTP 200，
/// `onError` 根本不触发，所以必须在 `onResponse` 里解一层 `data['code']`。
/// 一条日志同时带两者——只看 HTTP 全是 200 看不出业务失败，
/// 只看业务码又看不出超时与连接失败（spec §4.1）。
///
/// 它挂在 `ApiClient._request` 的 try/catch **下面**，因此能看到完整的
/// `DioException`——那些信息在上层会被压成一句 message 丢掉。
class LoggingInterceptor extends Interceptor {
  static const _startKey = 'log_start_ms';

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    options.extra[_startKey] = DateTime.now().millisecondsSinceEpoch;
    handler.next(options);
  }

  int _elapsed(RequestOptions o) {
    final start = o.extra[_startKey];
    if (start is! int) return -1;
    return DateTime.now().millisecondsSinceEpoch - start;
  }

  Map<String, Object?> _base(RequestOptions o) => {
        'm': o.method,
        'path': o.path,
        'ms': _elapsed(o),
      };

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) {
    final res = response;
    final f = _base(res.requestOptions)..['http'] = res.statusCode;

    final body = res.data;
    final code = body is Map ? (body['code'] as num?)?.toInt() : null;
    if (code != null) f['code'] = code;

    if (Log.config.logBody) {
      f['req'] = _safeBody(res.requestOptions.data, isRequest: true);
      f['res'] = _safeBody(body);
      f['hdr'] = redactHeaders(res.requestOptions.headers);
    }

    final msg = '${res.requestOptions.method} ${res.requestOptions.path}';
    // 业务码非 0 是 warn 而不是 info：它是失败，只是 HTTP 层没体现。
    if (code != null && code != 0) {
      Log.w(LogTag.net, msg, fields: f);
    } else {
      Log.i(LogTag.net, msg, fields: f);
    }
    handler.next(res);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final f = _base(err.requestOptions)
      ..['http'] = err.response?.statusCode
      ..['type'] = err.type.name;
    if (Log.config.logBody) {
      f['res'] = _safeBody(err.response?.data);
    }
    Log.e(
      LogTag.net,
      '${err.requestOptions.method} ${err.requestOptions.path}',
      error: err.message,
      fields: f,
    );
    handler.next(err);
  }

  /// body 一律过脱敏，并截断——单条日志不该因为一个大响应撑到几百 KB。
  ///
  /// `FormData`（图片上传）不展开：里面是字节流，记进日志既没用又会撑爆文件。
  Object? _safeBody(Object? body, {bool isRequest = false}) {
    if (body == null) return null;
    if (body is FormData) {
      return '<FormData fields=${body.fields.length} files=${body.files.length}>';
    }
    if (body is Map) {
      return redactMap(
        body.map((k, v) => MapEntry(k.toString(), v)),
        isRequest: isRequest,
      );
    }
    final s = body.toString();
    return s.length > 2000 ? '${s.substring(0, 2000)}…' : s;
  }
}
