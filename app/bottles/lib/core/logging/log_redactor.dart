/// 脱敏后的占位值。
const redactedMark = '***';

/// 字段名命中其中任意一个子串（不区分大小写）即脱敏。
///
/// 用**子串**而不是精确匹配，是为了覆盖 `refresh_token` / `accessToken` /
/// `verifyCode` 这类变体——精确匹配的名单永远追不上新接口。
/// 代价是可能误伤（比如某个业务字段叫 `barcode`，含 `code`），
/// 但**误伤的后果是少记一条信息，漏网的后果是泄露凭证**，这个取舍不用犹豫。
const _sensitiveParts = <String>[
  'password',
  'passwd',
  'token',
  'secret',
];

/// 只在**请求**方向脱敏的字段名。
///
/// `code` 在本项目里一名两用：请求体里是短信验证码（必须盖掉），
/// 响应体里是 `{code, msg, data}` 的业务状态码（最重要的排查字段，盖掉就白记了）。
/// 两者同名，只能靠方向区分——验证码只会出现在请求里。
const _requestOnlyParts = <String>['code'];

bool _isSensitive(String key, {required bool isRequest}) {
  final k = key.toLowerCase();
  if (_sensitiveParts.any(k.contains)) return true;
  return isRequest && _requestOnlyParts.any(k.contains);
}

/// 递归脱敏。嵌套结构里的敏感字段同样替换——登录请求体就是嵌套的。
///
/// [isRequest] 为 true 时额外盖掉只在请求里敏感的字段，见 [_requestOnlyParts]。
Map<String, Object?> redactMap(
  Map<String, Object?> input, {
  bool isRequest = false,
}) {
  final out = <String, Object?>{};
  input.forEach((k, v) {
    if (_isSensitive(k, isRequest: isRequest)) {
      out[k] = redactedMark;
      return;
    }
    out[k] = _redactValue(v, isRequest);
  });
  return out;
}

Object? _redactValue(Object? v, bool isRequest) {
  if (v is Map) {
    return redactMap(
      v.map((k, val) => MapEntry(k.toString(), val)),
      isRequest: isRequest,
    );
  }
  if (v is List) return v.map((e) => _redactValue(e, isRequest)).toList();
  return v;
}

/// 请求头脱敏。
///
/// `Authorization` 的值**永不记录**，连截断后的前几位也不记——
/// JWT 的头部是固定的 `eyJ`，记前几位没有任何诊断价值，
/// 却会让人误以为「记一点没关系」。只记有没有带。
Map<String, String> redactHeaders(Map<String, dynamic> headers) {
  final out = <String, String>{};
  headers.forEach((k, v) {
    if (k.toLowerCase() == 'authorization') return;
    out[k] = v.toString();
  });
  final has = headers.keys.any((k) => k.toLowerCase() == 'authorization');
  out['Authorization'] = has ? 'present' : 'absent';
  return out;
}
