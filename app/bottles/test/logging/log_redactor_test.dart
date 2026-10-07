import 'package:bottles/core/logging/log_redactor.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('redactMap', () {
    test('敏感字段一律替换，不区分大小写', () {
      final out = redactMap({
        'password': 'hunter2',
        'Password': 'hunter2',
        'id_token': 'eyJhbGciOi...',
        'idToken': 'eyJhbGciOi...',
        'token': 'abc',
        'secret': 'shh',
      });
      for (final v in out.values) {
        expect(v, redactedMark);
      }
    });

    test('验证码只在请求方向脱敏，响应里的业务码要留着', () {
      // `code` 一名两用：请求体里是短信验证码，响应体里是 {code,msg,data}
      // 的业务状态码。后者是排查时最关键的一个字段，盖掉就白记了。
      expect(redactMap({'code': '482913'}, isRequest: true)['code'],
          redactedMark);
      expect(redactMap({'code': 3003})['code'], 3003);
      expect(redactMap({'verifyCode': 'x'}, isRequest: true)['verifyCode'],
          redactedMark);
    });

    test('方向标记会传进嵌套结构', () {
      final out = redactMap({
        'user': {'code': '482913'},
      }, isRequest: true);
      expect((out['user'] as Map<String, Object?>)['code'], redactedMark);
    });

    test('非敏感字段原样保留', () {
      final out = redactMap({'content': '今晚的海', 'coins': 60, 'scope': 'local'});
      expect(out['content'], '今晚的海');
      expect(out['coins'], 60);
      expect(out['scope'], 'local');
    });

    test('嵌套结构里的敏感字段同样被替换', () {
      // 登录请求体就是嵌套的，只查一层会漏。
      final out = redactMap({
        'user': {'phone': '+919876543210', 'password': 'hunter2'},
        'list': [
          {'token': 'a'},
          {'ok': 1},
        ],
      });
      final user = out['user'] as Map<String, Object?>;
      expect(user['password'], redactedMark);
      final list = out['list'] as List<Object?>;
      expect((list[0] as Map<String, Object?>)['token'], redactedMark);
      expect((list[1] as Map<String, Object?>)['ok'], 1);
    });

    test('字段名只是包含敏感词也要替换', () {
      // refresh_token / accessToken 这类变体都要覆盖到。
      final out = redactMap({
        'refresh_token': 'x',
        'accessToken': 'y',
      });
      for (final v in out.values) {
        expect(v, redactedMark);
      }
    });
  });

  group('redactHeaders', () {
    test('Authorization 永不记录值，只记是否携带', () {
      final out = redactHeaders({'Authorization': 'Bearer eyJhbGciOi...'});
      // 值本身绝不出现——哪怕截断后的前几位也不行。
      expect(out['Authorization'], isNot(contains('eyJ')));
      expect(out['Authorization'], 'present');
    });

    test('没带 Authorization 时标记 absent', () {
      expect(redactHeaders({})['Authorization'], 'absent');
    });

    test('其余头原样保留', () {
      final out = redactHeaders({'Accept-Language': 'zh-CN'});
      expect(out['Accept-Language'], 'zh-CN');
    });
  });
}
