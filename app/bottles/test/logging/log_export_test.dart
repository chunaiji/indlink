import 'package:bottles/core/logging/log_export.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('JSONL 转成人能读的一行', () {
    final s = formatLogLine({
      't': '2026-09-19T17:23:41.882+08:00',
      'lv': 'error',
      'tag': 'net',
      'msg': 'POST /bottle/create',
      'http': 200,
      'code': 3003,
      'ms': 412,
      'sid': 'a3f27b19',
    });
    expect(s, contains('17:23:41'));
    expect(s, contains('[ERROR]'));
    expect(s, contains('[net]'));
    expect(s, contains('POST /bottle/create'));
    expect(s, contains('code=3003'));
    expect(s, contains('412ms'));
    // sid 不重复出现在每一行——导出文件头部写一次就够。
    expect(s, isNot(contains('a3f27b19')));
  });

  test('缺字段时不崩，能出多少出多少', () {
    final s = formatLogLine({'msg': 'bare'});
    expect(s, contains('bare'));
  });

  test('坏行原样返回，不吞掉', () {
    // 文件可能被写坏（断电 / 杀进程），这时宁可显示原始内容也不要静默丢弃。
    final s = formatLogLine({'raw': '{不是合法 JSON'});
    expect(s, contains('不是合法 JSON'));
  });
}
